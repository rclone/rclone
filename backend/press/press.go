// Package press provides wrappers for Fs and Object which implement compression.
package press

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/md5"
	"encoding/binary"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"time"

	"github.com/gabriel-vasile/mimetype"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/chunkedreader"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fspath"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/operations"
	// Used for Rcat
)

// Globals
// Register with Fs
func init() {
	// Build compression mode options. Show XZ options only if they're supported on the current system.
	compressionModeOptions := []fs.OptionExample{{ // Default compression mode options
		Value: "lz4",
		Help:  "Fast, real-time compression with reasonable compression ratios.",
	}, {
		Value: "gzip",
		Help:  "Standard gzip compression with fastest parameters.",
	}, {
		Value: "xz",
		Help:  "Standard xz compression with fastest parameters.",
	},
	}

	// Register our remote
	fs.Register(&fs.RegInfo{
		Name:        "press",
		Description: "Compress a remote",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:     "remote",
			Help:     "Remote to compress.",
			Required: true,
		}, {
			Name:     "compression_mode",
			Help:     "Compression mode.",
			Default:  "gzip",
			Examples: compressionModeOptions,
		}},
	})
}

// Constants
const bufferSize = 8388608 // Size of buffer when compressing or decompressing the entire file.
// Larger size means more multithreading with larger block sizes and thread counts.
// Currently at 8MB.
const initialChunkSize = 262144 // Initial and max sizes of chunks when reading parts of the file. Currently
const maxChunkSize = 8388608    // at 256KB and 8 MB.

const metaFileExt = ".meta"
const uncompressedFileExt = ".bin"

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

	remote := opt.Remote
	if strings.HasPrefix(remote, name+":") {
		return nil, errors.New("can't point press remote at itself - check the value of the remote setting")
	}

	wInfo, wName, wPath, wConfig, err := fs.ConfigFs(remote)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse remote %q to wrap", remote)
	}

	c, err := newCompressionForConfig(opt)
	if err != nil {
		return nil, err
	}
	// Strip trailing slashes if they exist in rpath
	rpath = strings.TrimRight(rpath, "\\/")

	// First, check for a file
	// If a metadata file was found, return an error. Otherwise, check for a directory
	remotePath := fspath.JoinRootPath(wPath, generateMetadataName(rpath))
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
		Fs:   wrappedFs,
		name: name,
		root: rpath,
		opt:  *opt,
		c:    c,
	}
	// the features here are ones we could support, and they are
	// ANDed with the ones from wrappedFs
	f.features = (&fs.Features{
		CaseInsensitive:         true,
		DuplicateFiles:          true,
		ReadMimeType:            false,
		WriteMimeType:           false,
		GetTier:                 true,
		SetTier:                 true,
		BucketBased:             true,
		CanHaveEmptyDirectories: true,
	}).Fill(f).Mask(wrappedFs).WrapsFs(f, wrappedFs)
	// We support reading MIME types no matter the wrapped fs
	f.features.ReadMimeType = true
	if wrappedFs.Features().Move == nil && wrappedFs.Features().Copy == nil {
		f.features.PutStream = nil
	}

	return f, err
}

// Converts an int64 to hex
func int64ToHex(number int64) string {
	intBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(intBytes, uint64(number))
	return hex.EncodeToString(intBytes)
}

// Converts hex to int64
func hexToInt64(hexNumber string) (int64, error) {
	intBytes, err := hex.DecodeString(hexNumber)
	if err != nil {
		return 0, err
	}
	return int64(binary.LittleEndian.Uint64(intBytes)), nil
}

// Processes a file name for a compressed file. Returns the original file name, the extension, and the size of the original file.
// Returns -2 for the original size if the file is uncompressed.
func processFileName(compressedFileName string) (origFileName string, extension string, origSize int64, err error) {
	// Separate the filename and size from the extension
	extensionPos := strings.LastIndex(compressedFileName, ".")
	if extensionPos == -1 {
		return "", "", 0, errors.New("File name has no extension")
	}
	nameWithSize := compressedFileName[:extensionPos]
	extension = compressedFileName[extensionPos:]
	// Separate the name with the size if this is a compressed file. Otherwise, just return nameWithSize (because it has no size appended)
	var name string
	var size int64
	if extension == uncompressedFileExt {
		name = nameWithSize
		size = -2
	} else {
		sizeLoc := len(nameWithSize) - 16
		name = nameWithSize[:sizeLoc]
		size, err = hexToInt64(nameWithSize[sizeLoc:])
		if err != nil {
			return "", "", 0, errors.New("Could not decode size")
		}
	}
	// Return everything
	return name, extension, size, nil
}

// Generates the file name for a metadata file
func generateMetadataName(remote string) (newRemote string) {
	return remote + metaFileExt
}

// Checks whether a file is a metadata file
func isMetadataFile(filename string) bool {
	return strings.HasSuffix(filename, metaFileExt)
}

