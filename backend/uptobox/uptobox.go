package uptobox

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/backend/uptobox/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/random"
	"github.com/rclone/rclone/lib/rest"
)

const (
	apiBaseURL     = "https://uptobox.com/api"
	minSleep       = 400 * time.Millisecond // api is extremely rate limited now
	maxSleep       = 5 * time.Second
	decayConstant  = 2 // bigger for slower decay, exponential
	attackConstant = 0 // start with max sleep
)

func init() {
	fs.Register(&fs.RegInfo{
		Name:        "uptobox",
		Description: "Uptobox",
		Config: func(ctx context.Context, name string, config configmap.Mapper) {
		},
		NewFs: NewFs,
		Options: []fs.Option{{
			Help: "Your access Token, get it from https://uptobox.com/my_account",
			Name: "access_token",
		}, {
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
	AccessToken string               `config:"access_token"`
	Enc         encoder.MultiEncoder `config:"encoding"`
}

// Fs is the interface a cloud storage system must provide
type Fs struct {
	root     string
	name     string
	opt      Options
	features *fs.Features
	srv      *rest.Client
	pacer    *fs.Pacer
	IDRegexp *regexp.Regexp
}

// Object represents an Uptobox object
type Object struct {
	fs          *Fs    // what this object is part of
	remote      string // The remote path
	hasMetaData bool   // whether info below has been set
	size        int64  // Bytes in the object
	//	modTime     time.Time // Modified time of the object
	code string
}

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
	return fmt.Sprintf("Uptobox root '%s'", f.root)
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

// retryErrorCodes is a slice of error codes that we will retry
var retryErrorCodes = []int{
	429, // Too Many Requests.
	500, // Internal Server Error
	502, // Bad Gateway
	503, // Service Unavailable
	504, // Gateway Timeout
	509, // Bandwidth Limit Exceeded
}

// shouldRetry returns a boolean as to whether this resp and err
// deserve to be retried.  It returns the err as a convenience
func shouldRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	return fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// dirPath returns an escaped file path (f.root, file)
func (f *Fs) dirPath(file string) string {
	//return path.Join(f.diskRoot, file)
	if file == "" || file == "." {
		return "//" + f.root
	}
	return "//" + path.Join(f.root, file)
}

// returns the full path based on root and the last element
func (f *Fs) splitPathFull(pth string) (string, string) {
	fullPath := strings.Trim(path.Join(f.root, pth), "/")

	i := len(fullPath) - 1
	for i >= 0 && fullPath[i] != '/' {
		i--
	}

	if i < 0 {
		return "//" + fullPath[:i+1], fullPath[i+1:]
	}

	// do not include the / at the split
	return "//" + fullPath[:i], fullPath[i+1:]
}

// splitPath is modified splitPath version that doesn't include the seperator
// in the base part
func (f *Fs) splitPath(pth string) (string, string) {
	// chop of any leading or trailing '/'
	pth = strings.Trim(pth, "/")

	i := len(pth) - 1
	for i >= 0 && pth[i] != '/' {
		i--
	}

	if i < 0 {
		return pth[:i+1], pth[i+1:]
	}
	return pth[:i], pth[i+1:]
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
func NewFs(ctx context.Context, name string, root string, config configmap.Mapper) (fs.Fs, error) {
	opt := new(Options)
	err := configstruct.Set(config, opt)
	if err != nil {
		return nil, err
	}

	f := &Fs{
		name:  name,
		root:  root,
		opt:   *opt,
		pacer: fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant), pacer.AttackConstant(attackConstant))),
	}
	f.root = root
	f.features = (&fs.Features{
		DuplicateFiles:          true,
		CanHaveEmptyDirectories: true,
		ReadMimeType:            false,
	}).Fill(ctx, f)

	client := fshttp.NewClient(ctx)
	f.srv = rest.NewClient(client).SetRoot(apiBaseURL)
	f.IDRegexp = regexp.MustCompile("https://uptobox.com/([a-zA-Z0-9]+)")

	_, err = f.readMetaDataForPath(ctx, f.dirPath(""), &api.MetadataRequestOptions{Limit: 10})
	if err != nil {
		if _, ok := err.(api.Error); !ok {
			return nil, err
		}
		// assume it's a file than
		oldRoot := f.root
		rootDir, file := f.splitPath(root)
		f.root = rootDir
		_, err = f.NewObject(ctx, file)
		if err == nil {
			return f, fs.ErrorIsFile
		}
		f.root = oldRoot
	}

	return f, nil
}

func (f *Fs) decodeError(resp *http.Response, response interface{}) (err error) {
	defer fs.CheckClose(resp.Body, &err)

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	// try to unmarshal into correct structure
	err = json.Unmarshal(body, response)
	if err == nil {
		return nil
	}
	// try to unmarshal into Error
	var apiErr api.Error
	err = json.Unmarshal(body, &apiErr)
	if err != nil {
		return err
	}
	return apiErr
}

func (f *Fs) readMetaDataForPath(ctx context.Context, path string, options *api.MetadataRequestOptions) (*api.ReadMetadataResponse, error) {
	opts := rest.Opts{
		Method: "GET",
		Path:   "/user/files",
		Parameters: url.Values{
			"token": []string{f.opt.AccessToken},
			"path":  []string{f.opt.Enc.FromStandardPath(path)},
			"limit": []string{strconv.FormatUint(options.Limit, 10)},
		},
	}

	if options.Offset != 0 {
		opts.Parameters.Set("offset", strconv.FormatUint(options.Offset, 10))
	}

	var err error
	var info api.ReadMetadataResponse
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.Call(ctx, &opts)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}

	err = f.decodeError(resp, &info)
	if err != nil {
		return nil, err
	}

	if info.StatusCode != 0 {
		return nil, errors.New(info.Message)
	}

	return &info, nil
}

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
	root := f.dirPath(dir)

	var limit uint64 = 100 // max number of objects per request - 100 seems to be the maximum the api accepts
	var page uint64 = 1
	var offset uint64 = 0 // for the next page of requests

	for {
		opts := &api.MetadataRequestOptions{
			Limit:  limit,
			Offset: offset,
		}

		info, err := f.readMetaDataForPath(ctx, root, opts)
		if err != nil {
			if apiErr, ok := err.(api.Error); ok {
				// might indicate other errors but we can probably assume not found here
				if apiErr.StatusCode == 1 {
					return nil, fs.ErrorDirNotFound
				}
			}
			return nil, err
		}

		for _, item := range info.Data.Files {
			remote := path.Join(dir, f.opt.Enc.ToStandardName(item.Name))
			o, err := f.newObjectWithInfo(ctx, remote, &item)
			if err != nil {
				continue
			}
			entries = append(entries, o)
		}

		// folders are always listed entirely on every page grr.
		if page == 1 {
			for _, item := range info.Data.Folders {
				remote := path.Join(dir, f.opt.Enc.ToStandardName(item.Name))
				d := fs.NewDir(remote, time.Time{}).SetID(strconv.FormatUint(item.FolderID, 10))
				entries = append(entries, d)
			}
		}

		//offset for the next page of items
		page++
		offset += limit
		//check if we reached end of list
		if page > uint64(info.Data.PageCount) {
			break
		}
	}
	return entries, nil
}

