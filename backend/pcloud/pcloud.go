// Package pcloud provides an interface to the Pcloud
// object storage system.
package pcloud

// FIXME implement ListR? /listfolder can do recursive lists

// FIXME cleanup returns login required?

// FIXME mime type? Fix overview if implement.

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/backend/pcloud/api"
	"github.com/rclone/rclone/fs"
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
	"github.com/rclone/rclone/lib/rest"
	"golang.org/x/oauth2"
)

const (
	rcloneClientID              = "DnONSzyJXpm"
	rcloneEncryptedClientSecret = "ej1OIF39VOQQ0PXaSdK9ztkLw3tdLNscW2157TKNQdQKkICR4uU7aFg4eFM"
	minSleep                    = 10 * time.Millisecond
	maxSleep                    = 2 * time.Second
	decayConstant               = 2 // bigger for slower decay, exponential
	defaultHostname             = "api.pcloud.com"
)

// Globals
var (
	// Description of how to auth for this app
	oauthConfig = &oauth2.Config{
		Scopes: nil,
		Endpoint: oauth2.Endpoint{
			AuthURL: "https://my.pcloud.com/oauth2/authorize",
			// TokenURL: "https://api.pcloud.com/oauth2_token", set by updateTokenURL
		},
		ClientID:     rcloneClientID,
		ClientSecret: obscure.MustReveal(rcloneEncryptedClientSecret),
		RedirectURL:  oauthutil.RedirectLocalhostURL,
	}
)

// Update the TokenURL with the actual hostname
func updateTokenURL(oauthConfig *oauth2.Config, hostname string) {
	oauthConfig.Endpoint.TokenURL = "https://" + hostname + "/oauth2_token"
}

// Register with Fs
func init() {
	updateTokenURL(oauthConfig, defaultHostname)
	fs.Register(&fs.RegInfo{
		Name:        "pcloud",
		Description: "Pcloud",
		NewFs:       NewFs,
		Config: func(name string, m configmap.Mapper) {
			optc := new(Options)
			err := configstruct.Set(m, optc)
			if err != nil {
				fs.Errorf(nil, "Failed to read config: %v", err)
			}
			updateTokenURL(oauthConfig, optc.Hostname)
			checkAuth := func(oauthConfig *oauth2.Config, auth *oauthutil.AuthResult) error {
				if auth == nil || auth.Form == nil {
					return errors.New("form not found in response")
				}
				hostname := auth.Form.Get("hostname")
				if hostname == "" {
					hostname = defaultHostname
				}
				// Save the hostname in the config
				m.Set("hostname", hostname)
				// Update the token URL
				updateTokenURL(oauthConfig, hostname)
				fs.Debugf(nil, "pcloud: got hostname %q", hostname)
				return nil
			}
			opt := oauthutil.Options{
				CheckAuth:    checkAuth,
				StateBlankOK: true, // pCloud seems to drop the state parameter now - see #4210
			}
			err = oauthutil.Config("pcloud", name, m, oauthConfig, &opt)
			if err != nil {
				log.Fatalf("Failed to configure token: %v", err)
			}
		},
		Options: append(oauthutil.SharedOptions, []fs.Option{{
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			// Encode invalid UTF-8 bytes as json doesn't handle them properly.
			//
			// TODO: Investigate Unicode simplification (＼ gets converted to \ server-side)
			Default: (encoder.Display |
				encoder.EncodeBackSlash |
				encoder.EncodeInvalidUtf8),
		}, {
			Name:     "root_folder_id",
			Help:     "Fill in for rclone to use a non root folder as its starting point.",
			Default:  "d0",
			Advanced: true,
		}, {
			Name: "hostname",
			Help: `Hostname to connect to.

This is normally set when rclone initially does the oauth connection,
however you will need to set it by hand if you are using remote config
with rclone authorize.
`,
			Default:  defaultHostname,
			Advanced: true,
			Examples: []fs.OptionExample{{
				Value: defaultHostname,
				Help:  "Original/US region",
			}, {
				Value: "eapi.pcloud.com",
				Help:  "EU region",
			}},
		}}...),
	})
}