// Generates the file name for a data file
func (c *Compression) generateDataName(remote string, size int64, compressed bool) (newRemote string) {
	if compressed {
		newRemote = remote + int64ToHex(size) + c.GetFileExtension()
	} else {
		newRemote = remote + uncompressedFileExt
	}
	return newRemote
}

// Generates the file name from a compression mode
func generateDataNameFromCompressionMode(remote string, size int64, mode int) (newRemote string) {
	if mode != Uncompressed {
		c, _ := NewCompressionPresetNumber(mode)
		newRemote = c.generateDataName(remote, size, true)
	} else {
		newRemote = remote + uncompressedFileExt
	}
	return newRemote
}

// Options defines the configuration for this backend
type Options struct {
	Remote          string `config:"remote"`
	CompressionMode string `config:"compression_mode"`
}

/*** FILESYSTEM FUNCTIONS ***/

// Fs represents a wrapped fs.Fs
type Fs struct {
	fs.Fs
	wrapper  fs.Fs
	name     string
	root     string
	opt      Options
	features *fs.Features // optional features
	c        *Compression
}

// Get an Object from a data DirEntry
func (f *Fs) addData(entries *fs.DirEntries, o fs.Object) {
	origFileName, _, size, err := processFileName(o.Remote())
	if err != nil {
		fs.Errorf(o, "Error on parsing file name: %v", err)
		return
	}
	if size == -2 { // File is uncompressed
		size = o.Size()
	}
	metaName := generateMetadataName(origFileName)
	*entries = append(*entries, f.newObjectSizeAndNameOnly(o, metaName, size))
}

// Directory names are unchanged. Just append.
func (f *Fs) addDir(entries *fs.DirEntries, dir fs.Directory) {
	*entries = append(*entries, f.newDir(dir))
}

// newDir returns a dir
func (f *Fs) newDir(dir fs.Directory) fs.Directory {
	return dir // We're using the same dir
}

