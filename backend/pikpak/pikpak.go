// Package pikpak provides an interface to the PikPak
package pikpak

// ------------------------------------------------------------
// NOTE
// ------------------------------------------------------------

// md5sum is not always available, sometimes given empty.

// Trashed files are not restored to the original location when using `batchUntrash`

// Can't stream without `--vfs-cache-mode=full`

// ------------------------------------------------------------
// TODO
// ------------------------------------------------------------

// * List() with options starred-only
// * user-configurable list chunk
// * backend command: untrash, iscached
// * api(event,task)

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"path"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/rclone/rclone/backend/pikpak/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/chunksize"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/atexit"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/oauthutil"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/random"
	"github.com/rclone/rclone/lib/rest"
	"golang.org/x/oauth2"
)

// Constants
const (
	clientID                 = "YUMx5nI8ZU8Ap8pm"
	clientVersion            = "2.0.0"
	packageName              = "mypikpak.com"
	defaultUserAgent         = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:129.0) Gecko/20100101 Firefox/129.0"
	minSleep                 = 100 * time.Millisecond
	maxSleep                 = 2 * time.Second
	taskWaitTime             = 500 * time.Millisecond
	decayConstant            = 2 // bigger for slower decay, exponential
	rootURL                  = "https://api-drive.mypikpak.com"
	minChunkSize             = fs.SizeSuffix(manager.MinUploadPartSize)
	defaultUploadConcurrency = manager.DefaultUploadConcurrency
)

// Globals
var (
	// Description of how to auth for this app
	oauthConfig = &oauth2.Config{
		Scopes: nil,
		Endpoint: oauth2.Endpoint{
			AuthURL:   "https://user.mypikpak.com/v1/auth/signin",
			TokenURL:  "https://user.mypikpak.com/v1/auth/token",
			AuthStyle: oauth2.AuthStyleInParams,
		},
		ClientID:    clientID,
		RedirectURL: oauthutil.RedirectURL,
	}
)

