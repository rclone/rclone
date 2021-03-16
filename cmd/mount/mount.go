// Package mount implements a FUSE mounting system for rclone remotes.

// +build linux freebsd

package mount

import (
	"fmt"
	"runtime"

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
func mountOptions(VFS *vfs.VFS, device string, opt *mountlib.Options) (options []fuse.MountOption) {
	options = []fuse.MountOption{
		fuse.MaxReadahead(uint32(opt.MaxReadAhead)),
		fuse.Subtype("rclone"),
		fuse.FSName(device),
		fuse.VolumeName(opt.VolumeName),

		// Options from benchmarking in the fuse module
		//fuse.MaxReadahead(64 * 1024 * 1024),
		//fuse.WritebackCache(),
	}
	if opt.AsyncRead {
		options = append(options, fuse.AsyncRead())
	}
	if opt.AllowNonEmpty {
		options = append(options, fuse.AllowNonEmptyMount())
	}
	if opt.AllowOther {
		options = append(options, fuse.AllowOther())
	}
	if opt.AllowRoot {
		// options = append(options, fuse.AllowRoot())
		fs.Errorf(nil, "Ignoring --allow-root. Support has been removed upstream - see https://github.com/bazil/fuse/issues/144 for more info")
	}
	if opt.DefaultPermissions {
		options = append(options, fuse.DefaultPermissions())
	}
	if VFS.Opt.ReadOnly {
		options = append(options, fuse.ReadOnly())
	}
	if opt.WritebackCache {
		options = append(options, fuse.WritebackCache())
	}
	if opt.DaemonTimeout != 0 {
		options = append(options, fuse.DaemonTimeout(fmt.Sprint(int(opt.DaemonTimeout.Seconds()))))
	}
	if len(opt.ExtraOptions) > 0 {
		fs.Errorf(nil, "-o/--option not supported with this FUSE backend")
	}
	if len(opt.ExtraFlags) > 0 {
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
func mount(VFS *vfs.VFS, mountpoint string, opt *mountlib.Options) (<-chan error, func() error, error) {
	if runtime.GOOS == "darwin" {
		fs.Logf(nil, "macOS users: please try \"rclone cmount\" as it will be the default in v1.54")
	}

	if opt.DebugFUSE {
		fuse.Debug = func(msg interface{}) {
			fs.Debugf("fuse", "%v", msg)
		}
	}

	f := VFS.Fs()
	fs.Debugf(f, "Mounting on %q", mountpoint)
	c, err := fuse.Mount(mountpoint, mountOptions(VFS, f.Name()+":"+f.Root(), opt)...)
	if err != nil {
		return nil, nil, err
	}

	filesys := NewFS(VFS, opt)
	filesys.server = fusefs.New(c, nil)

	// Serve the mount point in the background returning error to errChan
	errChan := make(chan error, 1)
	go func() {
		err := filesys.server.Serve(filesys)
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
