// Package googlephotos provides an interface to Google Photos
package googlephotos

// FIXME Resumable uploads not implemented - rclone can't resume uploads in general

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/backend/googlephotos/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/dirtree"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/log"
	"github.com/rclone/rclone/lib/batcher"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/oauthutil"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var (
	errCantUpload  = errors.New("can't upload files here")
	errCantMkdir   = errors.New("can't make directories here")
	errCantRmdir   = errors.New("can't remove this directory")
	errAlbumDelete = errors.New("google photos API does not implement deleting albums")
	errRemove      = errors.New("google photos API only implements removing files from albums")
	errOwnAlbums   = errors.New("google photos API only allows uploading to albums rclone created")
)

const (
	rcloneClientID              = "202264815644-rt1o1c9evjaotbpbab10m83i8cnjk077.apps.googleusercontent.com"
	rcloneEncryptedClientSecret = "kLJLretPefBgrDHosdml_nlF64HZ9mUcO85X5rdjYBPP8ChA-jr3Ow"
	rootURL                     = "https://photoslibrary.googleapis.com/v1"
	listChunks                  = 100 // chunk size to read directory listings
	albumChunks                 = 50  // chunk size to read album listings
	minSleep                    = 10 * time.Millisecond
	scopeReadOnly               = "https://www.googleapis.com/auth/photoslibrary.readonly"
	scopeReadWrite              = "https://www.googleapis.com/auth/photoslibrary"
	scopeAccess                 = 2 // position of access scope in list
)

var (
	// Description of how to auth for this app
	oauthConfig = &oauth2.Config{
		Scopes: []string{
			"openid",
			"profile",
			scopeReadWrite, // this must be at position scopeAccess
		},
		Endpoint:     google.Endpoint,
		ClientID:     rcloneClientID,
		ClientSecret: obscure.MustReveal(rcloneEncryptedClientSecret),
		RedirectURL:  oauthutil.RedirectURL,
	}

	// Configure the batcher
	defaultBatcherOptions = batcher.Options{
		MaxBatchSize:          50,
		DefaultTimeoutSync:    1000 * time.Millisecond,
		DefaultTimeoutAsync:   10 * time.Second,
		DefaultBatchSizeAsync: 50,
	}
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "google photos",
		Prefix:      "gphotos",
		Description: "Google Photos",
		NewFs:       NewFs,
		Config: func(ctx context.Context, name string, m configmap.Mapper, config fs.ConfigIn) (*fs.ConfigOut, error) {
			// Parse config into Options struct
			opt := new(Options)
			err := configstruct.Set(m, opt)
			if err != nil {
				return nil, fmt.Errorf("couldn't parse config into struct: %w", err)
			}

			switch config.State {
			case "":
				// Fill in the scopes
				if opt.ReadOnly {
					oauthConfig.Scopes[scopeAccess] = scopeReadOnly
				} else {
					oauthConfig.Scopes[scopeAccess] = scopeReadWrite
				}
				return oauthutil.ConfigOut("warning", &oauthutil.Options{
					OAuth2Config: oauthConfig,
				})
			case "warning":
				// Warn the user as required by google photos integration
				return fs.ConfigConfirm("warning_done", true, "config_warning", `Warning

IMPORTANT: All media items uploaded to Google Photos with rclone
are stored in full resolution at original quality.  These uploads
will count towards storage in your Google Account.`)
			case "warning_done":
				return nil, nil
			}
			return nil, fmt.Errorf("unknown state %q", config.State)
		},
		Options: append(append(oauthutil.SharedOptions, []fs.Option{{
			Name:    "read_only",
			Default: false,
			Help: `Set to make the Google Photos backend read only.

If you choose read only then rclone will only request read only access
to your photos, otherwise rclone will request full access.`,
		}, {
			Name:    "read_size",
			Default: false,
			Help: `Set to read the size of media items.

Normally rclone does not read the size of media items since this takes
another transaction.  This isn't necessary for syncing.  However
rclone mount needs to know the size of files in advance of reading
them, so setting this flag when using rclone mount is recommended if
you want to read the media.`,
			Advanced: true,
		}, {
			Name:     "start_year",
			Default:  2000,
			Help:     `Year limits the photos to be downloaded to those which are uploaded after the given year.`,
			Advanced: true,
		}, {
			Name:    "include_archived",
			Default: false,
			Help: `Also view and download archived media.

By default, rclone does not request archived media. Thus, when syncing,
archived media is not visible in directory listings or transferred.

Note that media in albums is always visible and synced, no matter
their archive status.

With this flag, archived media are always visible in directory
listings and transferred.

Without this flag, archived media will not be visible in directory
listings and won't be transferred.`,
			Advanced: true,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			Default: (encoder.Base |
				encoder.EncodeCrLf |
				encoder.EncodeInvalidUtf8),
		}}...), defaultBatcherOptions.FsOptions("")...),
	})
}

