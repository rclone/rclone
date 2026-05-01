// Package huaweidrive implements the Huawei Drive backend for rclone
package huaweidrive

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/backend/huaweidrive/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/oauthutil"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
	"golang.org/x/oauth2"
)

const (
	rcloneClientID              = "115505059"
	rcloneEncryptedClientSecret = "EHQ1exJbwabjEMrIYc81zcWaSXg6n_SA_vzeiC34_Dfyfwys2RoNBbuDGwlNFbBFCrb_RYdU_DMLtaI5NnM9F9FriQAVHeRaN1oROHFZtCU"
	minSleep                    = 10 * time.Millisecond
	maxSleep                    = 2 * time.Second
	decayConstant               = 2 // bigger for slower decay, exponential
	rootURL                     = "https://driveapis.cloud.huawei.com.cn/drive/v1"
	uploadURL                   = "https://driveapis.cloud.huawei.com.cn/upload/drive/v1"
	defaultChunkSize            = 8 * fs.Mebi
)

// OAuth2 configuration
var oauthConfig = &oauthutil.Config{
	Scopes: []string{
		"openid",
		"profile",
		"https://www.huawei.com/auth/drive",
		"https://www.huawei.com/auth/drive.file",
	},
	AuthURL:      "https://oauth-login.cloud.huawei.com/oauth2/v3/authorize",
	TokenURL:     "https://oauth-login.cloud.huawei.com/oauth2/v3/token",
	ClientID:     rcloneClientID,
	ClientSecret: obscure.MustReveal(rcloneEncryptedClientSecret),
	RedirectURL:  oauthutil.RedirectURL,
	AuthStyle:    oauth2.AuthStyleInParams,
	EndpointParams: url.Values{
		"access_type": {"offline"},
	},
}

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "huaweidrive",
		Description: "Huawei Drive",
		NewFs:       NewFs,
		MetadataInfo: &fs.MetadataInfo{
			System: map[string]fs.MetadataHelp{
				"content-type": {
					Help:     "MIME type of the object",
					Type:     "string",
					ReadOnly: true,
				},
				"description": {
					Help:     "Description of the file",
					Type:     "string",
					ReadOnly: false,
				},
				"sha256": {
					Help:     "SHA256 hash of the file",
					Type:     "string",
					ReadOnly: true,
				},
				"btime": {
					Help:     "Time when the file was created (RFC 3339)",
					Type:     "RFC 3339",
					ReadOnly: true,
				},
				"mtime": {
					Help:     "Time when the file content was last modified (RFC 3339)",
					Type:     "RFC 3339",
					ReadOnly: true,
				},
				"utime": {
					Help:     "Time when the file was last edited by the current user (RFC 3339)",
					Type:     "RFC 3339",
					ReadOnly: true,
				},
				"favorite": {
					Help:     "Whether the file is marked as favorite (true/false)",
					Type:     "string",
					ReadOnly: false,
				},
				"recycled": {
					Help:     "Whether the file is in recycle bin (true/false)",
					Type:     "string",
					ReadOnly: true,
				},
				"has-thumbnail": {
					Help:     "Whether the file has a thumbnail (true/false)",
					Type:     "string",
					ReadOnly: true,
				},
			},
			Help: `Huawei Drive supports reading and writing custom metadata.

User metadata can be set using the "properties" field in the API.
System metadata includes standard file information such as:
- content-type: MIME type of the object
- description: User-provided description
- favorite: Whether file is marked as favorite
- btime/mtime/utime: Various timestamps
- sha256: File hash
- recycled/has-thumbnail: File status flags

Custom metadata keys can be any string and will be stored in the file's properties.`,
		},
		Config: func(ctx context.Context, name string, m configmap.Mapper, config fs.ConfigIn) (*fs.ConfigOut, error) {
			return oauthutil.ConfigOut("", &oauthutil.Options{
				OAuth2Config: oauthConfig,
			})
		},
		Options: append(oauthutil.SharedOptions, []fs.Option{{
			Name:     "root_folder_id",
			Help:     "ID of the root folder.\n\nNormally this is auto-detected, but if it fails or you want to speed up startup,\nyou can set it manually.\n\nLeave blank normally.",
			Default:  "",
			Advanced: true,
		}, {
			Name:     "chunk_size",
			Help:     "Upload chunk size.\n\nMust be a power of 2 >= 256k and <= 64MB.",
			Default:  defaultChunkSize,
			Advanced: true,
		}, {
			Name:     "list_chunk",
			Default:  1000,
			Help:     "Size of listing chunk 1-1000.",
			Advanced: true,
		}, {
			Name:     "upload_cutoff",
			Help:     "Cutoff for switching to resumable upload.\n\nAny files larger than this will be uploaded using resumable upload.\nThe minimum is 0 and the maximum is 20 MiB (Huawei Drive API limit for single request uploads).",
			Default:  20 * fs.Mebi,
			Advanced: true,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			// Huawei Drive has strict filename restrictions
			// API error: "The fileName can not be blank and can not contain '<>|:\"*?/\\', cannot equal .. or . and not exceed max limit."
			// Additionally encode slash and some Unicode characters that might cause issues
			Default: (encoder.Display |
				encoder.EncodeBackSlash |
				encoder.EncodeSlash |
				encoder.EncodeInvalidUtf8 |
				encoder.EncodeRightSpace |
				encoder.EncodeLeftSpace |
				encoder.EncodeLeftTilde |
				encoder.EncodeRightPeriod |
				encoder.EncodeLeftPeriod |
				encoder.EncodeColon |
				encoder.EncodePipe |
				encoder.EncodeDoubleQuote |
				encoder.EncodeLtGt |
				encoder.EncodeQuestion |
				encoder.EncodeAsterisk |
				encoder.EncodeCtl |
				encoder.EncodeDot),
		}}...),
	})
}

// Options defines the configuration for this backend
type Options struct {
	RootFolderID string               `config:"root_folder_id"`
	ChunkSize    fs.SizeSuffix        `config:"chunk_size"`
	ListChunk    int                  `config:"list_chunk"`
	UploadCutoff fs.SizeSuffix        `config:"upload_cutoff"`
	Enc          encoder.MultiEncoder `config:"encoding"`
}

// itemMeta stores cached metadata for an item, used to resolve old paths
// in ChangeNotify events (e.g. when a file is moved/renamed/deleted).
type itemMeta struct {
	parentID string // ID of the parent directory
	name     string // leaf name of the item
	isDir    bool   // whether this is a directory
}

// Fs represents a remote Huawei Drive
type Fs struct {
	name string // name of this remote
	root string // root path in the remote

	opt      Options            // parsed options
	features *fs.Features       // optional features
	srv      *rest.Client       // the connection to the server
	pacer    *fs.Pacer          // pacer for API calls
	dirCache *dircache.DirCache // Map of directory path to directory id

	rootFolderID string // ID of the root folder (detected at runtime)
	domainURL    string // regional domain URL from About:get
	uploadURL    string // upload URL (may be updated after domain detection)

	itemMetaCacheMu sync.Mutex          // protects itemMetaCache
	itemMetaCache   map[string]itemMeta // map of item ID to metadata for ChangeNotify
}

// Object describes a Huawei Drive object
type Object struct {
	fs          *Fs       // what this object is part of
	remote      string    // The remote path
	hasMetaData bool      // whether info below has been set
	size        int64     // size of the object
	modTime     time.Time // modification time of the object
	id          string    // ID of the object
	sha256      string    // SHA256 hash of the object
	mimeType    string    // Content-Type of object from server (may not be file extension)
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
	return fmt.Sprintf("Huawei Drive root '%s'", f.root)
}

// Precision return the precision of this Fs
func (f *Fs) Precision() time.Duration {
	// Huawei Drive API does NOT preserve file modification times
	// Despite accepting editedTime/createdTime parameters, the API
	// always sets file times to the server's current timestamp
	return fs.ModTimeNotSupported
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.SHA256)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// About gets quota information
func (f *Fs) About(ctx context.Context) (usage *fs.Usage, err error) {
	opts := rest.Opts{
		Method: "GET",
		Path:   "/about",
		Parameters: url.Values{
			"fields": []string{"*"},
		},
	}

	var about api.About
	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, nil, &about)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get about info: %w", err)
	}

	// Convert string values to int64
	used, err := strconv.ParseInt(about.StorageQuota.UsedSpace, 10, 64)
	if err != nil {
		fs.Debugf(f, "Failed to parse used space %q: %v", about.StorageQuota.UsedSpace, err)
		used = 0
	}

	total, err := strconv.ParseInt(about.StorageQuota.UserCapacity, 10, 64)
	if err != nil {
		fs.Debugf(f, "Failed to parse total capacity %q: %v", about.StorageQuota.UserCapacity, err)
		total = 0
	}

	usage = &fs.Usage{
		Used:  fs.NewUsageValue(used),  // bytes in use
		Total: fs.NewUsageValue(total), // bytes total
	}

	return usage, nil
}

