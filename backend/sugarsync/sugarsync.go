// Package sugarsync provides an interface to the Sugarsync
// object storage system.
package sugarsync

/* FIXME

DirMove tests fails with: Can not move sync folder.

go test -v -short -run TestIntegration/FsMkdir/FsPutFiles/FsDirMove -verbose -dump-bodies

To work around this we use the remote "TestSugarSync:Test" to test with.

*/

import (
	"context"
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

	"github.com/rclone/rclone/backend/sugarsync/api"
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
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
)

/*
maxFileLength = 16383
canWriteUnnormalized = true
canReadUnnormalized   = true
canReadRenormalized   = false
canStream = true
*/

const (
	appID                     = "/sc/9068489/215_1736969337"
	accessKeyID               = "OTA2ODQ4OTE1NzEzNDAwNTI4Njc"
	encryptedPrivateAccessKey = "JONdXuRLNSRI5ue2Cr-vn-5m_YxyMNq9yHRKUQevqo8uaZjH502Z-x1axhyqOa8cDyldGq08RfFxozo"
	minSleep                  = 10 * time.Millisecond
	maxSleep                  = 2 * time.Second
	decayConstant             = 2 // bigger for slower decay, exponential
	rootURL                   = "https://api.sugarsync.com"
	listChunks                = 500             // chunk size to read directory listings
	expiryLeeway              = 5 * time.Minute // time before the token expires to renew
)

