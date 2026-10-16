package s3

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/config/configfile"
	fslog "github.com/rclone/rclone/fs/log"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fstest/mockfs"
	"github.com/rclone/rclone/fstest/mockobject"
	"github.com/rclone/rclone/lib/random"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const directFetchCopySourceURL = "https://source.example/object?X-Amz-Signature=offline-fixture"

type directFetchCopySourceObject struct {
	*mockobject.ContentMockObject
	opens    atomic.Int32
	metadata fs.Metadata
}

func (o *directFetchCopySourceObject) Metadata(context.Context) (fs.Metadata, error) {
	return o.metadata, nil
}

func (o *directFetchCopySourceObject) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	o.opens.Add(1)
	return o.ContentMockObject.Open(ctx, options...)
}

type directFetchCopySDKFixture struct {
	mode    string
	payload []byte
	started chan struct{}
	once    sync.Once

	mu           sync.Mutex
	stored       []byte
	directBodies [][]byte
	manualBodies [][]byte
	headerOnPut  []bool
}

func (f *directFetchCopySDKFixture) snapshot() (stored []byte, directBodies, manualBodies [][]byte, headerOnPut []bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	copyBodies := func(in [][]byte) [][]byte {
		out := make([][]byte, len(in))
		for i := range in {
			out[i] = append([]byte(nil), in[i]...)
		}
		return out
	}
	return append([]byte(nil), f.stored...), copyBodies(f.directBodies), copyBodies(f.manualBodies), append([]bool(nil), f.headerOnPut...)
}

func (f *directFetchCopySDKFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "body read failed", http.StatusInternalServerError)
		return
	}

	f.mu.Lock()
	stored := append([]byte(nil), f.stored...)
	f.mu.Unlock()

	switch r.Method {
	case http.MethodPut:
		direct := r.Header.Get(directFetchSourceHeader) != ""
		f.mu.Lock()
		f.headerOnPut = append(f.headerOnPut, direct)
		if direct {
			f.directBodies = append(f.directBodies, append([]byte(nil), body...))
		} else {
			f.manualBodies = append(f.manualBodies, append([]byte(nil), body...))
		}
		f.mu.Unlock()
		if direct {
			switch f.mode {
			case "fallback":
				w.Header().Set("Content-Type", "application/xml")
				w.Header().Set(directFetchErrorHeader, "DirectFetchSourceStatus 400")
				w.WriteHeader(http.StatusBadRequest)
				_, _ = io.WriteString(w, `<Error><Code>DirectFetchSourceStatus</Code><Message>400</Message></Error>`)
				return
			case "missing-length":
				w.Header().Set("Content-Type", "application/xml")
				w.Header().Set(directFetchErrorHeader, "DirectFetchMissingContentLength")
				w.WriteHeader(http.StatusBadRequest)
				_, _ = io.WriteString(w, `<Error><Code>DirectFetchMissingContentLength</Code><Message>Source response missing content-length header</Message></Error>`)
				return
			case "auth":
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(http.StatusForbidden)
				_, _ = io.WriteString(w, `<Error><Code>AccessDenied</Code><Message>destination denied</Message></Error>`)
				return
			case "cancel":
				f.once.Do(func() { close(f.started) })
				<-r.Context().Done()
				return
			default:
				f.mu.Lock()
				f.stored = append([]byte(nil), f.payload...)
				stored = append([]byte(nil), f.stored...)
				f.mu.Unlock()
			}
		} else {
			f.mu.Lock()
			f.stored = append([]byte(nil), body...)
			stored = append([]byte(nil), f.stored...)
			f.mu.Unlock()
		}
		sum := md5.Sum(stored)
		w.Header().Set("ETag", fmt.Sprintf(`"%x"`, sum))
	case http.MethodHead:
		w.Header().Set("Content-Length", strconv.Itoa(len(stored)))
		w.Header().Set("Last-Modified", "Wed, 01 Jan 2025 00:00:00 GMT")
		sum := md5.Sum(stored)
		w.Header().Set("ETag", fmt.Sprintf(`"%x"`, sum))
	case http.MethodGet:
		w.Header().Set("Content-Length", strconv.Itoa(len(stored)))
		w.Header().Set("Last-Modified", "Wed, 01 Jan 2025 00:00:00 GMT")
		_, _ = w.Write(stored)
	default:
		http.Error(w, "unexpected request", http.StatusBadRequest)
	}
}

