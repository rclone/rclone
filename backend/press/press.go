// Package press provides wrappers for Fs and Object which implement compression. Based off crypt.
package press

import (
	"fmt"
	"io"
	"strings"
	"bytes"
	"bufio"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/accounting"
	"github.com/ncw/rclone/fs/config/configmap"
	"github.com/ncw/rclone/fs/config/configstruct"
	"github.com/ncw/rclone/fs/fspath"
	"github.com/ncw/rclone/fs/hash"
	"github.com/ncw/rclone/fs/chunkedreader"
	"github.com/pkg/errors"
)

/**
NOTES:
Filenames are now <original file name>.<extension>
Size() function is really inefficient because it requires opening and seeking through the file
Files are now always compressed - the heuristic test function is defunct
**/

// Globals
// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "press",
		Description: "Compress a remote",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:     "remote",
			Help:     "Remote to compress.",
			Required: true,
		}, {
			Name:    "compression_mode",
			Help:    "Compression mode. XZ compression mode requires the xz binary to be in PATH.",
			Default: "gzip-min",
			Examples: []fs.OptionExample{
				{
					Value: "lz4",
					Help:  "Fast, real-time compression with reasonable compression ratios.",
				}, {
					Value: "snappy",
					Help:  "Google's compression algorithm. Slightly faster and larger than LZ4.",
				}, {
					Value: "gzip-min",
					Help:  "Standard gzip compression with fastest parameters.",
				}, {
					Value: "gzip-default",
					Help:  "Standard gzip compression with default parameters.",
				}, {
					Value: "xz-min",
					Help:  "Slow but powerful compression with reasonable speed.",
				}, {
					Value: "xz-default",
					Help:  "Slowest but best compression.",
				},
			},
		}},
	})
}

// Constants
const bufferSize = 8388608 // Size of buffer when compressing or decompressing the entire file.
			// Larger size means more multithreading with larger block sizes and thread counts.
			// Currently at 8MB.
const INITIAL_CHUNK_SIZE = 262144 // Initial and max sizes of chunks when reading parts of the file. Currently
const MAX_CHUNK_SIZE = 8388608    // at 256KB and 8 MB.

// dirName gets the containing directory of a remote
func dirName(remote string) (dir string) {
	index := strings.LastIndex(remote, "/")
	if index == -1 {
		return ""
	}
	return remote[:index]
}

// baseName gets the filename of a remote without directories
func baseName(remote string) (dir string) {
	index := strings.LastIndex(remote, "/")
	if index == -1 {
		return remote
	}
	return remote[index+1:]
}

// newCompressionForConfig constructs a Compression object for the given config name
func newCompressionForConfig(opt *Options) (*Compression, error) {
	c, err := NewCompressionPreset(opt.CompressionMode)
	return c, err
}

// NewFs contstructs an Fs from the path, container:path
func NewFs(name, rpath string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	c, err := newCompressionForConfig(opt)
	if err != nil {
		return nil, err
	}
	remote := opt.Remote
	if strings.HasPrefix(remote, name+":") {
		return nil, errors.New("can't point press remote at itself - check the value of the remote setting")
	}
	wInfo, wName, wPath, wConfig, err := fs.ConfigFs(remote)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse remote %q to wrap", remote)
	}
	// Strip trailing slashes if they exist in rpath
	rpath = strings.TrimRight(rpath, "/")

	// First, check for a file
	// If a file was found, create an fs for it. Otherwise, check for a directory
	remotePath := fspath.JoinRootPath(wPath, c.generateFileNameFromName(rpath))
	wrappedFs, err := wInfo.NewFs(wName, remotePath, wConfig)
	if err != fs.ErrorIsFile {
		remotePath = fspath.JoinRootPath(wPath, rpath)
		wrappedFs, err = wInfo.NewFs(wName, remotePath, wConfig)
	}
	if err != nil && err != fs.ErrorIsFile {
		return nil, errors.Wrapf(err, "failed to make remote %s:%q to wrap", wName, remotePath)
	}

	// Create the wrapping fs
	f := &Fs{
		Fs:     wrappedFs,
		name:   name,
		root:   rpath,
		opt:    *opt,
		c: c,
	}
	// the features here are ones we could support, and they are
	// ANDed with the ones from wrappedFs
	f.features = (&fs.Features{
		CaseInsensitive:         true,
		DuplicateFiles:          true,
		ReadMimeType:            false, // MimeTypes not supported with crypt
		WriteMimeType:           false,
		BucketBased:             true,
		CanHaveEmptyDirectories: true,
		SetTier:                 true,
		GetTier:                 true,
	}).Fill(f).Mask(wrappedFs).WrapsFs(f, wrappedFs)

	return f, err
}

