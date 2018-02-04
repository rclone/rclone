package alias

import (
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"testing"

	_ "github.com/ncw/rclone/backend/local" // pull in test backend
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/config"
	"github.com/stretchr/testify/require"
)

var (
	remoteName = "TestAlias"
	testPath   = "test"
	filesPath  = filepath.Join(testPath, "files")
)

func prepare(t *testing.T, root string) {
	config.LoadConfig()

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
	for _, test := range []struct {
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
			{"four", -1, true},
			{"one%.txt", 6, false},
			{"three", -1, true},
			{"two.html", 7, false},
		}},
		{"four", "../three", "", true, []testEntry{
			{"underthree.txt", 9, false},
		}},
	} {
		t.Run(fmt.Sprintf("%s fs %s list %s", test.remoteRoot, test.fsRoot, test.fsList), func(t *testing.T) {
			remoteRoot, err := filepath.Abs(filepath.FromSlash(path.Join("test/files", test.remoteRoot)))
			require.NoError(t, err)
			prepare(t, remoteRoot)
			f, err := fs.NewFs(fmt.Sprintf("%s:%s", remoteName, test.fsRoot))
			require.NoError(t, err)
			gotEntries, err := f.List(test.fsList)
			require.NoError(t, err)

			sort.Sort(gotEntries)

			require.Equal(t, len(test.entries), len(gotEntries))
			for i, gotEntry := range gotEntries {
				wantEntry := test.entries[i]

				require.Equal(t, wantEntry.remote, gotEntry.Remote())
				require.Equal(t, wantEntry.size, int64(gotEntry.Size()))
				_, isDir := gotEntry.(fs.Directory)
				require.Equal(t, wantEntry.isDir, isDir)
			}
		})
	}
}

func TestNewFSNoRemote(t *testing.T) {
	prepare(t, "")
	f, err := fs.NewFs(fmt.Sprintf("%s:", remoteName))

	require.Error(t, err)
	require.Nil(t, f)
}

func TestNewFSInvalidRemote(t *testing.T) {
	prepare(t, "not_existing_test_remote:")
	f, err := fs.NewFs(fmt.Sprintf("%s:", remoteName))

	require.Error(t, err)
	require.Nil(t, f)
}
