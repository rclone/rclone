// Package filen provides an interface to Filen cloud storage.
package filen

import (
	"context"
	"errors"
	"fmt"
	"io"
	pathModule "path"
	"strings"
	"sync"
	"time"

	sdk "github.com/FilenCloudDienste/filen-sdk-go/filen"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/client"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/types"

	"github.com/google/uuid"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/list"
	"github.com/rclone/rclone/lib/encoder"
	"golang.org/x/sync/errgroup"
)

func init() {
	fs.Register(&fs.RegInfo{
		Name:        "filen",
		Description: "Filen",
		NewFs:       NewFs,
		Options: []fs.Option{
			{
				Name:     "email",
				Help:     "Email of your Filen account",
				Required: true,
			},
			{
				Name:       "password",
				Help:       "Password of your Filen account",
				Required:   true,
				IsPassword: true,
				Sensitive:  true,
			},
			{
				Name: "api_key",
				Help: `API Key for your Filen account 

Get this using the Filen CLI export-api-key command
You can download the Filen CLI from https://github.com/FilenCloudDienste/filen-cli`,
				Required:   true,
				IsPassword: true,
				Sensitive:  true,
			},
			{
				Name: "upload_concurrency",
				Help: `Concurrency for chunked uploads.

This is the upper limit for how many transfers for the same file are running concurrently.
Setting this above to a value smaller than 1 will cause uploads to deadlock.

If you are uploading small numbers of large files over high-speed links
and these uploads do not fully utilize your bandwidth, then increasing
this may help to speed up the transfers.`,
				Default:  16,
				Advanced: true,
			},
			{
				Name:     config.ConfigEncoding,
				Help:     config.ConfigEncodingHelp,
				Advanced: true,
				Default:  encoder.Standard | encoder.EncodeInvalidUtf8,
			},
			{
				Name:      "master_keys",
				Help:      "Master Keys (internal use only)",
				Sensitive: true,
				Advanced:  true,
			}, {
				Name:      "private_key",
				Help:      "Private RSA Key (internal use only)",
				Sensitive: true,
				Advanced:  true,
			}, {
				Name:      "public_key",
				Help:      "Public RSA Key (internal use only)",
				Sensitive: true,
				Advanced:  true,
			}, {
				Name:     "auth_version",
				Help:     "Authentication Version (internal use only)",
				Advanced: true,
			}, {
				Name:      "base_folder_uuid",
				Help:      "UUID of Account Root Directory (internal use only)",
				Sensitive: true,
				Advanced:  true,
			},
		},
	})
}

// NewFs constructs a Fs at the path root
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	root = opt.Encoder.FromStandardPath(root)
	password, err := obscure.Reveal(opt.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to reveal password: %w", err)
	}
	apiKey, err := obscure.Reveal(opt.APIKey)
	if err != nil {
		return nil, fmt.Errorf("failed to reveal api key: %w", err)
	}

	var filen *sdk.Filen
	if password == "INTERNAL" {
		tsconfig := sdk.TSConfig{
			Email:          opt.Email,
			MasterKeys:     strings.Split(opt.MasterKeys, "|"),
			APIKey:         apiKey,
			PublicKey:      opt.PublicKey,
			PrivateKey:     opt.PrivateKey,
			AuthVersion:    opt.AuthVersion,
			BaseFolderUUID: opt.BaseFolderUUID,
		}
		filen, err = sdk.NewFromTSConfig(tsconfig)
		if err != nil {
			return nil, err
		}
	} else {
		filen, err = sdk.NewWithAPIKey(ctx, opt.Email, password, apiKey)
		if err != nil {
			return nil, err
		}
	}

	maybeRootDir, err := filen.FindDirectory(ctx, root)
	if errors.Is(err, fs.ErrorIsFile) { // FsIsFile special case
		var err2 error
		root = pathModule.Dir(root)
		maybeRootDir, err2 = filen.FindDirectory(ctx, root)
		if err2 != nil {
			return nil, err2
		}
	} else if err != nil {
		return nil, err
	}

	fileSystem := &Fs{
		name:        name,
		root:        Directory{},
		filen:       filen,
		Enc:         opt.Encoder,
		concurrency: opt.UploadConcurrency,
	}

	fileSystem.features = (&fs.Features{
		ReadMimeType:            true,
		WriteMimeType:           true,
		CanHaveEmptyDirectories: true,
		ChunkWriterDoesntSeek:   true,
	}).Fill(ctx, fileSystem)

	fileSystem.root = Directory{
		fs:        fileSystem,
		directory: maybeRootDir, // could be null at this point
		path:      root,
	}

	// must return the error from FindDirectory (see FsIsFile)
	return fileSystem, err
}

