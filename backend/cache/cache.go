// +build !plan9,!js

package cache

import (
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/backend/crypt"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fspath"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/fs/walk"
	"github.com/rclone/rclone/lib/atexit"
	"golang.org/x/time/rate"
)

const (
	// DefCacheChunkSize is the default value for chunk size
	DefCacheChunkSize = fs.SizeSuffix(5 * 1024 * 1024)
	// DefCacheTotalChunkSize is the default value for the maximum size of stored chunks
	DefCacheTotalChunkSize = fs.SizeSuffix(10 * 1024 * 1024 * 1024)
	// DefCacheChunkCleanInterval is the interval at which chunks are cleaned
	DefCacheChunkCleanInterval = fs.Duration(time.Minute)
	// DefCacheInfoAge is the default value for object info age
	DefCacheInfoAge = fs.Duration(6 * time.Hour)
	// DefCacheReadRetries is the default value for read retries
	DefCacheReadRetries = 10
	// DefCacheTotalWorkers is how many workers run in parallel to download chunks
	DefCacheTotalWorkers = 4
	// DefCacheChunkNoMemory will enable or disable in-memory storage for chunks
	DefCacheChunkNoMemory = false
	// DefCacheRps limits the number of requests per second to the source FS
	DefCacheRps = -1
	// DefCacheWrites will cache file data on writes through the cache
	DefCacheWrites = false
	// DefCacheTmpWaitTime says how long should files be stored in local cache before being uploaded
	DefCacheTmpWaitTime = fs.Duration(15 * time.Second)
	// DefCacheDbWaitTime defines how long the cache backend should wait for the DB to be available
	DefCacheDbWaitTime = fs.Duration(1 * time.Second)
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "cache",
		Description: "Cache a remote",
		NewFs:       NewFs,
		CommandHelp: commandHelp,
		Options: []fs.Option{{
			Name:     "remote",
			Help:     "Remote to cache.\nNormally should contain a ':' and a path, e.g. \"myremote:path/to/dir\",\n\"myremote:bucket\" or maybe \"myremote:\" (not recommended).",
			Required: true,
		}, {
			Name: "plex_url",
			Help: "The URL of the Plex server",
		}, {
			Name: "plex_username",
			Help: "The username of the Plex user",
		}, {
			Name:       "plex_password",
			Help:       "The password of the Plex user",
			IsPassword: true,
		}, {
			Name:     "plex_token",
			Help:     "The plex token for authentication - auto set normally",
			Hide:     fs.OptionHideBoth,
			Advanced: true,
		}, {
			Name:     "plex_insecure",
			Help:     "Skip all certificate verification when connecting to the Plex server",
			Advanced: true,
		}, {
			Name: "chunk_size",
			Help: `The size of a chunk (partial file data).

Use lower numbers for slower connections. If the chunk size is
changed, any downloaded chunks will be invalid and cache-chunk-path
will need to be cleared or unexpected EOF errors will occur.`,
			Default: DefCacheChunkSize,
			Examples: []fs.OptionExample{{
				Value: "1M",
				Help:  "1 MiB",
			}, {
				Value: "5M",
				Help:  "5 MiB",
			}, {
				Value: "10M",
				Help:  "10 MiB",
			}},
		}, {
			Name: "info_age",
			Help: `How long to cache file structure information (directory listings, file size, times, etc.). 
If all write operations are done through the cache then you can safely make
this value very large as the cache store will also be updated in real time.`,
			Default: DefCacheInfoAge,
			Examples: []fs.OptionExample{{
				Value: "1h",
				Help:  "1 hour",
			}, {
				Value: "24h",
				Help:  "24 hours",
			}, {
				Value: "48h",
				Help:  "48 hours",
			}},
		}, {
			Name: "chunk_total_size",
			Help: `The total size that the chunks can take up on the local disk.

If the cache exceeds this value then it will start to delete the
oldest chunks until it goes under this value.`,
			Default: DefCacheTotalChunkSize,
			Examples: []fs.OptionExample{{
				Value: "500M",
				Help:  "500 MiB",
			}, {
				Value: "1G",
				Help:  "1 GiB",
			}, {
				Value: "10G",
				Help:  "10 GiB",
			}},
		}, {
			Name:     "db_path",
			Default:  filepath.Join(config.CacheDir, "cache-backend"),
			Help:     "Directory to store file structure metadata DB.\nThe remote name is used as the DB file name.",
			Advanced: true,
		}, {
			Name:    "chunk_path",
			Default: filepath.Join(config.CacheDir, "cache-backend"),
			Help: `Directory to cache chunk files.

Path to where partial file data (chunks) are stored locally. The remote
name is appended to the final path.

This config follows the "--cache-db-path". If you specify a custom
location for "--cache-db-path" and don't specify one for "--cache-chunk-path"
then "--cache-chunk-path" will use the same path as "--cache-db-path".`,
			Advanced: true,
		}, {
			Name:     "db_purge",
			Default:  false,
			Help:     "Clear all the cached data for this remote on start.",
			Hide:     fs.OptionHideConfigurator,
			Advanced: true,
		}, {
			Name:    "chunk_clean_interval",
			Default: DefCacheChunkCleanInterval,
			Help: `How often should the cache perform cleanups of the chunk storage.
The default value should be ok for most people. If you find that the
cache goes over "cache-chunk-total-size" too often then try to lower
this value to force it to perform cleanups more often.`,
			Advanced: true,
		}, {
			Name:    "read_retries",
			Default: DefCacheReadRetries,
			Help: `How many times to retry a read from a cache storage.

Since reading from a cache stream is independent from downloading file
data, readers can get to a point where there's no more data in the
cache.  Most of the times this can indicate a connectivity issue if
cache isn't able to provide file data anymore.

For really slow connections, increase this to a point where the stream is
able to provide data but your experience will be very stuttering.`,
			Advanced: true,
		}, {
			Name:    "workers",
			Default: DefCacheTotalWorkers,
			Help: `How many workers should run in parallel to download chunks.

Higher values will mean more parallel processing (better CPU needed)
and more concurrent requests on the cloud provider.  This impacts
several aspects like the cloud provider API limits, more stress on the
hardware that rclone runs on but it also means that streams will be
more fluid and data will be available much more faster to readers.

**Note**: If the optional Plex integration is enabled then this
setting will adapt to the type of reading performed and the value
specified here will be used as a maximum number of workers to use.`,
			Advanced: true,
		}, {
			Name:    "chunk_no_memory",
			Default: DefCacheChunkNoMemory,
			Help: `Disable the in-memory cache for storing chunks during streaming.

By default, cache will keep file data during streaming in RAM as well
to provide it to readers as fast as possible.

This transient data is evicted as soon as it is read and the number of
chunks stored doesn't exceed the number of workers. However, depending
on other settings like "cache-chunk-size" and "cache-workers" this footprint
can increase if there are parallel streams too (multiple files being read
at the same time).

If the hardware permits it, use this feature to provide an overall better
performance during streaming but it can also be disabled if RAM is not
available on the local machine.`,
			Advanced: true,
		}, {
			Name:    "rps",
			Default: int(DefCacheRps),
			Help: `Limits the number of requests per second to the source FS (-1 to disable)

This setting places a hard limit on the number of requests per second
that cache will be doing to the cloud provider remote and try to
respect that value by setting waits between reads.

If you find that you're getting banned or limited on the cloud
provider through cache and know that a smaller number of requests per
second will allow you to work with it then you can use this setting
for that.

A good balance of all the other settings should make this setting
useless but it is available to set for more special cases.

**NOTE**: This will limit the number of requests during streams but
other API calls to the cloud provider like directory listings will
still pass.`,
			Advanced: true,
		}, {
			Name:    "writes",
			Default: DefCacheWrites,
			Help: `Cache file data on writes through the FS

If you need to read files immediately after you upload them through
cache you can enable this flag to have their data stored in the
cache store at the same time during upload.`,
			Advanced: true,
		}, {
			Name:    "tmp_upload_path",
			Default: "",
			Help: `Directory to keep temporary files until they are uploaded.

This is the path where cache will use as a temporary storage for new
files that need to be uploaded to the cloud provider.

Specifying a value will enable this feature. Without it, it is
completely disabled and files will be uploaded directly to the cloud
provider`,
			Advanced: true,
		}, {
			Name:    "tmp_wait_time",
			Default: DefCacheTmpWaitTime,
			Help: `How long should files be stored in local cache before being uploaded

This is the duration that a file must wait in the temporary location
_cache-tmp-upload-path_ before it is selected for upload.

Note that only one file is uploaded at a time and it can take longer
to start the upload if a queue formed for this purpose.`,
			Advanced: true,
		}, {
			Name:    "db_wait_time",
			Default: DefCacheDbWaitTime,
			Help: `How long to wait for the DB to be available - 0 is unlimited

Only one process can have the DB open at any one time, so rclone waits
for this duration for the DB to become available before it gives an
error.

If you set it to 0 then it will wait forever.`,
			Advanced: true,
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	Remote             string        `config:"remote"`
	PlexURL            string        `config:"plex_url"`
	PlexUsername       string        `config:"plex_username"`
	PlexPassword       string        `config:"plex_password"`
	PlexToken          string        `config:"plex_token"`
	PlexInsecure       bool          `config:"plex_insecure"`
	ChunkSize          fs.SizeSuffix `config:"chunk_size"`
	InfoAge            fs.Duration   `config:"info_age"`
	ChunkTotalSize     fs.SizeSuffix `config:"chunk_total_size"`
	DbPath             string        `config:"db_path"`
	ChunkPath          string        `config:"chunk_path"`
	DbPurge            bool          `config:"db_purge"`
	ChunkCleanInterval fs.Duration   `config:"chunk_clean_interval"`
	ReadRetries        int           `config:"read_retries"`
	TotalWorkers       int           `config:"workers"`
	ChunkNoMemory      bool          `config:"chunk_no_memory"`
	Rps                int           `config:"rps"`
	StoreWrites        bool          `config:"writes"`
	TempWritePath      string        `config:"tmp_upload_path"`
	TempWaitTime       fs.Duration   `config:"tmp_wait_time"`
	DbWaitTime         fs.Duration   `config:"db_wait_time"`
}

// Fs represents a wrapped fs.Fs
type Fs struct {
	fs.Fs
	wrapper fs.Fs

	name     string
	root     string
	opt      Options      // parsed options
	features *fs.Features // optional features
	cache    *Persistent
	tempFs   fs.Fs

	lastChunkCleanup time.Time
	cleanupMu        sync.Mutex
	rateLimiter      *rate.Limiter
	plexConnector    *plexConnector
	backgroundRunner *backgroundWriter
	cleanupChan      chan bool
	parentsForgetFn  []func(string, fs.EntryType)
	notifiedRemotes  map[string]bool
	notifiedMu       sync.Mutex
	parentsForgetMu  sync.Mutex
}

// parseRootPath returns a cleaned root path and a nil error or "" and an error when the path is invalid
func parseRootPath(path string) (string, error) {
	return strings.Trim(path, "/"), nil
}

// NewFs constructs an Fs from the path, container:path
func NewFs(ctx context.Context, name, rootPath string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	if opt.ChunkTotalSize < opt.ChunkSize*fs.SizeSuffix(opt.TotalWorkers) {
		return nil, errors.Errorf("don't set cache-chunk-total-size(%v) less than cache-chunk-size(%v) * cache-workers(%v)",
			opt.ChunkTotalSize, opt.ChunkSize, opt.TotalWorkers)
	}

	if strings.HasPrefix(opt.Remote, name+":") {
		return nil, errors.New("can't point cache remote at itself - check the value of the remote setting")
	}

	rpath, err := parseRootPath(rootPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to clean root path %q", rootPath)
	}

	remotePath := fspath.JoinRootPath(opt.Remote, rootPath)
	wrappedFs, wrapErr := cache.Get(ctx, remotePath)
	if wrapErr != nil && wrapErr != fs.ErrorIsFile {
		return nil, errors.Wrapf(wrapErr, "failed to make remote %q to wrap", remotePath)
	}
	var fsErr error
	fs.Debugf(name, "wrapped %v:%v at root %v", wrappedFs.Name(), wrappedFs.Root(), rpath)
	if wrapErr == fs.ErrorIsFile {
		fsErr = fs.ErrorIsFile
		rpath = cleanPath(path.Dir(rpath))
	}
	// configure cache backend
	if opt.DbPurge {
		fs.Debugf(name, "Purging the DB")
	}
	f := &Fs{
		Fs:               wrappedFs,
		name:             name,
		root:             rpath,
		opt:              *opt,
		lastChunkCleanup: time.Now().Truncate(time.Hour * 24 * 30),
		cleanupChan:      make(chan bool, 1),
		notifiedRemotes:  make(map[string]bool),
	}
	cache.PinUntilFinalized(f.Fs, f)
	f.rateLimiter = rate.NewLimiter(rate.Limit(float64(opt.Rps)), opt.TotalWorkers)

	f.plexConnector = &plexConnector{}
	if opt.PlexURL != "" {
		if opt.PlexToken != "" {
			f.plexConnector, err = newPlexConnectorWithToken(f, opt.PlexURL, opt.PlexToken, opt.PlexInsecure)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to connect to the Plex API %v", opt.PlexURL)
			}
		} else {
			if opt.PlexPassword != "" && opt.PlexUsername != "" {
				decPass, err := obscure.Reveal(opt.PlexPassword)
				if err != nil {
					decPass = opt.PlexPassword
				}
				f.plexConnector, err = newPlexConnector(f, opt.PlexURL, opt.PlexUsername, decPass, opt.PlexInsecure, func(token string) {
					m.Set("plex_token", token)
				})
				if err != nil {
					return nil, errors.Wrapf(err, "failed to connect to the Plex API %v", opt.PlexURL)
				}
			}
		}
	}

	dbPath := f.opt.DbPath
	chunkPath := f.opt.ChunkPath
	// if the dbPath is non default but the chunk path is default, we overwrite the last to follow the same one as dbPath
	if dbPath != filepath.Join(config.CacheDir, "cache-backend") &&
		chunkPath == filepath.Join(config.CacheDir, "cache-backend") {
		chunkPath = dbPath
	}
	if filepath.Ext(dbPath) != "" {
		dbPath = filepath.Dir(dbPath)
	}
	if filepath.Ext(chunkPath) != "" {
		chunkPath = filepath.Dir(chunkPath)
	}
	err = os.MkdirAll(dbPath, os.ModePerm)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create cache directory %v", dbPath)
	}
	err = os.MkdirAll(chunkPath, os.ModePerm)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create cache directory %v", chunkPath)
	}

	dbPath = filepath.Join(dbPath, name+".db")
	chunkPath = filepath.Join(chunkPath, name)
	fs.Infof(name, "Cache DB path: %v", dbPath)
	fs.Infof(name, "Cache chunk path: %v", chunkPath)
	f.cache, err = GetPersistent(dbPath, chunkPath, &Features{
		PurgeDb:    opt.DbPurge,
		DbWaitTime: time.Duration(opt.DbWaitTime),
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to start cache db")
	}
	// Trap SIGINT and SIGTERM to close the DB handle gracefully
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP)
	atexit.Register(func() {
		if opt.PlexURL != "" {
			f.plexConnector.closeWebsocket()
		}
		f.StopBackgroundRunners()
	})
	go func() {
		for {
			s := <-c
			if s == syscall.SIGHUP {
				fs.Infof(f, "Clearing cache from signal")
				f.DirCacheFlush()
			}
		}
	}()

	fs.Infof(name, "Chunk Memory: %v", !f.opt.ChunkNoMemory)
	fs.Infof(name, "Chunk Size: %v", f.opt.ChunkSize)
	fs.Infof(name, "Chunk Total Size: %v", f.opt.ChunkTotalSize)
	fs.Infof(name, "Chunk Clean Interval: %v", f.opt.ChunkCleanInterval)
	fs.Infof(name, "Workers: %v", f.opt.TotalWorkers)
	fs.Infof(name, "File Age: %v", f.opt.InfoAge)
	if f.opt.StoreWrites {
		fs.Infof(name, "Cache Writes: enabled")
	}

	if f.opt.TempWritePath != "" {
		err = os.MkdirAll(f.opt.TempWritePath, os.ModePerm)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create cache directory %v", f.opt.TempWritePath)
		}
		f.opt.TempWritePath = filepath.ToSlash(f.opt.TempWritePath)
		f.tempFs, err = cache.Get(ctx, f.opt.TempWritePath)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create temp fs: %v", err)
		}
		fs.Infof(name, "Upload Temp Rest Time: %v", f.opt.TempWaitTime)
		fs.Infof(name, "Upload Temp FS: %v", f.opt.TempWritePath)
		f.backgroundRunner, _ = initBackgroundUploader(f)
		go f.backgroundRunner.run()
	}

	go func() {
		for {
			time.Sleep(time.Duration(f.opt.ChunkCleanInterval))
			select {
			case <-f.cleanupChan:
				fs.Infof(f, "stopping cleanup")
				return
			default:
				fs.Debugf(f, "starting cleanup")
				f.CleanUpCache(false)
			}
		}
	}()

	if doChangeNotify := wrappedFs.Features().ChangeNotify; doChangeNotify != nil {
		pollInterval := make(chan time.Duration, 1)
		pollInterval <- time.Duration(f.opt.ChunkCleanInterval)
		doChangeNotify(ctx, f.receiveChangeNotify, pollInterval)
	}

	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
		DuplicateFiles:          false, // storage doesn't permit this
	}).Fill(ctx, f).Mask(ctx, wrappedFs).WrapsFs(f, wrappedFs)
	// override only those features that use a temp fs and it doesn't support them
	//f.features.ChangeNotify = f.ChangeNotify
	if f.opt.TempWritePath != "" {
		if f.tempFs.Features().Move == nil {
			f.features.Move = nil
		}
		if f.tempFs.Features().Move == nil {
			f.features.Move = nil
		}
		if f.tempFs.Features().DirMove == nil {
			f.features.DirMove = nil
		}
		if f.tempFs.Features().MergeDirs == nil {
			f.features.MergeDirs = nil
		}
	}
	// even if the wrapped fs doesn't support it, we still want it
	f.features.DirCacheFlush = f.DirCacheFlush

	rc.Add(rc.Call{
		Path:  "cache/expire",
		Fn:    f.httpExpireRemote,
		Title: "Purge a remote from cache",
		Help: `
Purge a remote from the cache backend. Supports either a directory or a file.
Params:
  - remote = path to remote (required)
  - withData = true/false to delete cached data (chunks) as well (optional)

Eg

    rclone rc cache/expire remote=path/to/sub/folder/
    rclone rc cache/expire remote=/ withData=true 
`,
	})

	rc.Add(rc.Call{
		Path:  "cache/stats",
		Fn:    f.httpStats,
		Title: "Get cache stats",
		Help: `
Show statistics for the cache remote.
`,
	})

	rc.Add(rc.Call{
		Path:  "cache/fetch",
		Fn:    f.rcFetch,
		Title: "Fetch file chunks",
		Help: `
Ensure the specified file chunks are cached on disk.

The chunks= parameter specifies the file chunks to check.
It takes a comma separated list of array slice indices.
The slice indices are similar to Python slices: start[:end]

start is the 0 based chunk number from the beginning of the file
to fetch inclusive. end is 0 based chunk number from the beginning
of the file to fetch exclusive.
Both values can be negative, in which case they count from the back
of the file. The value "-5:" represents the last 5 chunks of a file.

Some valid examples are:
":5,-5:" -> the first and last five chunks
"0,-2" -> the first and the second last chunk
"0:10" -> the first ten chunks

Any parameter with a key that starts with "file" can be used to
specify files to fetch, e.g.

    rclone rc cache/fetch chunks=0 file=hello file2=home/goodbye

File names will automatically be encrypted when the a crypt remote
is used on top of the cache.

`,
	})

	return f, fsErr
}

