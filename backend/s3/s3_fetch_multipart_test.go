package s3

import (
	"bufio"
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDirectFetchRanges(t *testing.T) {
	for _, size := range []int64{directFetchMaxSize + 1, 5 * 1024 * 1024 * 1024, 50_000_000_001} {
		ranges, err := directFetchRanges(size, minChunkSize, 10000)
		require.NoError(t, err)
		require.NotEmpty(t, ranges)
		assert.LessOrEqual(t, len(ranges), 10000)
		next := int64(0)
		for i, r := range ranges {
			assert.Equal(t, next, r.start)
			n := r.end - r.start + 1
			assert.Positive(t, n)
			assert.LessOrEqual(t, n, directFetchMaxSize)
			if i != len(ranges)-1 {
				assert.GreaterOrEqual(t, n, int64(minChunkSize))
			}
			next = r.end + 1
		}
		assert.Equal(t, size, next)
	}

	_, err := directFetchRanges(math.MaxInt64, minChunkSize, 10000)
	assert.ErrorIs(t, err, fs.ErrorCantCopy)
	_, err = directFetchRanges(directFetchMaxSize+1, minChunkSize, 1)
	assert.ErrorIs(t, err, fs.ErrorCantCopy)
}

func TestDirectFetchRangesBoundsAndConfiguration(t *testing.T) {
	tests := []struct {
		name      string
		size      int64
		chunkSize fs.SizeSuffix
		maxParts  int
		want      []directFetchRange
	}{
		{
			name:      "chunk above cap is capped",
			size:      directFetchMaxSize + 1,
			chunkSize: fs.SizeSuffix(directFetchMaxSize + 123),
			maxParts:  10,
			want: []directFetchRange{
				{start: 0, end: directFetchMaxSize - 1},
				{start: directFetchMaxSize, end: directFetchMaxSize},
			},
		},
		{
			name:      "chunk below minimum is raised",
			size:      int64(minChunkSize) + 1,
			chunkSize: 1,
			maxParts:  10,
			want: []directFetchRange{
				{start: 0, end: int64(minChunkSize) - 1},
				{start: int64(minChunkSize), end: int64(minChunkSize)},
			},
		},
		{
			name:      "exact divisibility",
			size:      2 * int64(minChunkSize),
			chunkSize: minChunkSize,
			maxParts:  10,
			want: []directFetchRange{
				{start: 0, end: int64(minChunkSize) - 1},
				{start: int64(minChunkSize), end: 2*int64(minChunkSize) - 1},
			},
		},
		{
			name:      "one byte final part",
			size:      2*int64(minChunkSize) + 1,
			chunkSize: minChunkSize,
			maxParts:  10,
			want: []directFetchRange{
				{start: 0, end: int64(minChunkSize) - 1},
				{start: int64(minChunkSize), end: 2*int64(minChunkSize) - 1},
				{start: 2 * int64(minChunkSize), end: 2 * int64(minChunkSize)},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := directFetchRanges(tt.size, tt.chunkSize, tt.maxParts)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}

	t.Run("max parts is clamped", func(t *testing.T) {
		ranges, err := directFetchRanges(directFetchMaxSize*int64(maxUploadParts), minChunkSize, maxUploadParts+1)
		require.NoError(t, err)
		require.Len(t, ranges, maxUploadParts)
		assert.Equal(t, directFetchRange{start: 0, end: directFetchMaxSize - 1}, ranges[0])
		assert.Equal(t, directFetchRange{
			start: directFetchMaxSize * int64(maxUploadParts-1),
			end:   directFetchMaxSize*int64(maxUploadParts) - 1,
		}, ranges[len(ranges)-1])
	})

	for _, size := range []int64{-1, 0} {
		_, err := directFetchRanges(size, minChunkSize, maxUploadParts)
		assert.ErrorIs(t, err, fs.ErrorCantCopy)
	}

	for _, maxParts := range []int{0, -1} {
		ranges, err := directFetchRanges(directFetchMaxSize, minChunkSize, maxParts)
		require.NoError(t, err)
		assert.Equal(t, []directFetchRange{{start: 0, end: directFetchMaxSize - 1}}, ranges)
	}
}

type directFetchMultipartPart struct {
	PartNumber int    `xml:"PartNumber"`
	ETag       string `xml:"ETag"`
}

type directFetchMultipartFixture struct {
	mu sync.Mutex

	createMode   string
	partModes    map[int]string
	completeMode string
	abortMode    string
	headMode     string
	headSize     int64

	partHook func(http.ResponseWriter, *http.Request, int)

	events       []string
	captures     []directFetchWireCapture
	completePart []directFetchMultipartPart
	activeParts  int
	maxActive    int
}

func (f *directFetchMultipartFixture) record(event string, r *http.Request, body []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, event)
	f.captures = append(f.captures, directFetchWireCapture{request: r.Clone(r.Context()), body: body})
}

func (f *directFetchMultipartFixture) partStarted(part int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, fmt.Sprintf("part-%d-start", part))
	f.activeParts++
	f.maxActive = max(f.maxActive, f.activeParts)
}

func (f *directFetchMultipartFixture) partFinished(part int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.activeParts--
	f.events = append(f.events, fmt.Sprintf("part-%d-finish", part))
}

func (f *directFetchMultipartFixture) snapshot() (events []string, captures []directFetchWireCapture, parts []directFetchMultipartPart, maxActive int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.events...), append([]directFetchWireCapture(nil), f.captures...), append([]directFetchMultipartPart(nil), f.completePart...), f.maxActive
}