// Options defines the configuration for this backend
type Options struct {
	ReadOnly        bool                 `config:"read_only"`
	ReadSize        bool                 `config:"read_size"`
	StartYear       int                  `config:"start_year"`
	IncludeArchived bool                 `config:"include_archived"`
	Enc             encoder.MultiEncoder `config:"encoding"`
	BatchMode       string               `config:"batch_mode"`
	BatchSize       int                  `config:"batch_size"`
	BatchTimeout    fs.Duration          `config:"batch_timeout"`
}

// Fs represents a remote storage server
type Fs struct {
	name       string                 // name of this remote
	root       string                 // the path we are working on if any
	opt        Options                // parsed options
	features   *fs.Features           // optional features
	unAuth     *rest.Client           // unauthenticated http client
	srv        *rest.Client           // the connection to the server
	ts         *oauthutil.TokenSource // token source for oauth2
	pacer      *fs.Pacer              // To pace the API calls
	startTime  time.Time              // time Fs was started - used for datestamps
	albumsMu   sync.Mutex             // protect albums (but not contents)
	albums     map[bool]*albums       // albums, shared or not
	uploadedMu sync.Mutex             // to protect the below
	uploaded   dirtree.DirTree        // record of uploaded items
	createMu   sync.Mutex             // held when creating albums to prevent dupes
	batcher    *batcher.Batcher[uploadedItem, *api.MediaItem]
}

