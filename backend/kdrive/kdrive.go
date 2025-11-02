// Package kdrive provides an interface to the kdrive
// object storage system.
package kdrive

// FIXME cleanup returns login required?

// FIXME mime type? Fix overview if implement.

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

	"github.com/rclone/rclone/backend/kdrive/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/list"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/oauthutil"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
	"golang.org/x/oauth2"
)

const (
	rcloneClientID              = "" // unused currently
	rcloneEncryptedClientSecret = "" // unused currently
	minSleep                    = 10 * time.Millisecond
	maxSleep                    = 2 * time.Second
	decayConstant               = 2 // bigger for slower decay, exponential
)

// Globals
var (
	// Description of how to auth for this app
	oauthConfig = &oauthutil.Config{
		Scopes:       []string{"user_info", "accounts", "drive"},
		AuthURL:      "https://login.infomaniak.com/authorize",
		TokenURL:     "https://login.infomaniak.com/token",
		ClientID:     rcloneClientID,
		ClientSecret: "", //obscure.MustReveal(rcloneEncryptedClientSecret),
		RedirectURL:  oauthutil.RedirectLocalhostURL,
	}
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "Kdrive",
		Description: "Infomaniak's kDrive",
		NewFs:       NewFs,
		Config: func(ctx context.Context, name string, m configmap.Mapper, config fs.ConfigIn) (*fs.ConfigOut, error) {
			optc := new(Options)
			err := configstruct.Set(m, optc)
			if err != nil {
				fs.Errorf(nil, "Failed to read config: %v", err)
			}
			checkAuth := func(oauthConfig *oauthutil.Config, auth *oauthutil.AuthResult) error {
				if auth == nil || auth.Form == nil {
					return errors.New("form not found in response")
				}
				return nil
			}
			return oauthutil.ConfigOut("", &oauthutil.Options{
				OAuth2Config: oauthConfig,
				CheckAuth:    checkAuth,
				StateBlankOK: true, // kdrive seems to drop the state parameter now - see #4210
			})
		},
		Options: append(oauthutil.SharedOptions, []fs.Option{{
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			// Encode invalid UTF-8 bytes as json doesn't handle them properly.
			//
			// TODO: Investigate Unicode simplification (＼ gets converted to \ server-side)
			Default: (encoder.Display |
				encoder.EncodeLeftSpace | encoder.EncodeRightSpace |
				encoder.EncodeInvalidUtf8),
		}, {
			Name:    "account_id",
			Help:    "Fill the account ID that is to be considered for this kdrive.",
			Default: "",
		}, {
			Name: "drive_id",
			Help: `Fill the drive ID for this kdrive.
When showing a folder on kdrive, you can find the drive_id here:
https://ksuite.infomaniak.com/{account_id}/kdrive/app/drive/{drive_id}/files/...`,
			Default: "",
		}, {
			Name:    "access_token",
			Help:    `Access token generated in Infomaniak profile manager.`,
			Default: "",
		},
		}...),
	})
}

// Options defines the configuration for this backend
type Options struct {
	Enc         encoder.MultiEncoder `config:"encoding"`
	AccountID   string               `config:"account_id"`
	DriveID     string               `config:"drive_id"`
	AccessToken string               `config:"access_token"`
}

// Fs represents a remote kdrive
type Fs struct {
	name       string             // name of this remote
	root       string             // the path we are working on
	opt        Options            // parsed options
	features   *fs.Features       // optional features
	ts         oauth2.TokenSource // the token source, used to create new clients
	srv        *rest.Client       // the connection to the server
	cleanupSrv *rest.Client       // the connection used for the cleanup method
	dirCache   *dircache.DirCache // Map of directory path to directory id
	pacer      *fs.Pacer          // pacer for API calls
	// tokenRenewer *oauthutil.Renew       // renew the token on expiry
}