// Return an Object from a path
//
// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(ctx context.Context, remote string, info *api.FileInfo) (fs.Object, error) {
	o := &Object{
		fs:          f,
		remote:      remote,
		size:        info.Size,
		code:        info.Code,
		hasMetaData: true,
	}
	return o, nil
}

// NewObject finds the Object at remote.  If it can't be found it
// returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	// no way to directly access an object by path so we have to list the parent dir
	entries, err := f.List(ctx, path.Dir(remote))
	if err != nil {
		// need to change error type
		// if the parent dir doesn't exist the object doesn't exist either
		if err == fs.ErrorDirNotFound {
			return nil, fs.ErrorObjectNotFound
		}
		return nil, err
	}
	for _, entry := range entries {
		if o, ok := entry.(fs.Object); ok {
			if o.Remote() == remote {
				return o, nil
			}
		}
	}
	return nil, fs.ErrorObjectNotFound
}

func (f *Fs) uploadFile(ctx context.Context, in io.Reader, size int64, filename string, uploadURL string, options ...fs.OpenOption) (*api.UploadResponse, error) {
	opts := rest.Opts{
		Method:               "POST",
		RootURL:              "https:" + uploadURL,
		Body:                 in,
		ContentLength:        &size,
		Options:              options,
		MultipartContentName: "files",
		MultipartFileName:    filename,
	}

	var err error
	var resp *http.Response
	var ul api.UploadResponse
	err = f.pacer.CallNoRetry(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &ul)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, errors.Wrap(err, "couldn't upload file")
	}
	return &ul, nil
}