func captureDirectFetchCopyLogs(run func()) string {
	var mu sync.Mutex
	var logs strings.Builder
	ci := fs.GetConfig(context.Background())
	oldConfigLevel := ci.LogLevel
	ci.LogLevel = fs.LogLevelDebug
	oldLevel := fslog.Handler.SetLevel(slog.LevelDebug)
	fslog.Handler.SetOutput(func(_ slog.Level, text string) {
		mu.Lock()
		defer mu.Unlock()
		logs.WriteString(text)
	})
	defer func() {
		fslog.Handler.ResetOutput()
		fslog.Handler.SetLevel(oldLevel)
		ci.LogLevel = oldConfigLevel
	}()
	run()
	mu.Lock()
	defer mu.Unlock()
	return logs.String()
}

func newDirectFetchCopySource(ctx context.Context, t *testing.T, payload []byte) (*mockfs.Fs, *directFetchCopySourceObject) {
	t.Helper()
	base, err := mockfs.NewFs(ctx, "direct-fetch-source", "source-root", nil)
	require.NoError(t, err)
	source := base.(*mockfs.Fs)
	source.Features().PublicLinkIsDirect = true
	source.Features().PublicLink = func(context.Context, string, fs.Duration, bool) (string, error) {
		return directFetchCopySourceURL, nil
	}
	src := &directFetchCopySourceObject{ContentMockObject: mockobject.New("source.bin").WithContent(payload, mockobject.SeekModeNone)}
	src.SetFs(source)
	return source, src
}

func readDirectFetchCopyObject(ctx context.Context, t *testing.T, got fs.Object) []byte {
	t.Helper()
	in, err := got.Open(ctx)
	require.NoError(t, err)
	data, readErr := io.ReadAll(in)
	closeErr := in.Close()
	require.NoError(t, readErr)
	require.NoError(t, closeErr)
	return data
}

func TestDirectFetchCopyThroughS3SDK(t *testing.T) {
	payload := []byte("offline direct fetch payload\n")
	for _, tc := range []struct {
		name             string
		mode             string
		wantError        bool
		wantOpens        int32
		wantBytes        int64
		wantServerCopies int64
		wantServerBytes  int64
		wantLog          string
		wantHeaderOnPut  []bool
		metadata         fs.Metadata
	}{
		{name: "success", wantBytes: int64(len(payload)), wantServerCopies: 1, wantServerBytes: int64(len(payload)), wantLog: "Copied (server-side URL fetch)", wantHeaderOnPut: []bool{true}},
		{name: "source rejection falls back", mode: "fallback", wantError: false, wantOpens: 1, wantBytes: int64(len(payload)), wantLog: "Copied (new)", wantHeaderOnPut: []bool{true, false}},
		{name: "missing source length falls back", mode: "missing-length", wantOpens: 1, wantBytes: int64(len(payload)), wantLog: "Copied (new)", wantHeaderOnPut: []bool{true, false}},
		{name: "content-encoded source uses ordinary copy", metadata: fs.Metadata{"content-encoding": "gzip"}, wantOpens: 1, wantBytes: int64(len(payload)), wantLog: "Copied (new)", wantHeaderOnPut: []bool{false}},
		{name: "destination auth is final", mode: "auth", wantError: true},
		{name: "cancellation is final", mode: "cancel", wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, ci := fs.AddConfig(context.Background())
			ci.Inplace = true
			ci.LowLevelRetries = 1
			ci.MultiThreadStreams = 0
			ci.LogLevel = fs.LogLevelDebug
			group := "direct-fetch-copy-" + random.String(12)
			ctx = accounting.WithStatsGroup(ctx, group)
			accounting.NewStatsGroup(ctx, group)
			_, src := newDirectFetchCopySource(ctx, t, payload)
			src.metadata = tc.metadata
			fixture := &directFetchCopySDKFixture{mode: tc.mode, payload: payload, started: make(chan struct{})}
			dst := newDirectFetchTestFs(ctx, t, "Fastly", fixture)

			copyCtx := ctx
			var cancel context.CancelFunc
			if tc.mode == "cancel" {
				copyCtx, cancel = context.WithCancel(ctx)
				defer cancel()
			}
			var got fs.Object
			var copyErr error
			logs := captureDirectFetchCopyLogs(func() {
				if tc.mode != "cancel" {
					got, copyErr = operations.Copy(copyCtx, dst, nil, "target.bin", src)
					return
				}
				result := make(chan struct{})
				go func() {
					got, copyErr = operations.Copy(copyCtx, dst, nil, "target.bin", src)
					close(result)
				}()
				select {
				case <-fixture.started:
					cancel()
				case <-time.After(5 * time.Second):
					t.Fatal("timed out waiting for the SDK request")
				}
				select {
				case <-result:
				case <-time.After(5 * time.Second):
					t.Fatal("timed out waiting for canceled copy")
				}
			})

			stored, directBodies, manualBodies, headerOnPut := fixture.snapshot()
			assert.Equal(t, tc.wantOpens, src.opens.Load())
			stats := accounting.Stats(ctx)
			remoteStats, statsErr := stats.RemoteStats(false)
			require.NoError(t, statsErr)
			assert.Equal(t, tc.wantBytes, stats.GetBytes())
			assert.Equal(t, tc.wantServerCopies, remoteStats["serverSideCopies"])
			assert.Equal(t, tc.wantServerBytes, remoteStats["serverSideCopyBytes"])

			if tc.wantError {
				require.Error(t, copyErr)
				assert.Nil(t, got)
				assert.Empty(t, manualBodies)
				assert.NotContains(t, logs, "Copied (new)")
				if tc.mode == "cancel" {
					assert.ErrorIs(t, copyErr, context.Canceled)
				} else {
					assert.NotErrorIs(t, copyErr, fs.ErrorCantCopy)
				}
				return
			}

			require.NoError(t, copyErr)
			require.NotNil(t, got)
			if strings.Contains(logs, directFetchCopySourceURL) {
				t.Fatal("copy logs exposed the source URL")
			}
			assert.Equal(t, payload, stored)
			assert.Equal(t, payload, readDirectFetchCopyObject(ctx, t, got))
			assert.Contains(t, logs, tc.wantLog)
			require.Equal(t, tc.wantHeaderOnPut, headerOnPut)
			for _, body := range directBodies {
				assert.Empty(t, body, "Direct Fetch must use a body-free PutObject")
			}
			if tc.wantOpens > 0 {
				require.Len(t, manualBodies, 1)
				assert.Equal(t, payload, manualBodies[0])
			} else {
				assert.Empty(t, manualBodies)
			}
		})
	}
}

