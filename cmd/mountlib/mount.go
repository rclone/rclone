package mountlib

import (
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	sysdnotify "github.com/iguanesolutions/go-systemd/v5/notify"
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
	NetworkMode        bool // Windows only
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
		DefaultOpt.DaemonTimeout = 10 * time.Minute
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
	flags.DurationVarP(flagSet, &Opt.AttrTimeout, "attr-timeout", "", Opt.AttrTimeout, "Time for which file/directory attributes are cached.")
	flags.StringArrayVarP(flagSet, &Opt.ExtraOptions, "option", "o", []string{}, "Option for libfuse/WinFsp. Repeat if required.")
	flags.StringArrayVarP(flagSet, &Opt.ExtraFlags, "fuse-flag", "", []string{}, "Flags or arguments to be passed direct to libfuse/WinFsp. Repeat if required.")
	// Non-Windows only
	flags.BoolVarP(flagSet, &Opt.Daemon, "daemon", "", Opt.Daemon, "Run mount as a daemon (background mode). Not supported on Windows.")
	flags.DurationVarP(flagSet, &Opt.DaemonTimeout, "daemon-timeout", "", Opt.DaemonTimeout, "Time limit for rclone to respond to kernel. Not supported on Windows.")
	flags.BoolVarP(flagSet, &Opt.DefaultPermissions, "default-permissions", "", Opt.DefaultPermissions, "Makes kernel enforce access control based on the file mode. Not supported on Windows.")
	flags.BoolVarP(flagSet, &Opt.AllowNonEmpty, "allow-non-empty", "", Opt.AllowNonEmpty, "Allow mounting over a non-empty directory. Not supported on Windows.")
	flags.BoolVarP(flagSet, &Opt.AllowRoot, "allow-root", "", Opt.AllowRoot, "Allow access to root user. Not supported on Windows.")
	flags.BoolVarP(flagSet, &Opt.AllowOther, "allow-other", "", Opt.AllowOther, "Allow access to other users. Not supported on Windows.")
	flags.BoolVarP(flagSet, &Opt.AsyncRead, "async-read", "", Opt.AsyncRead, "Use asynchronous reads. Not supported on Windows.")
	flags.FVarP(flagSet, &Opt.MaxReadAhead, "max-read-ahead", "", "The number of bytes that can be prefetched for sequential reads. Not supported on Windows.")
	flags.BoolVarP(flagSet, &Opt.WritebackCache, "write-back-cache", "", Opt.WritebackCache, "Makes kernel buffer writes before sending them to rclone. Without this, writethrough caching is used. Not supported on Windows.")
	// Windows and OSX
	flags.StringVarP(flagSet, &Opt.VolumeName, "volname", "", Opt.VolumeName, "Set the volume name. Supported on Windows and OSX only.")
	// OSX only
	flags.BoolVarP(flagSet, &Opt.NoAppleDouble, "noappledouble", "", Opt.NoAppleDouble, "Ignore Apple Double (._) and .DS_Store files. Supported on OSX only.")
	flags.BoolVarP(flagSet, &Opt.NoAppleXattr, "noapplexattr", "", Opt.NoAppleXattr, "Ignore all \"com.apple.*\" extended attributes. Supported on OSX only.")
	// Windows only
	flags.BoolVarP(flagSet, &Opt.NetworkMode, "network-mode", "", Opt.NetworkMode, "Mount as remote network drive, instead of fixed disk drive. Supported on Windows only")
}

