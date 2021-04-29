// Package box provides an interface to the Box
// object storage system.
package box

// FIXME Box only supports file names of 255 characters or less. Names
// that will not be supported are those that contain non-printable
// ascii, / or \, names with trailing spaces, and the special names
// “.” and “..”.

// FIXME box can copy a directory

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/env"
	"github.com/rclone/rclone/lib/jwtutil"

	"github.com/youmark/pkcs8"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/backend/box/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/oauthutil"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/jws"
)

const (
	rcloneClientID              = "d0374ba6pgmaguie02ge15sv1mllndho"
	rcloneEncryptedClientSecret = "sYbJYm99WB8jzeaLPU0OPDMJKIkZvD2qOn3SyEMfiJr03RdtDt3xcZEIudRhbIDL"
	minSleep                    = 10 * time.Millisecond
	maxSleep                    = 2 * time.Second
	decayConstant               = 2 // bigger for slower decay, exponential
	rootURL                     = "https://api.box.com/2.0"
	uploadURL                   = "https://upload.box.com/api/2.0"
	listChunks                  = 1000     // chunk size to read directory listings
	minUploadCutoff             = 50000000 // upload cutoff can be no lower than this
	defaultUploadCutoff         = 50 * 1024 * 1024
	tokenURL                    = "https://api.box.com/oauth2/token"
)

// Globals
var (
	// Description of how to auth for this app
	oauthConfig = &oauth2.Config{
		Scopes: nil,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://app.box.com/api/oauth2/authorize",
			TokenURL: "https://app.box.com/api/oauth2/token",
		},
		ClientID:     rcloneClientID,
		ClientSecret: obscure.MustReveal(rcloneEncryptedClientSecret),
		RedirectURL:  oauthutil.RedirectURL,
	}
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "box",
		Description: "Box",
		NewFs:       NewFs,
		Config: func(ctx context.Context, name string, m configmap.Mapper, config fs.ConfigIn) (*fs.ConfigOut, error) {
			jsonFile, ok := m.Get("box_config_file")
			boxSubType, boxSubTypeOk := m.Get("box_sub_type")
			boxAccessToken, boxAccessTokenOk := m.Get("access_token")
			var err error
			// If using box config.json, use JWT auth
			if ok && boxSubTypeOk && jsonFile != "" && boxSubType != "" {
				err = refreshJWTToken(ctx, jsonFile, boxSubType, name, m)
				if err != nil {
					return nil, errors.Wrap(err, "failed to configure token with jwt authentication")
				}
				// Else, if not using an access token, use oauth2
			} else if boxAccessToken == "" || !boxAccessTokenOk {
				return oauthutil.ConfigOut("", &oauthutil.Options{
					OAuth2Config: oauthConfig,
				})
			}
			return nil, nil
		},
		Options: append(oauthutil.SharedOptions, []fs.Option{{
			Name:     "root_folder_id",
			Help:     "Fill in for rclone to use a non root folder as its starting point.",
			Default:  "0",
			Advanced: true,
		}, {
			Name: "box_config_file",
			Help: "Box App config.json location\nLeave blank normally." + env.ShellExpandHelp,
		}, {
			Name: "access_token",
			Help: "Box App Primary Access Token\nLeave blank normally.",
		}, {
			Name:    "box_sub_type",
			Default: "user",
			Examples: []fs.OptionExample{{
				Value: "user",
				Help:  "Rclone should act on behalf of a user",
			}, {
				Value: "enterprise",
				Help:  "Rclone should act on behalf of a service account",
			}},
		}, {
			Name:     "upload_cutoff",
			Help:     "Cutoff for switching to multipart upload (>= 50 MiB).",
			Default:  fs.SizeSuffix(defaultUploadCutoff),
			Advanced: true,
		}, {
			Name:     "commit_retries",
			Help:     "Max number of times to try committing a multipart file.",
			Default:  100,
			Advanced: true,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			// From https://developer.box.com/docs/error-codes#section-400-bad-request :
			// > Box only supports file or folder names that are 255 characters or less.
			// > File names containing non-printable ascii, "/" or "\", names with leading
			// > or trailing spaces, and the special names “.” and “..” are also unsupported.
			//
			// Testing revealed names with leading spaces work fine.
			// Also encode invalid UTF-8 bytes as json doesn't handle them properly.
			Default: (encoder.Display |
				encoder.EncodeBackSlash |
				encoder.EncodeRightSpace |
				encoder.EncodeInvalidUtf8),
		}}...),
	})
}

