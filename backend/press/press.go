// Package press provides wrappers for Fs and Object which implement compression.
package press

import (
	"bufio"
	"bytes"
	"compress/gzip"
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

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/accounting"
	"github.com/ncw/rclone/fs/chunkedreader"
	"github.com/ncw/rclone/fs/config/configmap"
	"github.com/ncw/rclone/fs/config/configstruct"
	"github.com/ncw/rclone/fs/fspath"
	"github.com/ncw/rclone/fs/hash"
	"github.com/ncw/rclone/fs/operations" // Used for Rcat
	"github.com/pkg/errors"
)

/**
NOTES:
Filenames are now <original file name>.<extension>
Hashes and mime types now supported
Metadata files now used to store metadata and point to actual files
**/

// Globals
// Register with Fs
func init() {
	// Build compression mode options. Show XZ options only if they're supported on the current system.
	compressionModeOptions := []fs.OptionExample{{ // Default compression mode options
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
	},
	}
	if checkXZ() { // If XZ is on the system, append compression mode options that are only available with the XZ binary installed
		compressionModeOptions = append(compressionModeOptions, []fs.OptionExample{{
			Value: "xz-min",
			Help:  "Slow but powerful compression with reasonable speed.",
		}, {
			Value: "xz-default",
			Help:  "Slowest but best compression.",
		},
		}...)
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
			Help:     "Compression mode. Installing XZ will unlock XZ modes.",
			Default:  "gzip-min",
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
		BucketBased:             true,
		CanHaveEmptyDirectories: true,
		SetTier:                 true,
		GetTier:                 true,
	}).Fill(f).Mask(wrappedFs).WrapsFs(f, wrappedFs)
	// We support reading MIME types no matter the wrapped fs
	f.features.ReadMimeType = true

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

// Get an object from a metadata file
/*func (f *Fs) addMeta(entries *fs.DirEntries, mo fs.Object) {
	meta := readMetadata(mo)
	origFileName, err := processMetadataName(mo.Remote())
	if err != nil {
		fs.Errorf(mo, "Not a metadata file: %v", err)
		return
	}
	o, err := f.Fs.NewObject(generateDataNameFromCompressionMode(origFileName, meta.Size, meta.CompressionMode))
	if err != nil {
		fs.Errorf(mo, "Metadata corrupted: %v", err)
		return
	}
	*entries = append(*entries, f.newObject(o, mo, meta))
}*/

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
	// Read metadata from metadata object
	mo, err := f.Fs.NewObject(generateMetadataName(remote))
	if err != nil {
		return nil, err
	}
	meta := readMetadata(mo)
	if meta == nil {
		return nil, errors.New("error decoding metadata")
	}
	// Create our Object
	o, err := f.Fs.NewObject(generateDataNameFromCompressionMode(remote, meta.Size, meta.CompressionMode))
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
	mimeType, _ = mimetype.Detect(b.Bytes())
	in = io.MultiReader(bytes.NewReader(b.Bytes()), in)
	in = wrap(in)
	return in, compressible, mimeType, nil
}

// Verifies an object hash
func (f *Fs) verifyObjectHash(o fs.Object, hasher *hash.MultiHasher, ht hash.Type) (err error) {
	srcHash := hasher.Sums()[ht]
	var dstHash string
	dstHash, err = o.Hash(ht)
	if err != nil {
		return errors.Wrap(err, "failed to read destination hash")
	}
	if srcHash != "" && dstHash != "" && srcHash != dstHash {
		// remove object
		err = o.Remove()
		if err != nil {
			fs.Errorf(o, "Failed to remove corrupted object: %v", err)
		}
		return errors.Errorf("corrupted on transfer: %v compressed hashes differ %q vs %q", ht, srcHash, dstHash)
	}
	return nil
}

type putFn func(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error)

type blockDataAndError struct {
	err       error
	blockData []uint32
}

