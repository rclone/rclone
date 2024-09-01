// Package filescom provides an interface to the Files.com
// object storage system.
package filescom

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	files_sdk "github.com/Files-com/files-sdk-go/v3"
	"github.com/Files-com/files-sdk-go/v3/bundle"
	"github.com/Files-com/files-sdk-go/v3/file"
	file_migration "github.com/Files-com/files-sdk-go/v3/filemigration"
	"github.com/Files-com/files-sdk-go/v3/folder"
	"github.com/Files-com/files-sdk-go/v3/session"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/pacer"
)

/*
Run of rclone info
stringNeedsEscaping = []rune{
        '/', '\x00'
}
maxFileLength = 512 // for 1 byte unicode characters
maxFileLength = 512 // for 2 byte unicode characters
maxFileLength = 512 // for 3 byte unicode characters
maxFileLength = 512 // for 4 byte unicode characters
canWriteUnnormalized = true
canReadUnnormalized   = true
canReadRenormalized   = true
canStream = true
*/

const (
	minSleep      = 10 * time.Millisecond
	maxSleep      = 2 * time.Second
	decayConstant = 2 // bigger for slower decay, exponential

	folderNotEmpty = "processing-failure/folder-not-empty"
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "filescom",
		Description: "Files.com",
		NewFs:       NewFs,
		Options: []fs.Option{
			{
				Name: "site",
				Help: "Your site subdomain (e.g. mysite) or custom domain (e.g. myfiles.customdomain.com).",
			}, {
				Name: "username",
				Help: "The username used to authenticate with Files.com.",
			}, {
				Name:       "password",
				Help:       "The password used to authenticate with Files.com.",
				IsPassword: true,
			}, {
				Name:      "api_key",
				Help:      "The API key used to authenticate with Files.com.",
				Advanced:  true,
				Sensitive: true,
			}, {
				Name:     config.ConfigEncoding,
				Help:     config.ConfigEncodingHelp,
				Advanced: true,
				Default: (encoder.Display |
					encoder.EncodeBackSlash |
					encoder.EncodeRightSpace |
					encoder.EncodeRightCrLfHtVt |
					encoder.EncodeInvalidUtf8),
			}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	Site     string               `config:"site"`
	Username string               `config:"username"`
	Password string               `config:"password"`
	APIKey   string               `config:"api_key"`
	Enc      encoder.MultiEncoder `config:"encoding"`
}

// Fs represents a remote files.com server
type Fs struct {
	name            string                 // name of this remote
	root            string                 // the path we are working on
	opt             Options                // parsed options
	features        *fs.Features           // optional features
	fileClient      *file.Client           // the connection to the file API
	folderClient    *folder.Client         // the connection to the folder API
	migrationClient *file_migration.Client // the connection to the file migration API
	bundleClient    *bundle.Client         // the connection to the bundle API
	pacer           *fs.Pacer              // pacer for API calls
}

// Object describes a files object
//
// Will definitely have info but maybe not meta
type Object struct {
	fs       *Fs       // what this object is part of
	remote   string    // The remote path
	size     int64     // size of the object
	crc32    string    // CRC32 of the object content
	md5      string    // MD5 of the object content
	mimeType string    // Content-Type of the object
	modTime  time.Time // modification time of the object
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
	return fmt.Sprintf("files root '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Encode remote and turn it into an absolute path in the share
func (f *Fs) absPath(remote string) string {
	return f.opt.Enc.FromStandardPath(path.Join(f.root, remote))
}

// retryErrorCodes is a slice of error codes that we will retry
var retryErrorCodes = []int{
	429, // Too Many Requests.
	500, // Internal Server Error
	502, // Bad Gateway
	503, // Service Unavailable
	504, // Gateway Timeout
	509, // Bandwidth Limit Exceeded
}

// shouldRetry returns a boolean as to whether this err deserves to be
// retried.  It returns the err as a convenience
func shouldRetry(ctx context.Context, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}

	if apiErr, ok := err.(files_sdk.ResponseError); ok {
		for _, e := range retryErrorCodes {
			if apiErr.HttpCode == e {
				fs.Debugf(nil, "Retrying API error %v", err)
				return true, err
			}
		}
	}

	return fserrors.ShouldRetry(err), err
}

// readMetaDataForPath reads the metadata from the path
func (f *Fs) readMetaDataForPath(ctx context.Context, path string) (info *files_sdk.File, err error) {
	params := files_sdk.FileFindParams{
		Path: f.absPath(path),
	}

	var file files_sdk.File
	err = f.pacer.Call(func() (bool, error) {
		file, err = f.fileClient.Find(params, files_sdk.WithContext(ctx))
		return shouldRetry(ctx, err)
	})
	if err != nil {
		return nil, err
	}

	return &file, nil
}

// NewFs constructs an Fs from the path, container:path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	root = strings.Trim(root, "/")

	config, err := newClientConfig(ctx, opt)
	if err != nil {
		return nil, err
	}

	f := &Fs{
		name:            name,
		root:            root,
		opt:             *opt,
		fileClient:      &file.Client{Config: config},
		folderClient:    &folder.Client{Config: config},
		migrationClient: &file_migration.Client{Config: config},
		bundleClient:    &bundle.Client{Config: config},
		pacer:           fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
	}
	f.features = (&fs.Features{
		CaseInsensitive:          true,
		CanHaveEmptyDirectories:  true,
		ReadMimeType:             true,
		DirModTimeUpdatesOnWrite: true,
	}).Fill(ctx, f)

	if f.root != "" {
		info, err := f.readMetaDataForPath(ctx, "")
		if err == nil && !info.IsDir() {
			f.root = path.Dir(f.root)
			if f.root == "." {
				f.root = ""
			}
			return f, fs.ErrorIsFile
		}
	}

	return f, err
}

func newClientConfig(ctx context.Context, opt *Options) (config files_sdk.Config, err error) {
	if opt.Site != "" {
		if strings.Contains(opt.Site, ".") {
			config.EndpointOverride = opt.Site
		} else {
			config.Subdomain = opt.Site
		}

		_, err = url.ParseRequestURI(config.Endpoint())
		if err != nil {
			err = fmt.Errorf("invalid domain or subdomain: %v", opt.Site)
			return
		}
	}

	config = config.Init().SetCustomClient(fshttp.NewClient(ctx))

	if opt.APIKey != "" {
		config.APIKey = opt.APIKey
		return
	}

	if opt.Username == "" {
		err = errors.New("username not found")
		return
	}
	if opt.Password == "" {
		err = errors.New("password not found")
		return
	}
	opt.Password, err = obscure.Reveal(opt.Password)
	if err != nil {
		return
	}

	sessionClient := session.Client{Config: config}
	params := files_sdk.SessionCreateParams{
		Username: opt.Username,
		Password: opt.Password,
	}

	thisSession, err := sessionClient.Create(params, files_sdk.WithContext(ctx))
	if err != nil {
		err = fmt.Errorf("couldn't create session: %w", err)
		return
	}

	config.SessionId = thisSession.Id
	return
}

// Return an Object from a path
//
// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(ctx context.Context, remote string, file *files_sdk.File) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	var err error
	if file != nil {
		err = o.setMetaData(file)
	} else {
		err = o.readMetaData(ctx) // reads info and meta, returning an error
	}
	if err != nil {
		return nil, err
	}
	return o, nil
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	return f.newObjectWithInfo(ctx, remote, nil)
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
	var it *folder.Iter
	params := files_sdk.FolderListForParams{
		Path: f.absPath(dir),
	}

	err = f.pacer.Call(func() (bool, error) {
		it, err = f.folderClient.ListFor(params, files_sdk.WithContext(ctx))
		return shouldRetry(ctx, err)
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't list files: %w", err)
	}

	for it.Next() {
		item := ptr(it.File())
		remote := f.opt.Enc.ToStandardPath(item.DisplayName)
		remote = path.Join(dir, remote)
		if remote == dir {
			continue
		}

		if item.IsDir() {
			d := fs.NewDir(remote, item.ModTime())
			entries = append(entries, d)
		} else {
			o, err := f.newObjectWithInfo(ctx, remote, item)
			if err != nil {
				return nil, err
			}
			entries = append(entries, o)
		}
	}
	err = it.Err()
	if files_sdk.IsNotExist(err) {
		return nil, fs.ErrorDirNotFound
	}
	return
}

// Creates from the parameters passed in a half finished Object which
// must have setMetaData called on it
//
// Returns the object and error.
//
// Used to create new objects
func (f *Fs) createObject(ctx context.Context, remote string) (o *Object, err error) {
	// Create the directory for the object if it doesn't exist
	err = f.mkParentDir(ctx, remote)
	if err != nil {
		return
	}
	// Temporary Object under construction
	o = &Object{
		fs:     f,
		remote: remote,
	}
	return o, nil
}

// Put the object
//
// Copy the reader in to the new object which is returned.
//
// The new object may have been created if an error is returned
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	// Temporary Object under construction
	fs := &Object{
		fs:     f,
		remote: src.Remote(),
	}
	return fs, fs.Update(ctx, in, src, options...)
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.Put(ctx, in, src, options...)
}

