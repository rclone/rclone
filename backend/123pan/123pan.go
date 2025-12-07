// Package pan123 provides an interface to the 123Pan cloud storage system.
package pan123

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/backend/123pan/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
)

const (
	apiBaseURL = "https://open-api.123pan.com"
	minSleep   = 100 * time.Millisecond
	maxSleep   = 2 * time.Second
	rootID     = "0"
)

// rateLimiter implements per-endpoint QPS limiting using token bucket
type rateLimiter struct {
	tokens chan struct{}
	qps    int
}

// newRateLimiter creates a rate limiter with the specified QPS
func newRateLimiter(qps int) *rateLimiter {
	if qps <= 0 {
		return &rateLimiter{qps: 0}
	}
	rl := &rateLimiter{
		tokens: make(chan struct{}, qps),
		qps:    qps,
	}
	return rl
}

// acquire blocks until a token is available
func (rl *rateLimiter) acquire() {
	if rl.qps <= 0 {
		return
	}
	rl.tokens <- struct{}{}
}

// release returns a token after the rate limit period
func (rl *rateLimiter) release() {
	if rl.qps <= 0 {
		return
	}
	time.AfterFunc(time.Second, func() {
		<-rl.tokens
	})
}

// API rate limiters based on official 123Pan documentation
// QPS = queries per second per uid (free tier limits)
var (
	rlAccessToken    = newRateLimiter(8)  // api/v1/access_token: 8 QPS
	rlUserInfo       = newRateLimiter(10) // api/v1/user/info: 10 QPS
	rlFileListV2     = newRateLimiter(5)  // api/v2/file/list: 5 QPS
	rlDownloadInfo   = newRateLimiter(5)  // api/v1/file/download_info: 5 QPS
	rlMkdir          = newRateLimiter(5)  // upload/v1/file/mkdir: 5 QPS
	rlMove           = newRateLimiter(3)  // api/v1/file/move: 3 QPS
	rlTrash          = newRateLimiter(5)  // api/v1/file/trash: 5 QPS
	rlDelete         = newRateLimiter(1)  // api/v1/file/delete: 1 QPS
	rlUploadCreate   = newRateLimiter(5)  // upload/v1/file/create: 5 QPS
	rlUploadComplete = newRateLimiter(20) // upload/v1/file/upload_complete: 20 QPS
	rlShareCreate    = newRateLimiter(5)  // api/v1/share/create: 5 QPS (estimated)
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "123pan",
		Description: "123Pan (123云盘)",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:     "client_id",
			Help:     "OAuth Client ID from https://www.123pan.com/developer",
			Required: true,
		}, {
			Name:     "client_secret",
			Help:     "OAuth Client Secret",
			Required: true,
		}, {
			Name:      "access_token",
			Help:      "Access Token (optional, will be auto-refreshed if client_id and client_secret are provided)",
			Advanced:  true,
			Sensitive: true,
		}, {
			Name:    "upload_concurrency",
			Help:    "Concurrency for multipart uploads (1-32)",
			Default: 3,
		}, {
			Name:     "encoding",
			Help:     "Encoding for the backend",
			Advanced: true,
			// Matches OneDrive encoding for compatibility
			// 123Pan forbids: "\/:*?|><
			Default: (encoder.Display |
				encoder.EncodeBackSlash |
				encoder.EncodeLeftSpace |
				encoder.EncodeLeftTilde |
				encoder.EncodeRightPeriod |
				encoder.EncodeRightSpace |
				encoder.EncodeWin | // :?"*<>|
				encoder.EncodeInvalidUtf8),
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	ClientID          string               `config:"client_id"`
	ClientSecret      string               `config:"client_secret"`
	AccessToken       string               `config:"access_token"`
	UploadConcurrency int                  `config:"upload_concurrency"`
	Enc               encoder.MultiEncoder `config:"encoding"`
}

// Fs represents a remote 123pan server
type Fs struct {
	name     string             // name of this remote
	root     string             // the path we are working on
	opt      Options            // parsed options
	features *fs.Features       // optional features
	srv      *rest.Client       // the connection to the server
	dirCache *dircache.DirCache // Map of directory path to directory id
	pacer    *fs.Pacer          // pacer for API calls
	m        configmap.Mapper   // config mapper for saving tokens

	tokenMu      sync.Mutex // protects accessToken
	accessToken  string     // current access token
	tokenExpires time.Time  // when the token expires
}

