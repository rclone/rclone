// Internal tests for operations

package operations

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fstest/mockfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSizeDiffers(t *testing.T) {
	ctx := context.Background()
	ci := fs.GetConfig(ctx)
	when := time.Now()
	for _, test := range []struct {
		ignoreSize bool
		srcSize    int64
		dstSize    int64
		want       bool
	}{
		{false, 0, 0, false},
		{false, 1, 2, true},
		{false, 1, -1, false},
		{false, -1, 1, false},
		{true, 0, 0, false},
		{true, 1, 2, false},
		{true, 1, -1, false},
		{true, -1, 1, false},
	} {
		src := object.NewStaticObjectInfo("a", when, test.srcSize, true, nil, nil)
		dst := object.NewStaticObjectInfo("a", when, test.dstSize, true, nil, nil)
		oldIgnoreSize := ci.IgnoreSize
		ci.IgnoreSize = test.ignoreSize
		got := sizeDiffers(ctx, src, dst)
		ci.IgnoreSize = oldIgnoreSize
		assert.Equal(t, test.want, got, fmt.Sprintf("ignoreSize=%v, srcSize=%v, dstSize=%v", test.ignoreSize, test.srcSize, test.dstSize))
	}
}

func TestDirTransferEntry(t *testing.T) {
	ctx := context.Background()
	f, err := mockfs.NewFs(ctx, "mock", "", nil)
	require.NoError(t, err)
	dst := fs.NewDir("existing dir", time.Time{})
	root := fs.NewDir("", time.Time{})

	for _, test := range []struct {
		dst  fs.Directory
		dir  string
		want string
	}{
		{nil, "new dir", "new dir"},
		{nil, "", fmt.Sprint(f)},
		{dst, "ignored", "existing dir"},
		{root, "", fmt.Sprint(f)},
	} {
		got := dirTransferEntry(f, test.dst, test.dir)
		assert.Equal(t, test.want, got.Remote(), fmt.Sprintf("dst=%v, dir=%q", test.dst, test.dir))
	}
}
