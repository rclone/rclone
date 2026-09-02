package dropbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dropbox/dropbox-sdk-go-unofficial/v6/dropbox"
	"github.com/dropbox/dropbox-sdk-go-unofficial/v6/dropbox/files"
	"github.com/dropbox/dropbox-sdk-go-unofficial/v6/dropbox/sharing"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/rclone/rclone/lib/batcher"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type paperMetadataClient struct {
	files.ContextClient
	info *files.FileMetadata
}

func (c paperMetadataClient) GetMetadataContext(ctx context.Context, arg *files.GetMetadataArg) (files.IsMetadata, error) {
	if arg.Path == "document" {
		return c.info, nil
	}
	return nil, files.GetMetadataAPIError{
		APIError: dropbox.APIError{ErrorSummary: "path/not_found/"},
		EndpointError: &files.GetMetadataError{
			Tagged: dropbox.Tagged{Tag: files.GetMetadataErrorPath},
			Path: &files.LookupError{
				Tagged: dropbox.Tagged{Tag: files.LookupErrorNotFound},
			},
		},
	}
}

func TestInternalGetMetadataCancellation(t *testing.T) {
	requestStarted := make(chan struct{})
	releaseRequest := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(requestStarted)
		<-releaseRequest
	}))
	defer server.Close()
	defer close(releaseRequest)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := &Fs{
		srv: files.NewContext(dropbox.Config{
			Client: server.Client(),
			URLGenerator: func(hostType string, namespace string, route string) string {
				return server.URL
			},
		}),
		pacer: fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(0), pacer.MaxSleep(time.Millisecond))),
	}

	result := make(chan getMetadataResult, 1)
	go func() {
		result <- f.getMetadata(ctx, "/file")
	}()

	select {
	case <-requestStarted:
	case <-time.After(time.Second):
		t.Fatal("Dropbox request did not start")
	}
	cancel()

	select {
	case res := <-result:
		require.ErrorIs(t, res.err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("Dropbox request did not observe cancellation")
	}
}

type changeNotifyClient struct {
	files.ContextClient
	entries []files.IsMetadata
}

func (c changeNotifyClient) ListFolderLongpollContext(context.Context, *files.ListFolderLongpollArg) (*files.ListFolderLongpollResult, error) {
	return files.NewListFolderLongpollResult(true), nil
}

func (c changeNotifyClient) ListFolderContinueContext(context.Context, *files.ListFolderContinueArg) (*files.ListFolderResult, error) {
	return files.NewListFolderResult(c.entries, "next", false), nil
}

func TestTrimPrefixFold(t *testing.T) {
	for _, test := range []struct {
		name   string
		path   string
		prefix string
		want   string
	}{
		{name: "exact casing", path: "/Docs/Sub", prefix: "/Docs/", want: "Sub"},
		{name: "different casing", path: "/docs/Sub", prefix: "/Docs/", want: "Sub"},
		{name: "different UTF-8 lengths", path: "/K/Sub", prefix: "/K/", want: "Sub"},
		{name: "exact root", path: "/docs", prefix: "/Docs/", want: ""},
		{name: "sibling boundary", path: "/Docs2/File", prefix: "/Docs/", want: "/Docs2/File"},
		{name: "root remote", path: "/File", prefix: "/", want: "File"},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, trimPrefixFold(test.path, test.prefix))
		})
	}
}

func TestChangeNotifyTrimsRootCaseInsensitively(t *testing.T) {
	ctx := context.Background()
	client := changeNotifyClient{entries: []files.IsMetadata{
		&files.FolderMetadata{Metadata: files.Metadata{PathDisplay: "/docs/Sub"}},
		&files.FileMetadata{Metadata: files.Metadata{PathDisplay: "/docs/Sub/File.TXT"}},
		&files.DeletedMetadata{Metadata: files.Metadata{PathDisplay: "/docs/Gone.md"}},
	}}
	f := &Fs{
		ci:             fs.GetConfig(ctx),
		srv:            client,
		svc:            client,
		slashRootSlash: "/Docs/",
		pacer:          fs.NewPacer(ctx, pacer.NewDefault()),
	}

	type notification struct {
		path      string
		entryType fs.EntryType
	}
	var notifications []notification
	cursor, err := f.changeNotifyRunner(ctx, func(path string, entryType fs.EntryType) {
		notifications = append(notifications, notification{path: path, entryType: entryType})
	}, "start")
	require.NoError(t, err)
	assert.Equal(t, "next", cursor)
	assert.Equal(t, []notification{
		{path: "Sub", entryType: fs.EntryDirectory},
		{path: "Sub/File.TXT", entryType: fs.EntryObject},
		{path: "Gone.md", entryType: fs.EntryObject},
	}, notifications)
}

