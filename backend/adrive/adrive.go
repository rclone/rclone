// Package adrive provides an interface to the Aliyun Drive
// object storage system.
package adrive

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/rclone/rclone/backend/adrive/api"
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
	"golang.org/x/oauth2"
)

const (
	minSleep         = 10 * time.Millisecond
	maxSleep         = 2 * time.Second
	decayConstant    = 2 // bigger for slower decay, exponential
	defaultChunkSize = int64(10 * fs.Mebi)
	maxPartNum       = 10000
	rootURL          = "https://openapi.alipan.com"
	authURL          = "https://openapi.alipan.com/oauth/authorize"
	tokenURL         = "https://openapi.alipan.com/oauth/access_token"
)

// Globals
var (
	// Description of how to auth for this app
	oauthConfig = &oauthutil.Config{
		Scopes: []string{
			"user:base,file:all:read,file:all:write",
		},
		AuthURL:     authURL,
		TokenURL:    tokenURL,
		AuthStyle:   oauth2.AuthStyleInParams,
		RedirectURL: oauthutil.RedirectURL,
	}
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "adrive",
		Description: "Aliyun Drive",
		NewFs:       NewFs,
		Config: func(ctx context.Context, name string, m configmap.Mapper, config fs.ConfigIn) (*fs.ConfigOut, error) {
			return oauthutil.ConfigOut("", &oauthutil.Options{
				OAuth2Config: oauthConfig,
			})
		},
		Options: append(oauthutil.SharedOptions, []fs.Option{
			{
				Name:     config.ConfigEncoding,
				Help:     config.ConfigEncodingHelp,
				Advanced: true,
				Default: encoder.EncodeWin |
					encoder.EncodeSlash |
					encoder.EncodeBackSlash |
					encoder.EncodeInvalidUtf8, //  /*?:<>\"|
			},
		}...),
	})
}

// Options defines the configuration for this backend
type Options struct {
	Enc           encoder.MultiEncoder `config:"encoding"`
	RootFolderID  string               `config:"root_folder_id"`
	ChunkSize     fs.SizeSuffix        `config:"chunk_size"`
	RemoveWay     string               `config:"remove_way"`
	CheckNameMode string               `config:"check_name_mode"`
	AccessToken   string               `config:"access_token"`
	RefreshToken  string               `config:"refresh_token"`
	ExpiresAt     string               `config:"expires_at"`
	ClientID      string               `config:"client_id"`
	ClientSecret  string               `config:"client_secret"`
}

// Fs represents a remote adrive
type Fs struct {
	name         string             // name of this remote
	root         string             // the path we are working on
	opt          Options            // parsed options
	features     *fs.Features       // optional features
	srv          *rest.Client       // Aliyun Drive client
	dirCache     *dircache.DirCache // Map of directory path to directory id
	pacer        *fs.Pacer          // pacer for API calls
	tokenRenewer *oauthutil.Renew   // renew the token on expiry
	m            configmap.Mapper
	driveID      string
	rootID       string // the id of the root folder
}

