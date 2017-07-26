package mountlib

// Globals
import (
	"log"
	"os"
	"time"

	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/fs"
	"github.com/spf13/cobra"
)

// Options set by command line flags
var (
	NoModTime    = false
	NoChecksum   = false
	DebugFUSE    = false
	NoSeek       = false
	DirCacheTime = 5 * 60 * time.Second
	PollInterval = time.Minute
	// mount options
	ReadOnly                         = false
	AllowNonEmpty                    = false
	AllowRoot                        = false
	AllowOther                       = false
	DefaultPermissions               = false
	WritebackCache                   = false
	MaxReadAhead       fs.SizeSuffix = 128 * 1024
	Umask                            = 0
	UID                              = ^uint32(0) // these values instruct WinFSP-FUSE to use the current user
	GID                              = ^uint32(0) // overriden for non windows in mount_unix.go
	// foreground                 = false
	// default permissions for directories - modified by umask in Mount
	DirPerms     = os.FileMode(0777)
	FilePerms    = os.FileMode(0666)
	ExtraOptions *[]string
	ExtraFlags   *[]string
)

// NewMountCommand makes a mount command with the given name and Mount function
func NewMountCommand(commandName string, Mount func(f fs.Fs, mountpoint string) error) *cobra.Command {
	var commandDefintion = &cobra.Command{
		Use:   commandName + " remote:path /path/to/mountpoint",
		Short: `Mount the remote as a mountpoint. **EXPERIMENTAL**`,
		Long: `
rclone ` + commandName + ` allows Linux, FreeBSD, macOS and Windows to
mount any of Rclone's cloud storage systems as a file system with
FUSE.

This is **EXPERIMENTAL** - use with care.

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

### Installing on Windows ###

To run rclone ` + commandName + ` on Windows, you will need to
download and install [WinFsp](http://www.secfs.net/winfsp/).

WinFsp is an [open source](https://github.com/billziss-gh/winfsp)
Windows File System Proxy which makes it easy to write user space file
systems for Windows.  It provides a FUSE emulation layer which rclone
uses combination with
[cgofuse](https://github.com/billziss-gh/cgofuse).  Both of these
packages are by Bill Zissimopoulos who was very helpful during the
implementation of rclone ` + commandName + ` for Windows.

#### Windows caveats ####

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
which creates drives accessible for everyone on the system.

### Limitations ###

This can only write files seqentially, it can only seek when reading.
This means that many applications won't work with their files on an
rclone mount.

The bucket based remotes (eg Swift, S3, Google Compute Storage, B2,
Hubic) won't work from the root - you will need to specify a bucket,
or a path within the bucket.  So ` + "`swift:`" + ` won't work whereas
` + "`swift:bucket`" + ` will as will ` + "`swift:bucket/path`" + `.
None of these support the concept of directories, so empty
directories will have a tendency to disappear once they fall out of
the directory cache.

Only supported on Linux, FreeBSD, OS X and Windows at the moment.

### rclone ` + commandName + ` vs rclone sync/copy ##

File systems expect things to be 100% reliable, whereas cloud storage
systems are a long way from 100% reliable. The rclone sync/copy
commands cope with this with lots of retries.  However rclone ` + commandName + `
can't use retries in the same way without making local copies of the
uploads.  This might happen in the future, but for the moment rclone
` + commandName + ` won't do that, so will be less reliable than the rclone command.

### Filters ###

Note that all the rclone filters can be used to select a subset of the
files to be visible in the mount.

### Directory Cache ###

Using the ` + "`--dir-cache-time`" + ` flag, you can set how long a
directory should be considered up to date and not refreshed from the
backend. Changes made locally in the mount may appear immediately or
invalidate the cache. However, changes done on the remote will only
be picked up once the cache expires.

Alternatively, you can send a ` + "`SIGHUP`" + ` signal to rclone for
it to flush all directory caches, regardless of how old they are.
Assuming only one rclone instance is running, you can reset the cache
like this:

    kill -SIGHUP $(pidof rclone)

### Bugs ###

  * All the remotes should work for read, but some may not for write
    * those which need to know the size in advance won't - eg B2
    * maybe should pass in size as -1 to mean work it out
    * Or put in an an upload cache to cache the files on disk first
`,
		Run: func(command *cobra.Command, args []string) {
			cmd.CheckArgs(2, 2, command, args)
			fdst := cmd.NewFsDst(args)

			// Mask permissions
			DirPerms = 0777 &^ os.FileMode(Umask)
			FilePerms = 0666 &^ os.FileMode(Umask)

			// Show stats if the user has specifically requested them
			if cmd.ShowStats() {
				stopStats := cmd.StartStats()
				defer close(stopStats)
			}

			err := Mount(fdst, args[1])
			if err != nil {
				log.Fatalf("Fatal error: %v", err)
			}
		},
	}

	// Register the command
	cmd.Root.AddCommand(commandDefintion)

	// Add flags
	flags := commandDefintion.Flags()
	flags.BoolVarP(&NoModTime, "no-modtime", "", NoModTime, "Don't read/write the modification time (can speed things up).")
	flags.BoolVarP(&NoChecksum, "no-checksum", "", NoChecksum, "Don't compare checksums on up/download.")
	flags.BoolVarP(&DebugFUSE, "debug-fuse", "", DebugFUSE, "Debug the FUSE internals - needs -v.")
	flags.BoolVarP(&NoSeek, "no-seek", "", NoSeek, "Don't allow seeking in files.")
	flags.DurationVarP(&DirCacheTime, "dir-cache-time", "", DirCacheTime, "Time to cache directory entries for.")
	flags.DurationVarP(&PollInterval, "poll-interval", "", PollInterval, "Time to wait between polling for changes. Must be smaller than dir-cache-time. Only on supported remotes. Set to 0 to disable.")
	// mount options
	flags.BoolVarP(&ReadOnly, "read-only", "", ReadOnly, "Mount read-only.")
	flags.BoolVarP(&AllowNonEmpty, "allow-non-empty", "", AllowNonEmpty, "Allow mounting over a non-empty directory.")
	flags.BoolVarP(&AllowRoot, "allow-root", "", AllowRoot, "Allow access to root user.")
	flags.BoolVarP(&AllowOther, "allow-other", "", AllowOther, "Allow access to other users.")
	flags.BoolVarP(&DefaultPermissions, "default-permissions", "", DefaultPermissions, "Makes kernel enforce access control based on the file mode.")
	flags.BoolVarP(&WritebackCache, "write-back-cache", "", WritebackCache, "Makes kernel buffer writes before sending them to rclone. Without this, writethrough caching is used.")
	flags.VarP(&MaxReadAhead, "max-read-ahead", "", "The number of bytes that can be prefetched for sequential reads.")
	ExtraOptions = flags.StringArrayP("option", "o", []string{}, "Option for libfuse/WinFsp. Repeat if required.")
	ExtraFlags = flags.StringArrayP("fuse-flag", "", []string{}, "Flags or arguments to be passed direct to libfuse/WinFsp. Repeat if required.")
	//flags.BoolVarP(&foreground, "foreground", "", foreground, "Do not detach.")

	platformFlags(flags)
	return commandDefintion
}
