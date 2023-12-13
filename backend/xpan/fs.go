package xpan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rclone/rclone/backend/xpan/api"
	"github.com/rclone/rclone/backend/xpan/types"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/oauthutil"
	"github.com/rclone/rclone/lib/rest"
)

// Check the interfaces are satisfied
var (
	_ fs.Fs          = (*Fs)(nil)
	_ fs.PutStreamer = (*Fs)(nil)
	_ fs.Copier      = (*Fs)(nil)
	_ fs.Abouter     = (*Fs)(nil)
	_ fs.Mover       = (*Fs)(nil)
	_ fs.Shutdowner  = (*Fs)(nil)
)

// Fs xpan remote fs
type Fs struct {
	name         string
	root         string
	features     *fs.Features
	srv          *rateLimiterClient // the connection to the server
	pacer        *fs.Pacer          // pacer for API calls
	ts           *oauthutil.TokenSource
	tokenRenewer *oauthutil.Renew // renew the token on expiry
	opts         Options
}

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	if path.IsAbs(f.root) {
		return f.root
	}
	return fmt.Sprintf("/%s", f.root)
}

// String returns a description of the FS
func (f *Fs) String() string {
	return f.name + ":" + f.root
}

// Precision of the ModTimes in this Fs
func (f *Fs) Precision() time.Duration {
	return time.Second
}

// Hashes returns the supported hash types of the filesystem
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.None)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
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
	err = f.list(ctx, dir, func(item *api.Item) error {
		var e fs.DirEntry
		e, err = f.createDirEntry(ctx, item)
		if err != nil {
			return err
		}
		entries = append(entries, e)
		return nil
	})
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, api.ErrIllegalFilename) {
		return nil, fs.ErrorDirNotFound
	}
	return
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error ErrorObjectNotFound.
//
// If remote points to a directory then it should return
// ErrorIsDir if possible without doing any extra work,
// otherwise ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	return f.newObjectWithInfo(ctx, remote, nil)
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
	f.tokenRenewer.Start()
	defer f.tokenRenewer.Stop()
	var (
		item *api.Item
		err  error
	)
	if src.Size() > 0 && src.Size() <= int64(f.opts.ChunkSize) {
		item, err = f.singleUpload(ctx, in, src, options)
	} else {
		item, err = f.multipartUpload(ctx, in, src, options)
	}
	if err != nil {
		return nil, err
	}
	return f.newObjectWithInfo(ctx, src.Remote(), item)
}

// Mkdir makes the directory (container, bucket)
//
// Shouldn't return an error if it already exists
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	createReqBody := url.Values{
		"path":  {f.absolutePath(dir)},
		"isdir": {"1"},
	}
	params, err := f.newReqParams("create")
	if err != nil {
		return err
	}
	opts := rest.Opts{
		Method:      "POST",
		Path:        "/rest/2.0/xpan/file",
		Parameters:  params,
		ContentType: "application/x-www-form-urlencoded",
		Body:        strings.NewReader(createReqBody.Encode()),
	}
	var createResp api.MultipartCreateResponse
	err = f.pacer.Call(func() (bool, error) {
		_, err := f.srv.CallJSON(ctx, &opts, nil, &createResp)
		return false, err
	})
	if err != nil {
		return err
	}
	if createResp.ErrorNumber != 0 {
		return api.Err(createResp.ErrorNumber)
	}
	return nil
}

// Rmdir removes the directory (container, bucket) if empty
//
// Return an error if it doesn't exist or isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	if err := f.limitList(ctx, dir, 1, func(item *api.Item) error {
		return fs.ErrorDirectoryNotEmpty
	}); err != nil {
		return err
	}
	return f.filemanager(
		ctx, "delete", fmt.Sprintf("[\"%s\"]", f.absolutePath(dir)))
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.Put(ctx, in, src, options...)
}

// Copy src to this remote using server-side copy operations.
//
// # This is stored with the remote path given
//
// # It returns the destination Object and a possible error
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantCopy
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	newPath := f.absolutePath(remote)
	filelist, _ := json.Marshal([]api.CopyOrMove{{
		Path:    f.absolutePath(src.Remote()),
		Dest:    path.Dir(newPath),
		Newname: path.Base(newPath),
		Ondup:   "fail",
	}})
	err := f.filemanager(ctx, "copy", string(filelist))
	if err != nil {
		return nil, err
	}
	return f.newObjectWithInfo(ctx, remote, nil)
}