// Object describes a adrive object
//
// Will definitely have info but maybe not meta
type Object struct {
	fs          *Fs       // what this object is part of
	remote      string    // The remote path
	hasMetaData bool      // whether info below has been set
	size        int64     // size of the object
	modTime     time.Time // modification time of the object
	id          string    // ID of the object
	parentID    string    // ID of the parent directory
	sha1        string    // SHA-1 of the object content
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
	return fmt.Sprintf("Aliyun Drive root '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// parsePath parses a box 'url'
func parsePath(path string) (root string) {
	root = strings.Trim(path, "/")
	return
}

// parseTime parses a time string
func parseTime(timeStr string) time.Time {
	if timeStr == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return time.Time{}
	}
	return t
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

// shouldRetry returns a boolean as to whether this resp and err
// deserve to be retried.  It returns the err as a convenience
func shouldRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	authRetry := false

	if resp != nil && resp.StatusCode == 401 && strings.Contains(resp.Header.Get("Www-Authenticate"), "expired_token") {
		authRetry = true
		fs.Debugf(nil, "Should retry: %v", err)
	}

	if _, ok := err.(*api.Error); ok {
		fs.Debugf(nil, "Retrying API error %v", err)
		return true, err
	}

	return authRetry || fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// errorHandler parses a non 2xx error response into an error
func errorHandler(resp *http.Response) error {
	// Decode error response
	errResponse := new(api.Error)
	err := rest.DecodeJSON(resp, &errResponse)
	if err != nil {
		fs.Debugf(nil, "Couldn't decode error response: %v", err)
	}
	if errResponse.Code == 0 {
		errResponse.Code = resp.StatusCode
	}
	if errResponse.Message == "" {
		errResponse.Message = resp.Status
	}
	return errResponse
}

// NewFs constructs an Fs from the path, container:path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	root = parsePath(root)

	// Create HTTP client
	client := fshttp.NewClient(ctx)
	var ts *oauthutil.TokenSource
	// If not using an accessToken, create an oauth client and tokensource
	if opt.AccessToken == "" {
		client, ts, err = oauthutil.NewClient(ctx, name, m, oauthConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to configure Aliyun Drive: %w", err)
		}
	}

	// Create filesystem
	f := &Fs{
		name: name,
		root: root,
		opt:  *opt,
		srv:  rest.NewClient(client).SetRoot(rootURL),
		m:    m,
		pacer: fs.NewPacer(
			context.Background(),
			pacer.NewDefault(
				pacer.MinSleep(minSleep),
				pacer.MaxSleep(maxSleep),
				pacer.DecayConstant(decayConstant),
			),
		),
	}
	f.features = (&fs.Features{
		CaseInsensitive:         true,
		CanHaveEmptyDirectories: true,
	}).Fill(ctx, f)
	f.srv.SetErrorHandler(errorHandler)

	// Set up authentication
	if f.opt.AccessToken != "" {
		f.srv.SetHeader("Authorization", "Bearer "+f.opt.AccessToken)
	}
	// Set up token renewal if using OAuth2
	if ts != nil {
		f.tokenRenewer = oauthutil.NewRenew(f.String(), ts, func() error {
			_, err = f.GetUserInfo(ctx)
			return err
		})
	}

	// Set the root folder ID
	if f.opt.RootFolderID != "" {
		f.rootID = f.opt.RootFolderID
	} else {
		f.rootID = "root"
	}
	f.dirCache = dircache.New(root, f.rootID, f)

	// Find the current root
	err = f.dirCache.FindRoot(ctx, false)
	if err != nil {
		// Assume it is a file
		newRoot, remote := dircache.SplitPath(root)
		tempF := *f
		tempF.dirCache = dircache.New(newRoot, f.rootID, &tempF)
		tempF.root = newRoot
		// Make new Fs which is the parent
		err = tempF.dirCache.FindRoot(ctx, false)
		if err != nil {
			// No root so return old f
			return f, nil
		}
		_, err = tempF.newObjectWithInfo(ctx, remote, nil)
		if err != nil {
			if err == fs.ErrorObjectNotFound {
				// File doesn't exist so return old f
				return f, nil
			}
			return nil, err
		}
		f.features.Fill(ctx, &tempF)
		// XXX: update the old f here instead of returning tempF, since
		f.dirCache = tempF.dirCache
		// `features` were already filled with functions having *f as a receiver.
		f.root = tempF.root
		// return an error with an fs which points to the parent
		return f, fs.ErrorIsFile
	}

	// Get drive info using the OpenAPI client
	driveInfo, apiErr := f.GetDriveID(ctx)
	if apiErr != nil {
		return nil, fmt.Errorf("failed to get drive info: %v", apiErr)
	}
	f.driveID = driveInfo.DefaultDriveID

	return f, nil
}

// Return an Object from a path
//
// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(ctx context.Context, remote string, info *api.FileEntity) (fs.Object, error) {
	var err error
	o := &Object{
		fs:     f,
		remote: remote,
	}
	if info != nil {
		o.id = info.FileID
		o.parentID = info.ParentFileID
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
	items, err := f.listAll(ctx, pathID)
	if err != nil {
		return "", false, err
	}
	for _, item := range items {
		if strings.EqualFold(item.FileName, leaf) {
			pathIDOut = item.FileID
			found = true
			break
		}
	}

	return pathIDOut, found, err
}

// CreateDir makes a directory with pathID as parent and name leaf
func (f *Fs) CreateDir(ctx context.Context, pathID string, leaf string) (string, error) {
	result, apiErr := f.MkDirectory(ctx, f.driveID, pathID, leaf)
	if apiErr != nil {
		return "", fmt.Errorf("error creating directory: %v", apiErr)
	}
	return result.FileID, nil
}

// Lists the directory required calling the user function on each item found
//
// If the user fn ever returns true then it early exits with found = true
func (f *Fs) listAll(ctx context.Context, directoryID string) ([]*api.FileEntity, error) {
	result, apiErr := f.FileList(ctx, &api.FileListParam{
		DriveID:      f.driveID,
		ParentFileID: directoryID,
		OrderBy:      "name",
		Limit:        100,
	})
	if apiErr != nil {
		return nil, fmt.Errorf("error listing directory: %v", apiErr)
	}

	return result.Items, nil
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
func (f *Fs) List(ctx context.Context, dir string) (fs.DirEntries, error) {
	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return nil, err
	}

	// Get directory entries from the dirID
	fileList, apiErr := f.FileListGetAll(ctx, &api.FileListParam{
		DriveID:        f.driveID,
		ParentFileID:   directoryID,
		OrderBy:        "name",
		OrderDirection: "ASC",
	}, -1)
	if apiErr != nil {
		return nil, fmt.Errorf("error listing directory: %v", apiErr)
	}

	entries := make(fs.DirEntries, 0, len(fileList))
	for _, item := range fileList {
		remote := path.Join(dir, item.FileName)
		if item.FileType == api.ItemTypeFolder {
			// Parse UpdatedAt time
			modTime := time.Now()
			if item.UpdatedAt != "" {
				parsedTime, err := time.Parse(time.RFC3339, item.UpdatedAt)
				if err == nil {
					modTime = parsedTime.UTC()
				} else {
					fs.Debugf(remote, "Failed to parse UpdatedAt time %q: %v", item.UpdatedAt, err)
				}
			}

			d := fs.NewDir(remote, modTime).SetID(item.FileID).SetParentID(directoryID)
			entries = append(entries, d)
		} else {
			o, createErr := f.newObjectWithInfo(ctx, remote, item)
			if createErr == nil {
				entries = append(entries, o)
			}
		}
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
	// Create a new object
	o = &Object{
		fs:       f,
		remote:   remote,
		size:     size,
		modTime:  modTime,
		parentID: directoryID,
	}

	return o, leaf, directoryID, nil
}

// Put the object
//
// Copy the reader in to the new object which is returned.
//
// The new object may have been created if an error is returned
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	existingObj, err := f.NewObject(ctx, src.Remote())
	switch err {
	case nil:
		return existingObj, existingObj.Update(ctx, in, src, options...)
	case fs.ErrorObjectNotFound:
		return f.PutUnchecked(ctx, in, src, options...)
	default:
		return nil, err
	}
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.Put(ctx, in, src, options...)
}

// PutUnchecked the object into the container
//
// This will produce an error if the object already exists.
//
// Copy the reader in to the new object which is returned.
//
// The new object may have been created if an error is returned
func (f *Fs) PutUnchecked(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	remote := src.Remote()
	size := src.Size()
	modTime := src.ModTime(ctx)

	o, _, _, err := f.createObject(ctx, remote, modTime, size)
	if err != nil {
		return nil, err
	}
	return o, o.Update(ctx, in, src, options...)
}

// Mkdir creates the container if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	_, err := f.dirCache.FindDir(ctx, dir, true)
	return err
}

