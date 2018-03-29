// Package onedrive provides an interface to the Microsoft OneDrive
// object storage system.
package onedrive

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/ncw/rclone/backend/onedrive/api"
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/config"
	"github.com/ncw/rclone/fs/config/flags"
	"github.com/ncw/rclone/fs/config/obscure"
	"github.com/ncw/rclone/fs/fserrors"
	"github.com/ncw/rclone/fs/hash"
	"github.com/ncw/rclone/lib/dircache"
	"github.com/ncw/rclone/lib/oauthutil"
	"github.com/ncw/rclone/lib/pacer"
	"github.com/ncw/rclone/lib/readers"
	"github.com/ncw/rclone/lib/rest"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

const (
	rclonePersonalClientID              = "0000000044165769"
	rclonePersonalEncryptedClientSecret = "ugVWLNhKkVT1-cbTRO-6z1MlzwdW6aMwpKgNaFG-qXjEn_WfDnG9TVyRA5yuoliU"
	rcloneBusinessClientID              = "52857fec-4bc2-483f-9f1b-5fe28e97532c"
	rcloneBusinessEncryptedClientSecret = "6t4pC8l6L66SFYVIi8PgECDyjXy_ABo1nsTaE-Lr9LpzC6yT4vNOwHsakwwdEui0O6B0kX8_xbBLj91J"
	minSleep                            = 10 * time.Millisecond
	maxSleep                            = 2 * time.Second
	decayConstant                       = 2                                     // bigger for slower decay, exponential
	rootURLPersonal                     = "https://api.onedrive.com/v1.0/drive" // root URL for requests
	discoveryServiceURL                 = "https://api.office.com/discovery/"
	configResourceURL                   = "resource_url"
)

