// Package drive interfaces with the Google Drive object storage system
package drive

// FIXME need to deal with some corner cases
// * multiple files with the same name
// * files can be in multiple directories
// * can have directory loops
// * files with / in name

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"text/template"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/fspath"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/list"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/env"
	"github.com/rclone/rclone/lib/oauthutil"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/readers"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	drive_v2 "google.golang.org/api/drive/v2"
	drive "google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

// Constants
const (
	rcloneClientID              = "202264815644.apps.googleusercontent.com"
	rcloneEncryptedClientSecret = "eX8GpZTVx3vxMWVkuuBdDWmAUE6rGhTwVrvG9GhllYccSdj2-mvHVg"
	driveFolderType             = "application/vnd.google-apps.folder"
	shortcutMimeType            = "application/vnd.google-apps.shortcut"
	shortcutMimeTypeDangling    = "application/vnd.google-apps.shortcut.dangling" // synthetic mime type for internal use
	timeFormatIn                = time.RFC3339
	timeFormatOut               = "2006-01-02T15:04:05.000000000Z07:00"
	defaultMinSleep             = fs.Duration(100 * time.Millisecond)
	defaultBurst                = 100
	defaultExportExtensions     = "docx,xlsx,pptx,svg"
	scopePrefix                 = "https://www.googleapis.com/auth/"
	defaultScope                = "drive"
	// chunkSize is the size of the chunks created during a resumable upload and should be a power of two.
	// 1<<18 is the minimum size supported by the Google uploader, and there is no maximum.
	minChunkSize     = fs.SizeSuffix(googleapi.MinUploadChunkSize)
	defaultChunkSize = 8 * fs.Mebi
	partialFields    = "id,name,size,md5Checksum,sha1Checksum,sha256Checksum,trashed,explicitlyTrashed,modifiedTime,createdTime,mimeType,parents,webViewLink,shortcutDetails,exportLinks,resourceKey"
	listRGrouping    = 50   // number of IDs to search at once when using ListR
	listRInputBuffer = 1000 // size of input buffer when using ListR
	defaultXDGIcon   = "text-html"
)

// Globals
var (
	// Description of how to auth for this app
	driveConfig = &oauthutil.Config{
		Scopes:       []string{scopePrefix + "drive"},
		AuthURL:      google.Endpoint.AuthURL,
		TokenURL:     google.Endpoint.TokenURL,
		ClientID:     rcloneClientID,
		ClientSecret: obscure.MustReveal(rcloneEncryptedClientSecret),
		RedirectURL:  oauthutil.RedirectURL,
	}
	_mimeTypeToExtensionDuplicates = map[string]string{
		"application/x-vnd.oasis.opendocument.presentation": ".odp",
		"application/x-vnd.oasis.opendocument.spreadsheet":  ".ods",
		"application/x-vnd.oasis.opendocument.text":         ".odt",
		"image/jpg":   ".jpg",
		"image/x-bmp": ".bmp",
		"image/x-png": ".png",
		"text/rtf":    ".rtf",
	}
	_mimeTypeToExtension = map[string]string{
		"application/epub+zip":                            ".epub",
		"application/json":                                ".json",
		"application/msword":                              ".doc",
		"application/pdf":                                 ".pdf",
		"application/rtf":                                 ".rtf",
		"application/vnd.ms-excel":                        ".xls",
		"application/vnd.oasis.opendocument.presentation": ".odp",
		"application/vnd.oasis.opendocument.spreadsheet":  ".ods",
		"application/vnd.oasis.opendocument.text":         ".odt",
		"application/vnd.openxmlformats-officedocument.presentationml.presentation": ".pptx",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         ".xlsx",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   ".docx",
		"application/x-msmetafile":  ".wmf",
		"application/zip":           ".zip",
		"image/bmp":                 ".bmp",
		"image/jpeg":                ".jpg",
		"image/pjpeg":               ".pjpeg",
		"image/png":                 ".png",
		"image/svg+xml":             ".svg",
		"text/csv":                  ".csv",
		"text/html":                 ".html",
		"text/plain":                ".txt",
		"text/tab-separated-values": ".tsv",
		"text/markdown":             ".md",
	}
	_mimeTypeToExtensionLinks = map[string]string{
		"application/x-link-desktop": ".desktop",
		"application/x-link-html":    ".link.html",
		"application/x-link-url":     ".url",
		"application/x-link-webloc":  ".webloc",
	}
	_mimeTypeCustomTransform = map[string]string{
		"application/vnd.google-apps.script+json": "application/json",
	}
	_mimeTypeToXDGLinkIcons = map[string]string{
		"application/vnd.google-apps.document":     "x-office-document",
		"application/vnd.google-apps.drawing":      "x-office-drawing",
		"application/vnd.google-apps.presentation": "x-office-presentation",
		"application/vnd.google-apps.spreadsheet":  "x-office-spreadsheet",
	}
	fetchFormatsOnce sync.Once                     // make sure we fetch the export/import formats only once
	_exportFormats   map[string][]string           // allowed export MIME type conversions
	_importFormats   map[string][]string           // allowed import MIME type conversions
	templatesOnce    sync.Once                     // parse link templates only once
	_linkTemplates   map[string]*template.Template // available link types
)

// rwChoices type for fs.Bits
type rwChoices struct{}

func (rwChoices) Choices() []fs.BitsChoicesInfo {
	return []fs.BitsChoicesInfo{
		{Bit: uint64(rwOff), Name: "off"},
		{Bit: uint64(rwRead), Name: "read"},
		{Bit: uint64(rwWrite), Name: "write"},
		{Bit: uint64(rwFailOK), Name: "failok"},
	}
}

// rwChoice type alias
type rwChoice = fs.Bits[rwChoices]

const (
	rwRead rwChoice = 1 << iota
	rwWrite
	rwFailOK
	rwOff rwChoice = 0
)

// Examples for the options
var rwExamples = fs.OptionExamples{{
	Value: rwOff.String(),
	Help:  "Do not read or write the value",
}, {
	Value: rwRead.String(),
	Help:  "Read the value only",
}, {
	Value: rwWrite.String(),
	Help:  "Write the value only",
}, {
	Value: rwFailOK.String(),
	Help:  "If writing fails log errors only, don't fail the transfer",
}, {
	Value: (rwRead | rwWrite).String(),
	Help:  "Read and Write the value.",
}}

// Parse the scopes option returning a slice of scopes
func driveScopes(scopesString string) (scopes []string) {
	if scopesString == "" {
		scopesString = defaultScope
	}
	for _, scope := range strings.Split(scopesString, ",") {
		scope = strings.TrimSpace(scope)
		scopes = append(scopes, scopePrefix+scope)
	}
	return scopes
}

// Returns true if one of the scopes was "drive.appfolder"
func driveScopesContainsAppFolder(scopes []string) bool {
	return slices.Contains(scopes, scopePrefix+"drive.appfolder")
}

func driveOAuthOptions() []fs.Option {
	opts := []fs.Option{}
	for _, opt := range oauthutil.SharedOptions {
		if opt.Name == config.ConfigClientID {
			opt.Help = "Google Application Client Id\nSetting your own is recommended.\nSee https://rclone.org/drive/#making-your-own-client-id for how to create your own.\nIf you leave this blank, it will use an internal key which is low performance."
		}
		opts = append(opts, opt)
	}
	return opts
}

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "drive",
		Description: "Google Drive",
		NewFs:       NewFs,
		CommandHelp: commandHelp,
		Config: func(ctx context.Context, name string, m configmap.Mapper, config fs.ConfigIn) (*fs.ConfigOut, error) {
			// Parse config into Options struct
			opt := new(Options)
			err := configstruct.Set(m, opt)
			if err != nil {
				return nil, fmt.Errorf("couldn't parse config into struct: %w", err)
			}

			switch config.State {
			case "":
				// Fill in the scopes
				driveConfig.Scopes = driveScopes(opt.Scope)

				// Set the root_folder_id if using drive.appfolder
				if driveScopesContainsAppFolder(driveConfig.Scopes) {
					m.Set("root_folder_id", "appDataFolder")
				}

				if opt.ServiceAccountFile == "" && opt.ServiceAccountCredentials == "" && !opt.EnvAuth {
					return oauthutil.ConfigOut("teamdrive", &oauthutil.Options{
						OAuth2Config: driveConfig,
					})
				}
				return fs.ConfigGoto("teamdrive")
			case "teamdrive":
				if opt.TeamDriveID == "" {
					return fs.ConfigConfirm("teamdrive_ok", false, "config_change_team_drive", "Configure this as a Shared Drive (Team Drive)?\n")
				}
				return fs.ConfigConfirm("teamdrive_change", false, "config_change_team_drive", fmt.Sprintf("Change current Shared Drive (Team Drive) ID %q?\n", opt.TeamDriveID))
			case "teamdrive_ok":
				if config.Result == "false" {
					m.Set("team_drive", "")
					return nil, nil
				}
				return fs.ConfigGoto("teamdrive_config")
			case "teamdrive_change":
				if config.Result == "false" {
					return nil, nil
				}
				return fs.ConfigGoto("teamdrive_config")
			case "teamdrive_config":
				f, err := newFs(ctx, name, "", m)
				if err != nil {
					return nil, fmt.Errorf("failed to make Fs to list Shared Drives: %w", err)
				}
				teamDrives, err := f.listTeamDrives(ctx)
				if err != nil {
					return nil, err
				}
				if len(teamDrives) == 0 {
					return fs.ConfigError("", "No Shared Drives found in your account")
				}
				return fs.ConfigChoose("teamdrive_final", "config_team_drive", "Shared Drive", len(teamDrives), func(i int) (string, string) {
					teamDrive := teamDrives[i]
					return teamDrive.Id, teamDrive.Name
				})
			case "teamdrive_final":
				driveID := config.Result
				m.Set("team_drive", driveID)
				m.Set("root_folder_id", "")
				opt.TeamDriveID = driveID
				opt.RootFolderID = ""
				return nil, nil
			}
			return nil, fmt.Errorf("unknown state %q", config.State)
		},
		MetadataInfo: &fs.MetadataInfo{
			System: systemMetadataInfo,
			Help: `User metadata is stored in the properties field of the drive object.

Metadata is supported on files and directories.
`,
		},
		Options: append(driveOAuthOptions(), []fs.Option{{
			Name: "scope",
			Help: "Comma separated list of scopes that rclone should use when requesting access from drive.",
			Examples: []fs.OptionExample{{
				Value: "drive",
				Help:  "Full access all files, excluding Application Data Folder.",
			}, {
				Value: "drive.readonly",
				Help:  "Read-only access to file metadata and file contents.",
			}, {
				Value: "drive.file",
				Help:  "Access to files created by rclone only.\nThese are visible in the drive website.\nFile authorization is revoked when the user deauthorizes the app.",
			}, {
				Value: "drive.appfolder",
				Help:  "Allows read and write access to the Application Data folder.\nThis is not visible in the drive website.",
			}, {
				Value: "drive.metadata.readonly",
				Help:  "Allows read-only access to file metadata but\ndoes not allow any access to read or download file content.",
			}},
		}, {
			Name: "root_folder_id",
			Help: `ID of the root folder.
Leave blank normally.

Fill in to access "Computers" folders (see docs), or for rclone to use
a non root folder as its starting point.
`,
			Advanced:  true,
			Sensitive: true,
		}, {
			Name: "service_account_file",
			Help: "Service Account Credentials JSON file path.\n\nLeave blank normally.\nNeeded only if you want use SA instead of interactive login." + env.ShellExpandHelp,
		}, {
			Name:      "service_account_credentials",
			Help:      "Service Account Credentials JSON blob.\n\nLeave blank normally.\nNeeded only if you want use SA instead of interactive login.",
			Hide:      fs.OptionHideConfigurator,
			Advanced:  true,
			Sensitive: true,
		}, {
			Name:      "team_drive",
			Help:      "ID of the Shared Drive (Team Drive).",
			Hide:      fs.OptionHideConfigurator,
			Advanced:  true,
			Sensitive: true,
		}, {
			Name:     "auth_owner_only",
			Default:  false,
			Help:     "Only consider files owned by the authenticated user.",
			Advanced: true,
		}, {
			Name:     "use_trash",
			Default:  true,
			Help:     "Send files to the trash instead of deleting permanently.\n\nDefaults to true, namely sending files to the trash.\nUse `--drive-use-trash=false` to delete files permanently instead.",
			Advanced: true,
		}, {
			Name:    "copy_shortcut_content",
			Default: false,
			Help: `Server side copy contents of shortcuts instead of the shortcut.

When doing server side copies, normally rclone will copy shortcuts as
shortcuts.

If this flag is used then rclone will copy the contents of shortcuts
rather than shortcuts themselves when doing server side copies.`,
			Advanced: true,
		}, {
			Name:     "skip_gdocs",
			Default:  false,
			Help:     "Skip google documents in all listings.\n\nIf given, gdocs practically become invisible to rclone.",
			Advanced: true,
		}, {
			Name:    "show_all_gdocs",
			Default: false,
			Help: `Show all Google Docs including non-exportable ones in listings.

If you try a server side copy on a Google Form without this flag, you
will get this error:

    No export formats found for "application/vnd.google-apps.form"

However adding this flag will allow the form to be server side copied.

Note that rclone doesn't add extensions to the Google Docs file names
in this mode.

Do **not** use this flag when trying to download Google Docs - rclone
will fail to download them.
`,
			Advanced: true,
		}, {
			Name:    "skip_checksum_gphotos",
			Default: false,
			Help: `Skip checksums on Google photos and videos only.

Use this if you get checksum errors when transferring Google photos or
videos.

Setting this flag will cause Google photos and videos to return a
blank checksums.

Google photos are identified by being in the "photos" space.

Corrupted checksums are caused by Google modifying the image/video but
not updating the checksum.`,
			Advanced: true,
		}, {
			Name:    "shared_with_me",
			Default: false,
			Help: `Only show files that are shared with me.

Instructs rclone to operate on your "Shared with me" folder (where
Google Drive lets you access the files and folders others have shared
with you).

This works both with the "list" (lsd, lsl, etc.) and the "copy"
commands (copy, sync, etc.), and with all other commands too.`,
			Advanced: true,
		}, {
			Name:     "trashed_only",
			Default:  false,
			Help:     "Only show files that are in the trash.\n\nThis will show trashed files in their original directory structure.",
			Advanced: true,
		}, {
			Name:     "starred_only",
			Default:  false,
			Help:     "Only show files that are starred.",
			Advanced: true,
		}, {
			Name:     "formats",
			Default:  "",
			Help:     "Deprecated: See export_formats.",
			Advanced: true,
			Hide:     fs.OptionHideConfigurator,
		}, {
			Name:     "export_formats",
			Default:  defaultExportExtensions,
			Help:     "Comma separated list of preferred formats for downloading Google docs.",
			Advanced: true,
		}, {
			Name:     "import_formats",
			Default:  "",
			Help:     "Comma separated list of preferred formats for uploading Google docs.",
			Advanced: true,
		}, {
			Name:     "allow_import_name_change",
			Default:  false,
			Help:     "Allow the filetype to change when uploading Google docs.\n\nE.g. file.doc to file.docx. This will confuse sync and reupload every time.",
			Advanced: true,
		}, {
			Name:    "use_created_date",
			Default: false,
			Help: `Use file created date instead of modified date.

Useful when downloading data and you want the creation date used in
place of the last modified date.

**WARNING**: This flag may have some unexpected consequences.

When uploading to your drive all files will be overwritten unless they
haven't been modified since their creation. And the inverse will occur
while downloading.  This side effect can be avoided by using the
"--checksum" flag.

This feature was implemented to retain photos capture date as recorded
by google photos. You will first need to check the "Create a Google
Photos folder" option in your google drive settings. You can then copy
or move the photos locally and use the date the image was taken
(created) set as the modification date.`,
			Advanced: true,
			Hide:     fs.OptionHideConfigurator,
		}, {
			Name:    "use_shared_date",
			Default: false,
			Help: `Use date file was shared instead of modified date.

Note that, as with "--drive-use-created-date", this flag may have
unexpected consequences when uploading/downloading files.

If both this flag and "--drive-use-created-date" are set, the created
date is used.`,
			Advanced: true,
			Hide:     fs.OptionHideConfigurator,
		}, {
			Name:     "list_chunk",
			Default:  1000,
			Help:     "Size of listing chunk 100-1000, 0 to disable.",
			Advanced: true,
		}, {
			Name:      "impersonate",
			Default:   "",
			Help:      `Impersonate this user when using a service account.`,
			Advanced:  true,
			Sensitive: true,
		}, {
			Name:    "alternate_export",
			Default: false,
			Help:    "Deprecated: No longer needed.",
			Hide:    fs.OptionHideBoth,
		}, {
			Name:     "upload_cutoff",
			Default:  defaultChunkSize,
			Help:     "Cutoff for switching to chunked upload.",
			Advanced: true,
		}, {
			Name:    "chunk_size",
			Default: defaultChunkSize,
			Help: `Upload chunk size.

Must a power of 2 >= 256k.

Making this larger will improve performance, but note that each chunk
is buffered in memory one per transfer.

Reducing this will reduce memory usage but decrease performance.`,
			Advanced: true,
		}, {
			Name:    "acknowledge_abuse",
			Default: false,
			Help: `Set to allow files which return cannotDownloadAbusiveFile to be downloaded.

If downloading a file returns the error "This file has been identified
as malware or spam and cannot be downloaded" with the error code
"cannotDownloadAbusiveFile" then supply this flag to rclone to
indicate you acknowledge the risks of downloading the file and rclone
will download it anyway.

Note that if you are using service account it will need Manager
permission (not Content Manager) to for this flag to work. If the SA
does not have the right permission, Google will just ignore the flag.`,
			Advanced: true,
		}, {
			Name:     "keep_revision_forever",
			Default:  false,
			Help:     "Keep new head revision of each file forever.",
			Advanced: true,
		}, {
			Name:    "size_as_quota",
			Default: false,
			Help: `Show sizes as storage quota usage, not actual size.

Show the size of a file as the storage quota used. This is the
current version plus any older versions that have been set to keep
forever.

**WARNING**: This flag may have some unexpected consequences.

It is not recommended to set this flag in your config - the
recommended usage is using the flag form --drive-size-as-quota when
doing rclone ls/lsl/lsf/lsjson/etc only.

If you do use this flag for syncing (not recommended) then you will
need to use --ignore size also.`,
			Advanced: true,
			Hide:     fs.OptionHideConfigurator,
		}, {
			Name:     "v2_download_min_size",
			Default:  fs.SizeSuffix(-1),
			Help:     "If Object's are greater, use drive v2 API to download.",
			Advanced: true,
		}, {
			Name:     "pacer_min_sleep",
			Default:  defaultMinSleep,
			Help:     "Minimum time to sleep between API calls.",
			Advanced: true,
		}, {
			Name:     "pacer_burst",
			Default:  defaultBurst,
			Help:     "Number of API calls to allow without sleeping.",
			Advanced: true,
		}, {
			Name:    "server_side_across_configs",
			Default: false,
			Help: `Deprecated: use --server-side-across-configs instead.

Allow server-side operations (e.g. copy) to work across different drive configs.

This can be useful if you wish to do a server-side copy between two
different Google drives.  Note that this isn't enabled by default
because it isn't easy to tell if it will work between any two
configurations.`,
			Advanced: true,
		}, {
			Name:    "disable_http2",
			Default: true,
			Help: `Disable drive using http2.

There is currently an unsolved issue with the google drive backend and
HTTP/2.  HTTP/2 is therefore disabled by default for the drive backend
but can be re-enabled here.  When the issue is solved this flag will
be removed.

See: https://github.com/rclone/rclone/issues/3631

`,
			Advanced: true,
		}, {
			Name:    "stop_on_upload_limit",
			Default: false,
			Help: `Make upload limit errors be fatal.

At the time of writing it is only possible to upload 750 GiB of data to
Google Drive a day (this is an undocumented limit). When this limit is
reached Google Drive produces a slightly different error message. When
this flag is set it causes these errors to be fatal.  These will stop
the in-progress sync.

Note that this detection is relying on error message strings which
Google don't document so it may break in the future.

See: https://github.com/rclone/rclone/issues/3857
`,
			Advanced: true,
		}, {
			Name:    "stop_on_download_limit",
			Default: false,
			Help: `Make download limit errors be fatal.

At the time of writing it is only possible to download 10 TiB of data from
Google Drive a day (this is an undocumented limit). When this limit is
reached Google Drive produces a slightly different error message. When
this flag is set it causes these errors to be fatal.  These will stop
the in-progress sync.

Note that this detection is relying on error message strings which
Google don't document so it may break in the future.
`,
			Advanced: true,
		}, {
			Name: "skip_shortcuts",
			Help: `If set skip shortcut files.

Normally rclone dereferences shortcut files making them appear as if
they are the original file (see [the shortcuts section](#shortcuts)).
If this flag is set then rclone will ignore shortcut files completely.
`,
			Advanced: true,
			Default:  false,
		}, {
			Name: "skip_dangling_shortcuts",
			Help: `If set skip dangling shortcut files.

If this is set then rclone will not show any dangling shortcuts in listings.
`,
			Advanced: true,
			Default:  false,
		}, {
			Name: "resource_key",
			Help: `Resource key for accessing a link-shared file.

If you need to access files shared with a link like this

    https://drive.google.com/drive/folders/XXX?resourcekey=YYY&usp=sharing

Then you will need to use the first part "XXX" as the "root_folder_id"
and the second part "YYY" as the "resource_key" otherwise you will get
404 not found errors when trying to access the directory.

See: https://developers.google.com/drive/api/guides/resource-keys

This resource key requirement only applies to a subset of old files.

Note also that opening the folder once in the web interface (with the
user you've authenticated rclone with) seems to be enough so that the
resource key is not needed.
`,
			Advanced:  true,
			Sensitive: true,
		}, {
			Name: "fast_list_bug_fix",
			Help: `Work around a bug in Google Drive listing.

Normally rclone will work around a bug in Google Drive when using
--fast-list (ListR) where the search "(A in parents) or (B in
parents)" returns nothing sometimes. See #3114, #4289 and
https://issuetracker.google.com/issues/149522397

Rclone detects this by finding no items in more than one directory
when listing and retries them as lists of individual directories.

This means that if you have a lot of empty directories rclone will end
up listing them all individually and this can take many more API
calls.

This flag allows the work-around to be disabled. This is **not**
recommended in normal use - only if you have a particular case you are
having trouble with like many empty directories.
`,
			Advanced: true,
			Default:  true,
		}, {
			Name: "metadata_owner",
			Help: `Control whether owner should be read or written in metadata.

Owner is a standard part of the file metadata so is easy to read. But it
isn't always desirable to set the owner from the metadata.

Note that you can't set the owner on Shared Drives, and that setting
ownership will generate an email to the new owner (this can't be
disabled), and you can't transfer ownership to someone outside your
organization.
`,
			Advanced: true,
			Default:  rwRead,
			Examples: rwExamples,
		}, {
			Name: "metadata_permissions",
			Help: `Control whether permissions should be read or written in metadata.

Reading permissions metadata from files can be done quickly, but it
isn't always desirable to set the permissions from the metadata.

Note that rclone drops any inherited permissions on Shared Drives and
any owner permission on My Drives as these are duplicated in the owner
metadata.
`,
			Advanced: true,
			Default:  rwOff,
			Examples: rwExamples,
		}, {
			Name: "metadata_labels",
			Help: `Control whether labels should be read or written in metadata.

Reading labels metadata from files takes an extra API transaction and
will slow down listings. It isn't always desirable to set the labels
from the metadata.

The format of labels is documented in the drive API documentation at
https://developers.google.com/drive/api/reference/rest/v3/Label -
rclone just provides a JSON dump of this format.

When setting labels, the label and fields must already exist - rclone
will not create them. This means that if you are transferring labels
from two different accounts you will have to create the labels in
advance and use the metadata mapper to translate the IDs between the
two accounts.
`,
			Advanced: true,
			Default:  rwOff,
			Examples: rwExamples,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			// Encode invalid UTF-8 bytes as json doesn't handle them properly.
			// Don't encode / as it's a valid name character in drive.
			Default: encoder.EncodeInvalidUtf8,
		}, {
			Name:     "env_auth",
			Help:     "Get IAM credentials from runtime (environment variables or instance meta data if no env vars).\n\nOnly applies if service_account_file and service_account_credentials is blank.",
			Default:  false,
			Advanced: true,
			Examples: []fs.OptionExample{{
				Value: "false",
				Help:  "Enter credentials in the next step.",
			}, {
				Value: "true",
				Help:  "Get GCP IAM credentials from the environment (env vars or IAM).",
			}},
		}}...),
	})

	// register duplicate MIME types first
	// this allows them to be used with mime.ExtensionsByType() but
	// mime.TypeByExtension() will return the later registered MIME type
	for _, m := range []map[string]string{
		_mimeTypeToExtensionDuplicates, _mimeTypeToExtension, _mimeTypeToExtensionLinks,
	} {
		for mimeType, extension := range m {
			if err := mime.AddExtensionType(extension, mimeType); err != nil {
				fs.Errorf("Failed to register MIME type %q: %v", mimeType, err)
			}
		}
	}
}

