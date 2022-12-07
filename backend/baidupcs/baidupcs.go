// Author: Hao Zhang <hzhangxyz@outlook.com>
// Based on the box.go from rclone and baidupcs.go from cnly

package baidupcs

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"github.com/rclone/rclone/backend/baidupcs/openapi"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/hash"
	"io"
	"strconv"
	"strings"
	"time"
)

const (
	// These two key is applied from baidu
	AppKey    = "ZNpx3U78CKa79G5kwZ1a3vIwWYGRIVGG"
	SecretKey = "983RYscSgykeWVkfSap39KjNuh04zFYt" // TODO it may be dangerous to put it here
)

type Options struct {
	RefreshTime  string `config:"refresh_time"`
	RefreshToken string `config:"refresh_token"`
	AccessToken  string `config:"access_token"`
}
type Fs struct {
	name     string       // name of this remote
	root     string       // the path we are working on
	opt      *Options     // parsed options
	features *fs.Features // optional features
}
type Object struct {
	fs       *Fs
	path     string
	filename string
	size     int64
	mtime    time.Time
	ctime    time.Time
	isdir    bool
	md5      string
}

func getSrv() *openapi.APIClient {
	return openapi.NewAPIClient(openapi.NewConfiguration())
}

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "baidupcs",
		Description: "Baidu PCS (aka Baidu Yun, Baidu Pan, Baidu NetDisk)",
		NewFs: func(ctx context.Context, name string, root string, m configmap.Mapper) (fs.Fs, error) {
			fmt.Printf("baidupcs newfs called with root=%s\n", root)
			opt := new(Options)
			err := configstruct.Set(m, opt)
			if err != nil {
				return nil, err
			}

			currentTime := time.Now().Unix()
			intRefreshTime, _ := strconv.ParseInt(opt.RefreshTime, 10, 64)
			if currentTime > intRefreshTime {
				// need to refresh
				resp, _, err := getSrv().AuthApi.OauthTokenRefreshToken(ctx).RefreshToken(opt.RefreshToken).ClientId(AppKey).ClientSecret(SecretKey).Execute()
				if err != nil {
					return nil, err
				}
				m.Set("refresh_time", strconv.FormatInt(currentTime+int64(*resp.ExpiresIn), 10))
				m.Set("refresh_token", *resp.RefreshToken)
				m.Set("access_token", *resp.AccessToken)

				err = configstruct.Set(m, opt)
				if err != nil {
					return nil, err
				}
			}

			root = strings.Trim(root, "/")

			f := &Fs{
				name: name,
				root: root,
				opt:  opt,
			}
			f.features = (&fs.Features{
				// TODO need to check the whole list of possible features
				CaseInsensitive: true,
			}).Fill(ctx, f)

			// TODO if root is not a folder, should return fs.ErrorIsFile

			return f, nil
		},
		Config: func(ctx context.Context, name string, m configmap.Mapper, configIn fs.ConfigIn) (*fs.ConfigOut, error) {
			fmt.Printf("baidupcs config called\n")
			opt := new(Options)
			err := configstruct.Set(m, opt)
			if err != nil {
				return nil, err
			}

			currentTime := time.Now().Unix()
			if opt.AccessToken == "" {
				// need to get token
				// device code mode: https://pan.baidu.com/union/doc/fl1x114ti
				resp, _, err := getSrv().AuthApi.OauthTokenDeviceCode(ctx).ClientId(AppKey).Scope("basic,netdisk").Execute()
				if err != nil {
					return nil, err
				}
				code := resp.DeviceCode
				fmt.Printf("Please open the following URL %s with user code %s\n", *resp.VerificationUrl, *resp.UserCode)
				for {
					// polling
					time.Sleep(10 * time.Second) // baidu said the interval should be larger than 5s, ok, let do it every 10s
					resp, _, err := getSrv().AuthApi.OauthTokenDeviceToken(ctx).Code(*code).ClientId(AppKey).ClientSecret(SecretKey).Execute()
					if err == nil {
						m.Set("refresh_time", strconv.FormatInt(currentTime+int64(*resp.ExpiresIn), 10))
						m.Set("refresh_token", *resp.RefreshToken)
						m.Set("access_token", *resp.AccessToken)
						return nil, nil
					}
				}
			}
			return nil, nil
		},
		Options: []fs.Option{
			{Name: "refresh_time", Default: "", Advanced: true},
			{Name: "refresh_token", Default: "", Advanced: true},
			{Name: "access_token", Default: "", Advanced: true},
		},
	})
}

// TODO implements, including basic requirement and feature requirement