func TestInternalCheckPathLength(t *testing.T) {
	rep := func(n int, r rune) (out string) {
		rs := make([]rune, n)
		for i := range rs {
			rs[i] = r
		}
		return string(rs)
	}
	for _, test := range []struct {
		in string
		ok bool
	}{
		{in: "", ok: true},
		{in: rep(maxFileNameLength, 'a'), ok: true},
		{in: rep(maxFileNameLength+1, 'a'), ok: false},
		{in: rep(maxFileNameLength, '£'), ok: true},
		{in: rep(maxFileNameLength+1, '£'), ok: false},
		{in: rep(maxFileNameLength, '☺'), ok: true},
		{in: rep(maxFileNameLength+1, '☺'), ok: false},
		{in: rep(maxFileNameLength, '你'), ok: true},
		{in: rep(maxFileNameLength+1, '你'), ok: false},
		{in: "/ok/ok", ok: true},
		{in: "/ok/" + rep(maxFileNameLength, 'a') + "/ok", ok: true},
		{in: "/ok/" + rep(maxFileNameLength+1, 'a') + "/ok", ok: false},
		{in: "/ok/" + rep(maxFileNameLength, '£') + "/ok", ok: true},
		{in: "/ok/" + rep(maxFileNameLength+1, '£') + "/ok", ok: false},
		{in: "/ok/" + rep(maxFileNameLength, '☺') + "/ok", ok: true},
		{in: "/ok/" + rep(maxFileNameLength+1, '☺') + "/ok", ok: false},
		{in: "/ok/" + rep(maxFileNameLength, '你') + "/ok", ok: true},
		{in: "/ok/" + rep(maxFileNameLength+1, '你') + "/ok", ok: false},
	} {

		err := checkPathLength(test.in)
		assert.Equal(t, test.ok, err == nil, test.in)
	}
}

func TestPaperExportRemote(t *testing.T) {
	ctx := context.Background()
	info := &files.FileMetadata{
		ExportInfo: &files.ExportInfo{ExportAs: "markdown"},
	}
	f := &Fs{
		exportExts: []exportExtension{"md"},
		pacer:      fs.NewPacer(ctx, pacer.NewDefault()),
		srv:        paperMetadataClient{info: info},
	}

	direct, err := f.NewObject(ctx, "document.md")
	require.NoError(t, err)
	assert.Equal(t, "document.md", direct.Remote())

	listed, err := f.newObjectWithInfo(ctx, "document.md", info)
	require.NoError(t, err)
	assert.Equal(t, "document.md.md", listed.Remote())

	legacy, err := f.newObjectWithInfo(ctx, "document.paper", info)
	require.NoError(t, err)
	assert.Equal(t, "document.md", legacy.Remote())
}

// uploadSessionClient is a mock files.ContextClient which records the
// chunked upload calls made to it
type uploadSessionClient struct {
	files.ContextClient
	appends      int    // number of UploadSessionAppendV2Context calls
	maxAppends   int    // fail the append after this many calls to stop runaway loops
	bytesWritten int64  // bytes received by UploadSessionAppendV2Context
	finishCalled bool   // set if UploadSessionFinishContext was called
	appended     func() // if set, called after each successful append
}

var errTooManyAppends = errors.New("too many appends - upload looping?")

func (c *uploadSessionClient) UploadSessionStartContext(ctx context.Context, arg *files.UploadSessionStartArg, content io.Reader) (*files.UploadSessionStartResult, error) {
	return &files.UploadSessionStartResult{SessionId: "session"}, nil
}

