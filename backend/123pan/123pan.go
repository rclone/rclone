// Package pan123 provides an interface to the 123Pan cloud storage system.
package pan123

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/backend/123pan/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
)

const (
	apiBaseURL     = "https://open-api.123pan.com"
	rootID         = "0"
	minSleep       = 200 * time.Millisecond
	maxSleep       = 5 * time.Second
	trashBatchSize = 100 // Maximum files per trash API call
)

// parseID converts a string directory ID to int64
func parseID(id string) (int64, error) {
	return strconv.ParseInt(id, 10, 64)
}

// formatID converts an int64 ID to string
func formatID(id int64) string {
	return strconv.FormatInt(id, 10)
}

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "123pan",
		Description: "123Pan (123 Cloud Storage)",
		NewFs:       NewFs,
		Options: []fs.Option{
			{
				Name:      "client_id",
				Help:      "Client ID from 123pan developer console.\n\nGet it from https://www.123pan.com/developer",
				Required:  true,
				Sensitive: true,
			},
			{
				Name:      "client_secret",
				Help:      "Client Secret from 123pan developer console.\n\nGet it from https://www.123pan.com/developer",
				Required:  true,
				Sensitive: true,
			},
			{
				Name:      "access_token",
				Help:      "Access token (optional, will be auto-refreshed).",
				Advanced:  true,
				Sensitive: true,
				Hide:      fs.OptionHideBoth,
			},
			{
				Name:     "token_expiry",
				Help:     "Token expiry time (auto-managed).",
				Advanced: true,
				Hide:     fs.OptionHideBoth,
			},
			{
				Name:     "vip_level",
				Help:     "Cached VIP level (auto-managed). -1 means not cached.",
				Advanced: true,
				Hide:     fs.OptionHideBoth,
				Default:  -1,
			},
			{
				Name:     config.ConfigEncoding,
				Help:     config.ConfigEncodingHelp,
				Advanced: true,
				// 123pan has similar restrictions to OneDrive/Windows:
				// - Reserved characters: \ / : * ? " < > |
				// - Names can't begin with space, period, tilde, or control chars
				// - Names can't end with space, period, or control chars
				Default: (encoder.Display |
					encoder.EncodeBackSlash |
					encoder.EncodeLeftSpace |
					encoder.EncodeLeftPeriod |
					encoder.EncodeLeftTilde |
					encoder.EncodeLeftCrLfHtVt |
					encoder.EncodeRightSpace |
					encoder.EncodeRightPeriod |
					encoder.EncodeRightCrLfHtVt |
					encoder.EncodeWin | // :?"*<>|
					encoder.EncodeInvalidUtf8),
			},
		},
	})
}

// Options defines the configuration for this backend
type Options struct {
	ClientID     string               `config:"client_id"`
	ClientSecret string               `config:"client_secret"`
	AccessToken  string               `config:"access_token"`
	TokenExpiry  string               `config:"token_expiry"`
	VipLevel     int                  `config:"vip_level"`
	Enc          encoder.MultiEncoder `config:"encoding"`
}

// QPSLimits defines rate limits for different APIs
// See: https://123yunpan.yuque.com/org-wiki-123yunpan-muaork/cr6ced/txgcvbfgh0gtuad5
type QPSLimits struct {
	FileList       int // api/v2/file/list
	FileMove       int // api/v1/file/move
	FileTrash      int // api/v1/file/trash
	FileDelete     int // api/v1/file/delete (permanent delete)
	Mkdir          int // upload/v1/file/mkdir
	DownloadInfo   int // api/v1/file/download_info
	UploadCreate   int // upload/v2/file/create
	UploadComplete int // upload/v2/file/upload_complete
	FileRename     int // api/v1/file/name
	ShareCreate    int // api/v1/share/create
}

const unlimitedQPS = 100 // Use high value for "unlimited" APIs