func TestDirectFetchLive(t *testing.T) {
	sourceName := os.Getenv("RCLONE_DIRECT_FETCH_SOURCE")
	destName := os.Getenv("RCLONE_DIRECT_FETCH_DEST")
	if sourceName == "" || destName == "" {
		t.Skip("set explicit disposable source/destination paths to run Direct Fetch acceptance")
	}
	configfile.Install()
	ctx, ci := fs.AddConfig(context.Background())
	ci.LowLevelRetries = 1
	source, err := fs.NewFs(ctx, sourceName)
	if err != nil {
		t.Fatal("failed to open the explicitly configured source path")
	}
	dest, err := fs.NewFs(ctx, destName)
	if err != nil {
		t.Fatal("failed to open the explicitly configured destination path")
	}
	sourceS3, sourceOK := source.(*Fs)
	destS3, destOK := dest.(*Fs)
	if !sourceOK || !destOK {
		t.Fatal("live Direct Fetch acceptance requires S3 source and destination remotes")
	}
	if sourceS3.rootBucket == "" || sourceS3.rootDirectory == "" || destS3.rootBucket == "" || destS3.rootDirectory == "" {
		t.Fatal("live Direct Fetch acceptance requires explicit bucket and disposable prefix paths")
	}
	if sourceS3.Name() == destS3.Name() && sourceS3.rootBucket == destS3.rootBucket && sourceS3.rootDirectory == destS3.rootDirectory {
		t.Fatal("live Direct Fetch source and destination paths must be distinct")
	}
	if !source.Features().PublicLinkIsDirect || source.Features().PublicLink == nil {
		t.Fatal("live source is not eligible for direct public links")
	}
	if dest.Features().ServerSideFetchURL == nil {
		t.Fatal("live destination does not support Direct Fetch")
	}

	runDirectFetchLiveCases(t, ctx, source, dest)
	runDirectFetchLiveErrorCases(ctx, t, sourceS3, destS3)
	runDirectFetchLiveOrdinaryCases(ctx, t, source, dest, destName)
	runDirectFetchLiveLargeCase(ctx, t, source, destS3)
}

