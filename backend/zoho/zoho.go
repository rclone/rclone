// Package zoho provides an interface to the Zoho Workdrive
// storage system.
package zoho

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/random"
	"github.com/rclone/rclone/lib/readers"

	"github.com/rclone/rclone/backend/zoho/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/oauthutil"
	"github.com/rclone/rclone/lib/rest"
	"golang.org/x/oauth2"
)

const (
	rcloneClientID              = "1000.46MXF275FM2XV7QCHX5A7K3LGME66B"
	rcloneEncryptedClientSecret = "U-2gxclZQBcOG9NPhjiXAhj-f0uQ137D0zar8YyNHXHkQZlTeSpIOQfmCb4oSpvosJp_SJLXmLLeUA"
	minSleep                    = 10 * time.Millisecond
	maxSleep                    = 2 * time.Second
	decayConstant               = 2 // bigger for slower decay, exponential
	configRootID                = "root_folder_id"
)

// Globals
var (
	// Description of how to auth for this app
	oauthConfig = &oauth2.Config{
		Scopes: []string{
			"aaaserver.profile.read",
			"WorkDrive.team.READ",
			"WorkDrive.workspace.READ",
			"WorkDrive.files.ALL",
		},
		Endpoint: oauth2.Endpoint{
			AuthURL:   "https://accounts.zoho.eu/oauth/v2/auth",
			TokenURL:  "https://accounts.zoho.eu/oauth/v2/token",
			AuthStyle: oauth2.AuthStyleInParams,
		},
		ClientID:     rcloneClientID,
		ClientSecret: obscure.MustReveal(rcloneEncryptedClientSecret),
		RedirectURL:  oauthutil.RedirectLocalhostURL,
	}
	rootURL     = "https://workdrive.zoho.eu/api/v1"
	accountsURL = "https://accounts.zoho.eu"
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "zoho",
		Description: "Zoho",
		NewFs:       NewFs,
		Config: func(ctx context.Context, name string, m configmap.Mapper) {
			// Need to setup region before configuring oauth
			setupRegion(m)
			opt := oauthutil.Options{
				// No refresh token unless ApprovalForce is set
				OAuth2Opts: []oauth2.AuthCodeOption{oauth2.ApprovalForce},
			}
			if err := oauthutil.Config(ctx, "zoho", name, m, oauthConfig, &opt); err != nil {
				log.Fatalf("Failed to configure token: %v", err)
			}
			// We need to rewrite the token type to "Zoho-oauthtoken" because Zoho wants
			// it's own custom type
			token, err := oauthutil.GetToken(name, m)
			if err != nil {
				log.Fatalf("Failed to read token: %v", err)
			}
			if token.TokenType != "Zoho-oauthtoken" {
				token.TokenType = "Zoho-oauthtoken"
				err = oauthutil.PutToken(name, m, token, false)
				if err != nil {
					log.Fatalf("Failed to configure token: %v", err)
				}
			}
			if err = setupRoot(ctx, name, m); err != nil {
				log.Fatalf("Failed to configure root directory: %v", err)
			}
		},
		Options: append(oauthutil.SharedOptions, []fs.Option{{
			Name: "region",
			Help: "Zoho region to connect to. You'll have to use the region you organization is registered in.",
			Examples: []fs.OptionExample{{
				Value: "com",
				Help:  "United states / Global",
			}, {
				Value: "eu",
				Help:  "Europe",
			}, {
				Value: "in",
				Help:  "India",
			}, {
				Value: "com.au",
				Help:  "Australia",
			}}}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			Default: (encoder.EncodeZero |
				encoder.EncodeCtl |
				encoder.EncodeDel |
				encoder.EncodeInvalidUtf8),
		}}...),
	})
}

// Options defines the configuration for this backend
type Options struct {
	RootFolderID string               `config:"root_folder_id"`
	Region       string               `config:"region"`
	Enc          encoder.MultiEncoder `config:"encoding"`
}

// Fs represents a remote workdrive
type Fs struct {
	name     string             // name of this remote
	root     string             // the path we are working on
	opt      Options            // parsed options
	features *fs.Features       // optional features
	srv      *rest.Client       // the connection to the one drive server
	dirCache *dircache.DirCache // Map of directory path to directory id
	pacer    *fs.Pacer          // pacer for API calls
}

// Object describes a Zoho WorkDrive object
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