// Globals
var (
	// Description of how to auth for this app for a personal account
	oauthPersonalConfig = &oauth2.Config{
		Scopes: []string{
			"wl.signin",          // Allow single sign-on capabilities
			"wl.offline_access",  // Allow receiving a refresh token
			"onedrive.readwrite", // r/w perms to all of a user's OneDrive files
		},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://login.live.com/oauth20_authorize.srf",
			TokenURL: "https://login.live.com/oauth20_token.srf",
		},
		ClientID:     rclonePersonalClientID,
		ClientSecret: obscure.MustReveal(rclonePersonalEncryptedClientSecret),
		RedirectURL:  oauthutil.RedirectLocalhostURL,
	}

	// Description of how to auth for this app for a business account
	oauthBusinessConfig = &oauth2.Config{
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://login.microsoftonline.com/common/oauth2/authorize",
			TokenURL: "https://login.microsoftonline.com/common/oauth2/token",
		},
		ClientID:     rcloneBusinessClientID,
		ClientSecret: obscure.MustReveal(rcloneBusinessEncryptedClientSecret),
		RedirectURL:  oauthutil.RedirectLocalhostURL,
	}
	oauthBusinessResource = oauth2.SetAuthURLParam("resource", discoveryServiceURL)

	chunkSize = fs.SizeSuffix(10 * 1024 * 1024)
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "onedrive",
		Description: "Microsoft OneDrive",
		NewFs:       NewFs,
		Config: func(name string) {
			// choose account type
			fmt.Printf("Choose OneDrive account type?\n")
			fmt.Printf(" * Say b for a OneDrive business account\n")
			fmt.Printf(" * Say p for a personal OneDrive account\n")
			isPersonal := config.Command([]string{"bBusiness", "pPersonal"}) == 'p'

			if isPersonal {
				// for personal accounts we don't safe a field about the account
				err := oauthutil.Config("onedrive", name, oauthPersonalConfig)
				if err != nil {
					log.Fatalf("Failed to configure token: %v", err)
				}
			} else {
				err := oauthutil.Config("onedrive", name, oauthBusinessConfig, oauthBusinessResource)
				if err != nil {
					log.Fatalf("Failed to configure token: %v", err)
					return
				}

				// Are we running headless?
				if config.FileGet(name, config.ConfigAutomatic) != "" {
					// Yes, okay we are done
					return
				}

				type serviceResource struct {
					ServiceAPIVersion  string `json:"serviceApiVersion"`
					ServiceEndpointURI string `json:"serviceEndpointUri"`
					ServiceResourceID  string `json:"serviceResourceId"`
				}
				type serviceResponse struct {
					Services []serviceResource `json:"value"`
				}

				oAuthClient, _, err := oauthutil.NewClient(name, oauthBusinessConfig)
				if err != nil {
					log.Fatalf("Failed to configure OneDrive: %v", err)
					return
				}
				srv := rest.NewClient(oAuthClient)

				opts := rest.Opts{
					Method:  "GET",
					RootURL: discoveryServiceURL,
					Path:    "/v2.0/me/services",
				}
				services := serviceResponse{}
				resp, err := srv.CallJSON(&opts, nil, &services)
				if err != nil {
					fs.Errorf(nil, "Failed to query available services: %v", err)
					return
				}
				if resp.StatusCode != 200 {
					fs.Errorf(nil, "Failed to query available services: Got HTTP error code %d", resp.StatusCode)
					return
				}

				var resourcesURL []string
				var resourcesID []string

				for _, service := range services.Services {
					if service.ServiceAPIVersion == "v2.0" {
						resourcesID = append(resourcesID, service.ServiceResourceID)
						resourcesURL = append(resourcesURL, service.ServiceEndpointURI)
					}
					// we only support 2.0 API
					fs.Infof(nil, "Skipping API %s endpoint %s", service.ServiceAPIVersion, service.ServiceEndpointURI)
				}

				var foundService string
				if len(resourcesID) == 0 {
					fs.Errorf(nil, "No Service found")
					return
				} else if len(resourcesID) == 1 {
					foundService = resourcesID[0]
				} else {
					foundService = config.Choose("Choose resource URL", resourcesID, resourcesURL, false)
				}

				config.FileSet(name, configResourceURL, foundService)
				oauthBusinessResource = oauth2.SetAuthURLParam("resource", foundService)

				// get the token from the inital config
				// we need to update the token with a resource
				// specific token we will query now
				token, err := oauthutil.GetToken(name)
				if err != nil {
					fs.Errorf(nil, "Error while getting token: %s", err)
					return
				}

				// values for the token query
				values := url.Values{}
				values.Set("refresh_token", token.RefreshToken)
				values.Set("grant_type", "refresh_token")
				values.Set("resource", foundService)
				values.Set("client_id", oauthBusinessConfig.ClientID)
				values.Set("client_secret", oauthBusinessConfig.ClientSecret)
				opts = rest.Opts{
					Method:      "POST",
					RootURL:     oauthBusinessConfig.Endpoint.TokenURL,
					ContentType: "application/x-www-form-urlencoded",
					Body:        strings.NewReader(values.Encode()),
				}

				// tokenJSON is the struct representing the HTTP response from OAuth2
				// providers returning a token in JSON form.
				// we are only interested in the new tokens, all other fields we don't care
				type tokenJSON struct {
					AccessToken  string `json:"access_token"`
					RefreshToken string `json:"refresh_token"`
				}
				jsonToken := tokenJSON{}
				resp, err = srv.CallJSON(&opts, nil, &jsonToken)
				if err != nil {
					fs.Errorf(nil, "Failed to get resource token: %v", err)
					return
				}
				if resp.StatusCode != 200 {
					fs.Errorf(nil, "Failed to get resource token: Got HTTP error code %d", resp.StatusCode)
					return
				}

				// update the tokens
				token.AccessToken = jsonToken.AccessToken
				token.RefreshToken = jsonToken.RefreshToken

				// finally save them in the config
				err = oauthutil.PutToken(name, token, true)
				if err != nil {
					fs.Errorf(nil, "Error while setting token: %s", err)
				}
			}
		},
		Options: []fs.Option{{
			Name: config.ConfigClientID,
			Help: "Microsoft App Client Id - leave blank normally.",
		}, {
			Name: config.ConfigClientSecret,
			Help: "Microsoft App Client Secret - leave blank normally.",
		}},
	})

	flags.VarP(&chunkSize, "onedrive-chunk-size", "", "Above this size files will be chunked - must be multiple of 320k.")
}

// Fs represents a remote one drive
type Fs struct {
	name         string             // name of this remote
	root         string             // the path we are working on
	features     *fs.Features       // optional features
	srv          *rest.Client       // the connection to the one drive server
	dirCache     *dircache.DirCache // Map of directory path to directory id
	pacer        *pacer.Pacer       // pacer for API calls
	tokenRenewer *oauthutil.Renew   // renew the token on expiry
	isBusiness   bool               // true if this is an OneDrive Business account
}