// Processes file entries by removing compression data.
func (f *Fs) processEntries(entries fs.DirEntries) (newEntries fs.DirEntries, err error) {
	newEntries = entries[:0] // in place filter
	for _, entry := range entries {
		switch x := entry.(type) {
		case fs.Object:
			//			if isMetadataFile(x.Remote()) {
			//				f.addMeta(&newEntries, x) // Only care about metadata files; non-metadata files are redundant.
			//			}
			if !isMetadataFile(x.Remote()) {
				f.addData(&newEntries, x) // Only care about data files for now; metadata files are redundant.
			}
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
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	entries, err = f.Fs.List(ctx, dir)
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
func (f *Fs) ListR(ctx context.Context, dir string, callback fs.ListRCallback) (err error) {
	return f.Fs.Features().ListR(ctx, dir, func(entries fs.DirEntries) error {
		newEntries, err := f.processEntries(entries)
		if err != nil {
			return err
		}
		return callback(newEntries)
	})
}

// NewObject finds the Object at remote.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	// Read metadata from metadata object
	mo, err := f.Fs.NewObject(ctx, generateMetadataName(remote))
	if err != nil {
		return nil, err
	}
	meta := readMetadata(ctx, mo)
	if meta == nil {
		return nil, errors.New("error decoding metadata")
	}
	// Create our Object
	o, err := f.Fs.NewObject(ctx, generateDataNameFromCompressionMode(remote, meta.Size, meta.CompressionMode))
	return f.newObject(o, mo, meta), err
}

// Checks the compressibility and mime type of a file. Returns a rewinded reader, whether the file is compressible, and an error code.
func (c *Compression) checkFileCompressibilityAndType(in io.Reader) (newReader io.Reader, compressible bool, mimeType string, err error) {
	// Unwrap accounting, get compressibility of file, rewind reader, then wrap accounting back on
	in, wrap := accounting.UnWrap(in)
	var b bytes.Buffer
	_, err = io.CopyN(&b, in, c.HeuristicBytes)
	if err != nil && err != io.EOF {
		return nil, false, "", err
	}
	compressible, _, err = c.GetFileCompressionInfo(bytes.NewReader(b.Bytes()))
	if err != nil {
		return nil, false, "", err
	}
	mime := mimetype.Detect(b.Bytes())
	in = io.MultiReader(bytes.NewReader(b.Bytes()), in)
	in = wrap(in)
	return in, compressible, mime.String(), nil
}

// Verifies an object hash
func (f *Fs) verifyObjectHash(ctx context.Context, o fs.Object, hasher *hash.MultiHasher, ht hash.Type) (err error) {
	srcHash := hasher.Sums()[ht]
	var dstHash string
	dstHash, err = o.Hash(ctx, ht)
	if err != nil {
		return errors.Wrap(err, "failed to read destination hash")
	}
	if srcHash != "" && dstHash != "" && srcHash != dstHash {
		// remove object
		err = o.Remove(ctx)
		if err != nil {
			fs.Errorf(o, "Failed to remove corrupted object: %v", err)
		}
		return errors.Errorf("corrupted on transfer: %v compressed hashes differ %q vs %q", ht, srcHash, dstHash)
	}
	return nil
}

type putFn func(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error)

type blockDataAndError struct {
	err       error
	blockData []uint32
}

// Put a compressed version of a file. Returns a wrappable object and metadata.
func (f *Fs) putCompress(ctx context.Context, in io.Reader, src fs.ObjectInfo, options []fs.OpenOption, put putFn, mimeType string, verifyCompressedObject bool) (fs.Object, *ObjectMetadata, error) {
	// Unwrap reader accounting
	in, wrap := accounting.UnWrap(in)

	// Add the metadata hasher
	metaHasher := md5.New()
	in = io.TeeReader(in, metaHasher)

	// Compress the file
	var wrappedIn io.Reader
	pipeReader, pipeWriter := io.Pipe()
	compressionResult := make(chan blockDataAndError)
	go func() {
		blockData, err := f.c.CompressFileReturningBlockData(in, pipeWriter)
		closeErr := pipeWriter.Close()
		if closeErr != nil {
			fs.Errorf(nil, "Failed to close compression pipe: %v", err)
			if err == nil {
				err = closeErr
			}
		}
		compressionResult <- blockDataAndError{err: err, blockData: blockData}
	}()
	wrappedIn = wrap(bufio.NewReaderSize(pipeReader, bufferSize)) // Bufio required for multithreading

	// If verifyCompressedObject is on, find a hash the destination supports to compute a hash of
	// the compressed data.
	ht := f.Fs.Hashes().GetOne()
	var hasher *hash.MultiHasher
	var err error
	if ht != hash.None && verifyCompressedObject {
		// unwrap the accounting again
		wrappedIn, wrap = accounting.UnWrap(wrappedIn)
		hasher, err = hash.NewMultiHasherTypes(hash.NewHashSet(ht))
		if err != nil {
			return nil, nil, err
		}
		// add the hasher and re-wrap the accounting
		wrappedIn = io.TeeReader(wrappedIn, hasher)
		wrappedIn = wrap(wrappedIn)
	}

	// Transfer the data
	//o, err := put(ctx, wrappedIn, f.wrapInfo(src, f.c.generateDataName(src.Remote(), src.Size(), true), src.Size()), options...)
	o, err := operations.Rcat(ctx, f.Fs, f.c.generateDataName(src.Remote(), src.Size(), true), ioutil.NopCloser(wrappedIn), src.ModTime(ctx))
	if err != nil {
		if o != nil {
			removeErr := o.Remove(ctx)
			if removeErr != nil {
				fs.Errorf(o, "Failed to remove partially transferred object: %v", err)
			}
		}
		return nil, nil, err
	}
	// Check whether we got an error during compression
	result := <-compressionResult
	err = result.err
	if err != nil {
		if o != nil {
			removeErr := o.Remove(ctx)
			if removeErr != nil {
				fs.Errorf(o, "Failed to remove partially compressed object: %v", err)
			}
		}
		return nil, nil, err
	}

	// Generate metadata
	blockData := result.blockData
	_, _, decompressedSize := parseBlockData(blockData, f.c.BlockSize)
	meta := newMetadata(decompressedSize, f.c.CompressionMode, blockData, metaHasher.Sum(nil), mimeType)

	// Check the hashes of the compressed data if we were comparing them
	if ht != hash.None && hasher != nil {
		err := f.verifyObjectHash(ctx, o, hasher, ht)
		if err != nil {
			return nil, nil, err
		}
	}

	return o, meta, nil
}

// Put an uncompressed version of a file. Returns a wrappable object and metadata.
func (f *Fs) putUncompress(ctx context.Context, in io.Reader, src fs.ObjectInfo, options []fs.OpenOption, put putFn, mimeType string, verifyCompressedObject bool) (fs.Object, *ObjectMetadata, error) {
	// Unwrap the accounting, add our metadata hasher, then wrap it back on
	in, wrap := accounting.UnWrap(in)
	metaHasher := md5.New()
	in = io.TeeReader(in, metaHasher)
	wrappedIn := wrap(in)
	// If verifyCompressedObject is on, find a hash the destination supports to compute a hash of
	// the compressed data.
	ht := f.Fs.Hashes().GetOne()
	var hasher *hash.MultiHasher
	var err error
	if ht != hash.None && verifyCompressedObject {
		// unwrap the accounting again
		wrappedIn, wrap = accounting.UnWrap(wrappedIn)
		hasher, err = hash.NewMultiHasherTypes(hash.NewHashSet(ht))
		if err != nil {
			return nil, nil, err
		}
		// add the hasher and re-wrap the accounting
		wrappedIn = io.TeeReader(wrappedIn, hasher)
		wrappedIn = wrap(wrappedIn)
	}
	// Put the object
	o, err := put(ctx, wrappedIn, f.wrapInfo(src, f.c.generateDataName(src.Remote(), src.Size(), false), src.Size()), options...)
	//o, err := operations.Rcat(f, f.c.generateDataName(src.Remote(), src.Size(), false), wrappedIn, src.ModTime())
	if err != nil {
		if o != nil {
			removeErr := o.Remove(ctx)
			if removeErr != nil {
				fs.Errorf(o, "Failed to remove partially transferred object: %v", err)
			}
		}
		return nil, nil, err
	}
	// Check the hashes of the compressed data if we were comparing them
	if ht != hash.None && hasher != nil {
		err := f.verifyObjectHash(ctx, o, hasher, ht)
		if err != nil {
			return nil, nil, err
		}
	}
	// Return our object and metadata
	return o, newMetadata(o.Size(), Uncompressed, []uint32{}, metaHasher.Sum([]byte{}), mimeType), nil
}

// This function will write a metadata struct to a metadata Object for an src. Returns a wrappable metadata object.
func (f *Fs) putMetadata(ctx context.Context, meta *ObjectMetadata, src fs.ObjectInfo, options []fs.OpenOption, put putFn, verifyCompressedObject bool) (mo fs.Object, err error) {
	// Generate the metadata contents
	var b bytes.Buffer
	gzipWriter := gzip.NewWriter(&b)
	metadataEncoder := gob.NewEncoder(gzipWriter)
	err = metadataEncoder.Encode(meta)
	if err != nil {
		return nil, err
	}
	err = gzipWriter.Close()
	if err != nil {
		return nil, err
	}
	metaReader := bytes.NewReader(b.Bytes())

	// Put the data
	mo, err = put(ctx, metaReader, f.wrapInfo(src, generateMetadataName(src.Remote()), int64(b.Len())), options...)
	if err != nil {
		removeErr := mo.Remove(ctx)
		if removeErr != nil {
			fs.Errorf(mo, "Failed to remove partially transferred object: %v", err)
		}
		return nil, err
	}

	return mo, nil
}

// This function will put both the data and metadata for an Object.
// putData is the function used for data, while putMeta is the function used for metadata.
func (f *Fs) putWithCustomFunctions(ctx context.Context, in io.Reader, src fs.ObjectInfo, options []fs.OpenOption,
	putData putFn, putMeta putFn, compressible bool, mimeType string, verifyCompressedObject bool) (*Object, error) {
	// Put file then metadata
	var dataObject fs.Object
	var meta *ObjectMetadata
	var err error
	if compressible {
		dataObject, meta, err = f.putCompress(ctx, in, src, options, putData, mimeType, verifyCompressedObject)
	} else {
		dataObject, meta, err = f.putUncompress(ctx, in, src, options, putData, mimeType, verifyCompressedObject)
	}
	if err != nil {
		return nil, err
	}

	mo, err := f.putMetadata(ctx, meta, src, options, putMeta, verifyCompressedObject)

	// meta data upload may fail. in this case we try to remove the original object
	if err != nil {
		removeError := dataObject.Remove(ctx)
		if removeError != nil {
			return nil, removeError
		}
		return nil, err
	}
	return f.newObject(dataObject, mo, meta), err
}

// Put in to the remote path with the modTime given of the given size
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	// If there's already an existent objects we need to make sure to explcitly update it to make sure we don't leave
	// orphaned data. Alternatively we could also deleted (which would simpler) but has the disadvantage that it
	// destroys all server-side versioning.
	o, err := f.NewObject(ctx, src.Remote())
	if err == fs.ErrorObjectNotFound {
		// Get our file compressibility
		in, compressible, mimeType, err := f.c.checkFileCompressibilityAndType(in)
		if err != nil {
			return nil, err
		}
		return f.putWithCustomFunctions(ctx, in, src, options, f.Fs.Put, f.Fs.Put, compressible, mimeType, true)
	}
	if err != nil {
		return nil, err
	}
	return o, o.Update(ctx, in, src, options...)
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	oldObj, err := f.NewObject(ctx, src.Remote())
	if err != nil && err != fs.ErrorObjectNotFound {
		return nil, err
	}
	found := err == nil

	in, compressible, mimeType, err := f.c.checkFileCompressibilityAndType(in)
	if err != nil {
		return nil, err
	}
	newObj, err := f.putWithCustomFunctions(ctx, in, src, options, f.Fs.Features().PutStream, f.Fs.Put, compressible, mimeType, true)
	if err != nil {
		return nil, err
	}

	// Our transfer is now complete if we allready had an object with the same name we can safely remove it now
	// this is necessary to make sure we don't leave the remote in an inconsistent state.
	if found {
		err = oldObj.(*Object).Object.Remove(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "Could remove original object")
		}
	}

	moveFs, ok := f.Fs.(fs.Mover)
	var wrapObj fs.Object
	if ok {
		wrapObj, err = moveFs.Move(ctx, newObj.Object, f.c.generateDataName(src.Remote(), newObj.size, compressible))
		if err != nil {
			return nil, errors.Wrap(err, "Couldn't rename streamed object.")
		}
	}

	// If we don't have move we'll need to resort to serverside copy and remove
	copyFs, ok := f.Fs.(fs.Copier)
	if ok {
		wrapObj, err := copyFs.Copy(ctx, newObj.Object, f.c.generateDataName(src.Remote(), newObj.size, compressible))
		if err != nil {
			return nil, errors.Wrap(err, "Could't copy streamed object.")
		}
		// Remove the original
		err = newObj.Remove(ctx)
		if err != nil {
			return wrapObj, errors.Wrap(err, "Couldn't remove original streamed object. Remote may be in an incositent state.")
		}
	}

	newObj.Object = wrapObj

	return newObj, nil
}

