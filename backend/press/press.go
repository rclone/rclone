// Package press provides wrappers for Fs and Object which implement compression. Based off crypt.
package press

import (
	"fmt"
	"io"
	"strings"
	"bytes"
	"bufio"
	"reflect"
	"strconv"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/accounting"
	"github.com/ncw/rclone/fs/config/configmap"
	"github.com/ncw/rclone/fs/config/configstruct"
	"github.com/ncw/rclone/fs/fspath"
	"github.com/ncw/rclone/fs/hash"
	"github.com/pkg/errors"
)

/**
TODO:
- Buffering in the compression.go file causes problems. This needs investigation but isn't curcial.

NOTES:
Filenames are now <original file name><16 hex chars containing original size of file>.<extension>
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
			Help:     "Remote to compress. Should be a seekable remote like crypt.",
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
					Help:  "Google's compression algorithm. Slightly slower and larger than LZ4.",
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
	// Open the containing directory first
	containingRemotePath := fspath.JoinRootPath(wPath, dirName(rpath))
	containingFs, err := wInfo.NewFs(wName, containingRemotePath, wConfig)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to make remote %s:%q to check for possible files", wName, containingRemotePath)
	}
	// Looks for a valid file in the listing of the containing directory
	listing, err := containingFs.List("")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list %s:%q to find possible files", wName, containingRemotePath)
	}
	wrappedFilename := findFileInEntriesMatching(listing, baseName(rpath))
	// If a file was found, create an fs for it. Otherwise, check for a directory
	var wrappedFs fs.Fs
	var remotePath string
	if wrappedFilename != "" {
		remotePath = fspath.JoinRootPath(containingRemotePath, wrappedFilename)
		wrappedFs, err = wInfo.NewFs(wName, remotePath, wConfig)
	} else {
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
	}).Fill(f).Mask(wrappedFs).WrapsFs(f, wrappedFs)

	return f, err
}

// Processes a file name for a compressed file. Returns the original file name, the extension, and the size of the original file.
func processFileName(compressedFileName string) (origFileName string, extension string, size int64, err error) {
	// First, separate the filename from the extension
	extensionPos := strings.LastIndex(compressedFileName, ".")
	if extensionPos == -1 {
		return "", "", 0, errors.New("File name has no extension")
	}
	name := compressedFileName[:extensionPos]
	extension = compressedFileName[extensionPos:]
	// Get the last 16 chars of the non-extension file name (which are the hexadecimal encoded size)
	size, err = strconv.ParseInt(name[len(name)-16:], 16, 64)
	if err != nil {
		return "", "", 0, errors.New("File name does not contain size")
	}
	// Remove the size from the name
	name = name[:len(name)-16]
	// Return everything
	return name, extension, size, nil
}

// Generates a file name for a compressed version of an uncompressed file with a remote and size.
func (c *Compression) generateFileNameFromNameAndSize(remote string, size int64) (newRemote string) {
	return remote + fmt.Sprintf("%016x", size) + c.GetFileExtension()
}

// Generates a file name for a compressed version of an uncompressed file.
func (c *Compression) generateFileName(objectInfo fs.ObjectInfo) (remote string) {
	return c.generateFileNameFromNameAndSize(objectInfo.Remote(), objectInfo.Size())
}

// Options defines the configuration for this backend
type Options struct {
	Remote                  string `config:"remote"`
	CompressionMode         string `config:"compression_mode"`
}

// Fs represents a wrapped fs.Fs
type Fs struct {
	fs.Fs
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
	_, _, _, err := processFileName(remote)
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
/*
func (f *Fs) List(dir string) (entries fs.DirEntries, err error) {
	entries, err = f.Fs.List(f.cipher.EncryptDirName(dir))
	if err != nil {
		return nil, err
	}
	return f.encryptEntries(entries)
}*/

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

// findFileInEntriesMatching finds a file, which, when compression data stored in it is stripped, matches a file in a list of DirEntries. Returns empty string on not found.
func findFileInEntriesMatching(dirEntries fs.DirEntries, name string) (foundRemote string) {
	dirEntries.ForObject(func(dirEntry fs.Object){
		// First make sure this isn't a directory
		directory := reflect.TypeOf((*fs.Directory)(nil)).Elem()
		if !reflect.TypeOf(dirEntry).Implements(directory) {
			// Then test whether our processed file name matches
			currRemote := dirEntry.Remote()
			origFileName, _, _, _ := processFileName(currRemote)
			if name == origFileName {
				foundRemote = currRemote
			}
		}
	})
	return foundRemote
}

