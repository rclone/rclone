// Package hidrive provides an interface to the HiDrive object storage system.
package hidrive

// FIXME HiDrive only supports file or folder names of 255 characters or less.
// Operations that create files or folders with longer names will throw an HTTP error:
// - 422 Unprocessable Entity
// A more graceful way for rclone to handle this may be desirable.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"
	"time"

	"github.com/rclone/rclone/lib/encoder"

	"github.com/rclone/rclone/backend/hidrive/api"
	"github.com/rclone/rclone/backend/hidrive/hidrivehash"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/oauthutil"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
	"golang.org/x/oauth2"
)

const (
	rcloneClientID              = "6b0258fdda630d34db68a3ce3cbf19ae"
	rcloneEncryptedClientSecret = "GC7UDZ3Ra4jLcmfQSagKCDJ1JEy-mU6pBBhFrS3tDEHILrK7j3TQHUrglkO5SgZ_"
	minSleep                    = 10 * time.Millisecond
	maxSleep                    = 2 * time.Second
	decayConstant               = 2 // bigger for slower decay, exponential
	defaultUploadChunkSize      = 48 * fs.Mebi
	defaultUploadCutoff         = 2 * defaultUploadChunkSize
	defaultUploadConcurrency    = 4
)

// Globals
var (
	// Description of how to auth for this app.
	oauthConfig = &oauth2.Config{
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://my.hidrive.com/client/authorize",
			TokenURL: "https://my.hidrive.com/oauth2/token",
		},
		ClientID:     rcloneClientID,
		ClientSecret: obscure.MustReveal(rcloneEncryptedClientSecret),
		RedirectURL:  oauthutil.TitleBarRedirectURL,
	}
	// hidrivehashType is the hash.Type for HiDrive hashes.
	hidrivehashType hash.Type
)