func refreshJWTToken(ctx context.Context, jsonFile string, boxSubType string, name string, m configmap.Mapper) error {
	jsonFile = env.ShellExpand(jsonFile)
	boxConfig, err := getBoxConfig(jsonFile)
	if err != nil {
		return errors.Wrap(err, "get box config")
	}
	privateKey, err := getDecryptedPrivateKey(boxConfig)
	if err != nil {
		return errors.Wrap(err, "get decrypted private key")
	}
	claims, err := getClaims(boxConfig, boxSubType)
	if err != nil {
		return errors.Wrap(err, "get claims")
	}
	signingHeaders := getSigningHeaders(boxConfig)
	queryParams := getQueryParams(boxConfig)
	client := fshttp.NewClient(ctx)
	err = jwtutil.Config("box", name, claims, signingHeaders, queryParams, privateKey, m, client)
	return err
}

func getBoxConfig(configFile string) (boxConfig *api.ConfigJSON, err error) {
	file, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, errors.Wrap(err, "box: failed to read Box config")
	}
	err = json.Unmarshal(file, &boxConfig)
	if err != nil {
		return nil, errors.Wrap(err, "box: failed to parse Box config")
	}
	return boxConfig, nil
}

func getClaims(boxConfig *api.ConfigJSON, boxSubType string) (claims *jws.ClaimSet, err error) {
	val, err := jwtutil.RandomHex(20)
	if err != nil {
		return nil, errors.Wrap(err, "box: failed to generate random string for jti")
	}

	claims = &jws.ClaimSet{
		Iss: boxConfig.BoxAppSettings.ClientID,
		Sub: boxConfig.EnterpriseID,
		Aud: tokenURL,
		Exp: time.Now().Add(time.Second * 45).Unix(),
		PrivateClaims: map[string]interface{}{
			"box_sub_type": boxSubType,
			"aud":          tokenURL,
			"jti":          val,
		},
	}

	return claims, nil
}

func getSigningHeaders(boxConfig *api.ConfigJSON) *jws.Header {
	signingHeaders := &jws.Header{
		Algorithm: "RS256",
		Typ:       "JWT",
		KeyID:     boxConfig.BoxAppSettings.AppAuth.PublicKeyID,
	}

	return signingHeaders
}

func getQueryParams(boxConfig *api.ConfigJSON) map[string]string {
	queryParams := map[string]string{
		"client_id":     boxConfig.BoxAppSettings.ClientID,
		"client_secret": boxConfig.BoxAppSettings.ClientSecret,
	}

	return queryParams
}

func getDecryptedPrivateKey(boxConfig *api.ConfigJSON) (key *rsa.PrivateKey, err error) {

	block, rest := pem.Decode([]byte(boxConfig.BoxAppSettings.AppAuth.PrivateKey))
	if len(rest) > 0 {
		return nil, errors.Wrap(err, "box: extra data included in private key")
	}

	rsaKey, err := pkcs8.ParsePKCS8PrivateKey(block.Bytes, []byte(boxConfig.BoxAppSettings.AppAuth.Passphrase))
	if err != nil {
		return nil, errors.Wrap(err, "box: failed to decrypt private key")
	}

	return rsaKey.(*rsa.PrivateKey), nil
}

// Options defines the configuration for this backend
type Options struct {
	UploadCutoff  fs.SizeSuffix        `config:"upload_cutoff"`
	CommitRetries int                  `config:"commit_retries"`
	Enc           encoder.MultiEncoder `config:"encoding"`
	RootFolderID  string               `config:"root_folder_id"`
	AccessToken   string               `config:"access_token"`
}

// Fs represents a remote box
type Fs struct {
	name         string                // name of this remote
	root         string                // the path we are working on
	opt          Options               // parsed options
	features     *fs.Features          // optional features
	srv          *rest.Client          // the connection to the one drive server
	dirCache     *dircache.DirCache    // Map of directory path to directory id
	pacer        *fs.Pacer             // pacer for API calls
	tokenRenewer *oauthutil.Renew      // renew the token on expiry
	uploadToken  *pacer.TokenDispenser // control concurrency
}

