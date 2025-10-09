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
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			// Huawei Drive has strict filename restrictions
			// API error: "The fileName can not be blank and can not contain '<>|:\"*?/\\', cannot equal .. or . and not exceed max limit."
			Default: (encoder.Display |
				encoder.EncodeBackSlash |
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

// rootParentID returns the ID of the parent of the root directory
func (f *Fs) rootParentID() string {
	// For Huawei Drive, root directory doesn't have a parent
	// We use empty string to represent the root
	return ""
}

// getRootDirectoryID gets the actual root directory ID by looking for nonexistent_dir virtual flag
func (f *Fs) getRootDirectoryID(ctx context.Context) (string, error) {
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

	var result api.FileList
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, nil, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return "", fmt.Errorf("couldn't list files to detect root directory: %w", err)
	}

	// Look for nonexistent_dir virtual flag to get root directory ID
	for _, item := range result.Files {
		if item.FileName == "nonexistent_dir" && len(item.ParentFolder) > 0 {
			fs.Debugf(f, "Found root directory ID from nonexistent_dir: %s", item.ParentFolder[0])
			return item.ParentFolder[0], nil
		}
	}

	return "", fmt.Errorf("could not find root directory ID via nonexistent_dir virtual flag")
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
		name:  name,
		root:  root,
		opt:   *opt,
		srv:   srv,
		pacer: fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
	}

	f.features = (&fs.Features{
		CaseInsensitive:         false,
		DuplicateFiles:          false,
		ReadMimeType:            true,
		WriteMimeType:           true,
		CanHaveEmptyDirectories: true,
		BucketBased:             false,
		// 新增的特性支持
		FilterAware:      true,  // 支持过滤功能
		ReadMetadata:     true,  // 读取元数据 - 华为云盘支持 properties 字段
		WriteMetadata:    true,  // 写入元数据 - 华为云盘支持 properties 字段
		UserMetadata:     true,  // 用户自定义元数据
		ReadDirMetadata:  false, // 目录元数据读取（暂时禁用，需要额外实现）
		WriteDirMetadata: false, // 目录元数据写入（暂时禁用，需要额外实现）
		PartialUploads:   false, // 部分上传（华为云盘不支持断点续传的部分上传）
		NoMultiThreading: false, // 支持多线程
		SlowModTime:      false, // modTime 获取不慢
		SlowHash:         false, // 哈希计算不慢
	}).Fill(ctx, f)

	// Create directory cache
	f.dirCache = dircache.New(root, f.rootParentID(), f)

	// Find the current root
	err = f.dirCache.FindRoot(ctx, false)
	if err != nil {
		// Assume it is a file
		newRoot, remote := dircache.SplitPath(root)
		tempF := *f
		tempF.dirCache = dircache.New(newRoot, f.rootParentID(), &tempF)
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
		// XXX: update f.features here instead of tempF.features
		f.features = tempF.features
		// return an error with an fs which points to the parent
		return &tempF, fs.ErrorIsFile
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
	// Create the directory
	var req = api.CreateFolderRequest{
		FileName:    f.opt.Enc.FromStandardName(leaf),
		MimeType:    api.FolderMimeType,
		Description: "folder",
	}

	// For Huawei Drive, we always need to specify a parent folder
	// If pathID is empty or is the root parent ID, we need to get the actual root directory ID
	if pathID == "" || pathID == f.rootParentID() {
		// Get the actual root directory ID by detecting it from nonexistent_dir
		rootID, err := f.getRootDirectoryID(ctx)
		if err != nil {
			return "", fmt.Errorf("couldn't get root directory ID: %w", err)
		}
		if rootID != "" {
			req.ParentFolder = []string{rootID}
		}
	} else {
		// Use the provided pathID as parent
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
	if dirID != "" && dirID != f.rootParentID() {
		return f.listDirectory(ctx, dirID, fn)
	}

	// For root directory, we need to detect the root directory ID first
	return f.listRootDirectory(ctx, fn)
}

// listDirectory lists files in a specific directory using parentFolder filter
func (f *Fs) listDirectory(ctx context.Context, dirID string, fn listAllFn) (found bool, err error) {
	opts := rest.Opts{
		Method: "GET",
		Path:   "/files",
		Parameters: url.Values{
			"fields":     []string{"*"},
			"containers": []string{"drive"},
			"queryParam": []string{fmt.Sprintf("parentFolder='%s'", dirID)}, // Use queryParam to filter by parent folder
		},
	}

	// Set page size if specified
	if f.opt.ListChunk > 0 && f.opt.ListChunk <= 1000 {
		opts.Parameters.Set("pageSize", strconv.Itoa(f.opt.ListChunk))
	}

	fs.Debugf(f, "Listing directory %q with parentFolder filter", dirID)

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

	// Make a targeted API call with the detected root directory ID
	if rootDirectoryID != "" {
		fs.Debugf(f, "Making targeted API call for root directory: %s", rootDirectoryID)
		return f.listDirectory(ctx, rootDirectoryID, fn)
	}

	// Fallback: if we couldn't detect root directory ID, filter in memory
	fs.Debugf(f, "Could not detect root directory ID, filtering in memory")
	for _, item := range allFiles {
		if len(item.ParentFolder) > 0 && item.ParentFolder[0] == rootDirectoryID {
			if fn(&item) {
				found = true
			}
		}
	}

	return found, nil
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
	return f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.Call(ctx, &opts)
		return shouldRetry(ctx, resp, err)
	})
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
	if directoryID != "" {
		copyReq.ParentFolder = []string{directoryID}
	} else {
		// For root directory, get the actual root directory ID
		rootID, err := f.getRootDirectoryID(ctx)
		if err != nil {
			return nil, fmt.Errorf("couldn't get root directory ID for copy: %w", err)
		}
		if rootID != "" {
			copyReq.ParentFolder = []string{rootID}
		}
	}

	var info *api.File
	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, &copyReq, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	return f.newObjectWithInfo(ctx, remote, info)
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
			"fields": []string{"*"},
		},
	}

	moveReq := api.UpdateFileRequest{}

	// Set filename if we need to rename
	if needsRename {
		moveReq.FileName = f.opt.Enc.FromStandardName(dstLeaf)
	}

	// Set parent folders if we need to move directories
	// Based on Huawei API docs: addParentFolder和removeParentFolder要么都为空，要么都不为空
	if needsDirMove && len(currentParents) > 0 {
		moveReq.AddParentFolder = []string{dstDirectoryID}
		moveReq.RemoveParentFolder = currentParents
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
			} else {
				// This was just a same-directory rename, which worked correctly
				fs.Debugf(f, "Same-directory rename completed successfully")
			}
		} else {
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
			"fields": []string{"*"},
		},
	}

	moveReq := api.UpdateFileRequest{
		FileName: f.opt.Enc.FromStandardName(dstLeaf),
	}

	// Add new parent and remove old parent if they're different
	if dstDirectoryID != srcDirectoryID {
		// Special handling for root directory moves
		if dstDirectoryID == f.rootParentID() {
			// Moving to root directory - we need to get the actual root directory ID
			// For now, we'll skip the move if target is root to avoid the rootParentID error
			fs.Debugf(f, "Skipping directory move to root directory (not supported)")
			return fs.ErrorCantDirMove
		}
		moveReq.AddParentFolder = []string{dstDirectoryID}
		moveReq.RemoveParentFolder = []string{srcDirectoryID}
	}

	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, &moveReq, nil)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return err
	}

	srcFs.dirCache.FlushDir(srcRemote)
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
		if strings.EqualFold(item.FileName, leaf) && !item.IsDir() {
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
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	// Huawei Drive API does NOT support setting modification times
	// Despite accepting editedTime/createdTime parameters in the API,
	// the server always overwrites them with the current server timestamp
	// This has been confirmed through testing - the API returns success
	// but the file modification time remains as the server's current time
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

	// Determine upload method based on size
	if size >= 0 && size < int64(o.fs.opt.UploadCutoff) {
		return o.uploadSimple(ctx, in, leaf, directoryID, size, src.ModTime(ctx))
	}
	return o.uploadResume(ctx, in, leaf, directoryID, size, src.ModTime(ctx))
}

