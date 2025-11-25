//go:build !plan9 && !js

// Package azurefiles provides an interface to Microsoft Azure Files
package azurefiles

/*
   TODO

   This uses LastWriteTime which seems to work. The API return also
   has LastModified - needs investigation

   Needs pacer to have retries

   HTTP headers need to be passed

   Could support Metadata

   FIXME write mime type

   See FIXME markers

   Optional interfaces for Object
   - ID

*/

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"path"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azfile/directory"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azfile/file"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azfile/fileerror"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azfile/service"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azfile/share"
	"github.com/rclone/rclone/backend/azureblob/auth"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/list"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/readers"
)

const (
	maxFileSize           = 4 * fs.Tebi
	defaultChunkSize      = 4 * fs.Mebi
	storageDefaultBaseURL = "file.core.windows.net"
)

func init() {
	fs.Register(&fs.RegInfo{
		Name:        "azurefiles",
		Description: "Microsoft Azure Files",
		NewFs:       NewFs,
		Options: slices.Concat(auth.ConfigOptions, []fs.Option{{
			Name: "share_name",
			Help: `Azure Files Share Name.

This is required and is the name of the share to access.
`,
		}, {
			Name: "chunk_size",
			Help: `Upload chunk size.

Note that this is stored in memory and there may be up to
"--transfers" * "--azurefile-upload-concurrency" chunks stored at once
in memory.`,
			Default:  defaultChunkSize,
			Advanced: true,
		}, {
			Name: "upload_concurrency",
			Help: `Concurrency for multipart uploads.

This is the number of chunks of the same file that are uploaded
concurrently.

If you are uploading small numbers of large files over high-speed
links and these uploads do not fully utilize your bandwidth, then
increasing this may help to speed up the transfers.

Note that chunks are stored in memory and there may be up to
"--transfers" * "--azurefile-upload-concurrency" chunks stored at once
in memory.`,
			Default:  16,
			Advanced: true,
		}, {
			Name: "max_stream_size",
			Help: strings.ReplaceAll(`Max size for streamed files.

Azure files needs to know in advance how big the file will be. When
rclone doesn't know it uses this value instead.

This will be used when rclone is streaming data, the most common uses are:

- Uploading files with |--vfs-cache-mode off| with |rclone mount|
- Using |rclone rcat|
- Copying files with unknown length

You will need this much free space in the share as the file will be this size temporarily.
`, "|", "`"),
			Default:  10 * fs.Gibi,
			Advanced: true,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			Default: (encoder.EncodeDoubleQuote |
				encoder.EncodeBackSlash |
				encoder.EncodeSlash |
				encoder.EncodeColon |
				encoder.EncodePipe |
				encoder.EncodeLtGt |
				encoder.EncodeAsterisk |
				encoder.EncodeQuestion |
				encoder.EncodeInvalidUtf8 |
				encoder.EncodeCtl | encoder.EncodeDel |
				encoder.EncodeDot | encoder.EncodeRightPeriod),
		}}),
	})
}

// Options defines the configuration for this backend
type Options struct {
	auth.Options
	ShareName         string               `config:"share_name"`
	ChunkSize         fs.SizeSuffix        `config:"chunk_size"`
	MaxStreamSize     fs.SizeSuffix        `config:"max_stream_size"`
	UploadConcurrency int                  `config:"upload_concurrency"`
	Enc               encoder.MultiEncoder `config:"encoding"`
}

// Fs represents a root directory inside a share. The root directory can be ""
type Fs struct {
	name        string            // name of this remote
	root        string            // the path we are working on if any
	opt         Options           // parsed config options
	features    *fs.Features      // optional features
	shareClient *share.Client     // a client for the share itself
	svc         *directory.Client // the root service
}

// Object describes a Azure File Share File
type Object struct {
	fs          *Fs       // what this object is part of
	remote      string    // The remote path
	size        int64     // Size of the object
	md5         []byte    // MD5 hash if known
	modTime     time.Time // The modified time of the object if known
	contentType string    // content type if known
}

