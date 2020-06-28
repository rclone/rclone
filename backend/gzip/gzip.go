// Package gzip provides wrappers for Fs and Object which implement compression
package gzip

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/readers"
)

// Globals
// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "gzip",
		Description: "Compress/Decompress a remote",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:     "remote",
			Help:     "Remote to compress/decompress.\nNormally should contain a ':' and a path, eg \"myremote:path/to/dir\",\n\"myremote:bucket\" or maybe \"myremote:\" (not recommended).",
			Required: true,
		}},
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
	// remote := opt.Remote
	// if strings.HasPrefix(remote, name+":") {
	// 	return nil, errors.New("can't point gzip remote at itself - check the value of the remote setting")
	// }
	// wInfo, wName, wPath, wConfig, err := fs.ConfigFs(remote)
	// if err != nil {
	// 	return nil, errors.Wrapf(err, "failed to parse remote %q to wrap", remote)
	// }
	// Make sure to remove trailing . referring to the current dir
	// if path.Base(rpath) == "." {
	// 	rpath = strings.TrimSuffix(rpath, ".")
	// }
	// Look for a file first
	// remotePath := fspath.JoinRootPath(wPath, rpath)
	// wrappedFs, err := wInfo.NewFs(wName, remotePath, wConfig)
	// if that didn't produce a file, look for a directory
	// if err != fs.ErrorIsFile {
	// 	remotePath = fspath.JoinRootPath(wPath, rpath)
	// 	wrappedFs, err = wInfo.NewFs(wName, remotePath, wConfig)
	// }
	// if err != fs.ErrorIsFile && err != nil {
	// 	return nil, errors.Wrapf(err, "failed to make remote %s:%q to wrap", wName, remotePath)
	// }

	// Create the remote from the cache
	wrappedFs, err := cache.Get(ctx, root)
	if err != fs.ErrorIsFile && err != nil {
		return nil, errors.Wrapf(err, "failed to make %q to wrap", root)
	}
	f := &Fs{
		Fs:   wrappedFs,
		name: name,
		root: root,
		opt:  *opt,
	}
	// the features here are ones we could support, and they are
	// ANDed with the ones from wrappedFs
	f.features = (&fs.Features{
		CaseInsensitive:         true,
		DuplicateFiles:          true,
		ReadMimeType:            false, // MimeTypes not supported with gzip
		WriteMimeType:           false,
		BucketBased:             true,
		CanHaveEmptyDirectories: true,
		SetTier:                 true,
		GetTier:                 true,
	}).Fill(ctx, f).Mask(ctx, wrappedFs).WrapsFs(f, wrappedFs)

	return f, err
}

// Options defines the configuration for this backend
type Options struct {
	Remote string `config:"remote"`
}

// Fs represents a wrapped fs.Fs
type Fs struct {
	fs.Fs
	wrapper  fs.Fs
	name     string
	root     string
	opt      Options
	features *fs.Features // optional features
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
	return fmt.Sprintf("Compressed drive '%s:%s'", f.name, f.root)
}

// decompress some directory entries.  This alters entries returning
// it as newEntries.
func (f *Fs) decompressEntries(ctx context.Context, entries fs.DirEntries) (newEntries fs.DirEntries, err error) {
	newEntries = entries[:0] // in place filter
	for _, entry := range entries {
		switch x := entry.(type) {
		case fs.Object:
			// FIXME decide if decompressing here or not
			if strings.HasSuffix(x.Remote(), ".gz") {
				newEntries = append(newEntries, f.newObject(x))
			} else {
				newEntries = append(newEntries, entry)
			}
		case fs.Directory:
			newEntries = append(newEntries, entry)
		default:
			return nil, errors.Errorf("Unknown object type %T", entry)
		}
	}
	return newEntries, nil
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
	entries, err = f.Fs.List(ctx, dir)
	if err != nil {
		return nil, err
	}
	return f.decompressEntries(ctx, entries)
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
	return f.Fs.Features().ListR(ctx, dir, func(entries fs.DirEntries) error {
		newEntries, err := f.decompressEntries(ctx, entries)
		if err != nil {
			return err
		}
		return callback(newEntries)
	})
}

// returns the underlying name of in
func (f *Fs) compressFileName(in string) string {
	return in + ".gz"
}

// returns the compressed name of in
func (f *Fs) decompressFileName(in string) string {
	if strings.HasSuffix(in, ".gz") {
		in = in[:len(in)-3]
	}
	return in
}

// A small wrapper to make sure we close the gzipper and the
// underlying handle
type decompress struct {
	io.ReadCloser
	underlying io.Closer
}

