// Package squashfs implements a squashfs archiver for the archive backend
package squashfs

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/diskfs/go-diskfs/filesystem/squashfs"
	"github.com/rclone/rclone/backend/archive/archiver"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/log"
	"github.com/rclone/rclone/lib/readers"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfscommon"
)

func init() {
	archiver.Register(archiver.Archiver{
		New:       New,
		Extension: ".sqfs",
	})
}

// Fs represents a wrapped fs.Fs
type Fs struct {
	f           fs.Fs
	wrapper     fs.Fs
	name        string
	features    *fs.Features // optional features
	vfs         *vfs.VFS
	sqfs        *squashfs.FileSystem // interface to the squashfs
	c           *cache
	node        vfs.Node // squashfs file object - set if reading
	remote      string   // remote of the squashfs file object
	prefix      string   // position for objects
	prefixSlash string   // position for objects with a slash on
	root        string   // position to read from within the archive
}

// New constructs an Fs from the (wrappedFs, remote) with the objects
// prefix with prefix and rooted at root
func New(ctx context.Context, wrappedFs fs.Fs, remote, prefix, root string) (fs.Fs, error) {
	// FIXME vfs cache?
	// FIXME could factor out ReadFileHandle and just use that rather than the full VFS
	fs.Debugf(nil, "Squashfs: New: remote=%q, prefix=%q, root=%q", remote, prefix, root)
	vfsOpt := vfscommon.Opt
	vfsOpt.ReadWait = 0
	VFS := vfs.New(wrappedFs, &vfsOpt)
	node, err := VFS.Stat(remote)
	if err != nil {
		return nil, fmt.Errorf("failed to find %q archive: %w", remote, err)
	}

	c := newCache(node)

	// FIXME blocksize
	sqfs, err := squashfs.Read(c, node.Size(), 0, 1024*1024)
	if err != nil {
		return nil, fmt.Errorf("failed to read squashfs: %w", err)
	}

	f := &Fs{
		f:           wrappedFs,
		name:        path.Join(fs.ConfigString(wrappedFs), remote),
		vfs:         VFS,
		node:        node,
		sqfs:        sqfs,
		c:           c,
		remote:      remote,
		root:        strings.Trim(root, "/"),
		prefix:      prefix,
		prefixSlash: prefix + "/",
	}
	if prefix == "" {
		f.prefixSlash = ""
	}

	singleObject := false

	// Find the directory the root points to
	if f.root != "" && !strings.HasSuffix(root, "/") {
		native, err := f.toNative("")
		if err == nil {
			native = strings.TrimRight(native, "/")
			_, err := f.newObjectNative(native)
			if err == nil {
				// If it pointed to a file, find the directory above
				f.root = path.Dir(f.root)
				if f.root == "." || f.root == "/" {
					f.root = ""
				}
			}
		}
	}

	// FIXME
	// the features here are ones we could support, and they are
	// ANDed with the ones from wrappedFs
	//
	// FIXME some of these need to be forced on - CanHaveEmptyDirectories
	f.features = (&fs.Features{
		CaseInsensitive:         false,
		DuplicateFiles:          false,
		ReadMimeType:            false, // MimeTypes not supported with gsquashfs
		WriteMimeType:           false,
		BucketBased:             false,
		CanHaveEmptyDirectories: true,
	}).Fill(ctx, f).Mask(ctx, wrappedFs).WrapsFs(f, wrappedFs)

	if singleObject {
		return f, fs.ErrorIsFile
	}
	return f, nil
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
	return fmt.Sprintf("Squashfs %q", f.name)
}

// This turns a remote into a native path in the squashfs starting with a /
func (f *Fs) toNative(remote string) (string, error) {
	native := strings.Trim(remote, "/")
	if f.prefix == "" {
		native = "/" + native
	} else if native == f.prefix {
		native = "/"
	} else if !strings.HasPrefix(native, f.prefixSlash) {
		return "", fmt.Errorf("internal error: %q doesn't start with prefix %q", native, f.prefixSlash)
	} else {
		native = native[len(f.prefix):]
	}
	if f.root != "" {
		native = "/" + f.root + native
	}
	return native, nil
}

// Turn a (nativeDir, leaf) into a remote
func (f *Fs) fromNative(nativeDir string, leaf string) string {
	// fs.Debugf(nil, "nativeDir = %q, leaf = %q, root=%q", nativeDir, leaf, f.root)
	dir := nativeDir
	if f.root != "" {
		dir = strings.TrimPrefix(dir, "/"+f.root)
	}
	remote := f.prefixSlash + strings.Trim(path.Join(dir, leaf), "/")
	// fs.Debugf(nil, "dir = %q, remote=%q", dir, remote)
	return remote
}

// Convert a FileInfo into an Object from native dir
func (f *Fs) objectFromFileInfo(nativeDir string, item squashfs.FileStat) *Object {
	return &Object{
		fs:      f,
		remote:  f.fromNative(nativeDir, item.Name()),
		size:    item.Size(),
		modTime: item.ModTime(),
		item:    item,
	}
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
	defer log.Trace(f, "dir=%q", dir)("entries=%v, err=%v", &entries, &err)

	nativeDir, err := f.toNative(dir)
	if err != nil {
		return nil, err
	}

	items, err := f.sqfs.ReadDir(nativeDir)
	if err != nil {
		return nil, fmt.Errorf("read squashfs: couldn't read directory: %w", err)
	}

	entries = make(fs.DirEntries, 0, len(items))
	for _, fi := range items {
		item, ok := fi.(squashfs.FileStat)
		if !ok {
			return nil, fmt.Errorf("internal error: unexpected type for %q: %T", fi.Name(), fi)
		}
		// fs.Debugf(item.Name(), "entry = %#v", item)
		var entry fs.DirEntry
		if err != nil {
			return nil, fmt.Errorf("error reading item %q: %q", item.Name(), err)
		}
		if item.IsDir() {
			var remote = f.fromNative(nativeDir, item.Name())
			entry = fs.NewDir(remote, item.ModTime())
		} else {
			if item.Mode().IsRegular() {
				entry = f.objectFromFileInfo(nativeDir, item)
			} else {
				fs.Debugf(item.Name(), "FIXME Not regular file - skipping")
				continue
			}
		}
		entries = append(entries, entry)
	}

	// fs.Debugf(f, "dir=%q, entries=%v", dir, entries)
	return entries, nil
}

