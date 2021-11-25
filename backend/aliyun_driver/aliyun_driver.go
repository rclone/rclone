package aliyun_driver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"time"

	"github.com/rclone/rclone/backend/aliyun_driver/entity"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/rest"
)

//bf7744a65ef0451886e78fbda6b94f96

const (
	ItemTypeFolder = "folder"
	ItemTypeFile   = "file"
	rootId         = "root"

	url = "https://api.aliyundrive.com/v2"
	ua  = "Mozilla/5.0 (Macintosh; Intel Mac OS X 11_0_0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/86.0.4240.198 Safari/537.36"
)

type Fs struct {
	name        string
	ci          *fs.ConfigInfo
	srv         *rest.Client
	features    *fs.Features
	opt         Options
	root        string
	accessToken string
	ctx         context.Context
	driveId     string
	dirCache    *dircache.DirCache // Map of directory path to directory id
}

// Name 返回名称
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return f.root
}

// String converts this Fs to a string
func (f *Fs) String() string {
	return "Aliyun Driver"
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

func (f *Fs) Precision() time.Duration {

	return time.Second
}

func (f *Fs) Hashes() hash.Set {

	return 0
}

// List
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return nil, err
	}

	list, err := f.listAll(ctx, directoryID)
	for _, info := range list {
		remote := path.Join(dir, info.Name)
		if info.Type == ItemTypeFolder {
			f.dirCache.Put(remote, info.FileId)
			d := fs.NewDir(remote, info.UpdatedAt).SetID(info.FileId).SetParentID(dir)
			entries = append(entries, d)
		} else {
			o := f.newObjectWithInfo(ctx, info.Name, &info)
			entries = append(entries, o)
		}
	}
	return
}

// listAll 获取目录下全部文件
func (f *Fs) listAll(ctx context.Context, parentFileId string) ([]entity.ItemsOut, error) {
	request := entity.ListIn{
		Limit:        200,
		DriveId:      f.driveId,
		ParentFileId: parentFileId,
	}

	opts := rest.Opts{
		Method:  "POST",
		Path:    "/file/list",
		RootURL: url,
	}

	var out []entity.ItemsOut
	for {
		resp := entity.ListOut{}
		err := f.callJSON(ctx, &opts, request, &resp)
		if err != nil {
			return out, errors.New(resp.Code)
		}
		out = append(out, resp.Items...)
		if resp.NextMarker == "" {
			break
		} else {
			request.Marker = resp.NextMarker
		}
	}
	return out, nil
}

// isDirEmpty 判断目录是否为空
func (f *Fs) isDirEmpty(ctx context.Context, parentFileId string) bool {
	request := entity.ListIn{
		Limit:        1,
		DriveId:      f.driveId,
		ParentFileId: parentFileId,
	}
	opts := rest.Opts{
		Method:  "POST",
		Path:    "/file/list",
		RootURL: url,
	}
	resp := entity.ListOut{}
	err := f.callJSON(ctx, &opts, request, &resp)
	if err != nil {
		return false
	}
	return len(resp.Items) == 0
}

func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {

	return nil, nil
}

func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {

	return nil, nil
}

func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	_, err := f.dirCache.FindDir(ctx, dir, true)
	return err
}

// Rmdir 删除空目录
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return err
	}
	if directoryID == rootId {
		return errors.New("the root directory cannot be deleted")
	}

	if !f.isDirEmpty(ctx, directoryID) {
		return errors.New("directory is not be empty")
	}

	request := entity.RmDirIn{
		DriveId: f.driveId,
		FileId:  directoryID,
	}

	opts := rest.Opts{
		Method:  "POST",
		Path:    "/recyclebin/trash",
		RootURL: url,
	}
	var response entity.RmdirOut
	err = f.callJSON(ctx, &opts, request, &response)
	if err != nil {
		f.dirCache.FlushDir(dir)
	}
	return err
}

// Purge deletes all the files and the container
//
// Optional interface: Only implement this if you have a way of
// deleting all the files quicker than just running Remove() on the
// result of List()
func (f *Fs) Purge(ctx context.Context, dir string) error {
	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return err
	}
	request := entity.RmDirIn{
		DriveId: f.driveId,
		FileId:  directoryID,
	}

	opts := rest.Opts{
		Method:  "POST",
		Path:    "/recyclebin/trash",
		RootURL: url,
	}
	var response entity.RmdirOut
	err = f.callJSON(ctx, &opts, request, &response)
	if err != nil {
		f.dirCache.FlushDir(dir)
	}
	return err
}

// FindLeaf finds a directory of name leaf in the folder with ID pathID
func (f *Fs) FindLeaf(ctx context.Context, pathID, leaf string) (pathIDOut string, found bool, err error) {
	// Find the leaf in pathID
	items, err := f.listAll(f.ctx, pathID)
	if err != nil {
		return
	}
	for _, item := range items {
		if item.Name == leaf {
			pathIDOut = item.FileId
			found = true
		}
	}
	return
}

