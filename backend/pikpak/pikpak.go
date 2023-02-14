// Package pikpak provides an interface to the PikPak
package pikpak

// ------------------------------------------------------------
// NOTE
// ------------------------------------------------------------

// md5sum is not always available, sometimes given empty.

// sha1sum used for upload differs from the one with official apps.

// Trashed files are not restored to the original location when using `batchUntrash`

// Can't stream without `--vfs-cache-mode=full`

// ------------------------------------------------------------
// TODO
// ------------------------------------------------------------

// * List() with options starred-only
// * PublicLink() with more user-configurable options
// * uploadByResumable() with configurable chunk-size
// * user-configurable list chunk
// * Prefer api.Medias[].Link.Url for opening media
// * backend command: untrash, iscached
// * api(event,task)

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/rclone/rclone/backend/pikpak/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/hash"
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
	rcloneClientID              = "YcrttD06T9PIkqAY"
	rcloneEncryptedClientSecret = "LWuiaQ-Ddj0aOK7V11pCaemYcR65sHJTDRew16OfiSYEUl0DG3g"
	minSleep                    = 10 * time.Millisecond
	maxSleep                    = 2 * time.Second
	decayConstant               = 2 // bigger for slower decay, exponential
	rootURL                     = "https://api-drive.mypikpak.com"
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
		ClientID:     rcloneClientID,
		ClientSecret: obscure.MustReveal(rcloneEncryptedClientSecret),
		RedirectURL:  oauthutil.RedirectURL,
	}
)

// Returns OAuthOptions modified for pikpak
func pikpakOAuthOptions() []fs.Option {
	opts := []fs.Option{}
	for _, opt := range oauthutil.SharedOptions {
		if opt.Name == config.ConfigClientID {
			opt.Advanced = true
		} else if opt.Name == config.ConfigClientSecret {
			opt.Advanced = true
		}
		opts = append(opts, opt)
	}
	return opts
}

// pikpakAutorize retrieves OAuth token using user/pass and save it to rclone.conf
func pikpakAuthorize(ctx context.Context, opt *Options, name string, m configmap.Mapper) error {
	pass := obscure.MustReveal(opt.Password)
	t, err := oauthConfig.PasswordCredentialsToken(ctx, opt.Username, pass)
	if err != nil {
		return fmt.Errorf("failed to retrieve token using username/password: %w", err)
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
		Options: append(pikpakOAuthOptions(), []fs.Option{{
			Name:     "user",
			Help:     "Pikpak username.",
			Required: true,
		}, {
			Name:       "pass",
			Help:       "Pikpak password.",
			Required:   true,
			IsPassword: true,
		}, {
			Name: "root_folder_id",
			Help: `ID of the root folder.
Leave blank normally.

Fill in for rclone to use a non root folder as its starting point.
`,
			Advanced: true,
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
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			Default: (encoder.EncodeCtl |
				encoder.EncodeDot |
				encoder.EncodeBackSlash |
				encoder.EncodeSlash |
				encoder.EncodeWin |
				encoder.EncodeLeftSpace |
				encoder.EncodeLeftPeriod |
				encoder.EncodeLeftTilde |
				encoder.EncodeLeftCrLfHtVt |
				encoder.EncodeRightSpace |
				encoder.EncodeRightPeriod |
				encoder.EncodeRightCrLfHtVt |
				encoder.EncodeInvalidUtf8),
		}}...),
	})
}

// Options defines the configuration for this backend
type Options struct {
	Username            string               `config:"user"`
	Password            string               `config:"pass"`
	RootFolderID        string               `config:"root_folder_id"`
	UseTrash            bool                 `config:"use_trash"`
	TrashedOnly         bool                 `config:"trashed_only"`
	StarredOnly         bool                 `config:"starred_only"`
	HashMemoryThreshold fs.SizeSuffix        `config:"hash_memory_limit"`
	Enc                 encoder.MultiEncoder `config:"encoding"`
}