// Object describes a 123pan object
type Object struct {
	fs          *Fs       // what this object is part of
	remote      string    // The remote path
	hasMetaData bool      // whether info below has been set
	size        int64     // size of the object
	modTime     time.Time // modification time of the object
	id          int64     // ID of the object
	etag        string    // MD5 hash (etag)
	parentID    int64     // parent directory ID
}

// ------------------------------------------------------------

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return f.root
}

// String converts this Fs to a string
func (f *Fs) String() string {
	return fmt.Sprintf("123pan root '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Precision return the precision of this Fs
func (f *Fs) Precision() time.Duration {
	return time.Second
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.MD5)
}

// parsePath parses a 123pan path
func parsePath(path string) (root string) {
	root = strings.Trim(path, "/")
	return
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
// deserve to be retried.  It returns the err as a convenience
func (f *Fs) shouldRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	return fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// getAccessToken gets a valid access token, refreshing if necessary
func (f *Fs) getAccessToken(ctx context.Context) (string, error) {
	f.tokenMu.Lock()
	defer f.tokenMu.Unlock()

	// Check if we have a valid token
	if f.accessToken != "" && time.Now().Before(f.tokenExpires) {
		return f.accessToken, nil
	}

	// Need to refresh the token
	if f.opt.ClientID == "" || f.opt.ClientSecret == "" {
		if f.accessToken != "" {
			return f.accessToken, nil
		}
		return "", errors.New("no access token and no client credentials to refresh")
	}

	// Get new token
	var result api.AccessTokenResponse
	opts := rest.Opts{
		Method: "POST",
		Path:   "/api/v1/access_token",
		ExtraHeaders: map[string]string{
			"Content-Type": "application/json",
			"Platform":     "open_platform",
		},
	}

	body := api.AccessTokenRequest{
		ClientID:     f.opt.ClientID,
		ClientSecret: f.opt.ClientSecret,
	}

	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, body, &result)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return "", fmt.Errorf("failed to get access token: %w", err)
	}
	if !result.OK() {
		return "", fmt.Errorf("failed to get access token: %s", result.Error())
	}

	f.accessToken = result.Data.AccessToken
	// Set token expiry to 1 hour from now (conservative estimate)
	f.tokenExpires = time.Now().Add(1 * time.Hour)

	// Save token to config
	if f.m != nil {
		f.m.Set("access_token", f.accessToken)
	}

	return f.accessToken, nil
}

// callAPI makes an API call with proper authentication and rate limiting
func (f *Fs) callAPI(ctx context.Context, opts *rest.Opts, request interface{}, response interface{}, rl *rateLimiter) error {
	token, err := f.getAccessToken(ctx)
	if err != nil {
		return err
	}

	if opts.ExtraHeaders == nil {
		opts.ExtraHeaders = make(map[string]string)
	}
	opts.ExtraHeaders["Authorization"] = "Bearer " + token
	opts.ExtraHeaders["Platform"] = "open_platform"
	if opts.ContentType == "" && request != nil {
		opts.ExtraHeaders["Content-Type"] = "application/json"
	}

	// Apply rate limiting
	if rl != nil {
		rl.acquire()
		defer rl.release()
	}

	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, opts, request, response)
		return f.shouldRetry(ctx, resp, err)
	})

	// Handle 401 by refreshing token and retrying
	if resp != nil && resp.StatusCode == 401 {
		f.tokenMu.Lock()
		f.accessToken = ""
		f.tokenExpires = time.Time{}
		f.tokenMu.Unlock()

		token, err = f.getAccessToken(ctx)
		if err != nil {
			return err
		}
		opts.ExtraHeaders["Authorization"] = "Bearer " + token

		err = f.pacer.Call(func() (bool, error) {
			resp, err = f.srv.CallJSON(ctx, opts, request, response)
			return f.shouldRetry(ctx, resp, err)
		})
	}

	return err
}