// Factored out from NewFs so that it can be tested with opt *Options and without m configmap.Mapper
func newFsFromOptions(ctx context.Context, name, root string, opt *Options) (fs.Fs, error) {
	conf := auth.NewClientOpts[service.Client, service.ClientOptions, service.SharedKeyCredential]{
		DefaultBaseURL:                   storageDefaultBaseURL,
		NewClient:                        service.NewClient,
		NewClientFromConnectionString:    service.NewClientFromConnectionString,
		NewClientWithNoCredential:        service.NewClientWithNoCredential,
		NewClientWithSharedKeyCredential: service.NewClientWithSharedKeyCredential,
		NewSharedKeyCredential:           service.NewSharedKeyCredential,
		SetClientOptions: func(options *service.ClientOptions, policyClientOptions policy.ClientOptions) {
			options.ClientOptions = policyClientOptions
		},
	}
	res, err := auth.NewClient(ctx, conf, &opt.Options)
	if err != nil {
		return nil, err
	}
	// f.svc = res.Client
	// f.cred = res.Cred
	// f.sharedKeyCred = res.SharedKeyCred
	// f.anonymous = res.Anonymous

	shareClient := res.Client.NewShareClient(opt.ShareName)
	svc := shareClient.NewRootDirectoryClient()
	f := &Fs{
		shareClient: shareClient,
		svc:         svc,
		name:        name,
		root:        root,
		opt:         *opt,
	}
	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
		PartialUploads:          true, // files are visible as they are being uploaded
		CaseInsensitive:         true,
		SlowHash:                true, // calling Hash() generally takes an extra transaction
		ReadMimeType:            true,
		WriteMimeType:           true,
	}).Fill(ctx, f)

	// Check whether a file exists at this location
	_, propsErr := f.fileClient("").GetProperties(ctx, nil)
	if propsErr == nil {
		f.root = path.Dir(root)
		return f, fs.ErrorIsFile
	}

	return f, nil
}