func (f *Fs) mkdir(ctx context.Context, path string) error {
	if path == "" || path == "." {
		return nil
	}

	params := files_sdk.FolderCreateParams{
		Path:         path,
		MkdirParents: ptr(true),
	}

	err := f.pacer.Call(func() (bool, error) {
		_, err := f.folderClient.Create(params, files_sdk.WithContext(ctx))
		return shouldRetry(ctx, err)
	})
	if files_sdk.IsExist(err) {
		return nil
	}
	return err
}

// Make the parent directory of remote
func (f *Fs) mkParentDir(ctx context.Context, remote string) error {
	return f.mkdir(ctx, path.Dir(f.absPath(remote)))
}

// Mkdir creates the container if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	return f.mkdir(ctx, f.absPath(dir))
}

// DirSetModTime sets the directory modtime for dir
func (f *Fs) DirSetModTime(ctx context.Context, dir string, modTime time.Time) error {
	o := Object{
		fs:     f,
		remote: dir,
	}
	return o.SetModTime(ctx, modTime)
}

// purgeCheck removes the root directory, if check is set then it
// refuses to do so if it has anything in
func (f *Fs) purgeCheck(ctx context.Context, dir string, check bool) error {
	path := f.absPath(dir)
	if path == "" || path == "." {
		return errors.New("can't purge root directory")
	}

	params := files_sdk.FileDeleteParams{
		Path:      path,
		Recursive: ptr(!check),
	}

	err := f.pacer.Call(func() (bool, error) {
		err := f.fileClient.Delete(params, files_sdk.WithContext(ctx))
		// Allow for eventual consistency deletion of child objects.
		if isFolderNotEmpty(err) {
			return true, err
		}
		return shouldRetry(ctx, err)
	})
	if err != nil {
		if files_sdk.IsNotExist(err) {
			return fs.ErrorDirNotFound
		} else if isFolderNotEmpty(err) {
			return fs.ErrorDirectoryNotEmpty
		}

		return fmt.Errorf("rmdir failed: %w", err)
	}
	return nil
}