// Register the backend with Fs.
func init() {
	hidrivehashType = hash.RegisterHash("hidrive", "HiDriveHash", 40, hidrivehash.New)
	fs.Register(&fs.RegInfo{
		Name:        "hidrive",
		Description: "HiDrive",
		NewFs:       NewFs,
		Config: func(ctx context.Context, name string, m configmap.Mapper, config fs.ConfigIn) (*fs.ConfigOut, error) {
			// Parse config into Options struct
			opt := new(Options)
			err := configstruct.Set(m, opt)
			if err != nil {
				return nil, fmt.Errorf("couldn't parse config into struct: %w", err)
			}

			//fs.Debugf(nil, "hidrive: configuring oauth-token.")
			oauthConfig.Scopes = createHiDriveScopes(opt.ScopeRole, opt.ScopeAccess)
			return oauthutil.ConfigOut("", &oauthutil.Options{
				OAuth2Config: oauthConfig,
			})
		},
		Options: append(oauthutil.SharedOptions, []fs.Option{{
			Name:    "scope_access",
			Help:    "Access permissions that rclone should use when requesting access from HiDrive.",
			Default: "rw",
			Examples: []fs.OptionExample{{
				Value: "rw",
				Help:  "Read and write access to resources.",
			}, {
				Value: "ro",
				Help:  "Read-only access to resources.",
			}},
		}, {
			Name:    "scope_role",
			Help:    "User-level that rclone should use when requesting access from HiDrive.",
			Default: "user",
			Examples: []fs.OptionExample{{
				Value: "user",
				Help: `User-level access to management permissions.
This will be sufficient in most cases.`,
			}, {
				Value: "admin",
				Help:  "Extensive access to management permissions.",
			}, {
				Value: "owner",
				Help:  "Full access to management permissions.",
			}},
			Advanced: true,
		}, {
			Name: "root_prefix",
			Help: `The root/parent folder for all paths.

Fill in to use the specified folder as the parent for all paths given to the remote.
This way rclone can use any folder as its starting point.`,
			Default: "/",
			Examples: []fs.OptionExample{{
				Value: "/",
				Help: `The topmost directory accessible by rclone.
This will be equivalent with "root" if rclone uses a regular HiDrive user account.`,
			}, {
				Value: "root",
				Help:  `The topmost directory of the HiDrive user account`,
			}, {
				Value: "",
				Help: `This specifies that there is no root-prefix for your paths.
When using this you will always need to specify paths to this remote with a valid parent e.g. "remote:/path/to/dir" or "remote:root/path/to/dir".`,
			}},
			Advanced: true,
		}, {
			Name: "endpoint",
			Help: `Endpoint for the service.

This is the URL that API-calls will be made to.`,
			Default:  "https://api.hidrive.strato.com/2.1",
			Advanced: true,
		}, {
			Name: "disable_fetching_member_count",
			Help: `Do not fetch number of objects in directories unless it is absolutely necessary.

Requests may be faster if the number of objects in subdirectories is not fetched.`,
			Default:  false,
			Advanced: true,
		}, {
			Name: "chunk_size",
			Help: fmt.Sprintf(`Chunksize for chunked uploads.

Any files larger than the configured cutoff (or files of unknown size) will be uploaded in chunks of this size.

The upper limit for this is %v bytes (about %v).
That is the maximum amount of bytes a single upload-operation will support.
Setting this above the upper limit or to a negative value will cause uploads to fail.

Setting this to larger values may increase the upload speed at the cost of using more memory.
It can be set to smaller values smaller to save on memory.`, MaximumUploadBytes, fs.SizeSuffix(MaximumUploadBytes)),
			Default:  defaultUploadChunkSize,
			Advanced: true,
		}, {
			Name: "upload_cutoff",
			Help: fmt.Sprintf(`Cutoff/Threshold for chunked uploads.

Any files larger than this will be uploaded in chunks of the configured chunksize.

The upper limit for this is %v bytes (about %v).
That is the maximum amount of bytes a single upload-operation will support.
Setting this above the upper limit will cause uploads to fail.`, MaximumUploadBytes, fs.SizeSuffix(MaximumUploadBytes)),
			Default:  defaultUploadCutoff,
			Advanced: true,
		}, {
			Name: "upload_concurrency",
			Help: `Concurrency for chunked uploads.

This is the upper limit for how many transfers for the same file are running concurrently.
Setting this above to a value smaller than 1 will cause uploads to deadlock.

If you are uploading small numbers of large files over high-speed links
and these uploads do not fully utilize your bandwidth, then increasing
this may help to speed up the transfers.`,
			Default:  defaultUploadConcurrency,
			Advanced: true,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			// HiDrive only supports file or folder names of 255 characters or less.
			// Names containing "/" are not supported.
			// The special names "." and ".." are not supported.
			Default: (encoder.EncodeZero |
				encoder.EncodeSlash |
				encoder.EncodeDot),
		}}...),
	})
}

// Options defines the configuration for this backend.
type Options struct {
	EndpointAPI                 string               `config:"endpoint"`
	OptionalMemberCountDisabled bool                 `config:"disable_fetching_member_count"`
	UploadChunkSize             fs.SizeSuffix        `config:"chunk_size"`
	UploadCutoff                fs.SizeSuffix        `config:"upload_cutoff"`
	UploadConcurrency           int64                `config:"upload_concurrency"`
	Enc                         encoder.MultiEncoder `config:"encoding"`
	RootPrefix                  string               `config:"root_prefix"`
	ScopeAccess                 string               `config:"scope_access"`
	ScopeRole                   string               `config:"scope_role"`
}

// Fs represents a remote hidrive.
type Fs struct {
	name     string       // name of this remote
	root     string       // the path we are working on
	opt      Options      // parsed options
	features *fs.Features // optional features
	srv      *rest.Client // the connection to the server
	pacer    *fs.Pacer    // pacer for API calls
	// retryOnce is NOT intended as a pacer for API calls.
	// The intended use case is to repeat an action that failed because
	// some preconditions were not previously fulfilled.
	// Code using this should then establish these preconditions
	// and let the pacer retry the operation.
	retryOnce    *pacer.Pacer     // pacer with no delays to retry certain operations once
	tokenRenewer *oauthutil.Renew // renew the token on expiry
}