// Options defines the configuration for this backend
type Options struct {
	Scope                     string               `config:"scope"`
	RootFolderID              string               `config:"root_folder_id"`
	ServiceAccountFile        string               `config:"service_account_file"`
	ServiceAccountCredentials string               `config:"service_account_credentials"`
	TeamDriveID               string               `config:"team_drive"`
	AuthOwnerOnly             bool                 `config:"auth_owner_only"`
	UseTrash                  bool                 `config:"use_trash"`
	CopyShortcutContent       bool                 `config:"copy_shortcut_content"`
	SkipGdocs                 bool                 `config:"skip_gdocs"`
	ShowAllGdocs              bool                 `config:"show_all_gdocs"`
	SkipChecksumGphotos       bool                 `config:"skip_checksum_gphotos"`
	SharedWithMe              bool                 `config:"shared_with_me"`
	TrashedOnly               bool                 `config:"trashed_only"`
	StarredOnly               bool                 `config:"starred_only"`
	Extensions                string               `config:"formats"`
	ExportExtensions          string               `config:"export_formats"`
	ImportExtensions          string               `config:"import_formats"`
	AllowImportNameChange     bool                 `config:"allow_import_name_change"`
	UseCreatedDate            bool                 `config:"use_created_date"`
	UseSharedDate             bool                 `config:"use_shared_date"`
	ListChunk                 int64                `config:"list_chunk"`
	Impersonate               string               `config:"impersonate"`
	UploadCutoff              fs.SizeSuffix        `config:"upload_cutoff"`
	ChunkSize                 fs.SizeSuffix        `config:"chunk_size"`
	AcknowledgeAbuse          bool                 `config:"acknowledge_abuse"`
	KeepRevisionForever       bool                 `config:"keep_revision_forever"`
	SizeAsQuota               bool                 `config:"size_as_quota"`
	V2DownloadMinSize         fs.SizeSuffix        `config:"v2_download_min_size"`
	PacerMinSleep             fs.Duration          `config:"pacer_min_sleep"`
	PacerBurst                int                  `config:"pacer_burst"`
	ServerSideAcrossConfigs   bool                 `config:"server_side_across_configs"`
	DisableHTTP2              bool                 `config:"disable_http2"`
	StopOnUploadLimit         bool                 `config:"stop_on_upload_limit"`
	StopOnDownloadLimit       bool                 `config:"stop_on_download_limit"`
	SkipShortcuts             bool                 `config:"skip_shortcuts"`
	SkipDanglingShortcuts     bool                 `config:"skip_dangling_shortcuts"`
	ResourceKey               string               `config:"resource_key"`
	FastListBugFix            bool                 `config:"fast_list_bug_fix"`
	MetadataOwner             rwChoice             `config:"metadata_owner"`
	MetadataPermissions       rwChoice             `config:"metadata_permissions"`
	MetadataLabels            rwChoice             `config:"metadata_labels"`
	Enc                       encoder.MultiEncoder `config:"encoding"`
	EnvAuth                   bool                 `config:"env_auth"`
}

// Fs represents a remote drive server
type Fs struct {
	name             string             // name of this remote
	root             string             // the path we are working on
	opt              Options            // parsed options
	ci               *fs.ConfigInfo     // global config
	features         *fs.Features       // optional features
	svc              *drive.Service     // the connection to the drive server
	v2Svc            *drive_v2.Service  // used to create download links for the v2 api
	client           *http.Client       // authorized client
	rootFolderID     string             // the id of the root folder
	dirCache         *dircache.DirCache // Map of directory path to directory id
	lastQuery        string             // Last query string to check in unit tests
	pacer            *fs.Pacer          // To pace the API calls
	exportExtensions []string           // preferred extensions to download docs
	importMimeTypes  []string           // MIME types to convert to docs
	isTeamDrive      bool               // true if this is a team drive
	m                configmap.Mapper
	grouping         int32                        // number of IDs to search at once in ListR - read with atomic
	listRmu          *sync.Mutex                  // protects listRempties
	listRempties     map[string]struct{}          // IDs of supposedly empty directories which triggered grouping disable
	dirResourceKeys  *sync.Map                    // map directory ID to resource key
	permissionsMu    *sync.Mutex                  // protect the below
	permissions      map[string]*drive.Permission // map permission IDs to Permissions
}

type baseObject struct {
	fs           *Fs          // what this object is part of
	remote       string       // The remote path
	id           string       // Drive Id of this object
	modifiedDate string       // RFC3339 time it was last modified
	mimeType     string       // The object MIME type
	bytes        int64        // size of the object
	parents      []string     // IDs of the parent directories
	resourceKey  *string      // resourceKey is needed for link shared objects
	metadata     *fs.Metadata // metadata if known
}
type documentObject struct {
	baseObject
	url              string // Download URL of this object
	documentMimeType string // the original document MIME type
	extLen           int    // The length of the added export extension
}
type linkObject struct {
	baseObject
	content []byte // The file content generated by a link template
	extLen  int    // The length of the added export extension
}

// Object describes a drive object
type Object struct {
	baseObject
	url        string // Download URL of this object
	md5sum     string // md5sum of the object
	sha1sum    string // sha1sum of the object
	sha256sum  string // sha256sum of the object
	v2Download bool   // generate v2 download link ondemand
}