// withDefault returns value but if value is "" then it returns defaultValue
func withDefault(key, defaultValue string) (value string) {
	if value == "" {
		value = defaultValue
	}
	return value
}

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "sugarsync",
		Description: "Sugarsync",
		NewFs:       NewFs,
		Config: func(ctx context.Context, name string, m configmap.Mapper, config fs.ConfigIn) (*fs.ConfigOut, error) {
			opt := new(Options)
			err := configstruct.Set(m, opt)
			if err != nil {
				return nil, fmt.Errorf("failed to read options: %w", err)
			}

			switch config.State {
			case "":
				if opt.RefreshToken == "" {
					return fs.ConfigGoto("username")
				}
				return fs.ConfigConfirm("refresh", true, "config_refresh", "Already have a token - refresh?")
			case "refresh":
				if config.Result == "false" {
					return nil, nil
				}
				return fs.ConfigGoto("username")
			case "username":
				return fs.ConfigInput("password", "config_username", "username (email address)")
			case "password":
				m.Set("username", config.Result)
				return fs.ConfigPassword("auth", "config_password", "Your Sugarsync password.\n\nOnly required during setup and will not be stored.")
			case "auth":
				username, _ := m.Get("username")
				m.Set("username", "")
				password := config.Result

				authRequest := api.AppAuthorization{
					Username:         username,
					Password:         obscure.MustReveal(password),
					Application:      withDefault(opt.AppID, appID),
					AccessKeyID:      withDefault(opt.AccessKeyID, accessKeyID),
					PrivateAccessKey: withDefault(opt.PrivateAccessKey, obscure.MustReveal(encryptedPrivateAccessKey)),
				}

				var resp *http.Response
				opts := rest.Opts{
					Method: "POST",
					Path:   "/app-authorization",
				}
				srv := rest.NewClient(fshttp.NewClient(ctx)).SetRoot(rootURL) //  FIXME

				// FIXME
				//err = f.pacer.Call(func() (bool, error) {
				resp, err = srv.CallXML(context.Background(), &opts, &authRequest, nil)
				//	return shouldRetry(ctx, resp, err)
				//})
				if err != nil {
					return nil, fmt.Errorf("failed to get token: %w", err)
				}
				opt.RefreshToken = resp.Header.Get("Location")
				m.Set("refresh_token", opt.RefreshToken)
				return nil, nil
			}
			return nil, fmt.Errorf("unknown state %q", config.State)
		}, Options: []fs.Option{{
			Name: "app_id",
			Help: "Sugarsync App ID.\n\nLeave blank to use rclone's.",
		}, {
			Name: "access_key_id",
			Help: "Sugarsync Access Key ID.\n\nLeave blank to use rclone's.",
		}, {
			Name: "private_access_key",
			Help: "Sugarsync Private Access Key.\n\nLeave blank to use rclone's.",
		}, {
			Name:    "hard_delete",
			Help:    "Permanently delete files if true\notherwise put them in the deleted files.",
			Default: false,
		}, {
			Name:     "refresh_token",
			Help:     "Sugarsync refresh token.\n\nLeave blank normally, will be auto configured by rclone.",
			Advanced: true,
		}, {
			Name:     "authorization",
			Help:     "Sugarsync authorization.\n\nLeave blank normally, will be auto configured by rclone.",
			Advanced: true,
		}, {
			Name:     "authorization_expiry",
			Help:     "Sugarsync authorization expiry.\n\nLeave blank normally, will be auto configured by rclone.",
			Advanced: true,
		}, {
			Name:     "user",
			Help:     "Sugarsync user.\n\nLeave blank normally, will be auto configured by rclone.",
			Advanced: true,
		}, {
			Name:     "root_id",
			Help:     "Sugarsync root id.\n\nLeave blank normally, will be auto configured by rclone.",
			Advanced: true,
		}, {
			Name:     "deleted_id",
			Help:     "Sugarsync deleted folder id.\n\nLeave blank normally, will be auto configured by rclone.",
			Advanced: true,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			Default: (encoder.Base |
				encoder.EncodeCtl |
				encoder.EncodeInvalidUtf8),
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	AppID               string               `config:"app_id"`
	AccessKeyID         string               `config:"access_key_id"`
	PrivateAccessKey    string               `config:"private_access_key"`
	HardDelete          bool                 `config:"hard_delete"`
	RefreshToken        string               `config:"refresh_token"`
	Authorization       string               `config:"authorization"`
	AuthorizationExpiry string               `config:"authorization_expiry"`
	User                string               `config:"user"`
	RootID              string               `config:"root_id"`
	DeletedID           string               `config:"deleted_id"`
	Enc                 encoder.MultiEncoder `config:"encoding"`
}

// Fs represents a remote sugarsync
type Fs struct {
	name       string             // name of this remote
	root       string             // the path we are working on
	opt        Options            // parsed options
	features   *fs.Features       // optional features
	srv        *rest.Client       // the connection to the one drive server
	dirCache   *dircache.DirCache // Map of directory path to directory id
	pacer      *fs.Pacer          // pacer for API calls
	m          configmap.Mapper   // config file access
	authMu     sync.Mutex         // used when doing authorization
	authExpiry time.Time          // time the authorization expires
}

// Object describes a sugarsync object
//
// Will definitely have info but maybe not meta
type Object struct {
	fs          *Fs       // what this object is part of
	remote      string    // The remote path
	hasMetaData bool      // whether info below has been set
	size        int64     // size of the object
	modTime     time.Time // modification time of the object
	id          string    // ID of the object
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
	return fmt.Sprintf("sugarsync root '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// parsePath parses a sugarsync 'url'
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
func (f *Fs) readMetaDataForPath(ctx context.Context, path string) (info *api.File, err error) {
	// defer fs.Trace(f, "path=%q", path)("info=%+v, err=%v", &info, &err)
	leaf, directoryID, err := f.dirCache.FindPath(ctx, path, false)
	if err != nil {
		if err == fs.ErrorDirNotFound {
			return nil, fs.ErrorObjectNotFound
		}
		return nil, err
	}

	found, err := f.listAll(ctx, directoryID, func(item *api.File) bool {
		if strings.EqualFold(item.Name, leaf) {
			info = item
			return true
		}
		return false
	}, nil)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fs.ErrorObjectNotFound
	}
	return info, nil
}

// readMetaDataForID reads the metadata for a file from the ID
func (f *Fs) readMetaDataForID(ctx context.Context, ID string) (info *api.File, err error) {
	var resp *http.Response
	opts := rest.Opts{
		Method:  "GET",
		RootURL: ID,
	}
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallXML(ctx, &opts, nil, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return nil, fs.ErrorObjectNotFound
		}
		return nil, fmt.Errorf("failed to get authorization: %w", err)
	}
	return info, nil
}

// getAuthToken gets an Auth token from the refresh token
func (f *Fs) getAuthToken(ctx context.Context) error {
	fs.Debugf(f, "Renewing token")

	var authRequest = api.TokenAuthRequest{
		AccessKeyID:      withDefault(f.opt.AccessKeyID, accessKeyID),
		PrivateAccessKey: withDefault(f.opt.PrivateAccessKey, obscure.MustReveal(encryptedPrivateAccessKey)),
		RefreshToken:     f.opt.RefreshToken,
	}

	if authRequest.RefreshToken == "" {
		return errors.New("no refresh token found - run `rclone config reconnect`")
	}

	var authResponse api.Authorization
	var err error
	var resp *http.Response
	opts := rest.Opts{
		Method: "POST",
		Path:   "/authorization",
		ExtraHeaders: map[string]string{
			"Authorization": "", // unset Authorization
		},
	}
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallXML(ctx, &opts, &authRequest, &authResponse)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("failed to get authorization: %w", err)
	}
	f.opt.Authorization = resp.Header.Get("Location")
	f.authExpiry = authResponse.Expiration
	f.opt.User = authResponse.User

	// Cache the results
	f.m.Set("authorization", f.opt.Authorization)
	f.m.Set("authorization_expiry", f.authExpiry.Format(time.RFC3339))
	f.m.Set("user", f.opt.User)
	return nil
}