// Options defines the configuration for this backend
type Options struct {
	Enc          encoder.MultiEncoder `config:"encoding"`
	RootFolderID string               `config:"root_folder_id"`
	Hostname     string               `config:"hostname"`
}

// Fs represents a remote pcloud
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

// Object describes a pcloud object
//
// Will definitely have info but maybe not meta
type Object struct {
	fs          *Fs       // what this object is part of
	remote      string    // The remote path
	hasMetaData bool      // whether info below has been set
	size        int64     // size of the object
	modTime     time.Time // modification time of the object
	id          string    // ID of the object
	md5         string    // MD5 if known
	sha1        string    // SHA1 if known
	link        *api.GetFileLinkResult
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
	return fmt.Sprintf("pcloud root '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// parsePath parses a pcloud 'url'
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
func shouldRetry(resp *http.Response, err error) (bool, error) {
	doRetry := false

	// Check if it is an api.Error
	if apiErr, ok := err.(*api.Error); ok {
		// See https://docs.pcloud.com/errors/ for error treatment
		// Errors are classified as 1xxx, 2xxx etc
		switch apiErr.Result / 1000 {
		case 4: // 4xxx: rate limiting
			doRetry = true
		case 5: // 5xxx: internal errors
			doRetry = true
		}
	}

	if resp != nil && resp.StatusCode == 401 && len(resp.Header["Www-Authenticate"]) == 1 && strings.Index(resp.Header["Www-Authenticate"][0], "expired_token") >= 0 {
		doRetry = true
		fs.Debugf(nil, "Should retry: %v", err)
	}
	return doRetry || fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// readMetaDataForPath reads the metadata from the path
func (f *Fs) readMetaDataForPath(ctx context.Context, path string) (info *api.Item, err error) {
	// defer fs.Trace(f, "path=%q", path)("info=%+v, err=%v", &info, &err)
	leaf, directoryID, err := f.dirCache.FindPath(ctx, path, false)
	if err != nil {
		if err == fs.ErrorDirNotFound {
			return nil, fs.ErrorObjectNotFound
		}
		return nil, err
	}

	found, err := f.listAll(ctx, directoryID, false, true, func(item *api.Item) bool {
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

// errorHandler parses a non 2xx error response into an error
func errorHandler(resp *http.Response) error {
	// Decode error response
	errResponse := new(api.Error)
	err := rest.DecodeJSON(resp, &errResponse)
	if err != nil {
		fs.Debugf(nil, "Couldn't decode error response: %v", err)
	}
	if errResponse.ErrorString == "" {
		errResponse.ErrorString = resp.Status
	}
	if errResponse.Result == 0 {
		errResponse.Result = resp.StatusCode
	}
	return errResponse
}

// NewFs constructs an Fs from the path, container:path
func NewFs(name, root string, m configmap.Mapper) (fs.Fs, error) {
	ctx := context.Background()
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	root = parsePath(root)
	oAuthClient, ts, err := oauthutil.NewClient(name, m, oauthConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to configure Pcloud")
	}
	updateTokenURL(oauthConfig, opt.Hostname)

	f := &Fs{
		name:  name,
		root:  root,
		opt:   *opt,
		srv:   rest.NewClient(oAuthClient).SetRoot("https://" + opt.Hostname),
		pacer: fs.NewPacer(pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
	}
	f.features = (&fs.Features{
		CaseInsensitive:         false,
		CanHaveEmptyDirectories: true,
	}).Fill(f)
	f.srv.SetErrorHandler(errorHandler)

	// Renew the token in the background
	f.tokenRenewer = oauthutil.NewRenew(f.String(), ts, func() error {
		_, err := f.readMetaDataForPath(ctx, "")
		return err
	})

	// Get rootFolderID
	rootID := f.opt.RootFolderID
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
		if item.Name == leaf {
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
	var result api.ItemResult
	opts := rest.Opts{
		Method:     "POST",
		Path:       "/createfolder",
		Parameters: url.Values{},
	}
	opts.Parameters.Set("name", f.opt.Enc.FromStandardName(leaf))
	opts.Parameters.Set("folderid", dirIDtoNumber(pathID))
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		err = result.Error.Update(err)
		return shouldRetry(resp, err)
	})
	if err != nil {
		//fmt.Printf("...Error %v\n", err)
		return "", err
	}
	// fmt.Printf("...Id %q\n", *info.Id)
	return result.Metadata.ID, nil
}

// Converts a dirID which is usually 'd' followed by digits into just
// the digits
func dirIDtoNumber(dirID string) string {
	if len(dirID) > 0 && dirID[0] == 'd' {
		return dirID[1:]
	}
	fs.Debugf(nil, "Invalid directory id %q", dirID)
	return dirID
}

// Converts a fileID which is usually 'f' followed by digits into just
// the digits
func fileIDtoNumber(fileID string) string {
	if len(fileID) > 0 && fileID[0] == 'f' {
		return fileID[1:]
	}
	fs.Debugf(nil, "Invalid file id %q", fileID)
	return fileID
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
		Path:       "/listfolder",
		Parameters: url.Values{},
	}
	opts.Parameters.Set("folderid", dirIDtoNumber(dirID))
	// FIXME can do recursive

	var result api.ItemResult
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		err = result.Error.Update(err)
		return shouldRetry(resp, err)
	})
	if err != nil {
		return found, errors.Wrap(err, "couldn't list files")
	}
	for i := range result.Metadata.Contents {
		item := &result.Metadata.Contents[i]
		if item.IsFolder {
			if filesOnly {
				continue
			}
		} else {
			if directoriesOnly {
				continue
			}
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
		if info.IsFolder {
			// cache the directory ID for later lookups
			f.dirCache.Put(remote, info.ID)
			d := fs.NewDir(remote, info.ModTime()).SetID(info.ID)
			// FIXME more info from dir?
			entries = append(entries, d)
		} else {
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

// Put the object into the container
//
// Copy the reader in to the new object which is returned
//
// The new object may have been created if an error is returned
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
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

	opts := rest.Opts{
		Method:     "POST",
		Path:       "/deletefolder",
		Parameters: url.Values{},
	}
	opts.Parameters.Set("folderid", dirIDtoNumber(rootID))
	if !check {
		opts.Path = "/deletefolderrecursive"
	}
	var resp *http.Response
	var result api.ItemResult
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		err = result.Error.Update(err)
		return shouldRetry(resp, err)
	})
	if err != nil {
		return errors.Wrap(err, "rmdir failed")
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
	return time.Second
}

// Copy src to this remote using server side copy operations.
//
// This is stored with the remote path given
//
// It returns the destination Object and a possible error
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
	dstObj, leaf, directoryID, err := f.createObject(ctx, remote, srcObj.modTime, srcObj.size)
	if err != nil {
		return nil, err
	}

	// Copy the object
	opts := rest.Opts{
		Method:     "POST",
		Path:       "/copyfile",
		Parameters: url.Values{},
	}
	opts.Parameters.Set("fileid", fileIDtoNumber(srcObj.id))
	opts.Parameters.Set("toname", f.opt.Enc.FromStandardName(leaf))
	opts.Parameters.Set("tofolderid", dirIDtoNumber(directoryID))
	opts.Parameters.Set("mtime", fmt.Sprintf("%d", srcObj.modTime.Unix()))
	var resp *http.Response
	var result api.ItemResult
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		err = result.Error.Update(err)
		return shouldRetry(resp, err)
	})
	if err != nil {
		return nil, err
	}
	err = dstObj.setMetaData(&result.Metadata)
	if err != nil {
		return nil, err
	}
	return dstObj, nil
}

// Purge deletes all the files in the directory
//
// Optional interface: Only implement this if you have a way of
// deleting all the files quicker than just running Remove() on the
// result of List()
func (f *Fs) Purge(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir, false)
}

// CleanUp empties the trash
func (f *Fs) CleanUp(ctx context.Context) error {
	rootID, err := f.dirCache.RootID(ctx, false)
	if err != nil {
		return err
	}
	opts := rest.Opts{
		Method:     "POST",
		Path:       "/trash_clear",
		Parameters: url.Values{},
	}
	opts.Parameters.Set("folderid", dirIDtoNumber(rootID))
	var resp *http.Response
	var result api.Error
	return f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		err = result.Update(err)
		return shouldRetry(resp, err)
	})
}