// NewFs constructs an Fs from the path, container:path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	if opt.UploadConcurrency < 1 || opt.UploadConcurrency > 32 {
		opt.UploadConcurrency = 3
	}

	root = parsePath(root)

	client := fshttp.NewClient(ctx)
	f := &Fs{
		name:        name,
		root:        root,
		opt:         *opt,
		srv:         rest.NewClient(client).SetRoot(apiBaseURL),
		pacer:       fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep))),
		m:           m,
		accessToken: opt.AccessToken,
	}

	f.features = (&fs.Features{
		CaseInsensitive:         false,
		CanHaveEmptyDirectories: true,
		DuplicateFiles:          false,
	}).Fill(ctx, f)

	// Get rootID
	f.dirCache = dircache.New(root, rootID, f)

	// Find the current root
	err = f.dirCache.FindRoot(ctx, false)
	if err != nil {
		// Assume it is a file
		newRoot, remote := dircache.SplitPath(root)
		tempF := *f
		tempF.dirCache = dircache.New(newRoot, rootID, &tempF)
		tempF.root = newRoot
		// Make new Fs which is the parent
		err = tempF.dirCache.FindRoot(ctx, false)
		if err != nil {
			// No root so return old f
			return f, nil
		}
		_, err := tempF.newObjectWithInfo(ctx, remote, nil)
		if err != nil {
			if err == fs.ErrorObjectNotFound {
				// File doesn't exist so return old f
				return f, nil
			}
			return nil, err
		}
		f.dirCache = tempF.dirCache
		f.root = tempF.root
		// return an error with an fs which points to the parent
		return f, fs.ErrorIsFile
	}
	return f, nil
}

// FindLeaf finds a directory of name leaf in the folder with ID pathID
func (f *Fs) FindLeaf(ctx context.Context, pathID, leaf string) (pathIDOut string, found bool, err error) {
	// Find the leaf in pathID
	parentID, err := strconv.ParseInt(pathID, 10, 64)
	if err != nil {
		return "", false, err
	}

	files, err := f.listAllFiles(ctx, parentID)
	if err != nil {
		return "", false, err
	}

	for _, file := range files {
		if file.IsDir() && f.opt.Enc.ToStandardName(file.FileName) == leaf {
			return strconv.FormatInt(file.FileID, 10), true, nil
		}
	}

	return "", false, nil
}

// CreateDir makes a directory with pathID as parent and name leaf
func (f *Fs) CreateDir(ctx context.Context, pathID, leaf string) (newID string, err error) {
	parentID, err := strconv.ParseInt(pathID, 10, 64)
	if err != nil {
		return "", err
	}

	var result api.MkdirResponse
	opts := rest.Opts{
		Method: "POST",
		Path:   "/upload/v1/file/mkdir",
	}

	body := api.MkdirRequest{
		ParentID: strconv.FormatInt(parentID, 10),
		Name:     f.opt.Enc.FromStandardName(leaf),
	}

	err = f.callAPI(ctx, &opts, body, &result, rlMkdir)
	if err != nil {
		return "", err
	}
	if !result.OK() {
		return "", fmt.Errorf("mkdir failed: %s", result.Error())
	}

	return strconv.FormatInt(result.Data.DirID, 10), nil
}

// listAllFiles lists all files in a directory
func (f *Fs) listAllFiles(ctx context.Context, parentID int64) ([]api.File, error) {
	var files []api.File
	lastFileID := int64(0)

	for {
		var result api.FileListResponse
		opts := rest.Opts{
			Method: "GET",
			Path:   "/api/v2/file/list",
			Parameters: url.Values{
				"parentFileId": {strconv.FormatInt(parentID, 10)},
				"limit":        {"100"},
				"lastFileId":   {strconv.FormatInt(lastFileID, 10)},
				"trashed":      {"false"},
			},
		}

		err := f.callAPI(ctx, &opts, nil, &result, rlFileListV2)
		if err != nil {
			return nil, err
		}
		if !result.OK() {
			return nil, fmt.Errorf("list files failed: %s", result.Error())
		}

		// Filter out trashed files (API parameter may not work)
		for _, file := range result.Data.FileList {
			if file.Trashed == 0 {
				files = append(files, file)
			}
		}

		if result.Data.LastFileID == -1 {
			break
		}
		lastFileID = result.Data.LastFileID
	}

	return files, nil
}

// List the objects and directories in dir into entries
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return nil, err
	}

	parentID, err := strconv.ParseInt(directoryID, 10, 64)
	if err != nil {
		return nil, err
	}

	files, err := f.listAllFiles(ctx, parentID)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		remote := path.Join(dir, f.opt.Enc.ToStandardName(file.FileName))
		if file.IsDir() {
			// Cache the directory ID for later lookups
			f.dirCache.Put(remote, strconv.FormatInt(file.FileID, 10))
			d := fs.NewDir(remote, file.ModTime()).SetID(strconv.FormatInt(file.FileID, 10))
			entries = append(entries, d)
		} else {
			o := &Object{
				fs:          f,
				remote:      remote,
				hasMetaData: true,
				size:        file.Size,
				modTime:     file.ModTime(),
				id:          file.FileID,
				etag:        file.Etag,
				parentID:    file.ParentFileID,
			}
			entries = append(entries, o)
		}
	}

	return entries, nil
}

