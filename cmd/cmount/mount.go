// Package cmount implents a FUSE mounting system for rclone remotes.
//
// This uses the cgo based cgofuse library

// +build cmount
// +build cgo
// +build linux darwin freebsd windows

package cmount

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/billziss-gh/cgofuse/fuse"
	"github.com/ncw/rclone/cmd/mountlib"
	"github.com/ncw/rclone/fs"
	"github.com/pkg/errors"
)

func init() {
	name := "cmount"
	if runtime.GOOS == "windows" {
		name = "mount"
	}
	mountlib.NewMountCommand(name, Mount)
}

// mountOptions configures the options from the command line flags
func mountOptions(device string, mountpoint string) (options []string) {
	// Options
	options = []string{
		"-o", "fsname=" + device,
		"-o", "subtype=rclone",
		"-o", fmt.Sprintf("max_readahead=%d", mountlib.MaxReadAhead),
	}
	if mountlib.DebugFUSE {
		options = append(options, "-o", "debug")
	}

	// OSX options
	if runtime.GOOS == "darwin" {
		options = append(options, "-o", "volname="+device)
		options = append(options, "-o", "noappledouble")
		options = append(options, "-o", "noapplexattr")
	}

	// Windows options
	if runtime.GOOS == "windows" {
		// These cause WinFsp to mean the current user
		options = append(options, "-o", "uid=-1")
		options = append(options, "-o", "gid=-1")
		options = append(options, "--FileSystemName=rclone")
	}

	if mountlib.AllowNonEmpty {
		options = append(options, "-o", "nonempty")
	}
	if mountlib.AllowOther {
		options = append(options, "-o", "allow_other")
	}
	if mountlib.AllowRoot {
		options = append(options, "-o", "allow_root")
	}
	if mountlib.DefaultPermissions {
		options = append(options, "-o", "default_permissions")
	}
	if mountlib.ReadOnly {
		options = append(options, "-o", "ro")
	}
	if mountlib.WritebackCache {
		// FIXME? options = append(options, "-o", WritebackCache())
	}
	for _, option := range *mountlib.ExtraOptions {
		options = append(options, "-o", option)
	}
	for _, option := range *mountlib.ExtraFlags {
		options = append(options, option)
	}
	return options
}

// mount the file system
//
// The mount point will be ready when this returns.
//
// returns an error, and an error channel for the serve process to
// report an error when fusermount is called.
func mount(f fs.Fs, mountpoint string) (*mountlib.FS, <-chan error, func() error, error) {
	fs.Debugf(f, "Mounting on %q", mountpoint)

	// Check the mountpoint - in Windows the mountpoint musn't exist before the mount
	if runtime.GOOS != "windows" {
		fi, err := os.Stat(mountpoint)
		if err != nil {
			return nil, nil, nil, errors.Wrap(err, "mountpoint")
		}
		if !fi.IsDir() {
			return nil, nil, nil, errors.New("mountpoint is not a directory")
		}
	}

	// Create underlying FS
	fsys := NewFS(f)
	host := fuse.NewFileSystemHost(fsys)

	// Create options
	options := mountOptions(f.Name()+":"+f.Root(), mountpoint)
	fs.Debugf(f, "Mounting with options: %q", options)

	// Serve the mount point in the background returning error to errChan
	errChan := make(chan error, 1)
	go func() {
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
		fs.Debugf(nil, "Calling host.Unmount")
		if host.Unmount() {
			fs.Debugf(nil, "host.Unmount succeeded")
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
		return nil, nil, nil, err
	case <-fsys.ready:
	}

	// Wait for the mount point to be available on Windows
	// On Windows the Init signal comes slightly before the mount is ready
	if runtime.GOOS == "windows" {
		const totalWait = 10 * time.Second
		const individualWait = 10 * time.Millisecond
		for i := 0; i < int(totalWait/individualWait); i++ {
			_, err := os.Stat(mountpoint)
			if err == nil {
				goto found
			}
			time.Sleep(10 * time.Millisecond)
		}
		fs.Errorf(nil, "mountpoint %q didn't became available after %v - continuing anyway", mountpoint, totalWait)
	found:
	}

	return fsys.FS, errChan, unmount, nil
}

// Mount mounts the remote at mountpoint.
//
// If noModTime is set then it
func Mount(f fs.Fs, mountpoint string) error {
	// Mount it
	FS, errChan, _, err := mount(f, mountpoint)
	if err != nil {
		return errors.Wrap(err, "failed to mount FUSE fs")
	}

	// Note cgofuse unmounts the fs on SIGINT etc

	sigHup := make(chan os.Signal, 1)
	signal.Notify(sigHup, syscall.SIGHUP)

waitloop:
	for {
		select {
		// umount triggered outside the app
		case err = <-errChan:
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

	if err != nil {
		return errors.Wrap(err, "failed to umount FUSE fs")
	}

	return nil
}
