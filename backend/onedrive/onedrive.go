// Package onedrive provides an interface to the Microsoft OneDrive
// object storage system.
package onedrive

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
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

	"github.com/pkg/errors"
	"github.com/rclone/rclone/backend/onedrive/api"
	"github.com/rclone/rclone/backend/onedrive/quickxorhash"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fs/walk"
	"github.com/rclone/rclone/lib/atexit"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/oauthutil"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/readers"
	"github.com/rclone/rclone/lib/rest"
	"golang.org/x/oauth2"
)

const (
	rcloneClientID              = "b15665d9-eda6-4092-8539-0eec376afd59"
	rcloneEncryptedClientSecret = "_JUdzh3LnKNqSPcf4Wu5fgMFIQOI8glZu_akYgR8yf6egowNBg-R"
	minSleep                    = 10 * time.Millisecond
	maxSleep                    = 2 * time.Second
	decayConstant               = 2 // bigger for slower decay, exponential
	configDriveID               = "drive_id"
	configDriveType             = "drive_type"
	driveTypePersonal           = "personal"
	driveTypeBusiness           = "business"
	driveTypeSharepoint         = "documentLibrary"
	defaultChunkSize            = 10 * fs.Mebi
	chunkSizeMultiple           = 320 * fs.Kibi

	regionGlobal = "global"
	regionUS     = "us"
	regionDE     = "de"
	regionCN     = "cn"
)

// Globals
var (
	authPath  = "/common/oauth2/v2.0/authorize"
	tokenPath = "/common/oauth2/v2.0/token"

	// Description of how to auth for this app for a business account
	oauthConfig = &oauth2.Config{
		Scopes:       []string{"Files.Read", "Files.ReadWrite", "Files.Read.All", "Files.ReadWrite.All", "offline_access", "Sites.Read.All"},
		ClientID:     rcloneClientID,
		ClientSecret: obscure.MustReveal(rcloneEncryptedClientSecret),
		RedirectURL:  oauthutil.RedirectLocalhostURL,
	}

	graphAPIEndpoint = map[string]string{
		"global": "https://graph.microsoft.com",
		"us":     "https://graph.microsoft.us",
		"de":     "https://graph.microsoft.de",
		"cn":     "https://microsoftgraph.chinacloudapi.cn",
	}

	authEndpoint = map[string]string{
		"global": "https://login.microsoftonline.com",
		"us":     "https://login.microsoftonline.us",
		"de":     "https://login.microsoftonline.de",
		"cn":     "https://login.chinacloudapi.cn",
	}

	// QuickXorHashType is the hash.Type for OneDrive
	QuickXorHashType hash.Type
)

