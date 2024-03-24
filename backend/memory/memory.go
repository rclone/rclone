// Package memory provides an interface to an in memory object storage system
package memory

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/walk"
	"github.com/rclone/rclone/lib/bucket"
)

var (
	hashType = hash.MD5
	// the object storage is persistent
	buckets = newBucketsInfo()
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "memory",
		Description: "In memory object storage system.",
		NewFs:       NewFs,
		Options:     []fs.Option{},
	})
}

// Options defines the configuration for this backend
type Options struct{}

// Fs represents a remote memory server
type Fs struct {
	name          string       // name of this remote
	root          string       // the path we are working on if any
	opt           Options      // parsed config options
	rootBucket    string       // bucket part of root (if any)
	rootDirectory string       // directory part of root (if any)
	features      *fs.Features // optional features
}

// bucketsInfo holds info about all the buckets
type bucketsInfo struct {
	mu      sync.RWMutex
	buckets map[string]*bucketInfo
}

func newBucketsInfo() *bucketsInfo {
	return &bucketsInfo{
		buckets: make(map[string]*bucketInfo, 16),
	}
}

// getBucket gets a names bucket or nil
func (bi *bucketsInfo) getBucket(name string) (b *bucketInfo) {
	bi.mu.RLock()
	b = bi.buckets[name]
	bi.mu.RUnlock()
	return b
}

// makeBucket returns the bucket or makes it
func (bi *bucketsInfo) makeBucket(name string) (b *bucketInfo) {
	bi.mu.Lock()
	defer bi.mu.Unlock()
	b = bi.buckets[name]
	if b != nil {
		return b
	}
	b = newBucketInfo()
	bi.buckets[name] = b
	return b
}

// deleteBucket deleted the bucket or returns an error
func (bi *bucketsInfo) deleteBucket(name string) error {
	bi.mu.Lock()
	defer bi.mu.Unlock()
	b := bi.buckets[name]
	if b == nil {
		return fs.ErrorDirNotFound
	}
	if !b.isEmpty() {
		return fs.ErrorDirectoryNotEmpty
	}
	delete(bi.buckets, name)
	return nil
}

// getObjectData gets an object from (bucketName, bucketPath) or nil
func (bi *bucketsInfo) getObjectData(bucketName, bucketPath string) (od *objectData) {
	b := bi.getBucket(bucketName)
	if b == nil {
		return nil
	}
	return b.getObjectData(bucketPath)
}

// updateObjectData updates an object from (bucketName, bucketPath)
func (bi *bucketsInfo) updateObjectData(bucketName, bucketPath string, od *objectData) {
	b := bi.makeBucket(bucketName)
	b.mu.Lock()
	b.objects[bucketPath] = od
	b.mu.Unlock()
}

// removeObjectData removes an object from (bucketName, bucketPath) returning true if removed
func (bi *bucketsInfo) removeObjectData(bucketName, bucketPath string) (removed bool) {
	b := bi.getBucket(bucketName)
	if b != nil {
		b.mu.Lock()
		od := b.objects[bucketPath]
		if od != nil {
			delete(b.objects, bucketPath)
			removed = true
		}
		b.mu.Unlock()
	}
	return removed
}

// bucketInfo holds info about a single bucket
type bucketInfo struct {
	mu      sync.RWMutex
	objects map[string]*objectData
}

func newBucketInfo() *bucketInfo {
	return &bucketInfo{
		objects: make(map[string]*objectData, 16),
	}
}

// getBucket gets a names bucket or nil
func (bi *bucketInfo) getObjectData(name string) (od *objectData) {
	bi.mu.RLock()
	od = bi.objects[name]
	bi.mu.RUnlock()
	return od
}

// getBucket gets a names bucket or nil
func (bi *bucketInfo) isEmpty() (empty bool) {
	bi.mu.RLock()
	empty = len(bi.objects) == 0
	bi.mu.RUnlock()
	return empty
}

// the object data and metadata
type objectData struct {
	modTime  time.Time
	hash     string
	mimeType string
	data     []byte
}

// Object describes a memory object
type Object struct {
	fs     *Fs         // what this object is part of
	remote string      // The remote path
	od     *objectData // the object data
}

// ------------------------------------------------------------

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return f.root
}

