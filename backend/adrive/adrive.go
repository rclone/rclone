package adrive

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
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
	"golang.org/x/oauth2"
)

const (
	minSleep = 10 * time.Millisecond
	maxSleep = 2 * time.Second

	decayConstant = 2 // bigger for slower decay, exponential

	rootURL  = "https://openapi.alipan.com"
	authURL  = "https://openapi.alipan.com/oauth/authorize"
	tokenURL = "https://openapi.alipan.com/oauth/access_token"

	GrantTypeAuthorizationCode = "authorization_code"
	GrantTypeRefreshToken      = "refresh_token"

	ItemTypeFile   = "file"
	ItemTypeFolder = "folder"

	VipIdentityMember = "member"
	VipIdentityVip    = "vip"
	VipIdentitySVip   = "svip"

	FeatureCode1080p = "hd.1080p"
	FeatureCode1440p = "hd.1080p.plus"

	TrialStatusNoTrial    = "noTrial"
	TrialStatusOnTrial    = "onTrial"
	TrialStatusEndTrial   = "endTrial"
	TrialStatusAllowTrial = "allowTrial"

	CheckNameModeIgnore     = "ignore"
	CheckNameModeAutoRename = "auto_rename"
	CheckNameModeRefuse     = "refuse"

	RemoveWayDelete = "delete"
	RemoveWayTrash  = "trash"
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
	srv          *rest.Client       // the connection to the server
	dirCache     *dircache.DirCache // Map of directory path to directory id
	pacer        *fs.Pacer          // pacer for API calls
	tokenRenewer *oauthutil.Renew   // renew the token on expiry
	m            configmap.Mapper
	driveID      string
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
	mimeType    string    // The object MIME type
	parent      string    // ID of the parent directory
}

// Options defines the configuration for this backend
type Options struct {
	Enc           encoder.MultiEncoder `config:"encoding"`
	RootFolderID  string               `config:"root_folder_id"`
	ChunkSize     fs.SizeSuffix        `config:"chunk_size"`
	RemoveWay     string               `config:"remove_way"`
	CheckNameMode string               `config:"check_name_mode"`
	TokenType     string               `config:"token_type"`
	AccessToken   string               `config:"access_token"`
	RefreshToken  string               `config:"refresh_token"`
	ExpiresAt     string               `config:"expires_at"`
}

// parsePath parses a box 'url'
func parsePath(path string) (root string) {
	root = strings.Trim(path, "/")
	return
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

func errorHandler(resp *http.Response) error {
	// Decode error response
	errResponse := new(api.Error)
	err := rest.DecodeJSON(resp, &errResponse)
	if err != nil {
		fs.Debugf(nil, "Couldn't decode error response: %v", err)
	}
	if errResponse.Message == "" {
		errResponse.Message = resp.Status
	}
	if errResponse.Code == "" {
		errResponse.Code = strconv.Itoa(resp.StatusCode)
	}
	return errResponse
}

// shouldRetry returns a boolean as to whether this resp and err
// deserve to be retried.  It returns the err as a convenience
func shouldRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	return fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// list the objects into the function supplied
//
// If directories is set it only sends directories
// User function to process a File item from listAll
//
// Should return true to finish processing
type listAllFn func(*api.Item) bool

// Lists the directory required calling the user function on each item found
//
// If the user fn ever returns true then it early exits with found = true
func (f *Fs) listAll(ctx context.Context, directoryID string, fn listAllFn) (found bool, err error) {
	opts := rest.Opts{
		Method: "POST",
		Path:   "/adrive/v1.0/openFile/list",
	}

	request := api.ListRequest{
		DriveID:      f.driveID,
		ParentFileID: directoryID,
	}

OUTER:
	for {
		var result api.List
		var resp *http.Response
		err = f.pacer.Call(func() (bool, error) {
			resp, err = f.srv.CallJSON(ctx, &opts, request, &result)
			return shouldRetry(ctx, resp, err)
		})
		if err != nil {
			return found, fmt.Errorf("couldn't list files: %w", err)
		}
		if len(result.Items) == 0 {
			break
		}
		for i := range result.Items {
			item := &result.Items[i]

			item.Name = f.opt.Enc.ToStandardName(item.Name)
			if fn(item) {
				found = true
				break OUTER
			}
		}
	}
	return
}

// deleteObject removes an object by ID
func (f *Fs) deleteObject(ctx context.Context, id string) error {
	path := " /adrive/v1.0/openFile"
	if f.opt.RemoveWay == RemoveWayTrash {
		path += "/recyclebin/trash"
	}

	if f.opt.RemoveWay == RemoveWayDelete {
		path += "/delete"
	}

	opts := rest.Opts{
		Method:     "DELETE",
		Path:       path,
		RootURL:    rootURL,
		NoResponse: true,
	}

	req := api.DeleteFile{
		DriveID: f.driveID,
		FileID:  id,
	}

	return f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, req, nil)
		return shouldRetry(ctx, resp, err)
	})
}