// Object describes a box object
//
// Will definitely have info but maybe not meta
type Object struct {
	fs          *Fs       // what this object is part of
	remote      string    // The remote path
	hasMetaData bool      // whether info below has been set
	size        int64     // size of the object
	modTime     time.Time // modification time of the object
	id          string    // ID of the object
	publicLink  string    // Public Link for the object
	sha1        string    // SHA-1 of the object content
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
	return fmt.Sprintf("box root '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// parsePath parses a box 'url'
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
	authRetry := false

	if resp != nil && resp.StatusCode == 401 && strings.Contains(resp.Header.Get("Www-Authenticate"), "expired_token") {
		authRetry = true
		fs.Debugf(nil, "Should retry: %v", err)
	}
	return authRetry || fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
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
		if strings.EqualFold(item.Name, leaf) {
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
	if errResponse.Code == "" {
		errResponse.Code = resp.Status
	}
	if errResponse.Status == 0 {
		errResponse.Status = resp.StatusCode
	}
	return errResponse
}

// NewFs constructs an Fs from the path, container:path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	if opt.UploadCutoff < minUploadCutoff {
		return nil, errors.Errorf("box: upload cutoff (%v) must be greater than equal to %v", opt.UploadCutoff, fs.SizeSuffix(minUploadCutoff))
	}

	root = parsePath(root)

	client := fshttp.NewClient(ctx)
	var ts *oauthutil.TokenSource
	// If not using an accessToken, create an oauth client and tokensource
	if opt.AccessToken == "" {
		client, ts, err = oauthutil.NewClient(ctx, name, m, oauthConfig)
		if err != nil {
			return nil, errors.Wrap(err, "failed to configure Box")
		}
	}

	ci := fs.GetConfig(ctx)
	f := &Fs{
		name:        name,
		root:        root,
		opt:         *opt,
		srv:         rest.NewClient(client).SetRoot(rootURL),
		pacer:       fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
		uploadToken: pacer.NewTokenDispenser(ci.Transfers),
	}
	f.features = (&fs.Features{
		CaseInsensitive:         true,
		CanHaveEmptyDirectories: true,
	}).Fill(ctx, f)
	f.srv.SetErrorHandler(errorHandler)

	// If using an accessToken, set the Authorization header
	if f.opt.AccessToken != "" {
		f.srv.SetHeader("Authorization", "Bearer "+f.opt.AccessToken)
	}

	jsonFile, ok := m.Get("box_config_file")
	boxSubType, boxSubTypeOk := m.Get("box_sub_type")

	if ts != nil {
		// If using box config.json and JWT, renewing should just refresh the token and
		// should do so whether there are uploads pending or not.
		if ok && boxSubTypeOk && jsonFile != "" && boxSubType != "" {
			f.tokenRenewer = oauthutil.NewRenew(f.String(), ts, func() error {
				err := refreshJWTToken(ctx, jsonFile, boxSubType, name, m)
				return err
			})
			f.tokenRenewer.Start()
		} else {
			// Renew the token in the background
			f.tokenRenewer = oauthutil.NewRenew(f.String(), ts, func() error {
				_, err := f.readMetaDataForPath(ctx, "")
				return err
			})
		}
	}

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

// fieldsValue creates a url.Values with fields set to those in api.Item
func fieldsValue() url.Values {
	values := url.Values{}
	values.Set("fields", api.ItemFields)
	return values
}

// CreateDir makes a directory with pathID as parent and name leaf
func (f *Fs) CreateDir(ctx context.Context, pathID, leaf string) (newID string, err error) {
	// fs.Debugf(f, "CreateDir(%q, %q)\n", pathID, leaf)
	var resp *http.Response
	var info *api.Item
	opts := rest.Opts{
		Method:     "POST",
		Path:       "/folders",
		Parameters: fieldsValue(),
	}
	mkdir := api.CreateFolder{
		Name: f.opt.Enc.FromStandardName(leaf),
		Parent: api.Parent{
			ID: pathID,
		},
	}
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &mkdir, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		//fmt.Printf("...Error %v\n", err)
		return "", err
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
		Path:       "/folders/" + dirID + "/items",
		Parameters: fieldsValue(),
	}
	opts.Parameters.Set("limit", strconv.Itoa(listChunks))
	offset := 0
OUTER:
	for {
		opts.Parameters.Set("offset", strconv.Itoa(offset))

		var result api.FolderItems
		var resp *http.Response
		err = f.pacer.Call(func() (bool, error) {
			resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
			return shouldRetry(ctx, resp, err)
		})
		if err != nil {
			return found, errors.Wrap(err, "couldn't list files")
		}
		for i := range result.Entries {
			item := &result.Entries[i]
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
			if item.ItemStatus != api.ItemStatusActive {
				continue
			}
			item.Name = f.opt.Enc.ToStandardName(item.Name)
			if fn(item) {
				found = true
				break OUTER
			}
		}
		offset += result.Limit
		if offset >= result.TotalCount {
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
			d := fs.NewDir(remote, info.ModTime()).SetID(info.ID)
			// FIXME more info from dir?
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

// preUploadCheck checks to see if a file can be uploaded
//
// It returns "", nil if the file is good to go
// It returns "ID", nil if the file must be updated
func (f *Fs) preUploadCheck(ctx context.Context, leaf, directoryID string, size int64) (ID string, err error) {
	check := api.PreUploadCheck{
		Name: f.opt.Enc.FromStandardName(leaf),
		Parent: api.Parent{
			ID: directoryID,
		},
	}
	if size >= 0 {
		check.Size = &size
	}
	opts := rest.Opts{
		Method: "OPTIONS",
		Path:   "/files/content/",
	}
	var result api.PreUploadCheckResponse
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &check, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if apiErr, ok := err.(*api.Error); ok && apiErr.Code == "item_name_in_use" {
			var conflict api.PreUploadCheckConflict
			err = json.Unmarshal(apiErr.ContextInfo, &conflict)
			if err != nil {
				return "", errors.Wrap(err, "pre-upload check: JSON decode failed")
			}
			if conflict.Conflicts.Type != api.ItemTypeFile {
				return "", errors.Wrap(err, "pre-upload check: can't overwrite non file with file")
			}
			return conflict.Conflicts.ID, nil
		}
		return "", errors.Wrap(err, "pre-upload check")
	}
	return "", nil
}

// Put the object
//
// Copy the reader in to the new object which is returned
//
// The new object may have been created if an error is returned
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	// If directory doesn't exist, file doesn't exist so can upload
	remote := src.Remote()
	leaf, directoryID, err := f.dirCache.FindPath(ctx, remote, false)
	if err != nil {
		if err == fs.ErrorDirNotFound {
			return f.PutUnchecked(ctx, in, src, options...)
		}
		return nil, err
	}

	// Preflight check the upload, which returns the ID if the
	// object already exists
	ID, err := f.preUploadCheck(ctx, leaf, directoryID, src.Size())
	if err != nil {
		return nil, err
	}
	if ID == "" {
		return f.PutUnchecked(ctx, in, src, options...)
	}

	// If object exists then create a skeleton one with just id
	o := &Object{
		fs:     f,
		remote: remote,
		id:     ID,
	}
	return o, o.Update(ctx, in, src, options...)
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

// deleteObject removes an object by ID
func (f *Fs) deleteObject(ctx context.Context, id string) error {
	opts := rest.Opts{
		Method:     "DELETE",
		Path:       "/files/" + id,
		NoResponse: true,
	}
	return f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.Call(ctx, &opts)
		return shouldRetry(ctx, resp, err)
	})
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
		Path:       "/folders/" + rootID,
		Parameters: url.Values{},
		NoResponse: true,
	}
	opts.Parameters.Set("recursive", strconv.FormatBool(!check))
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.Call(ctx, &opts)
		return shouldRetry(ctx, resp, err)
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
	if strings.ToLower(srcPath) == strings.ToLower(dstPath) {
		return nil, errors.Errorf("can't copy %q -> %q as are same name when lowercase", srcPath, dstPath)
	}

	// Create temporary object
	dstObj, leaf, directoryID, err := f.createObject(ctx, remote, srcObj.modTime, srcObj.size)
	if err != nil {
		return nil, err
	}

	// Copy the object
	opts := rest.Opts{
		Method:     "POST",
		Path:       "/files/" + srcObj.id + "/copy",
		Parameters: fieldsValue(),
	}
	copyFile := api.CopyFile{
		Name: f.opt.Enc.FromStandardName(leaf),
		Parent: api.Parent{
			ID: directoryID,
		},
	}
	var resp *http.Response
	var info *api.Item
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &copyFile, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	err = dstObj.setMetaData(info)
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
func (f *Fs) Purge(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir, false)
}