// Temporarely disabled. There might be a way to implement this correctly but with the current handling metadata duplicate objects
// will break stuff. Right no I can't think of a way to make this work.

// PutUnchecked uploads the object
//
// This will create a duplicate if we upload a new file without
// checking to see if there is one already - use Put() for that.
/*func (f *Fs) PutUnchecked(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	// If PutUnchecked is supported, do it.
	// I'm unsure about this. With the current metadata model this might actually break things. Needs some manual testing.
	do := f.Fs.Features().PutUnchecked
	if do == nil {
		return nil, errors.New("can't PutUnchecked")
	}
	return f.putWithCustomFunctions(ctx, in, src, options, do, do, false)
}*/

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.MD5)
}

// Mkdir makes the directory (container, bucket)
//
// Shouldn't return an error if it already exists
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	return f.Fs.Mkdir(ctx, dir)
}

// Rmdir removes the directory (container, bucket) if empty
//
// Return an error if it doesn't exist or isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return f.Fs.Rmdir(ctx, dir)
}

// Purge all files in the root and the root directory
//
// Implement this if you have a way of deleting all the files
// quicker than just running Remove() on the result of List()
//
// Return an error if it doesn't exist
func (f *Fs) Purge(ctx context.Context, dir string) error {
	do := f.Fs.Features().Purge
	if do == nil {
		return fs.ErrorCantPurge
	}
	return do(ctx, dir)
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
	// We might be trying to overwrite a file with a newer version but due to size difference the name
	// is different. Therefore we have to remove the old file first (if it exists).
	dstFile, err := f.NewObject(ctx, remote)
	if err != nil && err != fs.ErrorObjectNotFound {
		return nil, err
	}
	if err == nil {
		err := dstFile.Remove(ctx)
		if err != nil {
			return nil, err
		}
	}

	// Copy over metadata
	err = o.loadMetadataIfNotLoaded(ctx)
	if err != nil {
		return nil, err
	}
	newFilename := generateMetadataName(remote)
	moResult, err := do(ctx, o.mo, newFilename)
	if err != nil {
		return nil, err
	}
	// Copy over data
	newFilename = generateDataNameFromCompressionMode(remote, src.Size(), o.meta.CompressionMode)
	oResult, err := do(ctx, o.Object, newFilename)
	if err != nil {
		return nil, err
	}
	return f.newObject(oResult, moResult, o.meta), nil
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
	// We might be trying to overwrite a file with a newer version but due to size difference the name
	// is different. Therefore we have to remove the old file first (if it exists).
	dstFile, err := f.NewObject(ctx, remote)
	if err != nil && err != fs.ErrorObjectNotFound {
		return nil, err
	}
	if err == nil {
		err := dstFile.Remove(ctx)
		if err != nil {
			return nil, err
		}
	}

	// Move metadata
	err = o.loadMetadataIfNotLoaded(ctx)
	if err != nil {
		return nil, err
	}
	newFilename := generateMetadataName(remote)
	moResult, err := do(ctx, o.mo, newFilename)
	if err != nil {
		return nil, err
	}

	// Move data
	newFilename = generateDataNameFromCompressionMode(remote, src.Size(), o.meta.CompressionMode)
	oResult, err := do(ctx, o.Object, newFilename)
	if err != nil {
		return nil, err
	}
	return f.newObject(oResult, moResult, o.meta), nil
}