func setupRegion(m configmap.Mapper) {
	region, ok := m.Get("region")
	if !ok {
		log.Fatalf("No region set\n")
	}
	rootURL = fmt.Sprintf("https://workdrive.zoho.%s/api/v1", region)
	accountsURL = fmt.Sprintf("https://accounts.zoho.%s", region)
	oauthConfig.Endpoint.AuthURL = fmt.Sprintf("https://accounts.zoho.%s/oauth/v2/auth", region)
	oauthConfig.Endpoint.TokenURL = fmt.Sprintf("https://accounts.zoho.%s/oauth/v2/token", region)
}

// ------------------------------------------------------------

func listTeams(ctx context.Context, uid int64, srv *rest.Client) ([]api.TeamWorkspace, error) {
	var teamList api.TeamWorkspaceResponse
	opts := rest.Opts{
		Method:       "GET",
		Path:         "/users/" + strconv.FormatInt(uid, 10) + "/teams",
		ExtraHeaders: map[string]string{"Accept": "application/vnd.api+json"},
	}
	_, err := srv.CallJSON(ctx, &opts, nil, &teamList)
	if err != nil {
		return nil, err
	}
	return teamList.TeamWorkspace, nil
}

func listWorkspaces(ctx context.Context, teamID string, srv *rest.Client) ([]api.TeamWorkspace, error) {
	var workspaceList api.TeamWorkspaceResponse
	opts := rest.Opts{
		Method:       "GET",
		Path:         "/teams/" + teamID + "/workspaces",
		ExtraHeaders: map[string]string{"Accept": "application/vnd.api+json"},
	}
	_, err := srv.CallJSON(ctx, &opts, nil, &workspaceList)
	if err != nil {
		return nil, err
	}
	return workspaceList.TeamWorkspace, nil
}

func setupRoot(ctx context.Context, name string, m configmap.Mapper) error {
	oAuthClient, _, err := oauthutil.NewClient(ctx, name, m, oauthConfig)
	if err != nil {
		log.Fatalf("Failed to load oAuthClient: %s", err)
	}
	authSrv := rest.NewClient(oAuthClient).SetRoot(accountsURL)
	opts := rest.Opts{
		Method: "GET",
		Path:   "/oauth/user/info",
	}

	var user api.User
	_, err = authSrv.CallJSON(ctx, &opts, nil, &user)
	if err != nil {
		return err
	}

	apiSrv := rest.NewClient(oAuthClient).SetRoot(rootURL)
	teams, err := listTeams(ctx, user.ZUID, apiSrv)
	if err != nil {
		return err
	}
	var teamIDs, teamNames []string
	for _, team := range teams {
		teamIDs = append(teamIDs, team.ID)
		teamNames = append(teamNames, team.Attributes.Name)
	}
	teamID := config.Choose("Enter a Team Drive ID", teamIDs, teamNames, true)

	workspaces, err := listWorkspaces(ctx, teamID, apiSrv)
	if err != nil {
		return err
	}
	var workspaceIDs, workspaceNames []string
	for _, workspace := range workspaces {
		workspaceIDs = append(workspaceIDs, workspace.ID)
		workspaceNames = append(workspaceNames, workspace.Attributes.Name)
	}
	worksspaceID := config.Choose("Enter a Workspace ID", workspaceIDs, workspaceNames, true)
	m.Set(configRootID, worksspaceID)
	return nil
}

// --------------------------------------------------------------

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

	if resp != nil && resp.StatusCode == 401 && len(resp.Header["Www-Authenticate"]) == 1 && strings.Index(resp.Header["Www-Authenticate"][0], "expired_token") >= 0 {
		authRetry = true
		fs.Debugf(nil, "Should retry: %v", err)
	}
	return authRetry || fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// --------------------------------------------------------------

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
	return fmt.Sprintf("zoho root '%s'", f.root)
}

// Precision return the precision of this Fs
func (f *Fs) Precision() time.Duration {
	return fs.ModTimeNotSupported
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.None)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// parsePath parses a zoho 'url'
func parsePath(path string) (root string) {
	root = strings.Trim(path, "/")
	return
}