// Register with Fs
func init() {
	QuickXorHashType = hash.RegisterHash("QuickXorHash", 40, quickxorhash.New)
	fs.Register(&fs.RegInfo{
		Name:        "onedrive",
		Description: "Microsoft OneDrive",
		NewFs:       NewFs,
		Config: func(ctx context.Context, name string, m configmap.Mapper) error {
			region, _ := m.Get("region")
			graphURL := graphAPIEndpoint[region] + "/v1.0"
			oauthConfig.Endpoint = oauth2.Endpoint{
				AuthURL:  authEndpoint[region] + authPath,
				TokenURL: authEndpoint[region] + tokenPath,
			}
			ci := fs.GetConfig(ctx)
			err := oauthutil.Config(ctx, "onedrive", name, m, oauthConfig, nil)
			if err != nil {
				return errors.Wrap(err, "failed to configure token")
			}

			// Stop if we are running non-interactive config
			if ci.AutoConfirm {
				return nil
			}

			type driveResource struct {
				DriveID   string `json:"id"`
				DriveName string `json:"name"`
				DriveType string `json:"driveType"`
			}
			type drivesResponse struct {
				Drives []driveResource `json:"value"`
			}

			type siteResource struct {
				SiteID   string `json:"id"`
				SiteName string `json:"displayName"`
				SiteURL  string `json:"webUrl"`
			}
			type siteResponse struct {
				Sites []siteResource `json:"value"`
			}

			oAuthClient, _, err := oauthutil.NewClient(ctx, name, m, oauthConfig)
			if err != nil {
				return errors.Wrap(err, "failed to configure OneDrive")
			}
			srv := rest.NewClient(oAuthClient)

			var opts rest.Opts
			var finalDriveID string
			var siteID string
			var relativePath string
			switch config.Choose("Your choice",
				[]string{"onedrive", "sharepoint", "url", "search", "driveid", "siteid", "path"},
				[]string{
					"OneDrive Personal or Business",
					"Root Sharepoint site",
					"Sharepoint site name or URL (e.g. mysite or https://contoso.sharepoint.com/sites/mysite)",
					"Search for a Sharepoint site",
					"Type in driveID (advanced)",
					"Type in SiteID (advanced)",
					"Sharepoint server-relative path (advanced, e.g. /teams/hr)",
				},
				false) {

			case "onedrive":
				opts = rest.Opts{
					Method:  "GET",
					RootURL: graphURL,
					Path:    "/me/drives",
				}
			case "sharepoint":
				opts = rest.Opts{
					Method:  "GET",
					RootURL: graphURL,
					Path:    "/sites/root/drives",
				}
			case "driveid":
				fmt.Printf("Paste your Drive ID here> ")
				finalDriveID = config.ReadLine()
			case "siteid":
				fmt.Printf("Paste your Site ID here> ")
				siteID = config.ReadLine()
			case "url":
				fmt.Println("Example: \"https://contoso.sharepoint.com/sites/mysite\" or \"mysite\"")
				fmt.Printf("Paste your Site URL here> ")
				siteURL := config.ReadLine()
				re := regexp.MustCompile(`https://.*\.sharepoint.com/sites/(.*)`)
				match := re.FindStringSubmatch(siteURL)
				if len(match) == 2 {
					relativePath = "/sites/" + match[1]
				} else {
					relativePath = "/sites/" + siteURL
				}
			case "path":
				fmt.Printf("Enter server-relative URL here> ")
				relativePath = config.ReadLine()
			case "search":
				fmt.Printf("What to search for> ")
				searchTerm := config.ReadLine()
				opts = rest.Opts{
					Method:  "GET",
					RootURL: graphURL,
					Path:    "/sites?search=" + searchTerm,
				}

				sites := siteResponse{}
				_, err := srv.CallJSON(ctx, &opts, nil, &sites)
				if err != nil {
					return errors.Wrap(err, "failed to query available sites")
				}

				if len(sites.Sites) == 0 {
					return errors.Errorf("search for %q returned no results", searchTerm)
				}
				fmt.Printf("Found %d sites, please select the one you want to use:\n", len(sites.Sites))
				for index, site := range sites.Sites {
					fmt.Printf("%d: %s (%s) id=%s\n", index, site.SiteName, site.SiteURL, site.SiteID)
				}
				siteID = sites.Sites[config.ChooseNumber("Chose drive to use:", 0, len(sites.Sites)-1)].SiteID
			}

			// if we use server-relative URL for finding the drive
			if relativePath != "" {
				opts = rest.Opts{
					Method:  "GET",
					RootURL: graphURL,
					Path:    "/sites/root:" + relativePath,
				}
				site := siteResource{}
				_, err := srv.CallJSON(ctx, &opts, nil, &site)
				if err != nil {
					return errors.Wrap(err, "failed to query available site by relative path")
				}
				siteID = site.SiteID
			}

			// if we have a siteID we need to ask for the drives
			if siteID != "" {
				opts = rest.Opts{
					Method:  "GET",
					RootURL: graphURL,
					Path:    "/sites/" + siteID + "/drives",
				}
			}

			// We don't have the final ID yet?
			// query Microsoft Graph
			if finalDriveID == "" {
				drives := drivesResponse{}
				_, err := srv.CallJSON(ctx, &opts, nil, &drives)
				if err != nil {
					return errors.Wrap(err, "failed to query available drives")
				}

				// Also call /me/drive as sometimes /me/drives doesn't return it #4068
				if opts.Path == "/me/drives" {
					opts.Path = "/me/drive"
					meDrive := driveResource{}
					_, err := srv.CallJSON(ctx, &opts, nil, &meDrive)
					if err != nil {
						return errors.Wrap(err, "failed to query available drives")
					}
					found := false
					for _, drive := range drives.Drives {
						if drive.DriveID == meDrive.DriveID {
							found = true
							break
						}
					}
					// add the me drive if not found already
					if !found {
						fs.Debugf(nil, "Adding %v to drives list from /me/drive", meDrive)
						drives.Drives = append(drives.Drives, meDrive)
					}
				}

				if len(drives.Drives) == 0 {
					return errors.New("no drives found")
				}
				fmt.Printf("Found %d drives, please select the one you want to use:\n", len(drives.Drives))
				for index, drive := range drives.Drives {
					fmt.Printf("%d: %s (%s) id=%s\n", index, drive.DriveName, drive.DriveType, drive.DriveID)
				}
				finalDriveID = drives.Drives[config.ChooseNumber("Chose drive to use:", 0, len(drives.Drives)-1)].DriveID
			}

			// Test the driveID and get drive type
			opts = rest.Opts{
				Method:  "GET",
				RootURL: graphURL,
				Path:    "/drives/" + finalDriveID + "/root"}
			var rootItem api.Item
			_, err = srv.CallJSON(ctx, &opts, nil, &rootItem)
			if err != nil {
				return errors.Wrapf(err, "failed to query root for drive %s", finalDriveID)
			}

			fmt.Printf("Found drive '%s' of type '%s', URL: %s\nIs that okay?\n", rootItem.Name, rootItem.ParentReference.DriveType, rootItem.WebURL)
			// This does not work, YET :)
			if !config.ConfirmWithConfig(ctx, m, "config_drive_ok", true) {
				return errors.New("cancelled by user")
			}

			m.Set(configDriveID, finalDriveID)
			m.Set(configDriveType, rootItem.ParentReference.DriveType)
			return nil
		},
		Options: append(oauthutil.SharedOptions, []fs.Option{{
			Name:    "region",
			Help:    "Choose national cloud region for OneDrive.",
			Default: "global",
			Examples: []fs.OptionExample{
				{
					Value: regionGlobal,
					Help:  "Microsoft Cloud Global",
				}, {
					Value: regionUS,
					Help:  "Microsoft Cloud for US Government",
				}, {
					Value: regionDE,
					Help:  "Microsoft Cloud Germany",
				}, {
					Value: regionCN,
					Help:  "Azure and Office 365 operated by 21Vianet in China",
				},
			},
		}, {
			Name: "chunk_size",
			Help: `Chunk size to upload files with - must be multiple of 320k (327,680 bytes).

Above this size files will be chunked - must be multiple of 320k (327,680 bytes) and
should not exceed 250M (262,144,000 bytes) else you may encounter \"Microsoft.SharePoint.Client.InvalidClientQueryException: The request message is too big.\"
Note that the chunks will be buffered into memory.`,
			Default:  defaultChunkSize,
			Advanced: true,
		}, {
			Name:     "drive_id",
			Help:     "The ID of the drive to use",
			Default:  "",
			Advanced: true,
		}, {
			Name:     "drive_type",
			Help:     "The type of the drive ( " + driveTypePersonal + " | " + driveTypeBusiness + " | " + driveTypeSharepoint + " )",
			Default:  "",
			Advanced: true,
		}, {
			Name: "expose_onenote_files",
			Help: `Set to make OneNote files show up in directory listings.

By default rclone will hide OneNote files in directory listings because
operations like "Open" and "Update" won't work on them.  But this
behaviour may also prevent you from deleting them.  If you want to
delete OneNote files or otherwise want them to show up in directory
listing, set this option.`,
			Default:  false,
			Advanced: true,
		}, {
			Name:    "server_side_across_configs",
			Default: false,
			Help: `Allow server-side operations (e.g. copy) to work across different onedrive configs.

This will only work if you are copying between two OneDrive *Personal* drives AND
the files to copy are already shared between them.  In other cases, rclone will
fall back to normal copy (which will be slightly slower).`,
			Advanced: true,
		}, {
			Name:     "list_chunk",
			Help:     "Size of listing chunk.",
			Default:  1000,
			Advanced: true,
		}, {
			Name:    "no_versions",
			Default: false,
			Help: `Remove all versions on modifying operations

Onedrive for business creates versions when rclone uploads new files
overwriting an existing one and when it sets the modification time.

These versions take up space out of the quota.

This flag checks for versions after file upload and setting
modification time and removes all but the last version.

**NB** Onedrive personal can't currently delete versions so don't use
this flag there.
`,
			Advanced: true,
		}, {
			Name:     "link_scope",
			Default:  "anonymous",
			Help:     `Set the scope of the links created by the link command.`,
			Advanced: true,
			Examples: []fs.OptionExample{{
				Value: "anonymous",
				Help:  "Anyone with the link has access, without needing to sign in. This may include people outside of your organization. Anonymous link support may be disabled by an administrator.",
			}, {
				Value: "organization",
				Help:  "Anyone signed into your organization (tenant) can use the link to get access. Only available in OneDrive for Business and SharePoint.",
			}},
		}, {
			Name:     "link_type",
			Default:  "view",
			Help:     `Set the type of the links created by the link command.`,
			Advanced: true,
			Examples: []fs.OptionExample{{
				Value: "view",
				Help:  "Creates a read-only link to the item.",
			}, {
				Value: "edit",
				Help:  "Creates a read-write link to the item.",
			}, {
				Value: "embed",
				Help:  "Creates an embeddable link to the item.",
			}},
		}, {
			Name:    "link_password",
			Default: "",
			Help: `Set the password for links created by the link command.

At the time of writing this only works with OneDrive personal paid accounts.
`,
			Advanced: true,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			// List of replaced characters:
			//   < (less than)     -> '＜' // FULLWIDTH LESS-THAN SIGN
			//   > (greater than)  -> '＞' // FULLWIDTH GREATER-THAN SIGN
			//   : (colon)         -> '：' // FULLWIDTH COLON
			//   " (double quote)  -> '＂' // FULLWIDTH QUOTATION MARK
			//   \ (backslash)     -> '＼' // FULLWIDTH REVERSE SOLIDUS
			//   | (vertical line) -> '｜' // FULLWIDTH VERTICAL LINE
			//   ? (question mark) -> '？' // FULLWIDTH QUESTION MARK
			//   * (asterisk)      -> '＊' // FULLWIDTH ASTERISK
			//
			// Folder names cannot begin with a tilde ('~')
			// List of replaced characters:
			//   ~ (tilde)        -> '～'  // FULLWIDTH TILDE
			//
			// Additionally names can't begin with a space ( ) or end with a period (.) or space ( ).
			// List of replaced characters:
			//   . (period)        -> '．' // FULLWIDTH FULL STOP
			//     (space)         -> '␠'  // SYMBOL FOR SPACE
			//
			// Also encode invalid UTF-8 bytes as json doesn't handle them.
			//
			// The OneDrive API documentation lists the set of reserved characters, but
			// testing showed this list is incomplete. This are the differences:
			//  - " (double quote) is rejected, but missing in the documentation
			//  - space at the end of file and folder names is rejected, but missing in the documentation
			//  - period at the end of file names is rejected, but missing in the documentation
			//
			// Adding these restrictions to the OneDrive API documentation yields exactly
			// the same rules as the Windows naming conventions.
			//
			// https://docs.microsoft.com/en-us/onedrive/developer/rest-api/concepts/addressing-driveitems?view=odsp-graph-online#path-encoding
			Default: (encoder.Display |
				encoder.EncodeBackSlash |
				encoder.EncodeLeftSpace |
				encoder.EncodeLeftTilde |
				encoder.EncodeRightPeriod |
				encoder.EncodeRightSpace |
				encoder.EncodeWin |
				encoder.EncodeInvalidUtf8),
		}}...),
	})
}