// parsePath parses a Huawei Drive 'url'
func parsePath(path string) (root string) {
	root = strings.Trim(path, "/")
	return
}

// rootAlias is the special file ID alias understood by the Huawei Drive API
// to mean "this user's drive root". GET /files/root returns the metadata of
// the real root folder, including its concrete id.
const rootAlias = "root"

// detectRootID discovers the user's drive root folder ID deterministically
// by calling GET /files/root. The Huawei Drive API treats the literal
// string "root" as an alias for the user's drive root, so this is the
// authoritative way to obtain the real root id without any guessing.
func (f *Fs) detectRootID(ctx context.Context) error {
	opts := rest.Opts{
		Method: "GET",
		Path:   "/files/" + rootAlias,
		Parameters: url.Values{
			"fields": []string{"id,fileName,mimeType,parentFolder"},
		},
	}

	var info api.File
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, nil, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("failed to resolve drive root via /files/root: %w", err)
	}

	if info.ID == "" {
		return fmt.Errorf("drive root response did not include an id")
	}

	f.rootFolderID = info.ID
	fs.Debugf(f, "Resolved drive root folder ID: %s", f.rootFolderID)
	return nil
}

// detectDomain calls About:get to discover the regional domain for better performance.
func (f *Fs) detectDomain(ctx context.Context) {
	opts := rest.Opts{
		Method: "GET",
		Path:   "/about",
		Parameters: url.Values{
			"fields": []string{"domain"},
		},
	}

	var about api.About
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, nil, &about)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		fs.Debugf(f, "Failed to detect regional domain: %v", err)
		return
	}

	if about.Domain != "" {
		if api.GlobalDomains[about.Domain] {
			// Already using the global domain, no switch needed
			fs.Debugf(f, "Using global domain %q", about.Domain)
		} else if regional, ok := api.DomainToRootURL[about.Domain]; ok {
			f.domainURL = regional
			f.srv.SetRoot(regional + "/drive/v1")
			f.uploadURL = regional + "/upload/drive/v1"
			fs.Debugf(f, "Switched to regional domain %q -> %s", about.Domain, regional)
		} else {
			fs.Debugf(f, "Unknown domain %q, keeping global domain", about.Domain)
		}
	}
}

// NewFs constructs an Fs from the path, container:path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	root = parsePath(root)

	// Create OAuth2 client - oauthutil.NewClient handles credential overrides automatically
	client, _, err := oauthutil.NewClient(ctx, name, m, oauthConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to configure Huawei Drive OAuth: %w", err)
	}

	// Create REST client with global domain initially
	srv := rest.NewClient(client).SetRoot(rootURL)

	f := &Fs{
		name:          name,
		root:          root,
		opt:           *opt,
		srv:           srv,
		pacer:         fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
		uploadURL:     uploadURL,
		itemMetaCache: make(map[string]itemMeta),
	}

	f.features = (&fs.Features{
		CaseInsensitive:         false,
		DuplicateFiles:          false,
		ReadMimeType:            true,
		WriteMimeType:           true,
		CanHaveEmptyDirectories: true,
		ReadMetadata:            true,
		WriteMetadata:           true,
		UserMetadata:            true,
	}).Fill(ctx, f)

	// Set root folder ID
	if f.opt.RootFolderID != "" {
		f.rootFolderID = f.opt.RootFolderID
	} else {
		// Detect root folder ID by listing files
		err = f.detectRootID(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to detect root folder ID: %w", err)
		}
		fs.Debugf(f, "'root_folder_id = %s' - save this in the config to speed up startup", f.rootFolderID)
	}

	// Detect regional domain via About:get
	f.detectDomain(ctx)

	// Create directory cache with detected root folder ID
	f.dirCache = dircache.New(root, f.rootFolderID, f)

	// Find the current root
	err = f.dirCache.FindRoot(ctx, false)
	if err != nil {
		// Assume it is a file
		newRoot, remote := dircache.SplitPath(root)
		tempF := &Fs{
			name:         f.name,
			root:         newRoot,
			opt:          f.opt,
			features:     f.features,
			srv:          f.srv,
			pacer:        f.pacer,
			rootFolderID: f.rootFolderID,
			domainURL:    f.domainURL,
			uploadURL:    f.uploadURL,
		}
		tempF.dirCache = dircache.New(newRoot, f.rootFolderID, tempF)
		_ = tempF.dirCache.FindRoot(ctx, false)

		_, fileErr := tempF.NewObject(ctx, remote)
		if fileErr == nil {
			f.dirCache = tempF.dirCache
			f.root = tempF.root
			return f, fs.ErrorIsFile
		}
		return f, nil
	}

	return f, nil
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
	// Encode the leaf name to match the stored (encoded) name in the API
	encodedLeaf := f.opt.Enc.FromStandardName(leaf)
	// Find the leaf in pathID
	found, err = f.listAll(ctx, pathID, func(item *api.File) bool {
		if item.FileName == encodedLeaf && item.IsDir() {
			pathIDOut = item.ID
			return true
		}
		return false
	})
	return pathIDOut, found, err
}

// CreateDir makes a directory with pathID as parent and name leaf
func (f *Fs) CreateDir(ctx context.Context, pathID, leaf string) (newID string, err error) {
	// Encode the directory name
	encodedLeaf := f.opt.Enc.FromStandardName(leaf)
	if encodedLeaf == "" {
		return "", fmt.Errorf("invalid directory name: cannot be empty")
	}

	// Create the directory
	var req = api.CreateFolderRequest{
		FileName:    encodedLeaf,
		MimeType:    api.FolderMimeType,
		Description: "folder",
	}

	// Set parent folder for the new directory
	if pathID != "" {
		req.ParentFolder = []string{pathID}
	}

	opts := rest.Opts{
		Method: "POST",
		Path:   "/files",
		Parameters: url.Values{
			"fields": []string{"*"},
		},
	}

	var info *api.File
	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, &req, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return "", fmt.Errorf("couldn't create directory %q: %w", leaf, err)
	}

	if info == nil || info.ID == "" {
		return "", fmt.Errorf("invalid response when creating directory %q", leaf)
	}

	return info.ID, nil
}

// list the objects into the function supplied
//
// If directories is set it only sends directories
// User function to process a File item from listAll
//
// Should return true to finish processing
type listAllFn func(*api.File) bool

// Lists the directory required calling the user function on each item found
//
// If the user fn ever returns true then it early exits with found = true
func (f *Fs) listAll(ctx context.Context, dirID string, fn listAllFn) (found bool, err error) {
	// For non-root directories, we can directly query with parentFolder filter
	if dirID != "" {
		return f.listDirectory(ctx, dirID, fn)
	}

	// For root directory (empty dirID), use detected root folder ID.
	// detectRootID always resolves a concrete id via /files/root, so this
	// must be set before any list call reaches us.
	if f.rootFolderID == "" {
		return false, fmt.Errorf("listAll called with empty dirID and no root folder ID resolved")
	}
	return f.listDirectory(ctx, f.rootFolderID, fn)
}

// listDirectory lists files in a specific directory using parentFolder filter
func (f *Fs) listDirectory(ctx context.Context, dirID string, fn listAllFn) (found bool, err error) {
	return f.listDirectoryWithFilter(ctx, dirID, nil, fn)
}