// Object describes a hidrive object.
//
// Will definitely have the remote-path but may lack meta-information.
type Object struct {
	fs          *Fs       // what this object is part of
	remote      string    // The remote path
	hasMetadata bool      // whether info below has been set
	size        int64     // size of the object
	modTime     time.Time // modification time of the object
	id          string    // ID of the object
	hash        string    // content-hash of the object
}

// ------------------------------------------------------------

// Name returns the name of the remote (as passed into NewFs).
func (f *Fs) Name() string {
	return f.name
}

// Root returns the name of the remote (as passed into NewFs).
func (f *Fs) Root() string {
	return f.root
}

// String returns a string-representation of this Fs.
func (f *Fs) String() string {
	return fmt.Sprintf("HiDrive root '%s'", f.root)
}

// Precision returns the precision of this Fs.
func (f *Fs) Precision() time.Duration {
	return time.Second
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hidrivehashType)
}

// Features returns the optional features of this Fs.
func (f *Fs) Features() *fs.Features {
	return f.features
}

// errorHandler parses a non 2xx error response into an error.
func errorHandler(resp *http.Response) error {
	// Decode error response.
	errResponse := new(api.Error)
	err := rest.DecodeJSON(resp, &errResponse)
	if err != nil {
		fs.Debugf(nil, "Couldn't decode error response: %v", err)
	}
	_, err = errResponse.Code.Int64()
	if err != nil {
		errResponse.Code = json.Number(strconv.Itoa(resp.StatusCode))
	}
	return errResponse
}

// NewFs creates a new file system from the path.
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	//fs.Debugf(nil, "hidrive: creating new Fs.")
	// Parse config into Options struct.
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	// Clean root-prefix and root-path.
	// NOTE: With the default-encoding "." and ".." will be encoded,
	// but with custom encodings without encoder.EncodeDot
	// "." and ".." will be interpreted as paths.
	if opt.RootPrefix != "" {
		opt.RootPrefix = path.Clean(opt.Enc.FromStandardPath(opt.RootPrefix))
	}
	root = path.Clean(opt.Enc.FromStandardPath(root))

	client, ts, err := oauthutil.NewClient(ctx, name, m, oauthConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to configure HiDrive: %w", err)
	}

	f := &Fs{
		name:      name,
		root:      root,
		opt:       *opt,
		srv:       rest.NewClient(client).SetRoot(opt.EndpointAPI),
		pacer:     fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
		retryOnce: pacer.New(pacer.RetriesOption(2), pacer.MaxConnectionsOption(-1), pacer.CalculatorOption(&pacer.ZeroDelayCalculator{})),
	}
	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
	}).Fill(ctx, f)
	f.srv.SetErrorHandler(errorHandler)

	if ts != nil {
		transaction := func() error {
			resolvedRoot := f.resolvePath("")
			_, err := f.fetchMetadataForPath(ctx, resolvedRoot, api.HiDriveObjectNoMetadataFields)
			return err
		}
		f.tokenRenewer = oauthutil.NewRenew(f.String(), ts, transaction)
	}

	// Do not allow the root-prefix to be nonexistent nor a directory,
	// but it can be empty.
	if f.opt.RootPrefix != "" {
		item, err := f.fetchMetadataForPath(ctx, f.opt.RootPrefix, api.HiDriveObjectNoMetadataFields)
		if err != nil {
			return nil, fmt.Errorf("could not access root-prefix: %w", err)
		}
		if item.Type != api.HiDriveObjectTypeDirectory {
			return nil, errors.New("the root-prefix needs to point to a valid directory or be empty")
		}
	}

	resolvedRoot := f.resolvePath("")
	item, err := f.fetchMetadataForPath(ctx, resolvedRoot, api.HiDriveObjectNoMetadataFields)
	if err != nil {
		if isHTTPError(err, 404) {
			// NOTE: NewFs needs to work with paths that do not exist,
			// in case they will be created later (see mkdir).
			return f, nil
		}
		return nil, fmt.Errorf("could not access root-path: %w", err)
	}
	if item.Type != api.HiDriveObjectTypeDirectory {
		fs.Debugf(f, "The root is not a directory. Setting its parent-directory as the new root.")
		// NOTE: There is no need to check
		// if the parent-directory is inside the root-prefix:
		// If the parent-directory was outside,
		// then the resolved path would be the root-prefix,
		// therefore the root-prefix would point to a file,
		// which has already been checked for.
		// In case the root-prefix is empty, this needs not be checked,
		// because top-level files cannot exist.
		f.root = path.Dir(f.root)
		return f, fs.ErrorIsFile
	}

	return f, nil
}