// Object describes a kdrive object
//
// Will definitely have info but maybe not meta
type Object struct {
	fs          *Fs       // what this object is part of
	remote      string    // The remote path
	hasMetaData bool      // whether info below has been set
	size        int64     // size of the object
	modTime     time.Time // modification time of the object
	id          string    // ID of the object
	xxh3        string    // XXH3 if known
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
	return fmt.Sprintf("kdrive root '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// parsePath parses a kdrive 'url'
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
	doRetry := false

	// Check if it is an api.Error
	if _, ok := err.(*api.ResultStatus); ok {
		// Errors are classified as 1xx, 2xx, etc.
		switch resp.StatusCode / 100 {
		case 4: // 4xxx: rate limiting
			doRetry = true
		case 5: // 5xxx: internal errors
			doRetry = true
		}
	}

	if resp != nil && resp.StatusCode == 401 && len(resp.Header["Www-Authenticate"]) == 1 && strings.Contains(resp.Header["Www-Authenticate"][0], "expired_token") {
		doRetry = true
		fs.Debugf(nil, "Should retry: %v", err)
	}
	return doRetry || fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// readMetaDataForPath reads the metadata from the path
func (f *Fs) readMetaDataForPath(ctx context.Context, path string) (info *api.Item, err error) {
	fs.Debugf(ctx, "readMetaDataForPath: path=%s", path)
	// defer fs.Trace(f, "path=%q", path)("info=%+v, err=%v", &info, &err)
	leaf, directoryID, err := f.dirCache.FindPath(ctx, path, false)
	if err != nil {
		if err == fs.ErrorDirNotFound {
			return nil, fs.ErrorObjectNotFound
		}
		return nil, err
	}

	found, err := f.listAll(ctx, directoryID, false, true, false, func(item *api.Item) bool {
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
	errResponse := new(api.ResultStatus)
	err := rest.DecodeJSON(resp, &errResponse)
	if err != nil {
		fs.Debugf(nil, "Couldn't decode error response: %v", err)
	}
	if errResponse.ErrorDetail.ErrorString == "" {
		errResponse.ErrorDetail.ErrorString = resp.Status
	}
	if errResponse.ErrorDetail.Result == "" {
		errResponse.ErrorDetail.Result = resp.Status
	}
	return errResponse
}

/*
// unused for now
func (f *Fs) retrieveDriveIdFromName(ctx context.Context, name string) (string, error) {
	// step 1: retrieve user id
	// fs.Debugf(f, "retrieveDriveIdFromName(%q)\n", name)
	var resp *http.Response
	var resultProfile api.Profile
	var resultDrives api.ListDrives
	var err error
	opts := rest.Opts{
		Method:     "GET",
		Path:       "/profile",
		Parameters: url.Values{},
	}
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &resultProfile)
		err = resultProfile.Error.Update(err)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		//fmt.Printf("...Error %v\n", err)
		return "", err
	}
	// fmt.Printf("...Id %q\n", *info.Id)
	userID := resultProfile.Data.UserID
	// step 2: retrieve the drives of that user
	opts = rest.Opts{
		Method:     "GET",
		Path:       fmt.Sprintf("/2/drive/users/%s/drives", userID),
		Parameters: url.Values{},
	}
	opts.Parameters.Set("account_id", f.opt.AccountID)
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &resultDrives)
		err = resultDrives.Error.Update(err)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		//fmt.Printf("...Error %v\n", err)
		return "", err
	}

	return strconv.Itoa(resultDrives.Data[0].DriveID), nil
}
*/
// NewFs constructs an Fs from the path, container:path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	root = parsePath(root)
	fs.Debugf(ctx, "NewFs: for root=%s", root)

	staticToken := oauth2.Token{AccessToken: opt.AccessToken}
	ts := oauth2.StaticTokenSource(&staticToken)
	oAuthClient := oauth2.NewClient(ctx, ts)
	//oAuthClient, ts, err := oauthutil.NewClient(ctx, name, m, oauthConfig)
	//if err != nil {
	//	return nil, fmt.Errorf("failed to configure kdrive: %w", err)
	//}

	f := &Fs{
		name:  name,
		root:  root,
		opt:   *opt,
		ts:    ts,
		srv:   rest.NewClient(oAuthClient).SetRoot("https://api.infomaniak.com"),
		pacer: fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
	}
	f.cleanupSrv = rest.NewClient(fshttp.NewClient(ctx)).SetRoot("https://api.infomaniak.com")
	f.features = (&fs.Features{
		CaseInsensitive:         false,
		CanHaveEmptyDirectories: true,
		PartialUploads:          true,
	}).Fill(ctx, f)
	f.srv.SetErrorHandler(errorHandler)

	// Renew the token in the background
	/*
		f.tokenRenewer = oauthutil.NewRenew(f.String(), f.ts, func() error {
			_, err := f.readMetaDataForPath(ctx, "")
			return err
		})
	*/

	// Get rootFolderID
	rootID := "1" // see https://developer.infomaniak.com/docs/api/get/3/drive/%7Bdrive_id%7D/files/%7Bfile_id%7D
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

// XOpenWriterAt opens with a handle for random access writes
//
// Pass in the remote desired and the size if known.
//
// It truncates any existing object.
//
// OpenWriterAt disabled because it seems to have been disabled at kdrive
// PUT /file_open?flags=XXX&folderid=XXX&name=XXX HTTP/1.1
//
//	{
//	        "result": 2003,
//	        "error": "Access denied. You do not have permissions to perform this operation."
//	}
func (f *Fs) XOpenWriterAt(ctx context.Context, remote string, size int64) (fs.WriterAtCloser, error) {
	client, err := f.newSingleConnClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("create client: %w", err)
	}
	// init an empty file
	leaf, directoryID, err := f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return nil, fmt.Errorf("resolve src: %w", err)
	}
	openResult, err := fileOpenNew(ctx, client, f, directoryID, leaf)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	if _, err := fileClose(ctx, client, f.pacer, openResult.FileDescriptor); err != nil {
		return nil, fmt.Errorf("close file: %w", err)
	}

	writer := &writerAt{
		ctx:    ctx,
		fs:     f,
		size:   size,
		remote: remote,
		fileID: openResult.Fileid,
	}

	return writer, nil
}

// Create a new http client, accepting keep-alive headers, limited to single connection.
// Necessary for kdrive fileops API, as it binds the session to the underlying TCP connection.
// File descriptors are only valid within the same connection and auto-closed when the connection is closed,
// hence we need a separate client (with single connection) for each fd to avoid all sorts of errors and race conditions.
func (f *Fs) newSingleConnClient(ctx context.Context) (*rest.Client, error) {
	baseClient := fshttp.NewClient(ctx)
	baseClient.Transport = fshttp.NewTransportCustom(ctx, func(t *http.Transport) {
		t.MaxConnsPerHost = 1
		t.DisableKeepAlives = false
	})
	// Set our own http client in the context
	ctx = oauthutil.Context(ctx, baseClient)
	// create a new oauth client, reuse the token source
	oAuthClient := oauth2.NewClient(ctx, f.ts)
	return rest.NewClient(oAuthClient).SetRoot("https://api.infomaniak.com"), nil
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
	fs.Debugf(ctx, "FindLeaf: leaf=%s", leaf)
	// Find the leaf in pathID
	found, err = f.listAll(ctx, pathID, true, false, false, func(item *api.Item) bool {
		if item.Name == leaf {
			pathIDOut = strconv.Itoa(item.ID)
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
	var result api.CreateDirResult
	opts := rest.Opts{
		Method:     "POST",
		Path:       fmt.Sprintf("/3/drive/%s/files/%s/directory", f.opt.DriveID, pathID),
		Parameters: url.Values{},
	}
	opts.Parameters.Set("name", f.opt.Enc.FromStandardName(leaf))
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		err = result.ResultStatus.Update(err)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		//fmt.Printf("...Error %v\n", err)
		return "", err
	}
	// fmt.Printf("...Id %q\n", *info.Id)
	return strconv.Itoa(result.Data.ID), nil
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
func (f *Fs) listAll(ctx context.Context, dirID string, directoriesOnly bool, filesOnly bool, recursive bool, fn listAllFn) (found bool, err error) {
	fs.Debugf(ctx, "Entering listAll")
	listSomeFiles := func(currentDirID string, fromCursor string) (api.SearchResult, error) {
		opts := rest.Opts{
			Method:     "GET",
			Path:       fmt.Sprintf("/3/drive/%s/files/%s/files", f.opt.DriveID, currentDirID),
			Parameters: url.Values{},
		}
		opts.Parameters.Set("with", "path")
		if len(fromCursor) > 0 {
			opts.Parameters.Set("cursor", fromCursor)
		}

		var result api.SearchResult
		var resp *http.Response
		err = f.pacer.Call(func() (bool, error) {
			resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
			err = result.ResultStatus.Update(err)
			return shouldRetry(ctx, resp, err)
		})
		if err != nil {
			return result, fmt.Errorf("couldn't list files: %w", err)
		}
		return result, nil
	}

	var recursiveContents func(currentDirID string, fromCursor string)
	recursiveContents = func(currentDirID string, fromCursor string) {
		result, err := listSomeFiles(currentDirID, fromCursor)
		if err != nil {
			return
		}

		// First, analyze what has been returned, and go in-depth if required
		for i := range result.Data {
			item := &result.Data[i]
			if item.Type == "dir" {
				if filesOnly {
					continue
				}
			} else {
				if directoriesOnly {
					continue
				}
			}
			item.Name = f.opt.Enc.ToStandardName(item.Name)
			item.FullPath = f.opt.Enc.ToStandardPath(item.FullPath)
			if fn(item) {
				found = true
				break
			}
			if recursive && item.Type == "dir" {
				recursiveContents(strconv.Itoa(item.ID), "" /*reset cursor*/)
			}
		}

		// Then load the rest of the files in that folder and apply the same logic
		if result.HasMore {
			recursiveContents(currentDirID, result.Cursor)
		}
	}
	recursiveContents(dirID, "")
	return
}

// listHelper iterates over all items from the directory
// and calls the callback for each element.
func (f *Fs) listHelper(ctx context.Context, dir string, recursive bool, callback func(entries fs.DirEntry) error) (err error) {
	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	fs.Debugf(ctx, "listHelper: root=%s dir=%s directoryID=%s", f.root, dir, directoryID)
	if err != nil {
		return err
	}
	var iErr error
	_, err = f.listAll(ctx, directoryID, false, false, recursive, func(info *api.Item) bool {
		fs.Debugf(ctx, "listHelper: trimming /%s out of %s", f.root, info.FullPath)
		remote := parsePath(strings.TrimPrefix(info.FullPath, "/"+f.root))
		if info.Type == "dir" {
			// cache the directory ID for later lookups
			fs.Debugf(ctx, "listAll: caching %s as %s", remote, strconv.Itoa(info.ID))
			f.dirCache.Put(remote, strconv.Itoa(info.ID))

			d := fs.NewDir(remote, info.ModTime()).SetID(strconv.Itoa(info.ID))
			// FIXME more info from dir?
			iErr = callback(d)
		} else {
			o, err := f.newObjectWithInfo(ctx, remote, info)
			if err != nil {
				iErr = err
				return true
			}
			iErr = callback(o)
		}
		if iErr != nil {
			return true
		}
		return false
	})
	if err != nil {
		return err
	}
	if iErr != nil {
		return iErr
	}
	return nil
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
	return list.WithListP(ctx, dir, f)
}

// ListP lists the objects and directories of the Fs starting
// from dir non recursively into out.
//
// dir should be "" to start from the root, and should not
// have trailing slashes.
//
// This should return ErrDirNotFound if the directory isn't
// found.
//
// It should call callback for each tranche of entries read.
// These need not be returned in any particular order.  If
// callback returns an error then the listing will stop
// immediately.
func (f *Fs) ListP(ctx context.Context, dir string, callback fs.ListRCallback) (err error) {
	list := list.NewHelper(callback)
	err = f.listHelper(ctx, dir, false, func(o fs.DirEntry) error {
		return list.Add(o)
	})
	if err != nil {
		return err
	}
	return list.Flush()
}

// ListR lists the objects and directories of the Fs starting
// from dir recursively into out.
func (f *Fs) ListR(ctx context.Context, dir string, callback fs.ListRCallback) (err error) {
	list := list.NewHelper(callback)
	err = f.listHelper(ctx, dir, true, func(o fs.DirEntry) error {
		return list.Add(o)
	})
	if err != nil {
		return err
	}
	return list.Flush()
}

// Creates from the parameters passed in a half finished Object which
// must have setMetaData called on it
//
// Returns the object, leaf, directoryID and error.
//
// Used to create new objects
func (f *Fs) createObject(ctx context.Context, remote string, modTime time.Time, size int64) (o *Object, leaf string, directoryID string, err error) {
	// Create the directory for the object if it doesn't exist
	fs.Debugf(ctx, "createObject: remote = %s", remote)
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
// Copy the reader in to the new object which is returned.
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
		Method:     "DELETE",
		Path:       fmt.Sprintf("/2/drive/%s/files/%s", f.opt.DriveID, rootID),
		Parameters: url.Values{},
	}
	var resp *http.Response
	var result api.CancellableResponse
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		err = result.ResultStatus.Update(err)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("rmdir failed: %w", err)
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

// Copy src to this remote using server-side copy operations.
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
	dstObj, leaf, directoryID, err := f.createObject(ctx, remote, srcObj.modTime, srcObj.size)
	if err != nil {
		return nil, err
	}

	// Copy the object
	opts := rest.Opts{
		Method:     "POST",
		Path:       fmt.Sprintf("/3/drive/%s/files/%s/copy/%s", f.opt.DriveID, srcObj.id, directoryID),
		Parameters: url.Values{},
	}
	opts.Parameters.Set("name", f.opt.Enc.FromStandardName(leaf))
	//opts.Parameters.Set("mtime", fmt.Sprintf("%d", uint64(srcObj.modTime.Unix())))
	var resp *http.Response
	var result api.FileCopyResponse
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		err = result.ResultStatus.Update(err)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	err = dstObj.setMetaData(&result.Data)
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
	opts := rest.Opts{
		Method:     "DELETE",
		Path:       fmt.Sprintf("/2/drive/%s/trash", f.opt.DriveID),
		Parameters: url.Values{},
	}
	var resp *http.Response
	var result api.ResultStatus
	var err error
	return f.pacer.Call(func() (bool, error) {
		resp, err = f.cleanupSrv.CallJSON(ctx, &opts, nil, &result)
		err = result.Update(err)
		return shouldRetry(ctx, resp, err)
	})
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

	// Create temporary object
	_, leaf, directoryID, err := f.createObject(ctx, remote, srcObj.modTime, srcObj.size)
	if err != nil {
		return nil, err
	}

	// Do the move
	opts := rest.Opts{
		Method:     "POST",
		Path:       fmt.Sprintf("/3/drive/%s/files/%s/move/%s", f.opt.DriveID, srcObj.id, directoryID),
		Parameters: url.Values{},
	}
	opts.Parameters.Set("name", f.opt.Enc.FromStandardName(leaf))
	var resp *http.Response
	var result api.CancellableResponse
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		err = result.ResultStatus.Update(err)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}

	dstObj, err := f.NewObject(ctx, remote)
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

	srcID, _, _, dstDirectoryID, dstLeaf, err := f.dirCache.DirMove(ctx, srcFs.dirCache, srcFs.root, srcRemote, f.root, dstRemote)
	if err != nil {
		return err
	}

	// Do the move
	opts := rest.Opts{
		Method:     "POST",
		Path:       fmt.Sprintf("/3/drive/%s/files/%s/move/%s", f.opt.DriveID, srcID, dstDirectoryID),
		Parameters: url.Values{},
	}
	opts.Parameters.Set("name", f.opt.Enc.FromStandardName(dstLeaf))
	var resp *http.Response
	var result api.CancellableResponse
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		err = result.ResultStatus.Update(err)
		return shouldRetry(ctx, resp, err)
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
		Method:     "GET",
		Path:       fmt.Sprintf("/2/drive/%s/files/%s/link", f.opt.DriveID, dirID),
		Parameters: url.Values{},
	}
	var result api.PubLinkResult
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, nil, &result)
		err = result.ResultStatus.Update(err)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return "", err
	}
	return result.Data.URL, err
}

