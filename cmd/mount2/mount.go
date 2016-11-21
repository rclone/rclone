// Package mount implents a FUSE mounting system for rclone remotes.

// +build linux darwin freebsd

package mount2

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
	"github.com/ncw/rclone/cmd/mountlib"
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/vfs"
	"github.com/okzk/sdnotify"
	"github.com/pkg/errors"
)

const (
	noisyDebug = false // set this to true for lots of noisy debug
)

func init() {
	mountlib.NewMountCommand("mount2", Mount)
}

// mountOptions configures the options from the command line flags
//
// man mount.fuse for more info and note the -o flag for other options
func mountOptions(fsys *FS, f fs.Fs) (mountOpts *fuse.MountOptions) {
	device := f.Name() + ":" + f.Root()
	mountOpts = &fuse.MountOptions{
		AllowOther:    mountlib.AllowOther,
		FsName:        device,
		Name:          "rclone",
		DisableXAttrs: true,
		Debug:         mountlib.DebugFUSE,
		MaxReadAhead:  int(mountlib.MaxReadAhead),
	}
	var opts []string
	// FIXME doesn't work opts = append(opts, fmt.Sprintf("max_readahead=%d", maxReadAhead))
	if mountlib.AllowNonEmpty {
		opts = append(opts, "nonempty")
	}
	if mountlib.AllowOther {
		opts = append(opts, "allow_other")
	}
	if mountlib.AllowRoot {
		opts = append(opts, "allow_root")
	}
	if mountlib.DefaultPermissions {
		opts = append(opts, "default_permissions")
	}
	if fsys.VFS.Opt.ReadOnly {
		opts = append(opts, "ro")
	}
	if mountlib.WritebackCache {
		log.Printf("FIXME --write-back-cache not supported")
		// FIXME opts = append(opts,fuse.WritebackCache())
	}
	// Some OS X only options
	if runtime.GOOS == "darwin" {
		opts = append(opts,
			// VolumeName sets the volume name shown in Finder.
			fmt.Sprintf("volname=%s", device),

			// NoAppleXattr makes OSXFUSE disallow extended attributes with the
			// prefix "com.apple.". This disables persistent Finder state and
			// other such information.
			"noapplexattr",

			// NoAppleDouble makes OSXFUSE disallow files with names used by OS X
			// to store extended attributes on file systems that do not support
			// them natively.
			//
			// Such file names are:
			//
			//     ._*
			//     .DS_Store
			"noappledouble",
		)
	}
	mountOpts.Options = opts
	return mountOpts
}

// mount the file system
//
// The mount point will be ready when this returns.
//
// returns an error, and an error channel for the serve process to
// report an error when fusermount is called.
func mount(f fs.Fs, mountpoint string) (*vfs.VFS, <-chan error, func() error, error) {
	fs.Debugf(f, "Mounting on %q", mountpoint)

	mountlib.DebugFUSE = true // FIXME

	fsys := NewFS(f)
	nodeFsOpts := &pathfs.PathNodeFsOptions{
		ClientInodes: false,
		Debug:        mountlib.DebugFUSE,
	}
	nodeFs := pathfs.NewPathNodeFs(fsys, nodeFsOpts)

	mOpts := nodefs.NewOptions() // default options
	// FIXME
	// mOpts.EntryTimeout = 10 * time.Second
	// mOpts.AttrTimeout = 10 * time.Second
	// mOpts.NegativeTimeout = 10 * time.Second
	mOpts.Debug = mountlib.DebugFUSE

	conn := nodefs.NewFileSystemConnector(nodeFs.Root(), mOpts)
	mountOpts := mountOptions(fsys, f)
	//mountOpts := &fuse.MountOptions{}
	server, err := fuse.NewServer(conn.RawFS(), mountpoint, mountOpts)

	//server, _, err := nodefs.MountRoot(mountpoint, nodeFs.Root(), mOpts)
	if err != nil {
		return nil, nil, nil, err
	}

	umount := server.Unmount

	// serverSettings := server.KernelSettings()
	// fs.Debugf(f, "Server settings %+v", serverSettings)

	// Serve the mount point in the background returning error to errChan
	errs := make(chan error, 1)
	go func() {
		server.Serve()
		errs <- nil
	}()

	// wait for the mount point to be mounted
	fs.Debugf(f, "Waiting for the mount to start...")
	<-fsys.mounted

	return fsys.VFS, errs, umount, nil
}

// Mount mounts the remote at mountpoint.
//
// If noModTime is set then it
func Mount(f fs.Fs, mountpoint string) error {
	// Mount it
	vfs, errChan, unmount, err := mount(f, mountpoint)
	if err != nil {
		return errors.Wrap(err, "failed to mount FUSE fs")
	}

	sigInt := make(chan os.Signal, 1)
	signal.Notify(sigInt, syscall.SIGINT, syscall.SIGTERM)
	sigHup := make(chan os.Signal, 1)
	signal.Notify(sigHup, syscall.SIGHUP)

	if err := sdnotify.SdNotifyReady(); err != nil && err != sdnotify.SdNotifyNoSocket {
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
			root, err := vfs.Root()
			if err != nil {
				fs.Errorf(f, "Error reading root: %v", err)
			} else {
				root.ForgetAll()
			}
		}
	}

	_ = sdnotify.SdNotifyStopping()
	if err != nil {
		return errors.Wrap(err, "failed to umount FUSE fs")
	}

	return nil
}
