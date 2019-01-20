// Package drive interfaces with the Google Drive object storage system

// +build go1.9

package drive

// FIXME need to deal with some corner cases
// * multiple files with the same name
// * files can be in multiple directories
// * can have directory loops
// * files with / in name

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/config"
	"github.com/ncw/rclone/fs/config/configmap"
	"github.com/ncw/rclone/fs/config/configstruct"
	"github.com/ncw/rclone/fs/config/obscure"
	"github.com/ncw/rclone/fs/fserrors"
	"github.com/ncw/rclone/fs/fshttp"
	"github.com/ncw/rclone/fs/hash"
	"github.com/ncw/rclone/fs/walk"
	"github.com/ncw/rclone/lib/dircache"
	"github.com/ncw/rclone/lib/oauthutil"
	"github.com/ncw/rclone/lib/pacer"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	drive_v2 "google.golang.org/api/drive/v2"
	drive "google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
)

// Constants
const (
	rcloneClientID              = "202264815644.apps.googleusercontent.com"
	rcloneEncryptedClientSecret = "eX8GpZTVx3vxMWVkuuBdDWmAUE6rGhTwVrvG9GhllYccSdj2-mvHVg"
	driveFolderType             = "application/vnd.google-apps.folder"
	timeFormatIn                = time.RFC3339
	timeFormatOut               = "2006-01-02T15:04:05.000000000Z07:00"
	defaultMinSleep             = fs.Duration(100 * time.Millisecond)
	defaultBurst                = 100
	defaultExportExtensions     = "docx,xlsx,pptx,svg"
	scopePrefix                 = "https://www.googleapis.com/auth/"
	defaultScope                = "drive"
	// chunkSize is the size of the chunks created during a resumable upload and should be a power of two.
	// 1<<18 is the minimum size supported by the Google uploader, and there is no maximum.
	minChunkSize     = 256 * fs.KibiByte
	defaultChunkSize = 8 * fs.MebiByte
	partialFields    = "id,name,size,md5Checksum,trashed,modifiedTime,createdTime,mimeType,parents,webViewLink"
)

// Globals
var (
	// Description of how to auth for this app
	driveConfig = &oauth2.Config{
		Scopes:       []string{scopePrefix + "drive"},
		Endpoint:     google.Endpoint,
		ClientID:     rcloneClientID,
		ClientSecret: obscure.MustReveal(rcloneEncryptedClientSecret),
		RedirectURL:  oauthutil.TitleBarRedirectURL,
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
	fetchFormatsOnce sync.Once                     // make sure we fetch the export/import formats only once
	_exportFormats   map[string][]string           // allowed export MIME type conversions
	_importFormats   map[string][]string           // allowed import MIME type conversions
	templatesOnce    sync.Once                     // parse link templates only once
	_linkTemplates   map[string]*template.Template // available link types
)

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
	for _, scope := range scopes {
		if scope == scopePrefix+"drive.appfolder" {
			return true
		}

	}
	return false
}

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "drive",
		Description: "Google Drive",
		NewFs:       NewFs,
		Config: func(name string, m configmap.Mapper) {
			// Parse config into Options struct
			opt := new(Options)
			err := configstruct.Set(m, opt)
			if err != nil {
				fs.Errorf(nil, "Couldn't parse config into struct: %v", err)
				return
			}

			// Fill in the scopes
			driveConfig.Scopes = driveScopes(opt.Scope)
			// Set the root_folder_id if using drive.appfolder
			if driveScopesContainsAppFolder(driveConfig.Scopes) {
				m.Set("root_folder_id", "appDataFolder")
			}

			if opt.ServiceAccountFile == "" {
				err = oauthutil.Config("drive", name, m, driveConfig)
				if err != nil {
					log.Fatalf("Failed to configure token: %v", err)
				}
			}
			err = configTeamDrive(opt, m, name)
			if err != nil {
				log.Fatalf("Failed to configure team drive: %v", err)
			}
		},
		Options: []fs.Option{{
			Name: config.ConfigClientID,
			Help: "Google Application Client Id\nLeave blank normally.",
		}, {
			Name: config.ConfigClientSecret,
			Help: "Google Application Client Secret\nLeave blank normally.",
		}, {
			Name: "scope",
			Help: "Scope that rclone should use when requesting access from drive.",
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
			Help: "ID of the root folder\nLeave blank normally.\nFill in to access \"Computers\" folders. (see docs).",
		}, {
			Name: "service_account_file",
			Help: "Service Account Credentials JSON file path \nLeave blank normally.\nNeeded only if you want use SA instead of interactive login.",
		}, {
			Name:     "service_account_credentials",
			Help:     "Service Account Credentials JSON blob\nLeave blank normally.\nNeeded only if you want use SA instead of interactive login.",
			Hide:     fs.OptionHideConfigurator,
			Advanced: true,
		}, {
			Name:     "team_drive",
			Help:     "ID of the Team Drive",
			Hide:     fs.OptionHideConfigurator,
			Advanced: true,
		}, {
			Name:     "auth_owner_only",
			Default:  false,
			Help:     "Only consider files owned by the authenticated user.",
			Advanced: true,
		}, {
			Name:     "use_trash",
			Default:  true,
			Help:     "Send files to the trash instead of deleting permanently.\nDefaults to true, namely sending files to the trash.\nUse `--drive-use-trash=false` to delete files permanently instead.",
			Advanced: true,
		}, {
			Name:     "skip_gdocs",
			Default:  false,
			Help:     "Skip google documents in all listings.\nIf given, gdocs practically become invisible to rclone.",
			Advanced: true,
		}, {
			Name:    "shared_with_me",
			Default: false,
			Help: `Only show files that are shared with me.

Instructs rclone to operate on your "Shared with me" folder (where
Google Drive lets you access the files and folders others have shared
with you).

This works both with the "list" (lsd, lsl, etc) and the "copy"
commands (copy, sync, etc), and with all other commands too.`,
			Advanced: true,
		}, {
			Name:     "trashed_only",
			Default:  false,
			Help:     "Only show files that are in the trash.\nThis will show trashed files in their original directory structure.",
			Advanced: true,
		}, {
			Name:     "formats",
			Default:  "",
			Help:     "Deprecated: see export_formats",
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
			Help:     "Allow the filetype to change when uploading Google docs (e.g. file.doc to file.docx). This will confuse sync and reupload every time.",
			Advanced: true,
		}, {
			Name:    "use_created_date",
			Default: false,
			Help: `Use file created date instead of modified date.,

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
		}, {
			Name:     "list_chunk",
			Default:  1000,
			Help:     "Size of listing chunk 100-1000. 0 to disable.",
			Advanced: true,
		}, {
			Name:     "impersonate",
			Default:  "",
			Help:     "Impersonate this user when using a service account.",
			Advanced: true,
		}, {
			Name:    "alternate_export",
			Default: false,
			Help: `Use alternate export URLs for google documents export.,

If this option is set this instructs rclone to use an alternate set of
export URLs for drive documents.  Users have reported that the
official export URLs can't export large documents, whereas these
unofficial ones can.

See rclone issue [#2243](https://github.com/ncw/rclone/issues/2243) for background,
[this google drive issue](https://issuetracker.google.com/issues/36761333) and
[this helpful post](https://www.labnol.org/internet/direct-links-for-google-drive/28356/).`,
			Advanced: true,
		}, {
			Name:     "upload_cutoff",
			Default:  defaultChunkSize,
			Help:     "Cutoff for switching to chunked upload",
			Advanced: true,
		}, {
			Name:    "chunk_size",
			Default: defaultChunkSize,
			Help: `Upload chunk size. Must a power of 2 >= 256k.

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
will download it anyway.`,
			Advanced: true,
		}, {
			Name:     "keep_revision_forever",
			Default:  false,
			Help:     "Keep new head revision of each file forever.",
			Advanced: true,
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
		}},
	})

	// register duplicate MIME types first
	// this allows them to be used with mime.ExtensionsByType() but
	// mime.TypeByExtension() will return the later registered MIME type
	for _, m := range []map[string]string{
		_mimeTypeToExtensionDuplicates, _mimeTypeToExtension, _mimeTypeToExtensionLinks,
	} {
		for mimeType, extension := range m {
			if err := mime.AddExtensionType(extension, mimeType); err != nil {
				log.Fatalf("Failed to register MIME type %q: %v", mimeType, err)
			}
		}
	}
}

