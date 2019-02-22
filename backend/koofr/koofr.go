package koofr

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/config/configmap"
	"github.com/ncw/rclone/fs/config/configstruct"
	"github.com/ncw/rclone/fs/config/obscure"
	"github.com/ncw/rclone/fs/hash"

	httpclient "github.com/koofr/go-httpclient"
	koofrclient "github.com/koofr/go-koofrclient"
)

func init() {
	fs.Register(&fs.RegInfo{
		Name:        "koofr",
		Description: "Koofr",
		NewFs:       NewFs,
		Options: []fs.Option{
			{
				Name:     "baseurl",
				Help:     "Base URL of the Koofr API to connect to",
				Default:  "https://app.koofr.net",
				Required: true,
				Advanced: true,
			}, {
				Name:     "mountid",
				Help:     "Mount ID of the mount to use. If omitted, the primary mount is used.",
				Required: false,
				Default:  "",
				Advanced: true,
			}, {
				Name:     "user",
				Help:     "Your Koofr user name",
				Required: true,
			}, {
				Name:       "password",
				Help:       "Your Koofr password for rclone (generate one at https://app.koofr.net/app/admin/preferences/password)",
				IsPassword: true,
				Required:   true,
			},
		},
	})
}

type options struct {
	BaseURL  string `config:"baseurl"`
	MountID  string `config:"mountid"`
	User     string `config:"user"`
	Password string `config:"password"`
}

type koofrfs struct {
	name     string
	mountID  string
	root     string
	opt      options
	features *fs.Features
	client   *koofrclient.KoofrClient
}

type object struct {
	fs     *koofrfs
	remote string
	info   koofrclient.FileInfo
}

func base(pth string) string {
	rv := path.Base(pth)
	if rv == "" || rv == "." {
		rv = "/"
	}
	return rv
}

func dir(pth string) string {
	rv := path.Dir(pth)
	if rv == "" || rv == "." {
		rv = "/"
	}
	return rv
}

func (o *object) String() string {
	return o.remote
}

func (o *object) Remote() string {
	return o.remote
}

func (o *object) ModTime() time.Time {
	return time.Unix(o.info.Modified/1000, (o.info.Modified%1000)*1000*1000)
}

func (o *object) Size() int64 {
	return o.info.Size
}

func (o *object) Fs() fs.Info {
	return o.fs
}

func (o *object) Hash(typ hash.Type) (string, error) {
	if typ == hash.MD5 {
		return o.info.Hash, nil
	}
	return "", nil
}

func (o *object) fullPath() string {
	return o.fs.fullPath(o.remote)
}

func (o *object) Storable() bool {
	return true
}

func (o *object) SetModTime(mtime time.Time) error {
	return nil
}

func (o *object) Open(options ...fs.OpenOption) (io.ReadCloser, error) {
	var sOff, eOff int64 = 0, -1

	for _, option := range options {
		switch x := option.(type) {
		case *fs.SeekOption:
			sOff = x.Offset
		case *fs.RangeOption:
			sOff = x.Start
			eOff = x.End
		default:
			if option.Mandatory() {
				fs.Logf(o, "Unsupported mandatory option: %v", option)
			}
		}
	}
	if sOff == 0 && eOff < 0 {
		return o.fs.client.FilesGet(o.fs.mountID, o.fullPath())
	}
	if sOff < 0 {
		sOff = o.Size() - eOff
		eOff = o.Size()
	}
	if eOff > o.Size() {
		eOff = o.Size()
	}
	span := &koofrclient.FileSpan{
		Start: sOff,
		End:   eOff,
	}
	return o.fs.client.FilesGetRange(o.fs.mountID, o.fullPath(), span)
}

func (o *object) Update(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	putopts := &koofrclient.PutFilter{
		ForceOverwrite:    true,
		NoRename:          true,
		IgnoreNonExisting: true,
	}
	fullPath := o.fullPath()
	dirPath := dir(fullPath)
	name := base(fullPath)
	err := o.fs.mkdir(dirPath)
	if err != nil {
		return err
	}
	info, err := o.fs.client.FilesPutOptions(o.fs.mountID, dirPath, name, in, putopts)
	if err != nil {
		return err
	}
	o.info = *info
	return nil
}