// DirMove moves src, srcRemote to this remote at dstRemote
// using server side move operations.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantDirMove
//
// If destination exists then return fs.ErrorDirExists
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) error {
	do := f.Fs.Features().DirMove
	if do == nil {
		return fs.ErrorCantDirMove
	}
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debugf(srcFs, "Can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}
	return do(ctx, srcFs.Fs, srcRemote, dstRemote)
}

// CleanUp the trash in the Fs
//
// Implement this if you have a way of emptying the trash or
// otherwise cleaning up old versions of files.
func (f *Fs) CleanUp(ctx context.Context) error {
	do := f.Fs.Features().CleanUp
	if do == nil {
		return errors.New("can't CleanUp")
	}
	return do(ctx)
}

// About gets quota information from the Fs
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	do := f.Fs.Features().About
	if do == nil {
		return nil, errors.New("About not supported")
	}
	return do(ctx)
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
func (f *Fs) MergeDirs(ctx context.Context, dirs []fs.Directory) error {
	do := f.Fs.Features().MergeDirs
	if do == nil {
		return errors.New("MergeDirs not supported")
	}
	out := make([]fs.Directory, len(dirs))
	for i, dir := range dirs {
		out[i] = fs.NewDirCopy(ctx, dir).SetRemote(dir.Remote())
	}
	return do(ctx, out)
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
func (f *Fs) ChangeNotify(ctx context.Context, notifyFunc func(string, fs.EntryType), pollIntervalChan <-chan time.Duration) {
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
			// Note: All we really need to do to monitor the object is to check whether the metadata changed,
			// as the metadata contains the hash. This will work unless there's a hash collision and the sizes stay the same.
			wrappedPath = generateMetadataName(path)
		default:
			fs.Errorf(path, "press ChangeNotify: ignoring unknown EntryType %d", entryType)
			return
		}
		notifyFunc(wrappedPath, entryType)
	}
	do(ctx, wrappedNotifyFunc, pollIntervalChan)
}