// deleteObject removes an object by ID
func (f *Fs) deleteObject(ctx context.Context, id string) error {
	_, apiErr := f.FileDelete(ctx, &api.FileBatchActionParam{
		DriveID: f.driveID,
		FileID:  id,
	})
	if apiErr != nil {
		return fmt.Errorf("error deleting object: %v", apiErr)
	}

	return nil
}

// purgeCheck removes the root directory, if check is set then it
// refuses to do so if it has anything in
func (f *Fs) purgeCheck(ctx context.Context, dir string) error {
	root := path.Join(f.root, dir)
	if root == "" {
		return errors.New("can't purge root directory")
	}
	fileID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return err
	}

	err = f.deleteObject(ctx, fileID)
	if err != nil {
		return fmt.Errorf("rmdir failed: %w", err)
	}
	f.dirCache.FlushDir(dir)

	return nil
}

// Rmdir deletes the root folder
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir)
}

// Precision return the precision of this Fs
func (f *Fs) Precision() time.Duration {
	return fs.ModTimeNotSupported
	// meaning that the modification times from the backend shouldn't be used for syncing
	// as they can't be set.
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
	err := srcObj.readMetaData(ctx)
	if err != nil {
		return nil, err
	}

	srcPath := srcObj.remote
	dstPath := remote
	if strings.EqualFold(srcPath, dstPath) {
		return nil, fmt.Errorf("can't copy %q -> %q as are same name when lowercase", srcPath, dstPath)
	}

	// Create temporary object
	dstObj, _, directoryID, err := f.createObject(ctx, remote, srcObj.modTime, srcObj.size)
	if err != nil {
		return nil, err
	}

	item, apiErr := f.FileCopy(ctx, &api.FileCopyParam{
		DriveID:        f.driveID,
		FileID:         srcObj.id,
		ToParentFileID: directoryID,
	})
	if apiErr != nil {
		return nil, fmt.Errorf("error copying file: %v", apiErr)
	}

	info, apiErr := f.FileInfoByID(ctx, f.driveID, item.FileID)
	if apiErr != nil {
		return nil, fmt.Errorf("error getting copied file: %v", apiErr)
	}
	err = dstObj.setMetaData(info)
	if err != nil {
		return nil, err
	}
	return dstObj, nil
}