// Options defines the configuration for this backend
type Options struct {
	Email             string               `config:"email"`
	Password          string               `config:"password"`
	APIKey            string               `config:"api_key"`
	Encoder           encoder.MultiEncoder `config:"encoding"`
	MasterKeys        string               `config:"master_keys"`
	PrivateKey        string               `config:"private_key"`
	PublicKey         string               `config:"public_key"`
	AuthVersion       int                  `config:"auth_version"`
	BaseFolderUUID    string               `config:"base_folder_uuid"`
	UploadConcurrency int                  `config:"upload_concurrency"`
}

// Fs represents a virtual filesystem mounted on a specific root folder
type Fs struct {
	name        string
	root        Directory
	filen       *sdk.Filen
	Enc         encoder.MultiEncoder
	features    *fs.Features
	concurrency int
}

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return f.root.path
}

// String converts this Fs to a string
func (f *Fs) String() string {
	return fmt.Sprintf("Filen %s at /%s", f.filen.Email, f.root.String())
}

// Precision return the precision of this Fs
func (f *Fs) Precision() time.Duration {
	return time.Millisecond
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.BLAKE3)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
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
	dir = f.Enc.FromStandardPath(dir)
	// find directory uuid
	directory, err := f.filen.FindDirectory(ctx, f.resolvePath(dir))
	if err != nil {
		return nil, err
	}

	if directory == nil {
		return nil, fs.ErrorDirNotFound
	}

	// read directory content
	files, directories, err := f.filen.ReadDirectory(ctx, directory)
	if err != nil {
		return nil, err
	}
	entries = make(fs.DirEntries, 0, len(files)+len(directories))

	for _, directory := range directories {
		entries = append(entries, &Directory{
			fs:        f,
			path:      pathModule.Join(dir, directory.Name),
			directory: directory,
		})
	}
	for _, file := range files {
		file := &Object{
			fs:   f,
			path: pathModule.Join(dir, file.Name),
			file: file,
		}
		entries = append(entries, file)
	}
	return entries, nil
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error ErrorObjectNotFound.
//
// If remote points to a directory then it should return
// ErrorIsDir if possible without doing any extra work,
// otherwise ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	remote = f.Enc.FromStandardPath(remote)
	file, err := f.filen.FindFile(ctx, f.resolvePath(remote))
	if err != nil {
		return nil, err
	}
	if file == nil {
		return nil, fs.ErrorObjectNotFound
	}
	return &Object{
		fs:   f,
		path: remote,
		file: file,
	}, nil
}

// Put in to the remote path with the modTime given of the given size
//
// When called from outside an Fs by rclone, src.Size() will always be >= 0.
// But for unknown-sized objects (indicated by src.Size() == -1), Put should either
// return an error or upload it properly (rather than e.g. calling panic).
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	for _, option := range options {
		if option.Mandatory() {
			fs.Logf(option, "Unsupported mandatory option: %v", option)
		}
	}
	path := f.Enc.FromStandardPath(src.Remote())
	resolvedPath := f.resolvePath(path)
	modTime := src.ModTime(ctx)
	parent, err := f.filen.FindDirectoryOrCreate(ctx, pathModule.Dir(resolvedPath))
	if err != nil {
		return nil, err
	}
	incompleteFile, err := types.NewIncompleteFile(f.filen.FileEncryptionVersion, pathModule.Base(resolvedPath), fs.MimeType(ctx, src), modTime, modTime, parent)
	if err != nil {
		return nil, err
	}
	uploadedFile, err := f.filen.UploadFile(ctx, incompleteFile, in)
	if err != nil {
		return nil, err
	}
	return &Object{
		fs:   f,
		path: path,
		file: uploadedFile,
	}, nil
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.Put(ctx, in, src, options...)
}

