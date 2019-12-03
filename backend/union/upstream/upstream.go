package upstream

import (
	"context"
	"io"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
)

var (
	// ErrUsageFieldNotSupported stats the usage field is not supported by the backend
	ErrUsageFieldNotSupported = errors.New("this usage field is not supported")
)

const (
	unInitilized uint32 = iota
	initilizing
	normal
	updating
)

// Fs is a wrap of any fs and its configs
type Fs struct {
	fs.Fs
	writable    bool
	creatable   bool
	usage       *fs.Usage     // Cache the usage
	cacheMutex  sync.RWMutex
	cacheExpiry int64     // usage cache expiry time
	cacheTime   time.Duration // cache duration
	cacheState  uint32        // if the cache is updating
}

// Directory describes a wrapped Directory
//
// This is a wrapped Directory which contains the upstream Fs
type Directory struct {
	fs.Directory
	f *Fs
}

// Object describes a wrapped Object
//
// This is a wrapped Object which contains the upstream Fs
type Object struct {
	fs.Object
	f *Fs
}

// Entry describe a warpped fs.DirEntry interface with the
// information of upstream Fs
type Entry interface {
	fs.DirEntry
	UpstreamFs() *Fs
}

// New creates a new Fs based on the
// string formatted `type:root_path(:ro/:nc)`
func New(remote, root string, cacheTime time.Duration) (*Fs, error) {
	_, configName, fsPath, err := fs.ParseRemote(remote)
	if err != nil {
		return nil, err
	}
	rFs := &Fs{
		writable:    true,
		creatable:   true,
		cacheExpiry: time.Now().Unix(),
		cacheTime:   cacheTime,
		usage:       &fs.Usage{},
	}
	if strings.HasSuffix(fsPath, ":ro") {
		rFs.writable = false
		rFs.creatable = false
		fsPath = fsPath[0 : len(fsPath)-3]
	} else if strings.HasSuffix(fsPath, ":nc") {
		rFs.writable = true
		rFs.creatable = false
		fsPath = fsPath[0 : len(fsPath)-3]
	}
	var rootString = path.Join(fsPath, filepath.ToSlash(root))
	if configName != "local" {
		rootString = configName + ":" + rootString
	}
	myFs, err := cache.Get(rootString)
	if err != nil && err != fs.ErrorIsFile {
		return nil, err
	}
	rFs.Fs = myFs
	return rFs, err
}

// WrapDirectory wraps a fs.Directory to include the info
// of the upstream Fs
func (f *Fs) WrapDirectory(e fs.Directory) *Directory {
	if e == nil {
		return nil
	}
	return &Directory{
		Directory: e,
		f:         f,
	}
}

// WrapObject wraps a fs.Object to include the info
// of the upstream Fs
func (f *Fs) WrapObject(o fs.Object) *Object {
	if o == nil {
		return nil
	}
	return &Object{
		Object: o,
		f:      f,
	}
}

// WrapEntry wraps a fs.DirEntry to include the info
// of the upstream Fs
func (f *Fs) WrapEntry(e fs.DirEntry) (Entry, error) {
	switch e.(type) {
	case fs.Object:
		return f.WrapObject(e.(fs.Object)), nil
	case fs.Directory:
		return f.WrapDirectory(e.(fs.Directory)), nil
	default:
		return nil, errors.Errorf("unknown object type %T", e)
	}
}

// UpstreamFs get the upstream Fs the entry is stored in
func (e *Directory) UpstreamFs() *Fs {
	return e.f
}

// UpstreamFs get the upstream Fs the entry is stored in
func (o *Object) UpstreamFs() *Fs {
	return o.f
}

// UnWrap returns the Object that this Object is wrapping or
// nil if it isn't wrapping anything
func (o *Object) UnWrap() fs.Object {
	return o.Object
}

// IsCreatable return if the fs is allowed to create new objects
func (f *Fs) IsCreatable() bool {
	return f.creatable
}

// IsWritable return if the fs is allowed to write
func (f *Fs) IsWritable() bool {
	return f.writable
}

