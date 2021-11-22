package aliyun_driver

import (
	"context"
	"fmt"
	"io"
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
	for _, info := range f.listAll(ctx, dir) {
		if info.Type == ItemTypeFolder {
			d := fs.NewDir(info.Name, info.UpdatedAt).SetID(info.FileId).SetParentID(dir)
			entries = append(entries, d)
		} else {
			//
			o := f.newObjectWithInfo(ctx, info.Name, &info)
			entries = append(entries, o)
		}
	}
	return
}

// listAll 获取目录下全部文件
func (f *Fs) listAll(ctx context.Context, dir string) []entity.ItemsOut {
	request := entity.ListIn{
		Limit:        200,
		DriveId:      f.driveId,
		ParentFileId: dir,
	}

	opts := rest.Opts{
		Method:  "POST",
		Path:    "/file/list",
		RootURL: url,
	}

	var out []entity.ItemsOut
	for {
		resp := entity.ListOut{}
		_, err := f.srv.CallJSON(f.ctx, &opts, request, &resp)
		if err == nil {
			out = append(out, resp.Items...)
		}
		if resp.NextMarker == "" {
			break
		} else {
			request.Marker = resp.NextMarker
		}
	}
	return out
}

func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {

	return nil, nil
}

func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {

	return nil, nil
}

func (f *Fs) Mkdir(ctx context.Context, dir string) error {

	return nil
}

func (f *Fs) Rmdir(ctx context.Context, dir string) error {

	return nil
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
	f.features = (&fs.Features{}).Fill(ctx, f)
	f.getAccessToken()
	f.getDriveId()
	return f, nil
}

func (f *Fs) getDriveId() {
	opts := rest.Opts{
		Method:  "POST",
		Path:    "/user/get",
		RootURL: url,
	}

	resp := struct {
		DefaultDriveId string `json:"default_drive_id"`
	}{}

	r, e := f.srv.CallJSON(f.ctx, &opts, struct{}{}, &resp)
	fmt.Println(r, e, resp)
}

// getAccessToken 获取getAccessToken
func (f *Fs) getAccessToken() {
	opts := rest.Opts{
		Method:  "POST",
		Path:    "/token/refresh",
		RootURL: "https://websv.aliyundrive.com",
	}
	request := entity.AccessTokenIn{RefreshToken: f.opt.RefreshToken}
	response := entity.AccessTokenOut{}

	_, err := f.srv.CallJSON(f.ctx, &opts, request, &response)
	if err != nil {
		//TODO
		fmt.Println("")
	}
	fmt.Println("-------------------- accessToken end --------------------")
	fmt.Println(response.AccessToken)
	fmt.Println("-------------------- accessToken begin --------------------")
	f.srv.SetHeader("authorization", response.AccessToken)
	f.driveId = response.DefaultDriveId
	f.accessToken = response.AccessToken
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