// Fs represents a remote pikpak
type Fs struct {
	name         string             // name of this remote
	root         string             // the path we are working on
	opt          Options            // parsed options
	features     *fs.Features       // optional features
	srv          *rest.Client       // the connection to the server
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
	id          string    // ID of the object
	size        int64     // size of the object
	modTime     time.Time // modification time of the object
	mimeType    string    // The object MIME type
	parent      string    // ID of the parent directories
	md5sum      string    // md5sum of the object
	hasMetaData bool      // whether info below has been set
	link        *api.Link // download links
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

// parentIdForRequest returns ParentId for api requests
func parentIdForRequest(Id string) string {
	if Id == "root" {
		return ""
	} else {
		return Id
	}
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

func (f *Fs) doAuthorize(ctx context.Context) (err error) {
	f.tokenMu.Lock()
	defer f.tokenMu.Unlock()

	if err := pikpakAuthorize(ctx, &f.opt, f.name, f.m); err != nil {
		return err
	}
	f.client, _, err = oauthutil.NewClient(ctx, f.name, f.m, oauthConfig)
	if err != nil {
		return fmt.Errorf("failed to create oauth client: %w", err)
	}
	f.srv = rest.NewClient(f.client).SetRoot(rootURL).SetErrorHandler(errorHandler)
	if err != nil {
		return fmt.Errorf("couldn't create rest client: %w", err)
	}
	if err == nil {
		f.pacer = fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant)))
	}
	return err
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

	if resp != nil && resp.StatusCode == 401 && len(resp.Header["Www-Authenticate"]) == 1 && strings.Contains(resp.Header["Www-Authenticate"][0], "expired_token") {
		authRetry = true
		fs.Debugf(nil, "Should retry: %v", err)
	}

	// traceback to possible api.Error wrapped in err, and re-authorize if necessary
	// "unauthenticated" (16): when access_token is invalid
	// "invalid_grant" (4126): when refresh_token is invalid
	var terr *oauth2.RetrieveError
	if errors.As(err, &terr) {
		apiErr := new(api.Error)
		if err := json.Unmarshal(terr.Body, apiErr); err == nil {
			if apiErr.Reason == "invalid_grant" {
				fs.Debugf(f, "Token is invalid. Trying to get a new one using username/password...")
				if err := f.doAuthorize(ctx); err != nil {
					return false, fserrors.FatalError(err)
				}
				return true, nil
			}
		}
	}

	switch apiErr := err.(type) {
	case *api.Error:
		if apiErr.Reason == "file_rename_uncompleted" {
			// "file_rename_uncompleted" (9): Renaming uncompleted file or folder is not supported
			// This error occurs when you attempt to rename objects
			// right after some server-side changes, e.g. DirMove, Move, Copy
			return true, nil
		} else if apiErr.Reason == "file_duplicated_name" {
			// "file_duplicated_name" (3): File name cannot be repeated
			// This error may occur when attempting to rename temp object (newly uploaded)
			// right after the old one is removed.
			return true, nil
		} else if apiErr.Reason == "task_daily_create_limit_vip" {
			// "task_daily_create_limit_vip" (11): Sorry, you have submitted too many tasks and have exceeded the current processing capacity, please try again tomorrow
			return false, fserrors.FatalError(err)
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

	root := parsePath(path)

	client, _, err := oauthutil.NewClient(ctx, name, m, oauthConfig)
	if err != nil {
		return nil, fmt.Errorf("pikpak: failed when making oauth client: %w", err)
	}

	f := &Fs{
		name:    name,
		root:    root,
		opt:     *opt,
		m:       m,
		tokenMu: new(sync.Mutex),
		client:  client,
		srv:     rest.NewClient(client).SetRoot(rootURL).SetErrorHandler(errorHandler),
		pacer:   fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
	}
	f.features = (&fs.Features{
		ReadMimeType:            true, // can read the mime type of objects
		CanHaveEmptyDirectories: true, // can have empty directories
	}).Fill(ctx, f)

	// Check if current token is valid and re-authorize if necessary.
	// It is somehow redundant since it is covered in shouldRetry().
	// However it is a preemptive measure to avoid possible retries while validating a token.
	if token, err := oauthutil.GetToken(name, m); err == nil && !token.Valid() {
		fs.Debugf(f, "Token is invalid. Trying to get a new one using username/password...")
		if err := f.doAuthorize(ctx); err != nil {
			return nil, fserrors.FatalError(err)
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
			pathIDOut = item.Id
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
	if parentId := parentIdForRequest(dirID); parentId != "" {
		params.Set("parent_id", parentId)
	}

	// Construct filter string
	filters := &api.Filters{}
	filters.Set("Phase", "eq", api.PhaseTypeComplete)
	filters.Set("Trashed", "eq", trashed)
	filters.Set("Kind", "eq", kind)
	if *filters != (api.Filters{}) {
		if filterStr, err := json.Marshal(filters); err == nil {
			params.Set("filters", string(filterStr))
		}
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
			resp, err = f.srv.CallJSON(ctx, &opts, nil, &info)
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
		f.dirCache.Put(remote, item.Id)
		d := fs.NewDir(remote, time.Time(item.ModifiedTime)).SetID(item.Id)
		if item.ParentId == "" {
			d.SetParentID("root")
		} else {
			d.SetParentID(item.ParentId)
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
		ParentId: parentIdForRequest(pathID),
	}
	info, err := f.requestNewFile(ctx, &req)
	if err != nil {
		return "", err
	}
	return info.File.Id, nil
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
		return nil, fmt.Errorf("failed to get drive#quota: %w", err)
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
	// TODO: can be improved further; expiration days, passcode, and ...
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
	req := api.RequestShare{
		FileIds:        []string{id},
		ShareTo:        "publiclink",
		ExpirationDays: -1,
		PassCodeOption: "NOT_REQUIRED",
	}
	info, err := f.requestShare(ctx, &req)
	if err != nil {
		return "", err
	}
	return info.ShareUrl, err
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
		Ids: IDs,
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

	var trashedFiles = false
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
		Method:     "PATCH",
		Path:       "/drive/v1/files/trash:empty",
		NoResponse: true, // Only returns `{"task_id":""}
	}
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.Call(ctx, &opts)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("couldn't empty trash: %w", err)
	}
	return nil
}

// Move the object
func (f *Fs) moveObjects(ctx context.Context, IDs []string, dirID string) (err error) {
	if len(IDs) == 0 {
		return nil
	}
	req := api.RequestBatch{
		Ids: IDs,
		To:  &map[string]string{"parent_id": parentIdForRequest(dirID)},
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
		size:    size,
		modTime: modTime,
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

	// Create temporary object
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
	dstObj.id = srcObj.id

	var info *api.File
	if srcLeaf != dstLeaf {
		// Rename
		info, err = f.renameObject(ctx, srcObj.id, dstLeaf)
		if err != nil {
			return nil, fmt.Errorf("move: couldn't rename moved file: %w", err)
		}
	} else {
		// Update info
		info, err = f.getFile(ctx, dstObj.id)
		if err != nil {
			return nil, fmt.Errorf("move: couldn't update moved file: %w", err)
		}
	}
	return dstObj, dstObj.setMetaData(info)
}

// copy objects
func (f *Fs) copyObjects(ctx context.Context, IDs []string, dirID string) (err error) {
	if len(IDs) == 0 {
		return nil
	}
	req := api.RequestBatch{
		Ids: IDs,
		To:  &map[string]string{"parent_id": parentIdForRequest(dirID)},
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

	// Create temporary object
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
		err = dstObj.setMetaData(info)
		if err != nil {
			return nil, err
		}
	} else {
		// Update info
		err = dstObj.readMetaData(ctx)
		if err != nil {
			return nil, fmt.Errorf("copy: couldn't locate copied file: %w", err)
		}
	}
	return dstObj, nil
}

// MergeDirs merges the contents of all the directories passed
// in into the first one and rmdirs the other directories.
func (f *Fs) MergeDirs(ctx context.Context, dirs []fs.Directory) (err error) {
	if len(dirs) < 2 {
		return nil
	}
	dstDir := dirs[0]
	for _, srcDir := range dirs[1:] {
		// list the objects that are NOT marked as trashed
		//
		// Moving objects in recycler throws following error:
		// file_move_from_recycle_bin (9): "Moving in recycler is not allowed"
		var IDs []string
		_, err = f.listAll(ctx, srcDir.ID(), "", "false", func(item *api.File) bool {
			fs.Infof(srcDir, "listing for merging %q", item.Name)
			IDs = append(IDs, item.Id)
			// API doesn't allow to move a large number of objects at once, so doing it in chunked
			if len(IDs) >= api.ListLimit {
				if err = f.moveObjects(ctx, IDs, dstDir.ID()); err != nil {
					return true
				} else {
					IDs = nil
				}
			}
			return false
		})
		if err != nil {
			return fmt.Errorf("MergeDirs list failed on %v: %w", srcDir, err)
		}
		// move them into place
		if err = f.moveObjects(ctx, IDs, dstDir.ID()); err != nil {
			return fmt.Errorf("MergeDirs move failed in %v: %w", srcDir, err)
		}
	}

	// rmdir (into trash) the now empty source directory
	var IDs []string
	for _, srcDir := range dirs[1:] {
		fs.Infof(srcDir, "removing empty directory")
		IDs = append(IDs, srcDir.ID())
		// API doesn't allow to delete a large number of objects at once, so doing it in chunked
		if len(IDs) >= api.ListLimit {
			if err = f.deleteObjects(ctx, IDs, true); err != nil {
				return err
			} else {
				IDs = nil
			}
		}
	}
	if err := f.deleteObjects(ctx, IDs, true); err != nil {
		return fmt.Errorf("MergeDirs failed to rmdir: %w", err)
	}
	return nil
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
		RootURL:          form.Url,
		Body:             formReader,
		ContentType:      contentType,
		ContentLength:    &contentLength,
		Options:          options,
		TransferEncoding: []string{"identity"},
		NoResponse:       true,
	}

	var resp *http.Response
	err = f.pacer.CallNoRetry(func() (bool, error) {
		resp, err = f.srv.Call(ctx, &opts)
		return f.shouldRetry(ctx, resp, err)
	})
	return
}

func (f *Fs) uploadByResumable(ctx context.Context, in io.Reader, resumable *api.Resumable, options ...fs.OpenOption) (err error) {
	p := resumable.Params
	endpoint := strings.Join(strings.Split(p.Endpoint, ".")[1:], ".") // "mypikpak.com"

	cfg := &aws.Config{
		Credentials: credentials.NewStaticCredentials(p.AccessKeyId, p.AccessKeySecret, p.SecurityToken),
		Region:      aws.String("pikpak"),
		Endpoint:    &endpoint,
	}
	sess, err := session.NewSession(cfg)
	if err != nil {
		return
	}

	uploader := s3manager.NewUploader(sess)
	// Upload input parameters
	uParams := &s3manager.UploadInput{
		Bucket: &p.Bucket,
		Key:    &p.Key,
		Body:   in,
	}
	// Perform upload with options different than the those in the Uploader.
	_, err = uploader.UploadWithContext(ctx, uParams, func(u *s3manager.Uploader) {
		// TODO can be user-configurable
		u.PartSize = 10 * 1024 * 1024 // 10MB part size
	})
	return
}

func (f *Fs) upload(ctx context.Context, in io.Reader, leaf, dirID string, size int64, options ...fs.OpenOption) (*api.File, error) {
	// Calculate sha1sum; grabbed from package jottacloud
	//
	// unwrap the accounting from the input, we use wrap to put it
	// back on after the buffering
	var wrap accounting.WrapFn
	in, wrap = accounting.UnWrap(in)
	var cleanup func()
	sha1Str, in, cleanup, err := readSHA1(in, size, int64(f.opt.HashMemoryThreshold))
	defer cleanup()
	if err != nil {
		return nil, fmt.Errorf("failed to calculate SHA1: %w", err)
	}
	// Wrap the accounting back onto the stream
	in = wrap(in)

	// determine upload type
	uploadType := api.UploadTypeResumable
	if size >= 0 && size < int64(5*fs.Mebi) {
		uploadType = api.UploadTypeForm
	}

	// request upload ticket to API
	req := api.RequestNewFile{
		Kind:       api.KindOfFile,
		Name:       f.opt.Enc.FromStandardName(leaf),
		ParentId:   parentIdForRequest(dirID),
		FolderType: "NORMAL",
		Size:       size,
		Hash:       strings.ToUpper(sha1Str),
		UploadType: uploadType,
	}
	if uploadType == api.UploadTypeResumable {
		req.Resumable = &map[string]string{"provider": "PROVIDER_ALIYUN"}
	}
	newfile, err := f.requestNewFile(ctx, &req)
	if err != nil {
		return nil, fmt.Errorf("failed to create a new file: %w", err)
	}
	if newfile.File == nil {
		return nil, fmt.Errorf("invalid response: %+v", newfile)
	} else if newfile.File.Phase == api.PhaseTypeComplete {
		// early return; in case of zero-byte objects
		return newfile.File, nil
	}

	if uploadType == api.UploadTypeForm && newfile.Form != nil {
		err = f.uploadByForm(ctx, in, req.Name, size, newfile.Form, options...)
	} else if uploadType == api.UploadTypeResumable && newfile.Resumable != nil {
		err = f.uploadByResumable(ctx, in, newfile.Resumable, options...)
	} else {
		return nil, fmt.Errorf("unable to proceed upload: %+v", newfile)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to upload: %w", err)
	}
	// refresh uploaded file info
	return f.getFile(ctx, newfile.File.Id)
}

// PutUnchecked the object into the container
//
// This will produce an error if the object already exists.
//
// Copy the reader in to the new object which is returned.
//
// The new object may have been created if an error is returned
func (f *Fs) PutUnchecked(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	remote := src.Remote()
	size := src.Size()

	// Create the directory for the object if it doesn't exist
	leaf, dirID, err := f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return nil, err
	}

	info, err := f.upload(ctx, in, leaf, dirID, size, options...)
	if err != nil {
		return nil, err
	}
	return f.newObjectWithInfo(ctx, remote, info)
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
		return f.PutUnchecked(ctx, in, src)
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
	return map[string]string{
		"Id":                user.Sub,
		"Username":          user.Name,
		"Email":             user.Email,
		"PhoneNumber":       user.PhoneNumber,
		"Password":          user.Password,
		"Status":            user.Status,
		"CreatedAt":         time.Time(user.CreatedAt).String(),
		"PasswordUpdatedAt": time.Time(user.PasswordUpdatedAt).String(),
	}, nil
}

// ------------------------------------------------------------

// get an id of file or directory
func (f *Fs) getID(ctx context.Context, path string) (id string, err error) {
	path = strings.Trim(path, "/")
	id, err = f.dirCache.FindDir(ctx, path, false)
	if err != nil {
		o, err := f.NewObject(ctx, path)
		if err != nil {
			return "", err
		}
		id = o.(fs.IDer).ID()
	}
	return id, nil
}

// add offline download task for url
func (f *Fs) addUrl(ctx context.Context, url, path string) (*api.Task, error) {
	req := api.RequestNewTask{
		Kind:       api.KindOfFile,
		UploadType: "UPLOAD_TYPE_URL",
		Url: &api.Url{
			Url: url,
		},
		FolderType: "DOWNLOAD",
	}
	if parentId, err := f.dirCache.FindDir(ctx, path, false); err == nil {
		req.ParentId = parentIdForRequest(parentId)
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
						derr := f.deleteObjects(ctx, []string{item.Id}, f.opt.UseTrash)
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
	Name:  "getid",
	Short: "Get an ID of a file or directory",
	Long: `This command is to obtain an ID of a file or directory.

Usage:

    rclone backend getid pikpak:path {subpath}

The 'path' should point to a directory not a file. Use an extra argument
'subpath' to get an ID of a file located in 'pikpak:path'.
`,
}, {
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
	case "getid":
		path := ""
		if len(arg) > 0 {
			path = arg[0]
		}
		return f.getID(ctx, path)
	case "addurl":
		if len(arg) != 1 {
			return nil, errors.New("need exactly 1 argument")
		}
		return f.addUrl(ctx, arg[0], "")
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

// setMetaData sets the metadata from info
func (o *Object) setMetaData(info *api.File) (err error) {
	if info.Kind == api.KindOfFolder {
		return fs.ErrorIsDir
	}
	if info.Kind != api.KindOfFile {
		return fmt.Errorf("%q is %q: %w", o.remote, info.Kind, fs.ErrorNotAFile)
	}
	o.hasMetaData = true
	o.id = info.Id
	o.size = info.Size
	o.modTime = time.Time(info.ModifiedTime)
	o.mimeType = info.MimeType
	if info.ParentId == "" {
		o.parent = "root"
	} else {
		o.parent = info.ParentId
	}
	o.md5sum = info.Md5Checksum
	if info.Links != nil {
		o.link = info.Links.ApplicationOctetStream
	}
	return nil
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
	if o.md5sum == "" {
		return "", nil
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
		return nil, errors.New("can't download - no id")
	}
	info, err := o.fs.getFile(ctx, o.id)
	if err != nil {
		return nil, fmt.Errorf("can't fetch download link: %w", err)
	}
	if err = o.setMetaData(info); err != nil {
		return nil, err
	}
	if o.link == nil {
		if o.size == 0 {
			// zero-byte objects may have no download link
			return io.NopCloser(bytes.NewBuffer([]byte(nil))), nil
		}
		return nil, errors.New("can't download - no link to download")
	}
	return o.open(ctx, o.link.Url, options...)
}

// Update the object with the contents of the io.Reader, modTime and size
//
// If existing is set then it updates the object rather than creating a new one.
//
// The new object may have been created if an error is returned
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	size := src.Size()
	remote := o.Remote()

	// Create the directory for the object if it doesn't exist
	leaf, dirID, err := o.fs.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return err
	}

	// We have to fall back to upload + rename
	tempName := "rcloneTemp" + random.String(8)
	info, err := o.fs.upload(ctx, in, tempName, dirID, size, options...)
	if err != nil {
		return err
	}

	// upload was successful, need to delete old object before rename
	if err = o.Remove(ctx); err != nil {
		return fmt.Errorf("failed to remove old object: %w", err)
	}

	// rename also updates metadata
	if info, err = o.fs.renameObject(ctx, info.Id, leaf); err != nil {
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
	_ fs.PutUncheckeder  = (*Fs)(nil)
	_ fs.PublicLinker    = (*Fs)(nil)
	_ fs.MergeDirser     = (*Fs)(nil)
	_ fs.Abouter         = (*Fs)(nil)
	_ fs.UserInfoer      = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.MimeTyper       = (*Object)(nil)
	_ fs.IDer            = (*Object)(nil)
	_ fs.ParentIDer      = (*Object)(nil)
)
