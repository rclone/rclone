package adrive

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
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
	"github.com/tickstep/aliyunpan-api/aliyunpan"
	"github.com/tickstep/aliyunpan-api/aliyunpan_open"
	"github.com/tickstep/aliyunpan-api/aliyunpan_open/openapi"
	"golang.org/x/oauth2"
)

const (
	minSleep      = 10 * time.Millisecond
	maxSleep      = 2 * time.Second
	decayConstant = 2 // bigger for slower decay, exponential

	defaultChunkSize = int64(524288)
	maxPartNum       = 10000

	rootURL  = "https://openapi.alipan.com"
	authURL  = "https://openapi.alipan.com/oauth/authorize"
	tokenURL = "https://openapi.alipan.com/oauth/access_token"

	GrantTypeAuthorizationCode = "authorization_code"
	GrantTypeRefreshToken      = "refresh_token"

	ItemTypeFile   = "file"
	ItemTypeFolder = "folder"
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

// Fs represents a remote adrive
type Fs struct {
	name         string             // name of this remote
	root         string             // the path we are working on
	opt          Options            // parsed options
	features     *fs.Features       // optional features
	client       *rest.Client       // Aliyun Drive client
	dirCache     *dircache.DirCache // Map of directory path to directory id
	pacer        *fs.Pacer          // pacer for API calls
	tokenRenewer *oauthutil.Renew   // renew the token on expiry
	m            configmap.Mapper
	driveID      string
	rootID       string // the id of the root folder
	openClient   *aliyunpan_open.OpenPanClient
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

// Lists the directory required calling the user function on each item found
//
// If the user fn ever returns true then it early exits with found = true
func (f *Fs) listAll(ctx context.Context, directoryID string) (items aliyunpan.FileList, err error) {
	// Convert to API items
	param := &aliyunpan.FileListParam{
		DriveId:      f.driveID,
		ParentFileId: directoryID,
		OrderBy:      "name",
		Limit:        100,
	}

	result, apiErr := f.openClient.FileList(param)
	if apiErr != nil {
		return nil, fmt.Errorf("error listing directory: %v", apiErr)
	}

	return result.FileList, nil
}

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

// deleteObject removes an object by ID
func (f *Fs) deleteObject(ctx context.Context, id string) error {
	_, err := f.openClient.FileDelete(&aliyunpan.FileBatchActionParam{
		DriveId: f.driveID,
		FileId:  id,
	})

	return err
}

// Creates from the parameters passed in a half finished Object which
// must have setMetaData called on it
//
// Returns the object, leaf, dirID and error.
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

// Return an Object from a path
//
// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(ctx context.Context, remote string, info *aliyunpan.FileEntity) (fs.Object, error) {
	o := &Object{
		fs:       f,
		remote:   remote,
		id:       info.FileId,
		parentID: info.ParentFileId,
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

// moveObject moves an object by ID
func (f *Fs) move(ctx context.Context, id, leaf, directoryID string) (info *aliyunpan.FileEntity, err error) {
	// Use OpenAPI client for file move operation
	moveFileParam := &aliyunpan.FileMoveParam{
		DriveId:        f.driveID,
		FileId:         id,
		ToParentFileId: directoryID,
	}

	result, apiErr := f.openClient.FileMove(moveFileParam)
	if apiErr != nil {
		return nil, fmt.Errorf("error moving file: %v", apiErr)
	}

	// Convert to FileEntity
	fileEntity, apiErr := f.openClient.FileInfoById(f.driveID, result.FileId)
	if apiErr != nil {
		return nil, fmt.Errorf("error getting moved file: %v", apiErr)
	}

	return fileEntity, nil
}

// setMetaData sets the metadata from info
func (o *Object) setMetaData(info *aliyunpan.FileEntity) (err error) {
	if info.FileType == ItemTypeFolder {
		return fs.ErrorIsDir
	}
	if info.FileType != ItemTypeFile {
		return fmt.Errorf("%q is %q: %w", o.remote, info.FileType, fs.ErrorNotAFile)
	}
	o.hasMetaData = true
	o.size = int64(info.FileSize)
	o.sha1 = info.ContentHash
	o.modTime = parseTime(info.CreatedAt)
	o.id = info.FileId
	o.parentID = info.ParentFileId
	return nil
}

// readMetaData gets the metadata if it hasn't already been fetched
//
// it also sets the info
func (o *Object) readMetaData(ctx context.Context) (err error) {
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
	var info aliyunpan.FileEntity
	for _, item := range list {
		if item.FileType == ItemTypeFile && strings.EqualFold(item.FileName, leaf) {
			info = *item
			break
		}
	}

	return o.setMetaData(&info)
}

// ------------------------------------------------------------

// About gets quota information
func (f *Fs) About(ctx context.Context) (usage *fs.Usage, err error) {
	about, err := f.openClient.GetUserInfo()
	if err != nil {
		return nil, err
	}

	usage = &fs.Usage{
		Used:  fs.NewUsageValue(int64(about.UsedSize)),
		Total: fs.NewUsageValue(int64(about.TotalSize)),
		Free:  fs.NewUsageValue(int64(about.TotalSize - about.UsedSize)),
	}

	return usage, nil
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

	item, err := f.openClient.FileCopy(&aliyunpan.FileCopyParam{
		DriveId:        f.driveID,
		ToParentFileId: directoryID,
		FileId:         srcObj.id,
	})
	if err != nil {
		return nil, err
	}

	info, err := f.openClient.FileInfoById(f.driveID, item.FileId)
	if err != nil {
		return nil, err
	}
	err = dstObj.setMetaData(info)
	if err != nil {
		return nil, err
	}
	return dstObj, nil
}

// DirCacheFlush implements fs.DirCacheFlusher.
func (f *Fs) DirCacheFlush() {
	f.dirCache.ResetRoot()
}

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

// Precision return the precision of this Fs
func (f *Fs) Precision() time.Duration {
	return fs.ModTimeNotSupported
	// meaning that the modification times from the backend shouldn't be used for syncing
	// as they can't be set.
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.SHA1)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// CreateDir implements dircache.DirCacher.
func (f *Fs) CreateDir(ctx context.Context, pathID string, leaf string) (newID string, err error) {
	var info *aliyunpan.FileEntity

	// TODO
	_, err = f.openClient.CreateUploadFile(&aliyunpan.CreateFileUploadParam{
		DriveId:       f.driveID,
		ParentFileId:  pathID,
		Name:          leaf,
		Type:          ItemTypeFolder,
		CheckNameMode: "refuse",
	})
	if err != nil {
		return "", err
	}

	// req := api.CreateFileRequest{
	// 	DriveID:       f.driveID,
	// 	ParentFileID:  pathID,
	// 	Name:          leaf,
	// 	Type:          ItemTypeFolder,
	// 	CheckNameMode: "refuse",
	// }
	// err = f.pacer.Call(func() (bool, error) {
	// 	resp, err = f.client.CallJSON(ctx, &opts, &req, &info)
	// 	return shouldRetry(ctx, resp, err)
	// })
	// if err != nil {
	// 	return "", err
	// }
	return info.FileId, nil
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
			pathIDOut = item.FileId
			found = true
			break
		}
	}

	return pathIDOut, found, err
}

// List implements fs.Fs.
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return nil, err
	}

	// Get directory entries from the dirID
	fileList, err := f.openClient.FileListGetAll(&aliyunpan.FileListParam{
		DriveId:        f.driveID,
		ParentFileId:   directoryID,
		OrderBy:        "name",
		OrderDirection: "ASC",
	}, -1)
	if err != nil {
		return nil, err
	}

	entries = make(fs.DirEntries, 0, len(fileList))
	for _, item := range fileList {
		remote := path.Join(dir, item.FileName)
		if item.FileType == ItemTypeFolder {
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

			d := fs.NewDir(remote, modTime).SetID(item.FileId).SetParentID(directoryID)
			entries = append(entries, d)
		} else {
			o, createErr := f.newObjectWithInfo(ctx, remote, item)
			if createErr == nil {
				entries = append(entries, o)
			}
		}
	}

	if err != nil {
		return nil, err
	}

	return entries, nil
}