func (o *object) Remove() error {
	return o.fs.client.FilesDelete(o.fs.mountID, o.fullPath())
}

func (f *koofrfs) Name() string {
	return f.name
}

func (f *koofrfs) Root() string {
	return f.root
}

func (f *koofrfs) String() string {
	return "koofr:" + f.mountID + ":" + f.root
}

func (f *koofrfs) Features() *fs.Features {
	return f.features
}

func (f *koofrfs) Precision() time.Duration {
	return fs.ModTimeNotSupported
}

func (f *koofrfs) Hashes() hash.Set {
	return hash.Set(hash.MD5)
}

func (f *koofrfs) fullPath(part string) string {
	return path.Join("/", f.root, part)
}

func NewFs(name, root string, m configmap.Mapper) (ff fs.Fs, err error) {
	opt := new(options)
	err = configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	pass, err := obscure.Reveal(opt.Password)
	if err != nil {
		return nil, err
	}
	client := koofrclient.NewKoofrClient(opt.BaseURL, false)
	basicAuth := fmt.Sprintf("Basic %s",
		base64.StdEncoding.EncodeToString([]byte(opt.User+":"+pass)))
	client.HTTPClient.Headers.Set("Authorization", basicAuth)
	mounts, err := client.Mounts()
	if err != nil {
		return nil, err
	}
	f := &koofrfs{
		name:   name,
		root:   root,
		opt:    *opt,
		client: client,
	}
	f.features = (&fs.Features{
		CaseInsensitive:         true,
		DuplicateFiles:          false,
		BucketBased:             false,
		CanHaveEmptyDirectories: true,
	}).Fill(f)
	for _, m := range mounts {
		if opt.MountID != "" {
			if m.Id == opt.MountID {
				f.mountID = m.Id
				break
			}
		} else if m.IsPrimary {
			f.mountID = m.Id
			break
		}
	}
	if f.mountID == "" {
		if opt.MountID == "" {
			return nil, errors.New("Failed to find primary mount")
		}
		return nil, errors.New("Failed to find mount " + opt.MountID)
	}
	rootFile, err := f.client.FilesInfo(f.mountID, "/"+f.root)
	if err == nil && rootFile.Type != "dir" {
		f.root = dir(f.root)
		err = fs.ErrorIsFile
	} else {
		err = nil
	}
	return f, err
}

func (f *koofrfs) List(dir string) (entries fs.DirEntries, err error) {
	files, err := f.client.FilesList(f.mountID, f.fullPath(dir))
	if err != nil {
		return nil, translateErrorsDir(err)
	}
	entries = make([]fs.DirEntry, len(files))
	for i, file := range files {
		if file.Type == "dir" {
			entries[i] = fs.NewDir(path.Join(dir, file.Name), time.Unix(0, 0))
		} else {
			entries[i] = &object{
				fs:     f,
				info:   file,
				remote: path.Join(dir, file.Name),
			}
		}
	}
	return entries, nil
}

func (f *koofrfs) NewObject(remote string) (obj fs.Object, err error) {
	info, err := f.client.FilesInfo(f.mountID, f.fullPath(remote))
	if err != nil {
		return nil, translateErrorsObject(err)
	}
	if info.Type == "dir" {
		return nil, fs.ErrorNotAFile
	}
	return &object{
		fs:     f,
		info:   info,
		remote: remote,
	}, nil
}