var (
	// Rate limits for free users (Developer API mode)
	// Documented: api/v2/file/list=5, file/move=3, file/delete=1, mkdir=5
	// Customer service confirmed: upload_complete=5, create=1, download_info=2
	// Unlimited: file/name, share/create, file/trash
	freeUserQPS = QPSLimits{
		FileList:       5,            // documented
		FileMove:       3,            // documented
		FileTrash:      unlimitedQPS, // confirmed unlimited
		FileDelete:     1,            // documented
		Mkdir:          5,            // documented
		DownloadInfo:   2,            // confirmed: 2 QPS
		UploadCreate:   1,            // confirmed: 1 QPS
		UploadComplete: 5,            // confirmed: 5 QPS
		FileRename:     unlimitedQPS, // confirmed unlimited
		ShareCreate:    unlimitedQPS, // confirmed unlimited
	}
	// Rate limits for VIP users (Developer API mode)
	// Documented: api/v2/file/list=10, file/move=10, file/delete=10, mkdir=20
	// Customer service confirmed: upload_complete=20, create=20, download_info=unlimited
	// Unlimited: file/name, share/create, file/trash
	vipUserQPS = QPSLimits{
		FileList:       10,           // documented
		FileMove:       10,           // documented
		FileTrash:      unlimitedQPS, // confirmed unlimited
		FileDelete:     10,           // documented
		Mkdir:          20,           // documented
		DownloadInfo:   unlimitedQPS, // confirmed unlimited
		UploadCreate:   20,           // confirmed: 20 QPS
		UploadComplete: 20,           // confirmed: 20 QPS
		FileRename:     unlimitedQPS, // confirmed unlimited
		ShareCreate:    unlimitedQPS, // confirmed unlimited
	}
)

// Fs represents a remote 123pan server
type Fs struct {
	name             string               // name of this remote
	root             string               // the path we are working on
	opt              Options              // parsed options
	features         *fs.Features         // optional features
	srv              *rest.Client         // the connection to the server
	dirCache         *dircache.DirCache   // Map of directory path to directory id
	pacer            *fs.Pacer            // pacer for API calls
	m                configmap.Mapper     // config map for saving tokens
	tokenMu          *sync.Mutex          // mutex for token refresh
	tokenExpiry      time.Time            // token expiration time
	tokenJustRefresh bool                 // flag to indicate token was just refreshed
	vipRefreshed     bool                 // flag to indicate VIP level was refreshed due to 429
	uid              uint64               // user ID
	isVip            bool                 // whether user is VIP
	vipLevel         int                  // VIP level: 0=free, 1=VIP, 2=SVIP, 3=长期VIP
	qpsLimits        QPSLimits            // current QPS limits
	apiPacers        map[string]*fs.Pacer // per-API pacers
}

// Object describes a 123pan object
type Object struct {
	fs          *Fs       // what this object is part of
	remote      string    // The remote path
	hasMetaData bool      // whether info below has been set
	size        int64     // size of the object
	modTime     time.Time // modification time of the object
	id          int64     // ID of the object
	etag        string    // MD5 hash
	parentID    int64     // parent directory ID
}

// ------------------------------------------------------------
// Fs interface methods
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
	return fmt.Sprintf("123pan root '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Precision returns the precision of the remote
// 123pan doesn't support setting modification times, so return ModTimeNotSupported
func (f *Fs) Precision() time.Duration {
	return fs.ModTimeNotSupported
}

// Hashes returns the supported hash sets
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.MD5)
}