// NewObject implements fs.Fs.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	return f.newObjectWithInfo(ctx, remote, nil)
}

// Put implements fs.Fs.
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	// TODO
	o := &Object{}
	return o, o.Update(ctx, in, src, options...)

	// 	existingObj, err := f.NewObject(ctx, src.Remote())
	// switch {
	// case err == nil:
	// 	return existingObj.Update(ctx, in, src, options...)
	// case errors.Is(err, fs.ErrorObjectNotFound):
	// 	// Not found so create it
	// 	return f.PutUnchecked(ctx, in, src, options...)
	// default:
	// 	return nil, err
	// }
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.Put(ctx, in, src, options...)
}

// PutUnchecked uploads the object
func (f *Fs) PutUnchecked(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	remote := src.Remote()
	modTime := src.ModTime(ctx)
	size := src.Size()

	// Create a new object
	o, leaf, directoryID, err := f.createObject(ctx, remote, modTime, size)
	if err != nil {
		return nil, err
	}

	// Save the object in the parent directory
	tempFile, err := os.CreateTemp("", "rclone-adrive-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() {
		_ = tempFile.Close()
		_ = os.Remove(tempFile.Name())
	}()

	// Copy the contents to the temp file
	_, err = io.Copy(tempFile, in)
	if err != nil {
		return nil, fmt.Errorf("failed to write to temp file: %w", err)
	}

	// Seek to the beginning for reading
	_, err = tempFile.Seek(0, io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("failed to seek temp file: %w", err)
	}

	// Create the file upload param
	// uploadParam := aliyunpan.FileUploadParam{
	// 	DriveId:       f.driveID,
	// 	Name:          leaf,
	// 	Size:          size,
	// 	ContentHash:   "", // Will be calculated by the SDK
	// 	ParentFileId:  directoryID,
	// 	CheckNameMode: f.opt.CheckNameMode,
	// 	CreateTime:    modTime,
	// 	LocalFilePath: tempFile.Name(),
	// 	Content:       nil, // Not using content directly to avoid memory issues
	// }

	// Upload the file
	// TODO
	result, apiErr := f.openClient.CreateUploadFile(&aliyunpan.CreateFileUploadParam{
		DriveId:       f.driveID,
		Name:          leaf,
		Size:          size,
		ContentHash:   "", // Will be calculated by the SDK
		ParentFileId:  directoryID,
		CheckNameMode: f.opt.CheckNameMode,
	})
	if apiErr != nil {
		return nil, fmt.Errorf("error uploading file: %v", apiErr)
	}

	// Get file metadata
	FileEntity, apiErr := o.fs.openClient.FileInfoById(o.fs.driveID, result.FileId)
	if apiErr != nil {
		return nil, fmt.Errorf("error getting file metadata after upload: %v", apiErr)
	}

	// Set metadata of object
	o.id = FileEntity.FileId
	o.sha1 = FileEntity.ContentHash
	o.size = size
	o.modTime = modTime
	o.parentID = directoryID
	o.hasMetaData = true

	return o, nil
}