// Object describes a storage object
//
// Will definitely have info but maybe not meta
type Object struct {
	fs       *Fs       // what this object is part of
	remote   string    // The remote path
	url      string    // download path
	id       string    // ID of this object
	bytes    int64     // Bytes in the object
	modTime  time.Time // Modified time of the object
	mimeType string
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
	return fmt.Sprintf("Google Photos path %q", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// dirTime returns the time to set a directory to
func (f *Fs) dirTime() time.Time {
	return f.startTime
}

// startYear returns the start year
func (f *Fs) startYear() int {
	return f.opt.StartYear
}

func (f *Fs) includeArchived() bool {
	return f.opt.IncludeArchived
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

// errorHandler parses a non 2xx error response into an error
func errorHandler(resp *http.Response) error {
	body, err := rest.ReadBody(resp)
	if err != nil {
		body = nil
	}
	// Google sends 404 messages as images so be prepared for that
	if strings.HasPrefix(resp.Header.Get("Content-Type"), "image/") {
		body = []byte("Image not found or broken")
	}
	var e = api.Error{
		Details: api.ErrorDetails{
			Code:    resp.StatusCode,
			Message: string(body),
			Status:  resp.Status,
		},
	}
	if body != nil {
		_ = json.Unmarshal(body, &e)
	}
	return &e
}

// NewFs constructs an Fs from the path, bucket:path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	baseClient := fshttp.NewClient(ctx)
	oAuthClient, ts, err := oauthutil.NewClientWithBaseClient(ctx, name, m, oauthConfig, baseClient)
	if err != nil {
		return nil, fmt.Errorf("failed to configure Box: %w", err)
	}

	root = strings.Trim(path.Clean(root), "/")
	if root == "." || root == "/" {
		root = ""
	}

	f := &Fs{
		name:      name,
		root:      root,
		opt:       *opt,
		unAuth:    rest.NewClient(baseClient),
		srv:       rest.NewClient(oAuthClient).SetRoot(rootURL),
		ts:        ts,
		pacer:     fs.NewPacer(ctx, pacer.NewGoogleDrive(pacer.MinSleep(minSleep))),
		startTime: time.Now(),
		albums:    map[bool]*albums{},
		uploaded:  dirtree.New(),
	}
	batcherOptions := defaultBatcherOptions
	batcherOptions.Mode = f.opt.BatchMode
	batcherOptions.Size = f.opt.BatchSize
	batcherOptions.Timeout = time.Duration(f.opt.BatchTimeout)
	f.batcher, err = batcher.New(ctx, f, f.commitBatch, batcherOptions)
	if err != nil {
		return nil, err
	}
	f.features = (&fs.Features{
		ReadMimeType: true,
	}).Fill(ctx, f)
	f.srv.SetErrorHandler(errorHandler)

	_, _, pattern := patterns.match(f.root, "", true)
	if pattern != nil && pattern.isFile {
		oldRoot := f.root
		var leaf string
		f.root, leaf = path.Split(f.root)
		f.root = strings.TrimRight(f.root, "/")
		_, err := f.NewObject(ctx, leaf)
		if err == nil {
			return f, fs.ErrorIsFile
		}
		f.root = oldRoot
	}
	return f, nil
}

// fetchEndpoint gets the openid endpoint named from the Google config
func (f *Fs) fetchEndpoint(ctx context.Context, name string) (endpoint string, err error) {
	// Get openID config without auth
	opts := rest.Opts{
		Method:  "GET",
		RootURL: "https://accounts.google.com/.well-known/openid-configuration",
	}
	var openIDconfig map[string]interface{}
	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.unAuth.CallJSON(ctx, &opts, nil, &openIDconfig)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return "", fmt.Errorf("couldn't read openID config: %w", err)
	}

	// Find userinfo endpoint
	endpoint, ok := openIDconfig[name].(string)
	if !ok {
		return "", fmt.Errorf("couldn't find %q from openID config", name)
	}

	return endpoint, nil
}

// UserInfo fetches info about the current user with oauth2
func (f *Fs) UserInfo(ctx context.Context) (userInfo map[string]string, err error) {
	endpoint, err := f.fetchEndpoint(ctx, "userinfo_endpoint")
	if err != nil {
		return nil, err
	}

	// Fetch the user info with auth
	opts := rest.Opts{
		Method:  "GET",
		RootURL: endpoint,
	}
	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, nil, &userInfo)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't read user info: %w", err)
	}
	return userInfo, nil
}

// Disconnect kills the token and refresh token
func (f *Fs) Disconnect(ctx context.Context) (err error) {
	endpoint, err := f.fetchEndpoint(ctx, "revocation_endpoint")
	if err != nil {
		return err
	}
	token, err := f.ts.Token()
	if err != nil {
		return err
	}

	// Revoke the token and the refresh token
	opts := rest.Opts{
		Method:  "POST",
		RootURL: endpoint,
		MultipartParams: url.Values{
			"token":           []string{token.AccessToken},
			"token_type_hint": []string{"access_token"},
		},
	}
	var res interface{}
	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, nil, &res)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("couldn't revoke token: %w", err)
	}
	fs.Infof(f, "res = %+v", res)
	return nil
}

// Return an Object from a path
//
// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(ctx context.Context, remote string, info *api.MediaItem) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	if info != nil {
		o.setMetaData(info)
	} else {
		err := o.readMetaData(ctx) // reads info and meta, returning an error
		if err != nil {
			return nil, err
		}
	}
	return o, nil
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	defer log.Trace(f, "remote=%q", remote)("")
	return f.newObjectWithInfo(ctx, remote, nil)
}