func writeDirectFetchMultipartError(w http.ResponseWriter, status int, code, message, detail string) {
	w.Header().Set("Content-Type", "application/xml")
	if detail != "" {
		w.Header().Set(directFetchErrorHeader, detail)
	}
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(w, "<Error><Code>%s</Code><Message>%s</Message></Error>", code, message)
}

func (f *directFetchMultipartFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	query := r.URL.Query()
	switch {
	case r.Method == http.MethodPost && query.Has("uploads"):
		f.record("create", r, body)
		switch f.createMode {
		case "error":
			writeDirectFetchMultipartError(w, http.StatusForbidden, "AccessDenied", "create denied", "")
		case "empty":
			w.Header().Set("Content-Type", "application/xml")
			_, _ = io.WriteString(w, `<CreateMultipartUploadResult><Bucket>bucket</Bucket><Key>target</Key></CreateMultipartUploadResult>`)
		default:
			w.Header().Set("Content-Type", "application/xml")
			_, _ = io.WriteString(w, `<CreateMultipartUploadResult><Bucket>bucket</Bucket><Key>target</Key><UploadId>upload-1</UploadId></CreateMultipartUploadResult>`)
		}
	case r.Method == http.MethodPut && query.Get("uploadId") != "":
		part, parseErr := strconv.Atoi(query.Get("partNumber"))
		if parseErr != nil {
			http.Error(w, parseErr.Error(), http.StatusBadRequest)
			return
		}
		f.record(fmt.Sprintf("part-%d", part), r, body)
		f.partStarted(part)
		defer f.partFinished(part)
		if f.partHook != nil {
			f.partHook(w, r, part)
			return
		}
		switch f.partModes[part] {
		case "source":
			writeDirectFetchMultipartError(w, http.StatusBadRequest, "DirectFetchSourceStatus", "503", "DirectFetchSourceStatus 503")
		case "changed":
			writeDirectFetchMultipartError(w, http.StatusBadRequest, "DirectFetchSourceStatus", "412", "DirectFetchSourceStatus 412")
		case "auth":
			writeDirectFetchMultipartError(w, http.StatusForbidden, "AccessDenied", "part denied", "")
		case "empty-etag":
			w.WriteHeader(http.StatusOK)
		default:
			w.Header().Set("ETag", fmt.Sprintf(`"part-%d"`, part))
		}
	case r.Method == http.MethodPost && query.Get("uploadId") != "":
		f.record("complete", r, body)
		var completion struct {
			Parts []directFetchMultipartPart `xml:"Part"`
		}
		if err := xml.Unmarshal(body, &completion); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		f.mu.Lock()
		f.completePart = append([]directFetchMultipartPart(nil), completion.Parts...)
		f.mu.Unlock()
		switch f.completeMode {
		case "error":
			writeDirectFetchMultipartError(w, http.StatusBadRequest, "InvalidRequest", "completion rejected", "")
		case "embedded-error":
			w.Header().Set("Content-Type", "application/xml")
			_, _ = io.WriteString(w, `<Error><Code>InvalidRequest</Code><Message>embedded completion error</Message></Error>`)
		case "empty":
			w.Header().Set("Content-Type", "application/xml")
			_, _ = io.WriteString(w, `<CompleteMultipartUploadResult><Bucket>bucket</Bucket><Key>target</Key></CompleteMultipartUploadResult>`)
		default:
			w.Header().Set("Content-Type", "application/xml")
			w.Header().Set("x-amz-version-id", "version-multipart")
			_, _ = io.WriteString(w, `<CompleteMultipartUploadResult><Bucket>bucket</Bucket><Key>target</Key><ETag>&quot;complete&quot;</ETag></CompleteMultipartUploadResult>`)
		}
	case r.Method == http.MethodDelete && query.Get("uploadId") != "":
		f.record("abort", r, body)
		switch f.abortMode {
		case "error":
			writeDirectFetchMultipartError(w, http.StatusInternalServerError, "InternalError", "abort failed", "")
		case "no-such-upload":
			writeDirectFetchMultipartError(w, http.StatusNotFound, "NoSuchUpload", "already absent", "")
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	case r.Method == http.MethodHead:
		f.record("head", r, body)
		if f.headMode == "error" {
			writeDirectFetchMultipartError(w, http.StatusForbidden, "AccessDenied", "HEAD denied", "")
			return
		}
		w.Header().Set("Content-Length", strconv.FormatInt(f.headSize, 10))
		w.Header().Set("Last-Modified", "Wed, 01 Jan 2025 00:00:00 GMT")
		w.Header().Set("ETag", `"head"`)
		w.Header().Set("Content-Type", "application/head")
		w.Header().Set("X-Amz-Meta-Result", "multipart-head")
	default:
		http.Error(w, "unexpected request", http.StatusBadRequest)
	}
}

func directFetchMultipartEvents(events []string, prefix string) []string {
	var filtered []string
	for _, event := range events {
		if strings.HasPrefix(event, prefix) {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

func directFetchMultipartCaptures(captures []directFetchWireCapture, method string) []directFetchWireCapture {
	var filtered []directFetchWireCapture
	for _, capture := range captures {
		if capture.request.Method == method {
			filtered = append(filtered, capture)
		}
	}
	return filtered
}

func TestDirectFetchMultipartUsesDirectFetchChunkSize(t *testing.T) {
	ctx, ci := fs.AddConfig(context.Background())
	ci.LowLevelRetries = 1
	size := 2*int64(minChunkSize) + 1
	fixture := &directFetchMultipartFixture{headSize: size}
	f := newDirectFetchTestFs(ctx, t, "Fastly", fixture)
	f.opt.UploadCutoff = maxUploadCutoff
	f.opt.ChunkSize = fs.SizeSuffix(directFetchMaxSize)
	f.opt.DirectFetchChunkSize = minChunkSize
	f.opt.DirectFetchConcurrency = 1

	got, err := f.ServerSideFetchURL(ctx, "target", "https://source.example/object", directFetchSource(size, nil))
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, size, got.Size())

	_, captures, completedParts, _ := fixture.snapshot()
	puts := directFetchMultipartCaptures(captures, http.MethodPut)
	require.Len(t, puts, 3)
	assertDirectFetchWireRequest(t, puts[0], "https://source.example/object", fmt.Sprintf("bytes=0-%d", int64(minChunkSize)-1))
	assertDirectFetchWireRequest(t, puts[1], "https://source.example/object", fmt.Sprintf("bytes=%d-%d", minChunkSize, 2*int64(minChunkSize)-1))
	assertDirectFetchWireRequest(t, puts[2], "https://source.example/object", fmt.Sprintf("bytes=%d-%d", 2*int64(minChunkSize), size-1))
	assert.Len(t, completedParts, 3)
}

func TestDirectFetchSinglePartUpToDirectFetchChunkSize(t *testing.T) {
	ctx, ci := fs.AddConfig(context.Background())
	ci.LowLevelRetries = 1
	size := int64(minChunkSize)
	captures := make(chan directFetchWireCapture, 4)
	f := newDirectFetchTestFs(ctx, t, "Fastly", directFetchSuccessHandler(captures, &size))
	f.opt.UploadCutoff = 0
	f.opt.DirectFetchChunkSize = minChunkSize

	got, err := f.ServerSideFetchURL(ctx, "target", "https://source.example/object", directFetchSource(size, nil))
	require.NoError(t, err)
	require.NotNil(t, got)

	put := <-captures
	assert.Equal(t, http.MethodPut, put.request.Method)
	assert.False(t, put.request.URL.Query().Has("uploadId"))
	assert.Equal(t, http.MethodHead, (<-captures).request.Method)
	assert.Empty(t, captures)
}

func TestDirectFetchMultipartUsesDirectFetchConcurrency(t *testing.T) {
	ctx, ci := fs.AddConfig(context.Background())
	ci.LowLevelRetries = 1
	const concurrency = 3
	size := 4 * int64(minChunkSize)
	fixture := &directFetchMultipartFixture{headSize: size}
	var mu sync.Mutex
	started := 0
	allStarted := make(chan struct{})
	fixture.partHook = func(w http.ResponseWriter, _ *http.Request, part int) {
		mu.Lock()
		started++
		if started == concurrency {
			close(allStarted)
		}
		mu.Unlock()
		select {
		case <-allStarted:
		case <-time.After(5 * time.Second):
		}
		w.Header().Set("ETag", fmt.Sprintf(`"part-%d"`, part))
	}
	f := newDirectFetchTestFs(ctx, t, "Fastly", fixture)
	f.opt.UploadConcurrency = 1
	f.opt.DirectFetchChunkSize = minChunkSize
	f.opt.DirectFetchConcurrency = concurrency

	got, err := f.ServerSideFetchURL(ctx, "target", "https://source.example/object", directFetchSource(size, nil))
	require.NoError(t, err)
	require.NotNil(t, got)

	_, _, completedParts, maxActive := fixture.snapshot()
	assert.Len(t, completedParts, 4)
	assert.Equal(t, concurrency, maxActive)
}

func TestDirectFetchOptions(t *testing.T) {
	ctx := context.Background()
	f := newDirectFetchTestFs(ctx, t, "Fastly", http.NotFoundHandler())
	assert.Equal(t, 64*fs.Mebi, f.opt.DirectFetchChunkSize)
	assert.Equal(t, 16, f.opt.DirectFetchConcurrency)

	reg, err := fs.Find("s3")
	require.NoError(t, err)
	m := fs.ConfigMap("s3", reg.Options, "direct-fetch-test", configmap.Simple{
		"provider":                "Fastly",
		"env_auth":                "false",
		"direct_fetch_chunk_size": "1Mi",
	})
	_, err = NewFs(ctx, "direct-fetch-test", "bucket", m)
	assert.ErrorContains(t, err, "direct fetch chunk size")
}

func TestDirectFetchMultipartLogsProgressWithoutSecrets(t *testing.T) {
	var logs bytes.Buffer
	ci := fs.GetConfig(context.Background())
	oldLogLevel := ci.LogLevel
	fs.SetLogger(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	t.Cleanup(func() {
		ci.LogLevel = oldLogLevel
		fs.SetLogger(slog.Default().Handler())
	})
	ci.LogLevel = fs.LogLevelDebug

	ctx, config := fs.AddConfig(context.Background())
	config.LowLevelRetries = 1
	size := 2*int64(minChunkSize) + 1
	fixture := &directFetchMultipartFixture{headSize: size}
	f := newDirectFetchTestFs(ctx, t, "Fastly", fixture)
	f.opt.DirectFetchChunkSize = minChunkSize
	f.opt.MaxUploadParts = 2
	f.opt.DirectFetchConcurrency = 1
	sourceURL := "https://source.example/object?X-Amz-Signature=do-not-log"

	got, err := f.ServerSideFetchURL(ctx, "target", sourceURL, directFetchSource(size, nil))
	require.NoError(t, err)
	require.NotNil(t, got)

	output := logs.String()
	assert.Contains(t, output, "starting multipart upload with 2 parts of size 5242881 bytes")
	assert.Contains(t, output, "part 1/2 (bytes=0-5242880) starting")
	assert.Contains(t, output, "wrote part 1/2 (bytes=0-5242880) with 5242881 bytes")
	assert.Contains(t, output, "multipart upload finished")
	assert.NotContains(t, output, sourceURL)
	assert.NotContains(t, output, "do-not-log")
	assert.NotContains(t, output, "upload-1")
}

func TestDirectFetchMultipartSuccess(t *testing.T) {
	ctx, ci := fs.AddConfig(context.Background())
	ci.LowLevelRetries = 1
	ci.Metadata = true
	t.Setenv("RCLONE_FETCH_MAPPER_HELPER", "1")
	exe, err := os.Executable()
	require.NoError(t, err)
	ci.MetadataMapper = fs.SpaceSepList{exe, "-test.run=^TestDirectFetchMapperProcess$"}

	fixture := &directFetchMultipartFixture{headSize: directFetchMaxSize + 11}
	part1Started := make(chan struct{})
	part2Done := make(chan struct{})
	fixture.partHook = func(w http.ResponseWriter, _ *http.Request, part int) {
		if part == 1 {
			close(part1Started)
			<-part2Done
		} else {
			<-part1Started
		}
		w.Header().Set("ETag", fmt.Sprintf(`"part-%d"`, part))
		if part == 2 {
			close(part2Done)
		}
	}
	f := newDirectFetchTestFsWithConfig(ctx, t, "Fastly", fixture, configmap.Simple{
		"server_side_encryption":   "AES256",
		"storage_class":            "STANDARD_IA",
		"requester_pays":           "true",
		"sse_customer_algorithm":   "AES256",
		"direct_fetch_chunk_size":  strconv.FormatInt(directFetchMaxSize, 10),
		"direct_fetch_concurrency": "2",
	})
	f.opt.Versions = true
	require.True(t, f.opt.UseMultipartUploads.Value)
	sourceURL := "https://source.example/large?X-Amz-Signature=secret"

	got, err := f.ServerSideFetchURL(ctx, "target", sourceURL,
		directFetchSource(directFetchMaxSize+1, fs.Metadata{"owner": "source", "source-only": "kept"}),
		fs.MetadataOption(fs.Metadata{"owner": "option", "option-only": "kept"}),
		&fs.HTTPOption{Key: "X-Amz-Meta-Owner", Value: "header"},
		&fs.HTTPOption{Key: "Cache-Control", Value: "private"},
		&fs.HTTPOption{Key: "If-Match", Value: `"old"`},
		&fs.HTTPOption{Key: "If-None-Match", Value: `"other"`})
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, directFetchMaxSize+11, got.Size())
	dst := got.(*Object)
	assert.Equal(t, "application/head", dst.mimeType)
	assert.Equal(t, "multipart-head", dst.meta["result"])
	assert.Equal(t, "version-multipart", deref(dst.versionID))
	assert.True(t, f.opt.UseMultipartUploads.Value)

	events, captures, completedParts, maxActive := fixture.snapshot()
	assert.Equal(t, "create", events[0])
	assert.Less(t, directFetchEventIndex(events, "part-2-finish"), directFetchEventIndex(events, "part-1-finish"))
	assert.Less(t, directFetchEventIndex(events, "part-1-finish"), directFetchEventIndex(events, "complete"))
	assert.Less(t, directFetchEventIndex(events, "complete"), directFetchEventIndex(events, "head"))
	assert.Equal(t, 2, maxActive)
	assert.Equal(t, []directFetchMultipartPart{{PartNumber: 1, ETag: `"part-1"`}, {PartNumber: 2, ETag: `"part-2"`}}, completedParts)
	assert.Empty(t, directFetchMultipartEvents(events, "abort"))

	posts := directFetchMultipartCaptures(captures, http.MethodPost)
	require.Len(t, posts, 2)
	create := posts[0]
	assert.True(t, create.request.URL.Query().Has("uploads"))
	assert.Empty(t, create.request.Header.Get(directFetchSourceHeader))
	assert.Equal(t, "kept", create.request.Header.Get("X-Amz-Meta-Source-Only"))
	assert.Equal(t, "kept", create.request.Header.Get("X-Amz-Meta-Option-Only"))
	assert.Equal(t, "header", create.request.Header.Get("X-Amz-Meta-Owner"))
	assert.Equal(t, "yes", create.request.Header.Get("X-Amz-Meta-Mapped"))
	assert.Equal(t, "private", create.request.Header.Get("Cache-Control"))
	assert.Equal(t, "text/source", create.request.Header.Get("Content-Type"))
	assert.Equal(t, "AES256", create.request.Header.Get("X-Amz-Server-Side-Encryption"))
	assert.Equal(t, "STANDARD_IA", create.request.Header.Get("X-Amz-Storage-Class"))
	assert.Equal(t, "requester", create.request.Header.Get("X-Amz-Request-Payer"))
	assert.Equal(t, "AES256", create.request.Header.Get("X-Amz-Server-Side-Encryption-Customer-Algorithm"))

	puts := directFetchMultipartCaptures(captures, http.MethodPut)
	require.Len(t, puts, 2)
	byPart := map[string]directFetchWireCapture{}
	for _, put := range puts {
		byPart[put.request.URL.Query().Get("partNumber")] = put
	}
	assertDirectFetchWireRequest(t, byPart["1"], sourceURL, "bytes=0-5368709119")
	assertDirectFetchWireRequest(t, byPart["2"], sourceURL, "bytes=5368709120-5368709120")
	for _, put := range puts {
		assertDirectFetchPin(t, put, directFetchTestSourceETag)
	}
	assertDirectFetchPin(t, create, "")
	for _, put := range puts {
		assert.Equal(t, "requester", put.request.Header.Get("X-Amz-Request-Payer"))
		assert.Equal(t, "AES256", put.request.Header.Get("X-Amz-Server-Side-Encryption-Customer-Algorithm"))
	}
	complete := posts[1]
	assert.Equal(t, `"old"`, complete.request.Header.Get("If-Match"))
	assert.Equal(t, `"other"`, complete.request.Header.Get("If-None-Match"))
	assert.Equal(t, "requester", complete.request.Header.Get("X-Amz-Request-Payer"))
	assert.Equal(t, "AES256", complete.request.Header.Get("X-Amz-Server-Side-Encryption-Customer-Algorithm"))
	assert.Empty(t, complete.request.Header.Get(directFetchSourceHeader))
	assertDirectFetchPin(t, complete, "")
	heads := directFetchMultipartCaptures(captures, http.MethodHead)
	require.Len(t, heads, 1)
	assert.Equal(t, "version-multipart", heads[0].request.URL.Query().Get("versionId"))
}

func TestDirectFetchMultipartFailureCleanup(t *testing.T) {
	tests := []struct {
		name             string
		configure        func(*directFetchMultipartFixture, *Fs)
		wantPart         bool
		wantComplete     bool
		wantAbort        bool
		wantHead         bool
		wantCantCopy     bool
		wantOperations   []string
		wantErrorContent []string
	}{
		{
			name: "creation error",
			configure: func(fixture *directFetchMultipartFixture, _ *Fs) {
				fixture.createMode = "error"
			},
			wantOperations: []string{"CreateMultipartUpload"},
		},
		{
			name: "empty upload ID",
			configure: func(fixture *directFetchMultipartFixture, _ *Fs) {
				fixture.createMode = "empty"
			},
			wantErrorContent: []string{"no upload ID"},
		},
		{
			name: "source status part failure",
			configure: func(fixture *directFetchMultipartFixture, _ *Fs) {
				fixture.partModes = map[int]string{1: "source"}
			},
			wantPart: true, wantAbort: true, wantCantCopy: true,
			wantOperations: []string{"UploadPart"},
		},
		{
			name: "source changed part failure",
			configure: func(fixture *directFetchMultipartFixture, _ *Fs) {
				fixture.partModes = map[int]string{1: "changed"}
			},
			wantPart: true, wantAbort: true, wantCantCopy: true,
			wantOperations: []string{"UploadPart"},
		},
		{
			name: "source status still aborts when leave parts is configured",
			configure: func(fixture *directFetchMultipartFixture, f *Fs) {
				fixture.partModes = map[int]string{1: "source"}
				f.opt.LeavePartsOnError = true
			},
			wantPart: true, wantAbort: true, wantCantCopy: true,
		},
		{
			name: "destination auth part failure",
			configure: func(fixture *directFetchMultipartFixture, _ *Fs) {
				fixture.partModes = map[int]string{1: "auth"}
			},
			wantPart: true, wantAbort: true,
			wantOperations: []string{"UploadPart"},
		},
		{
			name: "missing part ETag",
			configure: func(fixture *directFetchMultipartFixture, _ *Fs) {
				fixture.partModes = map[int]string{1: "empty-etag"}
			},
			wantPart: true, wantAbort: true,
			wantErrorContent: []string{"part 1", "no ETag"},
		},
		{
			name: "completion error",
			configure: func(fixture *directFetchMultipartFixture, _ *Fs) {
				fixture.completeMode = "error"
			},
			wantPart: true, wantComplete: true, wantAbort: true,
			wantOperations: []string{"CompleteMultipartUpload"},
		},
		{
			name: "HTTP 200 completion XML error",
			configure: func(fixture *directFetchMultipartFixture, _ *Fs) {
				fixture.completeMode = "embedded-error"
			},
			wantPart: true, wantComplete: true, wantAbort: true,
			wantOperations: []string{"CompleteMultipartUpload"},
		},
		{
			name: "empty successful completion",
			configure: func(fixture *directFetchMultipartFixture, _ *Fs) {
				fixture.completeMode = "empty"
			},
			wantPart: true, wantComplete: true, wantAbort: true,
			wantErrorContent: []string{"completion", "no ETag"},
		},
		{
			name: "abort failure preserves both errors without fallback",
			configure: func(fixture *directFetchMultipartFixture, _ *Fs) {
				fixture.partModes = map[int]string{1: "source"}
				fixture.abortMode = "error"
			},
			wantPart: true, wantAbort: true,
			wantErrorContent: []string{"UploadPart", "AbortMultipartUpload"},
		},
		{
			name: "abort NoSuchUpload permits classified fallback",
			configure: func(fixture *directFetchMultipartFixture, _ *Fs) {
				fixture.partModes = map[int]string{1: "source"}
				fixture.abortMode = "no-such-upload"
			},
			wantPart: true, wantAbort: true, wantCantCopy: true,
		},
		{
			name: "HEAD error after completion never aborts",
			configure: func(fixture *directFetchMultipartFixture, _ *Fs) {
				fixture.headMode = "error"
			},
			wantPart: true, wantComplete: true, wantHead: true,
			wantOperations: []string{"HeadObject"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, ci := fs.AddConfig(context.Background())
			ci.LowLevelRetries = 1
			fixture := &directFetchMultipartFixture{headSize: directFetchMaxSize + 1}
			f := newDirectFetchTestFs(ctx, t, "Fastly", fixture)
			f.opt.DirectFetchChunkSize = fs.SizeSuffix(directFetchMaxSize)
			f.opt.DirectFetchConcurrency = 1
			tt.configure(fixture, f)

			got, err := f.ServerSideFetchURL(ctx, "target", "https://source.example/large",
				directFetchSource(directFetchMaxSize+1, nil))
			assert.Nil(t, got)
			require.Error(t, err)
			assert.Equal(t, tt.wantCantCopy, errors.Is(err, fs.ErrorCantCopy))
			for _, operation := range tt.wantOperations {
				assert.Contains(t, err.Error(), operation)
			}
			if len(tt.wantOperations) == 1 {
				var directErr *directFetchError
				require.ErrorAs(t, err, &directErr)
				assert.Equal(t, tt.wantOperations[0], directErr.operation)
			}
			for _, content := range tt.wantErrorContent {
				assert.Contains(t, err.Error(), content)
			}

			events, _, _, _ := fixture.snapshot()
			assert.Equal(t, tt.wantPart, len(directFetchMultipartEvents(events, "part-")) > 0)
			assert.Equal(t, tt.wantComplete, len(directFetchMultipartEvents(events, "complete")) > 0)
			assert.Equal(t, tt.wantAbort, len(directFetchMultipartEvents(events, "abort")) > 0)
			assert.Equal(t, tt.wantHead, len(directFetchMultipartEvents(events, "head")) > 0)
			if tt.wantAbort {
				abortIndex := directFetchEventIndex(events, "abort")
				for i, event := range events {
					if strings.HasSuffix(event, "-finish") {
						assert.Less(t, i, abortIndex)
					}
				}
			}
		})
	}
}

func TestDirectFetchMultipartImpossiblePlanDoesNotCreate(t *testing.T) {
	ctx, _ := fs.AddConfig(context.Background())
	fixture := &directFetchMultipartFixture{}
	f := newDirectFetchTestFs(ctx, t, "Fastly", fixture)
	f.opt.DirectFetchChunkSize = minChunkSize
	f.opt.MaxUploadParts = maxUploadParts

	got, err := f.ServerSideFetchURL(ctx, "target", "https://source.example/impossible", directFetchSource(math.MaxInt64, nil))
	assert.Nil(t, got)
	assert.ErrorIs(t, err, fs.ErrorCantCopy)
	events, _, _, _ := fixture.snapshot()
	assert.Empty(t, events)
}

type directFetchObservingHTTPClient struct {
	base interface {
		Do(*http.Request) (*http.Response, error)
	}
	onPutDone func(*http.Request)
	onDelete  func(*http.Request)
}

func (c *directFetchObservingHTTPClient) Do(request *http.Request) (*http.Response, error) {
	if request.Method == http.MethodDelete {
		c.onDelete(request)
	}
	response, err := c.base.Do(request)
	if request.Method == http.MethodPut && c.onPutDone != nil {
		c.onPutDone(request)
	}
	return response, err
}

func TestDirectFetchMultipartFailurePreservesPrimaryAndAbortCauses(t *testing.T) {
	ctx, ci := fs.AddConfig(context.Background())
	ci.LowLevelRetries = 1
	fixture := &directFetchMultipartFixture{headSize: directFetchMaxSize + 1}
	f := newDirectFetchTestFs(ctx, t, "Fastly", fixture)
	f.opt.DirectFetchChunkSize = fs.SizeSuffix(directFetchMaxSize)
	f.opt.DirectFetchConcurrency = 1
	primaryErr := errors.New("primary upload sentinel")
	abortErr := errors.New("abort sentinel")
	options := f.c.Options()
	baseHTTPClient := options.HTTPClient
	options.HTTPClient = directFetchHTTPClientFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodPut && request.URL.Query().Get("uploadId") != "" {
			return nil, primaryErr
		}
		if request.Method == http.MethodDelete && request.URL.Query().Get("uploadId") != "" {
			return nil, abortErr
		}
		return baseHTTPClient.Do(request)
	})
	f.c = awss3.New(options)

	got, err := f.ServerSideFetchURL(ctx, "target", "https://source.example/large",
		directFetchSource(directFetchMaxSize+1, nil))

	assert.Nil(t, got)
	require.Error(t, err)
	assert.ErrorIs(t, err, primaryErr)
	assert.ErrorIs(t, err, abortErr)
	assert.NotErrorIs(t, err, fs.ErrorCantCopy)
}

type directFetchHTTPClientFunc func(*http.Request) (*http.Response, error)

func (f directFetchHTTPClientFunc) Do(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestDirectFetchCancelAbortsAfterWorkersWithIndependentDeadline(t *testing.T) {
	baseCtx, ci := fs.AddConfig(context.Background())
	ci.LowLevelRetries = 1
	ctx, cancel := context.WithCancel(baseCtx)
	defer cancel()
	fixture := &directFetchMultipartFixture{headSize: 2*directFetchMaxSize + 1}
	partStarted := make(chan struct{})
	fixture.partHook = func(_ http.ResponseWriter, r *http.Request, _ int) {
		close(partStarted)
		<-r.Context().Done()
	}
	f := newDirectFetchTestFs(baseCtx, t, "Fastly", fixture)
	f.opt.DirectFetchChunkSize = fs.SizeSuffix(directFetchMaxSize)
	f.opt.DirectFetchConcurrency = 1
	options := f.c.Options()
	partTransportDone := make(chan struct{})
	deleteObserved := make(chan struct{})
	var deadline time.Time
	var cleanupContextErr error
	options.HTTPClient = &directFetchObservingHTTPClient{
		base: options.HTTPClient,
		onPutDone: func(*http.Request) {
			close(partTransportDone)
		},
		onDelete: func(request *http.Request) {
			select {
			case <-partTransportDone:
			default:
				t.Error("abort began before the in-flight part transport returned")
			}
			deadline, _ = request.Context().Deadline()
			cleanupContextErr = request.Context().Err()
			close(deleteObserved)
		},
	}
	f.c = awss3.New(options)

	result := make(chan error, 1)
	go func() {
		_, err := f.ServerSideFetchURL(ctx, "target", "https://source.example/cancel",
			directFetchSource(2*directFetchMaxSize+1, nil))
		result <- err
	}()
	select {
	case <-partStarted:
		cancel()
	case err := <-result:
		require.FailNow(t, "multipart part did not start", "operation returned early: %v", err)
	}
	err := <-result
	<-deleteObserved
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	assert.NotErrorIs(t, err, fs.ErrorCantCopy)
	assert.NoError(t, cleanupContextErr)
	assert.False(t, deadline.IsZero())
	remaining := time.Until(deadline)
	assert.Positive(t, remaining)
	assert.LessOrEqual(t, remaining, directFetchCleanupTimeout)

	events, _, _, _ := fixture.snapshot()
	assert.Contains(t, events, "abort")
	assert.Empty(t, directFetchMultipartEvents(events, "complete"))
	assert.Empty(t, directFetchMultipartEvents(events, "head"))
}

func TestDirectFetchMultipartAbortsOnSignal(t *testing.T) {
	if os.Getenv("RCLONE_TEST_DIRECT_FETCH_SIGNAL") != "" {
		runDirectFetchUntilSignalled(t)
		return
	}
	if runtime.GOOS == "windows" || runtime.GOOS == "plan9" {
		t.Skip("needs SIGTERM")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	// the atexit signal handler calls os.Exit, so signal a child process
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestDirectFetchMultipartAbortsOnSignal$")
	cmd.Env = append(os.Environ(), "RCLONE_TEST_DIRECT_FETCH_SIGNAL=1")
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())
	var lines []string
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if scanner.Text() == "part started" {
			require.NoError(t, cmd.Process.Signal(syscall.SIGTERM))
		}
	}
	var exitErr *exec.ExitError
	require.ErrorAs(t, cmd.Wait(), &exitErr)
	// 128+SIGTERM is the exit code the atexit signal handler uses
	assert.Equal(t, 143, exitErr.ExitCode())
	assert.Contains(t, lines, "part started")
	assert.Contains(t, lines, "aborted")
}

// runDirectFetchUntilSignalled starts a multipart Direct Fetch whose
// parts never finish, printing progress for TestDirectFetchMultipartAbortsOnSignal.
func runDirectFetchUntilSignalled(t *testing.T) {
	ctx, ci := fs.AddConfig(context.Background())
	ci.LowLevelRetries = 1
	fixture := &directFetchMultipartFixture{headSize: 2*directFetchMaxSize + 1}
	fixture.partHook = func(_ http.ResponseWriter, r *http.Request, _ int) {
		fmt.Println("part started")
		<-r.Context().Done()
	}
	f := newDirectFetchTestFs(ctx, t, "Fastly", fixture)
	f.opt.DirectFetchChunkSize = fs.SizeSuffix(directFetchMaxSize)
	f.opt.DirectFetchConcurrency = 1
	options := f.c.Options()
	baseHTTPClient := options.HTTPClient
	options.HTTPClient = directFetchHTTPClientFunc(func(request *http.Request) (*http.Response, error) {
		response, err := baseHTTPClient.Do(request)
		if request.Method == http.MethodDelete && err == nil && response.StatusCode < 300 {
			fmt.Println("aborted")
		}
		return response, err
	})
	f.c = awss3.New(options)
	_, err := f.ServerSideFetchURL(ctx, "target", "https://source.example/signal",
		directFetchSource(2*directFetchMaxSize+1, nil))
	t.Fatalf("fetch returned without being signalled: %v", err)
}

func TestDirectFetchMultipartConcurrentCallsWithoutAccounting(t *testing.T) {
	ctx, ci := fs.AddConfig(context.Background())
	ci.LowLevelRetries = 1
	fixture := &directFetchMultipartFixture{headSize: directFetchMaxSize + 1}
	f := newDirectFetchTestFs(ctx, t, "Fastly", fixture)
	f.opt.DirectFetchChunkSize = fs.SizeSuffix(directFetchMaxSize)
	f.opt.DirectFetchConcurrency = 2
	sharedDummyStarted := directFetchNullAccounter.Started()

	const transfers = 8
	var wg sync.WaitGroup
	errs := make(chan error, transfers)
	for i := range transfers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := f.ServerSideFetchURL(ctx, fmt.Sprintf("target-%d", i), "https://source.example/large",
				directFetchSource(directFetchMaxSize+1, nil))
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	assert.Equal(t, sharedDummyStarted, directFetchNullAccounter.Started())
}

func directFetchEventIndex(items []string, want string) int {
	for i, item := range items {
		if item == want {
			return i
		}
	}
	return -1
}