// pikpakAutorize retrieves OAuth token using user/pass and save it to rclone.conf
func pikpakAuthorize(ctx context.Context, opt *Options, name string, m configmap.Mapper) error {
	if opt.Username == "" {
		return errors.New("no username")
	}
	pass, err := obscure.Reveal(opt.Password)
	if err != nil {
		return fmt.Errorf("failed to decode password - did you obscure it?: %w", err)
	}
	// new device id if necessary
	if len(opt.DeviceID) != 32 {
		opt.DeviceID = genDeviceID()
		m.Set("device_id", opt.DeviceID)
		fs.Infof(nil, "Using new device id %q", opt.DeviceID)
	}
	opts := rest.Opts{
		Method:  "POST",
		RootURL: "https://user.mypikpak.com/v1/auth/signin",
	}
	req := map[string]string{
		"username":  opt.Username,
		"password":  pass,
		"client_id": clientID,
	}
	var token api.Token
	rst := newPikpakClient(getClient(ctx, opt), opt).SetCaptchaTokener(ctx, m)
	_, err = rst.CallJSON(ctx, &opts, req, &token)
	if apiErr, ok := err.(*api.Error); ok {
		if apiErr.Reason == "captcha_invalid" && apiErr.Code == 4002 {
			rst.captcha.Invalidate()
			_, err = rst.CallJSON(ctx, &opts, req, &token)
		}
	}
	if err != nil {
		return fmt.Errorf("failed to retrieve token using username/password: %w", err)
	}
	t := &oauth2.Token{
		AccessToken:  token.AccessToken,
		TokenType:    token.TokenType,
		RefreshToken: token.RefreshToken,
		Expiry:       token.Expiry(),
	}
	return oauthutil.PutToken(name, m, t, false)
}

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "pikpak",
		Description: "PikPak",
		NewFs:       NewFs,
		CommandHelp: commandHelp,
		Config: func(ctx context.Context, name string, m configmap.Mapper, config fs.ConfigIn) (*fs.ConfigOut, error) {
			// Parse config into Options struct
			opt := new(Options)
			err := configstruct.Set(m, opt)
			if err != nil {
				return nil, fmt.Errorf("couldn't parse config into struct: %w", err)
			}

			switch config.State {
			case "":
				// Check token exists
				if _, err := oauthutil.GetToken(name, m); err != nil {
					return fs.ConfigGoto("authorize")
				}
				return fs.ConfigConfirm("authorize_ok", false, "consent_to_authorize", "Re-authorize for new token?")
			case "authorize_ok":
				if config.Result == "false" {
					return nil, nil
				}
				return fs.ConfigGoto("authorize")
			case "authorize":
				if err := pikpakAuthorize(ctx, opt, name, m); err != nil {
					return nil, err
				}
				return nil, nil
			}
			return nil, fmt.Errorf("unknown state %q", config.State)
		},
		Options: []fs.Option{{
			Name:      "user",
			Help:      "Pikpak username.",
			Required:  true,
			Sensitive: true,
		}, {
			Name:       "pass",
			Help:       "Pikpak password.",
			Required:   true,
			IsPassword: true,
		}, {
			Name:      "device_id",
			Help:      "Device ID used for authorization.",
			Advanced:  true,
			Sensitive: true,
		}, {
			Name:     "user_agent",
			Default:  defaultUserAgent,
			Advanced: true,
			Help: fmt.Sprintf(`HTTP user agent for pikpak.

Defaults to "%s" or "--pikpak-user-agent" provided on command line.`, defaultUserAgent),
		}, {
			Name: "root_folder_id",
			Help: `ID of the root folder.
Leave blank normally.

Fill in for rclone to use a non root folder as its starting point.
`,
			Advanced:  true,
			Sensitive: true,
		}, {
			Name:     "use_trash",
			Default:  true,
			Help:     "Send files to the trash instead of deleting permanently.\n\nDefaults to true, namely sending files to the trash.\nUse `--pikpak-use-trash=false` to delete files permanently instead.",
			Advanced: true,
		}, {
			Name:     "trashed_only",
			Default:  false,
			Help:     "Only show files that are in the trash.\n\nThis will show trashed files in their original directory structure.",
			Advanced: true,
		}, {
			Name:     "hash_memory_limit",
			Help:     "Files bigger than this will be cached on disk to calculate hash if required.",
			Default:  fs.SizeSuffix(10 * 1024 * 1024),
			Advanced: true,
		}, {
			Name: "chunk_size",
			Help: `Chunk size for multipart uploads.
	
Large files will be uploaded in chunks of this size.

Note that this is stored in memory and there may be up to
"--transfers" * "--pikpak-upload-concurrency" chunks stored at once
in memory.

If you are transferring large files over high-speed links and you have
enough memory, then increasing this will speed up the transfers.

Rclone will automatically increase the chunk size when uploading a
large file of known size to stay below the 10,000 chunks limit.

Increasing the chunk size decreases the accuracy of the progress
statistics displayed with "-P" flag.`,
			Default:  minChunkSize,
			Advanced: true,
		}, {
			Name: "upload_concurrency",
			Help: `Concurrency for multipart uploads.

This is the number of chunks of the same file that are uploaded
concurrently for multipart uploads.

Note that chunks are stored in memory and there may be up to
"--transfers" * "--pikpak-upload-concurrency" chunks stored at once
in memory.

If you are uploading small numbers of large files over high-speed links
and these uploads do not fully utilize your bandwidth, then increasing
this may help to speed up the transfers.`,
			Default:  defaultUploadConcurrency,
			Advanced: true,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			Default: (encoder.EncodeCtl |
				encoder.EncodeDot |
				encoder.EncodeBackSlash |
				encoder.EncodeSlash |
				encoder.EncodeDoubleQuote |
				encoder.EncodeAsterisk |
				encoder.EncodeColon |
				encoder.EncodeLtGt |
				encoder.EncodeQuestion |
				encoder.EncodePipe |
				encoder.EncodeLeftSpace |
				encoder.EncodeRightSpace |
				encoder.EncodeRightPeriod |
				encoder.EncodeInvalidUtf8),
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	Username            string               `config:"user"`
	Password            string               `config:"pass"`
	UserID              string               `config:"user_id"` // only available during runtime
	DeviceID            string               `config:"device_id"`
	UserAgent           string               `config:"user_agent"`
	RootFolderID        string               `config:"root_folder_id"`
	UseTrash            bool                 `config:"use_trash"`
	TrashedOnly         bool                 `config:"trashed_only"`
	HashMemoryThreshold fs.SizeSuffix        `config:"hash_memory_limit"`
	ChunkSize           fs.SizeSuffix        `config:"chunk_size"`
	UploadConcurrency   int                  `config:"upload_concurrency"`
	Enc                 encoder.MultiEncoder `config:"encoding"`
}

// Fs represents a remote pikpak
type Fs struct {
	name         string             // name of this remote
	root         string             // the path we are working on
	opt          Options            // parsed options
	features     *fs.Features       // optional features
	rst          *pikpakClient      // the connection to the server
	dirCache     *dircache.DirCache // Map of directory path to directory id
	pacer        *fs.Pacer          // pacer for API calls
	rootFolderID string             // the id of the root folder
	client       *http.Client       // authorized client
	m            configmap.Mapper
	tokenMu      *sync.Mutex // when renewing tokens
}

// Object describes a pikpak object
type Object struct {
	fs          *Fs       // what this object is part of
	remote      string    // The remote path
	hasMetaData bool      // whether info below has been set
	id          string    // ID of the object
	size        int64     // size of the object
	modTime     time.Time // modification time of the object
	mimeType    string    // The object MIME type
	parent      string    // ID of the parent directories
	gcid        string    // custom hash of the object
	md5sum      string    // md5sum of the object
	link        *api.Link // link to download the object
	linkMu      *sync.Mutex
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
	return fmt.Sprintf("PikPak root '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Precision return the precision of this Fs
func (f *Fs) Precision() time.Duration {
	return fs.ModTimeNotSupported
	// meaning that the modification times from the backend shouldn't be used for syncing
	// as they can't be set.
}

// DirCacheFlush resets the directory cache - used in testing as an
// optional interface
func (f *Fs) DirCacheFlush() {
	f.dirCache.ResetRoot()
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.MD5)
}

// parsePath parses a remote path
func parsePath(path string) (root string) {
	root = strings.Trim(path, "/")
	return
}

// parentIDForRequest returns ParentId for api requests
func parentIDForRequest(dirID string) string {
	if dirID == "root" {
		return ""
	}
	return dirID
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

// reAuthorize re-authorize oAuth token during runtime
func (f *Fs) reAuthorize(ctx context.Context) (err error) {
	f.tokenMu.Lock()
	defer f.tokenMu.Unlock()

	if err := pikpakAuthorize(ctx, &f.opt, f.name, f.m); err != nil {
		return err
	}
	return f.newClientWithPacer(ctx)
}

// shouldRetry returns a boolean as to whether this resp and err
// deserve to be retried.  It returns the err as a convenience
func (f *Fs) shouldRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	if err == nil {
		return false, nil
	}
	if fserrors.ShouldRetry(err) {
		return true, err
	}
	authRetry := false

	// traceback to possible api.Error wrapped in err, and re-authorize if necessary
	// "unauthenticated" (16): when access_token is invalid, but should be handled by oauthutil
	var terr *oauth2.RetrieveError
	if errors.As(err, &terr) {
		apiErr := new(api.Error)
		if err := json.Unmarshal(terr.Body, apiErr); err == nil {
			if apiErr.Reason == "invalid_grant" {
				// "invalid_grant" (4126): The refresh token is incorrect or expired				//
				// Invalid refresh token. It may have been refreshed by another process.
				authRetry = true
			}
		}
	}
	// Once err was processed by maybeWrapOAuthError() in lib/oauthutil,
	// the above code is no longer sufficient to handle the 'invalid_grant' error.
	if strings.Contains(err.Error(), "invalid_grant") {
		authRetry = true
	}

	if authRetry {
		if authErr := f.reAuthorize(ctx); authErr != nil {
			return false, fserrors.FatalError(authErr)
		}
	}

	switch apiErr := err.(type) {
	case *api.Error:
		if apiErr.Reason == "file_rename_uncompleted" {
			// "file_rename_uncompleted" (9): Renaming uncompleted file or folder is not supported
			// This error occurs when you attempt to rename objects
			// right after some server-side changes, e.g. DirMove, Move, Copy
			return true, err
		} else if apiErr.Reason == "file_duplicated_name" {
			// "file_duplicated_name" (3): File name cannot be repeated
			// This error may occur when attempting to rename temp object (newly uploaded)
			// right after the old one is removed.
			return true, err
		} else if apiErr.Reason == "task_daily_create_limit_vip" {
			// "task_daily_create_limit_vip" (11): Sorry, you have submitted too many tasks and have exceeded the current processing capacity, please try again tomorrow
			return false, fserrors.FatalError(err)
		} else if apiErr.Reason == "file_space_not_enough" {
			// "file_space_not_enough" (8): Storage space is not enough
			return false, fserrors.FatalError(err)
		} else if apiErr.Reason == "captcha_invalid" && apiErr.Code == 9 {
			// "captcha_invalid" (9): Verification code is invalid
			// This error occurred on the POST:/drive/v1/files endpoint
			// when a zero-byte file was uploaded with an invalid captcha token
			f.rst.captcha.Invalidate()
			return true, err
		}
	}

	return authRetry || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// errorHandler parses a non 2xx error response into an error
func errorHandler(resp *http.Response) error {
	// Decode error response
	errResponse := new(api.Error)
	err := rest.DecodeJSON(resp, &errResponse)
	if err != nil {
		fs.Debugf(nil, "Couldn't decode error response: %v", err)
	}
	if errResponse.Reason == "" {
		errResponse.Reason = resp.Status
	}
	if errResponse.Code == 0 {
		errResponse.Code = resp.StatusCode
	}
	return errResponse
}

// getClient makes an http client according to the options
func getClient(ctx context.Context, opt *Options) *http.Client {
	// Override few config settings and create a client
	newCtx, ci := fs.AddConfig(ctx)
	ci.UserAgent = opt.UserAgent
	return fshttp.NewClient(newCtx)
}

// newClientWithPacer sets a new http/rest client with a pacer to Fs
func (f *Fs) newClientWithPacer(ctx context.Context) (err error) {
	var ts *oauthutil.TokenSource
	f.client, ts, err = oauthutil.NewClientWithBaseClient(ctx, f.name, f.m, oauthConfig, getClient(ctx, &f.opt))
	if err != nil {
		return fmt.Errorf("failed to create oauth client: %w", err)
	}
	token, err := ts.Token()
	if err != nil {
		return err
	}
	// parse user_id from oauth access token for later use
	if parts := strings.Split(token.AccessToken, "."); len(parts) > 1 {
		jsonStr, _ := base64.URLEncoding.DecodeString(parts[1] + "===")
		info := struct {
			UserID string `json:"sub,omitempty"`
		}{}
		if jsonErr := json.Unmarshal(jsonStr, &info); jsonErr == nil {
			f.opt.UserID = info.UserID
		}
	}
	f.rst = newPikpakClient(f.client, &f.opt).SetCaptchaTokener(ctx, f.m)
	f.pacer = fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant)))
	return nil
}