// newObject constructs an Object by calling the given function metaFiller
// on an Object with no metadata.
//
// metaFiller should set the metadata of the object or
// return an appropriate error.
func (f *Fs) newObject(remote string, metaFiller func(*Object) error) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	var err error
	if metaFiller != nil {
		err = metaFiller(o)
	}
	if err != nil {
		return nil, err
	}
	return o, nil
}

// newObjectFromHiDriveObject constructs an Object from the given api.HiDriveObject.
func (f *Fs) newObjectFromHiDriveObject(remote string, info *api.HiDriveObject) (fs.Object, error) {
	metaFiller := func(o *Object) error {
		return o.setMetadata(info)
	}
	return f.newObject(remote, metaFiller)
}

// NewObject finds the Object at remote.
//
// If remote points to a directory then it returns fs.ErrorIsDir.
// If it can not be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	//fs.Debugf(f, "executing NewObject(%s).", remote)
	metaFiller := func(o *Object) error {
		return o.readMetadata(ctx)
	}
	return f.newObject(remote, metaFiller)
}

// List the objects and directories in dir into entries.
// The entries can be returned in any order,
// but should be for a complete directory.
//
// dir should be "" to list the root, and should not have trailing slashes.
//
// This returns fs.ErrorDirNotFound if the directory is not found.
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	//fs.Debugf(f, "executing List(%s).", dir)
	var iErr error
	addEntry := func(info *api.HiDriveObject) bool {
		fs.Debugf(f, "found directory-element with name %s", info.Name)
		remote := path.Join(dir, info.Name)
		if info.Type == api.HiDriveObjectTypeDirectory {
			d := fs.NewDir(remote, info.ModTime())
			d.SetID(info.ID)
			d.SetSize(info.Size)
			d.SetItems(info.MemberCount)
			entries = append(entries, d)
		} else if info.Type == api.HiDriveObjectTypeFile {
			o, err := f.newObjectFromHiDriveObject(remote, info)
			if err != nil {
				iErr = err
				return true
			}
			entries = append(entries, o)
		}
		return false
	}

	var fields []string
	if f.opt.OptionalMemberCountDisabled {
		fields = api.HiDriveObjectWithMetadataFields
	} else {
		fields = api.HiDriveObjectWithDirectoryMetadataFields
	}
	resolvedDir := f.resolvePath(dir)
	_, err = f.iterateOverDirectory(ctx, resolvedDir, AllMembers, addEntry, fields, Unsorted)

	if err != nil {
		if isHTTPError(err, 404) {
			return nil, fs.ErrorDirNotFound
		}
		return nil, err
	}
	if iErr != nil {
		return nil, iErr
	}
	return entries, nil
}

// Put the contents of the io.Reader into the remote path
// with the modTime given of the given size.
// The existing or new object is returned.
//
// A new object may have been created or
// an existing one accessed even if an error is returned,
// in which case both the object and the error will be returned.
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	remote := src.Remote()
	//fs.Debugf(f, "executing Put(%s, %v).", remote, options)

	existingObj, err := f.NewObject(ctx, remote)
	switch err {
	case nil:
		return existingObj, existingObj.Update(ctx, in, src, options...)
	case fs.ErrorObjectNotFound:
		// Object was not found, so create a new one.
		return f.PutUnchecked(ctx, in, src, options...)
	}
	return nil, err
}

// PutStream uploads the contents of the io.Reader to the remote path
// with the modTime given of indeterminate size.
// The existing or new object is returned.
//
// A new object may have been created or
// an existing one accessed even if an error is returned,
// in which case both the object and the error will be returned.
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	//fs.Debugf(f, "executing PutStream(%s, %v).", src.Remote(), options)

	return f.Put(ctx, in, src, options...)
}