// Mkdir creates the container if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	if dir == "" || dir == "." {
		return nil
	}

	parts := strings.Split(strings.Trim(dir, "/"), "/")
	parentID := f.opt.RootFolderID

	// Create each directory in the path if it doesn't exist
	for i, part := range parts {
		currentPath := strings.Join(parts[:i+1], "/")

		// Try to get the directory first
		info, err := f.openClient.FileInfoByPath(f.driveID, currentPath)
		if err != nil {
			// Directory doesn't exist, create it
			createParam := &aliyunpan.FileCreateFolderParam{
				DriveId:       f.driveID,
				Name:          part,
				ParentFileId:  parentID,
				CheckNameMode: f.opt.CheckNameMode,
			}

			result, apiErr := f.openClient.FileCreateFolder(createParam)
			if apiErr != nil {
				return fmt.Errorf("error creating directory %q: %v", part, apiErr)
			}

			// Use the new directory ID as parent for the next iteration
			parentID = result.FileId
		} else {
			// Directory exists, use its ID as parent for the next iteration
			if info.FileType != ItemTypeFolder {
				return fmt.Errorf("%q already exists but is not a directory", currentPath)
			}
			parentID = info.FileId
		}
	}

	return nil
}

// Rmdir implements fs.Fs.
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir)
}

// Move implements fs.Mover.
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't move - not same remote type")
		return nil, fs.ErrorCantMove
	}

	// Create temporary object
	dstObj, leaf, directoryID, err := f.createObject(ctx, remote, srcObj.modTime, srcObj.size)
	if err != nil {
		return nil, err
	}
	// Do the move
	info, err := f.move(ctx, srcObj.id, leaf, directoryID)
	if err != nil {
		return nil, err
	}

	err = dstObj.setMetaData(info)
	if err != nil {
		return nil, err
	}
	return dstObj, nil
}

// Purge implements fs.Purger.
func (f *Fs) Purge(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir)
}

