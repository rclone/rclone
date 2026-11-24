package filen

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	sdk "github.com/FilenCloudDienste/filen-sdk-go/filen"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/client"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/crypto"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/types"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig(t *testing.T) {
	ctx := context.Background()

	t.Run("AsksFor2FAWithoutAPIKey", func(t *testing.T) {
		m := configmap.Simple{
			"email":    "user@example.com",
			"password": obscure.MustObscure("secret"),
		}
		out, err := Config(ctx, "test", m, fs.ConfigIn{})
		require.NoError(t, err)
		require.NotNil(t, out)
		assert.Equal(t, "login", out.State)
		require.NotNil(t, out.Option)
		assert.Equal(t, "config_2fa", out.Option.Name)
		assert.False(t, out.Option.Required)
	})

	t.Run("AsksFor2FAWithBlankedAPIKey", func(t *testing.T) {
		m := configmap.Simple{
			"email":    "user@example.com",
			"password": obscure.MustObscure("secret"),
			"api_key":  obscure.MustObscure(""),
		}
		out, err := Config(ctx, "test", m, fs.ConfigIn{})
		require.NoError(t, err)
		require.NotNil(t, out)
		assert.Equal(t, "login", out.State)
	})

	t.Run("DoneWithoutCredentials", func(t *testing.T) {
		for _, m := range []configmap.Simple{
			{},
			{"email": "user@example.com"},
			{"password": obscure.MustObscure("secret")},
		} {
			out, err := Config(ctx, "test", m, fs.ConfigIn{})
			require.NoError(t, err)
			assert.Nil(t, out)
		}
	})

	t.Run("DoneWithAPIKey", func(t *testing.T) {
		m := configmap.Simple{
			"email":    "user@example.com",
			"password": obscure.MustObscure("secret"),
			"api_key":  obscure.MustObscure("key"),
		}
		out, err := Config(ctx, "test", m, fs.ConfigIn{})
		require.NoError(t, err)
		assert.Nil(t, out)
	})

	t.Run("DoneWithInternalPassword", func(t *testing.T) {
		m := configmap.Simple{
			"email":    "user@example.com",
			"password": obscure.MustObscure("INTERNAL"),
		}
		out, err := Config(ctx, "test", m, fs.ConfigIn{})
		require.NoError(t, err)
		assert.Nil(t, out)
	})

	t.Run("UnknownState", func(t *testing.T) {
		_, err := Config(ctx, "test", configmap.Simple{}, fs.ConfigIn{State: "bogus"})
		assert.ErrorContains(t, err, `unknown state "bogus"`)
	})
}

// fakeIngest stands in for the ingest server. The first upload of each index
// in fail is cut off with a retryable transport error.
type fakeIngest struct {
	mu       sync.Mutex
	fail     map[int]bool
	attempts map[int]int
	bodies   map[int][]byte
}

func (fi *fakeIngest) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	index, err := strconv.Atoi(r.URL.Query().Get("index"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	fi.mu.Lock()
	fi.attempts[index]++
	cutOff := fi.fail[index] && fi.attempts[index] == 1
	if !cutOff {
		fi.bodies[index] = body
	}
	fi.mu.Unlock()
	if cutOff {
		// The client fails reading the response with "unexpected EOF reading trailer".
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_, _ = conn.Write([]byte("HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\nTrailer: X-Test\r\n\r\n0\r\n"))
		_ = conn.Close()
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":true,"data":{"bucket":"b","region":"r"}}`))
}

// connectProxy tunnels every CONNECT request to target, so requests for the
// real ingest host reach a local server.
func connectProxy(target string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect {
			http.Error(w, "CONNECT only", http.StatusMethodNotAllowed)
			return
		}
		upstream, err := net.Dial("tcp", target)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			_ = upstream.Close()
			return
		}
		_, _ = conn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
		go func() {
			_, _ = io.Copy(upstream, conn)
			_ = upstream.Close()
		}()
		_, _ = io.Copy(conn, upstream)
		_ = conn.Close()
	})
}

// newRetryTestChunkWriter returns a chunkWriter for chunks of chunkSize that
// uploads to a local fake ingest server, where the first upload of each index
// in fail is cut off.
func newRetryTestChunkWriter(t *testing.T, chunkSize int64, fail map[int]bool) (context.Context, *chunkWriter, *fakeIngest) {
	ingest := &fakeIngest{
		fail:     fail,
		attempts: map[int]int{},
		bodies:   map[int][]byte{},
	}
	ingestSrv := httptest.NewTLSServer(ingest)
	t.Cleanup(ingestSrv.Close)
	proxySrv := httptest.NewServer(connectProxy(ingestSrv.Listener.Addr().String()))
	t.Cleanup(proxySrv.Close)

	ctx, ci := fs.AddConfig(context.Background())
	ci.HTTPProxy = proxySrv.URL
	ci.InsecureSkipVerify = true
	ci.DisableHTTP2 = true // so the ingest server can hijack the connection

	filenSDK := &sdk.Filen{
		Client:                client.NewWithAPIKey(ctx, "test-api-key"),
		FileEncryptionVersion: 3,
	}
	incFile, err := types.NewIncompleteFile(crypto.FileEncryptionVersion(3), "test.bin", "application/octet-stream", time.Now(), time.Now(), types.NewRootDirectory("root-uuid"))
	require.NoError(t, err)
	cw := &chunkWriter{
		FileUpload:      *filenSDK.NewFileUpload(incFile),
		filen:           filenSDK,
		pacer:           fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(0), pacer.MaxSleep(0))),
		chunkSize:       chunkSize,
		bucketAndRegion: make(chan client.V3UploadResponse, 1),
		knownChunks:     make(map[int64][]byte),
	}
	return ctx, cw, ingest
}