// String converts this Fs to a string
func (f *Fs) String() string {
	return fmt.Sprintf("Memory root '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// parsePath parses a remote 'url'
func parsePath(path string) (root string) {
	root = strings.Trim(path, "/")
	return
}

// split returns bucket and bucketPath from the rootRelativePath
// relative to f.root
func (f *Fs) split(rootRelativePath string) (bucketName, bucketPath string) {
	return bucket.Split(path.Join(f.root, rootRelativePath))
}

// split returns bucket and bucketPath from the object
func (o *Object) split() (bucket, bucketPath string) {
	return o.fs.split(o.remote)
}

// setRoot changes the root of the Fs
func (f *Fs) setRoot(root string) {
	f.root = parsePath(root)
	f.rootBucket, f.rootDirectory = bucket.Split(f.root)
}

// NewFs constructs an Fs from the path, bucket:path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	root = strings.Trim(root, "/")
	f := &Fs{
		name: name,
		root: root,
		opt:  *opt,
	}
	f.setRoot(root)
	f.features = (&fs.Features{
		ReadMimeType:      true,
		WriteMimeType:     true,
		BucketBased:       true,
		BucketBasedRootOK: true,
	}).Fill(ctx, f)
	if f.rootBucket != "" && f.rootDirectory != "" {
		od := buckets.getObjectData(f.rootBucket, f.rootDirectory)
		if od != nil {
			newRoot := path.Dir(f.root)
			if newRoot == "." {
				newRoot = ""
			}
			f.setRoot(newRoot)
			// return an error with an fs which points to the parent
			err = fs.ErrorIsFile
		}
	}
	return f, err
}

// newObject makes an object from a remote and an objectData
func (f *Fs) newObject(remote string, od *objectData) *Object {
	return &Object{fs: f, remote: remote, od: od}
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	bucket, bucketPath := f.split(remote)
	od := buckets.getObjectData(bucket, bucketPath)
	if od == nil {
		return nil, fs.ErrorObjectNotFound
	}
	return f.newObject(remote, od), nil
}

// listFn is called from list to handle an object.
type listFn func(remote string, entry fs.DirEntry, isDirectory bool) error

// list the buckets to fn
func (f *Fs) list(ctx context.Context, bucket, directory, prefix string, addBucket bool, recurse bool, fn listFn) (err error) {
	if prefix != "" {
		prefix += "/"
	}
	if directory != "" {
		directory += "/"
	}
	b := buckets.getBucket(bucket)
	if b == nil {
		return fs.ErrorDirNotFound
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	dirs := make(map[string]struct{})
	for absPath, od := range b.objects {
		if strings.HasPrefix(absPath, directory) {
			remote := absPath[len(prefix):]
			if !recurse {
				localPath := absPath[len(directory):]
				slash := strings.IndexRune(localPath, '/')
				if slash >= 0 {
					// send a directory if have a slash
					dir := strings.TrimPrefix(directory, f.rootDirectory+"/") + localPath[:slash]
					if addBucket {
						dir = path.Join(bucket, dir)
					}
					_, found := dirs[dir]
					if !found {
						err = fn(dir, fs.NewDir(dir, time.Time{}), true)
						if err != nil {
							return err
						}
						dirs[dir] = struct{}{}
					}
					continue // don't send this file if not recursing
				}
			}
			// send an object
			if addBucket {
				remote = path.Join(bucket, remote)
			}
			err = fn(remote, f.newObject(remote, od), false)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// listDir lists the bucket to the entries
func (f *Fs) listDir(ctx context.Context, bucket, directory, prefix string, addBucket bool) (entries fs.DirEntries, err error) {
	// List the objects and directories
	err = f.list(ctx, bucket, directory, prefix, addBucket, false, func(remote string, entry fs.DirEntry, isDirectory bool) error {
		entries = append(entries, entry)
		return nil
	})
	return entries, err
}

// listBuckets lists the buckets to entries
func (f *Fs) listBuckets(ctx context.Context) (entries fs.DirEntries, err error) {
	buckets.mu.RLock()
	defer buckets.mu.RUnlock()
	for name := range buckets.buckets {
		entries = append(entries, fs.NewDir(name, time.Time{}))
	}
	return entries, nil
}

// List the objects and directories in dir into entries.  The
// entries can be returned in any order but should be for a
// complete directory.
//
// dir should be "" to list the root, and should not have
// trailing slashes.
//
// This should return ErrDirNotFound if the directory isn't
// found.
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	// defer fslog.Trace(dir, "")("entries = %q, err = %v", &entries, &err)
	bucket, directory := f.split(dir)
	if bucket == "" {
		if directory != "" {
			return nil, fs.ErrorListBucketRequired
		}
		return f.listBuckets(ctx)
	}
	return f.listDir(ctx, bucket, directory, f.rootDirectory, f.rootBucket == "")
}

// ListR lists the objects and directories of the Fs starting
// from dir recursively into out.
//
// dir should be "" to start from the root, and should not
// have trailing slashes.
//
// This should return ErrDirNotFound if the directory isn't
// found.
//
// It should call callback for each tranche of entries read.
// These need not be returned in any particular order.  If
// callback returns an error then the listing will stop
// immediately.
//
// Don't implement this unless you have a more efficient way
// of listing recursively that doing a directory traversal.
func (f *Fs) ListR(ctx context.Context, dir string, callback fs.ListRCallback) (err error) {
	bucket, directory := f.split(dir)
	list := walk.NewListRHelper(callback)
	entries := fs.DirEntries{}
	listR := func(bucket, directory, prefix string, addBucket bool) error {
		err = f.list(ctx, bucket, directory, prefix, addBucket, true, func(remote string, entry fs.DirEntry, isDirectory bool) error {
			entries = append(entries, entry) // can't list.Add here -- could deadlock
			return nil
		})
		if err != nil {
			return err
		}
		for _, entry := range entries {
			err = list.Add(entry)
			if err != nil {
				return err
			}
		}
		return nil
	}
	if bucket == "" {
		entries, err := f.listBuckets(ctx)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			err = list.Add(entry)
			if err != nil {
				return err
			}
			bucket := entry.Remote()
			err = listR(bucket, "", f.rootDirectory, true)
			if err != nil {
				return err
			}
		}
	} else {
		err = listR(bucket, directory, f.rootDirectory, f.rootBucket == "")
		if err != nil {
			return err
		}
	}
	return list.Flush()
}

// Put the object into the bucket
//
// Copy the reader in to the new object which is returned.
//
// The new object may have been created if an error is returned
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	// Temporary Object under construction
	fs := &Object{
		fs:     f,
		remote: src.Remote(),
		od: &objectData{
			modTime: src.ModTime(ctx),
		},
	}
	return fs, fs.Update(ctx, in, src, options...)
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.Put(ctx, in, src, options...)
}

// Mkdir creates the bucket if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	bucket, _ := f.split(dir)
	buckets.makeBucket(bucket)
	return nil
}

// Rmdir deletes the bucket if the fs is at the root
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	bucket, directory := f.split(dir)
	if bucket == "" || directory != "" {
		return nil
	}
	return buckets.deleteBucket(bucket)
}

