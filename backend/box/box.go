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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang-jwt/jwt/v4"
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
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/env"
	"github.com/rclone/rclone/lib/jwtutil"
	"github.com/rclone/rclone/lib/oauthutil"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
	"github.com/youmark/pkcs8"
	"golang.org/x/oauth2"
)

const (
	rcloneClientID              = "d0374ba6pgmaguie02ge15sv1mllndho"
	rcloneEncryptedClientSecret = "sYbJYm99WB8jzeaLPU0OPDMJKIkZvD2qOn3SyEMfiJr03RdtDt3xcZEIudRhbIDL"
	minSleep                    = 10 * time.Millisecond
	maxSleep                    = 2 * time.Second
	decayConstant               = 2 // bigger for slower decay, exponential
	rootURL                     = "https://api.box.com/2.0"
	uploadURL                   = "https://upload.box.com/api/2.0"
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

type boxCustomClaims struct {
	jwt.StandardClaims
	BoxSubType string `json:"box_sub_type,omitempty"`
}

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
					return nil, fmt.Errorf("failed to configure token with jwt authentication: %w", err)
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
			Name:      "root_folder_id",
			Help:      "Fill in for rclone to use a non root folder as its starting point.",
			Default:   "0",
			Advanced:  true,
			Sensitive: true,
		}, {
			Name: "box_config_file",
			Help: "Box App config.json location\n\nLeave blank normally." + env.ShellExpandHelp,
		}, {
			Name:      "access_token",
			Help:      "Box App Primary Access Token\n\nLeave blank normally.",
			Sensitive: true,
		}, {
			Name:    "box_sub_type",
			Default: "user",
			Examples: []fs.OptionExample{{
				Value: "user",
				Help:  "Rclone should act on behalf of a user.",
			}, {
				Value: "enterprise",
				Help:  "Rclone should act on behalf of a service account.",
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
			Name:     "list_chunk",
			Default:  1000,
			Help:     "Size of listing chunk 1-1000.",
			Advanced: true,
		}, {
			Name:     "owned_by",
			Default:  "",
			Help:     "Only show items owned by the login (email address) passed in.",
			Advanced: true,
		}, {
			Name:    "impersonate",
			Default: "",
			Help: `Impersonate this user ID when using a service account.

Setting this flag allows rclone, when using a JWT service account, to
act on behalf of another user by setting the as-user header.

The user ID is the Box identifier for a user. User IDs can found for
any user via the GET /users endpoint, which is only available to
admins, or by calling the GET /users/me endpoint with an authenticated
user session.

See: https://developer.box.com/guides/authentication/jwt/as-user/
`,
			Advanced:  true,
			Sensitive: true,
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
		return fmt.Errorf("get box config: %w", err)
	}
	privateKey, err := getDecryptedPrivateKey(boxConfig)
	if err != nil {
		return fmt.Errorf("get decrypted private key: %w", err)
	}
	claims, err := getClaims(boxConfig, boxSubType)
	if err != nil {
		return fmt.Errorf("get claims: %w", err)
	}
	signingHeaders := getSigningHeaders(boxConfig)
	queryParams := getQueryParams(boxConfig)
	client := fshttp.NewClient(ctx)
	err = jwtutil.Config("box", name, tokenURL, *claims, signingHeaders, queryParams, privateKey, m, client)
	return err
}

func getBoxConfig(configFile string) (boxConfig *api.ConfigJSON, err error) {
	file, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("box: failed to read Box config: %w", err)
	}
	err = json.Unmarshal(file, &boxConfig)
	if err != nil {
		return nil, fmt.Errorf("box: failed to parse Box config: %w", err)
	}
	return boxConfig, nil
}

func getClaims(boxConfig *api.ConfigJSON, boxSubType string) (claims *boxCustomClaims, err error) {
	val, err := jwtutil.RandomHex(20)
	if err != nil {
		return nil, fmt.Errorf("box: failed to generate random string for jti: %w", err)
	}

	claims = &boxCustomClaims{
		//lint:ignore SA1019 since we need to use jwt.StandardClaims even if deprecated in jwt-go v4 until a more permanent solution is ready in time before jwt-go v5 where it is removed entirely
		//nolint:staticcheck // Don't include staticcheck when running golangci-lint to avoid SA1019
		StandardClaims: jwt.StandardClaims{
			Id:        val,
			Issuer:    boxConfig.BoxAppSettings.ClientID,
			Subject:   boxConfig.EnterpriseID,
			Audience:  tokenURL,
			ExpiresAt: time.Now().Add(time.Second * 45).Unix(),
		},
		BoxSubType: boxSubType,
	}
	return claims, nil
}

