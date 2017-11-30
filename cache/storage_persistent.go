// +build !plan9

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
	"path/filepath"

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

var boltMap = make(map[string]*Persistent)
var boltMapMx sync.Mutex

// GetPersistent returns a single instance for the specific store
func GetPersistent(dbPath string, refreshDb bool) (*Persistent, error) {
	// write lock to create one
	boltMapMx.Lock()
	defer boltMapMx.Unlock()
	if b, ok := boltMap[dbPath]; ok {
		return b, nil
	}

	bb, err := newPersistent(dbPath, refreshDb)
	if err != nil {
		return nil, err
	}
	boltMap[dbPath] = bb
	return boltMap[dbPath], nil
}

// Persistent is a wrapper of persistent storage for a bolt.DB file
type Persistent struct {
	Storage

	dbPath     string
	dataPath   string
	db         *bolt.DB
	cleanupMux sync.Mutex
}

// newPersistent builds a new wrapper and connects to the bolt.DB file
func newPersistent(dbPath string, refreshDb bool) (*Persistent, error) {
	dataPath := strings.TrimSuffix(dbPath, filepath.Ext(dbPath))

	b := &Persistent{
		dbPath:   dbPath,
		dataPath: dataPath,
	}

	err := b.Connect(refreshDb)
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
func (b *Persistent) Connect(refreshDb bool) error {
	var db *bolt.DB
	var err error

	if refreshDb {
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

// updateChunkTs is a convenience method to update a chunk timestamp to mark that it was used recently
func (b *Persistent) updateChunkTs(tx *bolt.Tx, path string, offset int64, t time.Duration) {
	tsBucket := tx.Bucket([]byte(DataTsBucket))
	tsVal := path + "-" + strconv.FormatInt(offset, 10)
	ts := time.Now().Add(t)
	found := false

	// delete previous timestamps for the same object
	c := tsBucket.Cursor()
	for k, v := c.First(); k != nil; k, v = c.Next() {
		if bytes.Equal(v, []byte(tsVal)) {
			if tsInCache := time.Unix(0, btoi(k)); tsInCache.After(ts) && !found {
				found = true
				continue
			}
			err := c.Delete()
			if err != nil {
				fs.Debugf(path, "failed to clean chunk: %v", err)
			}
		}
	}
	if found {
		return
	}

	err := tsBucket.Put(itob(ts.UnixNano()), []byte(tsVal))
	if err != nil {
		fs.Debugf(path, "failed to timestamp chunk: %v", err)
	}
}

// updateRootTs is a convenience method to update an object timestamp to mark that it was used recently
func (b *Persistent) updateRootTs(tx *bolt.Tx, path string, t time.Duration) {
	tsBucket := tx.Bucket([]byte(RootTsBucket))
	ts := time.Now().Add(t)
	found := false

	// delete previous timestamps for the same object
	c := tsBucket.Cursor()
	for k, v := c.First(); k != nil; k, v = c.Next() {
		if bytes.Equal(v, []byte(path)) {
			if tsInCache := time.Unix(0, btoi(k)); tsInCache.After(ts) && !found {
				found = true
				continue
			}
			err := c.Delete()
			if err != nil {
				fs.Debugf(path, "failed to clean object: %v", err)
			}
		}
	}
	if found {
		return
	}

	err := tsBucket.Put(itob(ts.UnixNano()), []byte(path))
	if err != nil {
		fs.Debugf(path, "failed to timestamp chunk: %v", err)
	}
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

		b.updateRootTs(tx, cachedDir.abs(), cachedDir.CacheFs.fileAge)
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
				fs.Debugf(cachedDir.abs(), "error during unmarshalling obj: %v", err)
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
	err := b.ExpireDir(fp)

	// delete chunks on disk
	// safe to ignore as the files might not have been open
	if err != nil {
		_ = os.RemoveAll(path.Join(b.dataPath, fp))
	}

	return nil
}

// ExpireDir will flush a CachedDirectory and all its objects from the objects
// chunks will remain as they are
func (b *Persistent) ExpireDir(fp string) error {
	parentDir, dirName := path.Split(fp)
	if fp == "" {
		return b.db.Update(func(tx *bolt.Tx) error {
			err := tx.DeleteBucket([]byte(RootBucket))
			if err != nil {
				fs.Debugf(fp, "couldn't delete from cache: %v", err)
				return err
			}
			err = tx.DeleteBucket([]byte(RootTsBucket))
			if err != nil {
				fs.Debugf(fp, "couldn't delete from cache: %v", err)
				return err
			}
			_, _ = tx.CreateBucketIfNotExists([]byte(RootBucket))
			_, _ = tx.CreateBucketIfNotExists([]byte(RootTsBucket))
			return nil
		})
	}

	return b.db.Update(func(tx *bolt.Tx) error {
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
		b.updateRootTs(tx, cachedObject.abs(), cachedObject.CacheFs.fileAge)
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
	p := cachedObject.abs()
	var data []byte

	fp := path.Join(b.dataPath, cachedObject.abs(), strconv.FormatInt(offset, 10))
	data, err := ioutil.ReadFile(fp)
	if err != nil {
		return nil, err
	}

	d := cachedObject.CacheFs.chunkAge
	if cachedObject.CacheFs.InWarmUp() {
		d = cachedObject.CacheFs.metaAge
	}

	err = b.db.Update(func(tx *bolt.Tx) error {
		b.updateChunkTs(tx, p, offset, d)
		return nil
	})

	return data, err
}

// AddChunk adds a new chunk of a cached object
func (b *Persistent) AddChunk(cachedObject *Object, data []byte, offset int64) error {
	t := cachedObject.CacheFs.chunkAge
	if cachedObject.CacheFs.InWarmUp() {
		t = cachedObject.CacheFs.metaAge
	}
	return b.AddChunkAhead(cachedObject.abs(), data, offset, t)
}

// AddChunkAhead adds a new chunk before caching an Object for it
// see fs.cacheWrites
func (b *Persistent) AddChunkAhead(fp string, data []byte, offset int64, t time.Duration) error {
	_ = os.MkdirAll(path.Join(b.dataPath, fp), os.ModePerm)

	filePath := path.Join(b.dataPath, fp, strconv.FormatInt(offset, 10))
	err := ioutil.WriteFile(filePath, data, os.ModePerm)
	if err != nil {
		return err
	}

	return b.db.Update(func(tx *bolt.Tx) error {
		b.updateChunkTs(tx, fp, offset, t)
		return nil
	})
}

// CleanChunksByAge will cleanup on a cron basis
func (b *Persistent) CleanChunksByAge(chunkAge time.Duration) {
	b.cleanupMux.Lock()
	defer b.cleanupMux.Unlock()
	var cntChunks int

	err := b.db.Update(func(tx *bolt.Tx) error {
		min := itob(0)
		max := itob(time.Now().UnixNano())

		dataTsBucket := tx.Bucket([]byte(DataTsBucket))
		if dataTsBucket == nil {
			return errors.Errorf("Couldn't open (%v) bucket", DataTsBucket)
		}
		// iterate through ts
		c := dataTsBucket.Cursor()
		for k, v := c.Seek(min); k != nil && bytes.Compare(k, max) <= 0; k, v = c.Next() {
			if v == nil {
				continue
			}
			// split to get (abs path - offset)
			val := string(v[:])
			sepIdx := strings.LastIndex(val, "-")
			pathCmp := val[:sepIdx]
			offsetCmp := val[sepIdx+1:]

			// delete this ts entry
			err := c.Delete()
			if err != nil {
				fs.Errorf(pathCmp, "failed deleting chunk ts during cleanup (%v): %v", val, err)
				continue
			}

			err = os.Remove(path.Join(b.dataPath, pathCmp, offsetCmp))
			if err == nil {
				cntChunks = cntChunks + 1
			}
		}
		fs.Infof("cache", "deleted (%v) chunks", cntChunks)
		return nil
	})

	if err != nil {
		fs.Errorf("cache", "cleanup failed: %v", err)
	}
}

// CleanEntriesByAge will cleanup on a cron basis
func (b *Persistent) CleanEntriesByAge(entryAge time.Duration) {
	b.cleanupMux.Lock()
	defer b.cleanupMux.Unlock()
	var cntEntries int

	err := b.db.Update(func(tx *bolt.Tx) error {
		min := itob(0)
		max := itob(time.Now().UnixNano())

		rootTsBucket := tx.Bucket([]byte(RootTsBucket))
		if rootTsBucket == nil {
			return errors.Errorf("Couldn't open (%v) bucket", rootTsBucket)
		}
		// iterate through ts
		c := rootTsBucket.Cursor()
		for k, v := c.Seek(min); k != nil && bytes.Compare(k, max) <= 0; k, v = c.Next() {
			if v == nil {
				continue
			}
			// get the path
			absPath := string(v)
			absDir, absName := path.Split(absPath)

			// delete this ts entry
			err := c.Delete()
			if err != nil {
				fs.Errorf(absPath, "failed deleting object during cleanup: %v", err)
				continue
			}

			// search for the entry in the root bucket, skip it if it's not found
			parentBucket := b.getBucket(cleanPath(absDir), false, tx)
			if parentBucket == nil {
				continue
			}
			_ = parentBucket.Delete([]byte(cleanPath(absName)))
			cntEntries = cntEntries + 1
		}
		fs.Infof("cache", "deleted (%v) entries", cntEntries)
		return nil
	})

	if err != nil {
		fs.Errorf("cache", "cleanup failed: %v", err)
	}
}

// CleanChunksByNeed is a noop for this implementation
func (b *Persistent) CleanChunksByNeed(offset int64) {
	// noop: we want to clean a Bolt DB by time only
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
		if k, v := c.First(); k != nil {
			// split to get (abs path - offset)
			val := string(v[:])
			p := val[:strings.LastIndex(val, "-")]
			r["data"]["oldest-ts"] = time.Unix(0, btoi(k))
			r["data"]["oldest-file"] = p
		}
		if k, v := c.Last(); k != nil {
			// split to get (abs path - offset)
			val := string(v[:])
			p := val[:strings.LastIndex(val, "-")]
			r["data"]["newest-ts"] = time.Unix(0, btoi(k))
			r["data"]["newest-file"] = p
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
	tsVal := path + "-" + strconv.FormatInt(offset, 10)

	err := b.db.View(func(tx *bolt.Tx) error {
		tsBucket := tx.Bucket([]byte(DataTsBucket))
		c := tsBucket.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if bytes.Equal(v, []byte(tsVal)) {
				t = time.Unix(0, btoi(k))
				return nil
			}
		}
		return errors.Errorf("not found %v-%v", path, offset)
	})

	return t, err
}

// GetRootTs retrieves the current timestamp of an object or dir
func (b *Persistent) GetRootTs(path string) (time.Time, error) {
	var t time.Time

	err := b.db.View(func(tx *bolt.Tx) error {
		tsBucket := tx.Bucket([]byte(RootTsBucket))
		c := tsBucket.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if bytes.Equal(v, []byte(path)) {
				t = time.Unix(0, btoi(k))
				return nil
			}
		}
		return errors.Errorf("not found %v", path)
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