// Creates from the parameters passed in a half finished Object which
// must have setMetaData called on it
//
// Returns the object, leaf, dirID and error.
//
// Used to create new objects
func (f *Fs) createObject(ctx context.Context, remote string, modTime time.Time, size int64) (o *Object, leaf string, dirID string, err error) {
	// Create the directory for the object if it doesn't exist
	leaf, dirID, err = f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return
	}
	// Temporary Object under construction
	o = &Object{
		fs:      f,
		remote:  remote,
		parent:  dirID,
		size:    size,
		modTime: modTime,
	}
	return o, leaf, dirID, nil
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
func (f *Fs) newObjectWithInfo(ctx context.Context, remote string, info *api.Item) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	var err error
	if info != nil {
		err = o.setMetaData(info)
	} else {
		err = o.readMetaData(ctx) // reads info and meta, returning an error
	}
	if err != nil {
		return nil, err
	}
	return o, nil
}

// readMetaDataForPath reads the metadata from the path
func (f *Fs) readMetaDataForPath(ctx context.Context, path string) (info *api.Item, err error) {
	leaf, dirID, err := f.dirCache.FindPath(ctx, path, false)
	if err != nil {
		if err == fs.ErrorDirNotFound {
			return nil, fs.ErrorObjectNotFound
		}
		return nil, err
	}

	found, err := f.listAll(ctx, dirID, func(item *api.Item) bool {
		if item.Name == leaf {
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

// getUserInfo gets UserInfo from API
func (f *Fs) getUserInfo(ctx context.Context) (info *api.User, err error) {
	opts := rest.Opts{
		Method: "GET",
		Path:   "/oauth/users/info",
	}
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get userinfo: %w", err)
	}
	return
}

// getDriveInfo gets DriveInfo from API
func (f *Fs) getDriveInfo(ctx context.Context) error {
	opts := rest.Opts{
		Method: "POST",
		Path:   "/adrive/v1.0/user/getDriveInfo",
	}
	var resp *http.Response
	var info *api.DriveInfo
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("failed to get driveinfo: %w", err)
	}

	f.driveID = info.DefaultDriveID

	return nil
}

// getSpaceInfo gets SpaceInfo from API
func (f *Fs) getSpaceInfo(ctx context.Context) (info *api.SpaceInfo, err error) {
	opts := rest.Opts{
		Method: "POST",
		Path:   "/adrive/v1.0/user/getSpaceInfo",
	}
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get spaceinfo: %w", err)
	}
	return
}

// getVipInfo gets VipInfo from API
func (f *Fs) getVipInfo(ctx context.Context) (info *api.VipInfo, err error) {
	opts := rest.Opts{
		Method: "POST",
		Path:   "/business/v1.0/user/getVipInfo",
	}
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get vipinfo: %w", err)
	}
	return
}

