// +build !plan9,go1.7

package cache

import (
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"os"

	"os/signal"
	"syscall"

	"github.com/ncw/rclone/backend/crypt"
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/config"
	"github.com/ncw/rclone/fs/config/flags"
	"github.com/ncw/rclone/fs/config/obscure"
	"github.com/ncw/rclone/fs/hash"
	"github.com/ncw/rclone/fs/walk"
	"github.com/ncw/rclone/lib/atexit"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"golang.org/x/time/rate"
)

const (
	// DefCacheChunkSize is the default value for chunk size
	DefCacheChunkSize = "5M"
	// DefCacheTotalChunkSize is the default value for the maximum size of stored chunks
	DefCacheTotalChunkSize = "10G"
	// DefCacheChunkCleanInterval is the interval at which chunks are cleaned
	DefCacheChunkCleanInterval = "1m"
	// DefCacheInfoAge is the default value for object info age
	DefCacheInfoAge = "6h"
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
	DefCacheTmpWaitTime = "15m"
)

// Globals
var (
	// Flags
	cacheDbPath             = flags.StringP("cache-db-path", "", filepath.Join(config.CacheDir, "cache-backend"), "Directory to cache DB")
	cacheChunkPath          = flags.StringP("cache-chunk-path", "", filepath.Join(config.CacheDir, "cache-backend"), "Directory to cached chunk files")
	cacheDbPurge            = flags.BoolP("cache-db-purge", "", false, "Purge the cache DB before")
	cacheChunkSize          = flags.StringP("cache-chunk-size", "", DefCacheChunkSize, "The size of a chunk")
	cacheTotalChunkSize     = flags.StringP("cache-total-chunk-size", "", DefCacheTotalChunkSize, "The total size which the chunks can take up from the disk")
	cacheChunkCleanInterval = flags.StringP("cache-chunk-clean-interval", "", DefCacheChunkCleanInterval, "Interval at which chunk cleanup runs")
	cacheInfoAge            = flags.StringP("cache-info-age", "", DefCacheInfoAge, "How much time should object info be stored in cache")
	cacheReadRetries        = flags.IntP("cache-read-retries", "", DefCacheReadRetries, "How many times to retry a read from a cache storage")
	cacheTotalWorkers       = flags.IntP("cache-workers", "", DefCacheTotalWorkers, "How many workers should run in parallel to download chunks")
	cacheChunkNoMemory      = flags.BoolP("cache-chunk-no-memory", "", DefCacheChunkNoMemory, "Disable the in-memory cache for storing chunks during streaming")
	cacheRps                = flags.IntP("cache-rps", "", int(DefCacheRps), "Limits the number of requests per second to the source FS. -1 disables the rate limiter")
	cacheStoreWrites        = flags.BoolP("cache-writes", "", DefCacheWrites, "Will cache file data on writes through the FS")
	cacheTempWritePath      = flags.StringP("cache-tmp-upload-path", "", "", "Directory to keep temporary files until they are uploaded to the cloud storage")
	cacheTempWaitTime       = flags.StringP("cache-tmp-wait-time", "", DefCacheTmpWaitTime, "How long should files be stored in local cache before being uploaded")
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "cache",
		Description: "Cache a remote",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name: "remote",
			Help: "Remote to cache.\nNormally should contain a ':' and a path, eg \"myremote:path/to/dir\",\n\"myremote:bucket\" or maybe \"myremote:\" (not recommended).",
		}, {
			Name:     "plex_url",
			Help:     "Optional: The URL of the Plex server",
			Optional: true,
		}, {
			Name:     "plex_username",
			Help:     "Optional: The username of the Plex user",
			Optional: true,
		}, {
			Name:       "plex_password",
			Help:       "Optional: The password of the Plex user",
			IsPassword: true,
			Optional:   true,
		}, {
			Name: "chunk_size",
			Help: "The size of a chunk. Lower value good for slow connections but can affect seamless reading. \nDefault: " + DefCacheChunkSize,
			Examples: []fs.OptionExample{
				{
					Value: "1m",
					Help:  "1MB",
				}, {
					Value: "5M",
					Help:  "5 MB",
				}, {
					Value: "10M",
					Help:  "10 MB",
				},
			},
			Optional: true,
		}, {
			Name: "info_age",
			Help: "How much time should object info (file size, file hashes etc) be stored in cache. Use a very high value if you don't plan on changing the source FS from outside the cache. \nAccepted units are: \"s\", \"m\", \"h\".\nDefault: " + DefCacheInfoAge,
			Examples: []fs.OptionExample{
				{
					Value: "1h",
					Help:  "1 hour",
				}, {
					Value: "24h",
					Help:  "24 hours",
				}, {
					Value: "48h",
					Help:  "48 hours",
				},
			},
			Optional: true,
		}, {
			Name: "chunk_total_size",
			Help: "The maximum size of stored chunks. When the storage grows beyond this size, the oldest chunks will be deleted. \nDefault: " + DefCacheTotalChunkSize,
			Examples: []fs.OptionExample{
				{
					Value: "500M",
					Help:  "500 MB",
				}, {
					Value: "1G",
					Help:  "1 GB",
				}, {
					Value: "10G",
					Help:  "10 GB",
				},
			},
			Optional: true,
		}},
	})
}

