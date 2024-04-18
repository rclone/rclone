//go:build !plan9 && !js

package cache

import (
	"context"
	"fmt"
	"io"
	"path"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/readers"
)

const (
	objectInCache       = "Object"
	objectPendingUpload = "TempObject"
)

// Object is a generic file like object that stores basic information about it
type Object struct {
	fs.Object `json:"-"`

	ParentFs      fs.Fs     `json:"-"`        // parent fs
	CacheFs       *Fs       `json:"-"`        // cache fs
	Name          string    `json:"name"`     // name of the directory
	Dir           string    `json:"dir"`      // abs path of the object
	CacheModTime  int64     `json:"modTime"`  // modification or creation time - IsZero for unknown
	CacheSize     int64     `json:"size"`     // size of directory and contents or -1 if unknown
	CacheStorable bool      `json:"storable"` // says whether this object can be stored
	CacheType     string    `json:"cacheType"`
	CacheTs       time.Time `json:"cacheTs"`
	cacheHashesMu sync.Mutex
	CacheHashes   map[hash.Type]string // all supported hashes cached

	refreshMutex sync.Mutex
}

// NewObject builds one from a generic fs.Object
func NewObject(f *Fs, remote string) *Object {
	fullRemote := path.Join(f.Root(), remote)
	dir, name := path.Split(fullRemote)

	cacheType := objectInCache
	parentFs := f.UnWrap()
	if f.opt.TempWritePath != "" {
		_, err := f.cache.SearchPendingUpload(fullRemote)
		if err == nil { // queued for upload
			cacheType = objectPendingUpload
			parentFs = f.tempFs
			fs.Debugf(fullRemote, "pending upload found")
		}
	}

	co := &Object{
		ParentFs:      parentFs,
		CacheFs:       f,
		Name:          cleanPath(name),
		Dir:           cleanPath(dir),
		CacheModTime:  time.Now().UnixNano(),
		CacheSize:     0,
		CacheStorable: false,
		CacheType:     cacheType,
		CacheTs:       time.Now(),
	}
	return co
}

// ObjectFromOriginal builds one from a generic fs.Object
func ObjectFromOriginal(ctx context.Context, f *Fs, o fs.Object) *Object {
	var co *Object
	fullRemote := cleanPath(path.Join(f.Root(), o.Remote()))
	dir, name := path.Split(fullRemote)

	cacheType := objectInCache
	parentFs := f.UnWrap()
	if f.opt.TempWritePath != "" {
		_, err := f.cache.SearchPendingUpload(fullRemote)
		if err == nil { // queued for upload
			cacheType = objectPendingUpload
			parentFs = f.tempFs
			fs.Debugf(fullRemote, "pending upload found")
		}
	}

	co = &Object{
		ParentFs:  parentFs,
		CacheFs:   f,
		Name:      cleanPath(name),
		Dir:       cleanPath(dir),
		CacheType: cacheType,
		CacheTs:   time.Now(),
	}
	co.updateData(ctx, o)
	return co
}

func (o *Object) updateData(ctx context.Context, source fs.Object) {
	o.Object = source
	o.CacheModTime = source.ModTime(ctx).UnixNano()
	o.CacheSize = source.Size()
	o.CacheStorable = source.Storable()
	o.CacheTs = time.Now()
	o.cacheHashesMu.Lock()
	o.CacheHashes = make(map[hash.Type]string)
	o.cacheHashesMu.Unlock()
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
	return o.CacheFs.cleanRootFromPath(p)
}

// abs returns the absolute path to the object
func (o *Object) abs() string {
	return path.Join(o.Dir, o.Name)
}

// ModTime returns the cached ModTime
func (o *Object) ModTime(ctx context.Context) time.Time {
	_ = o.refresh(ctx)
	return time.Unix(0, o.CacheModTime)
}

// Size returns the cached Size
func (o *Object) Size() int64 {
	_ = o.refresh(context.TODO())
	return o.CacheSize
}

// Storable returns the cached Storable
func (o *Object) Storable() bool {
	_ = o.refresh(context.TODO())
	return o.CacheStorable
}

// refresh will check if the object info is expired and request the info from source if it is
// all these conditions must be true to ignore a refresh
// 1. cache ts didn't expire yet
// 2. is not pending a notification from the wrapped fs
func (o *Object) refresh(ctx context.Context) error {
	isNotified := o.CacheFs.isNotifiedRemote(o.Remote())
	isExpired := time.Now().After(o.CacheTs.Add(time.Duration(o.CacheFs.opt.InfoAge)))
	if !isExpired && !isNotified {
		return nil
	}

	return o.refreshFromSource(ctx, true)
}

// refreshFromSource requests the original FS for the object in case it comes from a cached entry
func (o *Object) refreshFromSource(ctx context.Context, force bool) error {
	o.refreshMutex.Lock()
	defer o.refreshMutex.Unlock()
	var err error
	var liveObject fs.Object

	if o.Object != nil && !force {
		return nil
	}
	if o.isTempFile() {
		liveObject, err = o.ParentFs.NewObject(ctx, o.Remote())
		if err != nil {
			err = fmt.Errorf("in parent fs %v: %w", o.ParentFs, err)
		}
	} else {
		liveObject, err = o.CacheFs.Fs.NewObject(ctx, o.Remote())
		if err != nil {
			err = fmt.Errorf("in cache fs %v: %w", o.CacheFs.Fs, err)
		}
	}
	if err != nil {
		fs.Errorf(o, "error refreshing object in : %v", err)
		return err
	}
	o.updateData(ctx, liveObject)
	o.persist()

	return nil
}

