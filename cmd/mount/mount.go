// Package mount implements a FUSE mounting system for rclone remotes.

// +build linux,go1.13 darwin,go1.13 freebsd,go1.13

package mount

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"bazil.org/fuse"
	fusefs "bazil.org/fuse/fs"
	"github.com/okzk/sdnotify"
	"github.com/pkg/errors"
	"github.com/rclone/rclone/cmd/mountlib"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/atexit"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfsflags"
)

func init() {
	mountlib.NewMountCommand("mount", false, Mount)
	// Add mount to rc
	mountlib.AddRc("mount", mount)
}

// mountOptions configures the options from the command line flags
func mountOptions(device string) (options []fuse.MountOption) {
	options = []fuse.MountOption{
		fuse.MaxReadahead(uint32(mountlib.MaxReadAhead)),
		fuse.Subtype("rclone"),
		fuse.FSName(device),
		fuse.VolumeName(mountlib.VolumeName),

		// Options from benchmarking in the fuse module
		//fuse.MaxReadahead(64 * 1024 * 1024),
		//fuse.WritebackCache(),
	}
	if mountlib.AsyncRead {
		options = append(options, fuse.AsyncRead())
	}
	if mountlib.NoAppleDouble {
		options = append(options, fuse.NoAppleDouble())
	}
	if mountlib.NoAppleXattr {
		options = append(options, fuse.NoAppleXattr())
	}
	if mountlib.AllowNonEmpty {
		options = append(options, fuse.AllowNonEmptyMount())
	}
	if mountlib.AllowOther {
		options = append(options, fuse.AllowOther())
	}
	if mountlib.AllowRoot {
		// options = append(options, fuse.AllowRoot())
		fs.Errorf(nil, "Ignoring --allow-root. Support has been removed upstream - see https://github.com/bazil/fuse/issues/144 for more info")
	}
	if mountlib.DefaultPermissions {
		options = append(options, fuse.DefaultPermissions())
	}
	if vfsflags.Opt.ReadOnly {
		options = append(options, fuse.ReadOnly())
	}
	if mountlib.WritebackCache {
		options = append(options, fuse.WritebackCache())
	}
	if mountlib.DaemonTimeout != 0 {
		options = append(options, fuse.DaemonTimeout(fmt.Sprint(int(mountlib.DaemonTimeout.Seconds()))))
	}
	if len(mountlib.ExtraOptions) > 0 {
		fs.Errorf(nil, "-o/--option not supported with this FUSE backend")
	}
	if len(mountlib.ExtraFlags) > 0 {
		fs.Errorf(nil, "--fuse-flag not supported with this FUSE backend")
	}
	return options
}

// mount the file system
//
// The mount point will be ready when this returns.
//
// returns an error, and an error channel for the serve process to
// report an error when fusermount is called.
func mount(f fs.Fs, mountpoint string) (*vfs.VFS, <-chan error, func() error, error) {
	fs.Debugf(f, "Mounting on %q", mountpoint)
	c, err := fuse.Mount(mountpoint, mountOptions(f.Name()+":"+f.Root())...)
	if err != nil {
		return nil, nil, nil, err
	}

	filesys := NewFS(f)
	server := fusefs.New(c, nil)

	// Serve the mount point in the background returning error to errChan
	errChan := make(chan error, 1)
	go func() {
		err := server.Serve(filesys)
		closeErr := c.Close()
		if err == nil {
			err = closeErr
		}
		errChan <- err
	}()

	// check if the mount process has an error to report
	<-c.Ready
	if err := c.MountError; err != nil {
		return nil, nil, nil, err
	}

	unmount := func() error {
		// Shutdown the VFS
		filesys.VFS.Shutdown()
		return fuse.Unmount(mountpoint)
	}

	return filesys.VFS, errChan, unmount, nil
}

// Mount mounts the remote at mountpoint.
//
// If noModTime is set then it
func Mount(f fs.Fs, mountpoint string) error {
	if mountlib.DebugFUSE {
		fuse.Debug = func(msg interface{}) {
			fs.Debugf("fuse", "%v", msg)
		}
	}

	// Mount it
	FS, errChan, unmount, err := mount(f, mountpoint)
	if err != nil {
		return errors.Wrap(err, "failed to mount FUSE fs")
	}

	sigInt := make(chan os.Signal, 1)
	signal.Notify(sigInt, syscall.SIGINT, syscall.SIGTERM)
	sigHup := make(chan os.Signal, 1)
	signal.Notify(sigHup, syscall.SIGHUP)
	atexit.IgnoreSignals()
	atexit.Register(func() {
		_ = unmount()
	})

	if err := sdnotify.Ready(); err != nil && err != sdnotify.ErrSdNotifyNoSocket {
		return errors.Wrap(err, "failed to notify systemd")
	}

waitloop:
	for {
		select {
		// umount triggered outside the app
		case err = <-errChan:
			break waitloop
		// Program abort: umount
		case <-sigInt:
			err = unmount()
			break waitloop
		// user sent SIGHUP to clear the cache
		case <-sigHup:
			root, err := FS.Root()
			if err != nil {
				fs.Errorf(f, "Error reading root: %v", err)
			} else {
				root.ForgetAll()
			}
		}
	}

	_ = sdnotify.Stopping()
	if err != nil {
		return errors.Wrap(err, "failed to umount FUSE fs")
	}

	return nil
}
