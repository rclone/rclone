// Package mount implents a FUSE mounting system for rclone remotes.

// +build linux darwin freebsd

package mount

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"bazil.org/fuse"
	fusefs "bazil.org/fuse/fs"
	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/cmd/mountlib"
	"github.com/ncw/rclone/fs"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
)

// Globals
var (
	noModTime    = false
	noChecksum   = false
	debugFUSE    = false
	noSeek       = false
	dirCacheTime = 5 * 60 * time.Second
	// mount options
	readOnly                         = false
	allowNonEmpty                    = false
	allowRoot                        = false
	allowOther                       = false
	defaultPermissions               = false
	writebackCache                   = false
	maxReadAhead       fs.SizeSuffix = 128 * 1024
	umask                            = 0
	uid                              = uint32(unix.Geteuid())
	gid                              = uint32(unix.Getegid())
	// foreground                 = false
	// default permissions for directories - modified by umask in Mount
	dirPerms  = os.FileMode(0777)
	filePerms = os.FileMode(0666)
)

func init() {
	umask = unix.Umask(0) // read the umask
	unix.Umask(umask)     // set it back to what it was
	cmd.Root.AddCommand(commandDefintion)
	commandDefintion.Flags().BoolVarP(&noModTime, "no-modtime", "", noModTime, "Don't read/write the modification time (can speed things up).")
	commandDefintion.Flags().BoolVarP(&noChecksum, "no-checksum", "", noChecksum, "Don't compare checksums on up/download.")
	commandDefintion.Flags().BoolVarP(&debugFUSE, "debug-fuse", "", debugFUSE, "Debug the FUSE internals - needs -v.")
	commandDefintion.Flags().BoolVarP(&noSeek, "no-seek", "", noSeek, "Don't allow seeking in files.")
	commandDefintion.Flags().DurationVarP(&dirCacheTime, "dir-cache-time", "", dirCacheTime, "Time to cache directory entries for.")
	// mount options
	commandDefintion.Flags().BoolVarP(&readOnly, "read-only", "", readOnly, "Mount read-only.")
	commandDefintion.Flags().BoolVarP(&allowNonEmpty, "allow-non-empty", "", allowNonEmpty, "Allow mounting over a non-empty directory.")
	commandDefintion.Flags().BoolVarP(&allowRoot, "allow-root", "", allowRoot, "Allow access to root user.")
	commandDefintion.Flags().BoolVarP(&allowOther, "allow-other", "", allowOther, "Allow access to other users.")
	commandDefintion.Flags().BoolVarP(&defaultPermissions, "default-permissions", "", defaultPermissions, "Makes kernel enforce access control based on the file mode.")
	commandDefintion.Flags().BoolVarP(&writebackCache, "write-back-cache", "", writebackCache, "Makes kernel buffer writes before sending them to rclone. Without this, writethrough caching is used.")
	commandDefintion.Flags().VarP(&maxReadAhead, "max-read-ahead", "", "The number of bytes that can be prefetched for sequential reads.")
	commandDefintion.Flags().IntVarP(&umask, "umask", "", umask, "Override the permission bits set by the filesystem.")
	commandDefintion.Flags().Uint32VarP(&uid, "uid", "", uid, "Override the uid field set by the filesystem.")
	commandDefintion.Flags().Uint32VarP(&gid, "gid", "", gid, "Override the gid field set by the filesystem.")
	//commandDefintion.Flags().BoolVarP(&foreground, "foreground", "", foreground, "Do not detach.")
}