// newFs partially constructs Fs from the path
//
// It constructs a valid Fs but doesn't attempt to figure out whether
// it is a file or a directory.
func newFs(ctx context.Context, name, path string, m configmap.Mapper) (*Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	if err := configstruct.Set(m, opt); err != nil {
		return nil, err
	}
	if opt.ChunkSize < minChunkSize {
		return nil, fmt.Errorf("chunk size must be at least %s", minChunkSize)
	}

	root := parsePath(path)

	f := &Fs{
		name:    name,
		root:    root,
		opt:     *opt,
		m:       m,
		tokenMu: new(sync.Mutex),
	}
	f.features = (&fs.Features{
		ReadMimeType:            true, // can read the mime type of objects
		CanHaveEmptyDirectories: true, // can have empty directories
		NoMultiThreading:        true, // can't have multiple threads downloading
	}).Fill(ctx, f)

	// new device id if necessary
	if len(f.opt.DeviceID) != 32 {
		f.opt.DeviceID = genDeviceID()
		m.Set("device_id", f.opt.DeviceID)
		fs.Infof(nil, "Using new device id %q", f.opt.DeviceID)
	}

	if err := f.newClientWithPacer(ctx); err != nil {
		// re-authorize if necessary
		if strings.Contains(err.Error(), "invalid_grant") {
			return f, f.reAuthorize(ctx)
		}
	}

	return f, nil
}

// NewFs constructs an Fs from the path, container:path
func NewFs(ctx context.Context, name, path string, m configmap.Mapper) (fs.Fs, error) {
	f, err := newFs(ctx, name, path, m)
	if err != nil {
		return nil, err
	}

	// Set the root folder ID
	if f.opt.RootFolderID != "" {
		// use root_folder ID if set
		f.rootFolderID = f.opt.RootFolderID
	} else {
		// pseudo-root
		f.rootFolderID = "root"
	}

	f.dirCache = dircache.New(f.root, f.rootFolderID, f)

	// Find the current root
	err = f.dirCache.FindRoot(ctx, false)
	if err != nil {
		// Assume it is a file
		newRoot, remote := dircache.SplitPath(f.root)
		tempF := *f
		tempF.dirCache = dircache.New(newRoot, f.rootFolderID, &tempF)
		tempF.root = newRoot
		// Make new Fs which is the parent
		err = tempF.dirCache.FindRoot(ctx, false)
		if err != nil {
			// No root so return old f
			return f, nil
		}
		_, err := tempF.NewObject(ctx, remote)
		if err != nil {
			if err == fs.ErrorObjectNotFound {
				// File doesn't exist so return old f
				return f, nil
			}
			return nil, err
		}
		f.features.Fill(ctx, &tempF)
		// XXX: update the old f here instead of returning tempF, since
		// `features` were already filled with functions having *f as a receiver.
		// See https://github.com/rclone/rclone/issues/2182
		f.dirCache = tempF.dirCache
		f.root = tempF.root
		// return an error with an fs which points to the parent
		return f, fs.ErrorIsFile
	}
	return f, nil
}

// readMetaDataForPath reads the metadata from the path
func (f *Fs) readMetaDataForPath(ctx context.Context, path string) (info *api.File, err error) {
	leaf, dirID, err := f.dirCache.FindPath(ctx, path, false)
	if err != nil {
		if err == fs.ErrorDirNotFound {
			return nil, fs.ErrorObjectNotFound
		}
		return nil, err
	}

	// checking whether fileObj with name of leaf exists in dirID
	trashed := "false"
	if f.opt.TrashedOnly {
		trashed = "true"
	}
	found, err := f.listAll(ctx, dirID, api.KindOfFile, trashed, func(item *api.File) bool {
		if item.Name == leaf {
			info = item
			return true
		}
		return false
	})
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fs.ErrorObjectNotFound
	}
	return info, nil
}