// addID adds the ID to name
func addID(name string, ID string) string {
	idStr := "{" + ID + "}"
	if name == "" {
		return idStr
	}
	return name + " " + idStr
}

// addFileID adds the ID to the fileName passed in
func addFileID(fileName string, ID string) string {
	ext := path.Ext(fileName)
	base := fileName[:len(fileName)-len(ext)]
	return addID(base, ID) + ext
}

var idRe = regexp.MustCompile(`\{([A-Za-z0-9_-]{55,})\}`)

// findID finds an ID in string if one is there or ""
func findID(name string) string {
	match := idRe.FindStringSubmatch(name)
	if match == nil {
		return ""
	}
	return match[1]
}

// list the albums into an internal cache
// FIXME cache invalidation
func (f *Fs) listAlbums(ctx context.Context, shared bool) (all *albums, err error) {
	f.albumsMu.Lock()
	defer f.albumsMu.Unlock()
	all, ok := f.albums[shared]
	if ok && all != nil {
		return all, nil
	}
	opts := rest.Opts{
		Method:     "GET",
		Path:       "/albums",
		Parameters: url.Values{},
	}
	if shared {
		opts.Path = "/sharedAlbums"
	}
	all = newAlbums()
	opts.Parameters.Set("pageSize", strconv.Itoa(albumChunks))
	lastID := ""
	for {
		var result api.ListAlbums
		var resp *http.Response
		err = f.pacer.Call(func() (bool, error) {
			resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
			return shouldRetry(ctx, resp, err)
		})
		if err != nil {
			return nil, fmt.Errorf("couldn't list albums: %w", err)
		}
		newAlbums := result.Albums
		if shared {
			newAlbums = result.SharedAlbums
		}
		if len(newAlbums) > 0 && newAlbums[0].ID == lastID {
			// skip first if ID duplicated from last page
			newAlbums = newAlbums[1:]
		}
		if len(newAlbums) > 0 {
			lastID = newAlbums[len(newAlbums)-1].ID
		}
		for i := range newAlbums {
			anAlbum := newAlbums[i]
			anAlbum.Title = f.opt.Enc.FromStandardPath(anAlbum.Title)
			all.add(&anAlbum)
		}
		if result.NextPageToken == "" {
			break
		}
		opts.Parameters.Set("pageToken", result.NextPageToken)
	}
	f.albums[shared] = all
	return all, nil
}

// listFn is called from list to handle an object.
type listFn func(remote string, object *api.MediaItem, isDirectory bool) error

// list the objects into the function supplied
//
// dir is the starting directory, "" for root
//
// Set recurse to read sub directories
func (f *Fs) list(ctx context.Context, filter api.SearchFilter, fn listFn) (err error) {
	opts := rest.Opts{
		Method: "POST",
		Path:   "/mediaItems:search",
	}
	filter.PageSize = listChunks
	filter.PageToken = ""
	if filter.AlbumID == "" { // album ID and filters cannot be set together, else error 400 INVALID_ARGUMENT
		if filter.Filters == nil {
			filter.Filters = &api.Filters{}
		}
		filter.Filters.IncludeArchivedMedia = &f.opt.IncludeArchived
	}
	lastID := ""
	for {
		var result api.MediaItems
		var resp *http.Response
		err = f.pacer.Call(func() (bool, error) {
			resp, err = f.srv.CallJSON(ctx, &opts, &filter, &result)
			return shouldRetry(ctx, resp, err)
		})
		if err != nil {
			return fmt.Errorf("couldn't list files: %w", err)
		}
		items := result.MediaItems
		if len(items) > 0 && items[0].ID == lastID {
			// skip first if ID duplicated from last page
			items = items[1:]
		}
		if len(items) > 0 {
			lastID = items[len(items)-1].ID
		}
		for i := range items {
			item := &result.MediaItems[i]
			remote := item.Filename
			remote = strings.ReplaceAll(remote, "/", "／")
			err = fn(remote, item, false)
			if err != nil {
				return err
			}
		}
		if result.NextPageToken == "" {
			break
		}
		filter.PageToken = result.NextPageToken
	}

	return nil
}

