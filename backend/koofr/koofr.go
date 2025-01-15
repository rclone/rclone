// Package koofr provides an interface to the Koofr storage system.
package koofr

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/encoder"

	httpclient "github.com/koofr/go-httpclient"
	koofrclient "github.com/koofr/go-koofrclient"
)

// Register Fs with rclone
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "koofr",
		Description: "Koofr, Digi Storage and other Koofr-compatible storage providers",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name: fs.ConfigProvider,
			Help: "Choose your storage provider.",
			// NOTE if you add a new provider here, then add it in the
			// setProviderDefaults() function and update options accordingly
			Examples: []fs.OptionExample{{
				Value: "koofr",
				Help:  "Koofr, https://app.koofr.net/",
			}, {
				Value: "digistorage",
				Help:  "Digi Storage, https://storage.rcs-rds.ro/",
			}, {
				Value: "other",
				Help:  "Any other Koofr API compatible storage service",
			}},
		}, {
			Name:     "endpoint",
			Help:     "The Koofr API endpoint to use.",
			Provider: "other",
			Required: true,
		}, {
			Name:     "mountid",
			Help:     "Mount ID of the mount to use.\n\nIf omitted, the primary mount is used.",
			Advanced: true,
		}, {
			Name:     "setmtime",
			Help:     "Does the backend support setting modification time.\n\nSet this to false if you use a mount ID that points to a Dropbox or Amazon Drive backend.",
			Default:  true,
			Advanced: true,
		}, {
			Name:      "user",
			Help:      "Your user name.",
			Required:  true,
			Sensitive: true,
		}, {
			Name:       "password",
			Help:       "Your password for rclone generate one at https://app.koofr.net/app/admin/preferences/password.",
			Provider:   "koofr",
			IsPassword: true,
			Required:   true,
		}, {
			Name:       "password",
			Help:       "Your password for rclone generate one at https://storage.rcs-rds.ro/app/admin/preferences/password.",
			Provider:   "digistorage",
			IsPassword: true,
			Required:   true,
		}, {
			Name:       "password",
			Help:       "Your password for rclone (generate one at your service's settings page).",
			Provider:   "other",
			IsPassword: true,
			Required:   true,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			// Encode invalid UTF-8 bytes as json doesn't handle them properly.
			Default: (encoder.Display |
				encoder.EncodeBackSlash |
				encoder.EncodeInvalidUtf8),
		}},
	})
}

// Options represent the configuration of the Koofr backend
type Options struct {
	Provider string               `config:"provider"`
	Endpoint string               `config:"endpoint"`
	MountID  string               `config:"mountid"`
	User     string               `config:"user"`
	Password string               `config:"password"`
	SetMTime bool                 `config:"setmtime"`
	Enc      encoder.MultiEncoder `config:"encoding"`
}

// An Fs is a representation of a remote Koofr Fs
type Fs struct {
	name     string
	mountID  string
	root     string
	opt      Options
	features *fs.Features
	client   *koofrclient.KoofrClient
}

// An Object on the remote Koofr Fs
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

// String returns a string representation of the remote Object
func (o *Object) String() string {
	return o.remote
}

// Remote returns the remote path of the Object, relative to Fs root
func (o *Object) Remote() string {
	return o.remote
}

// ModTime returns the modification time of the Object
func (o *Object) ModTime(ctx context.Context) time.Time {
	return time.Unix(o.info.Modified/1000, (o.info.Modified%1000)*1000*1000)
}

// Size return the size of the Object in bytes
func (o *Object) Size() int64 {
	return o.info.Size
}

// Fs returns a reference to the Koofr Fs containing the Object
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Hash returns an MD5 hash of the Object
func (o *Object) Hash(ctx context.Context, typ hash.Type) (string, error) {
	if typ == hash.MD5 {
		return o.info.Hash, nil
	}
	return "", nil
}

// fullPath returns full path of the remote Object (including Fs root)
func (o *Object) fullPath() string {
	return o.fs.fullPath(o.remote)
}

