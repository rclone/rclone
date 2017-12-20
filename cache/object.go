// +build !plan9,go1.7

package cache

import (
	"encoding/json"
	"io"
	"os"
	"path"
	"sync"
	"time"

	"strconv"

	"github.com/ncw/rclone/fs"
)

// Object is a generic file like object that stores basic information about it
type Object struct {
	fs.Object `json:"-"`

	CacheFs       *Fs                    `json:"-"`        // cache fs
	Name          string                 `json:"name"`     // name of the directory
	Dir           string                 `json:"dir"`      // abs path of the object
	CacheModTime  int64                  `json:"modTime"`  // modification or creation time - IsZero for unknown
	CacheSize     int64                  `json:"size"`     // size of directory and contents or -1 if unknown
	CacheStorable bool                   `json:"storable"` // says whether this object can be stored
	CacheType     string                 `json:"cacheType"`
	CacheTs       time.Time              `json:"cacheTs"`
	cacheHashes   map[fs.HashType]string // all supported hashes cached

	refreshMutex sync.Mutex
}

// NewObject builds one from a generic fs.Object
func NewObject(f *Fs, remote string) *Object { //0745 379 768
	fullRemote := path.Join(f.Root(), remote)
	dir, name := path.Split(fullRemote)

	co := &Object{
		CacheFs:       f,
		Name:          cleanPath(name),
		Dir:           cleanPath(dir),
		CacheModTime:  time.Now().UnixNano(),
		CacheSize:     0,
		CacheStorable: false,
		CacheType:     "Object",
		CacheTs:       time.Now(),
	}
	return co
}

// MarshalJSON is needed to override the hashes map (needed to support older versions of Go)
func (o *Object) MarshalJSON() ([]byte, error) {
	hashes := make(map[string]string)
	for k, v := range o.cacheHashes {
		hashes[strconv.Itoa(int(k))] = v
	}

	type Alias Object
	return json.Marshal(&struct {
		Hashes map[string]string `json:"hashes"`
		*Alias
	}{
		Alias:  (*Alias)(o),
		Hashes: hashes,
	})
}

// UnmarshalJSON is needed to override the CacheHashes map (needed to support older versions of Go)
func (o *Object) UnmarshalJSON(b []byte) error {
	type Alias Object
	aux := &struct {
		Hashes map[string]string `json:"hashes"`
		*Alias
	}{
		Alias: (*Alias)(o),
	}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}

	o.cacheHashes = make(map[fs.HashType]string)
	for k, v := range aux.Hashes {
		ht, _ := strconv.Atoi(k)
		o.cacheHashes[fs.HashType(ht)] = v
	}

	return nil
}

// ObjectFromOriginal builds one from a generic fs.Object
func ObjectFromOriginal(f *Fs, o fs.Object) *Object {
	var co *Object
	fullRemote := cleanPath(path.Join(f.Root(), o.Remote()))

	dir, name := path.Split(fullRemote)
	co = &Object{
		CacheFs:   f,
		Name:      cleanPath(name),
		Dir:       cleanPath(dir),
		CacheType: "Object",
		CacheTs:   time.Now(),
	}
	co.updateData(o)
	return co
}

func (o *Object) updateData(source fs.Object) {
	o.Object = source
	o.CacheModTime = source.ModTime().UnixNano()
	o.CacheSize = source.Size()
	o.CacheStorable = source.Storable()
	o.CacheTs = time.Now()
	o.cacheHashes = make(map[fs.HashType]string)
}

// Fs returns its FS info
func (o *Object) Fs() fs.Info {
	return o.CacheFs
}

// String returns a human friendly name for this object
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.Remote()
}

// Remote returns the remote path
func (o *Object) Remote() string {
	p := path.Join(o.Dir, o.Name)
	if o.CacheFs.Root() != "" {
		p = p[len(o.CacheFs.Root()):] // trim out root
		if len(p) > 0 {               // remove first separator
			p = p[1:]
		}
	}

	return p
}

// abs returns the absolute path to the object
func (o *Object) abs() string {
	return path.Join(o.Dir, o.Name)
}

// parentRemote returns the absolute path parent remote
func (o *Object) parentRemote() string {
	absPath := o.abs()
	return cleanPath(path.Dir(absPath))
}

