package sync

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A reused stats group must not disable empty-directory cleanup for a clean
// move after an unrelated operation has failed.
func TestMoveDeleteEmptySrcDirsAfterUnrelatedGroupError(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	ctx := context.Background()
	t1 := time.Now()

	r.Mkdir(context.Background(), r.Flocal)
	r.WriteFile("seed/file.txt", "seed", t1)
	seedObj, err := r.Flocal.NewObject(ctx, "seed/file.txt")
	require.NoError(t, err)

	// Record an error on the shared group's StatsInfo, as an earlier failed
	// operation would do.
	badCtx := accounting.WithStatsGroup(ctx, "g")
	tr := accounting.Stats(badCtx).NewTransfer(seedObj, r.Fremote)
	tr.Done(badCtx, errors.New("file does not exist"))
	assert.True(t, accounting.Stats(badCtx).Errored())

	// The clean move uses the same group but has no errors of its own.
	goodCtx := accounting.WithStatsGroup(ctx, "g")
	file1 := r.WriteFile("nested/dir/file.txt", "content", t1)
	r.Mkdir(context.Background(), r.Fremote)
	fstest.CheckItems(t, r.Flocal, file1, fstest.NewItem("seed/file.txt", "seed", t1))

	require.NoError(t, moveDir(goodCtx, r.Fremote, r.Flocal, true, false))
	fstest.CheckItems(t, r.Fremote, file1, fstest.NewItem("seed/file.txt", "seed", t1))
	// The emptied directory is removed even though the shared group retains
	// the earlier error.
	assert.NoDirExists(t, r.Flocal.Root()+"/nested/dir")
}

// Control: the same move on a FRESH group deletes the emptied dir.
func TestMoveDeleteEmptySrcDirsCleanGroup(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	ctx := context.Background()
	t1 := time.Now()

	r.Mkdir(context.Background(), r.Flocal)
	file1 := r.WriteFile("nested/dir/file.txt", "content", t1)
	r.Mkdir(context.Background(), r.Fremote)
	fstest.CheckItems(t, r.Flocal, file1)

	require.NoError(t, moveDir(ctx, r.Fremote, r.Flocal, true, false))
	fstest.CheckItems(t, r.Fremote, file1)
	assert.NoDirExists(t, r.Flocal.Root()+"/nested/dir")
}