func (f *Fs) httpStats(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	out = make(rc.Params)
	m, err := f.Stats()
	if err != nil {
		return out, errors.Errorf("error while getting cache stats")
	}
	out["status"] = "ok"
	out["stats"] = m
	return out, nil
}

func (f *Fs) unwrapRemote(remote string) string {
	remote = cleanPath(remote)
	if remote != "" {
		// if it's wrapped by crypt we need to check what format we got
		if cryptFs, yes := f.isWrappedByCrypt(); yes {
			_, err := cryptFs.DecryptFileName(remote)
			// if it failed to decrypt then it is a decrypted format and we need to encrypt it
			if err != nil {
				return cryptFs.EncryptFileName(remote)
			}
			// else it's an encrypted format and we can use it as it is
		}
	}
	return remote
}

func (f *Fs) httpExpireRemote(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	out = make(rc.Params)
	remoteInt, ok := in["remote"]
	if !ok {
		return out, errors.Errorf("remote is needed")
	}
	remote := remoteInt.(string)
	withData := false
	_, ok = in["withData"]
	if ok {
		withData = true
	}

	remote = f.unwrapRemote(remote)
	if !f.cache.HasEntry(path.Join(f.Root(), remote)) {
		return out, errors.Errorf("%s doesn't exist in cache", remote)
	}

	co := NewObject(f, remote)
	err = f.cache.GetObject(co)
	if err != nil { // it could be a dir
		cd := NewDirectory(f, remote)
		err := f.cache.ExpireDir(cd)
		if err != nil {
			return out, errors.WithMessage(err, "error expiring directory")
		}
		// notify vfs too
		f.notifyChangeUpstream(cd.Remote(), fs.EntryDirectory)
		out["status"] = "ok"
		out["message"] = fmt.Sprintf("cached directory cleared: %v", remote)
		return out, nil
	}
	// expire the entry
	err = f.cache.ExpireObject(co, withData)
	if err != nil {
		return out, errors.WithMessage(err, "error expiring file")
	}
	// notify vfs too
	f.notifyChangeUpstream(co.Remote(), fs.EntryObject)

	out["status"] = "ok"
	out["message"] = fmt.Sprintf("cached file cleared: %v", remote)
	return out, nil
}

