// Package huaweidrive implements the Huawei Drive backend for rclone
package huaweidrive

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/rclone/rclone/backend/huaweidrive/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/oauthutil"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
)

const (
	rcloneClientID              = "115505115"
	rcloneEncryptedClientSecret = "2ecc862c65eeb3e281cb63178e3b74bc9a5100a27f8d36dbed89922f8ff7f434"
	minSleep                    = 10 * time.Millisecond
	maxSleep                    = 2 * time.Second
	decayConstant               = 2 // bigger for slower decay, exponential
	rootURL                     = "https://driveapis.cloud.huawei.com.cn/drive/v1"
	uploadURL                   = "https://driveapis.cloud.huawei.com.cn/upload/drive/v1"
	defaultChunkSize            = 8 * fs.Mebi
	maxFileSize                 = 50 * fs.Gibi // Maximum file size for Huawei Drive
)

// OAuth2 configuration
var oauthConfig = &oauthutil.Config{
	Scopes: []string{
		"https://www.huawei.com/auth/drive",
		"openid",
		"profile",
	},
	AuthURL:      "https://oauth-login.cloud.huawei.com/oauth2/v3/authorize",
	TokenURL:     "https://oauth-login.cloud.huawei.com/oauth2/v3/token",
	ClientID:     rcloneClientID,
	ClientSecret: rcloneEncryptedClientSecret,
	RedirectURL:  oauthutil.RedirectURL,
}

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "huaweidrive",
		Description: "Huawei Drive",
		NewFs:       NewFs,
		Config: func(ctx context.Context, name string, m configmap.Mapper, config fs.ConfigIn) (*fs.ConfigOut, error) {
			// Parse config into Options struct
			opt := new(Options)
			err := configstruct.Set(m, opt)
			if err != nil {
				return nil, fmt.Errorf("couldn't parse config into struct: %w", err)
			}

			// Update OAuth2 config with user provided client credentials
			clientID, _ := m.Get("client_id")
			clientSecret, _ := m.Get("client_secret")

			if clientID != "" {
				oauthConfig.ClientID = clientID
			}
			if clientSecret != "" {
				oauthConfig.ClientSecret = clientSecret
			}

			// Check if we have an access token
			accessToken, ok := m.Get("token")
			if accessToken == "" || !ok {
				// If no token, start OAuth2 flow
				return oauthutil.ConfigOut("", &oauthutil.Options{
					OAuth2Config: oauthConfig,
				})
			}
			return nil, nil
		},
		Options: append(oauthutil.SharedOptions, []fs.Option{{
			Name:     "chunk_size",
			Help:     "Upload chunk size.\n\nMust be a power of 2 >= 256k and <= 64MB.",
			Default:  defaultChunkSize,
			Advanced: true,
		}, {
			Name:     "list_chunk",
			Default:  1000,
			Help:     "Size of listing chunk 1-1000.",
			Advanced: true,
		}, {
			Name:     "upload_cutoff",
			Help:     "Cutoff for switching to multipart upload.\n\nAny files larger than this will be uploaded in chunks of chunk_size.\nThe minimum is 0 and the maximum is 5 GiB.",
			Default:  20 * fs.Mebi,
			Advanced: true,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			// Huawei Drive encoding similar to other cloud storage providers
			Default: (encoder.Display |
				encoder.EncodeBackSlash |
				encoder.EncodeInvalidUtf8 |
				encoder.EncodeRightSpace),
		}}...),
	})
}

// Options defines the configuration for this backend
type Options struct {
	ChunkSize    fs.SizeSuffix        `config:"chunk_size"`
	ListChunk    int                  `config:"list_chunk"`
	UploadCutoff fs.SizeSuffix        `config:"upload_cutoff"`
	Enc          encoder.MultiEncoder `config:"encoding"`
}

// Fs represents a remote Huawei Drive
type Fs struct {
	name     string             // name of this remote
	root     string             // root path in the remote
	opt      Options            // parsed options
	features *fs.Features       // optional features
	srv      *rest.Client       // the connection to the server
	pacer    *fs.Pacer          // pacer for API calls
	dirCache *dircache.DirCache // Map of directory path to directory id
}