// Fs implement interface Fs(Info), Abouter
// Fs impl Info
func (f *Fs) Name() string {
	return f.name
}
func (f *Fs) Root() string {
	return f.root
}
func (f *Fs) String() string {
	return fmt.Sprintf("baidupcs root '%s'", f.root)
}
func (f *Fs) Precision() time.Duration {
	return time.Second
}
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.MD5)
}
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Fs impl Fs
func (f *Fs) List(ctx context.Context, dir string) (fs.DirEntries, error) {
	fmt.Printf("baidupcs list called with dir=%s\n", dir)
	dir = fmt.Sprintf("/%s%s", f.root, dir)
	resp, _, err := getSrv().FileinfoApi.Xpanfilelist(ctx).AccessToken(f.opt.AccessToken).Folder("0").Web("0").Start("0").Limit(1000).Dir(dir).Order("name").Desc(0).Execute()
	if err != nil {
		return nil, err
	}
	respByte := []byte(resp)
	respResult := map[string]interface{}{}
	// TODO this part of code is ugly
	err = json.Unmarshal(respByte, &respResult)
	if err != nil {
		return nil, err
	}
	list := respResult["list"].([]interface{})
	entries := fs.DirEntries{}
	for _, item := range list {
		refinedItem := item.(map[string]interface{})
		var md5 string
		if refinedItem["md5"] != nil {
			md5 = refinedItem["md5"].(string)
		} else {
			md5 = ""
		}
		o := Object{
			fs:       f,
			path:     refinedItem["path"].(string),
			filename: refinedItem["server_filename"].(string),
			size:     int64(refinedItem["size"].(float64)),
			mtime:    time.Unix(int64(refinedItem["server_mtime"].(float64)), 0),
			ctime:    time.Unix(int64(refinedItem["server_ctime"].(float64)), 0),
			isdir:    int64(refinedItem["isdir"].(float64)) != 0,
			md5:      md5,
		}
		entries = append(entries, &o)
	}
	return entries, nil
}
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	// TODO
	return nil, errors.New("not implement")
}
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	// TODO
	return nil, errors.New("not implement")
}
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	// TODO.
	_, _, err := getSrv().FileuploadApi.Xpanfilecreate(ctx).AccessToken(f.opt.AccessToken).Path(dir).Isdir(1).Rtype(0).Execute()
	return err
}
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	// TODO does it support folder?, does it raise error if not empty?
	fileList := fmt.Sprintf("[\"%s\"]", dir)
	_, err := getSrv().FilemanagerApi.Filemanagerdelete(ctx).AccessToken(f.opt.AccessToken).Async(0).Filelist(fileList).Ondup("fail").Execute()
	return err
}

// Fs impl Abouter
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	resp, _, err := getSrv().UserinfoApi.Apiquota(ctx).AccessToken(f.opt.AccessToken).Checkexpire(0).Checkfree(0).Execute()
	if err != nil {
		return nil, err
	}
	return &fs.Usage{Total: resp.Total, Used: resp.Used, Free: resp.Free}, nil
}

// Object implement interface Object(ObjectInfo(DirEntry))
// Object implement DirEntry
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.filename
}
func (o *Object) Remote() string {
	return o.filename
}
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.mtime
}
func (o *Object) Size() int64 {
	return o.size
}

// Object implement ObjectInfo
func (o *Object) Fs() fs.Info {
	return o.fs
}
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	if t != hash.MD5 {
		return "", hash.ErrUnsupported
	}
	return o.md5, nil
}
func (o *Object) Storable() bool {
	return true
}

// Object implement Object
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	// TODO
	return errors.New("not implement")
}
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	// TODO
	return nil, errors.New("not implement")
}
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	// TODO
	return errors.New("not implement")
}
func (o *Object) Remove(ctx context.Context) error {
	// TODO
	return errors.New("not implement")
}

// Check the interfaces are satisfied
var (
	_ fs.Fs       = (*Fs)(nil)
	_ fs.Object   = (*Object)(nil)
	_ fs.DirEntry = (*Object)(nil)
)

// baidu SDK feature:
// UserinfoApi.Apiquota - get quota
// FileinfoApi.Xpanfilelist - get file list for a folder
// FilemanagerApi.Filemanagercopy - copy file
// FilemanagerApi.Filemanagermove - move file
// FilemanagerApi.Filemanagerrename - rename file
// FilemanagerApi.Filemanagerdelete - delete file
// FileuploadApi.Xpanfileprecreate, FileuploadApi.Pcssuperfile2, FileuploadApi.Xpanfilecreate - upload file
// MultimediafileApi.Xpanmultimediafilemetas - get direct link to download
