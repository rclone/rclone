package aliyun_driver

import (
	"context"
	"fmt"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/rest"
	"io"
	"time"
)

//bf7744a65ef0451886e78fbda6b94f96

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

func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {

	return
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

const url = "https://api.aliyundrive.com/v2"
const ua = "Mozilla/5.0 (Macintosh; Intel Mac OS X 11_0_0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/86.0.4240.198 Safari/537.36"

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

func (f *Fs) getAccessToken() {
	opts := rest.Opts{
		Method:  "POST",
		Path:    "/token/refresh",
		RootURL: "https://websv.aliyundrive.com",
	}
	request := struct {
		RefreshToken string `json:"refresh_token"`
	}{RefreshToken: f.opt.RefreshToken}

	response := struct {
		RefreshToken   string `json:"refresh_token"`
		AccessToken    string `json:"access_token"`
		DefaultDriveId string `json:"default_drive_id"`
	}{}

	_, err := f.srv.CallJSON(f.ctx, &opts, request, &response)

	if err != nil {

	}
	f.srv.SetHeader("authorization", response.AccessToken)
	f.driveId = response.DefaultDriveId

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