func (f *Fs) splitPath(remote string) (directory, leaf string) {
	directory, leaf = dircache.SplitPath(remote)
	if f.root != "" {
		// Adds the root folder to the path to get a full path
		directory = path.Join(f.root, directory)
	}
	return
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
		if item.Attributes.Name == leaf {
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

// readMetaDataForID reads the metadata for the object with given ID
func (f *Fs) readMetaDataForID(ctx context.Context, id string) (*api.Item, error) {
	opts := rest.Opts{
		Method:       "GET",
		Path:         "/files/" + id,
		ExtraHeaders: map[string]string{"Accept": "application/vnd.api+json"},
		Parameters:   url.Values{},
	}
	var result *api.ItemInfo
	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	return &result.Item, nil
}

// NewFs constructs an Fs from the path, container:path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	if err := configstruct.Set(m, opt); err != nil {
		return nil, err
	}
	setupRegion(m)

	root = parsePath(root)
	oAuthClient, _, err := oauthutil.NewClient(ctx, name, m, oauthConfig)
	if err != nil {
		return nil, err
	}

	f := &Fs{
		name:  name,
		root:  root,
		opt:   *opt,
		srv:   rest.NewClient(oAuthClient).SetRoot(rootURL),
		pacer: fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
	}
	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
	}).Fill(ctx, f)

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
		f.dirCache = tempF.dirCache
		f.root = tempF.root
		// return an error with an fs which points to the parent
		return f, fs.ErrorIsFile
	}
	return f, nil
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
		Method:       "GET",
		Path:         "/files/" + dirID + "/files",
		ExtraHeaders: map[string]string{"Accept": "application/vnd.api+json"},
		Parameters:   url.Values{},
	}
	opts.Parameters.Set("page[limit]", strconv.Itoa(10))
	offset := 0
OUTER:
	for {
		opts.Parameters.Set("page[offset]", strconv.Itoa(offset))

		var result api.ItemList
		var resp *http.Response
		err = f.pacer.Call(func() (bool, error) {
			resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
			return shouldRetry(ctx, resp, err)
		})
		if err != nil {
			return found, errors.Wrap(err, "couldn't list files")
		}
		if len(result.Items) == 0 {
			break
		}
		for i := range result.Items {
			item := &result.Items[i]
			if item.Attributes.IsFolder {
				if filesOnly {
					continue
				}
			} else {
				if directoriesOnly {
					continue
				}
			}
			item.Attributes.Name = f.opt.Enc.ToStandardName(item.Attributes.Name)
			if fn(item) {
				found = true
				break OUTER
			}
		}
		offset += 10
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
		remote := path.Join(dir, info.Attributes.Name)
		if info.Attributes.IsFolder {
			// cache the directory ID for later lookups
			f.dirCache.Put(remote, info.ID)
			d := fs.NewDir(remote, time.Time(info.Attributes.ModifiedTime)).SetID(info.ID)
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

// FindLeaf finds a directory of name leaf in the folder with ID pathID
func (f *Fs) FindLeaf(ctx context.Context, pathID, leaf string) (pathIDOut string, found bool, err error) {
	// Find the leaf in pathID
	found, err = f.listAll(ctx, pathID, true, false, func(item *api.Item) bool {
		if item.Attributes.Name == leaf {
			pathIDOut = item.ID
			return true
		}
		return false
	})
	return pathIDOut, found, err
}

// CreateDir makes a directory with pathID as parent and name leaf
func (f *Fs) CreateDir(ctx context.Context, pathID, leaf string) (newID string, err error) {
	//fs.Debugf(f, "CreateDir(%q, %q)\n", pathID, leaf)
	var resp *http.Response
	var info *api.ItemInfo
	opts := rest.Opts{
		Method:       "POST",
		Path:         "/files",
		ExtraHeaders: map[string]string{"Accept": "application/vnd.api+json"},
	}
	mkdir := api.WriteMetadataRequest{
		Data: api.WriteMetadata{
			Attributes: api.WriteAttributes{
				Name:     f.opt.Enc.FromStandardName(leaf),
				ParentID: pathID,
			},
			Type: "files",
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
	return info.Item.ID, nil
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

// Creates from the parameters passed in a half finished Object which
// must have setMetaData called on it
//
// Used to create new objects
func (f *Fs) createObject(ctx context.Context, remote string, size int64, modTime time.Time) (o *Object, leaf string, directoryID string, err error) {
	leaf, directoryID, err = f.dirCache.FindPath(ctx, remote, true)
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
	return
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
		return f.PutUnchecked(ctx, in, src)
	default:
		return nil, err
	}
}

func isSimpleName(s string) bool {
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r != '.') {
			return false
		}
	}
	return true
}

func (f *Fs) upload(ctx context.Context, name string, parent string, size int64, in io.Reader, options ...fs.OpenOption) (*api.Item, error) {
	params := url.Values{}
	params.Set("filename", name)
	params.Set("parent_id", parent)
	params.Set("override-name-exist", strconv.FormatBool(true))
	formReader, contentType, overhead, err := rest.MultipartUpload(in, nil, "content", name)
	if err != nil {
		return nil, errors.Wrap(err, "failed to make multipart upload")
	}

	contentLength := overhead + size
	opts := rest.Opts{
		Method:           "POST",
		Path:             "/upload",
		Body:             formReader,
		ContentType:      contentType,
		ContentLength:    &contentLength,
		Options:          options,
		Parameters:       params,
		TransferEncoding: []string{"identity"},
	}

	var resp *http.Response
	var uploadResponse *api.UploadResponse
	err = f.pacer.CallNoRetry(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &uploadResponse)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, errors.Wrap(err, "upload error")
	}
	if len(uploadResponse.Uploads) != 1 {
		return nil, errors.New("upload: invalid response")
	}
	// Received meta data is missing size so we have to read it again.
	info, err := f.readMetaDataForID(ctx, uploadResponse.Uploads[0].Attributes.RessourceID)
	if err != nil {
		return nil, err
	}

	return info, nil
}

// PutUnchecked the object into the container
//
// This will produce an error if the object already exists
//
// Copy the reader in to the new object which is returned
//
// The new object may have been created if an error is returned
func (f *Fs) PutUnchecked(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	size := src.Size()
	remote := src.Remote()

	// Create the directory for the object if it doesn't exist
	leaf, directoryID, err := f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return nil, err
	}

	if isSimpleName(leaf) {
		info, err := f.upload(ctx, f.opt.Enc.FromStandardName(leaf), directoryID, size, in, options...)
		if err != nil {
			return nil, err
		}
		return f.newObjectWithInfo(ctx, remote, info)
	}

	tempName := "rcloneTemp" + random.String(8)
	info, err := f.upload(ctx, tempName, directoryID, size, in, options...)
	if err != nil {
		return nil, err
	}

	o, err := f.newObjectWithInfo(ctx, remote, info)
	if err != nil {
		return nil, err
	}
	return o, o.(*Object).rename(ctx, leaf)
}