// About gets quota information from the Fs
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	params, err := f.newReqParams("")
	if err != nil {
		return nil, err
	}
	opts := rest.Opts{
		Method:     "GET",
		Path:       "/api/quota",
		Parameters: params,
	}
	var resp api.QuotaResponse
	err = f.pacer.Call(func() (bool, error) {
		_, err := f.srv.CallJSON(ctx, &opts, nil, &resp)
		return false, err
	})
	if err != nil {
		return nil, err
	}
	if resp.ErrorNumber != 0 {
		return nil, fmt.Errorf("quota error: %w", api.Err(resp.ErrorNumber))
	}
	return &fs.Usage{
		Total: &resp.Total,
		Used:  &resp.Used,
	}, nil
}

// Move src to this remote using server-side move operations.
//
// # This is stored with the remote path given
//
// # It returns the destination Object and a possible error
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantMove
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	newPath := f.absolutePath(remote)
	filelist, _ := json.Marshal([]api.CopyOrMove{{
		Path:    f.absolutePath(src.Remote()),
		Dest:    path.Dir(newPath),
		Newname: path.Base(newPath),
		Ondup:   "fail",
	}})
	err := f.filemanager(ctx, "move", string(filelist))
	if err != nil {
		return nil, err
	}
	return f.newObjectWithInfo(ctx, remote, nil)
}

// Shutdown the backend, closing any background tasks and any
// cached connections.
func (f *Fs) Shutdown(ctx context.Context) error {
	f.tokenRenewer.Shutdown()
	_ = os.RemoveAll(f.opts.TmpDir)
	return nil
}

func (f *Fs) filemanager(ctx context.Context, opera, filelist string) error {
	params, err := f.newReqParams("filemanager")
	if err != nil {
		return err
	}
	params.Set("opera", opera)

	reqBody := url.Values{}
	reqBody.Set("async", "0")
	reqBody.Set("filelist", filelist)
	opts := rest.Opts{
		Method:      "POST",
		Path:        "/rest/2.0/xpan/file",
		Parameters:  params,
		ContentType: "application/x-www-form-urlencoded",
		Body:        strings.NewReader(reqBody.Encode()),
	}
	var resp api.FileManageResponse
	err = f.pacer.Call(func() (bool, error) {
		_, err := f.srv.CallJSON(ctx, &opts, nil, &resp)
		return false, err
	})
	if err != nil {
		return err
	}
	if resp.ErrorNumber != 0 {
		return fmt.Errorf("filemanager-%s error: %w", opera, api.Err(resp.ErrorNumber))
	}
	return nil
}

func (f *Fs) newObjectWithInfo(ctx context.Context, remote string, item *api.Item) (*Object, error) {
	o := Object{
		fs:   f,
		item: item,
	}
	if o.item == nil {
		item, err := f.readFileMetaData(ctx, remote)
		if err != nil {
			return nil, err
		}
		if item.IsDir() {
			return nil, fs.ErrorIsDir
		}
		o.item = item
	}
	return &o, nil
}

func (f *Fs) createDirEntry(ctx context.Context, item *api.Item) (fs.DirEntry, error) {
	if item.IsDir() {
		modtime := time.Unix(int64(item.LocalModifyTime), 0)
		return fs.NewDir(f.removeLeadingRoot(item.Path), modtime), nil
	}
	return f.newObjectWithInfo(ctx, f.removeLeadingRoot(item.Path), item)
}

func (f *Fs) list(ctx context.Context, dir string, callback func(item *api.Item) error) (err error) {
	return f.limitList(ctx, dir, 1000, callback)
}

func (f *Fs) limitList(ctx context.Context, dir string, limit int, callback func(item *api.Item) error) (err error) {
	absolutePath := f.absolutePath(dir)
	params, err := f.newReqParams("list")
	if err != nil {
		return err
	}
	params.Set("dir", absolutePath)
	start := 0
	for {
		var listResponse api.ListFilesResponse
		params.Set("start", fmt.Sprintf("%d", start))
		params.Set("limit", fmt.Sprintf("%d", limit))
		opts := rest.Opts{
			Method:     "GET",
			Path:       "/rest/2.0/xpan/file",
			Parameters: params,
		}
		err = f.pacer.Call(func() (bool, error) {
			_, err := f.srv.CallJSON(ctx, &opts, nil, &listResponse)
			return false, err
		})
		if err != nil {
			return
		}
		if listResponse.ErrorNumber != 0 {
			err = api.Err(listResponse.ErrorNumber)
			return
		}
		for i := range listResponse.List {
			if err = callback(&listResponse.List[i]); err != nil {
				return
			}
		}
		if len(listResponse.List) < limit {
			break
		}
		start += limit
	}
	return
}

