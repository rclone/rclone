// Package terabox provides an interface to the Terabox storage system.
//
// resources for implementation:
// https://github.com/ivansaul/terabox_downloader
// https://gist.github.com/CypherpunkSamurai/58d8f2b669e101e893a6ecf3d3938412
// https://github.com/maiquocthinh/Terabox-DL
// https://github.com/fskhri/TeraboxDownloader
// https://github.com/AlistGo/alist/tree/main/drivers/terabox
//
// Documentation:
// https://www.terabox.com/integrations/docs?lang=en
package terabox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	libPath "path"
	"strconv"
	"sync"
	"time"

	"github.com/rclone/rclone/backend/terabox/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/rest"
)

const (
	baseURL = "https://www.terabox.com"

	// minSleep       = 400 * time.Millisecond // api is extremely rate limited now
	// maxSleep       = 5 * time.Second
	// decayConstant  = 2 // bigger for slower decay, exponential
	// attackConstant = 0 // start with max sleep
)

// Check the interfaces are satisfied
var (
	_ fs.Fs             = (*Fs)(nil)
	_ fs.Abouter        = (*Fs)(nil)
	_ fs.Copier         = (*Fs)(nil)
	_ fs.Mover          = (*Fs)(nil)
	_ fs.DirMover       = (*Fs)(nil)
	_ fs.Purger         = (*Fs)(nil)
	_ fs.CleanUpper     = (*Fs)(nil)
	_ fs.PutUncheckeder = (*Fs)(nil)

	_ fs.Object = (*Object)(nil)
)

func init() {
	fs.Register(&fs.RegInfo{
		Name:        "terabox",
		Description: "Terabox",
		NewFs:       NewFs,
		Options: []fs.Option{
			// {
			// 	Help:      "Your access token.",
			// 	Name:      "access_token",
			// 	Sensitive: true,
			// },
			{
				Help:     "Set full cookie string from browser or only 'ndus' value from cookie string",
				Name:     "cookie",
				Advanced: false,
				Required: true,
			},
			{
				Help:     "Clear Trash after deletion",
				Name:     "delete_permanently",
				Advanced: true,
				Default:  false,
			},
			{
				Help:     "Parallel upload threads",
				Name:     "upload_threads",
				Advanced: true,
				Default:  5,
			},
			{
				Help:     "Set custom header User Agent",
				Name:     "user_agent",
				Advanced: true,
				Default:  "terabox;1.37.0.7;PC;PC-Windows;10.0.22631;WindowsTeraBox",
			},
			{
				Help:     "Set extra debug level from 0 to 4 (0 - none; 1 - name of function and params; 2 - response output + body; 3 - request output, 4 - request body)",
				Name:     "debug_level",
				Advanced: true,
				Default:  0,
			},
			{
				Name:     config.ConfigEncoding,
				Help:     config.ConfigEncodingHelp,
				Advanced: true,
				// maxFileLength = 255
				Default: (encoder.Display |
					encoder.EncodeBackQuote |
					encoder.EncodeDoubleQuote |
					encoder.EncodeLtGt |
					encoder.EncodeLeftSpace |
					encoder.EncodeInvalidUtf8),
			}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	// AccessToken  string               `config:"access_token"`
	Cookie            string               `config:"cookie"`
	DeletePermanently bool                 `config:"delete_permanently"`
	UploadThreads     uint8                `config:"upload_threads"`
	UserAgent         string               `config:"user_agent"`
	DebugLevel        uint8                `config:"debug_level"`
	Enc               encoder.MultiEncoder `config:"encoding"`
}

//
//------------------------------------------------------------------------------------------------------------------------
//

// Fs is the interface a cloud storage system must provide
type Fs struct {
	root     string
	name     string
	opt      *Options
	features *fs.Features
	client   *rest.Client
	// pacer    *fs.Pacer

	origRoot     string
	origRootItem *api.Item

	baseURL     string
	notFirstRun bool

	// upload host should be got only once
	uploadHost   string
	uploadHostMX sync.Once

	// official access [added for future releases]
	accessToken string

	// unofficial access [web token required for upload]
	jsToken string

	isPremium   bool
	isPremiumMX sync.Once
}

// NewFs makes a new Fs object from the path
//
// The path is of the form remote:path
//
// Remotes are looked up in the config file.  If the remote isn't
// found then NotFoundInConfigFile will be returned.
//
// On Windows avoid single character remote names as they can be mixed
// up with drive letters.
//
// copyto: for srcFS the root is with the file name, for dstFS last segment (filename) will be cutted
func NewFs(ctx context.Context, name string, root string, config configmap.Mapper) (fs.Fs, error) {
	opt := new(Options)
	if err := configstruct.Set(config, opt); err != nil {
		return nil, err
	}
	opt.Cookie = valuedCookie(opt.Cookie)

	debug(opt, 1, "NewFS %s; %s; %+v;", name, root, opt)

	if len(root) > 0 {
		if root[0:1] == "." {
			root = root[1:]
		}

		if root[0:1] != "/" {
			root = "/" + root
		}

	} else if root == "" {
		root = "/"
	}

	f := &Fs{
		name:     name,
		root:     root,
		origRoot: root, // save origin root, because it can change, if path is file
		opt:      opt,
		// pacer: fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant), pacer.AttackConstant(attackConstant))),

		baseURL: baseURL,
		// jsToken: "",
	}

	f.features = (&fs.Features{
		ReadMetadata:            true,
		CanHaveEmptyDirectories: true,
	}).Fill(ctx, f)

	newCtx, clientConfig := fs.AddConfig(ctx)
	if opt.UserAgent != "" {
		clientConfig.UserAgent = opt.UserAgent
	}
	clientConfig.Timeout = 5 * fs.Duration(time.Second)

	f.client = rest.NewClient(fshttp.NewClient(newCtx))

	// update base url for official API Requests [not finished, some methods should be update for compatible]
	if f.accessToken != "" {
		f.baseURL += "/open"
	}

	// the root exists ever, have no reason to check it
	if root != "/" {
		var err error

		// cache the item, for do not request the same data, when will make NewObject on next step, if item is nil, then file or dir not found and we can skip request a List or NewObject
		if f.origRootItem, err = f.apiItemInfo(ctx, root, true); err != nil {
			if !api.ErrIsNum(err, -9) {
				return nil, err
			}
		} else if f.origRootItem.Isdir == 0 {
			f.root = libPath.Dir(root)

			// return an error with an fs which points to the parent of file
			return f, fs.ErrorIsFile
		}
	} else {
		// check the account is active
		if err := f.apiCheckLogin(ctx); err != nil {
			return nil, err
		}
	}

	return f, nil
}

//
// fs.Info interface implementation
//------------------------------------------------------------------------------------------------------------------------

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return f.root
}

// String returns a description of the FS
func (f *Fs) String() string {
	return fmt.Sprintf("Terabox root '%s'", f.root)
}

// Precision of the ModTimes in this Fs
func (f *Fs) Precision() time.Duration {
	return fs.ModTimeNotSupported
}

// Hashes returns the supported hash types of the filesystem
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.None)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

//
// fs.Abouter Interface implementation [optional]
//------------------------------------------------------------------------------------------------------------------------

// About gets quota information from the Fs.
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	debug(f.opt, 1, "About")

	info, err := f.apiQuotaInfo(ctx)
	if err != nil {
		return nil, err
	}

	free := info.Total - info.Used // the server returns a free value equal to the total, that's why we calculate it manually
	return &fs.Usage{Total: &info.Total, Used: &info.Used, Free: &free}, nil
}