// Directory describes a drive directory
type Directory struct {
	baseObject
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
	return fmt.Sprintf("Google drive root '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// shouldRetry determines whether a given err rates being retried
func (f *Fs) shouldRetry(ctx context.Context, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	if err == nil {
		return false, nil
	}
	if fserrors.ShouldRetry(err) {
		return true, err
	}
	switch gerr := err.(type) {
	case *googleapi.Error:
		if gerr.Code >= 500 && gerr.Code < 600 {
			// All 5xx errors should be retried
			return true, err
		}
		if len(gerr.Errors) > 0 {
			reason := gerr.Errors[0].Reason
			if reason == "rateLimitExceeded" || reason == "userRateLimitExceeded" {
				if f.opt.StopOnUploadLimit && gerr.Errors[0].Message == "User rate limit exceeded." {
					fs.Errorf(f, "Received upload limit error: %v", err)
					return false, fserrors.FatalError(err)
				}
				return true, err
			} else if f.opt.StopOnDownloadLimit && reason == "downloadQuotaExceeded" {
				fs.Errorf(f, "Received download limit error: %v", err)
				return false, fserrors.FatalError(err)
			} else if f.opt.StopOnUploadLimit && (reason == "quotaExceeded" || reason == "storageQuotaExceeded") {
				fs.Errorf(f, "Received upload limit error: %v", err)
				return false, fserrors.FatalError(err)
			} else if f.opt.StopOnUploadLimit && reason == "teamDriveFileLimitExceeded" {
				fs.Errorf(f, "Received Shared Drive file limit error: %v", err)
				return false, fserrors.FatalError(err)
			}
		}
	}
	return false, err
}

// parseParse parses a drive 'url'
func parseDrivePath(path string) (root string, err error) {
	root = strings.Trim(path, "/")
	return
}

// User function to process a File item from list
//
// Should return true to finish processing
type listFn func(*drive.File) bool

func containsString(slice []string, s string) bool {
	return slices.Contains(slice, s)
}

// getFile returns drive.File for the ID passed and fields passed in
func (f *Fs) getFile(ctx context.Context, ID string, fields googleapi.Field) (info *drive.File, err error) {
	err = f.pacer.Call(func() (bool, error) {
		info, err = f.svc.Files.Get(ID).
			Fields(fields).
			SupportsAllDrives(true).
			Context(ctx).Do()
		return f.shouldRetry(ctx, err)
	})
	return info, err
}

// getRootID returns the canonical ID for the "root" ID
func (f *Fs) getRootID(ctx context.Context) (string, error) {
	info, err := f.getFile(ctx, "root", "id")
	if err != nil {
		return "", fmt.Errorf("couldn't find root directory ID: %w", err)
	}
	return info.Id, nil
}

// Lists the directory required calling the user function on each item found
//
// If the user fn ever returns true then it early exits with found = true
//
// Search params: https://developers.google.com/drive/search-parameters
func (f *Fs) list(ctx context.Context, dirIDs []string, title string, directoriesOnly, filesOnly, trashedOnly, includeAll bool, fn listFn) (found bool, err error) {
	var query []string
	if !includeAll {
		q := "trashed=" + strconv.FormatBool(trashedOnly)
		if f.opt.TrashedOnly {
			q = fmt.Sprintf("(mimeType='%s' or %s)", driveFolderType, q)
		}
		query = append(query, q)
	}

	// Search with sharedWithMe will always return things listed in "Shared With Me" (without any parents)
	// We must not filter with parent when we try list "ROOT" with drive-shared-with-me
	// If we need to list file inside those shared folders, we must search it without sharedWithMe
	parentsQuery := bytes.NewBufferString("(")
	var resourceKeys []string
	for _, dirID := range dirIDs {
		if dirID == "" {
			continue
		}
		if parentsQuery.Len() > 1 {
			_, _ = parentsQuery.WriteString(" or ")
		}
		if (f.opt.SharedWithMe || f.opt.StarredOnly) && dirID == f.rootFolderID {
			if f.opt.SharedWithMe {
				_, _ = parentsQuery.WriteString("sharedWithMe=true")
			}
			if f.opt.StarredOnly {
				if f.opt.SharedWithMe {
					_, _ = parentsQuery.WriteString(" and ")
				}
				_, _ = parentsQuery.WriteString("starred=true")
			}
		} else {
			_, _ = fmt.Fprintf(parentsQuery, "'%s' in parents", dirID)
		}
		resourceKey, hasResourceKey := f.dirResourceKeys.Load(dirID)
		if hasResourceKey {
			resourceKeys = append(resourceKeys, fmt.Sprintf("%s/%s", dirID, resourceKey))
		}
	}
	resourceKeysHeader := strings.Join(resourceKeys, ",")
	if parentsQuery.Len() > 1 {
		_ = parentsQuery.WriteByte(')')
		query = append(query, parentsQuery.String())
	}
	var stems []string
	if title != "" {
		searchTitle := f.opt.Enc.FromStandardName(title)
		// Escaping the backslash isn't documented but seems to work
		searchTitle = strings.ReplaceAll(searchTitle, `\`, `\\`)
		searchTitle = strings.ReplaceAll(searchTitle, `'`, `\'`)

		var titleQuery bytes.Buffer
		_, _ = fmt.Fprintf(&titleQuery, "(name='%s'", searchTitle)
		if !directoriesOnly && !f.opt.SkipGdocs {
			// If the search title has an extension that is in the export extensions add a search
			// for the filename without the extension.
			// Assume that export extensions don't contain escape sequences.
			for _, ext := range f.exportExtensions {
				if strings.HasSuffix(searchTitle, ext) {
					stems = append(stems, title[:len(title)-len(ext)])
					_, _ = fmt.Fprintf(&titleQuery, " or name='%s'", searchTitle[:len(searchTitle)-len(ext)])
				}
			}
		}
		_ = titleQuery.WriteByte(')')
		query = append(query, titleQuery.String())
	}
	if directoriesOnly {
		query = append(query, fmt.Sprintf("(mimeType='%s' or mimeType='%s')", driveFolderType, shortcutMimeType))
	}
	if filesOnly {
		query = append(query, fmt.Sprintf("mimeType!='%s'", driveFolderType))
	}

	// Constrain query using filter if this remote is a sync/copy/walk source.
	if fi, use := filter.GetConfig(ctx), filter.GetUseFilter(ctx); fi != nil && use {
		queryByTime := func(op string, tm time.Time) {
			if tm.IsZero() {
				return
			}
			// https://developers.google.com/drive/api/v3/ref-search-terms#operators
			// Query times use RFC 3339 format, default timezone is UTC
			timeStr := tm.UTC().Format("2006-01-02T15:04:05")
			term := fmt.Sprintf("(modifiedTime %s '%s' or mimeType = '%s')", op, timeStr, driveFolderType)
			query = append(query, term)
		}
		queryByTime(">=", fi.ModTimeFrom)
		queryByTime("<=", fi.ModTimeTo)
	}

	list := f.svc.Files.List()
	queryString := strings.Join(query, " and ")
	if queryString != "" {
		list.Q(queryString)
		// fs.Debugf(f, "list query: %q", queryString)
	}
	f.lastQuery = queryString // for unit tests

	if f.opt.ListChunk > 0 {
		list.PageSize(f.opt.ListChunk)
	}
	list.SupportsAllDrives(true)
	list.IncludeItemsFromAllDrives(true)
	if f.isTeamDrive && !f.opt.SharedWithMe {
		list.DriveId(f.opt.TeamDriveID)
		list.Corpora("drive")
	}
	// If using appDataFolder then need to add Spaces
	if f.rootFolderID == "appDataFolder" {
		list.Spaces("appDataFolder")
	}
	// Add resource Keys if necessary
	if resourceKeysHeader != "" {
		list.Header().Add("X-Goog-Drive-Resource-Keys", resourceKeysHeader)
	}

	fields := fmt.Sprintf("files(%s),nextPageToken,incompleteSearch", f.getFileFields(ctx))

OUTER:
	for {
		var files *drive.FileList
		err = f.pacer.Call(func() (bool, error) {
			files, err = list.Fields(googleapi.Field(fields)).Context(ctx).Do()
			return f.shouldRetry(ctx, err)
		})
		if err != nil {
			return false, fmt.Errorf("couldn't list directory: %w", err)
		}
		if files.IncompleteSearch {
			fs.Errorf(f, "search result INCOMPLETE")
		}
		for _, item := range files.Files {
			item.Name = f.opt.Enc.ToStandardName(item.Name)
			if isShortcut(item) {
				// ignore shortcuts if directed
				if f.opt.SkipShortcuts {
					continue
				}
				// skip file shortcuts if directory only
				if directoriesOnly && item.ShortcutDetails.TargetMimeType != driveFolderType {
					continue
				}
				// skip directory shortcuts if file only
				if filesOnly && item.ShortcutDetails.TargetMimeType == driveFolderType {
					continue
				}
				item, err = f.resolveShortcut(ctx, item)
				if err != nil {
					return false, fmt.Errorf("list: %w", err)
				}
				// leave the dangling shortcut out of the listings
				// we've already logged about the dangling shortcut in resolveShortcut
				if f.opt.SkipDanglingShortcuts && item.MimeType == shortcutMimeTypeDangling {
					continue
				}
			}
			// Check the case of items is correct since
			// the `=` operator is case insensitive.
			if title != "" && title != item.Name {
				found := slices.Contains(stems, item.Name)
				if !found {
					continue
				}
				_, exportName, _, _ := f.findExportFormat(ctx, item)
				if exportName == "" || exportName != title {
					continue
				}
			}
			if fn(item) {
				found = true
				break OUTER
			}
		}
		if files.NextPageToken == "" {
			break
		}
		list.PageToken(files.NextPageToken)
	}
	return
}

// Returns true of x is a power of 2 or zero
func isPowerOfTwo(x int64) bool {
	switch {
	case x == 0:
		return true
	case x < 0:
		return false
	default:
		return (x & (x - 1)) == 0
	}
}

// add a charset parameter to all text/* MIME types
func fixMimeType(mimeTypeIn string) string {
	if mimeTypeIn == "" {
		return ""
	}
	mediaType, param, err := mime.ParseMediaType(mimeTypeIn)
	if err != nil {
		return mimeTypeIn
	}
	mimeTypeOut := mimeTypeIn
	if strings.HasPrefix(mediaType, "text/") && param["charset"] == "" {
		param["charset"] = "utf-8"
		mimeTypeOut = mime.FormatMediaType(mediaType, param)
	}
	if mimeTypeOut == "" {
		panic(fmt.Errorf("unable to fix MIME type %q", mimeTypeIn))
	}
	return mimeTypeOut
}

func fixMimeTypeMap(in map[string][]string) (out map[string][]string) {
	out = make(map[string][]string, len(in))
	for k, v := range in {
		for i, mt := range v {
			v[i] = fixMimeType(mt)
		}
		out[fixMimeType(k)] = v
	}
	return out
}

func isInternalMimeType(mimeType string) bool {
	return strings.HasPrefix(mimeType, "application/vnd.google-apps.")
}

func isLinkMimeType(mimeType string) bool {
	return strings.HasPrefix(mimeType, "application/x-link-")
}

// parseExtensions parses a list of comma separated extensions
// into a list of unique extensions with leading "." and a list of associated MIME types
func parseExtensions(extensionsIn ...string) (extensions, mimeTypes []string, err error) {
	for _, extensionText := range extensionsIn {
		for _, extension := range strings.Split(extensionText, ",") {
			extension = strings.ToLower(strings.TrimSpace(extension))
			if extension == "" {
				continue
			}
			if len(extension) > 0 && extension[0] != '.' {
				extension = "." + extension
			}
			mt := mime.TypeByExtension(extension)
			if mt == "" {
				return extensions, mimeTypes, fmt.Errorf("couldn't find MIME type for extension %q", extension)
			}
			if !containsString(extensions, extension) {
				extensions = append(extensions, extension)
				mimeTypes = append(mimeTypes, mt)
			}
		}
	}
	return
}

// getClient makes an http client according to the options
func getClient(ctx context.Context, opt *Options) *http.Client {
	t := fshttp.NewTransportCustom(ctx, func(t *http.Transport) {
		if opt.DisableHTTP2 {
			t.TLSNextProto = map[string]func(string, *tls.Conn) http.RoundTripper{}
		}
	})
	return &http.Client{
		Transport: t,
	}
}

func getServiceAccountClient(ctx context.Context, opt *Options, credentialsData []byte) (*http.Client, error) {
	scopes := driveScopes(opt.Scope)
	conf, err := google.JWTConfigFromJSON(credentialsData, scopes...)
	if err != nil {
		return nil, fmt.Errorf("error processing credentials: %w", err)
	}
	if opt.Impersonate != "" {
		conf.Subject = opt.Impersonate
	}
	ctxWithSpecialClient := oauthutil.Context(ctx, getClient(ctx, opt))
	return oauth2.NewClient(ctxWithSpecialClient, conf.TokenSource(ctxWithSpecialClient)), nil
}

func createOAuthClient(ctx context.Context, opt *Options, name string, m configmap.Mapper) (*http.Client, error) {
	var oAuthClient *http.Client
	var err error

	// try loading service account credentials from env variable, then from a file
	if len(opt.ServiceAccountCredentials) == 0 && opt.ServiceAccountFile != "" {
		loadedCreds, err := os.ReadFile(env.ShellExpand(opt.ServiceAccountFile))
		if err != nil {
			return nil, fmt.Errorf("error opening service account credentials file: %w", err)
		}
		opt.ServiceAccountCredentials = string(loadedCreds)
	}
	if opt.ServiceAccountCredentials != "" {
		oAuthClient, err = getServiceAccountClient(ctx, opt, []byte(opt.ServiceAccountCredentials))
		if err != nil {
			return nil, fmt.Errorf("failed to create oauth client from service account: %w", err)
		}
	} else if opt.EnvAuth {
		scopes := driveScopes(opt.Scope)
		oAuthClient, err = google.DefaultClient(ctx, scopes...)
		if err != nil {
			return nil, fmt.Errorf("failed to create client from environment: %w", err)
		}
	} else {
		oAuthClient, _, err = oauthutil.NewClientWithBaseClient(ctx, name, m, driveConfig, getClient(ctx, opt))
		if err != nil {
			return nil, fmt.Errorf("failed to create oauth client: %w", err)
		}
	}

	return oAuthClient, nil
}

func checkUploadChunkSize(cs fs.SizeSuffix) error {
	if !isPowerOfTwo(int64(cs)) {
		return fmt.Errorf("%v isn't a power of two", cs)
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

func checkUploadCutoff(cs fs.SizeSuffix) error {
	return nil
}

func (f *Fs) setUploadCutoff(cs fs.SizeSuffix) (old fs.SizeSuffix, err error) {
	err = checkUploadCutoff(cs)
	if err == nil {
		old, f.opt.UploadCutoff = f.opt.UploadCutoff, cs
	}
	return
}

// newFs partially constructs Fs from the path
//
// It constructs a valid Fs but doesn't attempt to figure out whether
// it is a file or a directory.
func newFs(ctx context.Context, name, path string, m configmap.Mapper) (*Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	err = checkUploadCutoff(opt.UploadCutoff)
	if err != nil {
		return nil, fmt.Errorf("drive: upload cutoff: %w", err)
	}
	err = checkUploadChunkSize(opt.ChunkSize)
	if err != nil {
		return nil, fmt.Errorf("drive: chunk size: %w", err)
	}

	oAuthClient, err := createOAuthClient(ctx, opt, name, m)
	if err != nil {
		return nil, fmt.Errorf("drive: failed when making oauth client: %w", err)
	}

	root, err := parseDrivePath(path)
	if err != nil {
		return nil, err
	}

	ci := fs.GetConfig(ctx)
	f := &Fs{
		name:            name,
		root:            root,
		opt:             *opt,
		ci:              ci,
		pacer:           fs.NewPacer(ctx, pacer.NewGoogleDrive(pacer.MinSleep(opt.PacerMinSleep), pacer.Burst(opt.PacerBurst))),
		m:               m,
		grouping:        listRGrouping,
		listRmu:         new(sync.Mutex),
		listRempties:    make(map[string]struct{}),
		dirResourceKeys: new(sync.Map),
		permissionsMu:   new(sync.Mutex),
		permissions:     make(map[string]*drive.Permission),
	}
	f.isTeamDrive = opt.TeamDriveID != ""
	f.features = (&fs.Features{
		DuplicateFiles:           true,
		ReadMimeType:             true,
		WriteMimeType:            true,
		CanHaveEmptyDirectories:  true,
		ServerSideAcrossConfigs:  opt.ServerSideAcrossConfigs,
		FilterAware:              true,
		ReadMetadata:             true,
		WriteMetadata:            true,
		UserMetadata:             true,
		ReadDirMetadata:          true,
		WriteDirMetadata:         true,
		WriteDirSetModTime:       true,
		UserDirMetadata:          true,
		DirModTimeUpdatesOnWrite: false, // FIXME need to check!
	}).Fill(ctx, f)

	// Create a new authorized Drive client.
	f.client = oAuthClient
	f.svc, err = drive.NewService(context.Background(), option.WithHTTPClient(f.client))
	if err != nil {
		return nil, fmt.Errorf("couldn't create Drive client: %w", err)
	}

	if f.opt.V2DownloadMinSize >= 0 {
		f.v2Svc, err = drive_v2.NewService(context.Background(), option.WithHTTPClient(f.client))
		if err != nil {
			return nil, fmt.Errorf("couldn't create Drive v2 client: %w", err)
		}
	}

	return f, nil
}

// NewFs constructs an Fs from the path, container:path
func NewFs(ctx context.Context, name, path string, m configmap.Mapper) (fs.Fs, error) {
	f, err := newFs(ctx, name, path, m)
	if err != nil {
		return nil, err
	}

	// Set the root folder ID
	if f.opt.RootFolderID != "" {
		// use root_folder ID if set
		f.rootFolderID = f.opt.RootFolderID
	} else if f.isTeamDrive {
		// otherwise use team_drive if set
		f.rootFolderID = f.opt.TeamDriveID
	} else {
		// otherwise look up the actual root ID
		rootID, err := f.getRootID(ctx)
		if err != nil {
			var gerr *googleapi.Error
			if errors.As(err, &gerr) && gerr.Code == 404 {
				// 404 means that this scope does not have permission to get the
				// root so just use "root"
				rootID = "root"
			} else {
				return nil, err
			}
		}
		f.rootFolderID = rootID
		fs.Debugf(f, "'root_folder_id = %s' - save this in the config to speed up startup", rootID)
	}

	f.dirCache = dircache.New(f.root, f.rootFolderID, f)

	// If resource key is set then cache it for the root folder id
	if f.opt.ResourceKey != "" {
		f.dirResourceKeys.Store(f.rootFolderID, f.opt.ResourceKey)
	}

	// Parse extensions
	if f.opt.Extensions != "" {
		if f.opt.ExportExtensions != defaultExportExtensions {
			return nil, errors.New("only one of 'formats' and 'export_formats' can be specified")
		}
		f.opt.Extensions, f.opt.ExportExtensions = "", f.opt.Extensions
	}
	f.exportExtensions, _, err = parseExtensions(f.opt.ExportExtensions, defaultExportExtensions)
	if err != nil {
		return nil, err
	}

	_, f.importMimeTypes, err = parseExtensions(f.opt.ImportExtensions)
	if err != nil {
		return nil, err
	}

	// Find the current root
	err = f.dirCache.FindRoot(ctx, false)
	if err != nil {
		// Assume it is a file
		newRoot, remote := dircache.SplitPath(f.root)
		tempF := *f
		tempF.dirCache = dircache.New(newRoot, f.rootFolderID, &tempF)
		tempF.root = newRoot
		// Make new Fs which is the parent
		err = tempF.dirCache.FindRoot(ctx, false)
		if err != nil {
			// No root so return old f
			return f, nil
		}
		_, err := tempF.NewObject(ctx, remote)
		if err != nil {
			// unable to list folder so return old f
			return f, nil
		}
		// XXX: update the old f here instead of returning tempF, since
		// `features` were already filled with functions having *f as a receiver.
		// See https://github.com/rclone/rclone/issues/2182
		f.dirCache = tempF.dirCache
		f.root = tempF.root
		return f, fs.ErrorIsFile
	}
	// fmt.Printf("Root id %s", f.dirCache.RootID())
	return f, nil
}

func (f *Fs) newBaseObject(ctx context.Context, remote string, info *drive.File) (o baseObject, err error) {
	modifiedDate := info.ModifiedTime
	if f.opt.UseCreatedDate {
		modifiedDate = info.CreatedTime
	} else if f.opt.UseSharedDate && info.SharedWithMeTime != "" {
		modifiedDate = info.SharedWithMeTime
	}
	size := info.Size
	if f.opt.SizeAsQuota {
		size = info.QuotaBytesUsed
	}
	o = baseObject{
		fs:           f,
		remote:       remote,
		id:           info.Id,
		modifiedDate: modifiedDate,
		mimeType:     info.MimeType,
		bytes:        size,
		parents:      info.Parents,
	}
	err = nil
	if fs.GetConfig(ctx).Metadata {
		err = o.parseMetadata(ctx, info)
	}
	return o, err
}

// getFileFields gets the fields for a normal file Get or List
func (f *Fs) getFileFields(ctx context.Context) (fields googleapi.Field) {
	fields = partialFields
	if f.opt.AuthOwnerOnly {
		fields += ",owners"
	}
	if f.opt.UseSharedDate {
		fields += ",sharedWithMeTime"
	}
	if f.opt.SkipChecksumGphotos {
		fields += ",spaces"
	}
	if f.opt.SizeAsQuota {
		fields += ",quotaBytesUsed"
	}
	if fs.GetConfig(ctx).Metadata {
		fields += "," + metadataFields
	}
	return fields
}

// newRegularObject creates an fs.Object for a normal drive.File
func (f *Fs) newRegularObject(ctx context.Context, remote string, info *drive.File) (obj fs.Object, err error) {
	// wipe checksum if SkipChecksumGphotos and file is type Photo or Video
	if f.opt.SkipChecksumGphotos {
		if slices.Contains(info.Spaces, "photos") {
			info.Md5Checksum = ""
			info.Sha1Checksum = ""
			info.Sha256Checksum = ""
		}
	}
	o := &Object{
		url:        fmt.Sprintf("%sfiles/%s?alt=media", f.svc.BasePath, actualID(info.Id)),
		md5sum:     strings.ToLower(info.Md5Checksum),
		sha1sum:    strings.ToLower(info.Sha1Checksum),
		sha256sum:  strings.ToLower(info.Sha256Checksum),
		v2Download: f.opt.V2DownloadMinSize != -1 && info.Size >= int64(f.opt.V2DownloadMinSize),
	}
	o.baseObject, err = f.newBaseObject(ctx, remote, info)
	if err != nil {
		return nil, err
	}
	if info.ResourceKey != "" {
		o.resourceKey = &info.ResourceKey
	}
	return o, nil
}

// newDocumentObject creates an fs.Object for a google docs drive.File
func (f *Fs) newDocumentObject(ctx context.Context, remote string, info *drive.File, extension, exportMimeType string) (fs.Object, error) {
	mediaType, _, err := mime.ParseMediaType(exportMimeType)
	if err != nil {
		return nil, err
	}
	url := info.ExportLinks[mediaType]
	baseObject, err := f.newBaseObject(ctx, remote+extension, info)
	if err != nil {
		return nil, err
	}
	baseObject.bytes = -1
	baseObject.mimeType = exportMimeType
	return &documentObject{
		baseObject:       baseObject,
		url:              url,
		documentMimeType: info.MimeType,
		extLen:           len(extension),
	}, nil
}

// newLinkObject creates an fs.Object that represents a link a google docs drive.File
func (f *Fs) newLinkObject(ctx context.Context, remote string, info *drive.File, extension, exportMimeType string) (fs.Object, error) {
	t := linkTemplate(exportMimeType)
	if t == nil {
		return nil, fmt.Errorf("unsupported link type %s", exportMimeType)
	}
	xdgIcon := _mimeTypeToXDGLinkIcons[info.MimeType]
	if xdgIcon == "" {
		xdgIcon = defaultXDGIcon
	}
	var buf bytes.Buffer
	err := t.Execute(&buf, struct {
		URL, Title, XDGIcon string
	}{
		info.WebViewLink, info.Name, xdgIcon,
	})
	if err != nil {
		return nil, fmt.Errorf("executing template failed: %w", err)
	}

	baseObject, err := f.newBaseObject(ctx, remote+extension, info)
	if err != nil {
		return nil, err
	}
	baseObject.bytes = int64(buf.Len())
	baseObject.mimeType = exportMimeType
	return &linkObject{
		baseObject: baseObject,
		content:    buf.Bytes(),
		extLen:     len(extension),
	}, nil
}

// newObjectWithInfo creates an fs.Object for any drive.File
//
// When the drive.File cannot be represented as an fs.Object it will return (nil, nil).
func (f *Fs) newObjectWithInfo(ctx context.Context, remote string, info *drive.File) (fs.Object, error) {
	// If item has MD5 sum it is a file stored on drive
	if info.Md5Checksum != "" {
		return f.newRegularObject(ctx, remote, info)
	}

	extension, exportName, exportMimeType, isDocument := f.findExportFormat(ctx, info)
	return f.newObjectWithExportInfo(ctx, remote, info, extension, exportName, exportMimeType, isDocument)
}

// newObjectWithExportInfo creates an fs.Object for any drive.File and the result of findExportFormat
//
// When the drive.File cannot be represented as an fs.Object it will return (nil, nil).
func (f *Fs) newObjectWithExportInfo(
	ctx context.Context, remote string, info *drive.File,
	extension, exportName, exportMimeType string, isDocument bool,
) (o fs.Object, err error) {
	// Note that resolveShortcut will have been called already if
	// we are being called from a listing. However the drive.Item
	// will have been resolved so this will do nothing.
	info, err = f.resolveShortcut(ctx, info)
	if err != nil {
		return nil, fmt.Errorf("new object: %w", err)
	}
	switch {
	case info.MimeType == driveFolderType:
		return nil, fs.ErrorIsDir
	case info.MimeType == shortcutMimeType:
		// We can only get here if f.opt.SkipShortcuts is set
		// and not from a listing. This is unlikely.
		fs.Debugf(remote, "Ignoring shortcut as skip shortcuts is set")
		return nil, fs.ErrorObjectNotFound
	case info.MimeType == shortcutMimeTypeDangling:
		// Pretend a dangling shortcut is a regular object
		// It will error if used, but appear in listings so it can be deleted
		return f.newRegularObject(ctx, remote, info)
	case info.Md5Checksum != "":
		// If item has MD5 sum it is a file stored on drive
		return f.newRegularObject(ctx, remote, info)
	case f.opt.SkipGdocs:
		fs.Debugf(remote, "Skipping google document type %q", info.MimeType)
		return nil, fs.ErrorObjectNotFound
	case f.opt.ShowAllGdocs:
		return f.newDocumentObject(ctx, remote, info, "", info.MimeType)
	default:
		// If item MimeType is in the ExportFormats then it is a google doc
		if !isDocument {
			fs.Debugf(remote, "Ignoring unknown document type %q", info.MimeType)
			return nil, fs.ErrorObjectNotFound
		}
		if extension == "" {
			fs.Debugf(remote, "No export formats found for %q", info.MimeType)
			return nil, fs.ErrorObjectNotFound
		}
		if isLinkMimeType(exportMimeType) {
			return f.newLinkObject(ctx, remote, info, extension, exportMimeType)
		}
		return f.newDocumentObject(ctx, remote, info, extension, exportMimeType)
	}
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	if strings.HasSuffix(remote, "/") {
		return nil, fs.ErrorIsDir
	}
	info, extension, exportName, exportMimeType, isDocument, err := f.getRemoteInfoWithExport(ctx, remote)
	if err != nil {
		return nil, err
	}

	remote = remote[:len(remote)-len(extension)]
	obj, err := f.newObjectWithExportInfo(ctx, remote, info, extension, exportName, exportMimeType, isDocument)
	switch {
	case err != nil:
		return nil, err
	case obj == nil:
		return nil, fs.ErrorObjectNotFound
	default:
		return obj, nil
	}
}

// FindLeaf finds a directory of name leaf in the folder with ID pathID
func (f *Fs) FindLeaf(ctx context.Context, pathID, leaf string) (pathIDOut string, found bool, err error) {
	// Find the leaf in pathID
	pathID = actualID(pathID)
	found, err = f.list(ctx, []string{pathID}, leaf, true, false, f.opt.TrashedOnly, false, func(item *drive.File) bool {
		if !f.opt.SkipGdocs {
			_, exportName, _, isDocument := f.findExportFormat(ctx, item)
			if exportName == leaf {
				pathIDOut = item.Id
				return true
			}
			if isDocument {
				return false
			}
		}
		if item.Name == leaf {
			pathIDOut = item.Id
			return true
		}
		return false
	})
	return pathIDOut, found, err
}

// createDir makes a directory with pathID as parent and name leaf with optional metadata
func (f *Fs) createDir(ctx context.Context, pathID, leaf string, metadata fs.Metadata) (info *drive.File, err error) {
	leaf = f.opt.Enc.FromStandardName(leaf)
	pathID = actualID(pathID)
	createInfo := &drive.File{
		Name:     leaf,
		MimeType: driveFolderType,
		Parents:  []string{pathID},
	}
	var updateMetadata updateMetadataFn
	if len(metadata) > 0 {
		updateMetadata, err = f.updateMetadata(ctx, createInfo, metadata, true, true)
		if err != nil {
			return nil, fmt.Errorf("create dir: failed to update metadata: %w", err)
		}
	}
	err = f.pacer.Call(func() (bool, error) {
		info, err = f.svc.Files.Create(createInfo).
			Fields(f.getFileFields(ctx)).
			SupportsAllDrives(true).
			Context(ctx).Do()
		return f.shouldRetry(ctx, err)
	})
	if err != nil {
		return nil, err
	}
	if updateMetadata != nil {
		err = updateMetadata(ctx, info)
		if err != nil {
			return nil, err
		}
	}
	return info, nil
}

// updateDir updates an existing a directory with the metadata passed in
func (f *Fs) updateDir(ctx context.Context, dirID string, metadata fs.Metadata) (info *drive.File, err error) {
	if len(metadata) == 0 {
		return f.getFile(ctx, dirID, f.getFileFields(ctx))
	}
	dirID = actualID(dirID)
	updateInfo := &drive.File{}
	updateMetadata, err := f.updateMetadata(ctx, updateInfo, metadata, true, true)
	if err != nil {
		return nil, fmt.Errorf("update dir: failed to update metadata from source object: %w", err)
	}
	err = f.pacer.Call(func() (bool, error) {
		info, err = f.svc.Files.Update(dirID, updateInfo).
			Fields(f.getFileFields(ctx)).
			SupportsAllDrives(true).
			Context(ctx).Do()
		return f.shouldRetry(ctx, err)
	})
	if err != nil {
		return nil, err
	}
	err = updateMetadata(ctx, info)
	if err != nil {
		return nil, err
	}
	return info, nil
}

// CreateDir makes a directory with pathID as parent and name leaf
func (f *Fs) CreateDir(ctx context.Context, pathID, leaf string) (newID string, err error) {
	info, err := f.createDir(ctx, pathID, leaf, nil)
	if err != nil {
		return "", err
	}
	return info.Id, nil
}

// isAuthOwned checks if any of the item owners is the authenticated owner
func isAuthOwned(item *drive.File) bool {
	for _, owner := range item.Owners {
		if owner.Me {
			return true
		}
	}
	return false
}

// linkTemplate returns the Template for a MIME type or nil if the
// MIME type does not represent a link
func linkTemplate(mt string) *template.Template {
	templatesOnce.Do(func() {
		_linkTemplates = map[string]*template.Template{
			"application/x-link-desktop": template.Must(
				template.New("application/x-link-desktop").Parse(desktopTemplate)),
			"application/x-link-html": template.Must(
				template.New("application/x-link-html").Parse(htmlTemplate)),
			"application/x-link-url": template.Must(
				template.New("application/x-link-url").Parse(urlTemplate)),
			"application/x-link-webloc": template.Must(
				template.New("application/x-link-webloc").Parse(weblocTemplate)),
		}
	})
	return _linkTemplates[mt]
}

func (f *Fs) fetchFormats(ctx context.Context) {
	fetchFormatsOnce.Do(func() {
		var about *drive.About
		var err error
		err = f.pacer.Call(func() (bool, error) {
			about, err = f.svc.About.Get().
				Fields("exportFormats,importFormats").
				Context(ctx).Do()
			return f.shouldRetry(ctx, err)
		})
		if err != nil {
			fs.Errorf(f, "Failed to get Drive exportFormats and importFormats: %v", err)
			_exportFormats = map[string][]string{}
			_importFormats = map[string][]string{}
			return
		}
		_exportFormats = fixMimeTypeMap(about.ExportFormats)
		_importFormats = fixMimeTypeMap(about.ImportFormats)
	})
}

// exportFormats returns the export formats from drive, fetching them
// if necessary.
//
// if the fetch fails then it will not export any drive formats
func (f *Fs) exportFormats(ctx context.Context) map[string][]string {
	f.fetchFormats(ctx)
	return _exportFormats
}

// importFormats returns the import formats from drive, fetching them
// if necessary.
//
// if the fetch fails then it will not import any drive formats
func (f *Fs) importFormats(ctx context.Context) map[string][]string {
	f.fetchFormats(ctx)
	return _importFormats
}

// findExportFormatByMimeType works out the optimum export settings
// for the given MIME type.
//
// Look through the exportExtensions and find the first format that can be
// converted.  If none found then return ("", "", false)
func (f *Fs) findExportFormatByMimeType(ctx context.Context, itemMimeType string) (
	extension, mimeType string, isDocument bool,
) {
	exportMimeTypes, isDocument := f.exportFormats(ctx)[itemMimeType]
	if isDocument {
		for _, _extension := range f.exportExtensions {
			_mimeType := mime.TypeByExtension(_extension)
			if isLinkMimeType(_mimeType) {
				return _extension, _mimeType, true
			}
			for _, emt := range exportMimeTypes {
				if emt == _mimeType {
					return _extension, emt, true
				}
				if _mimeType == _mimeTypeCustomTransform[emt] {
					return _extension, emt, true
				}
			}
		}
	}

	// If using a link type export and a more specific export
	// hasn't been found all docs should be exported
	for _, _extension := range f.exportExtensions {
		_mimeType := mime.TypeByExtension(_extension)
		if isLinkMimeType(_mimeType) {
			return _extension, _mimeType, true
		}
	}

	// else return empty
	return "", "", isDocument
}

// findExportFormat works out the optimum export settings
// for the given drive.File.
//
// Look through the exportExtensions and find the first format that can be
// converted.  If none found then return ("", "", "", false)
func (f *Fs) findExportFormat(ctx context.Context, item *drive.File) (extension, filename, mimeType string, isDocument bool) {
	// If item has MD5 sum it is a file stored on drive
	if item.Md5Checksum != "" {
		return
	}
	// Folders can't be documents
	if item.MimeType == driveFolderType {
		return
	}
	extension, mimeType, isDocument = f.findExportFormatByMimeType(ctx, item.MimeType)
	if extension != "" {
		filename = item.Name + extension
	}
	return
}

// findImportFormat finds the matching upload MIME type for a file
// If the given MIME type is in importMimeTypes, the matching upload
// MIME type is returned
//
// When no match is found "" is returned.
func (f *Fs) findImportFormat(ctx context.Context, mimeType string) string {
	mimeType = fixMimeType(mimeType)
	ifs := f.importFormats(ctx)
	for _, mt := range f.importMimeTypes {
		if mt == mimeType {
			importMimeTypes := ifs[mimeType]
			if l := len(importMimeTypes); l > 0 {
				if l > 1 {
					fs.Infof(f, "found %d import formats for %q: %q", l, mimeType, importMimeTypes)
				}
				return importMimeTypes[0]
			}
		}
	}
	return ""
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
	directoryID = actualID(directoryID)

	var iErr error
	_, err = f.list(ctx, []string{directoryID}, "", false, false, f.opt.TrashedOnly, false, func(item *drive.File) bool {
		entry, err := f.itemToDirEntry(ctx, path.Join(dir, item.Name), item)
		if err != nil {
			iErr = err
			return true
		}
		if entry != nil {
			entries = append(entries, entry)
		}
		return false
	})
	if err != nil {
		return nil, err
	}
	if iErr != nil {
		return nil, iErr
	}
	// If listing the root of a teamdrive and got no entries,
	// double check we have access
	if f.isTeamDrive && len(entries) == 0 && f.root == "" && dir == "" {
		err = f.teamDriveOK(ctx)
		if err != nil {
			return nil, err
		}
	}
	return entries, nil
}

// listREntry is a task to be executed by a litRRunner
type listREntry struct {
	id, path string
}

// listRSlices is a helper struct to sort two slices at once
type listRSlices struct {
	dirs  []string
	paths []string
}

func (s listRSlices) Sort() {
	sort.Sort(s)
}

func (s listRSlices) Len() int {
	return len(s.dirs)
}

func (s listRSlices) Swap(i, j int) {
	s.dirs[i], s.dirs[j] = s.dirs[j], s.dirs[i]
	s.paths[i], s.paths[j] = s.paths[j], s.paths[i]
}

func (s listRSlices) Less(i, j int) bool {
	return s.dirs[i] < s.dirs[j]
}

// listRRunner will read dirIDs from the in channel, perform the file listing and call cb with each DirEntry.
//
// In each cycle it will read up to grouping entries from the in channel without blocking.
// If an error occurs it will be send to the out channel and then return. Once the in channel is closed,
// nil is send to the out channel and the function returns.
func (f *Fs) listRRunner(ctx context.Context, wg *sync.WaitGroup, in chan listREntry, out chan<- error, cb func(fs.DirEntry) error, sendJob func(listREntry)) {
	var dirs []string
	var paths []string
	var grouping int32

	for dir := range in {
		dirs = append(dirs[:0], dir.id)
		paths = append(paths[:0], dir.path)
		grouping = atomic.LoadInt32(&f.grouping)
	waitloop:
		for i := int32(1); i < grouping; i++ {
			select {
			case d, ok := <-in:
				if !ok {
					break waitloop
				}
				dirs = append(dirs, d.id)
				paths = append(paths, d.path)
			default:
			}
		}
		listRSlices{dirs, paths}.Sort()
		var iErr error
		foundItems := false
		_, err := f.list(ctx, dirs, "", false, false, f.opt.TrashedOnly, false, func(item *drive.File) bool {
			// shared with me items have no parents when at the root
			if f.opt.SharedWithMe && len(item.Parents) == 0 && len(paths) == 1 && paths[0] == "" {
				item.Parents = dirs
			}
			for _, parent := range item.Parents {
				var i int
				foundItems = true
				earlyExit := false
				// If only one item in paths then no need to search for the ID
				// assuming google drive is doing its job properly.
				//
				// Note that we at the root when len(paths) == 1 && paths[0] == ""
				if len(paths) == 1 {
					// don't check parents at root because
					// - shared with me items have no parents at the root
					// - if using a root alias, e.g. "root" or "appDataFolder" the ID won't match
					i = 0
					// items at root can have more than one parent so we need to put
					// the item in just once.
					earlyExit = true
				} else {
					// only handle parents that are in the requested dirs list if not at root
					i = sort.SearchStrings(dirs, parent)
					if i == len(dirs) || dirs[i] != parent {
						continue
					}
				}
				remote := path.Join(paths[i], item.Name)
				entry, err := f.itemToDirEntry(ctx, remote, item)
				if err != nil {
					iErr = err
					return true
				}

				err = cb(entry)
				if err != nil {
					iErr = err
					return true
				}

				// If didn't check parents then insert only once
				if earlyExit {
					break
				}
			}
			return false
		})
		// Found no items in more than one directory. Retry these as
		// individual directories This is to work around a bug in google
		// drive where (A in parents) or (B in parents) returns nothing
		// sometimes. See #3114, #4289 and
		// https://issuetracker.google.com/issues/149522397
		if f.opt.FastListBugFix && len(dirs) > 1 && !foundItems {
			if atomic.SwapInt32(&f.grouping, 1) != 1 {
				fs.Debugf(f, "Disabling ListR to work around bug in drive as multi listing (%d) returned no entries", len(dirs))
			}
			f.listRmu.Lock()
			for i := range dirs {
				// Requeue the jobs
				job := listREntry{id: dirs[i], path: paths[i]}
				sendJob(job)
				// Make a note of these dirs - if they all turn
				// out to be empty then we can re-enable grouping
				f.listRempties[dirs[i]] = struct{}{}
			}
			f.listRmu.Unlock()
			fs.Debugf(f, "Recycled %d entries", len(dirs))
		}
		// If using a grouping of 1 and dir was empty then check to see if it
		// is part of the group that caused grouping to be disabled.
		if grouping == 1 && len(dirs) == 1 && !foundItems {
			f.listRmu.Lock()
			if _, found := f.listRempties[dirs[0]]; found {
				// Remove the ID
				delete(f.listRempties, dirs[0])
				// If no empties left => all the directories that
				// triggered the grouping being set to 1 were actually
				// empty so must have made a mistake
				if len(f.listRempties) == 0 {
					if atomic.SwapInt32(&f.grouping, listRGrouping) != listRGrouping {
						fs.Debugf(f, "Re-enabling ListR as previous detection was in error")
					}
				}
			}
			f.listRmu.Unlock()
		}

		for range dirs {
			wg.Done()
		}

		if iErr != nil {
			out <- iErr
			return
		}

		if err != nil {
			out <- err
			return
		}
	}
	out <- nil
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
// of listing recursively that doing a directory traversal.
func (f *Fs) ListR(ctx context.Context, dir string, callback fs.ListRCallback) (err error) {
	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return err
	}
	directoryID = actualID(directoryID)

	mu := sync.Mutex{} // protects in and overflow
	wg := sync.WaitGroup{}
	in := make(chan listREntry, listRInputBuffer)
	out := make(chan error, f.ci.Checkers)
	list := list.NewHelper(callback)
	overflow := []listREntry{}
	listed := 0

	// Send a job to the input channel if not closed. If the job
	// won't fit then queue it in the overflow slice.
	//
	// This will not block if the channel is full.
	sendJob := func(job listREntry) {
		mu.Lock()
		defer mu.Unlock()
		if in == nil {
			return
		}
		wg.Add(1)
		select {
		case in <- job:
		default:
			overflow = append(overflow, job)
			wg.Done()
		}
	}

	// Send the entry to the caller, queueing any directories as new jobs
	cb := func(entry fs.DirEntry) error {
		if d, isDir := entry.(fs.Directory); isDir {
			job := listREntry{actualID(d.ID()), d.Remote()}
			sendJob(job)
		}
		mu.Lock()
		defer mu.Unlock()
		listed++
		return list.Add(entry)
	}

	wg.Add(1)
	in <- listREntry{directoryID, dir}

	for range f.ci.Checkers {
		go f.listRRunner(ctx, &wg, in, out, cb, sendJob)
	}
	go func() {
		// wait until the all directories are processed
		wg.Wait()
		// if the input channel overflowed add the collected entries to the channel now
		for len(overflow) > 0 {
			mu.Lock()
			// only fill half of the channel to prevent entries being put into overflow again
			l := min(len(overflow), listRInputBuffer/2)
			wg.Add(l)
			for _, d := range overflow[:l] {
				in <- d
			}
			overflow = overflow[l:]
			mu.Unlock()

			// wait again for the completion of all directories
			wg.Wait()
		}
		mu.Lock()
		if in != nil {
			// notify all workers to exit
			close(in)
			in = nil
		}
		mu.Unlock()
	}()
	// wait until the all workers to finish
	for range f.ci.Checkers {
		e := <-out
		mu.Lock()
		// if one worker returns an error early, close the input so all other workers exit
		if e != nil && in != nil {
			err = e
			close(in)
			in = nil
		}
		mu.Unlock()
	}

	close(out)
	if err != nil {
		return err
	}

	err = list.Flush()
	if err != nil {
		return err
	}

	// If listing the root of a teamdrive and got no entries,
	// double check we have access
	if f.isTeamDrive && listed == 0 && f.root == "" && dir == "" {
		err = f.teamDriveOK(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

const shortcutSeparator = '\t'

// joinID adds an actual drive ID to the shortcut ID it came from
//
// directoryIDs in the dircache are these composite directory IDs so
// we must always unpack them before use.
func joinID(actual, shortcut string) string {
	return actual + string(shortcutSeparator) + shortcut
}

// splitID separates an actual ID and a shortcut ID from a composite
// ID. If there was no shortcut ID then it will return "" for it.
func splitID(compositeID string) (actualID, shortcutID string) {
	i := strings.IndexRune(compositeID, shortcutSeparator)
	if i < 0 {
		return compositeID, ""
	}
	return compositeID[:i], compositeID[i+1:]
}

// isShortcutID returns true if compositeID refers to a shortcut
func isShortcutID(compositeID string) bool {
	return strings.ContainsRune(compositeID, shortcutSeparator)
}

// actualID returns an actual ID from a composite ID
func actualID(compositeID string) (actualID string) {
	actualID, _ = splitID(compositeID)
	return actualID
}

// shortcutID returns a shortcut ID from a composite ID if available,
// or the actual ID if not.
func shortcutID(compositeID string) (shortcutID string) {
	actualID, shortcutID := splitID(compositeID)
	if shortcutID != "" {
		return shortcutID
	}
	return actualID
}

// isShortcut returns true of the item is a shortcut
func isShortcut(item *drive.File) bool {
	return item.MimeType == shortcutMimeType && item.ShortcutDetails != nil
}

// Dereference shortcut if required. It returns the newItem (which may
// be just item).
//
// If we return a new item then the ID will be adjusted to be a
// composite of the actual ID and the shortcut ID. This is to make
// sure that we have decided in all use places what we are doing with
// the ID.
//
// Note that we assume shortcuts can't point to shortcuts. Google
// drive web interface doesn't offer the option to create a shortcut
// to a shortcut. The documentation is silent on the issue.
func (f *Fs) resolveShortcut(ctx context.Context, item *drive.File) (newItem *drive.File, err error) {
	if f.opt.SkipShortcuts || item.MimeType != shortcutMimeType {
		return item, nil
	}
	if item.ShortcutDetails == nil {
		fs.Errorf(nil, "Expecting shortcutDetails in %v", item)
		return item, nil
	}
	newItem, err = f.getFile(ctx, item.ShortcutDetails.TargetId, f.getFileFields(ctx))
	if err != nil {
		var gerr *googleapi.Error
		if errors.As(err, &gerr) && gerr.Code == 404 {
			// 404 means dangling shortcut, so just return the shortcut with the mime type mangled
			fs.Logf(nil, "Dangling shortcut %q detected", item.Name)
			item.MimeType = shortcutMimeTypeDangling
			return item, nil
		}
		return nil, fmt.Errorf("failed to resolve shortcut: %w", err)
	}
	// make sure we use the Name, Parents and Trashed from the original item
	newItem.Name = item.Name
	newItem.Parents = item.Parents
	newItem.Trashed = item.Trashed
	// the new ID is a composite ID
	newItem.Id = joinID(newItem.Id, item.Id)
	return newItem, nil
}

// itemToDirEntry converts a drive.File to an fs.DirEntry.
// When the drive.File cannot be represented as an fs.DirEntry
// (nil, nil) is returned.
func (f *Fs) itemToDirEntry(ctx context.Context, remote string, item *drive.File) (entry fs.DirEntry, err error) {
	switch {
	case item.MimeType == driveFolderType:
		// cache the directory ID for later lookups
		f.dirCache.Put(remote, item.Id)
		// cache the resource key for later lookups
		if item.ResourceKey != "" {
			f.dirResourceKeys.Store(item.Id, item.ResourceKey)
		}
		baseObject, err := f.newBaseObject(ctx, remote, item)
		if err != nil {
			return nil, err
		}
		d := &Directory{baseObject: baseObject}
		return d, nil
	case f.opt.AuthOwnerOnly && !isAuthOwned(item):
		// ignore object
	default:
		entry, err = f.newObjectWithInfo(ctx, remote, item)
		if err == fs.ErrorObjectNotFound {
			return nil, nil
		}
		return entry, err
	}
	return nil, nil
}

// Creates a drive.File info from the parameters passed in.
//
// Used to create new objects
func (f *Fs) createFileInfo(ctx context.Context, remote string, modTime time.Time) (*drive.File, error) {
	leaf, directoryID, err := f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return nil, err
	}
	directoryID = actualID(directoryID)

	leaf = f.opt.Enc.FromStandardName(leaf)
	// Define the metadata for the file we are going to create.
	createInfo := &drive.File{
		Name:         leaf,
		Parents:      []string{directoryID},
		ModifiedTime: modTime.Format(timeFormatOut),
	}
	return createInfo, nil
}

// Put the object
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

// PutUnchecked uploads the object
//
// This will create a duplicate if we upload a new file without
// checking to see if there is one already - use Put() for that.
func (f *Fs) PutUnchecked(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	remote := src.Remote()
	size := src.Size()
	modTime := src.ModTime(ctx)
	srcMimeType := fs.MimeTypeFromName(remote)
	srcExt := path.Ext(remote)
	exportExt := ""
	importMimeType := ""

	if f.importMimeTypes != nil && !f.opt.SkipGdocs {
		importMimeType = f.findImportFormat(ctx, srcMimeType)

		if isInternalMimeType(importMimeType) {
			remote = remote[:len(remote)-len(srcExt)]

			exportExt, _, _ = f.findExportFormatByMimeType(ctx, importMimeType)
			if exportExt == "" {
				return nil, fmt.Errorf("no export format found for %q", importMimeType)
			}
			if exportExt != srcExt && !f.opt.AllowImportNameChange {
				return nil, fmt.Errorf("can't convert %q to a document with a different export filetype (%q)", srcExt, exportExt)
			}
		}
	}

	createInfo, err := f.createFileInfo(ctx, remote, modTime)
	if err != nil {
		return nil, err
	}
	if importMimeType != "" {
		createInfo.MimeType = importMimeType
	} else {
		createInfo.MimeType = fs.MimeTypeFromName(remote)
	}
	updateMetadata, err := f.fetchAndUpdateMetadata(ctx, src, options, createInfo, false)
	if err != nil {
		return nil, err
	}

	var info *drive.File
	if size >= 0 && size < int64(f.opt.UploadCutoff) {
		// Make the API request to upload metadata and file data.
		// Don't retry, return a retry error instead
		err = f.pacer.CallNoRetry(func() (bool, error) {
			info, err = f.svc.Files.Create(createInfo).
				Media(in, googleapi.ContentType(srcMimeType), googleapi.ChunkSize(0)).
				Fields(partialFields).
				SupportsAllDrives(true).
				KeepRevisionForever(f.opt.KeepRevisionForever).
				Context(ctx).Do()
			return f.shouldRetry(ctx, err)
		})
		if err != nil {
			return nil, err
		}
	} else {
		// Upload the file in chunks
		info, err = f.Upload(ctx, in, size, srcMimeType, "", remote, createInfo)
		if err != nil {
			return nil, err
		}
	}
	err = updateMetadata(ctx, info)
	if err != nil {
		return nil, err
	}
	return f.newObjectWithInfo(ctx, remote, info)
}

// MergeDirs merges the contents of all the directories passed
// in into the first one and rmdirs the other directories.
func (f *Fs) MergeDirs(ctx context.Context, dirs []fs.Directory) error {
	if len(dirs) < 2 {
		return nil
	}
	newDirs := dirs[:0]
	for _, dir := range dirs {
		if isShortcutID(dir.ID()) {
			fs.Infof(dir, "skipping shortcut directory")
			continue
		}
		newDirs = append(newDirs, dir)
	}
	dirs = newDirs
	if len(dirs) < 2 {
		return nil
	}
	dstDir := dirs[0]
	for _, srcDir := range dirs[1:] {
		// list the objects
		infos := []*drive.File{}
		_, err := f.list(ctx, []string{srcDir.ID()}, "", false, false, f.opt.TrashedOnly, true, func(info *drive.File) bool {
			infos = append(infos, info)
			return false
		})
		if err != nil {
			return fmt.Errorf("MergeDirs list failed on %v: %w", srcDir, err)
		}
		// move them into place
		for _, info := range infos {
			fs.Infof(srcDir, "merging %q", info.Name)
			// Move the file into the destination
			err = f.pacer.Call(func() (bool, error) {
				_, err = f.svc.Files.Update(info.Id, nil).
					RemoveParents(srcDir.ID()).
					AddParents(dstDir.ID()).
					Fields("").
					SupportsAllDrives(true).
					Context(ctx).Do()
				return f.shouldRetry(ctx, err)
			})
			if err != nil {
				return fmt.Errorf("MergeDirs move failed on %q in %v: %w", info.Name, srcDir, err)
			}
		}
		// rmdir (into trash) the now empty source directory
		fs.Infof(srcDir, "removing empty directory")
		err = f.delete(ctx, srcDir.ID(), true)
		if err != nil {
			return fmt.Errorf("MergeDirs move failed to rmdir %q: %w", srcDir, err)
		}
	}
	return nil
}

// Mkdir creates the container if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	_, err := f.dirCache.FindDir(ctx, dir, true)
	return err
}

// MkdirMetadata makes the directory passed in as dir.
//
// It shouldn't return an error if it already exists.
//
// If the metadata is not nil it is set.
//
// It returns the directory that was created.
func (f *Fs) MkdirMetadata(ctx context.Context, dir string, metadata fs.Metadata) (fs.Directory, error) {
	var info *drive.File
	dirID, err := f.dirCache.FindDir(ctx, dir, false)
	if err == fs.ErrorDirNotFound {
		// Directory does not exist so create it
		var leaf, parentID string
		leaf, parentID, err = f.dirCache.FindPath(ctx, dir, true)
		if err != nil {
			return nil, err
		}
		info, err = f.createDir(ctx, parentID, leaf, metadata)
	} else if err == nil {
		// Directory exists and needs updating
		info, err = f.updateDir(ctx, dirID, metadata)
	}
	if err != nil {
		return nil, err
	}

	// Convert the info into a directory entry
	entry, err := f.itemToDirEntry(ctx, dir, info)
	if err != nil {
		return nil, err
	}
	dirEntry, ok := entry.(fs.Directory)
	if !ok {
		return nil, fmt.Errorf("internal error: expecting %T to be an fs.Directory", entry)
	}

	return dirEntry, nil
}

// DirSetModTime sets the directory modtime for dir
func (f *Fs) DirSetModTime(ctx context.Context, dir string, modTime time.Time) error {
	dirID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return err
	}
	o := baseObject{
		fs:     f,
		remote: dir,
		id:     dirID,
	}
	return o.SetModTime(ctx, modTime)
}

// delete a file or directory unconditionally by ID
func (f *Fs) delete(ctx context.Context, id string, useTrash bool) error {
	return f.pacer.Call(func() (bool, error) {
		var err error
		if useTrash {
			info := drive.File{
				Trashed: true,
			}
			_, err = f.svc.Files.Update(id, &info).
				Fields("").
				SupportsAllDrives(true).
				Context(ctx).Do()
		} else {
			err = f.svc.Files.Delete(id).
				Fields("").
				SupportsAllDrives(true).
				Context(ctx).Do()
		}
		return f.shouldRetry(ctx, err)
	})
}

// purgeCheck removes the dir directory, if check is set then it
// refuses to do so if it has anything in
func (f *Fs) purgeCheck(ctx context.Context, dir string, check bool) error {
	root := path.Join(f.root, dir)
	dc := f.dirCache
	directoryID, err := dc.FindDir(ctx, dir, false)
	if err != nil {
		return err
	}
	directoryID, shortcutID := splitID(directoryID)
	// if directory is a shortcut remove it regardless
	if shortcutID != "" {
		return f.delete(ctx, shortcutID, f.opt.UseTrash)
	}
	trashedFiles := false
	if check {
		found, err := f.list(ctx, []string{directoryID}, "", false, false, f.opt.TrashedOnly, true, func(item *drive.File) bool {
			if !item.Trashed {
				fs.Debugf(dir, "Rmdir: contains file: %q", item.Name)
				return true
			}
			fs.Debugf(dir, "Rmdir: contains trashed file: %q", item.Name)
			trashedFiles = true
			return false
		})
		if err != nil {
			return err
		}
		if found {
			return fmt.Errorf("directory not empty")
		}
	}
	if root != "" {
		// trash the directory if it had trashed files
		// in or the user wants to trash, otherwise
		// delete it.
		err = f.delete(ctx, directoryID, trashedFiles || f.opt.UseTrash)
		if err != nil {
			return err
		}
	} else if check {
		return errors.New("can't purge root directory")
	}
	f.dirCache.FlushDir(dir)
	if err != nil {
		return err
	}
	return nil
}

// Rmdir deletes a directory
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir, true)
}

// Precision of the object storage system
func (f *Fs) Precision() time.Duration {
	return time.Millisecond
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
	var srcObj *baseObject
	ext := ""
	isDoc := false
	switch src := src.(type) {
	case *Object:
		srcObj = &src.baseObject
	case *documentObject:
		srcObj, ext = &src.baseObject, src.ext()
		isDoc = true
	case *linkObject:
		srcObj, ext = &src.baseObject, src.ext()
	default:
		fs.Debugf(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}

	// Look to see if there is an existing object before we remove
	// the extension from the remote
	existingObject, _ := f.NewObject(ctx, remote)

	// Adjust the remote name to be without the extension if we
	// are about to create a doc.
	if ext != "" {
		if !strings.HasSuffix(remote, ext) {
			fs.Debugf(src, "Can't copy - not same document type")
			return nil, fs.ErrorCantCopy
		}
		remote = remote[:len(remote)-len(ext)]
	}

	createInfo, err := f.createFileInfo(ctx, remote, src.ModTime(ctx))
	if err != nil {
		return nil, err
	}

	if isDoc {
		// preserve the description on copy for docs
		info, err := f.getFile(ctx, actualID(srcObj.id), "description")
		if err != nil {
			fs.Errorf(srcObj, "Failed to read description for Google Doc: %v", err)
		} else {
			createInfo.Description = info.Description
		}
	} else {
		// don't overwrite the description on copy for files
		// this should work for docs but it doesn't - it is probably a bug in Google Drive
		createInfo.Description = ""
	}

	// Adjust metadata if required
	updateMetadata, err := f.fetchAndUpdateMetadata(ctx, src, fs.MetadataAsOpenOptions(ctx), createInfo, false)
	if err != nil {
		return nil, err
	}

	// get the ID of the thing to copy
	// copy the contents if CopyShortcutContent
	// else copy the shortcut only

	id := shortcutID(srcObj.id)

	if f.opt.CopyShortcutContent {
		id = actualID(srcObj.id)
	}

	var info *drive.File
	err = f.pacer.Call(func() (bool, error) {
		copy := f.svc.Files.Copy(id, createInfo).
			Fields(f.getFileFields(ctx)).
			SupportsAllDrives(true).
			KeepRevisionForever(f.opt.KeepRevisionForever)
		srcObj.addResourceKey(copy.Header())
		info, err = copy.Context(ctx).Do()
		return f.shouldRetry(ctx, err)
	})
	if err != nil {
		return nil, err
	}
	newObject, err := f.newObjectWithInfo(ctx, remote, info)
	if err != nil {
		return nil, err
	}
	// Google docs aren't preserving their mod time after copy, so set them explicitly
	// See: https://github.com/rclone/rclone/issues/4517
	//
	// FIXME remove this when google fixes the problem!
	if isDoc {
		// A short sleep is needed here in order to make the
		// change effective, without it is ignored. This is
		// probably some eventual consistency nastiness.
		sleepTime := 2 * time.Second
		fs.Debugf(f, "Sleeping for %v before setting the modtime to work around drive bug - see #4517", sleepTime)
		time.Sleep(sleepTime)
		err = newObject.SetModTime(ctx, src.ModTime(ctx))
		if err != nil {
			return nil, err
		}
	}
	if existingObject != nil {
		err = existingObject.Remove(ctx)
		if err != nil {
			fs.Errorf(existingObject, "Failed to remove existing object after copy: %v", err)
		}
	}
	// Finalise metadata
	err = updateMetadata(ctx, info)
	if err != nil {
		return nil, err
	}
	return newObject, nil
}

// Purge deletes all the files and the container
//
// Optional interface: Only implement this if you have a way of
// deleting all the files quicker than just running Remove() on the
// result of List()
func (f *Fs) Purge(ctx context.Context, dir string) error {
	if f.opt.TrashedOnly {
		return errors.New("can't purge with --drive-trashed-only, use delete if you want to selectively delete files")
	}
	return f.purgeCheck(ctx, dir, false)
}

type cleanupResult struct {
	Errors int
}

func (r cleanupResult) Error() string {
	return fmt.Sprintf("%d errors during cleanup - see log", r.Errors)
}

func (f *Fs) cleanupTeamDrive(ctx context.Context, dir string, directoryID string) (r cleanupResult, err error) {
	_, err = f.list(ctx, []string{directoryID}, "", false, false, true, false, func(item *drive.File) bool {
		remote := path.Join(dir, item.Name)
		if item.ExplicitlyTrashed { // description is wrong - can also be set for folders - no need to recurse them
			err := f.delete(ctx, item.Id, false)
			if err != nil {
				r.Errors++
				fs.Errorf(remote, "%v", err)
			}
			return false
		}

		if item.MimeType == driveFolderType {
			if !isShortcutID(item.Id) {
				rNew, _ := f.cleanupTeamDrive(ctx, remote, item.Id)
				r.Errors += rNew.Errors
			}
			return false
		}
		return false
	})
	if err != nil {
		err = fmt.Errorf("failed to list directory: %w", err)
		r.Errors++
		fs.Errorf(dir, "%v", err)
	}
	if r.Errors != 0 {
		return r, r
	}
	return r, nil
}

// CleanUp empties the trash
func (f *Fs) CleanUp(ctx context.Context) error {
	if f.isTeamDrive {
		directoryID, err := f.dirCache.FindDir(ctx, "", false)
		if err != nil {
			return err
		}
		directoryID = actualID(directoryID)
		_, err = f.cleanupTeamDrive(ctx, "", directoryID)
		return err
	}
	err := f.pacer.Call(func() (bool, error) {
		err := f.svc.Files.EmptyTrash().Context(ctx).Do()
		return f.shouldRetry(ctx, err)
	})
	if err != nil {
		return err
	}
	fs.Logf(f, "Note that emptying the trash happens in the background and can take some time.")
	return nil
}

// teamDriveOK checks to see if we can access the team drive
func (f *Fs) teamDriveOK(ctx context.Context) (err error) {
	if !f.isTeamDrive {
		return nil
	}
	var td *drive.Drive
	err = f.pacer.Call(func() (bool, error) {
		td, err = f.svc.Drives.Get(f.opt.TeamDriveID).Fields("name,id,capabilities,createdTime,restrictions").Context(ctx).Do()
		return f.shouldRetry(ctx, err)
	})
	if err != nil {
		return fmt.Errorf("failed to get Shared Drive info: %w", err)
	}
	fs.Debugf(f, "read info from Shared Drive %q", td.Name)
	return err
}

// About gets quota information
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	if f.isTeamDrive {
		err := f.teamDriveOK(ctx)
		if err != nil {
			return nil, err
		}
		// Teamdrives don't appear to have a usage API so just return empty
		return &fs.Usage{}, nil
	}
	var about *drive.About
	var err error
	err = f.pacer.Call(func() (bool, error) {
		about, err = f.svc.About.Get().Fields("storageQuota").Context(ctx).Do()
		return f.shouldRetry(ctx, err)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get Drive storageQuota: %w", err)
	}
	q := about.StorageQuota
	usage := &fs.Usage{
		Used:    fs.NewUsageValue(q.UsageInDrive),           // bytes in use
		Trashed: fs.NewUsageValue(q.UsageInDriveTrash),      // bytes in trash
		Other:   fs.NewUsageValue(q.Usage - q.UsageInDrive), // other usage e.g. gmail in drive
	}
	if q.Limit > 0 {
		usage.Total = fs.NewUsageValue(q.Limit)          // quota of bytes that can be used
		usage.Free = fs.NewUsageValue(q.Limit - q.Usage) // bytes which can be uploaded before reaching the quota
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
	var srcObj *baseObject
	ext := ""
	switch src := src.(type) {
	case *Object:
		srcObj = &src.baseObject
	case *documentObject:
		srcObj, ext = &src.baseObject, src.ext()
	case *linkObject:
		srcObj, ext = &src.baseObject, src.ext()
	default:
		fs.Debugf(src, "Can't move - not same remote type")
		return nil, fs.ErrorCantMove
	}

	if ext != "" {
		if !strings.HasSuffix(remote, ext) {
			fs.Debugf(src, "Can't move - not same document type")
			return nil, fs.ErrorCantMove
		}
		remote = remote[:len(remote)-len(ext)]
	}

	_, srcParentID, err := srcObj.fs.dirCache.FindPath(ctx, src.Remote(), false)
	if err != nil {
		return nil, err
	}
	srcParentID = actualID(srcParentID)

	// Temporary Object under construction
	dstInfo, err := f.createFileInfo(ctx, remote, src.ModTime(ctx))
	if err != nil {
		return nil, err
	}
	dstParents := strings.Join(dstInfo.Parents, ",")
	dstInfo.Parents = nil

	// Adjust metadata if required
	updateMetadata, err := f.fetchAndUpdateMetadata(ctx, src, fs.MetadataAsOpenOptions(ctx), dstInfo, true)
	if err != nil {
		return nil, err
	}

	// Do the move
	var info *drive.File
	err = f.pacer.Call(func() (bool, error) {
		info, err = f.svc.Files.Update(shortcutID(srcObj.id), dstInfo).
			RemoveParents(srcParentID).
			AddParents(dstParents).
			Fields(f.getFileFields(ctx)).
			SupportsAllDrives(true).
			Context(ctx).Do()
		return f.shouldRetry(ctx, err)
	})
	if err != nil {
		return nil, err
	}

	// Finalise metadata
	err = updateMetadata(ctx, info)
	if err != nil {
		return nil, err
	}
	return f.newObjectWithInfo(ctx, remote, info)
}

// PublicLink adds a "readable by anyone with link" permission on the given file or folder.
func (f *Fs) PublicLink(ctx context.Context, remote string, expire fs.Duration, unlink bool) (link string, err error) {
	id, err := f.dirCache.FindDir(ctx, remote, false)
	if err == nil {
		fs.Debugf(f, "attempting to share directory '%s'", remote)
		id = shortcutID(id)
	} else {
		fs.Debugf(f, "attempting to share single file '%s'", remote)
		o, err := f.NewObject(ctx, remote)
		if err != nil {
			return "", err
		}
		id = shortcutID(o.(fs.IDer).ID())
	}

	permission := &drive.Permission{
		AllowFileDiscovery: false,
		Role:               "reader",
		Type:               "anyone",
	}

	err = f.pacer.Call(func() (bool, error) {
		// TODO: On TeamDrives this might fail if lacking permissions to change ACLs.
		// Need to either check `canShare` attribute on the object or see if a sufficient permission is already present.
		_, err = f.svc.Permissions.Create(id, permission).
			Fields("").
			SupportsAllDrives(true).
			Context(ctx).Do()
		return f.shouldRetry(ctx, err)
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("https://drive.google.com/open?id=%s", id), nil
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

	srcID, srcDirectoryID, srcLeaf, dstDirectoryID, dstLeaf, err := f.dirCache.DirMove(ctx, srcFs.dirCache, srcFs.root, srcRemote, f.root, dstRemote)
	if err != nil {
		return err
	}
	_ = srcLeaf

	dstDirectoryID = actualID(dstDirectoryID)
	srcDirectoryID = actualID(srcDirectoryID)

	// Do the move
	patch := drive.File{
		Name: dstLeaf,
	}
	err = f.pacer.Call(func() (bool, error) {
		_, err = f.svc.Files.Update(shortcutID(srcID), &patch).
			RemoveParents(srcDirectoryID).
			AddParents(dstDirectoryID).
			Fields("").
			SupportsAllDrives(true).
			Context(ctx).Do()
		return f.shouldRetry(ctx, err)
	})
	if err != nil {
		return err
	}
	srcFs.dirCache.FlushDir(srcRemote)
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
		// get the StartPageToken early so all changes from now on get processed
		startPageToken, err := f.changeNotifyStartPageToken(ctx)
		if err != nil {
			fs.Infof(f, "Failed to get StartPageToken: %s", err)
		}
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
				if startPageToken == "" {
					startPageToken, err = f.changeNotifyStartPageToken(ctx)
					if err != nil {
						fs.Infof(f, "Failed to get StartPageToken: %s", err)
						continue
					}
				}
				fs.Debugf(f, "Checking for changes on remote")
				startPageToken, err = f.changeNotifyRunner(ctx, notifyFunc, startPageToken)
				if err != nil {
					fs.Infof(f, "Change notify listener failure: %s", err)
				}
			}
		}
	}()
}

func (f *Fs) changeNotifyStartPageToken(ctx context.Context) (pageToken string, err error) {
	var startPageToken *drive.StartPageToken
	err = f.pacer.Call(func() (bool, error) {
		changes := f.svc.Changes.GetStartPageToken().SupportsAllDrives(true)
		if f.isTeamDrive {
			changes.DriveId(f.opt.TeamDriveID)
		}
		startPageToken, err = changes.Context(ctx).Do()
		return f.shouldRetry(ctx, err)
	})
	if err != nil {
		return
	}
	return startPageToken.StartPageToken, nil
}

func (f *Fs) changeNotifyRunner(ctx context.Context, notifyFunc func(string, fs.EntryType), startPageToken string) (newStartPageToken string, err error) {
	pageToken := startPageToken
	for {
		var changeList *drive.ChangeList

		err = f.pacer.Call(func() (bool, error) {
			changesCall := f.svc.Changes.List(pageToken).
				Fields("nextPageToken,newStartPageToken,changes(fileId,file(name,parents,mimeType))")
			if f.opt.ListChunk > 0 {
				changesCall.PageSize(f.opt.ListChunk)
			}
			changesCall.SupportsAllDrives(true)
			changesCall.IncludeItemsFromAllDrives(true)
			if f.isTeamDrive {
				changesCall.DriveId(f.opt.TeamDriveID)
			}
			// If using appDataFolder then need to add Spaces
			if f.rootFolderID == "appDataFolder" {
				changesCall.Spaces("appDataFolder")
			}
			changesCall.RestrictToMyDrive(!f.opt.SharedWithMe)
			changeList, err = changesCall.Context(ctx).Do()
			return f.shouldRetry(ctx, err)
		})
		if err != nil {
			return
		}

		type entryType struct {
			path      string
			entryType fs.EntryType
		}
		var pathsToClear []entryType
		for _, change := range changeList.Changes {
			// find the previous path
			if path, ok := f.dirCache.GetInv(change.FileId); ok {
				if change.File != nil && change.File.MimeType != driveFolderType {
					pathsToClear = append(pathsToClear, entryType{path: path, entryType: fs.EntryObject})
				} else {
					pathsToClear = append(pathsToClear, entryType{path: path, entryType: fs.EntryDirectory})
				}
			}

			// find the new path
			if change.File != nil {
				change.File.Name = f.opt.Enc.ToStandardName(change.File.Name)
				changeType := fs.EntryDirectory
				if change.File.MimeType != driveFolderType {
					changeType = fs.EntryObject
				}

				// translate the parent dir of this object
				if len(change.File.Parents) > 0 {
					for _, parent := range change.File.Parents {
						if parentPath, ok := f.dirCache.GetInv(parent); ok {
							// and append the drive file name to compute the full file name
							newPath := path.Join(parentPath, change.File.Name)
							// this will now clear the actual file too
							pathsToClear = append(pathsToClear, entryType{path: newPath, entryType: changeType})
						}
					}
				} else { // a true root object that is changed
					pathsToClear = append(pathsToClear, entryType{path: change.File.Name, entryType: changeType})
				}
			}
		}

		visitedPaths := make(map[string]struct{})
		for _, entry := range pathsToClear {
			if _, ok := visitedPaths[entry.path]; ok {
				continue
			}
			visitedPaths[entry.path] = struct{}{}
			notifyFunc(entry.path, entry.entryType)
		}

		switch {
		case changeList.NewStartPageToken != "":
			return changeList.NewStartPageToken, nil
		case changeList.NextPageToken != "":
			pageToken = changeList.NextPageToken
		default:
			return
		}
	}
}

// DirCacheFlush resets the directory cache - used in testing as an
// optional interface
func (f *Fs) DirCacheFlush() {
	f.dirCache.ResetRoot()
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.NewHashSet(hash.MD5, hash.SHA1, hash.SHA256)
}

func (f *Fs) changeChunkSize(chunkSizeString string) (err error) {
	chunkSizeInt, err := strconv.ParseInt(chunkSizeString, 10, 64)
	if err != nil {
		return fmt.Errorf("couldn't convert chunk size to int: %w", err)
	}
	chunkSize := fs.SizeSuffix(chunkSizeInt)
	if chunkSize == f.opt.ChunkSize {
		return nil
	}
	err = checkUploadChunkSize(chunkSize)
	if err == nil {
		f.opt.ChunkSize = chunkSize
	}
	return err
}

func (f *Fs) changeServiceAccountFile(ctx context.Context, file string) (err error) {
	fs.Debugf(nil, "Changing Service Account File from %s to %s", f.opt.ServiceAccountFile, file)
	if file == f.opt.ServiceAccountFile {
		return nil
	}
	oldSvc := f.svc
	oldv2Svc := f.v2Svc
	oldOAuthClient := f.client
	oldFile := f.opt.ServiceAccountFile
	oldCredentials := f.opt.ServiceAccountCredentials
	defer func() {
		// Undo all the changes instead of doing selective undo's
		if err != nil {
			f.svc = oldSvc
			f.v2Svc = oldv2Svc
			f.client = oldOAuthClient
			f.opt.ServiceAccountFile = oldFile
			f.opt.ServiceAccountCredentials = oldCredentials
		}
	}()
	f.opt.ServiceAccountFile = file
	f.opt.ServiceAccountCredentials = ""
	oAuthClient, err := createOAuthClient(ctx, &f.opt, f.name, f.m)
	if err != nil {
		return fmt.Errorf("drive: failed when making oauth client: %w", err)
	}
	f.client = oAuthClient
	f.svc, err = drive.NewService(context.Background(), option.WithHTTPClient(f.client))
	if err != nil {
		return fmt.Errorf("couldn't create Drive client: %w", err)
	}
	if f.opt.V2DownloadMinSize >= 0 {
		f.v2Svc, err = drive_v2.NewService(context.Background(), option.WithHTTPClient(f.client))
		if err != nil {
			return fmt.Errorf("couldn't create Drive v2 client: %w", err)
		}
	}
	return nil
}

// Create a shortcut from (f, srcPath) to (dstFs, dstPath)
//
// Will not overwrite existing files
func (f *Fs) makeShortcut(ctx context.Context, srcPath string, dstFs *Fs, dstPath string) (o fs.Object, err error) {
	srcFs := f
	srcPath = strings.Trim(srcPath, "/")
	dstPath = strings.Trim(dstPath, "/")
	if dstPath == "" {
		return nil, errors.New("shortcut destination can't be root directory")
	}

	// Find source
	var srcID string
	isDir := false
	if srcPath == "" {
		// source is root directory
		srcID, err = f.dirCache.RootID(ctx, false)
		if err != nil {
			return nil, err
		}
		isDir = true
	} else if srcObj, err := srcFs.NewObject(ctx, srcPath); err != nil {
		if err != fs.ErrorIsDir {
			return nil, fmt.Errorf("can't find source: %w", err)
		}
		// source was a directory
		srcID, err = srcFs.dirCache.FindDir(ctx, srcPath, false)
		if err != nil {
			return nil, fmt.Errorf("failed to find source dir: %w", err)
		}
		isDir = true
	} else {
		// source was a file
		srcID = srcObj.(*Object).id
	}
	srcID = actualID(srcID) // link to underlying object not to shortcut

	// Find destination
	_, err = dstFs.NewObject(ctx, dstPath)
	if err != fs.ErrorObjectNotFound {
		if err == nil {
			err = errors.New("existing file")
		} else if err == fs.ErrorIsDir {
			err = errors.New("existing directory")
		}
		return nil, fmt.Errorf("not overwriting shortcut target: %w", err)
	}

	// Create destination shortcut
	createInfo, err := dstFs.createFileInfo(ctx, dstPath, time.Now())
	if err != nil {
		return nil, fmt.Errorf("shortcut destination failed: %w", err)
	}
	createInfo.MimeType = shortcutMimeType
	createInfo.ShortcutDetails = &drive.FileShortcutDetails{
		TargetId: srcID,
	}

	var info *drive.File
	err = dstFs.pacer.Call(func() (bool, error) {
		info, err = dstFs.svc.Files.Create(createInfo).
			Fields(partialFields).
			SupportsAllDrives(true).
			KeepRevisionForever(dstFs.opt.KeepRevisionForever).
			Context(ctx).Do()
		return dstFs.shouldRetry(ctx, err)
	})
	if err != nil {
		return nil, fmt.Errorf("shortcut creation failed: %w", err)
	}
	if isDir {
		return nil, nil
	}
	return dstFs.newObjectWithInfo(ctx, dstPath, info)
}

// List all team drives
func (f *Fs) listTeamDrives(ctx context.Context) (drives []*drive.Drive, err error) {
	drives = []*drive.Drive{}
	listTeamDrives := f.svc.Drives.List().PageSize(100)
	var defaultFs Fs // default Fs with default Options
	for {
		var teamDrives *drive.DriveList
		err = f.pacer.Call(func() (bool, error) {
			teamDrives, err = listTeamDrives.Context(ctx).Do()
			return defaultFs.shouldRetry(ctx, err)
		})
		if err != nil {
			return drives, fmt.Errorf("listing Team Drives failed: %w", err)
		}
		drives = append(drives, teamDrives.Drives...)
		if teamDrives.NextPageToken == "" {
			break
		}
		listTeamDrives.PageToken(teamDrives.NextPageToken)
	}
	return drives, nil
}

type unTrashResult struct {
	Untrashed int
	Errors    int
}

func (r unTrashResult) Error() string {
	return fmt.Sprintf("%d errors while untrashing - see log", r.Errors)
}

// Restore the trashed files from dir, directoryID recursing if needed
func (f *Fs) unTrash(ctx context.Context, dir string, directoryID string, recurse bool) (r unTrashResult, err error) {
	directoryID = actualID(directoryID)
	fs.Debugf(dir, "finding trash to restore in directory %q", directoryID)
	_, err = f.list(ctx, []string{directoryID}, "", false, false, f.opt.TrashedOnly, true, func(item *drive.File) bool {
		remote := path.Join(dir, item.Name)
		if item.ExplicitlyTrashed {
			fs.Infof(remote, "restoring %q", item.Id)
			if operations.SkipDestructive(ctx, remote, "restore") {
				return false
			}
			update := drive.File{
				ForceSendFields: []string{"Trashed"}, // necessary to set false value
				Trashed:         false,
			}
			err := f.pacer.Call(func() (bool, error) {
				_, err := f.svc.Files.Update(item.Id, &update).
					SupportsAllDrives(true).
					Fields("trashed").
					Context(ctx).Do()
				return f.shouldRetry(ctx, err)
			})
			if err != nil {
				err = fmt.Errorf("failed to restore: %w", err)
				r.Errors++
				fs.Errorf(remote, "%v", err)
			} else {
				r.Untrashed++
			}
		}
		if recurse && item.MimeType == "application/vnd.google-apps.folder" {
			if !isShortcutID(item.Id) {
				rNew, _ := f.unTrash(ctx, remote, item.Id, recurse)
				r.Untrashed += rNew.Untrashed
				r.Errors += rNew.Errors
			}
		}
		return false
	})
	if err != nil {
		err = fmt.Errorf("failed to list directory: %w", err)
		r.Errors++
		fs.Errorf(dir, "%v", err)
	}
	if r.Errors != 0 {
		return r, r
	}
	return r, nil
}

// Untrash dir
func (f *Fs) unTrashDir(ctx context.Context, dir string, recurse bool) (r unTrashResult, err error) {
	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		r.Errors++
		return r, err
	}
	return f.unTrash(ctx, dir, directoryID, true)
}

// copy or move file with id to dest
func (f *Fs) copyOrMoveID(ctx context.Context, operation string, id, dest string) (err error) {
	info, err := f.getFile(ctx, id, f.getFileFields(ctx))
	if err != nil {
		return fmt.Errorf("couldn't find id: %w", err)
	}
	if info.MimeType == driveFolderType {
		return fmt.Errorf("can't %s directory use: rclone %s --drive-root-folder-id %s %s %s", operation, operation, id, fs.ConfigString(f), dest)
	}
	info.Name = f.opt.Enc.ToStandardName(info.Name)
	o, err := f.newObjectWithInfo(ctx, info.Name, info)
	if err != nil {
		return err
	}
	destDir, destLeaf, err := fspath.Split(dest)
	if err != nil {
		return err
	}
	if destLeaf == "" {
		destLeaf = path.Base(o.Remote())
	}
	if destDir == "" {
		destDir = "."
	}
	dstFs, err := cache.Get(ctx, destDir)
	if err != nil {
		return err
	}

	var opErr error
	if operation == "moveid" {
		_, opErr = operations.Move(ctx, dstFs, nil, destLeaf, o)
	} else {
		_, opErr = operations.Copy(ctx, dstFs, nil, destLeaf, o)
	}
	if opErr != nil {
		return fmt.Errorf("%s failed: %w", operation, opErr)
	}
	return nil
}

// Run the drive query calling fn on each entry found
func (f *Fs) queryFn(ctx context.Context, query string, fn func(*drive.File)) (err error) {
	list := f.svc.Files.List()
	if query != "" {
		list.Q(query)
	}

	if f.opt.ListChunk > 0 {
		list.PageSize(f.opt.ListChunk)
	}
	list.SupportsAllDrives(true)
	list.IncludeItemsFromAllDrives(true)
	if f.isTeamDrive && !f.opt.SharedWithMe {
		list.DriveId(f.opt.TeamDriveID)
		list.Corpora("drive")
	}
	// If using appDataFolder then need to add Spaces
	if f.rootFolderID == "appDataFolder" {
		list.Spaces("appDataFolder")
	}
	fields := fmt.Sprintf("files(%s),nextPageToken,incompleteSearch", f.getFileFields(ctx))
	for {
		var files *drive.FileList
		err = f.pacer.Call(func() (bool, error) {
			files, err = list.Fields(googleapi.Field(fields)).Context(ctx).Do()
			return f.shouldRetry(ctx, err)
		})
		if err != nil {
			return fmt.Errorf("failed to execute query: %w", err)
		}
		if files.IncompleteSearch {
			fs.Errorf(f, "search result INCOMPLETE")
		}
		for _, item := range files.Files {
			fn(item)
		}
		if files.NextPageToken == "" {
			break
		}
		list.PageToken(files.NextPageToken)
	}
	return nil
}

// Run the drive query returning the entries found
func (f *Fs) query(ctx context.Context, query string) (entries []*drive.File, err error) {
	var results []*drive.File
	err = f.queryFn(ctx, query, func(item *drive.File) {
		results = append(results, item)
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}

// Rescue, list or delete orphaned files
func (f *Fs) rescue(ctx context.Context, dirID string, delete bool) (err error) {
	return f.queryFn(ctx, "'me' in owners and trashed=false", func(item *drive.File) {
		if len(item.Parents) != 0 {
			return
		}
		// Have found an orphaned entry
		if delete {
			fs.Infof(item.Name, "Deleting orphan %q into trash", item.Id)
			err = f.delete(ctx, item.Id, true)
			if err != nil {
				fs.Errorf(item.Name, "Failed to delete orphan %q: %v", item.Id, err)
			}
		} else if dirID == "" {
			operations.SyncPrintf("%q, %q\n", item.Name, item.Id)
		} else {
			fs.Infof(item.Name, "Rescuing orphan %q", item.Id)
			err = f.pacer.Call(func() (bool, error) {
				_, err = f.svc.Files.Update(item.Id, nil).
					AddParents(dirID).
					Fields(f.getFileFields(ctx)).
					SupportsAllDrives(true).
					Context(ctx).Do()
				return f.shouldRetry(ctx, err)
			})
			if err != nil {
				fs.Errorf(item.Name, "Failed to rescue orphan %q: %v", item.Id, err)
			}
		}
	})
}

var commandHelp = []fs.CommandHelp{{
	Name:  "get",
	Short: "Get command for fetching the drive config parameters",
	Long: `This is a get command which will be used to fetch the various drive config parameters

Usage Examples:

    rclone backend get drive: [-o service_account_file] [-o chunk_size]
    rclone rc backend/command command=get fs=drive: [-o service_account_file] [-o chunk_size]
`,
	Opts: map[string]string{
		"chunk_size":           "show the current upload chunk size",
		"service_account_file": "show the current service account file",
	},
}, {
	Name:  "set",
	Short: "Set command for updating the drive config parameters",
	Long: `This is a set command which will be used to update the various drive config parameters

Usage Examples:

    rclone backend set drive: [-o service_account_file=sa.json] [-o chunk_size=67108864]
    rclone rc backend/command command=set fs=drive: [-o service_account_file=sa.json] [-o chunk_size=67108864]
`,
	Opts: map[string]string{
		"chunk_size":           "update the current upload chunk size",
		"service_account_file": "update the current service account file",
	},
}, {
	Name:  "shortcut",
	Short: "Create shortcuts from files or directories",
	Long: `This command creates shortcuts from files or directories.

Usage:

    rclone backend shortcut drive: source_item destination_shortcut
    rclone backend shortcut drive: source_item -o target=drive2: destination_shortcut

In the first example this creates a shortcut from the "source_item"
which can be a file or a directory to the "destination_shortcut". The
"source_item" and the "destination_shortcut" should be relative paths
from "drive:"

In the second example this creates a shortcut from the "source_item"
relative to "drive:" to the "destination_shortcut" relative to
"drive2:". This may fail with a permission error if the user
authenticated with "drive2:" can't read files from "drive:".
`,
	Opts: map[string]string{
		"target": "optional target remote for the shortcut destination",
	},
}, {
	Name:  "drives",
	Short: "List the Shared Drives available to this account",
	Long: `This command lists the Shared Drives (Team Drives) available to this
account.

Usage:

    rclone backend [-o config] drives drive:

This will return a JSON list of objects like this

    [
        {
            "id": "0ABCDEF-01234567890",
            "kind": "drive#teamDrive",
            "name": "My Drive"
        },
        {
            "id": "0ABCDEFabcdefghijkl",
            "kind": "drive#teamDrive",
            "name": "Test Drive"
        }
    ]

With the -o config parameter it will output the list in a format
suitable for adding to a config file to make aliases for all the
drives found and a combined drive.

    [My Drive]
    type = alias
    remote = drive,team_drive=0ABCDEF-01234567890,root_folder_id=:

    [Test Drive]
    type = alias
    remote = drive,team_drive=0ABCDEFabcdefghijkl,root_folder_id=:

    [AllDrives]
    type = combine
    upstreams = "My Drive=My Drive:" "Test Drive=Test Drive:"

Adding this to the rclone config file will cause those team drives to
be accessible with the aliases shown. Any illegal characters will be
substituted with "_" and duplicate names will have numbers suffixed.
It will also add a remote called AllDrives which shows all the shared
drives combined into one directory tree.
`,
}, {
	Name:  "untrash",
	Short: "Untrash files and directories",
	Long: `This command untrashes all the files and directories in the directory
passed in recursively.

Usage:

This takes an optional directory to trash which make this easier to
use via the API.

    rclone backend untrash drive:directory
    rclone backend --interactive untrash drive:directory subdir

Use the --interactive/-i or --dry-run flag to see what would be restored before restoring it.

Result:

    {
        "Untrashed": 17,
        "Errors": 0
    }
`,
}, {
	Name:  "copyid",
	Short: "Copy files by ID",
	Long: `This command copies files by ID

Usage:

    rclone backend copyid drive: ID path
    rclone backend copyid drive: ID1 path1 ID2 path2

It copies the drive file with ID given to the path (an rclone path which
will be passed internally to rclone copyto). The ID and path pairs can be
repeated.

The path should end with a / to indicate copy the file as named to
this directory. If it doesn't end with a / then the last path
component will be used as the file name.

If the destination is a drive backend then server-side copying will be
attempted if possible.

Use the --interactive/-i or --dry-run flag to see what would be copied before copying.
`,
}, {
	Name:  "moveid",
	Short: "Move files by ID",
	Long: `This command moves files by ID

Usage:

    rclone backend moveid drive: ID path
    rclone backend moveid drive: ID1 path1 ID2 path2

It moves the drive file with ID given to the path (an rclone path which
will be passed internally to rclone moveto).

The path should end with a / to indicate move the file as named to
this directory. If it doesn't end with a / then the last path
component will be used as the file name.

If the destination is a drive backend then server-side moving will be
attempted if possible.

Use the --interactive/-i or --dry-run flag to see what would be moved beforehand.
`,
}, {
	Name:  "exportformats",
	Short: "Dump the export formats for debug purposes",
}, {
	Name:  "importformats",
	Short: "Dump the import formats for debug purposes",
}, {
	Name:  "query",
	Short: "List files using Google Drive query language",
	Long: `This command lists files based on a query

Usage:

    rclone backend query drive: query
    
The query syntax is documented at [Google Drive Search query terms and 
operators](https://developers.google.com/drive/api/guides/ref-search-terms).

For example:

	rclone backend query drive: "'0ABc9DEFGHIJKLMNop0QRatUVW3X' in parents and name contains 'foo'"

If the query contains literal ' or \ characters, these need to be escaped with
\ characters. "'" becomes "\'" and "\" becomes "\\\", for example to match a 
file named "foo ' \.txt":

	rclone backend query drive: "name = 'foo \' \\\.txt'"

The result is a JSON array of matches, for example:

    [
	{
		"createdTime": "2017-06-29T19:58:28.537Z",
		"id": "0AxBe_CDEF4zkGHI4d0FjYko2QkD",
		"md5Checksum": "68518d16be0c6fbfab918be61d658032",
		"mimeType": "text/plain",
		"modifiedTime": "2024-02-02T10:40:02.874Z",
		"name": "foo ' \\.txt",
		"parents": [
			"0BxAe_BCDE4zkFGZpcWJGek0xbzC"
		],
		"resourceKey": "0-ABCDEFGHIXJQpIGqBJq3MC",
		"sha1Checksum": "8f284fa768bfb4e45d076a579ab3905ab6bfa893",
		"size": "311",
		"webViewLink": "https://drive.google.com/file/d/0AxBe_CDEF4zkGHI4d0FjYko2QkD/view?usp=drivesdk\u0026resourcekey=0-ABCDEFGHIXJQpIGqBJq3MC"
	}
    ]`,
}, {
	Name:  "rescue",
	Short: "Rescue or delete any orphaned files",
	Long: `This command rescues or deletes any orphaned files or directories.

Sometimes files can get orphaned in Google Drive. This means that they
are no longer in any folder in Google Drive.

This command finds those files and either rescues them to a directory
you specify or deletes them.

Usage:

This can be used in 3 ways.

First, list all orphaned files

    rclone backend rescue drive:

Second rescue all orphaned files to the directory indicated

    rclone backend rescue drive: "relative/path/to/rescue/directory"

e.g. To rescue all orphans to a directory called "Orphans" in the top level

    rclone backend rescue drive: Orphans

Third delete all orphaned files to the trash

    rclone backend rescue drive: -o delete
`,
}}

// Command the backend to run a named command
//
// The command run is name
// args may be used to read arguments from
// opts may be used to read optional arguments from
//
// The result should be capable of being JSON encoded
// If it is a string or a []string it will be shown to the user
// otherwise it will be JSON encoded and shown to the user like that
func (f *Fs) Command(ctx context.Context, name string, arg []string, opt map[string]string) (out any, err error) {
	switch name {
	case "get":
		out := make(map[string]string)
		if _, ok := opt["service_account_file"]; ok {
			out["service_account_file"] = f.opt.ServiceAccountFile
		}
		if _, ok := opt["chunk_size"]; ok {
			out["chunk_size"] = f.opt.ChunkSize.String()
		}
		return out, nil
	case "set":
		out := make(map[string]map[string]string)
		if serviceAccountFile, ok := opt["service_account_file"]; ok {
			serviceAccountMap := make(map[string]string)
			serviceAccountMap["previous"] = f.opt.ServiceAccountFile
			if err = f.changeServiceAccountFile(ctx, serviceAccountFile); err != nil {
				return out, err
			}
			f.m.Set("service_account_file", serviceAccountFile)
			serviceAccountMap["current"] = f.opt.ServiceAccountFile
			out["service_account_file"] = serviceAccountMap
		}
		if chunkSize, ok := opt["chunk_size"]; ok {
			chunkSizeMap := make(map[string]string)
			chunkSizeMap["previous"] = f.opt.ChunkSize.String()
			if err = f.changeChunkSize(chunkSize); err != nil {
				return out, err
			}
			chunkSizeString := f.opt.ChunkSize.String()
			f.m.Set("chunk_size", chunkSizeString)
			chunkSizeMap["current"] = chunkSizeString
			out["chunk_size"] = chunkSizeMap
		}
		return out, nil
	case "shortcut":
		if len(arg) != 2 {
			return nil, errors.New("need exactly 2 arguments")
		}
		dstFs := f
		target, ok := opt["target"]
		if ok {
			targetFs, err := cache.Get(ctx, target)
			if err != nil {
				return nil, fmt.Errorf("couldn't find target: %w", err)
			}
			dstFs, ok = targetFs.(*Fs)
			if !ok {
				return nil, errors.New("target is not a drive backend")
			}
		}
		return f.makeShortcut(ctx, arg[0], dstFs, arg[1])
	case "drives":
		drives, err := f.listTeamDrives(ctx)
		if err != nil {
			return nil, err
		}
		if _, ok := opt["config"]; ok {
			lines := []string{}
			upstreams := []string{}
			names := make(map[string]struct{}, len(drives))
			for i, drive := range drives {
				name := fspath.MakeConfigName(drive.Name)
				for {
					if _, found := names[name]; !found {
						break
					}
					name += fmt.Sprintf("-%d", i)
				}
				names[name] = struct{}{}
				lines = append(lines, "")
				lines = append(lines, fmt.Sprintf("[%s]", name))
				lines = append(lines, "type = alias")
				lines = append(lines, fmt.Sprintf("remote = %s,team_drive=%s,root_folder_id=:", f.name, drive.Id))
				upstreams = append(upstreams, fmt.Sprintf(`"%s=%s:"`, name, name))
			}
			lines = append(lines, "")
			lines = append(lines, "[AllDrives]")
			lines = append(lines, "type = combine")
			lines = append(lines, fmt.Sprintf("upstreams = %s", strings.Join(upstreams, " ")))
			return lines, nil
		}
		return drives, nil
	case "untrash":
		dir := ""
		if len(arg) > 0 {
			dir = arg[0]
		}
		return f.unTrashDir(ctx, dir, true)
	case "copyid", "moveid":
		if len(arg)%2 != 0 {
			return nil, errors.New("need an even number of arguments")
		}
		for len(arg) > 0 {
			id, dest := arg[0], arg[1]
			arg = arg[2:]
			err = f.copyOrMoveID(ctx, name, id, dest)
			if err != nil {
				return nil, fmt.Errorf("failed %s %q to %q: %w", name, id, dest, err)
			}
		}
		return nil, nil
	case "exportformats":
		return f.exportFormats(ctx), nil
	case "importformats":
		return f.importFormats(ctx), nil
	case "query":
		if len(arg) == 1 {
			query := arg[0]
			results, err := f.query(ctx, query)
			if err != nil {
				return nil, fmt.Errorf("failed to execute query: %q, error: %w", query, err)
			}
			return results, nil
		}
		return nil, errors.New("need a query argument")
	case "rescue":
		dirID := ""
		_, delete := opt["delete"]
		if len(arg) == 0 {
			// no arguments - list only
		} else if !delete && len(arg) == 1 {
			dir := arg[0]
			dirID, err = f.dirCache.FindDir(ctx, dir, true)
			if err != nil {
				return nil, fmt.Errorf("failed to find or create rescue directory %q: %w", dir, err)
			}
			fs.Infof(f, "Rescuing orphans into %q", dir)
		} else {
			return nil, errors.New("syntax error: need 0 or 1 args or -o delete")
		}
		return nil, f.rescue(ctx, dirID, delete)
	default:
		return nil, fs.ErrorCommandNotFound
	}
}

// ------------------------------------------------------------

// Fs returns the parent Fs
func (o *baseObject) Fs() fs.Info {
	return o.fs
}

// Return a string version
func (o *baseObject) String() string {
	return o.remote
}

// Return a string version
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// Remote returns the remote path
func (o *baseObject) Remote() string {
	return o.remote
}

// Hash returns the Md5sum of an object returning a lowercase hex string
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	if t == hash.MD5 {
		return o.md5sum, nil
	}
	if t == hash.SHA1 {
		return o.sha1sum, nil
	}
	if t == hash.SHA256 {
		return o.sha256sum, nil
	}
	return "", hash.ErrUnsupported
}

func (o *baseObject) Hash(ctx context.Context, t hash.Type) (string, error) {
	if t != hash.MD5 && t != hash.SHA1 && t != hash.SHA256 {
		return "", hash.ErrUnsupported
	}
	return "", nil
}

// Size returns the size of an object in bytes
func (o *baseObject) Size() int64 {
	return o.bytes
}

// getRemoteInfoWithExport returns a drive.File and the export settings for the remote
func (f *Fs) getRemoteInfoWithExport(ctx context.Context, remote string) (
	info *drive.File, extension, exportName, exportMimeType string, isDocument bool, err error,
) {
	leaf, directoryID, err := f.dirCache.FindPath(ctx, remote, false)
	if err != nil {
		if err == fs.ErrorDirNotFound {
			return nil, "", "", "", false, fs.ErrorObjectNotFound
		}
		return nil, "", "", "", false, err
	}
	directoryID = actualID(directoryID)

	found, err := f.list(ctx, []string{directoryID}, leaf, false, false, f.opt.TrashedOnly, false, func(item *drive.File) bool {
		if !f.opt.SkipGdocs {
			extension, exportName, exportMimeType, isDocument = f.findExportFormat(ctx, item)
			if exportName == leaf {
				info = item
				return true
			}
			if isDocument {
				return false
			}
		}
		if item.Name == leaf {
			info = item
			return true
		}
		return false
	})
	if err != nil {
		return nil, "", "", "", false, err
	}
	if !found {
		return nil, "", "", "", false, fs.ErrorObjectNotFound
	}
	return
}

// ModTime returns the modification time of the object
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *baseObject) ModTime(ctx context.Context) time.Time {
	modTime, err := time.Parse(timeFormatIn, o.modifiedDate)
	if err != nil {
		fs.Debugf(o, "Failed to read mtime from object: %v", err)
		return time.Now()
	}
	return modTime
}

// SetModTime sets the modification time of the drive fs object
func (o *baseObject) SetModTime(ctx context.Context, modTime time.Time) error {
	// New metadata
	updateInfo := &drive.File{
		ModifiedTime: modTime.Format(timeFormatOut),
	}
	// Set modified date
	var info *drive.File
	err := o.fs.pacer.Call(func() (bool, error) {
		var err error
		info, err = o.fs.svc.Files.Update(actualID(o.id), updateInfo).
			Fields(partialFields).
			SupportsAllDrives(true).
			Context(ctx).Do()
		return o.fs.shouldRetry(ctx, err)
	})
	if err != nil {
		return err
	}
	// Update info from read data
	o.modifiedDate = info.ModifiedTime
	return nil
}

// Storable returns a boolean as to whether this object is storable
func (o *baseObject) Storable() bool {
	return true
}

// addResourceKey adds a X-Goog-Drive-Resource-Keys header for this
// object if required.
func (o *baseObject) addResourceKey(header http.Header) {
	if o.resourceKey != nil {
		header.Add("X-Goog-Drive-Resource-Keys", fmt.Sprintf("%s/%s", o.id, *o.resourceKey))
	}
}

// httpResponse gets an http.Response object for the object
// using the url and method passed in
func (o *baseObject) httpResponse(ctx context.Context, url, method string, options []fs.OpenOption) (req *http.Request, res *http.Response, err error) {
	if url == "" {
		return nil, nil, errors.New("forbidden to download - check sharing permission")
	}
	req, err = http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return req, nil, err
	}
	fs.OpenOptionAddHTTPHeaders(req.Header, options)
	if o.bytes == 0 {
		// Don't supply range requests for 0 length objects as they always fail
		delete(req.Header, "Range")
	}
	o.addResourceKey(req.Header)
	err = o.fs.pacer.Call(func() (bool, error) {
		res, err = o.fs.client.Do(req)
		if err == nil {
			err = googleapi.CheckResponse(res)
			if err != nil {
				_ = res.Body.Close() // ignore error
			}
		}
		return o.fs.shouldRetry(ctx, err)
	})
	if err != nil {
		return req, nil, err
	}
	return req, res, nil
}

// openDocumentFile represents a documentObject open for reading.
// Updates the object size after read successfully.
type openDocumentFile struct {
	o       *documentObject // Object we are reading for
	in      io.ReadCloser   // reading from here
	bytes   int64           // number of bytes read on this connection
	eof     bool            // whether we have read end of file
	errored bool            // whether we have encountered an error during reading
}

// Read bytes from the object - see io.Reader
func (file *openDocumentFile) Read(p []byte) (n int, err error) {
	n, err = file.in.Read(p)
	file.bytes += int64(n)
	if err != nil && err != io.EOF {
		file.errored = true
	}
	if err == io.EOF {
		file.eof = true
	}
	return
}

// Close the object and update bytes read
func (file *openDocumentFile) Close() (err error) {
	// If end of file, update bytes read
	if file.eof && !file.errored {
		fs.Debugf(file.o, "Updating size of doc after download to %v", file.bytes)
		file.o.bytes = file.bytes
	}
	return file.in.Close()
}

// Check it satisfies the interfaces
var _ io.ReadCloser = (*openDocumentFile)(nil)

// Checks to see if err is a googleapi.Error with of type what
func isGoogleError(err error, what string) bool {
	if gerr, ok := err.(*googleapi.Error); ok {
		for _, error := range gerr.Errors {
			if error.Reason == what {
				return true
			}
		}
	}
	return false
}

// open a url for reading
func (o *baseObject) open(ctx context.Context, url string, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	_, res, err := o.httpResponse(ctx, url, "GET", options)
	if err != nil {
		if isGoogleError(err, "cannotDownloadAbusiveFile") {
			if o.fs.opt.AcknowledgeAbuse {
				// Retry acknowledging abuse
				if strings.ContainsRune(url, '?') {
					url += "&"
				} else {
					url += "?"
				}
				url += "acknowledgeAbuse=true"
				_, res, err = o.httpResponse(ctx, url, "GET", options)
			} else {
				err = fmt.Errorf("use the --drive-acknowledge-abuse flag to download this file: %w", err)
			}
		}
		if err != nil {
			return nil, fmt.Errorf("open file failed: %w", err)
		}
	}
	return res.Body, nil
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	if o.mimeType == shortcutMimeTypeDangling {
		return nil, errors.New("can't read dangling shortcut")
	}
	if o.v2Download {
		var v2File *drive_v2.File
		err = o.fs.pacer.Call(func() (bool, error) {
			v2File, err = o.fs.v2Svc.Files.Get(actualID(o.id)).
				Fields("downloadUrl").
				SupportsAllDrives(true).
				Context(ctx).Do()
			return o.fs.shouldRetry(ctx, err)
		})
		if err == nil {
			fs.Debugf(o, "Using v2 download: %v", v2File.DownloadUrl)
			o.url = v2File.DownloadUrl
			o.v2Download = false
		}
	}
	return o.baseObject.open(ctx, o.url, options...)
}

func (o *documentObject) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	// Update the size with what we are reading as it can change from
	// the HEAD in the listing to this GET. This stops rclone marking
	// the transfer as corrupted.
	var offset, end int64 = 0, -1
	newOptions := options[:0]
	for _, o := range options {
		// Note that Range requests don't work on Google docs:
		// https://developers.google.com/drive/v3/web/manage-downloads#partial_download
		// So do a subset of them manually
		switch x := o.(type) {
		case *fs.RangeOption:
			offset, end = x.Start, x.End
		case *fs.SeekOption:
			offset, end = x.Offset, -1
		default:
			newOptions = append(newOptions, o)
		}
	}
	options = newOptions
	if offset != 0 {
		return nil, errors.New("partial downloads are not supported while exporting Google Documents")
	}
	in, err = o.baseObject.open(ctx, o.url, options...)
	if in != nil {
		in = &openDocumentFile{o: o, in: in}
	}
	if end >= 0 {
		in = readers.NewLimitedReadCloser(in, end-offset+1)
	}
	return
}

func (o *linkObject) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	var offset, limit int64 = 0, -1
	data := o.content
	for _, option := range options {
		switch x := option.(type) {
		case *fs.SeekOption:
			offset = x.Offset
		case *fs.RangeOption:
			offset, limit = x.Decode(int64(len(data)))
		default:
			if option.Mandatory() {
				fs.Logf(o, "Unsupported mandatory option: %v", option)
			}
		}
	}
	if l := int64(len(data)); offset > l {
		offset = l
	}
	data = data[offset:]
	if limit != -1 && limit < int64(len(data)) {
		data = data[:limit]
	}

	return io.NopCloser(bytes.NewReader(data)), nil
}

