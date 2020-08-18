// Package cmount implements a FUSE mounting system for rclone remotes.
//
// This uses the cgo based cgofuse library

// +build cmount
// +build cgo
// +build linux darwin freebsd windows

package cmount

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/billziss-gh/cgofuse/fuse"
	"github.com/pkg/errors"
	"github.com/rclone/rclone/cmd/mountlib"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/vfs"
)

func init() {
	name := "cmount"
	if runtime.GOOS == "windows" {
		name = "mount"
	}
	mountlib.NewMountCommand(name, false, mount)
	mountlib.AddRc("cmount", mount)
}

// mountOptions configures the options from the command line flags
func mountOptions(VFS *vfs.VFS, device string, mountpoint string, opt *mountlib.Options) (options []string) {
	// Options
	options = []string{
		"-o", "fsname=" + device,
		"-o", "subtype=rclone",
		"-o", fmt.Sprintf("max_readahead=%d", opt.MaxReadAhead),
		"-o", fmt.Sprintf("attr_timeout=%g", opt.AttrTimeout.Seconds()),
		// This causes FUSE to supply O_TRUNC with the Open
		// call which is more efficient for cmount.  However
		// it does not work with cgofuse on Windows with
		// WinFSP so cmount must work with or without it.
		"-o", "atomic_o_trunc",
	}
	if opt.DebugFUSE {
		options = append(options, "-o", "debug")
	}

	// OSX options
	if runtime.GOOS == "darwin" {
		if opt.NoAppleDouble {
			options = append(options, "-o", "noappledouble")
		}
		if opt.NoAppleXattr {
			options = append(options, "-o", "noapplexattr")
		}
	}

	// determine if ExtraOptions already has an opt in
	hasOption := func(optionName string) bool {
		optionName += "="
		for _, option := range opt.ExtraOptions {
			if strings.HasPrefix(option, optionName) {
				return true
			}
		}
		return false
	}

	// Windows options
	if runtime.GOOS == "windows" {
		// These cause WinFsp to mean the current user
		if !hasOption("uid") {
			options = append(options, "-o", "uid=-1")
		}
		if !hasOption("gid") {
			options = append(options, "-o", "gid=-1")
		}
		options = append(options, "--FileSystemName=rclone")
	}

	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		if opt.VolumeName != "" {
			options = append(options, "-o", "volname="+opt.VolumeName)
		}
	}
	if opt.AllowNonEmpty {
		options = append(options, "-o", "nonempty")
	}
	if opt.AllowOther {
		options = append(options, "-o", "allow_other")
	}
	if opt.AllowRoot {
		options = append(options, "-o", "allow_root")
	}
	if opt.DefaultPermissions {
		options = append(options, "-o", "default_permissions")
	}
	if VFS.Opt.ReadOnly {
		options = append(options, "-o", "ro")
	}
	if opt.WritebackCache {
		// FIXME? options = append(options, "-o", WritebackCache())
	}
	if opt.DaemonTimeout != 0 {
		options = append(options, "-o", fmt.Sprintf("daemon_timeout=%d", int(opt.DaemonTimeout.Seconds())))
	}
	for _, option := range opt.ExtraOptions {
		options = append(options, "-o", option)
	}
	for _, option := range opt.ExtraFlags {
		options = append(options, option)
	}
	return options
}

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

// mount the file system
//
// The mount point will be ready when this returns.
//
// returns an error, and an error channel for the serve process to
// report an error when fusermount is called.
func mount(VFS *vfs.VFS, mountpoint string, opt *mountlib.Options) (<-chan error, func() error, error) {
	f := VFS.Fs()
	fs.Debugf(f, "Mounting on %q", mountpoint)

	// Check the mountpoint - in Windows the mountpoint mustn't exist before the mount
	if runtime.GOOS != "windows" {
		fi, err := os.Stat(mountpoint)
		if err != nil {
			return nil, nil, errors.Wrap(err, "mountpoint")
		}
		if !fi.IsDir() {
			return nil, nil, errors.New("mountpoint is not a directory")
		}
	}

	// Create underlying FS
	fsys := NewFS(VFS)
	host := fuse.NewFileSystemHost(fsys)
	host.SetCapReaddirPlus(true) // only works on Windows
	host.SetCapCaseInsensitive(f.Features().CaseInsensitive)

	// Create options
	options := mountOptions(VFS, f.Name()+":"+f.Root(), mountpoint, opt)
	fs.Debugf(f, "Mounting with options: %q", options)

	// Serve the mount point in the background returning error to errChan
	errChan := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				errChan <- errors.Errorf("mount failed: %v", r)
			}
		}()
		var err error
		ok := host.Mount(mountpoint, options)
		if !ok {
			err = errors.New("mount failed")
			fs.Errorf(f, "Mount failed")
		}
		errChan <- err
	}()

	// unmount
	unmount := func() error {
		// Shutdown the VFS
		fsys.VFS.Shutdown()
		fs.Debugf(nil, "Calling host.Unmount")
		if host.Unmount() {
			fs.Debugf(nil, "host.Unmount succeeded")
			if runtime.GOOS == "windows" {
				if !waitFor(func() bool {
					_, err := os.Stat(mountpoint)
					return err != nil
				}) {
					fs.Errorf(nil, "mountpoint %q didn't disappear after unmount - continuing anyway", mountpoint)
				}
			}
			return nil
		}
		fs.Debugf(nil, "host.Unmount failed")
		return errors.New("host unmount failed")
	}

	// Wait for the filesystem to become ready, checking the file
	// system didn't blow up before starting
	select {
	case err := <-errChan:
		err = errors.Wrap(err, "mount stopped before calling Init")
		return nil, nil, err
	case <-fsys.ready:
	}

	// Wait for the mount point to be available on Windows
	// On Windows the Init signal comes slightly before the mount is ready
	if runtime.GOOS == "windows" {
		if !waitFor(func() bool {
			_, err := os.Stat(mountpoint)
			return err == nil
		}) {
			fs.Errorf(nil, "mountpoint %q didn't became available on mount - continuing anyway", mountpoint)
		}
	}

	return errChan, unmount, nil
}