// Rmdir deletes the root folder
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir, true)
}

// Precision return the precision of this Fs
func (f *Fs) Precision() time.Duration {
	return time.Second
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
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (dstObj fs.Object, err error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}
	err = srcObj.readMetaData(ctx)
	if err != nil {
		return
	}

	srcPath := srcObj.fs.absPath(srcObj.remote)
	dstPath := f.absPath(remote)
	if strings.EqualFold(srcPath, dstPath) {
		return nil, fmt.Errorf("can't copy %q -> %q as are same name when lowercase", srcPath, dstPath)
	}

	// Create temporary object
	dstObj, err = f.createObject(ctx, remote)
	if err != nil {
		return
	}

	// Copy the object
	params := files_sdk.FileCopyParams{
		Path:        srcPath,
		Destination: dstPath,
		Overwrite:   ptr(true),
	}

	var action files_sdk.FileAction
	err = f.pacer.Call(func() (bool, error) {
		action, err = f.fileClient.Copy(params, files_sdk.WithContext(ctx))
		return shouldRetry(ctx, err)
	})
	if err != nil {
		return
	}

	err = f.waitForAction(ctx, action, "copy")
	if err != nil {
		return
	}

	err = dstObj.SetModTime(ctx, srcObj.modTime)
	return
}

