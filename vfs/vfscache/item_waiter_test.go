package vfscache

import (
	"os"
	"testing"
	"time"

	"github.com/rclone/rclone/lib/ranges"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestItemWaiterBeyondItemSize checks that a reader waiting for a range past
// the item's size, but within the size of the source object the downloaders
// hold, is still released.
//
// Download is called directly because _ensure clips the range against
// info.Size before it can reach the waiter queue. In the field the waiter is
// queued while info.Size is still full and info.Size shrinks afterwards.
func TestItemWaiterBeyondItemSize(t *testing.T) {
	r, c := newItemTestCache(t)

	const (
		remote     = "waiter.bin"
		fileSize   = 64 * 1024 * 1024
		headLen    = 64 * 1024
		shrunkSize = 1024 * 1024
		tailOffset = 48 * 1024 * 1024
	)

	_, obj, item := newFileLength(t, r, c, remote, fileSize)
	require.NoError(t, item.Open(obj))
	defer func() { _ = item.Close(nil) }()

	buf := make([]byte, headLen)
	n, err := item.ReadAt(buf, 0)
	require.NoError(t, err)
	require.Equal(t, headLen, n)

	osPath := c.toOSPath(remote)
	apparent := func() int64 {
		fi, statErr := os.Stat(osPath)
		require.NoError(t, statErr)
		return fi.Size()
	}
	require.Equal(t, int64(fileSize), obj.Size())
	require.Equal(t, int64(fileSize), apparent())

	head := ranges.Range{Pos: 0, Size: headLen}
	tail := ranges.Range{Pos: tailOffset, Size: 4096}
	require.True(t, item.info.Rs.Present(head))
	require.False(t, item.info.Rs.Present(tail), "tail must not be prefetched")

	// Shrink info.Size below the tail offset, as readAt does when the
	// remote object and the item disagree about the size.
	item.mu.Lock()
	require.NoError(t, item._truncate(shrunkSize))
	item.mu.Unlock()

	require.Equal(t, int64(shrunkSize), item.info.Size)
	require.Equal(t, int64(shrunkSize), apparent())

	done := make(chan error, 1)
	go func() { done <- item.downloaders.Download(tail) }()

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(30 * time.Second):
		t.Fatal("Download did not return: the waiter was never dispatched")
	}
}