// Put in to the remote path with the modTime given of the given size
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	o, err := f.Fs.Put(ctx, in, src, options...)
	if err != nil {
		return o, err
	}
	f.cacheMutex.Lock()
	defer f.cacheMutex.Unlock()
	size := src.Size()
	if f.usage.Used != nil {
		*f.usage.Used += size
	}
	if f.usage.Free != nil {
		*f.usage.Free -= size
	}
	return o, nil
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	do := f.Features().PutStream
	if do == nil {
		return nil, fs.ErrorNotImplemented
	}
	o, err := do(ctx, in, src, options...)
	if err != nil {
		return o, err
	}
	f.cacheMutex.Lock()
	defer f.cacheMutex.Unlock()
	size := o.Size()
	if f.usage.Used != nil {
		*f.usage.Used += size
	}
	if f.usage.Free != nil {
		*f.usage.Free -= size
	}
	return o, nil
}

// Update in to the object with the modTime given of the given size
//
// When called from outside a Fs by rclone, src.Size() will always be >= 0.
// But for unknown-sized objects (indicated by src.Size() == -1), Upload should either
// return an error or update the object properly (rather than e.g. calling panic).
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	size := o.Size()
	err := o.Object.Update(ctx, in, src, options...)
	if err != nil {
		return err
	}
	o.f.cacheMutex.Lock()
	defer o.f.cacheMutex.Unlock()
	delta := o.Size() - size
	if delta <= 0 {
		return nil
	}
	if o.f.usage.Used != nil {
		*o.f.usage.Used += size
	}
	if o.f.usage.Free != nil {
		*o.f.usage.Free -= size
	}
	return nil
}

// About gets quota information from the Fs
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	if atomic.LoadInt64(&f.cacheExpiry) <= time.Now().Unix() {
		err := f.updateUsage()
		if err != nil {
			return nil, ErrUsageFieldNotSupported
		}
	}
	f.cacheMutex.RLock()
	defer f.cacheMutex.RUnlock()
	return f.usage, nil
}

// GetFreeSpace get the free space of the fs
func (f *Fs) GetFreeSpace() (int64, error) {
	if atomic.LoadInt64(&f.cacheExpiry) <= time.Now().Unix() {
		err := f.updateUsage()
		if err != nil {
			return 0, ErrUsageFieldNotSupported
		}
	}
	f.cacheMutex.RLock()
	defer f.cacheMutex.RUnlock()
	if f.usage.Free == nil {
		return 0, ErrUsageFieldNotSupported
	}
	return *f.usage.Free, nil
}

// GetUsedSpace get the used space of the fs
func (f *Fs) GetUsedSpace() (int64, error) {
	if atomic.LoadInt64(&f.cacheExpiry) <= time.Now().Unix() {
		err := f.updateUsage()
		if err != nil {
			return 0, ErrUsageFieldNotSupported
		}
	}
	f.cacheMutex.RLock()
	defer f.cacheMutex.RUnlock()
	if f.usage.Used == nil {
		return 0, ErrUsageFieldNotSupported
	}
	return *f.usage.Used, nil
}

func (f *Fs) updateUsage() error {
	if do := f.Fs.Features().About; do == nil {
		return ErrUsageFieldNotSupported
	}
	if atomic.LoadUint32(&f.cacheState) == unInitilized {
		f.cacheMutex.Lock()
		defer f.cacheMutex.Unlock()
		if !atomic.CompareAndSwapUint32(&f.cacheState, unInitilized, initilizing) {
			return f.updateUsage()
		}
		return f.updateUsageCore(false)
	}
	if atomic.CompareAndSwapUint32(&f.cacheState, normal, updating) {
		go f.updateUsageCore(true)
	}
	return nil
}

func (f *Fs) updateUsageCore(lock bool) error {
	defer func() {
		atomic.StoreInt64(&f.cacheExpiry, time.Now().Add(f.cacheTime).Unix())
		atomic.StoreUint32(&f.cacheState, normal)
	}()
	if (lock) {
		f.cacheMutex.Lock()
		defer f.cacheMutex.Unlock()
	}
	// Run in background, should not be cancelled by user
	ctx, _ := context.WithTimeout(context.Background(), 15*time.Second)
	usage, err := f.Features().About(ctx)
	if err != nil {
		return err
	}
	f.usage = usage
	return nil
}