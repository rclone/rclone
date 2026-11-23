package filen

import (
	"bytes"
	"context"
	"encoding/hex"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
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

func TestWriteChunkRetryDoesNotDoubleEncrypt(t *testing.T) {
	var attempts atomic.Int32
	var storedBody []byte

	ingestHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if attempts.Add(1) == 1 {
			// Force a retryable transport error ("unexpected EOF reading trailer") on the first attempt.
			hj, ok := w.(http.Hijacker)
			if ok {
				conn, _, err := hj.Hijack()
				if err == nil {
					_, _ = conn.Write([]byte("HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\nTrailer: X-Test\r\n\r\n0\r\n"))
					_ = conn.Close()
					return
				}
			}
		}
		storedBody = body
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":true,"data":{"bucket":"b1","region":"r1"}}`))
	})

	tlsSrv := httptest.NewTLSServer(ingestHandler)
	defer tlsSrv.Close()

	// Proxy CONNECT requests to the local TLS ingest test server so no external DNS or network is used.
	proxySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodConnect, r.Method)
		upstream, err := net.Dial("tcp", tlsSrv.Listener.Addr().String())
		require.NoError(t, err)

		hj, ok := w.(http.Hijacker)
		require.True(t, ok)
		clientConn, _, err := hj.Hijack()
		require.NoError(t, err)

		_, err = clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
		require.NoError(t, err)

		go func() {
			_, _ = io.Copy(upstream, clientConn)
			_ = upstream.Close()
		}()
		go func() {
			_, _ = io.Copy(clientConn, upstream)
			_ = clientConn.Close()
		}()
	}))
	defer proxySrv.Close()

	ctx, ci := fs.AddConfig(context.Background())
	ci.HTTPProxy = proxySrv.URL
	ci.InsecureSkipVerify = true
	ci.DisableHTTP2 = true

	filenSDK := &sdk.Filen{
		Client:                client.NewWithAPIKey(ctx, "test-api-key"),
		FileEncryptionVersion: 3,
	}

	parent := types.NewRootDirectory("root-uuid")
	for _, tc := range []struct {
		name string
		size int
	}{
		{name: "PartialChunk", size: 1024},
		{name: "FullChunk", size: sdk.ChunkSize},
	} {
		t.Run(tc.name, func(t *testing.T) {
			attempts.Store(0)
			storedBody = nil

			incFile, err := types.NewIncompleteFile(crypto.FileEncryptionVersion(3), "test.bin", "application/octet-stream", time.Now(), time.Now(), parent)
			require.NoError(t, err)

			fu := filenSDK.NewFileUpload(incFile)
			cw := &chunkWriter{
				FileUpload:      *fu,
				filen:           filenSDK,
				pacer:           fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(0), pacer.MaxSleep(0))),
				chunkSize:       sdk.ChunkSize,
				bucketAndRegion: make(chan client.V3UploadResponse, 1),
				knownChunks:     make(map[int64][]byte),
			}

			plaintext := bytes.Repeat([]byte("PLAINTEXT-CHUNK-"), tc.size/16)
			n, err := cw.WriteChunk(ctx, 0, bytes.NewReader(plaintext))
			require.NoError(t, err)
			assert.Equal(t, int64(len(plaintext)), n)
			assert.Equal(t, int32(2), attempts.Load())

			decrypted, err := cw.EncryptionKey.DecryptData(storedBody)
			require.NoError(t, err)
			assert.Equal(t, plaintext, decrypted, "retried chunk should decrypt to original plaintext, not double-encrypted data")

			expectedHasher := filenSDK.NewFileUpload(incFile).Hasher
			_, err = expectedHasher.Write(plaintext)
			require.NoError(t, err)
			assert.Equal(t, hex.EncodeToString(expectedHasher.Sum(nil)), hex.EncodeToString(cw.Hasher.Sum(nil)))
		})
	}
}