// Options defines the configuration for this backend
type Options struct {
	Region                  string               `config:"region"`
	ChunkSize               fs.SizeSuffix        `config:"chunk_size"`
	DriveID                 string               `config:"drive_id"`
	DriveType               string               `config:"drive_type"`
	ExposeOneNoteFiles      bool                 `config:"expose_onenote_files"`
	ServerSideAcrossConfigs bool                 `config:"server_side_across_configs"`
	ListChunk               int64                `config:"list_chunk"`
	NoVersions              bool                 `config:"no_versions"`
	LinkScope               string               `config:"link_scope"`
	LinkType                string               `config:"link_type"`
	LinkPassword            string               `config:"link_password"`
	Enc                     encoder.MultiEncoder `config:"encoding"`
}

// Fs represents a remote one drive
type Fs struct {
	name         string             // name of this remote
	root         string             // the path we are working on
	opt          Options            // parsed options
	ci           *fs.ConfigInfo     // global config
	features     *fs.Features       // optional features
	srv          *rest.Client       // the connection to the one drive server
	dirCache     *dircache.DirCache // Map of directory path to directory id
	pacer        *fs.Pacer          // pacer for API calls
	tokenRenewer *oauthutil.Renew   // renew the token on expiry
	driveID      string             // ID to use for querying Microsoft Graph
	driveType    string             // https://developer.microsoft.com/en-us/graph/docs/api-reference/v1.0/resources/drive
}

// Object describes a one drive object
//
// Will definitely have info but maybe not meta
type Object struct {
	fs            *Fs       // what this object is part of
	remote        string    // The remote path
	hasMetaData   bool      // whether info below has been set
	isOneNoteFile bool      // Whether the object is a OneNote file
	size          int64     // size of the object
	modTime       time.Time // modification time of the object
	id            string    // ID of the object
	sha1          string    // SHA-1 of the object content
	quickxorhash  string    // QuickXorHash of the object content
	mimeType      string    // Content-Type of object from server (may not be as uploaded)
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

// parsePath parses a one drive 'url'
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

var gatewayTimeoutError sync.Once
var errAsyncJobAccessDenied = errors.New("async job failed - access denied")

// shouldRetry returns a boolean as to whether this resp and err
// deserve to be retried.  It returns the err as a convenience
func shouldRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	retry := false
	if resp != nil {
		switch resp.StatusCode {
		case 401:
			if len(resp.Header["Www-Authenticate"]) == 1 && strings.Index(resp.Header["Www-Authenticate"][0], "expired_token") >= 0 {
				retry = true
				fs.Debugf(nil, "Should retry: %v", err)
			} else if err != nil && strings.Contains(err.Error(), "Unable to initialize RPS") {
				retry = true
				fs.Debugf(nil, "HTTP 401: Unable to initialize RPS. Trying again.")
			}
		case 429: // Too Many Requests.
			// see https://docs.microsoft.com/en-us/sharepoint/dev/general-development/how-to-avoid-getting-throttled-or-blocked-in-sharepoint-online
			if values := resp.Header["Retry-After"]; len(values) == 1 && values[0] != "" {
				retryAfter, parseErr := strconv.Atoi(values[0])
				if parseErr != nil {
					fs.Debugf(nil, "Failed to parse Retry-After: %q: %v", values[0], parseErr)
				} else {
					duration := time.Second * time.Duration(retryAfter)
					retry = true
					err = pacer.RetryAfterError(err, duration)
					fs.Debugf(nil, "Too many requests. Trying again in %d seconds.", retryAfter)
				}
			}
		case 504: // Gateway timeout
			gatewayTimeoutError.Do(func() {
				fs.Errorf(nil, "%v: upload chunks may be taking too long - try reducing --onedrive-chunk-size or decreasing --transfers", err)
			})
		case 507: // Insufficient Storage
			return false, fserrors.FatalError(err)
		}
	}
	return retry || fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// readMetaDataForPathRelativeToID reads the metadata for a path relative to an item that is addressed by its normalized ID.
// if `relPath` == "", it reads the metadata for the item with that ID.
//
// We address items using the pattern `drives/driveID/items/itemID:/relativePath`
// instead of simply using `drives/driveID/root:/itemPath` because it works for
// "shared with me" folders in OneDrive Personal (See #2536, #2778)
// This path pattern comes from https://github.com/OneDrive/onedrive-api-docs/issues/908#issuecomment-417488480
//
// If `relPath` == '', do not append the slash (See #3664)
func (f *Fs) readMetaDataForPathRelativeToID(ctx context.Context, normalizedID string, relPath string) (info *api.Item, resp *http.Response, err error) {
	opts, _ := f.newOptsCallWithIDPath(normalizedID, relPath, true, "GET", "")

	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &info)
		return shouldRetry(ctx, resp, err)
	})

	return info, resp, err
}

