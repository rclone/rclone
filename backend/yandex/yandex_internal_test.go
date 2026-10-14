package yandex

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSetRoot checks that setRoot addresses paths with the "disk:/" prefix
// by default, and with the "app:/" prefix when app_folder is set.
func TestSetRoot(t *testing.T) {
	for _, test := range []struct {
		appFolder    bool
		root         string
		wantDiskRoot string
		wantFilePath string
	}{
		{false, "", "disk:/", "disk:/file.txt"},
		{false, "backups", "disk:/backups/", "disk:/backups/file.txt"},
		{true, "", "app:/", "app:/file.txt"},
		{true, "backups", "app:/backups/", "app:/backups/file.txt"},
	} {
		f := &Fs{opt: Options{AppFolder: test.appFolder}}
		f.setRoot(test.root)
		assert.Equal(t, test.wantDiskRoot, f.diskRoot, "diskRoot for root=%q app_folder=%v", test.root, test.appFolder)
		assert.Equal(t, test.wantFilePath, f.filePath("file.txt"), "filePath for root=%q app_folder=%v", test.root, test.appFolder)
	}
}

// TestCreateDir checks the path CreateDir sends to the API, both for the
// default disk:/ root and for the app:/ root used when app_folder is set.
// Yandex Disk does not resolve bare relative paths under the app folder
// root, so those must be sent with an explicit "app:/" prefix, but the
// default disk:/ behaviour (relying on the API's implicit disk root for
// bare relative paths, only prefixing when the path itself contains a ':')
// must stay unchanged for existing users.
func TestCreateDir(t *testing.T) {
	for _, test := range []struct {
		appFolder bool
		path      string
		wantPath  string
	}{
		{false, "/backups/", "/backups/"},
		{false, "/foo:bar/", "disk:/foo:bar/"},
		{true, "/backups/", "app:/backups/"},
		{true, "/foo:bar/", "app:/foo:bar/"},
	} {
		var gotPath string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Query().Get("path")
			w.WriteHeader(http.StatusCreated)
		}))

		ctx := context.Background()
		f := &Fs{
			opt:   Options{AppFolder: test.appFolder},
			srv:   rest.NewClient(server.Client()).SetRoot(server.URL),
			pacer: fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(time.Millisecond), pacer.MaxSleep(10*time.Millisecond))),
		}
		f.setRoot("")

		err := f.CreateDir(ctx, test.path)
		require.NoError(t, err, "path=%q app_folder=%v", test.path, test.appFolder)
		assert.Equal(t, test.wantPath, gotPath, "path=%q app_folder=%v", test.path, test.appFolder)
		server.Close()
	}
}

// TestDirMoveRoot checks that DirMove refuses to move a directory to the
// container root, both for the default disk:/ root and for the app:/ root
// used when app_folder is set.
func TestDirMoveRoot(t *testing.T) {
	for _, appFolder := range []bool{false, true} {
		f := &Fs{opt: Options{AppFolder: appFolder}}
		f.setRoot("")

		err := f.DirMove(context.Background(), f, "somedir", "")
		assert.EqualError(t, err, "can't move root directory", "app_folder=%v", appFolder)
	}
}
