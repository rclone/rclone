package memory

import (
	"path"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
)

type bucketsInfo struct {
	mu      sync.RWMutex
	buckets map[string]*bucketInfo
}

type bucketInfo struct {
	mu      sync.RWMutex
	objects map[string]*objectData
}

type objectData struct {
	modTime  time.Time
	size     int64
	mimeType string
	hash string
}

// makeBucket creates a new bucket if it doesn't exist
func (bi *bucketsInfo) makeBucket(name string) {
	bi.mu.Lock()
	defer bi.mu.Unlock()
	if _, exists := bi.buckets[name]; !exists {
		bi.buckets[name] = &bucketInfo{
			objects: make(map[string]*objectData),
		}
	}
}

// deleteBucket removes a bucket and its objects
func (bi *bucketsInfo) deleteBucket(name string) {
	bi.mu.Lock()
	defer bi.mu.Unlock()
	delete(bi.buckets, name)
}

// getBucket retrieves a bucket by name
func (bi *bucketsInfo) getBucket(name string) *bucketInfo {
	bi.mu.RLock()
	defer bi.mu.RUnlock()
	return bi.buckets[name]
}

// getObjectData retrieves object data by name or returns nil
func (b *bucketInfo) getObjectData(name string) *objectData {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.objects[name]
}

// storeObjectData saves object data in the bucket
func (b *bucketInfo) storeObjectData(name string, od *objectData) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.objects[name] = od
}

// removeObjectData removes object data safely
func (bi *bucketsInfo) removeObjectData(bucketName, bucketPath string) (removed bool) {
	bi.mu.Lock()
	defer bi.mu.Unlock()
	b := bi.buckets[bucketName]
	if b == nil {
		return false
	}
	if _, exists := b.objects[bucketPath]; exists {
		delete(b.objects, bucketPath)
		return true
	}
	return false
}

type Object struct {
	remote string
	od     *objectData
}

// NewObject creates a new object with given remote and data
func (f *Fs) newObject(remote string, od *objectData) *Object {
	return &Object{
		remote: remote,
		od:     od,
	}
}

// Accessor methods for Object
func (o *Object) ModTime() time.Time {
	return o.od.modTime
}

func (o *Object) MimeType() string {
	return o.od.mimeType
}

func (o *Object) Hash() string {
	return o.od.hash
}

// list iterates through objects and directories under a given directory
func (f *Fs) list(bucket, directory string, addBucket bool, fn func(string, fs.DirEntry, bool) error) error {
	b := f.buckets.getBucket(bucket)
	if b == nil {
		return fs.ErrorDirNotFound
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	knownDirs := make(map[string]struct{})
	for absPath, od := range b.objects {
		if !strings.HasPrefix(absPath, directory) {
			continue
		}
		localPath := absPath[len(directory):]
		slash := strings.IndexRune(localPath, '/')
		if slash >= 0 {
			dir := strings.TrimPrefix(directory, f.rootDirectory+"/") + localPath[:slash]
			if addBucket {
				dir = path.Join(bucket, dir)
			}
			if _, found := knownDirs[dir]; !found {
				knownDirs[dir] = struct{}{}
				err := fn(dir, fs.NewDir(dir, time.Time{}), true)
				if err != nil {
					return err
				}
			}
			continue
		}
		if addBucket {
			remote = path.Join(bucket, remote)
		}
		err := fn(remote, f.newObject(remote, od), false)
		if err != nil {
			return err
		}
	}
	return nil
}