// readMetaDataForPath reads the metadata from the path (relative to the absolute root)
func (f *Fs) readMetaDataForPath(ctx context.Context, path string) (info *api.Item, resp *http.Response, err error) {
	firstSlashIndex := strings.IndexRune(path, '/')

	if f.driveType != driveTypePersonal || firstSlashIndex == -1 {
		var opts rest.Opts
		opts = f.newOptsCallWithPath(ctx, path, "GET", "")
		opts.Path = strings.TrimSuffix(opts.Path, ":")
		err = f.pacer.Call(func() (bool, error) {
			resp, err = f.srv.CallJSON(ctx, &opts, nil, &info)
			return shouldRetry(ctx, resp, err)
		})
		return info, resp, err
	}

	// The following branch handles the case when we're using OneDrive Personal and the path is in a folder.
	// For OneDrive Personal, we need to consider the "shared with me" folders.
	// An item in such a folder can only be addressed by its ID relative to the sharer's driveID or
	// by its path relative to the folder's ID relative to the sharer's driveID.
	// Note: A "shared with me" folder can only be placed in the sharee's absolute root.
	// So we read metadata relative to a suitable folder's normalized ID.
	var dirCacheFoundRoot bool
	var rootNormalizedID string
	if f.dirCache != nil {
		rootNormalizedID, err = f.dirCache.RootID(ctx, false)
		dirCacheRootIDExists := err == nil
		if f.root == "" {
			// if f.root == "", it means f.root is the absolute root of the drive
			// and its ID should have been found in NewFs
			dirCacheFoundRoot = dirCacheRootIDExists
		} else if _, err := f.dirCache.RootParentID(ctx, false); err == nil {
			// if root is in a folder, it must have a parent folder, and
			// if dirCache has found root in NewFs, the parent folder's ID
			// should be present.
			// This RootParentID() check is a fix for #3164 which describes
			// a possible case where the root is not found.
			dirCacheFoundRoot = dirCacheRootIDExists
		}
	}

	relPath, insideRoot := getRelativePathInsideBase(f.root, path)
	var firstDir, baseNormalizedID string
	if !insideRoot || !dirCacheFoundRoot {
		// We do not have the normalized ID in dirCache for our query to base on. Query it manually.
		firstDir, relPath = path[:firstSlashIndex], path[firstSlashIndex+1:]
		info, resp, err := f.readMetaDataForPath(ctx, firstDir)
		if err != nil {
			return info, resp, err
		}
		baseNormalizedID = info.GetID()
	} else {
		if f.root != "" {
			// Read metadata based on root
			baseNormalizedID = rootNormalizedID
		} else {
			// Read metadata based on firstDir
			firstDir, relPath = path[:firstSlashIndex], path[firstSlashIndex+1:]
			baseNormalizedID, err = f.dirCache.FindDir(ctx, firstDir, false)
			if err != nil {
				return nil, nil, err
			}
		}
	}

	return f.readMetaDataForPathRelativeToID(ctx, baseNormalizedID, relPath)
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

func checkUploadChunkSize(cs fs.SizeSuffix) error {
	const minChunkSize = fs.SizeSuffixBase
	if cs%chunkSizeMultiple != 0 {
		return errors.Errorf("%s is not a multiple of %s", cs, chunkSizeMultiple)
	}
	if cs < minChunkSize {
		return errors.Errorf("%s is less than %s", cs, minChunkSize)
	}
	return nil
}

func (f *Fs) setUploadChunkSize(cs fs.SizeSuffix) (old fs.SizeSuffix, err error) {
	err = checkUploadChunkSize(cs)
	if err == nil {
		old, f.opt.ChunkSize = f.opt.ChunkSize, cs
	}
	return
}

// NewFs constructs an Fs from the path, container:path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	err = checkUploadChunkSize(opt.ChunkSize)
	if err != nil {
		return nil, errors.Wrap(err, "onedrive: chunk size")
	}

	if opt.DriveID == "" || opt.DriveType == "" {
		return nil, errors.New("unable to get drive_id and drive_type - if you are upgrading from older versions of rclone, please run `rclone config` and re-configure this backend")
	}

	rootURL := graphAPIEndpoint[opt.Region] + "/v1.0" + "/drives/" + opt.DriveID
	oauthConfig.Endpoint = oauth2.Endpoint{
		AuthURL:  authEndpoint[opt.Region] + authPath,
		TokenURL: authEndpoint[opt.Region] + tokenPath,
	}

	root = parsePath(root)
	oAuthClient, ts, err := oauthutil.NewClient(ctx, name, m, oauthConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to configure OneDrive")
	}

	ci := fs.GetConfig(ctx)
	f := &Fs{
		name:      name,
		root:      root,
		opt:       *opt,
		ci:        ci,
		driveID:   opt.DriveID,
		driveType: opt.DriveType,
		srv:       rest.NewClient(oAuthClient).SetRoot(rootURL),
		pacer:     fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
	}
	f.features = (&fs.Features{
		CaseInsensitive:         true,
		ReadMimeType:            true,
		CanHaveEmptyDirectories: true,
		ServerSideAcrossConfigs: opt.ServerSideAcrossConfigs,
	}).Fill(ctx, f)
	f.srv.SetErrorHandler(errorHandler)

	// Renew the token in the background
	f.tokenRenewer = oauthutil.NewRenew(f.String(), ts, func() error {
		_, _, err := f.readMetaDataForPath(ctx, "")
		return err
	})

	// Get rootID
	rootInfo, _, err := f.readMetaDataForPath(ctx, "")
	if err != nil || rootInfo.GetID() == "" {
		return nil, errors.Wrap(err, "failed to get root")
	}

	f.dirCache = dircache.New(root, rootInfo.GetID(), f)

	// Find the current root
	err = f.dirCache.FindRoot(ctx, false)
	if err != nil {
		// Assume it is a file
		newRoot, remote := dircache.SplitPath(root)
		tempF := *f
		tempF.dirCache = dircache.New(newRoot, rootInfo.ID, &tempF)
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
	// fs.Debugf(f, "FindLeaf(%q, %q)", pathID, leaf)
	_, ok := f.dirCache.GetInv(pathID)
	if !ok {
		return "", false, errors.New("couldn't find parent ID")
	}
	info, resp, err := f.readMetaDataForPathRelativeToID(ctx, pathID, leaf)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return "", false, nil
		}
		return "", false, err
	}
	if info.GetPackageType() == api.PackageTypeOneNote {
		return "", false, errors.New("found OneNote file when looking for folder")
	}
	if info.GetFolder() == nil {
		return "", false, errors.New("found file when looking for folder")
	}
	return info.GetID(), true, nil
}

// CreateDir makes a directory with pathID as parent and name leaf
func (f *Fs) CreateDir(ctx context.Context, dirID, leaf string) (newID string, err error) {
	// fs.Debugf(f, "CreateDir(%q, %q)\n", dirID, leaf)
	var resp *http.Response
	var info *api.Item
	opts := f.newOptsCall(dirID, "POST", "/children")
	mkdir := api.CreateItemRequest{
		Name:             f.opt.Enc.FromStandardName(leaf),
		ConflictBehavior: "fail",
	}
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &mkdir, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		//fmt.Printf("...Error %v\n", err)
		return "", err
	}

	//fmt.Printf("...Id %q\n", *info.Id)
	return info.GetID(), nil
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
	// Top parameter asks for bigger pages of data
	// https://dev.onedrive.com/odata/optional-query-parameters.htm
	opts := f.newOptsCall(dirID, "GET", fmt.Sprintf("/children?$top=%d", f.opt.ListChunk))