func (f *Fs) move(ctx context.Context, id, leaf, directoryID string) (info *api.Item, err error) {
	opts := rest.Opts{
		Method: "POST",
		Path:   "/adrive/v1.0/openFile/move",
	}

	move := api.FileMoveCopy{
		DriveID:        f.driveID,
		FileID:         id,
		ToParentFileID: directoryID,
		NewName:        &leaf,
		CheckNameMode:  &f.opt.CheckNameMode,
	}

	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &move, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}

	return info, nil
}

func (f *Fs) createFile(ctx context.Context) error {
	// TODO
	return nil
}

func (f *Fs) getUploadUrl(ctx context.Context) error {
	// TODO
	return nil
}

// setMetaData sets the metadata from info
func (o *Object) setMetaData(info *api.Item) (err error) {
	if info.Type == ItemTypeFolder {
		return fs.ErrorIsDir
	}
	if info.Type != ItemTypeFile {
		return fmt.Errorf("%q is %q: %w", o.remote, info.Type, fs.ErrorNotAFile)
	}
	o.hasMetaData = true
	o.size = int64(info.Size)
	o.modTime = info.CreatedAt
	o.id = info.FileID
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

func (o *Object) upload(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	// TODO
	return nil
}

// ------------------------------------------------------------

// About implements fs.Abouter.
func (f *Fs) About(ctx context.Context) (usage *fs.Usage, err error) {
	about, err := f.getSpaceInfo(ctx)
	if err != nil {
		return nil, err
	}

	usage = &fs.Usage{
		Used:  fs.NewUsageValue(about.UsedSize),
		Total: fs.NewUsageValue(about.TotalSize),
		Free:  fs.NewUsageValue(about.TotalSize - about.UsedSize),
	}

	return usage, nil
}

// Copy implements fs.Copier.
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

	request := api.FileMoveCopy{
		DriveID:        f.driveID,
		FileID:         srcObj.id,
		ToParentFileID: directoryID,
	}

	opts := rest.Opts{
		Method: "POST",
		Path:   "/adrive/v1.0/openFile/copy",
	}

	var resp *http.Response
	var copyResp api.FileCopyResp
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &request, &copyResp)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	if err := dstObj.readMetaData(ctx); err != nil {
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
	return hash.Set(hash.None)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// CreateDir implements dircache.DirCacher.
func (f *Fs) CreateDir(ctx context.Context, pathID string, leaf string) (newID string, err error) {
	var resp *http.Response
	var info *api.Item

	opts := rest.Opts{
		Method: "POST",
		Path:   "/adrive/v1.0/openFile/create",
	}

	req := api.CreateFolder{
		DriveID:       f.driveID,
		ParentFileID:  pathID,
		Name:          leaf,
		Type:          ItemTypeFolder,
		CheckNameMode: "refuse",
	}
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &req, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return "", err
	}
	return info.FileID, nil
}

// FindLeaf finds a directory of name leaf in the folder with ID pathID
func (f *Fs) FindLeaf(ctx context.Context, pathID, leaf string) (pathIDOut string, found bool, err error) {
	// Find the leaf in pathID
	found, err = f.listAll(ctx, pathID, func(item *api.Item) bool {
		if strings.EqualFold(item.Name, leaf) {
			pathIDOut = item.FileID
			return true
		}
		return false
	})
	return pathIDOut, found, err
}

