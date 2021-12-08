package aliyun_driver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/backend/aliyun_driver/entity"
	"github.com/rclone/rclone/backend/box/api"
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
	itemTypeFolder = "folder"
	itemTypeFile   = "file"
	rootId         = "root"

	rootUrl = "https://api.aliyundrive.com/v2"
	uA      = "Mozilla/5.0 (Macintosh; Intel Mac OS X 11_0_0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/86.0.4240.198 Safari/537.36"

	chunkSize = 10485760
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
		if info.Type == itemTypeFolder {
			f.dirCache.Put(remote, info.FileId)
			d := fs.NewDir(remote, info.UpdatedAt).SetID(info.FileId).SetParentID(dir)
			entries = append(entries, d)
		} else {
			o, err := f.newObjectWithInfo(ctx, info.Name, &info)
			if err == nil {
				entries = append(entries, o)
			}
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
		RootURL: rootUrl,
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
		RootURL: rootUrl,
	}
	resp := entity.ListOut{}
	err := f.callJSON(ctx, &opts, request, &resp)
	if err != nil {
		return false
	}
	return len(resp.Items) == 0
}

func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	return f.newObjectWithInfo(ctx, remote, nil)
}

func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	remote := src.Remote()
	leaf, directoryID, err := f.dirCache.FindPath(ctx, remote, false)
	if err != nil {
		if err == fs.ErrorDirNotFound {
			return f.PutUnchecked(ctx, in, src, options...)
		}
		return nil, err
	}

	fmt.Println(leaf, directoryID)

	return nil, nil
}

// PutUnchecked the object into the container
//
// This will produce an error if the object already exists
//
// Copy the reader in to the new object which is returned
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

// setMetaData sets the metadata from info
func (o *Object) setMetaData(info *entity.ItemsOut) (err error) {
	if info.Type == itemTypeFolder {
		return fs.ErrorIsDir
	}
	if info.Type != api.ItemTypeFile {
		return fmt.Errorf("%q is %q: %w", o.remote, info.Type, fs.ErrorNotAFile)
	}
	o.hasMetaData = true
	o.size = int64(info.Size)
	o.sha1 = info.ContentHash
	o.modTime = info.CreatedAt
	o.id = info.FileId
	o.downloadUrl = info.DownloadURL
	return nil
}

// Creates from the parameters passed in a half finished Object which
// must have setMetaData called on it
//
// Returns the object, leaf, directoryID and error
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
	err = f.deleteObject(ctx, directoryID)
	if err == nil {
		f.dirCache.FlushDir(dir)
	}
	return err
}

// 删除操作
func (f *Fs) deleteObject(ctx context.Context, fileId string) error {
	request := entity.DeleteIn{
		DriveId: f.driveId,
		FileId:  fileId,
	}

	opts := rest.Opts{
		Method:  "POST",
		Path:    "/recyclebin/trash",
		RootURL: rootUrl,
	}
	var response entity.DeleteOut
	err := f.callJSON(ctx, &opts, request, &response)
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
	err = f.deleteObject(ctx, directoryID)
	if err == nil {
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
		Type:          itemTypeFolder,
		Name:          leaf,
		ParentFileId:  pathID, //
	}
	opts := rest.Opts{
		Method:  "POST",
		Path:    "/file/create_with_proof",
		RootURL: rootUrl,
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
		RootURL: rootUrl,
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
	cli.SetHeader("User-Agent", uA)
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
	fmt.Println("-------------------- accessToken begin --------------------")
	fmt.Println(response.AccessToken)
	fmt.Println("-------------------- accessToken end --------------------")

	fmt.Println("-------------------- refreshToken begin --------------------")
	fmt.Println(response.RefreshToken)
	fmt.Println("-------------------- refreshToken end --------------------")

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
	fs          *Fs       // what this object is part of
	remote      string    // The remote path
	size        int64     // size of the object
	modTime     time.Time // modification time of the object
	id          string    // ID of the object
	sha1        string    // SHA-1 of the object content
	hasMetaData bool      //
	downloadUrl string    //
}

// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(ctx context.Context, remote string, info *entity.ItemsOut) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	var err error
	if info != nil {
		err = o.setMetaData(info)
	} else {
		err = o.readMetaData(ctx)
	}
	return o, err
}