// Convert a list item into a DirEntry
func (f *Fs) itemToDirEntry(ctx context.Context, remote string, item *api.MediaItem, isDirectory bool) (fs.DirEntry, error) {
	if isDirectory {
		d := fs.NewDir(remote, f.dirTime())
		return d, nil
	}
	o := &Object{
		fs:     f,
		remote: remote,
	}
	o.setMetaData(item)
	return o, nil
}

// listDir lists a single directory
func (f *Fs) listDir(ctx context.Context, prefix string, filter api.SearchFilter) (entries fs.DirEntries, err error) {
	// List the objects
	err = f.list(ctx, filter, func(remote string, item *api.MediaItem, isDirectory bool) error {
		entry, err := f.itemToDirEntry(ctx, prefix+remote, item, isDirectory)
		if err != nil {
			return err
		}
		if entry != nil {
			entries = append(entries, entry)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// Dedupe the file names
	dupes := map[string]int{}
	for _, entry := range entries {
		o, ok := entry.(*Object)
		if ok {
			dupes[o.remote]++
		}
	}
	for _, entry := range entries {
		o, ok := entry.(*Object)
		if ok {
			duplicated := dupes[o.remote] > 1
			if duplicated || o.remote == "" {
				o.remote = addFileID(o.remote, o.id)
			}
		}
	}
	return entries, err
}

// listUploads lists a single directory from the uploads
func (f *Fs) listUploads(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	f.uploadedMu.Lock()
	entries, ok := f.uploaded[dir]
	f.uploadedMu.Unlock()
	if !ok && dir != "" {
		return nil, fs.ErrorDirNotFound
	}
	return entries, nil
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
	defer log.Trace(f, "dir=%q", dir)("err=%v", &err)
	match, prefix, pattern := patterns.match(f.root, dir, false)
	if pattern == nil || pattern.isFile {
		return nil, fs.ErrorDirNotFound
	}
	if pattern.toEntries != nil {
		return pattern.toEntries(ctx, f, prefix, match)
	}
	return nil, fs.ErrorDirNotFound
}

// Put the object into the bucket
//
// Copy the reader in to the new object which is returned.
//
// The new object may have been created if an error is returned
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	defer log.Trace(f, "src=%+v", src)("")
	// Temporary Object under construction
	o := &Object{
		fs:     f,
		remote: src.Remote(),
	}
	return o, o.Update(ctx, in, src, options...)
}

// createAlbum creates the album
func (f *Fs) createAlbum(ctx context.Context, albumTitle string) (album *api.Album, err error) {
	opts := rest.Opts{
		Method:     "POST",
		Path:       "/albums",
		Parameters: url.Values{},
	}
	var request = api.CreateAlbum{
		Album: &api.Album{
			Title: albumTitle,
		},
	}
	var result api.Album
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, request, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't create album: %w", err)
	}
	f.albums[false].add(&result)
	return &result, nil
}

// getOrCreateAlbum gets an existing album or creates a new one
//
// It does the creation with the lock held to avoid duplicates
func (f *Fs) getOrCreateAlbum(ctx context.Context, albumTitle string) (album *api.Album, err error) {
	f.createMu.Lock()
	defer f.createMu.Unlock()
	albums, err := f.listAlbums(ctx, false)
	if err != nil {
		return nil, err
	}
	album, ok := albums.get(albumTitle)
	if ok {
		return album, nil
	}
	return f.createAlbum(ctx, albumTitle)
}

// Mkdir creates the album if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) (err error) {
	defer log.Trace(f, "dir=%q", dir)("err=%v", &err)
	match, prefix, pattern := patterns.match(f.root, dir, false)
	if pattern == nil {
		return fs.ErrorDirNotFound
	}
	if !pattern.canMkdir {
		return errCantMkdir
	}
	if pattern.isUpload {
		f.uploadedMu.Lock()
		d := fs.NewDir(strings.Trim(prefix, "/"), f.dirTime())
		f.uploaded.AddEntry(d)
		f.uploadedMu.Unlock()
		return nil
	}
	albumTitle := match[1]
	_, err = f.getOrCreateAlbum(ctx, albumTitle)
	return err
}