// dstPath starts from root and includes //
func (f *Fs) move(ctx context.Context, dstPath string, fileID string) (err error) {
	meta, err := f.readMetaDataForPath(ctx, dstPath, &api.MetadataRequestOptions{Limit: 10})
	if err != nil {
		return err
	}

	opts := rest.Opts{
		Method: "PATCH",
		Path:   "/user/files",
	}
	mv := api.CopyMoveFileRequest{
		Token:               f.opt.AccessToken,
		FileCodes:           fileID,
		DestinationFolderID: meta.Data.CurrentFolder.FolderID,
		Action:              "move",
	}

	var resp *http.Response
	var info api.UpdateResponse
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &mv, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return errors.Wrap(err, "couldn't move file")
	}
	if info.StatusCode != 0 {
		return errors.Errorf("move: api error: %d - %s", info.StatusCode, info.Message)
	}
	return err
}

// updateFileInformation set's various file attributes most importantly it's name
func (f *Fs) updateFileInformation(ctx context.Context, update *api.UpdateFileInformation) (err error) {
	opts := rest.Opts{
		Method: "PATCH",
		Path:   "/user/files",
	}

	var resp *http.Response
	var info api.UpdateResponse
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, update, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return errors.Wrap(err, "couldn't update file info")
	}
	if info.StatusCode != 0 {
		return errors.Errorf("updateFileInfo: api error: %d - %s", info.StatusCode, info.Message)
	}
	return err
}

func (f *Fs) putUnchecked(ctx context.Context, in io.Reader, remote string, size int64, options ...fs.OpenOption) (fs.Object, error) {
	if size > int64(200e9) { // max size 200GB
		return nil, errors.New("File too big, cant upload")
	} else if size == 0 {
		return nil, fs.ErrorCantUploadEmptyFiles
	}
	// yes it does take take 4 requests if we're uploading to root and 6+ if we're uploading to any subdir :(

	// create upload request
	opts := rest.Opts{
		Method: "GET",
		Path:   "/upload",
	}
	token := api.Token{
		Token: f.opt.AccessToken,
	}
	var info api.UploadInfo
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, &token, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	if info.StatusCode != 0 {
		return nil, errors.Errorf("putUnchecked: api error: %d - %s", info.StatusCode, info.Message)
	}
	// we need to have a safe name for the upload to work
	tmpName := "rcloneTemp" + random.String(8)
	upload, err := f.uploadFile(ctx, in, size, tmpName, info.Data.UploadLink, options...)
	if err != nil {
		return nil, err
	}
	if len(upload.Files) != 1 {
		return nil, errors.New("Upload: unexpected response")
	}
	match := f.IDRegexp.FindStringSubmatch(upload.Files[0].URL)

	// move file to destination folder
	base, leaf := f.splitPath(remote)
	fullBase := f.dirPath(base)

	if fullBase != "//" {
		// make all the parent folders
		err = f.Mkdir(ctx, base)
		if err != nil {
			// this might need some more error handling. if any of the following requests fail
			// we'll leave an orphaned temporary file floating around somewhere
			// they rarely fail though
			return nil, err
		}

		err = f.move(ctx, fullBase, match[1])
		if err != nil {
			return nil, err
		}
	}

	// rename file to final name
	err = f.updateFileInformation(ctx, &api.UpdateFileInformation{Token: f.opt.AccessToken, FileCode: match[1], NewName: f.opt.Enc.FromStandardName(leaf)})
	if err != nil {
		return nil, err
	}

	// finally fetch the file object.
	return f.NewObject(ctx, remote)
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
	existingObj, err := f.NewObject(ctx, src.Remote())
	switch err {
	case nil:
		return existingObj, existingObj.Update(ctx, in, src, options...)
	case fs.ErrorObjectNotFound:
		// Not found so create it
		return f.PutUnchecked(ctx, in, src, options...)
	default:
		return nil, err
	}
}

