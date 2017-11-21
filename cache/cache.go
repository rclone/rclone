// +build !plan9

package cache

import (
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"os"

	"github.com/ncw/rclone/fs"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"golang.org/x/time/rate"
)

const (
	// DefCacheChunkSize is the default value for chunk size
	DefCacheChunkSize = "5M"
	// DefCacheInfoAge is the default value for object info age
	DefCacheInfoAge = "6h"
	// DefCacheChunkAge is the default value for chunk age duration
	DefCacheChunkAge = "3h"
	// DefCacheMetaAge is the default value for chunk age duration
	DefCacheMetaAge = "3h"
	// DefCacheReadRetries is the default value for read retries
	DefCacheReadRetries = 3
	// DefCacheTotalWorkers is how many workers run in parallel to download chunks
	DefCacheTotalWorkers = 4
	// DefCacheChunkNoMemory will enable or disable in-memory storage for chunks
	DefCacheChunkNoMemory = false
	// DefCacheRps limits the number of requests per second to the source FS
	DefCacheRps = -1
	// DefWarmUpRatePerSeconds will apply a special config for warming up the cache
	DefWarmUpRatePerSeconds = "3/20"
	// DefCacheWrites will cache file data on writes through the cache
	DefCacheWrites = false
)

// Globals
var (
	// Flags
	cacheDbPath        = fs.StringP("cache-db-path", "", filepath.Join(fs.CacheDir, "cache-backend"), "Directory to cache DB")
	cacheDbPurge       = fs.BoolP("cache-db-purge", "", false, "Purge the cache DB before")
	cacheChunkSize     = fs.StringP("cache-chunk-size", "", DefCacheChunkSize, "The size of a chunk")
	cacheInfoAge       = fs.StringP("cache-info-age", "", DefCacheInfoAge, "How much time should object info be stored in cache")
	cacheChunkAge      = fs.StringP("cache-chunk-age", "", DefCacheChunkAge, "How much time should a chunk be in cache before cleanup")
	cacheMetaAge       = fs.StringP("cache-warm-up-age", "", DefCacheMetaAge, "How much time should data be cached during warm up")
	cacheReadRetries   = fs.IntP("cache-read-retries", "", DefCacheReadRetries, "How many times to retry a read from a cache storage")
	cacheTotalWorkers  = fs.IntP("cache-workers", "", DefCacheTotalWorkers, "How many workers should run in parallel to download chunks")
	cacheChunkNoMemory = fs.BoolP("cache-chunk-no-memory", "", DefCacheChunkNoMemory, "Disable the in-memory cache for storing chunks during streaming")
	cacheRps           = fs.IntP("cache-rps", "", int(DefCacheRps), "Limits the number of requests per second to the source FS. -1 disables the rate limiter")
	cacheWarmUp        = fs.StringP("cache-warm-up-rps", "", DefWarmUpRatePerSeconds, "Format is X/Y = how many X opens per Y seconds should trigger the warm up mode. See the docs")
	cacheStoreWrites   = fs.BoolP("cache-writes", "", DefCacheWrites, "Will cache file data on writes through the FS")
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
			Name: "chunk_age",
			Help: "How much time should a chunk (file data) be stored in cache. \nAccepted units are: \"s\", \"m\", \"h\".\nDefault: " + DefCacheChunkAge,
			Examples: []fs.OptionExample{
				{
					Value: "30s",
					Help:  "30 seconds",
				}, {
					Value: "1m",
					Help:  "1 minute",
				}, {
					Value: "1h30m",
					Help:  "1 hour and 30 minutes",
				},
			},
			Optional: true,
		}, {
			Name: "warmup_age",
			Help: "How much time should data be cached during warm up. \nAccepted units are: \"s\", \"m\", \"h\".\nDefault: " + DefCacheMetaAge,
			Examples: []fs.OptionExample{
				{
					Value: "3h",
					Help:  "3 hours",
				}, {
					Value: "6h",
					Help:  "6 hours",
				}, {
					Value: "24h",
					Help:  "24 hours",
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
	AddChunk(cachedObject *Object, data []byte, offset int64) error

	// AddChunkAhead adds a new chunk before caching an Object for it
	AddChunkAhead(fp string, data []byte, offset int64, t time.Duration) error

	// if the storage can cleanup on a cron basis
	// otherwise it can do a noop operation
	CleanChunksByAge(chunkAge time.Duration)

	// if the storage can cleanup chunks after we no longer need them
	// otherwise it can do a noop operation
	CleanChunksByNeed(offset int64)
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
	ExpireDir(fp string) error

	// will return an object (file) or error if it doesn't find it
	GetObject(cachedObject *Object) (err error)

	// add a new object to its parent directory
	// the directory structure (all the parents of this object) is created if its not found
	AddObject(cachedObject *Object) error

	// remove an object and all its chunks
	RemoveObject(fp string) error

	// Stats returns stats about the cache storage
	Stats() (map[string]map[string]interface{}, error)

	// if the storage can cleanup on a cron basis
	// otherwise it can do a noop operation
	CleanEntriesByAge(entryAge time.Duration)

	// Purge will flush the entire cache
	Purge()
}

// Fs represents a wrapped fs.Fs
type Fs struct {
	fs.Fs

	name     string
	root     string
	features *fs.Features // optional features
	cache    Storage

	fileAge              time.Duration
	chunkSize            int64
	chunkAge             time.Duration
	metaAge              time.Duration
	readRetries          int
	totalWorkers         int
	chunkMemory          bool
	warmUp               bool
	warmUpRate           int
	warmUpSec            int
	cacheWrites          bool
	originalTotalWorkers int
	originalChunkMemory  bool

	lastChunkCleanup  time.Time
	lastRootCleanup   time.Time
	lastOpenedEntries map[string]time.Time
	cleanupMu         sync.Mutex
	warmupMu          sync.Mutex
	rateLimiter       *rate.Limiter
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

	var chunkSize fs.SizeSuffix
	chunkSizeString := fs.ConfigFileGet(name, "chunk_size", DefCacheChunkSize)
	if *cacheChunkSize != DefCacheChunkSize {
		chunkSizeString = *cacheChunkSize
	}
	err := chunkSize.Set(chunkSizeString)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to understand chunk size", chunkSizeString)
	}
	infoAge := fs.ConfigFileGet(name, "info_age", DefCacheInfoAge)
	if *cacheInfoAge != DefCacheInfoAge {
		infoAge = *cacheInfoAge
	}
	infoDuration, err := time.ParseDuration(infoAge)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to understand duration", infoAge)
	}
	chunkAge := fs.ConfigFileGet(name, "chunk_age", DefCacheChunkAge)
	if *cacheChunkAge != DefCacheChunkAge {
		chunkAge = *cacheChunkAge
	}
	chunkDuration, err := time.ParseDuration(chunkAge)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to understand duration", chunkAge)
	}
	metaAge := fs.ConfigFileGet(name, "warmup_age", DefCacheMetaAge)
	if *cacheMetaAge != DefCacheMetaAge {
		metaAge = *cacheMetaAge
	}
	metaDuration, err := time.ParseDuration(metaAge)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to understand duration", metaAge)
	}
	warmupRps := strings.Split(*cacheWarmUp, "/")
	warmupRate, err := strconv.Atoi(warmupRps[0])
	if err != nil {
		return nil, errors.Wrapf(err, "failed to understand warm up rate", *cacheWarmUp)
	}
	warmupSec, err := strconv.Atoi(warmupRps[1])
	if err != nil {
		return nil, errors.Wrapf(err, "failed to understand warm up seconds", *cacheWarmUp)
	}
	// configure cache backend
	if *cacheDbPurge {
		fs.Debugf(name, "Purging the DB")
	}
	f := &Fs{
		Fs:                   wrappedFs,
		name:                 name,
		root:                 rpath,
		fileAge:              infoDuration,
		chunkSize:            int64(chunkSize),
		chunkAge:             chunkDuration,
		metaAge:              metaDuration,
		readRetries:          *cacheReadRetries,
		totalWorkers:         *cacheTotalWorkers,
		originalTotalWorkers: *cacheTotalWorkers,
		chunkMemory:          !*cacheChunkNoMemory,
		originalChunkMemory:  !*cacheChunkNoMemory,
		warmUp:               false,
		warmUpRate:           warmupRate,
		warmUpSec:            warmupSec,
		cacheWrites:          *cacheStoreWrites,
		lastChunkCleanup:     time.Now().Truncate(time.Hour * 24 * 30),
		lastRootCleanup:      time.Now().Truncate(time.Hour * 24 * 30),
		lastOpenedEntries:    make(map[string]time.Time),
	}
	f.rateLimiter = rate.NewLimiter(rate.Limit(float64(*cacheRps)), f.totalWorkers)

	dbPath := *cacheDbPath
	if filepath.Ext(dbPath) != "" {
		dbPath = filepath.Dir(dbPath)
	}
	err = os.MkdirAll(dbPath, os.ModePerm)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create cache directory %v", dbPath)
	}

	dbPath = filepath.Join(dbPath, name+".db")
	fs.Infof(name, "Storage DB path: %v", dbPath)
	f.cache = GetPersistent(dbPath, *cacheDbPurge)
	if err != nil {
		return nil, err
	}

	fs.Infof(name, "Chunk Memory: %v", f.chunkMemory)
	fs.Infof(name, "Chunk Size: %v", fs.SizeSuffix(f.chunkSize))
	fs.Infof(name, "Workers: %v", f.totalWorkers)
	fs.Infof(name, "File Age: %v", f.fileAge.String())
	fs.Infof(name, "Chunk Age: %v", f.chunkAge.String())
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
		DirCacheFlush:           f.DirCacheFlush,
		PutUnchecked:            f.PutUnchecked,
		CleanUp:                 f.CleanUp,
		UnWrap:                  f.UnWrap,
	}).Fill(f).Mask(wrappedFs)

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