//nolint:revive // Keep the acceptance helper signature aligned with the documented live-test scaffold.
func runDirectFetchLiveCases(t *testing.T, ctx context.Context, source, dest fs.Fs) {
	ctx, ci := fs.AddConfig(ctx)
	ci.Metadata = true
	var compressed bytes.Buffer
	zw := gzip.NewWriter(&compressed)
	_, err := zw.Write([]byte("Direct Fetch gzip acceptance\n"))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	cases := []struct {
		name           string
		data           []byte
		options        []fs.OpenOption
		contentEncoded bool
	}{
		{name: "zero", data: nil},
		{name: "binary", data: []byte{0, 1, 2, 127, 128, 255}},
		{name: "text", data: []byte("Direct Fetch acceptance\n")},
		{name: "gzip", data: compressed.Bytes(), contentEncoded: true, options: []fs.OpenOption{
			&fs.HTTPOption{Key: "Content-Encoding", Value: "gzip"},
		}},
	}
	prefix := "direct-fetch-" + random.String(16)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			name := path.Join(prefix, tc.name)
			testCtx := directFetchLiveStatsContext(ctx, name)
			registerDirectFetchLiveObjectCleanup(t, source, name)
			registerDirectFetchLiveObjectCleanup(t, dest, name)
			info := object.NewStaticObjectInfo(name, time.Unix(1700000000, 0), int64(len(tc.data)), true, nil, source).
				WithMetadata(fs.Metadata{"direct-fetch-case": tc.name})
			src, err := source.Put(testCtx, bytes.NewReader(tc.data), info, tc.options...)
			directFetchLiveRequireNoError(t, err, "source upload")
			out, err := operations.Copy(testCtx, dest, nil, name, src)
			directFetchLiveRequireNoError(t, err, "optimized copy")
			if out == nil {
				t.Fatal("optimized copy returned no destination object")
			}
			stats, err := accounting.Stats(testCtx).RemoteStats(false)
			directFetchLiveRequireNoError(t, err, "stats read")
			if tc.contentEncoded && !source.(*Fs).urlReturnsStoredBytes() {
				// The source may transcode content-encoded objects, so these
				// take the ordinary copy, whose handling of them is not
				// specific to Direct Fetch.
				assert.EqualValues(t, 0, stats["serverSideCopies"])
				assert.ErrorIs(t, checkDirectFetchSourceEncoding(testCtx, src), fs.ErrorCantCopy,
					"the source check, not a destination error, must decline")
				return
			}
			assert.EqualValues(t, 1, stats["serverSideCopies"])
			assert.Equal(t, info.Size(), out.Size())
			assert.WithinDuration(t, info.ModTime(testCtx), out.ModTime(testCtx), time.Second)
			dstObject, ok := out.(*Object)
			if !ok {
				t.Fatal("live destination object is not an S3 object")
			}
			assert.Equal(t, tc.name, dstObject.meta["direct-fetch-case"])
			got := directFetchLiveReadRawDestination(testCtx, t, dstObject)
			assert.True(t, bytes.Equal(tc.data, got), "stored destination bytes must match source bytes")
		})
	}
}