func (f *koofrfs) Put(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (obj fs.Object, err error) {
	putopts := &koofrclient.PutFilter{
		ForceOverwrite:    true,
		NoRename:          true,
		IgnoreNonExisting: true,
	}
	fullPath := f.fullPath(src.Remote())
	dirPath := dir(fullPath)
	name := base(fullPath)
	err = f.mkdir(dirPath)
	if err != nil {
		return nil, err
	}
	info, err := f.client.FilesPutOptions(f.mountID, dirPath, name, in, putopts)
	if err != nil {
		return nil, translateErrorsObject(err)
	}
	return &object{
		fs:     f,
		info:   *info,
		remote: src.Remote(),
	}, nil
}

func (f *koofrfs) PutStream(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.Put(in, src, options...)
}

func translateErrorsDir(err error) error {
	switch err := err.(type) {
	case httpclient.InvalidStatusError:
		if err.Got == http.StatusNotFound {
			return fs.ErrorDirNotFound
		}
	}
	return err
}

func translateErrorsObject(err error) error {
	switch err := err.(type) {
	case httpclient.InvalidStatusError:
		if err.Got == http.StatusNotFound {
			return fs.ErrorObjectNotFound
		}
	}
	return err
}

func (f *koofrfs) mkdir(fullPath string) error {
	if fullPath == "/" {
		return nil
	}
	info, err := f.client.FilesInfo(f.mountID, fullPath)
	if err == nil && info.Type == "dir" {
		return nil
	}
	err = translateErrorsDir(err)
	if err != nil && err != fs.ErrorDirNotFound {
		return err
	}
	dirs := strings.Split(fullPath, "/")
	parent := "/"
	for _, part := range dirs {
		if part == "" {
			continue
		}
		info, err = f.client.FilesInfo(f.mountID, path.Join(parent, part))
		if err != nil || info.Type != "dir" {
			err = translateErrorsDir(err)
			if err != nil && err != fs.ErrorDirNotFound {
				return err
			}
			err = f.client.FilesNewFolder(f.mountID, parent, part)
			if err != nil {
				return err
			}
		}
		parent = path.Join(parent, part)
	}
	return nil
}

func (f *koofrfs) Mkdir(dir string) error {
	fullPath := f.fullPath(dir)
	return f.mkdir(fullPath)
}

func (f *koofrfs) Rmdir(dir string) error {
	files, err := f.client.FilesList(f.mountID, f.fullPath(dir))
	if err != nil {
		return translateErrorsDir(err)
	}
	if len(files) > 0 {
		return fs.ErrorDirectoryNotEmpty
	}
	err = f.client.FilesDelete(f.mountID, f.fullPath(dir))
	if err != nil {
		return translateErrorsDir(err)
	}
	return nil
}

func (f *koofrfs) Copy(src fs.Object, remote string) (fs.Object, error) {
	dstFullPath := f.fullPath(remote)
	dstDir := dir(dstFullPath)
	err := f.mkdir(dstDir)
	if err != nil {
		return nil, fs.ErrorCantCopy
	}
	err = f.client.FilesCopy((src.(*object)).fs.mountID,
		(src.(*object)).fs.fullPath((src.(*object)).remote),
		f.mountID, dstFullPath)
	if err != nil {
		return nil, fs.ErrorCantCopy
	}
	return f.NewObject(remote)
}

func (f *koofrfs) Move(src fs.Object, remote string) (fs.Object, error) {
	srcObj := src.(*object)
	dstFullPath := f.fullPath(remote)
	dstDir := dir(dstFullPath)
	err := f.mkdir(dstDir)
	if err != nil {
		return nil, fs.ErrorCantMove
	}
	err = f.client.FilesMove(srcObj.fs.mountID,
		srcObj.fs.fullPath(srcObj.remote), f.mountID, dstFullPath)
	if err != nil {
		return nil, fs.ErrorCantMove
	}
	return f.NewObject(remote)
}

func (f *koofrfs) DirMove(src fs.Fs, srcRemote, dstRemote string) error {
	srcFs := src.(*koofrfs)
	srcFullPath := srcFs.fullPath(srcRemote)
	dstFullPath := f.fullPath(dstRemote)
	if srcFs.mountID == f.mountID && srcFullPath == dstFullPath {
		return fs.ErrorDirExists
	}
	dstDir := dir(dstFullPath)
	err := f.mkdir(dstDir)
	if err != nil {
		return fs.ErrorCantDirMove
	}
	err = f.client.FilesMove(srcFs.mountID, srcFullPath, f.mountID, dstFullPath)
	if err != nil {
		return fs.ErrorCantDirMove
	}
	return nil
}
