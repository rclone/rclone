// Package zip provides wrappers for Fs and Object which read and
// write a zip file.
//
// Use like :zip:remote:path/to/file.zip
package zip

/*
FIXME could maybe make an append to zip file with specially named zip
files having more info in?

So have base zip file file.zip then file.extra000.zip etc.

We read the objects sequentially but can transfer them in parallel.

FIXME what happens when we want to copy a file out of the zip?

So rclone copy remote:file.zip/file/inside.zip?
Maybe rclone copy remote:file.zip#file/inside.zip?
Or just look for first .zip file?

FIXME what happens when writing - need to know the end... Don't have a
backend Shutdown call.

Could have a read only Feature flag so we turn that on when we have a
zip. Then if placed in dest position could error. Might be useful for
http also.

FIXME this will perform poorly for unpacking as the VFS Reader is bad
at multiple streams.

FIXME not writing directories

cf zipinfo
drwxr-xr-x  3.0 unx        0 bx stor 19-Oct-05 12:14 rclone-v1.49.5-linux-amd64/

FIXME can probably check directories better than trailing /

FIXME enormous atexit bodge

FIXME crc32 skip on write bodge

Bodge bodge bodge

*/

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/dirtree"
	"github.com/rclone/rclone/fs/fspath"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/log"
	"github.com/rclone/rclone/lib/atexit"
	"github.com/rclone/rclone/lib/readers"
	"github.com/rclone/rclone/vfs"
)

// Globals
// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "zip",
		Description: "Read or write a zip file",
		NewFs:       NewFs,
		Options:     []fs.Option{},
	})
}

// NewFs constructs an Fs from the path, container:path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	// Parse the root which should be remote:path/to/file.zip
	parent, leaf, err := fspath.Split(root)
	if err != nil {
		return nil, fmt.Errorf("failed to parse root %q: %w", root, err)
	}
	if leaf == "" {
		return nil, fmt.Errorf("failed to parse root %q: not pointing to a file", root)
	}
	root = strings.TrimRight(parent, "/")

	// Create the remote from the cache
	wrappedFs, err := cache.Get(ctx, root)
	if err != nil {
		return nil, fmt.Errorf("failed to make %q to wrap: %w", root, err)
	}
	// FIXME vfs cache?
	// FIXME could factor out ReadFileHandle and just use that rather than the full VFS
	VFS := vfs.New(wrappedFs, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create VFS for %q: %w", root, err)
	}
	node, err := VFS.Stat(leaf)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to find %q object: %w", leaf, err)
	}
	if node != nil && node.IsDir() {
		return nil, fmt.Errorf("can't unzip a directory %q", leaf)
	}
	f := &Fs{
		f:    wrappedFs,
		name: name,
		root: root,
		opt:  *opt,
		vfs:  VFS,
		node: node,
		leaf: leaf,
	}
	// FIXME Maybe delay this until first read size if writing we don't need it?
	if node != nil {
		err = f.readZip()
		if err != nil {
			return nil, fmt.Errorf("failed to open zip file: %w", err)
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
		ReadMimeType:            false, // MimeTypes not supported with gzip
		WriteMimeType:           false,
		BucketBased:             false,
		CanHaveEmptyDirectories: true,
	}).Fill(ctx, f).Mask(ctx, wrappedFs).WrapsFs(f, wrappedFs)

	return f, nil
}

// Options defines the configuration for this backend
type Options struct {
}

// Fs represents a wrapped fs.Fs
type Fs struct {
	f        fs.Fs
	wrapper  fs.Fs
	name     string
	root     string
	opt      Options
	features *fs.Features // optional features
	vfs      *vfs.VFS
	node     vfs.Node        // zip file object - set if reading
	leaf     string          // leaf name of the zip file object
	dt       dirtree.DirTree // read from zipfile

	wrmu sync.Mutex // writing mutex protects the below
	wh   vfs.Handle // write handle
	zw   *zip.Writer
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
	return fmt.Sprintf("Zip '%s:%s'", f.name, f.root)
}

// readZip the zip file into f
func (f *Fs) readZip() (err error) {
	if f.node == nil {
		return fs.ErrorDirNotFound
	}
	size := f.node.Size()
	if size < 0 {
		return errors.New("can't read from zip file with unknown size")
	}
	r, err := f.node.Open(os.O_RDONLY)
	if err != nil {
		return fmt.Errorf("failed to open zip file: %w", err)
	}
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return fmt.Errorf("failed to read zip file: %w", err)
	}
	dt := dirtree.New()
	for _, file := range zr.File {
		remote := strings.Trim(path.Clean(file.Name), "/")
		if remote == "." {
			remote = ""
		}
		if strings.HasSuffix(file.Name, "/") {
			dir := fs.NewDir(remote, file.Modified)
			dt.AddDir(dir)
		} else {
			o := &Object{
				f:      f,
				remote: remote,
				fh:     &file.FileHeader,
				file:   file,
			}
			dt.Add(o)
		}
	}
	dt.CheckParents("")
	dt.Sort()
	f.dt = dt
	fs.Debugf(nil, "dt = %#v", dt)
	return nil
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
	entries, ok := f.dt[dir]
	if !ok {
		return nil, fs.ErrorDirNotFound
	}
	fs.Debugf(f, "dir=%q, entries=%v", dir, entries)
	return entries, nil
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
	for _, entries := range f.dt {
		err = callback(entries)
		if err != nil {
			return err
		}
	}
	return nil
}

