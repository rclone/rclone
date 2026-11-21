package bisync

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/cmd/bisync/bilib"
	"github.com/rclone/rclone/cmd/genfilters"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubEditFilters replaces genfilters with a function writing rules
// (or doing nothing if rules is "") and the confirmation with answer,
// recording which Fs was edited and whether the user was asked
func stubEditFilters(t *testing.T, rules string, answer bool) (edited *fs.Fs, asked *bool) {
	edited, asked = new(fs.Fs), new(bool)
	oldEdit, oldConfirm := editFiltersFn, confirmFn
	t.Cleanup(func() { editFiltersFn, confirmFn = oldEdit, oldConfirm })
	editFiltersFn = func(ctx context.Context, f fs.Fs, opt genfilters.Options) error {
		*edited = f
		if rules == "" {
			return nil
		}
		return os.WriteFile(opt.Edit, []byte(rules), 0o644)
	}
	confirmFn = func(bool) bool {
		*asked = true
		return answer
	}
	return edited, asked
}

// newEditFiltersTest makes Path1 with keep.txt, skip.txt and both.txt,
// Path2 with a different both.txt, and the Options to bisync them
func newEditFiltersTest(t *testing.T) (fs1, fs2 fs.Fs, opt *Options) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping test on non local remote")
	}
	dir := t.TempDir()
	path1, path2 := filepath.Join(dir, "path1"), filepath.Join(dir, "path2")
	require.NoError(t, os.Mkdir(path1, 0o755))
	require.NoError(t, os.Mkdir(path2, 0o755))
	for _, name := range []string{"keep.txt", "skip.txt", "both.txt"} {
		require.NoError(t, os.WriteFile(filepath.Join(path1, name), []byte(name), 0o644))
	}
	require.NoError(t, os.WriteFile(filepath.Join(path2, "both.txt"), []byte("Path2 version"), 0o644))
	ctx := context.Background()
	fs1, err := fs.NewFs(ctx, path1)
	require.NoError(t, err)
	fs2, err = fs.NewFs(ctx, path2)
	require.NoError(t, err)
	return fs1, fs2, &Options{
		Workdir:     filepath.Join(dir, "workdir"),
		FiltersFile: filepath.Join(dir, "filters.txt"),
		EditFilters: true,
	}
}

func TestEditFiltersErrors(t *testing.T) {
	fs1, fs2, opt := newEditFiltersTest(t)
	ctx := context.Background()
	stubEditFilters(t, "", false)

	both := *opt
	both.EditFiltersPath2 = true
	_, err := both.editFilters(ctx, fs1, fs2)
	assert.ErrorContains(t, err, "can't be used together")

	noFile := *opt
	noFile.FiltersFile = ""
	_, err = noFile.editFilters(ctx, fs1, fs2)
	assert.ErrorContains(t, err, "needs --filters-file")

	editFiltersFn = func(context.Context, fs.Fs, genfilters.Options) error { return genfilters.ErrCancelled }
	assert.ErrorIs(t, Bisync(ctx, fs1, fs2, opt), genfilters.ErrCancelled)
}

func TestEditFiltersResync(t *testing.T) {
	fs1, fs2, opt := newEditFiltersTest(t)
	edited, asked := stubEditFilters(t, "- /skip.txt\n+ **\n", true)
	require.NoError(t, Bisync(context.Background(), fs1, fs2, opt))
	assert.Equal(t, fs1, *edited)
	assert.True(t, *asked)
	assert.False(t, opt.Resync, "the caller's options are unchanged")

	// the run was a resync with the new filters, preferring Path1
	assert.FileExists(t, filepath.Join(fs2.Root(), "keep.txt"))
	assert.NoFileExists(t, filepath.Join(fs2.Root(), "skip.txt"))
	assert.FileExists(t, opt.FiltersFile+".md5")
	for _, f := range []fs.Fs{fs1, fs2} {
		got, err := os.ReadFile(filepath.Join(f.Root(), "both.txt"))
		require.NoError(t, err)
		assert.Equal(t, "both.txt", string(got))
	}

	// with the filters unchanged it is a normal run without asking
	_, asked = stubEditFilters(t, "", true)
	require.NoError(t, Bisync(context.Background(), fs1, fs2, opt))
	assert.False(t, *asked)
}

func TestEditFiltersDeclined(t *testing.T) {
	fs1, fs2, opt := newEditFiltersTest(t)
	opt.EditFilters, opt.EditFiltersPath2 = false, true
	edited, asked := stubEditFilters(t, "- /skip.txt\n+ **\n", false)
	require.NoError(t, Bisync(context.Background(), fs1, fs2, opt))
	assert.Equal(t, fs2, *edited)
	assert.True(t, *asked)
	// nothing was synced
	assert.NoFileExists(t, filepath.Join(fs2.Root(), "keep.txt"))
	assert.NoFileExists(t, opt.FiltersFile+".md5")
	assert.NoDirExists(t, opt.Workdir)
}

func TestEditFiltersWithResync(t *testing.T) {
	fs1, fs2, opt := newEditFiltersTest(t)
	opt.Resync = true
	_, asked := stubEditFilters(t, "- /skip.txt\n+ **\n", false)
	require.NoError(t, Bisync(context.Background(), fs1, fs2, opt))
	assert.False(t, *asked, "--resync was already asked for")
	assert.FileExists(t, filepath.Join(fs2.Root(), "keep.txt"))
	assert.NoFileExists(t, filepath.Join(fs2.Root(), "skip.txt"))
}

// TestEditFiltersSlowHashResync checks a resync started by editing gets
// the same defaults as one asked for with --resync
func TestEditFiltersSlowHashResync(t *testing.T) {
	fs1, fs2, opt := newEditFiltersTest(t)
	opt.CompareFlag = "size,modtime,checksum"
	opt.Compare.SlowHashSyncOnly = true
	stubEditFilters(t, "- /skip.txt\n+ **\n", true)
	var err error
	out := bilib.CaptureOutput(func() { err = Bisync(context.Background(), fs1, fs2, opt) })
	require.NoError(t, err)
	assert.Contains(t, string(out), "Ignoring checksums during --resync as --slow-hash-sync-only is set.")
}