func (f *Fs) rcFetch(ctx context.Context, in rc.Params) (rc.Params, error) {
	type chunkRange struct {
		start, end int64
	}
	parseChunks := func(ranges string) (crs []chunkRange, err error) {
		for _, part := range strings.Split(ranges, ",") {
			var start, end int64 = 0, math.MaxInt64
			switch ints := strings.Split(part, ":"); len(ints) {
			case 1:
				start, err = strconv.ParseInt(ints[0], 10, 64)
				if err != nil {
					return nil, errors.Errorf("invalid range: %q", part)
				}
				end = start + 1
			case 2:
				if ints[0] != "" {
					start, err = strconv.ParseInt(ints[0], 10, 64)
					if err != nil {
						return nil, errors.Errorf("invalid range: %q", part)
					}
				}
				if ints[1] != "" {
					end, err = strconv.ParseInt(ints[1], 10, 64)
					if err != nil {
						return nil, errors.Errorf("invalid range: %q", part)
					}
				}
			default:
				return nil, errors.Errorf("invalid range: %q", part)
			}
			crs = append(crs, chunkRange{start: start, end: end})
		}
		return
	}
	walkChunkRange := func(cr chunkRange, size int64, cb func(chunk int64)) {
		if size <= 0 {
			return
		}
		chunks := (size-1)/f.ChunkSize() + 1

		start, end := cr.start, cr.end
		if start < 0 {
			start += chunks
		}
		if end <= 0 {
			end += chunks
		}
		if end <= start {
			return
		}
		switch {
		case start < 0:
			start = 0
		case start >= chunks:
			return
		}
		switch {
		case end <= start:
			end = start + 1
		case end >= chunks:
			end = chunks
		}
		for i := start; i < end; i++ {
			cb(i)
		}
	}
	walkChunkRanges := func(crs []chunkRange, size int64, cb func(chunk int64)) {
		for _, cr := range crs {
			walkChunkRange(cr, size, cb)
		}
	}

	v, ok := in["chunks"]
	if !ok {
		return nil, errors.New("missing chunks parameter")
	}
	s, ok := v.(string)
	if !ok {
		return nil, errors.New("invalid chunks parameter")
	}
	delete(in, "chunks")
	crs, err := parseChunks(s)
	if err != nil {
		return nil, errors.Wrap(err, "invalid chunks parameter")
	}
	var files [][2]string
	for k, v := range in {
		if !strings.HasPrefix(k, "file") {
			return nil, errors.Errorf("invalid parameter %s=%s", k, v)
		}
		switch v := v.(type) {
		case string:
			files = append(files, [2]string{v, f.unwrapRemote(v)})
		default:
			return nil, errors.Errorf("invalid parameter %s=%s", k, v)
		}
	}
	type fileStatus struct {
		Error         string
		FetchedChunks int
	}
	fetchedChunks := make(map[string]fileStatus, len(files))
	for _, pair := range files {
		file, remote := pair[0], pair[1]
		var status fileStatus
		o, err := f.NewObject(ctx, remote)
		if err != nil {
			fetchedChunks[file] = fileStatus{Error: err.Error()}
			continue
		}
		co := o.(*Object)
		err = co.refreshFromSource(ctx, true)
		if err != nil {
			fetchedChunks[file] = fileStatus{Error: err.Error()}
			continue
		}
		handle := NewObjectHandle(ctx, co, f)
		handle.UseMemory = false
		handle.scaleWorkers(1)
		walkChunkRanges(crs, co.Size(), func(chunk int64) {
			_, err := handle.getChunk(chunk * f.ChunkSize())
			if err != nil {
				if status.Error == "" {
					status.Error = err.Error()
				}
			} else {
				status.FetchedChunks++
			}
		})
		fetchedChunks[file] = status
	}

	return rc.Params{"status": fetchedChunks}, nil
}

