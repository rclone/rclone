package dropbox

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/dropbox/dropbox-sdk-go-unofficial/v6/dropbox"
	"github.com/dropbox/dropbox-sdk-go-unofficial/v6/dropbox/files"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type changeNotifyClient struct {
	files.Client
	entries []files.IsMetadata
}

func (c changeNotifyClient) ListFolderLongpoll(*files.ListFolderLongpollArg) (*files.ListFolderLongpollResult, error) {
	return files.NewListFolderLongpollResult(true), nil
}

func (c changeNotifyClient) ListFolderContinue(*files.ListFolderContinueArg) (*files.ListFolderResult, error) {
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
		_, err = f.srv.PaperCreate(&arg, reader)
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
