package dropbox

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/dropbox/dropbox-sdk-go-unofficial/v6/dropbox/files"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// longpollClient satisfies the longpoll call without network access.
type longpollClient struct {
	files.ContextClient
}

func (c *longpollClient) ListFolderLongpollContext(ctx context.Context, arg *files.ListFolderLongpollArg) (*files.ListFolderLongpollResult, error) {
	return &files.ListFolderLongpollResult{Changes: true}, nil
}

// continueClient serves canned list_folder/continue responses.
type continueClient struct {
	files.ContextClient
	responses []string // raw JSON bodies returned in order
}

func (c *continueClient) ListFolderContinueContext(ctx context.Context, arg *files.ListFolderContinueArg) (*files.ListFolderResult, error) {
	if len(c.responses) == 0 {
		return &files.ListFolderResult{Cursor: arg.Cursor, HasMore: false}, nil
	}
	raw := c.responses[0]
	c.responses = c.responses[1:]
	var res files.ListFolderResult
	err := json.Unmarshal([]byte(raw), &res)
	return &res, err
}

// TestChangeNotifyRunnerTrimsRootCaseInsensitive checks that ChangeNotify
// entries are trimmed of the configured root even when the server-side
// display casing of PathDisplay differs from the casing typed in the config
// (https://github.com/rclone/rclone/issues/9692).
func TestChangeNotifyRunnerTrimsRootCaseInsensitive(t *testing.T) {
	for _, test := range []struct {
		name          string
		root          string // config root (user-typed casing)
		displayPath   string // PathDisplay as reported by Dropbox
		wantEntryPath string
	}{
		{
			name:          "matching case",
			root:          "Docs",
			displayPath:   "/Docs/file.txt",
			wantEntryPath: "file.txt",
		},
		{
			name:          "root casing differs from display",
			root:          "docs", // user typed lowercase
			displayPath:   "/Docs/file.txt",
			wantEntryPath: "file.txt",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			f := &Fs{
				ci:             fs.GetConfig(ctx),
				opt:            Options{Enc: encoder.Standard},
				slashRoot:      "/" + test.root,
				slashRootSlash: "/" + test.root + "/",
				pacer:          fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(0), pacer.MaxSleep(time.Millisecond))),
			}
			f.svc = &longpollClient{}
			f.srv = &continueClient{
				responses: []string{`{
					"entries": [
						{".tag": "file", "name": "file.txt", "path_display": "` + test.displayPath + `"},
						{".tag": "folder", "name": "dir", "path_display": "/` + test.root + `Sub/dir"}
					],
					"cursor": "c1",
					"has_more": false
				}`},
			}

			type notification struct {
				path  string
				entry fs.EntryType
			}
			got := []notification{}
			cursor, err := f.changeNotifyRunner(ctx, func(path string, entryType fs.EntryType) {
				got = append(got, notification{path, entryType})
			}, "start")
			require.NoError(t, err)
			assert.Equal(t, "c1", cursor)

			require.Len(t, got, 2)
			// The file entry must be trimmed despite the root/display casing
			// mismatch.
			assert.Equal(t, test.wantEntryPath, got[0].path)
			assert.Equal(t, fs.EntryObject, got[0].entry)
			assert.Equal(t, "/"+test.root+"Sub/dir", got[1].path)
			assert.Equal(t, fs.EntryDirectory, got[1].entry)
		})
	}
}

// TestChangeNotifyRunnerKeepsUnmatchedPaths checks that paths outside the
// configured root still reach the notify function untrimmed.
func TestChangeNotifyRunnerKeepsUnmatchedPaths(t *testing.T) {
	ctx := context.Background()
	f := &Fs{
		ci:             fs.GetConfig(ctx),
		opt:            Options{Enc: encoder.Standard},
		slashRoot:      "",
		slashRootSlash: "/",
		pacer:          fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(0), pacer.MaxSleep(time.Millisecond))),
	}
	f.svc = &longpollClient{}
	f.srv = &continueClient{
		responses: []string{`{
			"entries": [
				{".tag": "file", "name": "file.txt", "path_display": "/some/path/file.txt"}
			],
			"cursor": "c2",
			"has_more": false
		}`},
	}

	paths := []string{}
	_, err := f.changeNotifyRunner(ctx, func(path string, _ fs.EntryType) {
		paths = append(paths, path)
	}, "start")
	require.NoError(t, err)
	assert.Equal(t, []string{"some/path/file.txt"}, paths)
}
