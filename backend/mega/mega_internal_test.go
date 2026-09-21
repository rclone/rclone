package mega

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// InternalTestGhostAfterRemove checks that a file removed in a long-running
// process disappears from listings straight away rather than lingering as a
// ghost until go-mega's next event poll reconciles the in-memory tree.
//
// It puts, removes and lists in a tight loop to make any lingering entry
// show up reliably, exercising whichever delete mode the remote is
// configured with (hard_delete or the default trash).
func (f *Fs) InternalTestGhostAfterRemove(t *testing.T) {
	ctx := context.Background()

	dir := "ghost-test"
	require.NoError(t, f.Mkdir(ctx, dir))
	defer func() { _ = f.Rmdir(ctx, dir) }()

	contents := []byte("ghost test contents")
	for i := range 10 {
		remote := dir + "/test.txt"
		src := object.NewStaticObjectInfo(remote, time.Now(), int64(len(contents)), true, nil, nil)
		obj, err := f.Put(ctx, bytes.NewReader(contents), src)
		require.NoError(t, err)
		require.NoError(t, obj.Remove(ctx))

		// The file must be gone from the listing straight away - if the
		// in-memory tree isn't settled it lingers as a ghost.
		entries, err := f.List(ctx, dir)
		require.NoError(t, err)
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Remote())
		}
		assert.NotContains(t, names, remote, "file should be gone immediately after remove (iteration %d)", i)
	}
}

// InternalTest dispatches the backend specific internal tests
func (f *Fs) InternalTest(t *testing.T) {
	t.Run("GhostAfterRemove", f.InternalTestGhostAfterRemove)
}

var _ fstests.InternalTester = (*Fs)(nil)