// NewObject finds the Object at remote.
func (f *Fs) NewObject(remote string) (fs.Object, error) {
	// List the directory where the objects are, then look through the list for a remote that starts with our remote.
	dirNameRemote := dirName(remote)
	dirEntries, err := f.Fs.List(dirNameRemote)
	if err == fs.ErrorDirNotFound {
		return nil, fs.ErrorObjectNotFound
	}
	if err != nil {
		return nil, err
	}
	foundRemote := findFileInEntriesMatching(dirEntries, remote)
	// If we couldn't find an object starting with our remote, return an error
	if foundRemote == "" {
		return nil, fs.ErrorObjectNotFound
	}

	// Otherwise, we can get that object
	o, err := f.Fs.NewObject(foundRemote)
	if err != nil {
		fmt.Printf("%v\n", err)
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
	go func() {
		f.c.CompressFile(in, 0, pipeWriter)
		pipeWriter.Close()
	}()
	wrappedIn = wrap(bufio.NewReaderSize(pipeReader, bufferSize)) // Bufio required for multithreading

	// Find a hash the destination supports to compute a hash of
	// the encrypted data
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
		return nil, err
	}

	// Check the hashes of the encrypted data if we were comparing them
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
			return nil, errors.Errorf("corrupted on transfer: %v crypted hash differ %q vs %q", ht, srcHash, dstHash)
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
	newFilename := f.c.generateFileNameFromNameAndSize(remote, o.Size())
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
	newFilename := f.c.generateFileNameFromNameAndSize(remote, o.Size())
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

// Object describes a wrapped for being read from the Fs
//
// This decrypts the remote name and decrypts the data
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
	filename, _, _, err := processFileName(remote)
	if err != nil {
		fs.Debugf(o, "Error on getting remote path: %v", err)
		return remote
	}
	return filename
}

// Size returns the size of the file
func (o *Object) Size() int64 {
	// Same as getting size from an ObjectInfo (note: seems glitched for now. Needs some debug)
//	remote := o.Object.Remote()
//	_, _, size, err := processFileName(remote)
//	if err != nil {
//		fs.Debugf(o, "Error processing file name: %v", err)
//	}
//	return size

	in, err := o.Object.Open(&fs.SeekOption{Offset: 0})
	inSeek, ok := in.(io.ReadSeeker)
	if !ok {
		fs.Debugf(o, "Unable to get ReadSeeker to determine size")
		panic("Cannot get ReadSeeker to determine size")
		return -1
	}
	_, decompressedSize, err := o.f.c.DecompressFile(inSeek, o.Object.Size()) // Does not decompress the file until it is read from
	if err != nil {
		fs.Debugf(o, "Bad size for decompression: %v", err)
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

// ReadSeekCloser interface
type ReadSeekCloser interface {
	io.Reader
	io.Seeker
	io.Closer
	fs.RangeSeeker
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
	// Get the readCloser handle which starts at the start of the file. Note that this will actually return a readSeekCloser if we're running over crypt.
	readCloser, err := o.Object.Open(openOptions...)
	if err != nil {
		return nil, err
	}
	// Use reflection to get a readSeekCloser if possible
	readSeekCloser, ok := readCloser.(ReadSeekCloser)
	if !ok {
		return nil, errors.New("Wrapped remote does not support seeking")
	}
	// Get file handle
	FileHandle, _, err := o.f.c.DecompressFile(readSeekCloser, o.Object.Size())
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
	return combineReaderAndCloser(fileReader, readCloser), nil
}

// Using magic update function from crypt
// Update in to the object with the modTime given of the given size
func (o *Object) Update(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	update := func(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
		return o.Object, o.Object.Update(in, src, options...)
	}
	_, err := o.f.put(in, src, options, update)
	return err
}

// newDir returns a dir with the Name decrypted
func (f *Fs) newDir(dir fs.Directory) fs.Directory {
	return dir // We're using the same dir
}

// ObjectInfo describes a wrapped fs.ObjectInfo for being the source
//
// This encrypts the remote name and adjusts the size
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
	remote := o.Remote()
	_, _, size, err := processFileName(remote)
	if err != nil {
		fs.Debugf(o, "Error processing file name: %v", err)
	}
	return size
}

// Hash returns the selected checksum of the file
// If no checksum is available it returns ""
func (o *ObjectInfo) Hash(hash hash.Type) (string, error) {
	return "", nil
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
	_ fs.ObjectInfo      = (*ObjectInfo)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.ObjectUnWrapper = (*Object)(nil)
)
