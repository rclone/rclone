package fichier

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListFolderDates(t *testing.T) {
	for _, test := range []struct {
		name    string
		date    string
		wantErr bool
	}{
		{name: "dated", date: "2026-01-31 20:15:26"},
		{name: "unknown"},
		{name: "invalid", date: "not-a-date", wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, dir := range []string{"", "parent"} {
				t.Run(dir, func(t *testing.T) {
					ctx := context.Background()
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						assert.Equal(t, http.MethodPost, r.Method)
						assert.Empty(t, r.Header.Get("Authorization"))
						w.Header().Set("Content-Type", "application/json")
						switch r.URL.Path {
						case "/file/ls.cgi":
							assert.NoError(t, json.NewEncoder(w).Encode(FilesList{Status: "OK", Items: []File{{Filename: "file.txt", Size: 5}}}))
						case "/folder/ls.cgi":
							assert.NoError(t, json.NewEncoder(w).Encode(FoldersList{Status: "OK", SubFolders: []Folder{
								{ID: 42, Name: "linked", CreateDate: test.date},
								{ID: 43, Name: "dated", CreateDate: "2026-01-02 03:04:05"},
							}}))
						default:
							t.Errorf("unexpected request: %s", r.URL.Path)
							http.NotFound(w, r)
						}
					}))
					defer server.Close()
					f := &Fs{
						rest:  rest.NewClient(server.Client()).SetRoot(server.URL),
						pacer: fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(0))),
					}
					f.dirCache = dircache.New("", rootID, f)
					require.NoError(t, f.dirCache.FindRoot(ctx, false))
					if dir != "" {
						f.dirCache.Put(dir, "7")
					}
					entries, err := f.List(ctx, dir)
					if test.wantErr {
						require.ErrorContains(t, err, "not-a-date")
						return
					}
					require.NoError(t, err)
					require.Len(t, entries, 3)
					assert.Equal(t, path.Join(dir, "file.txt"), entries[0].Remote())
					assert.Equal(t, int64(5), entries[0].Size())
					linked, ok := entries[1].(fs.Directory)
					require.True(t, ok)
					assert.Equal(t, path.Join(dir, "linked"), linked.Remote())
					assert.Equal(t, "42", linked.ID())
					wantTime := time.Time(fs.GetConfig(ctx).DefaultTime)
					if test.date != "" {
						wantTime = time.Date(2026, 1, 31, 20, 15, 26, 0, time.UTC)
					}
					assert.Equal(t, wantTime, linked.ModTime(ctx))
					assert.Equal(t, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), entries[2].ModTime(ctx))
					id, found := f.dirCache.Get(linked.Remote())
					assert.True(t, found)
					assert.Equal(t, "42", id)
				})
			}
		})
	}
}