// Mkdir creates the container if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	_, err := f.dirCache.FindDir(ctx, dir, true)
	return err
}

// deleteObject removes an object by ID
func (f *Fs) deleteObject(ctx context.Context, id string) (err error) {
	var resp *http.Response
	opts := rest.Opts{
		Method:       "PATCH",
		Path:         "/files",
		ExtraHeaders: map[string]string{"Accept": "application/vnd.api+json"},
	}
	delete := api.WriteMultiMetadataRequest{
		Meta: []api.WriteMetadata{
			{
				Attributes: api.WriteAttributes{
					Status: "51", // Status "51" is deleted
				},
				ID:   id,
				Type: "files",
			},
		},
	}
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &delete, nil)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return errors.Wrap(err, "delete object failed")
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

	info, err := f.readMetaDataForID(ctx, rootID)
	if err != nil {
		return err
	}
	if check && info.Attributes.StorageInfo.Size > 0 {
		return fs.ErrorDirectoryNotEmpty
	}

	err = f.deleteObject(ctx, rootID)
	if err != nil {
		return errors.Wrap(err, "rmdir failed")
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

// Purge deletes all the files and the container
//
// Optional interface: Only implement this if you have a way of
// deleting all the files quicker than just running Remove() on the
// result of List()
func (f *Fs) Purge(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir, false)
}