// readMetaDataForPath reads the metadata from the path
func (f *Fs) readMetaDataForPath(ctx context.Context, path string) (info *api.File, err error) {
	leaf, directoryID, err := f.dirCache.FindPath(ctx, path, false)
	if err != nil {
		if err == fs.ErrorDirNotFound {
			return nil, fs.ErrorObjectNotFound
		}
		return nil, err
	}

	parentID, err := strconv.ParseInt(directoryID, 10, 64)
	if err != nil {
		return nil, err
	}

	files, err := f.listAllFiles(ctx, parentID)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if !file.IsDir() && f.opt.Enc.ToStandardName(file.FileName) == leaf {
			return &file, nil
		}
	}

	return nil, fs.ErrorObjectNotFound
}

// Return an Object from a path
func (f *Fs) newObjectWithInfo(ctx context.Context, remote string, info *api.File) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	var err error
	if info != nil {
		o.setMetaData(info)
	} else {
		err = o.readMetaData(ctx)
	}
	if err != nil {
		return nil, err
	}
	return o, nil
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	return f.newObjectWithInfo(ctx, remote, nil)
}

// Put the object into the container
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	remote := src.Remote()
	size := src.Size()

	// Create the directory for the object if it doesn't exist
	leaf, directoryID, err := f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return nil, err
	}

	parentID, err := strconv.ParseInt(directoryID, 10, 64)
	if err != nil {
		return nil, err
	}

	o := &Object{
		fs:     f,
		remote: remote,
	}

	return o, o.upload(ctx, in, leaf, size, parentID, options...)
}

// Mkdir creates the container if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	_, err := f.dirCache.FindDir(ctx, dir, true)
	return err
}

// Rmdir deletes the root folder
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return err
	}

	id, err := strconv.ParseInt(directoryID, 10, 64)
	if err != nil {
		return err
	}

	var result api.BaseResponse
	opts := rest.Opts{
		Method: "POST",
		Path:   "/api/v1/file/trash",
	}

	body := api.TrashRequest{
		FileIDs: []int64{id},
	}

	err = f.callAPI(ctx, &opts, body, &result, rlTrash)
	if err != nil {
		return err
	}
	if !result.OK() {
		return fmt.Errorf("rmdir failed: %s", result.Error())
	}

	f.dirCache.FlushDir(dir)
	return nil
}

// Move src to this remote using server-side move operations.
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't move - not same remote type")
		return nil, fs.ErrorCantMove
	}

	// Create temporary object
	leaf, directoryID, err := f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return nil, err
	}

	toParentID, err := strconv.ParseInt(directoryID, 10, 64)
	if err != nil {
		return nil, err
	}

	srcLeaf := path.Base(srcObj.remote)

	// If names are different, need to rename
	needRename := f.opt.Enc.FromStandardName(leaf) != f.opt.Enc.FromStandardName(srcLeaf)
	needMove := toParentID != srcObj.parentID

	if needMove {
		// Move the file
		var result api.BaseResponse
		opts := rest.Opts{
			Method: "POST",
			Path:   "/api/v1/file/move",
		}

		body := api.MoveRequest{
			FileIDs:        []int64{srcObj.id},
			ToParentFileID: toParentID,
		}

		err = f.callAPI(ctx, &opts, body, &result, rlMove)
		if err != nil {
			return nil, err
		}
		if !result.OK() {
			return nil, fmt.Errorf("move failed: %s", result.Error())
		}
	}

	if needRename {
		// Rename the file
		var result api.BaseResponse
		opts := rest.Opts{
			Method: "PUT",
			Path:   "/api/v1/file/name",
		}

		body := api.RenameRequest{
			FileID:   srcObj.id,
			FileName: f.opt.Enc.FromStandardName(leaf),
		}

		err = f.callAPI(ctx, &opts, body, &result, rlMove) // rename uses same rate as move
		if err != nil {
			return nil, err
		}
		if !result.OK() {
			return nil, fmt.Errorf("rename failed: %s", result.Error())
		}
	}

	// Create new object
	dstObj := &Object{
		fs:          f,
		remote:      remote,
		hasMetaData: true,
		size:        srcObj.size,
		modTime:     srcObj.modTime,
		id:          srcObj.id,
		etag:        srcObj.etag,
		parentID:    toParentID,
	}

	return dstObj, nil
}