// Return an Object from a path
//
// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(ctx context.Context, remote string, info *api.File) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
		linkMu: new(sync.Mutex),
	}
	var err error
	if info != nil {
		err = o.setMetaData(info)
	} else {
		err = o.readMetaData(ctx) // reads info and meta, returning an error
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

// FindLeaf finds a directory of name leaf in the folder with ID pathID
func (f *Fs) FindLeaf(ctx context.Context, pathID, leaf string) (pathIDOut string, found bool, err error) {
	// Find the leaf in pathID
	trashed := "false"
	if f.opt.TrashedOnly {
		// still need to list folders
		trashed = ""
	}
	found, err = f.listAll(ctx, pathID, api.KindOfFolder, trashed, func(item *api.File) bool {
		if item.Name == leaf {
			pathIDOut = item.ID
			return true
		}
		return false
	})
	return pathIDOut, found, err
}

// list the objects into the function supplied
//
// If directories is set it only sends directories
// User function to process a File item from listAll
//
// Should return true to finish processing
type listAllFn func(*api.File) bool

// Lists the directory required calling the user function on each item found
//
// If the user fn ever returns true then it early exits with found = true
func (f *Fs) listAll(ctx context.Context, dirID, kind, trashed string, fn listAllFn) (found bool, err error) {
	// Url Parameters
	params := url.Values{}
	params.Set("thumbnail_size", api.ThumbnailSizeM)
	params.Set("limit", strconv.Itoa(api.ListLimit))
	params.Set("with_audit", strconv.FormatBool(true))
	if parentID := parentIDForRequest(dirID); parentID != "" {
		params.Set("parent_id", parentID)
	}

	// Construct filter string
	filters := &api.Filters{}
	filters.Set("Phase", "eq", api.PhaseTypeComplete)
	filters.Set("Trashed", "eq", trashed)
	filters.Set("Kind", "eq", kind)
	if filterStr, err := json.Marshal(filters); err == nil {
		params.Set("filters", string(filterStr))
	}
	// fs.Debugf(f, "list params: %v", params)

	opts := rest.Opts{
		Method:     "GET",
		Path:       "/drive/v1/files",
		Parameters: params,
	}

	pageToken := ""
OUTER:
	for {
		opts.Parameters.Set("page_token", pageToken)

		var info api.FileList
		var resp *http.Response
		err = f.pacer.Call(func() (bool, error) {
			resp, err = f.rst.CallJSON(ctx, &opts, nil, &info)
			return f.shouldRetry(ctx, resp, err)
		})
		if err != nil {
			return found, fmt.Errorf("couldn't list files: %w", err)
		}
		if len(info.Files) == 0 {
			break
		}
		for _, item := range info.Files {
			item.Name = f.opt.Enc.ToStandardName(item.Name)
			if fn(item) {
				found = true
				break OUTER
			}
		}
		if info.NextPageToken == "" {
			break
		}
		pageToken = info.NextPageToken
	}
	return
}

// itemToDirEntry converts a api.File to an fs.DirEntry.
// When the api.File cannot be represented as an fs.DirEntry
// (nil, nil) is returned.
func (f *Fs) itemToDirEntry(ctx context.Context, remote string, item *api.File) (entry fs.DirEntry, err error) {
	switch {
	case item.Kind == api.KindOfFolder:
		// cache the directory ID for later lookups
		f.dirCache.Put(remote, item.ID)
		d := fs.NewDir(remote, time.Time(item.ModifiedTime)).SetID(item.ID)
		if item.ParentID == "" {
			d.SetParentID("root")
		} else {
			d.SetParentID(item.ParentID)
		}
		return d, nil
	case f.opt.TrashedOnly && !item.Trashed:
		// ignore object
	default:
		entry, err = f.newObjectWithInfo(ctx, remote, item)
		if err == fs.ErrorObjectNotFound {
			return nil, nil
		}
		return entry, err
	}
	return nil, nil
}

// List the objects and directories in dir into entries. The
// entries can be returned in any order but should be for a
// complete directory.
//
// dir should be "" to list the root, and should not have
// trailing slashes.
//
// This should return ErrDirNotFound if the directory isn't
// found.
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	// fs.Debugf(f, "List(%q)\n", dir)
	dirID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return nil, err
	}
	var iErr error
	trashed := "false"
	if f.opt.TrashedOnly {
		// still need to list folders
		trashed = ""
	}
	_, err = f.listAll(ctx, dirID, "", trashed, func(item *api.File) bool {
		entry, err := f.itemToDirEntry(ctx, path.Join(dir, item.Name), item)
		if err != nil {
			iErr = err
			return true
		}
		if entry != nil {
			entries = append(entries, entry)
		}
		return false
	})
	if err != nil {
		return nil, err
	}
	if iErr != nil {
		return nil, iErr
	}
	return entries, nil
}

// CreateDir makes a directory with pathID as parent and name leaf
func (f *Fs) CreateDir(ctx context.Context, pathID, leaf string) (newID string, err error) {
	// fs.Debugf(f, "CreateDir(%q, %q)\n", pathID, leaf)
	req := api.RequestNewFile{
		Name:     f.opt.Enc.FromStandardName(leaf),
		Kind:     api.KindOfFolder,
		ParentID: parentIDForRequest(pathID),
	}
	info, err := f.requestNewFile(ctx, &req)
	if err != nil {
		return "", err
	}
	return info.File.ID, nil
}

// Mkdir creates the container if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	_, err := f.dirCache.FindDir(ctx, dir, true)
	return err
}

// About gets quota information
func (f *Fs) About(ctx context.Context) (usage *fs.Usage, err error) {
	info, err := f.getAbout(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get drive quota: %w", err)
	}
	q := info.Quota
	usage = &fs.Usage{
		Used: fs.NewUsageValue(q.Usage), // bytes in use
		// Trashed: fs.NewUsageValue(q.UsageInTrash), // bytes in trash but this seems not working
	}
	if q.Limit > 0 {
		usage.Total = fs.NewUsageValue(q.Limit)          // quota of bytes that can be used
		usage.Free = fs.NewUsageValue(q.Limit - q.Usage) // bytes which can be uploaded before reaching the quota
	}
	return usage, nil
}

// PublicLink adds a "readable by anyone with link" permission on the given file or folder.
func (f *Fs) PublicLink(ctx context.Context, remote string, expire fs.Duration, unlink bool) (string, error) {
	id, err := f.dirCache.FindDir(ctx, remote, false)
	if err == nil {
		fs.Debugf(f, "attempting to share directory '%s'", remote)
	} else {
		fs.Debugf(f, "attempting to share single file '%s'", remote)
		o, err := f.NewObject(ctx, remote)
		if err != nil {
			return "", err
		}
		id = o.(*Object).id
	}
	expiry := -1
	if expire < fs.DurationOff {
		expiry = int(math.Ceil(time.Duration(expire).Hours() / 24))
	}
	req := api.RequestShare{
		FileIDs:        []string{id},
		ShareTo:        "publiclink",
		ExpirationDays: expiry,
		PassCodeOption: "NOT_REQUIRED",
	}
	info, err := f.requestShare(ctx, &req)
	if err != nil {
		return "", err
	}
	return info.ShareURL, err
}

// delete a file or directory by ID w/o using trash
func (f *Fs) deleteObjects(ctx context.Context, IDs []string, useTrash bool) (err error) {
	if len(IDs) == 0 {
		return nil
	}
	action := "batchDelete"
	if useTrash {
		action = "batchTrash"
	}
	req := api.RequestBatch{
		IDs: IDs,
	}
	if err := f.requestBatchAction(ctx, action, &req); err != nil {
		return fmt.Errorf("delete object failed: %w", err)
	}
	return nil
}