func runDirectFetchLiveErrorCases(ctx context.Context, t *testing.T, source, dest *Fs) {
	t.Run("expired source URL falls back", func(t *testing.T) {
		name := path.Join("direct-fetch-errors-"+random.String(16), "expired")
		registerDirectFetchLiveObjectCleanup(t, source, name)
		registerDirectFetchLiveObjectCleanup(t, dest, name)
		data := []byte("expired URL acceptance\n")
		info := object.NewStaticObjectInfo(name, time.Unix(1700000000, 0), int64(len(data)), true, nil, source)
		src, err := source.Put(ctx, bytes.NewReader(data), info)
		directFetchLiveRequireNoError(t, err, "expired fixture upload")
		sourceURL, err := source.Features().PublicLink(ctx, name, fs.Duration(time.Second), false)
		directFetchLiveRequireNoError(t, err, "one-second source link")
		expiresAt, ok := directFetchLiveURLExpiration(sourceURL)
		if !ok {
			t.Fatal("source returned a link without a usable nominal expiration")
		}
		deadlineCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		wait := time.Until(expiresAt.Add(time.Second))
		if wait > 0 {
			timer := time.NewTimer(wait)
			defer timer.Stop()
			select {
			case <-timer.C:
			case <-deadlineCtx.Done():
				t.Fatal("deadline elapsed while waiting for source URL expiration")
			}
		}
		got, fetchErr := dest.ServerSideFetchURL(deadlineCtx, name, sourceURL, src)
		assert.Nil(t, got)
		if fetchErr == nil {
			t.Fatal("expired source URL unexpectedly succeeded")
		}
		if !errors.Is(fetchErr, fs.ErrorCantCopy) {
			directFetchLiveLogErrorFixture(t, "expired URL", sourceURL, fetchErr)
			t.Error("expired source URL was not classified as an allowed fallback")
		}
		directFetchLiveAssertSourceResponseFixture(t, sourceURL, fetchErr)
		directFetchLiveLogErrorFixture(t, "expired URL", sourceURL, fetchErr)
	})

	t.Run("invalid destination credentials are final", func(t *testing.T) {
		name := path.Join("direct-fetch-errors-"+random.String(16), "invalid-destination-auth")
		registerDirectFetchLiveObjectCleanup(t, source, name)
		registerDirectFetchLiveObjectCleanup(t, dest, name)
		data := []byte("destination auth acceptance\n")
		info := object.NewStaticObjectInfo(name, time.Unix(1700000000, 0), int64(len(data)), true, nil, source)
		src, err := source.Put(ctx, bytes.NewReader(data), info)
		directFetchLiveRequireNoError(t, err, "auth fixture upload")
		sourceURL, err := source.Features().PublicLink(ctx, name, fs.Duration(time.Hour), false)
		directFetchLiveRequireNoError(t, err, "auth fixture source link")

		originalClient := dest.c
		options := originalClient.Options()
		options.Credentials = aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider("invalid-direct-fetch-key", "invalid-direct-fetch-secret", ""))
		dest.c = awss3.New(options)
		defer func() { dest.c = originalClient }()
		got, fetchErr := dest.ServerSideFetchURL(ctx, name, sourceURL, src)
		assert.Nil(t, got)
		if fetchErr == nil {
			t.Fatal("invalid destination credentials unexpectedly succeeded")
		}
		if errors.Is(fetchErr, fs.ErrorCantCopy) {
			directFetchLiveLogErrorFixture(t, "invalid destination credentials", sourceURL, fetchErr)
			t.Error("destination authentication error incorrectly authorized fallback")
		}
		directFetchLiveLogErrorFixture(t, "invalid destination credentials", sourceURL, fetchErr)
	})

	t.Run("multipart source rejection leaves no upload", func(t *testing.T) {
		name := path.Join("direct-fetch-errors-"+random.String(16), "multipart-rejection")
		registerDirectFetchLiveObjectCleanup(t, source, name)
		registerDirectFetchLiveObjectCleanup(t, dest, name)
		registerDirectFetchLiveMultipartCleanup(t, dest, name)
		data := []byte("multipart cleanup acceptance\n")
		info := object.NewStaticObjectInfo(name, time.Unix(1700000000, 0), int64(len(data)), true, nil, source)
		src, err := source.Put(ctx, bytes.NewReader(data), info)
		directFetchLiveRequireNoError(t, err, "multipart fixture upload")
		sourceURL, err := source.Features().PublicLink(ctx, name, fs.Duration(time.Hour), false)
		directFetchLiveRequireNoError(t, err, "multipart fixture source link")
		corruptURL, ok := directFetchLiveCorruptSignature(sourceURL)
		if !ok {
			t.Fatal("source link did not contain the expected signature field")
		}
		oldChunkSize, oldConcurrency := dest.opt.DirectFetchChunkSize, dest.opt.DirectFetchConcurrency
		dest.opt.DirectFetchChunkSize = fs.SizeSuffix(directFetchMaxSize)
		dest.opt.DirectFetchConcurrency = 1
		defer func() {
			dest.opt.DirectFetchChunkSize = oldChunkSize
			dest.opt.DirectFetchConcurrency = oldConcurrency
		}()
		tagger, ok := src.(fs.ETagger)
		if !ok || tagger.ETag(ctx) == "" {
			t.Fatal("source fixture has no ETag to pin the multipart fetch")
		}
		synthetic := fetchSourceInfo{
			StaticObjectInfo: object.NewStaticObjectInfo(name, src.ModTime(ctx), directFetchMaxSize+1, true, nil, source),
			etag:             tagger.ETag(ctx),
		}
		got, fetchErr := dest.ServerSideFetchURL(ctx, name, corruptURL, synthetic)
		assert.Nil(t, got)
		if fetchErr == nil {
			t.Fatal("corrupted source signature unexpectedly succeeded")
		}
		if !errors.Is(fetchErr, fs.ErrorCantCopy) {
			directFetchLiveLogErrorFixture(t, "multipart source rejection", corruptURL, fetchErr)
			t.Error("corrupted source signature was not classified as an allowed fallback")
		}
		directFetchLiveAssertSourceResponseFixture(t, corruptURL, fetchErr)
		directFetchLiveLogErrorFixture(t, "multipart source rejection", corruptURL, fetchErr)
		uploads, listErr := directFetchLiveMultipartUploads(ctx, dest, name)
		directFetchLiveRequireNoError(t, listErr, "multipart cleanup verification")
		if len(uploads) != 0 {
			t.Error("owned multipart upload remained after source rejection")
		}
	})
}

