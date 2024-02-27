// Package onedrive provides an interface to the Microsoft OneDrive
// object storage system.
package onedrive

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
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

	"github.com/rclone/rclone/backend/onedrive/api"
	"github.com/rclone/rclone/backend/onedrive/quickxorhash"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
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

	scopeAccess             = fs.SpaceSepList{"Files.Read", "Files.ReadWrite", "Files.Read.All", "Files.ReadWrite.All", "Sites.Read.All", "offline_access"}
	scopeAccessWithoutSites = fs.SpaceSepList{"Files.Read", "Files.ReadWrite", "Files.Read.All", "Files.ReadWrite.All", "offline_access"}

	// Description of how to auth for this app for a business account
	oauthConfig = &oauth2.Config{
		Scopes:       scopeAccess,
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
	QuickXorHashType = hash.RegisterHash("quickxor", "QuickXorHash", 40, quickxorhash.New)
	fs.Register(&fs.RegInfo{
		Name:        "onedrive",
		Description: "Microsoft OneDrive",
		NewFs:       NewFs,
		Config:      Config,
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
					Help:  "Azure and Office 365 operated by Vnet Group in China",
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
			Name:      "drive_id",
			Help:      "The ID of the drive to use.",
			Default:   "",
			Advanced:  true,
			Sensitive: true,
		}, {
			Name:     "drive_type",
			Help:     "The type of the drive (" + driveTypePersonal + " | " + driveTypeBusiness + " | " + driveTypeSharepoint + ").",
			Default:  "",
			Advanced: true,
		}, {
			Name: "root_folder_id",
			Help: `ID of the root folder.

This isn't normally needed, but in special circumstances you might
know the folder ID that you wish to access but not be able to get
there through a path traversal.
`,
			Advanced:  true,
			Sensitive: true,
		}, {
			Name: "access_scopes",
			Help: `Set scopes to be requested by rclone.

Choose or manually enter a custom space separated list with all scopes, that rclone should request.
`,
			Default:  scopeAccess,
			Advanced: true,
			Examples: []fs.OptionExample{
				{
					Value: "Files.Read Files.ReadWrite Files.Read.All Files.ReadWrite.All Sites.Read.All offline_access",
					Help:  "Read and write access to all resources",
				},
				{
					Value: "Files.Read Files.Read.All Sites.Read.All offline_access",
					Help:  "Read only access to all resources",
				},
				{
					Value: "Files.Read Files.ReadWrite Files.Read.All Files.ReadWrite.All offline_access",
					Help:  "Read and write access to all resources, without the ability to browse SharePoint sites. \nSame as if disable_site_permission was set to true",
				},
			}}, {
			Name: "disable_site_permission",
			Help: `Disable the request for Sites.Read.All permission.

If set to true, you will no longer be able to search for a SharePoint site when
configuring drive ID, because rclone will not request Sites.Read.All permission.
Set it to true if your organization didn't assign Sites.Read.All permission to the
application, and your organization disallows users to consent app permission
request on their own.`,
			Default:  false,
			Advanced: true,
			Hide:     fs.OptionHideBoth,
		}, {
			Name: "expose_onenote_files",
			Help: `Set to make OneNote files show up in directory listings.

By default, rclone will hide OneNote files in directory listings because
operations like "Open" and "Update" won't work on them.  But this
behaviour may also prevent you from deleting them.  If you want to
delete OneNote files or otherwise want them to show up in directory
listing, set this option.`,
			Default:  false,
			Advanced: true,
		}, {
			Name:    "server_side_across_configs",
			Default: false,
			Help: `Deprecated: use --server-side-across-configs instead.

Allow server-side operations (e.g. copy) to work across different onedrive configs.

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
			Help: `Remove all versions on modifying operations.

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
				Help:  "Anyone with the link has access, without needing to sign in.\nThis may include people outside of your organization.\nAnonymous link support may be disabled by an administrator.",
			}, {
				Value: "organization",
				Help:  "Anyone signed into your organization (tenant) can use the link to get access.\nOnly available in OneDrive for Business and SharePoint.",
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
			Advanced:  true,
			Sensitive: true,
		}, {
			Name:    "hash_type",
			Default: "auto",
			Help: `Specify the hash in use for the backend.

This specifies the hash type in use. If set to "auto" it will use the
default hash which is QuickXorHash.

Before rclone 1.62 an SHA1 hash was used by default for Onedrive
Personal. For 1.62 and later the default is to use a QuickXorHash for
all onedrive types. If an SHA1 hash is desired then set this option
accordingly.

From July 2023 QuickXorHash will be the only available hash for
both OneDrive for Business and OneDriver Personal.

This can be set to "none" to not use any hashes.

If the hash requested does not exist on the object, it will be
returned as an empty string which is treated as a missing hash by
rclone.
`,
			Examples: []fs.OptionExample{{
				Value: "auto",
				Help:  "Rclone chooses the best hash",
			}, {
				Value: "quickxor",
				Help:  "QuickXor",
			}, {
				Value: "sha1",
				Help:  "SHA1",
			}, {
				Value: "sha256",
				Help:  "SHA256",
			}, {
				Value: "crc32",
				Help:  "CRC32",
			}, {
				Value: "none",
				Help:  "None - don't use any hashes",
			}},
			Advanced: true,
		}, {
			Name:    "av_override",
			Default: false,
			Help: `Allows download of files the server thinks has a virus.

The onedrive/sharepoint server may check files uploaded with an Anti
Virus checker. If it detects any potential viruses or malware it will
block download of the file.

In this case you will see a message like this

    server reports this file is infected with a virus - use --onedrive-av-override to download anyway: Infected (name of virus): 403 Forbidden: 

If you are 100% sure you want to download this file anyway then use
the --onedrive-av-override flag, or av_override = true in the config
file.
`,
			Advanced: true,
		}, {
			Name:    "delta",
			Default: false,
			Help: strings.ReplaceAll(`If set rclone will use delta listing to implement recursive listings.

If this flag is set the the onedrive backend will advertise |ListR|
support for recursive listings.

Setting this flag speeds up these things greatly:

    rclone lsf -R onedrive:
    rclone size onedrive:
    rclone rc vfs/refresh recursive=true

**However** the delta listing API **only** works at the root of the
drive. If you use it not at the root then it recurses from the root
and discards all the data that is not under the directory you asked
for. So it will be correct but may not be very efficient.

This is why this flag is not set as the default.

As a rule of thumb if nearly all of your data is under rclone's root
directory (the |root/directory| in |onedrive:root/directory|) then
using this flag will be be a big performance win. If your data is
mostly not under the root then using this flag will be a big
performance loss.

It is recommended if you are mounting your onedrive at the root
(or near the root when using crypt) and using rclone |rc vfs/refresh|.
`, "|", "`"),
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

// Get the region and graphURL from the config
func getRegionURL(m configmap.Mapper) (region, graphURL string) {
	region, _ = m.Get("region")
	graphURL = graphAPIEndpoint[region] + "/v1.0"
	return region, graphURL
}

// Config for chooseDrive
type chooseDriveOpt struct {
	opts         rest.Opts
	finalDriveID string
	siteID       string
	relativePath string
}

// chooseDrive returns a query to choose which drive the user is interested in
func chooseDrive(ctx context.Context, name string, m configmap.Mapper, srv *rest.Client, opt chooseDriveOpt) (*fs.ConfigOut, error) {
	_, graphURL := getRegionURL(m)

	// if we use server-relative URL for finding the drive
	if opt.relativePath != "" {
		opt.opts = rest.Opts{
			Method:  "GET",
			RootURL: graphURL,
			Path:    "/sites/root:" + opt.relativePath,
		}
		site := api.SiteResource{}
		_, err := srv.CallJSON(ctx, &opt.opts, nil, &site)
		if err != nil {
			return fs.ConfigError("choose_type", fmt.Sprintf("Failed to query available site by relative path: %v", err))
		}
		opt.siteID = site.SiteID
	}

	// if we have a siteID we need to ask for the drives
	if opt.siteID != "" {
		opt.opts = rest.Opts{
			Method:  "GET",
			RootURL: graphURL,
			Path:    "/sites/" + opt.siteID + "/drives",
		}
	}

	drives := api.DrivesResponse{}

	// We don't have the final ID yet?
	// query Microsoft Graph
	if opt.finalDriveID == "" {
		_, err := srv.CallJSON(ctx, &opt.opts, nil, &drives)
		if err != nil {
			return fs.ConfigError("choose_type", fmt.Sprintf("Failed to query available drives: %v", err))
		}

		// Also call /me/drive as sometimes /me/drives doesn't return it #4068
		if opt.opts.Path == "/me/drives" {
			opt.opts.Path = "/me/drive"
			meDrive := api.DriveResource{}
			_, err := srv.CallJSON(ctx, &opt.opts, nil, &meDrive)
			if err != nil {
				return fs.ConfigError("choose_type", fmt.Sprintf("Failed to query available drives: %v", err))
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
	} else {
		drives.Drives = append(drives.Drives, api.DriveResource{
			DriveID:   opt.finalDriveID,
			DriveName: "Chosen Drive ID",
			DriveType: "drive",
		})
	}
	if len(drives.Drives) == 0 {
		return fs.ConfigError("choose_type", "No drives found")
	}
	return fs.ConfigChoose("driveid_final", "config_driveid", "Select drive you want to use", len(drives.Drives), func(i int) (string, string) {
		drive := drives.Drives[i]
		return drive.DriveID, fmt.Sprintf("%s (%s)", drive.DriveName, drive.DriveType)
	})
}

// Config the backend
func Config(ctx context.Context, name string, m configmap.Mapper, config fs.ConfigIn) (*fs.ConfigOut, error) {
	region, graphURL := getRegionURL(m)

	if config.State == "" {
		var accessScopes fs.SpaceSepList
		accessScopesString, _ := m.Get("access_scopes")
		err := accessScopes.Set(accessScopesString)
		if err != nil {
			return nil, fmt.Errorf("failed to parse access_scopes: %w", err)
		}
		oauthConfig.Scopes = []string(accessScopes)
		disableSitePermission, _ := m.Get("disable_site_permission")
		if disableSitePermission == "true" {
			oauthConfig.Scopes = scopeAccessWithoutSites
		}
		oauthConfig.Endpoint = oauth2.Endpoint{
			AuthURL:  authEndpoint[region] + authPath,
			TokenURL: authEndpoint[region] + tokenPath,
		}
		return oauthutil.ConfigOut("choose_type", &oauthutil.Options{
			OAuth2Config: oauthConfig,
		})
	}

	oAuthClient, _, err := oauthutil.NewClient(ctx, name, m, oauthConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to configure OneDrive: %w", err)
	}
	srv := rest.NewClient(oAuthClient)

	switch config.State {
	case "choose_type":
		return fs.ConfigChooseExclusiveFixed("choose_type_done", "config_type", "Type of connection", []fs.OptionExample{{
			Value: "onedrive",
			Help:  "OneDrive Personal or Business",
		}, {
			Value: "sharepoint",
			Help:  "Root Sharepoint site",
		}, {
			Value: "url",
			Help:  "Sharepoint site name or URL\nE.g. mysite or https://contoso.sharepoint.com/sites/mysite",
		}, {
			Value: "search",
			Help:  "Search for a Sharepoint site",
		}, {
			Value: "driveid",
			Help:  "Type in driveID (advanced)",
		}, {
			Value: "siteid",
			Help:  "Type in SiteID (advanced)",
		}, {
			Value: "path",
			Help:  "Sharepoint server-relative path (advanced)\nE.g. /teams/hr",
		}})
	case "choose_type_done":
		// Jump to next state according to config chosen
		return fs.ConfigGoto(config.Result)
	case "onedrive":
		return chooseDrive(ctx, name, m, srv, chooseDriveOpt{
			opts: rest.Opts{
				Method:  "GET",
				RootURL: graphURL,
				Path:    "/me/drives",
			},
		})
	case "sharepoint":
		return chooseDrive(ctx, name, m, srv, chooseDriveOpt{
			opts: rest.Opts{
				Method:  "GET",
				RootURL: graphURL,
				Path:    "/sites/root/drives",
			},
		})
	case "driveid":
		return fs.ConfigInput("driveid_end", "config_driveid_fixed", "Drive ID")
	case "driveid_end":
		return chooseDrive(ctx, name, m, srv, chooseDriveOpt{
			finalDriveID: config.Result,
		})
	case "siteid":
		return fs.ConfigInput("siteid_end", "config_siteid", "Site ID")
	case "siteid_end":
		return chooseDrive(ctx, name, m, srv, chooseDriveOpt{
			siteID: config.Result,
		})
	case "url":
		return fs.ConfigInput("url_end", "config_site_url", `Site URL

Examples:
- "mysite"
- "https://XXX.sharepoint.com/sites/mysite"
- "https://XXX.sharepoint.com/teams/ID"
`)
	case "url_end":
		siteURL := config.Result
		re := regexp.MustCompile(`https://.*\.sharepoint\.com(/.*)`)
		match := re.FindStringSubmatch(siteURL)
		if len(match) == 2 {
			return chooseDrive(ctx, name, m, srv, chooseDriveOpt{
				relativePath: match[1],
			})
		}
		return chooseDrive(ctx, name, m, srv, chooseDriveOpt{
			relativePath: "/sites/" + siteURL,
		})
	case "path":
		return fs.ConfigInput("path_end", "config_sharepoint_url", `Server-relative URL`)
	case "path_end":
		return chooseDrive(ctx, name, m, srv, chooseDriveOpt{
			relativePath: config.Result,
		})
	case "search":
		return fs.ConfigInput("search_end", "config_search_term", `Search term`)
	case "search_end":
		searchTerm := config.Result
		opts := rest.Opts{
			Method:  "GET",
			RootURL: graphURL,
			Path:    "/sites?search=" + searchTerm,
		}

		sites := api.SiteResponse{}
		_, err := srv.CallJSON(ctx, &opts, nil, &sites)
		if err != nil {
			return fs.ConfigError("choose_type", fmt.Sprintf("Failed to query available sites: %v", err))
		}

		if len(sites.Sites) == 0 {
			return fs.ConfigError("choose_type", fmt.Sprintf("search for %q returned no results", searchTerm))
		}
		return fs.ConfigChoose("search_sites", "config_site", `Select the Site you want to use`, len(sites.Sites), func(i int) (string, string) {
			site := sites.Sites[i]
			return site.SiteID, fmt.Sprintf("%s (%s)", site.SiteName, site.SiteURL)
		})
	case "search_sites":
		return chooseDrive(ctx, name, m, srv, chooseDriveOpt{
			siteID: config.Result,
		})
	case "driveid_final":
		finalDriveID := config.Result

		// Test the driveID and get drive type
		opts := rest.Opts{
			Method:  "GET",
			RootURL: graphURL,
			Path:    "/drives/" + finalDriveID + "/root"}
		var rootItem api.Item
		_, err = srv.CallJSON(ctx, &opts, nil, &rootItem)
		if err != nil {
			return fs.ConfigError("choose_type", fmt.Sprintf("Failed to query root for drive %q: %v", finalDriveID, err))
		}

		m.Set(configDriveID, finalDriveID)
		m.Set(configDriveType, rootItem.ParentReference.DriveType)

		return fs.ConfigConfirm("driveid_final_end", true, "config_drive_ok", fmt.Sprintf("Drive OK?\n\nFound drive %q of type %q\nURL: %s\n", rootItem.Name, rootItem.ParentReference.DriveType, rootItem.WebURL))
	case "driveid_final_end":
		if config.Result == "true" {
			return nil, nil
		}
		return fs.ConfigGoto("choose_type")
	}
	return nil, fmt.Errorf("unknown state %q", config.State)
}

// Options defines the configuration for this backend
type Options struct {
	Region                  string               `config:"region"`
	ChunkSize               fs.SizeSuffix        `config:"chunk_size"`
	DriveID                 string               `config:"drive_id"`
	DriveType               string               `config:"drive_type"`
	RootFolderID            string               `config:"root_folder_id"`
	DisableSitePermission   bool                 `config:"disable_site_permission"`
	AccessScopes            fs.SpaceSepList      `config:"access_scopes"`
	ExposeOneNoteFiles      bool                 `config:"expose_onenote_files"`
	ServerSideAcrossConfigs bool                 `config:"server_side_across_configs"`
	ListChunk               int64                `config:"list_chunk"`
	NoVersions              bool                 `config:"no_versions"`
	LinkScope               string               `config:"link_scope"`
	LinkType                string               `config:"link_type"`
	LinkPassword            string               `config:"link_password"`
	HashType                string               `config:"hash_type"`
	AVOverride              bool                 `config:"av_override"`
	Delta                   bool                 `config:"delta"`
	Enc                     encoder.MultiEncoder `config:"encoding"`
}

// Fs represents a remote OneDrive
type Fs struct {
	name         string             // name of this remote
	root         string             // the path we are working on
	opt          Options            // parsed options
	ci           *fs.ConfigInfo     // global config
	features     *fs.Features       // optional features
	srv          *rest.Client       // the connection to the OneDrive server
	unAuth       *rest.Client       // no authentication connection to the OneDrive server
	dirCache     *dircache.DirCache // Map of directory path to directory id
	pacer        *fs.Pacer          // pacer for API calls
	tokenRenewer *oauthutil.Renew   // renew the token on expiry
	driveID      string             // ID to use for querying Microsoft Graph
	driveType    string             // https://developer.microsoft.com/en-us/graph/docs/api-reference/v1.0/resources/drive
	hashType     hash.Type          // type of the hash we are using
}

// Object describes a OneDrive object
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
	hash          string    // Hash of the content, usually QuickXorHash but set as hash_type
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
	return fmt.Sprintf("OneDrive root '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// parsePath parses a OneDrive 'url'
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
		case 400:
			if apiErr, ok := err.(*api.Error); ok {
				if apiErr.ErrorInfo.InnerError.Code == "pathIsTooLong" {
					return false, fserrors.NoRetryError(err)
				}
			}
		case 401:
			if len(resp.Header["Www-Authenticate"]) == 1 && strings.Contains(resp.Header["Www-Authenticate"][0], "expired_token") {
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
// If `relPath` == ”, do not append the slash (See #3664)
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
		opts := f.newOptsCallWithPath(ctx, path, "GET", "")
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
		return fmt.Errorf("%s is not a multiple of %s", cs, chunkSizeMultiple)
	}
	if cs < minChunkSize {
		return fmt.Errorf("%s is less than %s", cs, minChunkSize)
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
		return nil, fmt.Errorf("onedrive: chunk size: %w", err)
	}

	if opt.DriveID == "" || opt.DriveType == "" {
		return nil, errors.New("unable to get drive_id and drive_type - if you are upgrading from older versions of rclone, please run `rclone config` and re-configure this backend")
	}

	rootURL := graphAPIEndpoint[opt.Region] + "/v1.0" + "/drives/" + opt.DriveID
	oauthConfig.Scopes = opt.AccessScopes
	if opt.DisableSitePermission {
		oauthConfig.Scopes = scopeAccessWithoutSites
	}
	oauthConfig.Endpoint = oauth2.Endpoint{
		AuthURL:  authEndpoint[opt.Region] + authPath,
		TokenURL: authEndpoint[opt.Region] + tokenPath,
	}

	client := fshttp.NewClient(ctx)
	root = parsePath(root)
	oAuthClient, ts, err := oauthutil.NewClientWithBaseClient(ctx, name, m, oauthConfig, client)
	if err != nil {
		return nil, fmt.Errorf("failed to configure OneDrive: %w", err)
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
		unAuth:    rest.NewClient(client).SetRoot(rootURL),
		pacer:     fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
		hashType:  QuickXorHashType,
	}
	f.features = (&fs.Features{
		CaseInsensitive:         true,
		ReadMimeType:            true,
		CanHaveEmptyDirectories: true,
		ServerSideAcrossConfigs: opt.ServerSideAcrossConfigs,
	}).Fill(ctx, f)
	f.srv.SetErrorHandler(errorHandler)

	// Set the user defined hash
	if opt.HashType == "auto" || opt.HashType == "" {
		opt.HashType = QuickXorHashType.String()
	}
	err = f.hashType.Set(opt.HashType)
	if err != nil {
		return nil, err
	}

	// Disable change polling in China region
	// See: https://github.com/rclone/rclone/issues/6444
	if f.opt.Region == regionCN {
		f.features.ChangeNotify = nil
	}

	// Renew the token in the background
	f.tokenRenewer = oauthutil.NewRenew(f.String(), ts, func() error {
		_, _, err := f.readMetaDataForPath(ctx, "")
		return err
	})

	// Get rootID
	var rootID = opt.RootFolderID
	if rootID == "" {
		rootInfo, _, err := f.readMetaDataForPath(ctx, "")
		if err != nil {
			return nil, fmt.Errorf("failed to get root: %w", err)
		}
		rootID = rootInfo.GetID()
	}
	if rootID == "" {
		return nil, errors.New("failed to get root: ID was empty")
	}

	f.dirCache = dircache.New(root, rootID, f)

	// ListR only supported if delta set
	if !f.opt.Delta {
		f.features.ListR = nil
	}

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
// If an error is returned then processing stops
type listAllFn func(*api.Item) error

// Lists the directory required calling the user function on each item found
//
// If the user fn ever returns true then it early exits with found = true
//
// This listing function works on both normal listings and delta listings
func (f *Fs) _listAll(ctx context.Context, dirID string, directoriesOnly bool, filesOnly bool, fn listAllFn, opts *rest.Opts, result any, pValue *[]api.Item, pNextLink *string) (err error) {
	for {
		var resp *http.Response
		err = f.pacer.Call(func() (bool, error) {
			resp, err = f.srv.CallJSON(ctx, opts, nil, result)
			return shouldRetry(ctx, resp, err)
		})
		if err != nil {
			return fmt.Errorf("couldn't list files: %w", err)
		}
		if len(*pValue) == 0 {
			break
		}
		for i := range *pValue {
			item := &(*pValue)[i]
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
			err = fn(item)
			if err != nil {
				return err
			}
		}
		if *pNextLink == "" {
			break
		}
		opts.Path = ""
		opts.Parameters = nil
		opts.RootURL = *pNextLink
		// reset results
		*pNextLink = ""
		*pValue = nil
	}
	return nil
}

// Lists the directory required calling the user function on each item found
//
// If the user fn ever returns true then it early exits with found = true
func (f *Fs) listAll(ctx context.Context, dirID string, directoriesOnly bool, filesOnly bool, fn listAllFn) (err error) {
	// Top parameter asks for bigger pages of data
	// https://dev.onedrive.com/odata/optional-query-parameters.htm
	opts := f.newOptsCall(dirID, "GET", fmt.Sprintf("/children?$top=%d", f.opt.ListChunk))
	var result api.ListChildrenResponse
	return f._listAll(ctx, dirID, directoriesOnly, filesOnly, fn, &opts, &result, &result.Value, &result.NextLink)
}

// Convert a list item into a DirEntry
//
// Can return nil for an item which should be skipped
func (f *Fs) itemToDirEntry(ctx context.Context, dir string, info *api.Item) (entry fs.DirEntry, err error) {
	if !f.opt.ExposeOneNoteFiles && info.GetPackageType() == api.PackageTypeOneNote {
		fs.Debugf(info.Name, "OneNote file not shown in directory listing")
		return nil, nil
	}
	remote := path.Join(dir, info.GetName())
	folder := info.GetFolder()
	if folder != nil {
		// cache the directory ID for later lookups
		id := info.GetID()
		f.dirCache.Put(remote, id)
		d := fs.NewDir(remote, time.Time(info.GetLastModifiedDateTime())).SetID(id)
		d.SetItems(folder.ChildCount)
		entry = d
	} else {
		o, err := f.newObjectWithInfo(ctx, remote, info)
		if err != nil {
			return nil, err
		}
		entry = o
	}
	return entry, nil
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
	err = f.listAll(ctx, directoryID, false, false, func(info *api.Item) error {
		entry, err := f.itemToDirEntry(ctx, dir, info)
		if err != nil {
			return err
		}
		if entry == nil {
			return nil
		}
		entries = append(entries, entry)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return entries, nil
}

// ListR lists the objects and directories of the Fs starting
// from dir recursively into out.
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
//
// Don't implement this unless you have a more efficient way
// of listing recursively than doing a directory traversal.
func (f *Fs) ListR(ctx context.Context, dir string, callback fs.ListRCallback) (err error) {
	// Make sure this ID is in the directory cache
	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return err
	}

	// ListR only works at the root of a onedrive, not on a folder
	// So we have to filter things outside of the root which is
	// inefficient.

	list := walk.NewListRHelper(callback)

	// list a folder conventionally - used for shared folders
	var listFolder func(dir string) error
	listFolder = func(dir string) error {
		entries, err := f.List(ctx, dir)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			err = list.Add(entry)
			if err != nil {
				return err
			}
			if _, isDir := entry.(fs.Directory); isDir {
				err = listFolder(entry.Remote())
				if err != nil {
					return err
				}
			}
		}
		return nil
	}

	// This code relies on the fact that directories are sent before their children. This isn't
	// mentioned in the docs though, so maybe it shouldn't be relied on.
	seen := map[string]struct{}{}
	fn := func(info *api.Item) error {
		var parentPath string
		var ok bool
		id := info.GetID()
		// The API can produce duplicates, so skip them
		if _, found := seen[id]; found {
			return nil
		}
		seen[id] = struct{}{}
		// Skip the root directory
		if id == directoryID {
			return nil
		}
		// Skip deleted items
		if info.Deleted != nil {
			return nil
		}
		dirID := info.GetParentReference().GetID()
		// Skip files that don't have their parent directory
		// cached as they are outside the root.
		parentPath, ok = f.dirCache.GetInv(dirID)
		if !ok {
			return nil
		}
		// Skip files not under the root directory
		remote := path.Join(parentPath, info.GetName())
		if dir != "" && !strings.HasPrefix(remote, dir+"/") {
			return nil
		}
		entry, err := f.itemToDirEntry(ctx, parentPath, info)
		if err != nil {
			return err
		}
		if entry == nil {
			return nil
		}
		err = list.Add(entry)
		if err != nil {
			return err
		}
		// If this is a shared folder, we'll need list it too
		if info.RemoteItem != nil && info.RemoteItem.Folder != nil {
			fs.Debugf(remote, "Listing shared directory")
			return listFolder(remote)
		}
		return nil
	}

	opts := rest.Opts{
		Method: "GET",
		Path:   "/root/delta",
		Parameters: map[string][]string{
			// "token": {token},
			"$top": {fmt.Sprintf("%d", f.opt.ListChunk)},
		},
	}

	var result api.DeltaResponse
	err = f._listAll(ctx, "", false, false, fn, &opts, &result, &result.Value, &result.NextLink)
	if err != nil {
		return err
	}

	return list.Flush()

}

// Shutdown shutdown the fs
func (f *Fs) Shutdown(ctx context.Context) error {
	f.tokenRenewer.Shutdown()
	return nil
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
		err := f.listAll(ctx, rootID, false, false, func(item *api.Item) error {
			return fs.ErrorDirectoryNotEmpty
		})
		if err != nil {
			return err
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
			return fmt.Errorf("async status result not JSON: %q: %w", body, err)
		}

		switch status.Status {
		case "failed":
			if strings.HasPrefix(status.ErrorCode, "AccessDenied_") {
				return errAsyncJobAccessDenied
			}
			fallthrough
		case "deleteFailed":
			return fmt.Errorf("%s: async operation returned %q", o.remote, status.Status)
		case "completed":
			err = o.readMetaData(ctx)
			if err != nil {
				return fmt.Errorf("async operation completed but readMetaData failed: %w", err)
			}
			return nil
		}

		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("async operation didn't complete after %v", f.ci.TimeoutOrInfinite())
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
		if strings.EqualFold(srcPath, dstPath) {
			return nil, fmt.Errorf("can't copy %q -> %q as are same name when lowercase", srcPath, dstPath)
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
		return nil, err
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
	return hash.Set(f.hashType)
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
		if resp != nil && resp.StatusCode == 400 && f.driveType != driveTypePersonal {
			return "", fmt.Errorf("%v (is making public links permitted by the org admin?)", err)
		}
		return "", err
	}

	shareURL := result.Link.WebURL

	// Convert share link to direct download link if target is not a folder
	// Not attempting to do the conversion for regional versions, just to be safe
	if f.opt.Region != regionGlobal {
		return shareURL, nil
	}
	if info.Folder != nil {
		fs.Debugf(nil, "Can't convert share link for folder to direct link - returning the link as is")
		return shareURL, nil
	}

	cnvFailMsg := "Don't know how to convert share link to direct link - returning the link as is"
	directURL := ""
	segments := strings.Split(shareURL, "/")
	switch f.driveType {
	case driveTypePersonal:
		// Method: https://stackoverflow.com/questions/37951114/direct-download-link-to-onedrive-file
		if len(segments) != 5 {
			fs.Logf(f, cnvFailMsg)
			return shareURL, nil
		}
		enc := base64.StdEncoding.EncodeToString([]byte(shareURL))
		enc = strings.ReplaceAll(enc, "/", "_")
		enc = strings.ReplaceAll(enc, "+", "-")
		enc = strings.ReplaceAll(enc, "=", "")
		directURL = fmt.Sprintf("https://api.onedrive.com/v1.0/shares/u!%s/root/content", enc)
	case driveTypeBusiness:
		// Method: https://docs.microsoft.com/en-us/sharepoint/dev/spfx/shorter-share-link-format
		// Example:
		//   https://{tenant}-my.sharepoint.com/:t:/g/personal/{user_email}/{Opaque_String}
		//   --convert to->
		//   https://{tenant}-my.sharepoint.com/personal/{user_email}/_layouts/15/download.aspx?share={Opaque_String}
		if len(segments) != 8 {
			fs.Logf(f, cnvFailMsg)
			return shareURL, nil
		}
		directURL = fmt.Sprintf("https://%s/%s/%s/_layouts/15/download.aspx?share=%s",
			segments[2], segments[5], segments[6], segments[7])
	case driveTypeSharepoint:
		// Method: Similar to driveTypeBusiness
		// Example:
		//   https://{tenant}.sharepoint.com/:t:/s/{site_name}/{Opaque_String}
		//   --convert to->
		//   https://{tenant}.sharepoint.com/sites/{site_name}/_layouts/15/download.aspx?share={Opaque_String}
		//
		//   https://{tenant}.sharepoint.com/:t:/t/{team_name}/{Opaque_String}
		//   --convert to->
		//   https://{tenant}.sharepoint.com/teams/{team_name}/_layouts/15/download.aspx?share={Opaque_String}
		//
		//   https://{tenant}.sharepoint.com/:t:/g/{Opaque_String}
		//   --convert to->
		//   https://{tenant}.sharepoint.com/_layouts/15/download.aspx?share={Opaque_String}
		if len(segments) < 6 || len(segments) > 7 {
			fs.Logf(f, cnvFailMsg)
			return shareURL, nil
		}
		pathPrefix := ""
		switch segments[4] {
		case "s": // Site
			pathPrefix = "/sites/" + segments[5]
		case "t": // Team
			pathPrefix = "/teams/" + segments[5]
		case "g": // Root site
		default:
			fs.Logf(f, cnvFailMsg)
			return shareURL, nil
		}
		directURL = fmt.Sprintf("https://%s%s/_layouts/15/download.aspx?share=%s",
			segments[2], pathPrefix, segments[len(segments)-1])
	}

	return directURL, nil
}

// CleanUp deletes all the hidden files.
func (f *Fs) CleanUp(ctx context.Context) error {
	token := make(chan struct{}, f.ci.Checkers)
	var wg sync.WaitGroup
	err := walk.Walk(ctx, f, "", true, -1, func(path string, entries fs.DirEntries, err error) error {
		if err != nil {
			fs.Errorf(f, "Failed to list %q: %v", path, err)
			return nil
		}
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

// Hash returns the SHA-1 of an object returning a lowercase hex string
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	if t == o.fs.hashType {
		return o.hash, nil
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
		return fs.ErrorIsDir
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
		o.hash = ""
		switch o.fs.hashType {
		case QuickXorHashType:
			if file.Hashes.QuickXorHash != "" {
				h, err := base64.StdEncoding.DecodeString(file.Hashes.QuickXorHash)
				if err != nil {
					fs.Errorf(o, "Failed to decode QuickXorHash %q: %v", file.Hashes.QuickXorHash, err)
				} else {
					o.hash = hex.EncodeToString(h)
				}
			}
		case hash.SHA1:
			o.hash = strings.ToLower(file.Hashes.Sha1Hash)
		case hash.SHA256:
			o.hash = strings.ToLower(file.Hashes.Sha256Hash)
		case hash.CRC32:
			o.hash = strings.ToLower(file.Hashes.Crc32Hash)
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
	if o.fs.opt.AVOverride {
		opts.Parameters = url.Values{"AVOverride": {"1"}}
	}

	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.Call(ctx, &opts)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if resp != nil {
			if virus := resp.Header.Get("X-Virus-Infected"); virus != "" {
				err = fmt.Errorf("server reports this file is infected with a virus - use --onedrive-av-override to download anyway: %s: %w", virus, err)
			}
		}
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
		return 0, fmt.Errorf("bad number of ranges in upload position: %v", info.NextExpectedRanges)
	}
	position := info.NextExpectedRanges[0]
	i := strings.IndexByte(position, '-')
	if i < 0 {
		return 0, fmt.Errorf("no '-' in next expected range: %q", position)
	}
	position = position[:i]
	pos, err = strconv.ParseInt(position, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("bad expected range: %q: %w", position, err)
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
		resp, err = o.fs.unAuth.Call(ctx, &opts)
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
				return false, fmt.Errorf("sent block already (skip %d < 0), can't rewind: %w", skip, err)
			case skip > chunkSize:
				return false, fmt.Errorf("position is in the future (skip %d > chunkSize %d), can't skip forward: %w", skip, chunkSize, err)
			case skip == chunkSize:
				fs.Debugf(o, "Skipping chunk as already sent (skip %d == chunkSize %d)", skip, chunkSize)
				return false, nil
			}
			return true, fmt.Errorf("retry this chunk skipping %d bytes: %w", skip, err)
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
 *       currently, the Vnet Group's API differs in the following places:
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
	if strings.Contains(ID, "#") {
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
// and https://{Endpoint}/drives/{driveID}/items/children('@a1')/{route}?@a1=URLEncode("'{leaf}'") (for Vnet Group)
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

// ChangeNotify calls the passed function with a path that has had changes.
// If the implementation uses polling, it should adhere to the given interval.
//
// Automatically restarts itself in case of unexpected behavior of the remote.
//
// Close the returned channel to stop being notified.
//
// The Onedrive implementation gives the whole hierarchy up to the  top when
// an object is changed. For instance, if a/b/c is changed, this function
// will call notifyFunc with a, a/b and a/b/c.
func (f *Fs) ChangeNotify(ctx context.Context, notifyFunc func(string, fs.EntryType), pollIntervalChan <-chan time.Duration) {
	go func() {
		// get the StartPageToken early so all changes from now on get processed
		nextDeltaToken, err := f.changeNotifyStartPageToken(ctx)
		if err != nil {
			fs.Errorf(f, "Could not get first deltaLink: %s", err)
			return
		}

		fs.Debugf(f, "Next delta token is: %s", nextDeltaToken)

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
				fs.Debugf(f, "Checking for changes on remote")
				nextDeltaToken, err = f.changeNotifyRunner(ctx, notifyFunc, nextDeltaToken)
				if err != nil {
					fs.Infof(f, "Change notify listener failure: %s", err)
				}
			}
		}
	}()
}

func (f *Fs) changeNotifyStartPageToken(ctx context.Context) (nextDeltaToken string, err error) {
	delta, err := f.changeNotifyNextChange(ctx, "latest")
	if err != nil {
		return
	}
	parsedURL, err := url.Parse(delta.DeltaLink)
	if err != nil {
		return
	}
	nextDeltaToken = parsedURL.Query().Get("token")
	return
}

func (f *Fs) changeNotifyNextChange(ctx context.Context, token string) (delta api.DeltaResponse, err error) {
	opts := f.buildDriveDeltaOpts(token)

	_, err = f.srv.CallJSON(ctx, &opts, nil, &delta)

	return
}

func (f *Fs) buildDriveDeltaOpts(token string) rest.Opts {
	rootURL := graphAPIEndpoint[f.opt.Region] + "/v1.0/drives"

	return rest.Opts{
		Method:     "GET",
		RootURL:    rootURL,
		Path:       "/" + f.driveID + "/root/delta",
		Parameters: map[string][]string{"token": {token}},
	}
}

func (f *Fs) changeNotifyRunner(ctx context.Context, notifyFunc func(string, fs.EntryType), deltaToken string) (nextDeltaToken string, err error) {
	delta, err := f.changeNotifyNextChange(ctx, deltaToken)
	if err != nil {
		return
	}
	parsedURL, err := url.Parse(delta.DeltaLink)
	if err != nil {
		return
	}
	nextDeltaToken = parsedURL.Query().Get("token")

	for _, item := range delta.Value {
		isDriveRootFolder := item.GetParentReference().ID == ""
		if isDriveRootFolder {
			continue
		}

		fullPath, err := getItemFullPath(&item)
		if err != nil {
			fs.Errorf(f, "Could not get item full path: %s", err)
			continue
		}

		if fullPath == f.root {
			continue
		}

		relName, insideRoot := getRelativePathInsideBase(f.root, fullPath)
		if !insideRoot {
			continue
		}

		if item.GetFile() != nil {
			notifyFunc(relName, fs.EntryObject)
		} else if item.GetFolder() != nil {
			notifyFunc(relName, fs.EntryDirectory)
		}
	}

	return
}

func getItemFullPath(item *api.Item) (fullPath string, err error) {
	err = nil
	fullPath = item.GetName()
	if parent := item.GetParentReference(); parent != nil && parent.Path != "" {
		pathParts := strings.SplitN(parent.Path, ":", 2)
		if len(pathParts) != 2 {
			err = fmt.Errorf("invalid parent path: %s", parent.Path)
			return
		}

		if pathParts[1] != "" {
			fullPath = strings.TrimPrefix(pathParts[1], "/") + "/" + fullPath
		}
	}
	return
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
	_ fs.ListRer         = (*Fs)(nil)
	_ fs.Shutdowner      = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.MimeTyper       = &Object{}
	_ fs.IDer            = &Object{}
)