func (o *baseObject) update(ctx context.Context, updateInfo *drive.File, uploadMimeType string, in io.Reader,
	src fs.ObjectInfo,
) (info *drive.File, err error) {
	// Make the API request to upload metadata and file data.
	size := src.Size()
	if size >= 0 && size < int64(o.fs.opt.UploadCutoff) {
		// Don't retry, return a retry error instead
		err = o.fs.pacer.CallNoRetry(func() (bool, error) {
			info, err = o.fs.svc.Files.Update(actualID(o.id), updateInfo).
				Media(in, googleapi.ContentType(uploadMimeType), googleapi.ChunkSize(0)).
				Fields(partialFields).
				SupportsAllDrives(true).
				KeepRevisionForever(o.fs.opt.KeepRevisionForever).
				Context(ctx).Do()
			return o.fs.shouldRetry(ctx, err)
		})
		return
	}
	// Upload the file in chunks
	return o.fs.Upload(ctx, in, size, uploadMimeType, o.id, o.remote, updateInfo)
}

// Update the already existing object
//
// Copy the reader into the object updating modTime and size.
//
// The new object may have been created if an error is returned
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	// If o is a shortcut
	if isShortcutID(o.id) {
		// Delete it first
		err := o.fs.delete(ctx, shortcutID(o.id), o.fs.opt.UseTrash)
		if err != nil {
			return err
		}
		// Then put the file as a new file
		newObj, err := o.fs.PutUnchecked(ctx, in, src, options...)
		if err != nil {
			return err
		}
		// Update the object
		if newO, ok := newObj.(*Object); ok {
			*o = *newO
		} else {
			fs.Debugf(newObj, "Failed to update object %T from new object %T", o, newObj)
		}
		return nil
	}
	srcMimeType := fs.MimeType(ctx, src)
	updateInfo := &drive.File{
		MimeType:     srcMimeType,
		ModifiedTime: src.ModTime(ctx).Format(timeFormatOut),
	}

	updateMetadata, err := o.fs.fetchAndUpdateMetadata(ctx, src, options, updateInfo, true)
	if err != nil {
		return err
	}

	info, err := o.baseObject.update(ctx, updateInfo, srcMimeType, in, src)
	if err != nil {
		return err
	}
	err = updateMetadata(ctx, info)
	if err != nil {
		return err
	}
	newO, err := o.fs.newObjectWithInfo(ctx, o.remote, info)
	if err != nil {
		return err
	}
	switch newO := newO.(type) {
	case *Object:
		*o = *newO
	default:
		return errors.New("object type changed by update")
	}

	return nil
}

