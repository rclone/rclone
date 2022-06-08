// Package compress provides wrappers for Fs and Object which implement compression.
package compress

import (
	"bufio"
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/buengese/sgzip"
	"github.com/gabriel-vasile/mimetype"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/chunkedreader"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fspath"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fs/operations"
)

// Globals
const (
	initialChunkSize = 262144  // Initial and max sizes of chunks when reading parts of the file. Currently
	maxChunkSize     = 8388608 // at 256 KiB and 8 MiB.

	bufferSize          = 8388608
	heuristicBytes      = 1048576
	minCompressionRatio = 1.1

	gzFileExt           = ".gz"
	metaFileExt         = ".json"
	uncompressedFileExt = ".bin"
)

// Compression modes
const (
	Uncompressed = 0
	Gzip         = 2
)

var nameRegexp = regexp.MustCompile(`^(.+?)\.([A-Za-z0-9-_]{11})$`)

// Register with Fs
func init() {
	// Build compression mode options.
	compressionModeOptions := []fs.OptionExample{
		{ // Default compression mode options {
			Value: "gzip",
			Help:  "Standard gzip compression with fastest parameters.",
		},
	}

	// Register our remote
	fs.Register(&fs.RegInfo{
		Name:        "compress",
		Description: "Compress a remote",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:     "remote",
			Help:     "Remote to compress.",
			Required: true,
		}, {
			Name:     "mode",
			Help:     "Compression mode.",
			Default:  "gzip",
			Examples: compressionModeOptions,
		}, {
			Name: "level",
			Help: `GZIP compression level (-2 to 9).

Generally -1 (default, equivalent to 5) is recommended.
Levels 1 to 9 increase compression at the cost of speed. Going past 6 
generally offers very little return.

Level -2 uses Huffmann encoding only. Only use if you know what you
are doing.
Level 0 turns off compression.`,
			Default:  sgzip.DefaultCompression,
			Advanced: true,
		}, {
			Name: "ram_cache_limit",
			Help: `Some remotes don't allow the upload of files with unknown size.
In this case the compressed file will need to be cached to determine
it's size.

Files smaller than this limit will be cached in RAM, files larger than 
this limit will be cached on disk.`,
			Default:  fs.SizeSuffix(20 * 1024 * 1024),
			Advanced: true,
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	Remote           string        `config:"remote"`
	CompressionMode  string        `config:"mode"`
	CompressionLevel int           `config:"level"`
	RAMCacheLimit    fs.SizeSuffix `config:"ram_cache_limit"`
}

/*** FILESYSTEM FUNCTIONS ***/

// Fs represents a wrapped fs.Fs
type Fs struct {
	fs.Fs
	wrapper  fs.Fs
	name     string
	root     string
	opt      Options
	mode     int          // compression mode id
	features *fs.Features // optional features
}

// NewFs contstructs an Fs from the path, container:path
func NewFs(ctx context.Context, name, rpath string, m configmap.Mapper) (fs.Fs, error) {
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
		return nil, fmt.Errorf("failed to parse remote %q to wrap: %w", remote, err)
	}

	// Strip trailing slashes if they exist in rpath
	rpath = strings.TrimRight(rpath, "\\/")

	// First, check for a file
	// If a metadata file was found, return an error. Otherwise, check for a directory
	remotePath := fspath.JoinRootPath(wPath, makeMetadataName(rpath))
	wrappedFs, err := wInfo.NewFs(ctx, wName, remotePath, wConfig)
	if err != fs.ErrorIsFile {
		remotePath = fspath.JoinRootPath(wPath, rpath)
		wrappedFs, err = wInfo.NewFs(ctx, wName, remotePath, wConfig)
	}
	if err != nil && err != fs.ErrorIsFile {
		return nil, fmt.Errorf("failed to make remote %s:%q to wrap: %w", wName, remotePath, err)
	}

	// Create the wrapping fs
	f := &Fs{
		Fs:   wrappedFs,
		name: name,
		root: rpath,
		opt:  *opt,
		mode: compressionModeFromName(opt.CompressionMode),
	}
	// the features here are ones we could support, and they are
	// ANDed with the ones from wrappedFs
	f.features = (&fs.Features{
		CaseInsensitive:         true,
		DuplicateFiles:          false,
		ReadMimeType:            false,
		WriteMimeType:           false,
		GetTier:                 true,
		SetTier:                 true,
		BucketBased:             true,
		CanHaveEmptyDirectories: true,
	}).Fill(ctx, f).Mask(ctx, wrappedFs).WrapsFs(f, wrappedFs)
	// We support reading MIME types no matter the wrapped fs
	f.features.ReadMimeType = true
	// We can only support putstream if we have serverside copy or move
	if !operations.CanServerSideMove(wrappedFs) {
		f.features.Disable("PutStream")
	}

	return f, err
}

func compressionModeFromName(name string) int {
	switch name {
	case "gzip":
		return Gzip
	default:
		return Uncompressed
	}
}

// Converts an int64 to base64
func int64ToBase64(number int64) string {
	intBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(intBytes, uint64(number))
	return base64.RawURLEncoding.EncodeToString(intBytes)
}

// Converts base64 to int64
func base64ToInt64(str string) (int64, error) {
	intBytes, err := base64.RawURLEncoding.DecodeString(str)
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
	extension = compressedFileName[extensionPos:]
	nameWithSize := compressedFileName[:extensionPos]
	if extension == uncompressedFileExt {
		return nameWithSize, extension, -2, nil
	}
	match := nameRegexp.FindStringSubmatch(nameWithSize)
	if match == nil || len(match) != 3 {
		return "", "", 0, errors.New("Invalid filename")
	}
	size, err := base64ToInt64(match[2])
	if err != nil {
		return "", "", 0, errors.New("Could not decode size")
	}
	return match[1], gzFileExt, size, nil
}

// Generates the file name for a metadata file
func makeMetadataName(remote string) (newRemote string) {
	return remote + metaFileExt
}

// Checks whether a file is a metadata file
func isMetadataFile(filename string) bool {
	return strings.HasSuffix(filename, metaFileExt)
}

// makeDataName generates the file name for a data file with specified compression mode
func makeDataName(remote string, size int64, mode int) (newRemote string) {
	if mode != Uncompressed {
		newRemote = remote + "." + int64ToBase64(size) + gzFileExt
	} else {
		newRemote = remote + uncompressedFileExt
	}
	return newRemote
}

// dataName generates the file name for data file
func (f *Fs) dataName(remote string, size int64, compressed bool) (name string) {
	if !compressed {
		return makeDataName(remote, size, Uncompressed)
	}
	return makeDataName(remote, size, f.mode)
}

// addData parses an object and adds it to the DirEntries
func (f *Fs) addData(entries *fs.DirEntries, o fs.Object) {
	origFileName, _, size, err := processFileName(o.Remote())
	if err != nil {
		fs.Errorf(o, "Error on parsing file name: %v", err)
		return
	}
	if size == -2 { // File is uncompressed
		size = o.Size()
	}
	metaName := makeMetadataName(origFileName)
	*entries = append(*entries, f.newObjectSizeAndNameOnly(o, metaName, size))
}

// addDir adds a dir to the dir entries
func (f *Fs) addDir(entries *fs.DirEntries, dir fs.Directory) {
	*entries = append(*entries, f.newDir(dir))
}

// newDir returns a dir
func (f *Fs) newDir(dir fs.Directory) fs.Directory {
	return dir // We're using the same dir
}

// processEntries parses the file names and adds metadata to the dir entries
func (f *Fs) processEntries(entries fs.DirEntries) (newEntries fs.DirEntries, err error) {
	newEntries = entries[:0] // in place filter
	for _, entry := range entries {
		switch x := entry.(type) {
		case fs.Object:
			if !isMetadataFile(x.Remote()) {
				f.addData(&newEntries, x) // Only care about data files for now; metadata files are redundant.
			}
		case fs.Directory:
			f.addDir(&newEntries, x)
		default:
			return nil, fmt.Errorf("Unknown object type %T", entry)
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
	mo, err := f.Fs.NewObject(ctx, makeMetadataName(remote))
	if err != nil {
		return nil, err
	}
	meta := readMetadata(ctx, mo)
	if meta == nil {
		return nil, errors.New("error decoding metadata")
	}
	// Create our Object
	o, err := f.Fs.NewObject(ctx, makeDataName(remote, meta.CompressionMetadata.Size, meta.Mode))
	return f.newObject(o, mo, meta), err
}

// checkCompressAndType checks if an object is compressible and determines it's mime type
// returns a multireader with the bytes that were read to determine mime type
func checkCompressAndType(in io.Reader) (newReader io.Reader, compressible bool, mimeType string, err error) {
	in, wrap := accounting.UnWrap(in)
	buf := make([]byte, heuristicBytes)
	n, err := in.Read(buf)
	buf = buf[:n]
	if err != nil && err != io.EOF {
		return nil, false, "", err
	}
	mime := mimetype.Detect(buf)
	compressible, err = isCompressible(bytes.NewReader(buf))
	if err != nil {
		return nil, false, "", err
	}
	in = io.MultiReader(bytes.NewReader(buf), in)
	return wrap(in), compressible, mime.String(), nil
}

// isCompressible checks the compression ratio of the provided data and returns true if the ratio exceeds
// the configured threshold
func isCompressible(r io.Reader) (bool, error) {
	var b bytes.Buffer
	w, err := sgzip.NewWriterLevel(&b, sgzip.DefaultCompression)
	if err != nil {
		return false, err
	}
	n, err := io.Copy(w, r)
	if err != nil {
		return false, err
	}
	err = w.Close()
	if err != nil {
		return false, err
	}
	ratio := float64(n) / float64(b.Len())
	return ratio > minCompressionRatio, nil
}

// verifyObjectHash verifies the Objects hash
func (f *Fs) verifyObjectHash(ctx context.Context, o fs.Object, hasher *hash.MultiHasher, ht hash.Type) error {
	srcHash := hasher.Sums()[ht]
	dstHash, err := o.Hash(ctx, ht)
	if err != nil {
		return fmt.Errorf("failed to read destination hash: %w", err)
	}
	if srcHash != "" && dstHash != "" && srcHash != dstHash {
		// remove object
		err = o.Remove(ctx)
		if err != nil {
			fs.Errorf(o, "Failed to remove corrupted object: %v", err)
		}
		return fmt.Errorf("corrupted on transfer: %v compressed hashes differ %q vs %q", ht, srcHash, dstHash)
	}
	return nil
}

type putFn func(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error)

type compressionResult struct {
	err  error
	meta sgzip.GzipMetadata
}

// replicating some of operations.Rcat functionality because we want to support remotes without streaming
// support and of course cannot know the size of a compressed file before compressing it.
func (f *Fs) rcat(ctx context.Context, dstFileName string, in io.ReadCloser, modTime time.Time, options []fs.OpenOption) (o fs.Object, err error) {

	// cache small files in memory and do normal upload
	buf := make([]byte, f.opt.RAMCacheLimit)
	if n, err := io.ReadFull(in, buf); err == io.EOF || err == io.ErrUnexpectedEOF {
		src := object.NewStaticObjectInfo(dstFileName, modTime, int64(len(buf[:n])), false, nil, f.Fs)
		return f.Fs.Put(ctx, bytes.NewBuffer(buf[:n]), src, options...)
	}

	// Need to include what we allready read
	in = &ReadCloserWrapper{
		Reader: io.MultiReader(bytes.NewReader(buf), in),
		Closer: in,
	}

	canStream := f.Fs.Features().PutStream != nil
	if canStream {
		src := object.NewStaticObjectInfo(dstFileName, modTime, -1, false, nil, f.Fs)
		return f.Fs.Features().PutStream(ctx, in, src, options...)
	}

	fs.Debugf(f, "Target remote doesn't support streaming uploads, creating temporary local file")
	tempFile, err := ioutil.TempFile("", "rclone-press-")
	defer func() {
		// these errors should be relatively uncritical and the upload should've succeeded so it's okay-ish
		// to ignore them
		_ = tempFile.Close()
		_ = os.Remove(tempFile.Name())
	}()
	if err != nil {
		return nil, fmt.Errorf("Failed to create temporary local FS to spool file: %w", err)
	}
	if _, err = io.Copy(tempFile, in); err != nil {
		return nil, fmt.Errorf("Failed to write temporary local file: %w", err)
	}
	if _, err = tempFile.Seek(0, 0); err != nil {
		return nil, err
	}
	finfo, err := tempFile.Stat()
	if err != nil {
		return nil, err
	}
	return f.Fs.Put(ctx, tempFile, object.NewStaticObjectInfo(dstFileName, modTime, finfo.Size(), false, nil, f.Fs))
}

// Put a compressed version of a file. Returns a wrappable object and metadata.
func (f *Fs) putCompress(ctx context.Context, in io.Reader, src fs.ObjectInfo, options []fs.OpenOption, mimeType string) (fs.Object, *ObjectMetadata, error) {
	// Unwrap reader accounting
	in, wrap := accounting.UnWrap(in)

	// Add the metadata hasher
	metaHasher := md5.New()
	in = io.TeeReader(in, metaHasher)

	// Compress the file
	pipeReader, pipeWriter := io.Pipe()
	results := make(chan compressionResult)
	go func() {
		gz, err := sgzip.NewWriterLevel(pipeWriter, f.opt.CompressionLevel)
		if err != nil {
			results <- compressionResult{err: err, meta: sgzip.GzipMetadata{}}
			return
		}
		_, err = io.Copy(gz, in)
		gzErr := gz.Close()
		if gzErr != nil {
			fs.Errorf(nil, "Failed to close compress: %v", gzErr)
			if err == nil {
				err = gzErr
			}
		}
		closeErr := pipeWriter.Close()
		if closeErr != nil {
			fs.Errorf(nil, "Failed to close pipe: %v", closeErr)
			if err == nil {
				err = closeErr
			}
		}
		results <- compressionResult{err: err, meta: gz.MetaData()}
	}()
	wrappedIn := wrap(bufio.NewReaderSize(pipeReader, bufferSize)) // Probably no longer needed as sgzip has it's own buffering

	// Find a hash the destination supports to compute a hash of
	// the compressed data.
	ht := f.Fs.Hashes().GetOne()
	var hasher *hash.MultiHasher
	var err error
	if ht != hash.None {
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
	o, err := f.rcat(ctx, makeDataName(src.Remote(), src.Size(), f.mode), ioutil.NopCloser(wrappedIn), src.ModTime(ctx), options)
	//o, err := operations.Rcat(ctx, f.Fs, makeDataName(src.Remote(), src.Size(), f.mode), ioutil.NopCloser(wrappedIn), src.ModTime(ctx))
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
	result := <-results
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
	meta := newMetadata(result.meta.Size, f.mode, result.meta, hex.EncodeToString(metaHasher.Sum(nil)), mimeType)

	// Check the hashes of the compressed data if we were comparing them
	if ht != hash.None && hasher != nil {
		err = f.verifyObjectHash(ctx, o, hasher, ht)
		if err != nil {
			return nil, nil, err
		}
	}

	return o, meta, nil
}

// Put an uncompressed version of a file. Returns a wrappable object and metadata.
func (f *Fs) putUncompress(ctx context.Context, in io.Reader, src fs.ObjectInfo, put putFn, options []fs.OpenOption, mimeType string) (fs.Object, *ObjectMetadata, error) {
	// Unwrap the accounting, add our metadata hasher, then wrap it back on
	in, wrap := accounting.UnWrap(in)

	hs := hash.NewHashSet(hash.MD5)
	ht := f.Fs.Hashes().GetOne()
	if !hs.Contains(ht) {
		hs.Add(ht)
	}
	metaHasher, err := hash.NewMultiHasherTypes(hs)
	if err != nil {
		return nil, nil, err
	}
	in = io.TeeReader(in, metaHasher)
	wrappedIn := wrap(in)

	// Put the object
	o, err := put(ctx, wrappedIn, f.wrapInfo(src, makeDataName(src.Remote(), src.Size(), Uncompressed), src.Size()), options...)
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
	if ht != hash.None {
		err := f.verifyObjectHash(ctx, o, metaHasher, ht)
		if err != nil {
			return nil, nil, err
		}
	}

	// Return our object and metadata
	sum, err := metaHasher.Sum(hash.MD5)
	if err != nil {
		return nil, nil, err
	}
	return o, newMetadata(o.Size(), Uncompressed, sgzip.GzipMetadata{}, hex.EncodeToString(sum), mimeType), nil
}

// This function will write a metadata struct to a metadata Object for an src. Returns a wrappable metadata object.
func (f *Fs) putMetadata(ctx context.Context, meta *ObjectMetadata, src fs.ObjectInfo, options []fs.OpenOption, put putFn) (mo fs.Object, err error) {
	// Generate the metadata contents
	data, err := json.Marshal(meta)
	if err != nil {
		return nil, err
	}
	metaReader := bytes.NewReader(data)

	// Put the data
	mo, err = put(ctx, metaReader, f.wrapInfo(src, makeMetadataName(src.Remote()), int64(len(data))), options...)
	if err != nil {
		if mo != nil {
			removeErr := mo.Remove(ctx)
			if removeErr != nil {
				fs.Errorf(mo, "Failed to remove partially transferred object: %v", err)
			}
		}
		return nil, err
	}

	return mo, nil
}

// This function will put both the data and metadata for an Object.
// putData is the function used for data, while putMeta is the function used for metadata.
// The putData function will only be used when the object is not compressible if the
// data is compressible this parameter will be ignored.
func (f *Fs) putWithCustomFunctions(ctx context.Context, in io.Reader, src fs.ObjectInfo, options []fs.OpenOption,
	putData putFn, putMeta putFn, compressible bool, mimeType string) (*Object, error) {
	// Put file then metadata
	var dataObject fs.Object
	var meta *ObjectMetadata
	var err error
	if compressible {
		dataObject, meta, err = f.putCompress(ctx, in, src, options, mimeType)
	} else {
		dataObject, meta, err = f.putUncompress(ctx, in, src, putData, options, mimeType)
	}
	if err != nil {
		return nil, err
	}

	mo, err := f.putMetadata(ctx, meta, src, options, putMeta)

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
	// If there's already an existent objects we need to make sure to explicitly update it to make sure we don't leave
	// orphaned data. Alternatively we could also deleted (which would simpler) but has the disadvantage that it
	// destroys all server-side versioning.
	o, err := f.NewObject(ctx, src.Remote())
	if err == fs.ErrorObjectNotFound {
		// Get our file compressibility
		in, compressible, mimeType, err := checkCompressAndType(in)
		if err != nil {
			return nil, err
		}
		return f.putWithCustomFunctions(ctx, in, src, options, f.Fs.Put, f.Fs.Put, compressible, mimeType)
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

	in, compressible, mimeType, err := checkCompressAndType(in)
	if err != nil {
		return nil, err
	}
	newObj, err := f.putWithCustomFunctions(ctx, in, src, options, f.Fs.Features().PutStream, f.Fs.Put, compressible, mimeType)
	if err != nil {
		return nil, err
	}

	// Our transfer is now complete. We have to make sure to remove the old object because our new object will
	// have a different name except when both the old and the new object where uncompressed.
	if found && (oldObj.(*Object).meta.Mode != Uncompressed || compressible) {
		err = oldObj.(*Object).Object.Remove(ctx)
		if err != nil {
			return nil, fmt.Errorf("Could remove original object: %w", err)
		}
	}

	// If our new object is compressed we have to rename it with the correct size.
	// Uncompressed objects don't store the size in the name so we they'll allready have the correct name.
	if compressible {
		wrapObj, err := operations.Move(ctx, f.Fs, nil, f.dataName(src.Remote(), newObj.size, compressible), newObj.Object)
		if err != nil {
			return nil, fmt.Errorf("Couldn't rename streamed Object.: %w", err)
		}
		newObj.Object = wrapObj
	}
	return newObj, nil
}

// Temporarely disabled. There might be a way to implement this correctly but with the current handling metadata duplicate objects
// will break stuff. Right no I can't think of a way to make this work.

// PutUnchecked uploads the object
//
// This will create a duplicate if we upload a new file without
// checking to see if there is one already - use Put() for that.

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
	newFilename := makeMetadataName(remote)
	moResult, err := do(ctx, o.mo, newFilename)
	if err != nil {
		return nil, err
	}
	// Copy over data
	newFilename = makeDataName(remote, src.Size(), o.meta.Mode)
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
	newFilename := makeMetadataName(remote)
	moResult, err := do(ctx, o.mo, newFilename)
	if err != nil {
		return nil, err
	}

	// Move data
	newFilename = makeDataName(remote, src.Size(), o.meta.Mode)
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
		return errors.New("not supported by underlying remote")
	}
	return do(ctx)
}

// About gets quota information from the Fs
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	do := f.Fs.Features().About
	if do == nil {
		return nil, errors.New("not supported by underlying remote")
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
			wrappedPath = makeMetadataName(path)
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
		return "", errors.New("can't PublicLink: not supported by underlying remote")
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
	Mode                int    // Compression mode of the file.
	Size                int64  // Size of the object.
	MD5                 string // MD5 hash of the file.
	MimeType            string // Mime type of the file
	CompressionMetadata sgzip.GzipMetadata
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
func newMetadata(size int64, mode int, cmeta sgzip.GzipMetadata, md5 string, mimeType string) *ObjectMetadata {
	meta := new(ObjectMetadata)
	meta.Size = size
	meta.Mode = mode
	meta.CompressionMetadata = cmeta
	meta.MD5 = md5
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
	jr := json.NewDecoder(rc)
	meta = new(ObjectMetadata)
	if err = jr.Decode(meta); err != nil {
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
	io.Reader
	io.Closer
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

	in, compressible, mimeType, err := checkCompressAndType(in)
	if err != nil {
		return err
	}

	// Since we are storing the filesize in the name the new object may have different name than the old
	// We'll make sure to delete the old object in this case
	var newObject *Object
	origName := o.Remote()
	if o.meta.Mode != Uncompressed || compressible {
		newObject, err = o.f.putWithCustomFunctions(ctx, in, o.f.wrapInfo(src, origName, src.Size()), options, o.f.Fs.Put, updateMeta, compressible, mimeType)
		if newObject.Object.Remote() != o.Object.Remote() {
			if removeErr := o.Object.Remove(ctx); removeErr != nil {
				return removeErr
			}
		}
	} else {
		// We can only support update when BOTH the old and the new object are uncompressed because only then
		// the filesize will be known beforehand and name will stay the same
		update := func(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
			return o.Object, o.Object.Update(ctx, in, src, options...)
		}
		// If we are, just update the object and metadata
		newObject, err = o.f.putWithCustomFunctions(ctx, in, src, options, update, updateMeta, compressible, mimeType)
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

// Shutdown the backend, closing any background tasks and any
// cached connections.
func (f *Fs) Shutdown(ctx context.Context) error {
	do := f.Fs.Features().Shutdown
	if do == nil {
		return nil
	}
	return do(ctx)
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
		fs.Errorf(o.f, "Could not get remote path for: %s", o.Object.Remote())
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
	if ht != hash.MD5 {
		return "", hash.ErrUnsupported
	}
	err := o.loadMetadataIfNotLoaded(ctx)
	if err != nil {
		return "", err
	}
	return o.meta.MD5, nil
}

// SetTier performs changing storage tier of the Object if
// multiple storage classes supported
func (o *Object) SetTier(tier string) error {
	do, ok := o.Object.(fs.SetTierer)
	mdo, mok := o.mo.(fs.SetTierer)
	if !(ok && mok) {
		return errors.New("press: underlying remote does not support SetTier")
	}
	if err := mdo.SetTier(tier); err != nil {
		return err
	}
	return do.SetTier(tier)
}

// GetTier returns storage tier or class of the Object
func (o *Object) GetTier() string {
	do, ok := o.mo.(fs.GetTierer)
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
	if o.meta.Mode == Uncompressed {
		return o.Object.Open(ctx, options...)
	}
	// Get offset and limit from OpenOptions, pass the rest to the underlying remote
	var openOptions = []fs.OpenOption{&fs.SeekOption{Offset: 0}}
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
	var file io.Reader
	if offset != 0 {
		file, err = sgzip.NewReaderAt(chunkedReader, &o.meta.CompressionMetadata, offset)
	} else {
		file, err = sgzip.NewReader(chunkedReader)
	}
	if err != nil {
		return nil, err
	}

	var fileReader io.Reader
	if limit != -1 {
		fileReader = io.LimitReader(file, limit)
	} else {
		fileReader = file
	}
	// Return a ReadCloser
	return ReadCloserWrapper{Reader: fileReader, Closer: chunkedReader}, nil
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
	return o.size
}

// ModTime returns the modification time
func (o *ObjectInfo) ModTime(ctx context.Context) time.Time {
	return o.src.ModTime(ctx)
}

// Hash returns the selected checksum of the file
// If no checksum is available it returns ""
func (o *ObjectInfo) Hash(ctx context.Context, ht hash.Type) (string, error) {
	return "", nil // cannot know the checksum
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
	_ fs.Shutdowner      = (*Fs)(nil)
	_ fs.ObjectInfo      = (*ObjectInfo)(nil)
	_ fs.GetTierer       = (*Object)(nil)
	_ fs.SetTierer       = (*Object)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.ObjectUnWrapper = (*Object)(nil)
	_ fs.IDer            = (*Object)(nil)
	_ fs.MimeTyper       = (*Object)(nil)
)