func runDirectFetchLiveOrdinaryCases(ctx context.Context, t *testing.T, source, dest fs.Fs, destName string) {
	for _, tc := range []struct {
		name string
	}{
		{name: "disabled destination feature"},
		{name: "ineligible source"},
		{name: "wire logging"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			name := path.Join("direct-fetch-ordinary-"+random.String(16), strings.ReplaceAll(tc.name, " ", "-"))
			testCtx := directFetchLiveStatsContext(ctx, name)
			destForCopy := dest
			if tc.name == "disabled destination feature" {
				disabledCtx, ci := fs.AddConfig(testCtx)
				ci.DisableFeatures = []string{"ServerSideFetchURL"}
				var err error
				destForCopy, err = fs.NewFs(disabledCtx, destName)
				if err != nil {
					t.Fatal("failed to open destination with Direct Fetch disabled")
				}
				if destForCopy.Features().ServerSideFetchURL != nil {
					t.Fatal("destination Direct Fetch feature was not disabled")
				}
				testCtx = disabledCtx
			}
			registerDirectFetchLiveObjectCleanup(t, source, name)
			registerDirectFetchLiveObjectCleanup(t, destForCopy, name)
			data := []byte("ordinary copy acceptance\n")
			info := object.NewStaticObjectInfo(name, time.Unix(1700000000, 0), int64(len(data)), true, nil, source)
			src, err := source.Put(testCtx, bytes.NewReader(data), info)
			directFetchLiveRequireNoError(t, err, "ordinary fixture upload")
			var copySource fs.Object
			copySource = src
			if tc.name == "ineligible source" {
				view := &directFetchLiveFsView{Fs: source, features: *source.Features()}
				view.features.PublicLinkIsDirect = false
				copySource = &directFetchLiveObjectView{Object: src, info: view}
			}
			if tc.name == "wire logging" {
				loggingCtx, ci := fs.AddConfig(testCtx)
				ci.Dump = fs.DumpHeaders
				testCtx = loggingCtx
			}
			var out fs.Object
			var copyErr error
			logs := captureDirectFetchCopyLogs(func() {
				out, copyErr = operations.Copy(testCtx, destForCopy, nil, name, copySource)
			})
			directFetchLiveRequireNoError(t, copyErr, "ordinary copy")
			if out == nil {
				t.Fatal("ordinary copy returned no destination object")
			}
			stats, err := accounting.Stats(testCtx).RemoteStats(false)
			directFetchLiveRequireNoError(t, err, "ordinary stats read")
			assert.EqualValues(t, 0, stats["serverSideCopies"])
			dstObject, ok := out.(*Object)
			if !ok {
				t.Fatal("ordinary destination object is not an S3 object")
			}
			got := directFetchLiveReadRawDestination(testCtx, t, dstObject)
			assert.Equal(t, data, got)
			lowerLogs := strings.ToLower(logs)
			if strings.Contains(lowerLogs, "x-amz-signature") || strings.Contains(lowerLogs, strings.ToLower(directFetchSourceHeader)) {
				t.Fatal("ordinary-copy logs exposed a signed source URL")
			}
		})
	}
}

type directFetchLiveFsView struct {
	fs.Fs
	features fs.Features
}

func (f *directFetchLiveFsView) Features() *fs.Features {
	return &f.features
}

type directFetchLiveObjectView struct {
	fs.Object
	info fs.Info
}

func (o *directFetchLiveObjectView) Fs() fs.Info {
	return o.info
}