// purgeCheck removes the root directory, if check is set then it
// refuses to do so if it has anything in
func (f *Fs) purgeCheck(ctx context.Context, dir string, check bool) error {
	root := path.Join(f.root, dir)
	if root == "" {
		return errors.New("can't purge root directory")
	}
	rootID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return err
	}

	trashedFiles := false
	if check {
		found, err := f.listAll(ctx, rootID, "", "", func(item *api.File) bool {
			if !item.Trashed {
				fs.Debugf(dir, "Rmdir: contains file: %q", item.Name)
				return true
			}
			fs.Debugf(dir, "Rmdir: contains trashed file: %q", item.Name)
			trashedFiles = true
			return false
		})
		if err != nil {
			return err
		}
		if found {
			return fs.ErrorDirectoryNotEmpty
		}
	}
	if root != "" {
		// trash the directory if it had trashed files
		// in or the user wants to trash, otherwise
		// delete it.
		err = f.deleteObjects(ctx, []string{rootID}, trashedFiles || f.opt.UseTrash)
		if err != nil {
			return err
		}
	} else if check {
		return errors.New("can't purge root directory")
	}
	f.dirCache.FlushDir(dir)
	if err != nil {
		return err
	}
	return nil
}

// Rmdir deletes the root folder
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir, true)
}

// Purge deletes all the files and the container
//
// Optional interface: Only implement this if you have a way of
// deleting all the files quicker than just running Remove() on the
// result of List()
func (f *Fs) Purge(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir, false)
}

// CleanUp empties the trash
func (f *Fs) CleanUp(ctx context.Context) (err error) {
	opts := rest.Opts{
		Method: "PATCH",
		Path:   "/drive/v1/files/trash:empty",
	}
	info := struct {
		TaskID string `json:"task_id"`
	}{}
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.rst.CallJSON(ctx, &opts, nil, &info)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("couldn't empty trash: %w", err)
	}
	return f.waitTask(ctx, info.TaskID)
}

// Move the object
func (f *Fs) moveObjects(ctx context.Context, IDs []string, dirID string) (err error) {
	if len(IDs) == 0 {
		return nil
	}
	req := api.RequestBatch{
		IDs: IDs,
		To:  map[string]string{"parent_id": parentIDForRequest(dirID)},
	}
	if err := f.requestBatchAction(ctx, "batchMove", &req); err != nil {
		return fmt.Errorf("move object failed: %w", err)
	}
	return nil
}

// renames the object
func (f *Fs) renameObject(ctx context.Context, ID, newName string) (info *api.File, err error) {
	req := api.File{
		Name: f.opt.Enc.FromStandardName(newName),
	}
	info, err = f.patchFile(ctx, ID, &req)
	if err != nil {
		return nil, fmt.Errorf("rename object failed: %w", err)
	}
	return
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

	srcID, srcParentID, srcLeaf, dstParentID, dstLeaf, err := f.dirCache.DirMove(ctx, srcFs.dirCache, srcFs.root, srcRemote, f.root, dstRemote)
	if err != nil {
		return err
	}

	if srcParentID != dstParentID {
		// Do the move
		err = f.moveObjects(ctx, []string{srcID}, dstParentID)
		if err != nil {
			return fmt.Errorf("couldn't dir move: %w", err)
		}
	}

	// Can't copy and change name in one step so we have to check if we have
	// the correct name after copy
	if srcLeaf != dstLeaf {
		_, err = f.renameObject(ctx, srcID, dstLeaf)
		if err != nil {
			return fmt.Errorf("dirmove: couldn't rename moved dir: %w", err)
		}
	}
	srcFs.dirCache.FlushDir(srcRemote)
	return nil
}

// Creates from the parameters passed in a half finished Object which
// must have setMetaData called on it
//
// Returns the object, leaf, dirID and error.
//
// Used to create new objects
func (f *Fs) createObject(ctx context.Context, remote string, modTime time.Time, size int64) (o *Object, leaf string, dirID string, err error) {
	// Create the directory for the object if it doesn't exist
	leaf, dirID, err = f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return
	}
	// Temporary Object under construction
	o = &Object{
		fs:      f,
		remote:  remote,
		parent:  dirID,
		size:    size,
		modTime: modTime,
		linkMu:  new(sync.Mutex),
	}
	return o, leaf, dirID, nil
}

// Move src to this remote using server-side move operations.
//
// This is stored with the remote path given.
//
// It returns the destination Object and a possible error.
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
	err := srcObj.readMetaData(ctx)
	if err != nil {
		return nil, err
	}

	srcLeaf, srcParentID, err := srcObj.fs.dirCache.FindPath(ctx, srcObj.remote, false)
	if err != nil {
		return nil, err
	}

	// Create temporary object - still missing id, mimeType, gcid, md5sum
	dstObj, dstLeaf, dstParentID, err := f.createObject(ctx, remote, srcObj.modTime, srcObj.size)
	if err != nil {
		return nil, err
	}

	if srcParentID != dstParentID {
		// Do the move
		if err = f.moveObjects(ctx, []string{srcObj.id}, dstParentID); err != nil {
			return nil, err
		}
	}
	// Manually update info of moved object to save API calls
	dstObj.id = srcObj.id
	dstObj.mimeType = srcObj.mimeType
	dstObj.gcid = srcObj.gcid
	dstObj.md5sum = srcObj.md5sum
	dstObj.hasMetaData = true

	if srcLeaf != dstLeaf {
		// Rename
		info, err := f.renameObject(ctx, srcObj.id, dstLeaf)
		if err != nil {
			return nil, fmt.Errorf("move: couldn't rename moved file: %w", err)
		}
		return dstObj, dstObj.setMetaData(info)
	}
	return dstObj, nil
}

// copy objects
func (f *Fs) copyObjects(ctx context.Context, IDs []string, dirID string) (err error) {
	if len(IDs) == 0 {
		return nil
	}
	req := api.RequestBatch{
		IDs: IDs,
		To:  map[string]string{"parent_id": parentIDForRequest(dirID)},
	}
	if err := f.requestBatchAction(ctx, "batchCopy", &req); err != nil {
		return fmt.Errorf("copy object failed: %w", err)
	}
	return nil
}

