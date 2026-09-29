package webhdfs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/rclone/rclone/backend/webhdfs/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
)

const (
	minSleep      = 20 * time.Millisecond
	maxSleep      = 10 * time.Second
	decayConstant = 2 // bigger for slower decay, exponential
	apiPrefix     = "/webhdfs/v1"
)

// Fs represents a WebHDFS server
type Fs struct {
	name     string
	root     string
	features *fs.Features // optional features
	opt      Options      // options for this backend
	endpoint string       // base URL, e.g. http://namenode:9870
	srv      *rest.Client
	pacer    *fs.Pacer // pacer for API calls
}

// Object describes a WebHDFS file
type Object struct {
	fs      *Fs
	remote  string
	size    int64
	modTime time.Time
}

// retryErrorCodes is a slice of error codes that we will retry
var retryErrorCodes = []int{
	429, // Too Many Requests
	500, // Internal Server Error
	502, // Bad Gateway
	503, // Service Unavailable
	504, // Gateway Timeout
}

// shouldRetry returns a boolean as to whether this resp and err
// deserve to be retried. It returns the err as a convenience
func shouldRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	if apiErr, ok := err.(*api.Error); ok && apiErr.IsFileNotFound() {
		return false, err
	}
	return fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// errorHandler parses a non 2xx response into an *api.Error
func errorHandler(resp *http.Response) error {
	body, err := rest.ReadBody(resp)
	if err != nil {
		body = nil
	}
	return api.NewError(resp.StatusCode, body)
}

// encodePath escapes each path segment for use in a URL path, leaving
// the separating slashes intact
func encodePath(p string) string {
	segments := strings.Split(p, "/")
	for i, s := range segments {
		segments[i] = url.PathEscape(s)
	}
	return strings.Join(segments, "/")
}

// NewFs constructs an Fs from the path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	endpoint := strings.TrimRight(opt.URL, "/")
	if endpoint == "" {
		return nil, errors.New("webhdfs: url must be set")
	}

	if opt.Password != "" {
		opt.Password, err = obscure.Reveal(opt.Password)
		if err != nil {
			return nil, fmt.Errorf("couldn't decrypt password: %w", err)
		}
	}

	client := fshttp.NewClient(ctx)
	srv := rest.NewClient(client).SetRoot(endpoint + apiPrefix)
	srv.SetErrorHandler(errorHandler)
	if opt.Username != "" && opt.Password != "" {
		srv.SetUserPass(opt.Username, opt.Password)
	}

	f := &Fs{
		name:     name,
		root:     root,
		opt:      *opt,
		endpoint: endpoint,
		srv:      srv,
		pacer:    fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
	}

	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
	}).Fill(ctx, f)

	info, err := f.getFileStatus(ctx, f.realpath(""))
	if err == nil && !info.IsDir() {
		f.root = path.Dir(f.root)
		return f, fs.ErrorIsFile
	}

	return f, nil
}

// Name of this fs
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return f.root
}

// String returns a description of the FS
func (f *Fs) String() string {
	return fmt.Sprintf("webhdfs://%s/%s", f.endpoint, f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Precision return the precision of this Fs
func (f *Fs) Precision() time.Duration {
	return time.Millisecond
}

// Hashes are not supported
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.None)
}

// params returns the base query parameters for an operation, including
// the "user.name" authentication parameter if configured
func (f *Fs) params(op string, extra url.Values) url.Values {
	v := url.Values{}
	for k, vals := range extra {
		v[k] = vals
	}
	v.Set("op", op)
	if f.opt.Username != "" {
		v.Set("user.name", f.opt.Username)
	}
	return v
}

// realpath make correct file path with leading '/'
func (f *Fs) realpath(dir string) string {
	return f.opt.Enc.FromStandardPath(xPath(f.Root(), dir))
}

