// Package mount implents a FUSE mounting system for rclone remotes.

// +build linux darwin freebsd

package mount

import (
	"bazil.org/fuse"
	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/fs"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// Globals
var (
	noModTime = false
	debugFUSE = false
)

func init() {
	cmd.Root.AddCommand(mountCmd)
	mountCmd.Flags().BoolVarP(&noModTime, "no-modtime", "", false, "Don't read the modification time (can speed things up).")
	mountCmd.Flags().BoolVarP(&debugFUSE, "debug-fuse", "", false, "Debug the FUSE internals - needs -v.")
}

var mountCmd = &cobra.Command{
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

This can only read files seqentially, or write files sequentially.  It
can't read and write or seek in files.

rclonefs inherits rclone's directory handling.  In rclone's world
directories don't really exist.  This means that empty directories
will have a tendency to disappear once they fall out of the directory
cache.

The bucket based FSes (eg swift, s3, google compute storage, b2) won't
work from the root - you will need to specify a bucket, or a path
within the bucket.  So ` + "`swift:`" + ` won't work whereas ` + "`swift:bucket`" + ` will
as will ` + "`swift:bucket/path`" + `.

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

### TODO ###

  * Check hashes on upload/download
  * Preserve timestamps
  * Move directories
`,
	RunE: func(command *cobra.Command, args []string) error {
		cmd.CheckArgs(2, 2, command, args)
		fdst := cmd.NewFsDst(args)
		return Mount(fdst, args[1])
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