type chunkWriter struct {
	sdk.FileUpload
	filen           *sdk.Filen
	bucketAndRegion chan client.V3UploadResponse
	chunkSize       int64

	chunksLock      sync.Mutex
	knownChunks     map[int][]byte // known chunks to be hashed
	nextChunkToHash int

	sizeLock sync.Mutex
	size     int64
}

func (cw *chunkWriter) WriteChunk(ctx context.Context, chunkNumber int, reader io.ReadSeeker) (bytesWritten int64, err error) {
	realChunkNumber := int(int64(chunkNumber) * (cw.chunkSize) / sdk.ChunkSize)
	chunk := make([]byte, sdk.ChunkSize, sdk.ChunkSize+cw.EncryptionKey.Cipher.Overhead())

	totalWritten := int64(0)
	for sliceStart := 0; sliceStart < int(cw.chunkSize); sliceStart += sdk.ChunkSize {
		chunk = chunk[:sdk.ChunkSize]
		chunkRead := 0
		for {
			read, err := reader.Read(chunk[chunkRead:])
			chunkRead += read
			if err == io.EOF || chunkRead == sdk.ChunkSize {
				break
			}
			if err != nil {
				return 0, err
			}
		}
		if chunkRead == 0 {
			break
		}
		chunkReadSlice := chunk[:chunkRead]
		err = func() error {
			cw.chunksLock.Lock()
			defer cw.chunksLock.Unlock()
			if cw.nextChunkToHash == realChunkNumber {
				_, err := cw.Hasher.Write(chunkReadSlice)
				if err != nil {
					return err
				}
				cw.nextChunkToHash++
				for ; ; cw.nextChunkToHash++ {
					chunk := cw.knownChunks[cw.nextChunkToHash]
					if chunk == nil {
						break
					}
					_, err := cw.Hasher.Write(chunk)
					if err != nil {
						return err
					}
					delete(cw.knownChunks, cw.nextChunkToHash)
				}
			} else {
				chunkCopy := make([]byte, len(chunkReadSlice))
				copy(chunkCopy, chunkReadSlice)
				cw.knownChunks[realChunkNumber] = chunkCopy
			}
			return nil
		}()
		if err != nil {
			return totalWritten, err
		}
		resp, err := cw.filen.UploadChunk(ctx, &cw.FileUpload, realChunkNumber, chunkReadSlice)
		select { // only care about getting this once
		case cw.bucketAndRegion <- *resp:
		default:
		}
		if err != nil {
			return totalWritten, err
		}
		totalWritten += int64(len(chunkReadSlice))
		realChunkNumber++
	}

	cw.sizeLock.Lock()
	cw.size += totalWritten
	cw.sizeLock.Unlock()
	return totalWritten, nil
}

func (cw *chunkWriter) Close(ctx context.Context) error {
	cw.chunksLock.Lock()
	defer close(cw.bucketAndRegion)
	defer cw.chunksLock.Unlock()
	cw.sizeLock.Lock()
	size := cw.size
	cw.sizeLock.Unlock()
	if len(cw.knownChunks) != 0 {
		return errors.New("not all chunks have been hashed")
	}
	_, err := cw.filen.CompleteFileUpload(ctx, &cw.FileUpload, cw.bucketAndRegion, size)
	return err
}

func (cw *chunkWriter) Abort(ctx context.Context) error {
	return nil
}