// Move src to this remote using server side move operations.
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
	opts := rest.Opts{
		Method:     "POST",
		Path:       "/renamefile",
		Parameters: url.Values{},
	}
	opts.Parameters.Set("fileid", fileIDtoNumber(srcObj.id))
	opts.Parameters.Set("toname", f.opt.Enc.FromStandardName(leaf))
	opts.Parameters.Set("tofolderid", dirIDtoNumber(directoryID))
	var resp *http.Response
	var result api.ItemResult
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		err = result.Error.Update(err)
		return shouldRetry(resp, err)
	})
	if err != nil {
		return nil, err
	}

	err = dstObj.setMetaData(&result.Metadata)
	if err != nil {
		return nil, err
	}
	return dstObj, nil
}

// DirMove moves src, srcRemote to this remote at dstRemote
// using server side move operations.
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

	srcID, _, _, dstDirectoryID, dstLeaf, err := f.dirCache.DirMove(ctx, srcFs.dirCache, srcFs.root, srcRemote, f.root, dstRemote)
	if err != nil {
		return err
	}

	// Do the move
	opts := rest.Opts{
		Method:     "POST",
		Path:       "/renamefolder",
		Parameters: url.Values{},
	}
	opts.Parameters.Set("folderid", dirIDtoNumber(srcID))
	opts.Parameters.Set("toname", f.opt.Enc.FromStandardName(dstLeaf))
	opts.Parameters.Set("tofolderid", dirIDtoNumber(dstDirectoryID))
	var resp *http.Response
	var result api.ItemResult
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		err = result.Error.Update(err)
		return shouldRetry(resp, err)
	})
	if err != nil {
		return err
	}

	srcFs.dirCache.FlushDir(srcRemote)
	return nil
}