// Rmdir deletes the bucket if the fs is at the root
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) (err error) {
	defer log.Trace(f, "dir=%q")("err=%v", &err)
	match, _, pattern := patterns.match(f.root, dir, false)
	if pattern == nil {
		return fs.ErrorDirNotFound
	}
	if !pattern.canMkdir {
		return errCantRmdir
	}
	if pattern.isUpload {
		f.uploadedMu.Lock()
		err = f.uploaded.Prune(map[string]bool{
			dir: true,
		})
		f.uploadedMu.Unlock()
		return err
	}
	albumTitle := match[1]
	allAlbums, err := f.listAlbums(ctx, false)
	if err != nil {
		return err
	}
	album, ok := allAlbums.get(albumTitle)
	if !ok {
		return fs.ErrorDirNotFound
	}
	_ = album
	return errAlbumDelete
}

// Precision returns the precision
func (f *Fs) Precision() time.Duration {
	return fs.ModTimeNotSupported
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.None)
}

// Shutdown the backend, closing any background tasks and any
// cached connections.
func (f *Fs) Shutdown(ctx context.Context) error {
	f.batcher.Shutdown()
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

// Hash returns the Md5sum of an object returning a lowercase hex string
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	defer log.Trace(o, "")("")
	if !o.fs.opt.ReadSize || o.bytes >= 0 {
		return o.bytes
	}
	ctx := context.TODO()
	err := o.readMetaData(ctx)
	if err != nil {
		fs.Debugf(o, "Size: Failed to read metadata: %v", err)
		return -1
	}
	var resp *http.Response
	opts := rest.Opts{
		Method:  "HEAD",
		RootURL: o.downloadURL(),
	}
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.Call(ctx, &opts)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		fs.Debugf(o, "Reading size failed: %v", err)
	} else {
		lengthStr := resp.Header.Get("Content-Length")
		length, err := strconv.ParseInt(lengthStr, 10, 64)
		if err != nil {
			fs.Debugf(o, "Reading size failed to parse Content_length %q: %v", lengthStr, err)
		} else {
			o.bytes = length
		}
	}
	return o.bytes
}

// setMetaData sets the fs data from a storage.Object
func (o *Object) setMetaData(info *api.MediaItem) {
	o.url = info.BaseURL
	o.id = info.ID
	o.bytes = -1 // FIXME
	o.mimeType = info.MimeType
	o.modTime = info.MediaMetadata.CreationTime
}

// readMetaData gets the metadata if it hasn't already been fetched
//
// it also sets the info
func (o *Object) readMetaData(ctx context.Context) (err error) {
	if !o.modTime.IsZero() && o.url != "" {
		return nil
	}
	dir, fileName := path.Split(o.remote)
	dir = strings.Trim(dir, "/")
	_, _, pattern := patterns.match(o.fs.root, o.remote, true)
	if pattern == nil {
		return fs.ErrorObjectNotFound
	}
	if !pattern.isFile {
		return fs.ErrorNotAFile
	}
	// If have ID fetch it directly
	if id := findID(fileName); id != "" {
		opts := rest.Opts{
			Method: "GET",
			Path:   "/mediaItems/" + id,
		}
		var item api.MediaItem
		var resp *http.Response
		err = o.fs.pacer.Call(func() (bool, error) {
			resp, err = o.fs.srv.CallJSON(ctx, &opts, nil, &item)
			return shouldRetry(ctx, resp, err)
		})
		if err != nil {
			return fmt.Errorf("couldn't get media item: %w", err)
		}
		o.setMetaData(&item)
		return nil
	}
	// Otherwise list the directory the file is in
	entries, err := o.fs.List(ctx, dir)
	if err != nil {
		if err == fs.ErrorDirNotFound {
			return fs.ErrorObjectNotFound
		}
		return err
	}
	// and find the file in the directory
	for _, entry := range entries {
		if entry.Remote() == o.remote {
			if newO, ok := entry.(*Object); ok {
				*o = *newO
				return nil
			}
		}
	}
	return fs.ErrorObjectNotFound
}