OUTER:
	for {
		var result api.ListChildrenResponse
		var resp *http.Response
		err = f.pacer.Call(func() (bool, error) {
			resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
			return shouldRetry(ctx, resp, err)
		})
		if err != nil {
			return found, errors.Wrap(err, "couldn't list files")
		}
		if len(result.Value) == 0 {
			break
		}
		for i := range result.Value {
			item := &result.Value[i]
			isFolder := item.GetFolder() != nil
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
			item.Name = f.opt.Enc.ToStandardName(item.GetName())
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
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return nil, err
	}
	var iErr error
	_, err = f.listAll(ctx, directoryID, false, false, func(info *api.Item) bool {
		if !f.opt.ExposeOneNoteFiles && info.GetPackageType() == api.PackageTypeOneNote {
			fs.Debugf(info.Name, "OneNote file not shown in directory listing")
			return false
		}

		remote := path.Join(dir, info.GetName())
		folder := info.GetFolder()
		if folder != nil {
			// cache the directory ID for later lookups
			id := info.GetID()
			f.dirCache.Put(remote, id)
			d := fs.NewDir(remote, time.Time(info.GetLastModifiedDateTime())).SetID(id)
			d.SetItems(folder.ChildCount)
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

// deleteObject removes an object by ID
func (f *Fs) deleteObject(ctx context.Context, id string) error {
	opts := f.newOptsCall(id, "DELETE", "")
	opts.NoResponse = true

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
	if check {
		// check to see if there are any items
		found, err := f.listAll(ctx, rootID, false, false, func(item *api.Item) bool {
			return true
		})
		if err != nil {
			return err
		}
		if found {
			return fs.ErrorDirectoryNotEmpty
		}
	}
	err = f.deleteObject(ctx, rootID)
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
	return time.Second
}

// waitForJob waits for the job with status in url to complete
func (f *Fs) waitForJob(ctx context.Context, location string, o *Object) error {
	deadline := time.Now().Add(f.ci.TimeoutOrInfinite())
	for time.Now().Before(deadline) {
		var resp *http.Response
		var err error
		var body []byte
		err = f.pacer.Call(func() (bool, error) {
			resp, err = http.Get(location)
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

		switch status.Status {
		case "failed":
			if strings.HasPrefix(status.ErrorCode, "AccessDenied_") {
				return errAsyncJobAccessDenied
			}
			fallthrough
		case "deleteFailed":
			return errors.Errorf("%s: async operation returned %q", o.remote, status.Status)
		case "completed":
			err = o.readMetaData(ctx)
			return errors.Wrapf(err, "async operation completed but readMetaData failed")
		}

		time.Sleep(1 * time.Second)
	}
	return errors.Errorf("async operation didn't complete after %v", f.ci.TimeoutOrInfinite())
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
	if f.driveType != srcObj.fs.driveType {
		fs.Debugf(src, "Can't server-side copy - drive types differ")
		return nil, fs.ErrorCantCopy
	}

	// For OneDrive Business, this is only supported within the same drive
	if f.driveType != driveTypePersonal && srcObj.fs.driveID != f.driveID {
		fs.Debugf(src, "Can't server-side copy - cross-drive but not OneDrive Personal")
		return nil, fs.ErrorCantCopy
	}

	err := srcObj.readMetaData(ctx)
	if err != nil {
		return nil, err
	}

	// Check we aren't overwriting a file on the same remote
	if srcObj.fs == f {
		srcPath := srcObj.rootPath()
		dstPath := f.rootPath(remote)
		if strings.ToLower(srcPath) == strings.ToLower(dstPath) {
			return nil, errors.Errorf("can't copy %q -> %q as are same name when lowercase", srcPath, dstPath)
		}
	}

	// Create temporary object
	dstObj, leaf, directoryID, err := f.createObject(ctx, remote, srcObj.modTime, srcObj.size)
	if err != nil {
		return nil, err
	}

	// Copy the object
	// The query param is a workaround for OneDrive Business for #4590
	opts := f.newOptsCall(srcObj.id, "POST", "/copy?@microsoft.graph.conflictBehavior=replace")
	opts.ExtraHeaders = map[string]string{"Prefer": "respond-async"}
	opts.NoResponse = true

	id, dstDriveID, _ := f.parseNormalizedID(directoryID)

	replacedLeaf := f.opt.Enc.FromStandardName(leaf)
	copyReq := api.CopyItemRequest{
		Name: &replacedLeaf,
		ParentReference: api.ItemReference{
			DriveID: dstDriveID,
			ID:      id,
		},
	}
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &copyReq, nil)
		return shouldRetry(ctx, resp, err)
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
	err = f.waitForJob(ctx, location, dstObj)
	if err == errAsyncJobAccessDenied {
		fs.Debugf(src, "Server-side copy failed - file not shared between drives")
		return nil, fs.ErrorCantCopy
	}
	if err != nil {
		return nil, err
	}

	// Copy does NOT copy the modTime from the source and there seems to
	// be no way to set date before
	// This will create TWO versions on OneDrive
	err = dstObj.SetModTime(ctx, srcObj.ModTime(ctx))
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

	id, dstDriveID, _ := f.parseNormalizedID(directoryID)
	_, srcObjDriveID, _ := f.parseNormalizedID(srcObj.id)

	if f.canonicalDriveID(dstDriveID) != srcObj.fs.canonicalDriveID(srcObjDriveID) {
		// https://docs.microsoft.com/en-us/graph/api/driveitem-move?view=graph-rest-1.0
		// "Items cannot be moved between Drives using this request."
		fs.Debugf(f, "Can't move files between drives (%q != %q)", dstDriveID, srcObjDriveID)
		return nil, fs.ErrorCantMove
	}

	// Move the object
	opts := f.newOptsCall(srcObj.id, "PATCH", "")

	move := api.MoveItemRequest{
		Name: f.opt.Enc.FromStandardName(leaf),
		ParentReference: &api.ItemReference{
			DriveID: dstDriveID,
			ID:      id,
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
		resp, err = f.srv.CallJSON(ctx, &opts, &move, &info)
		return shouldRetry(ctx, resp, err)
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

	parsedDstDirID, dstDriveID, _ := f.parseNormalizedID(dstDirectoryID)
	_, srcDriveID, _ := f.parseNormalizedID(srcID)

	if f.canonicalDriveID(dstDriveID) != srcFs.canonicalDriveID(srcDriveID) {
		// https://docs.microsoft.com/en-us/graph/api/driveitem-move?view=graph-rest-1.0
		// "Items cannot be moved between Drives using this request."
		fs.Debugf(f, "Can't move directories between drives (%q != %q)", dstDriveID, srcDriveID)
		return fs.ErrorCantDirMove
	}

	// Get timestamps of src so they can be preserved
	srcInfo, _, err := srcFs.readMetaDataForPathRelativeToID(ctx, srcID, "")
	if err != nil {
		return err
	}

	// Do the move
	opts := f.newOptsCall(srcID, "PATCH", "")
	move := api.MoveItemRequest{
		Name: f.opt.Enc.FromStandardName(dstLeaf),
		ParentReference: &api.ItemReference{
			DriveID: dstDriveID,
			ID:      parsedDstDirID,
		},
		// We set the mod time too as it gets reset otherwise
		FileSystemInfo: &api.FileSystemInfoFacet{
			CreatedDateTime:      srcInfo.CreatedDateTime,
			LastModifiedDateTime: srcInfo.LastModifiedDateTime,
		},
	}
	var resp *http.Response
	var info api.Item
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &move, &info)
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

// About gets quota information
func (f *Fs) About(ctx context.Context) (usage *fs.Usage, err error) {
	var drive api.Drive
	opts := rest.Opts{
		Method: "GET",
		Path:   "",
	}
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &drive)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, errors.Wrap(err, "about failed")
	}
	q := drive.Quota
	// On (some?) Onedrive sharepoints these are all 0 so return unknown in that case
	if q.Total == 0 && q.Used == 0 && q.Deleted == 0 && q.Remaining == 0 {
		return &fs.Usage{}, nil
	}
	usage = &fs.Usage{
		Total:   fs.NewUsageValue(q.Total),     // quota of bytes that can be used
		Used:    fs.NewUsageValue(q.Used),      // bytes in use
		Trashed: fs.NewUsageValue(q.Deleted),   // bytes in trash
		Free:    fs.NewUsageValue(q.Remaining), // bytes which can be uploaded before reaching the quota
	}
	return usage, nil
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	if f.driveType == driveTypePersonal {
		return hash.Set(hash.SHA1)
	}
	return hash.Set(QuickXorHashType)
}

// PublicLink returns a link for downloading without account.
func (f *Fs) PublicLink(ctx context.Context, remote string, expire fs.Duration, unlink bool) (link string, err error) {
	info, _, err := f.readMetaDataForPath(ctx, f.rootPath(remote))
	if err != nil {
		return "", err
	}
	opts := f.newOptsCall(info.GetID(), "POST", "/createLink")

	share := api.CreateShareLinkRequest{
		Type:     f.opt.LinkType,
		Scope:    f.opt.LinkScope,
		Password: f.opt.LinkPassword,
	}

	if expire < fs.DurationOff {
		expiry := time.Now().Add(time.Duration(expire))
		share.Expiry = &expiry
	}

	var resp *http.Response
	var result api.CreateShareLinkResponse
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &share, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		fmt.Println(err)
		return "", err
	}
	return result.Link.WebURL, nil
}

// CleanUp deletes all the hidden files.
func (f *Fs) CleanUp(ctx context.Context) error {
	token := make(chan struct{}, f.ci.Checkers)
	var wg sync.WaitGroup
	err := walk.Walk(ctx, f, "", true, -1, func(path string, entries fs.DirEntries, err error) error {
		err = entries.ForObjectError(func(obj fs.Object) error {
			o, ok := obj.(*Object)
			if !ok {
				return errors.New("internal error: not a onedrive object")
			}
			wg.Add(1)
			token <- struct{}{}
			go func() {
				defer func() {
					<-token
					wg.Done()
				}()
				err := o.deleteVersions(ctx)
				if err != nil {
					fs.Errorf(o, "Failed to remove versions: %v", err)
				}
			}()
			return nil
		})
		wg.Wait()
		return err
	})
	return err
}

// Finds and removes any old versions for o
func (o *Object) deleteVersions(ctx context.Context) error {
	opts := o.fs.newOptsCall(o.id, "GET", "/versions")
	var versions api.VersionsResponse
	err := o.fs.pacer.Call(func() (bool, error) {
		resp, err := o.fs.srv.CallJSON(ctx, &opts, nil, &versions)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return err
	}
	if len(versions.Versions) < 2 {
		return nil
	}
	for _, version := range versions.Versions[1:] {
		err = o.deleteVersion(ctx, version.ID)
		if err != nil {
			return err
		}
	}
	return nil
}

// Finds and removes any old versions for o
func (o *Object) deleteVersion(ctx context.Context, ID string) error {
	if operations.SkipDestructive(ctx, fmt.Sprintf("%s of %s", ID, o.remote), "delete version") {
		return nil
	}
	fs.Infof(o, "removing version %q", ID)
	opts := o.fs.newOptsCall(o.id, "DELETE", "/versions/"+ID)
	opts.NoResponse = true
	return o.fs.pacer.Call(func() (bool, error) {
		resp, err := o.fs.srv.Call(ctx, &opts)
		return shouldRetry(ctx, resp, err)
	})
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

// rootPath returns a path for use in server given a remote
func (f *Fs) rootPath(remote string) string {
	return f.rootSlash() + remote
}

// rootPath returns a path for use in local functions
func (o *Object) rootPath() string {
	return o.fs.rootPath(o.remote)
}

// srvPath returns a path for use in server given a remote
func (f *Fs) srvPath(remote string) string {
	return f.opt.Enc.FromStandardPath(f.rootSlash() + remote)
}

// srvPath returns a path for use in server
func (o *Object) srvPath() string {
	return o.fs.srvPath(o.remote)
}

// Hash returns the SHA-1 of an object returning a lowercase hex string
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	if o.fs.driveType == driveTypePersonal {
		if t == hash.SHA1 {
			return o.sha1, nil
		}
	} else {
		if t == QuickXorHashType {
			return o.quickxorhash, nil
		}
	}
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
	if info.GetFolder() != nil {
		return errors.Wrapf(fs.ErrorNotAFile, "%q", o.remote)
	}
	o.hasMetaData = true
	o.size = info.GetSize()

	o.isOneNoteFile = info.GetPackageType() == api.PackageTypeOneNote

	// Docs: https://docs.microsoft.com/en-us/onedrive/developer/rest-api/resources/hashes
	//
	// We use SHA1 for onedrive personal and QuickXorHash for onedrive for business
	file := info.GetFile()
	if file != nil {
		o.mimeType = file.MimeType
		if file.Hashes.Sha1Hash != "" {
			o.sha1 = strings.ToLower(file.Hashes.Sha1Hash)
		}
		if file.Hashes.QuickXorHash != "" {
			h, err := base64.StdEncoding.DecodeString(file.Hashes.QuickXorHash)
			if err != nil {
				fs.Errorf(o, "Failed to decode QuickXorHash %q: %v", file.Hashes.QuickXorHash, err)
			} else {
				o.quickxorhash = hex.EncodeToString(h)
			}
		}
	}
	fileSystemInfo := info.GetFileSystemInfo()
	if fileSystemInfo != nil {
		o.modTime = time.Time(fileSystemInfo.LastModifiedDateTime)
	} else {
		o.modTime = time.Time(info.GetLastModifiedDateTime())
	}
	o.id = info.GetID()
	return nil
}

// readMetaData gets the metadata if it hasn't already been fetched
//
// it also sets the info
func (o *Object) readMetaData(ctx context.Context) (err error) {
	if o.hasMetaData {
		return nil
	}
	info, _, err := o.fs.readMetaDataForPath(ctx, o.rootPath())
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
	opts := o.fs.newOptsCallWithPath(ctx, o.remote, "PATCH", "")
	update := api.SetFileSystemInfo{
		FileSystemInfo: api.FileSystemInfoFacet{
			CreatedDateTime:      api.Timestamp(modTime),
			LastModifiedDateTime: api.Timestamp(modTime),
		},
	}
	var info *api.Item
	err := o.fs.pacer.Call(func() (bool, error) {
		resp, err := o.fs.srv.CallJSON(ctx, &opts, &update, &info)
		return shouldRetry(ctx, resp, err)
	})
	// Remove versions if required
	if o.fs.opt.NoVersions {
		err := o.deleteVersions(ctx)
		if err != nil {
			fs.Errorf(o, "Failed to remove versions: %v", err)
		}
	}
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
	if o.isOneNoteFile {
		return nil, errors.New("can't open a OneNote file")
	}

	fs.FixRangeOption(options, o.size)
	var resp *http.Response
	opts := o.fs.newOptsCall(o.id, "GET", "/content")
	opts.Options = options

	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.Call(ctx, &opts)
		return shouldRetry(ctx, resp, err)
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
func (o *Object) createUploadSession(ctx context.Context, modTime time.Time) (response *api.CreateUploadResponse, err error) {
	opts := o.fs.newOptsCallWithPath(ctx, o.remote, "POST", "/createUploadSession")
	createRequest := api.CreateUploadRequest{}
	createRequest.Item.FileSystemInfo.CreatedDateTime = api.Timestamp(modTime)
	createRequest.Item.FileSystemInfo.LastModifiedDateTime = api.Timestamp(modTime)
	var resp *http.Response
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.CallJSON(ctx, &opts, &createRequest, &response)
		if apiErr, ok := err.(*api.Error); ok {
			if apiErr.ErrorInfo.Code == "nameAlreadyExists" {
				// Make the error more user-friendly
				err = errors.New(err.Error() + " (is it a OneNote file?)")
			}
		}
		return shouldRetry(ctx, resp, err)
	})
	return response, err
}

// getPosition gets the current position in a multipart upload
func (o *Object) getPosition(ctx context.Context, url string) (pos int64, err error) {
	opts := rest.Opts{
		Method:  "GET",
		RootURL: url,
	}
	var info api.UploadFragmentResponse
	var resp *http.Response
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.CallJSON(ctx, &opts, nil, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return 0, err
	}
	if len(info.NextExpectedRanges) != 1 {
		return 0, errors.Errorf("bad number of ranges in upload position: %v", info.NextExpectedRanges)
	}
	position := info.NextExpectedRanges[0]
	i := strings.IndexByte(position, '-')
	if i < 0 {
		return 0, errors.Errorf("no '-' in next expected range: %q", position)
	}
	position = position[:i]
	pos, err = strconv.ParseInt(position, 10, 64)
	if err != nil {
		return 0, errors.Wrapf(err, "bad expected range: %q", position)
	}
	return pos, nil
}

// uploadFragment uploads a part
func (o *Object) uploadFragment(ctx context.Context, url string, start int64, totalSize int64, chunk io.ReadSeeker, chunkSize int64, options ...fs.OpenOption) (info *api.Item, err error) {
	//	var response api.UploadFragmentResponse
	var resp *http.Response
	var body []byte
	var skip = int64(0)
	err = o.fs.pacer.Call(func() (bool, error) {
		toSend := chunkSize - skip
		opts := rest.Opts{
			Method:        "PUT",
			RootURL:       url,
			ContentLength: &toSend,
			ContentRange:  fmt.Sprintf("bytes %d-%d/%d", start+skip, start+chunkSize-1, totalSize),
			Body:          chunk,
			Options:       options,
		}
		_, _ = chunk.Seek(skip, io.SeekStart)
		resp, err = o.fs.srv.Call(ctx, &opts)
		if err != nil && resp != nil && resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
			fs.Debugf(o, "Received 416 error - reading current position from server: %v", err)
			pos, posErr := o.getPosition(ctx, url)
			if posErr != nil {
				fs.Debugf(o, "Failed to read position: %v", posErr)
				return false, posErr
			}
			skip = pos - start
			fs.Debugf(o, "Read position %d, chunk is %d..%d, bytes to skip = %d", pos, start, start+chunkSize, skip)
			switch {
			case skip < 0:
				return false, errors.Wrapf(err, "sent block already (skip %d < 0), can't rewind", skip)
			case skip > chunkSize:
				return false, errors.Wrapf(err, "position is in the future (skip %d > chunkSize %d), can't skip forward", skip, chunkSize)
			case skip == chunkSize:
				fs.Debugf(o, "Skipping chunk as already sent (skip %d == chunkSize %d)", skip, chunkSize)
				return false, nil
			}
			return true, errors.Wrapf(err, "retry this chunk skipping %d bytes", skip)
		}
		if err != nil {
			return shouldRetry(ctx, resp, err)
		}
		body, err = rest.ReadBody(resp)
		if err != nil {
			return shouldRetry(ctx, resp, err)
		}
		if resp.StatusCode == 200 || resp.StatusCode == 201 {
			// we are done :)
			// read the item
			info = &api.Item{}
			return false, json.Unmarshal(body, info)
		}
		return false, nil
	})
	return info, err
}

// cancelUploadSession cancels an upload session
func (o *Object) cancelUploadSession(ctx context.Context, url string) (err error) {
	opts := rest.Opts{
		Method:     "DELETE",
		RootURL:    url,
		NoResponse: true,
	}
	var resp *http.Response
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.Call(ctx, &opts)
		return shouldRetry(ctx, resp, err)
	})
	return
}

// uploadMultipart uploads a file using multipart upload
func (o *Object) uploadMultipart(ctx context.Context, in io.Reader, size int64, modTime time.Time, options ...fs.OpenOption) (info *api.Item, err error) {
	if size <= 0 {
		return nil, errors.New("unknown-sized upload not supported")
	}

	// Create upload session
	fs.Debugf(o, "Starting multipart upload")
	session, err := o.createUploadSession(ctx, modTime)
	if err != nil {
		return nil, err
	}
	uploadURL := session.UploadURL

	// Cancel the session if something went wrong
	defer atexit.OnError(&err, func() {
		fs.Debugf(o, "Cancelling multipart upload: %v", err)
		cancelErr := o.cancelUploadSession(ctx, uploadURL)
		if cancelErr != nil {
			fs.Logf(o, "Failed to cancel multipart upload: %v (upload failed due to: %v)", cancelErr, err)
		}
	})()

	// Upload the chunks
	remaining := size
	position := int64(0)
	for remaining > 0 {
		n := int64(o.fs.opt.ChunkSize)
		if remaining < n {
			n = remaining
		}
		seg := readers.NewRepeatableReader(io.LimitReader(in, n))
		fs.Debugf(o, "Uploading segment %d/%d size %d", position, size, n)
		info, err = o.uploadFragment(ctx, uploadURL, position, size, seg, n, options...)
		if err != nil {
			return nil, err
		}
		remaining -= n
		position += n
	}

	return info, nil
}

// Update the content of a remote file within 4 MiB size in one single request
// This function will set modtime after uploading, which will create a new version for the remote file
func (o *Object) uploadSinglepart(ctx context.Context, in io.Reader, size int64, modTime time.Time, options ...fs.OpenOption) (info *api.Item, err error) {
	if size < 0 || size > int64(fs.SizeSuffix(4*1024*1024)) {
		return nil, errors.New("size passed into uploadSinglepart must be >= 0 and <= 4 MiB")
	}

	fs.Debugf(o, "Starting singlepart upload")
	var resp *http.Response
	opts := o.fs.newOptsCallWithPath(ctx, o.remote, "PUT", "/content")
	opts.ContentLength = &size
	opts.Body = in
	opts.Options = options

	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.CallJSON(ctx, &opts, nil, &info)
		if apiErr, ok := err.(*api.Error); ok {
			if apiErr.ErrorInfo.Code == "nameAlreadyExists" {
				// Make the error more user-friendly
				err = errors.New(err.Error() + " (is it a OneNote file?)")
			}
		}
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}

	err = o.setMetaData(info)
	if err != nil {
		return nil, err
	}
	// Set the mod time now and read metadata
	return o.setModTime(ctx, modTime)
}

