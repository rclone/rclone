// +build !plan9,go1.7

package cache

import (
	"time"

	"bytes"
	"encoding/binary"
	"encoding/json"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"

	"io/ioutil"

	bolt "github.com/coreos/bbolt"
	"github.com/ncw/rclone/fs"
	"github.com/pkg/errors"
)

// Constants
const (
	RootBucket   = "root"
	RootTsBucket = "rootTs"
	DataTsBucket = "dataTs"
)

// Features flags for this storage type
type Features struct {
	PurgeDb bool // purge the db before starting
}

var boltMap = make(map[string]*Persistent)
var boltMapMx sync.Mutex

// GetPersistent returns a single instance for the specific store
func GetPersistent(dbPath, chunkPath string, f *Features) (*Persistent, error) {
	// write lock to create one
	boltMapMx.Lock()
	defer boltMapMx.Unlock()
	if b, ok := boltMap[dbPath]; ok {
		return b, nil
	}

	bb, err := newPersistent(dbPath, chunkPath, f)
	if err != nil {
		return nil, err
	}
	boltMap[dbPath] = bb
	return boltMap[dbPath], nil
}

type chunkInfo struct {
	Path   string
	Offset int64
	Size   int64
}

// Persistent is a wrapper of persistent storage for a bolt.DB file
type Persistent struct {
	Storage

	dbPath     string
	dataPath   string
	db         *bolt.DB
	cleanupMux sync.Mutex
	features   *Features
}

// newPersistent builds a new wrapper and connects to the bolt.DB file
func newPersistent(dbPath, chunkPath string, f *Features) (*Persistent, error) {
	b := &Persistent{
		dbPath:   dbPath,
		dataPath: chunkPath,
		features: f,
	}

	err := b.Connect()
	if err != nil {
		fs.Errorf(dbPath, "Error opening storage cache. Is there another rclone running on the same remote? %v", err)
		return nil, err
	}

	return b, nil
}

// String will return a human friendly string for this DB (currently the dbPath)
func (b *Persistent) String() string {
	return "<Cache DB> " + b.dbPath
}

// Connect creates a connection to the configured file
// refreshDb will delete the file before to create an empty DB if it's set to true
func (b *Persistent) Connect() error {
	var db *bolt.DB
	var err error

	if b.features.PurgeDb {
		err := os.Remove(b.dbPath)
		if err != nil {
			fs.Errorf(b, "failed to remove cache file: %v", err)
		}
		err = os.RemoveAll(b.dataPath)
		if err != nil {
			fs.Errorf(b, "failed to remove cache data: %v", err)
		}
	}

	err = os.MkdirAll(b.dataPath, os.ModePerm)
	if err != nil {
		return errors.Wrapf(err, "failed to create a data directory %q", b.dataPath)
	}
	db, err = bolt.Open(b.dbPath, 0644, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return errors.Wrapf(err, "failed to open a cache connection to %q", b.dbPath)
	}

	_ = db.Update(func(tx *bolt.Tx) error {
		_, _ = tx.CreateBucketIfNotExists([]byte(RootBucket))
		_, _ = tx.CreateBucketIfNotExists([]byte(RootTsBucket))
		_, _ = tx.CreateBucketIfNotExists([]byte(DataTsBucket))

		return nil
	})

	b.db = db
	return nil
}

// getBucket prepares and cleans a specific path of the form: /var/tmp and will iterate through each path component
// to get to the nested bucket of the final part (in this example: tmp)
func (b *Persistent) getBucket(dir string, createIfMissing bool, tx *bolt.Tx) *bolt.Bucket {
	cleanPath(dir)

	entries := strings.FieldsFunc(dir, func(c rune) bool {
		return os.PathSeparator == c
	})
	bucket := tx.Bucket([]byte(RootBucket))

	for _, entry := range entries {
		if createIfMissing {
			bucket, _ = bucket.CreateBucketIfNotExists([]byte(entry))
		} else {
			bucket = bucket.Bucket([]byte(entry))
		}

		if bucket == nil {
			return nil
		}
	}

	return bucket
}

// AddDir will update a CachedDirectory metadata and all its entries
func (b *Persistent) AddDir(cachedDir *Directory) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		bucket := b.getBucket(cachedDir.abs(), true, tx)
		if bucket == nil {
			return errors.Errorf("couldn't open bucket (%v)", cachedDir)
		}

		encoded, err := json.Marshal(cachedDir)
		if err != nil {
			return errors.Errorf("couldn't marshal object (%v): %v", cachedDir, err)
		}
		err = bucket.Put([]byte("."), encoded)
		if err != nil {
			return err
		}
		return nil
	})
}