// Options defines the configuration for this backend
type Options struct {
	Scope                     string        `config:"scope"`
	RootFolderID              string        `config:"root_folder_id"`
	ServiceAccountFile        string        `config:"service_account_file"`
	ServiceAccountCredentials string        `config:"service_account_credentials"`
	TeamDriveID               string        `config:"team_drive"`
	AuthOwnerOnly             bool          `config:"auth_owner_only"`
	UseTrash                  bool          `config:"use_trash"`
	SkipGdocs                 bool          `config:"skip_gdocs"`
	SharedWithMe              bool          `config:"shared_with_me"`
	TrashedOnly               bool          `config:"trashed_only"`
	Extensions                string        `config:"formats"`
	ExportExtensions          string        `config:"export_formats"`
	ImportExtensions          string        `config:"import_formats"`
	AllowImportNameChange     bool          `config:"allow_import_name_change"`
	UseCreatedDate            bool          `config:"use_created_date"`
	ListChunk                 int64         `config:"list_chunk"`
	Impersonate               string        `config:"impersonate"`
	AlternateExport           bool          `config:"alternate_export"`
	UploadCutoff              fs.SizeSuffix `config:"upload_cutoff"`
	ChunkSize                 fs.SizeSuffix `config:"chunk_size"`
	AcknowledgeAbuse          bool          `config:"acknowledge_abuse"`
	KeepRevisionForever       bool          `config:"keep_revision_forever"`
	V2DownloadMinSize         fs.SizeSuffix `config:"v2_download_min_size"`
	PacerMinSleep             fs.Duration   `config:"pacer_min_sleep"`
	PacerBurst                int           `config:"pacer_burst"`
}

// Fs represents a remote drive server
type Fs struct {
	name             string             // name of this remote
	root             string             // the path we are working on
	opt              Options            // parsed options
	features         *fs.Features       // optional features
	svc              *drive.Service     // the connection to the drive server
	v2Svc            *drive_v2.Service  // used to create download links for the v2 api
	client           *http.Client       // authorized client
	rootFolderID     string             // the id of the root folder
	dirCache         *dircache.DirCache // Map of directory path to directory id
	pacer            *pacer.Pacer       // To pace the API calls
	exportExtensions []string           // preferred extensions to download docs
	importMimeTypes  []string           // MIME types to convert to docs
	isTeamDrive      bool               // true if this is a team drive
}