// Purge deletes all the files and the container
//
// Optional interface: Only implement this if you have a way of
// deleting all the files quicker than just running Remove() on the
// result of List()
func (f *Fs) Purge(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir, false)
}

// move a file or folder
func (f *Fs) move(ctx context.Context, src *Fs, srcRemote string, dstRemote string) (info *files_sdk.File, err error) {
	// Move the object
	params := files_sdk.FileMoveParams{
		Path:        src.absPath(srcRemote),
		Destination: f.absPath(dstRemote),
	}

	var action files_sdk.FileAction
	err = f.pacer.Call(func() (bool, error) {
		action, err = f.fileClient.Move(params, files_sdk.WithContext(ctx))
		return shouldRetry(ctx, err)
	})
	if err != nil {
		return nil, err
	}

	err = f.waitForAction(ctx, action, "move")
	if err != nil {
		return nil, err
	}

	info, err = f.readMetaDataForPath(ctx, dstRemote)
	return
}

func (f *Fs) waitForAction(ctx context.Context, action files_sdk.FileAction, operation string) (err error) {
	var migration files_sdk.FileMigration
	err = f.pacer.Call(func() (bool, error) {
		migration, err = f.migrationClient.Wait(action, func(migration files_sdk.FileMigration) {
			// noop
		}, files_sdk.WithContext(ctx))
		return shouldRetry(ctx, err)
	})
	if err == nil && migration.Status != "completed" {
		return fmt.Errorf("%v did not complete successfully: %v", operation, migration.Status)
	}
	return
}

// Move src to this remote using server-side move operations.
//
// This is stored with the remote path given.
//
// It returns the destination Object and a possible error.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantMove
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't move - not same remote type")
		return nil, fs.ErrorCantMove
	}

	// Create temporary object
	dstObj, err := f.createObject(ctx, remote)
	if err != nil {
		return nil, err
	}

	// Do the move
	info, err := f.move(ctx, srcObj.fs, srcObj.remote, dstObj.remote)
	if err != nil {
		return nil, err
	}

	err = dstObj.setMetaData(info)
	if err != nil {
		return nil, err
	}
	return dstObj, nil
}

// DirMove moves src, srcRemote to this remote at dstRemote
// using server-side move operations.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantDirMove
//
// If destination exists then return fs.ErrorDirExists
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) (err error) {
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debugf(srcFs, "Can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}

	// Check if destination exists
	_, err = f.readMetaDataForPath(ctx, dstRemote)
	if err == nil {
		return fs.ErrorDirExists
	}

	// Create temporary object
	dstObj, err := f.createObject(ctx, dstRemote)
	if err != nil {
		return
	}

	// Do the move
	_, err = f.move(ctx, srcFs, srcRemote, dstObj.remote)
	return
}

// PublicLink adds a "readable by anyone with link" permission on the given file or folder.
func (f *Fs) PublicLink(ctx context.Context, remote string, expire fs.Duration, unlink bool) (url string, err error) {
	params := files_sdk.BundleCreateParams{
		Paths: []string{f.absPath(remote)},
	}
	if expire < fs.DurationOff {
		params.ExpiresAt = ptr(time.Now().Add(time.Duration(expire)))
	}

	var bundle files_sdk.Bundle
	err = f.pacer.Call(func() (bool, error) {
		bundle, err = f.bundleClient.Create(params, files_sdk.WithContext(ctx))
		return shouldRetry(ctx, err)
	})

	url = bundle.Url
	return
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.NewHashSet(hash.CRC32, hash.MD5)
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
	return o.remote
}

// Remote returns the remote path
func (o *Object) Remote() string {
	return o.remote
}

// Hash returns the MD5 of an object returning a lowercase hex string
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	switch t {
	case hash.CRC32:
		if o.crc32 == "" {
			return "", nil
		}
		return fmt.Sprintf("%08s", o.crc32), nil
	case hash.MD5:
		return o.md5, nil
	}
	return "", hash.ErrUnsupported
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	return o.size
}

// setMetaData sets the metadata from info
func (o *Object) setMetaData(file *files_sdk.File) error {
	o.modTime = file.ModTime()

	if !file.IsDir() {
		o.size = file.Size
		o.crc32 = file.Crc32
		o.md5 = file.Md5
		o.mimeType = file.MimeType
	}

	return nil
}