// Read the auth from the config file and refresh it if it is expired, setting it in srv
func (f *Fs) getAuth(req *http.Request) (err error) {
	f.authMu.Lock()
	defer f.authMu.Unlock()
	ctx := req.Context()

	// if have auth, check it is in date
	if f.opt.Authorization == "" || f.opt.User == "" || f.authExpiry.IsZero() || time.Until(f.authExpiry) < expiryLeeway {
		// Get the auth token
		f.srv.SetSigner(nil) // temporarily remove the signer so we don't infinitely recurse
		err = f.getAuthToken(ctx)
		f.srv.SetSigner(f.getAuth) // replace signer
		if err != nil {
			return err
		}
	}

	// Set Authorization header
	req.Header.Set("Authorization", f.opt.Authorization)

	return nil
}

// Read the user info into f
func (f *Fs) getUser(ctx context.Context) (user *api.User, err error) {
	var resp *http.Response
	opts := rest.Opts{
		Method: "GET",
		Path:   "/user",
	}
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallXML(ctx, &opts, nil, &user)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return user, nil
}

// Read the expiry time from a string
func parseExpiry(expiryString string) time.Time {
	if expiryString == "" {
		return time.Time{}
	}
	expiry, err := time.Parse(time.RFC3339, expiryString)
	if err != nil {
		fs.Debugf("sugarsync", "Invalid expiry time %q read from config", expiryString)
		return time.Time{}
	}
	return expiry
}

