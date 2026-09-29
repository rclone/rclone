package bisync_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/fs/rc/jobs"
	"github.com/rclone/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newRcBisync is newRcBisyncCtx with the rc jobs run in a plain context
func newRcBisync(t *testing.T, n int) (path1, path2 string, run func(in rc.Params) error) {
	return newRcBisyncCtx(context.Background(), t, n)
}

// newRcBisyncCtx creates Path1 with n files, resyncs it to Path2, and returns
// the paths and a function which runs sync/bisync on them as an rc job in
// ctx, whose config stands in for the flags the rcd was started with
func newRcBisyncCtx(ctx context.Context, t *testing.T, n int) (path1, path2 string, run func(in rc.Params) error) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping test on non local remote")
	}
	dir := t.TempDir()
	path1 = filepath.Join(dir, "path1")
	path2 = filepath.Join(dir, "path2")
	workdir := filepath.Join(dir, "workdir")
	require.NoError(t, os.Mkdir(path1, 0o755))
	require.NoError(t, os.Mkdir(path2, 0o755))
	for i := range n {
		require.NoError(t, os.WriteFile(filepath.Join(path1, fmt.Sprintf("file%d.txt", i)), []byte("content"), 0o644))
	}
	call := rc.Calls.Get("sync/bisync")
	require.NotNil(t, call)
	runIn := func(ctx context.Context, in rc.Params) error {
		in["path1"] = path1
		in["path2"] = path2
		in["workdir"] = workdir
		_, _, err := jobs.NewJob(ctx, call.Fn, in)
		return err
	}
	require.NoError(t, runIn(context.Background(), rc.Params{"resync": true}))
	return path1, path2, func(in rc.Params) error { return runIn(ctx, in) }
}

func TestRcDryRun(t *testing.T) {
	for _, test := range []struct {
		name   string
		in     rc.Params
		dryRun bool
	}{
		{"dryRun", rc.Params{"dryRun": true}, true},
		{"dry_run", rc.Params{"dry_run": true}, true},
		{"_config", rc.Params{"_config": rc.Params{"DryRun": true}}, true},
		{"dry_run and dryRun false", rc.Params{"dry_run": true, "dryRun": false}, true},
		{"_config and dryRun false", rc.Params{"_config": rc.Params{"DryRun": true}, "dryRun": false}, true},
		{"dryRun false", rc.Params{"dryRun": false}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			path1, path2, run := newRcBisync(t, 1)
			require.NoError(t, os.WriteFile(filepath.Join(path1, "new.txt"), []byte("new"), 0o644))
			require.NoError(t, run(test.in))
			if test.dryRun {
				assert.NoFileExists(t, filepath.Join(path2, "new.txt"))
			} else {
				assert.FileExists(t, filepath.Join(path2, "new.txt"))
			}
		})
	}
}

func TestRcMaxDelete(t *testing.T) {
	for _, test := range []struct {
		name    string
		in      rc.Params
		wantErr string
	}{
		{"default", rc.Params{}, ""},
		{"maxDelete", rc.Params{"maxDelete": 5}, "too many deletes"},
		{"max_delete allows", rc.Params{"max_delete": 20}, ""},
		{"max_delete aborts", rc.Params{"max_delete": 5}, "too many deletes"},
		{"_config MaxDelete aborts", rc.Params{"_config": rc.Params{"MaxDelete": 5}}, "too many deletes"},
		{"max_delete over 100", rc.Params{"max_delete": 500}, "--max-delete"},
	} {
		t.Run(test.name, func(t *testing.T) {
			path1, path2, run := newRcBisync(t, 10)
			require.NoError(t, os.Remove(filepath.Join(path1, "file0.txt")))
			err := run(test.in)
			if test.wantErr != "" {
				assert.ErrorContains(t, err, test.wantErr)
				assert.FileExists(t, filepath.Join(path2, "file0.txt"))
			} else {
				require.NoError(t, err)
				assert.NoFileExists(t, filepath.Join(path2, "file0.txt"))
			}
		})
	}
}

// Test a --max-delete the rcd was started with applies, as on the command line
func TestRcMaxDeleteFromRcd(t *testing.T) {
	for _, test := range []struct {
		maxDelete int64
		wantErr   string
	}{
		{5, "too many deletes"},
		{500, "--max-delete"},
	} {
		t.Run(fmt.Sprint(test.maxDelete), func(t *testing.T) {
			ctx, ci := fs.AddConfig(context.Background())
			ci.MaxDelete = test.maxDelete
			path1, path2, run := newRcBisyncCtx(ctx, t, 10)
			require.NoError(t, os.Remove(filepath.Join(path1, "file0.txt")))
			assert.ErrorContains(t, run(rc.Params{}), test.wantErr)
			assert.FileExists(t, filepath.Join(path2, "file0.txt"))
		})
	}
}