// GetDirEntries will return a CachedDirectory, its list of dir entries and/or an error if it encountered issues
func (b *Persistent) GetDirEntries(cachedDir *Directory) (fs.DirEntries, error) {
	var dirEntries fs.DirEntries

	err := b.db.View(func(tx *bolt.Tx) error {
		bucket := b.getBucket(cachedDir.abs(), false, tx)
		if bucket == nil {
			return errors.Errorf("couldn't open bucket (%v)", cachedDir.abs())
		}

		val := bucket.Get([]byte("."))
		if val != nil {
			err := json.Unmarshal(val, cachedDir)
			if err != nil {
				return errors.Errorf("error during unmarshalling obj: %v", err)
			}
		} else {
			return errors.Errorf("missing cached dir: %v", cachedDir)
		}

		c := bucket.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			// ignore metadata key: .
			if bytes.Equal(k, []byte(".")) {
				continue
			}
			entryPath := path.Join(cachedDir.Remote(), string(k))

			if v == nil { // directory
				// we try to find a cached meta for the dir
				currentBucket := c.Bucket().Bucket(k)
				if currentBucket == nil {
					return errors.Errorf("couldn't open bucket (%v)", string(k))
				}

				metaKey := currentBucket.Get([]byte("."))
				d := NewDirectory(cachedDir.CacheFs, entryPath)
				if metaKey != nil { //if we don't find it, we create an empty dir
					err := json.Unmarshal(metaKey, d)
					if err != nil { // if even this fails, we fallback to an empty dir
						fs.Debugf(string(k), "error during unmarshalling obj: %v", err)
					}
				}

				dirEntries = append(dirEntries, d)
			} else { // object
				o := NewObject(cachedDir.CacheFs, entryPath)
				err := json.Unmarshal(v, o)
				if err != nil {
					fs.Debugf(string(k), "error during unmarshalling obj: %v", err)
					continue
				}

				dirEntries = append(dirEntries, o)
			}
		}

		return nil
	})

	return dirEntries, err
}

// RemoveDir will delete a CachedDirectory, all its objects and all the chunks stored for it
func (b *Persistent) RemoveDir(fp string) error {
	var err error
	parentDir, dirName := path.Split(fp)
	if fp == "" {
		err = b.db.Update(func(tx *bolt.Tx) error {
			err := tx.DeleteBucket([]byte(RootBucket))
			if err != nil {
				fs.Debugf(fp, "couldn't delete from cache: %v", err)
				return err
			}
			_, _ = tx.CreateBucketIfNotExists([]byte(RootBucket))
			return nil
		})
	} else {
		err = b.db.Update(func(tx *bolt.Tx) error {
			bucket := b.getBucket(cleanPath(parentDir), false, tx)
			if bucket == nil {
				return errors.Errorf("couldn't open bucket (%v)", fp)
			}
			// delete the cached dir
			err := bucket.DeleteBucket([]byte(cleanPath(dirName)))
			if err != nil {
				fs.Debugf(fp, "couldn't delete from cache: %v", err)
			}
			return nil
		})
	}

	// delete chunks on disk
	// safe to ignore as the files might not have been open
	if err == nil {
		_ = os.RemoveAll(path.Join(b.dataPath, fp))
		_ = os.MkdirAll(b.dataPath, os.ModePerm)
	}

	return err
}

// ExpireDir will flush a CachedDirectory and all its objects from the objects
// chunks will remain as they are
func (b *Persistent) ExpireDir(cd *Directory) error {
	t := time.Now().Add(cd.CacheFs.fileAge * -1)
	cd.CacheTs = &t

	// expire all parents
	return b.db.Update(func(tx *bolt.Tx) error {
		// expire all the parents
		currentDir := cd.abs()
		for { // until we get to the root
			bucket := b.getBucket(currentDir, false, tx)
			if bucket != nil {
				val := bucket.Get([]byte("."))
				if val != nil {
					cd2 := &Directory{CacheFs: cd.CacheFs}
					err := json.Unmarshal(val, cd2)
					if err == nil {
						fs.Debugf(cd, "cache: expired %v", currentDir)
						cd2.CacheTs = &t
						enc2, _ := json.Marshal(cd2)
						_ = bucket.Put([]byte("."), enc2)
					}
				}
			}
			if currentDir == "" {
				break
			}
			currentDir = cleanPath(path.Dir(currentDir))
		}
		return nil
	})
}