// DirMove moves src, srcRemote to this remote at dstRemote
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) error {
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debugf(srcFs, "Can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}

	srcID, _, _, dstDirectoryID, dstLeaf, err := f.dirCache.DirMove(ctx, srcFs.dirCache, srcFs.root, srcRemote, f.root, dstRemote)
	if err != nil {
		return err
	}

	srcIDInt, err := strconv.ParseInt(srcID, 10, 64)
	if err != nil {
		return err
	}

	dstParentID, err := strconv.ParseInt(dstDirectoryID, 10, 64)
	if err != nil {
		return err
	}

	// Move the directory
	var result api.BaseResponse
	opts := rest.Opts{
		Method: "POST",
		Path:   "/api/v1/file/move",
	}

	body := api.MoveRequest{
		FileIDs:        []int64{srcIDInt},
		ToParentFileID: dstParentID,
	}

	err = f.callAPI(ctx, &opts, body, &result, rlMove)
	if err != nil {
		return err
	}
	if !result.OK() {
		return fmt.Errorf("dir move failed: %s", result.Error())
	}

	// Rename if needed
	srcLeaf := path.Base(srcRemote)
	if srcRemote == "" {
		srcLeaf = path.Base(srcFs.root)
	}
	if f.opt.Enc.FromStandardName(dstLeaf) != f.opt.Enc.FromStandardName(srcLeaf) {
		var renameResult api.BaseResponse
		renameOpts := rest.Opts{
			Method: "PUT",
			Path:   "/api/v1/file/name",
		}

		renameBody := api.RenameRequest{
			FileID:   srcIDInt,
			FileName: f.opt.Enc.FromStandardName(dstLeaf),
		}

		err = f.callAPI(ctx, &renameOpts, renameBody, &renameResult, rlMove)
		if err != nil {
			return err
		}
		if !renameResult.OK() {
			return fmt.Errorf("dir rename failed: %s", renameResult.Error())
		}
	}

	srcFs.dirCache.FlushDir(srcRemote)
	return nil
}

// Copy src to this remote using server-side copy operations.
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}

	// Create the directory for the object if it doesn't exist
	leaf, directoryID, err := f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return nil, err
	}

	parentID, err := strconv.ParseInt(directoryID, 10, 64)
	if err != nil {
		return nil, err
	}

	// Try to use instant upload (reuse) to copy
	var result api.UploadCreateResponse
	opts := rest.Opts{
		Method: "POST",
		Path:   "/upload/v2/file/create",
	}

	body := api.UploadCreateRequest{
		ParentFileID: parentID,
		Filename:     f.opt.Enc.FromStandardName(leaf),
		Etag:         strings.ToLower(srcObj.etag),
		Size:         srcObj.size,
		Duplicate:    2,
		ContainDir:   false,
	}

	err = f.callAPI(ctx, &opts, body, &result, rlUploadCreate)
	if err != nil {
		return nil, err
	}
	if !result.OK() {
		return nil, fmt.Errorf("copy failed: %s", result.Error())
	}

	// Check if instant upload (reuse) succeeded
	if !result.Data.Reuse {
		return nil, fs.ErrorCantCopy
	}

	// Create new object
	dstObj := &Object{
		fs:          f,
		remote:      remote,
		hasMetaData: true,
		size:        srcObj.size,
		modTime:     srcObj.modTime,
		id:          result.Data.FileID,
		etag:        srcObj.etag,
		parentID:    parentID,
	}

	return dstObj, nil
}

// About gets quota information
func (f *Fs) About(ctx context.Context) (usage *fs.Usage, err error) {
	var result api.UserInfoResponse
	opts := rest.Opts{
		Method: "GET",
		Path:   "/api/v1/user/info",
	}

	err = f.callAPI(ctx, &opts, nil, &result, rlUserInfo)
	if err != nil {
		return nil, err
	}
	if !result.OK() {
		return nil, fmt.Errorf("about failed: %s", result.Error())
	}

	total := result.Data.SpacePermanent + result.Data.SpaceTemp
	free := int64(total) - int64(result.Data.SpaceUsed)
	if free < 0 {
		free = 0
	}

	usage = &fs.Usage{
		Total: fs.NewUsageValue(int64(total)),
		Used:  fs.NewUsageValue(int64(result.Data.SpaceUsed)),
		Free:  fs.NewUsageValue(free),
	}
	return usage, nil
}