// Put a compressed version of a file. Returns a wrappable object and metadata.
func (f *Fs) putCompress(in io.Reader, src fs.ObjectInfo, options []fs.OpenOption, put putFn, mimeType string, verifyCompressedObject bool) (fs.Object, *ObjectMetadata, error) {
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
	//	o, err := put(wrappedIn, f.renameObjectInfo(src, f.c.generateDataName(src.Remote(), src.Size(), true), -1), options...)
	o, err := operations.Rcat(f.Fs, f.c.generateDataName(src.Remote(), src.Size(), true), ioutil.NopCloser(wrappedIn), src.ModTime())
	if err != nil {
		if o != nil {
			removeErr := o.Remove()
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
			removeErr := o.Remove()
			if removeErr != nil {
				fs.Errorf(o, "Failed to remove partially compressed object: %v", err)
			}
		}
		return nil, nil, err
	}

	// Generate metadata
	blockData := result.blockData
	_, _, decompressedSize := parseBlockData(blockData, f.c.BlockSize)
	meta := newMetadata(decompressedSize, f.c.CompressionMode, blockData, metaHasher.Sum([]byte{}), mimeType)

	// Check the hashes of the compressed data if we were comparing them
	if ht != hash.None && hasher != nil {
		err := f.verifyObjectHash(o, hasher, ht)
		if err != nil {
			return nil, nil, err
		}
	}

	return o, meta, nil
}

// Put an uncompressed version of a file. Returns a wrappable object and metadata.
func (f *Fs) putUncompress(in io.Reader, src fs.ObjectInfo, options []fs.OpenOption, put putFn, mimeType string, verifyCompressedObject bool) (fs.Object, *ObjectMetadata, error) {
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
	o, err := put(wrappedIn, f.renameObjectInfo(src, f.c.generateDataName(src.Remote(), src.Size(), false), src.Size()), options...)
	//o, err := operations.Rcat(f, f.c.generateDataName(src.Remote(), src.Size(), false), wrappedIn, src.ModTime())
	if err != nil {
		if o != nil {
			removeErr := o.Remove()
			if removeErr != nil {
				fs.Errorf(o, "Failed to remove partially transferred object: %v", err)
			}
		}
		return nil, nil, err
	}
	// Check the hashes of the compressed data if we were comparing them
	if ht != hash.None && hasher != nil {
		err := f.verifyObjectHash(o, hasher, ht)
		if err != nil {
			return nil, nil, err
		}
	}
	// Return our object and metadata
	return o, newMetadata(o.Size(), Uncompressed, []uint32{}, metaHasher.Sum([]byte{}), mimeType), nil
}

// This function will write a metadata struct to a metadata Object for an src. Returns a wrappable metadata object.
func (f *Fs) putMetadata(meta *ObjectMetadata, src fs.ObjectInfo, options []fs.OpenOption, put putFn, verifyCompressedObject bool) (mo fs.Object, err error) {
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
	var metaReader io.Reader
	metaReader = bytes.NewReader(b.Bytes())
	// If verifyCompressedObject is on, find a hash the destination supports to compute a hash of
	// the compressed data.
	ht := f.Fs.Hashes().GetOne()
	var hasher *hash.MultiHasher
	if ht != hash.None && verifyCompressedObject {
		hasher, err = hash.NewMultiHasherTypes(hash.NewHashSet(ht))
		if err != nil {
			return nil, err
		}
		metaReader = io.TeeReader(metaReader, hasher)
	}
	// Put the data
	mo, err = put(metaReader, f.renameObjectInfo(src, generateMetadataName(src.Remote()), int64(b.Len())), options...)
	if err != nil {
		removeErr := mo.Remove()
		if removeErr != nil {
			fs.Errorf(mo, "Failed to remove partially transferred object: %v", err)
		}
		return nil, err
	}
	// Check the hashes of the compressed data if we were comparing them
	if ht != hash.None && hasher != nil {
		err := f.verifyObjectHash(mo, hasher, ht)
		if err != nil {
			return nil, err
		}
	}

	return mo, nil
}

// This function will put both the data and metadata for an Object.
// putData is the function used for data, while putMeta is the function used for metadata.
func (f *Fs) putWithCustomFunctions(in io.Reader, src fs.ObjectInfo, options []fs.OpenOption, putData putFn, putMeta putFn, verifyCompressedObject bool) (*Object, error) {
	// Check compressibility of file
	in, compressible, mimeType, err := f.c.checkFileCompressibilityAndType(in)
	if err != nil {
		return nil, err
	}
	// Put file then metadata
	var dataObject fs.Object
	var meta *ObjectMetadata
	if compressible {
		dataObject, meta, err = f.putCompress(in, src, options, putData, mimeType, verifyCompressedObject)
	} else {
		dataObject, meta, err = f.putUncompress(in, src, options, putData, mimeType, verifyCompressedObject)
	}
	if err != nil {
		return nil, err
	}
	mo, err := f.putMetadata(meta, src, options, putMeta, verifyCompressedObject)
	return f.newObject(dataObject, mo, meta), err
}

