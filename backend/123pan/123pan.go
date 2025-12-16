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
	apiBaseURL = "https://open-api.123pan.com"
	rootID     = "0"
	minSleep   = 200 * time.Millisecond
	maxSleep   = 5 * time.Second
)

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
type QPSLimits struct {
	FileList     int
	FileMove     int
	FileDelete   int
	Mkdir        int
	DownloadInfo int
	UploadCreate int
}

var (
	// Rate limits for free users
	freeUserQPS = QPSLimits{
		FileList:     5,
		FileMove:     3,
		FileDelete:   1,
		Mkdir:        5,
		DownloadInfo: 5,
		UploadCreate: 2,
	}
	// Rate limits for VIP users
	vipUserQPS = QPSLimits{
		FileList:     10,
		FileMove:     10,
		FileDelete:   10,
		Mkdir:        20,
		DownloadInfo: 10,
		UploadCreate: 5,
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

// forEachFile iterates over all files in a directory, calling fn for each file.
// If fn returns true, the iteration stops early (useful for search operations).
// Returns an error if the listing fails.
func (f *Fs) forEachFile(ctx context.Context, parentID int64, fn func(file api.File) (stop bool)) error {
	lastFileID := int64(0)
	for lastFileID != -1 {
		resp, err := f.listFiles(ctx, parentID, 100, lastFileID)
		if err != nil {
			return err
		}

		for _, file := range resp.Data.FileList {
			if fn(file) {
				return nil
			}
		}
		lastFileID = resp.Data.LastFileID
	}
	return nil
}

// FindLeaf finds a directory of name leaf in the folder with ID pathID
func (f *Fs) FindLeaf(ctx context.Context, pathID, leaf string) (pathIDOut string, found bool, err error) {
	parentID, err := strconv.ParseInt(pathID, 10, 64)
	if err != nil {
		return "", false, err
	}

	err = f.forEachFile(ctx, parentID, func(file api.File) bool {
		standardName := f.opt.Enc.ToStandardName(file.Filename)
		if file.Trashed == 0 && standardName == leaf && file.Type == 1 {
			pathIDOut = strconv.FormatInt(file.FileID, 10)
			found = true
			return true
		}
		return false
	})

	return pathIDOut, found, err
}

// CreateDir makes a directory with pathID as parent and name leaf
func (f *Fs) CreateDir(ctx context.Context, pathID, leaf string) (newID string, err error) {
	parentID, err := strconv.ParseInt(pathID, 10, 64)
	if err != nil {
		return "", err
	}

	resp, err := f.mkdir(ctx, parentID, f.opt.Enc.FromStandardName(leaf))
	if err != nil {
		return "", err
	}

	return strconv.FormatInt(resp.Data.DirID, 10), nil
}

// List the objects and directories in dir into entries
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return nil, err
	}

	parentID, err := strconv.ParseInt(directoryID, 10, 64)
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
			d := fs.NewDir(remote, modTime).SetID(strconv.FormatInt(file.FileID, 10))
			entries = append(entries, d)
			f.dirCache.Put(remote, strconv.FormatInt(file.FileID, 10))
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

	parentID, err := strconv.ParseInt(directoryID, 10, 64)
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

	folderID, err := strconv.ParseInt(directoryID, 10, 64)
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

	dstParentID, err := strconv.ParseInt(dstDirectoryID, 10, 64)
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

	dstParentID, err := strconv.ParseInt(dstDirectoryID, 10, 64)
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
		dirIDInt, err := strconv.ParseInt(dirID, 10, 64)
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
	return strconv.FormatInt(o.id, 10)
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

	parentID, err := strconv.ParseInt(directoryID, 10, 64)
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
	_ dircache.DirCacher = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.IDer            = (*Object)(nil)
)