func (o *Object) readMetaData(ctx context.Context) (err error) {
	if o.hasMetaData {
		return nil
	}
	leaf, directoryID, err := o.fs.dirCache.FindPath(ctx, o.remote, false)
	if err != nil {
		if err == fs.ErrorDirNotFound {
			return fs.ErrorObjectNotFound
		}
		return err
	}

	list, err := o.fs.listAll(ctx, directoryID)
	if err != nil {
		return err
	}
	var info entity.ItemsOut
	for _, v := range list {
		if v.Type == itemTypeFile && strings.EqualFold(v.Name, leaf) {
			info = v
			break
		}
	}
	return o.setMetaData(&info)
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
	size := src.Size()
	modTime := src.ModTime(ctx)
	remote := o.Remote()

	leaf, directoryID, err := o.fs.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return err
	}
	return o.upload(ctx, in, leaf, directoryID, modTime, size)
}

//上传
func (o *Object) upload(ctx context.Context, in io.Reader, leaf, directoryID string, modTime time.Time, size int64) (err error) {
	chunkNum := int(math.Ceil(float64(size) / chunkSize))
	resp, err := o.preUplaod(ctx, leaf, directoryID, modTime, size, chunkNum)
	if err != nil {
		return err
	}
	if len(resp.PartInfoList) != chunkNum {
		return errors.New("预上传数量和分片数不一致")
	}
	//设置file_id
	o.id = resp.FileId
	//分片上传
	err = o.sliceUpload(ctx, resp.PartInfoList, in, size, int64(chunkNum))
	if err != nil {
		return err
	}
	//上传完成
	return o.complete(ctx, resp.FileId, resp.UploadId)
}

//预上传
func (o *Object) preUplaod(ctx context.Context, leaf, directoryID string, modTime time.Time, size int64, chunkNum int) (entity.PreUploadOut, error) {
	req := entity.PreUploadIn{
		DriveId:         o.fs.driveId,
		Name:            leaf,
		ParentFileId:    directoryID,
		Size:            size,
		CheckNameMode:   "refuse",
		ContentHashName: "none",
		ProofVersion:    "v1",
		Type:            "file",
		PartInfoList:    make([]entity.PartInfo, 0),
	}
	for i := 0; i < chunkNum; i++ {
		req.PartInfoList = append(req.PartInfoList, entity.PartInfo{PartNumber: i + 1})
	}
	opts := rest.Opts{
		Method:  "POST",
		Path:    "/file/create_with_proof",
		RootURL: rootUrl,
	}
	resp := entity.PreUploadOut{}
	err := o.fs.callJSON(ctx, &opts, req, &resp)
	return resp, err
}

// 分片上传
func (o *Object) sliceUpload(ctx context.Context, parts []entity.PartInfo, in io.Reader, size int64, chukNum int64) (err error) {
	// 开启多个go上传
	var wg sync.WaitGroup
	for k, p := range parts {
		newChunkSize := int64(chunkSize)
		if k == int(chukNum-1) {
			newChunkSize = size - chunkSize*int64(chukNum-1)
		}
		buf := make([]byte, newChunkSize)
		io.ReadFull(in, buf)
		u, _ := url.Parse(p.UploadUrl)
		wg.Add(1)
		go func(err *error) {
			defer wg.Done()
			opts := rest.Opts{
				Method:     "PUT",
				RootURL:    u.Host,
				Path:       u.Path,
				Parameters: u.Query(),
				Body:       bytes.NewReader(buf),
			}
			e := o.fs.callJSON(ctx, &opts, nil, nil)
			// 不考虑竞争情况
			if e != nil && err == nil {
				err = &e
			}
		}(&err)
	}
	wg.Wait()
	return err
}

// 完成上传
func (o *Object) complete(ctx context.Context, fileId, uploadId string) error {
	rep := entity.CompleteUploadIn{
		DriveId:  o.fs.driveId,
		FileId:   fileId,
		UploadId: uploadId,
	}
	opts := rest.Opts{
		Method:  "POST",
		Path:    "/file/complete",
		RootURL: rootUrl,
	}
	return o.fs.callJSON(ctx, &opts, rep, nil)
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	return o.fs.deleteObject(ctx, o.id)
}

// ID returns the ID of the Object if known, or "" if not
func (o *Object) ID() string {
	return o.id
}