// Close the decompress object
func (d *decompress) Close() error {
	err := d.ReadCloser.Close()
	err2 := d.underlying.Close()
	if err == nil {
		err = err2
	}
	return err
}

// Wrap in in a decompress object
func (f *Fs) decompressData(in io.ReadCloser) (io.ReadCloser, error) {
	rc, err := gzip.NewReader(in)
	if err != nil {
		return nil, err
	}
	return &decompress{ReadCloser: rc, underlying: in}, nil
}

type compress struct {
	gzipWriter *gzip.Writer
	pipeReader *io.PipeReader
	pipeWriter *io.PipeWriter
	errChan    chan error
}

func (c *compress) Read(p []byte) (n int, err error) {
	// fs.Debugf(nil, "Read(%d)", len(p))
	n, err = readers.ReadFill(c.pipeReader, p)
	if err != nil && err != io.EOF {
		_ = c.pipeWriter.CloseWithError(err)
	}
	// fs.Debugf(nil, "= %d, %v", n, err)
	return n, err
}

func (c *compress) Close() (err error) {
	// fs.Debugf(nil, "Close()")
	_ = c.pipeReader.Close()
	err = <-c.errChan
	// fs.Debugf(nil, "= %v", err)
	return err
}

func (f *Fs) compressData(in io.Reader) (rc io.ReadCloser, err error) {
	c := &compress{
		errChan: make(chan error, 1),
	}
	c.pipeReader, c.pipeWriter = io.Pipe()
	c.gzipWriter, err = gzip.NewWriterLevel(c.pipeWriter, 9) // FIXME
	if err != nil {
		return nil, err
	}
	go func() {
		_, err := io.Copy(c.gzipWriter, in)
		err2 := c.gzipWriter.Close()
		if err == nil {
			err = err2
		}
		_ = c.pipeWriter.CloseWithError(err)
		c.errChan <- err
	}()
	return c, nil
}

// NewObject finds the Object at remote.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	o, err := f.Fs.NewObject(ctx, f.compressFileName(remote))
	if err != nil {
		return nil, err
	}
	return f.newObject(o), nil
}

type putFn func(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error)

// put implements Put or PutStream
func (f *Fs) put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options []fs.OpenOption, put putFn) (fs.Object, error) {
	// Compress the data into compressor
	compressor, err := f.compressData(in)
	if err != nil {
		return nil, err
	}

	// Transfer the data
	o, err := put(ctx, compressor, f.newObjectInfo(src), options...)
	if err != nil {
		return nil, err
	}

	err = compressor.Close()
	if err != nil {
		return nil, err
	}

	return f.newObject(o), nil
}

// Put in to the remote path with the modTime given of the given size
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.put(ctx, in, src, options, f.Fs.Put)
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.put(ctx, in, src, options, f.Fs.Features().PutStream)
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.None)
}

// Copy src to this remote using server side copy operations.
//
// This is stored with the remote path given
//
// It returns the destination Object and a possible error
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantCopy
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	do := f.Fs.Features().Copy
	if do == nil {
		return nil, fs.ErrorCantCopy
	}
	o, ok := src.(*Object)
	if !ok {
		return nil, fs.ErrorCantCopy
	}
	oResult, err := do(ctx, o.Object, f.compressFileName(remote))
	if err != nil {
		return nil, err
	}
	return f.newObject(oResult), nil
}

// Move src to this remote using server side move operations.
//
// This is stored with the remote path given
//
// It returns the destination Object and a possible error
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantMove
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	do := f.Fs.Features().Move
	if do == nil {
		return nil, fs.ErrorCantMove
	}
	o, ok := src.(*Object)
	if !ok {
		return nil, fs.ErrorCantMove
	}
	oResult, err := do(ctx, o.Object, f.compressFileName(remote))
	if err != nil {
		return nil, err
	}
	return f.newObject(oResult), nil
}

// PutUnchecked uploads the object
//
// This will create a duplicate if we upload a new file without
// checking to see if there is one already - use Put() for that.
func (f *Fs) PutUnchecked(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	do := f.Fs.Features().PutUnchecked
	if do == nil {
		return nil, errors.New("can't PutUnchecked")
	}
	wrappedIn, err := f.compressData(in)
	if err != nil {
		return nil, err
	}
	o, err := do(ctx, wrappedIn, f.newObjectInfo(src))
	if err != nil {
		return nil, err
	}
	return f.newObject(o), nil
}

// UnWrap returns the Fs that this Fs is wrapping
func (f *Fs) UnWrap() fs.Fs {
	return f.Fs
}

// WrapFs returns the Fs that is wrapping this Fs
func (f *Fs) WrapFs() fs.Fs {
	return f.wrapper
}