// NewFs constructs an Fs from the path, bucket:path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	root = strings.Trim(root, "/")

	f := &Fs{
		name: name,
		root: root,
		opt:  *opt,
		m:    m,
		pacer: fs.NewPacer(ctx, pacer.NewDefault(
			pacer.MinSleep(minSleep),
			pacer.MaxSleep(maxSleep),
			pacer.DecayConstant(2),
		)),
		tokenMu:   new(sync.Mutex),
		qpsLimits: freeUserQPS, // default to free user limits
		apiPacers: make(map[string]*fs.Pacer),
	}

	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
		CaseInsensitive:         false,
	}).Fill(ctx, f)

	// Initialize HTTP client
	client := fshttp.NewClient(ctx)
	f.srv = rest.NewClient(client).SetRoot(apiBaseURL)
	f.srv.SetHeader("Platform", "open_platform")
	f.srv.SetHeader("Content-Type", "application/json")

	// Load saved token expiry
	if opt.TokenExpiry != "" {
		if expiry, err := time.Parse(time.RFC3339, opt.TokenExpiry); err == nil {
			f.tokenExpiry = expiry
		}
	}

	// Get or refresh access token
	if err := f.getAccessToken(ctx); err != nil {
		return nil, fmt.Errorf("failed to authenticate: %w", err)
	}

	// Initialize user level and QPS limits
	// Force refresh if token was just refreshed (VIP status may have changed)
	if err := f.initUserLevel(ctx, f.tokenJustRefresh); err != nil {
		fs.Debugf(f, "Could not refresh user info: %v", err)
	}
	f.tokenJustRefresh = false // Reset the flag

	// Initialize directory cache (root ID is "0")
	f.dirCache = dircache.New(root, rootID, f)

	// Find the current root
	err = f.dirCache.FindRoot(ctx, false)
	if err != nil {
		// Assume it is a file
		newRoot, remote := dircache.SplitPath(root)
		tempF := *f
		tempF.dirCache = dircache.New(newRoot, rootID, &tempF)
		tempF.root = newRoot

		err = tempF.dirCache.FindRoot(ctx, false)
		if err != nil {
			// No root so return old f
			return f, nil
		}

		_, err := tempF.NewObject(ctx, remote)
		if err != nil {
			if err == fs.ErrorObjectNotFound {
				// File doesn't exist so return old f
				return f, nil
			}
			return nil, err
		}

		f.dirCache = tempF.dirCache
		f.root = tempF.root
		return f, fs.ErrorIsFile
	}

	return f, nil
}

// listAllFiles fetches all files in a directory
// Note: No caching is used to ensure consistency across Fs instances.
// The API rate limits are handled by the pacer.
func (f *Fs) listAllFiles(ctx context.Context, parentID int64) ([]api.File, error) {
	// Fetch from API
	var allFiles []api.File
	lastFileID := int64(0)
	for lastFileID != -1 {
		resp, err := f.listFiles(ctx, parentID, 100, lastFileID)
		if err != nil {
			return nil, err
		}
		allFiles = append(allFiles, resp.Data.FileList...)
		lastFileID = resp.Data.LastFileID
	}
	return allFiles, nil
}

// forEachFile iterates over all files in a directory, calling fn for each file.
// If fn returns true, the iteration stops early (useful for search operations).
// Returns an error if the listing fails.
// This method uses directory listing cache to reduce API calls.
func (f *Fs) forEachFile(ctx context.Context, parentID int64, fn func(file api.File) (stop bool)) error {
	files, err := f.listAllFiles(ctx, parentID)
	if err != nil {
		return err
	}

	for _, file := range files {
		if fn(file) {
			return nil
		}
	}
	return nil
}

// FindLeaf finds a directory of name leaf in the folder with ID pathID
func (f *Fs) FindLeaf(ctx context.Context, pathID, leaf string) (pathIDOut string, found bool, err error) {
	parentID, err := parseID(pathID)
	if err != nil {
		return "", false, err
	}

	err = f.forEachFile(ctx, parentID, func(file api.File) bool {
		standardName := f.opt.Enc.ToStandardName(file.Filename)
		if file.Trashed == 0 && standardName == leaf && file.Type == 1 {
			pathIDOut = formatID(file.FileID)
			found = true
			return true
		}
		return false
	})

	return pathIDOut, found, err
}

// CreateDir makes a directory with pathID as parent and name leaf
func (f *Fs) CreateDir(ctx context.Context, pathID, leaf string) (newID string, err error) {
	parentID, err := parseID(pathID)
	if err != nil {
		return "", err
	}

	resp, err := f.mkdir(ctx, parentID, f.opt.Enc.FromStandardName(leaf))
	if err != nil {
		return "", err
	}

	return formatID(resp.Data.DirID), nil
}