func (f *Fs) rename(ctx context.Context, id, name string) (item *api.Item, err error) {
	var resp *http.Response
	opts := rest.Opts{
		Method:       "PATCH",
		Path:         "/files/" + id,
		ExtraHeaders: map[string]string{"Accept": "application/vnd.api+json"},
	}
	rename := api.WriteMetadataRequest{
		Data: api.WriteMetadata{
			Attributes: api.WriteAttributes{
				Name: f.opt.Enc.FromStandardName(name),
			},
			Type: "files",
		},
	}
	var result *api.ItemInfo
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &rename, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, errors.Wrap(err, "rename failed")
	}
	return &result.Item, nil
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
	dstObject, leaf, directoryID, err := f.createObject(ctx, remote, srcObj.size, srcObj.modTime)
	if err != nil {
		return nil, err
	}
	// Copy the object
	opts := rest.Opts{
		Method:       "POST",
		Path:         "/files/" + directoryID + "/copy",
		ExtraHeaders: map[string]string{"Accept": "application/vnd.api+json"},
	}
	copyFile := api.WriteMultiMetadataRequest{
		Meta: []api.WriteMetadata{
			{
				Attributes: api.WriteAttributes{
					RessourceID: srcObj.id,
				},
				Type: "files",
			},
		},
	}
	var resp *http.Response
	var result *api.ItemList
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &copyFile, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, errors.Wrap(err, "couldn't copy file")
	}
	// Server acts weird some times make sure we actually got
	// an item
	if len(result.Items) != 1 {
		return nil, errors.New("couldn't copy file: invalid response")
	}
	// Only set ID here because response is not complete Item struct
	dstObject.id = result.Items[0].ID

	// Can't copy and change name in one step so we have to check if we have
	// the correct name after copy
	if f.opt.Enc.ToStandardName(result.Items[0].Attributes.Name) != leaf {
		if err = dstObject.rename(ctx, leaf); err != nil {
			return nil, errors.Wrap(err, "copy: couldn't rename copied file")
		}
	}
	return dstObject, nil
}

func (f *Fs) move(ctx context.Context, srcID, parentID string) (item *api.Item, err error) {
	// Move the object
	opts := rest.Opts{
		Method:       "PATCH",
		Path:         "/files",
		ExtraHeaders: map[string]string{"Accept": "application/vnd.api+json"},
	}
	moveFile := api.WriteMultiMetadataRequest{
		Meta: []api.WriteMetadata{
			{
				Attributes: api.WriteAttributes{
					ParentID: parentID,
				},
				ID:   srcID,
				Type: "files",
			},
		},
	}
	var resp *http.Response
	var result *api.ItemList
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &moveFile, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, errors.Wrap(err, "move failed")
	}
	// Server acts weird some times make sure our array actually contains
	// a file
	if len(result.Items) != 1 {
		return nil, errors.New("move failed: invalid response")
	}
	return &result.Items[0], nil
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
	err := srcObj.readMetaData(ctx)
	if err != nil {
		return nil, err
	}

	srcLeaf, srcParentID, err := srcObj.fs.dirCache.FindPath(ctx, srcObj.remote, false)
	if err != nil {
		return nil, err
	}

	// Create temporary object
	dstObject, dstLeaf, directoryID, err := f.createObject(ctx, remote, srcObj.size, srcObj.modTime)
	if err != nil {
		return nil, err
	}

	needRename := srcLeaf != dstLeaf
	needMove := srcParentID != directoryID

	// rename the leaf to a temporary name if we are moving to
	// another directory to make sure we don't overwrite something
	// in the source directory by accident
	if needRename && needMove {
		tmpLeaf := "rcloneTemp" + random.String(8)
		if err = srcObj.rename(ctx, tmpLeaf); err != nil {
			return nil, errors.Wrap(err, "move: pre move rename failed")
		}
	}

	// do the move if required
	if needMove {
		item, err := f.move(ctx, srcObj.id, directoryID)
		if err != nil {
			return nil, err
		}
		// Only set ID here because response is not complete Item struct
		dstObject.id = item.ID
	} else {
		// same parent only need to rename
		dstObject.id = srcObj.id
	}

	// rename the leaf to its final name
	if needRename {
		if err = dstObject.rename(ctx, dstLeaf); err != nil {
			return nil, errors.Wrap(err, "move: couldn't rename moved file")
		}
	}
	return dstObject, nil
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

	srcID, srcDirectoryID, srcLeaf, dstDirectoryID, dstLeaf, err := f.dirCache.DirMove(ctx, srcFs.dirCache, srcFs.root, srcRemote, f.root, dstRemote)
	if err != nil {
		return err
	}
	// same parent only need to rename
	if srcDirectoryID == dstDirectoryID {
		_, err = f.rename(ctx, srcID, dstLeaf)
		return err
	}

	// do the move
	_, err = f.move(ctx, srcID, dstDirectoryID)
	if err != nil {
		return errors.Wrap(err, "couldn't dir move")
	}

	// Can't copy and change name in one step so we have to check if we have
	// the correct name after copy
	if srcLeaf != dstLeaf {
		_, err = f.rename(ctx, srcID, dstLeaf)
		if err != nil {
			return errors.Wrap(err, "dirmove: couldn't rename moved dir")
		}
	}
	srcFs.dirCache.FlushDir(srcRemote)
	return nil
}