// NewFs constructs an Fs from the path, container:path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	root = parsePath(root)
	client := fshttp.NewClient(ctx)
	f := &Fs{
		name:       name,
		root:       root,
		opt:        *opt,
		srv:        rest.NewClient(client).SetRoot(rootURL),
		pacer:      fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
		m:          m,
		authExpiry: parseExpiry(opt.AuthorizationExpiry),
	}
	f.features = (&fs.Features{
		CaseInsensitive:         true,
		CanHaveEmptyDirectories: true,
	}).Fill(ctx, f)
	f.srv.SetSigner(f.getAuth) // use signing hook to get the auth
	f.srv.SetErrorHandler(errorHandler)

	// Get rootID
	if f.opt.RootID == "" {
		user, err := f.getUser(ctx)
		if err != nil {
			return nil, err
		}
		f.opt.RootID = user.SyncFolders
		if strings.HasSuffix(f.opt.RootID, "/contents") {
			f.opt.RootID = f.opt.RootID[:len(f.opt.RootID)-9]
		} else {
			return nil, fmt.Errorf("unexpected rootID %q", f.opt.RootID)
		}
		// Cache the results
		f.m.Set("root_id", f.opt.RootID)
		f.opt.DeletedID = user.Deleted
		f.m.Set("deleted_id", f.opt.DeletedID)
	}
	f.dirCache = dircache.New(root, f.opt.RootID, f)

	// Find the current root
	err = f.dirCache.FindRoot(ctx, false)
	if err != nil {
		// Assume it is a file
		newRoot, remote := dircache.SplitPath(root)
		oldDirCache := f.dirCache
		f.dirCache = dircache.New(newRoot, f.opt.RootID, f)
		f.root = newRoot
		resetF := func() {
			f.dirCache = oldDirCache
			f.root = root
		}
		// Make new Fs which is the parent
		err = f.dirCache.FindRoot(ctx, false)
		if err != nil {
			// No root so return old f
			resetF()
			return f, nil
		}
		_, err := f.newObjectWithInfo(ctx, remote, nil)
		if err != nil {
			if err == fs.ErrorObjectNotFound {
				// File doesn't exist so return old f
				resetF()
				return f, nil
			}
			return nil, err
		}
		// return an error with an fs which points to the parent
		return f, fs.ErrorIsFile
	}
	return f, nil
}

var findError = regexp.MustCompile(`<h3>(.*?)</h3>`)

// errorHandler parses errors from the body
//
// Errors seem to be HTML with <h3> containing the error text
// <h3>Can not move sync folder.</h3>
func errorHandler(resp *http.Response) (err error) {
	body, err := rest.ReadBody(resp)
	if err != nil {
		return fmt.Errorf("error reading error out of body: %w", err)
	}
	match := findError.FindSubmatch(body)
	if match == nil || len(match) < 2 || len(match[1]) == 0 {
		return fmt.Errorf("HTTP error %v (%v) returned body: %q", resp.StatusCode, resp.Status, body)
	}
	return fmt.Errorf("HTTP error %v (%v): %s", resp.StatusCode, resp.Status, match[1])
}

// rootSlash returns root with a slash on if it is empty, otherwise empty string
func (f *Fs) rootSlash() string {
	if f.root == "" {
		return f.root
	}
	return f.root + "/"
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
	//fs.Debugf(f, "FindLeaf(%q, %q)", pathID, leaf)
	// Find the leaf in pathID
	found, err = f.listAll(ctx, pathID, nil, func(item *api.Collection) bool {
		if strings.EqualFold(item.Name, leaf) {
			pathIDOut = item.Ref
			return true
		}
		return false
	})
	// fs.Debugf(f, ">FindLeaf %q, %v, %v", pathIDOut, found, err)
	return pathIDOut, found, err
}

// CreateDir makes a directory with pathID as parent and name leaf
func (f *Fs) CreateDir(ctx context.Context, pathID, leaf string) (newID string, err error) {
	// fs.Debugf(f, "CreateDir(%q, %q)\n", pathID, leaf)
	var resp *http.Response
	opts := rest.Opts{
		Method:     "POST",
		RootURL:    pathID,
		NoResponse: true,
	}
	var mkdir interface{}
	if pathID == f.opt.RootID {
		// folders at the root are syncFolders
		mkdir = &api.CreateSyncFolder{
			Name: f.opt.Enc.FromStandardName(leaf),
		}
		opts.ExtraHeaders = map[string]string{
			"*X-SugarSync-API-Version": "1.5", // non canonical header
		}

	} else {
		mkdir = &api.CreateFolder{
			Name: f.opt.Enc.FromStandardName(leaf),
		}
	}
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallXML(ctx, &opts, mkdir, nil)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return "", err
	}
	newID = resp.Header.Get("Location")
	if newID == "" {
		// look up ID if not returned (e.g. for syncFolder)
		var found bool
		newID, found, err = f.FindLeaf(ctx, pathID, leaf)
		if err != nil {
			return "", err
		}
		if !found {
			return "", fmt.Errorf("couldn't find ID for newly created directory %q", leaf)
		}

	}
	return newID, nil
}