// DirCacheFlush resets the directory cache
func (f *Fs) DirCacheFlush() {
	f.dirCache.ResetRoot()
}

// Purge deletes all the files and directories in the path
func (f *Fs) Purge(ctx context.Context, dir string) error {
	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return err
	}

	id, err := strconv.ParseInt(directoryID, 10, 64)
	if err != nil {
		return err
	}

	// Trashing a directory also trashes all its contents
	var result api.BaseResponse
	opts := rest.Opts{
		Method: "POST",
		Path:   "/api/v1/file/trash",
	}

	body := api.TrashRequest{
		FileIDs: []int64{id},
	}

	err = f.callAPI(ctx, &opts, body, &result, rlTrash)
	if err != nil {
		return err
	}
	if !result.OK() {
		return fmt.Errorf("purge failed: %s", result.Error())
	}

	f.dirCache.FlushDir(dir)
	return nil
}

// PublicLink generates a public link to the remote path (usually readable by anyone)
func (f *Fs) PublicLink(ctx context.Context, remote string, expire fs.Duration, unlink bool) (string, error) {
	// Get the file/directory info
	o, err := f.NewObject(ctx, remote)
	if err != nil {
		// Try as a directory
		directoryID, dirErr := f.dirCache.FindDir(ctx, remote, false)
		if dirErr != nil {
			return "", err
		}

		id, parseErr := strconv.ParseInt(directoryID, 10, 64)
		if parseErr != nil {
			return "", parseErr
		}

		return f.createShareLink(ctx, id, remote)
	}

	obj := o.(*Object)
	return f.createShareLink(ctx, obj.id, remote)
}

// createShareLink creates a share link for the given file ID
func (f *Fs) createShareLink(ctx context.Context, fileID int64, name string) (string, error) {
	var result api.ShareCreateResponse
	opts := rest.Opts{
		Method: "POST",
		Path:   "/api/v1/share/create",
	}

	body := api.ShareCreateRequest{
		ShareName:   path.Base(name),
		ShareExpire: 0, // permanent
		FileIDList:  strconv.FormatInt(fileID, 10),
	}

	err := f.callAPI(ctx, &opts, body, &result, rlShareCreate)
	if err != nil {
		return "", err
	}
	if !result.OK() {
		return "", fmt.Errorf("create share link failed: %s", result.Error())
	}

	return "https://www.123pan.com/s/" + result.Data.ShareKey, nil
}

// listTrashedFiles lists all files in trash
func (f *Fs) listTrashedFiles(ctx context.Context) ([]api.File, error) {
	var trashedFiles []api.File
	lastFileID := int64(0)

	for {
		var result api.FileListResponse
		opts := rest.Opts{
			Method: "GET",
			Path:   "/api/v2/file/list",
			Parameters: url.Values{
				"parentFileId": {"0"},
				"limit":        {"100"},
				"lastFileId":   {strconv.FormatInt(lastFileID, 10)},
			},
		}

		err := f.callAPI(ctx, &opts, nil, &result, rlFileListV2)
		if err != nil {
			return nil, err
		}
		if !result.OK() {
			return nil, fmt.Errorf("list trashed files failed: %s", result.Error())
		}

		// Filter for trashed files only
		for _, file := range result.Data.FileList {
			if file.Trashed == 1 {
				trashedFiles = append(trashedFiles, file)
			}
		}

		if result.Data.LastFileID == -1 {
			break
		}
		lastFileID = result.Data.LastFileID
	}

	return trashedFiles, nil
}