// Update the object with the contents of the io.Reader, modTime and size
//
// The new object may have been created if an error is returned
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	if o.hasMetaData && o.isOneNoteFile {
		return errors.New("can't upload content to a OneNote file")
	}

	o.fs.tokenRenewer.Start()
	defer o.fs.tokenRenewer.Stop()

	size := src.Size()
	modTime := src.ModTime(ctx)

	var info *api.Item
	if size > 0 {
		info, err = o.uploadMultipart(ctx, in, size, modTime, options...)
	} else if size == 0 {
		info, err = o.uploadSinglepart(ctx, in, size, modTime, options...)
	} else {
		return errors.New("unknown-sized upload not supported")
	}
	if err != nil {
		return err
	}

	// If updating the file then remove versions
	if o.fs.opt.NoVersions && o.hasMetaData {
		err = o.deleteVersions(ctx)
		if err != nil {
			fs.Errorf(o, "Failed to remove versions: %v", err)
		}
	}

	return o.setMetaData(info)
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	return o.fs.deleteObject(ctx, o.id)
}

// MimeType of an Object if known, "" otherwise
func (o *Object) MimeType(ctx context.Context) string {
	return o.mimeType
}

// ID returns the ID of the Object if known, or "" if not
func (o *Object) ID() string {
	return o.id
}

