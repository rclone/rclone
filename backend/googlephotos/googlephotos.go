// Package googlephotos provides an interface to Google Photos
package googlephotos

// FIXME Resumable uploads not implemented - rclone can't resume uploads in general

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	golog "log"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ncw/rclone/backend/googlephotos/api"
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/config"
	"github.com/ncw/rclone/fs/config/configmap"
	"github.com/ncw/rclone/fs/config/configstruct"
	"github.com/ncw/rclone/fs/config/obscure"
	"github.com/ncw/rclone/fs/dirtree"
	"github.com/ncw/rclone/fs/fserrors"
	"github.com/ncw/rclone/fs/hash"
	"github.com/ncw/rclone/fs/log"
	"github.com/ncw/rclone/lib/oauthutil"
	"github.com/ncw/rclone/lib/pacer"
	"github.com/ncw/rclone/lib/rest"
	"github.com/pkg/errors"
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
)

var (
	// Description of how to auth for this app
	oauthConfig = &oauth2.Config{
		Scopes: []string{
			scopeReadWrite,
		},
		Endpoint:     google.Endpoint,
		ClientID:     rcloneClientID,
		ClientSecret: obscure.MustReveal(rcloneEncryptedClientSecret),
		RedirectURL:  oauthutil.TitleBarRedirectURL,
	}
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "google photos",
		Prefix:      "gphotos",
		Description: "Google Photos",
		NewFs:       NewFs,
		Config: func(name string, m configmap.Mapper) {
			// Parse config into Options struct
			opt := new(Options)
			err := configstruct.Set(m, opt)
			if err != nil {
				fs.Errorf(nil, "Couldn't parse config into struct: %v", err)
				return
			}

			// Fill in the scopes
			if opt.ReadOnly {
				oauthConfig.Scopes[0] = scopeReadOnly
			} else {
				oauthConfig.Scopes[0] = scopeReadWrite
			}

			// Do the oauth
			err = oauthutil.Config("google photos", name, m, oauthConfig)
			if err != nil {
				golog.Fatalf("Failed to configure token: %v", err)
			}

			// Warn the user
			fmt.Print(`
*** IMPORTANT: All media items uploaded to Google Photos with rclone
*** are stored in full resolution at original quality.  These uploads
*** will count towards storage in your Google Account.

`)

		},
		Options: []fs.Option{{
			Name: config.ConfigClientID,
			Help: "Google Application Client Id\nLeave blank normally.",
		}, {
			Name: config.ConfigClientSecret,
			Help: "Google Application Client Secret\nLeave blank normally.",
		}, {
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
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	ReadOnly bool `config:"read_only"`
	ReadSize bool `config:"read_size"`
}

// Fs represents a remote storage server
type Fs struct {
	name       string           // name of this remote
	root       string           // the path we are working on if any
	opt        Options          // parsed options
	features   *fs.Features     // optional features
	srv        *rest.Client     // the connection to the one drive server
	pacer      *fs.Pacer        // To pace the API calls
	startTime  time.Time        // time Fs was started - used for datestamps
	albums     map[bool]*albums // albums, shared or not
	uploadedMu sync.Mutex       // to protect the below
	uploaded   dirtree.DirTree  // record of uploaded items
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
func shouldRetry(resp *http.Response, err error) (bool, error) {
	return fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// errorHandler parses a non 2xx error response into an error
func errorHandler(resp *http.Response) error {
	body, err := rest.ReadBody(resp)
	if err != nil {
		body = nil
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
func NewFs(name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	oAuthClient, _, err := oauthutil.NewClient(name, m, oauthConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to configure Box")
	}

	root = strings.Trim(path.Clean(root), "/")
	if root == "." || root == "/" {
		root = ""
	}
	f := &Fs{
		name:      name,
		root:      root,
		opt:       *opt,
		srv:       rest.NewClient(oAuthClient).SetRoot(rootURL),
		pacer:     fs.NewPacer(pacer.NewGoogleDrive(pacer.MinSleep(minSleep))),
		startTime: time.Now(),
		albums:    map[bool]*albums{},
		uploaded:  dirtree.New(),
	}
	f.features = (&fs.Features{
		ReadMimeType: true,
	}).Fill(f)
	f.srv.SetErrorHandler(errorHandler)

	_, _, pattern := patterns.match(f.root, "", true)
	if pattern != nil && pattern.isFile {
		oldRoot := f.root
		var leaf string
		f.root, leaf = path.Split(f.root)
		f.root = strings.TrimRight(f.root, "/")
		_, err := f.NewObject(context.TODO(), leaf)
		if err == nil {
			return f, fs.ErrorIsFile
		}
		f.root = oldRoot
	}
	return f, nil
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
func (f *Fs) listAlbums(shared bool) (all *albums, err error) {
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
			resp, err = f.srv.CallJSON(&opts, nil, &result)
			return shouldRetry(resp, err)
		})
		if err != nil {
			return nil, errors.Wrap(err, "couldn't list albums")
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
			all.add(&newAlbums[i])
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
func (f *Fs) list(filter api.SearchFilter, fn listFn) (err error) {
	opts := rest.Opts{
		Method: "POST",
		Path:   "/mediaItems:search",
	}
	filter.PageSize = listChunks
	filter.PageToken = ""
	lastID := ""
	for {
		var result api.MediaItems
		var resp *http.Response
		err = f.pacer.Call(func() (bool, error) {
			resp, err = f.srv.CallJSON(&opts, &filter, &result)
			return shouldRetry(resp, err)
		})
		if err != nil {
			return errors.Wrap(err, "couldn't list files")
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
			remote = strings.Replace(remote, "/", "／", -1)
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
	err = f.list(filter, func(remote string, item *api.MediaItem, isDirectory bool) error {
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
// Copy the reader in to the new object which is returned
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
func (f *Fs) createAlbum(ctx context.Context, albumName string) (album *api.Album, err error) {
	opts := rest.Opts{
		Method:     "POST",
		Path:       "/albums",
		Parameters: url.Values{},
	}
	var request = api.CreateAlbum{
		Album: &api.Album{
			Title: albumName,
		},
	}
	var result api.Album
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(&opts, request, &result)
		return shouldRetry(resp, err)
	})
	if err != nil {
		return nil, errors.Wrap(err, "couldn't create album")
	}
	f.albums[false].add(&result)
	return &result, nil
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
	albumName := match[1]
	allAlbums, err := f.listAlbums(false)
	if err != nil {
		return err
	}
	_, ok := allAlbums.get(albumName)
	if ok {
		return nil
	}
	_, err = f.createAlbum(ctx, albumName)
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
	albumName := match[1]
	allAlbums, err := f.listAlbums(false)
	if err != nil {
		return err
	}
	album, ok := allAlbums.get(albumName)
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
		resp, err = o.fs.srv.Call(&opts)
		return shouldRetry(resp, err)
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
			resp, err = o.fs.srv.CallJSON(&opts, nil, &item)
			return shouldRetry(resp, err)
		})
		if err != nil {
			return errors.Wrap(err, "couldn't get media item")
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
		resp, err = o.fs.srv.Call(&opts)
		return shouldRetry(resp, err)
	})
	if err != nil {
		return nil, err
	}
	return resp.Body, err
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
		var albumName string
		albumName, fileName = match[1], match[2]

		// Create album if not found
		album, ok := o.fs.albums[false].get(albumName)
		if !ok {
			album, err = o.fs.createAlbum(ctx, albumName)
			if err != nil {
				return err
			}
		}

		// Check we can write to this album
		if !album.IsWriteable {
			return errOwnAlbums
		}

		albumID = album.ID
	}

	// Upload the media item in exchange for an UploadToken
	opts := rest.Opts{
		Method: "POST",
		Path:   "/uploads",
		ExtraHeaders: map[string]string{
			"X-Goog-Upload-File-Name": fileName,
			"X-Goog-Upload-Protocol":  "raw",
		},
		Body: in,
	}
	var token []byte
	var resp *http.Response
	err = o.fs.pacer.CallNoRetry(func() (bool, error) {
		resp, err = o.fs.srv.Call(&opts)
		if err != nil {
			_ = resp.Body.Close()
			return shouldRetry(resp, err)
		}
		token, err = rest.ReadBody(resp)
		return shouldRetry(resp, err)
	})
	if err != nil {
		return errors.Wrap(err, "couldn't upload file")
	}
	uploadToken := strings.TrimSpace(string(token))
	if uploadToken == "" {
		return errors.New("empty upload token")
	}

	// Create the media item from an UploadToken, optionally adding to an album
	opts = rest.Opts{
		Method: "POST",
		Path:   "/mediaItems:batchCreate",
	}
	var request = api.BatchCreateRequest{
		AlbumID: albumID,
		NewMediaItems: []api.NewMediaItem{
			{
				SimpleMediaItem: api.SimpleMediaItem{
					UploadToken: uploadToken,
				},
			},
		},
	}
	var result api.BatchCreateResponse
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.CallJSON(&opts, request, &result)
		return shouldRetry(resp, err)
	})
	if err != nil {
		return errors.Wrap(err, "failed to create media item")
	}
	if len(result.NewMediaItemResults) != 1 {
		return errors.New("bad response to BatchCreate wrong number of items")
	}
	mediaItemResult := result.NewMediaItemResults[0]
	if mediaItemResult.Status.Code != 0 {
		return errors.Errorf("upload failed: %s (%d)", mediaItemResult.Status.Message, mediaItemResult.Status.Code)
	}
	o.setMetaData(&mediaItemResult.MediaItem)

	// Add upload to internal storage
	if pattern.isUpload {
		o.fs.uploaded.AddEntry(o)
	}
	return nil
}

// Remove an object
func (o *Object) Remove(ctx context.Context) (err error) {
	match, _, pattern := patterns.match(o.fs.root, o.remote, true)
	if pattern == nil || !pattern.isFile || !pattern.canUpload || pattern.isUpload {
		return errRemove
	}
	albumName, fileName := match[1], match[2]
	album, ok := o.fs.albums[false].get(albumName)
	if !ok {
		return errors.Errorf("couldn't file %q in album %q for delete", fileName, albumName)
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
		resp, err = o.fs.srv.CallJSON(&opts, &request, nil)
		return shouldRetry(resp, err)
	})
	if err != nil {
		return errors.Wrap(err, "couldn't delete item from album")
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
	_ fs.Fs        = &Fs{}
	_ fs.Object    = &Object{}
	_ fs.MimeTyper = &Object{}
	_ fs.IDer      = &Object{}
)