// Storable returns true if the Object is storable
func (o *Object) Storable() bool {
	return true
}

// SetModTime is not supported
func (o *Object) SetModTime(ctx context.Context, mtime time.Time) error {
	return fs.ErrorCantSetModTimeWithoutDelete
}

// Open opens the Object for reading
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	var sOff, eOff int64 = 0, -1

	fs.FixRangeOption(options, o.Size())
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
	span := &koofrclient.FileSpan{
		Start: sOff,
		End:   eOff,
	}
	return o.fs.client.FilesGetRange(o.fs.mountID, o.fullPath(), span)
}

// Update updates the Object contents
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	mtime := src.ModTime(ctx).UnixNano() / 1000 / 1000
	putopts := &koofrclient.PutOptions{
		ForceOverwrite:             true,
		NoRename:                   true,
		OverwriteIgnoreNonExisting: true,
		SetModified:                &mtime,
	}
	fullPath := o.fullPath()
	dirPath := dir(fullPath)
	name := base(fullPath)
	err := o.fs.mkdir(dirPath)
	if err != nil {
		return err
	}
	info, err := o.fs.client.FilesPutWithOptions(o.fs.mountID, dirPath, name, in, putopts)
	if err != nil {
		return err
	}
	o.info = *info
	return nil
}

// Remove deletes the remote Object
func (o *Object) Remove(ctx context.Context) error {
	return o.fs.client.FilesDelete(o.fs.mountID, o.fullPath())
}

// Name returns the name of the Fs
func (f *Fs) Name() string {
	return f.name
}

// Root returns the root path of the Fs
func (f *Fs) Root() string {
	return f.root
}

// String returns a string representation of the Fs
func (f *Fs) String() string {
	return "koofr:" + f.mountID + ":" + f.root
}

// Features returns the optional features supported by this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Precision denotes that setting modification times is not supported
func (f *Fs) Precision() time.Duration {
	if !f.opt.SetMTime {
		return fs.ModTimeNotSupported
	}
	return time.Millisecond
}

// Hashes returns a set of hashes are Provided by the Fs
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.MD5)
}

// fullPath constructs a full, absolute path from an Fs root relative path,
func (f *Fs) fullPath(part string) string {
	return f.opt.Enc.FromStandardPath(path.Join("/", f.root, part))
}

func setProviderDefaults(opt *Options) {
	// handle old, provider-less configs
	if opt.Provider == "" {
		if opt.Endpoint == "" || strings.HasPrefix(opt.Endpoint, "https://app.koofr.net") {
			opt.Provider = "koofr"
		} else if strings.HasPrefix(opt.Endpoint, "https://storage.rcs-rds.ro") {
			opt.Provider = "digistorage"
		} else {
			opt.Provider = "other"
		}
	}
	// now assign an endpoint
	if opt.Provider == "koofr" {
		opt.Endpoint = "https://app.koofr.net"
	} else if opt.Provider == "digistorage" {
		opt.Endpoint = "https://storage.rcs-rds.ro"
	}
}

// NewFs constructs a new filesystem given a root path and rclone configuration options
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (ff fs.Fs, err error) {
	opt := new(Options)
	err = configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	setProviderDefaults(opt)
	return NewFsFromOptions(ctx, name, root, opt)
}

// NewFsFromOptions constructs a new filesystem given a root path and internal configuration options
func NewFsFromOptions(ctx context.Context, name, root string, opt *Options) (ff fs.Fs, err error) {
	pass, err := obscure.Reveal(opt.Password)
	if err != nil {
		return nil, err
	}
	httpClient := httpclient.New()
	httpClient.Client = fshttp.NewClient(ctx)
	client := koofrclient.NewKoofrClientWithHTTPClient(opt.Endpoint, httpClient)
	basicAuth := fmt.Sprintf("Basic %s",
		base64.StdEncoding.EncodeToString([]byte(opt.User+":"+pass)))
	client.HTTPClient.Headers.Set("Authorization", basicAuth)
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
	}).Fill(ctx, f)
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
			return nil, errors.New("failed to find primary mount")
		}
		return nil, errors.New("failed to find mount " + opt.MountID)
	}
	rootFile, err := f.client.FilesInfo(f.mountID, f.opt.Enc.FromStandardPath("/"+f.root))
	if err == nil && rootFile.Type != "dir" {
		f.root = dir(f.root)
		err = fs.ErrorIsFile
	} else {
		err = nil
	}
	return f, err
}