// PutUnchecked the contents of the io.Reader into the remote path
// with the modTime given of the given size.
// This guarantees that existing objects will not be overwritten.
// The new object is returned.
//
// This will produce an error if an object already exists at that path.
//
// In case the upload fails and an object has been created,
// this will try to delete the object at that path.
// In case the failed upload could not be deleted,
// both the object and the (upload-)error will be returned.
func (f *Fs) PutUnchecked(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	remote := src.Remote()
	modTime := src.ModTime(ctx)
	//fs.Debugf(f, "executing PutUnchecked(%s, %v).", remote, options)
	resolvedPath := f.resolvePath(remote)

	// NOTE: The file creation operation is a single atomic operation.
	// Thus uploading as much content as is reasonable
	// (i.e. everything up to the cutoff) in the first request,
	// avoids files being created on upload failure for small files.
	// (As opposed to creating an empty file and then uploading the content.)
	tmpReader, bytesRead, err := readerForChunk(in, int(f.opt.UploadCutoff))
	cutoffReader := cachedReader(tmpReader)
	if err != nil {
		return nil, err
	}

	var info *api.HiDriveObject
	err = f.retryOnce.Call(func() (bool, error) {
		var createErr error
		// Reset the reading index (in case this is a retry).
		if _, createErr = cutoffReader.Seek(0, io.SeekStart); createErr != nil {
			return false, createErr
		}
		info, createErr = f.createFile(ctx, resolvedPath, cutoffReader, modTime, IgnoreOnExist)

		if createErr == fs.ErrorDirNotFound {
			// Create the parent-directory for the object and repeat request.
			_, parentErr := f.createDirectories(ctx, path.Dir(resolvedPath), IgnoreOnExist)
			if parentErr != nil && parentErr != fs.ErrorDirExists {
				fs.Errorf(f, "Tried to create parent-directory for '%s', but failed.", resolvedPath)
				return false, parentErr
			}
			return true, createErr
		}
		return false, createErr
	})

	if err != nil {
		return nil, err
	}

	o, err := f.newObjectFromHiDriveObject(remote, info)

	if err != nil {
		return nil, err
	}

	if fs.SizeSuffix(bytesRead) < f.opt.UploadCutoff {
		return o, nil
	}
	// If there is more left to write, o.Update needs to skip ahead.
	// Use a fs.SeekOption with the current offset to do this.
	options = append(options, &fs.SeekOption{Offset: int64(bytesRead)})
	err = o.Update(ctx, in, src, options...)

	if err == nil {
		return o, nil
	}

	// Try to remove object at path after the its content could not be uploaded.
	deleteErr := f.pacer.Call(func() (bool, error) {
		deleteErr := o.Remove(ctx)
		return deleteErr == fs.ErrorObjectNotFound, deleteErr
	})

	if deleteErr == nil {
		return nil, err
	}

	fs.Errorf(f, "Tried to delete failed upload at path '%s', but failed: %v", resolvedPath, deleteErr)
	return o, err
}

// Mkdir creates the directory if it does not exist.
//
// This will create any missing parent directories.
//
// NOTE: If an error occurs while the parent directories are being created,
// any directories already created will NOT be deleted again.
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	//fs.Debugf(f, "executing Mkdir(%s).", dir)
	resolvedDir := f.resolvePath(dir)
	_, err := f.createDirectories(ctx, resolvedDir, IgnoreOnExist)

	if err == fs.ErrorDirExists {
		// NOTE: The conflict is caused by the directory already existing,
		// which should be ignored here.
		return nil
	}

	return err
}

// Rmdir removes the directory if empty.
//
// This returns fs.ErrorDirNotFound if the directory is not found.
// This returns fs.ErrorDirectoryNotEmpty if the directory is not empty.
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	//fs.Debugf(f, "executing Rmdir(%s).", dir)
	resolvedDir := f.resolvePath(dir)
	return f.deleteDirectory(ctx, resolvedDir, false)
}