// Object describes a one drive object
//
// Will definitely have info but maybe not meta
type Object struct {
	fs          *Fs       // what this object is part of
	remote      string    // The remote path
	hasMetaData bool      // whether info below has been set
	size        int64     // size of the object
	modTime     time.Time // modification time of the object
	id          string    // ID of the object
	sha1        string    // SHA-1 of the object content
	mimeType    string    // Content-Type of object from server (may not be as uploaded)
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
	return fmt.Sprintf("One drive root '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Pattern to match a one drive path
var matcher = regexp.MustCompile(`^([^/]*)(.*)$`)

// parsePath parses an one drive 'url'
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
	authRety := false

	if resp != nil && resp.StatusCode == 401 && len(resp.Header["Www-Authenticate"]) == 1 && strings.Index(resp.Header["Www-Authenticate"][0], "expired_token") >= 0 {
		authRety = true
		fs.Debugf(nil, "Should retry: %v", err)
	}
	return authRety || fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// readMetaDataForPath reads the metadata from the path
func (f *Fs) readMetaDataForPath(path string) (info *api.Item, resp *http.Response, err error) {
	opts := rest.Opts{
		Method: "GET",
		Path:   "/root:/" + rest.URLPathEscape(replaceReservedChars(path)),
	}
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(&opts, nil, &info)
		return shouldRetry(resp, err)
	})
	return info, resp, err
}

// errorHandler parses a non 2xx error response into an error
func errorHandler(resp *http.Response) error {
	// Decode error response
	errResponse := new(api.Error)
	err := rest.DecodeJSON(resp, &errResponse)
	if err != nil {
		fs.Debugf(nil, "Couldn't decode error response: %v", err)
	}
	if errResponse.ErrorInfo.Code == "" {
		errResponse.ErrorInfo.Code = resp.Status
	}
	return errResponse
}

// NewFs constructs an Fs from the path, container:path
func NewFs(name, root string) (fs.Fs, error) {
	// get the resource URL from the config file0
	resourceURL := config.FileGet(name, configResourceURL, "")
	// if we have a resource URL it's a business account otherwise a personal one
	var rootURL string
	var oauthConfig *oauth2.Config
	if resourceURL == "" {
		// personal account setup
		oauthConfig = oauthPersonalConfig
		rootURL = rootURLPersonal
	} else {
		// business account setup
		oauthConfig = oauthBusinessConfig
		rootURL = resourceURL + "_api/v2.0/drives/me"

		// update the URL in the AuthOptions
		oauthBusinessResource = oauth2.SetAuthURLParam("resource", resourceURL)
	}
	root = parsePath(root)
	oAuthClient, ts, err := oauthutil.NewClient(name, oauthConfig)
	if err != nil {
		log.Fatalf("Failed to configure OneDrive: %v", err)
	}

	f := &Fs{
		name:       name,
		root:       root,
		srv:        rest.NewClient(oAuthClient).SetRoot(rootURL),
		pacer:      pacer.New().SetMinSleep(minSleep).SetMaxSleep(maxSleep).SetDecayConstant(decayConstant),
		isBusiness: resourceURL != "",
	}
	f.features = (&fs.Features{
		CaseInsensitive: true,
		// OneDrive for business doesn't support mime types properly
		// so we disable it until resolved
		// https://github.com/OneDrive/onedrive-api-docs/issues/643
		ReadMimeType:            !f.isBusiness,
		CanHaveEmptyDirectories: true,
	}).Fill(f)
	f.srv.SetErrorHandler(errorHandler)

	// Renew the token in the background
	f.tokenRenewer = oauthutil.NewRenew(f.String(), ts, func() error {
		_, _, err := f.readMetaDataForPath("")
		return err
	})

	// Get rootID
	rootInfo, _, err := f.readMetaDataForPath("")
	if err != nil || rootInfo.ID == "" {
		return nil, errors.Wrap(err, "failed to get root")
	}

	f.dirCache = dircache.New(root, rootInfo.ID, f)

	// Find the current root
	err = f.dirCache.FindRoot(false)
	if err != nil {
		// Assume it is a file
		newRoot, remote := dircache.SplitPath(root)
		newF := *f
		newF.dirCache = dircache.New(newRoot, rootInfo.ID, &newF)
		newF.root = newRoot
		// Make new Fs which is the parent
		err = newF.dirCache.FindRoot(false)
		if err != nil {
			// No root so return old f
			return f, nil
		}
		_, err := newF.newObjectWithInfo(remote, nil)
		if err != nil {
			if err == fs.ErrorObjectNotFound {
				// File doesn't exist so return old f
				return f, nil
			}
			return nil, err
		}
		// return an error with an fs which points to the parent
		return &newF, fs.ErrorIsFile
	}
	return f, nil
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
func (f *Fs) newObjectWithInfo(remote string, info *api.Item) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	var err error
	if info != nil {
		// Set info
		err = o.setMetaData(info)
	} else {
		err = o.readMetaData() // reads info and meta, returning an error
	}
	if err != nil {
		return nil, err
	}
	return o, nil
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(remote string) (fs.Object, error) {
	return f.newObjectWithInfo(remote, nil)
}

// FindLeaf finds a directory of name leaf in the folder with ID pathID
func (f *Fs) FindLeaf(pathID, leaf string) (pathIDOut string, found bool, err error) {
	// fs.Debugf(f, "FindLeaf(%q, %q)", pathID, leaf)
	parent, ok := f.dirCache.GetInv(pathID)
	if !ok {
		return "", false, errors.New("couldn't find parent ID")
	}
	path := leaf
	if parent != "" {
		path = parent + "/" + path
	}
	if f.dirCache.FoundRoot() {
		path = f.rootSlash() + path
	}
	info, resp, err := f.readMetaDataForPath(path)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return "", false, nil
		}
		return "", false, err
	}
	if info.Folder == nil {
		return "", false, errors.New("found file when looking for folder")
	}
	return info.ID, true, nil
}