// NewFs constructs an Fs from the root
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	return newFsFromOptions(ctx, name, root, opt)
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
	return fmt.Sprintf("azurefiles root '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Precision return the precision of this Fs
//
// One second. FileREST API times are in RFC1123 which in the example shows a precision of seconds
// Source: https://learn.microsoft.com/en-us/rest/api/storageservices/representation-of-date-time-values-in-headers
func (f *Fs) Precision() time.Duration {
	return time.Second
}

// Hashes returns the supported hash sets.
//
// MD5: since it is listed as header in the response for file properties
// Source: https://learn.microsoft.com/en-us/rest/api/storageservices/get-file-properties
func (f *Fs) Hashes() hash.Set {
	return hash.NewHashSet(hash.MD5)
}

// Encode remote and turn it into an absolute path in the share
func (f *Fs) absPath(remote string) string {
	return f.opt.Enc.FromStandardPath(path.Join(f.root, remote))
}

// Make a directory client from the dir
func (f *Fs) dirClient(dir string) *directory.Client {
	return f.svc.NewSubdirectoryClient(f.absPath(dir))
}

// Make a file client from the remote
func (f *Fs) fileClient(remote string) *file.Client {
	return f.svc.NewFileClient(f.absPath(remote))
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
//
// Does not return ErrorIsDir when a directory exists instead of file. since the documentation
// for [rclone.fs.Fs.NewObject] rqeuires no extra work to determine whether it is directory
//
// This initiates a network request and returns an error if object is not found.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	resp, err := f.fileClient(remote).GetProperties(ctx, nil)
	if fileerror.HasCode(err, fileerror.ParentNotFound, fileerror.ResourceNotFound) {
		return nil, fs.ErrorObjectNotFound
	} else if err != nil {
		return nil, fmt.Errorf("unable to find object remote %q: %w", remote, err)
	}

	o := &Object{
		fs:     f,
		remote: remote,
	}
	o.setMetadata(&resp)
	return o, nil
}

// Make a directory using the absolute path from the root of the share
//
// This recursiely creating parent directories all the way to the root
// of the share.
func (f *Fs) absMkdir(ctx context.Context, absPath string) error {
	if absPath == "" {
		return nil
	}
	dirClient := f.svc.NewSubdirectoryClient(absPath)

	// now := time.Now()
	// smbProps := &file.SMBProperties{
	// 	LastWriteTime: &now,
	// }
	// dirCreateOptions := &directory.CreateOptions{
	// 	FileSMBProperties: smbProps,
	// }

	_, createDirErr := dirClient.Create(ctx, nil)
	if fileerror.HasCode(createDirErr, fileerror.ParentNotFound) {
		parentDir := path.Dir(absPath)
		if parentDir == absPath {
			return fmt.Errorf("internal error: infinite recursion since parent and remote are equal")
		}
		makeParentErr := f.absMkdir(ctx, parentDir)
		if makeParentErr != nil {
			return fmt.Errorf("could not make parent of %q: %w", absPath, makeParentErr)
		}
		return f.absMkdir(ctx, absPath)
	} else if fileerror.HasCode(createDirErr, fileerror.ResourceAlreadyExists) {
		return nil
	} else if createDirErr != nil {
		return fmt.Errorf("unable to MkDir: %w", createDirErr)
	}
	return nil
}

// Mkdir creates nested directories
func (f *Fs) Mkdir(ctx context.Context, remote string) error {
	return f.absMkdir(ctx, f.absPath(remote))
}

// Make the parent directory of remote
func (f *Fs) mkParentDir(ctx context.Context, remote string) error {
	// Can't make the parent of root
	if remote == "" {
		return nil
	}
	return f.Mkdir(ctx, path.Dir(remote))
}

// Rmdir deletes the root folder
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	dirClient := f.dirClient(dir)
	_, err := dirClient.Delete(ctx, nil)
	if err != nil {
		if fileerror.HasCode(err, fileerror.DirectoryNotEmpty) {
			return fs.ErrorDirectoryNotEmpty
		} else if fileerror.HasCode(err, fileerror.ResourceNotFound) {
			return fs.ErrorDirNotFound
		}
		return fmt.Errorf("could not rmdir dir %q: %w", dir, err)
	}
	return nil
}

// Put the object
//
// Copies the reader in to the new object. This new object is returned.
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

// List the objects and directories in dir into entries. The entries can be
// returned in any order but should be for a complete directory.
//
// dir should be "" to list the root, and should not have trailing slashes.
//
// This should return ErrDirNotFound if the directory isn't found.
func (f *Fs) List(ctx context.Context, dir string) (fs.DirEntries, error) {
	return list.WithListP(ctx, dir, f)
}

// ListP lists the objects and directories of the Fs starting
// from dir non recursively into out.
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
func (f *Fs) ListP(ctx context.Context, dir string, callback fs.ListRCallback) error {
	list := list.NewHelper(callback)
	subDirClient := f.dirClient(dir)

	// Checking whether directory exists
	_, err := subDirClient.GetProperties(ctx, nil)
	if fileerror.HasCode(err, fileerror.ParentNotFound, fileerror.ResourceNotFound) {
		return fs.ErrorDirNotFound
	} else if err != nil {
		return err
	}

	opt := &directory.ListFilesAndDirectoriesOptions{
		Include: directory.ListFilesInclude{
			Timestamps: true,
		},
	}
	pager := subDirClient.NewListFilesAndDirectoriesPager(opt)
	for pager.More() {
		resp, err := pager.NextPage(ctx)
		if err != nil {
			return err
		}
		for _, directory := range resp.Segment.Directories {
			// Name          *string `xml:"Name"`
			// Attributes    *string `xml:"Attributes"`
			// ID            *string `xml:"FileId"`
			// PermissionKey *string `xml:"PermissionKey"`
			// Properties.ContentLength  *int64       `xml:"Content-Length"`
			// Properties.ChangeTime     *time.Time   `xml:"ChangeTime"`
			// Properties.CreationTime   *time.Time   `xml:"CreationTime"`
			// Properties.ETag           *azcore.ETag `xml:"Etag"`
			// Properties.LastAccessTime *time.Time   `xml:"LastAccessTime"`
			// Properties.LastModified   *time.Time   `xml:"Last-Modified"`
			// Properties.LastWriteTime  *time.Time   `xml:"LastWriteTime"`
			var modTime time.Time
			if directory.Properties.LastWriteTime != nil {
				modTime = *directory.Properties.LastWriteTime
			}
			leaf := f.opt.Enc.ToStandardPath(*directory.Name)
			entry := fs.NewDir(path.Join(dir, leaf), modTime)
			if directory.ID != nil {
				entry.SetID(*directory.ID)
			}
			if directory.Properties.ContentLength != nil {
				entry.SetSize(*directory.Properties.ContentLength)
			}
			err = list.Add(entry)
			if err != nil {
				return err
			}
		}
		for _, file := range resp.Segment.Files {
			leaf := f.opt.Enc.ToStandardPath(*file.Name)
			entry := &Object{
				fs:     f,
				remote: path.Join(dir, leaf),
			}
			if file.Properties.ContentLength != nil {
				entry.size = *file.Properties.ContentLength
			}
			if file.Properties.LastWriteTime != nil {
				entry.modTime = *file.Properties.LastWriteTime
			}
			err = list.Add(entry)
			if err != nil {
				return err
			}
		}
	}
	return list.Flush()
}