// OpenChunkWriter returns the chunk size and a ChunkWriter
//
// Pass in the remote and the src object
// You can also use options to hint at the desired chunk size
func (f *Fs) OpenChunkWriter(ctx context.Context, remote string, src fs.ObjectInfo, options ...fs.OpenOption) (info fs.ChunkWriterInfo, writer fs.ChunkWriter, err error) {
	path := f.Enc.FromStandardPath(remote)
	resolvedPath := f.resolvePath(path)
	modTime := src.ModTime(ctx)

	chunkSize := int64(sdk.ChunkSize)
	for _, option := range options {
		switch x := option.(type) {
		case *fs.ChunkOption:
			chunkSize = x.ChunkSize
		default:
			if option.Mandatory() {
				fs.Logf(option, "Unsupported mandatory option: %v", option)
			}
		}
	}

	if chunkSize%sdk.ChunkSize != 0 {
		return info, nil, errors.New("chunk size must be a multiple of 1MB")
	}

	info = fs.ChunkWriterInfo{
		ChunkSize:         chunkSize,
		Concurrency:       f.concurrency,
		LeavePartsOnError: false,
	}

	parent, err := f.filen.FindDirectoryOrCreate(ctx, pathModule.Dir(resolvedPath))
	if err != nil {
		return info, nil, err
	}
	incompleteFile, err := types.NewIncompleteFile(f.filen.FileEncryptionVersion, pathModule.Base(resolvedPath), fs.MimeType(ctx, src), modTime, modTime, parent)
	if err != nil {
		return info, nil, err
	}
	// unused
	fu := f.filen.NewFileUpload(incompleteFile)
	return info, &chunkWriter{
		FileUpload:      *fu,
		filen:           f.filen,
		chunkSize:       chunkSize,
		bucketAndRegion: make(chan client.V3UploadResponse, 1),
		knownChunks:     make(map[int][]byte),
		nextChunkToHash: 0,
		size:            0,
	}, nil
}

// Mkdir makes the directory (container, bucket)
//
// Shouldn't return an error if it already exists
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	dirObj, err := f.filen.FindDirectoryOrCreate(ctx, f.resolvePath(f.Enc.FromStandardPath(dir)))
	if err != nil {
		return err
	}
	if dir == f.root.path {
		f.root.directory = dirObj
	}
	return nil
}

// Rmdir removes the directory (container, bucket) if empty
//
// Return an error if it doesn't exist or isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	// find directory
	resolvedPath := f.resolvePath(f.Enc.FromStandardPath(dir))
	//if resolvedPath == f.root.path {
	//	return fs.ErrorDirNotFound
	//}
	directory, err := f.filen.FindDirectory(ctx, resolvedPath)
	if err != nil {
		return err
	}
	if directory == nil {
		return errors.New("directory not found")
	}

	files, dirs, err := f.filen.ReadDirectory(ctx, directory)
	if err != nil {
		return err
	}
	if len(files) > 0 || len(dirs) > 0 {
		return errors.New("directory is not empty")
	}

	// trash directory
	err = f.filen.TrashDirectory(ctx, directory)
	if err != nil {
		return err
	}
	return nil
}

// Directory is Filen's directory type
type Directory struct {
	fs        *Fs
	path      string
	directory types.DirectoryInterface
}

// Fs returns read only access to the Fs that this object is part of
func (dir *Directory) Fs() fs.Info {
	return dir.fs
}

// String returns a description of the Object
func (dir *Directory) String() string {
	if dir == nil {
		return "<nil>"
	}
	return dir.Remote()
}

// Remote returns the remote path
func (dir *Directory) Remote() string {
	return dir.fs.Enc.ToStandardPath(dir.path)
}

// ModTime returns the modification date of the file
// It should return a best guess if one isn't available
func (dir *Directory) ModTime(ctx context.Context) time.Time {
	directory, ok := dir.directory.(*types.Directory)
	if !ok {
		return time.Time{} // todo add account creation time?
	}

	if directory.Created.IsZero() {
		obj, err := dir.fs.filen.FindDirectory(ctx, dir.fs.resolvePath(dir.path))
		newDir, ok := obj.(*types.Directory)
		if err != nil || !ok {
			return time.Now()
		}
		directory = newDir
		dir.directory = newDir
	}
	return directory.Created
}

// Size returns the size of the file
//
// filen doesn't have an efficient way to find the size of a directory
func (dir *Directory) Size() int64 {
	return -1
}

// Items returns the count of items in this directory or this
// directory and subdirectories if known, -1 for unknown
func (dir *Directory) Items() int64 {
	return -1
}

// ID returns the internal ID of this directory if known, or
// "" otherwise
func (dir *Directory) ID() string {
	return dir.directory.GetUUID()
}

// Object is Filen's normal file
type Object struct {
	fs      *Fs
	path    string
	file    *types.File
	isMoved bool
}

// Fs returns read only access to the Fs that this object is part of
func (o *Object) Fs() fs.Info {
	return o.fs
}