// List returns a list of items in a directory
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	files, err := f.client.FilesList(f.mountID, f.fullPath(dir))
	if err != nil {
		return nil, translateErrorsDir(err)
	}
	entries = make([]fs.DirEntry, len(files))
	for i, file := range files {
		remote := path.Join(dir, f.opt.Enc.ToStandardName(file.Name))
		if file.Type == "dir" {
			entries[i] = fs.NewDir(remote, time.Time{})
		} else {
			entries[i] = &Object{
				fs:     f,
				info:   file,
				remote: remote,
			}
		}
	}
	return entries, nil
}

// NewObject creates a new remote Object for a given remote path
func (f *Fs) NewObject(ctx context.Context, remote string) (obj fs.Object, err error) {
	info, err := f.client.FilesInfo(f.mountID, f.fullPath(remote))
	if err != nil {
		return nil, translateErrorsObject(err)
	}
	if info.Type == "dir" {
		return nil, fs.ErrorIsDir
	}
	return &Object{
		fs:     f,
		info:   info,
		remote: remote,
	}, nil
}

// Put updates a remote Object
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (obj fs.Object, err error) {
	mtime := src.ModTime(ctx).UnixNano() / 1000 / 1000
	putopts := &koofrclient.PutOptions{
		ForceOverwrite:             true,
		NoRename:                   true,
		OverwriteIgnoreNonExisting: true,
		SetModified:                &mtime,
	}
	fullPath := f.fullPath(src.Remote())
	dirPath := dir(fullPath)
	name := base(fullPath)
	err = f.mkdir(dirPath)
	if err != nil {
		return nil, err
	}
	info, err := f.client.FilesPutWithOptions(f.mountID, dirPath, name, in, putopts)
	if err != nil {
		return nil, translateErrorsObject(err)
	}
	return &Object{
		fs:     f,
		info:   *info,
		remote: src.Remote(),
	}, nil
}

// PutStream updates a remote Object with a stream of unknown size
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.Put(ctx, in, src, options...)
}

// isBadRequest is a predicate which holds true iff the error returned was
// HTTP status 400
func isBadRequest(err error) bool {
	switch err := err.(type) {
	case httpclient.InvalidStatusError:
		if err.Got == http.StatusBadRequest {
			return true
		}
	}
	return false
}

// translateErrorsDir translates koofr errors to rclone errors (for a dir
// operation)
func translateErrorsDir(err error) error {
	switch err := err.(type) {
	case httpclient.InvalidStatusError:
		if err.Got == http.StatusNotFound {
			return fs.ErrorDirNotFound
		}
	}
	return err
}

// translatesErrorsObject translates Koofr errors to rclone errors (for an object operation)
func translateErrorsObject(err error) error {
	switch err := err.(type) {
	case httpclient.InvalidStatusError:
		if err.Got == http.StatusNotFound {
			return fs.ErrorObjectNotFound
		}
	}
	return err
}

// mkdir creates a directory at the given remote path. Creates ancestors if
// necessary
func (f *Fs) mkdir(fullPath string) error {
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
			if err != nil && !isBadRequest(err) {
				return err
			}
		}
		parent = path.Join(parent, part)
	}
	return nil
}

// Mkdir creates a directory at the given remote path. Creates ancestors if
// necessary
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	fullPath := f.fullPath(dir)
	return f.mkdir(fullPath)
}

// Rmdir removes an (empty) directory at the given remote path
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
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