// DirCacheFlush resets the directory cache - used in testing as an
// optional interface
func (f *Fs) DirCacheFlush() {
	f.dirCache.ResetRoot()
}

func (f *Fs) linkDir(ctx context.Context, dirID string, expire fs.Duration) (string, error) {
	opts := rest.Opts{
		Method:     "POST",
		Path:       "/getfolderpublink",
		Parameters: url.Values{},
	}
	var result api.PubLinkResult
	opts.Parameters.Set("folderid", dirIDtoNumber(dirID))
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, nil, &result)
		err = result.Error.Update(err)
		return shouldRetry(resp, err)
	})
	if err != nil {
		return "", err
	}
	return result.Link, err
}

func (f *Fs) linkFile(ctx context.Context, path string, expire fs.Duration) (string, error) {
	obj, err := f.NewObject(ctx, path)
	if err != nil {
		return "", err
	}
	o := obj.(*Object)
	opts := rest.Opts{
		Method:     "POST",
		Path:       "/getfilepublink",
		Parameters: url.Values{},
	}
	var result api.PubLinkResult
	opts.Parameters.Set("fileid", fileIDtoNumber(o.id))
	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, nil, &result)
		err = result.Error.Update(err)
		return shouldRetry(resp, err)
	})
	if err != nil {
		return "", err
	}
	return result.Link, nil
}

// PublicLink adds a "readable by anyone with link" permission on the given file or folder.
func (f *Fs) PublicLink(ctx context.Context, remote string, expire fs.Duration, unlink bool) (string, error) {
	dirID, err := f.dirCache.FindDir(ctx, remote, false)
	if err == fs.ErrorDirNotFound {
		return f.linkFile(ctx, remote, expire)
	}
	if err != nil {
		return "", err
	}
	return f.linkDir(ctx, dirID, expire)
}