// List the objects and directories in dir into entries
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return nil, err
	}

	parentID, err := parseID(directoryID)
	if err != nil {
		return nil, err
	}

	err = f.forEachFile(ctx, parentID, func(file api.File) bool {
		// Skip trashed files
		if file.Trashed != 0 {
			return false
		}

		remote := path.Join(dir, f.opt.Enc.ToStandardName(file.Filename))

		if file.Type == 1 {
			// Directory
			modTime := parseTime(file.UpdateAt)
			d := fs.NewDir(remote, modTime).SetID(formatID(file.FileID))
			entries = append(entries, d)
			f.dirCache.Put(remote, formatID(file.FileID))
		} else {
			// File
			o := &Object{
				fs:          f,
				remote:      remote,
				hasMetaData: true,
				size:        file.Size,
				modTime:     parseTime(file.UpdateAt),
				id:          file.FileID,
				etag:        file.Etag,
				parentID:    file.ParentFileID,
			}
			entries = append(entries, o)
		}
		return false
	})

	return entries, err
}

// NewObject finds the Object at remote. If it can't be found it returns fs.ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	return f.newObjectWithInfo(ctx, remote, nil)
}

func (f *Fs) newObjectWithInfo(ctx context.Context, remote string, info *api.File) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}

	if info != nil {
		o.setMetaData(info)
	} else {
		err := o.readMetaData(ctx)
		if err != nil {
			return nil, err
		}
	}

	return o, nil
}

// Put uploads the object
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

// PutUnchecked uploads the object without checking for existing object
func (f *Fs) PutUnchecked(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	remote := src.Remote()
	size := src.Size()

	if size < 0 {
		return nil, errors.New("can't upload files of unknown size")
	}
	if size == 0 {
		return nil, fs.ErrorCantUploadEmptyFiles
	}

	// Find or create parent directory
	leaf, directoryID, err := f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return nil, err
	}

	parentID, err := parseID(directoryID)
	if err != nil {
		return nil, err
	}

	// Upload the file
	info, err := f.upload(ctx, in, parentID, f.opt.Enc.FromStandardName(leaf), size, options...)
	if err != nil {
		return nil, err
	}

	return f.newObjectWithInfo(ctx, remote, info)
}

// Mkdir creates the container if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	_, err := f.dirCache.FindDir(ctx, dir, true)
	return err
}

// Rmdir deletes the root folder
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return err
	}

	folderID, err := parseID(directoryID)
	if err != nil {
		return err
	}

	// Check if directory is empty with retries for eventual consistency
	// 123pan has eventual consistency, so we need to retry a few times
	// to ensure we don't delete a non-empty directory
	const maxRetries = 3
	for retry := 0; retry < maxRetries; retry++ {
		resp, err := f.listFiles(ctx, folderID, 100, 0)
		if err != nil {
			return err
		}

		// Check for non-trashed files
		for _, file := range resp.Data.FileList {
			if file.Trashed == 0 {
				return fs.ErrorDirectoryNotEmpty
			}
		}

		// If we got some results (even if all trashed), we can trust the API response
		if len(resp.Data.FileList) > 0 {
			break
		}

		// If empty response and not last retry, wait and try again
		// This handles eventual consistency where files might not be visible yet
		if retry < maxRetries-1 {
			fs.Debugf(f, "Rmdir: empty response, retrying %d/%d for eventual consistency", retry+1, maxRetries)
			time.Sleep(time.Second * time.Duration(retry+1))
		}
	}

	// Delete the directory
	err = f.trash(ctx, folderID)
	if err != nil {
		return err
	}

	f.dirCache.FlushDir(dir)
	return nil
}

// Move src to this remote using server side move operations
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't move - not same remote type")
		return nil, fs.ErrorCantMove
	}

	// Find destination directory
	dstLeaf, dstDirectoryID, err := f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return nil, err
	}

	dstParentID, err := parseID(dstDirectoryID)
	if err != nil {
		return nil, err
	}

	srcLeaf := path.Base(srcObj.remote)

	// Move to different directory if needed
	if srcObj.parentID != dstParentID {
		if err = f.move(ctx, srcObj.id, dstParentID); err != nil {
			return nil, err
		}
	}

	// Rename if needed
	if srcLeaf != dstLeaf {
		if err = f.rename(ctx, srcObj.id, f.opt.Enc.FromStandardName(dstLeaf)); err != nil {
			return nil, err
		}
	}

	return &Object{
		fs:          f,
		remote:      remote,
		hasMetaData: true,
		size:        srcObj.size,
		modTime:     srcObj.modTime,
		id:          srcObj.id,
		etag:        srcObj.etag,
		parentID:    dstParentID,
	}, nil
}