// Object describes a Huawei Drive object
type Object struct {
	fs          *Fs       // what this object is part of
	remote      string    // The remote path
	hasMetaData bool      // whether info below has been set
	size        int64     // size of the object
	modTime     time.Time // modification time of the object
	id          string    // ID of the object
	sha256      string    // SHA256 hash of the object
	mimeType    string    // Content-Type of object from server (may not be file extension)
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
	return fmt.Sprintf("Huawei Drive root '%s'", f.root)
}

// Precision return the precision of this Fs
func (f *Fs) Precision() time.Duration {
	return time.Second
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.SHA256)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// parsePath parses a Huawei Drive 'url'
func parsePath(path string) (root string) {
	root = strings.Trim(path, "/")
	return
}

// rootParentID returns the ID of the parent of the root directory
func (f *Fs) rootParentID() string {
	// For Huawei Drive, root directory doesn't have a parent
	// We use empty string to represent the root
	return ""
}

// NewFs constructs an Fs from the path, container:path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	root = parsePath(root)

	// Create OAuth2 client
	client, _, err := oauthutil.NewClient(ctx, name, m, oauthConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to configure Huawei Drive OAuth: %w", err)
	}

	// Create REST client
	srv := rest.NewClient(client).SetRoot(rootURL)

	f := &Fs{
		name:  name,
		root:  root,
		opt:   *opt,
		srv:   srv,
		pacer: fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
	}

	f.features = (&fs.Features{
		CaseInsensitive:         false,
		DuplicateFiles:          false,
		ReadMimeType:            true,
		WriteMimeType:           true,
		CanHaveEmptyDirectories: true,
		BucketBased:             false,
	}).Fill(ctx, f)

	// Create directory cache
	f.dirCache = dircache.New(root, f.rootParentID(), f)

	// Find the current root
	err = f.dirCache.FindRoot(ctx, false)
	if err != nil {
		// Assume it is a file
		newRoot, remote := dircache.SplitPath(root)
		tempF := *f
		tempF.dirCache = dircache.New(newRoot, f.rootParentID(), &tempF)
		tempF.root = newRoot
		// Make new Fs which is the parent
		err = tempF.dirCache.FindRoot(ctx, false)
		if err != nil {
			// No root so return old f
			return f, nil
		}
		_, err := tempF.newObjectWithInfo(ctx, remote, nil)
		if err != nil {
			if err == fs.ErrorObjectNotFound {
				// File doesn't exist so return old f
				return f, nil
			}
			return nil, err
		}
		f.features.Fill(ctx, &tempF)
		// XXX: update f.features here instead of tempF.features
		f.features = tempF.features
		// return an error with an fs which points to the parent
		return &tempF, fs.ErrorIsFile
	}
	return f, nil
}

// Return an Object from a path
//
// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(ctx context.Context, remote string, info *api.File) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	var err error
	if info != nil {
		// Set info
		err = o.setMetaData(info)
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

// FindLeaf finds a directory of name leaf in the folder with ID pathID
func (f *Fs) FindLeaf(ctx context.Context, pathID, leaf string) (pathIDOut string, found bool, err error) {
	// Find the leaf in pathID
	found, err = f.listAll(ctx, pathID, func(item *api.File) bool {
		if strings.EqualFold(item.FileName, leaf) && item.IsDir() {
			pathIDOut = item.ID
			return true
		}
		return false
	})
	return pathIDOut, found, err
}

// CreateDir makes a directory with pathID as parent and name leaf
func (f *Fs) CreateDir(ctx context.Context, pathID, leaf string) (newID string, err error) {
	// Create the directory
	var req = api.CreateFolderRequest{
		FileName:    f.opt.Enc.FromStandardName(leaf),
		MimeType:    api.FolderMimeType,
		Description: "folder",
	}

	// Set parent folder if specified
	if pathID != "" && pathID != f.rootParentID() {
		req.ParentFolder = []string{pathID}
	}

	opts := rest.Opts{
		Method: "POST",
		Path:   "/files",
		Parameters: url.Values{
			"fields": []string{"*"},
		},
	}

	var info *api.File
	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, &req, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return "", fmt.Errorf("couldn't create directory %q: %w", leaf, err)
	}

	if info == nil || info.ID == "" {
		return "", fmt.Errorf("invalid response when creating directory %q", leaf)
	}

	return info.ID, nil
}