// ModTime returns the modification time of the object
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *Object) ModTime(ctx context.Context) time.Time {
	defer log.Trace(o, "")("")
	err := o.readMetaData(ctx)
	if err != nil {
		fs.Debugf(o, "ModTime: Failed to read metadata: %v", err)
		return time.Now()
	}
	return o.modTime
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) (err error) {
	return fs.ErrorCantSetModTime
}

// Storable returns a boolean as to whether this object is storable
func (o *Object) Storable() bool {
	return true
}

// downloadURL returns the URL for a full bytes download for the object
func (o *Object) downloadURL() string {
	url := o.url + "=d"
	if strings.HasPrefix(o.mimeType, "video/") {
		url += "v"
	}
	return url
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	defer log.Trace(o, "")("")
	err = o.readMetaData(ctx)
	if err != nil {
		fs.Debugf(o, "Open: Failed to read metadata: %v", err)
		return nil, err
	}
	var resp *http.Response
	opts := rest.Opts{
		Method:  "GET",
		RootURL: o.downloadURL(),
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

// input to the batcher
type uploadedItem struct {
	AlbumID     string // desired album
	UploadToken string // upload ID
}

// Commit a batch of items to albumID returning the errors in errors
func (f *Fs) commitBatchAlbumID(ctx context.Context, items []uploadedItem, results []*api.MediaItem, errors []error, albumID string) {
	// Create the media item from an UploadToken, optionally adding to an album
	opts := rest.Opts{
		Method: "POST",
		Path:   "/mediaItems:batchCreate",
	}
	var request = api.BatchCreateRequest{
		AlbumID: albumID,
	}
	itemsInBatch := 0
	for i := range items {
		if items[i].AlbumID == albumID {
			request.NewMediaItems = append(request.NewMediaItems, api.NewMediaItem{
				SimpleMediaItem: api.SimpleMediaItem{
					UploadToken: items[i].UploadToken,
				},
			})
			itemsInBatch++
		}
	}
	var result api.BatchCreateResponse
	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, request, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		err = fmt.Errorf("failed to create media item: %w", err)
	}
	if err == nil && len(result.NewMediaItemResults) != itemsInBatch {
		err = fmt.Errorf("bad response to BatchCreate expecting %d items but got %d", itemsInBatch, len(result.NewMediaItemResults))
	}
	j := 0
	for i := range items {
		if items[i].AlbumID == albumID {
			if err == nil {
				media := &result.NewMediaItemResults[j]
				if media.Status.Code != 0 {
					errors[i] = fmt.Errorf("upload failed: %s (%d)", media.Status.Message, media.Status.Code)
				} else {
					results[i] = &media.MediaItem
				}
			} else {
				errors[i] = err
			}
			j++
		}
	}
}

// Called by the batcher to commit a batch
func (f *Fs) commitBatch(ctx context.Context, items []uploadedItem, results []*api.MediaItem, errors []error) (err error) {
	// Discover all the AlbumIDs as we have to upload these separately
	//
	// Should maybe have one batcher per AlbumID
	albumIDs := map[string]struct{}{}
	for i := range items {
		albumIDs[items[i].AlbumID] = struct{}{}
	}

	// batch the albums
	for albumID := range albumIDs {
		// errors returned in errors
		f.commitBatchAlbumID(ctx, items, results, errors, albumID)
	}
	return nil
}

// Update the object with the contents of the io.Reader, modTime and size
//
// The new object may have been created if an error is returned
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	defer log.Trace(o, "src=%+v", src)("err=%v", &err)
	match, _, pattern := patterns.match(o.fs.root, o.remote, true)
	if pattern == nil || !pattern.isFile || !pattern.canUpload {
		return errCantUpload
	}
	var (
		albumID  string
		fileName string
	)
	if pattern.isUpload {
		fileName = match[1]
	} else {
		var albumTitle string
		albumTitle, fileName = match[1], match[2]

		album, err := o.fs.getOrCreateAlbum(ctx, albumTitle)
		if err != nil {
			return err
		}

		if !album.IsWriteable {
			return errOwnAlbums
		}

		albumID = album.ID
	}

	// Upload the media item in exchange for an UploadToken
	opts := rest.Opts{
		Method:  "POST",
		Path:    "/uploads",
		Options: options,
		ExtraHeaders: map[string]string{
			"X-Goog-Upload-File-Name": fileName,
			"X-Goog-Upload-Protocol":  "raw",
		},
		Body: in,
	}
	var token []byte
	var resp *http.Response
	err = o.fs.pacer.CallNoRetry(func() (bool, error) {
		resp, err = o.fs.srv.Call(ctx, &opts)
		if err != nil {
			return shouldRetry(ctx, resp, err)
		}
		token, err = rest.ReadBody(resp)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("couldn't upload file: %w", err)
	}
	uploadToken := strings.TrimSpace(string(token))
	if uploadToken == "" {
		return errors.New("empty upload token")
	}

	uploaded := uploadedItem{
		AlbumID:     albumID,
		UploadToken: uploadToken,
	}

	// Save the upload into an album
	var info *api.MediaItem
	if o.fs.batcher.Batching() {
		info, err = o.fs.batcher.Commit(ctx, o.remote, uploaded)
	} else {
		errors := make([]error, 1)
		results := make([]*api.MediaItem, 1)
		err = o.fs.commitBatch(ctx, []uploadedItem{uploaded}, results, errors)
		if err != nil {
			err = errors[0]
			info = results[0]
		}
	}
	if err != nil {
		return fmt.Errorf("failed to commit batch: %w", err)
	}

	o.setMetaData(info)

	// Add upload to internal storage
	if pattern.isUpload {
		o.fs.uploadedMu.Lock()
		o.fs.uploaded.AddEntry(o)
		o.fs.uploadedMu.Unlock()
	}
	return nil
}

// Remove an object
func (o *Object) Remove(ctx context.Context) (err error) {
	match, _, pattern := patterns.match(o.fs.root, o.remote, true)
	if pattern == nil || !pattern.isFile || !pattern.canUpload || pattern.isUpload {
		return errRemove
	}
	albumTitle, fileName := match[1], match[2]
	album, ok := o.fs.albums[false].get(albumTitle)
	if !ok {
		return fmt.Errorf("couldn't file %q in album %q for delete", fileName, albumTitle)
	}
	opts := rest.Opts{
		Method:     "POST",
		Path:       "/albums/" + album.ID + ":batchRemoveMediaItems",
		NoResponse: true,
	}
	var request = api.BatchRemoveItems{
		MediaItemIds: []string{o.id},
	}
	var resp *http.Response
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.CallJSON(ctx, &opts, &request, nil)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("couldn't delete item from album: %w", err)
	}
	return nil
}

// MimeType of an Object if known, "" otherwise
func (o *Object) MimeType(ctx context.Context) string {
	return o.mimeType
}

// ID of an Object if known, "" otherwise
func (o *Object) ID() string {
	return o.id
}

// Check the interfaces are satisfied
var (
	_ fs.Fs           = &Fs{}
	_ fs.UserInfoer   = &Fs{}
	_ fs.Disconnecter = &Fs{}
	_ fs.Object       = &Object{}
	_ fs.MimeTyper    = &Object{}
	_ fs.IDer         = &Object{}
)
