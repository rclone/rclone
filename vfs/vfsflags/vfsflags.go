// Package vfsflags implements command line flags to set up a vfs
package vfsflags

import (
	"github.com/ncw/rclone/vfs"
	"github.com/spf13/pflag"
)

// Options set by command line flags
var (
	Opt = vfs.DefaultOpt
)

// AddFlags adds the non filing system specific flags to the command
func AddFlags(flags *pflag.FlagSet) {
	flags.BoolVarP(&Opt.NoModTime, "no-modtime", "", Opt.NoModTime, "Don't read/write the modification time (can speed things up).")
	flags.BoolVarP(&Opt.NoChecksum, "no-checksum", "", Opt.NoChecksum, "Don't compare checksums on up/download.")
	flags.BoolVarP(&Opt.NoSeek, "no-seek", "", Opt.NoSeek, "Don't allow seeking in files.")
	flags.DurationVarP(&Opt.DirCacheTime, "dir-cache-time", "", Opt.DirCacheTime, "Time to cache directory entries for.")
	flags.DurationVarP(&Opt.PollInterval, "poll-interval", "", Opt.PollInterval, "Time to wait between polling for changes. Must be smaller than dir-cache-time. Only on supported remotes. Set to 0 to disable.")
	flags.BoolVarP(&Opt.ReadOnly, "read-only", "", Opt.ReadOnly, "Mount read-only.")
	flags.VarP(&Opt.CacheMode, "cache-mode", "", "Cache mode off|minimal|writes|full")
	flags.DurationVarP(&Opt.CachePollInterval, "cache-poll-interval", "", Opt.CachePollInterval, "Interval to poll the cache for stale objects.")
	flags.DurationVarP(&Opt.CacheMaxAge, "cache-max-age", "", Opt.CacheMaxAge, "Max age of objects in the cache.")
	platformFlags(flags)
}