// Purge removes the directory and all of its contents.
//
// This returns fs.ErrorDirectoryNotEmpty if the directory is not empty.
func (f *Fs) Purge(ctx context.Context, dir string) error {
	//fs.Debugf(f, "executing Purge(%s).", dir)
	resolvedDir := f.resolvePath(dir)
	return f.deleteDirectory(ctx, resolvedDir, true)
}

// shouldRetryAndCreateParents returns a boolean as to whether the operation
// should be retried after the parent-directories of the destination have been created.
// If so, it will create the parent-directories.
//
// If any errors arise while finding the source or
// creating the parent-directory those will be returned.
// Otherwise returns the originalError.
func (f *Fs) shouldRetryAndCreateParents(ctx context.Context, destinationPath string, sourcePath string, originalError error) (bool, error) {
	if fserrors.ContextError(ctx, &originalError) {
		return false, originalError
	}
	if isHTTPError(originalError, 404) {
		// Check if source is missing.
		_, srcErr := f.fetchMetadataForPath(ctx, sourcePath, api.HiDriveObjectNoMetadataFields)
		if srcErr != nil {
			return false, srcErr
		}
		// Source exists, so the parent of the destination must have been missing.
		// Create the parent-directory and repeat request.
		_, parentErr := f.createDirectories(ctx, path.Dir(destinationPath), IgnoreOnExist)
		if parentErr != nil && parentErr != fs.ErrorDirExists {
			fs.Errorf(f, "Tried to create parent-directory for '%s', but failed.", destinationPath)
			return false, parentErr
		}
		return true, originalError
	}
	return false, originalError
}

// Copy src to this remote using server-side copy operations.
//
// It returns the destination Object and a possible error.
//
// This returns fs.ErrorCantCopy if the operation cannot be performed.
//
// NOTE: If an error occurs when copying the Object,
// any parent-directories already created will NOT be deleted again.
//
// NOTE: This operation will expand sparse areas in the content of the source-Object
// to blocks of 0-bytes in the destination-Object.
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}
	// Get the absolute path to the source.
	srcPath := srcObj.fs.resolvePath(srcObj.Remote())
	//fs.Debugf(f, "executing Copy(%s, %s).", srcPath, remote)
	dstPath := f.resolvePath(remote)

	var info *api.HiDriveObject
	err := f.retryOnce.Call(func() (bool, error) {
		var copyErr error
		info, copyErr = f.copyFile(ctx, srcPath, dstPath, OverwriteOnExist)
		return f.shouldRetryAndCreateParents(ctx, dstPath, srcPath, copyErr)
	})

	if err != nil {
		return nil, err
	}
	dstObj, err := f.newObjectFromHiDriveObject(remote, info)
	if err != nil {
		return nil, err
	}
	return dstObj, nil
}

// Move src to this remote using server-side move operations.
//
// It returns the destination Object and a possible error.
//
// This returns fs.ErrorCantMove if the operation cannot be performed.
//
// NOTE: If an error occurs when moving the Object,
// any parent-directories already created will NOT be deleted again.
//
// NOTE: This operation will expand sparse areas in the content of the source-Object
// to blocks of 0-bytes in the destination-Object.
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't move - not same remote type")
		return nil, fs.ErrorCantMove
	}
	// Get the absolute path to the source.
	srcPath := srcObj.fs.resolvePath(srcObj.Remote())
	//fs.Debugf(f, "executing Move(%s, %s).", srcPath, remote)
	dstPath := f.resolvePath(remote)

	var info *api.HiDriveObject
	err := f.retryOnce.Call(func() (bool, error) {
		var moveErr error
		info, moveErr = f.moveFile(ctx, srcPath, dstPath, OverwriteOnExist)
		return f.shouldRetryAndCreateParents(ctx, dstPath, srcPath, moveErr)
	})

	if err != nil {
		return nil, err
	}
	dstObj, err := f.newObjectFromHiDriveObject(remote, info)
	if err != nil {
		return nil, err
	}
	return dstObj, nil

}

