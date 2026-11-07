// Internal tests for operations

package operations

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
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

// objectWithID is a test object with an ID
type objectWithID struct {
	fs.Object
	id string
}

// ID returns the ID of the object
func (o objectWithID) ID() string {
	return o.id
}

func TestDedupeDeleteIdenticalKeepsObjectsWithoutID(t *testing.T) {
	ctx := context.Background()
	when := time.Now()
	isOne := []byte("This is one")

	// Objects which don't implement fs.IDer must still be considered
	// for deduplication. Dropping them makes dedupe silently do
	// nothing on duplicate-capable backends which have no IDs.
	noID := []fs.Object{
		object.NewMemoryObject("one", when, isOne),
		object.NewMemoryObject("one", when, isOne),
	}
	assert.Len(t, dedupeDeleteIdentical(ctx, hash.None, "one", noID), 2)

	// Objects with the same ID are the same file, so must still be
	// ignored as deleting them would lose data.
	sameID := []fs.Object{
		objectWithID{Object: object.NewMemoryObject("one", when, isOne), id: "same"},
		objectWithID{Object: object.NewMemoryObject("one", when, isOne), id: "same"},
	}
	assert.Len(t, dedupeDeleteIdentical(ctx, hash.None, "one", sameID), 0)
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