// receiveChangeNotify is a wrapper to notifications sent from the wrapped FS about changed files
func (f *Fs) receiveChangeNotify(forgetPath string, entryType fs.EntryType) {
	if crypt, yes := f.isWrappedByCrypt(); yes {
		decryptedPath, err := crypt.DecryptFileName(forgetPath)
		if err == nil {
			fs.Infof(decryptedPath, "received cache expiry notification")
		} else {
			fs.Infof(forgetPath, "received cache expiry notification")
		}
	} else {
		fs.Infof(forgetPath, "received cache expiry notification")
	}
	// notify upstreams too (vfs)
	f.notifyChangeUpstream(forgetPath, entryType)

	var cd *Directory
	if entryType == fs.EntryObject {
		co := NewObject(f, forgetPath)
		err := f.cache.GetObject(co)
		if err != nil {
			fs.Debugf(f, "got change notification for non cached entry %v", co)
		}
		err = f.cache.ExpireObject(co, true)
		if err != nil {
			fs.Debugf(forgetPath, "notify: error expiring '%v': %v", co, err)
		}
		cd = NewDirectory(f, cleanPath(path.Dir(co.Remote())))
	} else {
		cd = NewDirectory(f, forgetPath)
	}
	// we expire the dir
	err := f.cache.ExpireDir(cd)
	if err != nil {
		fs.Debugf(forgetPath, "notify: error expiring '%v': %v", cd, err)
	} else {
		fs.Debugf(forgetPath, "notify: expired '%v'", cd)
	}

	f.notifiedMu.Lock()
	defer f.notifiedMu.Unlock()
	f.notifiedRemotes[forgetPath] = true
	f.notifiedRemotes[cd.Remote()] = true
}

// notifyChangeUpstreamIfNeeded will check if the wrapped remote doesn't notify on changes
// or if we use a temp fs
func (f *Fs) notifyChangeUpstreamIfNeeded(remote string, entryType fs.EntryType) {
	if f.Fs.Features().ChangeNotify == nil || f.opt.TempWritePath != "" {
		f.notifyChangeUpstream(remote, entryType)
	}
}

// notifyChangeUpstream will loop through all the upstreams and notify
// of the provided remote (should be only a dir)
func (f *Fs) notifyChangeUpstream(remote string, entryType fs.EntryType) {
	f.parentsForgetMu.Lock()
	defer f.parentsForgetMu.Unlock()
	if len(f.parentsForgetFn) > 0 {
		for _, fn := range f.parentsForgetFn {
			fn(remote, entryType)
		}
	}
}

// ChangeNotify can subscribe multiple callers
// this is coupled with the wrapped fs ChangeNotify (if it supports it)
// and also notifies other caches (i.e VFS) to clear out whenever something changes
func (f *Fs) ChangeNotify(ctx context.Context, notifyFunc func(string, fs.EntryType), pollInterval <-chan time.Duration) {
	f.parentsForgetMu.Lock()
	defer f.parentsForgetMu.Unlock()
	fs.Debugf(f, "subscribing to ChangeNotify")
	f.parentsForgetFn = append(f.parentsForgetFn, notifyFunc)
	go func() {
		for range pollInterval {
		}
	}()
}

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return f.root
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// String returns a description of the FS
func (f *Fs) String() string {
	return fmt.Sprintf("Cache remote %s:%s", f.name, f.root)
}

// ChunkSize returns the configured chunk size
func (f *Fs) ChunkSize() int64 {
	return int64(f.opt.ChunkSize)
}

// InfoAge returns the configured file age
func (f *Fs) InfoAge() time.Duration {
	return time.Duration(f.opt.InfoAge)
}

// TempUploadWaitTime returns the configured temp file upload wait time
func (f *Fs) TempUploadWaitTime() time.Duration {
	return time.Duration(f.opt.TempWaitTime)
}

// NewObject finds the Object at remote.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	var err error

	fs.Debugf(f, "new object '%s'", remote)
	co := NewObject(f, remote)
	// search for entry in cache and validate it
	err = f.cache.GetObject(co)
	if err != nil {
		fs.Debugf(remote, "find: error: %v", err)
	} else if time.Now().After(co.CacheTs.Add(time.Duration(f.opt.InfoAge))) {
		fs.Debugf(co, "find: cold object: %+v", co)
	} else {
		fs.Debugf(co, "find: warm object: %v, expiring on: %v", co, co.CacheTs.Add(time.Duration(f.opt.InfoAge)))
		return co, nil
	}

	// search for entry in source or temp fs
	var obj fs.Object
	if f.opt.TempWritePath != "" {
		obj, err = f.tempFs.NewObject(ctx, remote)
		// not found in temp fs
		if err != nil {
			fs.Debugf(remote, "find: not found in local cache fs")
			obj, err = f.Fs.NewObject(ctx, remote)
		} else {
			fs.Debugf(obj, "find: found in local cache fs")
		}
	} else {
		obj, err = f.Fs.NewObject(ctx, remote)
	}

	// not found in either fs
	if err != nil {
		fs.Debugf(obj, "find failed: not found in either local or remote fs")
		return nil, err
	}

	// cache the new entry
	co = ObjectFromOriginal(ctx, f, obj).persist()
	fs.Debugf(co, "find: cached object")
	return co, nil
}