func (c *uploadSessionClient) UploadSessionAppendV2Context(ctx context.Context, arg *files.UploadSessionAppendArg, content io.Reader) error {
	// the real client fails the request if the context is cancelled
	if err := ctx.Err(); err != nil {
		return err
	}
	c.appends++
	if c.appends > c.maxAppends {
		return errTooManyAppends
	}
	n, err := io.Copy(io.Discard, content)
	if err != nil {
		return err
	}
	c.bytesWritten += n
	if c.appended != nil {
		c.appended()
	}
	return nil
}

func (c *uploadSessionClient) UploadSessionFinishContext(ctx context.Context, arg *files.UploadSessionFinishArg, content io.Reader) (*files.FileMetadata, error) {
	c.finishCalled = true
	return &files.FileMetadata{}, nil
}

// newUploadTestFs makes an Fs with a mock srv for testing uploadChunked
func newUploadTestFs(t *testing.T, srv files.ContextClient, chunkSize fs.SizeSuffix) *Fs {
	ctx := context.Background()
	f := &Fs{
		pacer: fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(time.Millisecond), pacer.MaxSleep(2*time.Millisecond))),
		srv:   srv,
	}
	f.opt.ChunkSize = chunkSize
	batcherOptions := defaultBatcherOptions
	batcherOptions.Mode = "off"
	var err error
	f.batcher, err = batcher.New(ctx, f, f.commitBatch, batcherOptions)
	require.NoError(t, err)
	return f
}

// endlessReader supplies bytes forever
type endlessReader struct{}

func (endlessReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'x'
	}
	return len(p), nil
}

func TestUploadChunkedEarlyEOF(t *testing.T) {
	ctx := context.Background()

	t.Run("MultiChunk", func(t *testing.T) {
		// The declared size spans 4 chunks but the source ends after 1.5
		client := &uploadSessionClient{maxAppends: 8}
		f := newUploadTestFs(t, client, 100)
		o := &Object{fs: f, remote: "test.bin"}
		_, err := o.uploadChunked(ctx, strings.NewReader(strings.Repeat("a", 150)), files.NewCommitInfo("/test.bin"), 400)
		require.Error(t, err)
		assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
		assert.False(t, client.finishCalled, "must not commit a truncated upload")
	})

	t.Run("SingleChunk", func(t *testing.T) {
		// The declared size fits in one chunk but the source ends early
		client := &uploadSessionClient{maxAppends: 8}
		f := newUploadTestFs(t, client, 500)
		o := &Object{fs: f, remote: "test.bin"}
		_, err := o.uploadChunked(ctx, strings.NewReader(strings.Repeat("a", 150)), files.NewCommitInfo("/test.bin"), 400)
		require.Error(t, err)
		assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
		assert.False(t, client.finishCalled, "must not commit a truncated upload")
	})

	t.Run("Complete", func(t *testing.T) {
		// A source which supplies exactly the declared size uploads OK
		client := &uploadSessionClient{maxAppends: 8}
		f := newUploadTestFs(t, client, 100)
		o := &Object{fs: f, remote: "test.bin"}
		entry, err := o.uploadChunked(ctx, strings.NewReader(strings.Repeat("a", 250)), files.NewCommitInfo("/test.bin"), 250)
		require.NoError(t, err)
		require.NotNil(t, entry)
		assert.True(t, client.finishCalled)
		assert.Equal(t, int64(250), client.bytesWritten)
	})
}

func TestUploadChunkedCancel(t *testing.T) {
	// Cancelling the context must stop the upload even though every
	// append is succeeding
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := &uploadSessionClient{maxAppends: 8}
	client.appended = func() {
		if client.appends == 2 {
			cancel()
		}
	}
	f := newUploadTestFs(t, client, 100)
	o := &Object{fs: f, remote: "test.bin"}
	_, err := o.uploadChunked(ctx, endlessReader{}, files.NewCommitInfo("/test.bin"), -1)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	assert.False(t, client.finishCalled)
}

func (f *Fs) importPaperForTest(t *testing.T) {
	content := `# test doc

Lorem ipsum __dolor__ sit amet
[link](http://google.com)
`

	arg := files.PaperCreateArg{
		Path:         f.slashRootSlash + "export.paper",
		ImportFormat: &files.ImportFormat{Tagged: dropbox.Tagged{Tag: files.ImportFormatMarkdown}},
	}
	var err error
	err = f.pacer.Call(func() (bool, error) {
		reader := strings.NewReader(content)
		_, err = f.srv.PaperCreateContext(context.Background(), &arg, reader)
		return shouldRetry(context.Background(), err)
	})
	require.NoError(t, err)
}