// Copy src to this remote using server side copy operations (via instant upload)
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}

	// Find destination directory
	dstLeaf, dstDirectoryID, err := f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return nil, err
	}

	dstParentID, err := parseID(dstDirectoryID)
	if err != nil {
		return nil, err
	}

	// Try instant upload (秒传) using the same etag
	createResp, err := f.createFile(ctx, dstParentID, f.opt.Enc.FromStandardName(dstLeaf), srcObj.etag, srcObj.size)
	if err != nil {
		return nil, err
	}

	// Only works if instant upload succeeded
	if !createResp.Data.Reuse || createResp.Data.FileID == 0 {
		return nil, fs.ErrorCantCopy
	}

	return &Object{
		fs:          f,
		remote:      remote,
		hasMetaData: true,
		size:        srcObj.size,
		modTime:     time.Now(),
		id:          createResp.Data.FileID,
		etag:        srcObj.etag,
		parentID:    dstParentID,
	}, nil
}

// About returns info about the 123pan account
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	info, err := f.getUserInfo(ctx)
	if err != nil {
		return nil, err
	}

	total := info.Data.SpacePermanent + info.Data.SpaceTemp
	used := info.Data.SpaceUsed
	free := total - used

	return &fs.Usage{
		Total: fs.NewUsageValue(total),
		Used:  fs.NewUsageValue(used),
		Free:  fs.NewUsageValue(free),
	}, nil
}

// PublicLink generates a public link to the remote path
func (f *Fs) PublicLink(ctx context.Context, remote string, expire fs.Duration, unlink bool) (string, error) {
	// unlink is not supported - 123pan doesn't have an API to remove share links by file
	if unlink {
		return "", fs.ErrorNotImplemented
	}

	// Find the object
	o, err := f.NewObject(ctx, remote)
	if err != nil {
		// Try as directory
		dirID, err := f.dirCache.FindDir(ctx, remote, false)
		if err != nil {
			return "", err
		}
		dirIDInt, err := parseID(dirID)
		if err != nil {
			return "", err
		}
		// Create share for directory
		return f.createShareLink(ctx, dirIDInt, path.Base(remote), expire)
	}

	// Create share for file
	obj := o.(*Object)
	return f.createShareLink(ctx, obj.id, path.Base(remote), expire)
}

// createShareLink creates a share link and returns the URL
func (f *Fs) createShareLink(ctx context.Context, fileID int64, name string, expire fs.Duration) (string, error) {
	// Convert expire duration to days
	// 123pan only supports: 1, 7, 30, or 0 (permanent)
	expireDays := 0 // default to permanent
	if expire > 0 {
		days := int(time.Duration(expire) / (24 * time.Hour))
		switch {
		case days <= 1:
			expireDays = 1
		case days <= 7:
			expireDays = 7
		case days <= 30:
			expireDays = 30
		default:
			expireDays = 0 // permanent for > 30 days
		}
	}

	resp, err := f.createShare(ctx, fileID, name, expireDays)
	if err != nil {
		return "", err
	}

	return "https://www.123pan.com/s/" + resp.Data.ShareKey, nil
}

// Purge deletes all the files and directories in dir
//
// 123pan's trash API can delete directories including their contents.
// If that fails, we fall back to batch deletion for efficiency.
func (f *Fs) Purge(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir, false)
}