// PublicLink generates a public link to the remote path (usually readable by anyone)
func (f *Fs) PublicLink(ctx context.Context, remote string, duration fs.Duration, unlink bool) (string, error) {
	do := f.Fs.Features().PublicLink
	if do == nil {
		return "", errors.New("PublicLink not supported")
	}
	o, err := f.NewObject(ctx, remote)
	if err != nil {
		// assume it is a directory
		return do(ctx, remote, duration, unlink)
	}
	return do(ctx, o.(*Object).Object.Remote(), duration, unlink)
}

/*** OBJECT FUNCTIONS ***/

// ObjectMetadata describes the metadata for an Object.
type ObjectMetadata struct {
	Size            int64    // Uncompressed size of the file.
	CompressionMode int      // Compression mode of the file.
	BlockData       []uint32 // Block indexing data for the file.
	Hash            []byte   // MD5 hash of the file.
	MimeType        string   // Mime type of the file
}

// Object with external metadata
type Object struct {
	fs.Object                 // Wraps around data object for this object
	f         *Fs             // Filesystem object is in
	mo        fs.Object       // Metadata object for this object
	moName    string          // Metadata file name for this object
	size      int64           // Size of this object
	meta      *ObjectMetadata // Metadata struct for this object (nil if not loaded)
}

// This function generates a metadata object
func newMetadata(size int64, compressionMode int, blockData []uint32, hash []byte, mimeType string) *ObjectMetadata {
	meta := new(ObjectMetadata)
	meta.Size = size
	meta.CompressionMode = compressionMode
	meta.BlockData = blockData
	meta.Hash = hash
	meta.MimeType = mimeType
	return meta
}

// This function will read the metadata from a metadata object.
func readMetadata(ctx context.Context, mo fs.Object) (meta *ObjectMetadata) {
	// Open our meradata object
	rc, err := mo.Open(ctx)
	if err != nil {
		return nil
	}
	defer func() {
		err := rc.Close()
		if err != nil {
			fs.Errorf(mo, "Error closing object: %v", err)
		}
	}()
	// Read gzipped compressed data from it
	gzipReader, err := gzip.NewReader(rc)
	if err != nil {
		return nil
	}
	// Decode the gob from that
	meta = new(ObjectMetadata)
	metadataDecoder := gob.NewDecoder(gzipReader)
	err = metadataDecoder.Decode(meta)
	// Cleanup and return
	_ = gzipReader.Close() // We don't really care if this is closed properly
	if err != nil {
		return nil
	}
	return meta
}

// Remove removes this object
func (o *Object) Remove(ctx context.Context) error {
	err := o.loadMetadataObjectIfNotLoaded(ctx)
	if err != nil {
		return err
	}
	err = o.mo.Remove(ctx)
	objErr := o.Object.Remove(ctx)
	if err != nil {
		return err
	}
	return objErr
}

// ReadCloserWrapper combines a Reader and a Closer to a ReadCloser
type ReadCloserWrapper struct {
	dataSource io.Reader
	closer     io.Closer
}

func combineReaderAndCloser(dataSource io.Reader, closer io.Closer) *ReadCloserWrapper {
	rc := new(ReadCloserWrapper)
	rc.dataSource = dataSource
	rc.closer = closer
	return rc
}