// list the objects into the function supplied
//
// If directories is set it only sends directories
// User function to process a File item from listAll
//
// Should return true to finish processing
type listAllFn func(*api.File) bool

// Lists the directory required calling the user function on each item found
//
// If the user fn ever returns true then it early exits with found = true
func (f *Fs) listAll(ctx context.Context, dirID string, fn listAllFn) (found bool, err error) {
	opts := rest.Opts{
		Method: "GET",
		Path:   "/files",
		Parameters: url.Values{
			"fields": []string{"*"},
		},
	}

	// Add query parameter for parent folder if not root
	if dirID != "" && dirID != f.rootParentID() {
		opts.Parameters.Set("queryParam", fmt.Sprintf("parentFolder='%s'", dirID))
	}

	// Set page size if specified
	if f.opt.ListChunk > 0 && f.opt.ListChunk <= 1000 {
		opts.Parameters.Set("pageSize", strconv.Itoa(f.opt.ListChunk))
	}

	var result api.FileList
	for {
		err = f.pacer.Call(func() (bool, error) {
			resp, err := f.srv.CallJSON(ctx, &opts, nil, &result)
			return shouldRetry(ctx, resp, err)
		})
		if err != nil {
			return false, fmt.Errorf("couldn't list directory %q: %w", dirID, err)
		}

		for i := range result.Files {
			item := &result.Files[i]
			if fn(item) {
				found = true
				return
			}
		}

		// Check for more pages
		if result.NextPageToken == "" {
			break
		}
		opts.Parameters.Set("pageToken", result.NextPageToken)
	}
	return
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
	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return nil, err
	}
	var iErr error
	_, err = f.listAll(ctx, directoryID, func(info *api.File) bool {
		remote := path.Join(dir, f.opt.Enc.ToStandardName(info.FileName))
		if info.IsDir() {
			// cache the directory ID for later lookups
			f.dirCache.Put(remote, info.ID)
			d := fs.NewDir(remote, info.EditedTime).SetID(info.ID)
			d.SetItems(0) // Huawei Drive doesn't return item count
			entries = append(entries, d)
		} else {
			o, err := f.newObjectWithInfo(ctx, remote, info)
			if err != nil {
				iErr = err
				return true
			}
			entries = append(entries, o)
		}
		return false
	})
	if err != nil {
		return nil, err
	}
	if iErr != nil {
		return nil, iErr
	}
	return entries, nil
}

// Creates from the parameters passed in a half finished Object which
// must have setMetaData called on it
//
// Returns the object, leaf, directoryID and error.
//
// Used to create new objects
func (f *Fs) createObject(ctx context.Context, remote string, modTime time.Time, size int64) (o *Object, leaf string, directoryID string, err error) {
	// Create the directory for the object if it doesn't exist
	leaf, directoryID, err = f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return
	}
	// Temporary Object under construction
	o = &Object{
		fs:     f,
		remote: remote,
	}
	return o, leaf, directoryID, nil
}

// Put the object into the container
//
// Copy the reader in to the new object which is returned.
//
// The new object may have been created if an error is returned
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	exisitingObj, err := f.newObjectWithInfo(ctx, src.Remote(), nil)
	if err == nil {
		err = exisitingObj.Update(ctx, in, src, options...)
		if err != nil {
			return nil, err
		}
		return exisitingObj, nil
	}
	if err != fs.ErrorObjectNotFound {
		return nil, err
	}
	// The object doesn't exist so create it
	return f.PutUnchecked(ctx, in, src, options...)
}

// PutUnchecked the object into the container
//
// This will produce duplicates if we haven't checked that there
// is an existing object first.
//
// Copy the reader in to the new object which is returned.
//
// The new object may have been created if an error is returned.
func (f *Fs) PutUnchecked(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	remote := src.Remote()
	size := src.Size()
	modTime := src.ModTime(ctx)

	o, _, _, err := f.createObject(ctx, remote, modTime, size)
	if err != nil {
		return nil, err
	}
	err = o.Update(ctx, in, src, options...)
	if err != nil {
		return nil, err
	}
	return o, nil
}