// GetObject will return a CachedObject from its parent directory or an error if it doesn't find it
func (b *Persistent) GetObject(cachedObject *Object) (err error) {
	return b.db.View(func(tx *bolt.Tx) error {
		bucket := b.getBucket(cachedObject.Dir, false, tx)
		if bucket == nil {
			return errors.Errorf("couldn't open parent bucket for %v", cachedObject.Dir)
		}
		val := bucket.Get([]byte(cachedObject.Name))
		if val != nil {
			return json.Unmarshal(val, cachedObject)
		}
		return errors.Errorf("couldn't find object (%v)", cachedObject.Name)
	})
}

// AddObject will create a cached object in its parent directory
func (b *Persistent) AddObject(cachedObject *Object) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		bucket := b.getBucket(cachedObject.Dir, true, tx)
		if bucket == nil {
			return errors.Errorf("couldn't open parent bucket for %v", cachedObject)
		}
		// cache Object Info
		encoded, err := json.Marshal(cachedObject)
		if err != nil {
			return errors.Errorf("couldn't marshal object (%v) info: %v", cachedObject, err)
		}
		err = bucket.Put([]byte(cachedObject.Name), []byte(encoded))
		if err != nil {
			return errors.Errorf("couldn't cache object (%v) info: %v", cachedObject, err)
		}
		return nil
	})
}

// RemoveObject will delete a single cached object and all the chunks which belong to it
func (b *Persistent) RemoveObject(fp string) error {
	parentDir, objName := path.Split(fp)
	return b.db.Update(func(tx *bolt.Tx) error {
		bucket := b.getBucket(cleanPath(parentDir), false, tx)
		if bucket == nil {
			return errors.Errorf("couldn't open parent bucket for %v", cleanPath(parentDir))
		}
		err := bucket.Delete([]byte(cleanPath(objName)))
		if err != nil {
			fs.Debugf(fp, "couldn't delete obj from storage: %v", err)
		}
		// delete chunks on disk
		// safe to ignore as the file might not have been open
		_ = os.RemoveAll(path.Join(b.dataPath, fp))
		return nil
	})
}

// HasEntry confirms the existence of a single entry (dir or object)
func (b *Persistent) HasEntry(remote string) bool {
	dir, name := path.Split(remote)
	dir = cleanPath(dir)
	name = cleanPath(name)

	err := b.db.View(func(tx *bolt.Tx) error {
		bucket := b.getBucket(dir, false, tx)
		if bucket == nil {
			return errors.Errorf("couldn't open parent bucket for %v", remote)
		}
		if f := bucket.Bucket([]byte(name)); f != nil {
			return nil
		}
		if f := bucket.Get([]byte(name)); f != nil {
			return nil
		}

		return errors.Errorf("couldn't find object (%v)", remote)
	})
	if err == nil {
		return true
	}
	return false
}

// HasChunk confirms the existence of a single chunk of an object
func (b *Persistent) HasChunk(cachedObject *Object, offset int64) bool {
	fp := path.Join(b.dataPath, cachedObject.abs(), strconv.FormatInt(offset, 10))
	if _, err := os.Stat(fp); !os.IsNotExist(err) {
		return true
	}
	return false
}

// GetChunk will retrieve a single chunk which belongs to a cached object or an error if it doesn't find it
func (b *Persistent) GetChunk(cachedObject *Object, offset int64) ([]byte, error) {
	var data []byte

	fp := path.Join(b.dataPath, cachedObject.abs(), strconv.FormatInt(offset, 10))
	data, err := ioutil.ReadFile(fp)
	if err != nil {
		return nil, err
	}

	return data, err
}