// Copy src to this remote using server side copy operations.
//
// This is stored with the remote path given.
//
// It returns the destination Object and a possible error.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantCopy
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}
	err := srcObj.readMetaData(ctx)
	if err != nil {
		return nil, err
	}

	// Create temporary object - still missing id, mimeType, gcid, md5sum
	dstObj, dstLeaf, dstParentID, err := f.createObject(ctx, remote, srcObj.modTime, srcObj.size)
	if err != nil {
		return nil, err
	}
	if srcObj.parent == dstParentID {
		// api restriction
		fs.Debugf(src, "Can't copy - same parent")
		return nil, fs.ErrorCantCopy
	}
	// Copy the object
	if err := f.copyObjects(ctx, []string{srcObj.id}, dstParentID); err != nil {
		return nil, fmt.Errorf("couldn't copy file: %w", err)
	}
	// Update info of the copied object with new parent but source name
	if info, err := dstObj.fs.readMetaDataForPath(ctx, srcObj.remote); err != nil {
		return nil, fmt.Errorf("copy: couldn't locate copied file: %w", err)
	} else if err = dstObj.setMetaData(info); err != nil {
		return nil, err
	}

	// Can't copy and change name in one step so we have to check if we have
	// the correct name after copy
	srcLeaf, _, err := srcObj.fs.dirCache.FindPath(ctx, srcObj.remote, false)
	if err != nil {
		return nil, err
	}

	if srcLeaf != dstLeaf {
		// Rename
		info, err := f.renameObject(ctx, dstObj.id, dstLeaf)
		if err != nil {
			return nil, fmt.Errorf("copy: couldn't rename copied file: %w", err)
		}
		return dstObj, dstObj.setMetaData(info)
	}
	return dstObj, nil
}

func (f *Fs) uploadByForm(ctx context.Context, in io.Reader, name string, size int64, form *api.Form, options ...fs.OpenOption) (err error) {
	// struct to map. transferring values from MultParts to url parameter
	params := url.Values{}
	iVal := reflect.ValueOf(&form.MultiParts).Elem()
	iTyp := iVal.Type()
	for i := 0; i < iVal.NumField(); i++ {
		params.Set(iTyp.Field(i).Tag.Get("json"), iVal.Field(i).String())
	}
	formReader, contentType, overhead, err := rest.MultipartUpload(ctx, in, params, "file", name)
	if err != nil {
		return fmt.Errorf("failed to make multipart upload: %w", err)
	}

	contentLength := overhead + size
	opts := rest.Opts{
		Method:           form.Method,
		RootURL:          form.URL,
		Body:             formReader,
		ContentType:      contentType,
		ContentLength:    &contentLength,
		Options:          options,
		TransferEncoding: []string{"identity"},
		NoResponse:       true,
	}

	var resp *http.Response
	err = f.pacer.CallNoRetry(func() (bool, error) {
		resp, err = f.rst.Call(ctx, &opts)
		return f.shouldRetry(ctx, resp, err)
	})
	return
}

func (f *Fs) uploadByResumable(ctx context.Context, in io.Reader, name string, size int64, resumable *api.Resumable) (err error) {
	p := resumable.Params

	// Create a credentials provider
	creds := credentials.NewStaticCredentialsProvider(p.AccessKeyID, p.AccessKeySecret, p.SecurityToken)

	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithCredentialsProvider(creds),
		awsconfig.WithRegion("pikpak"))
	if err != nil {
		return
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String("https://mypikpak.com/")
	})
	partSize := chunksize.Calculator(name, size, int(manager.MaxUploadParts), f.opt.ChunkSize)

	// Create an uploader with custom options
	uploader := manager.NewUploader(client, func(u *manager.Uploader) {
		u.PartSize = int64(partSize)
		u.Concurrency = f.opt.UploadConcurrency
	})
	// Perform an upload
	_, err = uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket: &p.Bucket,
		Key:    &p.Key,
		Body:   in,
	})
	return
}

func (f *Fs) upload(ctx context.Context, in io.Reader, leaf, dirID, gcid string, size int64, options ...fs.OpenOption) (info *api.File, err error) {
	// determine upload type
	uploadType := api.UploadTypeResumable
	// if size >= 0 && size < int64(5*fs.Mebi) {
	// 	uploadType = api.UploadTypeForm
	// }
	// stop using uploadByForm() cause it is not as reliable as uploadByResumable() for a large number of small files

	// request upload ticket to API
	req := api.RequestNewFile{
		Kind:       api.KindOfFile,
		Name:       f.opt.Enc.FromStandardName(leaf),
		ParentID:   parentIDForRequest(dirID),
		FolderType: "NORMAL",
		Size:       size,
		Hash:       strings.ToUpper(gcid),
		UploadType: uploadType,
	}
	if uploadType == api.UploadTypeResumable {
		req.Resumable = map[string]string{"provider": "PROVIDER_ALIYUN"}
	}
	new, err := f.requestNewFile(ctx, &req)
	if err != nil {
		return nil, fmt.Errorf("failed to create a new file: %w", err)
	}
	if new.File == nil {
		return nil, fmt.Errorf("invalid response: %+v", new)
	} else if new.File.Phase == api.PhaseTypeComplete {
		// early return; in case of zero-byte objects
		if acc, ok := in.(*accounting.Account); ok && acc != nil {
			// if `in io.Reader` is still in type of `*accounting.Account` (meaning that it is unused)
			// it is considered as a server side copy as no incoming/outgoing traffic occur at all
			acc.ServerSideTransferStart()
			acc.ServerSideCopyEnd(size)
		}
		return new.File, nil
	}

	defer atexit.OnError(&err, func() {
		fs.Debugf(leaf, "canceling upload: %v", err)
		if cancelErr := f.deleteObjects(ctx, []string{new.File.ID}, false); cancelErr != nil {
			fs.Logf(leaf, "failed to cancel upload: %v", cancelErr)
		}
		if cancelErr := f.deleteTask(ctx, new.Task.ID, false); cancelErr != nil {
			fs.Logf(leaf, "failed to cancel upload: %v", cancelErr)
		}
		fs.Debugf(leaf, "waiting %v for the cancellation to be effective", taskWaitTime)
		time.Sleep(taskWaitTime)
	})()

	if uploadType == api.UploadTypeForm && new.Form != nil {
		err = f.uploadByForm(ctx, in, req.Name, size, new.Form, options...)
	} else if uploadType == api.UploadTypeResumable && new.Resumable != nil {
		err = f.uploadByResumable(ctx, in, leaf, size, new.Resumable)
	} else {
		err = fmt.Errorf("no method available for uploading: %+v", new)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to upload: %w", err)
	}
	return new.File, f.waitTask(ctx, new.Task.ID)
}

// Put the object
//
// Copy the reader in to the new object which is returned.
//
// The new object may have been created if an error is returned
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	existingObj, err := f.NewObject(ctx, src.Remote())
	switch err {
	case nil:
		return existingObj, existingObj.Update(ctx, in, src, options...)
	case fs.ErrorObjectNotFound:
		// Not found so create it
		newObj := &Object{
			fs:     f,
			remote: src.Remote(),
			linkMu: new(sync.Mutex),
		}
		return newObj, newObj.upload(ctx, in, src, false, options...)
	default:
		return nil, err
	}
}