// getFileStatus reads the status of the file/directory at realpath, or
// returns fs.ErrorObjectNotFound if it doesn't exist
func (f *Fs) getFileStatus(ctx context.Context, realpath string) (*api.FileStatus, error) {
	var result api.FileStatusResponse
	var resp *http.Response
	err := f.pacer.Call(func() (bool, error) {
		opts := &rest.Opts{
			Method:     http.MethodGet,
			Path:       encodePath(realpath),
			Parameters: f.params("GETFILESTATUS", nil),
		}
		var err error
		resp, err = f.srv.CallJSON(ctx, opts, nil, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if apiErr, ok := err.(*api.Error); ok && apiErr.IsFileNotFound() {
			return nil, fs.ErrorObjectNotFound
		}
		return nil, err
	}
	return &result.FileStatus, nil
}

// listStatus lists the directory at realpath
func (f *Fs) listStatus(ctx context.Context, realpath string) ([]api.FileStatus, error) {
	var result api.FileStatusesResponse
	var resp *http.Response
	err := f.pacer.Call(func() (bool, error) {
		opts := &rest.Opts{
			Method:     http.MethodGet,
			Path:       encodePath(realpath),
			Parameters: f.params("LISTSTATUS", nil),
		}
		var err error
		resp, err = f.srv.CallJSON(ctx, opts, nil, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if apiErr, ok := err.(*api.Error); ok && apiErr.IsFileNotFound() {
			return nil, fs.ErrorDirNotFound
		}
		return nil, err
	}
	return result.FileStatuses.FileStatus, nil
}

// mkdirs creates a directory (and its parents) at realpath
func (f *Fs) mkdirs(ctx context.Context, realpath string) error {
	var result api.BooleanResponse
	return f.pacer.Call(func() (bool, error) {
		opts := &rest.Opts{
			Method:     http.MethodPut,
			Path:       encodePath(realpath),
			Parameters: f.params("MKDIRS", nil),
		}
		resp, err := f.srv.CallJSON(ctx, opts, nil, &result)
		return shouldRetry(ctx, resp, err)
	})
}

// remove deletes the file/directory at realpath
func (f *Fs) remove(ctx context.Context, realpath string, recursive bool) error {
	var result api.BooleanResponse
	err := f.pacer.Call(func() (bool, error) {
		opts := &rest.Opts{
			Method: http.MethodDelete,
			Path:   encodePath(realpath),
			Parameters: f.params("DELETE", url.Values{
				"recursive": []string{strconv.FormatBool(recursive)},
			}),
		}
		resp, err := f.srv.CallJSON(ctx, opts, nil, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return err
	}
	if !result.Boolean {
		return fmt.Errorf("webhdfs: failed to delete %q", realpath)
	}
	return nil
}

// rename renames/moves srcRealpath to dstRealpath
func (f *Fs) rename(ctx context.Context, srcRealpath, dstRealpath string) error {
	var result api.BooleanResponse
	err := f.pacer.Call(func() (bool, error) {
		opts := &rest.Opts{
			Method: http.MethodPut,
			Path:   encodePath(srcRealpath),
			Parameters: f.params("RENAME", url.Values{
				"destination": []string{dstRealpath},
			}),
		}
		resp, err := f.srv.CallJSON(ctx, opts, nil, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return err
	}
	if !result.Boolean {
		return fmt.Errorf("webhdfs: failed to rename %q to %q", srcRealpath, dstRealpath)
	}
	return nil
}

// setTimes sets the modification time of the file at realpath
func (f *Fs) setTimes(ctx context.Context, realpath string, modTime time.Time) error {
	return f.pacer.Call(func() (bool, error) {
		opts := &rest.Opts{
			Method: http.MethodPut,
			Path:   encodePath(realpath),
			Parameters: f.params("SETTIMES", url.Values{
				"modificationtime": []string{strconv.FormatInt(modTime.UnixMilli(), 10)},
			}),
			NoResponse: true,
		}
		resp, err := f.srv.Call(ctx, opts)
		return shouldRetry(ctx, resp, err)
	})
}

// create writes size bytes from in to the file at realpath using the
// two-step CREATE dance described by the WebHDFS REST API
func (f *Fs) create(ctx context.Context, realpath string, in io.Reader, size int64) error {
	var location string
	err := f.pacer.Call(func() (bool, error) {
		opts := &rest.Opts{
			Method: http.MethodPut,
			Path:   encodePath(realpath),
			Parameters: f.params("CREATE", url.Values{
				"overwrite": []string{"true"},
			}),
			NoRedirect:   true,
			IgnoreStatus: true,
		}
		resp, err := f.srv.Call(ctx, opts)
		if err != nil {
			return shouldRetry(ctx, resp, err)
		}
		if resp.StatusCode != http.StatusTemporaryRedirect {
			body, _ := rest.ReadBody(resp)
			err = api.NewError(resp.StatusCode, body)
			return shouldRetry(ctx, resp, err)
		}
		location = resp.Header.Get("Location")
		return false, resp.Body.Close()
	})
	if err != nil {
		return err
	}
	if location == "" {
		return errors.New("webhdfs: no Location header returned by namenode for CREATE")
	}

	opts := &rest.Opts{
		Method:     http.MethodPut,
		RootURL:    location,
		Body:       in,
		NoResponse: true,
	}
	if size >= 0 {
		opts.ContentLength = &size
	}
	_, err = f.srv.Call(ctx, opts)
	return err
}

// NewObject finds file at remote or returns fs.ErrorObjectNotFound
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	realpath := f.realpath(remote)
	info, err := f.getFileStatus(ctx, realpath)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fs.ErrorObjectNotFound
	}
	return &Object{
		fs:      f,
		remote:  remote,
		size:    info.Length,
		modTime: info.ModTime(),
	}, nil
}

// List the objects and directories in dir into entries
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	realpath := f.realpath(dir)

	info, err := f.getFileStatus(ctx, realpath)
	if err != nil {
		if err == fs.ErrorObjectNotFound {
			return nil, fs.ErrorDirNotFound
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, fs.ErrorDirNotFound
	}

	list, err := f.listStatus(ctx, realpath)
	if err != nil {
		return nil, err
	}
	for _, x := range list {
		stdName := f.opt.Enc.ToStandardName(x.PathSuffix)
		remote := path.Join(dir, stdName)
		if x.IsDir() {
			entries = append(entries, fs.NewDir(remote, x.ModTime()))
		} else {
			entries = append(entries, &Object{
				fs:      f,
				remote:  remote,
				size:    x.Length,
				modTime: x.ModTime(),
			})
		}
	}
	return entries, nil
}

// Put the object
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: src.Remote(),
	}
	err := o.Update(ctx, in, src, options...)
	return o, err
}

// Mkdir makes a directory
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	return f.mkdirs(ctx, f.realpath(dir))
}

// Rmdir deletes the directory if empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	realpath := f.realpath(dir)

	info, err := f.getFileStatus(ctx, realpath)
	if err != nil {
		if err == fs.ErrorObjectNotFound {
			return fs.ErrorDirNotFound
		}
		return err
	}
	if !info.IsDir() {
		return fs.ErrorDirNotFound
	}

	list, err := f.listStatus(ctx, realpath)
	if err != nil {
		return err
	}
	if len(list) > 0 {
		return fs.ErrorDirectoryNotEmpty
	}

	return f.remove(ctx, realpath, false)
}

// Purge deletes all the files and directories in dir
func (f *Fs) Purge(ctx context.Context, dir string) error {
	realpath := f.realpath(dir)

	info, err := f.getFileStatus(ctx, realpath)
	if err != nil {
		if err == fs.ErrorObjectNotFound {
			return fs.ErrorDirNotFound
		}
		return err
	}
	if !info.IsDir() {
		return fs.ErrorDirNotFound
	}

	return f.remove(ctx, realpath, true)
}

// Move src to this remote using server-side move operations.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantMove
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't move - not same remote type")
		return nil, fs.ErrorCantMove
	}

	srcRealpath := srcObj.fs.realpath(srcObj.remote)
	dstRealpath := f.realpath(remote)

	// Make sure the target folder exists
	if err := f.mkdirs(ctx, path.Dir(dstRealpath)); err != nil {
		return nil, err
	}

	if err := f.rename(ctx, srcRealpath, dstRealpath); err != nil {
		return nil, err
	}

	info, err := f.getFileStatus(ctx, dstRealpath)
	if err != nil {
		return nil, err
	}

	return &Object{
		fs:      f,
		remote:  remote,
		size:    info.Length,
		modTime: info.ModTime(),
	}, nil
}

// DirMove moves src, srcRemote to this remote at dstRemote using
// server-side move operations.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantDirMove
//
// If destination exists then return fs.ErrorDirExists
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) error {
	srcFs, ok := src.(*Fs)
	if !ok {
		return fs.ErrorCantDirMove
	}

	srcRealpath := srcFs.realpath(srcRemote)
	dstRealpath := f.realpath(dstRemote)

	if _, err := f.getFileStatus(ctx, dstRealpath); err == nil {
		return fs.ErrorDirExists
	} else if err != fs.ErrorObjectNotFound {
		return err
	}

	if err := f.mkdirs(ctx, path.Dir(dstRealpath)); err != nil {
		return err
	}

	return f.rename(ctx, srcRealpath, dstRealpath)
}

// Check the interfaces are satisfied
var (
	_ fs.Fs       = (*Fs)(nil)
	_ fs.Purger   = (*Fs)(nil)
	_ fs.Mover    = (*Fs)(nil)
	_ fs.DirMover = (*Fs)(nil)
)