// purgeCheck removes the directory, if check is set then it
// refuses to do so if it has anything in (for Rmdir)
func (f *Fs) purgeCheck(ctx context.Context, dir string, check bool) error {
	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return err
	}

	dirID, err := parseID(directoryID)
	if err != nil {
		return err
	}

	if check {
		// Check if directory is empty (for Rmdir)
		var hasContent bool
		err = f.forEachFile(ctx, dirID, func(file api.File) bool {
			if file.Trashed == 0 {
				hasContent = true
				return true // stop iteration
			}
			return false
		})
		if err != nil {
			return err
		}
		if hasContent {
			return fs.ErrorDirectoryNotEmpty
		}
	}

	// 123pan's trash API doesn't reliably support recursive delete for non-empty directories.
	// It may return success but not actually delete the contents.
	// So we always delete contents first, then delete the directory itself.

	// Recursively collect all file IDs (deepest first for safe deletion)
	fileIDs, err := f.collectAllFileIDs(ctx, dirID)
	if err != nil {
		return fmt.Errorf("Purge: failed to collect files: %w", err)
	}

	// Batch delete files
	for i := 0; i < len(fileIDs); i += trashBatchSize {
		end := i + trashBatchSize
		if end > len(fileIDs) {
			end = len(fileIDs)
		}
		if err := f.trashBatch(ctx, fileIDs[i:end]); err != nil {
			return fmt.Errorf("Purge: batch delete failed: %w", err)
		}
	}

	// Delete the directory itself (unless it's the true root directory)
	// When dir == "" but dirID != "0", we're deleting a subdirectory that is the root of this Fs
	if directoryID != rootID {
		if err := f.trash(ctx, dirID); err != nil {
			return fmt.Errorf("Purge: failed to delete directory: %w", err)
		}
	}

	f.dirCache.FlushDir(dir)
	return nil
}

// collectAllFileIDs recursively collects all file and directory IDs under parentID
// Returns IDs in reverse order (deepest first) for safe deletion
func (f *Fs) collectAllFileIDs(ctx context.Context, parentID int64) ([]int64, error) {
	var result []int64

	err := f.forEachFile(ctx, parentID, func(file api.File) bool {
		if file.Trashed != 0 {
			return false
		}
		if file.Type == 1 {
			// Directory - recurse first, then add directory ID
			subIDs, err := f.collectAllFileIDs(ctx, file.FileID)
			if err == nil {
				result = append(result, subIDs...)
			}
			result = append(result, file.FileID)
		} else {
			// File
			result = append(result, file.FileID)
		}
		return false
	})

	return result, err
}

// CleanUp empties the trash by permanently deleting all trashed files
func (f *Fs) CleanUp(ctx context.Context) error {
	// Collect all trashed file IDs by traversing the entire file tree
	// Note: 123pan's file list API returns both normal and trashed files
	trashedIDs, err := f.collectTrashedFileIDs(ctx, 0) // 0 is root
	if err != nil {
		return fmt.Errorf("CleanUp: failed to collect trashed files: %w", err)
	}

	if len(trashedIDs) == 0 {
		fs.Debugf(f, "CleanUp: no trashed files found")
		return nil
	}

	fs.Debugf(f, "CleanUp: found %d trashed files to permanently delete", len(trashedIDs))

	// Batch delete (max trashBatchSize per API call)
	for i := 0; i < len(trashedIDs); i += trashBatchSize {
		end := i + trashBatchSize
		if end > len(trashedIDs) {
			end = len(trashedIDs)
		}
		if err := f.deletePermanently(ctx, trashedIDs[i:end]); err != nil {
			return fmt.Errorf("CleanUp: batch delete failed: %w", err)
		}
	}

	fs.Infof(f, "CleanUp: permanently deleted %d trashed files", len(trashedIDs))
	return nil
}

// collectTrashedFileIDs recursively collects all trashed file IDs under parentID
func (f *Fs) collectTrashedFileIDs(ctx context.Context, parentID int64) ([]int64, error) {
	var result []int64

	err := f.forEachFile(ctx, parentID, func(file api.File) bool {
		if file.Trashed != 0 {
			// This file is in trash - collect it
			result = append(result, file.FileID)
		}
		if file.Type == 1 {
			// Directory - recurse into it (regardless of trashed status)
			// to find any trashed files inside
			subIDs, err := f.collectTrashedFileIDs(ctx, file.FileID)
			if err == nil {
				result = append(result, subIDs...)
			}
		}
		return false
	})

	return result, err
}