// Processes a file name for a compressed file. Returns the original file name, the extension, and the size of the original file.
func processFileName(compressedFileName string) (origFileName string, extension string, err error) {
	// Separate the filename from the extension
	extensionPos := strings.LastIndex(compressedFileName, ".")
	if extensionPos == -1 {
		return "", "", errors.New("File name has no extension")
	}
	name := compressedFileName[:extensionPos]
	extension = compressedFileName[extensionPos:]
	// Return everything
	return name, extension, nil
}

// Generates a file name for a compressed version of an uncompressed file with a remote.
func (c *Compression) generateFileNameFromName(remote string) (newRemote string) {
	return remote + c.GetFileExtension()
}

// Generates a file name for a compressed version of an uncompressed file.
func (c *Compression) generateFileName(objectInfo fs.ObjectInfo) (remote string) {
	return c.generateFileNameFromName(objectInfo.Remote())
}

// Options defines the configuration for this backend
type Options struct {
	Remote                  string `config:"remote"`
	CompressionMode         string `config:"compression_mode"`
}

// Fs represents a wrapped fs.Fs
type Fs struct {
	fs.Fs
	wrapper  fs.Fs
	name     string
	root     string
	opt      Options
	features *fs.Features // optional features
	c   *Compression
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

// Remove the last extension of the file that is used for determining whether a file is compressed.
func (f *Fs) add(entries *fs.DirEntries, obj fs.Object) {
	remote := obj.Remote()
	_, _, err := processFileName(remote)
	if err != nil {
		fs.Debugf(remote, "Skipping unrecognized file name: %v", err)
		return
	}
	*entries = append(*entries, f.newObject(obj))
}

// Directory names are unchanged. Just append.
func (f *Fs) addDir(entries *fs.DirEntries, dir fs.Directory) {
	*entries = append(*entries, f.newDir(dir))
}

// Processes file entries by removing compression data. Don't ask me how it works. I don't know.
func (f *Fs) processEntries(entries fs.DirEntries) (newEntries fs.DirEntries, err error) {
	newEntries = entries[:0] // in place filter
	for _, entry := range entries {
		switch x := entry.(type) {
		case fs.Object:
			f.add(&newEntries, x)
		case fs.Directory:
			f.addDir(&newEntries, x)
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
// List entries and process them
func (f *Fs) List(dir string) (entries fs.DirEntries, err error) {
	entries, err = f.Fs.List(dir)
	if err != nil {
		return nil, err
	}
	return f.processEntries(entries)
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
func (f *Fs) ListR(dir string, callback fs.ListRCallback) (err error) {
	return f.Fs.Features().ListR(dir, func(entries fs.DirEntries) error {
		newEntries, err := f.processEntries(entries)
		if err != nil {
			return err
		}
		return callback(newEntries)
	})
}

// NewObject finds the Object at remote.
func (f *Fs) NewObject(remote string) (fs.Object, error) {
	// Just generate the file name and get the object
	o, err := f.Fs.NewObject(f.c.generateFileNameFromName(remote))
	if err != nil {
		return nil, err
	}
	return f.newObject(o), nil
}

type putFn func(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error)

func (f *Fs) put(in io.Reader, src fs.ObjectInfo, options []fs.OpenOption, put putFn) (fs.Object, error) {
	// Unwrap reader accounting
	in, wrap := accounting.UnWrap(in)

	// Compress the file
	var wrappedIn io.Reader
	pipeReader, pipeWriter := io.Pipe()
	compressionError := make(chan error)
	go func() {
		err := f.c.CompressFile(in, 0, pipeWriter)
		pipeWriter.Close()
		compressionError <- err
	}()
	wrappedIn = wrap(bufio.NewReaderSize(pipeReader, bufferSize)) // Bufio required for multithreading

	// Find a hash the destination supports to compute a hash of
	// the compressed data
	ht := f.Fs.Hashes().GetOne()
	var hasher *hash.MultiHasher
	if ht != hash.None {
		hasher, err := hash.NewMultiHasherTypes(hash.NewHashSet(ht))
		if err != nil {
			return nil, err
		}
		// unwrap the accounting
		var wrap accounting.WrapFn
		wrappedIn, wrap = accounting.UnWrap(wrappedIn)
		// add the hasher
		wrappedIn = io.TeeReader(wrappedIn, hasher)
		// wrap the accounting back on
		wrappedIn = wrap(wrappedIn)
	}

	// Transfer the data
	o, err := put(wrappedIn, f.newObjectInfo(src), options...)
	if err != nil {
		o.Remove()
		return nil, err
	}

	// Check whether we got an error during compression
	err = <-compressionError
	if err != nil {
		o.Remove()
		return nil, err
	}

	// Check the hashes of the compressed data if we were comparing them
	if ht != hash.None && hasher != nil {
		srcHash := hasher.Sums()[ht]
		var dstHash string
		dstHash, err = o.Hash(ht)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read destination hash")
		}
		if srcHash != "" && dstHash != "" && srcHash != dstHash {
			// remove object
			err = o.Remove()
			if err != nil {
				fs.Errorf(o, "Failed to remove corrupted object: %v", err)
			}
			return nil, errors.Errorf("corrupted on transfer: %v compressed hashes differ %q vs %q", ht, srcHash, dstHash)
		}
	}

	return f.newObject(o), nil
}

// Put in to the remote path with the modTime given of the given size
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
func (f *Fs) Put(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.put(in, src, options, f.Fs.Put)
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
func (f *Fs) PutStream(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.put(in, src, options, f.Fs.Features().PutStream)
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.None)
}

// Mkdir makes the directory (container, bucket)
//
// Shouldn't return an error if it already exists
func (f *Fs) Mkdir(dir string) error {
	return f.Fs.Mkdir(dir)
}

// Rmdir removes the directory (container, bucket) if empty
//
// Return an error if it doesn't exist or isn't empty
func (f *Fs) Rmdir(dir string) error {
	return f.Fs.Rmdir(dir)
}

// Purge all files in the root and the root directory
//
// Implement this if you have a way of deleting all the files
// quicker than just running Remove() on the result of List()
//
// Return an error if it doesn't exist
func (f *Fs) Purge() error {
	do := f.Fs.Features().Purge
	if do == nil {
		return fs.ErrorCantPurge
	}
	return do()
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
func (f *Fs) Copy(src fs.Object, remote string) (fs.Object, error) {
	do := f.Fs.Features().Copy
	if do == nil {
		return nil, fs.ErrorCantCopy
	}
	o, ok := src.(*Object)
	if !ok {
		return nil, fs.ErrorCantCopy
	}
	newFilename := f.c.generateFileNameFromName(remote)
	oResult, err := do(o.Object, newFilename)
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
func (f *Fs) Move(src fs.Object, remote string) (fs.Object, error) {
	do := f.Fs.Features().Move
	if do == nil {
		return nil, fs.ErrorCantMove
	}
	o, ok := src.(*Object)
	if !ok {
		return nil, fs.ErrorCantMove
	}
	newFilename := f.c.generateFileNameFromName(remote)
	oResult, err := do(o.Object, newFilename)
	if err != nil {
		return nil, err
	}
	return f.newObject(oResult), nil
}

// DirMove moves src, srcRemote to this remote at dstRemote
// using server side move operations.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantDirMove
//
// If destination exists then return fs.ErrorDirExists
func (f *Fs) DirMove(src fs.Fs, srcRemote, dstRemote string) error {
	do := f.Fs.Features().DirMove
	if do == nil {
		return fs.ErrorCantDirMove
	}
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debugf(srcFs, "Can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}
	return do(srcFs.Fs, srcRemote, dstRemote)
}

// PutUnchecked uploads the object
//
// This will create a duplicate if we upload a new file without
// checking to see if there is one already - use Put() for that.
func (f *Fs) PutUnchecked(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	// Check if putUnchecked is supported
	do := f.Fs.Features().PutUnchecked
	if do == nil {
		return nil, errors.New("can't PutUnchecked")
	}

	// Unwrap reader accounting and perform heuristic
	in, wrap := accounting.UnWrap(in)
	startOfFile := make([]byte, f.c.HeuristicBytes)
	in.Read(startOfFile)
	compressible, _, err := f.c.GetFileCompressionInfo(bytes.NewReader(startOfFile))

	// Create rewinded reader and compress or copy it
	in = io.MultiReader(bytes.NewReader(startOfFile), in)
	var wrappedIn io.Reader
	if compressible { // Compress the data if it's compressible
		pipeReader, pipeWriter := io.Pipe()
		err := f.c.CompressFile(in, 0, pipeWriter)
		if err != nil {
			return nil, err
		}
		wrappedIn = wrap(bufio.NewReaderSize(pipeReader, bufferSize)) // Required for multithreading
	} else { // If the data is not compressible, just copy it
		wrappedIn = wrap(in)
	}

	// Do... Stuff?
	o, err := do(wrappedIn, f.newObjectInfo(src))
	if err != nil {
		return nil, err
	}
	return f.newObject(o), nil
}

// CleanUp the trash in the Fs
//
// Implement this if you have a way of emptying the trash or
// otherwise cleaning up old versions of files.
func (f *Fs) CleanUp() error {
	do := f.Fs.Features().CleanUp
	if do == nil {
		return errors.New("can't CleanUp")
	}
	return do()
}

// About gets quota information from the Fs
func (f *Fs) About() (*fs.Usage, error) {
	do := f.Fs.Features().About
	if do == nil {
		return nil, errors.New("About not supported")
	}
	return do()
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

// MergeDirs merges the contents of all the directories passed
// in into the first one and rmdirs the other directories.
func (f *Fs) MergeDirs(dirs []fs.Directory) error {
	do := f.Fs.Features().MergeDirs
	if do == nil {
		return errors.New("MergeDirs not supported")
	}
	out := make([]fs.Directory, len(dirs))
	for i, dir := range dirs {
		out[i] = fs.NewDirCopy(dir).SetRemote(dir.Remote())
	}
	return do(out)
}

// DirCacheFlush resets the directory cache - used in testing
// as an optional interface
func (f *Fs) DirCacheFlush() {
	do := f.Fs.Features().DirCacheFlush
	if do != nil {
		do()
	}
}

// ChangeNotify calls the passed function with a path
// that has had changes. If the implementation
// uses polling, it should adhere to the given interval.
func (f *Fs) ChangeNotify(notifyFunc func(string, fs.EntryType), pollIntervalChan <-chan time.Duration) {
	do := f.Fs.Features().ChangeNotify
	if do == nil {
		return
	}
	wrappedNotifyFunc := func(path string, entryType fs.EntryType) {
		fs.Logf(f, "path %q entryType %d", path, entryType)
		var (
			wrappedPath string
		)
		switch entryType {
		case fs.EntryDirectory:
			wrappedPath = path
		case fs.EntryObject:
			wrappedPath = f.c.generateFileNameFromName(path)
		default:
			fs.Errorf(path, "crypt ChangeNotify: ignoring unknown EntryType %d", entryType)
			return
		}
		notifyFunc(wrappedPath, entryType)
	}
	do(wrappedNotifyFunc, pollIntervalChan)
}

// PublicLink generates a public link to the remote path (usually readable by anyone)
func (f *Fs) PublicLink(remote string) (string, error) {
	do := f.Fs.Features().PublicLink
	if do == nil {
		return "", errors.New("PublicLink not supported")
	}
	o, err := f.NewObject(remote)
	if err != nil {
		// assume it is a directory
		return do(remote)
	}
	return do(o.(*Object).Object.Remote())
}

// Object describes a wrapped for being read from the Fs
//
// This decompresses the remote name and decompresses the data
type Object struct {
	fs.Object
	f *Fs
}

// This will initialize the variables of a new press Object wrapping o.
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
	filename, _, err := processFileName(remote)
	if err != nil {
		fs.Debugf(o, "Error on getting remote path: %v", err)
		return remote
	}
	return filename
}

// Size returns the size of the file
func (o *Object) Size() int64 {
	in := chunkedreader.New(o.Object, INITIAL_CHUNK_SIZE, MAX_CHUNK_SIZE)
	_, decompressedSize, err := o.f.c.DecompressFile(in, o.Object.Size()) // Does not decompress the file until it is read from
	if err != nil {
		fs.Debugf(o, "Unable to determine size: %v", err)
		return -1
	}
	return decompressedSize
}

// Hash returns the selected checksum of the file
// If no checksum is available it returns ""
func (o *Object) Hash(ht hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// UnWrap returns the wrapped Object
func (o *Object) UnWrap() fs.Object {
	return o.Object
}

// Combines a Reader and a Closer to a ReadCloser
type ReadCloserWrapper struct {
	dataSource io.Reader
	closer io.Closer
}
func combineReaderAndCloser(dataSource io.Reader, closer io.Closer) *ReadCloserWrapper {
	rc := new(ReadCloserWrapper)
	rc.dataSource = dataSource
	rc.closer = closer
	return rc
}
func (w *ReadCloserWrapper) Read(p []byte) (n int, err error) {
	return w.dataSource.Read(p)
}
func (w *ReadCloserWrapper) Close() error {
	return w.closer.Close()
}

// Open opens the file for read.  Call Close() on the returned io.ReadCloser. Note that this call requires quite a bit of overhead.
func (o *Object) Open(options ...fs.OpenOption) (rc io.ReadCloser, err error) {
	// Get offset and limit from OpenOptions, pass the rest to the underlying remote
	var openOptions []fs.OpenOption = []fs.OpenOption{&fs.SeekOption{Offset: 0}}
	var offset, limit int64 = 0, -1
	for _, option := range options {
		switch x := option.(type) {
			case *fs.SeekOption:
				offset = x.Offset
			case *fs.RangeOption:
				offset, limit = x.Decode(o.Size())
			default:
				openOptions = append(openOptions, option)
		}
	}
	// Get a chunkedreader for the wrapped object
	chunkedReader := chunkedreader.New(o.Object, INITIAL_CHUNK_SIZE, MAX_CHUNK_SIZE)
	// Get file handle
	FileHandle, _, err := o.f.c.DecompressFile(chunkedReader, o.Object.Size())
	if err != nil {
		return nil, err
	}
	// Seek and limit according to the options given
	FileHandle.Seek(offset, io.SeekStart)
	var fileReader io.Reader;
	if limit != -1 {
		fileReader = io.LimitReader(FileHandle, limit)
	} else {
		fileReader = FileHandle
	}
	// Return a ReadCloser
	return combineReaderAndCloser(fileReader, chunkedReader), nil
}

// Update in to the object with the modTime given of the given size
func (o *Object) Update(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	update := func(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
		return o.Object, o.Object.Update(in, src, options...)
	}
	_, err := o.f.put(in, src, options, update)
	return err
}

// newDir returns a dir
func (f *Fs) newDir(dir fs.Directory) fs.Directory {
	return dir // We're using the same dir
}

// ObjectInfo describes a wrapped fs.ObjectInfo for being the source
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
	return o.f.c.generateFileName(o.ObjectInfo)
}

// Size returns the size of the file
func (o *ObjectInfo) Size() int64 {
	obj, err := o.f.NewObject(o.Remote())
	if err != nil {
		fs.Debugf(o, "Cannot find file: %v", err)
		return -1
	}
	return obj.Size()
}

// Hash returns the selected checksum of the file
// If no checksum is available it returns ""
func (o *ObjectInfo) Hash(hash hash.Type) (string, error) {
	return "", nil
}

// ID returns the ID of the Object if known, or "" if not
func (o *Object) ID() string {
	do, ok := o.Object.(fs.IDer)
	if !ok {
		return ""
	}
	return do.ID()
}

// SetTier performs changing storage tier of the Object if
// multiple storage classes supported
func (o *Object) SetTier(tier string) error {
	do, ok := o.Object.(fs.SetTierer)
	if !ok {
		return errors.New("crypt: underlying remote does not support SetTier")
	}
	return do.SetTier(tier)
}

// GetTier returns storage tier or class of the Object
func (o *Object) GetTier() string {
	do, ok := o.Object.(fs.GetTierer)
	if !ok {
		return ""
	}
	return do.GetTier()
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = (*Fs)(nil)
	_ fs.Purger          = (*Fs)(nil)
	_ fs.Copier          = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ fs.PutUncheckeder  = (*Fs)(nil)
	_ fs.PutStreamer     = (*Fs)(nil)
	_ fs.CleanUpper      = (*Fs)(nil)
	_ fs.UnWrapper       = (*Fs)(nil)
	_ fs.ListRer         = (*Fs)(nil)
	_ fs.Abouter         = (*Fs)(nil)
	_ fs.Wrapper         = (*Fs)(nil)
	_ fs.MergeDirser     = (*Fs)(nil)
	_ fs.DirCacheFlusher = (*Fs)(nil)
	_ fs.ChangeNotifier  = (*Fs)(nil)
	_ fs.PublicLinker    = (*Fs)(nil)
	_ fs.ObjectInfo      = (*ObjectInfo)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.ObjectUnWrapper = (*Object)(nil)
	_ fs.IDer            = (*Object)(nil)
	_ fs.SetTierer       = (*Object)(nil)
	_ fs.GetTierer       = (*Object)(nil)
)