// ------------------------------------------------------------

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Size of object in bytes
func (o *Object) Size() int64 {
	return o.size
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

// fileClient makes a specialized client for this object
func (o *Object) fileClient() *file.Client {
	return o.fs.fileClient(o.remote)
}

// set the metadata from file.GetPropertiesResponse
func (o *Object) setMetadata(resp *file.GetPropertiesResponse) {
	if resp.ContentLength != nil {
		o.size = *resp.ContentLength
	}
	o.md5 = resp.ContentMD5
	if resp.FileLastWriteTime != nil {
		o.modTime = *resp.FileLastWriteTime
	}
	if resp.ContentType != nil {
		o.contentType = *resp.ContentType
	}
}

// getMetadata gets the metadata if it hasn't already been fetched
func (o *Object) getMetadata(ctx context.Context) error {
	resp, err := o.fileClient().GetProperties(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to fetch properties: %w", err)
	}
	o.setMetadata(&resp)
	return nil
}

// Hash returns the MD5 of an object returning a lowercase hex string
//
// May make a network request because the [fs.List] method does not
// return MD5 hashes for DirEntry
func (o *Object) Hash(ctx context.Context, ty hash.Type) (string, error) {
	if ty != hash.MD5 {
		return "", hash.ErrUnsupported
	}
	if len(o.md5) == 0 {
		err := o.getMetadata(ctx)
		if err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(o.md5), nil
}

// MimeType returns the content type of the Object if
// known, or "" if not
func (o *Object) MimeType(ctx context.Context) string {
	if o.contentType == "" {
		err := o.getMetadata(ctx)
		if err != nil {
			fs.Errorf(o, "Failed to fetch Content-Type")
		}
	}
	return o.contentType
}

// Storable returns a boolean showing whether this object storable
func (o *Object) Storable() bool {
	return true
}

// ModTime returns the modification time of the object
//
// Returns time.Now() if not present
func (o *Object) ModTime(ctx context.Context) time.Time {
	if o.modTime.IsZero() {
		return time.Now()
	}
	return o.modTime
}

// SetModTime sets the modification time
func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
	opt := file.SetHTTPHeadersOptions{
		SMBProperties: &file.SMBProperties{
			LastWriteTime: &t,
		},
		HTTPHeaders: &file.HTTPHeaders{
			ContentMD5:  o.md5,
			ContentType: &o.contentType,
		},
	}
	_, err := o.fileClient().SetHTTPHeaders(ctx, &opt)
	if err != nil {
		return fmt.Errorf("unable to set modTime: %w", err)
	}
	o.modTime = t
	return nil
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	if _, err := o.fileClient().Delete(ctx, nil); err != nil {
		return fmt.Errorf("unable to delete remote %q: %w", o.remote, err)
	}
	return nil
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	// Offset and Count for range download
	var offset int64
	var count int64
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
		default:
			if option.Mandatory() {
				fs.Logf(o, "Unsupported mandatory option: %v", option)
			}
		}
	}
	opt := file.DownloadStreamOptions{
		Range: file.HTTPRange{
			Offset: offset,
			Count:  count,
		},
	}
	resp, err := o.fileClient().DownloadStream(ctx, &opt)
	if err != nil {
		return nil, fmt.Errorf("could not open remote %q: %w", o.remote, err)
	}
	return resp.Body, nil
}

