// Package object defines some useful Objects
package object

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/ioutil"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/hash"
)

// NewStaticObjectInfo returns a static ObjectInfo
// If hashes is nil and fs is not nil, the hash map will be replaced with
// empty hashes of the types supported by the fs.
func NewStaticObjectInfo(remote string, modTime time.Time, size int64, storable bool, hashes map[hash.Type]string, fs fs.Info) fs.ObjectInfo {
	info := &staticObjectInfo{
		remote:   remote,
		modTime:  modTime,
		size:     size,
		storable: storable,
		hashes:   hashes,
		fs:       fs,
	}
	if fs != nil && hashes == nil {
		set := fs.Hashes().Array()
		info.hashes = make(map[hash.Type]string)
		for _, ht := range set {
			info.hashes[ht] = ""
		}
	}
	return info
}

type staticObjectInfo struct {
	remote   string
	modTime  time.Time
	size     int64
	storable bool
	hashes   map[hash.Type]string
	fs       fs.Info
}

func (i *staticObjectInfo) Fs() fs.Info                           { return i.fs }
func (i *staticObjectInfo) Remote() string                        { return i.remote }
func (i *staticObjectInfo) String() string                        { return i.remote }
func (i *staticObjectInfo) ModTime(ctx context.Context) time.Time { return i.modTime }
func (i *staticObjectInfo) Size() int64                           { return i.size }
func (i *staticObjectInfo) Storable() bool                        { return i.storable }
func (i *staticObjectInfo) Hash(ctx context.Context, h hash.Type) (string, error) {
	if len(i.hashes) == 0 {
		return "", hash.ErrUnsupported
	}
	if hash, ok := i.hashes[h]; ok {
		return hash, nil
	}
	return "", hash.ErrUnsupported
}

// MemoryFs is an in memory Fs, it only supports FsInfo and Put
var MemoryFs memoryFs

// memoryFs is an in memory fs
type memoryFs struct{}

// Name of the remote (as passed into NewFs)
func (memoryFs) Name() string { return "memory" }

// Root of the remote (as passed into NewFs)
func (memoryFs) Root() string { return "" }

// String returns a description of the FS
func (memoryFs) String() string { return "memory" }

// Precision of the ModTimes in this Fs
func (memoryFs) Precision() time.Duration { return time.Nanosecond }

// Returns the supported hash types of the filesystem
func (memoryFs) Hashes() hash.Set { return hash.Supported }

// Features returns the optional features of this Fs
func (memoryFs) Features() *fs.Features { return &fs.Features{} }

// List the objects and directories in dir into entries.  The
// entries can be returned in any order but should be for a
// complete directory.
//
// dir should be "" to list the root, and should not have
// trailing slashes.
//
// This should return ErrDirNotFound if the directory isn't
// found.
func (memoryFs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	return nil, nil
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error ErrorObjectNotFound.
func (memoryFs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	return nil, fs.ErrorObjectNotFound
}

// Put in to the remote path with the modTime given of the given size
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
func (memoryFs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	o := NewMemoryObject(src.Remote(), src.ModTime(ctx), nil)
	return o, o.Update(ctx, in, src, options...)
}

// Mkdir makes the directory (container, bucket)
//
// Shouldn't return an error if it already exists
func (memoryFs) Mkdir(ctx context.Context, dir string) error {
	return errors.New("memoryFs: can't make directory")
}

// Rmdir removes the directory (container, bucket) if empty
//
// Return an error if it doesn't exist or isn't empty
func (memoryFs) Rmdir(ctx context.Context, dir string) error {
	return fs.ErrorDirNotFound
}

var _ fs.Fs = MemoryFs

// MemoryObject is an in memory object
type MemoryObject struct {
	remote  string
	modTime time.Time
	content []byte
}

// NewMemoryObject returns an in memory Object with the modTime and content passed in
func NewMemoryObject(remote string, modTime time.Time, content []byte) *MemoryObject {
	return &MemoryObject{
		remote:  remote,
		modTime: modTime,
		content: content,
	}
}

// Content returns the underlying buffer
func (o *MemoryObject) Content() []byte {
	return o.content
}

// Fs returns read only access to the Fs that this object is part of
func (o *MemoryObject) Fs() fs.Info {
	return MemoryFs
}

// Remote returns the remote path
func (o *MemoryObject) Remote() string {
	return o.remote
}

// String returns a description of the Object
func (o *MemoryObject) String() string {
	return o.remote
}

// ModTime returns the modification date of the file
func (o *MemoryObject) ModTime(ctx context.Context) time.Time {
	return o.modTime
}

// Size returns the size of the file
func (o *MemoryObject) Size() int64 {
	return int64(len(o.content))
}

// Storable says whether this object can be stored
func (o *MemoryObject) Storable() bool {
	return true
}

// Hash returns the requested hash of the contents
func (o *MemoryObject) Hash(ctx context.Context, h hash.Type) (string, error) {
	hash, err := hash.NewMultiHasherTypes(hash.Set(h))
	if err != nil {
		return "", err
	}
	_, err = hash.Write(o.content)
	if err != nil {
		return "", err
	}
	return hash.Sums()[h], nil
}

// SetModTime sets the metadata on the object to set the modification date
func (o *MemoryObject) SetModTime(ctx context.Context, modTime time.Time) error {
	o.modTime = modTime
	return nil
}

// Open opens the file for read.  Call Close() on the returned io.ReadCloser
func (o *MemoryObject) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	content := o.content
	for _, option := range options {
		switch x := option.(type) {
		case *fs.RangeOption:
			content = o.content[x.Start:x.End]
		case *fs.SeekOption:
			content = o.content[x.Offset:]
		default:
			if option.Mandatory() {
				fs.Logf(o, "Unsupported mandatory option: %v", option)
			}
		}
	}
	return ioutil.NopCloser(bytes.NewBuffer(content)), nil
}

// Update in to the object with the modTime given of the given size
//
// This re-uses the internal buffer if at all possible.
func (o *MemoryObject) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	size := src.Size()
	if size == 0 {
		o.content = nil
	} else if size < 0 || int64(cap(o.content)) < size {
		o.content, err = ioutil.ReadAll(in)
	} else {
		o.content = o.content[:size]
		_, err = io.ReadFull(in, o.content)
	}
	o.modTime = src.ModTime(ctx)
	return err
}

// Remove this object
func (o *MemoryObject) Remove(ctx context.Context) error {
	return errors.New("memoryObject.Remove not supported")
}