// CreateDir makes a directory with pathID as parent and name leaf
func (f *Fs) CreateDir(pathID, leaf string) (newID string, err error) {
	// fs.Debugf(f, "CreateDir(%q, %q)\n", pathID, leaf)
	var resp *http.Response
	var info *api.Item
	opts := rest.Opts{
		Method: "POST",
		Path:   "/items/" + pathID + "/children",
	}
	mkdir := api.CreateItemRequest{
		Name:             replaceReservedChars(leaf),
		ConflictBehavior: "fail",
	}
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(&opts, &mkdir, &info)
		return shouldRetry(resp, err)
	})
	if err != nil {
		//fmt.Printf("...Error %v\n", err)
		return "", err
	}
	//fmt.Printf("...Id %q\n", *info.Id)
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
func (f *Fs) listAll(dirID string, directoriesOnly bool, filesOnly bool, fn listAllFn) (found bool, err error) {
	// Top parameter asks for bigger pages of data
	// https://dev.onedrive.com/odata/optional-query-parameters.htm
	opts := rest.Opts{
		Method: "GET",
		Path:   "/items/" + dirID + "/children?top=1000",
	}
OUTER:
	for {
		var result api.ListChildrenResponse
		var resp *http.Response
		err = f.pacer.Call(func() (bool, error) {
			resp, err = f.srv.CallJSON(&opts, nil, &result)
			return shouldRetry(resp, err)
		})
		if err != nil {
			return found, errors.Wrap(err, "couldn't list files")
		}
		if len(result.Value) == 0 {
			break
		}
		for i := range result.Value {
			item := &result.Value[i]
			isFolder := item.Folder != nil
			if isFolder {
				if filesOnly {
					continue
				}
			} else {
				if directoriesOnly {
					continue
				}
			}
			if item.Deleted != nil {
				continue
			}
			item.Name = restoreReservedChars(item.Name)
			if fn(item) {
				found = true
				break OUTER
			}
		}
		if result.NextLink == "" {
			break
		}
		opts.Path = ""
		opts.RootURL = result.NextLink
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
func (f *Fs) List(dir string) (entries fs.DirEntries, err error) {
	err = f.dirCache.FindRoot(false)
	if err != nil {
		return nil, err
	}
	directoryID, err := f.dirCache.FindDir(dir, false)
	if err != nil {
		return nil, err
	}
	var iErr error
	_, err = f.listAll(directoryID, false, false, func(info *api.Item) bool {
		remote := path.Join(dir, info.Name)
		if info.Folder != nil {
			// cache the directory ID for later lookups
			f.dirCache.Put(remote, info.ID)
			d := fs.NewDir(remote, time.Time(info.LastModifiedDateTime)).SetID(info.ID)
			if info.Folder != nil {
				d.SetItems(info.Folder.ChildCount)
			}
			entries = append(entries, d)
		} else {
			o, err := f.newObjectWithInfo(remote, info)
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
func (f *Fs) createObject(remote string, modTime time.Time, size int64) (o *Object, leaf string, directoryID string, err error) {
	// Create the directory for the object if it doesn't exist
	leaf, directoryID, err = f.dirCache.FindRootAndPath(remote, true)
	if err != nil {
		return nil, leaf, directoryID, err
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
func (f *Fs) Put(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	remote := src.Remote()
	size := src.Size()
	modTime := src.ModTime()

	o, _, _, err := f.createObject(remote, modTime, size)
	if err != nil {
		return nil, err
	}
	return o, o.Update(in, src, options...)
}

// Mkdir creates the container if it doesn't exist
func (f *Fs) Mkdir(dir string) error {
	err := f.dirCache.FindRoot(true)
	if err != nil {
		return err
	}
	if dir != "" {
		_, err = f.dirCache.FindDir(dir, true)
	}
	return err
}

// deleteObject removes an object by ID
func (f *Fs) deleteObject(id string) error {
	opts := rest.Opts{
		Method:     "DELETE",
		Path:       "/items/" + id,
		NoResponse: true,
	}
	return f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.Call(&opts)
		return shouldRetry(resp, err)
	})
}

// purgeCheck removes the root directory, if check is set then it
// refuses to do so if it has anything in
func (f *Fs) purgeCheck(dir string, check bool) error {
	root := path.Join(f.root, dir)
	if root == "" {
		return errors.New("can't purge root directory")
	}
	dc := f.dirCache
	err := dc.FindRoot(false)
	if err != nil {
		return err
	}
	rootID, err := dc.FindDir(dir, false)
	if err != nil {
		return err
	}
	item, _, err := f.readMetaDataForPath(root)
	if err != nil {
		return err
	}
	if item.Folder == nil {
		return errors.New("not a folder")
	}
	if check && item.Folder.ChildCount != 0 {
		return errors.New("folder not empty")
	}
	err = f.deleteObject(rootID)
	if err != nil {
		return err
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
func (f *Fs) Rmdir(dir string) error {
	return f.purgeCheck(dir, true)
}

// Precision return the precision of this Fs
func (f *Fs) Precision() time.Duration {
	return time.Second
}

// waitForJob waits for the job with status in url to complete
func (f *Fs) waitForJob(location string, o *Object) error {
	deadline := time.Now().Add(fs.Config.Timeout)
	for time.Now().Before(deadline) {
		opts := rest.Opts{
			Method:       "GET",
			RootURL:      location,
			IgnoreStatus: true, // Ignore the http status response since it seems to return valid info on 500 errors
		}
		var resp *http.Response
		var err error
		var body []byte
		err = f.pacer.Call(func() (bool, error) {
			resp, err = f.srv.Call(&opts)
			if err != nil {
				return fserrors.ShouldRetry(err), err
			}
			body, err = rest.ReadBody(resp)
			return fserrors.ShouldRetry(err), err
		})
		if err != nil {
			return err
		}
		// Try to decode the body first as an api.AsyncOperationStatus
		var status api.AsyncOperationStatus
		err = json.Unmarshal(body, &status)
		if err != nil {
			return errors.Wrapf(err, "async status result not JSON: %q", body)
		}
		// See if we decoded anything...
		if !(status.Operation == "" && status.PercentageComplete == 0 && status.Status == "") {
			if status.Status == "failed" || status.Status == "deleteFailed" {
				return errors.Errorf("%s: async operation %q returned %q", o.remote, status.Operation, status.Status)
			}
		} else if resp.StatusCode == 200 {
			var info api.Item
			err = json.Unmarshal(body, &info)
			if err != nil {
				return errors.Wrapf(err, "async item result not JSON: %q", body)
			}
			return o.setMetaData(&info)
		}
		time.Sleep(1 * time.Second)
	}
	return errors.Errorf("async operation didn't complete after %v", fs.Config.Timeout)
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
func (f *Fs) Copy(src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}
	err := srcObj.readMetaData()
	if err != nil {
		return nil, err
	}

	srcPath := srcObj.fs.rootSlash() + srcObj.remote
	dstPath := f.rootSlash() + remote
	if strings.ToLower(srcPath) == strings.ToLower(dstPath) {
		return nil, errors.Errorf("can't copy %q -> %q as are same name when lowercase", srcPath, dstPath)
	}

	// Create temporary object
	dstObj, leaf, directoryID, err := f.createObject(remote, srcObj.modTime, srcObj.size)
	if err != nil {
		return nil, err
	}

	// Copy the object
	opts := rest.Opts{
		Method:       "POST",
		Path:         "/items/" + srcObj.id + "/action.copy",
		ExtraHeaders: map[string]string{"Prefer": "respond-async"},
		NoResponse:   true,
	}
	replacedLeaf := replaceReservedChars(leaf)
	copy := api.CopyItemRequest{
		Name: &replacedLeaf,
		ParentReference: api.ItemReference{
			ID: directoryID,
		},
	}
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(&opts, &copy, nil)
		return shouldRetry(resp, err)
	})
	if err != nil {
		return nil, err
	}

	// read location header
	location := resp.Header.Get("Location")
	if location == "" {
		return nil, errors.New("didn't receive location header in copy response")
	}

	// Wait for job to finish
	err = f.waitForJob(location, dstObj)
	if err != nil {
		return nil, err
	}

	// Copy does NOT copy the modTime from the source and there seems to
	// be no way to set date before
	// This will create TWO versions on OneDrive
	err = dstObj.SetModTime(srcObj.ModTime())
	if err != nil {
		return nil, err
	}

	return dstObj, nil
}

// Purge deletes all the files and the container
//
// Optional interface: Only implement this if you have a way of
// deleting all the files quicker than just running Remove() on the
// result of List()
func (f *Fs) Purge() error {
	return f.purgeCheck("", false)
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
func (f *Fs) Move(src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't move - not same remote type")
		return nil, fs.ErrorCantMove
	}

	// Create temporary object
	dstObj, leaf, directoryID, err := f.createObject(remote, srcObj.modTime, srcObj.size)
	if err != nil {
		return nil, err
	}

	// Move the object
	opts := rest.Opts{
		Method: "PATCH",
		Path:   "/items/" + srcObj.id,
	}
	move := api.MoveItemRequest{
		Name: replaceReservedChars(leaf),
		ParentReference: &api.ItemReference{
			ID: directoryID,
		},
		// We set the mod time too as it gets reset otherwise
		FileSystemInfo: &api.FileSystemInfoFacet{
			CreatedDateTime:      api.Timestamp(srcObj.modTime),
			LastModifiedDateTime: api.Timestamp(srcObj.modTime),
		},
	}
	var resp *http.Response
	var info api.Item
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(&opts, &move, &info)
		return shouldRetry(resp, err)
	})
	if err != nil {
		return nil, err
	}

	err = dstObj.setMetaData(&info)
	if err != nil {
		return nil, err
	}
	return dstObj, nil
}

// DirCacheFlush resets the directory cache - used in testing as an
// optional interface
func (f *Fs) DirCacheFlush() {
	f.dirCache.ResetRoot()
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.SHA1)
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

// srvPath returns a path for use in server
func (o *Object) srvPath() string {
	return replaceReservedChars(o.fs.rootSlash() + o.remote)
}

// Hash returns the SHA-1 of an object returning a lowercase hex string
func (o *Object) Hash(t hash.Type) (string, error) {
	if t != hash.SHA1 {
		return "", hash.ErrUnsupported
	}
	return o.sha1, nil
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	err := o.readMetaData()
	if err != nil {
		fs.Logf(o, "Failed to read metadata: %v", err)
		return 0
	}
	return o.size
}

// setMetaData sets the metadata from info
func (o *Object) setMetaData(info *api.Item) (err error) {
	if info.Folder != nil {
		return errors.Wrapf(fs.ErrorNotAFile, "%q", o.remote)
	}
	o.hasMetaData = true
	o.size = info.Size

	// Docs: https://dev.onedrive.com/facets/hashes_facet.htm
	//
	// The docs state both that the hashes are returned as hex
	// strings, and as base64 strings. Testing reveals they are in
	// fact uppercase hex strings.
	//
	// In OneDrive for Business, SHA1 and CRC32 hash values are not returned for files.
	if info.File != nil {
		o.mimeType = info.File.MimeType
		if info.File.Hashes.Sha1Hash != "" {
			o.sha1 = strings.ToLower(info.File.Hashes.Sha1Hash)
		}
	}
	if info.FileSystemInfo != nil {
		o.modTime = time.Time(info.FileSystemInfo.LastModifiedDateTime)
	} else {
		o.modTime = time.Time(info.LastModifiedDateTime)
	}
	o.id = info.ID
	return nil
}

// readMetaData gets the metadata if it hasn't already been fetched
//
// it also sets the info
func (o *Object) readMetaData() (err error) {
	if o.hasMetaData {
		return nil
	}
	info, _, err := o.fs.readMetaDataForPath(o.srvPath())
	if err != nil {
		if apiErr, ok := err.(*api.Error); ok {
			if apiErr.ErrorInfo.Code == "itemNotFound" {
				return fs.ErrorObjectNotFound
			}
		}
		return err
	}
	return o.setMetaData(info)
}

// ModTime returns the modification time of the object
//
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *Object) ModTime() time.Time {
	err := o.readMetaData()
	if err != nil {
		fs.Logf(o, "Failed to read metadata: %v", err)
		return time.Now()
	}
	return o.modTime
}

// setModTime sets the modification time of the local fs object
func (o *Object) setModTime(modTime time.Time) (*api.Item, error) {
	opts := rest.Opts{
		Method: "PATCH",
		Path:   "/root:/" + rest.URLPathEscape(o.srvPath()),
	}
	update := api.SetFileSystemInfo{
		FileSystemInfo: api.FileSystemInfoFacet{
			CreatedDateTime:      api.Timestamp(modTime),
			LastModifiedDateTime: api.Timestamp(modTime),
		},
	}
	var info *api.Item
	err := o.fs.pacer.Call(func() (bool, error) {
		resp, err := o.fs.srv.CallJSON(&opts, &update, &info)
		return shouldRetry(resp, err)
	})
	return info, err
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(modTime time.Time) error {
	info, err := o.setModTime(modTime)
	if err != nil {
		return err
	}
	return o.setMetaData(info)
}

// Storable returns a boolean showing whether this object storable
func (o *Object) Storable() bool {
	return true
}

// Open an object for read
func (o *Object) Open(options ...fs.OpenOption) (in io.ReadCloser, err error) {
	if o.id == "" {
		return nil, errors.New("can't download - no id")
	}
	fs.FixRangeOption(options, o.size)
	var resp *http.Response
	opts := rest.Opts{
		Method:  "GET",
		Path:    "/items/" + o.id + "/content",
		Options: options,
	}
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.Call(&opts)
		return shouldRetry(resp, err)
	})
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusOK && resp.ContentLength > 0 && resp.Header.Get("Content-Range") == "" {
		//Overwrite size with actual size since size readings from Onedrive is unreliable.
		o.size = resp.ContentLength
	}
	return resp.Body, err
}

// createUploadSession creates an upload session for the object
func (o *Object) createUploadSession(modTime time.Time) (response *api.CreateUploadResponse, err error) {
	opts := rest.Opts{
		Method: "POST",
		Path:   "/root:/" + rest.URLPathEscape(o.srvPath()) + ":/upload.createSession",
	}
	createRequest := api.CreateUploadRequest{}
	createRequest.Item.FileSystemInfo.CreatedDateTime = api.Timestamp(modTime)
	createRequest.Item.FileSystemInfo.LastModifiedDateTime = api.Timestamp(modTime)
	var resp *http.Response
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.CallJSON(&opts, &createRequest, &response)
		return shouldRetry(resp, err)
	})
	return response, err
}