func (f *Fs) linkFile(ctx context.Context, path string, expire fs.Duration) (string, error) {
	obj, err := f.NewObject(ctx, path)
	if err != nil {
		return "", err
	}
	o := obj.(*Object)
	opts := rest.Opts{
		Method:     "GET",
		Path:       fmt.Sprintf("/2/drive/%s/files/%s/link", f.opt.DriveID, o.id),
		Parameters: url.Values{},
	}
	var result api.PubLinkResult
	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, nil, &result)
		err = result.ResultStatus.Update(err)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return "", err
	}
	return result.Data.URL, nil
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
		Method:     "GET",
		Path:       fmt.Sprintf("/2/drive/%s", f.opt.DriveID),
		Parameters: url.Values{},
	}
	opts.Parameters.Set("only", "used_size,size")
	var resp *http.Response
	var q api.QuotaInfo
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &q)
		err = q.ResultStatus.Update(err)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	free := max(q.Data.Size-q.Data.UsedSize, 0)
	usage = &fs.Usage{
		Total: fs.NewUsageValue(q.Data.Size),     // quota of bytes that can be used
		Used:  fs.NewUsageValue(q.Data.UsedSize), // bytes in use
		Free:  fs.NewUsageValue(free),            // bytes which can be uploaded before reaching the quota
	}
	return usage, nil
}