// DirMove moves from src at srcRemote to this remote at dstRemote
// using server-side move operations.
//
// This returns fs.ErrorCantCopy if the operation cannot be performed.
// This returns fs.ErrorDirExists if the destination already exists.
//
// NOTE: If an error occurs when moving the directory,
// any parent-directories already created will NOT be deleted again.
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) error {
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debugf(srcFs, "Can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}

	// Get the absolute path to the source.
	srcPath := srcFs.resolvePath(srcRemote)
	//fs.Debugf(f, "executing DirMove(%s, %s).", srcPath, dstRemote)
	dstPath := f.resolvePath(dstRemote)

	err := f.retryOnce.Call(func() (bool, error) {
		var moveErr error
		_, moveErr = f.moveDirectory(ctx, srcPath, dstPath, IgnoreOnExist)
		return f.shouldRetryAndCreateParents(ctx, dstPath, srcPath, moveErr)
	})

	if err != nil {
		if isHTTPError(err, 409) {
			return fs.ErrorDirExists
		}
		return err
	}
	return nil
}

// Shutdown shutdown the fs
func (f *Fs) Shutdown(ctx context.Context) error {
	f.tokenRenewer.Shutdown()
	return nil
}

// ------------------------------------------------------------

// Fs returns the parent Fs.
func (o *Object) Fs() fs.Info {
	return o.fs
}

// String returns a string-representation of this Object.
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// Remote returns the remote path.
func (o *Object) Remote() string {
	return o.remote
}

// ID returns the ID of the Object if known, or "" if not.
func (o *Object) ID() string {
	err := o.readMetadata(context.TODO())
	if err != nil {
		fs.Logf(o, "Failed to read metadata: %v", err)
		return ""
	}
	return o.id
}

// Hash returns the selected checksum of the file.
// If no checksum is available it returns "".
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	err := o.readMetadata(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to read hash from metadata: %w", err)
	}
	switch t {
	case hidrivehashType:
		return o.hash, nil
	default:
		return "", hash.ErrUnsupported
	}
}

// Size returns the size of an object in bytes.
func (o *Object) Size() int64 {
	err := o.readMetadata(context.TODO())
	if err != nil {
		fs.Logf(o, "Failed to read metadata: %v", err)
		return -1
	}
	return o.size
}

// setMetadata sets the metadata from info.
func (o *Object) setMetadata(info *api.HiDriveObject) error {
	if info.Type == api.HiDriveObjectTypeDirectory {
		return fs.ErrorIsDir
	}
	if info.Type != api.HiDriveObjectTypeFile {
		return fmt.Errorf("%q is %q: %w", o.remote, info.Type, fs.ErrorNotAFile)
	}
	o.hasMetadata = true
	o.size = info.Size
	o.modTime = info.ModTime()
	o.id = info.ID
	o.hash = info.ContentHash
	return nil
}

// readMetadata fetches the metadata if it has not already been fetched.
func (o *Object) readMetadata(ctx context.Context) error {
	if o.hasMetadata {
		return nil
	}
	resolvedPath := o.fs.resolvePath(o.remote)
	info, err := o.fs.fetchMetadataForPath(ctx, resolvedPath, api.HiDriveObjectWithMetadataFields)
	if err != nil {
		if isHTTPError(err, 404) {
			return fs.ErrorObjectNotFound
		}
		return err
	}
	return o.setMetadata(info)
}

// ModTime returns the modification time of the object.
func (o *Object) ModTime(ctx context.Context) time.Time {
	err := o.readMetadata(ctx)
	if err != nil {
		fs.Logf(o, "Failed to read metadata: %v", err)
		return time.Now()
	}
	return o.modTime
}

// SetModTime sets the metadata on the object to set the modification date.
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	parameters := api.NewQueryParameters()
	resolvedPath := o.fs.resolvePath(o.remote)
	parameters.SetPath(resolvedPath)
	err := parameters.SetTime("mtime", modTime)

	if err != nil {
		return err
	}

	opts := rest.Opts{
		Method:     "PATCH",
		Path:       "/meta",
		Parameters: parameters.Values,
		NoResponse: true,
	}

	var resp *http.Response
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.Call(ctx, &opts)
		return o.fs.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return err
	}
	o.modTime = modTime
	return nil
}