//
// fs.Fs interface implementation
//------------------------------------------------------------------------------------------------------------------------

// List the objects and directories in dir into entries.  The
// entries can be returned in any order but should be for a
// complete directory.
//
// dir should be "" to list the root, and should not have
// trailing slashes.
//
// This should return ErrDirNotFound if the directory isn't
// found.
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	debug(f.opt, 1, "List %s;", dir)

	if f.root != "/" && f.origRootItem == nil {
		return nil, fs.ErrorDirNotFound
	}

	list, err := f.apiList(ctx, libPath.Join(f.root, dir))
	if err != nil {
		if api.ErrIsNum(err, -9) {
			return nil, fs.ErrorDirNotFound
		}

		return nil, err
	}

	for _, item := range list {
		remote := libPath.Join(dir, f.opt.Enc.ToStandardName(item.Name))
		if item.Isdir > 0 {
			dir := fs.NewDir(remote, time.Time{}).SetID(strconv.FormatUint(item.ID, 10))
			entries = append(entries, dir)
		} else {
			file := &Object{
				fs:      f,
				id:      item.ID,
				remote:  remote,
				size:    item.Size,
				modTime: time.Unix(item.ServerModifiedTime, 0),
				hash:    item.MD5,
			}

			entries = append(entries, file)
		}
	}

	return entries, nil
}

// NewObject finds the Object at remote.  If it can't be found it
// returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	debug(f.opt, 1, "NewObject %s;", remote)

	var item *api.Item
	if f.origRoot == libPath.Join(f.root, remote) {
		if f.origRootItem == nil {
			return nil, fs.ErrorObjectNotFound
		}
		item = f.origRootItem
	} else {
		var err error
		item, err = f.apiItemInfo(ctx, libPath.Join(f.root, remote), true)
		if err != nil {
			if api.ErrIsNum(err, -9) {
				return nil, fs.ErrorObjectNotFound
			}

			return nil, err
		}
	}

	if item.Isdir > 0 {
		return nil, fs.ErrorIsDir
	}

	return &Object{
		fs:           f,
		id:           item.ID,
		remote:       remote,
		size:         item.Size,
		modTime:      time.Unix(item.ServerModifiedTime, 0),
		hash:         item.MD5,
		downloadLink: item.DownloadLink,
	}, nil
}