func TestWriteChunkRetry(t *testing.T) {
	// two full sub-chunks and a partial last one, the last two failing once
	ctx, cw, ingest := newRetryTestChunkWriter(t, 3*sdk.ChunkSize, map[int]bool{1: true, 2: true})

	// Random data, so a retry that re-reads the wrong part of the chunk is caught.
	plaintext := make([]byte, 2*sdk.ChunkSize+1000)
	_, _ = rand.New(rand.NewSource(1)).Read(plaintext)

	// Retries must not read the source again, which may have changed since.
	n, err := cw.WriteChunk(ctx, 0, noSeekReader{bytes.NewReader(plaintext)})
	require.NoError(t, err)
	assert.Equal(t, int64(len(plaintext)), n)

	// The file hash must cover the plaintext that was uploaded.
	wantHash := cw.filen.NewFileUpload(&cw.IncompleteFile).Hasher
	_, _ = wantHash.Write(plaintext)
	assert.Equal(t, wantHash.Sum(nil), cw.Hasher.Sum(nil), "file hash")
	assert.Empty(t, cw.knownChunks)

	ingest.mu.Lock()
	defer ingest.mu.Unlock()
	assert.Equal(t, map[int]int{0: 1, 1: 2, 2: 2}, ingest.attempts)
	for index := range 3 {
		decrypted, err := cw.EncryptionKey.DecryptData(ingest.bodies[index])
		require.NoError(t, err, "chunk %d", index)
		want := plaintext[index*sdk.ChunkSize : min((index+1)*sdk.ChunkSize, len(plaintext))]
		assert.True(t, bytes.Equal(want, decrypted), "chunk %d doesn't decrypt to its plaintext", index)
	}
}

// noSeekReader is an io.ReadSeeker that can't seek.
type noSeekReader struct {
	io.Reader
}

func (noSeekReader) Seek(offset int64, whence int) (int64, error) {
	return 0, errors.New("noSeekReader: Seek called")
}

func TestWriteChunkOutOfOrder(t *testing.T) {
	const chunkSize = 2 * sdk.ChunkSize
	plaintext := make([]byte, 7*chunkSize+1000) // 8 chunks, 15 sub-chunks
	_, _ = rand.New(rand.NewSource(2)).Read(plaintext)
	numChunks := (len(plaintext) + chunkSize - 1) / chunkSize
	numSubChunks := (len(plaintext) + sdk.ChunkSize - 1) / sdk.ChunkSize
	fail := map[int]bool{}
	for i := 0; i < numSubChunks; i += 3 {
		fail[i] = true
	}
	ctx, cw, ingest := newRetryTestChunkWriter(t, chunkSize, fail)

	writeChunk := func(chunk int) {
		data := plaintext[chunk*chunkSize : min((chunk+1)*chunkSize, len(plaintext))]
		n, err := cw.WriteChunk(ctx, chunk, bytes.NewReader(data))
		assert.NoError(t, err, "chunk %d", chunk)
		assert.Equal(t, int64(len(data)), n, "chunk %d", chunk)
	}
	// Write every chunk but the first concurrently, so they all have to wait in
	// knownChunks to be hashed until the first is written.
	var wg sync.WaitGroup
	for chunk := numChunks - 1; chunk > 0; chunk-- {
		wg.Add(1)
		go func() {
			defer wg.Done()
			writeChunk(chunk)
		}()
	}
	wg.Wait()
	writeChunk(0)

	wantHash := cw.filen.NewFileUpload(&cw.IncompleteFile).Hasher
	_, _ = wantHash.Write(plaintext)
	assert.Equal(t, wantHash.Sum(nil), cw.Hasher.Sum(nil), "file hash")
	assert.Empty(t, cw.knownChunks)
	assert.Equal(t, int64(len(plaintext)), cw.size)

	ingest.mu.Lock()
	defer ingest.mu.Unlock()
	require.Equal(t, numSubChunks, len(ingest.bodies))
	for index := range numSubChunks {
		decrypted, err := cw.EncryptionKey.DecryptData(ingest.bodies[index])
		require.NoError(t, err, "sub-chunk %d", index)
		want := plaintext[index*sdk.ChunkSize : min((index+1)*sdk.ChunkSize, len(plaintext))]
		assert.True(t, bytes.Equal(want, decrypted), "sub-chunk %d doesn't decrypt to its plaintext", index)
		wantAttempts := 1
		if fail[index] {
			wantAttempts = 2
		}
		assert.Equal(t, wantAttempts, ingest.attempts[index], "attempts for sub-chunk %d", index)
	}
}