// move a file or folder
func (f *Fs) move(ctx context.Context, endpoint, id, leaf, directoryID string) (info *api.Item, err error) {
	// Move the object
	opts := rest.Opts{
		Method:     "PUT",
		Path:       endpoint + id,
		Parameters: fieldsValue(),
	}
	move := api.UpdateFileMove{
		Name: f.opt.Enc.FromStandardName(leaf),
		Parent: api.Parent{
			ID: directoryID,
		},
	}
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &move, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	return info, nil
}

// About gets quota information
func (f *Fs) About(ctx context.Context) (usage *fs.Usage, err error) {
	opts := rest.Opts{
		Method: "GET",
		Path:   "/users/me",
	}
	var user api.User
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &user)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to read user info")
	}
	// FIXME max upload size would be useful to use in Update
	usage = &fs.Usage{
		Used:  fs.NewUsageValue(user.SpaceUsed),                    // bytes in use
		Total: fs.NewUsageValue(user.SpaceAmount),                  // bytes total
		Free:  fs.NewUsageValue(user.SpaceAmount - user.SpaceUsed), // bytes free
	}
	return usage, nil
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
	info, err := f.move(ctx, "/files/", srcObj.id, leaf, directoryID)
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
	_, err = f.move(ctx, "/folders/", srcID, dstLeaf, dstDirectoryID)
	if err != nil {
		return err
	}
	srcFs.dirCache.FlushDir(srcRemote)
	return nil
}