// Fs represents a wrapped fs.Fs
type Fs struct {
	fs.Fs
	wrapper fs.Fs

	name     string
	root     string
	features *fs.Features // optional features
	cache    *Persistent

	fileAge            time.Duration
	chunkSize          int64
	chunkTotalSize     int64
	chunkCleanInterval time.Duration
	readRetries        int
	totalWorkers       int
	totalMaxWorkers    int
	chunkMemory        bool
	cacheWrites        bool
	tempWritePath      string
	tempWriteWait      time.Duration
	tempFs             fs.Fs

	lastChunkCleanup time.Time
	cleanupMu        sync.Mutex
	rateLimiter      *rate.Limiter
	plexConnector    *plexConnector
	backgroundRunner *backgroundWriter
	cleanupChan      chan bool
	parentsForgetFn  []func(string)
}

// parseRootPath returns a cleaned root path and a nil error or "" and an error when the path is invalid
func parseRootPath(path string) (string, error) {
	return strings.Trim(path, "/"), nil
}

// NewFs constructs a Fs from the path, container:path
func NewFs(name, rootPath string) (fs.Fs, error) {
	remote := config.FileGet(name, "remote")
	if strings.HasPrefix(remote, name+":") {
		return nil, errors.New("can't point cache remote at itself - check the value of the remote setting")
	}

	rpath, err := parseRootPath(rootPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to clean root path %q", rootPath)
	}

	remotePath := path.Join(remote, rpath)
	wrappedFs, wrapErr := fs.NewFs(remotePath)
	if wrapErr != nil && wrapErr != fs.ErrorIsFile {
		return nil, errors.Wrapf(wrapErr, "failed to make remote %q to wrap", remotePath)
	}
	var fsErr error
	fs.Debugf(name, "wrapped %v:%v at root %v", wrappedFs.Name(), wrappedFs.Root(), rpath)
	if wrapErr == fs.ErrorIsFile {
		fsErr = fs.ErrorIsFile
		rpath = cleanPath(path.Dir(rpath))
	}
	plexURL := config.FileGet(name, "plex_url")
	plexToken := config.FileGet(name, "plex_token")
	var chunkSize fs.SizeSuffix
	chunkSizeString := config.FileGet(name, "chunk_size", DefCacheChunkSize)
	if *cacheChunkSize != DefCacheChunkSize {
		chunkSizeString = *cacheChunkSize
	}
	err = chunkSize.Set(chunkSizeString)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to understand chunk size %v", chunkSizeString)
	}
	var chunkTotalSize fs.SizeSuffix
	chunkTotalSizeString := config.FileGet(name, "chunk_total_size", DefCacheTotalChunkSize)
	if *cacheTotalChunkSize != DefCacheTotalChunkSize {
		chunkTotalSizeString = *cacheTotalChunkSize
	}
	err = chunkTotalSize.Set(chunkTotalSizeString)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to understand chunk total size %v", chunkTotalSizeString)
	}
	chunkCleanIntervalStr := *cacheChunkCleanInterval
	chunkCleanInterval, err := time.ParseDuration(chunkCleanIntervalStr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to understand duration %v", chunkCleanIntervalStr)
	}
	infoAge := config.FileGet(name, "info_age", DefCacheInfoAge)
	if *cacheInfoAge != DefCacheInfoAge {
		infoAge = *cacheInfoAge
	}
	infoDuration, err := time.ParseDuration(infoAge)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to understand duration %v", infoAge)
	}
	waitTime, err := time.ParseDuration(*cacheTempWaitTime)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to understand duration %v", *cacheTempWaitTime)
	}
	// configure cache backend
	if *cacheDbPurge {
		fs.Debugf(name, "Purging the DB")
	}
	f := &Fs{
		Fs:                 wrappedFs,
		name:               name,
		root:               rpath,
		fileAge:            infoDuration,
		chunkSize:          int64(chunkSize),
		chunkTotalSize:     int64(chunkTotalSize),
		chunkCleanInterval: chunkCleanInterval,
		readRetries:        *cacheReadRetries,
		totalWorkers:       *cacheTotalWorkers,
		totalMaxWorkers:    *cacheTotalWorkers,
		chunkMemory:        !*cacheChunkNoMemory,
		cacheWrites:        *cacheStoreWrites,
		lastChunkCleanup:   time.Now().Truncate(time.Hour * 24 * 30),
		tempWritePath:      *cacheTempWritePath,
		tempWriteWait:      waitTime,
		cleanupChan:        make(chan bool, 1),
	}
	if f.chunkTotalSize < (f.chunkSize * int64(f.totalWorkers)) {
		return nil, errors.Errorf("don't set cache-total-chunk-size(%v) less than cache-chunk-size(%v) * cache-workers(%v)",
			f.chunkTotalSize, f.chunkSize, f.totalWorkers)
	}
	f.rateLimiter = rate.NewLimiter(rate.Limit(float64(*cacheRps)), f.totalWorkers)

	f.plexConnector = &plexConnector{}
	if plexURL != "" {
		if plexToken != "" {
			f.plexConnector, err = newPlexConnectorWithToken(f, plexURL, plexToken)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to connect to the Plex API %v", plexURL)
			}
		} else {
			plexUsername := config.FileGet(name, "plex_username")
			plexPassword := config.FileGet(name, "plex_password")
			if plexPassword != "" && plexUsername != "" {
				decPass, err := obscure.Reveal(plexPassword)
				if err != nil {
					decPass = plexPassword
				}
				f.plexConnector, err = newPlexConnector(f, plexURL, plexUsername, decPass)
				if err != nil {
					return nil, errors.Wrapf(err, "failed to connect to the Plex API %v", plexURL)
				}
			}
		}
	}

	dbPath := *cacheDbPath
	chunkPath := *cacheChunkPath
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
		PurgeDb: *cacheDbPurge,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to start cache db")
	}
	// Trap SIGINT and SIGTERM to close the DB handle gracefully
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP)
	atexit.Register(func() {
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

	fs.Infof(name, "Chunk Memory: %v", f.chunkMemory)
	fs.Infof(name, "Chunk Size: %v", fs.SizeSuffix(f.chunkSize))
	fs.Infof(name, "Chunk Total Size: %v", fs.SizeSuffix(f.chunkTotalSize))
	fs.Infof(name, "Chunk Clean Interval: %v", f.chunkCleanInterval.String())
	fs.Infof(name, "Workers: %v", f.totalWorkers)
	fs.Infof(name, "File Age: %v", f.fileAge.String())
	if f.cacheWrites {
		fs.Infof(name, "Cache Writes: enabled")
	}

	if f.tempWritePath != "" {
		err = os.MkdirAll(f.tempWritePath, os.ModePerm)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create cache directory %v", f.tempWritePath)
		}
		f.tempWritePath = filepath.ToSlash(f.tempWritePath)
		f.tempFs, err = fs.NewFs(f.tempWritePath)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create temp fs: %v", err)
		}
		fs.Infof(name, "Upload Temp Rest Time: %v", f.tempWriteWait.String())
		fs.Infof(name, "Upload Temp FS: %v", f.tempWritePath)
		f.backgroundRunner, _ = initBackgroundUploader(f)
		go f.backgroundRunner.run()
	}

	go func() {
		for {
			time.Sleep(f.chunkCleanInterval)
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

	if doDirChangeNotify := wrappedFs.Features().DirChangeNotify; doDirChangeNotify != nil {
		doDirChangeNotify(f.receiveDirChangeNotify, f.chunkCleanInterval)
	}

	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
		DuplicateFiles:          false, // storage doesn't permit this
	}).Fill(f).Mask(wrappedFs).WrapsFs(f, wrappedFs)
	// override only those features that use a temp fs and it doesn't support them
	f.features.DirChangeNotify = f.DirChangeNotify
	if f.tempWritePath != "" {
		if f.tempFs.Features().Copy == nil {
			f.features.Copy = nil
		}
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

	return f, fsErr
}