// newObjectNative finds the object at the native path passed in
func (f *Fs) newObjectNative(nativePath string) (o fs.Object, err error) {
	// get the path and filename
	dir, leaf := path.Split(nativePath)
	dir = strings.TrimRight(dir, "/")
	leaf = strings.Trim(leaf, "/")

	// FIXME need to detect directory not found
	fis, err := f.sqfs.ReadDir(dir)
	if err != nil {

		return nil, fs.ErrorObjectNotFound
	}

	for _, fi := range fis {
		if fi.Name() == leaf {
			if fi.IsDir() {
				return nil, fs.ErrorNotAFile
			}
			item, ok := fi.(squashfs.FileStat)
			if !ok {
				return nil, fmt.Errorf("internal error: unexpected type for %q: %T", fi.Name(), fi)
			}
			o = f.objectFromFileInfo(dir, item)
			break
		}
	}
	if o == nil {
		return nil, fs.ErrorObjectNotFound
	}
	return o, nil
}

// NewObject finds the Object at remote.
func (f *Fs) NewObject(ctx context.Context, remote string) (o fs.Object, err error) {
	defer log.Trace(f, "remote=%q", remote)("obj=%v, err=%v", &o, &err)

	nativePath, err := f.toNative(remote)
	if err != nil {
		return nil, err
	}
	return f.newObjectNative(nativePath)
}

// Precision of the ModTimes in this Fs
func (f *Fs) Precision() time.Duration {
	return time.Second
}

// Mkdir makes the directory (container, bucket)
//
// Shouldn't return an error if it already exists
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	return vfs.EROFS
}

// Rmdir removes the directory (container, bucket) if empty
//
// Return an error if it doesn't exist or isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return vfs.EROFS
}

// Put in to the remote path with the modTime given of the given size
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (o fs.Object, err error) {
	return nil, vfs.EROFS
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.None)
}

// UnWrap returns the Fs that this Fs is wrapping
func (f *Fs) UnWrap() fs.Fs {
	return f.f
}

// WrapFs returns the Fs that is wrapping this Fs
func (f *Fs) WrapFs() fs.Fs {
	return f.wrapper
}

// SetWrapper sets the Fs that is wrapping this Fs
func (f *Fs) SetWrapper(wrapper fs.Fs) {
	f.wrapper = wrapper
}

// Object describes an object to be read from the raw squashfs file
type Object struct {
	fs      *Fs
	remote  string
	size    int64
	modTime time.Time
	item    squashfs.FileStat
}

// Fs returns read only access to the Fs that this object is part of
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

// Turn a squashfs path into a full path for the parent Fs
// func (o *Object) path(remote string) string {
// 	return path.Join(o.fs.prefix, remote)
// }

// Remote returns the remote path
func (o *Object) Remote() string {
	return o.remote
}

// Size returns the size of the file
func (o *Object) Size() int64 {
	return o.size
}

// ModTime returns the modification time of the object
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.modTime
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	return vfs.EROFS
}

// Storable raturns a boolean indicating if this object is storable
func (o *Object) Storable() bool {
	return true
}

// Hash returns the selected checksum of the file
// If no checksum is available it returns ""
func (o *Object) Hash(ctx context.Context, ht hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// Open opens the file for read.  Call Close() on the returned io.ReadCloser
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (rc io.ReadCloser, err error) {
	var offset, limit int64 = 0, -1
	for _, option := range options {
		switch x := option.(type) {
		case *fs.SeekOption:
			offset = x.Offset
		case *fs.RangeOption:
			offset, limit = x.Decode(o.Size())
		default:
			if option.Mandatory() {
				fs.Logf(o, "Unsupported mandatory option: %v", option)
			}
		}
	}

	remote, err := o.fs.toNative(o.remote)
	if err != nil {
		return nil, err
	}

	fs.Debugf(o, "Opening %q", remote)
	//fh, err := o.fs.sqfs.OpenFile(remote, os.O_RDONLY)
	fh, err := o.item.Open()
	if err != nil {
		return nil, err
	}

	// discard data from start as necessary
	if offset > 0 {
		_, err = fh.Seek(offset, io.SeekStart)
		if err != nil {
			return nil, err
		}
	}
	// If limited then don't return everything
	if limit >= 0 {
		fs.Debugf(nil, "limit=%d, offset=%d, options=%v", limit, offset, options)
		return readers.NewLimitedReadCloser(fh, limit), nil
	}

	return fh, nil
}

// Update in to the object with the modTime given of the given size
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	return vfs.EROFS
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	return vfs.EROFS
}

// Check the interfaces are satisfied
var (
	_ fs.Fs        = (*Fs)(nil)
	_ fs.UnWrapper = (*Fs)(nil)
	_ fs.Wrapper   = (*Fs)(nil)
	_ fs.Object    = (*Object)(nil)
)
