package vfscommon

import (
	"os"
	"runtime"
	"time"

	"github.com/rclone/rclone/fs"
)

// OptionsInfo describes the Options in use
var OptionsInfo = fs.Options{{
	Name:    "no_modtime",
	Default: false,
	Help:    "Don't read/write the modification time (can speed things up)",
	Groups:  "VFS",
}, {
	Name:    "no_checksum",
	Default: false,
	Help:    "Don't compare checksums on up/download",
	Groups:  "VFS",
}, {
	Name:    "no_seek",
	Default: false,
	Help:    "Don't allow seeking in files",
	Groups:  "VFS",
}, {
	Name:    "dir_cache_time",
	Default: fs.Duration(5 * 60 * time.Second),
	Help:    "Time to cache directory entries for",
	Groups:  "VFS",
}, {
	Name:    "vfs_refresh",
	Default: false,
	Help:    "Refreshes the directory cache recursively in the background on start",
	Groups:  "VFS",
}, {
	Name:    "poll_interval",
	Default: fs.Duration(time.Minute),
	Help:    "Time to wait between polling for changes, must be smaller than dir-cache-time and only on supported remotes (set 0 to disable)",
	Groups:  "VFS",
}, {
	Name:    "read_only",
	Default: false,
	Help:    "Only allow read-only access",
	Groups:  "VFS",
}, {
	Name:    "vfs_cache_mode",
	Default: CacheModeOff,
	Help:    "Cache mode off|minimal|writes|full",
	Groups:  "VFS",
}, {
	Name:    "vfs_cache_poll_interval",
	Default: fs.Duration(60 * time.Second),
	Help:    "Interval to poll the cache for stale objects",
	Groups:  "VFS",
}, {
	Name:    "vfs_cache_max_age",
	Default: fs.Duration(3600 * time.Second),
	Help:    "Max time since last access of objects in the cache",
	Groups:  "VFS",
}, {
	Name:    "vfs_cache_max_size",
	Default: fs.SizeSuffix(-1),
	Help:    "Max total size of objects in the cache",
	Groups:  "VFS",
}, {
	Name:    "vfs_cache_min_free_space",
	Default: fs.SizeSuffix(-1),
	Help:    "Target minimum free space on the disk containing the cache",
	Groups:  "VFS",
}, {
	Name:    "vfs_read_chunk_size",
	Default: 128 * fs.Mebi,
	Help:    "Read the source objects in chunks",
	Groups:  "VFS",
}, {
	Name:    "vfs_read_chunk_size_limit",
	Default: fs.SizeSuffix(-1),
	Help:    "If greater than --vfs-read-chunk-size, double the chunk size after each chunk read, until the limit is reached ('off' is unlimited)",
	Groups:  "VFS",
}, {
	Name:    "vfs_read_chunk_streams",
	Default: 0,
	Help:    "The number of parallel streams to read at once",
	Groups:  "VFS",
}, {
	Name:    "dir_perms",
	Default: FileMode(0777),
	Help:    "Directory permissions",
	Groups:  "VFS",
}, {
	Name:    "file_perms",
	Default: FileMode(0666),
	Help:    "File permissions",
	Groups:  "VFS",
}, {
	Name:    "vfs_case_insensitive",
	Default: runtime.GOOS == "windows" || runtime.GOOS == "darwin", // default to true on Windows and Mac, false otherwise,
	Help:    "If a file name not found, find a case insensitive match",
	Groups:  "VFS",
}, {
	Name:    "vfs_block_norm_dupes",
	Default: false,
	Help:    "If duplicate filenames exist in the same directory (after normalization), log an error and hide the duplicates (may have a performance cost)",
	Groups:  "VFS",
}, {
	Name:    "vfs_write_wait",
	Default: fs.Duration(1000 * time.Millisecond),
	Help:    "Time to wait for in-sequence write before giving error",
	Groups:  "VFS",
}, {
	Name:    "vfs_read_wait",
	Default: fs.Duration(20 * time.Millisecond),
	Help:    "Time to wait for in-sequence read before seeking",
	Groups:  "VFS",
}, {
	Name:    "vfs_write_back",
	Default: fs.Duration(5 * time.Second),
	Help:    "Time to writeback files after last use when using cache",
	Groups:  "VFS",
}, {
	Name:    "vfs_read_ahead",
	Default: 0 * fs.Mebi,
	Help:    "Extra read ahead over --buffer-size when using cache-mode full",
	Groups:  "VFS",
}, {
	Name:    "vfs_used_is_size",
	Default: false,
	Help:    "Use the `rclone size` algorithm for Used size",
	Groups:  "VFS",
}, {
	Name:    "vfs_fast_fingerprint",
	Default: false,
	Help:    "Use fast (less accurate) fingerprints for change detection",
	Groups:  "VFS",
}, {
	Name:    "vfs_disk_space_total_size",
	Default: fs.SizeSuffix(-1),
	Help:    "Specify the total space of disk",
	Groups:  "VFS",
}, {
	Name:    "umask",
	Default: FileMode(getUmask()),
	Help:    "Override the permission bits set by the filesystem (not supported on Windows)",
	Groups:  "VFS",
}, {
	Name:    "uid",
	Default: getUID(),
	Help:    "Override the uid field set by the filesystem (not supported on Windows)",
	Groups:  "VFS",
}, {
	Name:    "gid",
	Default: getGID(),
	Help:    "Override the gid field set by the filesystem (not supported on Windows)",
	Groups:  "VFS",
}}

