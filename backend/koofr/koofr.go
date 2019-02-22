package koofr

import (
	"errors"
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
		Description: "Koofr Connection",
		NewFs:       NewFs,
		Options: []fs.Option{
			{
				Name:     "baseurl",
				Help:     "Base URL of the Koofr API to connect to",
				Default:  "https://app.koofr.net",
				Required: true,
			}, {
				Name:     "user",
				Help:     "Your Koofr user name",
				Required: true,
			}, {
				Name:       "password",
				Help:       "Your Koofr app password",
				IsPassword: true,
				Required:   true,
			},
		},
	})
}

type Options struct {
	BaseUrl  string `config:"baseurl"`
	User     string `config:"user"`
	Password string `config:"password"`
}

type Fs struct {
	name     string
	mountId  string
	root     string
	opt      Options
	features *fs.Features
	client   *koofrclient.KoofrClient
}

type Object struct {
	fs     *Fs
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

func (o *Object) String() string {
	return o.remote
}

func (o *Object) Remote() string {
	return o.remote
}

func (o *Object) ModTime() time.Time {
	return time.Unix(o.info.Modified/1000, (o.info.Modified%1000)*1000*1000)
}

func (o *Object) Size() int64 {
	return o.info.Size
}

func (o *Object) Fs() fs.Info {
	return o.fs
}

func (o *Object) Hash(typ hash.Type) (string, error) {
	if typ == hash.MD5 {
		return o.info.Hash, nil
	} else {
		return "", nil
	}
}

func (o *Object) fullPath() string {
	return o.fs.fullPath(o.remote)
}

func (o *Object) Storable() bool {
	return true
}

func (o *Object) SetModTime(mtime time.Time) error {
	return nil
}

func (o *Object) Open(options ...fs.OpenOption) (io.ReadCloser, error) {
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
		return o.fs.client.FilesGet(o.fs.mountId, o.fullPath())
	} else {
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
		return o.fs.client.FilesGetRange(o.fs.mountId, o.fullPath(), span)
	}
}

func (o *Object) Update(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	putopts := &koofrclient.PutFilter{
		ForceOverwrite:    true,
		NoRename:          true,
		IgnoreNonExisting: true,
	}
	fullPath := o.fullPath()
	dirPath := dir(fullPath)
	name := base(fullPath)
	o.fs.mkdir(dirPath)
	info, err := o.fs.client.FilesPutOptions(o.fs.mountId, dirPath, name, in, putopts)
	if err != nil {
		return err
	}
	o.info = *info
	return nil
}

func (o *Object) Remove() error {
	return o.fs.client.FilesDelete(o.fs.mountId, o.fullPath())
}

func (f *Fs) Name() string {
	return f.name
}

func (f *Fs) Root() string {
	return f.root
}

func (f *Fs) String() string {
	return "koofr:" + f.mountId + ":" + f.root
}

func (f *Fs) Features() *fs.Features {
	return f.features
}

func (f *Fs) Precision() time.Duration {
	return fs.ModTimeNotSupported
}

func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.MD5)
}

func (f *Fs) fullPath(part string) string {
	return path.Join("/", f.root, part)
}

func NewFs(name, root string, m configmap.Mapper) (ff fs.Fs, err error) {
	opt := new(Options)
	err = configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	pass, err := obscure.Reveal(opt.Password)
	if err != nil {
		return nil, err
	}
	client := koofrclient.NewKoofrClient(opt.BaseUrl, false)
	err = client.Authenticate(opt.User, pass)
	if err != nil {
		return nil, err
	}
	mounts, err := client.Mounts()
	if err != nil {
		return nil, err
	}
	f := &Fs{
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
		if m.IsPrimary {
			f.mountId = m.Id
		}
	}
	if f.mountId == "" {
		return nil, errors.New("Failed to find primary mount")
	}
	rootFile, err := f.client.FilesInfo(f.mountId, "/"+f.root)
	if err == nil && rootFile.Type != "dir" {
		f.root = dir(f.root)
		err = fs.ErrorIsFile
	} else {
		err = nil
	}
	return f, err
}

func (f *Fs) List(dir string) (entries fs.DirEntries, err error) {
	files, err := f.client.FilesList(f.mountId, f.fullPath(dir))
	if err != nil {
		return nil, translateErrorsDir(err)
	}
	entries = make([]fs.DirEntry, len(files))
	for i, file := range files {
		if file.Type == "dir" {
			entries[i] = fs.NewDir(path.Join(dir, file.Name), time.Unix(0, 0))
		} else {
			entries[i] = &Object{
				fs:     f,
				info:   file,
				remote: path.Join(dir, file.Name),
			}
		}
	}
	return entries, nil
}