// Shutdown shutdown the fs
func (f *Fs) Shutdown(ctx context.Context) error {
	//f.tokenRenewer.Shutdown()
	return nil
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.XXH3)
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
func (o *Object) retrieveHash(ctx context.Context) (err error) {
	var resp *http.Response
	var result api.ChecksumFileResult

	opts := rest.Opts{
		Method:     "GET",
		Path:       fmt.Sprintf("/2/drive/%s/files/%s/hash", o.fs.opt.DriveID, o.id),
		Parameters: url.Values{},
	}
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.CallJSON(ctx, &opts, nil, &result)
		err = result.ResultStatus.Update(err)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return err
	}
	o.setHash(strings.TrimPrefix(result.Data.Hash, "xxh3:"))
	return nil
}

// Hash returns the SHA-1 of an object returning a lowercase hex string
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	var pHash *string
	switch t {
	case hash.XXH3:
		pHash = &o.xxh3
	default:
		return "", hash.ErrUnsupported
	}
	if o.xxh3 == "" {
		err := o.retrieveHash(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to get hash: %w", err)
		}
	}
	return *pHash, nil
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
	if info.Type == "dir" {
		return fmt.Errorf("%q is a folder: %w", o.remote, fs.ErrorNotAFile)
	}
	o.hasMetaData = true
	o.size = info.Size
	o.modTime = info.ModTime()
	o.id = strconv.Itoa(info.ID)
	return nil
}