func runDirectFetchLiveLargeCase(ctx context.Context, t *testing.T, source fs.Fs, dest *Fs) {
	t.Run("pre-existing large object", func(t *testing.T) {
		if os.Getenv("RCLONE_DIRECT_FETCH_LARGE") != "1" {
			t.Skip("set RCLONE_DIRECT_FETCH_LARGE=1 after approving billable large-object acceptance")
		}
		largeObject := os.Getenv("RCLONE_DIRECT_FETCH_LARGE_OBJECT")
		if largeObject == "" {
			t.Skip("set RCLONE_DIRECT_FETCH_LARGE_OBJECT to an approved pre-existing source object")
		}
		src, err := source.NewObject(ctx, largeObject)
		if err != nil {
			t.Fatal("failed to open the approved pre-existing large source object")
		}
		if src.Size() <= directFetchMaxSize {
			t.Fatal("approved large source object does not exceed the Direct Fetch single-request limit")
		}
		name := path.Join("direct-fetch-large-"+random.String(16), "copy")
		registerDirectFetchLiveObjectCleanup(t, dest, name)
		testCtx := directFetchLiveStatsContext(ctx, name)
		out, err := operations.Copy(testCtx, dest, nil, name, src)
		directFetchLiveRequireNoError(t, err, "large optimized copy")
		if out == nil {
			t.Fatal("large optimized copy returned no destination object")
		}
		stats, err := accounting.Stats(testCtx).RemoteStats(false)
		directFetchLiveRequireNoError(t, err, "large stats read")
		assert.EqualValues(t, 1, stats["serverSideCopies"])
		dstObject, ok := out.(*Object)
		if !ok {
			t.Fatal("large destination object is not an S3 object")
		}
		sourceHash := directFetchLiveObjectSHA256(testCtx, t, src)
		destinationHash := directFetchLiveRawDestinationSHA256(testCtx, t, dstObject)
		assert.Equal(t, sourceHash, destinationHash)
	})
}

func directFetchLiveStatsContext(ctx context.Context, suffix string) context.Context {
	group := "direct-fetch-live-" + random.String(12) + "-" + path.Base(suffix)
	ctx = accounting.WithStatsGroup(ctx, group)
	accounting.NewStatsGroup(ctx, group)
	return ctx
}

func registerDirectFetchLiveObjectCleanup(t *testing.T, remote fs.Fs, name string) {
	t.Helper()
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		existing, err := remote.NewObject(cleanupCtx, name)
		if errors.Is(err, fs.ErrorObjectNotFound) {
			return
		}
		if err != nil {
			t.Error("failed to inspect an owned live-test object during cleanup")
			return
		}
		if err := existing.Remove(cleanupCtx); err != nil {
			t.Error("failed to remove an owned live-test object")
		}
	})
}

func directFetchLiveRequireNoError(t *testing.T, err error, action string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s failed", action)
	}
}

func directFetchLiveReadRawDestination(ctx context.Context, t *testing.T, dstObject *Object) []byte {
	t.Helper()
	bucket, key := dstObject.split()
	raw, err := dstObject.fs.c.GetObject(ctx, &awss3.GetObjectInput{
		Bucket: &bucket, Key: &key, VersionId: dstObject.versionID,
	})
	directFetchLiveRequireNoError(t, err, "raw destination read")
	got, readErr := io.ReadAll(raw.Body)
	closeErr := raw.Body.Close()
	directFetchLiveRequireNoError(t, readErr, "raw destination body read")
	directFetchLiveRequireNoError(t, closeErr, "raw destination body close")
	return got
}

func directFetchLiveURLExpiration(sourceURL string) (time.Time, bool) {
	parsed, err := url.Parse(sourceURL)
	if err != nil {
		return time.Time{}, false
	}
	signedAt, err := time.Parse("20060102T150405Z", parsed.Query().Get("X-Amz-Date"))
	if err != nil {
		return time.Time{}, false
	}
	seconds, err := strconv.ParseInt(parsed.Query().Get("X-Amz-Expires"), 10, 64)
	if err != nil || seconds < 1 {
		return time.Time{}, false
	}
	return signedAt.Add(time.Duration(seconds) * time.Second), true
}

func directFetchLiveCorruptSignature(sourceURL string) (string, bool) {
	parsed, err := url.Parse(sourceURL)
	if err != nil {
		return "", false
	}
	query := parsed.Query()
	if !query.Has("X-Amz-Signature") {
		return "", false
	}
	query.Set("X-Amz-Signature", strings.Repeat("0", 64))
	parsed.RawQuery = query.Encode()
	return parsed.String(), true
}

