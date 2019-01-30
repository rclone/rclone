package mountlib

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/config"
	"github.com/ncw/rclone/fs/config/flags"
	"github.com/ncw/rclone/vfs"
	"github.com/ncw/rclone/vfs/vfsflags"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// Options set by command line flags
var (
	DebugFUSE                        = false
	AllowNonEmpty                    = false
	AllowRoot                        = false
	AllowOther                       = false
	DefaultPermissions               = false
	WritebackCache                   = false
	Daemon                           = false
	MaxReadAhead       fs.SizeSuffix = 128 * 1024
	ExtraOptions       []string
	ExtraFlags         []string
	AttrTimeout        = 1 * time.Second // how long the kernel caches attribute for
	VolumeName         string
	NoAppleDouble      = true        // use noappledouble by default
	NoAppleXattr       = false       // do not use noapplexattr by default
	DaemonTimeout      time.Duration // OSXFUSE only
)

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
func NewMountCommand(commandName string, Mount func(f fs.Fs, mountpoint string) error) *cobra.Command {
	var commandDefintion = &cobra.Command{
		Use:   commandName + " remote:path /path/to/mountpoint",
		Short: `Mount the remote as file system on a mountpoint.`,
		Long: `
rclone ` + commandName + ` allows Linux, FreeBSD, macOS and Windows to
mount any of Rclone's cloud storage systems as a file system with
FUSE.

First set up your remote using ` + "`rclone config`" + `.  Check it works with ` + "`rclone ls`" + ` etc.

Start the mount like this

    rclone ` + commandName + ` remote:path/to/files /path/to/local/mount

Or on Windows like this where X: is an unused drive letter

    rclone ` + commandName + ` remote:path/to/files X:

When the program ends, either via Ctrl+C or receiving a SIGINT or SIGTERM signal,
the mount is automatically stopped.

The umount operation can fail, for example when the mountpoint is busy.
When that happens, it is the user's responsibility to stop the mount manually with

    # Linux
    fusermount -u /path/to/local/mount
    # OS X
    umount /path/to/local/mount

### Installing on Windows

To run rclone ` + commandName + ` on Windows, you will need to
download and install [WinFsp](http://www.secfs.net/winfsp/).

WinFsp is an [open source](https://github.com/billziss-gh/winfsp)
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

### Limitations

Without the use of "--vfs-cache-mode" this can only write files
sequentially, it can only seek when reading.  This means that many
applications won't work with their files on an rclone mount without
"--vfs-cache-mode writes" or "--vfs-cache-mode full".  See the [File
Caching](#file-caching) section for more info.

The bucket based remotes (eg Swift, S3, Google Compute Storage, B2,
Hubic) won't work from the root - you will need to specify a bucket,
or a path within the bucket.  So ` + "`swift:`" + ` won't work whereas
` + "`swift:bucket`" + ` will as will ` + "`swift:bucket/path`" + `.
None of these support the concept of directories, so empty
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
[rclone using too much memory](https://github.com/ncw/rclone/issues/2157),
[rclone not serving files to samba](https://forum.rclone.org/t/rclone-1-39-vs-1-40-mount-issue/5112)
and [excessive time listing directories](https://github.com/ncw/rclone/issues/2095#issuecomment-371141147).

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

			if Daemon {
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
			if !AllowNonEmpty && runtime.GOOS != "windows" {
				err := checkMountEmpty(mountpoint)
				if err != nil {
					log.Fatalf("Fatal error: %v", err)
				}
			}

			// Work out the volume name, removing special
			// characters from it if necessary
			if VolumeName == "" {
				VolumeName = fdst.Name() + ":" + fdst.Root()
			}
			VolumeName = strings.Replace(VolumeName, ":", " ", -1)
			VolumeName = strings.Replace(VolumeName, "/", " ", -1)
			VolumeName = strings.TrimSpace(VolumeName)

			// Start background task if --background is specified
			if Daemon {
				daemonized := startBackgroundMode()
				if daemonized {
					return
				}
			}

			err := Mount(fdst, mountpoint)
			if err != nil {
				log.Fatalf("Fatal error: %v", err)
			}
		},
	}

	// Register the command
	cmd.Root.AddCommand(commandDefintion)

	// Add flags
	flagSet := commandDefintion.Flags()
	flags.BoolVarP(flagSet, &DebugFUSE, "debug-fuse", "", DebugFUSE, "Debug the FUSE internals - needs -v.")
	// mount options
	flags.BoolVarP(flagSet, &AllowNonEmpty, "allow-non-empty", "", AllowNonEmpty, "Allow mounting over a non-empty directory.")
	flags.BoolVarP(flagSet, &AllowRoot, "allow-root", "", AllowRoot, "Allow access to root user.")
	flags.BoolVarP(flagSet, &AllowOther, "allow-other", "", AllowOther, "Allow access to other users.")
	flags.BoolVarP(flagSet, &DefaultPermissions, "default-permissions", "", DefaultPermissions, "Makes kernel enforce access control based on the file mode.")
	flags.BoolVarP(flagSet, &WritebackCache, "write-back-cache", "", WritebackCache, "Makes kernel buffer writes before sending them to rclone. Without this, writethrough caching is used.")
	flags.FVarP(flagSet, &MaxReadAhead, "max-read-ahead", "", "The number of bytes that can be prefetched for sequential reads.")
	flags.DurationVarP(flagSet, &AttrTimeout, "attr-timeout", "", AttrTimeout, "Time for which file/directory attributes are cached.")
	flags.StringArrayVarP(flagSet, &ExtraOptions, "option", "o", []string{}, "Option for libfuse/WinFsp. Repeat if required.")
	flags.StringArrayVarP(flagSet, &ExtraFlags, "fuse-flag", "", []string{}, "Flags or arguments to be passed direct to libfuse/WinFsp. Repeat if required.")
	flags.BoolVarP(flagSet, &Daemon, "daemon", "", Daemon, "Run mount as a daemon (background mode).")
	flags.StringVarP(flagSet, &VolumeName, "volname", "", VolumeName, "Set the volume name (not supported by all OSes).")
	flags.DurationVarP(flagSet, &DaemonTimeout, "daemon-timeout", "", DaemonTimeout, "Time limit for rclone to respond to kernel (not supported by all OSes).")

	if runtime.GOOS == "darwin" {
		flags.BoolVarP(flagSet, &NoAppleDouble, "noappledouble", "", NoAppleDouble, "Sets the OSXFUSE option noappledouble.")
		flags.BoolVarP(flagSet, &NoAppleXattr, "noapplexattr", "", NoAppleXattr, "Sets the OSXFUSE option noapplexattr.")
	}

	// Add in the generic flags
	vfsflags.AddFlags(flagSet)

	return commandDefintion
}

// ClipBlocks clips the blocks pointed to to the OS max
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
