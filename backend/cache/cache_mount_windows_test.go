// +build windows
// +build !race

package cache_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/billziss-gh/cgofuse/fuse"
	"github.com/pkg/errors"
	"github.com/rclone/rclone/cmd/cmount"
	"github.com/rclone/rclone/cmd/mountlib"
	"github.com/rclone/rclone/fs"
	"github.com/stretchr/testify/require"
)

// waitFor runs fn() until it returns true or the timeout expires
func waitFor(fn func() bool) (ok bool) {
	const totalWait = 10 * time.Second
	const individualWait = 10 * time.Millisecond
	for i := 0; i < int(totalWait/individualWait); i++ {
		ok = fn()
		if ok {
			return ok
		}
		time.Sleep(individualWait)
	}
	return false
}

func (r *run) mountFs(t *testing.T, f fs.Fs) {
	// FIXME implement cmount
	t.Skip("windows not supported yet")

	device := f.Name() + ":" + f.Root()
	options := []string{
		"-o", "fsname=" + device,
		"-o", "subtype=rclone",
		"-o", fmt.Sprintf("max_readahead=%d", mountlib.MaxReadAhead),
		"-o", "uid=-1",
		"-o", "gid=-1",
		"-o", "allow_other",
		// This causes FUSE to supply O_TRUNC with the Open
		// call which is more efficient for cmount.  However
		// it does not work with cgofuse on Windows with
		// WinFSP so cmount must work with or without it.
		"-o", "atomic_o_trunc",
		"--FileSystemName=rclone",
	}

	fsys := cmount.NewFS(f)
	host := fuse.NewFileSystemHost(fsys)

	// Serve the mount point in the background returning error to errChan
	r.unmountRes = make(chan error, 1)
	go func() {
		var err error
		ok := host.Mount(r.mntDir, options)
		if !ok {
			err = errors.New("mount failed")
		}
		r.unmountRes <- err
	}()

	// unmount
	r.unmountFn = func() error {
		// Shutdown the VFS
		fsys.VFS.Shutdown()
		if host.Unmount() {
			if !waitFor(func() bool {
				_, err := os.Stat(r.mntDir)
				return err != nil
			}) {
				t.Fatalf("mountpoint %q didn't disappear after unmount - continuing anyway", r.mntDir)
			}
			return nil
		}
		return errors.New("host unmount failed")
	}

	// Wait for the filesystem to become ready, checking the file
	// system didn't blow up before starting
	select {
	case err := <-r.unmountRes:
		require.NoError(t, err)
	case <-time.After(time.Second * 3):
	}

	// Wait for the mount point to be available on Windows
	// On Windows the Init signal comes slightly before the mount is ready
	if !waitFor(func() bool {
		_, err := os.Stat(r.mntDir)
		return err == nil
	}) {
		t.Errorf("mountpoint %q didn't became available on mount", r.mntDir)
	}

	r.vfs = fsys.VFS
	r.isMounted = true
}

func (r *run) unmountFs(t *testing.T, f fs.Fs) {
	// FIXME implement cmount
	t.Skip("windows not supported yet")
	var err error

	for i := 0; i < 4; i++ {
		err = r.unmountFn()
		if err != nil {
			//log.Printf("signal to umount failed - retrying: %v", err)
			time.Sleep(3 * time.Second)
			continue
		}
		break
	}
	require.NoError(t, err)
	err = <-r.unmountRes
	require.NoError(t, err)
	err = r.vfs.CleanUp()
	require.NoError(t, err)
	r.isMounted = false
}