func (o *documentObject) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	srcMimeType := fs.MimeType(ctx, src)
	importMimeType := ""
	updateInfo := &drive.File{
		MimeType:     srcMimeType,
		ModifiedTime: src.ModTime(ctx).Format(timeFormatOut),
	}

	if o.fs.importMimeTypes == nil || o.fs.opt.SkipGdocs {
		return fmt.Errorf("can't update google document type without --drive-import-formats")
	}
	importMimeType = o.fs.findImportFormat(ctx, updateInfo.MimeType)
	if importMimeType == "" {
		return fmt.Errorf("no import format found for %q", srcMimeType)
	}
	if importMimeType != o.documentMimeType {
		return fmt.Errorf("can't change google document type (o: %q, src: %q, import: %q)", o.documentMimeType, srcMimeType, importMimeType)
	}
	updateInfo.MimeType = importMimeType

	info, err := o.baseObject.update(ctx, updateInfo, srcMimeType, in, src)
	if err != nil {
		return err
	}

	remote := src.Remote()
	remote = remote[:len(remote)-o.extLen]

	newO, err := o.fs.newObjectWithInfo(ctx, remote, info)
	if err != nil {
		return err
	}
	switch newO := newO.(type) {
	case *documentObject:
		*o = *newO
	default:
		return errors.New("object type changed by update")
	}

	return nil
}

