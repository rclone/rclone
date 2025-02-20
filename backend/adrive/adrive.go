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
	client       *Client            // Aliyun Drive client
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

// Lists the directory required calling the user function on each item found
//
// If the user fn ever returns true then it early exits with found = true
func (f *Fs) listAll(ctx context.Context, directoryID string) (items []api.Item, err error) {
	opts := rest.Opts{
		Method: "POST",
		Path:   "/adrive/v1.0/openFile/list",
	}

	request := api.FileListReq{
		DriveID:      f.driveID,
		ParentFileID: directoryID,
	}

	var result api.FileListResp
	_, err = f.client.CallJSON(ctx, &opts, &request, &result)
	if err != nil {
		return nil, err
	}

	return result.Items, nil
}

// deleteObject removes an object by ID
func (f *Fs) deleteObject(ctx context.Context, id string) error {
	path := "/adrive/v1.0/openFile"
	if f.opt.RemoveWay == RemoveWayTrash {
		path += "/recyclebin/trash"
	} else if f.opt.RemoveWay == RemoveWayDelete {
		path += "/delete"
	}

	opts := rest.Opts{
		Method:     "DELETE",
		Path:       path,
		NoResponse: true,
	}

	req := api.DeleteFile{
		DriveID: f.driveID,
		FileID:  id,
	}

	_, err := f.client.CallJSON(ctx, &opts, &req, nil)
	return err
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
		fs:       f,
		remote:   remote,
		parentID: dirID,
		size:     size,
		modTime:  modTime,
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

// getUserInfo gets UserInfo from API
func (f *Fs) getUserInfo(ctx context.Context) (info *api.User, err error) {
	opts := rest.Opts{
		Method: "GET",
		Path:   "/oauth/users/info",
	}
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.client.CallJSON(ctx, &opts, nil, &info)
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
		resp, err = f.client.CallJSON(ctx, &opts, nil, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("failed to get driveinfo: %w", err)
	}

	f.client.driveID = info.DefaultDriveID

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
		resp, err = f.client.CallJSON(ctx, &opts, nil, &info)
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
		resp, err = f.client.CallJSON(ctx, &opts, nil, &info)
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
		resp, err = f.client.CallJSON(ctx, &opts, &move, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}

	return info, nil
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
	o.parentID = info.ParentFileID
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
	var info api.Item
	for _, item := range list {
		if item.Type == ItemTypeFile && strings.EqualFold(item.Name, leaf) {
			info = item
			break
		}
	}

	return o.setMetaData(&info)
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
		resp, err = f.client.CallJSON(ctx, &opts, &request, &copyResp)
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
	return hash.Set(hash.SHA1)
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
		resp, err = f.client.CallJSON(ctx, &opts, &req, &info)
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
	items, err := f.listAll(ctx, pathID)
	if err != nil {
		return "", false, err
	}
	for _, item := range items {
		if strings.EqualFold(item.Name, leaf) {
			pathIDOut = item.FileID
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
	var iErr error

	list, err := f.listAll(ctx, directoryID)
	for _, info := range list {
		remote := path.Join(dir, info.Name)
		if info.Type == ItemTypeFolder {
			f.dirCache.Put(remote, info.FileID)
			d := fs.NewDir(remote, info.UpdatedAt).SetID(info.FileID).SetParentID(dir)
			entries = append(entries, d)
		} else {
			o, err := f.newObjectWithInfo(ctx, remote, &info)
			if err == nil {
				entries = append(entries, o)
			}
		}
	}

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
	o := &Object{}
	return o, o.Update(ctx, in, src, options...)
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
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	opts := rest.Opts{
		Method: "POST",
		Path:   "/adrive/v1.0/openFile/getDownloadUrl",
	}

	req := api.DownloadReq{
		DriveID: o.fs.driveID,
		FileID:  o.id,
	}

	var resp *http.Response
	var download api.DownloadResp
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.client.CallJSON(ctx, &opts, &req, &download)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}

	fs.FixRangeOption(options, o.size)

	opts = rest.Opts{
		Method:  download.Method,
		RootURL: download.Url,
		Options: options,
	}
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.client.Call(ctx, &opts)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	return resp.Body, err
}

// Update implements fs.Object.
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	// return o.upload(ctx, in, src, options...)
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

	if opt.AccessToken == "" {
		client, ts, err = oauthutil.NewClient(ctx, name, m, oauthConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to configure Aliyun Drive: %w", err)
		}
	}

	// Create filesystem
	f := &Fs{
		name:   name,
		root:   root,
		opt:    *opt,
		client: NewClient(client, rootURL),
		m:      m,
		pacer:  fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
	}

	f.features = (&fs.Features{
		CaseInsensitive:         true,
		CanHaveEmptyDirectories: true,
	}).Fill(ctx, f)

	// Set up authentication
	if f.opt.AccessToken != "" {
		f.client.c.SetHeader("Authorization", "Bearer "+f.opt.AccessToken)
	}

	if ts != nil {
		f.tokenRenewer = oauthutil.NewRenew(f.String(), ts, func() error {
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
		// XXX: update the old f here instead of returning tempF, since
		// `features` were already filled with functions having *f as a receiver.
		f.dirCache = tempF.dirCache
		f.root = tempF.root
		// return an error with an fs which points to the parent
		return f, fs.ErrorIsFile
	}

	// Get drive info
	err = f.getDriveInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get drive info: %w", err)
	}

	return f, nil
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