// CreateDir makes a directory with pathID as parent and name leaf
func (f *Fs) CreateDir(ctx context.Context, pathID, leaf string) (newID string, err error) {
	request := entity.MakeDirIn{
		CheckNameMode: "refuse",
		DriveId:       f.driveId,
		Type:          ItemTypeFolder,
		Name:          leaf,
		ParentFileId:  pathID, //
	}
	opts := rest.Opts{
		Method:  "POST",
		Path:    "/file/create_with_proof",
		RootURL: url,
	}
	var response entity.MkdirOut
	err = f.callJSON(ctx, &opts, request, &response)
	return response.FileId, err
}

// About gets quota information
func (f *Fs) About(ctx context.Context) (usage *fs.Usage, err error) {
	opts := rest.Opts{
		Method:  "POST",
		Path:    "/databox/get_personal_info",
		RootURL: url,
	}
	var resp entity.PersonalInfoOut
	err = f.callJSON(ctx, &opts, nil, &resp)
	if err != nil {
		return nil, fmt.Errorf("failed to read user info: %w", err)
	}
	usage = &fs.Usage{
		Used:  fs.NewUsageValue(resp.PersonalSpaceInfo.UsedSize),                                    // bytes in use
		Total: fs.NewUsageValue(resp.PersonalSpaceInfo.TotalSize),                                   // bytes total
		Free:  fs.NewUsageValue(resp.PersonalSpaceInfo.TotalSize - resp.PersonalSpaceInfo.UsedSize), // bytes free
	}
	return usage, nil
}

// DirCacheFlush resets the directory cache - used in testing as an
// optional interface
func (f *Fs) DirCacheFlush() {
	f.dirCache.ResetRoot()
}

// NewFs constructs an Fs from the path, bucket:path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	ci := fs.GetConfig(ctx)

	opt := new(Options)
	err := configstruct.Set(m, opt)

	if err != nil {
		return nil, err
	}

	cli := rest.NewClient(fshttp.NewClient(ctx))
	cli.SetHeader("User-Agent", ua)
	f := &Fs{
		name: name,
		ci:   ci,
		srv:  cli,
		opt:  *opt,
		root: root,
		ctx:  ctx,
	}

	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
	}).Fill(ctx, f)
	err = f.getAccessToken()
	f.dirCache = dircache.New(root, rootId, f)
	return f, err
}

// getAccessToken 获取getAccessToken
func (f *Fs) getAccessToken() error {
	opts := rest.Opts{
		Method:  "POST",
		Path:    "/token/refresh",
		RootURL: "https://websv.aliyundrive.com",
	}
	request := entity.AccessTokenIn{RefreshToken: f.opt.RefreshToken}
	response := entity.AccessTokenOut{}

	err := f.callJSON(f.ctx, &opts, request, &response)
	if err != nil {
		return err
	}

	//未获取到正常的值返回异常
	if response.AccessToken == "" || response.RefreshToken == "" {
		return errors.New("get accessToken or refreshToken error")
	}
	fmt.Println("-------------------- accessToken end --------------------")
	fmt.Println(response.AccessToken)
	fmt.Println("-------------------- accessToken begin --------------------")

	fmt.Println("-------------------- refreshToken end --------------------")
	fmt.Println(response.RefreshToken)
	fmt.Println("-------------------- refreshToken begin --------------------")

	f.srv.SetHeader("authorization", response.AccessToken)
	f.driveId = response.DefaultDriveId
	f.accessToken = response.AccessToken
	return nil
}

func (f *Fs) callJSON(ctx context.Context, opts *rest.Opts, request interface{}, response interface{}) error {
	resp, err := f.srv.CallJSON(ctx, opts, request, response)
	if err != nil {
		errResponse := entity.ErrorResponse{}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		err = json.Unmarshal(body, &errResponse)
		if err != nil {
			return err
		}
		if errResponse.Code != "" {
			if errResponse.Code == "AccessTokenInvalid" {
				f.getAccessToken()
				return f.callJSON(ctx, opts, request, response)
			}
			return errors.New(errResponse.Code)
		}
		return err
	}
	return nil
}

// Options defines the configuration for this backend
type Options struct {
	RefreshToken string `config:"refresh_token"`
}

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "aliyun-driver",
		Description: "Aliyun Driver",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:     "refresh-token",
			Help:     "refresh-token",
			Required: true,
		}},
	})
}

type Object struct {
	fs      *Fs       // what this object is part of
	remote  string    // The remote path
	size    int64     // size of the object
	modTime time.Time // modification time of the object
	id      string    // ID of the object
	sha1    string    // SHA-1 of the object content
}

// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(ctx context.Context, remote string, info *entity.ItemsOut) fs.Object {
	o := &Object{
		fs:      f,
		remote:  remote,
		size:    int64(info.Size),
		sha1:    info.ContentHash,
		modTime: info.UpdatedAt,
		id:      info.FileId,
	}
	return o
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
	return o.size
}

// ModTime returns the modification time of the object
//
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.modTime
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	return nil
}

// Storable returns a boolean showing whether this object storable
func (o *Object) Storable() bool {
	return true
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {

	return nil, err
}

// Update the object with the contents of the io.Reader, modTime and size
//
// If existing is set then it updates the object rather than creating a new one
//
// The new object may have been created if an error is returned
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	return err
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	return nil
}

// ID returns the ID of the Object if known, or "" if not
func (o *Object) ID() string {
	return o.id
}
