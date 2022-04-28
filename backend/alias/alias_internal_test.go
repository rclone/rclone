package alias

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"testing"

	_ "github.com/rclone/rclone/backend/local" // pull in test backend
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configfile"
	"github.com/stretchr/testify/require"
)

var (
	remoteName = "TestAlias"
)

func prepare(t *testing.T, root string) {
	configfile.Install()

	// Configure the remote
	config.FileSet(remoteName, "type", "alias")
	config.FileSet(remoteName, "remote", root)
}

func TestNewFS(t *testing.T) {
	type testEntry struct {
		remote string
		size   int64
		isDir  bool
	}
	for testi, test := range []struct {
		remoteRoot string
		fsRoot     string
		fsList     string
		wantOK     bool
		entries    []testEntry
	}{
		{"", "", "", true, []testEntry{
			{"four", -1, true},
			{"one%.txt", 6, false},
			{"three", -1, true},
			{"two.html", 7, false},
		}},
		{"", "four", "", true, []testEntry{
			{"five", -1, true},
			{"under four.txt", 9, false},
		}},
		{"", "", "four", true, []testEntry{
			{"four/five", -1, true},
			{"four/under four.txt", 9, false},
		}},
		{"four", "..", "", true, []testEntry{
			{"five", -1, true},
			{"under four.txt", 9, false},
		}},
		{"", "../../three", "", true, []testEntry{
			{"underthree.txt", 9, false},
		}},
		{"four", "../../five", "", true, []testEntry{
			{"underfive.txt", 6, false},
		}},
	} {
		what := fmt.Sprintf("test %d remoteRoot=%q, fsRoot=%q, fsList=%q", testi, test.remoteRoot, test.fsRoot, test.fsList)

		remoteRoot, err := filepath.Abs(filepath.FromSlash(path.Join("test/files", test.remoteRoot)))
		require.NoError(t, err, what)
		prepare(t, remoteRoot)
		f, err := fs.NewFs(context.Background(), fmt.Sprintf("%s:%s", remoteName, test.fsRoot))
		require.NoError(t, err, what)
		gotEntries, err := f.List(context.Background(), test.fsList)
		require.NoError(t, err, what)

		sort.Sort(gotEntries)

		require.Equal(t, len(test.entries), len(gotEntries), what)
		for i, gotEntry := range gotEntries {
			what := fmt.Sprintf("%s, entry=%d", what, i)
			wantEntry := test.entries[i]

			require.Equal(t, wantEntry.remote, gotEntry.Remote(), what)
			require.Equal(t, wantEntry.size, gotEntry.Size(), what)
			_, isDir := gotEntry.(fs.Directory)
			require.Equal(t, wantEntry.isDir, isDir, what)
		}
	}
}

func TestNewFSNoRemote(t *testing.T) {
	prepare(t, "")
	f, err := fs.NewFs(context.Background(), fmt.Sprintf("%s:", remoteName))

	require.Error(t, err)
	require.Nil(t, f)
}

func TestNewFSInvalidRemote(t *testing.T) {
	prepare(t, "not_existing_test_remote:")
	f, err := fs.NewFs(context.Background(), fmt.Sprintf("%s:", remoteName))

	require.Error(t, err)
	require.Nil(t, f)
}