// This function will put both the data and metadata for an Object, using the default f.Fs.Put for metadata and checking file hashes.
func (f *Fs) put(in io.Reader, src fs.ObjectInfo, options []fs.OpenOption, put putFn) (*Object, error) {
	return f.putWithCustomFunctions(in, src, options, put, f.Fs.Put, true)
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

// PutUnchecked uploads the object
//
// This will create a duplicate if we upload a new file without
// checking to see if there is one already - use Put() for that.
func (f *Fs) PutUnchecked(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	// If PutUnchecked is supported, do it.
	do := f.Fs.Features().PutUnchecked
	if do == nil {
		return nil, errors.New("can't PutUnchecked")
	}
	return f.putWithCustomFunctions(in, src, options, do, do, false)
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.MD5)
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
	// Copy over metadata
	err := o.loadMetadataObjectIfNotLoaded()
	if err != nil {
		return nil, err
	}
	newFilename := generateMetadataName(remote)
	moResult, err := do(o.mo, newFilename)
	if err != nil {
		return nil, err
	}
	// Copy over data
	newFilename = generateDataNameFromCompressionMode(remote, src.Size(), o.meta.CompressionMode)
	oResult, err := do(o.Object, newFilename)
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
func (f *Fs) Move(src fs.Object, remote string) (fs.Object, error) {
	do := f.Fs.Features().Move
	if do == nil {
		return nil, fs.ErrorCantMove
	}
	o, ok := src.(*Object)
	if !ok {
		return nil, fs.ErrorCantMove
	}
	// Move metadata
	err := o.loadMetadataObjectIfNotLoaded()
	if err != nil {
		return nil, err
	}
	newFilename := generateMetadataName(remote)
	moResult, err := do(o.mo, newFilename)
	if err != nil {
		return nil, err
	}
	// Move data
	newFilename = generateDataNameFromCompressionMode(remote, src.Size(), o.meta.CompressionMode)
	oResult, err := do(o.Object, newFilename)
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
			// Note: All we really need to do to monitor the object is to check whether the metadata changed,
			// as the metadata contains the hash. This will work unless there's a hash collision and the sizes stay the same.
			wrappedPath = generateMetadataName(path)
		default:
			fs.Errorf(path, "press ChangeNotify: ignoring unknown EntryType %d", entryType)
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
func readMetadata(mo fs.Object) (meta *ObjectMetadata) {
	// Open our meradata object
	rc, err := mo.Open()
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
func (o *Object) loadMetadataIfNotLoaded() (err error) {
	err = o.loadMetadataObjectIfNotLoaded()
	if err != nil {
		return err
	}
	if o.meta == nil {
		o.meta = readMetadata(o.mo)
	}
	return err
}

// This loads the metadata object of a press Object if it's not loaded yet
func (o *Object) loadMetadataObjectIfNotLoaded() (err error) {
	if o.mo == nil {
		o.mo, err = o.f.Fs.NewObject(o.moName)
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

// Remove removes this object
func (o *Object) Remove() error {
	err := o.loadMetadataObjectIfNotLoaded()
	if err != nil {
		return err
	}
	err = o.mo.Remove()
	objErr := o.Object.Remove()
	if err != nil {
		return err
	}
	return objErr
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

// Hash returns the selected checksum of the file
// If no checksum is available it returns ""
func (o *Object) Hash(ht hash.Type) (string, error) {
	err := o.loadMetadataIfNotLoaded()
	if err != nil {
		return "", err
	}
	if ht&hash.MD5 == 0 {
		return "", hash.ErrUnsupported
	}
	return hex.EncodeToString(o.meta.Hash), nil
}

// MimeType returns the MIME type of the file
func (o *Object) MimeType() string {
	err := o.loadMetadataIfNotLoaded()
	if err != nil {
		return "error/error"
	}
	return o.meta.MimeType
}

// UnWrap returns the wrapped Object
func (o *Object) UnWrap() fs.Object {
	return o.Object
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

// Open opens the file for read.  Call Close() on the returned io.ReadCloser. Note that this call requires quite a bit of overhead.
func (o *Object) Open(options ...fs.OpenOption) (rc io.ReadCloser, err error) {
	err = o.loadMetadataIfNotLoaded()
	if err != nil {
		return nil, err
	}
	// If we're uncompressed, just pass this to the underlying object
	if o.meta.CompressionMode == Uncompressed {
		return o.Object.Open(options...)
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
	chunkedReader := chunkedreader.New(o.Object, initialChunkSize, maxChunkSize)
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

// Update in to the object with the modTime given of the given size
func (o *Object) Update(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	err = o.loadMetadataIfNotLoaded() // Loads metadata object too
	if err != nil {
		return err
	}
	// Function that updates metadata object
	updateMeta := func(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
		return o.mo, o.mo.Update(in, src, options...)
	}
	// Get our file compressibility
	in, compressible, _, err := o.f.c.checkFileCompressibilityAndType(in)
	if err != nil {
		return err
	}
	// Check if we're updating an uncompressed file with an uncompressible object
	var newObject *Object
	origName := o.Remote()
	if o.meta.CompressionMode != Uncompressed || compressible {
		// If we aren't, we must either move-then-update or reupload-then-remove the object, and update the metadata.
		// Check if this FS supports moving
		moveFs, ok := o.f.Fs.(fs.Mover)
		if ok { // If this fs supports moving, use move-then-update. This may help keep some versioning alive.
			// First, move the object
			var movedObject fs.Object
			movedObject, err = moveFs.Move(o.Object, o.f.c.generateDataName(o.Remote(), src.Size(), compressible))
			if err != nil {
				return err
			}
			// Create function that updates moved object, then update
			update := func(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
				return movedObject, movedObject.Update(in, src, options...)
			}
			newObject, err = o.f.putWithCustomFunctions(in, src, options, update, updateMeta, true)
		} else { // If this fs does not support moving, fall back to reuploading the object then removing the old one.
			newObject, err = o.f.putWithCustomFunctions(in, o.f.renameObjectInfo(src, origName, src.Size()), options, o.f.Fs.Put, updateMeta, true)
			removeErr := o.Object.Remove() // Note: We must do remove later so a failed update doesn't destroy data.
			if removeErr != nil {
				return removeErr
			}
		}
	} else {
		// Function that updates object
		update := func(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
			return o.Object, o.Object.Update(in, src, options...)
		}
		// If we are, just update the object and metadata
		newObject, err = o.f.putWithCustomFunctions(in, src, options, update, updateMeta, true)
	}
	// Update object metadata and return
	o.Object = newObject.Object
	o.meta = newObject.meta
	o.size = newObject.size
	return err
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

// RenamedObjectInfo is the renamed representation of an ObjectInfo
type RenamedObjectInfo struct {
	fs.ObjectInfo
	remote string
	size   int64
}

func (f *Fs) renameObjectInfo(src fs.ObjectInfo, newRemote string, size int64) *RenamedObjectInfo {
	return &RenamedObjectInfo{
		ObjectInfo: src,
		remote:     newRemote,
		size:       size,
	}
}

// Remote gets the remote of the RenamedObjectInfo
func (o *RenamedObjectInfo) Remote() string {
	return o.remote
}

// Size is unknown
func (o *RenamedObjectInfo) Size() int64 {
	return o.size
}

// ObjectInfo describes a wrapped fs.ObjectInfo for being the source
type ObjectInfo struct {
	fs.ObjectInfo
	f    *Fs
	meta *ObjectMetadata
}

// Gets a new ObjectInfo from an src and a metadata struct
func (f *Fs) newObjectInfo(src fs.ObjectInfo) *ObjectInfo {
	return &ObjectInfo{
		ObjectInfo: src,
		f:          f,
		meta:       nil,
	}
}

// Fs returns read only access to the Fs that this object is part of
func (o *ObjectInfo) Fs() fs.Info {
	return o.f
}

// Remote returns the remote path
func (o *ObjectInfo) Remote() string {
	origFileName, _, _, err := processFileName(o.ObjectInfo.Remote())
	if err != nil {
		fs.Errorf(o, "Could not get remote path for: %s", o.ObjectInfo.Remote())
		return o.ObjectInfo.Remote()
	}
	return origFileName
}

// Size returns the size of the file
func (o *ObjectInfo) Size() int64 {
	_, _, size, err := processFileName(o.ObjectInfo.Remote())
	if err != nil {
		fs.Errorf(o, "Could not get size for: %s", o.ObjectInfo.Remote())
		return -1
	}
	if size == -2 { // File is uncompressed
		return o.ObjectInfo.Size()
	}
	return size
}

// Hash returns the selected checksum of the file
// If no checksum is available it returns ""
func (o *ObjectInfo) Hash(ht hash.Type) (string, error) {
	if o.meta == nil {
		mo, err := o.f.NewObject(generateMetadataName(o.Remote()))
		if err != nil {
			return "", err
		}
		o.meta = readMetadata(mo)
	}
	if ht&hash.MD5 == 0 {
		return "", hash.ErrUnsupported
	}
	return hex.EncodeToString(o.meta.Hash), nil
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
	_ fs.MimeTyper       = (*Object)(nil)
)
