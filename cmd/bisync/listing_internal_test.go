package bisync

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileListPut(t *testing.T) {
	old := time.Date(2026, 9, 29, 12, 0, 1, 555555555, time.UTC)
	for _, test := range []struct {
		name    string
		modtime time.Time
	}{
		{"slightly older", old.Add(-124 * time.Microsecond)},
		{"slightly newer", old.Add(124 * time.Microsecond)},
		{"same second, truncated", old.Truncate(time.Second)},
		{"same second, rounded up", old.Round(time.Second)},
	} {
		t.Run(test.name, func(t *testing.T) {
			ls := newFileList()
			ls.put("file", 1, old, "", "-", "-")
			ls.put("file", 2, test.modtime, "", "-", "-")
			assert.True(t, test.modtime.Equal(ls.getTime("file")), "got %v, want %v", ls.getTime("file"), test.modtime)
			assert.EqualValues(t, 2, ls.getSize("file"))
		})
	}
}

// Test a file whose new version is less than a second older than the old one
func TestBisyncCloseModtimes(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping test on non local remote")
	}
	ctx := context.Background()
	unchanged := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	newRun := func(t *testing.T) (path1, path2 string, run func(resync bool) error) {
		// t.TempDir includes the test name, which makes the listing names too long
		dir, err := os.MkdirTemp("", "bisync")
		require.NoError(t, err)
		t.Cleanup(func() { _ = os.RemoveAll(dir) })
		path1, path2 = filepath.Join(dir, "path1"), filepath.Join(dir, "path2")
		workdir := filepath.Join(dir, "workdir")
		for _, path := range []string{path1, path2} {
			require.NoError(t, os.Mkdir(path, 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(path, "file1.txt"), []byte("unchanged"), 0o644))
			require.NoError(t, os.Chtimes(filepath.Join(path, "file1.txt"), unchanged, unchanged))
		}
		run = func(resync bool) error {
			f1, err := fs.NewFs(ctx, path1)
			require.NoError(t, err)
			f2, err := fs.NewFs(ctx, path2)
			require.NoError(t, err)
			return Bisync(ctx, f1, f2, &Options{Workdir: workdir, Resync: resync, CheckSync: CheckSyncTrue, MaxDelete: DefaultMaxDelete})
		}
		return path1, path2, run
	}
	writeFile := func(t *testing.T, path, content string, modtime time.Time) {
		file := filepath.Join(path, "file0.txt")
		require.NoError(t, os.WriteFile(file, []byte(content), 0o644))
		require.NoError(t, os.Chtimes(file, modtime, modtime))
	}
	// listingTime returns the modtime recorded for file0.txt in the listing for path (1 or 2)
	listingTime := func(t *testing.T, path1 string, path int) time.Time {
		listings, err := filepath.Glob(filepath.Join(filepath.Dir(path1), "workdir", "*.path"+string(rune('0'+path))+".lst"))
		require.NoError(t, err)
		require.Len(t, listings, 1)
		data, err := os.ReadFile(listings[0])
		require.NoError(t, err)
		for line := range strings.SplitSeq(string(data), "\n") {
			if strings.HasSuffix(line, `"file0.txt"`) {
				fields := strings.Fields(line)
				modtime, err := time.Parse(timeFormat, fields[4])
				require.NoError(t, err)
				return modtime
			}
		}
		t.Fatal("file0.txt not in listing")
		return time.Time{}
	}
	// fileTime returns file0.txt's modtime as the filesystem stored it (Windows keeps 100ns)
	fileTime := func(t *testing.T, path string) time.Time {
		info, err := os.Stat(filepath.Join(path, "file0.txt"))
		require.NoError(t, err)
		return info.ModTime()
	}
	modtime := time.Date(2026, 9, 29, 12, 0, 1, 555555555, time.UTC)

	t.Run("resync", func(t *testing.T) {
		path1, path2, run := newRun(t)
		writeFile(t, path1, "path1", modtime)
		writeFile(t, path2, "path2, newer", modtime.Add(124*time.Microsecond))
		require.NoError(t, run(true))
		assert.True(t, fileTime(t, path2).Equal(listingTime(t, path1, 2)), "Path2 listing has %v, file has %v", listingTime(t, path1, 2), fileTime(t, path2))
	})

	t.Run("older edit", func(t *testing.T) {
		path1, path2, run := newRun(t)
		writeFile(t, path1, "original", modtime)
		require.NoError(t, run(true))
		writeFile(t, path1, "edited", modtime.Add(-200*time.Microsecond))
		require.NoError(t, run(false))
		assert.True(t, fileTime(t, path1).Equal(listingTime(t, path1, 1)), "Path1 listing has %v, file has %v", listingTime(t, path1, 1), fileTime(t, path1))
		assert.True(t, fileTime(t, path2).Equal(listingTime(t, path1, 2)), "Path2 listing has %v, file has %v", listingTime(t, path1, 2), fileTime(t, path2))
	})
}