func directFetchLiveAssertSourceResponseFixture(t *testing.T, sourceURL string, err error) {
	t.Helper()
	var directErr *directFetchError
	if !errors.As(err, &directErr) {
		t.Error("source rejection did not retain a typed Direct Fetch response")
		return
	}
	if directErr.status != http.StatusBadRequest || directErr.code != "DirectFetchSourceStatus" ||
		!regexp.MustCompile(`^DirectFetchSourceStatus [1-5][0-9]{2}$`).MatchString(directErr.detail) {
		detail := safeDirectFetchText(directErr.detail, sourceURL, true)
		t.Errorf("unexpected sanitized source rejection fixture: HTTP %d code %q detail %q", directErr.status, directErr.code, detail)
	}
}

func directFetchLiveLogErrorFixture(t *testing.T, label, sourceURL string, err error) {
	t.Helper()
	var directErr *directFetchError
	if !errors.As(err, &directErr) {
		t.Logf("%s fixture had no typed Direct Fetch response", label)
		return
	}
	code := safeDirectFetchText(directErr.code, sourceURL, true)
	detail := safeDirectFetchText(directErr.detail, sourceURL, true)
	requestID := safeDirectFetchText(directErr.requestID, sourceURL, true)
	t.Logf("%s fixture: HTTP %d code %q detail %q request %q", label, directErr.status, code, detail, requestID)
}

func directFetchLiveMultipartUploads(ctx context.Context, dest *Fs, remote string) ([]types.MultipartUpload, error) {
	bucket, key := dest.split(remote)
	var uploads []types.MultipartUpload
	var keyMarker, uploadIDMarker *string
	for {
		out, err := dest.c.ListMultipartUploads(ctx, &awss3.ListMultipartUploadsInput{
			Bucket: &bucket, Prefix: &key, KeyMarker: keyMarker, UploadIdMarker: uploadIDMarker,
		})
		if err != nil {
			return nil, err
		}
		for _, upload := range out.Uploads {
			if upload.Key != nil && *upload.Key == key {
				uploads = append(uploads, upload)
			}
		}
		if out.IsTruncated == nil || !*out.IsTruncated {
			return uploads, nil
		}
		keyMarker, uploadIDMarker = out.NextKeyMarker, out.NextUploadIdMarker
	}
}

func registerDirectFetchLiveMultipartCleanup(t *testing.T, dest *Fs, remote string) {
	t.Helper()
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		uploads, err := directFetchLiveMultipartUploads(cleanupCtx, dest, remote)
		if err != nil {
			t.Error("failed to inspect owned live-test multipart uploads during cleanup")
			return
		}
		bucket, key := dest.split(remote)
		for _, upload := range uploads {
			if upload.UploadId == nil {
				continue
			}
			_, err := dest.c.AbortMultipartUpload(cleanupCtx, &awss3.AbortMultipartUploadInput{
				Bucket: &bucket, Key: &key, UploadId: upload.UploadId,
			})
			if err != nil {
				t.Error("failed to abort an owned live-test multipart upload")
			}
		}
	})
}

func directFetchLiveObjectSHA256(ctx context.Context, t *testing.T, src fs.Object) [sha256.Size]byte {
	t.Helper()
	in, err := src.Open(ctx)
	directFetchLiveRequireNoError(t, err, "large source verification open")
	h := sha256.New()
	_, copyErr := io.Copy(h, in)
	closeErr := in.Close()
	directFetchLiveRequireNoError(t, copyErr, "large source verification read")
	directFetchLiveRequireNoError(t, closeErr, "large source verification close")
	var sum [sha256.Size]byte
	copy(sum[:], h.Sum(nil))
	return sum
}

func directFetchLiveRawDestinationSHA256(ctx context.Context, t *testing.T, dstObject *Object) [sha256.Size]byte {
	t.Helper()
	bucket, key := dstObject.split()
	raw, err := dstObject.fs.c.GetObject(ctx, &awss3.GetObjectInput{
		Bucket: &bucket, Key: &key, VersionId: dstObject.versionID,
	})
	directFetchLiveRequireNoError(t, err, "large raw destination verification open")
	h := sha256.New()
	_, copyErr := io.Copy(h, raw.Body)
	closeErr := raw.Body.Close()
	directFetchLiveRequireNoError(t, copyErr, "large raw destination verification read")
	directFetchLiveRequireNoError(t, closeErr, "large raw destination verification close")
	var sum [sha256.Size]byte
	copy(sum[:], h.Sum(nil))
	return sum
}