// List the objects and directories in dir into entries
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	fs.Debugf(f, "list '%s'", dir)
	cd := ShallowDirectory(f, dir)

	// search for cached dir entries and validate them
	entries, err = f.cache.GetDirEntries(cd)
	if err != nil {
		fs.Debugf(dir, "list: error: %v", err)
	} else if time.Now().After(cd.CacheTs.Add(time.Duration(f.opt.InfoAge))) {
		fs.Debugf(dir, "list: cold listing: %v", cd.CacheTs)
	} else if len(entries) == 0 {
		// TODO: read empty dirs from source?
		fs.Debugf(dir, "list: empty listing")
	} else {
		fs.Debugf(dir, "list: warm %v from cache for: %v, expiring on: %v", len(entries), cd.abs(), cd.CacheTs.Add(time.Duration(f.opt.InfoAge)))
		fs.Debugf(dir, "list: cached entries: %v", entries)
		return entries, nil
	}

	// we first search any temporary files stored locally
	var cachedEntries fs.DirEntries
	if f.opt.TempWritePath != "" {
		queuedEntries, err := f.cache.searchPendingUploadFromDir(cd.abs())
		if err != nil {
			fs.Errorf(dir, "list: error getting pending uploads: %v", err)
		} else {
			fs.Debugf(dir, "list: read %v from temp fs", len(queuedEntries))
			fs.Debugf(dir, "list: temp fs entries: %v", queuedEntries)

			for _, queuedRemote := range queuedEntries {
				queuedEntry, err := f.tempFs.NewObject(ctx, f.cleanRootFromPath(queuedRemote))
				if err != nil {
					fs.Debugf(dir, "list: temp file not found in local fs: %v", err)
					continue
				}
				co := ObjectFromOriginal(ctx, f, queuedEntry).persist()
				fs.Debugf(co, "list: cached temp object")
				cachedEntries = append(cachedEntries, co)
			}
		}
	}

	// search from the source
	sourceEntries, err := f.Fs.List(ctx, dir)
	if err != nil {
		return nil, err
	}
	fs.Debugf(dir, "list: read %v from source", len(sourceEntries))
	fs.Debugf(dir, "list: source entries: %v", sourceEntries)

	sort.Sort(sourceEntries)
	for _, entry := range entries {
		entryRemote := entry.Remote()
		i := sort.Search(len(sourceEntries), func(i int) bool { return sourceEntries[i].Remote() >= entryRemote })
		if i < len(sourceEntries) && sourceEntries[i].Remote() == entryRemote {
			continue
		}
		fp := path.Join(f.Root(), entryRemote)
		switch entry.(type) {
		case fs.Object:
			_ = f.cache.RemoveObject(fp)
		case fs.Directory:
			_ = f.cache.RemoveDir(fp)
		}
		fs.Debugf(dir, "list: remove entry: %v", entryRemote)
	}
	entries = nil

	// and then iterate over the ones from source (temp Objects will override source ones)
	var batchDirectories []*Directory
	sort.Sort(cachedEntries)
	tmpCnt := len(cachedEntries)
	for _, entry := range sourceEntries {
		switch o := entry.(type) {
		case fs.Object:
			// skip over temporary objects (might be uploading)
			oRemote := o.Remote()
			i := sort.Search(tmpCnt, func(i int) bool { return cachedEntries[i].Remote() >= oRemote })
			if i < tmpCnt && cachedEntries[i].Remote() == oRemote {
				continue
			}
			co := ObjectFromOriginal(ctx, f, o).persist()
			cachedEntries = append(cachedEntries, co)
			fs.Debugf(dir, "list: cached object: %v", co)
		case fs.Directory:
			cdd := DirectoryFromOriginal(ctx, f, o)
			// check if the dir isn't expired and add it in cache if it isn't
			if cdd2, err := f.cache.GetDir(cdd.abs()); err != nil || time.Now().Before(cdd2.CacheTs.Add(time.Duration(f.opt.InfoAge))) {
				batchDirectories = append(batchDirectories, cdd)
			}
			cachedEntries = append(cachedEntries, cdd)
		default:
			fs.Debugf(entry, "list: Unknown object type %T", entry)
		}
	}
	err = f.cache.AddBatchDir(batchDirectories)
	if err != nil {
		fs.Errorf(dir, "list: error caching directories from listing %v", dir)
	} else {
		fs.Debugf(dir, "list: cached directories: %v", len(batchDirectories))
	}

	// cache dir meta
	t := time.Now()
	cd.CacheTs = &t
	err = f.cache.AddDir(cd)
	if err != nil {
		fs.Errorf(cd, "list: save error: '%v'", err)
	} else {
		fs.Debugf(dir, "list: cached dir: '%v', cache ts: %v", cd.abs(), cd.CacheTs)
	}

	return cachedEntries, nil
}

func (f *Fs) recurse(ctx context.Context, dir string, list *walk.ListRHelper) error {
	entries, err := f.List(ctx, dir)
	if err != nil {
		return err
	}

	for i := 0; i < len(entries); i++ {
		innerDir, ok := entries[i].(fs.Directory)
		if ok {
			err := f.recurse(ctx, innerDir.Remote(), list)
			if err != nil {
				return err
			}
		}

		err := list.Add(entries[i])
		if err != nil {
			return err
		}
	}

	return nil
}

// ListR lists the objects and directories of the Fs starting
// from dir recursively into out.
func (f *Fs) ListR(ctx context.Context, dir string, callback fs.ListRCallback) (err error) {
	fs.Debugf(f, "list recursively from '%s'", dir)

	// we check if the source FS supports ListR
	// if it does, we'll use that to get all the entries, cache them and return
	do := f.Fs.Features().ListR
	if do != nil {
		return do(ctx, dir, func(entries fs.DirEntries) error {
			// we got called back with a set of entries so let's cache them and call the original callback
			for _, entry := range entries {
				switch o := entry.(type) {
				case fs.Object:
					_ = f.cache.AddObject(ObjectFromOriginal(ctx, f, o))
				case fs.Directory:
					_ = f.cache.AddDir(DirectoryFromOriginal(ctx, f, o))
				default:
					return errors.Errorf("Unknown object type %T", entry)
				}
			}

			// call the original callback
			return callback(entries)
		})
	}

	// if we're here, we're gonna do a standard recursive traversal and cache everything
	list := walk.NewListRHelper(callback)
	err = f.recurse(ctx, dir, list)
	if err != nil {
		return err
	}

	return list.Flush()
}

// Mkdir makes the directory (container, bucket)
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	fs.Debugf(f, "mkdir '%s'", dir)
	err := f.Fs.Mkdir(ctx, dir)
	if err != nil {
		return err
	}
	fs.Debugf(dir, "mkdir: created dir in source fs")

	cd := NewDirectory(f, cleanPath(dir))
	err = f.cache.AddDir(cd)
	if err != nil {
		fs.Errorf(dir, "mkdir: add error: %v", err)
	} else {
		fs.Debugf(cd, "mkdir: added to cache")
	}
	// expire parent of new dir
	parentCd := NewDirectory(f, cleanPath(path.Dir(dir)))
	err = f.cache.ExpireDir(parentCd)
	if err != nil {
		fs.Errorf(parentCd, "mkdir: cache expire error: %v", err)
	} else {
		fs.Infof(parentCd, "mkdir: cache expired")
	}
	// advertise to ChangeNotify if wrapped doesn't do that
	f.notifyChangeUpstreamIfNeeded(parentCd.Remote(), fs.EntryDirectory)

	return nil
}

// Rmdir removes the directory (container, bucket) if empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	fs.Debugf(f, "rmdir '%s'", dir)

	if f.opt.TempWritePath != "" {
		// pause background uploads
		f.backgroundRunner.pause()
		defer f.backgroundRunner.play()

		// we check if the source exists on the remote and make the same move on it too if it does
		// otherwise, we skip this step
		_, err := f.UnWrap().List(ctx, dir)
		if err == nil {
			err := f.Fs.Rmdir(ctx, dir)
			if err != nil {
				return err
			}
			fs.Debugf(dir, "rmdir: removed dir in source fs")
		}

		var queuedEntries []*Object
		err = walk.ListR(ctx, f.tempFs, dir, true, -1, walk.ListObjects, func(entries fs.DirEntries) error {
			for _, o := range entries {
				if oo, ok := o.(fs.Object); ok {
					co := ObjectFromOriginal(ctx, f, oo)
					queuedEntries = append(queuedEntries, co)
				}
			}
			return nil
		})
		if err != nil {
			fs.Errorf(dir, "rmdir: error getting pending uploads: %v", err)
		} else {
			fs.Debugf(dir, "rmdir: read %v from temp fs", len(queuedEntries))
			fs.Debugf(dir, "rmdir: temp fs entries: %v", queuedEntries)
			if len(queuedEntries) > 0 {
				fs.Errorf(dir, "rmdir: temporary dir not empty: %v", queuedEntries)
				return fs.ErrorDirectoryNotEmpty
			}
		}
	} else {
		err := f.Fs.Rmdir(ctx, dir)
		if err != nil {
			return err
		}
		fs.Debugf(dir, "rmdir: removed dir in source fs")
	}

	// remove dir data
	d := NewDirectory(f, dir)
	err := f.cache.RemoveDir(d.abs())
	if err != nil {
		fs.Errorf(dir, "rmdir: remove error: %v", err)
	} else {
		fs.Debugf(d, "rmdir: removed from cache")
	}
	// expire parent
	parentCd := NewDirectory(f, cleanPath(path.Dir(dir)))
	err = f.cache.ExpireDir(parentCd)
	if err != nil {
		fs.Errorf(dir, "rmdir: cache expire error: %v", err)
	} else {
		fs.Infof(parentCd, "rmdir: cache expired")
	}
	// advertise to ChangeNotify if wrapped doesn't do that
	f.notifyChangeUpstreamIfNeeded(parentCd.Remote(), fs.EntryDirectory)

	return nil
}