// Storable says whether this object can be stored.
func (o *Object) Storable() bool {
	return true
}

// Open an object for reading.
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	parameters := api.NewQueryParameters()
	resolvedPath := o.fs.resolvePath(o.remote)
	parameters.SetPath(resolvedPath)

	fs.FixRangeOption(options, o.Size())
	opts := rest.Opts{
		Method:     "GET",
		Path:       "/file",
		Parameters: parameters.Values,
		Options:    options,
	}
	var resp *http.Response
	var err error
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.Call(ctx, &opts)
		return o.fs.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	return resp.Body, err
}

// Update the existing object
// with the contents of the io.Reader, modTime and size.
//
// For unknown-sized contents (indicated by src.Size() == -1)
// this will try to properly upload it in multiple chunks.
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	//fs.Debugf(o.fs, "executing Update(%s, %v).", o.remote, options)
	modTime := src.ModTime(ctx)
	resolvedPath := o.fs.resolvePath(o.remote)

	if o.fs.tokenRenewer != nil {
		o.fs.tokenRenewer.Start()
		defer o.fs.tokenRenewer.Stop()
	}

	// PutUnchecked can pass a valid SeekOption to skip ahead.
	var offset uint64
	for _, option := range options {
		if seekoption, ok := option.(*fs.SeekOption); ok {
			offset = uint64(seekoption.Offset)
			break
		}
	}

	var info *api.HiDriveObject
	var err, metaErr error
	if offset > 0 || src.Size() == -1 || src.Size() >= int64(o.fs.opt.UploadCutoff) {
		fs.Debugf(o.fs, "Uploading with chunks of size %v and %v transfers in parallel at path '%s'.", int(o.fs.opt.UploadChunkSize), o.fs.opt.UploadConcurrency, resolvedPath)
		// NOTE: o.fs.opt.UploadChunkSize should always
		// be between 0 and MaximumUploadBytes,
		// so the conversion to an int does not cause problems for valid inputs.
		if offset > 0 {
			// NOTE: The offset is only set
			// when the file was newly created,
			// therefore the file does not need truncating.
			_, err = o.fs.updateFileChunked(ctx, resolvedPath, in, offset, int(o.fs.opt.UploadChunkSize), o.fs.opt.UploadConcurrency)
			if err == nil {
				err = o.SetModTime(ctx, modTime)
			}
		} else {
			_, _, err = o.fs.uploadFileChunked(ctx, resolvedPath, in, modTime, int(o.fs.opt.UploadChunkSize), o.fs.opt.UploadConcurrency)
		}
		// Try to check if object was updated, either way.
		// Metadata should be updated even if the upload fails.
		info, metaErr = o.fs.fetchMetadataForPath(ctx, resolvedPath, api.HiDriveObjectWithMetadataFields)
	} else {
		info, err = o.fs.overwriteFile(ctx, resolvedPath, cachedReader(in), modTime)
		metaErr = err
	}

	// Update metadata of this object,
	// if there was no error with getting the metadata.
	if metaErr == nil {
		metaErr = o.setMetadata(info)
	}

	// Errors with the upload-process are more relevant, return those first.
	if err != nil {
		return err
	}
	return metaErr
}

// Remove an object.
func (o *Object) Remove(ctx context.Context) error {
	resolvedPath := o.fs.resolvePath(o.remote)
	return o.fs.deleteObject(ctx, resolvedPath)
}

// Check the interfaces are satisfied.
var (
	_ fs.Fs             = (*Fs)(nil)
	_ fs.Purger         = (*Fs)(nil)
	_ fs.PutStreamer    = (*Fs)(nil)
	_ fs.PutUncheckeder = (*Fs)(nil)
	_ fs.Copier         = (*Fs)(nil)
	_ fs.Mover          = (*Fs)(nil)
	_ fs.DirMover       = (*Fs)(nil)
	_ fs.Shutdowner     = (*Fs)(nil)
	_ fs.Object         = (*Object)(nil)
	_ fs.IDer           = (*Object)(nil)
)