// String returns a description of the Object
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.Remote()
}

// Remote returns the remote path
func (o *Object) Remote() string {
	return o.fs.Enc.ToStandardPath(o.path)
}

// ModTime returns the modification date of the file
// It should return a best guess if one isn't available
func (o *Object) ModTime(ctx context.Context) time.Time {
	if o.file.LastModified.IsZero() {
		newFile, err := o.fs.filen.FindFile(ctx, o.fs.resolvePath(o.path))
		if err == nil && newFile != nil {
			o.file = newFile
		}
	}
	return o.file.LastModified
}

// Size returns the size of the file
func (o *Object) Size() int64 {
	return o.file.Size
}

// Hash returns the selected checksum of the file
// If no checksum is available it returns ""
func (o *Object) Hash(ctx context.Context, ty hash.Type) (string, error) {
	if ty != hash.BLAKE3 {
		return "", hash.ErrUnsupported
	}
	if o.file.Hash == "" {
		foundFile, err := o.fs.filen.FindFile(ctx, o.fs.resolvePath(o.path))
		if err != nil {
			return "", err
		}
		if foundFile == nil {
			return "", fs.ErrorObjectNotFound
		}
		o.file = foundFile
	}
	return o.file.Hash, nil
}

// Storable says whether this object can be stored
func (o *Object) Storable() bool {
	return true
}

// SetModTime sets the metadata on the object to set the modification date
func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
	o.file.LastModified = t
	return o.fs.filen.UpdateMeta(ctx, o.file)
}

// Open opens the file for read.  Call Close() on the returned io.ReadCloser
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	fs.FixRangeOption(options, o.Size())
	// Create variables to hold our options
	var offset int64
	var limit int64 = -1 // -1 means no limit

	// Parse the options
	for _, option := range options {
		switch opt := option.(type) {
		case *fs.RangeOption:
			offset = opt.Start
			limit = opt.End + 1 // +1 because End is inclusive
		case *fs.SeekOption:
			offset = opt.Offset
		default:
			if option.Mandatory() {
				fs.Logf(option, "Unsupported mandatory option: %v", option)
			}
		}
	}

	// Get the base reader
	readCloser := o.fs.filen.GetDownloadReaderWithOffset(ctx, o.file, offset, limit)
	return readCloser, nil
}

// Update in to the object with the modTime given of the given size
//
// When called from outside an Fs by rclone, src.Size() will always be >= 0.
// But for unknown-sized objects (indicated by src.Size() == -1), Upload should either
// return an error or update the object properly (rather than e.g. calling panic).
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	for _, option := range options {
		if option.Mandatory() {
			fs.Logf(option, "Unsupported mandatory option: %v", option)
		}
	}
	newModTime := src.ModTime(ctx)
	newIncomplete, err := o.file.NewFromBase(o.fs.filen.FileEncryptionVersion)
	if err != nil {
		return err
	}
	newIncomplete.LastModified = newModTime
	newIncomplete.Created = newModTime
	newIncomplete.SetMimeType(fs.MimeType(ctx, src))
	uploadedFile, err := o.fs.filen.UploadFile(ctx, newIncomplete, in)
	if err != nil {
		return err
	}
	o.file = uploadedFile
	return nil
}

// Remove this object
func (o *Object) Remove(ctx context.Context) error {
	if o.isMoved { // doesn't exist at this path
		return nil
	}
	err := o.fs.filen.TrashFile(ctx, *o.file)
	if err != nil {
		return err
	}
	return nil
}

// MimeType returns the content type of the Object if
// known, or "" if not
func (o *Object) MimeType(_ context.Context) string {
	return o.file.MimeType
}

// ID returns the ID of the Object if known, or "" if not
func (o *Object) ID() string {
	return o.file.GetUUID()
}

// ParentID returns the ID of the parent directory if known or nil if not
func (o *Object) ParentID() string {
	return o.file.GetParent()
}