// CleanUp empties the trash
func (f *Fs) CleanUp(ctx context.Context) error {
	trashedFiles, err := f.listTrashedFiles(ctx)
	if err != nil {
		return err
	}

	if len(trashedFiles) == 0 {
		return nil
	}

	// Delete files in batches of 100 (API limit)
	for i := 0; i < len(trashedFiles); i += 100 {
		end := i + 100
		if end > len(trashedFiles) {
			end = len(trashedFiles)
		}

		batch := trashedFiles[i:end]
		fileIDs := make([]int64, len(batch))
		for j, file := range batch {
			fileIDs[j] = file.FileID
		}

		var result api.BaseResponse
		opts := rest.Opts{
			Method: "POST",
			Path:   "/api/v1/file/delete",
		}

		body := api.DeleteRequest{
			FileIDs: fileIDs,
		}

		err := f.callAPI(ctx, &opts, body, &result, rlDelete)
		if err != nil {
			return err
		}
		if !result.OK() {
			return fmt.Errorf("cleanup failed: %s", result.Error())
		}
	}

	return nil
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

// Hash returns the MD5 of an object returning a lowercase hex string
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	if t != hash.MD5 {
		return "", hash.ErrUnsupported
	}
	return strings.ToLower(o.etag), nil
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	return o.size
}

// setMetaData sets the metadata from info
func (o *Object) setMetaData(info *api.File) {
	o.hasMetaData = true
	o.size = info.Size
	o.modTime = info.ModTime()
	o.id = info.FileID
	o.etag = info.Etag
	o.parentID = info.ParentFileID
}

// readMetaData gets the metadata if it hasn't already been fetched
func (o *Object) readMetaData(ctx context.Context) (err error) {
	if o.hasMetaData {
		return nil
	}
	info, err := o.fs.readMetaDataForPath(ctx, o.remote)
	if err != nil {
		return err
	}
	o.setMetaData(info)
	return nil
}

// ModTime returns the modification time of the object
func (o *Object) ModTime(ctx context.Context) time.Time {
	err := o.readMetaData(ctx)
	if err != nil {
		fs.Logf(o, "Failed to read metadata: %v", err)
		return time.Now()
	}
	return o.modTime
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	// 123Pan API doesn't support setting modification time
	return fs.ErrorCantSetModTime
}

// Storable returns a boolean showing whether this object storable
func (o *Object) Storable() bool {
	return true
}