// ------------------------------------------------------------
// Object interface methods
// ------------------------------------------------------------

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Remote returns the remote path
func (o *Object) Remote() string {
	return o.remote
}

// String returns a string representation
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// Size returns the size of the object
func (o *Object) Size() int64 {
	return o.size
}

// ModTime returns the modification time of the object
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.modTime
}

// SetModTime sets the modification time (not supported)
func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
	return fs.ErrorCantSetModTime
}

// Storable returns whether object is storable
func (o *Object) Storable() bool {
	return true
}

// Hash returns the MD5 hash
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	if t != hash.MD5 {
		return "", hash.ErrUnsupported
	}
	return o.etag, nil
}

// ID returns the object ID
func (o *Object) ID() string {
	return formatID(o.id)
}

// Open opens the Object for reading
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	fs.FixRangeOption(options, o.size)

	// Get download URL
	resp, err := o.fs.getDownloadInfo(ctx, o.id)
	if err != nil {
		return nil, err
	}

	// Make HTTP request to download URL
	opts := rest.Opts{
		Method:  "GET",
		RootURL: resp.Data.DownloadURL,
		Options: options,
	}

	var httpResp *http.Response
	err = o.fs.pacer.Call(func() (bool, error) {
		httpResp, err = o.fs.srv.Call(ctx, &opts)
		return shouldRetry(ctx, httpResp, err)
	})
	if err != nil {
		return nil, err
	}

	return httpResp.Body, nil
}

// Update updates the object with the contents of the io.Reader
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	size := src.Size()
	if size < 0 {
		return errors.New("can't upload files of unknown size")
	}
	if size == 0 {
		return fs.ErrorCantUploadEmptyFiles
	}

	// Upload new content
	info, err := o.fs.upload(ctx, in, o.parentID, o.fs.opt.Enc.FromStandardName(path.Base(o.remote)), size, options...)
	if err != nil {
		return err
	}

	// Delete old file if ID changed
	if info.FileID != o.id {
		_ = o.fs.trash(ctx, o.id)
	}

	o.setMetaData(info)
	return nil
}

// Remove deletes the object
func (o *Object) Remove(ctx context.Context) error {
	return o.fs.trash(ctx, o.id)
}

// setMetaData sets the metadata from api.File
func (o *Object) setMetaData(info *api.File) {
	o.hasMetaData = true
	o.size = info.Size
	o.modTime = parseTime(info.UpdateAt)
	o.id = info.FileID
	o.etag = info.Etag
	o.parentID = info.ParentFileID
}

// readMetaData reads metadata from the server
func (o *Object) readMetaData(ctx context.Context) error {
	leaf, directoryID, err := o.fs.dirCache.FindPath(ctx, o.remote, false)
	if err != nil {
		if err == fs.ErrorDirNotFound {
			return fs.ErrorObjectNotFound
		}
		return err
	}

	parentID, err := parseID(directoryID)
	if err != nil {
		return err
	}

	var found bool
	err = o.fs.forEachFile(ctx, parentID, func(file api.File) bool {
		standardName := o.fs.opt.Enc.ToStandardName(file.Filename)
		if file.Trashed == 0 && standardName == leaf && file.Type == 0 {
			o.setMetaData(&file)
			found = true
			return true
		}
		return false
	})
	if err != nil {
		return err
	}

	if !found {
		return fs.ErrorObjectNotFound
	}
	return nil
}

// parseTime parses time from 123pan format
func parseTime(timeStr string) time.Time {
	// Format: "2006-01-02 15:04:05" in UTC+8
	loc := time.FixedZone("UTC+8", 8*60*60)
	t, err := time.ParseInLocation("2006-01-02 15:04:05", timeStr, loc)
	if err != nil {
		return time.Now()
	}
	return t
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.Copier          = (*Fs)(nil)
	_ fs.Abouter         = (*Fs)(nil)
	_ fs.PutUncheckeder  = (*Fs)(nil)
	_ fs.PublicLinker    = (*Fs)(nil)
	_ fs.Purger          = (*Fs)(nil)
	_ fs.CleanUpper      = (*Fs)(nil)
	_ dircache.DirCacher = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.IDer            = (*Object)(nil)
)