// PutUnchecked uploads the object
//
// This will create a duplicate if we upload a new file without
// checking to see if there is one already - use Put() for that.
func (f *Fs) PutUnchecked(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.putUnchecked(ctx, in, src.Remote(), src.Size(), options...)
}

// CreateDir dir creates a directory with the given parent path
// base starts from root and may or may not include //
func (f *Fs) CreateDir(ctx context.Context, base string, leaf string) (err error) {
	base = "//" + strings.Trim(base, "/")

	var resp *http.Response
	var apiErr api.Error
	opts := rest.Opts{
		Method: "PUT",
		Path:   "/user/files",
	}
	mkdir := api.CreateFolderRequest{
		Name:  f.opt.Enc.FromStandardName(leaf),
		Path:  f.opt.Enc.FromStandardPath(base),
		Token: f.opt.AccessToken,
	}
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &mkdir, &apiErr)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return err
	}
	// checking if the dir exists beforehand would be slower so we'll just ignore the error here
	if apiErr.StatusCode != 0 && !strings.Contains(apiErr.Data, "already exists") {
		return apiErr
	}
	return nil
}

func (f *Fs) mkDirs(ctx context.Context, path string) (err error) {
	// chop of any leading or trailing slashes
	dirs := strings.Split(path, "/")
	var base = ""
	for _, element := range dirs {
		// create every dir one by one
		if element != "" {
			err = f.CreateDir(ctx, base, element)
			if err != nil {
				return err
			}
			base += "/" + element
		}
	}
	return nil
}

// Mkdir makes the directory (container, bucket)
//
// Shouldn't return an error if it already exists
func (f *Fs) Mkdir(ctx context.Context, dir string) (err error) {
	if dir == "" || dir == "." {
		return f.mkDirs(ctx, f.root)
	}
	return f.mkDirs(ctx, path.Join(f.root, dir))
}

// may or may not delete folders with contents?
func (f *Fs) purge(ctx context.Context, folderID uint64) (err error) {
	var resp *http.Response
	var apiErr api.Error
	opts := rest.Opts{
		Method: "DELETE",
		Path:   "/user/files",
	}
	rm := api.DeleteFolderRequest{
		FolderID: folderID,
		Token:    f.opt.AccessToken,
	}
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &rm, &apiErr)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return err
	}
	if apiErr.StatusCode != 0 {
		return apiErr
	}
	return nil
}

// Rmdir removes the directory (container, bucket) if empty
//
// Return an error if it doesn't exist or isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	info, err := f.readMetaDataForPath(ctx, f.dirPath(dir), &api.MetadataRequestOptions{Limit: 10})
	if err != nil {
		return err
	}
	if info.Data.CurrentFolder.FileCount > 0 {
		return fs.ErrorDirectoryNotEmpty
	}

	return f.purge(ctx, info.Data.CurrentFolder.FolderID)
}

// Move src to this remote using server side move operations.
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't move - not same remote type")
		return nil, fs.ErrorCantMove
	}

	srcBase, srcLeaf := srcObj.fs.splitPathFull(src.Remote())
	dstBase, dstLeaf := f.splitPathFull(remote)

	needRename := srcLeaf != dstLeaf
	needMove := srcBase != dstBase

	// do the move if required
	if needMove {
		err := f.mkDirs(ctx, strings.Trim(dstBase, "/"))
		if err != nil {
			return nil, errors.Wrap(err, "move: failed to make destination dirs")
		}

		err = f.move(ctx, dstBase, srcObj.code)
		if err != nil {
			return nil, err
		}
	}

	// rename to final name if we need to
	if needRename {
		err := f.updateFileInformation(ctx, &api.UpdateFileInformation{Token: f.opt.AccessToken, FileCode: srcObj.code, NewName: f.opt.Enc.FromStandardName(dstLeaf)})
		if err != nil {
			return nil, errors.Wrap(err, "move: failed final rename")
		}
	}

	// copy the old object and apply the changes
	var newObj Object
	newObj = *srcObj
	newObj.remote = remote
	newObj.fs = f
	return &newObj, nil
}