// listDirectoryWithFilter lists files in a specific directory with optional additional filters
func (f *Fs) listDirectoryWithFilter(ctx context.Context, dirID string, extraFilter *QueryFilter, fn listAllFn) (found bool, err error) {
	// Create base query filter for parent folder
	filter := NewQueryFilter()
	filter.AddParentFolder(dirID)

	// Add any extra filters
	if extraFilter != nil {
		filter.Conditions = append(filter.Conditions, extraFilter.Conditions...)
	}

	opts := rest.Opts{
		Method: "GET",
		Path:   "/files",
		Parameters: url.Values{
			"fields":     []string{"*"},
			"containers": []string{"drive"},
		},
	}

	// Add query filter if we have conditions
	queryParam := filter.String()
	if queryParam != "" {
		opts.Parameters.Set("queryParam", queryParam)
	}

	// Set page size if specified
	if f.opt.ListChunk > 0 && f.opt.ListChunk <= 1000 {
		opts.Parameters.Set("pageSize", strconv.Itoa(f.opt.ListChunk))
	}

	fs.Debugf(f, "Listing directory %q", dirID)

	for {
		var result api.FileList
		err = f.pacer.Call(func() (bool, error) {
			resp, err := f.srv.CallJSON(ctx, &opts, nil, &result)
			return shouldRetry(ctx, resp, err)
		})
		if err != nil {
			return false, fmt.Errorf("couldn't list directory %q: %w", dirID, err)
		}

		// Process files directly - no need for further filtering since API does it for us
		for _, item := range result.Files {
			if fn(&item) {
				found = true
				return found, nil
			}
		}

		// Check for more pages
		if result.NextPageToken == "" {
			break
		}
		opts.Parameters.Set("pageToken", result.NextPageToken)
	}

	return found, nil
}

// QueryFilter represents a Huawei Drive query filter
type QueryFilter struct {
	Conditions []string
}

// NewQueryFilter creates a new empty query filter
func NewQueryFilter() *QueryFilter {
	return &QueryFilter{}
}

// AddParentFolder adds a parent folder filter condition
func (q *QueryFilter) AddParentFolder(parentID string) {
	if parentID != "" {
		q.Conditions = append(q.Conditions, fmt.Sprintf("'%s' in parentFolder", parentID))
	}
}

// AddMimeType adds a mime type filter condition
func (q *QueryFilter) AddMimeType(mimeType string) {
	if mimeType != "" {
		q.Conditions = append(q.Conditions, fmt.Sprintf("mimeType = '%s'", mimeType))
	}
}

// AddMimeTypeNot adds a negative mime type filter condition
func (q *QueryFilter) AddMimeTypeNot(mimeType string) {
	if mimeType != "" {
		q.Conditions = append(q.Conditions, fmt.Sprintf("mimeType != '%s'", mimeType))
	}
}

// AddFileName adds a file name filter condition
func (q *QueryFilter) AddFileName(fileName string) {
	if fileName != "" {
		// Escape single quotes in filename
		escapedName := strings.ReplaceAll(fileName, "'", "\\'")
		q.Conditions = append(q.Conditions, fmt.Sprintf("fileName = '%s'", escapedName))
	}
}

// AddFileNameContains adds a file name contains filter condition
func (q *QueryFilter) AddFileNameContains(fileName string) {
	if fileName != "" {
		// Escape single quotes in filename
		escapedName := strings.ReplaceAll(fileName, "'", "\\'")
		q.Conditions = append(q.Conditions, fmt.Sprintf("fileName contains '%s'", escapedName))
	}
}

// AddRecycled adds a recycled status filter condition
func (q *QueryFilter) AddRecycled(recycled bool) {
	q.Conditions = append(q.Conditions, fmt.Sprintf("recycled = %t", recycled))
}

// AddDirectlyRecycled adds a directly recycled status filter condition
func (q *QueryFilter) AddDirectlyRecycled(directlyRecycled bool) {
	q.Conditions = append(q.Conditions, fmt.Sprintf("directlyRecycled = %t", directlyRecycled))
}

// AddFavorite adds a favorite status filter condition
func (q *QueryFilter) AddFavorite(favorite bool) {
	q.Conditions = append(q.Conditions, fmt.Sprintf("favorite = %t", favorite))
}

// AddEditedTimeRange adds an edited time range filter condition
func (q *QueryFilter) AddEditedTimeRange(operator string, editedTime time.Time) {
	if !editedTime.IsZero() {
		timeStr := editedTime.Format(time.RFC3339)
		q.Conditions = append(q.Conditions, fmt.Sprintf("editedTime %s '%s'", operator, timeStr))
	}
}

// String returns the query filter as a string for the API
func (q *QueryFilter) String() string {
	if len(q.Conditions) == 0 {
		return ""
	}
	return strings.Join(q.Conditions, " and ")
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
	directoryID, err = f.ensureDirectoryID(ctx, dir, directoryID)
	if err != nil {
		return nil, err
	}
	var iErr error
	_, err = f.listAll(ctx, directoryID, func(info *api.File) bool {
		remote := path.Join(dir, f.opt.Enc.ToStandardName(info.FileName))
		if info.IsDir() {
			// cache the directory ID for later lookups
			f.dirCache.Put(remote, info.ID)
			d := fs.NewDir(remote, info.EditedTime).SetID(info.ID)
			d.SetItems(0) // Huawei Drive doesn't return item count
			entries = append(entries, d)
		} else {
			o, err := f.newObjectWithInfo(ctx, remote, info)
			if err != nil {
				iErr = err
				return true
			}
			entries = append(entries, o)
		}
		// Cache metadata for ChangeNotify old path resolution
		f.itemMetaCacheMu.Lock()
		f.itemMetaCache[info.ID] = itemMeta{
			parentID: directoryID,
			name:     info.FileName,
			isDir:    info.IsDir(),
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

func (f *Fs) ensureDirectoryID(ctx context.Context, dir, directoryID string) (string, error) {
	if f.root == "" || f.rootFolderID == "" || directoryID != f.rootFolderID {
		return directoryID, nil
	}
	f.dirCache.ResetRoot()
	return f.dirCache.FindDir(ctx, dir, false)
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
		fs:      f,
		remote:  remote,
		modTime: modTime, // Store the modification time in the object
		size:    size,    // Store the size in the object
	}
	return o, leaf, directoryID, nil
}

// Put the object into the container
//
// Copy the reader in to the new object which is returned.
//
// The new object may have been created if an error is returned
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	existingObj, err := f.newObjectWithInfo(ctx, src.Remote(), nil)
	if err == nil {
		err = existingObj.Update(ctx, in, src, options...)
		if err != nil {
			return nil, err
		}
		return existingObj, nil
	}
	if err != fs.ErrorObjectNotFound {
		return nil, err
	}
	// The object doesn't exist so create it
	o, err := f.PutUnchecked(ctx, in, src, options...)
	if err == nil {
		return o, nil
	}
	// PutUnchecked may fail because a sibling goroutine raced us and a file
	// with the same name already exists on the server (the server rejects
	// auto-rename, see uploadMultipart). Recover by locating the existing
	// object and updating it.
	if !isFileExistsErr(err) {
		return nil, err
	}
	existingObj, lookupErr := f.newObjectWithInfo(ctx, src.Remote(), nil)
	if lookupErr != nil {
		return nil, err
	}
	if updateErr := existingObj.Update(ctx, in, src, options...); updateErr != nil {
		return nil, updateErr
	}
	return existingObj, nil
}

// isFileExistsErr reports whether err is the Huawei Drive "file already
// exists" / "auto-rename rejected" error returned when we set autoRename=3.
func isFileExistsErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	// 21004014 / FILENAME_EXISTED is the documented duplicate-name code.
	return strings.Contains(msg, "21004014") ||
		strings.Contains(msg, "FILENAME_EXISTED") ||
		strings.Contains(msg, "FILE_NAME_EXISTED") ||
		strings.Contains(msg, "FILE_EXIST") ||
		strings.Contains(msg, "FILE_ALREADY_EXIST") ||
		strings.Contains(msg, "DUPLICATE_FILE_NAME")
}

// PutUnchecked the object into the container
//
// This will produce duplicates if we haven't checked that there
// is an existing object first.
//
// Copy the reader in to the new object which is returned.
//
// The new object may have been created if an error is returned.
func (f *Fs) PutUnchecked(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	remote := src.Remote()
	size := src.Size()
	modTime := src.ModTime(ctx)

	o, _, _, err := f.createObject(ctx, remote, modTime, size)
	if err != nil {
		return nil, err
	}
	err = o.Update(ctx, in, src, options...)
	if err != nil {
		return nil, err
	}
	return o, nil
}

// Mkdir creates the container if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	_, err := f.dirCache.FindDir(ctx, dir, true)
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
	rootID, err := dc.FindDir(ctx, dir, false)
	if err != nil {
		return err
	}

	if check {
		found, err := f.listAll(ctx, rootID, func(item *api.File) bool {
			return true // return found
		})
		if err != nil {
			return err
		}
		if found {
			return fs.ErrorDirectoryNotEmpty
		}
	}

	opts := rest.Opts{
		Method: "DELETE",
		Path:   "/files/" + rootID,
	}
	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.Call(ctx, &opts)
		return shouldRetry(ctx, resp, err)
	})
	if err == nil {
		// Clear the directory cache after successful deletion
		f.dirCache.FlushDir(dir)
	}
	return err
}