// Shutdown shutdown the fs
func (f *Fs) Shutdown(ctx context.Context) error {
	f.tokenRenewer.Shutdown()
	return nil
}

// UserInfo implements fs.UserInfoer.
func (f *Fs) UserInfo(ctx context.Context) (userInfo map[string]string, err error) {
	user, err := f.openClient.GetUserInfo()
	if err != nil {
		return nil, err
	}

	userInfo = map[string]string{
		"UserName": user.UserName,
		"Email":    user.Email,
		"Phone":    user.Phone,
		"Role":     string(user.Role),
		"Status":   string(user.Status),
		"Nickname": user.Nickname,
	}

	userInfo["Expire"] = user.ThirdPartyVipExpire
	userInfo["ThirdPartyVip"] = strconv.FormatBool(user.ThirdPartyVip)
	userInfo["ThirdPartyVipExpire"] = user.ThirdPartyVipExpire

	return userInfo, nil
}

// ------------------------------------------------------------

// ID returns the ID of the Object if known, or "" if not
func (o *Object) ID() string {
	return o.id
}

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

// Remote implements fs.Object.
func (o *Object) Remote() string {
	return o.remote
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

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	err := o.readMetaData(context.TODO())
	if err != nil {
		fs.Logf(o, "Failed to read metadata: %v", err)
		return 0
	}
	return o.size
}

// Hash returns the selected checksum of the file
// If no checksum is available it returns ""
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	if t != hash.SHA1 {
		return "", hash.ErrUnsupported
	}
	return o.sha1, nil
}

// Storable returns a boolean showing whether this object storable
func (o *Object) Storable() bool {
	return true
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
	return fs.ErrorCantSetModTime
}

// Open implements fs.Object.
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	if o.id == "" {
		return nil, errors.New("can't open without object ID")
	}

	// Get download URL from OpenAPI
	downloadUrl, apiErr := o.fs.openClient.GetFileDownloadUrl(&aliyunpan.GetFileDownloadUrlParam{
		DriveId: o.fs.driveID,
		FileId:  o.id,
	})
	if apiErr != nil {
		return nil, fmt.Errorf("error getting download URL: %v", apiErr)
	}

	if downloadUrl == nil || downloadUrl.Url == "" {
		return nil, errors.New("empty download URL")
	}

	// TODO
	resp, err := http.Get(downloadUrl.Url)

	fs.FixRangeOption(options, o.Size())

	return resp.Body, err
}

// Update implements fs.Object.
// TODO
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	size := src.Size()
	modTime := src.ModTime(ctx)
	remote := o.Remote()

	// Save the object in the parent directory
	tempFile, err := os.CreateTemp("", "rclone-adrive-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() {
		_ = tempFile.Close()
		_ = os.Remove(tempFile.Name())
	}()

	// Copy the contents to the temp file
	_, err = io.Copy(tempFile, in)
	if err != nil {
		return fmt.Errorf("failed to write to temp file: %w", err)
	}

	// Seek to the beginning for reading
	_, err = tempFile.Seek(0, io.SeekStart)
	if err != nil {
		return fmt.Errorf("failed to seek temp file: %w", err)
	}

	// Get leaf name and parent ID
	var leaf, directoryID string
	if o.id != "" {
		// Try to get the current parent directory from the existing object
		info, err := o.Stat(ctx)
		if err != nil {
			if errors.Is(err, fs.ErrorObjectNotFound) {
				// If the object doesn't exist, we need to find the directory to create it in
				remoteDir := path.Dir(remote)
				remoteName := path.Base(remote)
				leaf, directoryID, err = o.fs.resolvePath(ctx, remoteDir)
				if err != nil {
					return fmt.Errorf("error resolving path: %w", err)
				}
				leaf = remoteName
			} else {
				return fmt.Errorf("error getting current object info: %w", err)
			}
		} else {
			// Use the existing parent ID and name
			directoryID = info.ParentFileId
			leaf = info.FileName
		}
	} else {
		// Find the directory for this object
		remoteDir := path.Dir(remote)
		remoteName := path.Base(remote)
		leaf, directoryID, err = o.fs.resolvePath(ctx, remoteDir)
		if err != nil {
			return fmt.Errorf("error resolving path: %w", err)
		}
		leaf = remoteName
	}

	// Create the file upload param
	uploadParam := &aliyunpan.FileUploadParam{
		DriveId:       o.fs.driveID,
		Name:          leaf,
		Size:          size,
		ContentHash:   "", // Will be calculated by the SDK
		ParentFileId:  directoryID,
		CheckNameMode: o.fs.opt.CheckNameMode,
		CreateTime:    modTime,
		LocalFilePath: tempFile.Name(),
		Content:       nil, // Not using content directly to avoid memory issues
	}

	var fileID string
	var apiErr *aliyunpan.ApiError

	// If updating an existing file
	if o.id != "" {
		// Set the file ID for update
		uploadParam.FileId = o.id

		// Update existing file
		fileID, apiErr = o.fs.openClient.FileUpload(uploadParam)
	} else {
		// Upload new file
		fileID, apiErr = o.fs.openClient.FileUpload(uploadParam)
	}

	if apiErr != nil {
		return fmt.Errorf("error uploading file: %v", apiErr)
	}

	// Get file metadata
	FileEntity, apiErr := o.fs.openClient.FileInfoById(o.fs.driveID, fileID)
	if apiErr != nil {
		return fmt.Errorf("error getting file metadata after upload: %v", apiErr)
	}

	// Set metadata of object
	o.id = FileEntity.FileId
	o.sha1 = FileEntity.ContentHash
	o.size = size
	o.modTime = modTime
	o.parentID = directoryID
	o.hasMetaData = true

	return nil
}