// renameDir renames a directory
func (f *Fs) renameDir(ctx context.Context, folderID uint64, newName string) (err error) {
	var resp *http.Response
	var apiErr api.Error
	opts := rest.Opts{
		Method: "PATCH",
		Path:   "/user/files",
	}
	rename := api.RenameFolderRequest{
		Token:    f.opt.AccessToken,
		FolderID: folderID,
		NewName:  newName,
	}
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &rename, &apiErr)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return err
	}
	if apiErr.StatusCode != 0 {
		return apiErr
	}
	return nil
}

// DirMove moves src, srcRemote to this remote at dstRemote
// using server-side move operations.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantDirMove
//
// If destination exists then return fs.ErrorDirExists
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) error {
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debugf(srcFs, "Can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}

	// find out source
	srcPath := srcFs.dirPath(srcRemote)
	srcInfo, err := f.readMetaDataForPath(ctx, srcPath, &api.MetadataRequestOptions{Limit: 1})
	if err != nil {
		return errors.Wrap(err, "dirmove: source not found")
	}
	// check if the destination allready exists
	dstPath := f.dirPath(dstRemote)
	dstInfo, err := f.readMetaDataForPath(ctx, dstPath, &api.MetadataRequestOptions{Limit: 1})
	if err == nil {
		return fs.ErrorDirExists
	}

	// make the destination parent path
	dstBase, dstName := f.splitPathFull(dstRemote)
	err = f.mkDirs(ctx, strings.Trim(dstBase, "/"))
	if err != nil {
		return errors.Wrap(err, "dirmove: failed to create dirs")
	}

	// find the destination parent dir
	dstInfo, err = f.readMetaDataForPath(ctx, dstBase, &api.MetadataRequestOptions{Limit: 1})
	if err != nil {
		return errors.Wrap(err, "dirmove: failed to read destination")
	}
	srcBase, srcName := srcFs.splitPathFull(srcRemote)

	needRename := srcName != dstName
	needMove := srcBase != dstBase

	// if we have to rename we'll have to use a temporary name since
	// there could allready be a directory with the same name as the src directory
	if needRename {
		// rename to a temporary name
		tmpName := "rcloneTemp" + random.String(8)
		err = f.renameDir(ctx, srcInfo.Data.CurrentFolder.FolderID, tmpName)
		if err != nil {
			return errors.Wrap(err, "dirmove: failed initial rename")
		}
	}

	// do the move
	if needMove {
		opts := rest.Opts{
			Method: "PATCH",
			Path:   "/user/files",
		}
		move := api.MoveFolderRequest{
			Token:               f.opt.AccessToken,
			FolderID:            srcInfo.Data.CurrentFolder.FolderID,
			DestinationFolderID: dstInfo.Data.CurrentFolder.FolderID,
			Action:              "move",
		}
		var resp *http.Response
		var apiErr api.Error
		err = f.pacer.Call(func() (bool, error) {
			resp, err = f.srv.CallJSON(ctx, &opts, &move, &apiErr)
			return shouldRetry(ctx, resp, err)
		})
		if err != nil {
			return errors.Wrap(err, "dirmove: failed to move")
		}
		if apiErr.StatusCode != 0 {
			return apiErr
		}
	}

	// rename to final name
	if needRename {
		err = f.renameDir(ctx, srcInfo.Data.CurrentFolder.FolderID, dstName)
		if err != nil {
			return errors.Wrap(err, "dirmove: failed final rename")
		}
	}
	return nil
}

func (f *Fs) copy(ctx context.Context, dstPath string, fileID string) (err error) {
	meta, err := f.readMetaDataForPath(ctx, dstPath, &api.MetadataRequestOptions{Limit: 10})
	if err != nil {
		return err
	}

	opts := rest.Opts{
		Method: "PATCH",
		Path:   "/user/files",
	}
	cp := api.CopyMoveFileRequest{
		Token:               f.opt.AccessToken,
		FileCodes:           fileID,
		DestinationFolderID: meta.Data.CurrentFolder.FolderID,
		Action:              "copy",
	}

	var resp *http.Response
	var info api.UpdateResponse
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &cp, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return errors.Wrap(err, "couldn't copy file")
	}
	if info.StatusCode != 0 {
		return errors.Errorf("copy: api error: %d - %s", info.StatusCode, info.Message)
	}
	return err
}

