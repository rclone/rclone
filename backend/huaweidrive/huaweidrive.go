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
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/oauthutil"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
	"golang.org/x/oauth2"
	"golang.org/x/text/unicode/norm"
)

const (
	rcloneClientID              = "115505059"
	rcloneEncryptedClientSecret = "3effe4596fb0874e3a982e1b4237143aea206fed826dab4db45c2dc6210a70be"
	minSleep                    = 10 * time.Millisecond
	maxSleep                    = 2 * time.Second
	decayConstant               = 2 // bigger for slower decay, exponential
	rootURL                     = "https://driveapis.cloud.huawei.com.cn/drive/v1"
	uploadURL                   = "https://driveapis.cloud.huawei.com.cn/upload/drive/v1"
	defaultChunkSize            = 8 * fs.Mebi
	maxFileSize                 = 50 * fs.Gibi // Maximum file size for Huawei Drive
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
	ClientSecret: rcloneEncryptedClientSecret,
	RedirectURL:  oauthutil.RedirectURL,
	AuthStyle:    oauth2.AuthStyleInParams, // Send client credentials in request body
	EndpointParams: url.Values{
		"access_type": {"offline"}, // Request refresh token as per Huawei docs
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
			// Parse config into Options struct
			opt := new(Options)
			err := configstruct.Set(m, opt)
			if err != nil {
				return nil, fmt.Errorf("couldn't parse config into struct: %w", err)
			}

			// Update OAuth2 config with user provided client credentials
			clientID, _ := m.Get("client_id")
			clientSecret, _ := m.Get("client_secret")

			if clientID != "" {
				oauthConfig.ClientID = clientID
			}
			if clientSecret != "" {
				oauthConfig.ClientSecret = clientSecret
			}

			// Check if we have an access token
			accessToken, ok := m.Get("token")
			if accessToken == "" || !ok {
				// If no token, start OAuth2 flow
				return oauthutil.ConfigOut("", &oauthutil.Options{
					OAuth2Config: oauthConfig,
				})
			}
			return nil, nil
		},
		Options: append(oauthutil.SharedOptions, []fs.Option{{
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
			Help:     "Cutoff for switching to multipart upload.\n\nAny files larger than this will be uploaded in chunks of chunk_size.\nThe minimum is 0 and the maximum is 5 GiB.",
			Default:  20 * fs.Mebi,
			Advanced: true,
		}, {
			Name: "unicode_normalization",
			Help: `Apply unicode NFC normalization to paths and filenames.

This flag can be used to normalize file names into unicode NFC form
that are read from and sent to Huawei Drive.

This can be useful to ensure consistent behavior with bisync and other
tools that expect normalized unicode file names.

Note: Huawei Drive may perform its own unicode normalization on the server side.`,
			Default:  true,
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
	ChunkSize    fs.SizeSuffix        `config:"chunk_size"`
	ListChunk    int                  `config:"list_chunk"`
	UploadCutoff fs.SizeSuffix        `config:"upload_cutoff"`
	UTFNorm      bool                 `config:"unicode_normalization"`
	Enc          encoder.MultiEncoder `config:"encoding"`
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

	// Cache for root directory ID to avoid repeated API calls
	rootDirID     string // cached root directory ID
	rootDirIDOnce bool   // whether root directory ID has been detected

	// Changes API support for cache invalidation
	changesCursor string     // cursor for tracking changes
	changesOnce   *sync.Once // ensure changes cursor is initialized once

	// Directory modification time cache (since Huawei Drive doesn't support setting dir modtime)
	dirModTimeMu sync.RWMutex         // protects dirModTimes
	dirModTimes  map[string]time.Time // map of directory path to override modtime
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

// normalizeFileName applies Unicode NFC normalization if enabled
func (f *Fs) normalizeFileName(filename string) string {
	if f.opt.UTFNorm {
		return norm.NFC.String(filename)
	}
	return filename
}

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
	// Huawei Drive API supports modification times with second precision
	// Note: For files, the API may override with server timestamp during uploads,
	// but directory modification times can be set via DirSetModTime feature
	return time.Second
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

// rootParentID returns the ID of the parent of the root directory
func (f *Fs) rootParentID() string {
	// For Huawei Drive, we use a special marker for root parent
	// The actual root directory ID will be determined during runtime
	// This avoids making network calls during Fs construction
	return "HUAWEI_DRIVE_ROOT_PARENT"
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

	// Create OAuth2 client
	client, _, err := oauthutil.NewClient(ctx, name, m, oauthConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to configure Huawei Drive OAuth: %w", err)
	}

	// Create REST client
	srv := rest.NewClient(client).SetRoot(rootURL)

	f := &Fs{
		name:        name,
		root:        root,
		opt:         *opt,
		srv:         srv,
		pacer:       fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
		changesOnce: &sync.Once{},
		dirModTimes: make(map[string]time.Time),
	}

	f.features = (&fs.Features{
		CaseInsensitive:         false,
		DuplicateFiles:          false,
		ReadMimeType:            true,
		WriteMimeType:           true,
		CanHaveEmptyDirectories: true,
		BucketBased:             false,
		// 新增的特性支持
		FilterAware:              true,            // 支持过滤器
		ReadMetadata:             true,            // 读取元数据 - 华为云盘支持 properties 字段
		WriteMetadata:            true,            // 写入元数据 - 华为云盘支持 properties 字段
		UserMetadata:             true,            // 用户自定义元数据
		ReadDirMetadata:          false,           // 目录元数据读取（暂时禁用，需要额外实现）
		WriteDirMetadata:         false,           // 目录元数据写入（暂时禁用，需要额外实现）
		WriteDirSetModTime:       true,            // 支持设置目录修改时间
		DirModTimeUpdatesOnWrite: false,           // 目录修改时间不会在写入时自动更新
		PartialUploads:           false,           // 部分上传（华为云盘不支持断点续传的部分上传）
		NoMultiThreading:         false,           // 支持多线程
		SlowModTime:              false,           // modTime 获取不慢
		SlowHash:                 false,           // 哈希计算不慢
		DirSetModTime:            f.DirSetModTime, // 支持设置目录修改时间
	}).Fill(ctx, f)

	// Create directory cache
	f.dirCache = dircache.New(root, f.rootParentID(), f)

	// Find the current root
	err = f.dirCache.FindRoot(ctx, false)
	if err != nil {
		// Assume it is a file (following Google Drive pattern)
		newRoot, remote := dircache.SplitPath(root)

		// Create a temporary Fs pointing to parent directory
		tempF := &Fs{
			name:        f.name,
			root:        newRoot,
			opt:         f.opt,
			srv:         f.srv,
			pacer:       f.pacer,
			changesOnce: &sync.Once{},
		}
		tempF.features = f.features
		tempF.dirCache = dircache.New(newRoot, f.rootParentID(), tempF)

		// Try to verify parent directory exists, but don't fail if it doesn't work
		// due to special character encoding issues with Huawei Drive
		_ = tempF.dirCache.FindRoot(ctx, false)

		// Check if the file exists in the parent directory (try even if FindRoot failed)
		_, fileErr := tempF.NewObject(ctx, remote)

		if fileErr == nil {
			// Found the file! Update the original f object and return it with ErrorIsFile
			// Note: Update original f instead of returning tempF to maintain feature consistency
			f.dirCache = tempF.dirCache
			f.root = tempF.root
			return f, fs.ErrorIsFile
		}

		// If both parent directory and file access failed, return original f
		return f, nil
	}

	// Check if the root path is actually pointing to a file (following S3 pattern)
	if f.root != "" && !strings.HasSuffix(root, "/") {
		// Check to see if the root path is actually an existing file
		oldRoot := f.root
		newRoot, leaf := path.Split(oldRoot)
		if leaf != "" {
			// Remove trailing slash from newRoot if present
			newRoot = strings.TrimSuffix(newRoot, "/")

			// Temporarily change root to parent directory
			f.root = newRoot
			f.dirCache = dircache.New(newRoot, f.rootParentID(), f)

			// Try to find the parent directory first
			err = f.dirCache.FindRoot(ctx, false)
			if err == nil {
				// Parent directory exists, now check if leaf is a file
				_, err := f.NewObject(ctx, leaf)
				if err == nil {
					// It's a file! Return ErrorIsFile with f pointing to parent
					return f, fs.ErrorIsFile
				}
			}

			// Not a file or parent not found, restore original root
			f.root = oldRoot
			f.dirCache = dircache.New(oldRoot, f.rootParentID(), f)
		}
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
	// Find the leaf in pathID
	found, err = f.listAll(ctx, pathID, func(item *api.File) bool {
		if strings.EqualFold(item.FileName, leaf) && item.IsDir() {
			pathIDOut = item.ID
			return true
		}
		return false
	})
	return pathIDOut, found, err
}

// CreateDir makes a directory with pathID as parent and name leaf
func (f *Fs) CreateDir(ctx context.Context, pathID, leaf string) (newID string, err error) {
	// Normalize and encode the directory name
	normalizedLeaf := f.normalizeFileName(leaf)
	encodedLeaf := f.opt.Enc.FromStandardName(normalizedLeaf)
	if encodedLeaf == "" {
		return "", fmt.Errorf("invalid directory name: cannot be empty")
	}

	// Create the directory
	var req = api.CreateFolderRequest{
		FileName:    encodedLeaf,
		MimeType:    api.FolderMimeType,
		Description: "folder",
	}

	// For Huawei Drive, set parent folder if pathID is provided and not the root parent ID
	if pathID != "" && pathID != f.rootParentID() {
		// Use the provided pathID as parent
		req.ParentFolder = []string{pathID}
	}
	// If pathID is empty or is the root parent ID, don't set parentFolder
	// The API will create the folder in the drive root by default

	opts := rest.Opts{
		Method: "POST",
		Path:   "/files",
		Parameters: url.Values{
			"fields":     []string{"*"},
			"autoRename": []string{"false"},
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

	// Verify the directory was created successfully by attempting to list it
	fs.Debugf(f, "Verifying created directory %s (ID: %s)", encodedLeaf, info.ID)
	_, verifyErr := f.listAll(ctx, info.ID, func(item *api.File) bool {
		return false // We don't need to process items, just verify access
	})
	if verifyErr != nil {
		fs.Debugf(f, "Warning: Created directory %s verification failed: %v", encodedLeaf, verifyErr)
		// Don't fail here as the directory might still be valid, just log the warning
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
	if dirID != "" && dirID != f.rootParentID() {
		return f.listDirectory(ctx, dirID, fn)
	}

	// For root directory, we need to detect the root directory ID first
	return f.listRootDirectory(ctx, fn)
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

	fs.Debugf(f, "Listing directory %q with filter: %q", dirID, queryParam)

	for {
		var result api.FileList
		err = f.pacer.Call(func() (bool, error) {
			resp, err := f.srv.CallJSON(ctx, &opts, nil, &result)
			return shouldRetry(ctx, resp, err)
		})
		if err != nil {
			return false, fmt.Errorf("couldn't list directory %q: %w", dirID, err)
		}

		fs.Debugf(f, "Directory API call returned %d files, NextPageToken: %q", len(result.Files), result.NextPageToken)

		// Process files directly - no need for further filtering since API does it for us
		for _, item := range result.Files {
			if fn(&item) {
				found = true
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

// listRootDirectory lists files in the root directory by first detecting the root directory ID
func (f *Fs) listRootDirectory(ctx context.Context, fn listAllFn) (found bool, err error) {
	// Check if we already have cached root directory ID
	if f.rootDirIDOnce && f.rootDirID != "" {
		fs.Debugf(f, "Using cached root directory ID: %s", f.rootDirID)
		return f.listDirectory(ctx, f.rootDirID, fn)
	}

	opts := rest.Opts{
		Method: "GET",
		Path:   "/files",
		Parameters: url.Values{
			"fields":     []string{"*"},
			"containers": []string{"drive"},
		},
	}

	// Set page size if specified
	if f.opt.ListChunk > 0 && f.opt.ListChunk <= 1000 {
		opts.Parameters.Set("pageSize", strconv.Itoa(f.opt.ListChunk))
	}

	// First, collect all files to analyze and find root directory ID
	var allFiles []api.File
	var result api.FileList

	fs.Debugf(f, "Listing root directory - collecting all files first to detect root ID")

	for {
		err = f.pacer.Call(func() (bool, error) {
			resp, err := f.srv.CallJSON(ctx, &opts, nil, &result)
			return shouldRetry(ctx, resp, err)
		})
		if err != nil {
			return false, fmt.Errorf("couldn't list root directory: %w", err)
		}

		fs.Debugf(f, "Root detection API call returned %d files, NextPageToken: %q", len(result.Files), result.NextPageToken)
		allFiles = append(allFiles, result.Files...)

		// Check for more pages
		if result.NextPageToken == "" {
			break
		}
		opts.Parameters.Set("pageToken", result.NextPageToken)
	}

	// Detect the root directory ID using nonexistent_dir virtual flag
	// nonexistent_dir is a virtual directory created by Huawei Drive API as a positioning flag
	// Its parentFolder field points to the actual root directory ID
	// This is Huawei Drive's design to help developers identify the root directory ID
	// since the root directory itself has no parent
	var rootDirectoryID string
	for _, item := range allFiles {
		if item.FileName == "nonexistent_dir" && len(item.ParentFolder) > 0 {
			rootDirectoryID = item.ParentFolder[0]
			fs.Debugf(f, "Found root directory ID from nonexistent_dir virtual flag: %s", rootDirectoryID)
			break
		}
	}

	// Cache the detected root directory ID and make a targeted API call
	if rootDirectoryID != "" {
		f.rootDirID = rootDirectoryID
		f.rootDirIDOnce = true
		fs.Debugf(f, "Cached and making targeted API call for root directory: %s", rootDirectoryID)
		return f.listDirectory(ctx, rootDirectoryID, fn)
	}

	// Mark that we've attempted root detection (even if failed) to avoid repeated attempts
	f.rootDirIDOnce = true

	// Fallback: if we couldn't detect root directory ID, process all files without parentFolder
	// Files in root directory typically have empty ParentFolder or specific root ID
	fs.Debugf(f, "Could not detect root directory ID, processing all files and filtering for root files")
	for _, item := range allFiles {
		// Skip the nonexistent_dir virtual flag from results
		if item.FileName == "nonexistent_dir" {
			continue
		}
		// For root directory files, we process all files that aren't in subdirectories
		// This is a fallback approach when root detection fails
		if fn(&item) {
			found = true
		}
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

	var iErr error
	_, err = f.listAll(ctx, directoryID, func(info *api.File) bool {
		remote := path.Join(dir, f.opt.Enc.ToStandardName(f.normalizeFileName(info.FileName)))
		if info.IsDir() {
			// cache the directory ID for later lookups
			f.dirCache.Put(remote, info.ID)

			// Check if we have a custom modification time for this directory
			modTime := info.EditedTime
			f.dirModTimeMu.RLock()
			if customTime, exists := f.dirModTimes[remote]; exists {
				modTime = customTime
				fs.Errorf(f, "Using custom modtime for dir %s: %v (original: %v)", remote, modTime, info.EditedTime)
			}
			f.dirModTimeMu.RUnlock()

			d := fs.NewDir(remote, modTime).SetID(info.ID)
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
	exisitingObj, err := f.newObjectWithInfo(ctx, src.Remote(), nil)
	if err == nil {
		err = exisitingObj.Update(ctx, in, src, options...)
		if err != nil {
			return nil, err
		}
		return exisitingObj, nil
	}
	if err != fs.ErrorObjectNotFound {
		return nil, err
	}
	// The object doesn't exist so create it
	return f.PutUnchecked(ctx, in, src, options...)
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
	// Clear any existing cache entries for this directory path to avoid conflicts
	f.dirCache.FlushDir(dir)
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

	// Clear cache after successful deletion
	if err == nil {
		dc.FlushDir(dir)
	}

	return err
}

// Rmdir deletes the root folder
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir, true)
}

// FlushDirCache flushes the directory cache for a specific directory
func (f *Fs) FlushDirCache(dir string) {
	f.dirCache.FlushDir(dir)
}

// DirSetModTime sets the modification time of the directory
func (f *Fs) DirSetModTime(ctx context.Context, dir string, modTime time.Time) error {
	// Find the directory ID
	dirID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return fmt.Errorf("failed to find directory %q: %w", dir, err)
	}

	// Set modification time using the same method as files
	opts := rest.Opts{
		Method: "PATCH",
		Path:   "/files/" + dirID,
		Parameters: url.Values{
			"fields":     []string{"*"},
			"autoRename": []string{"false"},
		},
	}

	update := api.UpdateFileRequest{
		EditedTime: &modTime,
	}

	var info *api.File
	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, &update, &info)
		return shouldRetry(ctx, resp, err)
	})

	if err != nil {
		fs.Debugf(f, "DirSetModTime: failed to set API time for %s, storing in memory: %v", dir, err)
		// Fallback to storing in memory if API call fails
		f.dirModTimeMu.Lock()
		defer f.dirModTimeMu.Unlock()
		f.dirModTimes[dir] = modTime
		fs.Debugf(f, "DirSetModTime: stored in memory %s -> %v", dir, modTime)
		return nil
	}

	fs.Debugf(f, "DirSetModTime: successfully set API time %s -> %v", dir, modTime)

	// Remove from memory cache since we successfully set it via API
	f.dirModTimeMu.Lock()
	defer f.dirModTimeMu.Unlock()
	delete(f.dirModTimes, dir)

	return nil
}

// getStartCursor gets the initial cursor for change tracking
func (f *Fs) getStartCursor(ctx context.Context) (string, error) {
	opts := rest.Opts{
		Method: "GET",
		Path:   "/changes/getStartCursor",
		Parameters: url.Values{
			"fields": {"*"},
		},
	}

	var resp api.StartCursor
	err := f.pacer.Call(func() (bool, error) {
		resp = api.StartCursor{}
		httpResp, err := f.srv.CallJSON(ctx, &opts, nil, &resp)
		return shouldRetry(ctx, httpResp, err)
	})
	if err != nil {
		return "", fmt.Errorf("failed to get start cursor: %w", err)
	}

	return resp.StartCursor, nil
}

// initializeChangesCursor initializes the changes cursor if not already set
func (f *Fs) initializeChangesCursor(ctx context.Context) error {
	var initErr error
	f.changesOnce.Do(func() {
		if f.changesCursor == "" {
			cursor, err := f.getStartCursor(ctx)
			if err != nil {
				fs.Debugf(f, "Changes API not available, disabling ChangeNotify: %v", err)
				initErr = err
				return
			}
			f.changesCursor = cursor
			fs.Debugf(f, "Initialized changes cursor: %s", cursor)
		}
	})
	return initErr
}

// getChanges retrieves changes since the last cursor
func (f *Fs) getChanges(ctx context.Context) (*api.ChangesList, error) {
	// Initialize cursor if needed
	if err := f.initializeChangesCursor(ctx); err != nil {
		return nil, err
	}

	opts := rest.Opts{
		Method: "GET",
		Path:   "/changes",
		Parameters: url.Values{
			"cursor":   {f.changesCursor},
			"pageSize": {"100"},
			"fields":   {"*"},
		},
	}

	var resp api.ChangesList
	err := f.pacer.Call(func() (bool, error) {
		resp = api.ChangesList{}
		httpResp, err := f.srv.CallJSON(ctx, &opts, nil, &resp)
		return shouldRetry(ctx, httpResp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get changes: %w", err)
	}

	return &resp, nil
}

// ChangeNotify calls the passed function with a path that has had changes.
// If the implementation uses polling, it should adhere to the given interval.
//
// Automatically restarts itself in case of unexpected behavior of the remote.
//
// Close the returned channel to stop being notified.
func (f *Fs) ChangeNotify(ctx context.Context, notifyFunc func(string, fs.EntryType), pollIntervalChan <-chan time.Duration) {
	go func() {
		// Initialize changes cursor early so all changes from now on get processed
		if err := f.initializeChangesCursor(ctx); err != nil {
			fs.Infof(f, "Failed to initialize changes cursor: %s", err)
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
				fs.Debugf(f, "Checking for changes on remote")
				err := f.changeNotifyRunner(ctx, notifyFunc)
				if err != nil {
					fs.Infof(f, "Change notify listener failure: %s", err)
				}
			}
		}
	}()
}

// changeNotifyRunner gets changes and notifies about them
func (f *Fs) changeNotifyRunner(ctx context.Context, notifyFunc func(string, fs.EntryType)) error {
	changes, err := f.getChanges(ctx)
	if err != nil {
		return err
	}

	if len(changes.Changes) == 0 {
		return nil
	}

	fs.Debugf(f, "Processing %d changes", len(changes.Changes))

	for _, change := range changes.Changes {
		if change.File == nil {
			continue
		}

		// Determine the entry type and path
		var entryType fs.EntryType
		var path string

		if change.File.IsDir() {
			entryType = fs.EntryDirectory
			path = f.opt.Enc.ToStandardPath(change.File.FileName)
		} else {
			entryType = fs.EntryObject
			path = f.opt.Enc.ToStandardPath(change.File.FileName)
		}

		// Notify about the change
		if path != "" {
			fs.Debugf(f, "Change detected: %s %s (deleted: %v)", change.ChangeType, path, change.Deleted)
			notifyFunc(path, entryType)
		}

		// Also notify parent directory for cache invalidation
		if !change.File.IsDir() {
			var parentDir string
			if lastSlash := strings.LastIndex(path, "/"); lastSlash >= 0 {
				parentDir = path[:lastSlash]
			} else {
				parentDir = ""
			}
			if parentDir != "" {
				notifyFunc(parentDir, fs.EntryDirectory)
			}
		}
	}

	// Update cursor for next call
	if changes.NextCursor != "" {
		f.changesCursor = changes.NextCursor
		fs.Debugf(f, "Updated changes cursor to: %s", f.changesCursor)
	}

	return nil
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
			"fields":     []string{"*"},
			"autoRename": []string{"false"},
		},
	}

	copyReq := api.CopyFileRequest{
		FileName: f.opt.Enc.FromStandardName(f.normalizeFileName(leaf)),
	}

	// Set parent folder - this should be in the request body, not as query parameter
	if directoryID != "" && directoryID != f.rootParentID() {
		copyReq.ParentFolder = []string{directoryID}
	}

	// Set the modification time to preserve it across copy
	srcModTime := srcObj.ModTime(ctx)
	if !srcModTime.IsZero() {
		copyReq.EditedTime = &srcModTime
		fs.Debugf(f, "Copy: preserving modification time %v for %s", srcModTime, remote)
	}

	// Copy metadata from source object if available
	var srcMetadata fs.Metadata
	var hasMetadata bool
	if metadata, err := srcObj.Metadata(ctx); err == nil && len(metadata) > 0 {
		copyReq.Properties = make(map[string]interface{})
		for key, value := range metadata {
			copyReq.Properties[key] = value
		}
		srcMetadata = metadata
		hasMetadata = true
		fs.Debugf(f, "Copy: copying metadata %+v to %s", metadata, remote)
	} else {
		fs.Debugf(f, "Copy: no metadata to copy for %s (err=%v)", srcObj.remote, err)
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
	if hasMetadata {
		if huaweiObj, ok := dstObj.(*Object); ok {
			if dstMetadata, err := huaweiObj.Metadata(ctx); err == nil {
				// Check if all source metadata was preserved
				allMatched := len(srcMetadata) == len(dstMetadata)
				if allMatched {
					for key, expectedValue := range srcMetadata {
						if actualValue, exists := dstMetadata[key]; !exists || actualValue != expectedValue {
							allMatched = false
							break
						}
					}
				}

				// If metadata doesn't match exactly, set it manually
				if !allMatched {
					fs.Debugf(f, "Copy: metadata not preserved by API (src=%v, dst=%v), setting manually", srcMetadata, dstMetadata)
					if err := huaweiObj.SetMetadata(ctx, srcMetadata); err != nil {
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
	currentParents := srcObj.getParentIDs()
	fs.Debugf(f, "Move: src=%q -> dst=%q (leaf=%q, dirID=%q, currentParents=%v)",
		src.Remote(), remote, dstLeaf, dstDirectoryID, currentParents)

	// Check what type of operation we need
	needsDirMove := len(currentParents) == 0 || currentParents[0] != dstDirectoryID
	needsRename := srcObj.remote != dstLeaf

	// If no changes needed, return the existing object
	if !needsDirMove && !needsRename {
		return src, nil
	}

	// Prepare the API request
	opts := rest.Opts{
		Method: "PATCH",
		Path:   "/files/" + srcObj.id,
		Parameters: url.Values{
			"fields":     []string{"*"},
			"autoRename": []string{"false"},
		},
	}

	moveReq := api.UpdateFileRequest{}

	// Preserve the current modification time during move/rename
	currentModTime := srcObj.ModTime(ctx)
	if !currentModTime.IsZero() {
		moveReq.EditedTime = &currentModTime
		fs.Debugf(f, "Move: preserving modification time %v", currentModTime)
	}

	// Set filename if we need to rename
	if needsRename {
		moveReq.FileName = f.opt.Enc.FromStandardName(f.normalizeFileName(dstLeaf))
	}

	// Set parent folders if we need to move directories
	// Based on Huawei API docs: addParentFolder和removeParentFolder要么都为空，要么都不为空
	// These are request body parameters, not query parameters
	if needsDirMove && len(currentParents) > 0 {
		// Only set these if both source and destination are not root
		if dstDirectoryID != f.rootParentID() {
			moveReq.AddParentFolder = []string{dstDirectoryID}
		}

		// Remove old parent folders that are not root
		var parentsToRemove []string
		for _, parent := range currentParents {
			if parent != f.rootParentID() && parent != "" {
				parentsToRemove = append(parentsToRemove, parent)
			}
		}
		if len(parentsToRemove) > 0 {
			moveReq.RemoveParentFolder = parentsToRemove
		}
	}

	fs.Debugf(f, "Move request: needsDirMove=%v, needsRename=%v, req=%+v",
		needsDirMove, needsRename, moveReq)

	// Execute the API call
	var info *api.File
	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, &moveReq, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("move operation failed: %w", err)
	}

	fs.Debugf(f, "Move response: fileName=%q, parentFolder=%v", info.FileName, info.ParentFolder)

	// Verify cross-directory moves actually worked
	// Huawei Drive API has a known issue where it returns success but doesn't actually move the file
	// Only check for cross-directory move failures when we actually requested a directory move
	if needsDirMove && len(info.ParentFolder) > 0 {
		actualParent := info.ParentFolder[0]
		expectedParent := dstDirectoryID

		// Verify cross-directory moves actually worked
		// For same-directory renames, the actual parent should remain the same as current parent
		if len(currentParents) > 0 && actualParent == currentParents[0] {
			// File is still in the same directory as before
			// Check if this was supposed to be a cross-directory move
			srcDir := currentParents[0]
			expectedDstDir := expectedParent

			// Handle root directory case: if expectedParent is empty, it means root directory
			// and we should compare with the actual root directory ID from currentParents
			if expectedDstDir == "" {
				expectedDstDir = srcDir // For root moves, expected dir should be same as current
			}

			if srcDir != expectedDstDir {
				// This was supposed to be a cross-directory move but file stayed in old location
				fs.Debugf(f, "Cross-directory move failed: API returned success but file remained in old location. Expected parent=%q, got=%q. Falling back to copy+delete.",
					expectedDstDir, actualParent)
				return nil, fs.ErrorCantMove
			}
			// This was just a same-directory rename, which worked correctly
			fs.Debugf(f, "Same-directory rename completed successfully")
		} else if actualParent != expectedParent {
			// File moved to a different directory, verify it's the expected one
			if actualParent != expectedParent {
				fs.Debugf(f, "Cross-directory move failed: API returned success but file moved to wrong location. Expected parent=%q, got=%q. Falling back to copy+delete.",
					expectedParent, actualParent)
				return nil, fs.ErrorCantMove
			}
		}
	}

	return f.newObjectWithInfo(ctx, remote, info)
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

	// Move the directory by updating parent folders
	opts := rest.Opts{
		Method: "PATCH",
		Path:   "/files/" + srcID,
		Parameters: url.Values{
			"fields":     []string{"*"},
			"autoRename": []string{"false"},
		},
	}

	moveReq := api.UpdateFileRequest{
		FileName: f.opt.Enc.FromStandardName(f.normalizeFileName(dstLeaf)),
	}

	// Add new parent and remove old parent if they're different
	// According to API docs, addParentFolder and removeParentFolder must appear together and both must be non-empty
	if dstDirectoryID != srcDirectoryID {
		// Huawei Drive API has strict requirements for parent folder operations
		// Let's handle different scenarios based on source and destination

		srcIsRoot := (srcDirectoryID == f.rootParentID() || srcDirectoryID == "")
		dstIsRoot := (dstDirectoryID == f.rootParentID() || dstDirectoryID == "")

		if !srcIsRoot && !dstIsRoot {
			// Both are non-root directories - this should work
			opts.Parameters.Set("addParentFolder", dstDirectoryID)
			opts.Parameters.Set("removeParentFolder", srcDirectoryID)
		} else if srcIsRoot && !dstIsRoot {
			// Moving from root to a directory
			// We need a valid removeParentFolder - use the cached root directory ID
			if f.rootDirIDOnce && f.rootDirID != "" {
				opts.Parameters.Set("addParentFolder", dstDirectoryID)
				opts.Parameters.Set("removeParentFolder", f.rootDirID)
			} else {
				fs.Debugf(f, "DirMove: Cannot move from root - root directory ID not available")
				return fs.ErrorCantDirMove
			}
		} else if !srcIsRoot && dstIsRoot {
			// Moving from a directory to root
			// We need a valid addParentFolder - use the cached root directory ID
			if f.rootDirIDOnce && f.rootDirID != "" {
				opts.Parameters.Set("addParentFolder", f.rootDirID)
				opts.Parameters.Set("removeParentFolder", srcDirectoryID)
			} else {
				fs.Debugf(f, "DirMove: Cannot move to root - root directory ID not available")
				return fs.ErrorCantDirMove
			}
		} else {
			// Both are root - this should be a simple rename, no parent change needed
			fs.Debugf(f, "DirMove: Root to root move, no parent folder changes needed")
		}
	}

	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, &moveReq, nil)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return err
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
		remote := path.Join(dir, f.opt.Enc.ToStandardName(f.normalizeFileName(info.FileName)))

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
			if o, err := f.newObjectWithInfo(ctx, remote, info); err == nil {
				entries = append(entries, o)
			}
		}
		return true
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
	return o.sha256, nil
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
func (o *Object) getParentIDs() []string {
	if !o.hasMetaData {
		// Try to read metadata if we don't have it
		if err := o.readMetaData(context.TODO()); err != nil {
			fs.Debugf(o, "Failed to read metadata for parent IDs: %v", err)
			return nil
		}
	}

	// We need to get the parent IDs from the stored metadata
	// This requires making an API call since we only store basic info
	info, err := o.fs.readMetaDataForPath(context.TODO(), o.remote)
	if err != nil {
		fs.Debugf(o, "Failed to read full metadata for parent IDs: %v", err)
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
		standardName := f.opt.Enc.ToStandardName(f.normalizeFileName(item.FileName))
		if strings.EqualFold(standardName, leaf) && !item.IsDir() {
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

// setModTime sets the modification time of the local fs object
func (o *Object) setModTime(ctx context.Context, modTime time.Time) (*api.File, error) {
	if o == nil {
		return nil, errors.New("can't set modification time - object is nil")
	}
	if o.id == "" {
		return nil, errors.New("can't set modification time - no id")
	}

	opts := rest.Opts{
		Method: "PATCH",
		Path:   "/files/" + o.id,
		Parameters: url.Values{
			"fields":     []string{"*"},
			"autoRename": []string{"false"},
		},
	}

	update := api.UpdateFileRequest{
		EditedTime: &modTime,
	}

	var info *api.File
	err := o.fs.pacer.Call(func() (bool, error) {
		resp, err := o.fs.srv.CallJSON(ctx, &opts, &update, &info)
		return shouldRetry(ctx, resp, err)
	})

	if err != nil {
		return nil, fmt.Errorf("failed to set modification time: %w", err)
	}

	return info, nil
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

	// Add directory verification if we have a parent directory
	if directoryID != "" && directoryID != o.fs.rootParentID() {
		// Verify directory exists by doing a quick lookup
		_, found := o.fs.dirCache.Get(directoryID)
		if !found {
			fs.Debugf(o, "Directory cache miss for %s, refreshing cache", directoryID)
			// Clear cache for this directory and try to find it again
			o.fs.dirCache.FlushDir(remote)
			leaf, directoryID, err = o.fs.dirCache.FindPath(ctx, remote, true)
			if err != nil {
				return fmt.Errorf("failed to refresh directory path for %q: %w", remote, err)
			}
		}
	}

	// Determine upload method based on size
	if size >= 0 && size < int64(o.fs.opt.UploadCutoff) {
		err = o.uploadSimple(ctx, in, leaf, directoryID, size, src.ModTime(ctx), src)
	} else {
		err = o.uploadResume(ctx, in, leaf, directoryID, size, src.ModTime(ctx), src)
	}

	// Handle metadata if upload was successful
	if err == nil {
		// Get metadata from the source ObjectInfo and merge with options metadata
		metadata, metaErr := fs.GetMetadataOptions(ctx, o.fs, src, options)
		if metaErr != nil {
			fs.Debugf(o, "Update: Error getting metadata options: %v", metaErr)
		} else if len(metadata) > 0 {
			fs.Debugf(o, "Update: File size before metadata: %d", o.size)
			fs.Debugf(o, "Update: Setting metadata after upload: %v", metadata)

			// Read the file info before setting metadata to compare
			beforeObj, beforeErr := o.fs.NewObject(ctx, o.Remote())
			if beforeErr == nil {
				fs.Debugf(o, "Update: File size before SetMetadata (from API): %d", beforeObj.Size())
			}

			setMetaErr := o.SetMetadata(ctx, metadata)
			if setMetaErr != nil {
				fs.Debugf(o, "Update: Failed to set metadata: %v", setMetaErr)
				// Don't fail the upload just because metadata setting failed
			} else {
				// Check file size after setting metadata
				afterObj, afterErr := o.fs.NewObject(ctx, o.Remote())
				if afterErr == nil {
					fs.Debugf(o, "Update: File size after SetMetadata (from API): %d", afterObj.Size())
					if afterObj.Size() != beforeObj.Size() {
						fs.Errorf(o, "Update: WARNING - File size changed from %d to %d after SetMetadata!", beforeObj.Size(), afterObj.Size())
					}
				}
			}
		}
	}

	return err
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

	// If we have a directoryID, verify it exists before uploading
	if directoryID != "" && directoryID != o.fs.rootParentID() {
		// Verify the directory exists by attempting to list it
		fs.Debugf(o, "Verifying directory %s exists before upload for file %s", directoryID, leaf)
		_, listErr := o.fs.listAll(ctx, directoryID, func(item *api.File) bool {
			return false // We don't actually need to process items, just verify access
		})
		if listErr != nil {
			fs.Errorf(o, "Directory %s verification failed: %v, will upload to root instead", directoryID, listErr)
			// If directory verification fails, upload without parent (to root)
			directoryID = ""
		} else {
			fs.Debugf(o, "Directory %s verification successful for file %s", directoryID, leaf)
		}
	}

	// Create metadata
	metadata := map[string]interface{}{
		"fileName": o.fs.normalizeFileName(leaf),
		"mimeType": mimeType,
	}

	// Set modification time if provided
	if !modTime.IsZero() {
		metadata["editedTime"] = modTime.Format(time.RFC3339)
		fs.Debugf(o, "Upload: Setting editedTime to %v", modTime)
	}

	// Set parent folder if directoryID is provided and not the root parent ID
	if directoryID != "" && directoryID != o.fs.rootParentID() {
		metadata["parentFolder"] = []string{directoryID}
		fs.Debugf(o, "Upload: Setting parentFolder to %s for file %s", directoryID, leaf)
	} else {
		fs.Debugf(o, "Upload: No parentFolder set for file %s (directoryID: %s, rootParentID: %s)", leaf, directoryID, o.fs.rootParentID())
	}
	// If directoryID is empty or is the root parent ID, don't set parentFolder
	// The API will upload the file to the drive root by default

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

	opts := rest.Opts{
		Method: "POST",
		Parameters: url.Values{
			"uploadType": []string{"multipart"},
			"fields":     []string{"*"},
			"autoRename": []string{"false"},
		},
		Body:        bytes.NewReader(buf.Bytes()),
		ContentType: fmt.Sprintf("multipart/related; boundary=%s", writer.Boundary()),
		RootURL:     uploadURL,
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
			"autoRename": []string{"false"},
		},
		ExtraHeaders: map[string]string{
			"X-Upload-Content-Length": strconv.FormatInt(size, 10),
		},
		RootURL: uploadURL,
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

	// Set modification time if provided
	if !modTime.IsZero() {
		metadata["editedTime"] = modTime.Format(time.RFC3339)
		fs.Debugf(o, "Upload: Setting editedTime to %v for resumable upload", modTime)
	}

	if directoryID != "" && directoryID != o.fs.rootParentID() {
		metadata["parentFolder"] = []string{directoryID}
	}
	// If directoryID is empty or is the root parent ID, don't set parentFolder
	// The API will upload the file to the drive root by default

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
			Body:    bytes.NewReader(chunk),
			ExtraHeaders: map[string]string{
				"Content-Range":  fmt.Sprintf("bytes %d-%d/%d", offset, end, size),
				"Content-Length": strconv.Itoa(n),
				"Content-Type":   mimeType,
			},
		}

		var info *api.File
		var chunkResp *http.Response
		err = o.fs.pacer.Call(func() (bool, error) {
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
				Body:    bytes.NewReader([]byte{}), // Empty body
				ExtraHeaders: map[string]string{
					"Content-Range":  fmt.Sprintf("bytes */%d", size),
					"Content-Length": "0",
				},
			}

			var finalInfo *api.File
			err = o.fs.pacer.Call(func() (bool, error) {
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
	err := o.fs.pacer.Call(func() (bool, error) {
		resp, err := o.fs.srv.Call(ctx, &opts)
		return shouldRetry(ctx, resp, err)
	})
	if err == nil {
		// Flush the directory cache for the parent directory to reflect the deletion
		dir := path.Dir(o.remote)
		if dir == "." {
			dir = ""
		}
		o.fs.dirCache.FlushDir(dir)
	}
	return err
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

	// Track whether mtime was explicitly provided
	var mtimeProvided bool

	// Process metadata and separate into properties and app settings
	for key, value := range metadata {
		switch key {
		case "description":
			// Standard description field
			updateReq.Description = value
		case "favorite":
			// Convert string boolean to actual boolean
			if favorite, err := strconv.ParseBool(value); err == nil {
				updateReq.Favorite = favorite
			}
		case "mtime":
			// Handle modification time specially
			if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
				updateReq.EditedTime = &t
				mtimeProvided = true
				fs.Debugf(o, "SetMetadata: Setting editedTime to %v", t)
			} else {
				fs.Debugf(o, "SetMetadata: Failed to parse mtime %q: %v", value, err)
			}
		// Note: Other metadata like btime, content-type, sha256 are read-only
		// and cannot be directly set via the update API
		default:
			// All other user metadata goes into Properties by default
			// This makes the backend compatible with rclone's user metadata expectations
			updateReq.Properties[key] = value
		}
	}

	// If mtime was not explicitly provided, preserve the current modification time
	if !mtimeProvided {
		currentModTime := o.ModTime(ctx)
		if !currentModTime.IsZero() {
			updateReq.EditedTime = &currentModTime
			fs.Debugf(o, "SetMetadata: Preserving current editedTime %v", currentModTime)
		}
	}

	// Only proceed if we have something to update
	if len(updateReq.Properties) == 0 && len(updateReq.AppSettings) == 0 &&
		updateReq.Description == "" && updateReq.EditedTime == nil {
		// No updateable metadata provided
		fs.Debugf(o, "SetMetadata: No updateable metadata provided")
		return nil
	}

	fs.Debugf(o, "SetMetadata: Updating %d properties, %d app settings, description=%q",
		len(updateReq.Properties), len(updateReq.AppSettings), updateReq.Description)

	// Make the API call to update the file metadata
	opts := rest.Opts{
		Method: "PATCH",
		Path:   "/files/" + o.id,
		Parameters: url.Values{
			"autoRename": []string{"false"},
		},
	}

	var info *api.File

	fs.Debugf(o, "SetMetadata: Making PATCH request to %s with payload: %+v", opts.Path, updateReq)

	err := o.fs.pacer.Call(func() (bool, error) {
		resp, err := o.fs.srv.CallJSON(ctx, &opts, &updateReq, &info)
		if resp != nil {
			fs.Debugf(o, "SetMetadata: HTTP response status: %d", resp.StatusCode)
		}
		if info != nil {
			fs.Debugf(o, "SetMetadata: Response file info - size: %d, version: %d", info.Size, info.Version)
		}
		return shouldRetry(ctx, resp, err)
	})

	if err != nil {
		return fmt.Errorf("failed to set metadata: %w", err)
	}

	fs.Debugf(o, "SetMetadata: API response file size: %d (may be incorrect)", info.Size)

	// The API response sometimes returns incorrect file information (size=0, version=0)
	// So we need to refresh the object info by querying the API again
	if info.Size == 0 && info.Version == 0 {
		fs.Debugf(o, "SetMetadata: API response has zero size/version, refreshing object info")
		freshInfo, err := o.fs.readMetaDataForPath(ctx, o.Remote())
		if err != nil {
			fs.Debugf(o, "SetMetadata: Failed to refresh object info: %v", err)
			// Fall back to using the original API response
			return o.setMetaData(info)
		}
		fs.Debugf(o, "SetMetadata: Refreshed file size: %d", freshInfo.Size)
		return o.setMetaData(freshInfo)
	}

	// Update the object's cached metadata
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
	if resp != nil {
		switch resp.StatusCode {
		case 400:
			// Handle specific 400 error codes from Huawei Drive API
			if strings.Contains(err.Error(), "21004001") || strings.Contains(err.Error(), "LACK_OF_PARAM") {
				return false, fserrors.NoRetryError(fmt.Errorf("missing required parameters: %w", err))
			}
			if strings.Contains(err.Error(), "21004002") || strings.Contains(err.Error(), "PARAM_INVALID") {
				return false, fserrors.NoRetryError(fmt.Errorf("invalid parameters: %w", err))
			}
			if strings.Contains(err.Error(), "21004009") || strings.Contains(err.Error(), "PARENTFOLDER_NOT_FOUND") {
				return false, fs.ErrorDirNotFound
			}
			// For other 400 errors, don't retry
			return false, fserrors.NoRetryError(fmt.Errorf("bad request: %w", err))
		case 401:
			// For 401 errors, allow OAuth library to handle token refresh
			// Error 22044011 typically means Access Token expired - let OAuth handle refresh
			if strings.Contains(err.Error(), "22044011") {
				fs.Debugf(nil, "Access token expired (error 22044011) - OAuth will attempt refresh")
				return true, err // Allow retry so OAuth can refresh the token
			}
			// For other 401 errors, don't retry to avoid infinite loops
			return false, fserrors.NoRetryError(fmt.Errorf("unauthorized: %w", err))
		case 403:
			// Handle specific 403 error codes
			if strings.Contains(err.Error(), "21004032") || strings.Contains(err.Error(), "SERVICE_NOT_SUPPORT") {
				return false, fserrors.NoRetryError(fmt.Errorf("service not supported: %w", err))
			}
			if strings.Contains(err.Error(), "21004035") || strings.Contains(err.Error(), "INSUFFICIENT_SCOPE") {
				return false, fserrors.NoRetryError(fmt.Errorf("insufficient OAuth scope: %w", err))
			}
			if strings.Contains(err.Error(), "21004036") || strings.Contains(err.Error(), "INSUFFICIENT_PERMISSION") {
				return false, fserrors.NoRetryError(fmt.Errorf("insufficient permissions: %w", err))
			}
			if strings.Contains(err.Error(), "21074033") || strings.Contains(err.Error(), "AGREEMENT_NOT_SIGNED") {
				return false, fserrors.NoRetryError(fmt.Errorf("user agreement not signed: %w", err))
			}
			if strings.Contains(err.Error(), "21084031") || strings.Contains(err.Error(), "DATA_MIGRATING") {
				// Data migration in progress - this might be temporary, allow retry
				fs.Debugf(nil, "Data migration in progress, will retry")
				return true, err
			}
			return false, fserrors.NoRetryError(fmt.Errorf("forbidden: %w", err))
		case 404:
			// For 404 errors (like invalid upload endpoints), don't retry
			return false, fserrors.NoRetryError(fmt.Errorf("not found - check API endpoint: %w", err))
		case 409:
			return false, fs.ErrorDirExists
		case 410:
			// Handle 410 errors - cursor expired or temp data cleared
			if strings.Contains(err.Error(), "21084100") || strings.Contains(err.Error(), "CURSOR_EXPIRED") {
				return false, fserrors.NoRetryError(fmt.Errorf("cursor expired, restart listing: %w", err))
			}
			if strings.Contains(err.Error(), "21084101") || strings.Contains(err.Error(), "TEMP_DATA_CLEARED") {
				return false, fserrors.NoRetryError(fmt.Errorf("temporary data cleared, restart operation: %w", err))
			}
			return false, fserrors.NoRetryError(fmt.Errorf("gone: %w", err))
		case 500:
			// Handle specific 500 error codes
			if strings.Contains(err.Error(), "21005006") || strings.Contains(err.Error(), "SERVER_TEMP_ERROR") ||
				strings.Contains(err.Error(), "21085002") || strings.Contains(err.Error(), "OUTER_SERVICE_UNAVAILABLE") ||
				strings.Contains(err.Error(), "21085006") {
				// These are temporary server errors, allow retry
				fs.Debugf(nil, "Temporary server error, will retry: %v", err)
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

// Check the interfaces are satisfied
var (
	_ fs.Fs              = (*Fs)(nil)
	_ fs.Purger          = (*Fs)(nil)
	_ fs.Copier          = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ fs.DirSetModTimer  = (*Fs)(nil)
	_ fs.ChangeNotifier  = (*Fs)(nil)
	_ fs.ListRer         = (*Fs)(nil)
	_ fs.CleanUpper      = (*Fs)(nil)
	_ fs.Abouter         = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.Metadataer      = (*Object)(nil)
	_ dircache.DirCacher = (*Fs)(nil)
)