func TestRcInvalidParams(t *testing.T) {
	for _, test := range []struct {
		key   string
		value any
	}{
		{"force", "yes"},
		{"maxDeleteRenamesAware", "yes"},
		{"filtersFile", []string{"filters.txt"}},
		{"checkSync", 1},
		{"conflictResolve", "bogus"},
		{"maxLock", "soon"},
	} {
		t.Run(test.key, func(t *testing.T) {
			path1, path2, run := newRcBisync(t, 1)
			require.NoError(t, os.WriteFile(filepath.Join(path1, "new.txt"), []byte("new"), 0o644))
			err := run(rc.Params{test.key: test.value})
			assert.Error(t, err)
			assert.NoFileExists(t, filepath.Join(path2, "new.txt"))
		})
	}
}

func TestRcBackupDir(t *testing.T) {
	for _, test := range []struct {
		key     string
		changed int // the path whose change overwrites the file on the other path
	}{
		{"backupDir1", 2},
		{"backupdir1", 2},
		{"backupDir2", 1},
		{"backupdir2", 1},
	} {
		t.Run(test.key, func(t *testing.T) {
			path1, path2, run := newRcBisync(t, 2)
			backupDir := filepath.Join(filepath.Dir(path1), "backup")
			changedPath := map[int]string{1: path1, 2: path2}[test.changed]
			require.NoError(t, os.WriteFile(filepath.Join(changedPath, "file0.txt"), []byte("changed"), 0o644))
			require.NoError(t, run(rc.Params{test.key: backupDir}))
			assert.FileExists(t, filepath.Join(backupDir, "file0.txt"))
		})
	}
}

func TestRcEnums(t *testing.T) {
	read := func(t *testing.T, path string) string {
		b, err := os.ReadFile(filepath.Join(path, "file0.txt"))
		require.NoError(t, err)
		return string(b)
	}
	// changeBoth makes file0.txt differ on both paths, with Path2 an hour newer
	changeBoth := func(t *testing.T, path1, path2 string) {
		now := time.Now()
		for path, content := range map[string]string{path1: "path1", path2: "path2 wins"} {
			file := filepath.Join(path, "file0.txt")
			require.NoError(t, os.WriteFile(file, []byte(content), 0o644))
			mtime := now
			if path == path1 {
				mtime = now.Add(-time.Hour)
			}
			require.NoError(t, os.Chtimes(file, mtime, mtime))
		}
	}
	names := func(t *testing.T, path string) (names []string) {
		entries, err := os.ReadDir(path)
		require.NoError(t, err)
		for _, e := range entries {
			names = append(names, e.Name())
		}
		return names
	}
	t.Run("conflictResolve and conflictLoser", func(t *testing.T) {
		path1, path2, run := newRcBisync(t, 2)
		changeBoth(t, path1, path2)
		require.NoError(t, run(rc.Params{"conflictResolve": "path2", "conflictLoser": "delete"}))
		for _, path := range []string{path1, path2} {
			assert.Equal(t, []string{"file0.txt", "file1.txt"}, names(t, path))
			assert.Equal(t, "path2 wins", read(t, path))
		}
	})
	t.Run("conflict defaults", func(t *testing.T) {
		path1, path2, run := newRcBisync(t, 2)
		changeBoth(t, path1, path2)
		require.NoError(t, run(rc.Params{}))
		assert.NotContains(t, names(t, path1), "file0.txt", "with no winner both versions are renamed")
	})
	for _, test := range []struct {
		name string
		in   rc.Params
		want string
	}{
		{"resyncMode default", rc.Params{"resync": true}, "path1"},
		{"resyncMode path2", rc.Params{"resync": true, "resyncMode": "path2"}, "path2 wins"},
	} {
		t.Run(test.name, func(t *testing.T) {
			path1, path2, run := newRcBisync(t, 2)
			changeBoth(t, path1, path2)
			require.NoError(t, run(test.in))
			assert.Equal(t, test.want, read(t, path1))
			assert.Equal(t, test.want, read(t, path2))
		})
	}
	t.Run("checkSync only", func(t *testing.T) {
		path1, path2, run := newRcBisync(t, 1)
		require.NoError(t, os.WriteFile(filepath.Join(path1, "new.txt"), []byte("new"), 0o644))
		require.NoError(t, run(rc.Params{"checkSync": "only"}))
		assert.NoFileExists(t, filepath.Join(path2, "new.txt"))
	})
}
