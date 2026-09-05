package squashfs

import (
	"context"
	"testing"

	_ "github.com/rclone/rclone/backend/local"
	fscache "github.com/rclone/rclone/fs/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A backslash is an ordinary character in a file name on the systems
// squashfs images are made on, so an entry containing one must be
// listed and openable like any other file.
func TestBackslashInName(t *testing.T) {
	ctx := context.Background()
	localFs, err := fscache.Get(ctx, "testdata")
	require.NoError(t, err)

	f, err := New(ctx, localFs, "backslash.sqfs", "", "")
	require.NoError(t, err)

	entries, err := f.List(ctx, "")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, `back\slash.txt`, entries[0].Remote())

	o, err := f.NewObject(ctx, `back\slash.txt`)
	require.NoError(t, err)
	assert.Equal(t, int64(6), o.Size())
}