var commandDefintion = &cobra.Command{
	Use:   "mount remote:path /path/to/mountpoint",
	Short: `Mount the remote as a mountpoint. **EXPERIMENTAL**`,
	Long: `
rclone mount allows Linux, FreeBSD and macOS to mount any of Rclone's
cloud storage systems as a file system with FUSE.

This is **EXPERIMENTAL** - use with care.

First set up your remote using ` + "`rclone config`" + `.  Check it works with ` + "`rclone ls`" + ` etc.

Start the mount like this

    rclone mount remote:path/to/files /path/to/local/mount

When the program ends, either via Ctrl+C or receiving a SIGINT or SIGTERM signal,
the mount is automatically stopped.

The umount operation can fail, for example when the mountpoint is busy.
When that happens, it is the user's responsibility to stop the mount manually with

    # Linux
    fusermount -u /path/to/local/mount
    # OS X
    umount /path/to/local/mount

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

Only supported on Linux, FreeBSD and OS X at the moment.

### rclone mount vs rclone sync/copy ##

File systems expect things to be 100% reliable, whereas cloud storage
systems are a long way from 100% reliable. The rclone sync/copy
commands cope with this with lots of retries.  However rclone mount
can't use retries in the same way without making local copies of the
uploads.  This might happen in the future, but for the moment rclone
mount won't do that, so will be less reliable than the rclone command.

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
Assuming only one rlcone instance is running, you can reset the cache
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
		err := Mount(fdst, args[1])
		if err != nil {
			log.Fatalf("Fatal error: %v", err)
		}
	},
}

// mountOptions configures the options from the command line flags
func mountOptions(device string) (options []fuse.MountOption) {
	options = []fuse.MountOption{
		fuse.MaxReadahead(uint32(maxReadAhead)),
		fuse.Subtype("rclone"),
		fuse.FSName(device), fuse.VolumeName(device),
		fuse.NoAppleDouble(),
		fuse.NoAppleXattr(),

		// Options from benchmarking in the fuse module
		//fuse.MaxReadahead(64 * 1024 * 1024),
		//fuse.AsyncRead(), - FIXME this causes
		// ReadFileHandle.Read error: read /home/files/ISOs/xubuntu-15.10-desktop-amd64.iso: bad file descriptor
		// which is probably related to errors people are having
		//fuse.WritebackCache(),
	}
	if allowNonEmpty {
		options = append(options, fuse.AllowNonEmptyMount())
	}
	if allowOther {
		options = append(options, fuse.AllowOther())
	}
	if allowRoot {
		options = append(options, fuse.AllowRoot())
	}
	if defaultPermissions {
		options = append(options, fuse.DefaultPermissions())
	}
	if readOnly {
		options = append(options, fuse.ReadOnly())
	}
	if writebackCache {
		options = append(options, fuse.WritebackCache())
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
	c, err := fuse.Mount(mountpoint, mountOptions(f.Name()+":"+f.Root())...)
	if err != nil {
		return nil, nil, nil, err
	}

	filesys := NewFS(f)
	server := fusefs.New(c, nil)

	// Serve the mount point in the background returning error to errChan
	errChan := make(chan error, 1)
	go func() {
		err := server.Serve(filesys)
		closeErr := c.Close()
		if err == nil {
			err = closeErr
		}
		errChan <- err
	}()

	// check if the mount process has an error to report
	<-c.Ready
	if err := c.MountError; err != nil {
		return nil, nil, nil, err
	}

	unmount := func() error {
		return fuse.Unmount(mountpoint)
	}

	return filesys.FS, errChan, unmount, nil
}

// Mount mounts the remote at mountpoint.
//
// If noModTime is set then it
func Mount(f fs.Fs, mountpoint string) error {
	if debugFUSE {
		fuse.Debug = func(msg interface{}) {
			fs.Debugf("fuse", "%v", msg)
		}
	}

	// Set permissions
	dirPerms = 0777 &^ os.FileMode(umask)
	filePerms = 0666 &^ os.FileMode(umask)

	// Show stats if the user has specifically requested them
	if cmd.ShowStats() {
		stopStats := cmd.StartStats()
		defer close(stopStats)
	}

	// Mount it
	FS, errChan, unmount, err := mount(f, mountpoint)
	if err != nil {
		return errors.Wrap(err, "failed to mount FUSE fs")
	}

	sigInt := make(chan os.Signal, 1)
	signal.Notify(sigInt, syscall.SIGINT, syscall.SIGTERM)
	sigHup := make(chan os.Signal, 1)
	signal.Notify(sigHup, syscall.SIGHUP)

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