func (f *Fs) removeLeadingRoot(path string) string {
	cutPrefix := func(s, prefix string) string {
		if !strings.HasPrefix(s, prefix) {
			return s
		}
		return s[len(prefix):]
	}
	path = f.opts.Enc.ToStandardPath(path)
	if f.root != "" {
		path = cutPrefix(path, "/"+f.root)
		path = cutPrefix(path, "/")
	}
	return path
}

func (f *Fs) absolutePath(relativePath string) string {
	return path.Join("/", f.opts.Enc.FromStandardPath(f.root), f.opts.Enc.FromStandardPath(relativePath))
}

func (f *Fs) readFileMetaData(ctx context.Context, remote string) (*api.Item, error) {
	fileDir := path.Clean(path.Dir(remote))
	if fileDir == "." {
		fileDir = ""
	}
	remote = strings.ToLower(remote)
	var retItem *api.Item
	errFound := errors.New("found")
	err := f.list(ctx, fileDir, func(item *api.Item) error {
		itemFilePath := strings.ToLower(f.removeLeadingRoot(item.Path))
		if itemFilePath == remote {
			retItem = item
			return errFound // stop iterating
		}
		return nil
	})
	if err == errFound {
		return retItem, nil
	}
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, api.ErrIllegalFilename) {
		return nil, fs.ErrorObjectNotFound
	}
	if err != nil {
		return nil, err
	}
	return nil, fs.ErrorObjectNotFound
}

func (f *Fs) newReqParams(method string) (url.Values, error) {
	accessToken, err := f.ts.Token()
	if err != nil {
		return nil, err
	}
	params := url.Values{}
	params.Set("method", method)
	params.Set("access_token", accessToken.AccessToken)
	return params, nil
}

func (f *Fs) detectModTime(ctx context.Context, src fs.ObjectInfo) time.Time {
	mt := src.ModTime(ctx)
	if modTime := ctx.Value(types.ContextKeyModTime); modTime != nil {
		if mtint, ok := modTime.(int64); ok {
			mt = time.Unix(mtint, 0)
		}
	}
	return mt
}

func (f *Fs) singleUpload(ctx context.Context, in io.Reader, src fs.ObjectInfo, options []fs.OpenOption) (*api.Item, error) {
	absolutePath := f.absolutePath(src.Remote())
	params, err := f.newReqParams("upload")
	params.Set("path", absolutePath)
	params.Set("ondup", "overwrite")
	if err != nil {
		return nil, err
	}
	opts := rest.Opts{
		Method:     "POST",
		Path:       "/rest/2.0/pcs/file",
		RootURL:    "https://d.pcs.baidu.com",
		Options:    options,
		Parameters: params,
	}

	r, contentType, overhead, err := rest.MultipartUpload(
		ctx, in, url.Values{}, "file", filepath.Base(src.Remote()))
	if err != nil {
		return nil, err
	}
	contentLength := overhead + src.Size()
	opts.ContentType = contentType
	opts.Body = r
	opts.ContentLength = &contentLength

	var uploadResponse api.SingleUpoadResponse
	err = f.pacer.Call(func() (bool, error) {
		_, err := f.srv.CallJSON(ctx, &opts, nil, &uploadResponse)
		return false, err
	})
	if err != nil {
		return nil, err
	}
	if uploadResponse.ErrorNumber != 0 {
		return nil, api.Err(uploadResponse.ErrorNumber)
	}
	uploadResponse.Item.LocalCreateTime = uint(f.detectModTime(ctx, src).Unix())
	uploadResponse.Item.LocalModifyTime = uploadResponse.LocalCreateTime
	return &uploadResponse.Item, nil
}