// DirMove moves src, srcRemote to this remote at dstRemote
// using server-side move operations.
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) error {
	fs.Debugf(f, "move dir '%s'/'%s' -> '%s'/'%s'", src.Root(), srcRemote, f.Root(), dstRemote)

	do := f.Fs.Features().DirMove
	if do == nil {
		return fs.ErrorCantDirMove
	}
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Errorf(srcFs, "can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}
	if srcFs.Fs.Name() != f.Fs.Name() {
		fs.Errorf(srcFs, "can't move directory - not wrapping same remotes")
		return fs.ErrorCantDirMove
	}

	if f.opt.TempWritePath != "" {
		// pause background uploads
		f.backgroundRunner.pause()
		defer f.backgroundRunner.play()

		_, errInWrap := srcFs.UnWrap().List(ctx, srcRemote)
		_, errInTemp := f.tempFs.List(ctx, srcRemote)
		// not found in either fs
		if errInWrap != nil && errInTemp != nil {
			return fs.ErrorDirNotFound
		}

		// we check if the source exists on the remote and make the same move on it too if it does
		// otherwise, we skip this step
		if errInWrap == nil {
			err := do(ctx, srcFs.UnWrap(), srcRemote, dstRemote)
			if err != nil {
				return err
			}
			fs.Debugf(srcRemote, "movedir: dir moved in the source fs")
		}
		// we need to check if the directory exists in the temp fs
		// and skip the move if it doesn't
		if errInTemp != nil {
			goto cleanup
		}

		var queuedEntries []*Object
		err := walk.ListR(ctx, f.tempFs, srcRemote, true, -1, walk.ListObjects, func(entries fs.DirEntries) error {
			for _, o := range entries {
				if oo, ok := o.(fs.Object); ok {
					co := ObjectFromOriginal(ctx, f, oo)
					queuedEntries = append(queuedEntries, co)
					if co.tempFileStartedUpload() {
						fs.Errorf(co, "can't move - upload has already started. need to finish that")
						return fs.ErrorCantDirMove
					}
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		fs.Debugf(srcRemote, "dirmove: read %v from temp fs", len(queuedEntries))
		fs.Debugf(srcRemote, "dirmove: temp fs entries: %v", queuedEntries)

		do := f.tempFs.Features().DirMove
		if do == nil {
			fs.Errorf(srcRemote, "dirmove: can't move dir in temp fs")
			return fs.ErrorCantDirMove
		}
		err = do(ctx, f.tempFs, srcRemote, dstRemote)
		if err != nil {
			return err
		}
		err = f.cache.ReconcileTempUploads(ctx, f)
		if err != nil {
			return err
		}
	} else {
		err := do(ctx, srcFs.UnWrap(), srcRemote, dstRemote)
		if err != nil {
			return err
		}
		fs.Debugf(srcRemote, "movedir: dir moved in the source fs")
	}
cleanup:

	// delete src dir from cache along with all chunks
	srcDir := NewDirectory(srcFs, srcRemote)
	err := f.cache.RemoveDir(srcDir.abs())
	if err != nil {
		fs.Errorf(srcDir, "dirmove: remove error: %v", err)
	} else {
		fs.Debugf(srcDir, "dirmove: removed cached dir")
	}
	// expire src parent
	srcParent := NewDirectory(f, cleanPath(path.Dir(srcRemote)))
	err = f.cache.ExpireDir(srcParent)
	if err != nil {
		fs.Errorf(srcParent, "dirmove: cache expire error: %v", err)
	} else {
		fs.Debugf(srcParent, "dirmove: cache expired")
	}
	// advertise to ChangeNotify if wrapped doesn't do that
	f.notifyChangeUpstreamIfNeeded(srcParent.Remote(), fs.EntryDirectory)

	// expire parent dir at the destination path
	dstParent := NewDirectory(f, cleanPath(path.Dir(dstRemote)))
	err = f.cache.ExpireDir(dstParent)
	if err != nil {
		fs.Errorf(dstParent, "dirmove: cache expire error: %v", err)
	} else {
		fs.Debugf(dstParent, "dirmove: cache expired")
	}
	// advertise to ChangeNotify if wrapped doesn't do that
	f.notifyChangeUpstreamIfNeeded(dstParent.Remote(), fs.EntryDirectory)
	// TODO: precache dst dir and save the chunks

	return nil
}

// cacheReader will split the stream of a reader to be cached at the same time it is read by the original source
func (f *Fs) cacheReader(u io.Reader, src fs.ObjectInfo, originalRead func(inn io.Reader)) {
	// create the pipe and tee reader
	pr, pw := io.Pipe()
	tr := io.TeeReader(u, pw)

	// create channel to synchronize
	done := make(chan bool)
	defer close(done)

	go func() {
		// notify the cache reader that we're complete after the source FS finishes
		defer func() {
			_ = pw.Close()
		}()
		// process original reading
		originalRead(tr)
		// signal complete
		done <- true
	}()

	go func() {
		var offset int64
		for {
			chunk := make([]byte, f.opt.ChunkSize)
			readSize, err := io.ReadFull(pr, chunk)
			// we ignore 3 failures which are ok:
			// 1. EOF - original reading finished and we got a full buffer too
			// 2. ErrUnexpectedEOF - original reading finished and partial buffer
			// 3. ErrClosedPipe - source remote reader was closed (usually means it reached the end) and we need to stop too
			// if we have a different error: we're going to error out the original reading too and stop this
			if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF && err != io.ErrClosedPipe {
				fs.Errorf(src, "error saving new data in cache. offset: %v, err: %v", offset, err)
				_ = pr.CloseWithError(err)
				break
			}
			// if we have some bytes we cache them
			if readSize > 0 {
				chunk = chunk[:readSize]
				err2 := f.cache.AddChunk(cleanPath(path.Join(f.root, src.Remote())), chunk, offset)
				if err2 != nil {
					fs.Errorf(src, "error saving new data in cache '%v'", err2)
					_ = pr.CloseWithError(err2)
					break
				}
				offset += int64(readSize)
			}
			// stuff should be closed but let's be sure
			if err == io.EOF || err == io.ErrUnexpectedEOF || err == io.ErrClosedPipe {
				_ = pr.Close()
				break
			}
		}

		// signal complete
		done <- true
	}()

	// wait until both are done
	for c := 0; c < 2; c++ {
		<-done
	}
}

type putFn func(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error)

// put in to the remote path
func (f *Fs) put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options []fs.OpenOption, put putFn) (fs.Object, error) {
	var err error
	var obj fs.Object

	// queue for upload and store in temp fs if configured
	if f.opt.TempWritePath != "" {
		// we need to clear the caches before a put through temp fs
		parentCd := NewDirectory(f, cleanPath(path.Dir(src.Remote())))
		_ = f.cache.ExpireDir(parentCd)
		f.notifyChangeUpstreamIfNeeded(parentCd.Remote(), fs.EntryDirectory)

		obj, err = f.tempFs.Put(ctx, in, src, options...)
		if err != nil {
			fs.Errorf(obj, "put: failed to upload in temp fs: %v", err)
			return nil, err
		}
		fs.Infof(obj, "put: uploaded in temp fs")
		err = f.cache.addPendingUpload(path.Join(f.Root(), src.Remote()), false)
		if err != nil {
			fs.Errorf(obj, "put: failed to queue for upload: %v", err)
			return nil, err
		}
		fs.Infof(obj, "put: queued for upload")
		// if cache writes is enabled write it first through cache
	} else if f.opt.StoreWrites {
		f.cacheReader(in, src, func(inn io.Reader) {
			obj, err = put(ctx, inn, src, options...)
		})
		if err == nil {
			fs.Debugf(obj, "put: uploaded to remote fs and saved in cache")
		}
		// last option: save it directly in remote fs
	} else {
		obj, err = put(ctx, in, src, options...)
		if err == nil {
			fs.Debugf(obj, "put: uploaded to remote fs")
		}
	}
	// validate and stop if errors are found
	if err != nil {
		fs.Errorf(src, "put: error uploading: %v", err)
		return nil, err
	}

	// cache the new file
	cachedObj := ObjectFromOriginal(ctx, f, obj)

	// deleting cached chunks and info to be replaced with new ones
	_ = f.cache.RemoveObject(cachedObj.abs())

	cachedObj.persist()
	fs.Debugf(cachedObj, "put: added to cache")

	// expire parent
	parentCd := NewDirectory(f, cleanPath(path.Dir(cachedObj.Remote())))
	err = f.cache.ExpireDir(parentCd)
	if err != nil {
		fs.Errorf(cachedObj, "put: cache expire error: %v", err)
	} else {
		fs.Infof(parentCd, "put: cache expired")
	}
	// advertise to ChangeNotify
	f.notifyChangeUpstreamIfNeeded(parentCd.Remote(), fs.EntryDirectory)

	return cachedObj, nil
}

// Put in to the remote path with the modTime given of the given size
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	fs.Debugf(f, "put data at '%s'", src.Remote())
	return f.put(ctx, in, src, options, f.Fs.Put)
}

// PutUnchecked uploads the object
func (f *Fs) PutUnchecked(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	do := f.Fs.Features().PutUnchecked
	if do == nil {
		return nil, errors.New("can't PutUnchecked")
	}
	fs.Debugf(f, "put data unchecked in '%s'", src.Remote())
	return f.put(ctx, in, src, options, do)
}

// PutStream uploads the object
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	do := f.Fs.Features().PutStream
	if do == nil {
		return nil, errors.New("can't PutStream")
	}
	fs.Debugf(f, "put data streaming in '%s'", src.Remote())
	return f.put(ctx, in, src, options, do)
}

// Copy src to this remote using server-side copy operations.
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	fs.Debugf(f, "copy obj '%s' -> '%s'", src, remote)

	do := f.Fs.Features().Copy
	if do == nil {
		fs.Errorf(src, "source remote (%v) doesn't support Copy", src.Fs())
		return nil, fs.ErrorCantCopy
	}
	if f.opt.TempWritePath != "" && src.Fs() == f.tempFs {
		return nil, fs.ErrorCantCopy
	}
	// the source must be a cached object or we abort
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Errorf(srcObj, "can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}
	// both the source cache fs and this cache fs need to wrap the same remote
	if srcObj.CacheFs.Fs.Name() != f.Fs.Name() {
		fs.Errorf(srcObj, "can't copy - not wrapping same remotes")
		return nil, fs.ErrorCantCopy
	}
	// refresh from source or abort
	if err := srcObj.refreshFromSource(ctx, false); err != nil {
		fs.Errorf(f, "can't copy %v - %v", src, err)
		return nil, fs.ErrorCantCopy
	}

	if srcObj.isTempFile() {
		// we check if the feature is still active
		if f.opt.TempWritePath == "" {
			fs.Errorf(srcObj, "can't copy - this is a local cached file but this feature is turned off this run")
			return nil, fs.ErrorCantCopy
		}

		do = srcObj.ParentFs.Features().Copy
		if do == nil {
			fs.Errorf(src, "parent remote (%v) doesn't support Copy", srcObj.ParentFs)
			return nil, fs.ErrorCantCopy
		}
	}

	obj, err := do(ctx, srcObj.Object, remote)
	if err != nil {
		fs.Errorf(srcObj, "error moving in cache: %v", err)
		return nil, err
	}
	fs.Debugf(obj, "copy: file copied")

	// persist new
	co := ObjectFromOriginal(ctx, f, obj).persist()
	fs.Debugf(co, "copy: added to cache")
	// expire the destination path
	parentCd := NewDirectory(f, cleanPath(path.Dir(co.Remote())))
	err = f.cache.ExpireDir(parentCd)
	if err != nil {
		fs.Errorf(parentCd, "copy: cache expire error: %v", err)
	} else {
		fs.Infof(parentCd, "copy: cache expired")
	}
	// advertise to ChangeNotify if wrapped doesn't do that
	f.notifyChangeUpstreamIfNeeded(parentCd.Remote(), fs.EntryDirectory)
	// expire src parent
	srcParent := NewDirectory(f, cleanPath(path.Dir(src.Remote())))
	err = f.cache.ExpireDir(srcParent)
	if err != nil {
		fs.Errorf(srcParent, "copy: cache expire error: %v", err)
	} else {
		fs.Infof(srcParent, "copy: cache expired")
	}
	// advertise to ChangeNotify if wrapped doesn't do that
	f.notifyChangeUpstreamIfNeeded(srcParent.Remote(), fs.EntryDirectory)

	return co, nil
}

// Move src to this remote using server-side move operations.
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	fs.Debugf(f, "moving obj '%s' -> %s", src, remote)

	// if source fs doesn't support move abort
	do := f.Fs.Features().Move
	if do == nil {
		fs.Errorf(src, "source remote (%v) doesn't support Move", src.Fs())
		return nil, fs.ErrorCantMove
	}
	// the source must be a cached object or we abort
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Errorf(srcObj, "can't move - not same remote type")
		return nil, fs.ErrorCantMove
	}
	// both the source cache fs and this cache fs need to wrap the same remote
	if srcObj.CacheFs.Fs.Name() != f.Fs.Name() {
		fs.Errorf(srcObj, "can't move - not wrapping same remote types")
		return nil, fs.ErrorCantMove
	}
	// refresh from source or abort
	if err := srcObj.refreshFromSource(ctx, false); err != nil {
		fs.Errorf(f, "can't move %v - %v", src, err)
		return nil, fs.ErrorCantMove
	}

	// if this is a temp object then we perform the changes locally
	if srcObj.isTempFile() {
		// we check if the feature is still active
		if f.opt.TempWritePath == "" {
			fs.Errorf(srcObj, "can't move - this is a local cached file but this feature is turned off this run")
			return nil, fs.ErrorCantMove
		}
		// pause background uploads
		f.backgroundRunner.pause()
		defer f.backgroundRunner.play()

		// started uploads can't be moved until they complete
		if srcObj.tempFileStartedUpload() {
			fs.Errorf(srcObj, "can't move - upload has already started. need to finish that")
			return nil, fs.ErrorCantMove
		}
		do = f.tempFs.Features().Move

		// we must also update the pending queue
		err := f.cache.updatePendingUpload(srcObj.abs(), func(item *tempUploadInfo) error {
			item.DestPath = path.Join(f.Root(), remote)
			item.AddedOn = time.Now()
			return nil
		})
		if err != nil {
			fs.Errorf(srcObj, "failed to rename queued file for upload: %v", err)
			return nil, fs.ErrorCantMove
		}
		fs.Debugf(srcObj, "move: queued file moved to %v", remote)
	}

	obj, err := do(ctx, srcObj.Object, remote)
	if err != nil {
		fs.Errorf(srcObj, "error moving: %v", err)
		return nil, err
	}
	fs.Debugf(obj, "move: file moved")

	// remove old
	err = f.cache.RemoveObject(srcObj.abs())
	if err != nil {
		fs.Errorf(srcObj, "move: remove error: %v", err)
	} else {
		fs.Debugf(srcObj, "move: removed from cache")
	}
	// expire old parent
	parentCd := NewDirectory(f, cleanPath(path.Dir(srcObj.Remote())))
	err = f.cache.ExpireDir(parentCd)
	if err != nil {
		fs.Errorf(parentCd, "move: parent cache expire error: %v", err)
	} else {
		fs.Infof(parentCd, "move: cache expired")
	}
	// advertise to ChangeNotify if wrapped doesn't do that
	f.notifyChangeUpstreamIfNeeded(parentCd.Remote(), fs.EntryDirectory)
	// persist new
	cachedObj := ObjectFromOriginal(ctx, f, obj).persist()
	fs.Debugf(cachedObj, "move: added to cache")
	// expire new parent
	parentCd = NewDirectory(f, cleanPath(path.Dir(cachedObj.Remote())))
	err = f.cache.ExpireDir(parentCd)
	if err != nil {
		fs.Errorf(parentCd, "move: expire error: %v", err)
	} else {
		fs.Infof(parentCd, "move: cache expired")
	}
	// advertise to ChangeNotify if wrapped doesn't do that
	f.notifyChangeUpstreamIfNeeded(parentCd.Remote(), fs.EntryDirectory)

	return cachedObj, nil
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return f.Fs.Hashes()
}