// AddChunk adds a new chunk of a cached object
func (b *Persistent) AddChunk(fp string, data []byte, offset int64) error {
	_ = os.MkdirAll(path.Join(b.dataPath, fp), os.ModePerm)

	filePath := path.Join(b.dataPath, fp, strconv.FormatInt(offset, 10))
	err := ioutil.WriteFile(filePath, data, os.ModePerm)
	if err != nil {
		return err
	}

	return b.db.Update(func(tx *bolt.Tx) error {
		tsBucket := tx.Bucket([]byte(DataTsBucket))
		ts := time.Now()
		found := false

		// delete (older) timestamps for the same object
		c := tsBucket.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var ci chunkInfo
			err = json.Unmarshal(v, &ci)
			if err != nil {
				continue
			}
			if ci.Path == fp && ci.Offset == offset {
				if tsInCache := time.Unix(0, btoi(k)); tsInCache.After(ts) && !found {
					found = true
					continue
				}
				err := c.Delete()
				if err != nil {
					fs.Debugf(fp, "failed to clean chunk: %v", err)
				}
			}
		}
		// don't overwrite if a newer one is already there
		if found {
			return nil
		}
		enc, err := json.Marshal(chunkInfo{Path: fp, Offset: offset, Size: int64(len(data))})
		if err != nil {
			fs.Debugf(fp, "failed to timestamp chunk: %v", err)
		}
		err = tsBucket.Put(itob(ts.UnixNano()), enc)
		if err != nil {
			fs.Debugf(fp, "failed to timestamp chunk: %v", err)
		}
		return nil
	})
}

// CleanChunksByAge will cleanup on a cron basis
func (b *Persistent) CleanChunksByAge(chunkAge time.Duration) {
	// NOOP
}

// CleanChunksByNeed is a noop for this implementation
func (b *Persistent) CleanChunksByNeed(offset int64) {
	// noop: we want to clean a Bolt DB by time only
}

// CleanChunksBySize will cleanup chunks after the total size passes a certain point
func (b *Persistent) CleanChunksBySize(maxSize int64) {
	b.cleanupMux.Lock()
	defer b.cleanupMux.Unlock()
	var cntChunks int

	err := b.db.Update(func(tx *bolt.Tx) error {
		dataTsBucket := tx.Bucket([]byte(DataTsBucket))
		if dataTsBucket == nil {
			return errors.Errorf("Couldn't open (%v) bucket", DataTsBucket)
		}
		// iterate through ts
		c := dataTsBucket.Cursor()
		totalSize := int64(0)
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var ci chunkInfo
			err := json.Unmarshal(v, &ci)
			if err != nil {
				continue
			}

			totalSize += ci.Size
		}

		if totalSize > maxSize {
			needToClean := totalSize - maxSize
			for k, v := c.First(); k != nil; k, v = c.Next() {
				var ci chunkInfo
				err := json.Unmarshal(v, &ci)
				if err != nil {
					continue
				}
				// delete this ts entry
				err = c.Delete()
				if err != nil {
					fs.Errorf(ci.Path, "failed deleting chunk ts during cleanup (%v): %v", ci.Offset, err)
					continue
				}
				err = os.Remove(path.Join(b.dataPath, ci.Path, strconv.FormatInt(ci.Offset, 10)))
				if err == nil {
					cntChunks++
					needToClean -= ci.Size
					if needToClean <= 0 {
						break
					}
				}
			}
		}
		fs.Infof("cache", "deleted (%v) chunks", cntChunks)
		return nil
	})

	if err != nil {
		if err == bolt.ErrDatabaseNotOpen {
			// we're likely a late janitor and we need to end quietly as there's no guarantee of what exists anymore
			return
		}
		fs.Errorf("cache", "cleanup failed: %v", err)
	}
}

