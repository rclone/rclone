// Package mount implents a FUSE mounting system for rclone remotes.

// +build linux darwin freebsd

package mount

import (
	"log"
	"os"
	"time"

	"bazil.org/fuse"
	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/fs"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
)

// Globals
var (
	noModTime    = false
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
	commandDefintion.Flags().BoolVarP(&noModTime, "no-modtime", "", noModTime, "Don't read the modification time (can speed things up).")
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

    rclone mount remote:path/to/files /path/to/local/mount &

Stop the mount with

    fusermount -u /path/to/local/mount

Or with OS X

    umount -u /path/to/local/mount

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

### Bugs ###

  * All the remotes should work for read, but some may not for write
    * those which need to know the size in advance won't - eg B2
    * maybe should pass in size as -1 to mean work it out
    * Or put in an an upload cache to cache the files on disk first

### TODO ###

  * Check hashes on upload/download
  * Preserve timestamps
  * Move directories
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

// Mount mounts the remote at mountpoint.
//
// If noModTime is set then it
func Mount(f fs.Fs, mountpoint string) error {
	if debugFUSE {
		fuse.Debug = func(msg interface{}) {
			fs.Debug("fuse", "%v", msg)
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
	errChan, err := mount(f, mountpoint)
	if err != nil {
		return errors.Wrap(err, "failed to mount FUSE fs")
	}

	// Wait for umount
	err = <-errChan
	if err != nil {
		return errors.Wrap(err, "failed to umount FUSE fs")
	}

	return nil
}
