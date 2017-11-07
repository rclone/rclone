// Package vfsflags implements command line flags to set up a vfs
package vfsflags

import (
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/vfs"
	"github.com/spf13/pflag"
)

// Options set by command line flags
var (
	Opt = vfs.DefaultOpt
)

// AddFlags adds the non filing system specific flags to the command
func AddFlags(flags *pflag.FlagSet) {
	fs.BoolVarP(flags, &Opt.NoModTime, "no-modtime", "", Opt.NoModTime, "Don't read/write the modification time (can speed things up).")
	fs.BoolVarP(flags, &Opt.NoChecksum, "no-checksum", "", Opt.NoChecksum, "Don't compare checksums on up/download.")
	fs.BoolVarP(flags, &Opt.NoSeek, "no-seek", "", Opt.NoSeek, "Don't allow seeking in files.")
	fs.DurationVarP(flags, &Opt.DirCacheTime, "dir-cache-time", "", Opt.DirCacheTime, "Time to cache directory entries for.")
	fs.DurationVarP(flags, &Opt.PollInterval, "poll-interval", "", Opt.PollInterval, "Time to wait between polling for changes. Must be smaller than dir-cache-time. Only on supported remotes. Set to 0 to disable.")
	fs.BoolVarP(flags, &Opt.ReadOnly, "read-only", "", Opt.ReadOnly, "Mount read-only.")
	fs.FlagsVarP(flags, &Opt.CacheMode, "cache-mode", "", "Cache mode off|minimal|writes|full")
	fs.DurationVarP(flags, &Opt.CachePollInterval, "cache-poll-interval", "", Opt.CachePollInterval, "Interval to poll the cache for stale objects.")
	fs.DurationVarP(flags, &Opt.CacheMaxAge, "cache-max-age", "", Opt.CacheMaxAge, "Max age of objects in the cache.")
	platformFlags(flags)
}