// originalSettingWorkers will return the original value of this config
func (f *Fs) originalSettingWorkers() int {
	return f.originalTotalWorkers
}

// originalSettingChunkNoMemory will return the original value of this config
func (f *Fs) originalSettingChunkNoMemory() bool {
	return f.originalChunkMemory
}

// InWarmUp says if cache warm up is active
func (f *Fs) InWarmUp() bool {
	return f.warmUp
}

// enableWarmUp will enable the warm up state of this cache along with the relevant settings
func (f *Fs) enableWarmUp() {
	f.totalWorkers = 1
	f.chunkMemory = false
	f.warmUp = true
}

// disableWarmUp will disable the warm up state of this cache along with the relevant settings
func (f *Fs) disableWarmUp() {
	f.totalWorkers = f.originalSettingWorkers()
	f.chunkMemory = !f.originalSettingChunkNoMemory()
	f.warmUp = false
}

// NewObject finds the Object at remote.
func (f *Fs) NewObject(remote string) (fs.Object, error) {
	co := NewObject(f, remote)
	err := f.cache.GetObject(co)
	if err == nil {
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

	cd := NewDirectory(f, dir)
	entries, err = f.cache.GetDirEntries(cd)
	if err != nil {
		fs.Debugf(dir, "no dir entries in cache: %v", err)
	} else if len(entries) == 0 {
		// TODO: read empty dirs from source?
	} else {
		return entries, nil
	}

	entries, err = f.Fs.List(dir)
	if err != nil {
		return nil, err
	}

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

	// make an empty dir
	_ = f.cache.AddDir(NewDirectory(f, dir))

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

	_ = f.cache.RemoveDir(NewDirectory(f, dir).abs())

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

	srcDir := NewDirectory(srcFs, srcRemote)
	// clear any likely dir cached
	_ = f.cache.ExpireDir(srcDir.parentRemote())
	_ = f.cache.ExpireDir(NewDirectory(srcFs, dstRemote).parentRemote())
	// delete src dir
	_ = f.cache.RemoveDir(srcDir.abs())

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
				err2 := f.cache.AddChunkAhead(cleanPath(path.Join(f.root, src.Remote())), chunk, offset, f.metaAge)
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

// Put in to the remote path with the modTime given of the given size
func (f *Fs) Put(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	fs.Debugf(f, "put data at '%s'", src.Remote())

	var err error
	var obj fs.Object
	if f.cacheWrites {
		f.cacheReader(in, src, func(inn io.Reader) {
			obj, err = f.Fs.Put(inn, src, options...)
		})
	} else {
		obj, err = f.Fs.Put(in, src, options...)
	}

	if err != nil {
		fs.Errorf(src, "error saving in cache: %v", err)
		return nil, err
	}
	cachedObj := ObjectFromOriginal(f, obj).persist()

	// clean cache
	go f.CleanUpCache(false)
	return cachedObj, nil
}

// PutUnchecked uploads the object
func (f *Fs) PutUnchecked(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	do := f.Fs.Features().PutUnchecked
	if do == nil {
		return nil, errors.New("can't PutUnchecked")
	}
	fs.Infof(f, "put data unchecked in '%s'", src.Remote())

	var err error
	var obj fs.Object
	if f.cacheWrites {
		f.cacheReader(in, src, func(inn io.Reader) {
			obj, err = f.Fs.Put(inn, src, options...)
		})
	} else {
		obj, err = f.Fs.Put(in, src, options...)
	}

	if err != nil {
		fs.Errorf(src, "error saving in cache: %v", err)
		return nil, err
	}
	cachedObj := ObjectFromOriginal(f, obj).persist()

	// clean cache
	go f.CleanUpCache(false)
	return cachedObj, nil
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
	cachedObj := ObjectFromOriginal(f, obj).persist()
	_ = f.cache.ExpireDir(cachedObj.parentRemote())

	// clean cache
	go f.CleanUpCache(false)
	return cachedObj, nil
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
	_ = f.cache.ExpireDir(srcObj.parentRemote())
	_ = f.cache.RemoveObject(srcObj.abs())

	// persist new
	cachedObj := ObjectFromOriginal(f, obj)
	cachedObj.persist()
	_ = f.cache.ExpireDir(cachedObj.parentRemote())

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

	f.warmupMu.Lock()
	defer f.warmupMu.Unlock()
	f.lastOpenedEntries = make(map[string]time.Time)

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

// CheckIfWarmupNeeded changes the FS settings during warmups
func (f *Fs) CheckIfWarmupNeeded(remote string) {
	f.warmupMu.Lock()
	defer f.warmupMu.Unlock()

	secondCount := time.Duration(f.warmUpSec)
	rate := f.warmUpRate

	// clean up entries older than the needed time frame needed
	for k, v := range f.lastOpenedEntries {
		if time.Now().After(v.Add(time.Second * secondCount)) {
			delete(f.lastOpenedEntries, k)
		}
	}
	f.lastOpenedEntries[remote] = time.Now()

	// simple check for the current load
	if len(f.lastOpenedEntries) >= rate && !f.warmUp {
		fs.Infof(f, "turning on cache warmup")
		f.enableWarmUp()
	} else if len(f.lastOpenedEntries) < rate && f.warmUp {
		fs.Infof(f, "turning off cache warmup")
		f.disableWarmUp()
	}
}

// CleanUpCache will cleanup only the cache data that is expired
func (f *Fs) CleanUpCache(ignoreLastTs bool) {
	f.cleanupMu.Lock()
	defer f.cleanupMu.Unlock()

	if ignoreLastTs || time.Now().After(f.lastChunkCleanup.Add(f.chunkAge/4)) {
		fs.Infof("cache", "running chunks cleanup")
		f.cache.CleanChunksByAge(f.chunkAge)
		f.lastChunkCleanup = time.Now()
	}

	if ignoreLastTs || time.Now().After(f.lastRootCleanup.Add(f.fileAge/4)) {
		fs.Infof("cache", "running root cleanup")
		f.cache.CleanEntriesByAge(f.fileAge)
		f.lastRootCleanup = time.Now()
	}
}

// UnWrap returns the Fs that this Fs is wrapping
func (f *Fs) UnWrap() fs.Fs {
	return f.Fs
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
	_ fs.CleanUpper     = (*Fs)(nil)
	_ fs.UnWrapper      = (*Fs)(nil)
	_ fs.ListRer        = (*Fs)(nil)
)