// uploadSimple uploads a file using simple upload for files < upload_cutoff
func (o *Object) uploadSimple(ctx context.Context, in io.Reader, leaf, directoryID string, size int64, modTime time.Time) (err error) {
	// For files < 20MB, we should use multipart upload with metadata
	// According to Huawei Drive API documentation
	return o.uploadMultipart(ctx, in, leaf, directoryID, size, modTime)
}

// uploadMultipart uploads a file using multipart upload with metadata
func (o *Object) uploadMultipart(ctx context.Context, in io.Reader, leaf, directoryID string, size int64, modTime time.Time) (err error) {
	// Use the main fs client for authentication, but with upload URL
	srv := o.fs.srv

	// Detect content type
	mimeType := mime.TypeByExtension(path.Ext(leaf))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	// Create metadata
	metadata := map[string]interface{}{
		"fileName": leaf,
		"mimeType": mimeType,
	}

	// Note: We don't set editedTime/createdTime here because
	// Huawei Drive API ignores these parameters and always uses server time

	// Set parent folder - always required for Huawei Drive
	if directoryID != "" && directoryID != o.fs.rootParentID() {
		metadata["parentFolder"] = []string{directoryID}
	} else {
		// For root directory, get the actual root directory ID
		rootID, err := o.fs.getRootDirectoryID(ctx)
		if err != nil {
			return fmt.Errorf("couldn't get root directory ID for upload: %w", err)
		}
		if rootID != "" {
			metadata["parentFolder"] = []string{rootID}
		}
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

	opts := rest.Opts{
		Method: "POST",
		Parameters: url.Values{
			"uploadType": []string{"multipart"},
			"fields":     []string{"*"},
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
func (o *Object) uploadResume(ctx context.Context, in io.Reader, leaf, directoryID string, size int64, modTime time.Time) (err error) {
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
		RootURL: uploadURL,
	}

	// Detect content type
	mimeType := mime.TypeByExtension(path.Ext(leaf))
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

	if directoryID != "" && directoryID != o.fs.rootParentID() {
		metadata["parentFolder"] = []string{directoryID}
	} else {
		// For root directory, get the actual root directory ID
		rootID, err := o.fs.getRootDirectoryID(ctx)
		if err != nil {
			return fmt.Errorf("couldn't get root directory ID for resumable upload: %w", err)
		}
		if rootID != "" {
			metadata["parentFolder"] = []string{rootID}
		}
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
	return o.fs.pacer.Call(func() (bool, error) {
		resp, err := o.fs.srv.Call(ctx, &opts)
		return shouldRetry(ctx, resp, err)
	})
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
			return false, fserrors.NoRetryError(fmt.Errorf("forbidden: %w", err))
		case 404:
			// For 404 errors (like invalid upload endpoints), don't retry
			return false, fserrors.NoRetryError(fmt.Errorf("not found - check API endpoint: %w", err))
		case 409:
			return false, fs.ErrorDirExists
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
	_ fs.ListRer         = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ dircache.DirCacher = (*Fs)(nil)
)
