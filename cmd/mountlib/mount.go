package mountlib

import (
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/okzk/sdnotify"
	"github.com/pkg/errors"
	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/lib/atexit"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfsflags"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Options for creating the mount
type Options struct {
	DebugFUSE          bool
	AllowNonEmpty      bool
	AllowRoot          bool
	AllowOther         bool
	DefaultPermissions bool
	WritebackCache     bool
	Daemon             bool
	MaxReadAhead       fs.SizeSuffix
	ExtraOptions       []string
	ExtraFlags         []string
	AttrTimeout        time.Duration // how long the kernel caches attribute for
	VolumeName         string
	NoAppleDouble      bool
	NoAppleXattr       bool
	DaemonTimeout      time.Duration // OSXFUSE only
	AsyncRead          bool
}

// DefaultOpt is the default values for creating the mount
var DefaultOpt = Options{
	MaxReadAhead:  128 * 1024,
	AttrTimeout:   1 * time.Second, // how long the kernel caches attribute for
	NoAppleDouble: true,            // use noappledouble by default
	NoAppleXattr:  false,           // do not use noapplexattr by default
	AsyncRead:     true,            // do async reads by default
}

type (
	// UnmountFn is called to unmount the file system
	UnmountFn func() error
	// MountFn is called to mount the file system
	MountFn func(VFS *vfs.VFS, mountpoint string, opt *Options) (<-chan error, func() error, error)
)

// Global constants
const (
	MaxLeafSize = 1024 // don't pass file names longer than this
)

func init() {
	// DaemonTimeout defaults to non zero for macOS
	if runtime.GOOS == "darwin" {
		DefaultOpt.DaemonTimeout = 15 * time.Minute
	}
}

// Options set by command line flags
var (
	Opt = DefaultOpt
)

// AddFlags adds the non filing system specific flags to the command
func AddFlags(flagSet *pflag.FlagSet) {
	rc.AddOption("mount", &Opt)
	flags.BoolVarP(flagSet, &Opt.DebugFUSE, "debug-fuse", "", Opt.DebugFUSE, "Debug the FUSE internals - needs -v.")
	flags.BoolVarP(flagSet, &Opt.AllowNonEmpty, "allow-non-empty", "", Opt.AllowNonEmpty, "Allow mounting over a non-empty directory (not Windows).")
	flags.BoolVarP(flagSet, &Opt.AllowRoot, "allow-root", "", Opt.AllowRoot, "Allow access to root user.")
	flags.BoolVarP(flagSet, &Opt.AllowOther, "allow-other", "", Opt.AllowOther, "Allow access to other users.")
	flags.BoolVarP(flagSet, &Opt.DefaultPermissions, "default-permissions", "", Opt.DefaultPermissions, "Makes kernel enforce access control based on the file mode.")
	flags.BoolVarP(flagSet, &Opt.WritebackCache, "write-back-cache", "", Opt.WritebackCache, "Makes kernel buffer writes before sending them to rclone. Without this, writethrough caching is used.")
	flags.FVarP(flagSet, &Opt.MaxReadAhead, "max-read-ahead", "", "The number of bytes that can be prefetched for sequential reads.")
	flags.DurationVarP(flagSet, &Opt.AttrTimeout, "attr-timeout", "", Opt.AttrTimeout, "Time for which file/directory attributes are cached.")
	flags.StringArrayVarP(flagSet, &Opt.ExtraOptions, "option", "o", []string{}, "Option for libfuse/WinFsp. Repeat if required.")
	flags.StringArrayVarP(flagSet, &Opt.ExtraFlags, "fuse-flag", "", []string{}, "Flags or arguments to be passed direct to libfuse/WinFsp. Repeat if required.")
	flags.BoolVarP(flagSet, &Opt.Daemon, "daemon", "", Opt.Daemon, "Run mount as a daemon (background mode).")
	flags.StringVarP(flagSet, &Opt.VolumeName, "volname", "", Opt.VolumeName, "Set the volume name (not supported by all OSes).")
	flags.DurationVarP(flagSet, &Opt.DaemonTimeout, "daemon-timeout", "", Opt.DaemonTimeout, "Time limit for rclone to respond to kernel (not supported by all OSes).")
	flags.BoolVarP(flagSet, &Opt.AsyncRead, "async-read", "", Opt.AsyncRead, "Use asynchronous reads.")
	if runtime.GOOS == "darwin" {
		flags.BoolVarP(flagSet, &Opt.NoAppleDouble, "noappledouble", "", Opt.NoAppleDouble, "Sets the OSXFUSE option noappledouble.")
		flags.BoolVarP(flagSet, &Opt.NoAppleXattr, "noapplexattr", "", Opt.NoAppleXattr, "Sets the OSXFUSE option noapplexattr.")
	}
}

// Check is folder is empty
func checkMountEmpty(mountpoint string) error {
	fp, fpErr := os.Open(mountpoint)

	if fpErr != nil {
		return errors.Wrap(fpErr, "Can not open: "+mountpoint)
	}
	defer fs.CheckClose(fp, &fpErr)

	_, fpErr = fp.Readdirnames(1)

	// directory is not empty
	if fpErr != io.EOF {
		var e error
		var errorMsg = "Directory is not empty: " + mountpoint + " If you want to mount it anyway use: --allow-non-empty option"
		if fpErr == nil {
			e = errors.New(errorMsg)
		} else {
			e = errors.Wrap(fpErr, errorMsg)
		}
		return e
	}
	return nil
}

// Check the root doesn't overlap the mountpoint
func checkMountpointOverlap(root, mountpoint string) error {
	abs := func(x string) string {
		if absX, err := filepath.EvalSymlinks(x); err == nil {
			x = absX
		}
		if absX, err := filepath.Abs(x); err == nil {
			x = absX
		}
		x = filepath.ToSlash(x)
		if !strings.HasSuffix(x, "/") {
			x += "/"
		}
		return x
	}
	rootAbs, mountpointAbs := abs(root), abs(mountpoint)
	if strings.HasPrefix(rootAbs, mountpointAbs) || strings.HasPrefix(mountpointAbs, rootAbs) {
		return errors.Errorf("mount point %q and directory to be mounted %q mustn't overlap", mountpoint, root)
	}
	return nil
}

// NewMountCommand makes a mount command with the given name and Mount function
func NewMountCommand(commandName string, hidden bool, mount MountFn) *cobra.Command {
	var commandDefinition = &cobra.Command{
		Use:    commandName + " remote:path /path/to/mountpoint",
		Hidden: hidden,
		Short:  `Mount the remote as file system on a mountpoint.`,
		Long: `
rclone ` + commandName + ` allows Linux, FreeBSD, macOS and Windows to
mount any of Rclone's cloud storage systems as a file system with
FUSE.

First set up your remote using ` + "`rclone config`" + `.  Check it works with ` + "`rclone ls`" + ` etc.

You can either run mount in foreground mode or background (daemon) mode. Mount runs in
foreground mode by default, use the --daemon flag to specify background mode mode.
Background mode is only supported on Linux and OSX, you can only run mount in
foreground mode on Windows.

On Linux/macOS/FreeBSD Start the mount like this where ` + "`/path/to/local/mount`" + `
is an **empty** **existing** directory.

    rclone ` + commandName + ` remote:path/to/files /path/to/local/mount

Or on Windows like this where ` + "`X:`" + ` is an unused drive letter
or use a path to **non-existent** directory.

    rclone ` + commandName + ` remote:path/to/files X:
    rclone ` + commandName + ` remote:path/to/files C:\path\to\nonexistent\directory

When running in background mode the user will have to stop the mount manually (specified below).

When the program ends while in foreground mode, either via Ctrl+C or receiving
a SIGINT or SIGTERM signal, the mount is automatically stopped.

The umount operation can fail, for example when the mountpoint is busy.
When that happens, it is the user's responsibility to stop the mount manually.

Stopping the mount manually:

    # Linux
    fusermount -u /path/to/local/mount
    # OS X
    umount /path/to/local/mount

### Installing on Windows

To run rclone ` + commandName + ` on Windows, you will need to
download and install [WinFsp](http://www.secfs.net/winfsp/).

[WinFsp](https://github.com/billziss-gh/winfsp) is an open source
Windows File System Proxy which makes it easy to write user space file
systems for Windows.  It provides a FUSE emulation layer which rclone
uses combination with
[cgofuse](https://github.com/billziss-gh/cgofuse).  Both of these
packages are by Bill Zissimopoulos who was very helpful during the
implementation of rclone ` + commandName + ` for Windows.

#### Windows caveats

Note that drives created as Administrator are not visible by other
accounts (including the account that was elevated as
Administrator). So if you start a Windows drive from an Administrative
Command Prompt and then try to access the same drive from Explorer
(which does not run as Administrator), you will not be able to see the
new drive.

The easiest way around this is to start the drive from a normal
command prompt. It is also possible to start a drive from the SYSTEM
account (using [the WinFsp.Launcher
infrastructure](https://github.com/billziss-gh/winfsp/wiki/WinFsp-Service-Architecture))
which creates drives accessible for everyone on the system or
alternatively using [the nssm service manager](https://nssm.cc/usage).

#### Mount as a network drive

By default, rclone will mount the remote as a normal drive. However,
you can also mount it as a **Network Drive** (or **Network Share**, as
mentioned in some places)

Unlike other systems, Windows provides a different filesystem type for
network drives.  Windows and other programs treat the network drives
and fixed/removable drives differently: In network drives, many I/O
operations are optimized, as the high latency and low reliability
(compared to a normal drive) of a network is expected.

Although many people prefer network shares to be mounted as normal
system drives, this might cause some issues, such as programs not
working as expected or freezes and errors while operating with the
mounted remote in Windows Explorer. If you experience any of those,
consider mounting rclone remotes as network shares, as Windows expects
normal drives to be fast and reliable, while cloud storage is far from
that.  See also [Limitations](#limitations) section below for more
info

Add "--fuse-flag --VolumePrefix=\server\share" to your "mount"
command, **replacing "share" with any other name of your choice if you
are mounting more than one remote**. Otherwise, the mountpoints will
conflict and your mounted filesystems will overlap.

[Read more about drive mapping](https://en.wikipedia.org/wiki/Drive_mapping)

### Limitations

Without the use of "--vfs-cache-mode" this can only write files
sequentially, it can only seek when reading.  This means that many
applications won't work with their files on an rclone mount without
"--vfs-cache-mode writes" or "--vfs-cache-mode full".  See the [File
Caching](#file-caching) section for more info.

The bucket based remotes (eg Swift, S3, Google Compute Storage, B2,
Hubic) do not support the concept of empty directories, so empty
directories will have a tendency to disappear once they fall out of
the directory cache.

Only supported on Linux, FreeBSD, OS X and Windows at the moment.

### rclone ` + commandName + ` vs rclone sync/copy

File systems expect things to be 100% reliable, whereas cloud storage
systems are a long way from 100% reliable. The rclone sync/copy
commands cope with this with lots of retries.  However rclone ` + commandName + `
can't use retries in the same way without making local copies of the
uploads. Look at the [file caching](#file-caching)
for solutions to make ` + commandName + ` more reliable.

### Attribute caching

You can use the flag --attr-timeout to set the time the kernel caches
the attributes (size, modification time etc) for directory entries.

The default is "1s" which caches files just long enough to avoid
too many callbacks to rclone from the kernel.

In theory 0s should be the correct value for filesystems which can
change outside the control of the kernel. However this causes quite a
few problems such as
[rclone using too much memory](https://github.com/rclone/rclone/issues/2157),
[rclone not serving files to samba](https://forum.rclone.org/t/rclone-1-39-vs-1-40-mount-issue/5112)
and [excessive time listing directories](https://github.com/rclone/rclone/issues/2095#issuecomment-371141147).

The kernel can cache the info about a file for the time given by
"--attr-timeout". You may see corruption if the remote file changes
length during this window.  It will show up as either a truncated file
or a file with garbage on the end.  With "--attr-timeout 1s" this is
very unlikely but not impossible.  The higher you set "--attr-timeout"
the more likely it is.  The default setting of "1s" is the lowest
setting which mitigates the problems above.

If you set it higher ('10s' or '1m' say) then the kernel will call
back to rclone less often making it more efficient, however there is
more chance of the corruption issue above.

If files don't change on the remote outside of the control of rclone
then there is no chance of corruption.

This is the same as setting the attr_timeout option in mount.fuse.

### Filters

Note that all the rclone filters can be used to select a subset of the
files to be visible in the mount.

### systemd

When running rclone ` + commandName + ` as a systemd service, it is possible
to use Type=notify. In this case the service will enter the started state
after the mountpoint has been successfully set up.
Units having the rclone ` + commandName + ` service specified as a requirement
will see all files and folders immediately in this mode.

### chunked reading ###

--vfs-read-chunk-size will enable reading the source objects in parts.
This can reduce the used download quota for some remotes by requesting only chunks
from the remote that are actually read at the cost of an increased number of requests.

When --vfs-read-chunk-size-limit is also specified and greater than --vfs-read-chunk-size,
the chunk size for each open file will get doubled for each chunk read, until the
specified value is reached. A value of -1 will disable the limit and the chunk size will
grow indefinitely.

With --vfs-read-chunk-size 100M and --vfs-read-chunk-size-limit 0 the following
parts will be downloaded: 0-100M, 100M-200M, 200M-300M, 300M-400M and so on.
When --vfs-read-chunk-size-limit 500M is specified, the result would be
0-100M, 100M-300M, 300M-700M, 700M-1200M, 1200M-1700M and so on.

Chunked reading will only work with --vfs-cache-mode < full, as the file will always
be copied to the vfs cache before opening with --vfs-cache-mode full.
` + vfs.Help,
		Run: func(command *cobra.Command, args []string) {
			cmd.CheckArgs(2, 2, command, args)
			opt := Opt // make a copy of the options

			if opt.Daemon {
				config.PassConfigKeyForDaemonization = true
			}

			mountpoint := args[1]
			fdst := cmd.NewFsDir(args)
			if fdst.Name() == "" || fdst.Name() == "local" {
				err := checkMountpointOverlap(fdst.Root(), mountpoint)
				if err != nil {
					log.Fatalf("Fatal error: %v", err)
				}
			}

			// Show stats if the user has specifically requested them
			if cmd.ShowStats() {
				defer cmd.StartStats()()
			}

			// Skip checkMountEmpty if --allow-non-empty flag is used or if
			// the Operating System is Windows
			if !opt.AllowNonEmpty && runtime.GOOS != "windows" {
				err := checkMountEmpty(mountpoint)
				if err != nil {
					log.Fatalf("Fatal error: %v", err)
				}
			} else if opt.AllowNonEmpty && runtime.GOOS == "windows" {
				fs.Logf(nil, "--allow-non-empty flag does nothing on Windows")
			}

			// Work out the volume name, removing special
			// characters from it if necessary
			if opt.VolumeName == "" {
				opt.VolumeName = fdst.Name() + ":" + fdst.Root()
			}
			opt.VolumeName = strings.Replace(opt.VolumeName, ":", " ", -1)
			opt.VolumeName = strings.Replace(opt.VolumeName, "/", " ", -1)
			opt.VolumeName = strings.TrimSpace(opt.VolumeName)
			if runtime.GOOS == "windows" && len(opt.VolumeName) > 32 {
				opt.VolumeName = opt.VolumeName[:32]
			}

			// Start background task if --background is specified
			if opt.Daemon {
				daemonized := startBackgroundMode()
				if daemonized {
					return
				}
			}

			VFS := vfs.New(fdst, &vfsflags.Opt)
			err := Mount(VFS, mountpoint, mount, &opt)
			if err != nil {
				log.Fatalf("Fatal error: %v", err)
			}
		},
	}

	// Register the command
	cmd.Root.AddCommand(commandDefinition)

	// Add flags
	cmdFlags := commandDefinition.Flags()
	AddFlags(cmdFlags)
	vfsflags.AddFlags(cmdFlags)

	return commandDefinition
}

// ClipBlocks clips the blocks pointed to the OS max
func ClipBlocks(b *uint64) {
	var max uint64
	switch runtime.GOOS {
	case "windows":
		if runtime.GOARCH == "386" {
			max = (1 << 32) - 1
		} else {
			max = (1 << 43) - 1
		}
	case "darwin":
		// OSX FUSE only supports 32 bit number of blocks
		// https://github.com/osxfuse/osxfuse/issues/396
		max = (1 << 32) - 1
	default:
		// no clipping
		return
	}
	if *b > max {
		*b = max
	}
}

// Mount mounts the remote at mountpoint.
//
// If noModTime is set then it
func Mount(VFS *vfs.VFS, mountpoint string, mount MountFn, opt *Options) error {
	if opt == nil {
		opt = &DefaultOpt
	}

	// Mount it
	errChan, unmount, err := mount(VFS, mountpoint, opt)
	if err != nil {
		return errors.Wrap(err, "failed to mount FUSE fs")
	}

	// Unmount on exit
	fnHandle := atexit.Register(func() {
		_ = unmount()
		_ = sdnotify.Stopping()
	})
	defer atexit.Unregister(fnHandle)

	// Notify systemd
	if err := sdnotify.Ready(); err != nil && err != sdnotify.ErrSdNotifyNoSocket {
		return errors.Wrap(err, "failed to notify systemd")
	}

	// Reload VFS cache on SIGHUP
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
			root, err := VFS.Root()
			if err != nil {
				fs.Errorf(VFS.Fs(), "Error reading root: %v", err)
			} else {
				root.ForgetAll()
			}
		}
	}

	_ = unmount()
	_ = sdnotify.Stopping()

	if err != nil {
		return errors.Wrap(err, "failed to umount FUSE fs")
	}

	return nil
}
