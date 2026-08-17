package vfs

import (
	"context"
	"path"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// waitForInvalidations reads ch until every path in want has been seen,
// failing the test on timeout. Extra paths are ignored.
func waitForInvalidations(t *testing.T, ch <-chan string, want ...string) {
	need := map[string]bool{}
	for _, w := range want {
		need[w] = true
	}
	timeout := time.After(5 * time.Second)
	for len(need) > 0 {
		select {
		case p := <-ch:
			delete(need, p)
		case <-timeout:
			t.Fatalf("timed out waiting for invalidation of %v", need)
		}
	}
}

// set up a VFS with dir/file1 and dir/file2 looked up (cached) and a hook
// feeding invalidated paths into the returned channel.
func invalidateTestVFS(t *testing.T) (r *fstest.Run, vfs *VFS, ch chan string) {
	r, vfs = newTestVFS(t)
	ctx := context.Background()
	file1 := r.WriteObject(ctx, "dir/file1", "content one", t1)
	file2 := r.WriteObject(ctx, "dir/file2", "content two two", t1)
	r.CheckRemoteItems(t, file1, file2)

	// Look them up so they are cached
	for _, path := range []string{"dir", "dir/file1", "dir/file2"} {
		_, err := vfs.Stat(path)
		require.NoError(t, err)
	}

	ch = make(chan string, 64)
	vfs.AddInvalidateKernelCacheHook(func(parent Node, name string, node Node) {
		ch <- path.Join(parent.Path(), name)
	})
	return r, vfs, ch
}

// ForgetPath on a file invalidates that file's kernel entry
func TestInvalidateForgetPath(t *testing.T) {
	_, vfs, ch := invalidateTestVFS(t)
	root, err := vfs.Root()
	require.NoError(t, err)

	root.ForgetPath("dir/file1", fs.EntryObject)

	waitForInvalidations(t, ch, "dir/file1")
}

// ForgetPath on a directory invalidates the directory and everything
// cached under it
func TestInvalidateForgetDir(t *testing.T) {
	_, vfs, ch := invalidateTestVFS(t)
	root, err := vfs.Root()
	require.NoError(t, err)

	root.ForgetPath("dir", fs.EntryDirectory)

	waitForInvalidations(t, ch, "dir", "dir/file1", "dir/file2")
}

// vfs/forget with no arguments invalidates the whole looked-up tree
func TestInvalidateForgetAll(t *testing.T) {
	r, _, ch := invalidateTestVFS(t)

	call := rc.Calls.Get("vfs/forget")
	require.NotNil(t, call)
	_, err := call.Fn(context.Background(), rc.Params{"fs": fs.ConfigString(r.Fremote)})
	require.NoError(t, err)

	waitForInvalidations(t, ch, "dir", "dir/file1", "dir/file2")
}

// changeNotify invalidates the changed node
func TestInvalidateChangeNotify(t *testing.T) {
	_, vfs, ch := invalidateTestVFS(t)
	root, err := vfs.Root()
	require.NoError(t, err)

	root.changeNotify("dir/file1", fs.EntryObject)

	waitForInvalidations(t, ch, "dir/file1")
}

// changeNotify for a path which is not in the VFS cache still invalidates
// the kernel's entry (which may be a negative one) via the cached parent
func TestInvalidateChangeNotifyUncached(t *testing.T) {
	_, vfs, ch := invalidateTestVFS(t)
	root, err := vfs.Root()
	require.NoError(t, err)

	root.changeNotify("dir/new-file", fs.EntryObject)

	waitForInvalidations(t, ch, "dir/new-file")
}

// vfs/refresh invalidates the refreshed subtree
func TestInvalidateRefresh(t *testing.T) {
	r, _, ch := invalidateTestVFS(t)

	call := rc.Calls.Get("vfs/refresh")
	require.NotNil(t, call)
	out, err := call.Fn(context.Background(), rc.Params{"fs": fs.ConfigString(r.Fremote)})
	require.NoError(t, err)
	require.Equal(t, rc.Params{"result": map[string]string{"": "OK"}}, out)

	waitForInvalidations(t, ch, "dir/file1", "dir/file2")
}

// Two hooks both fire (a VFS can be shared by more than one mount), and
// unsubscribing removes only that hook.
func TestInvalidateMultipleHooks(t *testing.T) {
	_, vfs, ch1 := invalidateTestVFS(t)
	root, err := vfs.Root()
	require.NoError(t, err)

	ch2 := make(chan string, 64)
	remove2 := vfs.AddInvalidateKernelCacheHook(func(parent Node, name string, node Node) {
		ch2 <- path.Join(parent.Path(), name)
	})

	root.ForgetPath("dir/file1", fs.EntryObject)
	waitForInvalidations(t, ch1, "dir/file1")
	waitForInvalidations(t, ch2, "dir/file1")

	// Remove the second hook - it should stop receiving
	remove2()

	// Trigger two invalidations in sequence. Dispatches are serial, so by
	// the time ch1 has seen the second one, any erroneous ch2 delivery of
	// the first would have happened already.
	root.ForgetPath("dir/file2", fs.EntryObject)
	waitForInvalidations(t, ch1, "dir/file2")
	root.ForgetPath("dir/file1", fs.EntryObject)
	waitForInvalidations(t, ch1, "dir/file1")

	assert.Empty(t, ch2, "removed hook still fired")
}

// Unsubscribing waits for an in-flight hook to finish, so a mount can tear
// down safely.
func TestInvalidateUnsubscribeBarrier(t *testing.T) {
	r, vfs := newTestVFS(t)
	file1 := r.WriteObject(context.Background(), "dir/file1", "content one", t1)
	r.CheckRemoteItems(t, file1)
	_, err := vfs.Stat("dir/file1")
	require.NoError(t, err)
	root, err := vfs.Root()
	require.NoError(t, err)

	entered := make(chan struct{})
	release := make(chan struct{})
	remove := vfs.AddInvalidateKernelCacheHook(func(parent Node, name string, node Node) {
		close(entered)
		<-release
	})

	// Trigger one invalidation and wait until the hook is running and blocked.
	root.ForgetPath("dir/file1", fs.EntryObject)
	<-entered

	removed := make(chan struct{})
	go func() {
		remove()
		close(removed)
	}()

	// remove() must not return while the hook is still in-flight.
	select {
	case <-removed:
		t.Fatal("remove() returned while a hook was still running")
	case <-time.After(100 * time.Millisecond):
	}

	close(release)
	select {
	case <-removed:
	case <-time.After(2 * time.Second):
		t.Fatal("remove() did not return after the hook finished")
	}
}