func (f *Fs) NewObject(remote string) (obj fs.Object, err error) {
	info, err := f.client.FilesInfo(f.mountId, f.fullPath(remote))
	if err != nil {
		return nil, translateErrorsObject(err)
	}
	if info.Type == "dir" {
		return nil, fs.ErrorNotAFile
	}
	return &Object{
		fs:     f,
		info:   info,
		remote: remote,
	}, nil
}

func (f *Fs) Put(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (obj fs.Object, err error) {
	putopts := &koofrclient.PutFilter{
		ForceOverwrite:    true,
		NoRename:          true,
		IgnoreNonExisting: true,
	}
	fullPath := f.fullPath(src.Remote())
	dirPath := dir(fullPath)
	name := base(fullPath)
	f.mkdir(dirPath)
	info, err := f.client.FilesPutOptions(f.mountId, dirPath, name, in, putopts)
	if err != nil {
		return nil, translateErrorsDir(err)
	}
	return &Object{
		fs:     f,
		info:   *info,
		remote: src.Remote(),
	}, nil
}

func (f *Fs) PutStream(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
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

func (f *Fs) mkdir(fullPath string) error {
	if fullPath == "/" {
		return nil
	}
	info, err := f.client.FilesInfo(f.mountId, fullPath)
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
		info, err = f.client.FilesInfo(f.mountId, path.Join(parent, part))
		if err == nil && info.Type == "dir" {

		} else {
			err = translateErrorsDir(err)
			if err != nil && err != fs.ErrorDirNotFound {
				return err
			}
			err = f.client.FilesNewFolder(f.mountId, parent, part)
			if err != nil {
				return err
			}
		}
		parent = path.Join(parent, part)
	}
	return nil
}

func (f *Fs) Mkdir(dir string) error {
	fullPath := f.fullPath(dir)
	return f.mkdir(fullPath)
}

func (f *Fs) Rmdir(dir string) error {
	files, err := f.client.FilesList(f.mountId, f.fullPath(dir))
	if err != nil {
		return translateErrorsDir(err)
	}
	if len(files) > 0 {
		return fs.ErrorDirectoryNotEmpty
	}
	err = f.client.FilesDelete(f.mountId, f.fullPath(dir))
	if err != nil {
		return translateErrorsDir(err)
	}
	return nil
}

func (f *Fs) Copy(src fs.Object, remote string) (fs.Object, error) {
	dstFullPath := f.fullPath(remote)
	dstDir := dir(dstFullPath)
	err := f.mkdir(dstDir)
	if err != nil {
		return nil, fs.ErrorCantCopy
	}
	err = f.client.FilesCopy((src.(*Object)).fs.mountId,
		(src.(*Object)).fs.fullPath((src.(*Object)).remote),
		f.mountId, dstFullPath)
	if err != nil {
		return nil, fs.ErrorCantCopy
	}
	return f.NewObject(remote)
}

func (f *Fs) Move(src fs.Object, remote string) (fs.Object, error) {
	srcObj := src.(*Object)
	dstFullPath := f.fullPath(remote)
	dstDir := dir(dstFullPath)
	err := f.mkdir(dstDir)
	if err != nil {
		return nil, fs.ErrorCantMove
	}
	err = f.client.FilesMove(srcObj.fs.mountId,
		srcObj.fs.fullPath(srcObj.remote), f.mountId, dstFullPath)
	if err != nil {
		return nil, fs.ErrorCantMove
	}
	return f.NewObject(remote)
}

func (f *Fs) DirMove(src fs.Fs, srcRemote, dstRemote string) error {
	srcFs := src.(*Fs)
	srcFullPath := srcFs.fullPath(srcRemote)
	dstFullPath := f.fullPath(dstRemote)
	if srcFs.mountId == f.mountId && srcFullPath == dstFullPath {
		return fs.ErrorDirExists
	}
	dstDir := dir(dstFullPath)
	err := f.mkdir(dstDir)
	if err != nil {
		return fs.ErrorCantDirMove
	}
	err = f.client.FilesMove(srcFs.mountId, srcFullPath, f.mountId, dstFullPath)
	if err != nil {
		return fs.ErrorCantDirMove
	} else {
		return nil
	}
}