// SetModTime sets the ModTime of this object
func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
	if err := o.refreshFromSource(ctx, false); err != nil {
		return err
	}

	err := o.Object.SetModTime(ctx, t)
	if err != nil {
		return err
	}

	o.CacheModTime = t.UnixNano()
	o.persist()
	fs.Debugf(o, "updated ModTime: %v", t)

	return nil
}

// Open is used to request a specific part of the file using fs.RangeOption
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	var err error

	if o.Object == nil {
		err = o.refreshFromSource(ctx, true)
	} else {
		err = o.refresh(ctx)
	}
	if err != nil {
		return nil, err
	}

	cacheReader := NewObjectHandle(ctx, o, o.CacheFs)
	var offset, limit int64 = 0, -1
	for _, option := range options {
		switch x := option.(type) {
		case *fs.SeekOption:
			offset = x.Offset
		case *fs.RangeOption:
			offset, limit = x.Decode(o.Size())
		}
		_, err = cacheReader.Seek(offset, io.SeekStart)
		if err != nil {
			return nil, err
		}
	}

	return readers.NewLimitedReadCloser(cacheReader, limit), nil
}

// Update will change the object data
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	if err := o.refreshFromSource(ctx, false); err != nil {
		return err
	}
	// pause background uploads if active
	if o.CacheFs.opt.TempWritePath != "" {
		o.CacheFs.backgroundRunner.pause()
		defer o.CacheFs.backgroundRunner.play()
		// don't allow started uploads
		if o.isTempFile() && o.tempFileStartedUpload() {
			return fmt.Errorf("%v is currently uploading, can't update", o)
		}
	}
	fs.Debugf(o, "updating object contents with size %v", src.Size())

	// FIXME use reliable upload
	err := o.Object.Update(ctx, in, src, options...)
	if err != nil {
		fs.Errorf(o, "error updating source: %v", err)
		return err
	}

	// deleting cached chunks and info to be replaced with new ones
	_ = o.CacheFs.cache.RemoveObject(o.abs())
	// advertise to ChangeNotify if wrapped doesn't do that
	o.CacheFs.notifyChangeUpstreamIfNeeded(o.Remote(), fs.EntryObject)

	o.CacheModTime = src.ModTime(ctx).UnixNano()
	o.CacheSize = src.Size()
	o.cacheHashesMu.Lock()
	o.CacheHashes = make(map[hash.Type]string)
	o.cacheHashesMu.Unlock()
	o.CacheTs = time.Now()
	o.persist()

	return nil
}

// Remove deletes the object from both the cache and the source
func (o *Object) Remove(ctx context.Context) error {
	if err := o.refreshFromSource(ctx, false); err != nil {
		return err
	}
	// pause background uploads if active
	if o.CacheFs.opt.TempWritePath != "" {
		o.CacheFs.backgroundRunner.pause()
		defer o.CacheFs.backgroundRunner.play()
		// don't allow started uploads
		if o.isTempFile() && o.tempFileStartedUpload() {
			return fmt.Errorf("%v is currently uploading, can't delete", o)
		}
	}
	err := o.Object.Remove(ctx)
	if err != nil {
		return err
	}

	fs.Debugf(o, "removing object")
	_ = o.CacheFs.cache.RemoveObject(o.abs())
	_ = o.CacheFs.cache.removePendingUpload(o.abs())
	parentCd := NewDirectory(o.CacheFs, cleanPath(path.Dir(o.Remote())))
	_ = o.CacheFs.cache.ExpireDir(parentCd)
	// advertise to ChangeNotify if wrapped doesn't do that
	o.CacheFs.notifyChangeUpstreamIfNeeded(parentCd.Remote(), fs.EntryDirectory)

	return nil
}

// Hash requests a hash of the object and stores in the cache
// since it might or might not be called, this is lazy loaded
func (o *Object) Hash(ctx context.Context, ht hash.Type) (string, error) {
	_ = o.refresh(ctx)
	o.cacheHashesMu.Lock()
	if o.CacheHashes == nil {
		o.CacheHashes = make(map[hash.Type]string)
	}
	cachedHash, found := o.CacheHashes[ht]
	o.cacheHashesMu.Unlock()
	if found {
		return cachedHash, nil
	}
	if err := o.refreshFromSource(ctx, false); err != nil {
		return "", err
	}
	liveHash, err := o.Object.Hash(ctx, ht)
	if err != nil {
		return "", err
	}
	o.cacheHashesMu.Lock()
	o.CacheHashes[ht] = liveHash
	o.cacheHashesMu.Unlock()

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

func (o *Object) isTempFile() bool {
	_, err := o.CacheFs.cache.SearchPendingUpload(o.abs())
	if err != nil {
		o.CacheType = objectInCache
		return false
	}

	o.CacheType = objectPendingUpload
	return true
}

func (o *Object) tempFileStartedUpload() bool {
	started, err := o.CacheFs.cache.SearchPendingUpload(o.abs())
	if err != nil {
		return false
	}
	return started
}

// UnWrap returns the Object that this Object is wrapping or
// nil if it isn't wrapping anything
func (o *Object) UnWrap() fs.Object {
	return o.Object
}

var (
	_ fs.Object          = (*Object)(nil)
	_ fs.ObjectUnWrapper = (*Object)(nil)
)