// setHash sets the hashes from that passed in
func (o *Object) setHash(hash string) {
	o.xxh3 = hash
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
	/*
		// TOFE: there isn't currently any way of setting mtime on the remote !
		filename, directoryID, err := o.fs.dirCache.FindPath(ctx, o.Remote(), true)
		if err != nil {
			return err
		}

		fileID := o.id
		filename = o.fs.opt.Enc.FromStandardName(filename)
		opts := rest.Opts{
			Method:           "PUT",
			Path:             "/copyfile",
			Parameters:       url.Values{},
			TransferEncoding: []string{"identity"}, // kdrive doesn't like chunked encoding
			ExtraHeaders: map[string]string{
				"Connection": "keep-alive",
			},
		}
		opts.Parameters.Set("fileid", fileID)
		opts.Parameters.Set("folderid", dirIDtoNumber(directoryID))
		opts.Parameters.Set("toname", filename)
		opts.Parameters.Set("tofolderid", dirIDtoNumber(directoryID))
		opts.Parameters.Set("ctime", strconv.FormatInt(modTime.Unix(), 10))
		opts.Parameters.Set("mtime", strconv.FormatInt(modTime.Unix(), 10))

		result := &api.ItemResult{}
		err = o.fs.pacer.CallNoRetry(func() (bool, error) {
			resp, err := o.fs.srv.CallJSON(ctx, &opts, nil, result)
			err = result.Error.Update(err)
			return shouldRetry(ctx, resp, err)
		})
		if err != nil {
			return fmt.Errorf("update mtime: copyfile: %w", err)
		}
		if err := o.setMetaData(&result.Metadata); err != nil {
			return err
		}
	*/

	return nil
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
	var resp *http.Response
	var result api.PubLinkResult
	opts := rest.Opts{
		Method:     "GET",
		Path:       fmt.Sprintf("/2/drive/%s/files/%s/link", o.fs.opt.DriveID, o.id),
		Parameters: url.Values{},
	}
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.CallJSON(ctx, &opts, nil, &result)
		err = result.ResultStatus.Update(err)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return "", err
	}
	return result.Data.URL, nil
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
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	return resp.Body, err
}