// Returns a pointer to t - useful for returning pointers to constants
func ptr[T any](t T) *T {
	return &t
}

var warnStreamUpload sync.Once

// Update the object with the contents of the io.Reader, modTime, size and MD5 hash
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	var (
		size           = src.Size()
		sizeUnknown    = false
		hashUnknown    = true
		fc             = o.fileClient()
		isNewlyCreated = o.modTime.IsZero()
		counter        *readers.CountingReader
		md5Hash        []byte
		hasher         = md5.New()
	)

	if size > int64(maxFileSize) {
		return fmt.Errorf("update: max supported file size is %vB. provided size is %vB", maxFileSize, fs.SizeSuffix(size))
	} else if size < 0 {
		size = int64(o.fs.opt.MaxStreamSize)
		sizeUnknown = true
		warnStreamUpload.Do(func() {
			fs.Logf(o.fs, "Streaming uploads will have maximum file size of %v - adjust with --azurefiles-max-stream-size", o.fs.opt.MaxStreamSize)
		})
	}

	if isNewlyCreated {
		// Make parent directory
		if mkDirErr := o.fs.mkParentDir(ctx, src.Remote()); mkDirErr != nil {
			return fmt.Errorf("update: unable to make parent directories: %w", mkDirErr)
		}
		// Create the file at the size given
		if _, createErr := fc.Create(ctx, size, nil); createErr != nil {
			return fmt.Errorf("update: unable to create file: %w", createErr)
		}
	} else if size != o.Size() {
		// Resize the file if needed
		if _, resizeErr := fc.Resize(ctx, size, nil); resizeErr != nil {
			return fmt.Errorf("update: unable to resize while trying to update: %w ", resizeErr)
		}
	}

	// Measure the size if it is unknown
	if sizeUnknown {
		counter = readers.NewCountingReader(in)
		in = counter
	}

	// Check we have a source MD5 hash...
	if hashStr, err := src.Hash(ctx, hash.MD5); err == nil && hashStr != "" {
		md5Hash, err = hex.DecodeString(hashStr)
		if err == nil {
			hashUnknown = false
		} else {
			fs.Errorf(o, "internal error: decoding hex encoded md5 %q: %v", hashStr, err)
		}
	}

	// ...if not calculate one
	if hashUnknown {
		in = io.TeeReader(in, hasher)
	}

	// Upload the file
	opt := file.UploadStreamOptions{
		ChunkSize:   int64(o.fs.opt.ChunkSize),
		Concurrency: o.fs.opt.UploadConcurrency,
	}
	if err := fc.UploadStream(ctx, in, &opt); err != nil {
		// Remove partially uploaded file on error
		if isNewlyCreated {
			if _, delErr := fc.Delete(ctx, nil); delErr != nil {
				fs.Errorf(o, "failed to delete partially uploaded file: %v", delErr)
			}
		}
		return fmt.Errorf("update: failed to upload stream: %w", err)
	}

	if sizeUnknown {
		// Read the uploaded size - the file will be truncated to that size by updateSizeHashModTime
		size = int64(counter.BytesRead())
	}
	if hashUnknown {
		md5Hash = hasher.Sum(nil)
	}

	// Update the properties
	modTime := src.ModTime(ctx)
	contentType := fs.MimeType(ctx, src)
	httpHeaders := file.HTTPHeaders{
		ContentMD5:  md5Hash,
		ContentType: &contentType,
	}
	// Apply upload options (also allows one to overwrite content-type)
	for _, option := range options {
		key, value := option.Header()
		lowerKey := strings.ToLower(key)
		switch lowerKey {
		case "cache-control":
			httpHeaders.CacheControl = &value
		case "content-disposition":
			httpHeaders.ContentDisposition = &value
		case "content-encoding":
			httpHeaders.ContentEncoding = &value
		case "content-language":
			httpHeaders.ContentLanguage = &value
		case "content-type":
			httpHeaders.ContentType = &value
		}
	}
	_, err = fc.SetHTTPHeaders(ctx, &file.SetHTTPHeadersOptions{
		FileContentLength: &size,
		SMBProperties: &file.SMBProperties{
			LastWriteTime: &modTime,
		},
		HTTPHeaders: &httpHeaders,
	})
	if err != nil {
		return fmt.Errorf("update: failed to set properties: %w", err)
	}

	// Make sure Object is in sync
	o.size = size
	o.md5 = md5Hash
	o.modTime = modTime
	o.contentType = contentType
	return nil
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
	err := f.mkParentDir(ctx, remote)
	if err != nil {
		return nil, fmt.Errorf("Move: mkParentDir failed: %w", err)
	}
	opt := file.RenameOptions{
		IgnoreReadOnly:  ptr(true),
		ReplaceIfExists: ptr(true),
	}
	dstAbsPath := f.absPath(remote)
	fc := srcObj.fileClient()
	_, err = fc.Rename(ctx, dstAbsPath, &opt)
	if err != nil {
		return nil, fmt.Errorf("Move: Rename failed: %w", err)
	}
	dstObj, err := f.NewObject(ctx, remote)
	if err != nil {
		return nil, fmt.Errorf("Move: NewObject failed: %w", err)
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
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) error {
	dstFs := f
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debugf(srcFs, "Can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}

	_, err := dstFs.dirClient(dstRemote).GetProperties(ctx, nil)
	if err == nil {
		return fs.ErrorDirExists
	}
	if !fileerror.HasCode(err, fileerror.ParentNotFound, fileerror.ResourceNotFound) {
		return fmt.Errorf("DirMove: failed to get status of destination directory: %w", err)
	}

	err = dstFs.mkParentDir(ctx, dstRemote)
	if err != nil {
		return fmt.Errorf("DirMove: mkParentDir failed: %w", err)
	}

	opt := directory.RenameOptions{
		IgnoreReadOnly:  ptr(false),
		ReplaceIfExists: ptr(false),
	}
	dstAbsPath := dstFs.absPath(dstRemote)
	dirClient := srcFs.dirClient(srcRemote)
	_, err = dirClient.Rename(ctx, dstAbsPath, &opt)
	if err != nil {
		if fileerror.HasCode(err, fileerror.ResourceAlreadyExists) {
			return fs.ErrorDirExists
		}
		return fmt.Errorf("DirMove: Rename failed: %w", err)
	}
	return nil
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
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}
	err := f.mkParentDir(ctx, remote)
	if err != nil {
		return nil, fmt.Errorf("Copy: mkParentDir failed: %w", err)
	}
	opt := file.StartCopyFromURLOptions{
		CopyFileSMBInfo: &file.CopyFileSMBInfo{
			Attributes:         file.SourceCopyFileAttributes{},
			ChangeTime:         file.SourceCopyFileChangeTime{},
			CreationTime:       file.SourceCopyFileCreationTime{},
			LastWriteTime:      file.SourceCopyFileLastWriteTime{},
			PermissionCopyMode: ptr(file.PermissionCopyModeTypeSource),
			IgnoreReadOnly:     ptr(true),
		},
	}
	srcURL := srcObj.fileClient().URL()
	fc := f.fileClient(remote)
	startCopy, err := fc.StartCopyFromURL(ctx, srcURL, &opt)
	if err != nil {
		return nil, fmt.Errorf("Copy failed: %w", err)
	}

	// Poll for completion if necessary
	//
	// The for loop is never executed for same storage account copies.
	copyStatus := startCopy.CopyStatus
	var properties file.GetPropertiesResponse
	pollTime := 100 * time.Millisecond

	for copyStatus != nil && string(*copyStatus) == string(file.CopyStatusTypePending) {
		time.Sleep(pollTime)

		properties, err = fc.GetProperties(ctx, &file.GetPropertiesOptions{})
		if err != nil {
			return nil, err
		}
		copyStatus = properties.CopyStatus
		pollTime = min(2*pollTime, time.Second)
	}

	dstObj, err := f.NewObject(ctx, remote)
	if err != nil {
		return nil, fmt.Errorf("Copy: NewObject failed: %w", err)
	}
	return dstObj, nil
}

