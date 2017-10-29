// Package vfsflags implements command line flags to set up a vfs
package vfsflags

import (
	"os"
	"time"

	"github.com/ncw/rclone/vfs"
	"github.com/spf13/pflag"
)

// Options set by command line flags
var (
	Opt = vfs.Options{
		NoModTime:    false,
		NoChecksum:   false,
		NoSeek:       false,
		DirCacheTime: 5 * 60 * time.Second,
		PollInterval: time.Minute,
		ReadOnly:     false,
		Umask:        0,
		UID:          ^uint32(0), // these values instruct WinFSP-FUSE to use the current user
		GID:          ^uint32(0), // overriden for non windows in mount_unix.go
		DirPerms:     os.FileMode(0777),
		FilePerms:    os.FileMode(0666),
	}
)

// AddFlags adds the non filing system specific flags to the command
func AddFlags(flags *pflag.FlagSet) {
	flags.BoolVarP(&Opt.NoModTime, "no-modtime", "", Opt.NoModTime, "Don't read/write the modification time (can speed things up).")
	flags.BoolVarP(&Opt.NoChecksum, "no-checksum", "", Opt.NoChecksum, "Don't compare checksums on up/download.")
	flags.BoolVarP(&Opt.NoSeek, "no-seek", "", Opt.NoSeek, "Don't allow seeking in files.")
	flags.DurationVarP(&Opt.DirCacheTime, "dir-cache-time", "", Opt.DirCacheTime, "Time to cache directory entries for.")
	flags.DurationVarP(&Opt.PollInterval, "poll-interval", "", Opt.PollInterval, "Time to wait between polling for changes. Must be smaller than dir-cache-time. Only on supported remotes. Set to 0 to disable.")
	flags.BoolVarP(&Opt.ReadOnly, "read-only", "", Opt.ReadOnly, "Mount read-only.")
	platformFlags(flags)
}
