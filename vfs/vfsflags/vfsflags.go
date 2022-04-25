// Package vfsflags implements command line flags to set up a vfs
package vfsflags

import (
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/spf13/pflag"
)

// Options set by command line flags
var (
	Opt       = vfscommon.DefaultOpt
	DirPerms  = &FileMode{Mode: &Opt.DirPerms}
	FilePerms = &FileMode{Mode: &Opt.FilePerms}
)

// AddFlags adds the non filing system specific flags to the command
func AddFlags(flagSet *pflag.FlagSet) {
	rc.AddOption("vfs", &Opt)
	flags.BoolVarP(flagSet, &Opt.NoModTime, "no-modtime", "", Opt.NoModTime, "Don't read/write the modification time (can speed things up)")
	flags.BoolVarP(flagSet, &Opt.NoChecksum, "no-checksum", "", Opt.NoChecksum, "Don't compare checksums on up/download")
	flags.BoolVarP(flagSet, &Opt.NoSeek, "no-seek", "", Opt.NoSeek, "Don't allow seeking in files")
	flags.DurationVarP(flagSet, &Opt.DirCacheTime, "dir-cache-time", "", Opt.DirCacheTime, "Time to cache directory entries for")
	flags.DurationVarP(flagSet, &Opt.PollInterval, "poll-interval", "", Opt.PollInterval, "Time to wait between polling for changes, must be smaller than dir-cache-time and only on supported remotes (set 0 to disable)")
	flags.BoolVarP(flagSet, &Opt.ReadOnly, "read-only", "", Opt.ReadOnly, "Only allow read-only access")
	flags.FVarP(flagSet, &Opt.CacheMode, "vfs-cache-mode", "", "Cache mode off|minimal|writes|full")
	flags.DurationVarP(flagSet, &Opt.CachePollInterval, "vfs-cache-poll-interval", "", Opt.CachePollInterval, "Interval to poll the cache for stale objects")
	flags.DurationVarP(flagSet, &Opt.CacheMaxAge, "vfs-cache-max-age", "", Opt.CacheMaxAge, "Max age of objects in the cache")
	flags.FVarP(flagSet, &Opt.CacheMaxSize, "vfs-cache-max-size", "", "Max total size of objects in the cache")
	flags.FVarP(flagSet, &Opt.ChunkSize, "vfs-read-chunk-size", "", "Read the source objects in chunks")
	flags.FVarP(flagSet, &Opt.ChunkSizeLimit, "vfs-read-chunk-size-limit", "", "If greater than --vfs-read-chunk-size, double the chunk size after each chunk read, until the limit is reached ('off' is unlimited)")
	flags.FVarP(flagSet, DirPerms, "dir-perms", "", "Directory permissions")
	flags.FVarP(flagSet, FilePerms, "file-perms", "", "File permissions")
	flags.BoolVarP(flagSet, &Opt.CaseInsensitive, "vfs-case-insensitive", "", Opt.CaseInsensitive, "If a file name not found, find a case insensitive match")
	flags.DurationVarP(flagSet, &Opt.WriteWait, "vfs-write-wait", "", Opt.WriteWait, "Time to wait for in-sequence write before giving error")
	flags.DurationVarP(flagSet, &Opt.ReadWait, "vfs-read-wait", "", Opt.ReadWait, "Time to wait for in-sequence read before seeking")
	flags.DurationVarP(flagSet, &Opt.WriteBack, "vfs-write-back", "", Opt.WriteBack, "Time to writeback files after last use when using cache")
	flags.FVarP(flagSet, &Opt.ReadAhead, "vfs-read-ahead", "", "Extra read ahead over --buffer-size when using cache-mode full")
	flags.BoolVarP(flagSet, &Opt.UsedIsSize, "vfs-used-is-size", "", Opt.UsedIsSize, "Use the `rclone size` algorithm for Used size")
	flags.BoolVarP(flagSet, &Opt.FastFingerprint, "vfs-fast-fingerprint", "", Opt.FastFingerprint, "Use fast (less accurate) fingerprints for change detection")
	platformFlags(flagSet)
}