/*
 *       URL Build routine area start
 *       1. In this area, region-related URL rewrites are applied. As the API is blackbox,
 *          we cannot thoroughly test this part. Please be extremely careful while changing them.
 *       2. If possible, please don't introduce region related code in other region, but patch these helper functions.
 *       3. To avoid region-related issues, please don't manually build rest.Opts from scratch.
 *          Instead, use these helper function, and customize the URL afterwards if needed.
 *
 *       currently, the 21ViaNet's API differs in the following places:
 *       - https://{Endpoint}/drives/{driveID}/items/{leaf}:/{route}
 *           - this API doesn't work (gives invalid request)
 *           - can be replaced with the following API:
 *               - https://{Endpoint}/drives/{driveID}/items/children('{leaf}')/{route}
 *                   - however, this API does NOT support multi-level leaf like a/b/c
 *               - https://{Endpoint}/drives/{driveID}/items/children('@a1')/{route}?@a1=URLEncode("'{leaf}'")
 *                   - this API does support multi-level leaf like a/b/c
 *       - https://{Endpoint}/drives/{driveID}/root/children('@a1')/{route}?@a1=URLEncode({path})
 *	         - Same as above
 */

// parseNormalizedID parses a normalized ID (may be in the form `driveID#itemID` or just `itemID`)
// and returns itemID, driveID, rootURL.
// Such a normalized ID can come from (*Item).GetID()
func (f *Fs) parseNormalizedID(ID string) (string, string, string) {
	rootURL := graphAPIEndpoint[f.opt.Region] + "/v1.0/drives"
	if strings.Index(ID, "#") >= 0 {
		s := strings.Split(ID, "#")
		return s[1], s[0], rootURL
	}
	return ID, "", ""
}