// UserInfo fetches info about the current user
func (f *Fs) UserInfo(ctx context.Context) (userInfo map[string]string, err error) {
	user, err := f.getUserInfo(ctx)
	if err != nil {
		return nil, err
	}
	userInfo = map[string]string{
		"Id":                user.Sub,
		"Username":          user.Name,
		"Email":             user.Email,
		"PhoneNumber":       user.PhoneNumber,
		"Password":          user.Password,
		"Status":            user.Status,
		"CreatedAt":         time.Time(user.CreatedAt).String(),
		"PasswordUpdatedAt": time.Time(user.PasswordUpdatedAt).String(),
	}
	if vip, err := f.getVIPInfo(ctx); err == nil && vip.Result == "ACCEPTED" {
		userInfo["VIPExpiresAt"] = time.Time(vip.Data.Expire).String()
		userInfo["VIPStatus"] = vip.Data.Status
		userInfo["VIPType"] = vip.Data.Type
	}
	return userInfo, nil
}

// ------------------------------------------------------------

// add offline download task for url
func (f *Fs) addURL(ctx context.Context, url, path string) (*api.Task, error) {
	req := api.RequestNewTask{
		Kind:       api.KindOfFile,
		UploadType: "UPLOAD_TYPE_URL",
		URL: &api.URL{
			URL: url,
		},
		FolderType: "DOWNLOAD",
	}
	if parentID, err := f.dirCache.FindDir(ctx, path, false); err == nil {
		req.ParentID = parentIDForRequest(parentID)
		req.FolderType = ""
	}
	return f.requestNewTask(ctx, &req)
}

type decompressDirResult struct {
	Decompressed  int
	SourceDeleted int
	Errors        int
}

func (r decompressDirResult) Error() string {
	return fmt.Sprintf("%d error(s) while decompressing - see log", r.Errors)
}

// decompress file/files in a directory of an ID
func (f *Fs) decompressDir(ctx context.Context, filename, id, password string, srcDelete bool) (r decompressDirResult, err error) {
	_, err = f.listAll(ctx, id, api.KindOfFile, "false", func(item *api.File) bool {
		if item.MimeType == "application/zip" || item.MimeType == "application/x-7z-compressed" || item.MimeType == "application/x-rar-compressed" {
			if filename == "" || filename == item.Name {
				res, err := f.requestDecompress(ctx, item, password)
				if err != nil {
					err = fmt.Errorf("unexpected error while requesting decompress of %q: %w", item.Name, err)
					r.Errors++
					fs.Errorf(f, "%v", err)
				} else if res.Status != "OK" {
					r.Errors++
					fs.Errorf(f, "%q: %d files: %s", item.Name, res.FilesNum, res.Status)
				} else {
					r.Decompressed++
					fs.Infof(f, "%q: %d files: %s", item.Name, res.FilesNum, res.Status)
					if srcDelete {
						derr := f.deleteObjects(ctx, []string{item.ID}, f.opt.UseTrash)
						if derr != nil {
							derr = fmt.Errorf("failed to delete %q: %w", item.Name, derr)
							r.Errors++
							fs.Errorf(f, "%v", derr)
						} else {
							r.SourceDeleted++
						}
					}
				}
			}
		}
		return false
	})
	if err != nil {
		err = fmt.Errorf("couldn't list files to decompress: %w", err)
		r.Errors++
		fs.Errorf(f, "%v", err)
	}
	if r.Errors != 0 {
		return r, r
	}
	return r, nil
}

var commandHelp = []fs.CommandHelp{{
	Name:  "addurl",
	Short: "Add offline download task for url",
	Long: `This command adds offline download task for url.

Usage:

    rclone backend addurl pikpak:dirpath url

Downloads will be stored in 'dirpath'. If 'dirpath' is invalid, 
download will fallback to default 'My Pack' folder.
`,
}, {
	Name:  "decompress",
	Short: "Request decompress of a file/files in a folder",
	Long: `This command requests decompress of file/files in a folder.

Usage:

    rclone backend decompress pikpak:dirpath {filename} -o password=password
    rclone backend decompress pikpak:dirpath {filename} -o delete-src-file

An optional argument 'filename' can be specified for a file located in 
'pikpak:dirpath'. You may want to pass '-o password=password' for a 
password-protected files. Also, pass '-o delete-src-file' to delete 
source files after decompression finished.

Result:

    {
        "Decompressed": 17,
        "SourceDeleted": 0,
        "Errors": 0
    }
`,
}}

// Command the backend to run a named command
//
// The command run is name
// args may be used to read arguments from
// opts may be used to read optional arguments from
//
// The result should be capable of being JSON encoded
// If it is a string or a []string it will be shown to the user
// otherwise it will be JSON encoded and shown to the user like that
func (f *Fs) Command(ctx context.Context, name string, arg []string, opt map[string]string) (out interface{}, err error) {
	switch name {
	case "addurl":
		if len(arg) != 1 {
			return nil, errors.New("need exactly 1 argument")
		}
		return f.addURL(ctx, arg[0], "")
	case "decompress":
		filename := ""
		if len(arg) > 0 {
			filename = arg[0]
		}
		id, err := f.dirCache.FindDir(ctx, "", false)
		if err != nil {
			return nil, fmt.Errorf("failed to get an ID of dirpath: %w", err)
		}
		password := ""
		if pass, ok := opt["password"]; ok {
			password = pass
		}
		_, srcDelete := opt["delete-src-file"]
		return f.decompressDir(ctx, filename, id, password, srcDelete)
	default:
		return nil, fs.ErrorCommandNotFound
	}
}

// ------------------------------------------------------------

// parseFileID gets fid parameter from url query
func parseFileID(s string) string {
	if u, err := url.Parse(s); err == nil {
		if q, err := url.ParseQuery(u.RawQuery); err == nil {
			return q.Get("fid")
		}
	}
	return ""
}

// setMetaData sets the metadata from info
func (o *Object) setMetaData(info *api.File) (err error) {
	if info.Kind == api.KindOfFolder {
		return fs.ErrorIsDir
	}
	if info.Kind != api.KindOfFile {
		return fmt.Errorf("%q is %q: %w", o.remote, info.Kind, fs.ErrorNotAFile)
	}
	o.hasMetaData = true
	o.id = info.ID
	o.size = info.Size
	o.modTime = time.Time(info.ModifiedTime)
	o.mimeType = info.MimeType
	if info.ParentID == "" {
		o.parent = "root"
	} else {
		o.parent = info.ParentID
	}
	o.gcid = info.Hash
	o.md5sum = info.Md5Checksum
	if info.Links.ApplicationOctetStream != nil {
		o.link = info.Links.ApplicationOctetStream
		if fid := parseFileID(o.link.URL); fid != "" {
			for mid, media := range info.Medias {
				if media.Link == nil {
					continue
				}
				if mfid := parseFileID(media.Link.URL); fid == mfid {
					fs.Debugf(o, "Using a media link from Medias[%d]", mid)
					o.link = media.Link
					break
				}
			}
		}
	}
	return nil
}