// About gets quota information
func (f *Fs) About(ctx context.Context) (usage *fs.Usage, err error) {
	info, err := f.readMetaDataForID(ctx, f.opt.RootFolderID)
	if err != nil {
		return nil, err
	}
	usage = &fs.Usage{
		Used: fs.NewUsageValue(info.Attributes.StorageInfo.Size),
	}
	return usage, nil
}

// DirCacheFlush resets the directory cache - used in testing as an
// optional interface
func (f *Fs) DirCacheFlush() {
	f.dirCache.ResetRoot()
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

// Storable returns a boolean showing whether this object storable
func (o *Object) Storable() bool {
	return true
}

// Hash returns the SHA-1 of an object returning a lowercase hex string
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	return "", nil
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	if err := o.readMetaData(context.TODO()); err != nil {
		fs.Logf(o, "Failed to read metadata: %v", err)
		return 0
	}
	return o.size
}

// setMetaData sets the metadata from info
func (o *Object) setMetaData(info *api.Item) (err error) {
	if info.Attributes.IsFolder {
		return fs.ErrorNotAFile
	}
	o.hasMetaData = true
	o.size = info.Attributes.StorageInfo.Size
	o.modTime = time.Time(info.Attributes.ModifiedTime)
	o.id = info.ID
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

// ModTime returns the modification time of the object
//
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.modTime
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	return fs.ErrorCantSetModTime
}

// rename renames an object in place
//
// this a separate api call then move with zoho
func (o *Object) rename(ctx context.Context, name string) (err error) {
	item, err := o.fs.rename(ctx, o.id, name)
	if err != nil {
		return err
	}
	return o.setMetaData(item)
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	if o.id == "" {
		return nil, errors.New("can't download - no id")
	}
	var start, end int64 = 0, o.size
	partialContent := false
	for _, option := range options {
		switch x := option.(type) {
		case *fs.SeekOption:
			start = x.Offset
			partialContent = true
		case *fs.RangeOption:
			if x.Start >= 0 {
				start = x.Start
				if x.End > 0 && x.End < o.size {
					end = x.End + 1
				}
			} else {
				// {-1, 20} should load the last 20 characters [len-20:len]
				start = o.size - x.End
			}
			partialContent = true
		default:
			if option.Mandatory() {
				fs.Logf(nil, "Unsupported mandatory option: %v", option)
			}
		}
	}
	var resp *http.Response
	opts := rest.Opts{
		Method:  "GET",
		Path:    "/download/" + o.id,
		Options: options,
	}
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.Call(ctx, &opts)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	if partialContent && resp.StatusCode == 200 {
		if start > 0 {
			// We need to read and discard the beginning of the data...
			_, err = io.CopyN(ioutil.Discard, resp.Body, start)
			if err != nil {
				if resp != nil {
					_ = resp.Body.Close()
				}
				return nil, err
			}
		}
		// ... and return a limited reader for the remaining of the data
		return readers.NewLimitedReadCloser(resp.Body, end-start), nil
	}
	return resp.Body, nil
}

// Update the object with the contents of the io.Reader, modTime and size
//
// If existing is set then it updates the object rather than creating a new one
//
// The new object may have been created if an error is returned
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	size := src.Size()
	remote := o.Remote()

	// Create the directory for the object if it doesn't exist
	leaf, directoryID, err := o.fs.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return err
	}

	if isSimpleName(leaf) {
		// Simple name we can just overwrite the old file
		info, err := o.fs.upload(ctx, o.fs.opt.Enc.FromStandardName(leaf), directoryID, size, in, options...)
		if err != nil {
			return err
		}
		return o.setMetaData(info)
	}

	// We have to fall back to upload + rename
	tempName := "rcloneTemp" + random.String(8)
	info, err := o.fs.upload(ctx, tempName, directoryID, size, in, options...)
	if err != nil {
		return err
	}

	// upload was successfull, need to delete old object before rename
	if err = o.Remove(ctx); err != nil {
		return errors.Wrap(err, "failed to remove old object")
	}
	if err = o.setMetaData(info); err != nil {
		return err
	}

	// rename also updates metadata
	return o.rename(ctx, leaf)
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
	_ fs.Copier          = (*Fs)(nil)
	_ fs.Abouter         = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ fs.DirCacheFlusher = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.IDer            = (*Object)(nil)
)