func init() {
	fs.RegisterGlobalOptions(fs.OptionsInfo{Name: "vfs", Opt: &Opt, Options: OptionsInfo})
}

// Options is options for creating the vfs
type Options struct {
	NoSeek             bool          `config:"no_seek"`        // don't allow seeking if set
	NoChecksum         bool          `config:"no_checksum"`    // don't check checksums if set
	ReadOnly           bool          `config:"read_only"`      // if set VFS is read only
	NoModTime          bool          `config:"no_modtime"`     // don't read mod times for files
	DirCacheTime       fs.Duration   `config:"dir_cache_time"` // how long to consider directory listing cache valid
	Refresh            bool          `config:"vfs_refresh"`    // refreshes the directory listing recursively on start
	PollInterval       fs.Duration   `config:"poll_interval"`
	Umask              FileMode      `config:"umask"`
	UID                uint32        `config:"uid"`
	GID                uint32        `config:"gid"`
	DirPerms           FileMode      `config:"dir_perms"`
	FilePerms          FileMode      `config:"file_perms"`
	ChunkSize          fs.SizeSuffix `config:"vfs_read_chunk_size"`       // if > 0 read files in chunks
	ChunkSizeLimit     fs.SizeSuffix `config:"vfs_read_chunk_size_limit"` // if > ChunkSize double the chunk size after each chunk until reached
	ChunkStreams       int           `config:"vfs_read_chunk_streams"`    // Number of download streams to use
	CacheMode          CacheMode     `config:"vfs_cache_mode"`
	CacheMaxAge        fs.Duration   `config:"vfs_cache_max_age"`
	CacheMaxSize       fs.SizeSuffix `config:"vfs_cache_max_size"`
	CacheMinFreeSpace  fs.SizeSuffix `config:"vfs_cache_min_free_space"`
	CachePollInterval  fs.Duration   `config:"vfs_cache_poll_interval"`
	CaseInsensitive    bool          `config:"vfs_case_insensitive"`
	BlockNormDupes     bool          `config:"vfs_block_norm_dupes"`
	WriteWait          fs.Duration   `config:"vfs_write_wait"`       // time to wait for in-sequence write
	ReadWait           fs.Duration   `config:"vfs_read_wait"`        // time to wait for in-sequence read
	WriteBack          fs.Duration   `config:"vfs_write_back"`       // time to wait before writing back dirty files
	ReadAhead          fs.SizeSuffix `config:"vfs_read_ahead"`       // bytes to read ahead in cache mode "full"
	UsedIsSize         bool          `config:"vfs_used_is_size"`     // if true, use the `rclone size` algorithm for Used size
	FastFingerprint    bool          `config:"vfs_fast_fingerprint"` // if set use fast fingerprints
	DiskSpaceTotalSize fs.SizeSuffix `config:"vfs_disk_space_total_size"`
}

// Opt is the default options modified by the environment variables and command line flags
var Opt Options

// Init the options, making sure everything is within range
func (opt *Options) Init() {
	// Mask the permissions with the umask
	opt.DirPerms &= ^opt.Umask
	opt.FilePerms &= ^opt.Umask

	// Make sure directories are returned as directories
	opt.DirPerms |= FileMode(os.ModeDir)
}