// Purge all files in the directory specified
//
// Implement this if you have a way of deleting all the files
// quicker than just running Remove() on the result of List()
//
// Return an error if it doesn't exist
func (f *Fs) Purge(ctx context.Context, dir string) error {
	path := f.resolvePath(f.Enc.FromStandardPath(dir))
	foundDir, err := f.filen.FindDirectory(ctx, path)
	if err != nil {
		return err
	} else if foundDir == nil {
		return fs.ErrorDirNotFound
	}
	return f.filen.TrashDirectory(ctx, foundDir)
}

// Move src to this remote using server-side move operations.
//
// # This is stored with the remote path given
//
// # It returns the destination Object and a possible error
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantMove
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	obj, ok := src.(*Object)
	if !ok {
		return nil, fmt.Errorf("can't move %T: %w", src, fs.ErrorCantMove)
	}
	newRemote := f.Enc.FromStandardPath(remote)
	oldPath, newPath := obj.fs.resolvePath(f.Enc.FromStandardPath(src.Remote())), f.resolvePath(newRemote)
	oldParentPath, newParentPath := pathModule.Dir(oldPath), pathModule.Dir(newPath)
	oldName, newName := pathModule.Base(oldPath), pathModule.Base(newPath)
	if oldPath == newPath {
		return nil, fs.ErrorCantMove
	}
	err := f.filen.Lock(ctx)
	if err != nil {
		return nil, err
	}
	defer f.filen.Unlock()
	if oldParentPath == newParentPath {
		err = f.rename(ctx, obj.file, newPath, newName)
	} else if newName == oldName {
		err = f.move(ctx, obj.file, newPath, newParentPath)
	} else {
		err = f.moveWithRename(ctx, obj.file, oldPath, oldName, newPath, newParentPath, newName)
	}
	if err != nil {
		return nil, err
	}
	return moveFileObjIntoNewPath(obj, newRemote), nil
}

// moveWithRename moves item to newPath
// using a more complex set of operations designed to handle the fact that
// Filen doesn't support a single moveRename operation
// which requires some annoying hackery to get around reliably
func (f *Fs) moveWithRename(ctx context.Context, item types.NonRootFileSystemObject, oldPath, oldName, newPath, newParentPath, newName string) error {
	g, gCtx := errgroup.WithContext(ctx)
	var (
		newParentDir  types.DirectoryInterface
		renamedToUUID bool
	)

	// rename to random UUID first
	g.Go(func() error {
		err := f.filen.Rename(gCtx, item, uuid.NewString())
		if err != nil {
			return fmt.Errorf("failed to rename file: %w : %w", err, fs.ErrorCantMove)
		}
		renamedToUUID = true
		return nil
	})
	defer func() {
		// safety to try and not leave the item in a bad state
		if renamedToUUID {
			err := f.filen.Rename(ctx, item, oldName)
			if err != nil {
				fmt.Printf("ERROR: FAILED TO REVERT UUID RENAME for file %s: %s", oldPath, err)
			}
		}
	}()

	// find parent dir
	g.Go(func() error {
		var err error
		newParentDir, err = f.filen.FindDirectoryOrCreate(gCtx, newParentPath)
		return err
	})

	if err := g.Wait(); err != nil {
		return err
	}

	// move
	oldParentUUID := item.GetParent()
	err := f.filen.MoveItem(ctx, item, newParentDir.GetUUID(), true)
	if err != nil {
		return fmt.Errorf("failed to move file: %w : %w", err, fs.ErrorCantMove)
	}
	defer func() {
		// safety to try and not leave the item in a bad state
		if renamedToUUID {
			err := f.filen.MoveItem(ctx, item, oldParentUUID, true)
			if err != nil {
				fmt.Printf("ERROR: FAILED TO REVERT MOVE for file %s: %s", oldPath, err)
			}
		}
	}()

	// rename to final name
	err = f.filen.Rename(ctx, item, newName)
	if err != nil {
		return fmt.Errorf("failed to rename file: %w : %w", err, fs.ErrorCantMove)
	}
	renamedToUUID = false

	return nil
}

// move moves item to newPath
// by finding the parent and calling moveWithParentUUID
func (f *Fs) move(ctx context.Context, item types.NonRootFileSystemObject, newPath, newParentPath string) error {
	newParentDir, err := f.filen.FindDirectoryOrCreate(ctx, newParentPath)
	if err != nil {
		return fmt.Errorf("failed to find or create directory: %w : %w", err, fs.ErrorCantMove)
	}
	return f.moveWithParentUUID(ctx, item, newParentDir.GetUUID())
}