// Put in to the remote path with the modTime given of the given size
//
// When called from outside an Fs by rclone, src.Size() will always be >= 0.
// But for unknown-sized objects (indicated by src.Size() == -1), Put should either
// return an error or upload it properly (rather than e.g. calling panic).
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	debug(f.opt, 1, "Put %p; %+v; %+v;", in, src, options)

	if src.Size() < 0 {
		return nil, errors.New("refusing to update with unknown size")
	}

	if err := f.apiFileUpload(ctx, libPath.Join(f.root, src.Remote()), src.Size(), src.ModTime(ctx), in, options, 0); err != nil {
		return nil, err
	}

	return f.NewObject(ctx, src.Remote())
}

// Mkdir makes the directory (container, bucket)
//
// Shouldn't return an error if it already exists
func (f *Fs) Mkdir(ctx context.Context, dir string) (err error) {
	debug(f.opt, 1, "Mkdir %s;", dir)

	pth := libPath.Join(f.root, dir)
	if pth == "" || pth == "." || pth == "/" {
		return nil
	}

	if err := f.apiMkDir(ctx, pth); err != nil && !api.ErrIsNum(err, -8) {
		return err
	}

	return nil
}

// Rmdir removes the directory (container, bucket) if empty
//
// Return an error if it doesn't exist or isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	debug(f.opt, 1, "Rmdir %s;", dir)

	if items, err := f.List(ctx, dir); err != nil {
		return err
	} else if len(items) != 0 {
		return fs.ErrorDirectoryNotEmpty
	}

	return f.apiOperation(ctx, "delete", []api.OperationalItem{
		{Path: libPath.Join(f.root, dir)},
	})
}

//
// fs.PutUncheckeder Interface implementation [optional]
//------------------------------------------------------------------------------------------------------------------------

// PutUnchecked put in to the remote path with the modTime given of the given size
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
//
// May create duplicates or return errors if src already
// exists.
func (f *Fs) PutUnchecked(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	debug(f.opt, 1, "PutUnchecked %p; %+v; %+v;", in, src, options)

	if src.Size() < 0 {
		return nil, errors.New("refusing to update with unknown size")
	}

	if err := f.apiFileUpload(ctx, libPath.Join(f.root, src.Remote()), src.Size(), src.ModTime(ctx), in, options, 1); err != nil {
		return nil, err
	}

	return f.NewObject(ctx, src.Remote())
}

//
// fs.Copier Interface implementation [optional]
//------------------------------------------------------------------------------------------------------------------------

// Copy src to this remote using server side operations.
// It returns the destination Object and a possible error
//
// # Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantCopy
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	debug(f.opt, 1, "Copy %+v; %s;", src, remote)

	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}

	srcPath := libPath.Join(srcObj.fs.root, srcObj.remote)
	if f.origRootItem == nil && f.root != "/" {
		if err := f.apiMkDir(ctx, f.root); err != nil && !api.ErrIsNum(err, -8) {
			return nil, err
		}
	}

	if err := f.apiOperation(ctx, "copy", []api.OperationalItem{{Path: srcPath, Destination: f.root, NewName: remote}}); err != nil {
		return nil, fmt.Errorf("couldn't copy file: %w", err)
	}

	return f.NewObject(ctx, remote)
}

//
// fs.Mover Interface implementation [optional]
//------------------------------------------------------------------------------------------------------------------------

// Move src to this remote using server side move operations.
// This is stored with the remote path given
// It returns the destination Object and a possible error
//
// # Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantMove
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	debug(f.opt, 1, "Move %+v; %s;", src, remote)

	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}

	srcPath := libPath.Join(srcObj.fs.root, srcObj.remote)
	if f.origRootItem == nil && f.root != "/" {
		if err := f.apiMkDir(ctx, f.root); err != nil && !api.ErrIsNum(err, -8) {
			return nil, err
		}
	}

	if err := f.apiOperation(ctx, "move", []api.OperationalItem{{Path: srcPath, Destination: f.root, NewName: remote}}); err != nil {
		return nil, fmt.Errorf("couldn't move file: %w", err)
	}

	return f.NewObject(ctx, remote)
}

//
// fs.DirMove Interface implementation [optional]
//------------------------------------------------------------------------------------------------------------------------

