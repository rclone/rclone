// Package zoho provides an interface to the Zoho Workdrive
// storage system.
package zoho

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

	"github.com/google/uuid"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/random"

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
	maxSleep                    = 60 * time.Second
	decayConstant               = 2 // bigger for slower decay, exponential
	configRootID                = "root_folder_id"

	defaultUploadCutoff = 10 * 1024 * 1024 // 10 MiB
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
			"ZohoFiles.files.ALL",
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
	downloadURL = "https://download.zoho.eu/v1/workdrive"
	uploadURL   = "http://upload.zoho.eu/workdrive-api/v1/"
	accountsURL = "https://accounts.zoho.eu"
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "zoho",
		Description: "Zoho",
		NewFs:       NewFs,
		Config: func(ctx context.Context, name string, m configmap.Mapper, config fs.ConfigIn) (*fs.ConfigOut, error) {
			// Need to setup region before configuring oauth
			err := setupRegion(m)
			if err != nil {
				return nil, err
			}
			getSrvs := func() (authSrv, apiSrv *rest.Client, err error) {
				oAuthClient, _, err := oauthutil.NewClient(ctx, name, m, oauthConfig)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to load OAuth client: %w", err)
				}
				authSrv = rest.NewClient(oAuthClient).SetRoot(accountsURL)
				apiSrv = rest.NewClient(oAuthClient).SetRoot(rootURL)
				return authSrv, apiSrv, nil
			}

			switch config.State {
			case "":
				return oauthutil.ConfigOut("type", &oauthutil.Options{
					OAuth2Config: oauthConfig,
					// No refresh token unless ApprovalForce is set
					OAuth2Opts: []oauth2.AuthCodeOption{oauth2.ApprovalForce},
				})
			case "type":
				// We need to rewrite the token type to "Zoho-oauthtoken" because Zoho wants
				// it's own custom type
				token, err := oauthutil.GetToken(name, m)
				if err != nil {
					return nil, fmt.Errorf("failed to read token: %w", err)
				}
				if token.TokenType != "Zoho-oauthtoken" {
					token.TokenType = "Zoho-oauthtoken"
					err = oauthutil.PutToken(name, m, token, false)
					if err != nil {
						return nil, fmt.Errorf("failed to configure token: %w", err)
					}
				}

				_, apiSrv, err := getSrvs()
				if err != nil {
					return nil, err
				}

				userInfo, err := getUserInfo(ctx, apiSrv)
				if err != nil {
					return nil, err
				}
				// If personal Edition only one private Space is available. Directly configure that.
				if userInfo.Data.Attributes.Edition == "PERSONAL" {
					return fs.ConfigResult("private_space", userInfo.Data.ID)
				}
				// Otherwise go to team selection
				return fs.ConfigResult("team", userInfo.Data.ID)
			case "private_space":
				_, apiSrv, err := getSrvs()
				if err != nil {
					return nil, err
				}

				workspaces, err := getPrivateSpaces(ctx, config.Result, apiSrv)
				if err != nil {
					return nil, err
				}
				return fs.ConfigChoose("workspace_end", "config_workspace", "Workspace ID", len(workspaces), func(i int) (string, string) {
					workspace := workspaces[i]
					return workspace.ID, workspace.Name
				})
			case "team":
				_, apiSrv, err := getSrvs()
				if err != nil {
					return nil, err
				}

				// Get the teams
				teams, err := listTeams(ctx, config.Result, apiSrv)
				if err != nil {
					return nil, err
				}
				return fs.ConfigChoose("workspace", "config_team_drive_id", "Team Drive ID", len(teams), func(i int) (string, string) {
					team := teams[i]
					return team.ID, team.Attributes.Name
				})
			case "workspace":
				_, apiSrv, err := getSrvs()
				if err != nil {
					return nil, err
				}
				teamID := config.Result
				workspaces, err := listWorkspaces(ctx, teamID, apiSrv)
				if err != nil {
					return nil, err
				}
				currentTeamInfo, err := getCurrentTeamInfo(ctx, teamID, apiSrv)
				if err != nil {
					return nil, err
				}
				privateSpaces, err := getPrivateSpaces(ctx, currentTeamInfo.Data.ID, apiSrv)
				if err != nil {
					return nil, err
				}
				workspaces = append(workspaces, privateSpaces...)

				return fs.ConfigChoose("workspace_end", "config_workspace", "Workspace ID", len(workspaces), func(i int) (string, string) {
					workspace := workspaces[i]
					return workspace.ID, workspace.Name
				})
			case "workspace_end":
				workspaceID := config.Result
				m.Set(configRootID, workspaceID)
				return nil, nil
			}
			return nil, fmt.Errorf("unknown state %q", config.State)
		},
		Options: append(oauthutil.SharedOptions, []fs.Option{{
			Name: "region",
			Help: `Zoho region to connect to.

You'll have to use the region your organization is registered in. If
not sure use the same top level domain as you connect to in your
browser.`,
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
				Value: "jp",
				Help:  "Japan",
			}, {
				Value: "com.cn",
				Help:  "China",
			}, {
				Value: "com.au",
				Help:  "Australia",
			}},
		}, {
			Name:     "upload_cutoff",
			Help:     "Cutoff for switching to large file upload api (>= 10 MiB).",
			Default:  fs.SizeSuffix(defaultUploadCutoff),
			Advanced: true,
		}, {
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
	UploadCutoff fs.SizeSuffix        `config:"upload_cutoff"`
	RootFolderID string               `config:"root_folder_id"`
	Region       string               `config:"region"`
	Enc          encoder.MultiEncoder `config:"encoding"`
}