// moveWithParentUUID moves item to newParentUUID
// using a simple filen.MoveItem operation
func (f *Fs) moveWithParentUUID(ctx context.Context, item types.NonRootFileSystemObject, newParentUUID string) error {
	err := f.filen.MoveItem(ctx, item, newParentUUID, true)
	if err != nil {
		return fmt.Errorf("failed to move file: %w : %w", err, fs.ErrorCantMove)
	}

	return nil
}

// rename moves item to newPath
// using a simple Filen rename operation
func (f *Fs) rename(ctx context.Context, item types.NonRootFileSystemObject, newPath string, newName string) error {
	err := f.filen.Rename(ctx, item, newName)
	if err != nil {
		return fmt.Errorf("failed to rename item: %w : %w", err, fs.ErrorCantMove)
	}
	return nil
}

// moveFileObjIntoNewPath 'moves' an existing object into a new path
// invalidating the previous object
// and making a copy with the passed path
//
// this is to work around the fact that rclone expects to have to delete a file after moving
func moveFileObjIntoNewPath(o *Object, newPath string) *Object {
	newFile := &Object{
		fs:   o.fs,
		path: newPath,
		file: o.file,
	}
	o.isMoved = true
	return newFile
}

// DirMove moves src, srcRemote to this remote at dstRemote
// using server-side move operations.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantDirMove
//
// If destination exists then return fs.ErrorDirExists
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) error {

	srcF, ok := src.(*Fs)
	if !ok || srcF == nil {
		return fs.ErrorCantDirMove
	}
	err := f.filen.Lock(ctx)
	if err != nil {
		return err
	}
	defer f.filen.Unlock()
	g, gCtx := errgroup.WithContext(ctx)
	var (
		srcDirInt types.DirectoryInterface
		dstDir    types.DirectoryInterface
		srcPath   = srcF.resolvePath(srcF.Enc.FromStandardPath(srcRemote))
		dstPath   = f.resolvePath(f.Enc.FromStandardPath(dstRemote))
	)
	if srcPath == dstPath {
		return fs.ErrorDirExists
	}

	g.Go(func() error {
		var err error
		srcDirInt, err = srcF.filen.FindDirectory(gCtx, srcPath)
		return err
	})
	g.Go(func() error {
		var err error
		dstDir, err = f.filen.FindDirectory(gCtx, dstPath)
		return err
	})

	if err := g.Wait(); err != nil {
		return err
	}

	if srcDirInt == nil {
		return fs.ErrorDirNotFound
	}

	if dstDir != nil {
		return fs.ErrorDirExists
	}

	srcDir, ok := srcDirInt.(*types.Directory)
	if !ok {
		return fs.ErrorCantDirMove
	}

	return f.dirMoveEntireDir(ctx, srcDir, srcPath, dstPath)
}

// dirMoveEntireDir moves srcDir to newPath
// used for the case where the target directory doesn't exist
func (f *Fs) dirMoveEntireDir(ctx context.Context, srcDir *types.Directory, oldPath string, newPath string) error {
	oldParentPath, newParentPath := pathModule.Dir(oldPath), pathModule.Dir(newPath)
	oldName, newName := pathModule.Base(oldPath), pathModule.Base(newPath)
	var err error
	if oldPath == newPath {
		return fs.ErrorDirExists
	} else if oldParentPath == newParentPath {
		err = f.rename(ctx, srcDir, newPath, newName)
	} else if newName == oldName {
		err = f.move(ctx, srcDir, newPath, newParentPath)
	} else {
		err = f.moveWithRename(ctx, srcDir, oldPath, oldName, newPath, newParentPath, newName)
	}
	if err != nil {
		return err
	}
	return err
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
	basePath := f.Enc.FromStandardPath(dir)
	path := f.resolvePath(basePath)
	foundDir, err := f.filen.FindDirectory(ctx, path)
	if err != nil {
		return err
	}
	if foundDir == nil {
		return fs.ErrorDirNotFound
	}

	files, dirs, err := f.filen.ListRecursive(ctx, foundDir)
	if err != nil {
		return err
	}
	listHelper := list.NewHelper(callback)
	// have to build paths
	uuidDirMap, uuidPathMap := buildUUIDDirMaps(basePath, foundDir, dirs)

	for _, dir := range dirs {
		path, err := getPathForUUID(dir.GetUUID(), uuidPathMap, uuidDirMap)
		if err != nil {
			return err
		}
		err = listHelper.Add(&Directory{
			fs:        f,
			directory: dir,
			path:      path,
		})
		if err != nil {
			return err
		}
	}

	for _, file := range files {
		parentPath, err := getPathForUUID(file.GetParent(), uuidPathMap, uuidDirMap)
		if err != nil {
			return err
		}
		err = listHelper.Add(&Object{
			fs:   f,
			file: file,
			path: pathModule.Join(parentPath, file.GetName()),
		})
		if err != nil {
			return err
		}
	}
	return listHelper.Flush()
}