// Remove implements fs.Object.
func (o *Object) Remove(ctx context.Context) error {
	return o.fs.deleteObject(ctx, o.id)
}

// ParentID implements fs.ParentIDer.
func (o *Object) ParentID() string {
	return o.parentID
}

// Stat returns the stat info of the object
func (o *Object) Stat(ctx context.Context) (info *aliyunpan.FileEntity, err error) {
	// Try to get from id
	if o.id != "" {
		info, err = o.fs.openClient.FileInfoById(o.fs.driveID, o.id)
		if err != nil {
			// Check for file not found type errors
			return nil, fs.ErrorObjectNotFound
		}
		return info, nil
	}

	// Otherwise get by path
	leaf, directoryID, err := o.fs.resolvePath(ctx, o.remote)
	if err != nil {
		return nil, fmt.Errorf("error resolving path: %w", err)
	}

	info, err = o.fs.openClient.FileInfoByPath(o.fs.driveID, path.Join(directoryID, leaf))
	if err != nil {
		// Check for file not found type errors
		return nil, fs.ErrorObjectNotFound
	}

	return info, nil
}

// resolvePath resolves a remote path to its leaf and directory ID
func (f *Fs) resolvePath(ctx context.Context, remote string) (leaf, directoryID string, err error) {
	// Find the parent directory
	if remote == "" {
		return "", f.opt.RootFolderID, nil
	}

	// Split the path into directory and leaf name
	dir, leaf := path.Split(remote)
	if dir == "" {
		// No directory, so use root
		return leaf, f.opt.RootFolderID, nil
	}

	// Find the directory ID
	dir = strings.TrimSuffix(dir, "/")
	info, err := f.openClient.FileInfoByPath(f.driveID, dir)
	if err != nil {
		// Directory not found
		return "", "", fs.ErrorDirNotFound
	}

	if info.FileType != ItemTypeFolder {
		return "", "", fs.ErrorIsFile
	}

	return leaf, info.FileId, nil
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

func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	// Create HTTP client
	client := fshttp.NewClient(ctx)

	var ts *oauthutil.TokenSource
	var apiToken openapi.ApiToken
	var apiConfig openapi.ApiConfig

	if opt.ClientID != "" && opt.ClientSecret != "" {
		// Use OpenAPI with client credentials
		if opt.AccessToken == "" {
			return nil, fmt.Errorf("access_token is required for OpenAPI authentication")
		}

		apiToken = openapi.ApiToken{
			AccessToken: opt.AccessToken,
		}

		if opt.ExpiresAt != "" {
			expireSec, err := strconv.ParseInt(opt.ExpiresAt, 10, 64)
			if err == nil {
				apiToken.ExpiredAt = expireSec
			}
		}

		apiConfig = openapi.ApiConfig{
			ClientId:     opt.ClientID,
			ClientSecret: opt.ClientSecret,
		}
	} else if opt.AccessToken == "" {
		// Standard OAuth2 flow
		client, ts, err = oauthutil.NewClient(ctx, name, m, oauthConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to configure Aliyun Drive: %w", err)
		}
	} else {
		// Direct token usage (legacy)
		apiToken = openapi.ApiToken{
			AccessToken: opt.AccessToken,
		}

		if opt.ExpiresAt != "" {
			expireSec, err := strconv.ParseInt(opt.ExpiresAt, 10, 64)
			if err == nil {
				apiToken.ExpiredAt = expireSec
			}
		}
	}

	// Create filesystem
	f := &Fs{
		name:   name,
		root:   root,
		opt:    *opt,
		client: rest.NewClient(client),
		m:      m,
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
	f.client.SetErrorHandler(errorHandler)
	f.client.SetRoot(rootURL)

	// Set up authentication
	if f.opt.AccessToken != "" {
		f.client.SetHeader("Authorization", "Bearer "+f.opt.AccessToken)
	}

	// Set up OpenAPI client
	f.openClient = aliyunpan_open.NewOpenPanClient(apiConfig, apiToken, func(userId string, newToken openapi.ApiToken) error {
		// Update token in config
		m.Set("access_token", newToken.AccessToken)
		m.Set("expires_at", fmt.Sprintf("%d", newToken.ExpiredAt))
		return nil
	})

	// Set up token renewal if using OAuth2
	if ts != nil {
		f.tokenRenewer = oauthutil.NewRenew(f.String(), ts, func() error {
			token, err := ts.Token()
			if err != nil {
				return err
			}

			// Update the OpenAPI client with the new token
			// TODO
			_ = openapi.ApiToken{
				AccessToken: token.AccessToken,
				ExpiredAt:   time.Now().Add(time.Duration(token.Expiry.Unix()-time.Now().Unix()) * time.Second).Unix(),
			}
			f.openClient.RefreshNewAccessToken()

			return nil
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
		// XXX: update the old f here instead of returning tempF, since
		// `features` were already filled with functions having *f as a receiver.
		f.dirCache = tempF.dirCache
		f.root = tempF.root
		// return an error with an fs which points to the parent
		return f, fs.ErrorIsFile
	}

	// Get drive info using the OpenAPI client
	userInfo, apiErr := f.openClient.GetUserInfo()
	if apiErr != nil {
		return nil, fmt.Errorf("failed to get user info: %v", apiErr)
	}

	// Set drive ID
	f.driveID = userInfo.FileDriveId

	return f, nil
}

// Call makes a call to the API using the params passed in
func (f *Fs) Call(ctx context.Context, opts *rest.Opts) (resp *http.Response, err error) {
	return f.CallWithPacer(ctx, opts, f.pacer)
}

// CallWithPacer makes a call to the API using the params passed in using the pacer passed in
func (f *Fs) CallWithPacer(ctx context.Context, opts *rest.Opts, pacer *fs.Pacer) (resp *http.Response, err error) {
	err = pacer.Call(func() (bool, error) {
		resp, err = f.client.Call(ctx, opts)
		return shouldRetry(ctx, resp, err)
	})
	return resp, err
}

var retryErrorCodes = []int{
	403,
	404,
	429, // Too Many Requests
	500, // Internal Server Error
	502, // Bad Gateway
	503, // Service Unavailable
	504, // Gateway Timeout
	509, // Bandwidth Limit Exceeded
}

// shouldRetry returns true if err is nil, or if it's a retryable error
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
	apiErr := new(api.Error)
	err := rest.DecodeJSON(resp, &apiErr)
	if err != nil {
		fs.Debugf(nil, "Failed to decode error response: %v", err)
		// If we can't decode the error response, create a basic error
		apiErr.Code = resp.StatusCode
		apiErr.Message = resp.Status
		return apiErr
	}

	// Ensure we have an error code and message
	if apiErr.Code == 0 {
		apiErr.Code = resp.StatusCode
	}
	if apiErr.Message == "" {
		apiErr.Message = resp.Status
	}

	return apiErr
}

// Check interfaces
var (
	_ fs.Abouter         = (*Fs)(nil)
	_ fs.Copier          = (*Fs)(nil)
	_ fs.DirCacheFlusher = (*Fs)(nil)
	_ fs.Fs              = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.Purger          = (*Fs)(nil)
	_ fs.PutStreamer     = (*Fs)(nil)
	_ fs.Shutdowner      = (*Fs)(nil)
	_ fs.UserInfoer      = (*Fs)(nil)
	_ fs.IDer            = (*Object)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.ParentIDer      = (*Object)(nil)
)