// About gets quota information
func (f *Fs) About(ctx context.Context) (usage *fs.Usage, err error) {
	opts := rest.Opts{
		Method: "POST",
		Path:   "/userinfo",
	}
	var resp *http.Response
	var q api.UserInfo
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &q)
		err = q.Error.Update(err)
		return shouldRetry(resp, err)
	})
	if err != nil {
		return nil, errors.Wrap(err, "about failed")
	}
	usage = &fs.Usage{
		Total: fs.NewUsageValue(q.Quota),               // quota of bytes that can be used
		Used:  fs.NewUsageValue(q.UsedQuota),           // bytes in use
		Free:  fs.NewUsageValue(q.Quota - q.UsedQuota), // bytes which can be uploaded before reaching the quota
	}
	return usage, nil
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.MD5 | hash.SHA1)
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

// getHashes fetches the hashes into the object
func (o *Object) getHashes(ctx context.Context) (err error) {
	var resp *http.Response
	var result api.ChecksumFileResult
	opts := rest.Opts{
		Method:     "GET",
		Path:       "/checksumfile",
		Parameters: url.Values{},
	}
	opts.Parameters.Set("fileid", fileIDtoNumber(o.id))
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.CallJSON(ctx, &opts, nil, &result)
		err = result.Error.Update(err)
		return shouldRetry(resp, err)
	})
	if err != nil {
		return err
	}
	o.setHashes(&result.Hashes)
	return o.setMetaData(&result.Metadata)
}

// Hash returns the SHA-1 of an object returning a lowercase hex string
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	if t != hash.MD5 && t != hash.SHA1 {
		return "", hash.ErrUnsupported
	}
	if o.md5 == "" && o.sha1 == "" {
		err := o.getHashes(ctx)
		if err != nil {
			return "", errors.Wrap(err, "failed to get hash")
		}
	}
	if t == hash.MD5 {
		return o.md5, nil
	}
	return o.sha1, nil
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
	if info.IsFolder {
		return errors.Wrapf(fs.ErrorNotAFile, "%q is a folder", o.remote)
	}
	o.hasMetaData = true
	o.size = info.Size
	o.modTime = info.ModTime()
	o.id = info.ID
	return nil
}

// setHashes sets the hashes from that passed in
func (o *Object) setHashes(hashes *api.Hashes) {
	o.sha1 = hashes.SHA1
	o.md5 = hashes.MD5
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
		//if apiErr, ok := err.(*api.Error); ok {
		// FIXME
		// if apiErr.Code == "not_found" || apiErr.Code == "trashed" {
		// 	return fs.ErrorObjectNotFound
		// }
		//}
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
	// Pcloud doesn't have a way of doing this so returning this
	// error will cause the file to be re-uploaded to set the time.
	return fs.ErrorCantSetModTime
}

// Storable returns a boolean showing whether this object storable
func (o *Object) Storable() bool {
	return true
}