// Precision of the remote
func (f *Fs) Precision() time.Duration {
	return time.Nanosecond
}

// Copy src to this remote using server-side copy operations.
//
// This is stored with the remote path given.
//
// It returns the destination Object and a possible error.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantCopy
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	dstBucket, dstPath := f.split(remote)
	_ = buckets.makeBucket(dstBucket)
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}
	srcBucket, srcPath := srcObj.split()
	od := buckets.getObjectData(srcBucket, srcPath)
	if od == nil {
		return nil, fs.ErrorObjectNotFound
	}
	odCopy := *od
	buckets.updateObjectData(dstBucket, dstPath, &odCopy)
	return f.NewObject(ctx, remote)
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hashType)
}

// ------------------------------------------------------------

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Return a string version
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.Remote()
}

// Remote returns the remote path
func (o *Object) Remote() string {
	return o.remote
}

// Hash returns the hash of an object returning a lowercase hex string
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	if t != hashType {
		return "", hash.ErrUnsupported
	}
	if o.od.hash == "" {
		sum := md5.Sum(o.od.data)
		o.od.hash = hex.EncodeToString(sum[:])
	}
	return o.od.hash, nil
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	return int64(len(o.od.data))
}

// ModTime returns the modification time of the object
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
//
// SHA-1 will also be updated once the request has completed.
func (o *Object) ModTime(ctx context.Context) (result time.Time) {
	return o.od.modTime
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	o.od.modTime = modTime
	return nil
}

// Storable returns if this object is storable
func (o *Object) Storable() bool {
	return true
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	var offset, limit int64 = 0, -1
	for _, option := range options {
		switch x := option.(type) {
		case *fs.RangeOption:
			offset, limit = x.Decode(int64(len(o.od.data)))
		case *fs.SeekOption:
			offset = x.Offset
		default:
			if option.Mandatory() {
				fs.Logf(o, "Unsupported mandatory option: %v", option)
			}
		}
	}
	if offset > int64(len(o.od.data)) {
		offset = int64(len(o.od.data))
	}
	data := o.od.data[offset:]
	if limit >= 0 {
		if limit > int64(len(data)) {
			limit = int64(len(data))
		}
		data = data[:limit]
	}
	return io.NopCloser(bytes.NewBuffer(data)), nil
}

// Update the object with the contents of the io.Reader, modTime and size
//
// The new object may have been created if an error is returned
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	bucket, bucketPath := o.split()
	data, err := io.ReadAll(in)
	if err != nil {
		return fmt.Errorf("failed to update memory object: %w", err)
	}
	o.od = &objectData{
		data:     data,
		hash:     "",
		modTime:  src.ModTime(ctx),
		mimeType: fs.MimeType(ctx, src),
	}
	buckets.updateObjectData(bucket, bucketPath, o.od)
	return nil
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	bucket, bucketPath := o.split()
	removed := buckets.removeObjectData(bucket, bucketPath)
	if !removed {
		return fs.ErrorObjectNotFound
	}
	return nil
}

// MimeType of an Object if known, "" otherwise
func (o *Object) MimeType(ctx context.Context) string {
	return o.od.mimeType
}

// Check the interfaces are satisfied
var (
	_ fs.Fs          = &Fs{}
	_ fs.Copier      = &Fs{}
	_ fs.PutStreamer = &Fs{}
	_ fs.ListRer     = &Fs{}
	_ fs.Object      = &Object{}
	_ fs.MimeTyper   = &Object{}
)