// DirMove moves src, srcRemote to this remote at dstRemote
// using server-side move operations.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantDirMove
//
// If destination exists then return fs.ErrorDirExists
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) error {
	debug(f.opt, 1, "DirMove %+v; %s; %s;", src, srcRemote, dstRemote)

	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debugf(src, "Can't copy - not same remote type")
		return fs.ErrorCantDirMove
	}

	srcPath := libPath.Join(srcFs.root, srcRemote)
	if srcFs.root != "/" && srcFs.origRootItem == nil {
		return fmt.Errorf("source directory not found")
	}

	dstPath, name := libPath.Split(libPath.Join(f.root, dstRemote))
	if name == "" {
		return fmt.Errorf("couldn't move root directory")
	}

	if f.root != "/" && f.origRootItem == nil {
		if err := f.apiMkDir(ctx, f.root); err != nil && !api.ErrIsNum(err, -8) {
			return err
		}
	}

	if err := f.apiOperation(ctx, "move", []api.OperationalItem{{Path: srcPath, Destination: dstPath, NewName: name}}); err != nil {
		return fmt.Errorf("couldn't move directory: %w", err)
	}

	return nil
}

// fs.Purger Interface implementation [optional]
// ------------------------------------------------------------------------------------------------------------------------

// Purge all files in the directory specified
//
// Implement this if you have a way of deleting all the files
// quicker than just running Remove() on the result of List()
//
// Return an error if it doesn't exist
func (f *Fs) Purge(ctx context.Context, dir string) error {
	debug(f.opt, 1, "Purge %s;", dir)

	return f.apiOperation(ctx, "delete", []api.OperationalItem{
		{Path: libPath.Join(f.root, dir)},
	})
}

//
// fs.Cleaner Interface implementation [optional]
//------------------------------------------------------------------------------------------------------------------------

// CleanUp the trash in the Fs
//
// Implement this if you have a way of emptying the trash or
// otherwise cleaning up old versions of files.
func (f *Fs) CleanUp(ctx context.Context) error {
	debug(f.opt, 1, "CleanUp")

	return f.apiCleanRecycleBin(ctx)
}

//
//
//

// Object represents an Terabox object
type Object struct {
	fs           *Fs       // what this object is part of
	id           uint64    // file id
	remote       string    // The remote path
	size         int64     // Bytes in the object
	modTime      time.Time // Modified time of the object
	hash         string    // md5
	downloadLink string    // download link, available only for for objects which created by NewObject method
}

//
// ObjectInfo Interface implementation
//------------------------------------------------------------------------------------------------------------------------

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

// ModTime returns the modification time of the object
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.modTime
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	return o.size
}

// Hash returns the Md5sum of an object returning a lowercase hex string
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	return o.hash, nil
}

// Storable returns whether this object is storable
func (o *Object) Storable() bool {
	return true
}

//
// fs.IDer Interface implementation [optional]
//------------------------------------------------------------------------------------------------------------------------

// ID returns the ID of the Object if known, or "" if not
func (o *Object) ID() string {
	return fmt.Sprintf("%d", o.id)
}

//
// Object Interface implementation
//------------------------------------------------------------------------------------------------------------------------

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	return fs.ErrorCantSetModTime
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	debug(o.fs.opt, 1, "Object Open %+v;", options)

	if o.downloadLink == "" {
		if item, err := o.fs.apiItemInfo(ctx, libPath.Join(o.fs.root, o.remote), true); err == nil && item.DownloadLink != "" {
			o.downloadLink = item.DownloadLink
		}
	}

	if o.downloadLink == "" {
		if res, err := o.fs.apiDownloadLink(ctx, o.id); err != nil {
			return nil, err
		} else if len(res.DownloadLink) > 0 {
			o.downloadLink = res.DownloadLink[0].URL
		}
	}

	if o.downloadLink == "" {
		return nil, fs.ErrorObjectNotFound
	}

	fs.FixRangeOption(options, o.size)
	resp, err := o.fs.client.Call(ctx, &rest.Opts{Method: http.MethodGet, RootURL: o.downloadLink, Options: options})
	if err != nil {
		return nil, err
	}

	return resp.Body, nil
}

// Update the already existing object
//
// Copy the reader into the object updating modTime and size.
//
// The new object may have been created if an error is returned
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	debug(o.fs.opt, 1, "Object Update %p; %+v; %+v;", in, src, options)

	if src.Size() < 0 {
		return errors.New("refusing to update with unknown size")
	}

	if err := o.fs.apiFileUpload(ctx, libPath.Join(o.fs.root, o.remote), src.Size(), src.ModTime(ctx), in, options, 3); err != nil {
		return err
	}

	// Fetch new object after deleting the duplicate
	newO, err := o.fs.NewObject(ctx, o.Remote())
	if err != nil {
		return err
	}

	// Replace guts of old object with new one
	*o = *newO.(*Object)

	return nil
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	debug(o.fs.opt, 1, "Remove")

	return o.fs.apiOperation(ctx, "delete", []api.OperationalItem{
		{Path: libPath.Join(o.fs.root, o.remote)},
	})
}