// uploadFragment uploads a part
func (o *Object) uploadFragment(url string, start int64, totalSize int64, chunk io.ReadSeeker, chunkSize int64) (info *api.Item, err error) {
	opts := rest.Opts{
		Method:        "PUT",
		RootURL:       url,
		ContentLength: &chunkSize,
		ContentRange:  fmt.Sprintf("bytes %d-%d/%d", start, start+chunkSize-1, totalSize),
		Body:          chunk,
	}
	//	var response api.UploadFragmentResponse
	var resp *http.Response
	err = o.fs.pacer.Call(func() (bool, error) {
		_, _ = chunk.Seek(0, 0)
		resp, err = o.fs.srv.Call(&opts)
		if resp != nil {
			defer fs.CheckClose(resp.Body, &err)
		}
		retry, err := shouldRetry(resp, err)
		if !retry && resp != nil {
			if resp.StatusCode == 200 || resp.StatusCode == 201 {
				// we are done :)
				// read the item
				info = &api.Item{}
				return false, json.NewDecoder(resp.Body).Decode(info)
			}
		}
		return retry, err
	})
	return info, err
}

// cancelUploadSession cancels an upload session
func (o *Object) cancelUploadSession(url string) (err error) {
	opts := rest.Opts{
		Method:     "DELETE",
		RootURL:    url,
		NoResponse: true,
	}
	var resp *http.Response
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.Call(&opts)
		return shouldRetry(resp, err)
	})
	return
}