func getSigningHeaders(boxConfig *api.ConfigJSON) map[string]interface{} {
	signingHeaders := map[string]interface{}{
		"kid": boxConfig.BoxAppSettings.AppAuth.PublicKeyID,
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
		return nil, fmt.Errorf("box: extra data included in private key: %w", err)
	}

	rsaKey, err := pkcs8.ParsePKCS8PrivateKey(block.Bytes, []byte(boxConfig.BoxAppSettings.AppAuth.Passphrase))
	if err != nil {
		return nil, fmt.Errorf("box: failed to decrypt private key: %w", err)
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
	ListChunk     int                  `config:"list_chunk"`
	OwnedBy       string               `config:"owned_by"`
	Impersonate   string               `config:"impersonate"`
}

// ItemMeta defines metadata we cache for each Item ID
type ItemMeta struct {
	SequenceID int64  // the most recent event processed for this item
	ParentID   string // ID of the parent directory of this item
	Name       string // leaf name of this item
}

// Fs represents a remote box
type Fs struct {
	name            string                // name of this remote
	root            string                // the path we are working on
	opt             Options               // parsed options
	features        *fs.Features          // optional features
	srv             *rest.Client          // the connection to the server
	dirCache        *dircache.DirCache    // Map of directory path to directory id
	pacer           *fs.Pacer             // pacer for API calls
	tokenRenewer    *oauthutil.Renew      // renew the token on expiry
	uploadToken     *pacer.TokenDispenser // control concurrency
	itemMetaCacheMu *sync.Mutex           // protects itemMetaCache
	itemMetaCache   map[string]ItemMeta   // map of Item ID to selected metadata
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

	// Box API errors which should be retries
	if apiErr, ok := err.(*api.Error); ok && apiErr.Code == "operation_blocked_temporary" {
		fs.Debugf(nil, "Retrying API error %v", err)
		return true, err
	}

	return authRetry || fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// readMetaDataForPath reads the metadata from the path
func (f *Fs) readMetaDataForPath(ctx context.Context, path string) (info *api.Item, err error) {
	// defer log.Trace(f, "path=%q", path)("info=%+v, err=%v", &info, &err)
	leaf, directoryID, err := f.dirCache.FindPath(ctx, path, false)
	if err != nil {
		if err == fs.ErrorDirNotFound {
			return nil, fs.ErrorObjectNotFound
		}
		return nil, err
	}

	// Use preupload to find the ID
	itemMini, err := f.preUploadCheck(ctx, leaf, directoryID, -1)
	if err != nil {
		return nil, err
	}
	if itemMini == nil {
		return nil, fs.ErrorObjectNotFound
	}

	// Now we have the ID we can look up the object proper
	opts := rest.Opts{
		Method:     "GET",
		Path:       "/files/" + itemMini.ID,
		Parameters: fieldsValue(),
	}
	var item api.Item
	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, nil, &item)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	return &item, nil
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
		return nil, fmt.Errorf("box: upload cutoff (%v) must be greater than equal to %v", opt.UploadCutoff, fs.SizeSuffix(minUploadCutoff))
	}

	root = parsePath(root)

	client := fshttp.NewClient(ctx)
	var ts *oauthutil.TokenSource
	// If not using an accessToken, create an oauth client and tokensource
	if opt.AccessToken == "" {
		client, ts, err = oauthutil.NewClient(ctx, name, m, oauthConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to configure Box: %w", err)
		}
	}

	ci := fs.GetConfig(ctx)
	f := &Fs{
		name:            name,
		root:            root,
		opt:             *opt,
		srv:             rest.NewClient(client).SetRoot(rootURL),
		pacer:           fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
		uploadToken:     pacer.NewTokenDispenser(ci.Transfers),
		itemMetaCacheMu: new(sync.Mutex),
		itemMetaCache:   make(map[string]ItemMeta),
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

	// If using impersonate set an as-user header
	if f.opt.Impersonate != "" {
		f.srv.SetHeader("as-user", f.opt.Impersonate)
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
	found, err = f.listAll(ctx, pathID, true, false, true, func(item *api.Item) bool {
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
func (f *Fs) listAll(ctx context.Context, dirID string, directoriesOnly bool, filesOnly bool, activeOnly bool, fn listAllFn) (found bool, err error) {
	opts := rest.Opts{
		Method:     "GET",
		Path:       "/folders/" + dirID + "/items",
		Parameters: fieldsValue(),
	}
	opts.Parameters.Set("limit", strconv.Itoa(f.opt.ListChunk))
	opts.Parameters.Set("usemarker", "true")
	var marker *string
OUTER:
	for {
		if marker != nil {
			opts.Parameters.Set("marker", *marker)
		}

		var result api.FolderItems
		var resp *http.Response
		err = f.pacer.Call(func() (bool, error) {
			resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
			return shouldRetry(ctx, resp, err)
		})
		if err != nil {
			return found, fmt.Errorf("couldn't list files: %w", err)
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
			if activeOnly && item.ItemStatus != api.ItemStatusActive {
				continue
			}
			if f.opt.OwnedBy != "" && f.opt.OwnedBy != item.OwnedBy.Login {
				continue
			}
			item.Name = f.opt.Enc.ToStandardName(item.Name)
			if fn(item) {
				found = true
				break OUTER
			}
		}
		marker = result.NextMarker
		if marker == nil {
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
	_, err = f.listAll(ctx, directoryID, false, false, true, func(info *api.Item) bool {
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

		// Cache some metadata for this Item to help us process events later
		// on. In particular, the box event API does not provide the old path
		// of the Item when it is renamed/deleted/moved/etc.
		f.itemMetaCacheMu.Lock()
		cachedItemMeta, found := f.itemMetaCache[info.ID]
		if !found || cachedItemMeta.SequenceID < info.SequenceID {
			f.itemMetaCache[info.ID] = ItemMeta{SequenceID: info.SequenceID, ParentID: directoryID, Name: info.Name}
		}
		f.itemMetaCacheMu.Unlock()

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
// Returns the object, leaf, directoryID and error.
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
func (f *Fs) preUploadCheck(ctx context.Context, leaf, directoryID string, size int64) (item *api.ItemMini, err error) {
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
				return nil, fmt.Errorf("pre-upload check: JSON decode failed: %w", err)
			}
			if conflict.Conflicts.Type != api.ItemTypeFile {
				return nil, fs.ErrorIsDir
			}
			return &conflict.Conflicts, nil
		}
		return nil, fmt.Errorf("pre-upload check: %w", err)
	}
	return nil, nil
}

// Put the object
//
// Copy the reader in to the new object which is returned.
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
	item, err := f.preUploadCheck(ctx, leaf, directoryID, src.Size())
	if err != nil {
		return nil, err
	}
	if item == nil {
		return f.PutUnchecked(ctx, in, src, options...)
	}

	// If object exists then create a skeleton one with just id
	o := &Object{
		fs:     f,
		remote: remote,
		id:     item.ID,
	}
	return o, o.Update(ctx, in, src, options...)
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.Put(ctx, in, src, options...)
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
		return nil, fmt.Errorf("failed to read user info: %w", err)
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
	var (
		deleteErrors       atomic.Uint64
		concurrencyControl = make(chan struct{}, fs.GetConfig(ctx).Checkers)
		wg                 sync.WaitGroup
	)
	_, err = f.listAll(ctx, "trash", false, false, false, func(item *api.Item) bool {
		if item.Type == api.ItemTypeFolder || item.Type == api.ItemTypeFile {
			wg.Add(1)
			concurrencyControl <- struct{}{}
			go func() {
				defer func() {
					<-concurrencyControl
					wg.Done()
				}()
				err := f.deletePermanently(ctx, item.Type, item.ID)
				if err != nil {
					fs.Errorf(f, "failed to delete trash item %q (%q): %v", item.Name, item.ID, err)
					deleteErrors.Add(1)
				}
			}()
		} else {
			fs.Debugf(f, "Ignoring %q - unknown type %q", item.Name, item.Type)
		}
		return false
	})
	wg.Wait()
	if deleteErrors.Load() != 0 {
		return fmt.Errorf("failed to delete %d trash items", deleteErrors.Load())
	}
	return err
}

// Shutdown shutdown the fs
func (f *Fs) Shutdown(ctx context.Context) error {
	f.tokenRenewer.Shutdown()
	return nil
}

// ChangeNotify calls the passed function with a path that has had changes.
// If the implementation uses polling, it should adhere to the given interval.
//
// Automatically restarts itself in case of unexpected behavior of the remote.
//
// Close the returned channel to stop being notified.
func (f *Fs) ChangeNotify(ctx context.Context, notifyFunc func(string, fs.EntryType), pollIntervalChan <-chan time.Duration) {
	go func() {
		// get the `stream_position` early so all changes from now on get processed
		streamPosition, err := f.changeNotifyStreamPosition(ctx)
		if err != nil {
			fs.Infof(f, "Failed to get StreamPosition: %s", err)
		}

		// box can send duplicate Event IDs. Use this map to track and filter
		// the ones we've already processed.
		processedEventIDs := make(map[string]time.Time)

		var ticker *time.Ticker
		var tickerC <-chan time.Time
		for {
			select {
			case pollInterval, ok := <-pollIntervalChan:
				if !ok {
					if ticker != nil {
						ticker.Stop()
					}
					return
				}
				if ticker != nil {
					ticker.Stop()
					ticker, tickerC = nil, nil
				}
				if pollInterval != 0 {
					ticker = time.NewTicker(pollInterval)
					tickerC = ticker.C
				}
			case <-tickerC:
				if streamPosition == "" {
					streamPosition, err = f.changeNotifyStreamPosition(ctx)
					if err != nil {
						fs.Infof(f, "Failed to get StreamPosition: %s", err)
						continue
					}
				}

				// Garbage collect EventIDs older than 1 minute
				for eventID, timestamp := range processedEventIDs {
					if time.Since(timestamp) > time.Minute {
						delete(processedEventIDs, eventID)
					}
				}

				streamPosition, err = f.changeNotifyRunner(ctx, notifyFunc, streamPosition, processedEventIDs)
				if err != nil {
					fs.Infof(f, "Change notify listener failure: %s", err)
				}
			}
		}
	}()
}

func (f *Fs) changeNotifyStreamPosition(ctx context.Context) (streamPosition string, err error) {
	opts := rest.Opts{
		Method:     "GET",
		Path:       "/events",
		Parameters: fieldsValue(),
	}
	opts.Parameters.Set("stream_position", "now")
	opts.Parameters.Set("stream_type", "changes")

	var result api.Events
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return "", err
	}

	return strconv.FormatInt(result.NextStreamPosition, 10), nil
}

// Attempts to construct the full path for an object, given the ID of its
// parent directory and the name of the object.
//
// Can return "" if the parentID is not currently in the directory cache.
func (f *Fs) getFullPath(parentID string, childName string) (fullPath string) {
	fullPath = ""
	name := f.opt.Enc.ToStandardName(childName)
	if parentID != "" {
		if parentDir, ok := f.dirCache.GetInv(parentID); ok {
			if len(parentDir) > 0 {
				fullPath = parentDir + "/" + name
			} else {
				fullPath = name
			}
		}
	} else {
		// No parent, this object is at the root
		fullPath = name
	}
	return fullPath
}

func (f *Fs) changeNotifyRunner(ctx context.Context, notifyFunc func(string, fs.EntryType), streamPosition string, processedEventIDs map[string]time.Time) (nextStreamPosition string, err error) {
	nextStreamPosition = streamPosition

	for {
		limit := f.opt.ListChunk

		// box only allows a max of 500 events
		if limit > 500 {
			limit = 500
		}

		opts := rest.Opts{
			Method:     "GET",
			Path:       "/events",
			Parameters: fieldsValue(),
		}
		opts.Parameters.Set("stream_position", nextStreamPosition)
		opts.Parameters.Set("stream_type", "changes")
		opts.Parameters.Set("limit", strconv.Itoa(limit))

		var result api.Events
		var resp *http.Response
		fs.Debugf(f, "Checking for changes on remote (next_stream_position: %q)", nextStreamPosition)
		err = f.pacer.Call(func() (bool, error) {
			resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
			return shouldRetry(ctx, resp, err)
		})
		if err != nil {
			return "", err
		}

		if result.ChunkSize != int64(len(result.Entries)) {
			return "", fmt.Errorf("invalid response to event request, chunk_size (%v) not equal to number of entries (%v)", result.ChunkSize, len(result.Entries))
		}

		nextStreamPosition = strconv.FormatInt(result.NextStreamPosition, 10)
		if result.ChunkSize == 0 {
			return nextStreamPosition, nil
		}

		type pathToClear struct {
			path      string
			entryType fs.EntryType
		}
		var pathsToClear []pathToClear
		newEventIDs := 0
		for _, entry := range result.Entries {
			eventDetails := fmt.Sprintf("[%q(%d)|%s|%s|%s|%s]", entry.Source.Name, entry.Source.SequenceID,
				entry.Source.Type, entry.EventType, entry.Source.ID, entry.EventID)

			if entry.EventID == "" {
				fs.Debugf(f, "%s ignored due to missing EventID", eventDetails)
				continue
			}
			if _, ok := processedEventIDs[entry.EventID]; ok {
				fs.Debugf(f, "%s ignored due to duplicate EventID", eventDetails)
				continue
			}
			processedEventIDs[entry.EventID] = time.Now()
			newEventIDs++

			if entry.Source.ID == "" { // missing File or Folder ID
				fs.Debugf(f, "%s ignored due to missing SourceID", eventDetails)
				continue
			}
			if entry.Source.Type != api.ItemTypeFile && entry.Source.Type != api.ItemTypeFolder { // event is not for a file or folder
				fs.Debugf(f, "%s ignored due to unsupported SourceType", eventDetails)
				continue
			}

			// Only interested in event types that result in a file tree change
			if _, found := api.FileTreeChangeEventTypes[entry.EventType]; !found {
				fs.Debugf(f, "%s ignored due to unsupported EventType", eventDetails)
				continue
			}

			f.itemMetaCacheMu.Lock()
			itemMeta, cachedItemMetaFound := f.itemMetaCache[entry.Source.ID]
			if cachedItemMetaFound {
				if itemMeta.SequenceID >= entry.Source.SequenceID {
					// Item in the cache has the same or newer SequenceID than
					// this event. Ignore this event, it must be old.
					f.itemMetaCacheMu.Unlock()
					fs.Debugf(f, "%s ignored due to old SequenceID (%q)", eventDetails, itemMeta.SequenceID)
					continue
				}

				// This event is newer. Delete its entry from the cache,
				// we'll notify about its change below, then it's up to a
				// future list operation to repopulate the cache.
				delete(f.itemMetaCache, entry.Source.ID)
			}
			f.itemMetaCacheMu.Unlock()

			entryType := fs.EntryDirectory
			if entry.Source.Type == api.ItemTypeFile {
				entryType = fs.EntryObject
			}

			// The box event only includes the new path for the object (e.g.
			// the path after the object was moved). If there was an old path
			// saved in our cache, it must be cleared.
			if cachedItemMetaFound {
				path := f.getFullPath(itemMeta.ParentID, itemMeta.Name)
				if path != "" {
					fs.Debugf(f, "%s added old path (%q) for notify", eventDetails, path)
					pathsToClear = append(pathsToClear, pathToClear{path: path, entryType: entryType})
				} else {
					fs.Debugf(f, "%s old parent not cached", eventDetails)
				}

				// If this is a directory, also delete it from the dir cache.
				// This will effectively invalidate the item metadata cache
				// entries for all descendents of this directory, since we
				// will no longer be able to construct a full path for them.
				// This is exactly what we want, since we don't want to notify
				// on the paths of these descendents if one of their ancestors
				// has been renamed/deleted.
				if entry.Source.Type == api.ItemTypeFolder {
					f.dirCache.FlushDir(path)
				}
			}

			// If the item is "active", then it is not trashed or deleted, so
			// it potentially has a valid parent.
			//
			// Construct the new path of the object, based on the Parent ID
			// and its name. If we get an empty result, it means we don't
			// currently know about this object so notification is unnecessary.
			if entry.Source.ItemStatus == api.ItemStatusActive {
				path := f.getFullPath(entry.Source.Parent.ID, entry.Source.Name)
				if path != "" {
					fs.Debugf(f, "%s added new path (%q) for notify", eventDetails, path)
					pathsToClear = append(pathsToClear, pathToClear{path: path, entryType: entryType})
				} else {
					fs.Debugf(f, "%s new parent not found", eventDetails)
				}
			}
		}

		// box can sometimes repeatedly return the same Event IDs within a
		// short period of time. If it stops giving us new ones, treat it
		// the same as if it returned us none at all.
		if newEventIDs == 0 {
			return nextStreamPosition, nil
		}

		notifiedPaths := make(map[string]bool)
		for _, p := range pathsToClear {
			if _, ok := notifiedPaths[p.path]; ok {
				continue
			}
			notifiedPaths[p.path] = true
			notifyFunc(p.path, p.entryType)
		}
		fs.Debugf(f, "Received %v events, resulting in %v paths and %v notifications", len(result.Entries), len(pathsToClear), len(notifiedPaths))
	}
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
	if info.Type == api.ItemTypeFolder {
		return fs.ErrorIsDir
	}
	if info.Type != api.ItemTypeFile {
		return fmt.Errorf("%q is %q: %w", o.remote, info.Type, fs.ErrorNotAFile)
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
		return fmt.Errorf("failed to upload %v - not sure why", o)
	}
	return o.setMetaData(&result.Entries[0])
}

// Update the object with the contents of the io.Reader, modTime and size
//
// If existing is set then it updates the object rather than creating a new one.
//
// The new object may have been created if an error is returned.
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
	_ fs.Shutdowner      = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.IDer            = (*Object)(nil)
)