// Update the object with the contents of the io.Reader, modTime and size
//
// If existing is set then it updates the object rather than creating a new one.
//
// The new object may have been created if an error is returned
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	//o.fs.tokenRenewer.Start()
	//defer o.fs.tokenRenewer.Stop()

	size := src.Size() // NB can upload without size
	modTime := src.ModTime(ctx)
	remote := o.Remote()

	if size < 0 {
		return errors.New("can't upload unknown sizes objects")
	}

	// Create the directory for the object if it doesn't exist
	leaf, directoryID, err := o.fs.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return err
	}

	// This API doesn't support chunk uploads, so it's just for now
	var resp *http.Response
	var result api.UploadFileResponse
	opts := rest.Opts{
		Method:           "POST",
		Path:             fmt.Sprintf("/3/drive/%s/upload", o.fs.opt.DriveID),
		Body:             in,
		ContentType:      fs.MimeType(ctx, src),
		ContentLength:    &size,
		Parameters:       url.Values{},
		TransferEncoding: []string{"identity"}, // kdrive doesn't like chunked encoding
		Options:          options,
	}
	leaf = o.fs.opt.Enc.FromStandardName(leaf)
	opts.Parameters.Set("file_name", leaf)
	opts.Parameters.Set("directory_id", directoryID)
	opts.Parameters.Set("total_size", fmt.Sprintf("%d", size))
	opts.Parameters.Set("created_at", fmt.Sprintf("%d", uint64(modTime.Unix())))

	// Special treatment for a 0 length upload.  This doesn't work
	// with PUT even with Content-Length set (by setting
	// opts.Body=0), so upload it as a multipart form POST with
	// Content-Length set.
	if size == 0 {
		formReader, contentType, overhead, err := rest.MultipartUpload(ctx, in, opts.Parameters, "content", leaf)
		if err != nil {
			return fmt.Errorf("failed to make multipart upload for 0 length file: %w", err)
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
		err = result.ResultStatus.Update(err)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		// sometimes kdrive leaves a half complete file on
		// error, so delete it if it exists, trying a few times
		for range 5 {
			delObj, delErr := o.fs.NewObject(ctx, o.remote)
			if delErr == nil && delObj != nil {
				_ = delObj.Remove(ctx)
				break
			}
			time.Sleep(time.Second)
		}
		return err
	}
	if result.ResultStatus.IsError() {
		return fmt.Errorf("failed to upload %v - not sure why", o)
	}

	return o.readMetaData(ctx)
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	opts := rest.Opts{
		Method:     "DELETE",
		Path:       fmt.Sprintf("/2/drive/%s/files/%s", o.fs.opt.DriveID, o.id),
		Parameters: url.Values{},
	}
	var result api.CancellableResponse
	return o.fs.pacer.Call(func() (bool, error) {
		resp, err := o.fs.srv.CallJSON(ctx, &opts, nil, &result)
		err = result.ResultStatus.Update(err)
		return shouldRetry(ctx, resp, err)
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
	_ fs.ListRer         = (*Fs)(nil)
	_ fs.ListPer         = (*Fs)(nil)
	_ fs.Abouter         = (*Fs)(nil)
	_ fs.Shutdowner      = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.IDer            = (*Object)(nil)
)