// uploadMultipart uploads a file using multipart upload
func (o *Object) uploadMultipart(in io.Reader, size int64, modTime time.Time) (info *api.Item, err error) {
	if chunkSize%(320*1024) != 0 {
		return nil, errors.Errorf("chunk size %d is not a multiple of 320k", chunkSize)
	}

	// Create upload session
	fs.Debugf(o, "Starting multipart upload")
	session, err := o.createUploadSession(modTime)
	if err != nil {
		return nil, err
	}
	uploadURL := session.UploadURL

	// Cancel the session if something went wrong
	defer func() {
		if err != nil {
			fs.Debugf(o, "Cancelling multipart upload: %v", err)
			cancelErr := o.cancelUploadSession(uploadURL)
			if cancelErr != nil {
				fs.Logf(o, "Failed to cancel multipart upload: %v", err)
			}
		}
	}()

	// Upload the chunks
	remaining := size
	position := int64(0)
	for remaining > 0 {
		n := int64(chunkSize)
		if remaining < n {
			n = remaining
		}
		seg := readers.NewRepeatableReader(io.LimitReader(in, n))
		fs.Debugf(o, "Uploading segment %d/%d size %d", position, size, n)
		info, err = o.uploadFragment(uploadURL, position, size, seg, n)
		if err != nil {
			return nil, err
		}
		remaining -= n
		position += n
	}

	return info, nil
}