// PublicLink adds a "readable by anyone with link" permission on the given file or folder.
func (f *Fs) PublicLink(ctx context.Context, remote string, expire fs.Duration, unlink bool) (string, error) {
	id, err := f.dirCache.FindDir(ctx, remote, false)
	var opts rest.Opts
	if err == nil {
		fs.Debugf(f, "attempting to share directory '%s'", remote)

		opts = rest.Opts{
			Method:     "PUT",
			Path:       "/folders/" + id,
			Parameters: fieldsValue(),
		}
	} else {
		fs.Debugf(f, "attempting to share single file '%s'", remote)
		o, err := f.NewObject(ctx, remote)
		if err != nil {
			return "", err
		}

		if o.(*Object).publicLink != "" {
			return o.(*Object).publicLink, nil
		}

		opts = rest.Opts{
			Method:     "PUT",
			Path:       "/files/" + o.(*Object).id,
			Parameters: fieldsValue(),
		}
	}

	shareLink := api.CreateSharedLink{}
	var info api.Item
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &shareLink, &info)
		return shouldRetry(ctx, resp, err)
	})
	return info.SharedLink.URL, err
}

// deletePermanently permanently deletes a trashed file
func (f *Fs) deletePermanently(ctx context.Context, itemType, id string) error {
	opts := rest.Opts{
		Method:     "DELETE",
		NoResponse: true,
	}
	if itemType == api.ItemTypeFile {
		opts.Path = "/files/" + id + "/trash"
	} else {
		opts.Path = "/folders/" + id + "/trash"
	}
	return f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.Call(ctx, &opts)
		return shouldRetry(ctx, resp, err)
	})
}