func buildUUIDDirMaps(rootPath string, rootDir types.DirectoryInterface, dirs []*types.Directory) (map[string]types.DirectoryInterface, map[string]string) {
	uuidPathMap := make(map[string]string, len(dirs)+1)
	uuidPathMap[rootDir.GetUUID()] = rootPath

	uuidDirMap := make(map[string]types.DirectoryInterface, len(dirs)+1)
	uuidDirMap[rootDir.GetUUID()] = rootDir
	for _, dir := range dirs {
		uuidDirMap[dir.GetUUID()] = dir
	}
	return uuidDirMap, uuidPathMap
}

func getPathForUUID(uuid string, uuidPathMap map[string]string, uuidDirMap map[string]types.DirectoryInterface) (string, error) {
	if path, ok := uuidPathMap[uuid]; ok {
		return path, nil
	}
	dir, ok := uuidDirMap[uuid]
	if !ok {
		return "", fs.ErrorDirNotFound
	}
	parentPath, err := getPathForUUID(dir.GetParent(), uuidPathMap, uuidDirMap)
	if err != nil {
		return "", err
	}
	path := pathModule.Join(parentPath, dir.GetName())
	uuidPathMap[uuid] = path
	return path, nil
}

// About gets quota information from the Fs
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	userInfo, err := f.filen.GetUserInfo(ctx)
	if err != nil {
		return nil, err
	}

	total := int64(userInfo.MaxStorage)
	used := int64(userInfo.UsedStorage)
	free := total - used
	return &fs.Usage{
		Total:   &total,
		Used:    &used,
		Trashed: nil,
		Other:   nil,
		Free:    &free,
		Objects: nil,
	}, nil
}

// CleanUp the trash in the Fs
func (f *Fs) CleanUp(ctx context.Context) error {
	// not sure if this is implemented correctly, since this trashes ALL trash
	// not just the trash in the currently mounted fs
	// not currently wiping file versions because that feels dangerous
	// especially since versioning can be toggled on/off
	return f.filen.EmptyTrash(ctx)
}

// helpers

// resolvePath returns the absolute path specified by the input path, which is seen relative to the remote's root.
func (f *Fs) resolvePath(path string) string {
	return pathModule.Join(f.root.path, path)
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = &Fs{}
	_ fs.Mover           = &Fs{}
	_ fs.DirMover        = &Fs{}
	_ fs.Purger          = &Fs{}
	_ fs.PutStreamer     = &Fs{}
	_ fs.CleanUpper      = &Fs{}
	_ fs.ListRer         = &Fs{}
	_ fs.Abouter         = &Fs{}
	_ fs.OpenChunkWriter = &Fs{}
	_ fs.Directory       = &Directory{}
	_ fs.Object          = &Object{}
	_ fs.MimeTyper       = &Object{}
	_ fs.IDer            = &Object{}
	_ fs.ParentIDer      = &Object{}
	_ fs.ChunkWriter     = &chunkWriter{}
)

// todo PublicLinker,
// we could technically implement ChangeNotifier, but
// 1) the current implementation on Filen's side isn't great, it's worth waiting until SSE
// 2) I'm not really clear that the benefits are so great
// a bunch of the information would get wasted, since the Filen does actually specify exact updates,
// whereas rclone seems to only accept a path and object type