// uploadSinglepart uploads a file as a single part
func (o *Object) uploadSinglepart(in io.Reader, size int64, modTime time.Time) (info *api.Item, err error) {
	var resp *http.Response
	opts := rest.Opts{
		Method:        "PUT",
		Path:          "/root:/" + rest.URLPathEscape(o.srvPath()) + ":/content",
		ContentLength: &size,
		Body:          in,
	}
	// for go1.8 (see release notes) we must nil the Body if we want a
	// "Content-Length: 0" header which onedrive requires for all files.
	if size == 0 {
		opts.Body = nil
	}
	err = o.fs.pacer.CallNoRetry(func() (bool, error) {
		resp, err = o.fs.srv.CallJSON(&opts, nil, &info)
		return shouldRetry(resp, err)
	})
	if err != nil {
		return nil, err
	}
	err = o.setMetaData(info)
	if err != nil {
		return nil, err
	}
	// Set the mod time now and read metadata
	return o.setModTime(modTime)
}

// Update the object with the contents of the io.Reader, modTime and size
//
// The new object may have been created if an error is returned
func (o *Object) Update(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	o.fs.tokenRenewer.Start()
	defer o.fs.tokenRenewer.Stop()

	size := src.Size()
	modTime := src.ModTime()

	var info *api.Item
	if size <= 0 {
		// This is for 0 length files, or files with an unknown size
		info, err = o.uploadSinglepart(in, size, modTime)
	} else {
		info, err = o.uploadMultipart(in, size, modTime)
	}
	if err != nil {
		return err
	}
	return o.setMetaData(info)
}

// Remove an object
func (o *Object) Remove() error {
	return o.fs.deleteObject(o.id)
}

// MimeType of an Object if known, "" otherwise
func (o *Object) MimeType() string {
	return o.mimeType
}

// Check the interfaces are satisfied
var (
	_ fs.Fs     = (*Fs)(nil)
	_ fs.Purger = (*Fs)(nil)
	_ fs.Copier = (*Fs)(nil)
	_ fs.Mover  = (*Fs)(nil)
	// _ fs.DirMover = (*Fs)(nil)
	_ fs.DirCacheFlusher = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.MimeTyper       = &Object{}
)