// Rmdir deletes the root folder
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir, true)
}

// Purge deletes all the files in the directory
//
// Optional interface: Only implement this if you have a way of
// deleting all the files quicker than just running Remove() on the
// result of List()
func (f *Fs) Purge(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir, false)
}

// Copy src to this remote using server-side copy operations.
//
// This is stored with the remote path given.
//
// It returns the destination Object and a possible error.
//
// Will only be called if src.Fs().Name() == f.Name().
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
	if srcPath == dstPath {
		return nil, fmt.Errorf("can't copy %q -> %q as they are same name", srcPath, dstPath)
	}

	// Check if destination file already exists
	dstObj, err := f.NewObject(ctx, remote)
	if err == nil {
		// Destination exists, we need to delete it first to ensure proper overwrite
		fs.Debugf(f, "Copy: destination file exists, deleting before copy")
		err = dstObj.Remove(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to remove existing destination file: %w", err)
		}
	} else if err != fs.ErrorObjectNotFound {
		return nil, err
	}

	// Create the directory for the object if it doesn't exist
	leaf, directoryID, err := f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return nil, err
	}

	// Copy the object
	opts := rest.Opts{
		Method: "POST",
		Path:   "/files/" + srcObj.id + "/copy",
		Parameters: url.Values{
			"fields": []string{"*"},
		},
	}

	copyReq := api.CopyFileRequest{
		FileName: f.opt.Enc.FromStandardName(leaf),
	}

	// Set parent folder for the copy destination
	if directoryID != "" {
		copyReq.ParentFolder = []string{directoryID}
	}

	// Copy user metadata: merge source metadata with MetadataSet overrides
	mergedMeta, err := fs.GetMetadataOptions(ctx, f, src, fs.MetadataAsOpenOptions(ctx))
	if err != nil {
		fs.Debugf(f, "Copy: failed to get metadata options: %v", err)
	}
	if len(mergedMeta) > 0 {
		copyReq.Properties = make(map[string]interface{})
		for key, value := range mergedMeta {
			switch key {
			case "content-type", "sha256", "btime", "mtime", "utime",
				"recycled", "has-thumbnail":
				// Skip read-only system metadata
			case "description":
				// Description is a top-level field, not a property
			case "favorite":
				// Favorite is not part of CopyFileRequest
			default:
				copyReq.Properties[key] = value
			}
		}
		if len(copyReq.Properties) == 0 {
			copyReq.Properties = nil
		}
	}
	// If directoryID is empty or is the root parent ID, don't set parentFolder
	// The API will copy the file to the drive root by default

	var info *api.File
	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, &copyReq, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}

	// Create the new object
	dstObj, newErr := f.newObjectWithInfo(ctx, remote, info)
	if newErr != nil {
		return nil, newErr
	}

	// If metadata was requested but not properly preserved, set it manually
	if len(mergedMeta) > 0 {
		if huaweiObj, ok := dstObj.(*Object); ok {
			if dstMetadata, err := huaweiObj.Metadata(ctx); err == nil {
				// Check if all merged metadata was preserved
				allMatched := true
				for key, expectedValue := range mergedMeta {
					if actualValue, exists := dstMetadata[key]; !exists || actualValue != expectedValue {
						allMatched = false
						break
					}
				}

				// If metadata doesn't match exactly, set it manually
				if !allMatched {
					fs.Debugf(f, "Copy: metadata not preserved by API, setting manually")
					if err := huaweiObj.SetMetadata(ctx, mergedMeta); err != nil {
						fs.Errorf(f, "Copy: failed to set metadata on copied file: %v", err)
					}
				}
			}
		}
	}

	return dstObj, nil
}

// Move src to this remote using server-side move operations.
//
// This is stored with the remote path given.
//
// It returns the destination Object and a possible error.
//
// Will only be called if src.Fs().Name() == f.Name().
//
// If it isn't possible then return fs.ErrorCantMove
//
// NOTE: Huawei Drive API has limitations with cross-directory moves:
// - Same-directory renames work perfectly
// - Cross-directory moves often fail silently (API returns success but file doesn't move)
// - We detect this by verifying the parentFolder after the API call
// - When detected, we return fs.ErrorCantMove to trigger rclone's copy+delete fallback
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't move - not same remote type")
		return nil, fs.ErrorCantMove
	}

	// Find the destination directory and leaf name
	dstLeaf, dstDirectoryID, err := f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return nil, err
	}

	// Get current parent folders
	currentParents := srcObj.getParentIDs(ctx)

	// Check what type of operation we need
	needsDirMove := len(currentParents) == 0 || currentParents[0] != dstDirectoryID
	srcLeaf := path.Base(srcObj.remote)
	needsRename := srcLeaf != dstLeaf

	// If no changes needed, return the existing object
	if !needsDirMove && !needsRename {
		return src, nil
	}

	// Get merged metadata BEFORE the move, because after renaming/moving
	// the source path will no longer be valid for metadata reads
	mergedMeta, _ := fs.GetMetadataOptions(ctx, f, src, fs.MetadataAsOpenOptions(ctx))

	moveReq := api.UpdateFileRequest{}

	// Set filename if we need to rename
	if needsRename {
		moveReq.FileName = f.opt.Enc.FromStandardName(dstLeaf)
	}

	var info *api.File
	if needsDirMove {
		if len(currentParents) == 0 || dstDirectoryID == "" {
			return nil, fs.ErrorCantMove
		}
		info, err = f.updateFile(ctx, srcObj.id, url.Values{
			"addParentFolder":    []string{dstDirectoryID},
			"removeParentFolder": []string{currentParents[0]},
		}, &moveReq)
	} else {
		info, err = f.updateFile(ctx, srcObj.id, nil, &moveReq)
	}
	if err != nil {
		return nil, fmt.Errorf("move operation failed: %w", err)
	}

	// Verify cross-directory moves actually worked.
	if needsDirMove {
		if info == nil || len(info.ParentFolder) == 0 {
			fs.Debugf(f, "Cross-directory move returned no parentFolder. Falling back to copy+delete.")
			return nil, fs.ErrorCantMove
		}
		parentFound := false
		for _, actualParent := range info.ParentFolder {
			if actualParent == dstDirectoryID {
				parentFound = true
				break
			}
		}
		if !parentFound {
			fs.Debugf(f, "Cross-directory move failed: expected parent=%q, got=%v. Falling back to copy+delete.",
				dstDirectoryID, info.ParentFolder)
			return nil, fs.ErrorCantMove
		}
	}

	dstObj, err := f.newObjectWithInfo(ctx, remote, info)
	if err != nil {
		return nil, err
	}

	// Apply metadata overrides if any (using pre-move merged metadata)
	if len(mergedMeta) > 0 {
		if huaweiObj, ok := dstObj.(*Object); ok {
			if err := huaweiObj.SetMetadata(ctx, mergedMeta); err != nil {
				fs.Debugf(f, "Move: failed to set metadata: %v", err)
			}
		}
	}

	return dstObj, nil
}

func (f *Fs) updateFile(ctx context.Context, fileID string, parameters url.Values, req *api.UpdateFileRequest) (info *api.File, err error) {
	if parameters == nil {
		parameters = url.Values{}
	}
	parameters.Set("fields", "*")
	opts := rest.Opts{
		Method:     "PATCH",
		Path:       "/files/" + fileID,
		Parameters: parameters,
	}
	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, req, &info)
		return shouldRetry(ctx, resp, err)
	})
	return info, err
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
	_ = srcLeaf // not used in this implementation

	fs.Debugf(f, "DirMove: srcID=%q srcDirectoryID=%q srcLeaf=%q dstDirectoryID=%q dstLeaf=%q", srcID, srcDirectoryID, srcLeaf, dstDirectoryID, dstLeaf)

	moveReq := api.UpdateFileRequest{
		FileName: f.opt.Enc.FromStandardName(dstLeaf),
	}

	var info *api.File
	if dstDirectoryID != srcDirectoryID {
		if srcDirectoryID == "" || dstDirectoryID == "" {
			return fmt.Errorf("can't move directory: source or destination parent ID is empty")
		}
		info, err = f.updateFile(ctx, srcID, url.Values{
			"addParentFolder":    []string{dstDirectoryID},
			"removeParentFolder": []string{srcDirectoryID},
		}, &moveReq)
	} else {
		info, err = f.updateFile(ctx, srcID, nil, &moveReq)
	}
	if err != nil {
		return err
	}

	// Verify the move worked
	if info != nil {
		fs.Debugf(f, "DirMove result: id=%q fileName=%q parentFolder=%v", info.ID, info.FileName, info.ParentFolder)
	}

	// Clear the directory cache for both source and destination
	srcFs.dirCache.FlushDir(srcRemote)
	f.dirCache.FlushDir(dstRemote)
	return nil
}