// Check if folder is empty
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
		// Warning! "|" will be replaced by backticks below
		//          "@" will be replaced by the command name
		Long: strings.ReplaceAll(strings.ReplaceAll(`
rclone @ allows Linux, FreeBSD, macOS and Windows to
mount any of Rclone's cloud storage systems as a file system with
FUSE.

First set up your remote using |rclone config|.  Check it works with |rclone ls| etc.

On Linux and OSX, you can either run mount in foreground mode or background (daemon) mode.
Mount runs in foreground mode by default, use the |--daemon| flag to specify background mode.
You can only run mount in foreground mode on Windows.

On Linux/macOS/FreeBSD start the mount like this, where |/path/to/local/mount|
is an **empty** **existing** directory:

    rclone @ remote:path/to/files /path/to/local/mount

On Windows you can start a mount in different ways. See [below](#mounting-modes-on-windows)
for details. The following examples will mount to an automatically assigned drive,
to specific drive letter |X:|, to path |C:\path\parent\mount|
(where parent directory or drive must exist, and mount must **not** exist,
and is not supported when [mounting as a network drive](#mounting-modes-on-windows)), and
the last example will mount as network share |\\cloud\remote| and map it to an
automatically assigned drive:

    rclone @ remote:path/to/files *
    rclone @ remote:path/to/files X:
    rclone @ remote:path/to/files C:\path\parent\mount
    rclone @ remote:path/to/files \\cloud\remote

When the program ends while in foreground mode, either via Ctrl+C or receiving
a SIGINT or SIGTERM signal, the mount should be automatically stopped.

When running in background mode the user will have to stop the mount manually:

    # Linux
    fusermount -u /path/to/local/mount
    # OS X
    umount /path/to/local/mount

The umount operation can fail, for example when the mountpoint is busy.
When that happens, it is the user's responsibility to stop the mount manually.

The size of the mounted file system will be set according to information retrieved
from the remote, the same as returned by the [rclone about](https://rclone.org/commands/rclone_about/)
command. Remotes with unlimited storage may report the used size only,
then an additional 1 PiB of free space is assumed. If the remote does not
[support](https://rclone.org/overview/#optional-features) the about feature
at all, then 1 PiB is set as both the total and the free size.

**Note**: As of |rclone| 1.52.2, |rclone mount| now requires Go version 1.13
or newer on some platforms depending on the underlying FUSE library in use.

### Installing on Windows

To run rclone @ on Windows, you will need to
download and install [WinFsp](http://www.secfs.net/winfsp/).

[WinFsp](https://github.com/billziss-gh/winfsp) is an open source
Windows File System Proxy which makes it easy to write user space file
systems for Windows.  It provides a FUSE emulation layer which rclone
uses combination with [cgofuse](https://github.com/billziss-gh/cgofuse).
Both of these packages are by Bill Zissimopoulos who was very helpful
during the implementation of rclone @ for Windows.

#### Mounting modes on windows

Unlike other operating systems, Microsoft Windows provides a different filesystem
type for network and fixed drives. It optimises access on the assumption fixed
disk drives are fast and reliable, while network drives have relatively high latency
and less reliability. Some settings can also be differentiated between the two types,
for example that Windows Explorer should just display icons and not create preview
thumbnails for image and video files on network drives.

In most cases, rclone will mount the remote as a normal, fixed disk drive by default.
However, you can also choose to mount it as a remote network drive, often described
as a network share. If you mount an rclone remote using the default, fixed drive mode
and experience unexpected program errors, freezes or other issues, consider mounting
as a network drive instead.

When mounting as a fixed disk drive you can either mount to an unused drive letter,
or to a path representing a **non-existent** subdirectory of an **existing** parent
directory or drive. Using the special value |*| will tell rclone to
automatically assign the next available drive letter, starting with Z: and moving backward.
Examples:

    rclone @ remote:path/to/files *
    rclone @ remote:path/to/files X:
    rclone @ remote:path/to/files C:\path\parent\mount
    rclone @ remote:path/to/files X:

Option |--volname| can be used to set a custom volume name for the mounted
file system. The default is to use the remote name and path.

To mount as network drive, you can add option |--network-mode|
to your @ command. Mounting to a directory path is not supported in
this mode, it is a limitation Windows imposes on junctions, so the remote must always
be mounted to a drive letter.

    rclone @ remote:path/to/files X: --network-mode

A volume name specified with |--volname| will be used to create the network share path.
A complete UNC path, such as |\\cloud\remote|, optionally with path
|\\cloud\remote\madeup\path|, will be used as is. Any other
string will be used as the share part, after a default prefix |\\server\|.
If no volume name is specified then |\\server\share| will be used.
You must make sure the volume name is unique when you are mounting more than one drive,
or else the mount command will fail. The share name will treated as the volume label for
the mapped drive, shown in Windows Explorer etc, while the complete
|\\server\share| will be reported as the remote UNC path by
|net use| etc, just like a normal network drive mapping.

If you specify a full network share UNC path with |--volname|, this will implicitely
set the |--network-mode| option, so the following two examples have same result:

    rclone @ remote:path/to/files X: --network-mode
    rclone @ remote:path/to/files X: --volname \\server\share

You may also specify the network share UNC path as the mountpoint itself. Then rclone
will automatically assign a drive letter, same as with |*| and use that as
mountpoint, and instead use the UNC path specified as the volume name, as if it were
specified with the |--volname| option. This will also implicitely set
the |--network-mode| option. This means the following two examples have same result:

    rclone @ remote:path/to/files \\cloud\remote
    rclone @ remote:path/to/files * --volname \\cloud\remote

There is yet another way to enable network mode, and to set the share path,
and that is to pass the "native" libfuse/WinFsp option directly:
|--fuse-flag --VolumePrefix=\server\share|. Note that the path
must be with just a single backslash prefix in this case.


*Note:* In previous versions of rclone this was the only supported method.

[Read more about drive mapping](https://en.wikipedia.org/wiki/Drive_mapping)

See also [Limitations](#limitations) section below.

#### Windows filesystem permissions

The FUSE emulation layer on Windows must convert between the POSIX-based
permission model used in FUSE, and the permission model used in Windows,
based on access-control lists (ACL).

The mounted filesystem will normally get three entries in its access-control list (ACL),
representing permissions for the POSIX permission scopes: Owner, group and others.
By default, the owner and group will be taken from the current user, and the built-in
group "Everyone" will be used to represent others. The user/group can be customized
with FUSE options "UserName" and "GroupName",
e.g. |-o UserName=user123 -o GroupName="Authenticated Users"|.

The permissions on each entry will be set according to
[options](#options) |--dir-perms| and |--file-perms|,
which takes a value in traditional [numeric notation](https://en.wikipedia.org/wiki/File-system_permissions#Numeric_notation),
where the default corresponds to |--file-perms 0666 --dir-perms 0777|.

Note that the mapping of permissions is not always trivial, and the result
you see in Windows Explorer may not be exactly like you expected.
For example, when setting a value that includes write access, this will be
mapped to individual permissions "write attributes", "write data" and "append data",
but not "write extended attributes". Windows will then show this as basic
permission "Special" instead of "Write", because "Write" includes the
"write extended attributes" permission.

If you set POSIX permissions for only allowing access to the owner, using
|--file-perms 0600 --dir-perms 0700|, the user group and the built-in "Everyone"
group will still be given some special permissions, such as "read attributes"
and "read permissions", in Windows. This is done for compatibility reasons,
e.g. to allow users without additional permissions to be able to read basic
metadata about files like in UNIX. One case that may arise is that other programs
(incorrectly) interprets this as the file being accessible by everyone. For example
an SSH client may warn about "unprotected private key file".

WinFsp 2021 (version 1.9) introduces a new FUSE option "FileSecurity",
that allows the complete specification of file security descriptors using
[SDDL](https://docs.microsoft.com/en-us/windows/win32/secauthz/security-descriptor-string-format).
With this you can work around issues such as the mentioned "unprotected private key file"
by specifying |-o FileSecurity="D:P(A;;FA;;;OW)"|, for file all access (FA) to the owner (OW).

#### Windows caveats

Drives created as Administrator are not visible to other accounts,
not even an account that was elevated to Administrator with the
User Account Control (UAC) feature. A result of this is that if you mount
to a drive letter from a Command Prompt run as Administrator, and then try
to access the same drive from Windows Explorer (which does not run as
Administrator), you will not be able to see the mounted drive.

If you don't need to access the drive from applications running with
administrative privileges, the easiest way around this is to always
create the mount from a non-elevated command prompt.

To make mapped drives available to the user account that created them
regardless if elevated or not, there is a special Windows setting called
[linked connections](https://docs.microsoft.com/en-us/troubleshoot/windows-client/networking/mapped-drives-not-available-from-elevated-command#detail-to-configure-the-enablelinkedconnections-registry-entry)
that can be enabled.

It is also possible to make a drive mount available to everyone on the system,
by running the process creating it as the built-in SYSTEM account.
There are several ways to do this: One is to use the command-line
utility [PsExec](https://docs.microsoft.com/en-us/sysinternals/downloads/psexec),
from Microsoft's Sysinternals suite, which has option |-s| to start
processes as the SYSTEM account. Another alternative is to run the mount
command from a Windows Scheduled Task, or a Windows Service, configured
to run as the SYSTEM account. A third alternative is to use the
[WinFsp.Launcher infrastructure](https://github.com/billziss-gh/winfsp/wiki/WinFsp-Service-Architecture)).
Note that when running rclone as another user, it will not use
the configuration file from your profile unless you tell it to
with the [|--config|](https://rclone.org/docs/#config-config-file) option.
Read more in the [install documentation](https://rclone.org/install/).

Note that mapping to a directory path, instead of a drive letter,
does not suffer from the same limitations.

### Limitations

Without the use of |--vfs-cache-mode| this can only write files
sequentially, it can only seek when reading.  This means that many
applications won't work with their files on an rclone mount without
|--vfs-cache-mode writes| or |--vfs-cache-mode full|.
See the [VFS File Caching](#vfs-file-caching) section for more info.

The bucket based remotes (e.g. Swift, S3, Google Compute Storage, B2,
Hubic) do not support the concept of empty directories, so empty
directories will have a tendency to disappear once they fall out of
the directory cache.

Only supported on Linux, FreeBSD, OS X and Windows at the moment.

### rclone @ vs rclone sync/copy

File systems expect things to be 100% reliable, whereas cloud storage
systems are a long way from 100% reliable. The rclone sync/copy
commands cope with this with lots of retries.  However rclone @
can't use retries in the same way without making local copies of the
uploads. Look at the [VFS File Caching](#vfs-file-caching)
for solutions to make @ more reliable.

### Attribute caching

You can use the flag |--attr-timeout| to set the time the kernel caches
the attributes (size, modification time, etc.) for directory entries.

The default is |1s| which caches files just long enough to avoid
too many callbacks to rclone from the kernel.

In theory 0s should be the correct value for filesystems which can
change outside the control of the kernel. However this causes quite a
few problems such as
[rclone using too much memory](https://github.com/rclone/rclone/issues/2157),
[rclone not serving files to samba](https://forum.rclone.org/t/rclone-1-39-vs-1-40-mount-issue/5112)
and [excessive time listing directories](https://github.com/rclone/rclone/issues/2095#issuecomment-371141147).

The kernel can cache the info about a file for the time given by
|--attr-timeout|. You may see corruption if the remote file changes
length during this window.  It will show up as either a truncated file
or a file with garbage on the end.  With |--attr-timeout 1s| this is
very unlikely but not impossible.  The higher you set |--attr-timeout|
the more likely it is.  The default setting of "1s" is the lowest
setting which mitigates the problems above.

If you set it higher (|10s| or |1m| say) then the kernel will call
back to rclone less often making it more efficient, however there is
more chance of the corruption issue above.

If files don't change on the remote outside of the control of rclone
then there is no chance of corruption.

This is the same as setting the attr_timeout option in mount.fuse.

### Filters

Note that all the rclone filters can be used to select a subset of the
files to be visible in the mount.

### systemd

When running rclone @ as a systemd service, it is possible
to use Type=notify. In this case the service will enter the started state
after the mountpoint has been successfully set up.
Units having the rclone @ service specified as a requirement
will see all files and folders immediately in this mode.

### chunked reading

|--vfs-read-chunk-size| will enable reading the source objects in parts.
This can reduce the used download quota for some remotes by requesting only chunks
from the remote that are actually read at the cost of an increased number of requests.

When |--vfs-read-chunk-size-limit| is also specified and greater than
|--vfs-read-chunk-size|, the chunk size for each open file will get doubled
for each chunk read, until the specified value is reached. A value of |-1| will disable
the limit and the chunk size will grow indefinitely.

With |--vfs-read-chunk-size 100M| and |--vfs-read-chunk-size-limit 0|
the following parts will be downloaded: 0-100M, 100M-200M, 200M-300M, 300M-400M and so on.
When |--vfs-read-chunk-size-limit 500M| is specified, the result would be
0-100M, 100M-300M, 300M-700M, 700M-1200M, 1200M-1700M and so on.
`, "|", "`"), "@", commandName) + vfs.Help,
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

			// Inform about ignored flags on Windows,
			// and if not on Windows and not --allow-non-empty flag is used
			// verify that mountpoint is empty.
			if runtime.GOOS == "windows" {
				if opt.AllowNonEmpty {
					fs.Logf(nil, "--allow-non-empty flag does nothing on Windows")
				}
				if opt.AllowRoot {
					fs.Logf(nil, "--allow-root flag does nothing on Windows")
				}
				if opt.AllowOther {
					fs.Logf(nil, "--allow-other flag does nothing on Windows")
				}
			} else if !opt.AllowNonEmpty {
				err := checkMountEmpty(mountpoint)
				if err != nil {
					log.Fatalf("Fatal error: %v", err)
				}
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
	var finaliseOnce sync.Once
	finalise := func() {
		finaliseOnce.Do(func() {
			_ = sysdnotify.Stopping()
			_ = unmount()
		})
	}
	fnHandle := atexit.Register(finalise)
	defer atexit.Unregister(fnHandle)

	// Notify systemd
	if err := sysdnotify.Ready(); err != nil {
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

	finalise()

	if err != nil {
		return errors.Wrap(err, "failed to umount FUSE fs")
	}

	return nil
}