// NewObject finds the Object at remote.
func (f *Fs) NewObject(ctx context.Context, remote string) (o fs.Object, err error) {
	defer log.Trace(f, "remote=%q", remote)("obj=%v, err=%v", &o, &err)
	if f.dt == nil {
		return nil, fs.ErrorObjectNotFound
	}
	_, entry := f.dt.Find(remote)
	if entry == nil {
		return nil, fs.ErrorObjectNotFound
	}
	o, ok := entry.(*Object)
	if !ok {
		return nil, fs.ErrorNotAFile
	}
	return o, nil
}

// Precision of the ModTimes in this Fs
func (f *Fs) Precision() time.Duration {
	return time.Second
}

// Mkdir makes the directory (container, bucket)
//
// Shouldn't return an error if it already exists
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	// FIXME should this create a "directory" entry if not already created?
	return nil
}

// Rmdir removes the directory (container, bucket) if empty
//
// Return an error if it doesn't exist or isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return errors.New("can't remove directories from zip file")
}

// Wrapper for src to make it into os.FileInfo
type fileInfo struct {
	src fs.ObjectInfo
}

// Name - base name of the file (actually full name for zip)
func (fi fileInfo) Name() string {
	return fi.src.Remote()
}

// Size - length in bytes for regular files; system-dependent for others
func (fi fileInfo) Size() int64 {
	return fi.src.Size()
}

// Mode - file mode bits
func (fi fileInfo) Mode() os.FileMode {
	return 0777
}

// ModTime - modification time
func (fi fileInfo) ModTime() time.Time {
	return fi.src.ModTime(context.Background())
}

// IsDir - abbreviation for Mode().IsDir()
func (fi fileInfo) IsDir() bool {
	return false
}

// Sys - underlying data source (can return nil)
func (fi fileInfo) Sys() interface{} {
	return nil
}

// check type
var _ os.FileInfo = fileInfo{}

// Put in to the remote path with the modTime given of the given size
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (o fs.Object, err error) {
	defer log.Trace(f, "src=%v", src)("obj=%v, err=%v", &o, &err)
	f.wrmu.Lock()
	defer f.wrmu.Unlock()

	size := src.Size()
	if size < 0 {
		return nil, errors.New("can't zip unknown sized objects")
	}

	if f.zw == nil {
		wh, err := f.vfs.OpenFile(f.leaf, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0777)
		if err != nil {
			return nil, fmt.Errorf("zip put: failed to open output file: %w", err)
		}
		f.wh = wh
		f.zw = zip.NewWriter(f.wh)
		if err != nil {
			return nil, fmt.Errorf("zip put: failed to create zip writer: %w", err)
		}

		// FIXME enormous bodge to work around no lifecycle for backends
		atexit.Register(func() {
			err = f.zw.Close()
			if err != nil {
				fs.Errorf(f, "Error closing zip writer: %v", err)
			}
			err = f.wh.Close()
			if err != nil {
				fs.Errorf(f, "Error closing zip file: %v", err)
			}
		})
	}

	// FIXME not putting in directories?

	fh, err := zip.FileInfoHeader(fileInfo{src})
	if err != nil {
		return nil, fmt.Errorf("zip put: failed to create file header: %w", err)
	}
	w, err := f.zw.CreateHeader(fh)
	if err != nil {
		return nil, fmt.Errorf("zip put: failed to open file: %w", err)
	}
	// FIXME just a guess!
	// should probably try a better heuristic?
	if size < 10 {
		fh.Method = zip.Store
	} else {
		fh.Method = zip.Deflate
	}
	_, err = io.CopyN(w, in, size)
	if err != nil {
		return nil, fmt.Errorf("zip put: failed to copy file: %w", err)
	}

	// err = w.Close()
	// if err != nil {
	// 	return nil,fmt.Errorf("zip put: failed to close file: %w", err)
	// }

	o = &Object{
		f:      f,
		fh:     fh,
		remote: src.Remote(),
	}

	return o, nil

}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.CRC32)
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

// Object describes an object to be read from the raw zip file
type Object struct {
	f      *Fs
	remote string
	fh     *zip.FileHeader
	file   *zip.File
}

// Fs returns read only access to the Fs that this object is part of
func (o *Object) Fs() fs.Info {
	return o.f
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

// Size returns the size of the file
func (o *Object) Size() int64 {
	return int64(o.fh.UncompressedSize64)
}

// ModTime returns the modification time of the object
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.fh.Modified
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	fs.Debugf(o, "Can't set mod time - ignoring")
	return nil
}

// Storable raturns a boolean indicating if this object is storable
func (o *Object) Storable() bool {
	return true
}

// Hash returns the selected checksum of the file
// If no checksum is available it returns ""
func (o *Object) Hash(ctx context.Context, ht hash.Type) (string, error) {
	if ht == hash.CRC32 {
		// FIXME return empty CRC if writing
		if o.f.dt == nil {
			return "", nil
		}
		return fmt.Sprintf("%08x", o.fh.CRC32), nil
	}
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

	rc, err = o.file.Open()
	if err != nil {
		return nil, err
	}

	// discard data from start as necessary
	if offset > 0 {
		_, err = io.CopyN(ioutil.Discard, rc, offset)
		if err != nil {
			return nil, err
		}
	}
	// If limited then don't return everything
	if limit >= 0 {
		return readers.NewLimitedReadCloser(rc, limit-offset), nil
	}

	return rc, nil
}

// Update in to the object with the modTime given of the given size
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	return errors.New("can't Update zip files")
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	return errors.New("can't Remove zip file objects")
}

// Check the interfaces are satisfied
var (
	_ fs.Fs        = (*Fs)(nil)
	_ fs.UnWrapper = (*Fs)(nil)
	_ fs.ListRer   = (*Fs)(nil)
	//	_ fs.Abouter         = (*Fs)(nil) - FIXME can implemnet
	_ fs.Wrapper = (*Fs)(nil)
	_ fs.Object  = (*Object)(nil)
)