// getDownloadURL gets the download URL for the object
func (o *Object) getDownloadURL(ctx context.Context) (string, error) {
	var result api.DownloadInfoResponse
	opts := rest.Opts{
		Method: "GET",
		Path:   "/api/v1/file/download_info",
		Parameters: url.Values{
			"fileId": {strconv.FormatInt(o.id, 10)},
		},
	}

	err := o.fs.callAPI(ctx, &opts, nil, &result, rlDownloadInfo)
	if err != nil {
		return "", err
	}
	if !result.OK() {
		return "", fmt.Errorf("get download info failed: %s", result.Error())
	}

	return result.Data.DownloadURL, nil
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	downloadURL, err := o.getDownloadURL(ctx)
	if err != nil {
		return nil, err
	}

	var resp *http.Response
	opts := rest.Opts{
		Method:  "GET",
		RootURL: downloadURL,
		Options: options,
	}
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.Call(ctx, &opts)
		return o.fs.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// upload uploads the object
func (o *Object) upload(ctx context.Context, in io.Reader, leaf string, size int64, parentID int64, options ...fs.OpenOption) error {
	// Calculate MD5 hash of the file
	var data []byte
	var err error
	var etag string

	if size > 0 {
		data, err = io.ReadAll(in)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}
		h := md5.Sum(data)
		etag = hex.EncodeToString(h[:])
	} else {
		etag = "d41d8cd98f00b204e9800998ecf8427e" // MD5 of empty string
	}

	// Step 1: Create file
	var createResult api.UploadCreateResponse
	createOpts := rest.Opts{
		Method: "POST",
		Path:   "/upload/v2/file/create",
	}

	createBody := api.UploadCreateRequest{
		ParentFileID: parentID,
		Filename:     o.fs.opt.Enc.FromStandardName(leaf),
		Etag:         strings.ToLower(etag),
		Size:         size,
		Duplicate:    2, // Overwrite existing
		ContainDir:   false,
	}

	err = o.fs.callAPI(ctx, &createOpts, createBody, &createResult, rlUploadCreate)
	if err != nil {
		return err
	}
	if !createResult.OK() {
		return fmt.Errorf("create file failed: %s", createResult.Error())
	}

	// Check if instant upload (reuse) succeeded
	if createResult.Data.Reuse {
		o.hasMetaData = true
		o.size = size
		o.modTime = time.Now()
		o.id = createResult.Data.FileID
		o.etag = etag
		o.parentID = parentID
		return nil
	}

	// Step 2: Upload slices
	if len(createResult.Data.Servers) == 0 {
		return errors.New("no upload server returned")
	}
	uploadDomain := createResult.Data.Servers[0]
	sliceSize := createResult.Data.SliceSize
	preuploadID := createResult.Data.PreuploadID

	if size > 0 {
		reader := bytes.NewReader(data)
		numSlices := (size + sliceSize - 1) / sliceSize

		for i := int64(0); i < numSlices; i++ {
			sliceStart := i * sliceSize
			sliceEnd := sliceStart + sliceSize
			if sliceEnd > size {
				sliceEnd = size
			}
			sliceData := data[sliceStart:sliceEnd]
			sliceNum := i + 1

			// Calculate slice MD5
			sliceHash := md5.Sum(sliceData)
			sliceMD5 := hex.EncodeToString(sliceHash[:])

			// Upload slice using multipart form
			var b bytes.Buffer
			w := multipart.NewWriter(&b)

			_ = w.WriteField("preuploadID", preuploadID)
			_ = w.WriteField("sliceNo", strconv.FormatInt(sliceNum, 10))
			_ = w.WriteField("sliceMD5", sliceMD5)

			fw, err := w.CreateFormFile("slice", fmt.Sprintf("%s.part%d", leaf, sliceNum))
			if err != nil {
				return err
			}
			_, err = io.Copy(fw, bytes.NewReader(sliceData))
			if err != nil {
				return err
			}
			w.Close()

			token, err := o.fs.getAccessToken(ctx)
			if err != nil {
				return err
			}

			req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadDomain+"/upload/v2/file/slice", &b)
			if err != nil {
				return err
			}
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", w.FormDataContentType())
			req.Header.Set("Platform", "open_platform")

			client := fshttp.NewClient(ctx)
			resp, err := client.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("slice upload failed: status %d, body: %s", resp.StatusCode, string(body))
			}

			var sliceResult api.BaseResponse
			err = json.NewDecoder(resp.Body).Decode(&sliceResult)
			if err != nil {
				return err
			}
			if !sliceResult.OK() {
				return fmt.Errorf("slice upload failed: %s", sliceResult.Error())
			}
		}

		// Reset reader for potential reuse
		reader.Seek(0, io.SeekStart)
	}

	// Step 3: Complete upload
	for i := 0; i < 60; i++ {
		var completeResult api.UploadCompleteResponse
		completeOpts := rest.Opts{
			Method: "POST",
			Path:   "/upload/v2/file/upload_complete",
		}

		completeBody := api.UploadCompleteRequest{
			PreuploadID: preuploadID,
		}

		err = o.fs.callAPI(ctx, &completeOpts, completeBody, &completeResult, rlUploadComplete)
		if err == nil && completeResult.Data.Completed && completeResult.Data.FileID != 0 {
			o.hasMetaData = true
			o.size = size
			o.modTime = time.Now()
			o.id = completeResult.Data.FileID
			o.etag = etag
			o.parentID = parentID
			return nil
		}

		// Poll every second
		time.Sleep(time.Second)
	}

	return errors.New("upload complete timeout")
}

// Update the object with the contents of the io.Reader, modTime and size
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	size := src.Size()
	remote := o.Remote()

	// Create the directory for the object if it doesn't exist
	leaf, directoryID, err := o.fs.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return err
	}

	parentID, err := strconv.ParseInt(directoryID, 10, 64)
	if err != nil {
		return err
	}

	return o.upload(ctx, in, leaf, size, parentID, options...)
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	var result api.BaseResponse
	opts := rest.Opts{
		Method: "POST",
		Path:   "/api/v1/file/trash",
	}

	body := api.TrashRequest{
		FileIDs: []int64{o.id},
	}

	err := o.fs.callAPI(ctx, &opts, body, &result, rlTrash)
	if err != nil {
		return err
	}
	if !result.OK() {
		return fmt.Errorf("remove failed: %s", result.Error())
	}

	return nil
}

// ID returns the ID of the Object if known, or "" if not
func (o *Object) ID() string {
	return strconv.FormatInt(o.id, 10)
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ fs.Copier          = (*Fs)(nil)
	_ fs.Abouter         = (*Fs)(nil)
	_ fs.DirCacheFlusher = (*Fs)(nil)
	_ fs.Purger          = (*Fs)(nil)
	_ fs.PublicLinker    = (*Fs)(nil)
	_ fs.CleanUpper      = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.IDer            = (*Object)(nil)
)
