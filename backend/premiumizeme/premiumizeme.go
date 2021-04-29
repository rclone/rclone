// Package premiumizeme provides an interface to the premiumize.me
// object storage system.
package premiumizeme

/*
Run of rclone info
stringNeedsEscaping = []rune{
	0x00, 0x0A, 0x0D, 0x22, 0x2F, 0x5C, 0xBF, 0xFE
	0x00, 0x0A, 0x0D, '"',  '/',  '\\', 0xBF, 0xFE
}
maxFileLength = 255
canWriteUnnormalized = true
canReadUnnormalized   = true
canReadRenormalized   = false
canStream = false
*/

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/backend/premiumizeme/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/oauthutil"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/random"
	"github.com/rclone/rclone/lib/rest"
	"golang.org/x/oauth2"
)

const (
	rcloneClientID              = "658922194"
	rcloneEncryptedClientSecret = "B5YIvQoRIhcpAYs8HYeyjb9gK-ftmZEbqdh_gNfc4RgO9Q"
	minSleep                    = 10 * time.Millisecond
	maxSleep                    = 2 * time.Second
	decayConstant               = 2   // bigger for slower decay, exponential
	rootID                      = "0" // ID of root folder is always this
	rootURL                     = "https://www.premiumize.me/api"
)

// Globals
var (
	// Description of how to auth for this app
	oauthConfig = &oauth2.Config{
		Scopes: nil,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://www.premiumize.me/authorize",
			TokenURL: "https://www.premiumize.me/token",
		},
		ClientID:     rcloneClientID,
		ClientSecret: obscure.MustReveal(rcloneEncryptedClientSecret),
		RedirectURL:  oauthutil.RedirectURL,
	}
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "premiumizeme",
		Description: "premiumize.me",
		NewFs:       NewFs,
		Config: func(ctx context.Context, name string, m configmap.Mapper, config fs.ConfigIn) (*fs.ConfigOut, error) {
			return oauthutil.ConfigOut("", &oauthutil.Options{
				OAuth2Config: oauthConfig,
			})
		},
		Options: []fs.Option{{
			Name: "api_key",
			Help: `API Key.

This is not normally used - use oauth instead.
`,
			Hide:    fs.OptionHideBoth,
			Default: "",
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			// Encode invalid UTF-8 bytes as json doesn't handle them properly.
			Default: (encoder.Display |
				encoder.EncodeBackSlash |
				encoder.EncodeDoubleQuote |
				encoder.EncodeInvalidUtf8),
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	APIKey string               `config:"api_key"`
	Enc    encoder.MultiEncoder `config:"encoding"`
}

// Fs represents a remote cloud storage system
type Fs struct {
	name         string             // name of this remote
	root         string             // the path we are working on
	opt          Options            // parsed options
	features     *fs.Features       // optional features
	srv          *rest.Client       // the connection to the server
	dirCache     *dircache.DirCache // Map of directory path to directory id
	pacer        *fs.Pacer          // pacer for API calls
	tokenRenewer *oauthutil.Renew   // renew the token on expiry
}

// Object describes a file
type Object struct {
	fs          *Fs       // what this object is part of
	remote      string    // The remote path
	hasMetaData bool      // metadata is present and correct
	size        int64     // size of the object
	modTime     time.Time // modification time of the object
	id          string    // ID of the object
	parentID    string    // ID of parent directory
	mimeType    string    // Mime type of object
	url         string    // URL to download file
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
	return fmt.Sprintf("premiumize.me root '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// parsePath parses a premiumize.me 'url'
func parsePath(path string) (root string) {
	root = strings.Trim(path, "/")
	return
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

// readMetaDataForPath reads the metadata from the path
func (f *Fs) readMetaDataForPath(ctx context.Context, path string, directoriesOnly bool, filesOnly bool) (info *api.Item, err error) {
	// defer fs.Trace(f, "path=%q", path)("info=%+v, err=%v", &info, &err)
	leaf, directoryID, err := f.dirCache.FindPath(ctx, path, false)
	if err != nil {
		if err == fs.ErrorDirNotFound {
			return nil, fs.ErrorObjectNotFound
		}
		return nil, err
	}

	lcLeaf := strings.ToLower(leaf)
	found, err := f.listAll(ctx, directoryID, directoriesOnly, filesOnly, func(item *api.Item) bool {
		if strings.ToLower(item.Name) == lcLeaf {
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

// errorHandler parses a non 2xx error response into an error
func errorHandler(resp *http.Response) error {
	body, err := rest.ReadBody(resp)
	if err != nil {
		body = nil
	}
	var e = api.Response{
		Message: string(body),
		Status:  fmt.Sprintf("%s (%d)", resp.Status, resp.StatusCode),
	}
	if body != nil {
		_ = json.Unmarshal(body, &e)
	}
	return &e
}

// Return a url.Values with the api key in
func (f *Fs) baseParams() url.Values {
	params := url.Values{}
	if f.opt.APIKey != "" {
		params.Add("apikey", f.opt.APIKey)
	}
	return params
}

// NewFs constructs an Fs from the path, container:path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	root = parsePath(root)

	var client *http.Client
	var ts *oauthutil.TokenSource
	if opt.APIKey == "" {
		client, ts, err = oauthutil.NewClient(ctx, name, m, oauthConfig)
		if err != nil {
			return nil, errors.Wrap(err, "failed to configure premiumize.me")
		}
	} else {
		client = fshttp.NewClient(ctx)
	}

	f := &Fs{
		name:  name,
		root:  root,
		opt:   *opt,
		srv:   rest.NewClient(client).SetRoot(rootURL),
		pacer: fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
	}
	f.features = (&fs.Features{
		CaseInsensitive:         true,
		CanHaveEmptyDirectories: true,
		ReadMimeType:            true,
	}).Fill(ctx, f)
	f.srv.SetErrorHandler(errorHandler)

	// Renew the token in the background
	if ts != nil {
		f.tokenRenewer = oauthutil.NewRenew(f.String(), ts, func() error {
			_, err := f.About(ctx)
			return err
		})
	}

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

// Return an Object from a path
//
// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(ctx context.Context, remote string, info *api.Item) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	var err error
	if info != nil {
		// Set info
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
	found, err = f.listAll(ctx, pathID, true, false, func(item *api.Item) bool {
		if strings.EqualFold(item.Name, leaf) {
			pathIDOut = item.ID
			return true
		}
		return false
	})
	return pathIDOut, found, err
}

// CreateDir makes a directory with pathID as parent and name leaf
func (f *Fs) CreateDir(ctx context.Context, pathID, leaf string) (newID string, err error) {
	// fs.Debugf(f, "CreateDir(%q, %q)\n", pathID, leaf)
	var resp *http.Response
	var info api.FolderCreateResponse
	opts := rest.Opts{
		Method:     "POST",
		Path:       "/folder/create",
		Parameters: f.baseParams(),
		MultipartParams: url.Values{
			"name":      {f.opt.Enc.FromStandardName(leaf)},
			"parent_id": {pathID},
		},
	}
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		//fmt.Printf("...Error %v\n", err)
		return "", errors.Wrap(err, "CreateDir http")
	}
	if err = info.AsErr(); err != nil {
		return "", errors.Wrap(err, "CreateDir")
	}
	// fmt.Printf("...Id %q\n", *info.Id)
	return info.ID, nil
}

// list the objects into the function supplied
//
// If directories is set it only sends directories
// User function to process a File item from listAll
//
// Should return true to finish processing
type listAllFn func(*api.Item) bool

// Lists the directory required calling the user function on each item found
//
// If the user fn ever returns true then it early exits with found = true
func (f *Fs) listAll(ctx context.Context, dirID string, directoriesOnly bool, filesOnly bool, fn listAllFn) (found bool, err error) {
	opts := rest.Opts{
		Method:     "GET",
		Path:       "/folder/list",
		Parameters: f.baseParams(),
	}
	opts.Parameters.Set("id", dirID)
	opts.Parameters.Set("includebreadcrumbs", "false")

	var result api.FolderListResponse
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return found, errors.Wrap(err, "couldn't list files")
	}
	if err = result.AsErr(); err != nil {
		return found, errors.Wrap(err, "error while listing")
	}
	for i := range result.Content {
		item := &result.Content[i]
		if item.Type == api.ItemTypeFolder {
			if filesOnly {
				continue
			}
		} else if item.Type == api.ItemTypeFile {
			if directoriesOnly {
				continue
			}
		} else {
			fs.Debugf(f, "Ignoring %q - unknown type %q", item.Name, item.Type)
			continue
		}
		item.Name = f.opt.Enc.ToStandardName(item.Name)
		if fn(item) {
			found = true
			break
		}
	}

	return
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
	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return nil, err
	}
	var iErr error
	_, err = f.listAll(ctx, directoryID, false, false, func(info *api.Item) bool {
		remote := path.Join(dir, info.Name)
		if info.Type == api.ItemTypeFolder {
			// cache the directory ID for later lookups
			f.dirCache.Put(remote, info.ID)
			d := fs.NewDir(remote, time.Unix(info.CreatedAt, 0)).SetID(info.ID)
			entries = append(entries, d)
		} else if info.Type == api.ItemTypeFile {
			o, err := f.newObjectWithInfo(ctx, remote, info)
			if err != nil {
				iErr = err
				return true
			}
			entries = append(entries, o)
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

// Creates from the parameters passed in a half finished Object which
// must have setMetaData called on it
//
// Returns the object, leaf, directoryID and error
//
// Used to create new objects
func (f *Fs) createObject(ctx context.Context, remote string, modTime time.Time, size int64) (o *Object, leaf string, directoryID string, err error) {
	// Create the directory for the object if it doesn't exist
	leaf, directoryID, err = f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return
	}
	// Temporary Object under construction
	o = &Object{
		fs:     f,
		remote: remote,
	}
	return o, leaf, directoryID, nil
}

// Put the object
//
// Copy the reader in to the new object which is returned
//
// The new object may have been created if an error is returned
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	existingObj, err := f.newObjectWithInfo(ctx, src.Remote(), nil)
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

// PutUnchecked the object into the container
//
// This will produce an error if the object already exists
//
// Copy the reader in to the new object which is returned
//
// The new object may have been created if an error is returned
func (f *Fs) PutUnchecked(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	remote := src.Remote()
	size := src.Size()
	modTime := src.ModTime(ctx)

	o, _, _, err := f.createObject(ctx, remote, modTime, size)
	if err != nil {
		return nil, err
	}
	return o, o.Update(ctx, in, src, options...)
}

// Mkdir creates the container if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	_, err := f.dirCache.FindDir(ctx, dir, true)
	return err
}

// purgeCheck removes the root directory, if check is set then it
// refuses to do so if it has anything in
func (f *Fs) purgeCheck(ctx context.Context, dir string, check bool) error {
	root := path.Join(f.root, dir)
	if root == "" {
		return errors.New("can't purge root directory")
	}
	dc := f.dirCache
	rootID, err := dc.FindDir(ctx, dir, false)
	if err != nil {
		return err
	}

	// need to check if empty as it will delete recursively by default
	if check {
		found, err := f.listAll(ctx, rootID, false, false, func(item *api.Item) bool {
			return true
		})
		if err != nil {
			return errors.Wrap(err, "purgeCheck")
		}
		if found {
			return fs.ErrorDirectoryNotEmpty
		}
	}

	opts := rest.Opts{
		Method: "POST",
		Path:   "/folder/delete",
		MultipartParams: url.Values{
			"id": {rootID},
		},
		Parameters: f.baseParams(),
	}
	var resp *http.Response
	var result api.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return errors.Wrap(err, "rmdir failed")
	}
	if err = result.AsErr(); err != nil {
		return errors.Wrap(err, "rmdir")
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

// Precision return the precision of this Fs
func (f *Fs) Precision() time.Duration {
	return fs.ModTimeNotSupported
}

// Purge deletes all the files in the directory
//
// Optional interface: Only implement this if you have a way of
// deleting all the files quicker than just running Remove() on the
// result of List()
func (f *Fs) Purge(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir, false)
}

// move a file or folder
//
// This is complicated by the fact that there is an API to move files
// between directories and a separate one to rename them.  We try to
// call the minimum number of API calls.
func (f *Fs) move(ctx context.Context, isFile bool, id, oldLeaf, newLeaf, oldDirectoryID, newDirectoryID string) (err error) {
	newLeaf = f.opt.Enc.FromStandardName(newLeaf)
	oldLeaf = f.opt.Enc.FromStandardName(oldLeaf)
	doRenameLeaf := oldLeaf != newLeaf
	doMove := oldDirectoryID != newDirectoryID

	// Now rename the leaf to a temporary name if we are moving to
	// another directory to make sure we don't overwrite something
	// in the destination directory by accident
	if doRenameLeaf && doMove {
		tmpLeaf := newLeaf + "." + random.String(8)
		err = f.renameLeaf(ctx, isFile, id, tmpLeaf)
		if err != nil {
			return errors.Wrap(err, "Move rename leaf")
		}
	}

	// Move the object to a new directory (with the existing name)
	// if required
	if doMove {
		opts := rest.Opts{
			Method:     "POST",
			Path:       "/folder/paste",
			Parameters: f.baseParams(),
			MultipartParams: url.Values{
				"id": {newDirectoryID},
			},
		}
		if isFile {
			opts.MultipartParams.Set("files[]", id)
		} else {
			opts.MultipartParams.Set("folders[]", id)
		}
		//replacedLeaf := enc.FromStandardName(leaf)
		var resp *http.Response
		var result api.Response
		err = f.pacer.Call(func() (bool, error) {
			resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
			return shouldRetry(ctx, resp, err)
		})
		if err != nil {
			return errors.Wrap(err, "Move http")
		}
		if err = result.AsErr(); err != nil {
			return errors.Wrap(err, "Move")
		}
	}

	// Rename the leaf to its final name if required
	if doRenameLeaf {
		err = f.renameLeaf(ctx, isFile, id, newLeaf)
		if err != nil {
			return errors.Wrap(err, "Move rename leaf")
		}
	}

	return nil
}

// Move src to this remote using server-side move operations.
//
// This is stored with the remote path given
//
// It returns the destination Object and a possible error
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

	// Create temporary object
	dstObj, leaf, directoryID, err := f.createObject(ctx, remote, srcObj.modTime, srcObj.size)
	if err != nil {
		return nil, err
	}

	// Do the move
	err = f.move(ctx, true, srcObj.id, path.Base(srcObj.remote), leaf, srcObj.parentID, directoryID)
	if err != nil {
		return nil, err
	}

	err = dstObj.readMetaData(ctx)
	if err != nil {
		return nil, err
	}
	return dstObj, nil
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

	srcID, srcDirectoryID, srcLeaf, dstDirectoryID, dstLeaf, err := f.dirCache.DirMove(ctx, srcFs.dirCache, srcFs.root, srcRemote, f.root, dstRemote)
	if err != nil {
		return err
	}

	// Do the move
	err = f.move(ctx, false, srcID, srcLeaf, dstLeaf, srcDirectoryID, dstDirectoryID)
	if err != nil {
		return err
	}
	srcFs.dirCache.FlushDir(srcRemote)
	return nil
}

// PublicLink adds a "readable by anyone with link" permission on the given file or folder.
func (f *Fs) PublicLink(ctx context.Context, remote string, expire fs.Duration, unlink bool) (string, error) {
	_, err := f.dirCache.FindDir(ctx, remote, false)
	if err == nil {
		return "", fs.ErrorCantShareDirectories
	}
	o, err := f.NewObject(ctx, remote)
	if err != nil {
		return "", err
	}
	return o.(*Object).url, nil
}

// About gets quota information
func (f *Fs) About(ctx context.Context) (usage *fs.Usage, err error) {
	var resp *http.Response
	var info api.AccountInfoResponse
	opts := rest.Opts{
		Method:     "POST",
		Path:       "/account/info",
		Parameters: f.baseParams(),
	}
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, errors.Wrap(err, "CreateDir http")
	}
	if err = info.AsErr(); err != nil {
		return nil, errors.Wrap(err, "CreateDir")
	}
	usage = &fs.Usage{
		Used: fs.NewUsageValue(int64(info.SpaceUsed)),
	}
	return usage, nil
}

// DirCacheFlush resets the directory cache - used in testing as an
// optional interface
func (f *Fs) DirCacheFlush() {
	f.dirCache.ResetRoot()
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

// Hash returns the SHA-1 of an object returning a lowercase hex string
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	return "", hash.ErrUnsupported
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

// setMetaData sets the metadata from info
func (o *Object) setMetaData(info *api.Item) (err error) {
	if info.Type != "file" {
		return errors.Wrapf(fs.ErrorNotAFile, "%q is %q", o.remote, info.Type)
	}
	o.hasMetaData = true
	o.size = info.Size
	o.modTime = time.Unix(info.CreatedAt, 0)
	o.id = info.ID
	o.mimeType = info.MimeType
	o.url = info.Link
	return nil
}

// readMetaData gets the metadata if it hasn't already been fetched
//
// it also sets the info
func (o *Object) readMetaData(ctx context.Context) (err error) {
	if o.hasMetaData {
		return nil
	}
	info, err := o.fs.readMetaDataForPath(ctx, o.remote, false, true)
	if err != nil {
		return err
	}
	return o.setMetaData(info)
}

// ModTime returns the modification time of the object
//
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
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

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	if o.url == "" {
		return nil, errors.New("can't download - no URL")
	}
	fs.FixRangeOption(options, o.size)
	var resp *http.Response
	opts := rest.Opts{
		Path:    "",
		RootURL: o.url,
		Method:  "GET",
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

// Update the object with the contents of the io.Reader, modTime and size
//
// If existing is set then it updates the object rather than creating a new one
//
// The new object may have been created if an error is returned
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	remote := o.Remote()
	size := src.Size()

	// Create the directory for the object if it doesn't exist
	leaf, directoryID, err := o.fs.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return err
	}
	leaf = o.fs.opt.Enc.FromStandardName(leaf)

	var resp *http.Response
	var info api.FolderUploadinfoResponse
	opts := rest.Opts{
		Method:     "POST",
		Path:       "/folder/uploadinfo",
		Parameters: o.fs.baseParams(),
		Options:    options,
		MultipartParams: url.Values{
			"id": {directoryID},
		},
	}
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.CallJSON(ctx, &opts, nil, &info)
		if err != nil {
			return shouldRetry(ctx, resp, err)
		}
		// Just check the download URL resolves - sometimes
		// the URLs returned by premiumize.me don't resolve so
		// this needs a retry.
		var u *url.URL
		u, err = url.Parse(info.URL)
		if err != nil {
			return true, errors.Wrap(err, "failed to parse download URL")
		}
		_, err = net.LookupIP(u.Hostname())
		if err != nil {
			return true, errors.Wrap(err, "failed to resolve download URL")
		}
		return false, nil
	})
	if err != nil {
		return errors.Wrap(err, "upload get URL http")
	}
	if err = info.AsErr(); err != nil {
		return errors.Wrap(err, "upload get URL")
	}

	// if file exists then rename it out the way otherwise uploads can fail
	uploaded := false
	var oldID = o.id
	if o.hasMetaData {
		newLeaf := leaf + "." + random.String(8)
		fs.Debugf(o, "Moving old file out the way to %q", newLeaf)
		err = o.fs.renameLeaf(ctx, true, oldID, newLeaf)
		if err != nil {
			return errors.Wrap(err, "upload rename old file")
		}
		defer func() {
			// on failed upload rename old file back
			if !uploaded {
				fs.Debugf(o, "Renaming old file back (from %q to %q) since upload failed", leaf, newLeaf)
				newErr := o.fs.renameLeaf(ctx, true, oldID, leaf)
				if newErr != nil && err == nil {
					err = errors.Wrap(newErr, "upload renaming old file back")
				}
			}
		}()
	}

	opts = rest.Opts{
		Method:  "POST",
		RootURL: info.URL,
		Body:    in,
		MultipartParams: url.Values{
			"token": {info.Token},
		},
		MultipartContentName: "file", // ..name of the parameter which is the attached file
		MultipartFileName:    leaf,   // ..name of the file for the attached file
		ContentLength:        &size,
	}
	var result api.Response
	err = o.fs.pacer.CallNoRetry(func() (bool, error) {
		resp, err = o.fs.srv.CallJSON(ctx, &opts, nil, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return errors.Wrap(err, "upload file http")
	}
	if err = result.AsErr(); err != nil {
		return errors.Wrap(err, "upload file")
	}

	// on successful upload, remove old file if it exists
	uploaded = true
	if o.hasMetaData {
		fs.Debugf(o, "Removing old file")
		err := o.fs.remove(ctx, oldID)
		if err != nil {
			return errors.Wrap(err, "upload remove old file")
		}
	}

	o.hasMetaData = false
	return o.readMetaData(ctx)
}

// Rename the leaf of a file or directory in a directory
func (f *Fs) renameLeaf(ctx context.Context, isFile bool, id string, newLeaf string) (err error) {
	opts := rest.Opts{
		Method: "POST",
		MultipartParams: url.Values{
			"id":   {id},
			"name": {newLeaf},
		},
		Parameters: f.baseParams(),
	}
	if isFile {
		opts.Path = "/item/rename"
	} else {
		opts.Path = "/folder/rename"
	}
	var resp *http.Response
	var result api.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return errors.Wrap(err, "rename http")
	}
	if err = result.AsErr(); err != nil {
		return errors.Wrap(err, "rename")
	}
	return nil
}

// Remove an object by ID
func (f *Fs) remove(ctx context.Context, id string) (err error) {
	opts := rest.Opts{
		Method: "POST",
		Path:   "/item/delete",
		MultipartParams: url.Values{
			"id": {id},
		},
		Parameters: f.baseParams(),
	}
	var resp *http.Response
	var result api.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return errors.Wrap(err, "remove http")
	}
	if err = result.AsErr(); err != nil {
		return errors.Wrap(err, "remove")
	}
	return nil
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	err := o.readMetaData(ctx)
	if err != nil {
		return errors.Wrap(err, "Remove: Failed to read metadata")
	}
	return o.fs.remove(ctx, o.id)
}

// MimeType of an Object if known, "" otherwise
func (o *Object) MimeType(ctx context.Context) string {
	return o.mimeType
}

// ID returns the ID of the Object if known, or "" if not
func (o *Object) ID() string {
	return o.id
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = (*Fs)(nil)
	_ fs.Purger          = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ fs.DirCacheFlusher = (*Fs)(nil)
	_ fs.Abouter         = (*Fs)(nil)
	_ fs.PublicLinker    = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.MimeTyper       = (*Object)(nil)
	_ fs.IDer            = (*Object)(nil)
)