func (f *Fs) receiveDirChangeNotify(forgetPath string) {
	fs.Debugf(f, "notify: expiring cache for '%v'", forgetPath)
	// notify upstreams too (vfs)
	f.notifyDirChange(forgetPath)

	var cd *Directory
	co := NewObject(f, forgetPath)
	err := f.cache.GetObject(co)
	if err == nil {
		cd = NewDirectory(f, cleanPath(path.Dir(co.Remote())))
	} else {
		cd = NewDirectory(f, forgetPath)
	}

	// we list all the cached objects and expire all of them
	entries, err := f.cache.GetDirEntries(cd)
	if err != nil {
		fs.Debugf(forgetPath, "notify: ignoring notification on non cached dir")
		return
	}
	for i := 0; i < len(entries); i++ {
		if co, ok := entries[i].(*Object); ok {
			co.CacheTs = time.Now().Add(f.fileAge * -1)
			err = f.cache.AddObject(co)
			if err != nil {
				fs.Errorf(forgetPath, "notify: error expiring '%v': %v", co, err)
			} else {
				fs.Debugf(forgetPath, "notify: expired %v", co)
			}
		}
	}
	// finally, we expire the dir as well
	err = f.cache.ExpireDir(cd)
	if err != nil {
		fs.Errorf(forgetPath, "notify: error expiring '%v': %v", cd, err)
	} else {
		fs.Debugf(forgetPath, "notify: expired '%v'", cd)
	}
}