func (f *Fs) multipartUpload(ctx context.Context, in io.Reader, src fs.ObjectInfo, options []fs.OpenOption) (*api.Item, error) {
	inWrapper := newChunkReader(in, int(f.opts.ChunkSize))
	tmpFilePath := filepath.Join(f.opts.TmpDir, uuid.NewString())
	tmpF, err := os.OpenFile(tmpFilePath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.Remove(tmpFilePath) }()
	fileSize, err := io.Copy(tmpF, inWrapper)
	if err != nil {
		return nil, err
	}
	if fileSize == 0 {
		return nil, errors.New("can not upload empty file")
	}
	err = tmpF.Close()
	if err != nil {
		return nil, err
	}
	if err != nil {
		return nil, err
	}

	// precreate
	precreateParams, err := f.newReqParams("precreate")
	if err != nil {
		return nil, err
	}
	reqBody := url.Values{}
	reqBody.Set("path", f.absolutePath(src.Remote()))
	reqBody.Set("size", fmt.Sprintf("%d", fileSize))
	reqBody.Set("isdir", "0")
	reqBody.Set("rtype", "3")
	reqBody.Set("block_list", api.ArrayValue(inWrapper._md5s, ""))
	reqBody.Set("autoinit", "1")
	opts := rest.Opts{
		Method:      "POST",
		Path:        "/rest/2.0/xpan/file",
		Options:     options,
		Parameters:  precreateParams,
		ContentType: "application/x-www-form-urlencoded",
		Body:        strings.NewReader(reqBody.Encode()),
	}
	var resp api.MultipartUploadResponse
	err = f.pacer.Call(func() (bool, error) {
		_, err := f.srv.CallJSON(ctx, &opts, nil, &resp)
		return false, err
	})
	if err != nil {
		return nil, err
	}
	if resp.ErrorNumber != 0 {
		return nil, fmt.Errorf("multipartUploadPrecreate: %w", api.Err(resp.ErrorNumber))
	}

	// upload parts
	uploadParams, err := f.newReqParams("upload")
	if err != nil {
		return nil, err
	}
	uploadParams.Set("type", "tmpfile")
	uploadParams.Set("path", reqBody.Get("path"))
	uploadParams.Set("uploadid", resp.UploadID)
	needUploadFile, err := os.Open(tmpFilePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = needUploadFile.Close() }()
	for partseq := 0; partseq < len(inWrapper._md5s); partseq++ {
		uploadParams.Set("partseq", fmt.Sprintf("%d", partseq))
		opts = rest.Opts{
			Method:     "POST",
			Path:       "/rest/2.0/pcs/superfile2",
			RootURL:    "https://d.pcs.baidu.com",
			Options:    options,
			Parameters: uploadParams,
		}

		r, contentType, overhead, err := rest.MultipartUpload(
			ctx, io.LimitReader(needUploadFile, int64(f.opts.ChunkSize)), url.Values{},
			"file", f.opts.Enc.FromStandardName(filepath.Base(src.Remote())))
		if err != nil {
			return nil, err
		}
		chunkSize := int64(f.opts.ChunkSize)
		if partseq == len(inWrapper._md5s)-1 {
			chunkSize = int64(inWrapper._chunkBytesCounter)
		}
		contentLength := overhead + chunkSize
		opts.ContentType = contentType
		opts.Body = r
		opts.ContentLength = &contentLength
		var resp api.UploadPartResponse
		if err = f.pacer.Call(func() (bool, error) {
			_, err := f.srv.CallJSON(ctx, &opts, nil, &resp)
			return false, err
		}); err != nil {
			return nil, err
		}
		if resp.ErrorNumber != 0 {
			return nil, fmt.Errorf("multipartUploadPart%d: %w", partseq, api.Err(resp.ErrorNumber))
		}
		fs.Debugf(f, "partseq %d, md5: %s", partseq, resp.Md5)
	}

	// create
	modTime := fmt.Sprintf("%d", f.detectModTime(ctx, src).Unix())
	createReqBody := url.Values{}
	createReqBody.Set("path", reqBody.Get("path"))
	createReqBody.Set("size", reqBody.Get("size"))
	createReqBody.Set("isdir", reqBody.Get("isdir"))
	createReqBody.Set("block_list", reqBody.Get("block_list"))
	createReqBody.Set("uploadid", resp.UploadID)
	createReqBody.Set("rtype", reqBody.Get("rtype"))
	createReqBody.Set("local_ctime", modTime)
	createReqBody.Set("local_mtime", modTime)

	createParams, err := f.newReqParams("create")
	if err != nil {
		return nil, err
	}

	opts = rest.Opts{
		Method:      "POST",
		Path:        "/rest/2.0/xpan/file",
		Options:     options,
		Parameters:  createParams,
		ContentType: "application/x-www-form-urlencoded",
		Body:        strings.NewReader(createReqBody.Encode()),
	}
	var createResp api.MultipartCreateResponse
	if err = f.pacer.Call(func() (bool, error) {
		_, err := f.srv.CallJSON(ctx, &opts, nil, &createResp)
		return false, err
	}); err != nil {
		return nil, err
	}
	if createResp.ErrorNumber != 0 {
		return nil, fmt.Errorf("multipartUploadCreate: %w", api.Err(createResp.ErrorNumber))
	}
	createResp.Item.LocalCreateTime = uint(f.detectModTime(ctx, src).Unix())
	createResp.Item.LocalModifyTime = createResp.LocalCreateTime
	return &createResp.Item, nil
}
