// Package mount implements a FUSE mounting system for rclone remotes.

// +build linux,go1.13 darwin,go1.13 freebsd,go1.13

package mount

import (
	"fmt"

	"bazil.org/fuse"
	fusefs "bazil.org/fuse/fs"
	"github.com/rclone/rclone/cmd/mountlib"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/vfs"
)

func init() {
	mountlib.NewMountCommand("mount", false, mount)
	mountlib.AddRc("mount", mount)
}

// mountOptions configures the options from the command line flags
func mountOptions(VFS *vfs.VFS, device string) (options []fuse.MountOption) {
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
	if VFS.Opt.ReadOnly {
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
func mount(VFS *vfs.VFS, mountpoint string) (<-chan error, func() error, error) {
	if mountlib.DebugFUSE {
		fuse.Debug = func(msg interface{}) {
			fs.Debugf("fuse", "%v", msg)
		}
	}

	f := VFS.Fs()
	fs.Debugf(f, "Mounting on %q", mountpoint)
	c, err := fuse.Mount(mountpoint, mountOptions(VFS, f.Name()+":"+f.Root())...)
	if err != nil {
		return nil, nil, err
	}

	filesys := NewFS(VFS)
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
		return nil, nil, err
	}

	unmount := func() error {
		// Shutdown the VFS
		filesys.VFS.Shutdown()
		return fuse.Unmount(mountpoint)
	}

	return errChan, unmount, nil
}
