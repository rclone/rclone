package accounting

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/fstest/mockfs"
	"github.com/rclone/rclone/fstest/mockobject"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransfer(t *testing.T) {
	ctx := context.Background()
	s := NewStats(ctx)

	o := mockobject.Object("obj")
	srcFs, err := mockfs.NewFs(ctx, "srcFs", "srcFs", nil)
	require.NoError(t, err)
	dstFs, err := mockfs.NewFs(ctx, "dstFs", "dstFs", nil)
	require.NoError(t, err)

	tr := newTransfer(s, o, srcFs, dstFs)

	t.Run("Snapshot", func(t *testing.T) {
		snap := tr.Snapshot()
		assert.Equal(t, "obj", snap.Name)
		assert.Equal(t, int64(0), snap.Size)
		assert.Equal(t, int64(0), snap.Bytes)
		assert.Equal(t, false, snap.Checked)
		assert.Equal(t, false, snap.StartedAt.IsZero())
		assert.Equal(t, true, snap.CompletedAt.IsZero())
		assert.Equal(t, nil, snap.Error)
		assert.Equal(t, "", snap.Group)
		assert.Equal(t, "srcFs:srcFs", snap.SrcFs)
		assert.Equal(t, "dstFs:dstFs", snap.DstFs)
	})

	t.Run("Done", func(t *testing.T) {
		tr.Done(ctx, io.EOF)
		snap := tr.Snapshot()
		assert.Equal(t, "obj", snap.Name)
		assert.Equal(t, int64(0), snap.Size)
		assert.Equal(t, int64(0), snap.Bytes)
		assert.Equal(t, false, snap.Checked)
		assert.Equal(t, false, snap.StartedAt.IsZero())
		assert.Equal(t, false, snap.CompletedAt.IsZero())
		assert.Equal(t, true, errors.Is(snap.Error, io.EOF))
		assert.Equal(t, "", snap.Group)
		assert.Equal(t, "srcFs:srcFs", snap.SrcFs)
		assert.Equal(t, "dstFs:dstFs", snap.DstFs)
	})

	t.Run("rcStats", func(t *testing.T) {
		out := tr.rcStats()
		assert.Equal(t, rc.Params{
			"name":  "obj",
			"size":  int64(0),
			"srcFs": "srcFs:srcFs",
			"dstFs": "dstFs:dstFs",
		}, out)
	})
}