// Stats returns a go map with the stats key values
func (b *Persistent) Stats() (map[string]map[string]interface{}, error) {
	r := make(map[string]map[string]interface{})
	r["data"] = make(map[string]interface{})
	r["data"]["oldest-ts"] = time.Now()
	r["data"]["oldest-file"] = ""
	r["data"]["newest-ts"] = time.Now()
	r["data"]["newest-file"] = ""
	r["data"]["total-chunks"] = 0
	r["data"]["total-size"] = int64(0)
	r["files"] = make(map[string]interface{})
	r["files"]["oldest-ts"] = time.Now()
	r["files"]["oldest-name"] = ""
	r["files"]["newest-ts"] = time.Now()
	r["files"]["newest-name"] = ""
	r["files"]["total-files"] = 0

	_ = b.db.View(func(tx *bolt.Tx) error {
		dataTsBucket := tx.Bucket([]byte(DataTsBucket))
		rootTsBucket := tx.Bucket([]byte(RootTsBucket))

		var totalDirs int
		var totalFiles int
		_ = b.iterateBuckets(tx.Bucket([]byte(RootBucket)), func(name string) {
			totalDirs++
		}, func(key string, val []byte) {
			totalFiles++
		})
		r["files"]["total-dir"] = totalDirs
		r["files"]["total-files"] = totalFiles

		c := dataTsBucket.Cursor()

		totalChunks := 0
		totalSize := int64(0)
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var ci chunkInfo
			err := json.Unmarshal(v, &ci)
			if err != nil {
				continue
			}
			totalChunks++
			totalSize += ci.Size
		}
		r["data"]["total-chunks"] = totalChunks
		r["data"]["total-size"] = totalSize

		if k, v := c.First(); k != nil {
			var ci chunkInfo
			_ = json.Unmarshal(v, &ci)
			r["data"]["oldest-ts"] = time.Unix(0, btoi(k))
			r["data"]["oldest-file"] = ci.Path
		}
		if k, v := c.Last(); k != nil {
			var ci chunkInfo
			_ = json.Unmarshal(v, &ci)
			r["data"]["newest-ts"] = time.Unix(0, btoi(k))
			r["data"]["newest-file"] = ci.Path
		}

		c = rootTsBucket.Cursor()
		if k, v := c.First(); k != nil {
			// split to get (abs path - offset)
			r["files"]["oldest-ts"] = time.Unix(0, btoi(k))
			r["files"]["oldest-name"] = string(v)
		}
		if k, v := c.Last(); k != nil {
			r["files"]["newest-ts"] = time.Unix(0, btoi(k))
			r["files"]["newest-name"] = string(v)
		}

		return nil
	})

	return r, nil
}

// Purge will flush the entire cache
func (b *Persistent) Purge() {
	b.cleanupMux.Lock()
	defer b.cleanupMux.Unlock()

	_ = b.db.Update(func(tx *bolt.Tx) error {
		_ = tx.DeleteBucket([]byte(RootBucket))
		_ = tx.DeleteBucket([]byte(RootTsBucket))
		_ = tx.DeleteBucket([]byte(DataTsBucket))

		_, _ = tx.CreateBucketIfNotExists([]byte(RootBucket))
		_, _ = tx.CreateBucketIfNotExists([]byte(RootTsBucket))
		_, _ = tx.CreateBucketIfNotExists([]byte(DataTsBucket))

		return nil
	})

	err := os.RemoveAll(b.dataPath)
	if err != nil {
		fs.Errorf(b, "issue removing data folder: %v", err)
	}
	err = os.MkdirAll(b.dataPath, os.ModePerm)
	if err != nil {
		fs.Errorf(b, "issue removing data folder: %v", err)
	}
}

// GetChunkTs retrieves the current timestamp of this chunk
func (b *Persistent) GetChunkTs(path string, offset int64) (time.Time, error) {
	var t time.Time

	err := b.db.View(func(tx *bolt.Tx) error {
		tsBucket := tx.Bucket([]byte(DataTsBucket))
		c := tsBucket.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var ci chunkInfo
			err := json.Unmarshal(v, &ci)
			if err != nil {
				continue
			}
			if ci.Path == path && ci.Offset == offset {
				t = time.Unix(0, btoi(k))
				return nil
			}
		}
		return errors.Errorf("not found %v-%v", path, offset)
	})

	return t, err
}

func (b *Persistent) iterateBuckets(buk *bolt.Bucket, bucketFn func(name string), kvFn func(key string, val []byte)) error {
	err := b.db.View(func(tx *bolt.Tx) error {
		var c *bolt.Cursor
		if buk == nil {
			c = tx.Cursor()
		} else {
			c = buk.Cursor()
		}
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if v == nil {
				var buk2 *bolt.Bucket
				if buk == nil {
					buk2 = tx.Bucket(k)
				} else {
					buk2 = buk.Bucket(k)
				}

				bucketFn(string(k))
				_ = b.iterateBuckets(buk2, bucketFn, kvFn)
			} else {
				kvFn(string(k), v)
			}
		}
		return nil
	})

	return err
}

// Close should be called when the program ends gracefully
func (b *Persistent) Close() {
	b.cleanupMux.Lock()
	defer b.cleanupMux.Unlock()

	err := b.db.Close()
	if err != nil {
		fs.Errorf(b, "closing handle: %v", err)
	}
}

// itob returns an 8-byte big endian representation of v.
func itob(v int64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(v))
	return b
}

func btoi(d []byte) int64 {
	return int64(binary.BigEndian.Uint64(d))
}

// cloneBytes returns a copy of a given slice.
func cloneBytes(v []byte) []byte {
	var clone = make([]byte, len(v))
	copy(clone, v)
	return clone
}