// parentDir returns the absolute path parent remote
func (o *Object) parentDir() *Directory {
	return NewDirectory(o.CacheFs, cleanPath(path.Dir(o.Remote())))
}

// ModTime returns the cached ModTime
func (o *Object) ModTime() time.Time {
	return time.Unix(0, o.CacheModTime)
}

// Size returns the cached Size
func (o *Object) Size() int64 {
	return o.CacheSize
}

// Storable returns the cached Storable
func (o *Object) Storable() bool {
	return o.CacheStorable
}

// refreshFromSource requests the original FS for the object in case it comes from a cached entry
func (o *Object) refreshFromSource() error {
	o.refreshMutex.Lock()
	defer o.refreshMutex.Unlock()

	if o.Object != nil {
		return nil
	}

	liveObject, err := o.CacheFs.Fs.NewObject(o.Remote())
	if err != nil {
		fs.Errorf(o, "error refreshing object: %v", err)
		return err
	}
	o.updateData(liveObject)
	o.persist()

	return nil
}

// SetModTime sets the ModTime of this object
func (o *Object) SetModTime(t time.Time) error {
	if err := o.refreshFromSource(); err != nil {
		return err
	}

	err := o.Object.SetModTime(t)
	if err != nil {
		return err
	}

	o.CacheModTime = t.UnixNano()
	o.persist()
	fs.Debugf(o.Fs(), "updated ModTime %v: %v", o, t)

	return nil
}

// Open is used to request a specific part of the file using fs.RangeOption
func (o *Object) Open(options ...fs.OpenOption) (io.ReadCloser, error) {
	if err := o.refreshFromSource(); err != nil {
		return nil, err
	}

	var err error
	cacheReader := NewObjectHandle(o)
	for _, option := range options {
		switch x := option.(type) {
		case *fs.SeekOption:
			_, err = cacheReader.Seek(x.Offset, os.SEEK_SET)
		case *fs.RangeOption:
			_, err = cacheReader.Seek(x.Start, os.SEEK_SET)
		}
		if err != nil {
			return cacheReader, err
		}
	}

	return cacheReader, nil
}

// Update will change the object data
func (o *Object) Update(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	if err := o.refreshFromSource(); err != nil {
		return err
	}
	fs.Infof(o, "updating object contents with size %v", src.Size())

	// deleting cached chunks and info to be replaced with new ones
	_ = o.CacheFs.cache.RemoveObject(o.abs())

	err := o.Object.Update(in, src, options...)
	if err != nil {
		fs.Errorf(o, "error updating source: %v", err)
		return err
	}

	o.CacheModTime = src.ModTime().UnixNano()
	o.CacheSize = src.Size()
	o.cacheHashes = make(map[fs.HashType]string)
	o.persist()

	return nil
}

// Remove deletes the object from both the cache and the source
func (o *Object) Remove() error {
	if err := o.refreshFromSource(); err != nil {
		return err
	}
	err := o.Object.Remove()
	if err != nil {
		return err
	}
	fs.Infof(o, "removing object")

	_ = o.CacheFs.cache.RemoveObject(o.abs())
	return err
}

// Hash requests a hash of the object and stores in the cache
// since it might or might not be called, this is lazy loaded
func (o *Object) Hash(ht fs.HashType) (string, error) {
	if o.cacheHashes == nil {
		o.cacheHashes = make(map[fs.HashType]string)
	}

	cachedHash, found := o.cacheHashes[ht]
	if found {
		return cachedHash, nil
	}

	if err := o.refreshFromSource(); err != nil {
		return "", err
	}

	liveHash, err := o.Object.Hash(ht)
	if err != nil {
		return "", err
	}

	o.cacheHashes[ht] = liveHash

	o.persist()
	fs.Debugf(o, "object hash cached: %v", liveHash)

	return liveHash, nil
}

// persist adds this object to the persistent cache
func (o *Object) persist() *Object {
	err := o.CacheFs.cache.AddObject(o)
	if err != nil {
		fs.Errorf(o, "failed to cache object: %v", err)
	}
	return o
}

var (
	_ fs.Object = (*Object)(nil)
)