// SetWrapper sets the Fs that is wrapping this Fs
func (f *Fs) SetWrapper(wrapper fs.Fs) {
	f.wrapper = wrapper
}

// Object describes a wrapped for being read from the Fs
//
// This decompresss the remote name and decompresss the data
type Object struct {
	fs.Object
	f *Fs
}

func (f *Fs) newObject(o fs.Object) *Object {
	return &Object{
		Object: o,
		f:      f,
	}
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
	remote := o.Object.Remote()
	return o.f.decompressFileName(remote)
}

// Size returns the size of the file
func (o *Object) Size() int64 {
	return -1
}

// Hash returns the selected checksum of the file
// If no checksum is available it returns ""
func (o *Object) Hash(ctx context.Context, ht hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// UnWrap returns the wrapped Object
func (o *Object) UnWrap() fs.Object {
	return o.Object
}

// Open opens the file for read.  Call Close() on the returned io.ReadCloser
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (rc io.ReadCloser, err error) {
	var openOptions []fs.OpenOption
	var offset, limit int64 = 0, -1
	for _, option := range options {
		switch x := option.(type) {
		case *fs.SeekOption:
			offset = x.Offset
		case *fs.RangeOption:
			// FIXME o.Size() is wrong!
			offset, limit = x.Decode(o.Size())
		default:
			// pass on Options to underlying open if appropriate
			openOptions = append(openOptions, option)
		}
	}
	rc, err = o.Object.Open(ctx, openOptions...)
	if err != nil {
		return nil, err
	}
	rc, err = o.f.decompressData(rc)
	if err != nil {
		return nil, err
	}
	// if offset set then discard some data
	if offset > 0 {
		_, err := io.CopyN(ioutil.Discard, rc, offset)
		if err != nil {
			return nil, err
		}
	}
	// if limit set then apply limiter
	if limit >= 0 {
		rc = readers.NewLimitedReadCloser(rc, limit)
	}
	return rc, nil
}

// Update in to the object with the modTime given of the given size
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	update := func(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
		return o.Object, o.Object.Update(ctx, in, src, options...)
	}
	_, err := o.f.put(ctx, in, src, options, update)
	return err
}

// ObjectInfo describes a wrapped fs.ObjectInfo for being the source
//
// This compresss the remote name and adjusts the size
type ObjectInfo struct {
	fs.ObjectInfo
	f *Fs
}

func (f *Fs) newObjectInfo(src fs.ObjectInfo) *ObjectInfo {
	return &ObjectInfo{
		ObjectInfo: src,
		f:          f,
	}
}

// Fs returns read only access to the Fs that this object is part of
func (o *ObjectInfo) Fs() fs.Info {
	return o.f
}

// Remote returns the remote path
func (o *ObjectInfo) Remote() string {
	return o.f.compressFileName(o.ObjectInfo.Remote())
}

// Size returns the size of the file
func (o *ObjectInfo) Size() int64 {
	return -1
}

// Hash returns the selected checksum of the file
// If no checksum is available it returns ""
func (o *ObjectInfo) Hash(ctx context.Context, hash hash.Type) (string, error) {
	return "", nil
}

// Check the interfaces are satisfied
var (
	_ fs.Fs = (*Fs)(nil)
	//	_ fs.Purger          = (*Fs)(nil)
	_ fs.Copier = (*Fs)(nil)
	_ fs.Mover  = (*Fs)(nil)
	//	_ fs.DirMover        = (*Fs)(nil)
	//	_ fs.Commander       = (*Fs)(nil)
	_ fs.PutUncheckeder = (*Fs)(nil)
	_ fs.PutStreamer    = (*Fs)(nil)
	//	_ fs.CleanUpper      = (*Fs)(nil)
	_ fs.UnWrapper = (*Fs)(nil)
	_ fs.ListRer   = (*Fs)(nil)
	//	_ fs.Abouter         = (*Fs)(nil)
	_ fs.Wrapper = (*Fs)(nil)
	//	_ fs.MergeDirser     = (*Fs)(nil)
	//	_ fs.DirCacheFlusher = (*Fs)(nil)
	//	_ fs.ChangeNotifier  = (*Fs)(nil)
	//	_ fs.PublicLinker    = (*Fs)(nil)
	//	_ fs.UserInfoer      = (*Fs)(nil)
	//	_ fs.Disconnecter    = (*Fs)(nil)
	_ fs.ObjectInfo      = (*ObjectInfo)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.ObjectUnWrapper = (*Object)(nil)

//	_ fs.IDer            = (*Object)(nil)
//	_ fs.SetTierer       = (*Object)(nil)
//	_ fs.GetTierer       = (*Object)(nil)
)
