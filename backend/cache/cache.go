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
)

// Globals
var (
	// Flags
	cacheDbPath             = fs.StringP("cache-db-path", "", filepath.Join(fs.CacheDir, "cache-backend"), "Directory to cache DB")
	cacheChunkPath          = fs.StringP("cache-chunk-path", "", filepath.Join(fs.CacheDir, "cache-backend"), "Directory to cached chunk files")
	cacheDbPurge            = fs.BoolP("cache-db-purge", "", false, "Purge the cache DB before")
	cacheChunkSize          = fs.StringP("cache-chunk-size", "", DefCacheChunkSize, "The size of a chunk")
	cacheTotalChunkSize     = fs.StringP("cache-total-chunk-size", "", DefCacheTotalChunkSize, "The total size which the chunks can take up from the disk")
	cacheChunkCleanInterval = fs.StringP("cache-chunk-clean-interval", "", DefCacheChunkCleanInterval, "Interval at which chunk cleanup runs")
	cacheInfoAge            = fs.StringP("cache-info-age", "", DefCacheInfoAge, "How much time should object info be stored in cache")
	cacheReadRetries        = fs.IntP("cache-read-retries", "", DefCacheReadRetries, "How many times to retry a read from a cache storage")
	cacheTotalWorkers       = fs.IntP("cache-workers", "", DefCacheTotalWorkers, "How many workers should run in parallel to download chunks")
	cacheChunkNoMemory      = fs.BoolP("cache-chunk-no-memory", "", DefCacheChunkNoMemory, "Disable the in-memory cache for storing chunks during streaming")
	cacheRps                = fs.IntP("cache-rps", "", int(DefCacheRps), "Limits the number of requests per second to the source FS. -1 disables the rate limiter")
	cacheStoreWrites        = fs.BoolP("cache-writes", "", DefCacheWrites, "Will cache file data on writes through the FS")
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

// ChunkStorage is a storage type that supports only chunk operations (i.e in RAM)
type ChunkStorage interface {
	// will check if the chunk is in storage. should be fast and not read the chunk itself if possible
	HasChunk(cachedObject *Object, offset int64) bool

	// returns the chunk in storage. return an error if it's not
	GetChunk(cachedObject *Object, offset int64) ([]byte, error)

	// add a new chunk
	AddChunk(fp string, data []byte, offset int64) error

	// if the storage can cleanup on a cron basis
	// otherwise it can do a noop operation
	CleanChunksByAge(chunkAge time.Duration)

	// if the storage can cleanup chunks after we no longer need them
	// otherwise it can do a noop operation
	CleanChunksByNeed(offset int64)

	// if the storage can cleanup chunks after the total size passes a certain point
	// otherwise it can do a noop operation
	CleanChunksBySize(maxSize int64)
}

// Storage is a storage type (Bolt) which needs to support both chunk and file based operations
type Storage interface {
	ChunkStorage

	// will update/create a directory or an error if it's not found
	AddDir(cachedDir *Directory) error

	// will return a directory with all the entries in it or an error if it's not found
	GetDirEntries(cachedDir *Directory) (fs.DirEntries, error)

	// remove a directory and all the objects and chunks in it
	RemoveDir(fp string) error

	// remove a directory and all the objects and chunks in it
	ExpireDir(cd *Directory) error

	// will return an object (file) or error if it doesn't find it
	GetObject(cachedObject *Object) (err error)

	// add a new object to its parent directory
	// the directory structure (all the parents of this object) is created if its not found
	AddObject(cachedObject *Object) error

	// remove an object and all its chunks
	RemoveObject(fp string) error

	// Stats returns stats about the cache storage
	Stats() (map[string]map[string]interface{}, error)

	// Purge will flush the entire cache
	Purge()

	// Close should be called when the program ends gracefully
	Close()
}

// Fs represents a wrapped fs.Fs
type Fs struct {
	fs.Fs
	wrapper fs.Fs

	name     string
	root     string
	features *fs.Features // optional features
	cache    Storage

	fileAge            time.Duration
	chunkSize          int64
	chunkTotalSize     int64
	chunkCleanInterval time.Duration
	readRetries        int
	totalWorkers       int
	totalMaxWorkers    int
	chunkMemory        bool
	cacheWrites        bool

	lastChunkCleanup time.Time
	cleanupMu        sync.Mutex
	rateLimiter      *rate.Limiter
	plexConnector    *plexConnector
}

// NewFs contstructs an Fs from the path, container:path
func NewFs(name, rpath string) (fs.Fs, error) {
	remote := fs.ConfigFileGet(name, "remote")
	if strings.HasPrefix(remote, name+":") {
		return nil, errors.New("can't point cache remote at itself - check the value of the remote setting")
	}
	// Look for a file first
	remotePath := path.Join(remote, rpath)
	wrappedFs, wrapErr := fs.NewFs(remotePath)
	if wrapErr != fs.ErrorIsFile && wrapErr != nil {
		return nil, errors.Wrapf(wrapErr, "failed to make remote %q to wrap", remotePath)
	}
	fs.Debugf(name, "wrapped %v:%v at root %v", wrappedFs.Name(), wrappedFs.Root(), rpath)

	plexURL := fs.ConfigFileGet(name, "plex_url")
	plexToken := fs.ConfigFileGet(name, "plex_token")
	var chunkSize fs.SizeSuffix
	chunkSizeString := fs.ConfigFileGet(name, "chunk_size", DefCacheChunkSize)
	if *cacheChunkSize != DefCacheChunkSize {
		chunkSizeString = *cacheChunkSize
	}
	err := chunkSize.Set(chunkSizeString)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to understand chunk size", chunkSizeString)
	}
	var chunkTotalSize fs.SizeSuffix
	chunkTotalSizeString := fs.ConfigFileGet(name, "chunk_total_size", DefCacheTotalChunkSize)
	if *cacheTotalChunkSize != DefCacheTotalChunkSize {
		chunkTotalSizeString = *cacheTotalChunkSize
	}
	err = chunkTotalSize.Set(chunkTotalSizeString)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to understand chunk total size", chunkTotalSizeString)
	}
	chunkCleanIntervalStr := *cacheChunkCleanInterval
	chunkCleanInterval, err := time.ParseDuration(chunkCleanIntervalStr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to understand duration %v", chunkCleanIntervalStr)
	}
	infoAge := fs.ConfigFileGet(name, "info_age", DefCacheInfoAge)
	if *cacheInfoAge != DefCacheInfoAge {
		infoAge = *cacheInfoAge
	}
	infoDuration, err := time.ParseDuration(infoAge)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to understand duration", infoAge)
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
			plexUsername := fs.ConfigFileGet(name, "plex_username")
			plexPassword := fs.ConfigFileGet(name, "plex_password")
			if plexPassword != "" && plexUsername != "" {
				decPass, err := fs.Reveal(plexPassword)
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
	if dbPath != filepath.Join(fs.CacheDir, "cache-backend") &&
		chunkPath == filepath.Join(fs.CacheDir, "cache-backend") {
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
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		for {
			s := <-c
			if s == syscall.SIGINT || s == syscall.SIGTERM {
				fs.Debugf(f, "Got signal: %v", s)
				f.cache.Close()
			} else if s == syscall.SIGHUP {
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
	fs.Infof(name, "Cache Writes: %v", f.cacheWrites)

	go f.CleanUpCache(false)

	// TODO: Explore something here but now it's not something we want
	// when writing from cache, source FS will send a notification and clear it out immediately
	//setup dir notification
	//doDirChangeNotify := wrappedFs.Features().DirChangeNotify
	//if doDirChangeNotify != nil {
	//	doDirChangeNotify(func(dir string) {
	//		d := NewAbsDirectory(f, dir)
	//		d.Flush()
	//		fs.Infof(dir, "updated from notification")
	//	}, time.Second * 10)
	//}

	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
		DuplicateFiles:          false, // storage doesn't permit this
		Purge:                   f.Purge,
		Copy:                    f.Copy,
		Move:                    f.Move,
		DirMove:                 f.DirMove,
		DirChangeNotify:         nil,
		PutUnchecked:            f.PutUnchecked,
		PutStream:               f.PutStream,
		CleanUp:                 f.CleanUp,
		UnWrap:                  f.UnWrap,
		WrapFs:                  f.WrapFs,
		SetWrapper:              f.SetWrapper,
	}).Fill(f).Mask(wrappedFs).WrapsFs(f, wrappedFs)
	f.features.DirCacheFlush = f.DirCacheFlush

	return f, wrapErr
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
	return fmt.Sprintf("%s:%s", f.name, f.root)
}

// ChunkSize returns the configured chunk size
func (f *Fs) ChunkSize() int64 {
	return f.chunkSize
}

// NewObject finds the Object at remote.
func (f *Fs) NewObject(remote string) (fs.Object, error) {
	co := NewObject(f, remote)
	err := f.cache.GetObject(co)
	if err != nil {
		fs.Debugf(remote, "find: error: %v", err)
	} else if time.Now().After(co.CacheTs.Add(f.fileAge)) {
		fs.Debugf(remote, "find: cold object ts: %v", co.CacheTs)
	} else {
		fs.Debugf(remote, "find: warm object ts: %v", co.CacheTs)
		return co, nil
	}
	obj, err := f.Fs.NewObject(remote)
	if err != nil {
		return nil, err
	}
	co = ObjectFromOriginal(f, obj)
	co.persist()
	return co, nil
}

// List the objects and directories in dir into entries
func (f *Fs) List(dir string) (entries fs.DirEntries, err error) {
	// clean cache
	go f.CleanUpCache(false)

	cd := ShallowDirectory(f, dir)
	entries, err = f.cache.GetDirEntries(cd)
	if err != nil {
		fs.Debugf(dir, "list: error: %v", err)
	} else if time.Now().After(cd.CacheTs.Add(f.fileAge)) {
		fs.Debugf(dir, "list: cold listing: %v", cd.CacheTs)
	} else if len(entries) == 0 {
		// TODO: read empty dirs from source?
		fs.Debugf(dir, "list: empty listing")
	} else {
		fs.Debugf(dir, "list: warm %v from cache for: %v, ts: %v", len(entries), cd.abs(), cd.CacheTs)
		return entries, nil
	}

	entries, err = f.Fs.List(dir)
	if err != nil {
		return nil, err
	}
	fs.Debugf(dir, "list: read %v from source", len(entries))

	var cachedEntries fs.DirEntries
	for _, entry := range entries {
		switch o := entry.(type) {
		case fs.Object:
			co := ObjectFromOriginal(f, o)
			co.persist()
			cachedEntries = append(cachedEntries, co)
		case fs.Directory:
			cd := DirectoryFromOriginal(f, o)
			err = f.cache.AddDir(cd)
			cachedEntries = append(cachedEntries, cd)
		default:
			err = errors.Errorf("Unknown object type %T", entry)
		}
	}
	if err != nil {
		fs.Errorf(dir, "err caching listing: %v", err)
	} else {
		t := time.Now()
		cd.CacheTs = &t
		err := f.cache.AddDir(cd)
		if err != nil {
			fs.Errorf(cd, "list: save error: %v", err)
		}
	}

	return cachedEntries, nil
}

func (f *Fs) recurse(dir string, list *fs.ListRHelper) error {
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
	list := fs.NewListRHelper(callback)
	err = f.recurse(dir, list)
	if err != nil {
		return err
	}

	return list.Flush()
}

// Mkdir makes the directory (container, bucket)
func (f *Fs) Mkdir(dir string) error {
	err := f.Fs.Mkdir(dir)
	if err != nil {
		return err
	}
	if dir == "" && f.Root() == "" { // creating the root is possible but we don't need that cached as we have it already
		fs.Debugf(dir, "skipping empty dir in cache")
		return nil
	}
	fs.Infof(f, "create dir '%s'", dir)

	// expire parent of new dir
	cd := NewDirectory(f, cleanPath(dir))
	err = f.cache.AddDir(cd)
	if err != nil {
		fs.Errorf(dir, "mkdir: add error: %v", err)
	}
	parentCd := NewDirectory(f, cleanPath(path.Dir(dir)))
	err = f.cache.ExpireDir(parentCd)
	if err != nil {
		fs.Errorf(dir, "mkdir: expire error: %v", err)
	}

	// clean cache
	go f.CleanUpCache(false)
	return nil
}

// Rmdir removes the directory (container, bucket) if empty
func (f *Fs) Rmdir(dir string) error {
	err := f.Fs.Rmdir(dir)
	if err != nil {
		return err
	}
	fs.Infof(f, "rm dir '%s'", dir)

	// remove dir data
	d := NewDirectory(f, dir)
	err = f.cache.RemoveDir(d.abs())
	if err != nil {
		fs.Errorf(dir, "rmdir: remove error: %v", err)
	}
	// expire parent
	parentCd := NewDirectory(f, cleanPath(path.Dir(dir)))
	err = f.cache.ExpireDir(parentCd)
	if err != nil {
		fs.Errorf(dir, "rmdir: expire error: %v", err)
	}

	// clean cache
	go f.CleanUpCache(false)
	return nil
}

// DirMove moves src, srcRemote to this remote at dstRemote
// using server side move operations.
func (f *Fs) DirMove(src fs.Fs, srcRemote, dstRemote string) error {
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
	fs.Infof(f, "move dir '%s'/'%s' -> '%s'", srcRemote, srcFs.Root(), dstRemote)

	err := do(src.Features().UnWrap(), srcRemote, dstRemote)
	if err != nil {
		return err
	}

	// delete src dir from cache along with all chunks
	srcDir := NewDirectory(srcFs, srcRemote)
	err = f.cache.RemoveDir(srcDir.abs())
	if err != nil {
		fs.Errorf(srcRemote, "dirmove: remove error: %v", err)
	}
	// expire src parent
	srcParent := NewDirectory(f, cleanPath(path.Dir(srcRemote)))
	err = f.cache.ExpireDir(srcParent)
	if err != nil {
		fs.Errorf(srcRemote, "dirmove: expire error: %v", err)
	}

	// expire parent dir at the destination path
	dstParent := NewDirectory(f, cleanPath(path.Dir(dstRemote)))
	err = f.cache.ExpireDir(dstParent)
	if err != nil {
		fs.Errorf(dstRemote, "dirmove: expire error: %v", err)
	}
	// TODO: precache dst dir and save the chunks

	// clean cache
	go f.CleanUpCache(false)
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
	if f.cacheWrites {
		f.cacheReader(in, src, func(inn io.Reader) {
			obj, err = put(inn, src, options...)
		})
	} else {
		obj, err = put(in, src, options...)
	}
	if err != nil {
		fs.Errorf(src, "error saving in cache: %v", err)
		return nil, err
	}
	cachedObj := ObjectFromOriginal(f, obj).persist()
	// expire parent
	err = f.cache.ExpireDir(cachedObj.parentDir())
	if err != nil {
		fs.Errorf(cachedObj, "put: expire error: %v", err)
	}

	// clean cache
	go f.CleanUpCache(false)
	return cachedObj, nil
}

// Put in to the remote path with the modTime given of the given size
func (f *Fs) Put(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	fs.Infof(f, "put data at '%s'", src.Remote())
	return f.put(in, src, options, f.Fs.Put)
}

// PutUnchecked uploads the object
func (f *Fs) PutUnchecked(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	do := f.Fs.Features().PutUnchecked
	if do == nil {
		return nil, errors.New("can't PutUnchecked")
	}
	fs.Infof(f, "put data unchecked in '%s'", src.Remote())
	return f.put(in, src, options, do)
}

// PutStream uploads the object
func (f *Fs) PutStream(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	do := f.Fs.Features().PutStream
	if do == nil {
		return nil, errors.New("can't PutStream")
	}
	fs.Infof(f, "put data streaming in '%s'", src.Remote())
	return f.put(in, src, options, do)
}

// Copy src to this remote using server side copy operations.
func (f *Fs) Copy(src fs.Object, remote string) (fs.Object, error) {
	do := f.Fs.Features().Copy
	if do == nil {
		fs.Errorf(src, "source remote (%v) doesn't support Copy", src.Fs())
		return nil, fs.ErrorCantCopy
	}

	srcObj, ok := src.(*Object)
	if !ok {
		fs.Errorf(srcObj, "can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}
	if srcObj.CacheFs.Fs.Name() != f.Fs.Name() {
		fs.Errorf(srcObj, "can't copy - not wrapping same remote types")
		return nil, fs.ErrorCantCopy
	}
	fs.Infof(f, "copy obj '%s' -> '%s'", srcObj.abs(), remote)

	// store in cache
	if err := srcObj.refreshFromSource(); err != nil {
		fs.Errorf(f, "can't move %v - %v", src, err)
		return nil, fs.ErrorCantCopy
	}
	obj, err := do(srcObj.Object, remote)
	if err != nil {
		fs.Errorf(srcObj, "error moving in cache: %v", err)
		return nil, err
	}

	// persist new
	co := ObjectFromOriginal(f, obj).persist()
	// expire the destination path
	err = f.cache.ExpireDir(co.parentDir())
	if err != nil {
		fs.Errorf(co, "copy: expire error: %v", err)
	}

	// expire src parent
	srcParent := NewDirectory(f, cleanPath(path.Dir(src.Remote())))
	err = f.cache.ExpireDir(srcParent)
	if err != nil {
		fs.Errorf(src, "copy: expire error: %v", err)
	}

	// clean cache
	go f.CleanUpCache(false)
	return co, nil
}

// Move src to this remote using server side move operations.
func (f *Fs) Move(src fs.Object, remote string) (fs.Object, error) {
	do := f.Fs.Features().Move
	if do == nil {
		fs.Errorf(src, "source remote (%v) doesn't support Move", src.Fs())
		return nil, fs.ErrorCantMove
	}

	srcObj, ok := src.(*Object)
	if !ok {
		fs.Errorf(srcObj, "can't move - not same remote type")
		return nil, fs.ErrorCantMove
	}
	if srcObj.CacheFs.Fs.Name() != f.Fs.Name() {
		fs.Errorf(srcObj, "can't move - not wrapping same remote types")
		return nil, fs.ErrorCantMove
	}
	fs.Infof(f, "moving obj '%s' -> %s", srcObj.abs(), remote)

	// save in cache
	if err := srcObj.refreshFromSource(); err != nil {
		fs.Errorf(f, "can't move %v - %v", src, err)
		return nil, fs.ErrorCantMove
	}
	obj, err := do(srcObj.Object, remote)
	if err != nil {
		fs.Errorf(srcObj, "error moving in cache: %v", err)
		return nil, err
	}

	// remove old
	err = f.cache.RemoveObject(srcObj.abs())
	if err != nil {
		fs.Errorf(srcObj, "move: remove error: %v", err)
	}
	// expire old parent
	err = f.cache.ExpireDir(srcObj.parentDir())
	if err != nil {
		fs.Errorf(srcObj, "move: expire error: %v", err)
	}

	// persist new
	cachedObj := ObjectFromOriginal(f, obj).persist()
	// expire new parent
	err = f.cache.ExpireDir(cachedObj.parentDir())
	if err != nil {
		fs.Errorf(cachedObj, "move: expire error: %v", err)
	}

	// clean cache
	go f.CleanUpCache(false)
	return cachedObj, nil
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() fs.HashSet {
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

// OpenRateLimited will execute a closure under a rate limiter watch
func (f *Fs) OpenRateLimited(fn func() (io.ReadCloser, error)) (io.ReadCloser, error) {
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

// Wrap returns the Fs that is wrapping this Fs
func (f *Fs) isWrappedByCrypt() (*crypt.Fs, bool) {
	if f.wrapper == nil {
		return nil, false
	}
	c, ok := f.wrapper.(*crypt.Fs)
	return c, ok
}

// DirCacheFlush flushes the dir cache
func (f *Fs) DirCacheFlush() {
	_ = f.cache.RemoveDir("")
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
)