// Mkdir creates the container if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	_, err := f.dirCache.FindDir(ctx, dir, true)
	return err
}

// purgeCheck removes the root directory, if check is set then it
// refuses to do so if it has anything in
func (f *Fs) purgeCheck(ctx context.Context, dir string, check bool) error {
	root := path.Join(f.root, dir)
	if root == "" {
		return errors.New("can't purge root directory")
	}
	dc := f.dirCache
	rootID, err := dc.FindDir(ctx, dir, false)
	if err != nil {
		return err
	}

	if check {
		found, err := f.listAll(ctx, rootID, func(item *api.File) bool {
			return true // return found
		})
		if err != nil {
			return err
		}
		if found {
			return fs.ErrorDirectoryNotEmpty
		}
	}

	opts := rest.Opts{
		Method: "DELETE",
		Path:   "/files/" + rootID,
	}
	return f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.Call(ctx, &opts)
		return shouldRetry(ctx, resp, err)
	})
}

// Rmdir deletes the root folder
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir, true)
}

// Purge deletes all the files in the directory
//
// Optional interface: Only implement this if you have a way of
// deleting all the files quicker than just running Remove() on the
// result of List()
func (f *Fs) Purge(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir, false)
}

// Copy src to this remote using server-side copy operations.
//
// This is stored with the remote path given.
//
// It returns the destination Object and a possible error.
//
// Will only be called if src.Fs().Name() == f.Name().
//
// If it isn't possible then return fs.ErrorCantCopy
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}

	err := srcObj.readMetaData(ctx)
	if err != nil {
		return nil, err
	}

	srcPath := srcObj.fs.rootSlash() + srcObj.remote
	dstPath := f.rootSlash() + remote
	if srcPath == dstPath {
		return nil, fmt.Errorf("can't copy %q -> %q as they are same name", srcPath, dstPath)
	}

	// Create the directory for the object if it doesn't exist
	leaf, directoryID, err := f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return nil, err
	}

	// Copy the object
	opts := rest.Opts{
		Method: "POST",
		Path:   "/files/" + srcObj.id + "/copy",
		Parameters: url.Values{
			"fields": []string{"*"},
		},
	}

	copyReq := api.CopyFileRequest{
		FileName: f.opt.Enc.FromStandardName(leaf),
	}
	if directoryID != "" {
		copyReq.ParentFolder = []string{directoryID}
	}

	var info *api.File
	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, &copyReq, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	return f.newObjectWithInfo(ctx, remote, info)
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

// Hash returns the SHA256 of the object
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	if t != hash.SHA256 {
		return "", hash.ErrUnsupported
	}
	return o.sha256, nil
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	err := o.readMetaData(context.TODO())
	if err != nil {
		fs.Logf(o, "Failed to read metadata: %v", err)
		return 0
	}
	return o.size
}

// setMetaData sets the metadata from info
func (o *Object) setMetaData(info *api.File) (err error) {
	if info.IsDir() {
		return fs.ErrorIsDir
	}
	o.hasMetaData = true
	o.size = info.Size
	o.sha256 = info.SHA256
	o.modTime = info.EditedTime
	o.id = info.ID
	o.mimeType = info.MimeType
	return nil
}

// readMetaData gets the metadata if it hasn't already been fetched
//
// it also sets the info
func (o *Object) readMetaData(ctx context.Context) (err error) {
	if o.hasMetaData {
		return nil
	}
	info, err := o.fs.readMetaDataForPath(ctx, o.remote)
	if err != nil {
		return err
	}
	return o.setMetaData(info)
}

// readMetaDataForPath reads the metadata from the path
func (f *Fs) readMetaDataForPath(ctx context.Context, path string) (info *api.File, err error) {
	// defer log.Trace(f, "path=%q", path)("info=%+v, err=%v", &info, &err)
	leaf, directoryID, err := f.dirCache.FindPath(ctx, path, false)
	if err != nil {
		if err == fs.ErrorDirNotFound {
			return nil, fs.ErrorObjectNotFound
		}
		return nil, err
	}

	found, err := f.listAll(ctx, directoryID, func(item *api.File) bool {
		if strings.EqualFold(item.FileName, leaf) && !item.IsDir() {
			info = item
			return true
		}
		return false
	})
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fs.ErrorObjectNotFound
	}
	return info, nil
}