// CleanUp empties the trash
func (f *Fs) CleanUp(ctx context.Context) error {
	opts := rest.Opts{
		Method: "DELETE",
		Path:   "/files/recycle",
		Parameters: url.Values{
			"containers": []string{"drive"},
		},
	}

	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.Call(ctx, &opts)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("failed to empty recycle bin: %w", err)
	}

	fs.Debugf(f, "Successfully emptied recycle bin")
	return nil
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
	directoryID, err = f.ensureDirectoryID(ctx, dir, directoryID)
	if err != nil {
		return err
	}

	// Use a simple recursive approach instead of goroutines to avoid complexity
	return f.listRRecursive(ctx, dir, directoryID, callback)
}

// listRRecursive is a helper function for ListR that recursively lists directories
func (f *Fs) listRRecursive(ctx context.Context, dir string, directoryID string, callback fs.ListRCallback) error {
	var entries fs.DirEntries
	var dirs []struct {
		path string
		id   string
	}

	// List current directory
	_, err := f.listAll(ctx, directoryID, func(info *api.File) bool {
		remote := path.Join(dir, f.opt.Enc.ToStandardName(info.FileName))

		if info.IsDir() {
			// It's a directory
			directory := fs.NewDir(remote, info.EditedTime).SetID(info.ID)
			entries = append(entries, directory)

			// Store directory for later recursive processing
			dirs = append(dirs, struct {
				path string
				id   string
			}{remote, info.ID})
		} else {
			// It's a file
			o, err := f.newObjectWithInfo(ctx, remote, info)
			if err != nil {
				fs.Debugf(remote, "Skipping file in ListR: %v", err)
			} else {
				entries = append(entries, o)
			}
		}
		// Cache metadata for ChangeNotify old path resolution
		f.itemMetaCacheMu.Lock()
		f.itemMetaCache[info.ID] = itemMeta{
			parentID: directoryID,
			name:     info.FileName,
			isDir:    info.IsDir(),
		}
		f.itemMetaCacheMu.Unlock()
		return false
	})

	if err != nil {
		return err
	}

	// Send current directory entries to callback
	if len(entries) > 0 {
		if err := callback(entries); err != nil {
			return err
		}
	}

	// Recursively process subdirectories
	for _, subdir := range dirs {
		if err := f.listRRecursive(ctx, subdir.path, subdir.id, callback); err != nil {
			return err
		}
	}

	return nil
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

// Hash returns the SHA256 of the object
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	if t != hash.SHA256 {
		return "", hash.ErrUnsupported
	}
	if o.sha256 == "" && o.size == 0 {
		// Huawei Drive API does not return SHA256 for 0-byte files,
		// so return the well-known SHA256 of empty content.
		return "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", nil
	}
	return o.sha256, nil
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	if o.hasMetaData {
		return o.size
	}
	err := o.readMetaData(context.TODO())
	if err != nil {
		fs.Logf(o, "Failed to read metadata: %v", err)
		return 0
	}
	return o.size
}

// MimeType returns the content type of the object if known, or "" if not
func (o *Object) MimeType(ctx context.Context) string {
	err := o.readMetaData(ctx)
	if err != nil {
		fs.Logf(o, "Failed to read metadata for mime type: %v", err)
		return ""
	}
	return o.mimeType
}

// setMetaData sets the metadata from info
func (o *Object) setMetaData(info *api.File) (err error) {
	if info.IsDir() {
		return fs.ErrorIsDir
	}
	o.hasMetaData = true
	o.size = info.Size
	o.sha256 = info.SHA256
	o.modTime = info.EditedTime
	o.id = info.ID
	o.mimeType = info.MimeType
	return nil
}