// Purge deletes all the files and the container
//
// Optional interface: Only implement this if you have a way of
// deleting all the files quicker than just running Remove() on the
// result of List()
func (f *Fs) Purge(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir)
}

// move a file or folder
func (f *Fs) move(ctx context.Context, id, directoryID string) (*api.FileEntity, error) {
	// Use OpenAPI client for file move operation
	moveFileParam := &api.FileMoveParam{
		DriveID:        f.driveID,
		FileID:         id,
		ToParentFileID: directoryID,
	}

	result, apiErr := f.FileMove(ctx, moveFileParam)
	if apiErr != nil {
		return nil, fmt.Errorf("error moving file: %v", apiErr)
	}

	// Convert to FileEntity
	fileEntity, apiErr := f.FileInfoByID(ctx, f.driveID, result.FileID)
	if apiErr != nil {
		return nil, fmt.Errorf("error getting moved file: %v", apiErr)
	}

	return fileEntity, nil
}

// About gets quota information
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	spaceInfo, apiErr := f.GetSpaceInfo(ctx)
	if apiErr != nil {
		return nil, fmt.Errorf("error getting space info: %v", apiErr)
	}

	usage := &fs.Usage{
		Used:  fs.NewUsageValue(spaceInfo.PersonalSpaceInfo.UsedSize),
		Total: fs.NewUsageValue(spaceInfo.PersonalSpaceInfo.TotalSize),
		Free:  fs.NewUsageValue(spaceInfo.PersonalSpaceInfo.TotalSize - spaceInfo.PersonalSpaceInfo.UsedSize),
	}

	return usage, nil
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
	dstObj, _, directoryID, err := f.createObject(ctx, remote, srcObj.modTime, srcObj.size)
	if err != nil {
		return nil, err
	}
	// Do the move
	info, err := f.move(ctx, srcObj.id, directoryID)
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
// If destination exists then return fs.ErrorDirExists
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) error {
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debugf(srcFs, "Can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}

	srcID, _, _, dstDirectoryID, _, err := f.dirCache.DirMove(ctx, srcFs.dirCache, srcFs.root, srcRemote, f.root, dstRemote)
	if err != nil {
		return err
	}

	// Do the move
	_, err = f.move(ctx, srcID, dstDirectoryID)
	if err != nil {
		return err
	}
	srcFs.dirCache.FlushDir(srcRemote)
	return nil
}

// Shutdown shutdown the fs
func (f *Fs) Shutdown(ctx context.Context) error {
	f.tokenRenewer.Shutdown()
	return nil
}

// DirCacheFlush resets the directory cache - used in testing as an
// optional interface
func (f *Fs) DirCacheFlush() {
	f.dirCache.ResetRoot()
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.SHA1)
}