// Read function
func (w *ReadCloserWrapper) Read(p []byte) (n int, err error) {
	return w.dataSource.Read(p)
}

// Close function
func (w *ReadCloserWrapper) Close() error {
	return w.closer.Close()
}

// Update in to the object with the modTime given of the given size
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	err = o.loadMetadataIfNotLoaded(ctx) // Loads metadata object too
	if err != nil {
		return err
	}
	// Function that updates metadata object
	updateMeta := func(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
		return o.mo, o.mo.Update(ctx, in, src, options...)
	}

	// Get our file compressibility
	in, compressible, mimeType, err := o.f.c.checkFileCompressibilityAndType(in)
	if err != nil {
		return err
	}

	// Since we're encoding the original filesize in the name we'll need to make sure that this name is updated before the actual update
	var newObject *Object
	origName := o.Remote()
	if o.meta.CompressionMode != Uncompressed || compressible {
		// If we aren't, we must either move-then-update or reupload-then-remove the object, and update the metadata.
		// Check if this FS supports moving
		moveFs, ok := o.f.Fs.(fs.Mover)
		if ok { // If this fs supports moving, use move-then-update. This may help keep some versioning alive.
			// First, move the object
			var movedObject fs.Object
			movedObject, err = moveFs.Move(ctx, o.Object, o.f.c.generateDataName(o.Remote(), src.Size(), compressible))
			if err != nil {
				return err
			}
			// Create function that updates moved object, then update
			update := func(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
				return movedObject, movedObject.Update(ctx, in, src, options...)
			}
			newObject, err = o.f.putWithCustomFunctions(ctx, in, src, options, update, updateMeta, compressible, mimeType, true)
		} else { // If this fs does not support moving, fall back to reuploading the object then removing the old one.
			newObject, err = o.f.putWithCustomFunctions(ctx, in, o.f.wrapInfo(src, origName, src.Size()), options, o.f.Fs.Put, updateMeta, compressible, mimeType, true)
			removeErr := o.Object.Remove(ctx) // Note: We must do remove later so a failed update doesn't destroy data.
			if removeErr != nil {
				return removeErr
			}
		}
	} else {
		// Function that updates object
		update := func(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
			return o.Object, o.Object.Update(ctx, in, src, options...)
		}
		// If we are, just update the object and metadata
		newObject, err = o.f.putWithCustomFunctions(ctx, in, src, options, update, updateMeta, compressible, mimeType, true)
	}
	if err != nil {
		return err
	}
	// Update object metadata and return
	o.Object = newObject.Object
	o.meta = newObject.meta
	o.size = newObject.size
	return nil
}

// This will initialize the variables of a new press Object. The metadata object, mo, and metadata struct, meta, must be specified.
func (f *Fs) newObject(o fs.Object, mo fs.Object, meta *ObjectMetadata) *Object {
	return &Object{
		Object: o,
		f:      f,
		mo:     mo,
		moName: mo.Remote(),
		size:   meta.Size,
		meta:   meta,
	}
}

// This initializes the variables of a press Object with only the size. The metadata will be loaded later on demand.
func (f *Fs) newObjectSizeAndNameOnly(o fs.Object, moName string, size int64) *Object {
	return &Object{
		Object: o,
		f:      f,
		mo:     nil,
		moName: moName,
		size:   size,
		meta:   nil,
	}
}

// This loads the metadata of a press Object if it's not loaded yet
func (o *Object) loadMetadataIfNotLoaded(ctx context.Context) (err error) {
	err = o.loadMetadataObjectIfNotLoaded(ctx)
	if err != nil {
		return err
	}
	if o.meta == nil {
		o.meta = readMetadata(ctx, o.mo)
	}
	return err
}

// This loads the metadata object of a press Object if it's not loaded yet
func (o *Object) loadMetadataObjectIfNotLoaded(ctx context.Context) (err error) {
	if o.mo == nil {
		o.mo, err = o.f.Fs.NewObject(ctx, o.moName)
	}
	return err
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
	origFileName, _, _, err := processFileName(o.Object.Remote())
	if err != nil {
		fs.Errorf(o, "Could not get remote path for: %s", o.Object.Remote())
		return o.Object.Remote()
	}
	return origFileName
}

// Size returns the size of the file
func (o *Object) Size() int64 {
	if o.meta == nil {
		return o.size
	}
	return o.meta.Size
}

// MimeType returns the MIME type of the file
func (o *Object) MimeType(ctx context.Context) string {
	err := o.loadMetadataIfNotLoaded(ctx)
	if err != nil {
		return "error/error"
	}
	return o.meta.MimeType
}