// getParentIDs returns the parent folder IDs for this object
func (o *Object) getParentIDs(ctx context.Context) []string {
	// We need to get the parent IDs from the API
	info, err := o.fs.readMetaDataForPath(ctx, o.remote)
	if err != nil {
		fs.Debugf(o, "Failed to read metadata for parent IDs: %v", err)
		return nil
	}
	return info.ParentFolder
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

// readMetaDataForPath reads the metadata from the path
func (f *Fs) readMetaDataForPath(ctx context.Context, path string) (info *api.File, err error) {
	// defer log.Trace(f, "path=%q", path)("info=%+v, err=%v", &info, &err)
	leaf, directoryID, err := f.dirCache.FindPath(ctx, path, false)
	if err != nil {
		if err == fs.ErrorDirNotFound {
			return nil, fs.ErrorObjectNotFound
		}
		return nil, err
	}

	found, err := f.listAll(ctx, directoryID, func(item *api.File) bool {
		// Convert the API filename to standard name for comparison
		standardName := f.opt.Enc.ToStandardName(item.FileName)
		if standardName == leaf && !item.IsDir() {
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

// SetModTime sets the modification time of the local fs object
//
// Huawei Drive API does not support setting modification times,
// so this always returns ErrorCantSetModTime.
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
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
	if o.size == 0 {
		return io.NopCloser(strings.NewReader("")), nil
	}

	fs.FixRangeOption(options, o.size)
	opts := rest.Opts{
		Method: "GET",
		Path:   "/files/" + o.id,
		Parameters: url.Values{
			"form": []string{"content"},
		},
		Options: options,
	}
	var resp *http.Response
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.Call(ctx, &opts)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open file %q for reading: %w", o.remote, err)
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

	leaf = o.fs.opt.Enc.FromStandardName(leaf)

	// Handle unknown size by spooling to a temp file to determine size
	if size < 0 {
		tmpFile, err := os.CreateTemp("", "rclone-huaweidrive-upload-")
		if err != nil {
			return fmt.Errorf("failed to create temp file for unknown size upload: %w", err)
		}
		defer func() {
			_ = tmpFile.Close()
			_ = os.Remove(tmpFile.Name())
		}()
		size, err = io.Copy(tmpFile, in)
		if err != nil {
			return fmt.Errorf("failed to spool content for unknown size upload: %w", err)
		}
		if _, err = tmpFile.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("failed to seek temp file: %w", err)
		}
		in = tmpFile
	}

	if size == 0 {
		err = o.uploadEmpty(ctx, leaf, directoryID, src)
	} else {
		// Determine upload method based on size
		// Cap multipart upload at 20 MiB (Huawei Drive API limit)
		cutoff := int64(o.fs.opt.UploadCutoff)
		const maxMultipartSize = 20 * 1024 * 1024
		if cutoff > maxMultipartSize {
			cutoff = maxMultipartSize
		}
		if size < cutoff {
			err = o.uploadSimple(ctx, in, leaf, directoryID, size, src.ModTime(ctx), src)
		} else {
			err = o.uploadResume(ctx, in, leaf, directoryID, size, src.ModTime(ctx), src)
		}
	}

	// Handle metadata if upload was successful
	if err == nil {
		// Get metadata from the source ObjectInfo and merge with options metadata
		metadata, metaErr := fs.GetMetadataOptions(ctx, o.fs, src, options)
		if metaErr != nil {
			fs.Debugf(o, "Failed to get metadata options: %v", metaErr)
		} else if len(metadata) > 0 {
			if setMetaErr := o.SetMetadata(ctx, metadata); setMetaErr != nil {
				fs.Debugf(o, "Failed to set metadata after upload: %v", setMetaErr)
			}
		}
	}

	return err
}

// uploadEmpty creates a zero-byte file using metadata-only file creation.
func (o *Object) uploadEmpty(ctx context.Context, leaf, directoryID string, src fs.ObjectInfo) error {
	if leaf == "" {
		return fmt.Errorf("invalid filename: cannot be empty")
	}

	if o.id != "" {
		if err := o.Remove(ctx); err != nil && !errors.Is(err, fs.ErrorObjectNotFound) {
			return fmt.Errorf("failed to replace existing file %q with empty file: %w", leaf, err)
		}
		o.id = ""
	}

	mimeType := "application/octet-stream"
	if src != nil {
		if mimeTyper, ok := src.(fs.MimeTyper); ok {
			if srcMimeType := mimeTyper.MimeType(ctx); srcMimeType != "" {
				mimeType = srcMimeType
			}
		}
	}
	if detectedMimeType := mime.TypeByExtension(path.Ext(leaf)); detectedMimeType != "" && mimeType == "application/octet-stream" {
		mimeType = detectedMimeType
	}

	req := api.CreateFolderRequest{
		FileName: leaf,
		MimeType: mimeType,
	}
	if directoryID != "" {
		req.ParentFolder = []string{directoryID}
	}
	// Reject server-side auto-rename on duplicate filenames; see uploadMultipart.
	if o.id == "" {
		req.AutoRename = 3
	}

	opts := rest.Opts{
		Method: "POST",
		Path:   "/files",
		Parameters: url.Values{
			"fields": []string{"*"},
		},
	}

	var info *api.File
	err := o.fs.pacer.Call(func() (bool, error) {
		resp, err := o.fs.srv.CallJSON(ctx, &opts, &req, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("failed to create empty file %q: %w", leaf, err)
	}
	if info == nil {
		return fmt.Errorf("no file info returned when creating empty file %q", leaf)
	}
	return o.setMetaData(info)
}

// uploadSimple uploads a file using simple upload for files < upload_cutoff
func (o *Object) uploadSimple(ctx context.Context, in io.Reader, leaf, directoryID string, size int64, modTime time.Time, src fs.ObjectInfo) (err error) {
	// For files < 20MB, we should use multipart upload with metadata
	// According to Huawei Drive API documentation
	return o.uploadMultipart(ctx, in, leaf, directoryID, size, modTime, src)
}

// uploadMultipart uploads a file using multipart upload with metadata
func (o *Object) uploadMultipart(ctx context.Context, in io.Reader, leaf, directoryID string, size int64, modTime time.Time, src fs.ObjectInfo) (err error) {
	// Use the main fs client for authentication, but with upload URL
	srv := o.fs.srv

	// Try to get MIME type from source object first, then fall back to extension detection
	var mimeType string
	if src != nil {
		// Try to get MIME type from source object if it implements MimeTyper
		if mimeTyper, ok := src.(fs.MimeTyper); ok {
			mimeType = mimeTyper.MimeType(ctx)
		}
	}

	// If we couldn't get MIME type from source, detect from extension
	if mimeType == "" {
		mimeType = mime.TypeByExtension(path.Ext(leaf))
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	// Basic filename validation
	if leaf == "" {
		return fmt.Errorf("invalid filename: cannot be empty")
	}

	// Create metadata
	metadata := map[string]interface{}{
		"fileName": leaf,
		"mimeType": mimeType,
	}

	// Note: We don't set editedTime/createdTime here because
	// Huawei Drive API ignores these parameters and always uses server time

	// Set parent folder
	if directoryID != "" {
		metadata["parentFolder"] = []string{directoryID}
	}

	// Reject server-side auto-rename on duplicate filenames so the caller can
	// detect conflicts and decide what to do (instead of getting a silently
	// renamed "file (1)").
	if o.id == "" {
		metadata["autoRename"] = 3
	}

	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add metadata part - use exact format from Huawei docs
	metadataWriter, err := writer.CreatePart(textproto.MIMEHeader{
		"Content-Type": []string{"application/json; charset=UTF-8"},
	})
	if err != nil {
		return fmt.Errorf("failed to create metadata part: %w", err)
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	_, err = metadataWriter.Write(metadataJSON)
	if err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	// Add file content part - use application/octet-stream as per Huawei docs
	fileWriter, err := writer.CreatePart(textproto.MIMEHeader{
		"Content-Type": []string{"application/octet-stream"},
	})
	if err != nil {
		return fmt.Errorf("failed to create file part: %w", err)
	}

	// Read and write file content
	_, err = io.Copy(fileWriter, in)
	if err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	err = writer.Close()
	if err != nil {
		return fmt.Errorf("failed to close multipart writer: %w", err)
	}

	bodyBytes := buf.Bytes()
	opts := rest.Opts{
		Method: "POST",
		Parameters: url.Values{
			"uploadType": []string{"multipart"},
			"fields":     []string{"*"},
		},
		ContentType: fmt.Sprintf("multipart/related; boundary=%s", writer.Boundary()),
		RootURL:     o.fs.uploadURL,
	}

	if o.id != "" {
		// Update existing file
		opts.Path = "/files/" + o.id
		opts.Method = "PUT"
	} else {
		// Create new file
		opts.Path = "/files"
	}

	var info *api.File
	err = o.fs.pacer.Call(func() (bool, error) {
		opts.Body = bytes.NewReader(bodyBytes)
		resp, err := srv.CallJSON(ctx, &opts, nil, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("failed to upload file %q: %w", leaf, err)
	}

	if info == nil {
		return fmt.Errorf("no file info returned when uploading %q", leaf)
	}

	return o.setMetaData(info)
}

// uploadResume uploads a file using resumable upload for files >= upload_cutoff
func (o *Object) uploadResume(ctx context.Context, in io.Reader, leaf, directoryID string, size int64, modTime time.Time, src fs.ObjectInfo) (err error) {
	// Use the main fs client for authentication
	srv := o.fs.srv

	// First, initialize the resumable upload
	opts := rest.Opts{
		Method: "POST",
		Parameters: url.Values{
			"uploadType": []string{"resume"},
			"fields":     []string{"*"},
		},
		ExtraHeaders: map[string]string{
			"X-Upload-Content-Length": strconv.FormatInt(size, 10),
		},
		RootURL: o.fs.uploadURL,
	}

	// Try to get MIME type from source object first, then fall back to extension detection
	var mimeType string
	if src != nil {
		// Try to get MIME type from source object if it implements MimeTyper
		if mimeTyper, ok := src.(fs.MimeTyper); ok {
			mimeType = mimeTyper.MimeType(ctx)
		}
	}

	// If we couldn't get MIME type from source, detect from extension
	if mimeType == "" {
		mimeType = mime.TypeByExtension(path.Ext(leaf))
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	opts.ExtraHeaders["X-Upload-Content-Type"] = mimeType

	if o.id != "" {
		// Update existing file
		opts.Path = "/files/" + o.id
		opts.Method = "PUT"
	} else {
		// Create new file
		opts.Path = "/files"
	}

	// Prepare metadata for resumable upload initialization
	metadata := map[string]interface{}{
		"fileName": leaf,
	}

	// Note: We don't set editedTime/createdTime here because
	// Huawei Drive API ignores these parameters and always uses server time

	if directoryID != "" {
		metadata["parentFolder"] = []string{directoryID}
	}

	// Reject server-side auto-rename on duplicate filenames; see uploadMultipart.
	if o.id == "" {
		metadata["autoRename"] = 3
	}

	var resp *http.Response
	var initResp api.ResumeUploadInitResponse
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = srv.CallJSON(ctx, &opts, metadata, &initResp)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("failed to initialize resumable upload for %q: %w", leaf, err)
	}

	// Get the upload URL from Location header
	location := resp.Header.Get("Location")
	if location == "" {
		return fmt.Errorf("no upload URL returned for %q", leaf)
	}

	// Now upload the content in chunks
	chunkSize := int64(o.fs.opt.ChunkSize)
	if chunkSize < 256*1024 {
		chunkSize = 256 * 1024 // Minimum chunk size according to Huawei Drive API
	}
	if chunkSize > 64*1024*1024 {
		chunkSize = 64 * 1024 * 1024 // Maximum single upload size
	}

	buf := make([]byte, chunkSize)
	var offset int64

	for offset < size {
		n, err := io.ReadFull(in, buf)
		if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
			return fmt.Errorf("failed to read chunk at offset %d: %w", offset, err)
		}

		chunk := buf[:n]
		end := offset + int64(n) - 1

		// Upload this chunk using the original client with OAuth authentication
		chunkOpts := rest.Opts{
			Method:  "PUT",
			RootURL: location,
			Path:    "", // Empty path since RootURL contains the full URL
			ExtraHeaders: map[string]string{
				"Content-Range":  fmt.Sprintf("bytes %d-%d/%d", offset, end, size),
				"Content-Length": strconv.Itoa(n),
				"Content-Type":   mimeType,
			},
		}

		var info *api.File
		var chunkResp *http.Response
		err = o.fs.pacer.Call(func() (bool, error) {
			chunkOpts.Body = bytes.NewReader(chunk)
			var err error
			chunkResp, err = srv.CallJSON(ctx, &chunkOpts, nil, &info)
			// 308 means continue uploading (incomplete)
			if chunkResp != nil && chunkResp.StatusCode == 308 {
				// Clear info for incomplete upload
				info = nil
				return false, nil
			}
			// 200/201 means upload complete - let the response be processed
			if chunkResp != nil && (chunkResp.StatusCode == 200 || chunkResp.StatusCode == 201) {
				return false, err // Return err (should be nil) to allow response processing
			}
			return shouldRetry(ctx, chunkResp, err)
		})
		if err != nil {
			return fmt.Errorf("failed to upload chunk at offset %d: %w", offset, err)
		}

		offset += int64(n)

		// If we got file info back, we're done
		if info != nil && info.ID != "" {
			return o.setMetaData(info)
		}

		// Check if this was the last chunk and upload is complete (308 with all bytes uploaded)
		if chunkResp != nil && chunkResp.StatusCode == 308 && offset >= size {
			// Upload is complete, but we need to get file info
			// Send a final request to get the created file info
			finalOpts := rest.Opts{
				Method:  "PUT",
				RootURL: location,
				Path:    "",
				ExtraHeaders: map[string]string{
					"Content-Range":  fmt.Sprintf("bytes */%d", size),
					"Content-Length": "0",
				},
			}

			var finalInfo *api.File
			err = o.fs.pacer.Call(func() (bool, error) {
				finalOpts.Body = bytes.NewReader([]byte{})
				finalResp, err := srv.CallJSON(ctx, &finalOpts, nil, &finalInfo)
				if finalResp != nil && (finalResp.StatusCode == 200 || finalResp.StatusCode == 201) {
					return false, err
				}
				return shouldRetry(ctx, finalResp, err)
			})
			if err != nil {
				return fmt.Errorf("failed to finalize upload for %q: %w", leaf, err)
			}

			if finalInfo != nil && finalInfo.ID != "" {
				return o.setMetaData(finalInfo)
			}
		}
	}

	return fmt.Errorf("upload completed but no file info received for %q", leaf)
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	opts := rest.Opts{
		Method: "DELETE",
		Path:   "/files/" + o.id,
	}
	return o.fs.pacer.Call(func() (bool, error) {
		resp, err := o.fs.srv.Call(ctx, &opts)
		return shouldRetry(ctx, resp, err)
	})
}

// Metadata returns metadata for an object
//
// It should return nil if there is no Metadata
func (o *Object) Metadata(ctx context.Context) (metadata fs.Metadata, err error) {
	// Get the full file information from the API
	info, err := o.fs.readMetaDataForPath(ctx, o.remote)
	if err != nil {
		fs.Logf(o, "Failed to read metadata: %v", err)
		return nil, err
	}

	// Create metadata map and populate it with available fields
	metadata = make(fs.Metadata)

	// Standard file information
	if info.Description != "" {
		metadata["description"] = info.Description
	}
	if info.MimeType != "" {
		metadata["content-type"] = info.MimeType
	}
	if info.SHA256 != "" {
		metadata["sha256"] = info.SHA256
	}
	if !info.CreatedTime.IsZero() {
		metadata["btime"] = info.CreatedTime.Format(time.RFC3339Nano)
	}
	if !info.EditedTime.IsZero() {
		metadata["mtime"] = info.EditedTime.Format(time.RFC3339Nano)
	}
	if !info.EditedByMeTime.IsZero() {
		metadata["utime"] = info.EditedByMeTime.Format(time.RFC3339Nano)
	}

	// File attributes as string representations
	if info.Favorite {
		metadata["favorite"] = "true"
	}
	if info.Recycled {
		metadata["recycled"] = "true"
	}
	if info.ExistThumbnail {
		metadata["has-thumbnail"] = "true"
	}

	// Custom properties from Properties field
	// Return these directly without prefix for user metadata compatibility
	for key, value := range info.Properties {
		if valueStr, ok := value.(string); ok {
			metadata[key] = valueStr
		} else if value != nil {
			metadata[key] = fmt.Sprintf("%v", value)
		}
	}

	// App settings from AppSettings field
	// Also return these directly without prefix for user metadata compatibility
	for key, value := range info.AppSettings {
		if valueStr, ok := value.(string); ok {
			metadata[key] = valueStr
		} else if value != nil {
			metadata[key] = fmt.Sprintf("%v", value)
		}
	}

	return metadata, nil
}

// SetMetadata sets metadata for an Object
//
// It should return fs.ErrorNotImplemented if it can't set metadata
func (o *Object) SetMetadata(ctx context.Context, metadata fs.Metadata) error {
	fs.Debugf(o, "SetMetadata called with %d keys: %v", len(metadata), metadata)

	// Prepare the update request payload
	updateReq := api.UpdateFileRequest{
		Properties:  make(map[string]interface{}),
		AppSettings: make(map[string]interface{}),
	}

	// Process metadata and separate into properties and app settings
	for key, value := range metadata {
		switch key {
		case "description":
			// Standard description field
			updateReq.Description = value
		case "favorite":
			if favorite, err := strconv.ParseBool(value); err == nil {
				updateReq.Favorite = api.BoolPtr(favorite)
			}
		case "content-type":
			// Allow setting/overriding MIME type
			updateReq.MimeType = value
		// Note: Other metadata like btime, mtime, sha256 are read-only
		// and cannot be directly set via the update API
		default:
			// All other user metadata goes into Properties by default
			// This makes the backend compatible with rclone's user metadata expectations
			updateReq.Properties[key] = value
		}
	}

	// Only proceed if we have something to update
	if len(updateReq.Properties) == 0 && len(updateReq.AppSettings) == 0 &&
		updateReq.Description == "" && updateReq.Favorite == nil &&
		updateReq.MimeType == "" {
		return nil
	}

	// Make the API call to update the file metadata
	opts := rest.Opts{
		Method: "PATCH",
		Path:   "/files/" + o.id,
		Parameters: url.Values{
			"fields": []string{"*"},
		},
	}

	var info *api.File
	err := o.fs.pacer.Call(func() (bool, error) {
		resp, err := o.fs.srv.CallJSON(ctx, &opts, &updateReq, &info)
		return shouldRetry(ctx, resp, err)
	})

	if err != nil {
		return fmt.Errorf("failed to set metadata: %w", err)
	}

	// The API response sometimes returns incomplete file info after PATCH
	// Refresh from the API if needed
	if info.Size == 0 && info.Version == 0 {
		freshInfo, err := o.fs.readMetaDataForPath(ctx, o.Remote())
		if err != nil {
			return o.setMetaData(info)
		}
		return o.setMetaData(freshInfo)
	}

	return o.setMetaData(info)
}

// rootSlash returns root with a trailing slash
func (f *Fs) rootSlash() string {
	if f.root == "" {
		return f.root
	}
	return f.root + "/"
}

// shouldRetry returns a boolean as to whether this err deserves to be retried
func shouldRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}

	// Handle specific Huawei Drive error responses
	if resp != nil && err != nil {
		errMsg := err.Error()
		switch resp.StatusCode {
		case 400:
			if strings.Contains(errMsg, "21004001") || strings.Contains(errMsg, "LACK_OF_PARAM") {
				return false, fserrors.NoRetryError(fmt.Errorf("missing required parameters: %w", err))
			}
			if strings.Contains(errMsg, "21004002") || strings.Contains(errMsg, "PARAM_INVALID") {
				return false, fserrors.NoRetryError(fmt.Errorf("invalid parameters: %w", err))
			}
			if strings.Contains(errMsg, "21004009") || strings.Contains(errMsg, "PARENTFOLDER_NOT_FOUND") {
				return false, fs.ErrorDirNotFound
			}
			return false, fserrors.NoRetryError(fmt.Errorf("bad request: %w", err))
		case 401:
			if strings.Contains(errMsg, "22044011") {
				return true, err
			}
			return false, fserrors.NoRetryError(fmt.Errorf("unauthorized: %w", err))
		case 403:
			if strings.Contains(errMsg, "21004032") || strings.Contains(errMsg, "SERVICE_NOT_SUPPORT") {
				return false, fserrors.NoRetryError(fmt.Errorf("service not supported: %w", err))
			}
			if strings.Contains(errMsg, "21004035") || strings.Contains(errMsg, "INSUFFICIENT_SCOPE") {
				return false, fserrors.NoRetryError(fmt.Errorf("insufficient OAuth scope: %w", err))
			}
			if strings.Contains(errMsg, "21004036") || strings.Contains(errMsg, "INSUFFICIENT_PERMISSION") {
				return false, fserrors.NoRetryError(fmt.Errorf("insufficient permissions: %w", err))
			}
			if strings.Contains(errMsg, "21074033") || strings.Contains(errMsg, "AGREEMENT_NOT_SIGNED") {
				return false, fserrors.NoRetryError(fmt.Errorf("user agreement not signed: %w", err))
			}
			if strings.Contains(errMsg, "21084031") || strings.Contains(errMsg, "DATA_MIGRATING") {
				return true, err
			}
			if strings.Contains(errMsg, "21084038") || strings.Contains(errMsg, "OUTER_SERVICE_ERROR") {
				return true, err
			}
			return false, fserrors.NoRetryError(fmt.Errorf("forbidden: %w", err))
		case 404:
			return false, fserrors.NoRetryError(fmt.Errorf("not found: %w", err))
		case 409:
			return false, fs.ErrorDirExists
		case 410:
			if strings.Contains(errMsg, "21084100") || strings.Contains(errMsg, "CURSOR_EXPIRED") {
				return false, fserrors.NoRetryError(fmt.Errorf("cursor expired: %w", err))
			}
			if strings.Contains(errMsg, "21084101") || strings.Contains(errMsg, "TEMP_DATA_CLEARED") {
				return false, fserrors.NoRetryError(fmt.Errorf("temporary data cleared: %w", err))
			}
			return false, fserrors.NoRetryError(fmt.Errorf("gone: %w", err))
		case 429:
			if strings.Contains(errMsg, "22074292") || strings.Contains(errMsg, "APP_REQUEST_TOO_MANY") {
				fs.Debugf(nil, "Downloaded too frequently, sleeping 60s before retry")
				return true, pacer.RetryAfterError(err, 60*time.Second)
			}
		case 500:
			if strings.Contains(errMsg, "21005006") || strings.Contains(errMsg, "SERVER_TEMP_ERROR") ||
				strings.Contains(errMsg, "21085002") || strings.Contains(errMsg, "OUTER_SERVICE_UNAVAILABLE") ||
				strings.Contains(errMsg, "21085006") {
				return true, err
			}
		}
	}

	return fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// retryErrorCodes is a slice of error codes that we will retry
var retryErrorCodes = []int{
	429, // Too Many Requests
	500, // Internal Server Error
	502, // Bad Gateway
	503, // Service Unavailable
	504, // Gateway Timeout
	509, // Bandwidth Limit Exceeded
}

// DirCacheFlush resets the directory cache - used in testing as an optional interface
func (f *Fs) DirCacheFlush() {
	f.dirCache.ResetRoot()
}

// changeNotifyStartCursor gets the start cursor for change notifications
func (f *Fs) changeNotifyStartCursor(ctx context.Context) (string, error) {
	opts := rest.Opts{
		Method: "GET",
		Path:   "/changes/getStartCursor",
		Parameters: url.Values{
			"fields": []string{"*"},
		},
	}
	var result api.StartCursor
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, nil, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return "", fmt.Errorf("failed to get start cursor: %w", err)
	}
	return result.StartCursor, nil
}

// changeNotifyRunner polls for changes and notifies the caller
func (f *Fs) changeNotifyRunner(ctx context.Context, notifyFunc func(string, fs.EntryType), cursor string) (string, error) {
	for {
		opts := rest.Opts{
			Method: "GET",
			Path:   "/changes",
			Parameters: url.Values{
				"fields": []string{"*"},
				"cursor": []string{cursor},
			},
		}
		var result api.ChangeList
		err := f.pacer.Call(func() (bool, error) {
			resp, err := f.srv.CallJSON(ctx, &opts, nil, &result)
			return shouldRetry(ctx, resp, err)
		})
		if err != nil {
			return "", err
		}

		type entryInfo struct {
			path      string
			entryType fs.EntryType
		}
		var pathsToClear []entryInfo

		fs.Debugf(f, "changeNotifyRunner: got %d changes", len(result.Changes))
		for _, change := range result.Changes {
			// Check if we know the old path from dirCache (directories)
			if oldPath, ok := f.dirCache.GetInv(change.FileID); ok {
				entryType := fs.EntryObject
				if change.File != nil && change.File.IsDir() {
					entryType = fs.EntryDirectory
				}
				pathsToClear = append(pathsToClear, entryInfo{path: oldPath, entryType: entryType})
			}

			// Check if we know the old path from itemMetaCache (files and directories)
			f.itemMetaCacheMu.Lock()
			if cached, ok := f.itemMetaCache[change.FileID]; ok {
				entryType := fs.EntryObject
				if cached.isDir {
					entryType = fs.EntryDirectory
				}
				if parentPath, ok := f.dirCache.GetInv(cached.parentID); ok {
					oldPath := path.Join(parentPath, f.opt.Enc.ToStandardName(cached.name))
					pathsToClear = append(pathsToClear, entryInfo{path: oldPath, entryType: entryType})
				}
				// Remove stale entry; future List calls will repopulate
				delete(f.itemMetaCache, change.FileID)
			}
			f.itemMetaCacheMu.Unlock()

			// Find the new path if the file still exists
			if change.File != nil && !change.Deleted {
				entryType := fs.EntryObject
				if change.File.IsDir() {
					entryType = fs.EntryDirectory
				}
				fileName := f.opt.Enc.ToStandardName(change.File.FileName)
				if len(change.File.ParentFolder) > 0 {
					for _, parent := range change.File.ParentFolder {
						if parentPath, ok := f.dirCache.GetInv(parent); ok {
							newPath := path.Join(parentPath, fileName)
							pathsToClear = append(pathsToClear, entryInfo{path: newPath, entryType: entryType})
						} else {
							fs.Debugf(f, "Parent %s not in dirCache for file %s", parent, fileName)
						}
					}
				} else {
					pathsToClear = append(pathsToClear, entryInfo{path: fileName, entryType: entryType})
				}
			}
		}

		// Deduplicate and notify
		visited := make(map[string]struct{})
		for _, entry := range pathsToClear {
			if _, ok := visited[entry.path]; ok {
				continue
			}
			visited[entry.path] = struct{}{}
			notifyFunc(entry.path, entry.entryType)
		}

		// Check for next page or completion
		if result.NextCursor != "" {
			cursor = result.NextCursor
			continue
		}
		if result.NewStartCursor != "" {
			// Always advance to the new start cursor provided by the API.
			// Keeping a stale cursor risks CURSOR_EXPIRED on the next poll,
			// which would cause a gap in change notifications.
			return result.NewStartCursor, nil
		}
		return cursor, nil
	}
}

// ChangeNotify calls the passed function with a path that has had changes.
// It polls the Huawei Drive Changes API at the given interval.
func (f *Fs) ChangeNotify(ctx context.Context, notifyFunc func(string, fs.EntryType), pollIntervalChan <-chan time.Duration) {
	go func() {
		cursor, err := f.changeNotifyStartCursor(ctx)
		if err != nil {
			fs.Infof(f, "Failed to get start cursor: %s", err)
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
				if cursor == "" {
					cursor, err = f.changeNotifyStartCursor(ctx)
					if err != nil {
						fs.Infof(f, "Failed to get start cursor: %s", err)
						continue
					}
				}
				fs.Debugf(f, "Checking for changes on remote")
				cursor, err = f.changeNotifyRunner(ctx, notifyFunc, cursor)
				if err != nil {
					fs.Infof(f, "Change notify listener failure: %s", err)
				}
			}
		}
	}()
}

// UserInfo returns info about the connected user
func (f *Fs) UserInfo(ctx context.Context) (map[string]string, error) {
	opts := rest.Opts{
		Method: "GET",
		Path:   "/about",
		Parameters: url.Values{
			"fields": []string{"user"},
		},
	}

	var about api.About
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, nil, &about)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}

	info := map[string]string{
		"Name": about.User.DisplayName,
	}
	if about.User.PermissionID != "" {
		info["Id"] = about.User.PermissionID
	}
	return info, nil
}

// Disconnect the current user by removing stored token
func (f *Fs) Disconnect(ctx context.Context) error {
	config.FileDeleteKey(f.name, config.ConfigToken)
	return nil
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = (*Fs)(nil)
	_ fs.Purger          = (*Fs)(nil)
	_ fs.Copier          = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ fs.ListRer         = (*Fs)(nil)
	_ fs.CleanUpper      = (*Fs)(nil)
	_ fs.Abouter         = (*Fs)(nil)
	_ fs.ChangeNotifier  = (*Fs)(nil)
	_ fs.UserInfoer      = (*Fs)(nil)
	_ fs.Disconnecter    = (*Fs)(nil)
	_ fs.DirCacheFlusher = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.MimeTyper       = (*Object)(nil)
	_ fs.Metadataer      = (*Object)(nil)
	_ dircache.DirCacher = (*Fs)(nil)
)
