//go:build linux || (darwin && amd64)

// Package mount2 implements a FUSE mounting system for rclone remotes.
package mount2

import (
	"fmt"
	"runtime"
	"time"

	fusefs "github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/rclone/rclone/cmd/mountlib"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/vfs"
)

func init() {
	mountlib.NewMountCommand("mount2", true, mount)
	mountlib.AddRc("mount2", mount)
}

// mountOptions configures the options from the command line flags
//
// man mount.fuse for more info and note the -o flag for other options
func mountOptions(fsys *FS, f fs.Fs, opt *mountlib.Options) (mountOpts *fuse.MountOptions) {
	mountOpts = &fuse.MountOptions{
		AllowOther:         fsys.opt.AllowOther,
		FsName:             opt.DeviceName,
		Name:               "rclone",
		DisableXAttrs:      true,
		Debug:              fsys.opt.DebugFUSE,
		MaxReadAhead:       int(fsys.opt.MaxReadAhead),
		MaxWrite:           1024 * 1024, // Linux v4.20+ caps requests at 1 MiB
		DisableReadDirPlus: true,

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

			// MaxWrite is the max size for read and write requests. If 0, use
			// go-fuse default (currently 64 kiB).
			// This number is internally capped at MAX_KERNEL_WRITE (higher values don't make
			// sense).
			//
			// Non-direct-io reads are mostly served via kernel readahead, which is
			// additionally subject to the MaxReadAhead limit.
			//
			// Implementation notes:
			//
			// There's four values the Linux kernel looks at when deciding the request size:
			// * MaxWrite, passed via InitOut.MaxWrite. Limits the WRITE size.
			// * max_read, passed via a string mount option. Limits the READ size.
			//   go-fuse sets max_read equal to MaxWrite.
			//   You can see the current max_read value in /proc/self/mounts .
			// * MaxPages, passed via InitOut.MaxPages. In Linux 4.20 and later, the value
			//   can go up to 1 MiB and go-fuse calculates the MaxPages value acc.
			//   to MaxWrite, rounding up.
			//   On older kernels, the value is fixed at 128 kiB and the
			//   passed value is ignored. No request can be larger than MaxPages, so
			//   READ and WRITE are effectively capped at MaxPages.
			// * MaxReadAhead, passed via InitOut.MaxReadAhead.
			MaxWrite int

			// MaxReadAhead is the max read ahead size to use. It controls how much data the
			// kernel reads in advance to satisfy future read requests from applications.
			// How much exactly is subject to clever heuristics in the kernel
			// (see https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git/tree/mm/readahead.c?h=v6.2-rc5#n375
			// if you are brave) and hence also depends on the kernel version.
			//
			// If 0, use kernel default. This number is capped at the kernel maximum
			// (128 kiB on Linux) and cannot be larger than MaxWrite.
			//
			// MaxReadAhead only affects buffered reads (=non-direct-io), but even then, the
			// kernel can and does send larger reads to satisfy read requests from applications
			// (up to MaxWrite or VM_READAHEAD_PAGES=128 kiB, whichever is less).
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

			// If set, the kernel caches all Readlink return values. The
			// filesystem must use content notification to force the
			// kernel to issue a new Readlink call.
			EnableSymlinkCaching bool

			// If set, ask kernel not to do automatic data cache invalidation.
			// The filesystem is fully responsible for invalidating data cache.
			ExplicitDataCacheControl bool

			// Disable ReadDirPlus capability so ReadDir is used instead. Simple
			// directory queries (i.e. 'ls' without '-l') can be faster with
			// ReadDir, as no per-file stat calls are needed
			DisableReadDirPlus bool
		*/

	}
	var opts []string
	// FIXME doesn't work opts = append(opts, fmt.Sprintf("max_readahead=%d", maxReadAhead))
	if fsys.opt.AllowOther {
		opts = append(opts, "allow_other")
	}
	if fsys.opt.AllowRoot {
		opts = append(opts, "allow_root")
	}
	if fsys.opt.DefaultPermissions {
		opts = append(opts, "default_permissions")
	}
	if fsys.VFS.Opt.ReadOnly {
		opts = append(opts, "ro")
	}
	if fsys.opt.WritebackCache {
		fs.Printf(nil, "FIXME --write-back-cache not supported")
		// FIXME opts = append(opts,fuse.WritebackCache())
	}
	// Some OS X only options
	if runtime.GOOS == "darwin" {
		opts = append(opts,
			// VolumeName sets the volume name shown in Finder.
			fmt.Sprintf("volname=%s", opt.VolumeName),

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
func mount(VFS *vfs.VFS, mountpoint string, opt *mountlib.Options) (<-chan error, func() error, error) {
	f := VFS.Fs()
	if err := mountlib.CheckOverlap(f, mountpoint); err != nil {
		return nil, nil, err
	}
	if err := mountlib.CheckAllowNonEmpty(mountpoint, opt); err != nil {
		return nil, nil, err
	}
	fs.Debugf(f, "Mounting on %q", mountpoint)

	fsys := NewFS(VFS, opt)

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
	mountOpts := mountOptions(fsys, f, opt)

	// FIXME fill out
	opts := fusefs.Options{
		MountOptions: *mountOpts,
		EntryTimeout: (*time.Duration)(&opt.AttrTimeout),
		AttrTimeout:  (*time.Duration)(&opt.AttrTimeout),
		GID:          VFS.Opt.GID,
		UID:          VFS.Opt.UID,
	}

	root, err := fsys.Root()
	if err != nil {
		return nil, nil, err
	}

	rawFS := fusefs.NewNodeFS(root, &opts)
	server, err := fuse.NewServer(rawFS, mountpoint, &opts.MountOptions)
	if err != nil {
		return nil, nil, err
	}

	//mountOpts := &fuse.MountOptions{}
	//server, err := fusefs.Mount(mountpoint, fsys, &opts)
	// server, err := fusefs.Mount(mountpoint, root, &opts)
	// if err != nil {
	// 	return nil, nil, err
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
		return nil, nil, err
	}

	fs.Debugf(f, "Mount started")
	return errs, umount, nil
}