// readMetaData gets the metadata if it hasn't already been fetched
//
// it also sets the info
func (o *Object) readMetaData(ctx context.Context) (err error) {
	file, err := o.fs.readMetaDataForPath(ctx, o.remote)
	if err != nil {
		if files_sdk.IsNotExist(err) {
			return fs.ErrorObjectNotFound
		}
		return err
	}
	if file.IsDir() {
		return fs.ErrorIsDir
	}
	return o.setMetaData(file)
}

// ModTime returns the modification time of the object
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.modTime
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) (err error) {
	params := files_sdk.FileUpdateParams{
		Path:          o.fs.absPath(o.remote),
		ProvidedMtime: &modTime,
	}

	var file files_sdk.File
	err = o.fs.pacer.Call(func() (bool, error) {
		file, err = o.fs.fileClient.Update(params, files_sdk.WithContext(ctx))
		return shouldRetry(ctx, err)
	})
	if err != nil {
		return err
	}
	return o.setMetaData(&file)
}

// Storable returns a boolean showing whether this object storable
func (o *Object) Storable() bool {
	return true
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	// Offset and Count for range download
	var offset, count int64
	fs.FixRangeOption(options, o.size)
	for _, option := range options {
		switch x := option.(type) {
		case *fs.RangeOption:
			offset, count = x.Decode(o.size)
			if count < 0 {
				count = o.size - offset
			}
		case *fs.SeekOption:
			offset = x.Offset
			count = o.size - offset
		default:
			if option.Mandatory() {
				fs.Logf(o, "Unsupported mandatory option: %v", option)
			}
		}
	}

	params := files_sdk.FileDownloadParams{
		Path: o.fs.absPath(o.remote),
	}

	headers := &http.Header{}
	headers.Set("Range", fmt.Sprintf("bytes=%v-%v", offset, offset+count-1))
	err = o.fs.pacer.Call(func() (bool, error) {
		_, err = o.fs.fileClient.Download(
			params,
			files_sdk.WithContext(ctx),
			files_sdk.RequestHeadersOption(headers),
			files_sdk.ResponseBodyOption(func(closer io.ReadCloser) error {
				in = closer
				return err
			}),
		)
		return shouldRetry(ctx, err)
	})
	return
}

// Returns a pointer to t - useful for returning pointers to constants
func ptr[T any](t T) *T {
	return &t
}

func isFolderNotEmpty(err error) bool {
	var re files_sdk.ResponseError
	ok := errors.As(err, &re)
	return ok && re.Type == folderNotEmpty
}

// Update the object with the contents of the io.Reader, modTime and size
//
// If existing is set then it updates the object rather than creating a new one.
//
// The new object may have been created if an error is returned.
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	uploadOpts := []file.UploadOption{
		file.UploadWithContext(ctx),
		file.UploadWithReader(in),
		file.UploadWithDestinationPath(o.fs.absPath(o.remote)),
		file.UploadWithProvidedMtime(src.ModTime(ctx)),
	}

	err := o.fs.pacer.Call(func() (bool, error) {
		err := o.fs.fileClient.Upload(uploadOpts...)
		return shouldRetry(ctx, err)
	})
	if err != nil {
		return err
	}

	return o.readMetaData(ctx)
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	params := files_sdk.FileDeleteParams{
		Path: o.fs.absPath(o.remote),
	}

	return o.fs.pacer.Call(func() (bool, error) {
		err := o.fs.fileClient.Delete(params, files_sdk.WithContext(ctx))
		return shouldRetry(ctx, err)
	})
}

// MimeType of an Object if known, "" otherwise
func (o *Object) MimeType(ctx context.Context) string {
	return o.mimeType
}

// Check the interfaces are satisfied
var (
	_ fs.Fs           = (*Fs)(nil)
	_ fs.Purger       = (*Fs)(nil)
	_ fs.PutStreamer  = (*Fs)(nil)
	_ fs.Copier       = (*Fs)(nil)
	_ fs.Mover        = (*Fs)(nil)
	_ fs.DirMover     = (*Fs)(nil)
	_ fs.PublicLinker = (*Fs)(nil)
	_ fs.Object       = (*Object)(nil)
	_ fs.MimeTyper    = (*Object)(nil)
)