// Copy copies a remote Object to the given path
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	dstFullPath := f.fullPath(remote)
	dstDir := dir(dstFullPath)
	err := f.mkdir(dstDir)
	if err != nil {
		return nil, fs.ErrorCantCopy
	}
	mtime := src.ModTime(ctx).UnixNano() / 1000 / 1000
	err = f.client.FilesCopy((src.(*Object)).fs.mountID,
		(src.(*Object)).fs.fullPath((src.(*Object)).remote),
		f.mountID, dstFullPath, koofrclient.CopyOptions{SetModified: &mtime})
	if err != nil {
		return nil, fs.ErrorCantCopy
	}
	return f.NewObject(ctx, remote)
}

// Move moves a remote Object to the given path
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj := src.(*Object)
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
	return f.NewObject(ctx, remote)
}

// DirMove moves a remote directory to the given path
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) error {
	srcFs := src.(*Fs)
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

// About reports space usage (with a MiB precision)
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	mount, err := f.client.MountsDetails(f.mountID)
	if err != nil {
		return nil, err
	}
	return &fs.Usage{
		Total:   fs.NewUsageValue(mount.SpaceTotal * 1024 * 1024),
		Used:    fs.NewUsageValue(mount.SpaceUsed * 1024 * 1024),
		Trashed: nil,
		Other:   nil,
		Free:    fs.NewUsageValue((mount.SpaceTotal - mount.SpaceUsed) * 1024 * 1024),
		Objects: nil,
	}, nil
}

// Purge purges the complete Fs
func (f *Fs) Purge(ctx context.Context) error {
	err := translateErrorsDir(f.client.FilesDelete(f.mountID, f.fullPath("")))
	return err
}

// linkCreate is a Koofr API request for creating a public link
type linkCreate struct {
	Path string `json:"path"`
}

// link is a Koofr API response to creating a public link
type link struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Path             string `json:"path"`
	Counter          int64  `json:"counter"`
	URL              string `json:"url"`
	ShortURL         string `json:"shortUrl"`
	Hash             string `json:"hash"`
	Host             string `json:"host"`
	HasPassword      bool   `json:"hasPassword"`
	Password         string `json:"password"`
	ValidFrom        int64  `json:"validFrom"`
	ValidTo          int64  `json:"validTo"`
	PasswordRequired bool   `json:"passwordRequired"`
}

// createLink makes a Koofr API call to create a public link
func createLink(c *koofrclient.KoofrClient, mountID string, path string) (*link, error) {
	linkCreate := linkCreate{
		Path: path,
	}
	linkData := link{}

	request := httpclient.RequestData{
		Method:         "POST",
		Path:           "/api/v2/mounts/" + mountID + "/links",
		ExpectedStatus: []int{http.StatusOK, http.StatusCreated},
		ReqEncoding:    httpclient.EncodingJSON,
		ReqValue:       linkCreate,
		RespEncoding:   httpclient.EncodingJSON,
		RespValue:      &linkData,
	}

	_, err := c.Request(&request)
	if err != nil {
		return nil, err
	}
	return &linkData, nil
}

// PublicLink creates a public link to the remote path
func (f *Fs) PublicLink(ctx context.Context, remote string, expire fs.Duration, unlink bool) (string, error) {
	linkData, err := createLink(f.client, f.mountID, f.fullPath(remote))
	if err != nil {
		return "", translateErrorsDir(err)
	}

	// URL returned by API looks like following:
	//
	// https://app.koofr.net/links/35d9fb92-74a3-4930-b4ed-57f123bfb1a6
	//
	// Direct url looks like following:
	//
	// https://app.koofr.net/content/links/39a6cc01-3b23-477a-8059-c0fb3b0f15de/files/get?path=%2F
	//
	// I am not sure about meaning of "path" parameter; in my experiments
	// it is always "%2F", and omitting it or putting any other value
	// results in 404.
	//
	// There is one more quirk: direct link to file in / returns that file,
	// direct link to file somewhere else in hierarchy returns zip archive
	// with one member.
	link := linkData.URL
	link = strings.ReplaceAll(link, "/links", "/content/links")
	link += "/files/get?path=%2F"

	return link, nil
}