// Fs represents a remote workdrive
type Fs struct {
	name        string             // name of this remote
	root        string             // the path we are working on
	opt         Options            // parsed options
	features    *fs.Features       // optional features
	srv         *rest.Client       // the connection to the server
	downloadsrv *rest.Client       // the connection to the download server
	uploadsrv   *rest.Client       // the connection to the upload server
	dirCache    *dircache.DirCache // Map of directory path to directory id
	pacer       *fs.Pacer          // pacer for API calls
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

func setupRegion(m configmap.Mapper) error {
	region, ok := m.Get("region")
	if !ok || region == "" {
		return errors.New("no region set")
	}
	rootURL = fmt.Sprintf("https://workdrive.zoho.%s/api/v1", region)
	downloadURL = fmt.Sprintf("https://download.zoho.%s/v1/workdrive", region)
	uploadURL = fmt.Sprintf("https://upload.zoho.%s/workdrive-api/v1", region)
	accountsURL = fmt.Sprintf("https://accounts.zoho.%s", region)
	oauthConfig.Endpoint.AuthURL = fmt.Sprintf("https://accounts.zoho.%s/oauth/v2/auth", region)
	oauthConfig.Endpoint.TokenURL = fmt.Sprintf("https://accounts.zoho.%s/oauth/v2/token", region)
	return nil
}

// ------------------------------------------------------------

type workspaceInfo struct {
	ID   string
	Name string
}

func getUserInfo(ctx context.Context, srv *rest.Client) (*api.UserInfoResponse, error) {
	var userInfo api.UserInfoResponse
	opts := rest.Opts{
		Method:       "GET",
		Path:         "/users/me",
		ExtraHeaders: map[string]string{"Accept": "application/vnd.api+json"},
	}
	_, err := srv.CallJSON(ctx, &opts, nil, &userInfo)
	if err != nil {
		return nil, err
	}
	return &userInfo, nil
}

func getCurrentTeamInfo(ctx context.Context, teamID string, srv *rest.Client) (*api.CurrentTeamInfo, error) {
	var currentTeamInfo api.CurrentTeamInfo
	opts := rest.Opts{
		Method:       "GET",
		Path:         "/teams/" + teamID + "/currentuser",
		ExtraHeaders: map[string]string{"Accept": "application/vnd.api+json"},
	}
	_, err := srv.CallJSON(ctx, &opts, nil, &currentTeamInfo)
	if err != nil {
		return nil, err
	}
	return &currentTeamInfo, err
}

func getPrivateSpaces(ctx context.Context, teamUserID string, srv *rest.Client) ([]workspaceInfo, error) {
	var privateSpaceListResponse api.TeamWorkspaceResponse
	opts := rest.Opts{
		Method:       "GET",
		Path:         "/users/" + teamUserID + "/privatespace",
		ExtraHeaders: map[string]string{"Accept": "application/vnd.api+json"},
	}
	_, err := srv.CallJSON(ctx, &opts, nil, &privateSpaceListResponse)
	if err != nil {
		return nil, err
	}

	workspaceList := make([]workspaceInfo, 0, len(privateSpaceListResponse.TeamWorkspace))
	for _, workspace := range privateSpaceListResponse.TeamWorkspace {
		workspaceList = append(workspaceList, workspaceInfo{ID: workspace.ID, Name: "My Space"})
	}
	return workspaceList, err
}

func listTeams(ctx context.Context, zuid string, srv *rest.Client) ([]api.TeamWorkspace, error) {
	var teamList api.TeamWorkspaceResponse
	opts := rest.Opts{
		Method:       "GET",
		Path:         "/users/" + zuid + "/teams",
		ExtraHeaders: map[string]string{"Accept": "application/vnd.api+json"},
	}
	_, err := srv.CallJSON(ctx, &opts, nil, &teamList)
	if err != nil {
		return nil, err
	}
	return teamList.TeamWorkspace, nil
}

func listWorkspaces(ctx context.Context, teamID string, srv *rest.Client) ([]workspaceInfo, error) {
	var workspaceListResponse api.TeamWorkspaceResponse
	opts := rest.Opts{
		Method:       "GET",
		Path:         "/teams/" + teamID + "/workspaces",
		ExtraHeaders: map[string]string{"Accept": "application/vnd.api+json"},
	}
	_, err := srv.CallJSON(ctx, &opts, nil, &workspaceListResponse)
	if err != nil {
		return nil, err
	}

	workspaceList := make([]workspaceInfo, 0, len(workspaceListResponse.TeamWorkspace))
	for _, workspace := range workspaceListResponse.TeamWorkspace {
		workspaceList = append(workspaceList, workspaceInfo{ID: workspace.ID, Name: workspace.Attributes.Name})
	}

	return workspaceList, nil
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

	// Bail out early if we are missing OAuth Scopes.
	if resp != nil && resp.StatusCode == 401 && strings.Contains(resp.Status, "INVALID_OAUTHSCOPE") {
		fs.Errorf(nil, "zoho: missing OAuth Scope. Run rclone config reconnect to fix this issue.")
		return false, err
	}

	if resp != nil && resp.StatusCode == 401 && len(resp.Header["Www-Authenticate"]) == 1 && strings.Contains(resp.Header["Www-Authenticate"][0], "expired_token") {
		authRetry = true
		fs.Debugf(nil, "Should retry: %v", err)
	}
	if resp != nil && resp.StatusCode == 429 {
		err = pacer.RetryAfterError(err, 60*time.Second)
		fs.Debugf(nil, "Too many requests. Trying again in %d seconds.", 60)
		return true, err
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

	if opt.UploadCutoff < defaultUploadCutoff {
		return nil, fmt.Errorf("zoho: upload cutoff (%v) must be greater than equal to %v", opt.UploadCutoff, fs.SizeSuffix(defaultUploadCutoff))
	}

	err := setupRegion(m)
	if err != nil {
		return nil, err
	}

	root = parsePath(root)
	oAuthClient, _, err := oauthutil.NewClient(ctx, name, m, oauthConfig)
	if err != nil {
		return nil, err
	}

	f := &Fs{
		name:        name,
		root:        root,
		opt:         *opt,
		srv:         rest.NewClient(oAuthClient).SetRoot(rootURL),
		downloadsrv: rest.NewClient(oAuthClient).SetRoot(downloadURL),
		uploadsrv:   rest.NewClient(oAuthClient).SetRoot(uploadURL),
		pacer:       fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
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
	const listItemsLimit = 1000
	opts := rest.Opts{
		Method:       "GET",
		Path:         "/files/" + dirID + "/files",
		ExtraHeaders: map[string]string{"Accept": "application/vnd.api+json"},
		Parameters: url.Values{
			"page[limit]": {strconv.Itoa(listItemsLimit)},
			"page[next]":  {"0"},
		},
	}
OUTER:
	for {
		var result api.ItemList
		var resp *http.Response
		err = f.pacer.Call(func() (bool, error) {
			resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
			return shouldRetry(ctx, resp, err)
		})
		if err != nil {
			return found, fmt.Errorf("couldn't list files: %w", err)
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
		if !result.Links.Cursor.HasNext {
			break
		}
		// Fetch the next from the URL in the response
		nextURL, err := url.Parse(result.Links.Cursor.Next)
		if err != nil {
			return found, fmt.Errorf("failed to parse next link as URL: %w", err)
		}
		opts.Parameters.Set("page[next]", nextURL.Query().Get("page[next]"))
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

func (f *Fs) uploadLargeFile(ctx context.Context, name string, parent string, size int64, in io.Reader, options ...fs.OpenOption) (*api.Item, error) {
	opts := rest.Opts{
		Method:        "POST",
		Path:          "/stream/upload",
		Body:          in,
		ContentLength: &size,
		ContentType:   "application/octet-stream",
		Options:       options,
		ExtraHeaders: map[string]string{
			"x-filename":          url.QueryEscape(name),
			"x-parent_id":         parent,
			"override-name-exist": "true",
			"upload-id":           uuid.New().String(),
			"x-streammode":        "1",
		},
	}

	var err error
	var resp *http.Response
	var uploadResponse *api.LargeUploadResponse
	err = f.pacer.CallNoRetry(func() (bool, error) {
		resp, err = f.uploadsrv.CallJSON(ctx, &opts, nil, &uploadResponse)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("upload large error: %v", err)
	}
	if len(uploadResponse.Uploads) != 1 {
		return nil, errors.New("upload: invalid response")
	}
	upload := uploadResponse.Uploads[0]
	uploadInfo, err := upload.GetUploadFileInfo()
	if err != nil {
		return nil, fmt.Errorf("upload error: %w", err)
	}

	// Fill in the api.Item from the api.UploadFileInfo
	var info api.Item
	info.ID = upload.Attributes.RessourceID
	info.Attributes.Name = upload.Attributes.FileName
	// info.Attributes.Type = not used
	info.Attributes.IsFolder = false
	// info.Attributes.CreatedTime = not used
	info.Attributes.ModifiedTime = uploadInfo.GetModTime()
	// info.Attributes.UploadedTime = 0 not used
	info.Attributes.StorageInfo.Size = uploadInfo.Size
	info.Attributes.StorageInfo.FileCount = 0
	info.Attributes.StorageInfo.FolderCount = 0

	return &info, nil
}

func (f *Fs) upload(ctx context.Context, name string, parent string, size int64, in io.Reader, options ...fs.OpenOption) (*api.Item, error) {
	params := url.Values{}
	params.Set("filename", url.QueryEscape(name))
	params.Set("parent_id", parent)
	params.Set("override-name-exist", strconv.FormatBool(true))
	formReader, contentType, overhead, err := rest.MultipartUpload(ctx, in, nil, "content", name)
	if err != nil {
		return nil, fmt.Errorf("failed to make multipart upload: %w", err)
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
		return nil, fmt.Errorf("upload error: %w", err)
	}
	if len(uploadResponse.Uploads) != 1 {
		return nil, errors.New("upload: invalid response")
	}
	upload := uploadResponse.Uploads[0]
	uploadInfo, err := upload.GetUploadFileInfo()
	if err != nil {
		return nil, fmt.Errorf("upload error: %w", err)
	}

	// Fill in the api.Item from the api.UploadFileInfo
	var info api.Item
	info.ID = upload.Attributes.RessourceID
	info.Attributes.Name = upload.Attributes.FileName
	// info.Attributes.Type = not used
	info.Attributes.IsFolder = false
	// info.Attributes.CreatedTime = not used
	info.Attributes.ModifiedTime = uploadInfo.GetModTime()
	// info.Attributes.UploadedTime = 0 not used
	info.Attributes.StorageInfo.Size = uploadInfo.Size
	info.Attributes.StorageInfo.FileCount = 0
	info.Attributes.StorageInfo.FolderCount = 0

	return &info, nil
}

// Put the object into the container
//
// Copy the reader in to the new object which is returned.
//
// The new object may have been created if an error is returned
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	existingObj, err := f.NewObject(ctx, src.Remote())
	switch err {
	case nil:
		return existingObj, existingObj.Update(ctx, in, src, options...)
	case fs.ErrorObjectNotFound:
		size := src.Size()
		remote := src.Remote()

		// Create the directory for the object if it doesn't exist
		leaf, directoryID, err := f.dirCache.FindPath(ctx, remote, true)
		if err != nil {
			return nil, err
		}

		// use normal upload API for small sizes (<10MiB)
		if size < int64(f.opt.UploadCutoff) {
			info, err := f.upload(ctx, f.opt.Enc.FromStandardName(leaf), directoryID, size, in, options...)
			if err != nil {
				return nil, err
			}

			return f.newObjectWithInfo(ctx, remote, info)
		}

		// large file API otherwise
		info, err := f.uploadLargeFile(ctx, f.opt.Enc.FromStandardName(leaf), directoryID, size, in, options...)
		if err != nil {
			return nil, err
		}

		return f.newObjectWithInfo(ctx, remote, info)
	default:
		return nil, err
	}
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
		return fmt.Errorf("delete object failed: %w", err)
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
		return fmt.Errorf("rmdir failed: %w", err)
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
		return nil, fmt.Errorf("rename failed: %w", err)
	}
	return &result.Item, nil
}

// Copy src to this remote using server side copy operations.
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
		return nil, fmt.Errorf("couldn't copy file: %w", err)
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
			return nil, fmt.Errorf("copy: couldn't rename copied file: %w", err)
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
		return nil, fmt.Errorf("move failed: %w", err)
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
			return nil, fmt.Errorf("move: pre move rename failed: %w", err)
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
			return nil, fmt.Errorf("move: couldn't rename moved file: %w", err)
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
		return fmt.Errorf("couldn't dir move: %w", err)
	}

	// Can't copy and change name in one step so we have to check if we have
	// the correct name after copy
	if srcLeaf != dstLeaf {
		_, err = f.rename(ctx, srcID, dstLeaf)
		if err != nil {
			return fmt.Errorf("dirmove: couldn't rename moved dir: %w", err)
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
		return fs.ErrorIsDir
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
	var resp *http.Response
	fs.FixRangeOption(options, o.size)
	opts := rest.Opts{
		Method:  "GET",
		Path:    "/download/" + o.id,
		Options: options,
	}
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.downloadsrv.Call(ctx, &opts)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// Update the object with the contents of the io.Reader, modTime and size
//
// If existing is set then it updates the object rather than creating a new one.
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

	// use normal upload API for small sizes (<10MiB)
	if size < int64(o.fs.opt.UploadCutoff) {
		info, err := o.fs.upload(ctx, o.fs.opt.Enc.FromStandardName(leaf), directoryID, size, in, options...)
		if err != nil {
			return err
		}

		return o.setMetaData(info)
	}

	// large file API otherwise
	info, err := o.fs.uploadLargeFile(ctx, o.fs.opt.Enc.FromStandardName(leaf), directoryID, size, in, options...)
	if err != nil {
		return err
	}

	return o.setMetaData(info)
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