// newOptsCall build the rest.Opts structure with *a normalizedID(driveID#fileID, or simply fileID)*
// using url template https://{Endpoint}/drives/{driveID}/items/{itemID}/{route}
func (f *Fs) newOptsCall(normalizedID string, method string, route string) (opts rest.Opts) {
	id, drive, rootURL := f.parseNormalizedID(normalizedID)

	if drive != "" {
		return rest.Opts{
			Method:  method,
			RootURL: rootURL,
			Path:    "/" + drive + "/items/" + id + route,
		}
	}
	return rest.Opts{
		Method: method,
		Path:   "/items/" + id + route,
	}
}

func escapeSingleQuote(str string) string {
	return strings.ReplaceAll(str, "'", "''")
}

// newOptsCallWithIDPath build the rest.Opts structure with *a normalizedID (driveID#fileID, or simply fileID) and leaf*
// using url template https://{Endpoint}/drives/{driveID}/items/{leaf}:/{route} (for international OneDrive)
// or https://{Endpoint}/drives/{driveID}/items/children('{leaf}')/{route}
// and https://{Endpoint}/drives/{driveID}/items/children('@a1')/{route}?@a1=URLEncode("'{leaf}'") (for 21ViaNet)
// if isPath is false, this function will only work when the leaf is "" or a child name (i.e. it doesn't accept multi-level leaf)
// if isPath is true, multi-level leaf like a/b/c can be passed
func (f *Fs) newOptsCallWithIDPath(normalizedID string, leaf string, isPath bool, method string, route string) (opts rest.Opts, ok bool) {
	encoder := f.opt.Enc.FromStandardName
	if isPath {
		encoder = f.opt.Enc.FromStandardPath
	}
	trueDirID, drive, rootURL := f.parseNormalizedID(normalizedID)
	if drive == "" {
		trueDirID = normalizedID
	}
	entity := "/items/" + trueDirID + ":/" + withTrailingColon(rest.URLPathEscape(encoder(leaf))) + route
	if f.opt.Region == regionCN {
		if isPath {
			entity = "/items/" + trueDirID + "/children('@a1')" + route + "?@a1=" + url.QueryEscape("'"+encoder(escapeSingleQuote(leaf))+"'")
		} else {
			entity = "/items/" + trueDirID + "/children('" + rest.URLPathEscape(encoder(escapeSingleQuote(leaf))) + "')" + route
		}
	}
	if drive == "" {
		ok = false
		opts = rest.Opts{
			Method: method,
			Path:   entity,
		}
		return
	}
	ok = true
	opts = rest.Opts{
		Method:  method,
		RootURL: rootURL,
		Path:    "/" + drive + entity,
	}
	return
}

// newOptsCallWithIDPath build the rest.Opts structure with an *absolute path start from root*
// using url template https://{Endpoint}/drives/{driveID}/root:/{path}:/{route}
// or https://{Endpoint}/drives/{driveID}/root/children('@a1')/{route}?@a1=URLEncode({path})
func (f *Fs) newOptsCallWithRootPath(path string, method string, route string) (opts rest.Opts) {
	path = strings.TrimSuffix(path, "/")
	newURL := "/root:/" + withTrailingColon(rest.URLPathEscape(f.opt.Enc.FromStandardPath(path))) + route
	if f.opt.Region == regionCN {
		newURL = "/root/children('@a1')" + route + "?@a1=" + url.QueryEscape("'"+escapeSingleQuote(f.opt.Enc.FromStandardPath(path))+"'")
	}
	return rest.Opts{
		Method: method,
		Path:   newURL,
	}
}

// newOptsCallWithPath build the rest.Opt intelligently.
// It will first try to resolve the path using dircache, which enables support for "Share with me" files.
// If present in cache, then use ID + Path variant, else fallback into RootPath variant
func (f *Fs) newOptsCallWithPath(ctx context.Context, path string, method string, route string) (opts rest.Opts) {
	if path == "" {
		url := "/root" + route
		return rest.Opts{
			Method: method,
			Path:   url,
		}
	}

	// find dircache
	leaf, directoryID, _ := f.dirCache.FindPath(ctx, path, false)
	// try to use IDPath variant first
	if opts, ok := f.newOptsCallWithIDPath(directoryID, leaf, false, method, route); ok {
		return opts
	}
	// fallback to use RootPath variant first
	return f.newOptsCallWithRootPath(path, method, route)
}

/*
 *       URL Build routine area end
 */

// Returns the canonical form of the driveID
func (f *Fs) canonicalDriveID(driveID string) (canonicalDriveID string) {
	if driveID == "" {
		canonicalDriveID = f.opt.DriveID
	} else {
		canonicalDriveID = driveID
	}
	canonicalDriveID = strings.ToLower(canonicalDriveID)
	return canonicalDriveID
}

// getRelativePathInsideBase checks if `target` is inside `base`. If so, it
// returns a relative path for `target` based on `base` and a boolean `true`.
// Otherwise returns "", false.
func getRelativePathInsideBase(base, target string) (string, bool) {
	if base == "" {
		return target, true
	}

	baseSlash := base + "/"
	if strings.HasPrefix(target+"/", baseSlash) {
		return target[len(baseSlash):], true
	}
	return "", false
}

// Adds a ":" at the end of `remotePath` in a proper manner.
// If `remotePath` already ends with "/", change it to ":/"
// If `remotePath` is "", return "".
// A workaround for #2720 and #3039
func withTrailingColon(remotePath string) string {
	if remotePath == "" {
		return ""
	}

	if strings.HasSuffix(remotePath, "/") {
		return remotePath[:len(remotePath)-1] + ":/"
	}
	return remotePath + ":"
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = (*Fs)(nil)
	_ fs.Purger          = (*Fs)(nil)
	_ fs.Copier          = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ fs.DirCacheFlusher = (*Fs)(nil)
	_ fs.Abouter         = (*Fs)(nil)
	_ fs.PublicLinker    = (*Fs)(nil)
	_ fs.CleanUpper      = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.MimeTyper       = &Object{}
	_ fs.IDer            = &Object{}
)