func (o *linkObject) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	return errors.New("cannot update link files")
}

// Remove an object
func (o *baseObject) Remove(ctx context.Context) error {
	if len(o.parents) > 1 {
		return errors.New("can't delete safely - has multiple parents")
	}
	return o.fs.delete(ctx, shortcutID(o.id), o.fs.opt.UseTrash)
}

// MimeType of an Object if known, "" otherwise
func (o *baseObject) MimeType(ctx context.Context) string {
	return o.mimeType
}

// ID returns the ID of the Object if known, or "" if not
func (o *baseObject) ID() string {
	return o.id
}

// ParentID returns the ID of the Object parent if known, or "" if not
func (o *baseObject) ParentID() string {
	if len(o.parents) > 0 {
		return o.parents[0]
	}
	return ""
}

// Metadata returns metadata for an object
//
// It should return nil if there is no Metadata
func (o *baseObject) Metadata(ctx context.Context) (metadata fs.Metadata, err error) {
	if o.metadata != nil {
		return *o.metadata, nil
	}
	fs.Debugf(o, "Fetching metadata")
	id := actualID(o.id)
	info, err := o.fs.getFile(ctx, id, o.fs.getFileFields(ctx))
	if err != nil {
		return nil, err
	}
	err = o.parseMetadata(ctx, info)
	if err != nil {
		return nil, err
	}
	return *o.metadata, nil
}