// UserInfo implements fs.UserInfoer.
func (f *Fs) UserInfo(ctx context.Context) (map[string]string, error) {
	user, apiErr := f.GetUserInfo(ctx)
	if apiErr != nil {
		return nil, fmt.Errorf("error getting user info: %v", apiErr)
	}

	userInfo := map[string]string{
		"UserName": user.UserName,
		"Email":    user.Email,
		"Phone":    user.Phone,
		"Role":     user.Role,
		"Status":   user.Status,
		"Nickname": user.Nickname,
	}

	return userInfo, nil
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

// Hash returns the SHA-1 of an object returning a lowercase hex string
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	if t != hash.SHA1 {
		return "", hash.ErrUnsupported
	}
	return o.sha1, nil
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
func (o *Object) setMetaData(info *api.FileEntity) error {
	if info.FileType == api.ItemTypeFolder {
		return fs.ErrorIsDir
	}
	if info.FileType != api.ItemTypeFile {
		return fmt.Errorf("%q is %q: %w", o.remote, info.FileType, fs.ErrorNotAFile)
	}
	o.hasMetaData = true
	o.size = int64(info.FileSize)
	o.sha1 = strings.ToLower(info.ContentHash)
	o.modTime = parseTime(info.CreatedAt)
	o.id = info.FileID
	o.parentID = info.ParentFileID
	return nil
}

// readMetaData gets the metadata if it hasn't already been fetched
//
// it also sets the info
func (o *Object) readMetaData(ctx context.Context) error {
	if o.hasMetaData {
		return nil
	}

	leaf, dirID, err := o.fs.dirCache.FindPath(ctx, o.remote, false)
	if err != nil {
		if err == fs.ErrorDirNotFound {
			return fs.ErrorObjectNotFound
		}
		return err
	}

	list, err := o.fs.listAll(ctx, dirID)
	if err != nil {
		return err
	}

	found := false
	var info *api.FileEntity
	for _, item := range list {
		if item.FileType == api.ItemTypeFile && strings.EqualFold(item.FileName, leaf) {
			info = item
			found = true
			break
		}
	}

	if !found {
		return fs.ErrorObjectNotFound
	}

	return o.setMetaData(info)
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
func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
	return fs.ErrorCantSetModTime
}

// Storable returns a boolean showing whether this object storable
func (o *Object) Storable() bool {
	return true
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	if o.id == "" {
		return nil, errors.New("can't download: no id")
	}
	if o.size == 0 {
		// zero-byte objects may have no download link
		return io.NopCloser(bytes.NewBuffer([]byte(nil))), nil
	}

	// Get download URL from OpenAPI
	downloadURL, apiErr := o.fs.GetFileDownloadURL(ctx, &api.GetFileDownloadURLParam{
		DriveID: o.fs.driveID,
		FileID:  o.id,
	})
	if apiErr != nil {
		return nil, fmt.Errorf("error getting download URL: %v", apiErr)
	}

	if downloadURL == nil || downloadURL.URL == "" {
		return nil, errors.New("empty download URL")
	}

	if downloadURL.URL == "" {
		return nil, errors.New("forbidden to download - check sharing permission")
	}
	req, err := http.NewRequestWithContext(ctx, "GET", downloadURL.URL, nil)
	if err != nil {
		return nil, err
	}
	fs.FixRangeOption(options, o.size)
	fs.OpenOptionAddHTTPHeaders(req.Header, options)
	var res *http.Response
	if o.size == 0 {
		// Don't supply range requests for 0 length objects as they always fail
		delete(req.Header, "Range")
	}
	err = o.fs.pacer.Call(func() (bool, error) {
		res, err = o.fs.srv.Do(req)
		return shouldRetry(ctx, res, err)
	})
	if err != nil {
		return nil, err
	}
	return res.Body, nil
}

// Update the object with the contents of the io.Reader, modTime and size
//
// If existing is set then it updates the object rather than creating a new one.
//
// The new object may have been created if an error is returned.
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	if o.fs.tokenRenewer != nil {
		o.fs.tokenRenewer.Start()
		defer o.fs.tokenRenewer.Stop()
	}

	// Create the directory for the object if it doesn't exist
	leaf, directoryID, err := o.fs.dirCache.FindPath(ctx, o.Remote(), true)
	if err != nil {
		return err
	}

	return o.upload(ctx, in, leaf, directoryID, src.Size())
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	return o.fs.deleteObject(ctx, o.id)
}

// ParentID returns the ID of the parent directory
func (o *Object) ParentID() string {
	return o.parentID
}

// ID returns the ID of the Object if known, or "" if not
func (o *Object) ID() string {
	return o.id
}

// Check interfaces
var (
	_ fs.Fs              = (*Fs)(nil)
	_ fs.Purger          = (*Fs)(nil)
	_ fs.PutStreamer     = (*Fs)(nil)
	_ fs.Copier          = (*Fs)(nil)
	_ fs.Abouter         = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ fs.DirCacheFlusher = (*Fs)(nil)
	_ fs.Shutdowner      = (*Fs)(nil)
	_ fs.UserInfoer      = (*Fs)(nil)
	_ fs.IDer            = (*Object)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.ParentIDer      = (*Object)(nil)
)