// list the objects into the function supplied
//
// Should return true to finish processing
type listAllFileFn func(*api.File) bool

// list the folders into the function supplied
//
// Should return true to finish processing
type listAllFolderFn func(*api.Collection) bool

// Lists the directory required calling the user function on each item found
//
// If the user fn ever returns true then it early exits with found = true
func (f *Fs) listAll(ctx context.Context, dirID string, fileFn listAllFileFn, folderFn listAllFolderFn) (found bool, err error) {
	opts := rest.Opts{
		Method:     "GET",
		RootURL:    dirID,
		Path:       "/contents",
		Parameters: url.Values{},
	}
	opts.Parameters.Set("max", strconv.Itoa(listChunks))
	start := 0
OUTER:
	for {
		opts.Parameters.Set("start", strconv.Itoa(start))

		var result api.CollectionContents
		var resp *http.Response
		err = f.pacer.Call(func() (bool, error) {
			resp, err = f.srv.CallXML(ctx, &opts, nil, &result)
			return shouldRetry(ctx, resp, err)
		})
		if err != nil {
			return found, fmt.Errorf("couldn't list files: %w", err)
		}
		if fileFn != nil {
			for i := range result.Files {
				item := &result.Files[i]
				item.Name = f.opt.Enc.ToStandardName(item.Name)
				if fileFn(item) {
					found = true
					break OUTER
				}
			}
		}
		if folderFn != nil {
			for i := range result.Collections {
				item := &result.Collections[i]
				item.Name = f.opt.Enc.ToStandardName(item.Name)
				if folderFn(item) {
					found = true
					break OUTER
				}
			}
		}
		if !result.HasMore {
			break
		}
		start = result.End + 1
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
	_, err = f.listAll(ctx, directoryID,
		func(info *api.File) bool {
			remote := path.Join(dir, info.Name)
			o, err := f.newObjectWithInfo(ctx, remote, info)
			if err != nil {
				iErr = err
				return true
			}
			entries = append(entries, o)
			return false
		},
		func(info *api.Collection) bool {
			remote := path.Join(dir, info.Name)
			id := info.Ref
			// cache the directory ID for later lookups
			f.dirCache.Put(remote, id)
			d := fs.NewDir(remote, info.TimeCreated).SetID(id)
			entries = append(entries, d)
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

// PutStream uploads to the remote path with the modTime given of indeterminate size
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.Put(ctx, in, src, options...)
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

// delete removes an object or directory by ID either putting it
// in the Deleted files or deleting it permanently
func (f *Fs) delete(ctx context.Context, isFile bool, id string, remote string, hardDelete bool) (err error) {
	if hardDelete {
		opts := rest.Opts{
			Method:     "DELETE",
			RootURL:    id,
			NoResponse: true,
		}
		return f.pacer.Call(func() (bool, error) {
			resp, err := f.srv.Call(ctx, &opts)
			return shouldRetry(ctx, resp, err)
		})
	}
	// Move file/dir to deleted files if not hard delete
	leaf := path.Base(remote)
	if isFile {
		_, err = f.moveFile(ctx, id, leaf, f.opt.DeletedID)
	} else {
		err = f.moveDir(ctx, id, leaf, f.opt.DeletedID)
	}
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
	directoryID, err := dc.FindDir(ctx, dir, false)
	if err != nil {
		return err
	}

	if check {
		found, err := f.listAll(ctx, directoryID, func(item *api.File) bool {
			return true
		}, func(item *api.Collection) bool {
			return true
		})
		if err != nil {
			return err
		}
		if found {
			return fs.ErrorDirectoryNotEmpty
		}
	}

	err = f.delete(ctx, false, directoryID, root, f.opt.HardDelete || check)
	if err != nil {
		return err
	}

	f.dirCache.FlushDir(dir)
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

// Copy src to this remote using server-side copy operations.
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

	srcPath := srcObj.fs.rootSlash() + srcObj.remote
	dstPath := f.rootSlash() + remote
	if strings.EqualFold(srcPath, dstPath) {
		return nil, fmt.Errorf("can't copy %q -> %q as are same name when lowercase", srcPath, dstPath)
	}

	// Create temporary object
	dstObj, leaf, directoryID, err := f.createObject(ctx, remote, srcObj.modTime, srcObj.size)
	if err != nil {
		return nil, err
	}

	// Copy the object
	opts := rest.Opts{
		Method:     "POST",
		RootURL:    directoryID,
		NoResponse: true,
	}
	copyFile := api.CopyFile{
		Name:   f.opt.Enc.FromStandardName(leaf),
		Source: srcObj.id,
	}
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallXML(ctx, &opts, &copyFile, nil)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	dstObj.id = resp.Header.Get("Location")
	err = dstObj.readMetaData(ctx)
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
	// Caution: Deleting a folder may orphan objects. It's important
	// to remove the contents of the folder before you delete the
	// folder. That's because removing a folder using DELETE does not
	// remove the objects contained within the folder. If you delete
	// a folder without first deleting its contents, the contents may
	// be rendered inaccessible.
	//
	// An alternative to permanently deleting a folder is moving it to the
	// Deleted Files folder. A folder (and all its contents) in the
	// Deleted Files folder can be recovered. Your app can retrieve the
	// link to the user's Deleted Files folder from the <deleted> element
	// in the user resource representation. Your application can then move
	// a folder to the Deleted Files folder by issuing an HTTP PUT request
	// to the URL that represents the file resource and provide as input,
	// XML that specifies in the <parent> element the link to the Deleted
	// Files folder.
	if f.opt.HardDelete {
		return fs.ErrorCantPurge
	}
	return f.purgeCheck(ctx, dir, false)
}

// moveFile moves a file server-side
func (f *Fs) moveFile(ctx context.Context, id, leaf, directoryID string) (info *api.File, err error) {
	opts := rest.Opts{
		Method:  "PUT",
		RootURL: id,
	}
	move := api.MoveFile{
		Name:   f.opt.Enc.FromStandardName(leaf),
		Parent: directoryID,
	}
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallXML(ctx, &opts, &move, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	// The docs say that there is nothing returned but apparently
	// there is... however it doesn't have Ref
	//
	// If ref not set, assume it hasn't changed
	if info.Ref == "" {
		info.Ref = id
	}
	return info, nil
}

// moveDir moves a folder server-side
func (f *Fs) moveDir(ctx context.Context, id, leaf, directoryID string) (err error) {
	// Move the object
	opts := rest.Opts{
		Method:     "PUT",
		RootURL:    id,
		NoResponse: true,
	}
	move := api.MoveFolder{
		Name:   f.opt.Enc.FromStandardName(leaf),
		Parent: directoryID,
	}
	var resp *http.Response
	return f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallXML(ctx, &opts, &move, nil)
		return shouldRetry(ctx, resp, err)
	})
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
	info, err := f.moveFile(ctx, srcObj.id, leaf, directoryID)
	if err != nil {
		return nil, err
	}

	err = dstObj.setMetaData(info)
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
	err = f.moveDir(ctx, srcID, dstLeaf, dstDirectoryID)
	if err != nil {
		return err
	}
	srcFs.dirCache.FlushDir(srcRemote)
	return nil
}

// PublicLink adds a "readable by anyone with link" permission on the given file or folder.
func (f *Fs) PublicLink(ctx context.Context, remote string, expire fs.Duration, unlink bool) (string, error) {
	obj, err := f.NewObject(ctx, remote)
	if err != nil {
		return "", err
	}
	o, ok := obj.(*Object)
	if !ok {
		return "", errors.New("internal error: not an Object")
	}
	opts := rest.Opts{
		Method:  "PUT",
		RootURL: o.id,
	}
	linkFile := api.SetPublicLink{
		PublicLink: api.PublicLink{Enabled: true},
	}
	var resp *http.Response
	var info *api.File
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallXML(ctx, &opts, &linkFile, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return "", err
	}
	return info.PublicLink.URL, err
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
func (o *Object) setMetaData(info *api.File) (err error) {
	o.hasMetaData = true
	o.size = info.Size
	o.modTime = info.LastModified
	if info.Ref != "" {
		o.id = info.Ref
	} else if o.id == "" {
		return errors.New("no ID found in response")
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
	var info *api.File
	if o.id != "" {
		info, err = o.fs.readMetaDataForID(ctx, o.id)
	} else {
		info, err = o.fs.readMetaDataForPath(ctx, o.remote)
	}
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
	// Sugarsync doesn't support setting the mod time.
	//
	// In theory (but not in the docs) you could patch the object,
	// however it doesn't work.
	return fs.ErrorCantSetModTime
}

// Storable returns a boolean showing whether this object storable
func (o *Object) Storable() bool {
	return true
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	if o.id == "" {
		return nil, errors.New("can't download - no id")
	}
	fs.FixRangeOption(options, o.size)
	var resp *http.Response
	opts := rest.Opts{
		Method:  "GET",
		RootURL: o.id,
		Path:    "/data",
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

// createFile makes an (empty) file with pathID as parent and name leaf and returns the ID
func (f *Fs) createFile(ctx context.Context, pathID, leaf, mimeType string) (newID string, err error) {
	var resp *http.Response
	opts := rest.Opts{
		Method:     "POST",
		RootURL:    pathID,
		NoResponse: true,
	}
	mkdir := api.CreateFile{
		Name:      f.opt.Enc.FromStandardName(leaf),
		MediaType: mimeType,
	}
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallXML(ctx, &opts, &mkdir, nil)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return "", err
	}
	return resp.Header.Get("Location"), nil
}

// Update the object with the contents of the io.Reader, modTime and size
//
// If existing is set then it updates the object rather than creating a new one
//
// The new object may have been created if an error is returned
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	size := src.Size()
	// modTime := src.ModTime(ctx)
	remote := o.Remote()

	// Create the directory for the object if it doesn't exist
	leaf, directoryID, err := o.fs.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return err
	}

	// if file doesn't exist, create it
	if o.id == "" {
		o.id, err = o.fs.createFile(ctx, directoryID, leaf, fs.MimeType(ctx, src))
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}
		if o.id == "" {
			return errors.New("failed to create file: no ID")
		}
		// if created the file and returning an error then delete the file
		defer func() {
			if err != nil {
				delErr := o.fs.delete(ctx, true, o.id, remote, o.fs.opt.HardDelete)
				if delErr != nil {
					fs.Errorf(o, "failed to remove failed upload: %v", delErr)
				}
			}
		}()
	}

	var resp *http.Response
	opts := rest.Opts{
		Method:     "PUT",
		RootURL:    o.id,
		Path:       "/data",
		NoResponse: true,
		Options:    options,
		Body:       in,
	}
	if size >= 0 {
		opts.ContentLength = &size
	}
	err = o.fs.pacer.CallNoRetry(func() (bool, error) {
		resp, err = o.fs.srv.Call(ctx, &opts)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}

	o.hasMetaData = false
	return o.readMetaData(ctx)
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	return o.fs.delete(ctx, true, o.id, o.remote, o.fs.opt.HardDelete)
}

// ID returns the ID of the Object if known, or "" if not
func (o *Object) ID() string {
	return o.id
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = (*Fs)(nil)
	_ fs.Purger          = (*Fs)(nil)
	_ fs.PutStreamer     = (*Fs)(nil)
	_ fs.Copier          = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ fs.DirCacheFlusher = (*Fs)(nil)
	_ fs.PublicLinker    = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.IDer            = (*Object)(nil)
)
