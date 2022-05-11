// Package cmount implements a FUSE mounting system for rclone remotes.
//
// This uses the cgo based cgofuse library

//go:build cmount && ((linux && cgo) || (darwin && cgo) || (freebsd && cgo) || windows)
// +build cmount
// +build linux,cgo darwin,cgo freebsd,cgo windows

package cmount

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/rclone/rclone/cmd/mountlib"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/atexit"
	"github.com/rclone/rclone/lib/buildinfo"
	"github.com/rclone/rclone/vfs"
	"github.com/winfsp/cgofuse/fuse"
)

func init() {
	name := "cmount"
	cmountOnly := ProvidedBy(runtime.GOOS)
	if cmountOnly {
		name = "mount"
	}
	cmd := mountlib.NewMountCommand(name, false, mount)
	if cmountOnly {
		cmd.Aliases = append(cmd.Aliases, "cmount")
	}
	mountlib.AddRc("cmount", mount)
	buildinfo.Tags = append(buildinfo.Tags, "cmount")
}

// Find the option string in the current options
func findOption(name string, options []string) (found bool) {
	for _, option := range options {
		if option == "-o" {
			continue
		}
		if strings.Contains(option, name) {
			return true
		}
	}
	return false
}

// mountOptions configures the options from the command line flags
func mountOptions(VFS *vfs.VFS, device string, mountpoint string, opt *mountlib.Options) (options []string) {
	// Options
	options = []string{
		"-o", fmt.Sprintf("attr_timeout=%g", opt.AttrTimeout.Seconds()),
	}
	if opt.DebugFUSE {
		options = append(options, "-o", "debug")
	}

	if runtime.GOOS == "windows" {
		options = append(options, "-o", "uid=-1")
		options = append(options, "-o", "gid=-1")
		options = append(options, "--FileSystemName=rclone")
		if opt.VolumeName != "" {
			if opt.NetworkMode {
				options = append(options, "--VolumePrefix="+opt.VolumeName)
			} else {
				options = append(options, "-o", "volname="+opt.VolumeName)
			}
		}
	} else {
		options = append(options, "-o", "fsname="+device)
		options = append(options, "-o", "subtype=rclone")
		options = append(options, "-o", fmt.Sprintf("max_readahead=%d", opt.MaxReadAhead))
		// This causes FUSE to supply O_TRUNC with the Open
		// call which is more efficient for cmount.  However
		// it does not work with cgofuse on Windows with
		// WinFSP so cmount must work with or without it.
		options = append(options, "-o", "atomic_o_trunc")
		if opt.DaemonTimeout != 0 {
			options = append(options, "-o", fmt.Sprintf("daemon_timeout=%d", int(opt.DaemonTimeout.Seconds())))
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
		if runtime.GOOS == "darwin" {
			if opt.VolumeName != "" {
				options = append(options, "-o", "volname="+opt.VolumeName)
			}
			if opt.NoAppleDouble {
				options = append(options, "-o", "noappledouble")
			}
			if opt.NoAppleXattr {
				options = append(options, "-o", "noapplexattr")
			}
		}
	}
	for _, option := range opt.ExtraOptions {
		options = append(options, "-o", option)
	}
	for _, option := range opt.ExtraFlags {
		options = append(options, option)
	}
	if runtime.GOOS == "darwin" {
		if !findOption("modules=iconv", options) {
			iconv := "modules=iconv,from_code=UTF-8,to_code=UTF-8-MAC"
			options = append(options, "-o", iconv)
			fs.Debugf(nil, "Adding \"-o %s\" for macOS", iconv)
		}
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
func mount(VFS *vfs.VFS, mountPath string, opt *mountlib.Options) (<-chan error, func() error, error) {
	// Get mountpoint using OS specific logic
	mountpoint, err := getMountpoint(mountPath, opt)
	if err != nil {
		return nil, nil, err
	}
	fs.Debugf(nil, "Mounting on %q (%q)", mountpoint, opt.VolumeName)

	// Create underlying FS
	f := VFS.Fs()
	fsys := NewFS(VFS)
	host := fuse.NewFileSystemHost(fsys)
	host.SetCapReaddirPlus(true) // only works on Windows
	host.SetCapCaseInsensitive(f.Features().CaseInsensitive)

	// Create options
	options := mountOptions(VFS, opt.DeviceName, mountpoint, opt)
	fs.Debugf(f, "Mounting with options: %q", options)

	// Serve the mount point in the background returning error to errChan
	errChan := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				errChan <- fmt.Errorf("mount failed: %v", r)
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
		var umountOK bool
		if atomic.LoadInt32(&fsys.destroyed) != 0 {
			fs.Debugf(nil, "Not calling host.Unmount as mount already Destroyed")
			umountOK = true
		} else if atexit.Signalled() {
			// If we have received a signal then FUSE will be shutting down already
			fs.Debugf(nil, "Not calling host.Unmount as signal received")
			umountOK = true
		} else {
			fs.Debugf(nil, "Calling host.Unmount")
			umountOK = host.Unmount()
		}
		if umountOK {
			fs.Debugf(nil, "Unmounted successfully")
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
		err = fmt.Errorf("mount stopped before calling Init: %w", err)
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