// Purge all files in the directory
func (f *Fs) Purge(ctx context.Context, dir string) error {
	if dir == "" {
		// FIXME this isn't quite right as it should purge the dir prefix
		fs.Infof(f, "purging cache")
		f.cache.Purge()
	}

	do := f.Fs.Features().Purge
	if do == nil {
		return fs.ErrorCantPurge
	}

	err := do(ctx, dir)
	if err != nil {
		return err
	}

	return nil
}

// CleanUp the trash in the Fs
func (f *Fs) CleanUp(ctx context.Context) error {
	f.CleanUpCache(false)

	do := f.Fs.Features().CleanUp
	if do == nil {
		return nil
	}

	return do(ctx)
}

// About gets quota information from the Fs
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	do := f.Fs.Features().About
	if do == nil {
		return nil, errors.New("About not supported")
	}
	return do(ctx)
}

// Stats returns stats about the cache storage
func (f *Fs) Stats() (map[string]map[string]interface{}, error) {
	return f.cache.Stats()
}

// openRateLimited will execute a closure under a rate limiter watch
func (f *Fs) openRateLimited(fn func() (io.ReadCloser, error)) (io.ReadCloser, error) {
	var err error
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	start := time.Now()

	if err = f.rateLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	elapsed := time.Since(start)
	if elapsed > time.Second*2 {
		fs.Debugf(f, "rate limited: %s", elapsed)
	}
	return fn()
}