// List implements fs.Fs.
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return nil, err
	}
	var iErr error

	_, err = f.listAll(ctx, directoryID, func(info *api.Item) bool {
		remote := path.Join(dir, info.Name)
		switch info.Type {
		case ItemTypeFolder:
			// cache the directory ID for later lookups
			f.dirCache.Put(remote, info.FileID)
			d := fs.NewDir(remote, info.UpdatedAt).SetID(info.FileID).SetParentID(dir)
			// FIXME more info from dir?
			entries = append(entries, d)
		case ItemTypeFile:
			var o fs.Object
			o, err = f.newObjectWithInfo(ctx, remote, info)
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

// NewObject implements fs.Fs.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	return f.newObjectWithInfo(ctx, remote, nil)
}

// Put implements fs.Fs.
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	// TODO
	panic("unimplemented")
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.Put(ctx, in, src, options...)
}

// Mkdir creates the container if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	_, err := f.dirCache.FindDir(ctx, dir, true)
	return err
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
	user, err := f.getUserInfo(ctx)
	if err != nil {
		return nil, err
	}

	userInfo = map[string]string{
		"Name":   user.Name,
		"Avatar": user.Avatar,
		"Phone":  *user.Phone,
	}

	if vip, err := f.getVipInfo(ctx); err == nil {
		userInfo["Identity"] = vip.Identity
		userInfo["Level"] = *vip.Level
		userInfo["Expire"] = time.Time(vip.Expire).String()
		userInfo["ThirdPartyVip"] = strconv.FormatBool(vip.ThirdPartyVip)
		userInfo["ThirdPartyVipExpire"] = *vip.ThirdPartyVipExpire
	}

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
	return "", nil
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
	// TODO
	panic("unimplemented")
}

// Update implements fs.Object.
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	return o.upload(ctx, in, src, options...)
}

// Remove implements fs.Object.
func (o *Object) Remove(ctx context.Context) error {
	return o.fs.deleteObject(ctx, o.id)
}

// ParentID implements fs.ParentIDer.
func (o *Object) ParentID() string {
	return o.parent
}

func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	root = parsePath(root)

	client := fshttp.NewClient(ctx)
	var ts *oauthutil.TokenSource

	if opt.AccessToken == "" {
		client, ts, err = oauthutil.NewClient(ctx, name, m, oauthConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to configure Box: %w", err)
		}
	}

	f := &Fs{
		name: name,
		root: root,
		opt:  *opt,
		srv:  rest.NewClient(client).SetRoot(rootURL),
		m:    m,
		pacer: fs.NewPacer(ctx, pacer.NewDefault(
			pacer.MinSleep(minSleep),
			pacer.MaxSleep(maxSleep),
			pacer.DecayConstant(decayConstant),
		)),
	}

	f.features = (&fs.Features{
		CaseInsensitive:         true,
		CanHaveEmptyDirectories: true,
	}).Fill(ctx, f)
	f.srv.SetErrorHandler(errorHandler)

	if f.opt.AccessToken != "" {
		f.srv.SetHeader("Authorization", "Bearer "+f.opt.AccessToken)
	}

	if ts != nil {
		f.tokenRenewer = oauthutil.NewRenew(f.String(), ts, func() error {
			_, err = f.readMetaDataForPath(ctx, "")
			return err
		})
	}

	rootID := f.opt.RootFolderID
	f.dirCache = dircache.New(root, rootID, f)

	err = f.dirCache.FindRoot(ctx, false)
	if err != nil {
		newRoot, remote := dircache.SplitPath(root)
		tempF := *f
		tempF.dirCache = dircache.New(newRoot, rootID, &tempF)
		tempF.root = newRoot
		err = tempF.dirCache.FindRoot(ctx, false)
		if err != nil {
			return f, nil
		}
		_, err = tempF.newObjectWithInfo(ctx, remote, nil)
		if err != nil {
			if err == fs.ErrorObjectNotFound {
				return f, nil
			}
			return nil, err
		}
		f.features.Fill(ctx, &tempF)
		// XXX: update the old f here instead of returning tempF, since
		// `features` were already filled with functions having *f as a receiver.
		// See https://github.com/rclone/rclone/issues/2182
		f.dirCache = tempF.dirCache
		f.root = tempF.root
		// return an error with an fs which points to the parent
		return f, fs.ErrorIsFile
	}

	f.getDriveInfo(ctx)

	return f, nil
}

// Check the interfaces are satisfied
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