// ModTime returns the modification time of the object
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *Object) ModTime(ctx context.Context) time.Time {
	err := o.readMetaData(ctx)
	if err != nil {
		fs.Logf(o, "Failed to read metadata: %v", err)
		return time.Now()
	}
	return o.modTime
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	opts := rest.Opts{
		Method: "PATCH",
		Path:   "/files/" + o.id,
		Parameters: url.Values{
			"fields": []string{"*"},
		},
	}

	update := api.UpdateFileRequest{
		EditedTime: &modTime,
	}

	var info *api.File
	err := o.fs.pacer.Call(func() (bool, error) {
		resp, err := o.fs.srv.CallJSON(ctx, &opts, &update, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return err
	}
	return o.setMetaData(info)
}

// Storable returns a boolean showing whether this object storable
func (o *Object) Storable() bool {
	return true
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	if o.id == "" {
		return nil, errors.New("can't download - no id")
	}
	if o.size == 0 {
		return io.NopCloser(strings.NewReader("")), nil
	}

	fs.FixRangeOption(options, o.size)
	opts := rest.Opts{
		Method: "GET",
		Path:   "/files/" + o.id,
		Parameters: url.Values{
			"form": []string{"content"},
		},
		Options: options,
	}
	var resp *http.Response
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.Call(ctx, &opts)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open file %q for reading: %w", o.remote, err)
	}
	return resp.Body, nil
}

// Update the object with the contents of the io.Reader, modTime and size
//
// If existing is set then it updates the object rather than creating a new one.
//
// The new object may have been created if an error is returned
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	size := src.Size()
	remote := o.Remote()

	// Create the directory for the object if it doesn't exist
	leaf, directoryID, err := o.fs.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return err
	}

	leaf = o.fs.opt.Enc.FromStandardName(leaf)

	// Determine upload method based on size
	if size >= 0 && size < int64(o.fs.opt.UploadCutoff) {
		return o.uploadSimple(ctx, in, leaf, directoryID, size, src.ModTime(ctx))
	}
	return o.uploadResume(ctx, in, leaf, directoryID, size, src.ModTime(ctx))
}

// uploadSimple uploads a file using simple upload for files < upload_cutoff
func (o *Object) uploadSimple(ctx context.Context, in io.Reader, leaf, directoryID string, size int64, modTime time.Time) (err error) {
	// Use upload URL for uploads
	srv := rest.NewClient(fshttp.NewClient(ctx)).SetRoot(uploadURL)

	opts := rest.Opts{
		Method: "POST",
		Parameters: url.Values{
			"uploadType": []string{"content"},
			"fields":     []string{"*"},
		},
		Body: in,
	}

	if o.id != "" {
		// Update existing file
		opts.Path = "/files/" + o.id
		opts.Method = "PUT"
	} else {
		// Create new file
		opts.Path = "/files"
	}

	// Detect content type
	mimeType := mime.TypeByExtension(path.Ext(leaf))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	opts.ContentType = mimeType

	var info *api.File
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err := srv.CallJSON(ctx, &opts, nil, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("failed to upload file %q: %w", leaf, err)
	}

	if info == nil {
		return fmt.Errorf("no file info returned when uploading %q", leaf)
	}

	return o.setMetaData(info)
}

