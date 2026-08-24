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

// Repro for rclone#9634: a stats group reused across rc calls keeps errors
// from an earlier unrelated operation, which permanently disables
// --delete-empty-src-dirs for later clean moves in the same group.
func TestMoveDeleteEmptySrcDirsAfterUnrelatedGroupError(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	ctx := context.Background()
	t1 := time.Now()

	r.Mkdir(context.Background(), r.Flocal)
	r.WriteFile("seed/file.txt", "seed", t1)
	seedObj, err := r.Flocal.NewObject(ctx, "seed/file.txt")
	require.NoError(t, err)

	// Simulate an earlier failed operation in the same _group (e.g. an
	// operations/copyfile of a nonexistent file). Transfer.Done() counts the
	// error on the shared group's StatsInfo; nothing in a later clean run
	// clears it.
	badCtx := accounting.WithStatsGroup(ctx, "g")
	tr := accounting.Stats(badCtx).NewTransfer(seedObj, r.Fremote)
	tr.Done(badCtx, errors.New("file does not exist"))
	assert.True(t, accounting.Stats(badCtx).Errored())

	// A completely clean move in the SAME group must still delete the
	// now-empty source dirs, but deleteEmptyDirectories() checks
	// accounting.Stats(ctx).Errored() on the shared group stats and refuses.
	goodCtx := accounting.WithStatsGroup(ctx, "g")
	file1 := r.WriteFile("nested/dir/file.txt", "content", t1)
	r.Mkdir(context.Background(), r.Fremote)
	fstest.CheckItems(t, r.Flocal, file1, fstest.NewItem("seed/file.txt", "seed", t1))

	require.NoError(t, moveDir(goodCtx, r.Fremote, r.Flocal, true, false))
	fstest.CheckItems(t, r.Fremote, file1, fstest.NewItem("seed/file.txt", "seed", t1))
	// nested/dir was emptied by the move; a clean group would have removed it.
	assert.DirExists(t, r.Flocal.Root()+"/nested/dir")
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