// downloadURL fetches the download link
func (o *Object) downloadURL(ctx context.Context) (URL string, err error) {
	if o.id == "" {
		return "", errors.New("can't download - no id")
	}
	if o.link.IsValid() {
		return o.link.URL(), nil
	}
	var resp *http.Response
	var result api.GetFileLinkResult
	opts := rest.Opts{
		Method:     "GET",
		Path:       "/getfilelink",
		Parameters: url.Values{},
	}
	opts.Parameters.Set("fileid", fileIDtoNumber(o.id))
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.CallJSON(ctx, &opts, nil, &result)
		err = result.Error.Update(err)
		return shouldRetry(resp, err)
	})
	if err != nil {
		return "", err
	}
	if !result.IsValid() {
		return "", errors.Errorf("fetched invalid link %+v", result)
	}
	o.link = &result
	return o.link.URL(), nil
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	url, err := o.downloadURL(ctx)
	if err != nil {
		return nil, err
	}
	var resp *http.Response
	opts := rest.Opts{
		Method:  "GET",
		RootURL: url,
		Options: options,
	}
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.Call(ctx, &opts)
		return shouldRetry(resp, err)
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
	o.fs.tokenRenewer.Start()
	defer o.fs.tokenRenewer.Stop()

	size := src.Size() // NB can upload without size
	modTime := src.ModTime(ctx)
	remote := o.Remote()

	// Create the directory for the object if it doesn't exist
	leaf, directoryID, err := o.fs.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return err
	}

	// Experiments with pcloud indicate that it doesn't like any
	// form of request which doesn't have a Content-Length.
	// According to the docs if you close the connection at the
	// end then it should work without Content-Length, but I
	// couldn't get this to work using opts.Close (which sets
	// http.Request.Close).
	//
	// This means that chunked transfer encoding needs to be
	// disabled and a Content-Length needs to be supplied.  This
	// also rules out streaming.
	//
	// Docs: https://docs.pcloud.com/methods/file/uploadfile.html
	var resp *http.Response
	var result api.UploadFileResponse
	opts := rest.Opts{
		Method:           "PUT",
		Path:             "/uploadfile",
		Body:             in,
		ContentType:      fs.MimeType(ctx, o),
		ContentLength:    &size,
		Parameters:       url.Values{},
		TransferEncoding: []string{"identity"}, // pcloud doesn't like chunked encoding
		Options:          options,
	}
	leaf = o.fs.opt.Enc.FromStandardName(leaf)
	opts.Parameters.Set("filename", leaf)
	opts.Parameters.Set("folderid", dirIDtoNumber(directoryID))
	opts.Parameters.Set("nopartial", "1")
	opts.Parameters.Set("mtime", fmt.Sprintf("%d", modTime.Unix()))

	// Special treatment for a 0 length upload.  This doesn't work
	// with PUT even with Content-Length set (by setting
	// opts.Body=0), so upload it as a multpart form POST with
	// Content-Length set.
	if size == 0 {
		formReader, contentType, overhead, err := rest.MultipartUpload(in, opts.Parameters, "content", leaf)
		if err != nil {
			return errors.Wrap(err, "failed to make multipart upload for 0 length file")
		}

		contentLength := overhead + size

		opts.ContentType = contentType
		opts.Body = formReader
		opts.Method = "POST"
		opts.Parameters = nil
		opts.ContentLength = &contentLength
	}

	err = o.fs.pacer.CallNoRetry(func() (bool, error) {
		resp, err = o.fs.srv.CallJSON(ctx, &opts, nil, &result)
		err = result.Error.Update(err)
		return shouldRetry(resp, err)
	})
	if err != nil {
		// sometimes pcloud leaves a half complete file on
		// error, so delete it if it exists
		delObj, delErr := o.fs.NewObject(ctx, o.remote)
		if delErr == nil && delObj != nil {
			_ = delObj.Remove(ctx)
		}
		return err
	}
	if len(result.Items) != 1 {
		return errors.Errorf("failed to upload %v - not sure why", o)
	}
	o.setHashes(&result.Checksums[0])
	return o.setMetaData(&result.Items[0])
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	opts := rest.Opts{
		Method:     "POST",
		Path:       "/deletefile",
		Parameters: url.Values{},
	}
	var result api.ItemResult
	opts.Parameters.Set("fileid", fileIDtoNumber(o.id))
	return o.fs.pacer.Call(func() (bool, error) {
		resp, err := o.fs.srv.CallJSON(ctx, &opts, nil, &result)
		err = result.Error.Update(err)
		return shouldRetry(resp, err)
	})
}

// ID returns the ID of the Object if known, or "" if not
func (o *Object) ID() string {
	return o.id
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = (*Fs)(nil)
	_ fs.Purger          = (*Fs)(nil)
	_ fs.CleanUpper      = (*Fs)(nil)
	_ fs.Copier          = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ fs.DirCacheFlusher = (*Fs)(nil)
	_ fs.PublicLinker    = (*Fs)(nil)
	_ fs.Abouter         = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.IDer            = (*Object)(nil)
)
