package bisync

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/lib/random"
	"github.com/stretchr/testify/require"
)

// TestBisyncStaleGroupError checks that an error recorded against a
// long-lived stats group by an earlier, unrelated operation (as happens
// under rclone rcd with a reused "_group") does not make a clean bisync
// run abort as critical, while a listing error during the run itself
// still does.
func TestBisyncStaleGroupError(t *testing.T) {
	ctx := context.Background()
	ctx = accounting.WithStatsGroup(ctx, "bisync-test-"+random.String(8))

	dir1 := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir1, "file1.txt"), []byte("hello"), 0644))
	fs1, err := cache.Get(ctx, dir1)
	require.NoError(t, err)
	fs2, err := cache.Get(ctx, t.TempDir())
	require.NoError(t, err)

	opt := &Options{Workdir: t.TempDir(), Resync: true}
	require.NoError(t, Bisync(ctx, fs1, fs2, opt))

	// Simulate an earlier, unrelated operation having recorded an error
	// against the same group before this run starts.
	_ = accounting.Stats(ctx).Error(errors.New("earlier unrelated error"))
	require.True(t, accounting.Stats(ctx).Errored())

	opt.Resync = false
	require.NoError(t, Bisync(ctx, fs1, fs2, opt))

	// A listing error during the run itself must still abort.
	require.NoError(t, os.RemoveAll(dir1))
	err = Bisync(ctx, fs1, fs2, opt)
	require.ErrorIs(t, err, ErrBisyncAborted)
}
