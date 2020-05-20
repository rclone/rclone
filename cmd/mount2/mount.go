// Package mount implements a FUSE mounting system for rclone remotes.

// +build linux darwin,amd64

package mount2

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	fusefs "github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/okzk/sdnotify"
	"github.com/pkg/errors"
	"github.com/rclone/rclone/cmd/mountlib"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/atexit"
	"github.com/rclone/rclone/vfs"
)

func init() {
	mountlib.NewMountCommand("mount2", true, Mount)

	// Add mount to rc
	mountlib.AddRc("mount2", mount)

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

		// RememberInodes: true,
		// SingleThreaded: true,

		/*
			AllowOther bool

			// Options are passed as -o string to fusermount.
			Options []string

			// Default is _DEFAULT_BACKGROUND_TASKS, 12.  This numbers
			// controls the allowed number of requests that relate to
			// async I/O.  Concurrency for synchronous I/O is not limited.
			MaxBackground int

			// Write size to use.  If 0, use default. This number is
			// capped at the kernel maximum.
			MaxWrite int

			// Max read ahead to use.  If 0, use default. This number is
			// capped at the kernel maximum.
			MaxReadAhead int

			// If IgnoreSecurityLabels is set, all security related xattr
			// requests will return NO_DATA without passing through the
			// user defined filesystem.  You should only set this if you
			// file system implements extended attributes, and you are not
			// interested in security labels.
			IgnoreSecurityLabels bool // ignoring labels should be provided as a fusermount mount option.

			// If RememberInodes is set, we will never forget inodes.
			// This may be useful for NFS.
			RememberInodes bool

			// Values shown in "df -T" and friends
			// First column, "Filesystem"
			FsName string

			// Second column, "Type", will be shown as "fuse." + Name
			Name string

			// If set, wrap the file system in a single-threaded locking wrapper.
			SingleThreaded bool

			// If set, return ENOSYS for Getxattr calls, so the kernel does not issue any
			// Xattr operations at all.
			DisableXAttrs bool

			// If set, print debugging information.
			Debug bool

			// If set, ask kernel to forward file locks to FUSE. If using,
			// you must implement the GetLk/SetLk/SetLkw methods.
			EnableLocks bool

			// If set, ask kernel not to do automatic data cache invalidation.
			// The filesystem is fully responsible for invalidating data cache.
			ExplicitDataCacheControl bool
		*/

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

	fsys := NewFS(f)
	// nodeFsOpts := &fusefs.PathNodeFsOptions{
	// 	ClientInodes: false,
	// 	Debug:        mountlib.DebugFUSE,
	// }
	// nodeFs := fusefs.NewPathNodeFs(fsys, nodeFsOpts)

	//mOpts := fusefs.NewOptions() // default options
	// FIXME
	// mOpts.EntryTimeout = 10 * time.Second
	// mOpts.AttrTimeout = 10 * time.Second
	// mOpts.NegativeTimeout = 10 * time.Second
	//mOpts.Debug = mountlib.DebugFUSE

	//conn := fusefs.NewFileSystemConnector(nodeFs.Root(), mOpts)
	mountOpts := mountOptions(fsys, f)

	// FIXME fill out
	opts := fusefs.Options{
		MountOptions: *mountOpts,
		EntryTimeout: &mountlib.AttrTimeout,
		AttrTimeout:  &mountlib.AttrTimeout,
		// UID
		// GID
	}

	root, err := fsys.Root()
	if err != nil {
		return nil, nil, nil, err
	}

	rawFS := fusefs.NewNodeFS(root, &opts)
	server, err := fuse.NewServer(rawFS, mountpoint, &opts.MountOptions)
	if err != nil {
		return nil, nil, nil, err
	}

	//mountOpts := &fuse.MountOptions{}
	//server, err := fusefs.Mount(mountpoint, fsys, &opts)
	// server, err := fusefs.Mount(mountpoint, root, &opts)
	// if err != nil {
	// 	return nil, nil, nil, err
	// }

	umount := func() error {
		// Shutdown the VFS
		fsys.VFS.Shutdown()
		return server.Unmount()
	}

	// serverSettings := server.KernelSettings()
	// fs.Debugf(f, "Server settings %+v", serverSettings)

	// Serve the mount point in the background returning error to errChan
	errs := make(chan error, 1)
	go func() {
		server.Serve()
		errs <- nil
	}()

	fs.Debugf(f, "Waiting for the mount to start...")
	err = server.WaitMount()
	if err != nil {
		return nil, nil, nil, err
	}

	fs.Debugf(f, "Mount started")
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
			root, err := vfs.Root()
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