// Hash returns the selected checksum of the file
// If no checksum is available it returns ""
func (o *Object) Hash(ctx context.Context, ht hash.Type) (string, error) {
	err := o.loadMetadataIfNotLoaded(ctx)
	if err != nil {
		return "", err
	}
	if ht&hash.MD5 == 0 {
		return "", hash.ErrUnsupported
	}
	return hex.EncodeToString(o.meta.Hash), nil
}

// SetTier performs changing storage tier of the Object if
// multiple storage classes supported
func (o *Object) SetTier(tier string) error {
	do, ok := o.Object.(fs.SetTierer)
	if !ok {
		return errors.New("press: underlying remote does not support SetTier")
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

// UnWrap returns the wrapped Object
func (o *Object) UnWrap() fs.Object {
	return o.Object
}

// Open opens the file for read.  Call Close() on the returned io.ReadCloser. Note that this call requires quite a bit of overhead.
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (rc io.ReadCloser, err error) {
	err = o.loadMetadataIfNotLoaded(ctx)
	if err != nil {
		return nil, err
	}
	// If we're uncompressed, just pass this to the underlying object
	if o.meta.CompressionMode == Uncompressed {
		return o.Object.Open(ctx, options...)
	}
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
	chunkedReader := chunkedreader.New(ctx, o.Object, initialChunkSize, maxChunkSize)
	// Get file handle
	c, err := NewCompressionPresetNumber(o.meta.CompressionMode)
	if err != nil {
		return nil, err
	}
	FileHandle, _, err := c.DecompressFileExtData(chunkedReader, o.Object.Size(), o.meta.BlockData)
	if err != nil {
		return nil, err
	}
	// Seek and limit according to the options given
	// Note: This if statement is not required anymore because all 0-size files will be uncompressed. I'm leaving this here just in case I come back here debugging.
	if offset != 0 { // Note: this if statement is only required because seeking to 0 on a 0-size file makes chunkedReader complain about an "invalid seek position".
		_, err = FileHandle.Seek(offset, io.SeekStart)
		if err != nil {
			return nil, err
		}
	}
	var fileReader io.Reader
	if limit != -1 {
		fileReader = io.LimitReader(FileHandle, limit)
	} else {
		fileReader = FileHandle
	}
	// Return a ReadCloser
	return combineReaderAndCloser(fileReader, chunkedReader), nil
}

// ObjectInfo describes a wrapped fs.ObjectInfo for being the source
type ObjectInfo struct {
	src    fs.ObjectInfo
	fs     *Fs
	remote string
	size   int64
}

func (f *Fs) wrapInfo(src fs.ObjectInfo, newRemote string, size int64) *ObjectInfo {
	return &ObjectInfo{
		src:    src,
		fs:     f,
		remote: newRemote,
		size:   size,
	}
}

// Fs returns read only access to the Fs that this object is part of
func (o *ObjectInfo) Fs() fs.Info {
	if o.fs == nil {
		panic("stub ObjectInfo")
	}
	return o.fs
}

// String returns string representation
func (o *ObjectInfo) String() string {
	return o.src.String()
}

// Storable returns whether object is storable
func (o *ObjectInfo) Storable() bool {
	return o.src.Storable()
}

// Remote returns the remote path
func (o *ObjectInfo) Remote() string {
	if o.remote != "" {
		return o.remote
	}
	return o.src.Remote()
}

// Size returns the size of the file
func (o *ObjectInfo) Size() int64 {
	if o.size != -1 {
		return o.size
	}
	return o.src.Size()
}

// ModTime returns the modification time
func (o *ObjectInfo) ModTime(ctx context.Context) time.Time {
	return o.src.ModTime(ctx)
}

// Hash returns the selected checksum of the file
// If no checksum is available it returns ""
func (o *ObjectInfo) Hash(ctx context.Context, ht hash.Type) (string, error) {
	if ht != hash.MD5 {
		return "", hash.ErrUnsupported
	}
	if o.Size() != o.src.Size() {
		return "", hash.ErrUnsupported
	}
	value, err := o.src.Hash(ctx, ht)
	if err == hash.ErrUnsupported {
		return "", hash.ErrUnsupported
	}
	return value, err
}

// ID returns the ID of the Object if known, or "" if not
func (o *Object) ID() string {
	do, ok := o.Object.(fs.IDer)
	if !ok {
		return ""
	}
	return do.ID()
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

// Return a string version
func (f *Fs) String() string {
	return fmt.Sprintf("Compressed: %s:%s", f.name, f.root)
}

// Precision returns the precision of this Fs
func (f *Fs) Precision() time.Duration {
	return f.Fs.Precision()
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = (*Fs)(nil)
	_ fs.Purger          = (*Fs)(nil)
	_ fs.Copier          = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
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
	_ fs.GetTierer       = (*Object)(nil)
	_ fs.SetTierer       = (*Object)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.ObjectUnWrapper = (*Object)(nil)
	_ fs.IDer            = (*Object)(nil)
	_ fs.MimeTyper       = (*Object)(nil)
)