type baseObject struct {
	fs           *Fs    // what this object is part of
	remote       string // The remote path
	id           string // Drive Id of this object
	modifiedDate string // RFC3339 time it was last modified
	mimeType     string // The object MIME type
	bytes        int64  // size of the object
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
	v2Download bool   // generate v2 download link ondemand
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

// shouldRetry determines whehter a given err rates being retried
func shouldRetry(err error) (bool, error) {
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
				return true, err
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
	for _, e := range slice {
		if e == s {
			return true
		}
	}
	return false
}

// Lists the directory required calling the user function on each item found
//
// If the user fn ever returns true then it early exits with found = true
//
// Search params: https://developers.google.com/drive/search-parameters
func (f *Fs) list(dirIDs []string, title string, directoriesOnly, filesOnly, includeAll bool, fn listFn) (found bool, err error) {
	var query []string
	if !includeAll {
		q := "trashed=" + strconv.FormatBool(f.opt.TrashedOnly)
		if f.opt.TrashedOnly {
			q = fmt.Sprintf("(mimeType='%s' or %s)", driveFolderType, q)
		}
		query = append(query, q)
	}
	// Search with sharedWithMe will always return things listed in "Shared With Me" (without any parents)
	// We must not filter with parent when we try list "ROOT" with drive-shared-with-me
	// If we need to list file inside those shared folders, we must search it without sharedWithMe
	parentsQuery := bytes.NewBufferString("(")
	for _, dirID := range dirIDs {
		if dirID == "" {
			continue
		}
		if parentsQuery.Len() > 1 {
			_, _ = parentsQuery.WriteString(" or ")
		}
		if f.opt.SharedWithMe && dirID == f.rootFolderID {
			_, _ = parentsQuery.WriteString("sharedWithMe=true")
		} else {
			_, _ = fmt.Fprintf(parentsQuery, "'%s' in parents", dirID)
		}
	}
	if parentsQuery.Len() > 1 {
		_ = parentsQuery.WriteByte(')')
		query = append(query, parentsQuery.String())
	}
	var stems []string
	if title != "" {
		// Escaping the backslash isn't documented but seems to work
		searchTitle := strings.Replace(title, `\`, `\\`, -1)
		searchTitle = strings.Replace(searchTitle, `'`, `\'`, -1)
		// Convert ／ to / for search
		searchTitle = strings.Replace(searchTitle, "／", "/", -1)

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
		query = append(query, fmt.Sprintf("mimeType='%s'", driveFolderType))
	}
	if filesOnly {
		query = append(query, fmt.Sprintf("mimeType!='%s'", driveFolderType))
	}
	list := f.svc.Files.List()
	if len(query) > 0 {
		list.Q(strings.Join(query, " and "))
		// fmt.Printf("list Query = %q\n", query)
	}
	if f.opt.ListChunk > 0 {
		list.PageSize(f.opt.ListChunk)
	}
	if f.isTeamDrive {
		list.TeamDriveId(f.opt.TeamDriveID)
		list.SupportsTeamDrives(true)
		list.IncludeTeamDriveItems(true)
		list.Corpora("teamDrive")
	}
	// If using appDataFolder then need to add Spaces
	if f.rootFolderID == "appDataFolder" {
		list.Spaces("appDataFolder")
	}

	var fields = partialFields

	if f.opt.AuthOwnerOnly {
		fields += ",owners"
	}

	fields = fmt.Sprintf("files(%s),nextPageToken", fields)

OUTER:
	for {
		var files *drive.FileList
		err = f.pacer.Call(func() (bool, error) {
			files, err = list.Fields(googleapi.Field(fields)).Do()
			return shouldRetry(err)
		})
		if err != nil {
			return false, errors.Wrap(err, "couldn't list directory")
		}
		for _, item := range files.Files {
			// Convert / to ／ for listing purposes
			item.Name = strings.Replace(item.Name, "/", "／", -1)
			// Check the case of items is correct since
			// the `=` operator is case insensitive.

			if title != "" && title != item.Name {
				found := false
				for _, stem := range stems {
					if stem == item.Name {
						found = true
						break
					}
				}
				if !found {
					continue
				}
				_, exportName, _, _ := f.findExportFormat(item)
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
func fixMimeType(mimeType string) string {
	mediaType, param, err := mime.ParseMediaType(mimeType)
	if err != nil {
		return mimeType
	}
	if strings.HasPrefix(mimeType, "text/") && param["charset"] == "" {
		param["charset"] = "utf-8"
		mimeType = mime.FormatMediaType(mediaType, param)
	}
	return mimeType
}
func fixMimeTypeMap(m map[string][]string) map[string][]string {
	for _, v := range m {
		for i, mt := range v {
			fixed := fixMimeType(mt)
			if fixed == "" {
				panic(errors.Errorf("unable to fix MIME type %q", mt))
			}
			v[i] = fixed
		}
	}
	return m
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
				return extensions, mimeTypes, errors.Errorf("couldn't find MIME type for extension %q", extension)
			}
			if !containsString(extensions, extension) {
				extensions = append(extensions, extension)
				mimeTypes = append(mimeTypes, mt)
			}
		}
	}
	return
}

// Figure out if the user wants to use a team drive
func configTeamDrive(opt *Options, m configmap.Mapper, name string) error {
	// Stop if we are running non-interactive config
	if fs.Config.AutoConfirm {
		return nil
	}
	if opt.TeamDriveID == "" {
		fmt.Printf("Configure this as a team drive?\n")
	} else {
		fmt.Printf("Change current team drive ID %q?\n", opt.TeamDriveID)
	}
	if !config.Confirm() {
		return nil
	}
	client, err := createOAuthClient(opt, name, m)
	if err != nil {
		return errors.Wrap(err, "config team drive failed to create oauth client")
	}
	svc, err := drive.New(client)
	if err != nil {
		return errors.Wrap(err, "config team drive failed to make drive client")
	}
	fmt.Printf("Fetching team drive list...\n")
	var driveIDs, driveNames []string
	listTeamDrives := svc.Teamdrives.List().PageSize(100)
	listFailed := false
	for {
		var teamDrives *drive.TeamDriveList
		err = newPacer(opt).Call(func() (bool, error) {
			teamDrives, err = listTeamDrives.Do()
			return shouldRetry(err)
		})
		if err != nil {
			fmt.Printf("Listing team drives failed: %v\n", err)
			listFailed = true
			break
		}
		for _, drive := range teamDrives.TeamDrives {
			driveIDs = append(driveIDs, drive.Id)
			driveNames = append(driveNames, drive.Name)
		}
		if teamDrives.NextPageToken == "" {
			break
		}
		listTeamDrives.PageToken(teamDrives.NextPageToken)
	}
	var driveID string
	if !listFailed && len(driveIDs) == 0 {
		fmt.Printf("No team drives found in your account")
	} else {
		driveID = config.Choose("Enter a Team Drive ID", driveIDs, driveNames, true)
	}
	m.Set("team_drive", driveID)
	opt.TeamDriveID = driveID
	return nil
}

// newPacer makes a pacer configured for drive
func newPacer(opt *Options) *pacer.Pacer {
	return pacer.New().SetMinSleep(time.Duration(opt.PacerMinSleep)).SetBurst(opt.PacerBurst).SetPacer(pacer.GoogleDrivePacer)
}

func getServiceAccountClient(opt *Options, credentialsData []byte) (*http.Client, error) {
	scopes := driveScopes(opt.Scope)
	conf, err := google.JWTConfigFromJSON(credentialsData, scopes...)
	if err != nil {
		return nil, errors.Wrap(err, "error processing credentials")
	}
	if opt.Impersonate != "" {
		conf.Subject = opt.Impersonate
	}
	ctxWithSpecialClient := oauthutil.Context(fshttp.NewClient(fs.Config))
	return oauth2.NewClient(ctxWithSpecialClient, conf.TokenSource(ctxWithSpecialClient)), nil
}

func createOAuthClient(opt *Options, name string, m configmap.Mapper) (*http.Client, error) {
	var oAuthClient *http.Client
	var err error

	// try loading service account credentials from env variable, then from a file
	if len(opt.ServiceAccountCredentials) == 0 && opt.ServiceAccountFile != "" {
		loadedCreds, err := ioutil.ReadFile(os.ExpandEnv(opt.ServiceAccountFile))
		if err != nil {
			return nil, errors.Wrap(err, "error opening service account credentials file")
		}
		opt.ServiceAccountCredentials = string(loadedCreds)
	}
	if opt.ServiceAccountCredentials != "" {
		oAuthClient, err = getServiceAccountClient(opt, []byte(opt.ServiceAccountCredentials))
		if err != nil {
			return nil, errors.Wrap(err, "failed to create oauth client from service account")
		}
	} else {
		oAuthClient, _, err = oauthutil.NewClient(name, m, driveConfig)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create oauth client")
		}
	}

	return oAuthClient, nil
}

func checkUploadChunkSize(cs fs.SizeSuffix) error {
	if !isPowerOfTwo(int64(cs)) {
		return errors.Errorf("%v isn't a power of two", cs)
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

// NewFs contstructs an Fs from the path, container:path
func NewFs(name, path string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	err = checkUploadCutoff(opt.UploadCutoff)
	if err != nil {
		return nil, errors.Wrap(err, "drive: upload cutoff")
	}
	err = checkUploadChunkSize(opt.ChunkSize)
	if err != nil {
		return nil, errors.Wrap(err, "drive: chunk size")
	}

	oAuthClient, err := createOAuthClient(opt, name, m)
	if err != nil {
		return nil, errors.Wrap(err, "drive: failed when making oauth client")
	}

	root, err := parseDrivePath(path)
	if err != nil {
		return nil, err
	}

	f := &Fs{
		name:  name,
		root:  root,
		opt:   *opt,
		pacer: newPacer(opt),
	}
	f.isTeamDrive = opt.TeamDriveID != ""
	f.features = (&fs.Features{
		DuplicateFiles:          true,
		ReadMimeType:            true,
		WriteMimeType:           true,
		CanHaveEmptyDirectories: true,
	}).Fill(f)

	// Create a new authorized Drive client.
	f.client = oAuthClient
	f.svc, err = drive.New(f.client)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't create Drive client")
	}

	if f.opt.V2DownloadMinSize >= 0 {
		f.v2Svc, err = drive_v2.New(f.client)
		if err != nil {
			return nil, errors.Wrap(err, "couldn't create Drive v2 client")
		}
	}

	// set root folder for a team drive or query the user root folder
	if f.isTeamDrive {
		f.rootFolderID = f.opt.TeamDriveID
	} else {
		f.rootFolderID = "root"
	}

	// override root folder if set in the config
	if opt.RootFolderID != "" {
		f.rootFolderID = opt.RootFolderID
	}

	f.dirCache = dircache.New(root, f.rootFolderID, f)

	// Parse extensions
	if opt.Extensions != "" {
		if opt.ExportExtensions != defaultExportExtensions {
			return nil, errors.New("only one of 'formats' and 'export_formats' can be specified")
		}
		opt.Extensions, opt.ExportExtensions = "", opt.Extensions
	}
	f.exportExtensions, _, err = parseExtensions(opt.ExportExtensions, defaultExportExtensions)
	if err != nil {
		return nil, err
	}

	_, f.importMimeTypes, err = parseExtensions(opt.ImportExtensions)
	if err != nil {
		return nil, err
	}

	// Find the current root
	err = f.dirCache.FindRoot(false)
	if err != nil {
		// Assume it is a file
		newRoot, remote := dircache.SplitPath(root)
		tempF := *f
		tempF.dirCache = dircache.New(newRoot, f.rootFolderID, &tempF)
		tempF.root = newRoot
		// Make new Fs which is the parent
		err = tempF.dirCache.FindRoot(false)
		if err != nil {
			// No root so return old f
			return f, nil
		}
		_, err := tempF.NewObject(remote)
		if err != nil {
			// unable to list folder so return old f
			return f, nil
		}
		// XXX: update the old f here instead of returning tempF, since
		// `features` were already filled with functions having *f as a receiver.
		// See https://github.com/ncw/rclone/issues/2182
		f.dirCache = tempF.dirCache
		f.root = tempF.root
		return f, fs.ErrorIsFile
	}
	// fmt.Printf("Root id %s", f.dirCache.RootID())
	return f, nil
}

func (f *Fs) newBaseObject(remote string, info *drive.File) baseObject {
	modifiedDate := info.ModifiedTime
	if f.opt.UseCreatedDate {
		modifiedDate = info.CreatedTime
	}
	return baseObject{
		fs:           f,
		remote:       remote,
		id:           info.Id,
		modifiedDate: modifiedDate,
		mimeType:     info.MimeType,
		bytes:        info.Size,
	}
}

// newRegularObject creates a fs.Object for a normal drive.File
func (f *Fs) newRegularObject(remote string, info *drive.File) fs.Object {
	return &Object{
		baseObject: f.newBaseObject(remote, info),
		url:        fmt.Sprintf("%sfiles/%s?alt=media", f.svc.BasePath, info.Id),
		md5sum:     strings.ToLower(info.Md5Checksum),
		v2Download: f.opt.V2DownloadMinSize != -1 && info.Size >= int64(f.opt.V2DownloadMinSize),
	}
}

// newDocumentObject creates a fs.Object for a google docs drive.File
func (f *Fs) newDocumentObject(remote string, info *drive.File, extension, exportMimeType string) (fs.Object, error) {
	mediaType, _, err := mime.ParseMediaType(exportMimeType)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("%sfiles/%s/export?mimeType=%s", f.svc.BasePath, info.Id, url.QueryEscape(mediaType))
	if f.opt.AlternateExport {
		switch info.MimeType {
		case "application/vnd.google-apps.drawing":
			url = fmt.Sprintf("https://docs.google.com/drawings/d/%s/export/%s", info.Id, extension[1:])
		case "application/vnd.google-apps.document":
			url = fmt.Sprintf("https://docs.google.com/document/d/%s/export?format=%s", info.Id, extension[1:])
		case "application/vnd.google-apps.spreadsheet":
			url = fmt.Sprintf("https://docs.google.com/spreadsheets/d/%s/export?format=%s", info.Id, extension[1:])
		case "application/vnd.google-apps.presentation":
			url = fmt.Sprintf("https://docs.google.com/presentation/d/%s/export/%s", info.Id, extension[1:])
		}
	}
	baseObject := f.newBaseObject(remote+extension, info)
	baseObject.bytes = -1
	baseObject.mimeType = exportMimeType
	return &documentObject{
		baseObject:       baseObject,
		url:              url,
		documentMimeType: info.MimeType,
		extLen:           len(extension),
	}, nil
}

// newLinkObject creates a fs.Object that represents a link a google docs drive.File
func (f *Fs) newLinkObject(remote string, info *drive.File, extension, exportMimeType string) (fs.Object, error) {
	t := linkTemplate(exportMimeType)
	if t == nil {
		return nil, errors.Errorf("unsupported link type %s", exportMimeType)
	}
	var buf bytes.Buffer
	err := t.Execute(&buf, struct {
		URL, Title string
	}{
		info.WebViewLink, info.Name,
	})
	if err != nil {
		return nil, errors.Wrap(err, "executing template failed")
	}

	baseObject := f.newBaseObject(remote+extension, info)
	baseObject.bytes = int64(buf.Len())
	baseObject.mimeType = exportMimeType
	return &linkObject{
		baseObject: baseObject,
		content:    buf.Bytes(),
		extLen:     len(extension),
	}, nil
}

// newObjectWithInfo creates a fs.Object for any drive.File
//
// When the drive.File cannot be represented as a fs.Object it will return (nil, nil).
func (f *Fs) newObjectWithInfo(remote string, info *drive.File) (fs.Object, error) {
	// If item has MD5 sum or a length it is a file stored on drive
	if info.Md5Checksum != "" || info.Size > 0 {
		return f.newRegularObject(remote, info), nil
	}

	extension, exportName, exportMimeType, isDocument := f.findExportFormat(info)
	return f.newObjectWithExportInfo(remote, info, extension, exportName, exportMimeType, isDocument)
}

// newObjectWithExportInfo creates a fs.Object for any drive.File and the result of findExportFormat
//
// When the drive.File cannot be represented as a fs.Object it will return (nil, nil).
func (f *Fs) newObjectWithExportInfo(
	remote string, info *drive.File,
	extension, exportName, exportMimeType string, isDocument bool) (fs.Object, error) {
	switch {
	case info.Md5Checksum != "" || info.Size > 0:
		// If item has MD5 sum or a length it is a file stored on drive
		return f.newRegularObject(remote, info), nil
	case f.opt.SkipGdocs:
		fs.Debugf(remote, "Skipping google document type %q", info.MimeType)
		return nil, nil
	default:
		// If item MimeType is in the ExportFormats then it is a google doc
		if !isDocument {
			fs.Debugf(remote, "Ignoring unknown document type %q", info.MimeType)
			return nil, nil
		}
		if extension == "" {
			fs.Debugf(remote, "No export formats found for %q", info.MimeType)
			return nil, nil
		}
		if isLinkMimeType(exportMimeType) {
			return f.newLinkObject(remote, info, extension, exportMimeType)
		}
		return f.newDocumentObject(remote, info, extension, exportMimeType)
	}
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(remote string) (fs.Object, error) {
	info, extension, exportName, exportMimeType, isDocument, err := f.getRemoteInfoWithExport(remote)
	if err != nil {
		return nil, err
	}

	remote = remote[:len(remote)-len(extension)]
	obj, err := f.newObjectWithExportInfo(remote, info, extension, exportName, exportMimeType, isDocument)
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
func (f *Fs) FindLeaf(pathID, leaf string) (pathIDOut string, found bool, err error) {
	// Find the leaf in pathID
	found, err = f.list([]string{pathID}, leaf, true, false, false, func(item *drive.File) bool {
		if !f.opt.SkipGdocs {
			_, exportName, _, isDocument := f.findExportFormat(item)
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

// CreateDir makes a directory with pathID as parent and name leaf
func (f *Fs) CreateDir(pathID, leaf string) (newID string, err error) {
	// fmt.Println("Making", path)
	// Define the metadata for the directory we are going to create.
	createInfo := &drive.File{
		Name:        leaf,
		Description: leaf,
		MimeType:    driveFolderType,
		Parents:     []string{pathID},
	}
	var info *drive.File
	err = f.pacer.Call(func() (bool, error) {
		info, err = f.svc.Files.Create(createInfo).
			Fields("id").
			SupportsTeamDrives(f.isTeamDrive).
			Do()
		return shouldRetry(err)
	})
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
func (f *Fs) fetchFormats() {
	fetchFormatsOnce.Do(func() {
		var about *drive.About
		var err error
		err = f.pacer.Call(func() (bool, error) {
			about, err = f.svc.About.Get().
				Fields("exportFormats,importFormats").
				Do()
			return shouldRetry(err)
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
func (f *Fs) exportFormats() map[string][]string {
	f.fetchFormats()
	return _exportFormats
}

// importFormats returns the import formats from drive, fetching them
// if necessary.
//
// if the fetch fails then it will not import any drive formats
func (f *Fs) importFormats() map[string][]string {
	f.fetchFormats()
	return _importFormats
}

// findExportFormatByMimeType works out the optimum export settings
// for the given MIME type.
//
// Look through the exportExtensions and find the first format that can be
// converted.  If none found then return ("", "", false)
func (f *Fs) findExportFormatByMimeType(itemMimeType string) (
	extension, mimeType string, isDocument bool) {
	exportMimeTypes, isDocument := f.exportFormats()[itemMimeType]
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

	// else return empty
	return "", "", isDocument
}

// findExportFormatByMimeType works out the optimum export settings
// for the given drive.File.
//
// Look through the exportExtensions and find the first format that can be
// converted.  If none found then return ("", "", "", false)
func (f *Fs) findExportFormat(item *drive.File) (extension, filename, mimeType string, isDocument bool) {
	extension, mimeType, isDocument = f.findExportFormatByMimeType(item.MimeType)
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
func (f *Fs) findImportFormat(mimeType string) string {
	mimeType = fixMimeType(mimeType)
	ifs := f.importFormats()
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
	_, err = f.list([]string{directoryID}, "", false, false, false, func(item *drive.File) bool {
		entry, err := f.itemToDirEntry(path.Join(dir, item.Name), item)
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
	return entries, nil
}

// listRRunner will read dirIDs from the in channel, perform the file listing an call cb with each DirEntry.
//
// In each cycle, will wait up to 10ms to read up to grouping entries from the in channel.
// If an error occurs it will be send to the out channel and then return. Once the in channel is closed,
// nil is send to the out channel and the function returns.
func (f *Fs) listRRunner(wg *sync.WaitGroup, in <-chan string, out chan<- error, cb func(fs.DirEntry) error, grouping int) {
	var dirs []string

	for dir := range in {
		dirs = append(dirs[:0], dir)
		wait := time.After(10 * time.Millisecond)
	waitloop:
		for i := 1; i < grouping; i++ {
			select {
			case d, ok := <-in:
				if !ok {
					break waitloop
				}
				dirs = append(dirs, d)
			case <-wait:
				break waitloop
			}
		}
		var iErr error
		_, err := f.list(dirs, "", false, false, false, func(item *drive.File) bool {
			parentPath := ""
			if len(item.Parents) > 0 {
				p, ok := f.dirCache.GetInv(item.Parents[0])
				if ok {
					parentPath = p
				}
			}
			remote := path.Join(parentPath, item.Name)
			entry, err := f.itemToDirEntry(remote, item)
			if err != nil {
				iErr = err
				return true
			}

			err = cb(entry)
			if err != nil {
				iErr = err
				return true
			}
			return false
		})
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
func (f *Fs) ListR(dir string, callback fs.ListRCallback) (err error) {
	const (
		grouping    = 50
		inputBuffer = 1000
	)

	err = f.dirCache.FindRoot(false)
	if err != nil {
		return err
	}
	directoryID, err := f.dirCache.FindDir(dir, false)
	if err != nil {
		return err
	}

	mu := sync.Mutex{} // protects in and overflow
	wg := sync.WaitGroup{}
	in := make(chan string, inputBuffer)
	out := make(chan error, fs.Config.Checkers)
	list := walk.NewListRHelper(callback)
	overfflow := []string{}

	cb := func(entry fs.DirEntry) error {
		mu.Lock()
		defer mu.Unlock()
		if d, isDir := entry.(*fs.Dir); isDir && in != nil {
			select {
			case in <- d.ID():
				wg.Add(1)
			default:
				overfflow = append(overfflow, d.ID())
			}
		}
		return list.Add(entry)
	}

	wg.Add(1)
	in <- directoryID

	for i := 0; i < fs.Config.Checkers; i++ {
		go f.listRRunner(&wg, in, out, cb, grouping)
	}
	go func() {
		// wait until the all directories are processed
		wg.Wait()
		// if the input channel overflowed add the collected entries to the channel now
		for len(overfflow) > 0 {
			mu.Lock()
			l := len(overfflow)
			// only fill half of the channel to prevent entries beeing put into overfflow again
			if l > inputBuffer/2 {
				l = inputBuffer / 2
			}
			wg.Add(l)
			for _, d := range overfflow[:l] {
				in <- d
			}
			overfflow = overfflow[l:]
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
	for i := 0; i < fs.Config.Checkers; i++ {
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

	return list.Flush()
}

// itemToDirEntry converts a drive.File to a fs.DirEntry.
// When the drive.File cannot be represented as a fs.DirEntry
// (nil, nil) is returned.
func (f *Fs) itemToDirEntry(remote string, item *drive.File) (fs.DirEntry, error) {
	switch {
	case item.MimeType == driveFolderType:
		// cache the directory ID for later lookups
		f.dirCache.Put(remote, item.Id)
		when, _ := time.Parse(timeFormatIn, item.ModifiedTime)
		d := fs.NewDir(remote, when).SetID(item.Id)
		return d, nil
	case f.opt.AuthOwnerOnly && !isAuthOwned(item):
		// ignore object
	default:
		return f.newObjectWithInfo(remote, item)
	}
	return nil, nil
}

// Creates a drive.File info from the parameters passed in.
//
// Used to create new objects
func (f *Fs) createFileInfo(remote string, modTime time.Time) (*drive.File, error) {
	leaf, directoryID, err := f.dirCache.FindRootAndPath(remote, true)
	if err != nil {
		return nil, err
	}

	// Define the metadata for the file we are going to create.
	createInfo := &drive.File{
		Name:         leaf,
		Description:  leaf,
		Parents:      []string{directoryID},
		MimeType:     fs.MimeTypeFromName(remote),
		ModifiedTime: modTime.Format(timeFormatOut),
	}
	return createInfo, nil
}

// Put the object
//
// Copy the reader in to the new object which is returned
//
// The new object may have been created if an error is returned
func (f *Fs) Put(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	exisitingObj, err := f.NewObject(src.Remote())
	switch err {
	case nil:
		return exisitingObj, exisitingObj.Update(in, src, options...)
	case fs.ErrorObjectNotFound:
		// Not found so create it
		return f.PutUnchecked(in, src, options...)
	default:
		return nil, err
	}
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
func (f *Fs) PutStream(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.Put(in, src, options...)
}

// PutUnchecked uploads the object
//
// This will create a duplicate if we upload a new file without
// checking to see if there is one already - use Put() for that.
func (f *Fs) PutUnchecked(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	remote := src.Remote()
	size := src.Size()
	modTime := src.ModTime()
	srcMimeType := fs.MimeTypeFromName(remote)
	srcExt := path.Ext(remote)
	exportExt := ""
	importMimeType := ""

	if f.importMimeTypes != nil && !f.opt.SkipGdocs {
		importMimeType = f.findImportFormat(srcMimeType)

		if isInternalMimeType(importMimeType) {
			remote = remote[:len(remote)-len(srcExt)]

			exportExt, _, _ = f.findExportFormatByMimeType(importMimeType)
			if exportExt == "" {
				return nil, errors.Errorf("No export format found for %q", importMimeType)
			}
			if exportExt != srcExt && !f.opt.AllowImportNameChange {
				return nil, errors.Errorf("Can't convert %q to a document with a different export filetype (%q)", srcExt, exportExt)
			}
		}
	}

	createInfo, err := f.createFileInfo(remote, modTime)
	if err != nil {
		return nil, err
	}
	if importMimeType != "" {
		createInfo.MimeType = importMimeType
	}

	var info *drive.File
	if size == 0 || size < int64(f.opt.UploadCutoff) {
		// Make the API request to upload metadata and file data.
		// Don't retry, return a retry error instead
		err = f.pacer.CallNoRetry(func() (bool, error) {
			info, err = f.svc.Files.Create(createInfo).
				Media(in, googleapi.ContentType(srcMimeType)).
				Fields(partialFields).
				SupportsTeamDrives(f.isTeamDrive).
				KeepRevisionForever(f.opt.KeepRevisionForever).
				Do()
			return shouldRetry(err)
		})
		if err != nil {
			return nil, err
		}
	} else {
		// Upload the file in chunks
		info, err = f.Upload(in, size, srcMimeType, "", remote, createInfo)
		if err != nil {
			return nil, err
		}
	}
	return f.newObjectWithInfo(remote, info)
}

// MergeDirs merges the contents of all the directories passed
// in into the first one and rmdirs the other directories.
func (f *Fs) MergeDirs(dirs []fs.Directory) error {
	if len(dirs) < 2 {
		return nil
	}
	dstDir := dirs[0]
	for _, srcDir := range dirs[1:] {
		// list the the objects
		infos := []*drive.File{}
		_, err := f.list([]string{srcDir.ID()}, "", false, false, true, func(info *drive.File) bool {
			infos = append(infos, info)
			return false
		})
		if err != nil {
			return errors.Wrapf(err, "MergeDirs list failed on %v", srcDir)
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
					SupportsTeamDrives(f.isTeamDrive).
					Do()
				return shouldRetry(err)
			})
			if err != nil {
				return errors.Wrapf(err, "MergDirs move failed on %q in %v", info.Name, srcDir)
			}
		}
		// rmdir (into trash) the now empty source directory
		fs.Infof(srcDir, "removing empty directory")
		err = f.rmdir(srcDir.ID(), true)
		if err != nil {
			return errors.Wrapf(err, "MergDirs move failed to rmdir %q", srcDir)
		}
	}
	return nil
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

// Rmdir deletes a directory unconditionally by ID
func (f *Fs) rmdir(directoryID string, useTrash bool) error {
	return f.pacer.Call(func() (bool, error) {
		var err error
		if useTrash {
			info := drive.File{
				Trashed: true,
			}
			_, err = f.svc.Files.Update(directoryID, &info).
				Fields("").
				SupportsTeamDrives(f.isTeamDrive).
				Do()
		} else {
			err = f.svc.Files.Delete(directoryID).
				Fields("").
				SupportsTeamDrives(f.isTeamDrive).
				Do()
		}
		return shouldRetry(err)
	})
}

// Rmdir deletes a directory
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(dir string) error {
	root := path.Join(f.root, dir)
	dc := f.dirCache
	directoryID, err := dc.FindDir(dir, false)
	if err != nil {
		return err
	}
	var trashedFiles = false
	found, err := f.list([]string{directoryID}, "", false, false, true, func(item *drive.File) bool {
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
		return errors.Errorf("directory not empty")
	}
	if root != "" {
		// trash the directory if it had trashed files
		// in or the user wants to trash, otherwise
		// delete it.
		err = f.rmdir(directoryID, trashedFiles || f.opt.UseTrash)
		if err != nil {
			return err
		}
	}
	f.dirCache.FlushDir(dir)
	if err != nil {
		return err
	}
	return nil
}

// Precision of the object storage system
func (f *Fs) Precision() time.Duration {
	return time.Millisecond
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
		fs.Debugf(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}

	if ext != "" {
		if !strings.HasSuffix(remote, ext) {
			fs.Debugf(src, "Can't copy - not same document type")
			return nil, fs.ErrorCantCopy
		}
		remote = remote[:len(remote)-len(ext)]
	}

	createInfo, err := f.createFileInfo(remote, src.ModTime())
	if err != nil {
		return nil, err
	}

	var info *drive.File
	err = f.pacer.Call(func() (bool, error) {
		info, err = f.svc.Files.Copy(srcObj.id, createInfo).
			Fields(partialFields).
			SupportsTeamDrives(f.isTeamDrive).
			KeepRevisionForever(f.opt.KeepRevisionForever).
			Do()
		return shouldRetry(err)
	})
	if err != nil {
		return nil, err
	}
	return f.newObjectWithInfo(remote, info)
}

// Purge deletes all the files and the container
//
// Optional interface: Only implement this if you have a way of
// deleting all the files quicker than just running Remove() on the
// result of List()
func (f *Fs) Purge() error {
	if f.root == "" {
		return errors.New("can't purge root directory")
	}
	err := f.dirCache.FindRoot(false)
	if err != nil {
		return err
	}
	err = f.pacer.Call(func() (bool, error) {
		if f.opt.UseTrash {
			info := drive.File{
				Trashed: true,
			}
			_, err = f.svc.Files.Update(f.dirCache.RootID(), &info).
				Fields("").
				SupportsTeamDrives(f.isTeamDrive).
				Do()
		} else {
			err = f.svc.Files.Delete(f.dirCache.RootID()).
				Fields("").
				SupportsTeamDrives(f.isTeamDrive).
				Do()
		}
		return shouldRetry(err)
	})
	f.dirCache.ResetRoot()
	if err != nil {
		return err
	}
	return nil
}

// CleanUp empties the trash
func (f *Fs) CleanUp() error {
	err := f.pacer.Call(func() (bool, error) {
		err := f.svc.Files.EmptyTrash().Do()
		return shouldRetry(err)
	})

	if err != nil {
		return err
	}
	return nil
}

// About gets quota information
func (f *Fs) About() (*fs.Usage, error) {
	if f.isTeamDrive {
		// Teamdrives don't appear to have a usage API so just return empty
		return &fs.Usage{}, nil
	}
	var about *drive.About
	var err error
	err = f.pacer.Call(func() (bool, error) {
		about, err = f.svc.About.Get().Fields("storageQuota").Do()
		return shouldRetry(err)
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get Drive storageQuota")
	}
	q := about.StorageQuota
	usage := &fs.Usage{
		Used:    fs.NewUsageValue(q.UsageInDrive),           // bytes in use
		Trashed: fs.NewUsageValue(q.UsageInDriveTrash),      // bytes in trash
		Other:   fs.NewUsageValue(q.Usage - q.UsageInDrive), // other usage eg gmail in drive
	}
	if q.Limit > 0 {
		usage.Total = fs.NewUsageValue(q.Limit)          // quota of bytes that can be used
		usage.Free = fs.NewUsageValue(q.Limit - q.Usage) // bytes which can be uploaded before reaching the quota
	}
	return usage, nil
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

	_, srcParentID, err := srcObj.fs.dirCache.FindPath(src.Remote(), false)
	if err != nil {
		return nil, err
	}

	// Temporary Object under construction
	dstInfo, err := f.createFileInfo(remote, src.ModTime())
	if err != nil {
		return nil, err
	}
	dstParents := strings.Join(dstInfo.Parents, ",")
	dstInfo.Parents = nil

	// Do the move
	var info *drive.File
	err = f.pacer.Call(func() (bool, error) {
		info, err = f.svc.Files.Update(srcObj.id, dstInfo).
			RemoveParents(srcParentID).
			AddParents(dstParents).
			Fields(partialFields).
			SupportsTeamDrives(f.isTeamDrive).
			Do()
		return shouldRetry(err)
	})
	if err != nil {
		return nil, err
	}

	return f.newObjectWithInfo(remote, info)
}

// PublicLink adds a "readable by anyone with link" permission on the given file or folder.
func (f *Fs) PublicLink(remote string) (link string, err error) {
	id, err := f.dirCache.FindDir(remote, false)
	if err == nil {
		fs.Debugf(f, "attempting to share directory '%s'", remote)
	} else {
		fs.Debugf(f, "attempting to share single file '%s'", remote)
		o, err := f.NewObject(remote)
		if err != nil {
			return "", err
		}
		id = o.(fs.IDer).ID()
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
			SupportsTeamDrives(f.isTeamDrive).
			Do()
		return shouldRetry(err)
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("https://drive.google.com/open?id=%s", id), nil
}

// DirMove moves src, srcRemote to this remote at dstRemote
// using server side move operations.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantDirMove
//
// If destination exists then return fs.ErrorDirExists
func (f *Fs) DirMove(src fs.Fs, srcRemote, dstRemote string) error {
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debugf(srcFs, "Can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}
	srcPath := path.Join(srcFs.root, srcRemote)
	dstPath := path.Join(f.root, dstRemote)

	// Refuse to move to or from the root
	if srcPath == "" || dstPath == "" {
		fs.Debugf(src, "DirMove error: Can't move root")
		return errors.New("can't move root directory")
	}

	// find the root src directory
	err := srcFs.dirCache.FindRoot(false)
	if err != nil {
		return err
	}

	// find the root dst directory
	if dstRemote != "" {
		err = f.dirCache.FindRoot(true)
		if err != nil {
			return err
		}
	} else {
		if f.dirCache.FoundRoot() {
			return fs.ErrorDirExists
		}
	}

	// Find ID of dst parent, creating subdirs if necessary
	var leaf, dstDirectoryID string
	findPath := dstRemote
	if dstRemote == "" {
		findPath = f.root
	}
	leaf, dstDirectoryID, err = f.dirCache.FindPath(findPath, true)
	if err != nil {
		return err
	}

	// Check destination does not exist
	if dstRemote != "" {
		_, err = f.dirCache.FindDir(dstRemote, false)
		if err == fs.ErrorDirNotFound {
			// OK
		} else if err != nil {
			return err
		} else {
			return fs.ErrorDirExists
		}
	}

	// Find ID of src parent
	var srcDirectoryID string
	if srcRemote == "" {
		srcDirectoryID, err = srcFs.dirCache.RootParentID()
	} else {
		_, srcDirectoryID, err = srcFs.dirCache.FindPath(srcRemote, false)
	}
	if err != nil {
		return err
	}

	// Find ID of src
	srcID, err := srcFs.dirCache.FindDir(srcRemote, false)
	if err != nil {
		return err
	}

	// Do the move
	patch := drive.File{
		Name: leaf,
	}
	err = f.pacer.Call(func() (bool, error) {
		_, err = f.svc.Files.Update(srcID, &patch).
			RemoveParents(srcDirectoryID).
			AddParents(dstDirectoryID).
			Fields("").
			SupportsTeamDrives(f.isTeamDrive).
			Do()
		return shouldRetry(err)
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
// Automatically restarts itself in case of unexpected behaviour of the remote.
//
// Close the returned channel to stop being notified.
func (f *Fs) ChangeNotify(notifyFunc func(string, fs.EntryType), pollIntervalChan <-chan time.Duration) {
	go func() {
		// get the StartPageToken early so all changes from now on get processed
		startPageToken, err := f.changeNotifyStartPageToken()
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
					startPageToken, err = f.changeNotifyStartPageToken()
					if err != nil {
						fs.Infof(f, "Failed to get StartPageToken: %s", err)
						continue
					}
				}
				fs.Debugf(f, "Checking for changes on remote")
				startPageToken, err = f.changeNotifyRunner(notifyFunc, startPageToken)
				if err != nil {
					fs.Infof(f, "Change notify listener failure: %s", err)
				}
			}
		}
	}()
}
func (f *Fs) changeNotifyStartPageToken() (pageToken string, err error) {
	var startPageToken *drive.StartPageToken
	err = f.pacer.Call(func() (bool, error) {
		startPageToken, err = f.svc.Changes.GetStartPageToken().
			SupportsTeamDrives(f.isTeamDrive).
			Do()
		return shouldRetry(err)
	})
	if err != nil {
		return
	}
	return startPageToken.StartPageToken, nil
}

func (f *Fs) changeNotifyRunner(notifyFunc func(string, fs.EntryType), startPageToken string) (newStartPageToken string, err error) {
	pageToken := startPageToken
	for {
		var changeList *drive.ChangeList

		err = f.pacer.Call(func() (bool, error) {
			changesCall := f.svc.Changes.List(pageToken).
				Fields("nextPageToken,newStartPageToken,changes(fileId,file(name,parents,mimeType))")
			if f.opt.ListChunk > 0 {
				changesCall.PageSize(f.opt.ListChunk)
			}
			if f.isTeamDrive {
				changesCall.TeamDriveId(f.opt.TeamDriveID)
				changesCall.SupportsTeamDrives(true)
				changesCall.IncludeTeamDriveItems(true)
			}
			changeList, err = changesCall.Do()
			return shouldRetry(err)
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
				changeType := fs.EntryDirectory
				if change.File.MimeType != driveFolderType {
					changeType = fs.EntryObject
				}

				// translate the parent dir of this object
				if len(change.File.Parents) > 0 {
					if parentPath, ok := f.dirCache.GetInv(change.File.Parents[0]); ok {
						// and append the drive file name to compute the full file name
						newPath := path.Join(parentPath, change.File.Name)
						// this will now clear the actual file too
						pathsToClear = append(pathsToClear, entryType{path: newPath, entryType: changeType})
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
	return hash.Set(hash.MD5)
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
func (o *Object) Hash(t hash.Type) (string, error) {
	if t != hash.MD5 {
		return "", hash.ErrUnsupported
	}
	return o.md5sum, nil
}
func (o *baseObject) Hash(t hash.Type) (string, error) {
	if t != hash.MD5 {
		return "", hash.ErrUnsupported
	}
	return "", nil
}

// Size returns the size of an object in bytes
func (o *baseObject) Size() int64 {
	return o.bytes
}

// getRemoteInfo returns a drive.File for the remote
func (f *Fs) getRemoteInfo(remote string) (info *drive.File, err error) {
	info, _, _, _, _, err = f.getRemoteInfoWithExport(remote)
	return
}

// getRemoteInfoWithExport returns a drive.File and the export settings for the remote
func (f *Fs) getRemoteInfoWithExport(remote string) (
	info *drive.File, extension, exportName, exportMimeType string, isDocument bool, err error) {
	leaf, directoryID, err := f.dirCache.FindRootAndPath(remote, false)
	if err != nil {
		if err == fs.ErrorDirNotFound {
			return nil, "", "", "", false, fs.ErrorObjectNotFound
		}
		return nil, "", "", "", false, err
	}

	found, err := f.list([]string{directoryID}, leaf, false, true, false, func(item *drive.File) bool {
		if !f.opt.SkipGdocs {
			extension, exportName, exportMimeType, isDocument = f.findExportFormat(item)
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
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *baseObject) ModTime() time.Time {
	modTime, err := time.Parse(timeFormatIn, o.modifiedDate)
	if err != nil {
		fs.Debugf(o, "Failed to read mtime from object: %v", err)
		return time.Now()
	}
	return modTime
}

// SetModTime sets the modification time of the drive fs object
func (o *baseObject) SetModTime(modTime time.Time) error {
	// New metadata
	updateInfo := &drive.File{
		ModifiedTime: modTime.Format(timeFormatOut),
	}
	// Set modified date
	var info *drive.File
	err := o.fs.pacer.Call(func() (bool, error) {
		var err error
		info, err = o.fs.svc.Files.Update(o.id, updateInfo).
			Fields(partialFields).
			SupportsTeamDrives(o.fs.isTeamDrive).
			Do()
		return shouldRetry(err)
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

// httpResponse gets an http.Response object for the object
// using the url and method passed in
func (o *baseObject) httpResponse(url, method string, options []fs.OpenOption) (req *http.Request, res *http.Response, err error) {
	if url == "" {
		return nil, nil, errors.New("forbidden to download - check sharing permission")
	}
	req, err = http.NewRequest(method, url, nil)
	if err != nil {
		return req, nil, err
	}
	fs.OpenOptionAddHTTPHeaders(req.Header, options)
	err = o.fs.pacer.Call(func() (bool, error) {
		res, err = o.fs.client.Do(req)
		if err == nil {
			err = googleapi.CheckResponse(res)
			if err != nil {
				_ = res.Body.Close() // ignore error
			}
		}
		return shouldRetry(err)
	})
	if err != nil {
		return req, nil, err
	}
	return req, res, nil
}

// openDocumentFile represents an documentObject open for reading.
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
func (o *baseObject) open(url string, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	_, res, err := o.httpResponse(url, "GET", options)
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
				_, res, err = o.httpResponse(url, "GET", options)
			} else {
				err = errors.Wrap(err, "Use the --drive-acknowledge-abuse flag to download this file")
			}
		}
		if err != nil {
			return nil, errors.Wrap(err, "open file failed")
		}
	}
	return res.Body, nil
}

// Open an object for read
func (o *Object) Open(options ...fs.OpenOption) (in io.ReadCloser, err error) {
	if o.v2Download {
		var v2File *drive_v2.File
		err = o.fs.pacer.Call(func() (bool, error) {
			v2File, err = o.fs.v2Svc.Files.Get(o.id).
				Fields("downloadUrl").
				SupportsTeamDrives(o.fs.isTeamDrive).
				Do()
			return shouldRetry(err)
		})
		if err == nil {
			fs.Debugf(o, "Using v2 download: %v", v2File.DownloadUrl)
			o.url = v2File.DownloadUrl
			o.v2Download = false
		}
	}
	return o.baseObject.open(o.url, options...)
}
func (o *documentObject) Open(options ...fs.OpenOption) (in io.ReadCloser, err error) {
	// Update the size with what we are reading as it can change from
	// the HEAD in the listing to this GET. This stops rclone marking
	// the transfer as corrupted.
	for _, o := range options {
		// https://developers.google.com/drive/v3/web/manage-downloads#partial_download
		if _, ok := o.(*fs.RangeOption); ok {
			return nil, errors.New("partial downloads are not supported while exporting Google Documents")
		}
	}
	in, err = o.baseObject.open(o.url, options...)
	if in != nil {
		in = &openDocumentFile{o: o, in: in}
	}
	return
}
func (o *linkObject) Open(options ...fs.OpenOption) (in io.ReadCloser, err error) {
	var offset, limit int64 = 0, -1
	var data = o.content
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

	return ioutil.NopCloser(bytes.NewReader(data)), nil
}

func (o *baseObject) update(updateInfo *drive.File, uploadMimeType string, in io.Reader,
	src fs.ObjectInfo) (info *drive.File, err error) {
	// Make the API request to upload metadata and file data.
	size := src.Size()
	if size == 0 || size < int64(o.fs.opt.UploadCutoff) {
		// Don't retry, return a retry error instead
		err = o.fs.pacer.CallNoRetry(func() (bool, error) {
			info, err = o.fs.svc.Files.Update(o.id, updateInfo).
				Media(in, googleapi.ContentType(uploadMimeType)).
				Fields(partialFields).
				SupportsTeamDrives(o.fs.isTeamDrive).
				KeepRevisionForever(o.fs.opt.KeepRevisionForever).
				Do()
			return shouldRetry(err)
		})
		return
	}
	// Upload the file in chunks
	return o.fs.Upload(in, size, uploadMimeType, o.id, o.remote, updateInfo)
}

// Update the already existing object
//
// Copy the reader into the object updating modTime and size
//
// The new object may have been created if an error is returned
func (o *Object) Update(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	srcMimeType := fs.MimeType(src)
	updateInfo := &drive.File{
		MimeType:     srcMimeType,
		ModifiedTime: src.ModTime().Format(timeFormatOut),
	}
	info, err := o.baseObject.update(updateInfo, srcMimeType, in, src)
	if err != nil {
		return err
	}
	newO, err := o.fs.newObjectWithInfo(src.Remote(), info)
	switch newO := newO.(type) {
	case *Object:
		*o = *newO
	default:
		return errors.New("object type changed by update")
	}

	return nil
}
func (o *documentObject) Update(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	srcMimeType := fs.MimeType(src)
	importMimeType := ""
	updateInfo := &drive.File{
		MimeType:     srcMimeType,
		ModifiedTime: src.ModTime().Format(timeFormatOut),
	}

	if o.fs.importMimeTypes == nil || o.fs.opt.SkipGdocs {
		return errors.Errorf("can't update google document type without --drive-import-formats")
	}
	importMimeType = o.fs.findImportFormat(updateInfo.MimeType)
	if importMimeType == "" {
		return errors.Errorf("no import format found for %q", srcMimeType)
	}
	if importMimeType != o.documentMimeType {
		return errors.Errorf("can't change google document type (o: %q, src: %q, import: %q)", o.documentMimeType, srcMimeType, importMimeType)
	}
	updateInfo.MimeType = importMimeType

	info, err := o.baseObject.update(updateInfo, srcMimeType, in, src)
	if err != nil {
		return err
	}

	remote := src.Remote()
	remote = remote[:len(remote)-o.extLen]

	newO, err := o.fs.newObjectWithInfo(remote, info)
	switch newO := newO.(type) {
	case *documentObject:
		*o = *newO
	default:
		return errors.New("object type changed by update")
	}

	return nil
}

func (o *linkObject) Update(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	return errors.New("cannot update link files")
}

// Remove an object
func (o *baseObject) Remove() error {
	var err error
	err = o.fs.pacer.Call(func() (bool, error) {
		if o.fs.opt.UseTrash {
			info := drive.File{
				Trashed: true,
			}
			_, err = o.fs.svc.Files.Update(o.id, &info).
				Fields("").
				SupportsTeamDrives(o.fs.isTeamDrive).
				Do()
		} else {
			err = o.fs.svc.Files.Delete(o.id).
				Fields("").
				SupportsTeamDrives(o.fs.isTeamDrive).
				Do()
		}
		return shouldRetry(err)
	})
	return err
}

// MimeType of an Object if known, "" otherwise
func (o *baseObject) MimeType() string {
	return o.mimeType
}

// ID returns the ID of the Object if known, or "" if not
func (o *baseObject) ID() string {
	return o.id
}

func (o *documentObject) ext() string {
	return o.baseObject.remote[len(o.baseObject.remote)-o.extLen:]
}
func (o *linkObject) ext() string {
	return o.baseObject.remote[len(o.baseObject.remote)-o.extLen:]
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
Icon=text-html
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
	_ fs.DirCacheFlusher = (*Fs)(nil)
	_ fs.ChangeNotifier  = (*Fs)(nil)
	_ fs.PutUncheckeder  = (*Fs)(nil)
	_ fs.PublicLinker    = (*Fs)(nil)
	_ fs.ListRer         = (*Fs)(nil)
	_ fs.MergeDirser     = (*Fs)(nil)
	_ fs.Abouter         = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.MimeTyper       = (*Object)(nil)
	_ fs.IDer            = (*Object)(nil)
	_ fs.Object          = (*documentObject)(nil)
	_ fs.MimeTyper       = (*documentObject)(nil)
	_ fs.IDer            = (*documentObject)(nil)
	_ fs.Object          = (*linkObject)(nil)
	_ fs.MimeTyper       = (*linkObject)(nil)
	_ fs.IDer            = (*linkObject)(nil)
)