// uploadResume uploads a file using resumable upload for files >= upload_cutoff
func (o *Object) uploadResume(ctx context.Context, in io.Reader, leaf, directoryID string, size int64, modTime time.Time) (err error) {
	// Use upload URL for uploads
	srv := rest.NewClient(fshttp.NewClient(ctx)).SetRoot(uploadURL)

	// First, initialize the resumable upload
	opts := rest.Opts{
		Method: "POST",
		Parameters: url.Values{
			"uploadType": []string{"resume"},
			"fields":     []string{"*"},
		},
		ExtraHeaders: map[string]string{
			"X-Upload-Content-Length": strconv.FormatInt(size, 10),
		},
	}

	// Detect content type
	mimeType := mime.TypeByExtension(path.Ext(leaf))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	opts.ExtraHeaders["X-Upload-Content-Type"] = mimeType

	if o.id != "" {
		// Update existing file
		opts.Path = "/files/" + o.id
		opts.Method = "PUT"
	} else {
		// Create new file
		opts.Path = "/files"
	}

	// Prepare metadata for resumable upload initialization
	metadata := map[string]interface{}{
		"fileName": leaf,
	}
	if directoryID != "" && directoryID != o.fs.rootParentID() {
		metadata["parentFolder"] = []string{directoryID}
	}

	var resp *http.Response
	var initResp api.ResumeUploadInitResponse
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = srv.CallJSON(ctx, &opts, metadata, &initResp)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("failed to initialize resumable upload for %q: %w", leaf, err)
	}

	// Get the upload URL from Location header
	location := resp.Header.Get("Location")
	if location == "" {
		return fmt.Errorf("no upload URL returned for %q", leaf)
	}

	// Now upload the content in chunks
	chunkSize := int64(o.fs.opt.ChunkSize)
	if chunkSize < 256*1024 {
		chunkSize = 256 * 1024 // Minimum chunk size according to Huawei Drive API
	}
	if chunkSize > 64*1024*1024 {
		chunkSize = 64 * 1024 * 1024 // Maximum single upload size
	}

	buf := make([]byte, chunkSize)
	var offset int64

	for offset < size {
		n, err := io.ReadFull(in, buf)
		if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
			return fmt.Errorf("failed to read chunk at offset %d: %w", offset, err)
		}

		chunk := buf[:n]
		end := offset + int64(n) - 1

		// Upload this chunk
		chunkSrv := rest.NewClient(fshttp.NewClient(ctx))
		chunkOpts := rest.Opts{
			Method:  "PUT",
			RootURL: location,
			Body:    bytes.NewReader(chunk),
			ExtraHeaders: map[string]string{
				"Content-Range":  fmt.Sprintf("bytes %d-%d/%d", offset, end, size),
				"Content-Length": strconv.Itoa(n),
				"Content-Type":   mimeType,
			},
		}

		var info *api.File
		err = o.fs.pacer.Call(func() (bool, error) {
			resp, err := chunkSrv.CallJSON(ctx, &chunkOpts, nil, &info)
			// 308 means continue uploading (incomplete)
			if resp != nil && resp.StatusCode == 308 {
				return false, nil
			}
			// 200/201 means upload complete
			if resp != nil && (resp.StatusCode == 200 || resp.StatusCode == 201) {
				return false, nil
			}
			return shouldRetry(ctx, resp, err)
		})
		if err != nil {
			return fmt.Errorf("failed to upload chunk at offset %d: %w", offset, err)
		}

		offset += int64(n)

		// If we got file info back, we're done
		if info != nil && info.ID != "" {
			return o.setMetaData(info)
		}
	}

	return fmt.Errorf("upload completed but no file info received for %q", leaf)
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	opts := rest.Opts{
		Method: "DELETE",
		Path:   "/files/" + o.id,
	}
	return o.fs.pacer.Call(func() (bool, error) {
		resp, err := o.fs.srv.Call(ctx, &opts)
		return shouldRetry(ctx, resp, err)
	})
}

// rootSlash returns root with a trailing slash
func (f *Fs) rootSlash() string {
	if f.root == "" {
		return f.root
	}
	return f.root + "/"
}

// shouldRetry returns a boolean as to whether this err deserves to be retried
func shouldRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}

	// Handle specific Huawei Drive error responses
	if resp != nil {
		switch resp.StatusCode {
		case 401:
			return false, fserrors.NoRetryError(fmt.Errorf("unauthorized: %w", err))
		case 403:
			return false, fserrors.NoRetryError(fmt.Errorf("forbidden: %w", err))
		case 404:
			return false, fs.ErrorObjectNotFound
		case 409:
			return false, fs.ErrorDirExists
		}
	}

	return fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// retryErrorCodes is a slice of error codes that we will retry
var retryErrorCodes = []int{
	429, // Too Many Requests
	500, // Internal Server Error
	502, // Bad Gateway
	503, // Service Unavailable
	504, // Gateway Timeout
	509, // Bandwidth Limit Exceeded
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = (*Fs)(nil)
	_ fs.Purger          = (*Fs)(nil)
	_ fs.Copier          = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ dircache.DirCacher = (*Fs)(nil)
)