func (f *Fs) InternalTestPaperExport(t *testing.T) {
	ctx := context.Background()
	f.importPaperForTest(t)

	f.exportExts = []exportExtension{"html"}

	obj, err := f.NewObject(ctx, "export.html")
	require.NoError(t, err)

	rc, err := obj.Open(ctx)
	require.NoError(t, err)
	defer func() { require.NoError(t, rc.Close()) }()

	buf, err := io.ReadAll(rc)
	require.NoError(t, err)
	text := string(buf)

	for _, excerpt := range []string{
		"Lorem ipsum",
		"<b>dolor</b>",
		`href="http://google.com"`,
	} {
		require.Contains(t, text, excerpt)
	}
}
func (f *Fs) InternalTest(t *testing.T) {
	t.Run("PaperExport", f.InternalTestPaperExport)
}

var _ fstests.InternalTester = (*Fs)(nil)

// newSharingTestFs builds a minimal *Fs whose sharing client talks to a
// local mock server instead of the real Dropbox API.
func newSharingTestFs(t *testing.T, handler http.HandlerFunc) *Fs {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	cfg := dropbox.Config{
		Client: &http.Client{Transport: http.DefaultTransport},
		URLGenerator: func(hostType, namespace, route string) string {
			return fmt.Sprintf("%s/2/%s/%s", ts.URL, namespace, route)
		},
	}

	return &Fs{
		opt: Options{
			Enc: encoder.Base |
				encoder.EncodeBackSlash |
				encoder.EncodeDel |
				encoder.EncodeRightSpace |
				encoder.EncodeInvalidUtf8,
		},
		sharing: sharing.NewContext(cfg),
		pacer:   fs.NewPacer(context.Background(), pacer.NewDefault(pacer.MinSleep(time.Millisecond))),
	}
}

func mustParseDBXTime(t *testing.T, s string) *dropbox.DBXTime {
	t.Helper()
	tm, err := time.Parse(time.RFC3339, s)
	require.NoError(t, err)
	return (*dropbox.DBXTime)(&tm)
}

func receivedFilesHandler(t *testing.T, rawName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "list_received_files") {
			http.NotFound(w, r)
			return
		}
		resp := sharing.ListFilesResult{
			Entries: []*sharing.SharedFileMetadata{{
				Id:          "id:123",
				Name:        rawName,
				PreviewUrl:  "https://dropbox.com/preview/123",
				Policy:      &sharing.FolderPolicy{},
				TimeInvited: mustParseDBXTime(t, "2024-01-01T00:00:00Z"),
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

// TestListReceivedFilesDecodesName confirms received-file names are decoded
// via Enc.ToStandardName before being returned, matching listSharedFolders'
// handling of shared-folder names.
func TestListReceivedFilesDecodesName(t *testing.T) {
	const rawName = "report.txt␠"
	f := newSharingTestFs(t, receivedFilesHandler(t, rawName))

	wantName := f.opt.Enc.ToStandardName(rawName)
	require.NotEqual(t, rawName, wantName)

	var gotNames []string
	err := f.listReceivedFiles(context.Background(), func(entry fs.DirEntry) error {
		gotNames = append(gotNames, entry.Remote())
		return nil
	})
	require.NoError(t, err)
	require.Len(t, gotNames, 1)
	assert.Equal(t, wantName, gotNames[0])
}

// TestFindSharedFileResolvesDecodedName confirms findSharedFile can look up
// a received file by its standard, decoded rclone-visible name.
func TestFindSharedFileResolvesDecodedName(t *testing.T) {
	const rawName = "report.txt␠"
	f := newSharingTestFs(t, receivedFilesHandler(t, rawName))

	encodedName := f.opt.Enc.ToStandardName(rawName)
	require.NotEqual(t, rawName, encodedName)

	o, err := f.findSharedFile(context.Background(), encodedName)
	require.NoError(t, err)
	assert.Equal(t, encodedName, o.remote)
}
