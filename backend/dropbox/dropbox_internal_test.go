package dropbox

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dropbox/dropbox-sdk-go-unofficial/v6/dropbox"
	"github.com/dropbox/dropbox-sdk-go-unofficial/v6/dropbox/files"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest/fstests"
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