// Implementation of WriterAt
type writerAt struct {
	ctx  context.Context
	f    *Fs
	fc   *file.Client
	mu   sync.Mutex // protects variables below
	size int64
}

// Adaptor to add a Close method to bytes.Reader
type bytesReaderCloser struct {
	*bytes.Reader
}

// Close the bytesReaderCloser
func (bytesReaderCloser) Close() error {
	return nil
}

// WriteAt writes len(p) bytes from p to the underlying data stream
// at offset off. It returns the number of bytes written from p (0 <= n <= len(p))
// and any error encountered that caused the write to stop early.
// WriteAt must return a non-nil error if it returns n < len(p).
//
// If WriteAt is writing to a destination with a seek offset,
// WriteAt should not affect nor be affected by the underlying
// seek offset.
//
// Clients of WriteAt can execute parallel WriteAt calls on the same
// destination if the ranges do not overlap.
//
// Implementations must not retain p.
func (w *writerAt) WriteAt(p []byte, off int64) (n int, err error) {
	endOffset := off + int64(len(p))
	w.mu.Lock()
	if w.size < endOffset {
		_, err = w.fc.Resize(w.ctx, endOffset, nil)
		if err != nil {
			w.mu.Unlock()
			return 0, fmt.Errorf("WriteAt: failed to resize file: %w ", err)
		}
		w.size = endOffset
	}
	w.mu.Unlock()

	in := bytesReaderCloser{bytes.NewReader(p)}
	_, err = w.fc.UploadRange(w.ctx, off, in, nil)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

// Close the writer
func (w *writerAt) Close() error {
	// FIXME should we be doing something here?
	return nil
}

// OpenWriterAt opens with a handle for random access writes
//
// Pass in the remote desired and the size if known.
//
// It truncates any existing object
func (f *Fs) OpenWriterAt(ctx context.Context, remote string, size int64) (fs.WriterAtCloser, error) {
	err := f.mkParentDir(ctx, remote)
	if err != nil {
		return nil, fmt.Errorf("OpenWriterAt: failed to create parent directory: %w", err)
	}
	fc := f.fileClient(remote)
	if size < 0 {
		size = 0
	}
	_, err = fc.Create(ctx, size, nil)
	if err != nil {
		return nil, fmt.Errorf("OpenWriterAt: unable to create file: %w", err)
	}
	w := &writerAt{
		ctx:  ctx,
		f:    f,
		fc:   fc,
		size: size,
	}
	return w, nil
}

// About gets quota information
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	stats, err := f.shareClient.GetStatistics(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to read share statistics: %w", err)
	}
	usage := &fs.Usage{
		Used: stats.ShareUsageBytes, // bytes in use
	}
	return usage, nil
}

// Check the interfaces are satisfied
var (
	_ fs.Fs             = &Fs{}
	_ fs.PutStreamer    = &Fs{}
	_ fs.Abouter        = &Fs{}
	_ fs.Mover          = &Fs{}
	_ fs.DirMover       = &Fs{}
	_ fs.Copier         = &Fs{}
	_ fs.OpenWriterAter = &Fs{}
	_ fs.ListPer        = &Fs{}
	_ fs.Object         = &Object{}
	_ fs.MimeTyper      = &Object{}
)