// notifyDirChange takes a remote (can be dir or entry) and
// tries to determine which is it and notify upstreams of the dir change
func (f *Fs) notifyDirChange(remote string) {
	var cd *Directory
	co := NewObject(f, remote)
	err := f.cache.GetObject(co)
	if err == nil {
		pd := cleanPath(path.Dir(remote))
		cd = NewDirectory(f, pd)
	} else {
		cd = NewDirectory(f, remote)
	}

	f.notifyDirChangeUpstream(cd.Remote())
}

// notifyDirChangeUpstream will loop through all the upstreams and notify
// of the provided remote (should be only a dir)
func (f *Fs) notifyDirChangeUpstream(remote string) {
	if len(f.parentsForgetFn) > 0 {
		for _, fn := range f.parentsForgetFn {
			fn(remote)
		}
	}
}

// DirChangeNotify can subsribe multiple callers
// this is coupled with the wrapped fs DirChangeNotify (if it supports it)
// and also notifies other caches (i.e VFS) to clear out whenever something changes
func (f *Fs) DirChangeNotify(notifyFunc func(string), pollInterval time.Duration) chan bool {
	fs.Debugf(f, "subscribing to DirChangeNotify")
	f.parentsForgetFn = append(f.parentsForgetFn, notifyFunc)
	return make(chan bool)
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
	return f.chunkSize
}

// InfoAge returns the configured file age
func (f *Fs) InfoAge() time.Duration {
	return f.fileAge
}

// TempUploadWaitTime returns the configured temp file upload wait time
func (f *Fs) TempUploadWaitTime() time.Duration {
	return f.tempWriteWait
}

// NewObject finds the Object at remote.
func (f *Fs) NewObject(remote string) (fs.Object, error) {
	var err error

	fs.Debugf(f, "new object '%s'", remote)
	co := NewObject(f, remote)
	// search for entry in cache and validate it
	err = f.cache.GetObject(co)
	if err != nil {
		fs.Debugf(remote, "find: error: %v", err)
	} else if time.Now().After(co.CacheTs.Add(f.fileAge)) {
		fs.Debugf(co, "find: cold object: %+v", co)
	} else {
		fs.Debugf(co, "find: warm object: %v, expiring on: %v", co, co.CacheTs.Add(f.fileAge))
		return co, nil
	}

	// search for entry in source or temp fs
	var obj fs.Object
	err = nil
	if f.tempWritePath != "" {
		obj, err = f.tempFs.NewObject(remote)
		// not found in temp fs
		if err != nil {
			fs.Debugf(remote, "find: not found in local cache fs")
			obj, err = f.Fs.NewObject(remote)
		} else {
			fs.Debugf(obj, "find: found in local cache fs")
		}
	} else {
		obj, err = f.Fs.NewObject(remote)
	}

	// not found in either fs
	if err != nil {
		fs.Debugf(obj, "find failed: not found in either local or remote fs")
		return nil, err
	}

	// cache the new entry
	co = ObjectFromOriginal(f, obj).persist()
	fs.Debugf(co, "find: cached object")
	return co, nil
}