// CleanUpCache will cleanup only the cache data that is expired
func (f *Fs) CleanUpCache(ignoreLastTs bool) {
	f.cleanupMu.Lock()
	defer f.cleanupMu.Unlock()

	if ignoreLastTs || time.Now().After(f.lastChunkCleanup.Add(time.Duration(f.opt.ChunkCleanInterval))) {
		f.cache.CleanChunksBySize(int64(f.opt.ChunkTotalSize))
		f.lastChunkCleanup = time.Now()
	}
}

// StopBackgroundRunners will signall all the runners to stop their work
// can be triggered from a terminate signal or from testing between runs
func (f *Fs) StopBackgroundRunners() {
	f.cleanupChan <- false
	if f.opt.TempWritePath != "" && f.backgroundRunner != nil && f.backgroundRunner.isRunning() {
		f.backgroundRunner.close()
	}
	f.cache.Close()
	fs.Debugf(f, "Services stopped")
}

// UnWrap returns the Fs that this Fs is wrapping
func (f *Fs) UnWrap() fs.Fs {
	return f.Fs
}

// WrapFs returns the Fs that is wrapping this Fs
func (f *Fs) WrapFs() fs.Fs {
	return f.wrapper
}

// SetWrapper sets the Fs that is wrapping this Fs
func (f *Fs) SetWrapper(wrapper fs.Fs) {
	f.wrapper = wrapper
}

// isWrappedByCrypt checks if this is wrapped by a crypt remote
func (f *Fs) isWrappedByCrypt() (*crypt.Fs, bool) {
	if f.wrapper == nil {
		return nil, false
	}
	c, ok := f.wrapper.(*crypt.Fs)
	return c, ok
}

// cleanRootFromPath trims the root of the current fs from a path
func (f *Fs) cleanRootFromPath(p string) string {
	if f.Root() != "" {
		p = p[len(f.Root()):] // trim out root
		if len(p) > 0 {       // remove first separator
			p = p[1:]
		}
	}

	return p
}

func (f *Fs) isRootInPath(p string) bool {
	if f.Root() == "" {
		return true
	}
	return strings.HasPrefix(p, f.Root()+"/")
}

// MergeDirs merges the contents of all the directories passed
// in into the first one and rmdirs the other directories.
func (f *Fs) MergeDirs(ctx context.Context, dirs []fs.Directory) error {
	do := f.Fs.Features().MergeDirs
	if do == nil {
		return errors.New("MergeDirs not supported")
	}
	for _, dir := range dirs {
		_ = f.cache.RemoveDir(dir.Remote())
	}
	return do(ctx, dirs)
}

// DirCacheFlush flushes the dir cache
func (f *Fs) DirCacheFlush() {
	_ = f.cache.RemoveDir("")
}

// GetBackgroundUploadChannel returns a channel that can be listened to for remote activities that happen
// in the background
func (f *Fs) GetBackgroundUploadChannel() chan BackgroundUploadState {
	if f.opt.TempWritePath != "" {
		return f.backgroundRunner.notifyCh
	}
	return nil
}

func (f *Fs) isNotifiedRemote(remote string) bool {
	f.notifiedMu.Lock()
	defer f.notifiedMu.Unlock()

	n, ok := f.notifiedRemotes[remote]
	if !ok || !n {
		return false
	}

	delete(f.notifiedRemotes, remote)
	return n
}

func cleanPath(p string) string {
	p = path.Clean(p)
	if p == "." || p == "/" {
		p = ""
	}

	return p
}

// UserInfo returns info about the connected user
func (f *Fs) UserInfo(ctx context.Context) (map[string]string, error) {
	do := f.Fs.Features().UserInfo
	if do == nil {
		return nil, fs.ErrorNotImplemented
	}
	return do(ctx)
}

// Disconnect the current user
func (f *Fs) Disconnect(ctx context.Context) error {
	do := f.Fs.Features().Disconnect
	if do == nil {
		return fs.ErrorNotImplemented
	}
	return do(ctx)
}

// Shutdown the backend, closing any background tasks and any
// cached connections.
func (f *Fs) Shutdown(ctx context.Context) error {
	do := f.Fs.Features().Shutdown
	if do == nil {
		return nil
	}
	return do(ctx)
}

var commandHelp = []fs.CommandHelp{
	{
		Name:  "stats",
		Short: "Print stats on the cache backend in JSON format.",
	},
}

// Command the backend to run a named command
//
// The command run is name
// args may be used to read arguments from
// opts may be used to read optional arguments from
//
// The result should be capable of being JSON encoded
// If it is a string or a []string it will be shown to the user
// otherwise it will be JSON encoded and shown to the user like that
func (f *Fs) Command(ctx context.Context, name string, arg []string, opt map[string]string) (interface{}, error) {
	switch name {
	case "stats":
		return f.Stats()
	default:
		return nil, fs.ErrorCommandNotFound
	}
}

// Check the interfaces are satisfied
var (
	_ fs.Fs             = (*Fs)(nil)
	_ fs.Purger         = (*Fs)(nil)
	_ fs.Copier         = (*Fs)(nil)
	_ fs.Mover          = (*Fs)(nil)
	_ fs.DirMover       = (*Fs)(nil)
	_ fs.PutUncheckeder = (*Fs)(nil)
	_ fs.PutStreamer    = (*Fs)(nil)
	_ fs.CleanUpper     = (*Fs)(nil)
	_ fs.UnWrapper      = (*Fs)(nil)
	_ fs.Wrapper        = (*Fs)(nil)
	_ fs.ListRer        = (*Fs)(nil)
	_ fs.ChangeNotifier = (*Fs)(nil)
	_ fs.Abouter        = (*Fs)(nil)
	_ fs.UserInfoer     = (*Fs)(nil)
	_ fs.Disconnecter   = (*Fs)(nil)
	_ fs.Commander      = (*Fs)(nil)
	_ fs.MergeDirser    = (*Fs)(nil)
	_ fs.Shutdowner     = (*Fs)(nil)
)