// setMetaDataWithLink ensures a link for opening an object
func (o *Object) setMetaDataWithLink(ctx context.Context) error {
	o.linkMu.Lock()
	defer o.linkMu.Unlock()

	// check if the current link is valid
	if o.link.Valid() {
		return nil
	}

	info, err := o.fs.getFile(ctx, o.id)
	if err != nil {
		return err
	}
	return o.setMetaData(info)
}

// readMetaData gets the metadata if it hasn't already been fetched
//
// it also sets the info
func (o *Object) readMetaData(ctx context.Context) (err error) {
	if o.hasMetaData {
		return nil
	}
	info, err := o.fs.readMetaDataForPath(ctx, o.remote)
	if err != nil {
		return err
	}
	return o.setMetaData(info)
}

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

// Hash returns the Md5sum of an object returning a lowercase hex string
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	if t != hash.MD5 {
		return "", hash.ErrUnsupported
	}
	return strings.ToLower(o.md5sum), nil
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	err := o.readMetaData(context.TODO())
	if err != nil {
		fs.Logf(o, "Failed to read metadata: %v", err)
		return 0
	}
	return o.size
}

// MimeType of an Object if known, "" otherwise
func (o *Object) MimeType(ctx context.Context) string {
	return o.mimeType
}

// ID returns the ID of the Object if known, or "" if not
func (o *Object) ID() string {
	return o.id
}

// ParentID returns the ID of the Object parent if known, or "" if not
func (o *Object) ParentID() string {
	return o.parent
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
	return fs.ErrorCantSetModTime
}

// Storable returns a boolean showing whether this object storable
func (o *Object) Storable() bool {
	return true
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	return o.fs.deleteObjects(ctx, []string{o.id}, o.fs.opt.UseTrash)
}

// httpResponse gets an http.Response object for the object
// using the url and method passed in
func (o *Object) httpResponse(ctx context.Context, url, method string, options []fs.OpenOption) (res *http.Response, err error) {
	if url == "" {
		return nil, errors.New("forbidden to download - check sharing permission")
	}
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, err
	}
	fs.FixRangeOption(options, o.size)
	fs.OpenOptionAddHTTPHeaders(req.Header, options)
	if o.size == 0 {
		// Don't supply range requests for 0 length objects as they always fail
		delete(req.Header, "Range")
	}
	err = o.fs.pacer.Call(func() (bool, error) {
		res, err = o.fs.client.Do(req)
		return o.fs.shouldRetry(ctx, res, err)
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// open a url for reading
func (o *Object) open(ctx context.Context, url string, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	res, err := o.httpResponse(ctx, url, "GET", options)
	if err != nil {
		return nil, fmt.Errorf("open file failed: %w", err)
	}
	return res.Body, nil
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	if o.id == "" {
		return nil, errors.New("can't download: no id")
	}
	if o.size == 0 {
		// zero-byte objects may have no download link
		return io.NopCloser(bytes.NewBuffer([]byte(nil))), nil
	}
	if err = o.setMetaDataWithLink(ctx); err != nil {
		return nil, fmt.Errorf("can't download: %w", err)
	}
	return o.open(ctx, o.link.URL, options...)
}

// Update the object with the contents of the io.Reader, modTime and size
//
// If existing is set then it updates the object rather than creating a new one.
//
// The new object may have been created if an error is returned
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	return o.upload(ctx, in, src, true, options...)
}

// upload uploads the object with or without using a temporary file name
func (o *Object) upload(ctx context.Context, in io.Reader, src fs.ObjectInfo, withTemp bool, options ...fs.OpenOption) (err error) {
	size := src.Size()
	remote := o.Remote()

	// Create the directory for the object if it doesn't exist
	leaf, dirID, err := o.fs.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return err
	}

	// Calculate gcid; grabbed from package jottacloud
	gcid, err := o.fs.getGcid(ctx, src)
	if err != nil || gcid == "" {
		fs.Debugf(o, "calculating gcid: %v", err)
		if srcObj := fs.UnWrapObjectInfo(src); srcObj != nil && srcObj.Fs().Features().IsLocal {
			// No buffering; directly calculate gcid from source
			rc, err := srcObj.Open(ctx)
			if err != nil {
				return fmt.Errorf("failed to open src: %w", err)
			}
			defer fs.CheckClose(rc, &err)

			if gcid, err = calcGcid(rc, srcObj.Size()); err != nil {
				return fmt.Errorf("failed to calculate gcid: %w", err)
			}
		} else {
			var cleanup func()
			gcid, in, cleanup, err = readGcid(in, size, int64(o.fs.opt.HashMemoryThreshold))
			defer cleanup()
			if err != nil {
				return fmt.Errorf("failed to calculate gcid: %w", err)
			}
		}
	}
	fs.Debugf(o, "gcid = %s", gcid)

	if !withTemp {
		info, err := o.fs.upload(ctx, in, leaf, dirID, gcid, size, options...)
		if err != nil {
			return err
		}
		return o.setMetaData(info)
	}

	// We have to fall back to upload + rename
	tempName := "rcloneTemp" + random.String(8)
	info, err := o.fs.upload(ctx, in, tempName, dirID, gcid, size, options...)
	if err != nil {
		return err
	}

	// upload was successful, need to delete old object before rename
	if err = o.Remove(ctx); err != nil {
		return fmt.Errorf("failed to remove old object: %w", err)
	}

	// rename also updates metadata
	if info, err = o.fs.renameObject(ctx, info.ID, leaf); err != nil {
		return fmt.Errorf("failed to rename temp object: %w", err)
	}
	return o.setMetaData(info)
}

// Check the interfaces are satisfied
var (
	// _ fs.ListRer         = (*Fs)(nil)
	// _ fs.ChangeNotifier  = (*Fs)(nil)
	// _ fs.PutStreamer     = (*Fs)(nil)
	_ fs.Fs              = (*Fs)(nil)
	_ fs.Purger          = (*Fs)(nil)
	_ fs.CleanUpper      = (*Fs)(nil)
	_ fs.Copier          = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ fs.Commander       = (*Fs)(nil)
	_ fs.DirCacheFlusher = (*Fs)(nil)
	_ fs.PublicLinker    = (*Fs)(nil)
	_ fs.Abouter         = (*Fs)(nil)
	_ fs.UserInfoer      = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.MimeTyper       = (*Object)(nil)
	_ fs.IDer            = (*Object)(nil)
	_ fs.ParentIDer      = (*Object)(nil)
)