// List the objects and directories in dir into entries
func (f *Fs) List(dir string) (entries fs.DirEntries, err error) {
	fs.Debugf(f, "list '%s'", dir)
	cd := ShallowDirectory(f, dir)

	// search for cached dir entries and validate them
	entries, err = f.cache.GetDirEntries(cd)
	if err != nil {
		fs.Debugf(dir, "list: error: %v", err)
	} else if time.Now().After(cd.CacheTs.Add(f.fileAge)) {
		fs.Debugf(dir, "list: cold listing: %v", cd.CacheTs)
	} else if len(entries) == 0 {
		// TODO: read empty dirs from source?
		fs.Debugf(dir, "list: empty listing")
	} else {
		fs.Debugf(dir, "list: warm %v from cache for: %v, expiring on: %v", len(entries), cd.abs(), cd.CacheTs.Add(f.fileAge))
		fs.Debugf(dir, "list: cached entries: %v", entries)
		return entries, nil
	}
	// FIXME need to clean existing cached listing

	// we first search any temporary files stored locally
	var cachedEntries fs.DirEntries
	if f.tempWritePath != "" {
		queuedEntries, err := f.cache.searchPendingUploadFromDir(cd.abs())
		if err != nil {
			fs.Errorf(dir, "list: error getting pending uploads: %v", err)
		} else {
			fs.Debugf(dir, "list: read %v from temp fs", len(queuedEntries))
			fs.Debugf(dir, "list: temp fs entries: %v", queuedEntries)

			for _, queuedRemote := range queuedEntries {
				queuedEntry, err := f.tempFs.NewObject(f.cleanRootFromPath(queuedRemote))
				if err != nil {
					fs.Debugf(dir, "list: temp file not found in local fs: %v", err)
					continue
				}
				co := ObjectFromOriginal(f, queuedEntry).persist()
				fs.Debugf(co, "list: cached temp object")
				cachedEntries = append(cachedEntries, co)
			}
		}
	}

	// search from the source
	entries, err = f.Fs.List(dir)
	if err != nil {
		return nil, err
	}
	fs.Debugf(dir, "list: read %v from source", len(entries))
	fs.Debugf(dir, "list: source entries: %v", entries)

	// and then iterate over the ones from source (temp Objects will override source ones)
	for _, entry := range entries {
		switch o := entry.(type) {
		case fs.Object:
			// skip over temporary objects (might be uploading)
			found := false
			for _, t := range cachedEntries {
				if t.Remote() == o.Remote() {
					found = true
					break
				}
			}
			if found {
				continue
			}
			co := ObjectFromOriginal(f, o).persist()
			cachedEntries = append(cachedEntries, co)
			fs.Debugf(dir, "list: cached object: %v", co)
		case fs.Directory:
			cdd := DirectoryFromOriginal(f, o)
			err := f.cache.AddDir(cdd)
			if err != nil {
				fs.Errorf(dir, "list: error caching dir from listing %v", o)
			} else {
				fs.Debugf(dir, "list: cached dir: %v", cdd)
			}
			cachedEntries = append(cachedEntries, cdd)
		default:
			fs.Debugf(entry, "list: Unknown object type %T", entry)
		}
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

func (f *Fs) recurse(dir string, list *walk.ListRHelper) error {
	entries, err := f.List(dir)
	if err != nil {
		return err
	}

	for i := 0; i < len(entries); i++ {
		innerDir, ok := entries[i].(fs.Directory)
		if ok {
			err := f.recurse(innerDir.Remote(), list)
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
func (f *Fs) ListR(dir string, callback fs.ListRCallback) (err error) {
	fs.Debugf(f, "list recursively from '%s'", dir)

	// we check if the source FS supports ListR
	// if it does, we'll use that to get all the entries, cache them and return
	do := f.Fs.Features().ListR
	if do != nil {
		return do(dir, func(entries fs.DirEntries) error {
			// we got called back with a set of entries so let's cache them and call the original callback
			for _, entry := range entries {
				switch o := entry.(type) {
				case fs.Object:
					_ = f.cache.AddObject(ObjectFromOriginal(f, o))
				case fs.Directory:
					_ = f.cache.AddDir(DirectoryFromOriginal(f, o))
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
	err = f.recurse(dir, list)
	if err != nil {
		return err
	}

	return list.Flush()
}

// Mkdir makes the directory (container, bucket)
func (f *Fs) Mkdir(dir string) error {
	fs.Debugf(f, "mkdir '%s'", dir)
	err := f.Fs.Mkdir(dir)
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
	// advertise to DirChangeNotify if wrapped doesn't do that
	if f.Fs.Features().DirChangeNotify == nil {
		f.notifyDirChangeUpstream(parentCd.Remote())
	}

	return nil
}

// Rmdir removes the directory (container, bucket) if empty
func (f *Fs) Rmdir(dir string) error {
	fs.Debugf(f, "rmdir '%s'", dir)

	if f.tempWritePath != "" {
		// pause background uploads
		f.backgroundRunner.pause()
		defer f.backgroundRunner.play()

		// we check if the source exists on the remote and make the same move on it too if it does
		// otherwise, we skip this step
		_, err := f.UnWrap().List(dir)
		if err == nil {
			err := f.Fs.Rmdir(dir)
			if err != nil {
				return err
			}
			fs.Debugf(dir, "rmdir: removed dir in source fs")
		}

		var queuedEntries []*Object
		err = walk.Walk(f.tempFs, dir, true, -1, func(path string, entries fs.DirEntries, err error) error {
			for _, o := range entries {
				if oo, ok := o.(fs.Object); ok {
					co := ObjectFromOriginal(f, oo)
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
				fs.Errorf(dir, "rmdir: temporary dir not empty")
				return fs.ErrorDirectoryNotEmpty
			}
		}
	} else {
		err := f.Fs.Rmdir(dir)
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
	// advertise to DirChangeNotify if wrapped doesn't do that
	if f.Fs.Features().DirChangeNotify == nil {
		f.notifyDirChangeUpstream(parentCd.Remote())
	}

	return nil
}

// DirMove moves src, srcRemote to this remote at dstRemote
// using server side move operations.
func (f *Fs) DirMove(src fs.Fs, srcRemote, dstRemote string) error {
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

	if f.tempWritePath != "" {
		// pause background uploads
		f.backgroundRunner.pause()
		defer f.backgroundRunner.play()

		// we check if the source exists on the remote and make the same move on it too if it does
		// otherwise, we skip this step
		_, err := srcFs.UnWrap().List(srcRemote)
		if err == nil {
			err := do(srcFs.UnWrap(), srcRemote, dstRemote)
			if err != nil {
				return err
			}
			fs.Debugf(srcRemote, "movedir: dir moved in the source fs")
		}

		var queuedEntries []*Object
		err = walk.Walk(f.tempFs, srcRemote, true, -1, func(path string, entries fs.DirEntries, err error) error {
			for _, o := range entries {
				if oo, ok := o.(fs.Object); ok {
					co := ObjectFromOriginal(f, oo)
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
		err = do(f.tempFs, srcRemote, dstRemote)
		if err != nil {
			return err
		}
		err = f.cache.ReconcileTempUploads(f)
		if err != nil {
			return err
		}
	} else {
		err := do(srcFs.UnWrap(), srcRemote, dstRemote)
		if err != nil {
			return err
		}
		fs.Debugf(srcRemote, "movedir: dir moved in the source fs")
	}

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
	// advertise to DirChangeNotify if wrapped doesn't do that
	if f.Fs.Features().DirChangeNotify == nil {
		f.notifyDirChangeUpstream(srcParent.Remote())
	}

	// expire parent dir at the destination path
	dstParent := NewDirectory(f, cleanPath(path.Dir(dstRemote)))
	err = f.cache.ExpireDir(dstParent)
	if err != nil {
		fs.Errorf(dstParent, "dirmove: cache expire error: %v", err)
	} else {
		fs.Debugf(dstParent, "dirmove: cache expired")
	}
	// advertise to DirChangeNotify if wrapped doesn't do that
	if f.Fs.Features().DirChangeNotify == nil {
		f.notifyDirChangeUpstream(dstParent.Remote())
	}
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
			chunk := make([]byte, f.chunkSize)
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

type putFn func(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error)

// put in to the remote path
func (f *Fs) put(in io.Reader, src fs.ObjectInfo, options []fs.OpenOption, put putFn) (fs.Object, error) {
	var err error
	var obj fs.Object

	// queue for upload and store in temp fs if configured
	if f.tempWritePath != "" {
		obj, err = f.tempFs.Put(in, src, options...)
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
	} else if f.cacheWrites {
		f.cacheReader(in, src, func(inn io.Reader) {
			obj, err = put(inn, src, options...)
		})
		if err == nil {
			fs.Debugf(obj, "put: uploaded to remote fs and saved in cache")
		}
		// last option: save it directly in remote fs
	} else {
		obj, err = put(in, src, options...)
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
	cachedObj := ObjectFromOriginal(f, obj).persist()
	fs.Debugf(cachedObj, "put: added to cache")
	// expire parent
	parentCd := NewDirectory(f, cleanPath(path.Dir(cachedObj.Remote())))
	err = f.cache.ExpireDir(parentCd)
	if err != nil {
		fs.Errorf(cachedObj, "put: cache expire error: %v", err)
	} else {
		fs.Infof(parentCd, "put: cache expired")
	}
	// advertise to DirChangeNotify if wrapped doesn't do that
	if f.Fs.Features().DirChangeNotify == nil {
		f.notifyDirChangeUpstream(parentCd.Remote())
	}

	return cachedObj, nil
}

// Put in to the remote path with the modTime given of the given size
func (f *Fs) Put(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	fs.Debugf(f, "put data at '%s'", src.Remote())
	return f.put(in, src, options, f.Fs.Put)
}

// PutUnchecked uploads the object
func (f *Fs) PutUnchecked(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	do := f.Fs.Features().PutUnchecked
	if do == nil {
		return nil, errors.New("can't PutUnchecked")
	}
	fs.Debugf(f, "put data unchecked in '%s'", src.Remote())
	return f.put(in, src, options, do)
}

// PutStream uploads the object
func (f *Fs) PutStream(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	do := f.Fs.Features().PutStream
	if do == nil {
		return nil, errors.New("can't PutStream")
	}
	fs.Debugf(f, "put data streaming in '%s'", src.Remote())
	return f.put(in, src, options, do)
}

// Copy src to this remote using server side copy operations.
func (f *Fs) Copy(src fs.Object, remote string) (fs.Object, error) {
	fs.Debugf(f, "copy obj '%s' -> '%s'", src, remote)

	do := f.Fs.Features().Copy
	if do == nil {
		fs.Errorf(src, "source remote (%v) doesn't support Copy", src.Fs())
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
	if err := srcObj.refreshFromSource(false); err != nil {
		fs.Errorf(f, "can't copy %v - %v", src, err)
		return nil, fs.ErrorCantCopy
	}

	if srcObj.isTempFile() {
		// we check if the feature is stil active
		if f.tempWritePath == "" {
			fs.Errorf(srcObj, "can't copy - this is a local cached file but this feature is turned off this run")
			return nil, fs.ErrorCantCopy
		}

		do = srcObj.ParentFs.Features().Copy
		if do == nil {
			fs.Errorf(src, "parent remote (%v) doesn't support Copy", srcObj.ParentFs)
			return nil, fs.ErrorCantCopy
		}
	}

	obj, err := do(srcObj.Object, remote)
	if err != nil {
		fs.Errorf(srcObj, "error moving in cache: %v", err)
		return nil, err
	}
	fs.Debugf(obj, "copy: file copied")

	// persist new
	co := ObjectFromOriginal(f, obj).persist()
	fs.Debugf(co, "copy: added to cache")
	// expire the destination path
	parentCd := NewDirectory(f, cleanPath(path.Dir(co.Remote())))
	err = f.cache.ExpireDir(parentCd)
	if err != nil {
		fs.Errorf(parentCd, "copy: cache expire error: %v", err)
	} else {
		fs.Infof(parentCd, "copy: cache expired")
	}
	// advertise to DirChangeNotify if wrapped doesn't do that
	if f.Fs.Features().DirChangeNotify == nil {
		f.notifyDirChangeUpstream(parentCd.Remote())
	}
	// expire src parent
	srcParent := NewDirectory(f, cleanPath(path.Dir(src.Remote())))
	err = f.cache.ExpireDir(srcParent)
	if err != nil {
		fs.Errorf(srcParent, "copy: cache expire error: %v", err)
	} else {
		fs.Infof(srcParent, "copy: cache expired")
	}
	// advertise to DirChangeNotify if wrapped doesn't do that
	if f.Fs.Features().DirChangeNotify == nil {
		f.notifyDirChangeUpstream(srcParent.Remote())
	}

	return co, nil
}

// Move src to this remote using server side move operations.
func (f *Fs) Move(src fs.Object, remote string) (fs.Object, error) {
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
	if err := srcObj.refreshFromSource(false); err != nil {
		fs.Errorf(f, "can't move %v - %v", src, err)
		return nil, fs.ErrorCantMove
	}

	// if this is a temp object then we perform the changes locally
	if srcObj.isTempFile() {
		// we check if the feature is stil active
		if f.tempWritePath == "" {
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

	obj, err := do(srcObj.Object, remote)
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
	// advertise to DirChangeNotify if wrapped doesn't do that
	if f.Fs.Features().DirChangeNotify == nil {
		f.notifyDirChangeUpstream(parentCd.Remote())
	}
	// persist new
	cachedObj := ObjectFromOriginal(f, obj).persist()
	fs.Debugf(cachedObj, "move: added to cache")
	// expire new parent
	parentCd = NewDirectory(f, cleanPath(path.Dir(cachedObj.Remote())))
	err = f.cache.ExpireDir(parentCd)
	if err != nil {
		fs.Errorf(parentCd, "move: expire error: %v", err)
	} else {
		fs.Infof(parentCd, "move: cache expired")
	}
	// advertise to DirChangeNotify if wrapped doesn't do that
	if f.Fs.Features().DirChangeNotify == nil {
		f.notifyDirChangeUpstream(parentCd.Remote())
	}

	return cachedObj, nil
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return f.Fs.Hashes()
}

// Purge all files in the root and the root directory
func (f *Fs) Purge() error {
	fs.Infof(f, "purging cache")
	f.cache.Purge()

	do := f.Fs.Features().Purge
	if do == nil {
		return nil
	}

	err := do()
	if err != nil {
		return err
	}

	return nil
}

// CleanUp the trash in the Fs
func (f *Fs) CleanUp() error {
	f.CleanUpCache(false)

	do := f.Fs.Features().CleanUp
	if do == nil {
		return nil
	}

	return do()
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

	if ignoreLastTs || time.Now().After(f.lastChunkCleanup.Add(f.chunkCleanInterval)) {
		f.cache.CleanChunksBySize(f.chunkTotalSize)
		f.lastChunkCleanup = time.Now()
	}
}

// StopBackgroundRunners will signall all the runners to stop their work
// can be triggered from a terminate signal or from testing between runs
func (f *Fs) StopBackgroundRunners() {
	f.cleanupChan <- false
	if f.tempWritePath != "" && f.backgroundRunner != nil && f.backgroundRunner.isRunning() {
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

// DirCacheFlush flushes the dir cache
func (f *Fs) DirCacheFlush() {
	_ = f.cache.RemoveDir("")
}

// GetBackgroundUploadChannel returns a channel that can be listened to for remote activities that happen
// in the background
func (f *Fs) GetBackgroundUploadChannel() chan BackgroundUploadState {
	if f.tempWritePath != "" {
		return f.backgroundRunner.notifyCh
	}
	return nil
}

func cleanPath(p string) string {
	p = path.Clean(p)
	if p == "." || p == "/" {
		p = ""
	}

	return p
}

// Check the interfaces are satisfied
var (
	_ fs.Fs                = (*Fs)(nil)
	_ fs.Purger            = (*Fs)(nil)
	_ fs.Copier            = (*Fs)(nil)
	_ fs.Mover             = (*Fs)(nil)
	_ fs.DirMover          = (*Fs)(nil)
	_ fs.PutUncheckeder    = (*Fs)(nil)
	_ fs.PutStreamer       = (*Fs)(nil)
	_ fs.CleanUpper        = (*Fs)(nil)
	_ fs.UnWrapper         = (*Fs)(nil)
	_ fs.Wrapper           = (*Fs)(nil)
	_ fs.ListRer           = (*Fs)(nil)
	_ fs.DirChangeNotifier = (*Fs)(nil)
)