// Copy src to this remote using server side move operations.
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantMove
	}

	_, srcLeaf := f.splitPath(src.Remote())
	dstBase, dstLeaf := f.splitPath(remote)

	needRename := srcLeaf != dstLeaf

	err := f.mkDirs(ctx, path.Join(f.root, dstBase))
	if err != nil {
		return nil, errors.Wrap(err, "copy: failed to make destination dirs")
	}

	err = f.copy(ctx, f.dirPath(dstBase), srcObj.code)
	if err != nil {
		return nil, err
	}

	newObj, err := f.NewObject(ctx, path.Join(dstBase, srcLeaf))
	if err != nil {
		return nil, errors.Wrap(err, "copy: couldn't find copied object")
	}

	if needRename {
		err := f.updateFileInformation(ctx, &api.UpdateFileInformation{Token: f.opt.AccessToken, FileCode: newObj.(*Object).code, NewName: f.opt.Enc.FromStandardName(dstLeaf)})
		if err != nil {
			return nil, errors.Wrap(err, "copy: failed final rename")
		}
		newObj.(*Object).remote = remote
	}

	return newObj, nil
}

// ------------------------------------------------------------

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

// Returns the full remote path for the object
func (o *Object) filePath() string {
	return o.fs.dirPath(o.remote)
}

// ModTime returns the modification time of the object
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *Object) ModTime(ctx context.Context) time.Time {
	return time.Now()
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	return o.size
}

// Hash returns the Md5sum of an object returning a lowercase hex string
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// ID returns the ID of the Object if known, or "" if not
func (o *Object) ID() string {
	return o.code
}

// Storable returns whether this object is storable
func (o *Object) Storable() bool {
	return true
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	return fs.ErrorCantSetModTime
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	opts := rest.Opts{
		Method: "GET",
		Path:   "/link",
		Parameters: url.Values{
			"token":     []string{o.fs.opt.AccessToken},
			"file_code": []string{o.code},
		},
	}
	var dl api.Download
	var resp *http.Response
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.CallJSON(ctx, &opts, nil, &dl)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, errors.Wrap(err, "open: failed to get download link")
	}

	fs.FixRangeOption(options, o.size)
	opts = rest.Opts{
		Method:  "GET",
		RootURL: dl.Data.DownloadLink,
		Options: options,
	}

	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.Call(ctx, &opts)
		return shouldRetry(ctx, resp, err)
	})

	if err != nil {
		return nil, err
	}
	return resp.Body, err
}

// Update the already existing object
//
// Copy the reader into the object updating modTime and size
//
// The new object may have been created if an error is returned
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	if src.Size() < 0 {
		return errors.New("refusing to update with unknown size")
	}

	// upload with new size but old name
	info, err := o.fs.putUnchecked(ctx, in, o.Remote(), src.Size(), options...)
	if err != nil {
		return err
	}

	// delete duplicate object after successful upload
	err = o.Remove(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to remove old version")
	}

	// Replace guts of old object with new one
	*o = *info.(*Object)

	return nil
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	opts := rest.Opts{
		Method: "DELETE",
		Path:   "/user/files",
	}
	delete := api.RemoveFileRequest{
		Token:     o.fs.opt.AccessToken,
		FileCodes: o.code,
	}
	var info api.UpdateResponse
	err := o.fs.pacer.Call(func() (bool, error) {
		resp, err := o.fs.srv.CallJSON(ctx, &opts, &delete, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return err
	}
	if info.StatusCode != 0 {
		return errors.Errorf("remove: api error: %d - %s", info.StatusCode, info.Message)
	}
	return nil
}

// Check the interfaces are satisfied
var (
	_ fs.Fs       = (*Fs)(nil)
	_ fs.Copier   = (*Fs)(nil)
	_ fs.Mover    = (*Fs)(nil)
	_ fs.DirMover = (*Fs)(nil)
	_ fs.Object   = (*Object)(nil)
)