func (o *documentObject) ext() string {
	return o.baseObject.remote[len(o.baseObject.remote)-o.extLen:]
}

func (o *linkObject) ext() string {
	return o.baseObject.remote[len(o.baseObject.remote)-o.extLen:]
}

// Items returns the count of items in this directory or this
// directory and subdirectories if known, -1 for unknown
func (d *Directory) Items() int64 {
	return -1
}

// SetMetadata sets metadata for a Directory
//
// It should return fs.ErrorNotImplemented if it can't set metadata
func (d *Directory) SetMetadata(ctx context.Context, metadata fs.Metadata) error {
	info, err := d.fs.updateDir(ctx, d.id, metadata)
	if err != nil {
		return fmt.Errorf("failed to update directory info: %w", err)
	}
	// Update directory from info returned
	baseObject, err := d.fs.newBaseObject(ctx, d.remote, info)
	if err != nil {
		return fmt.Errorf("failed to process directory info: %w", err)
	}
	d.baseObject = baseObject
	return err
}

// Hash does nothing on a directory
//
// This method is implemented with the incorrect type signature to
// stop the Directory type asserting to fs.Object or fs.ObjectInfo
func (d *Directory) Hash() {
	// Does nothing
}

// templates for document link files
const (
	urlTemplate = `[InternetShortcut]{{"\r"}}
URL={{ .URL }}{{"\r"}}
`
	weblocTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
  <dict>
    <key>URL</key>
    <string>{{ .URL }}</string>
  </dict>
</plist>
`
	desktopTemplate = `[Desktop Entry]
Encoding=UTF-8
Name={{ .Title }}
URL={{ .URL }}
Icon={{ .XDGIcon }}
Type=Link
`
	htmlTemplate = `<html>
<head>
  <meta http-equiv="refresh" content="0; url={{ .URL }}" />
  <title>{{ .Title }}</title>
</head>
<body>
  Loading <a href="{{ .URL }}">{{ .Title }}</a>
</body>
</html>
`
)

// Check the interfaces are satisfied
var (
	_ fs.Fs              = (*Fs)(nil)
	_ fs.Purger          = (*Fs)(nil)
	_ fs.CleanUpper      = (*Fs)(nil)
	_ fs.PutStreamer     = (*Fs)(nil)
	_ fs.Copier          = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ fs.Commander       = (*Fs)(nil)
	_ fs.DirCacheFlusher = (*Fs)(nil)
	_ fs.ChangeNotifier  = (*Fs)(nil)
	_ fs.PutUncheckeder  = (*Fs)(nil)
	_ fs.PublicLinker    = (*Fs)(nil)
	_ fs.ListRer         = (*Fs)(nil)
	_ fs.MergeDirser     = (*Fs)(nil)
	_ fs.DirSetModTimer  = (*Fs)(nil)
	_ fs.MkdirMetadataer = (*Fs)(nil)
	_ fs.Abouter         = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.MimeTyper       = (*Object)(nil)
	_ fs.IDer            = (*Object)(nil)
	_ fs.ParentIDer      = (*Object)(nil)
	_ fs.Metadataer      = (*Object)(nil)
	_ fs.Object          = (*documentObject)(nil)
	_ fs.MimeTyper       = (*documentObject)(nil)
	_ fs.IDer            = (*documentObject)(nil)
	_ fs.ParentIDer      = (*documentObject)(nil)
	_ fs.Object          = (*linkObject)(nil)
	_ fs.MimeTyper       = (*linkObject)(nil)
	_ fs.IDer            = (*linkObject)(nil)
	_ fs.ParentIDer      = (*linkObject)(nil)
	_ fs.Directory       = (*Directory)(nil)
	_ fs.SetModTimer     = (*Directory)(nil)
	_ fs.SetMetadataer   = (*Directory)(nil)
	_ fs.ParentIDer      = (*Directory)(nil)
)