// CleanUp empties the trash
func (f *Fs) CleanUp(ctx context.Context) (err error) {
	opts := rest.Opts{
		Method: "GET",
		Path:   "/folders/trash/items",
		Parameters: url.Values{
			"fields": []string{"type", "id"},
		},
	}
	opts.Parameters.Set("limit", strconv.Itoa(listChunks))
	offset := 0
	for {
		opts.Parameters.Set("offset", strconv.Itoa(offset))

		var result api.FolderItems
		var resp *http.Response
		err = f.pacer.Call(func() (bool, error) {
			resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
			return shouldRetry(ctx, resp, err)
		})
		if err != nil {
			return errors.Wrap(err, "couldn't list trash")
		}
		for i := range result.Entries {
			item := &result.Entries[i]
			if item.Type == api.ItemTypeFolder || item.Type == api.ItemTypeFile {
				err := f.deletePermanently(ctx, item.Type, item.ID)
				if err != nil {
					return errors.Wrap(err, "failed to delete file")
				}
			} else {
				fs.Debugf(f, "Ignoring %q - unknown type %q", item.Name, item.Type)
				continue
			}
		}
		offset += result.Limit
		if offset >= result.TotalCount {
			break
		}
	}
	return
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

// Hash returns the SHA-1 of an object returning a lowercase hex string
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	if t != hash.SHA1 {
		return "", hash.ErrUnsupported
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
	if info.Type != api.ItemTypeFile {
		return errors.Wrapf(fs.ErrorNotAFile, "%q is %q", o.remote, info.Type)
	}
	o.hasMetaData = true
	o.size = int64(info.Size)
	o.sha1 = info.SHA1
	o.modTime = info.ModTime()
	o.id = info.ID
	o.publicLink = info.SharedLink.URL
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
		if apiErr, ok := err.(*api.Error); ok {
			if apiErr.Code == "not_found" || apiErr.Code == "trashed" {
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
func (o *Object) ModTime(ctx context.Context) time.Time {
	err := o.readMetaData(ctx)
	if err != nil {
		fs.Logf(o, "Failed to read metadata: %v", err)
		return time.Now()
	}
	return o.modTime
}

// setModTime sets the modification time of the local fs object
func (o *Object) setModTime(ctx context.Context, modTime time.Time) (*api.Item, error) {
	opts := rest.Opts{
		Method:     "PUT",
		Path:       "/files/" + o.id,
		Parameters: fieldsValue(),
	}
	update := api.UpdateFileModTime{
		ContentModifiedAt: api.Time(modTime),
	}
	var info *api.Item
	err := o.fs.pacer.Call(func() (bool, error) {
		resp, err := o.fs.srv.CallJSON(ctx, &opts, &update, &info)
		return shouldRetry(ctx, resp, err)
	})
	return info, err
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	info, err := o.setModTime(ctx, modTime)
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
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	if o.id == "" {
		return nil, errors.New("can't download - no id")
	}
	fs.FixRangeOption(options, o.size)
	var resp *http.Response
	opts := rest.Opts{
		Method:  "GET",
		Path:    "/files/" + o.id + "/content",
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

// upload does a single non-multipart upload
//
// This is recommended for less than 50 MiB of content
func (o *Object) upload(ctx context.Context, in io.Reader, leaf, directoryID string, modTime time.Time, options ...fs.OpenOption) (err error) {
	upload := api.UploadFile{
		Name:              o.fs.opt.Enc.FromStandardName(leaf),
		ContentModifiedAt: api.Time(modTime),
		ContentCreatedAt:  api.Time(modTime),
		Parent: api.Parent{
			ID: directoryID,
		},
	}

	var resp *http.Response
	var result api.FolderItems
	opts := rest.Opts{
		Method:                "POST",
		Body:                  in,
		MultipartMetadataName: "attributes",
		MultipartContentName:  "contents",
		MultipartFileName:     upload.Name,
		RootURL:               uploadURL,
		Options:               options,
	}
	// If object has an ID then it is existing so create a new version
	if o.id != "" {
		opts.Path = "/files/" + o.id + "/content"
	} else {
		opts.Path = "/files/content"
	}
	err = o.fs.pacer.CallNoRetry(func() (bool, error) {
		resp, err = o.fs.srv.CallJSON(ctx, &opts, &upload, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return err
	}
	if result.TotalCount != 1 || len(result.Entries) != 1 {
		return errors.Errorf("failed to upload %v - not sure why", o)
	}
	return o.setMetaData(&result.Entries[0])
}

// Update the object with the contents of the io.Reader, modTime and size
//
// If existing is set then it updates the object rather than creating a new one
//
// The new object may have been created if an error is returned
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	if o.fs.tokenRenewer != nil {
		o.fs.tokenRenewer.Start()
		defer o.fs.tokenRenewer.Stop()
	}

	size := src.Size()
	modTime := src.ModTime(ctx)
	remote := o.Remote()

	// Create the directory for the object if it doesn't exist
	leaf, directoryID, err := o.fs.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return err
	}

	// Upload with simple or multipart
	if size <= int64(o.fs.opt.UploadCutoff) {
		err = o.upload(ctx, in, leaf, directoryID, modTime, options...)
	} else {
		err = o.uploadMultipart(ctx, in, leaf, directoryID, size, modTime, options...)
	}
	return err
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	return o.fs.deleteObject(ctx, o.id)
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
	_ fs.Abouter         = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ fs.DirCacheFlusher = (*Fs)(nil)
	_ fs.PublicLinker    = (*Fs)(nil)
	_ fs.CleanUpper      = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.IDer            = (*Object)(nil)
)
