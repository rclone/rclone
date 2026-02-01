// Package gphotosmobile provides an interface to Google Photos via the mobile API
package gphotosmobile

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/hash"
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "gphotos_mobile",
		Description: "Google Photos (Mobile API - full access)",
		NewFs:       NewFs,
		Config: func(ctx context.Context, name string, m configmap.Mapper, configIn fs.ConfigIn) (*fs.ConfigOut, error) {
			switch configIn.State {
			case "":
				// Post-config validation: check auth_data and show account info
				authData, _ := m.Get("auth_data")
				if authData == "" {
					return nil, nil
				}
				email := parseEmail(authData)
				if email == "" {
					fs.Logf(nil, "Warning: could not parse email from auth_data")
					return nil, nil
				}
				// Test the auth by getting a bearer token
				deviceModel, _ := m.Get("device_model")
				deviceMake, _ := m.Get("device_make")
				api := NewMobileAPI(ctx, authData, deviceModel, deviceMake)
				_, err := api.bearerToken()
				if err != nil {
					return fs.ConfigError("", fmt.Sprintf("Authentication failed for %s: %v\nCheck that your auth_data is correct and not expired.", email, err))
				}
				fs.Infof(nil, "Successfully authenticated as %s", email)
				return nil, nil // Done — auth works
			}
			return nil, fmt.Errorf("unknown config state %q", configIn.State)
		},
		Options: []fs.Option{{
			Name:      "auth_data",
			Help:      "Google Photos mobile API auth data.\n\nThis is the auth string containing your Android device credentials\nfor Google Photos. It starts with 'androidId='.\n\nSee the documentation for instructions on how to obtain it\nusing Google Photos ReVanced and ADB logcat.",
			Required:  true,
			Sensitive: true,
		}, {
			Name:     "cache_db_path",
			Help:     "Path to SQLite cache database.\n\nLeave empty for default (~/.gpmc/<email>/storage.db).\nThe cache stores the media library index locally for fast listing.",
			Default:  "",
			Advanced: true,
		}, {
			Name:     "device_model",
			Help:     "Device model to report to Google Photos.\n\nThis is included in the user agent and upload metadata.\nLeave empty for default (Pixel 9a).",
			Default:  "",
			Advanced: true,
		}, {
			Name:     "device_make",
			Help:     "Device manufacturer to report to Google Photos.\n\nThis is included in upload metadata.\nLeave empty for default (Google).",
			Default:  "",
			Advanced: true,
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	AuthData    string `config:"auth_data"`
	CacheDBPath string `config:"cache_db_path"`
	DeviceModel string `config:"device_model"`
	DeviceMake  string `config:"device_make"`
}

// Fs represents a remote Google Photos storage via mobile API
type Fs struct {
	name      string         // name of this remote
	root      string         // the path we are working on
	opt       Options        // parsed options
	features  *fs.Features   // optional features
	api       *MobileAPI     // mobile API client
	cache     *Cache         // SQLite cache
	cacheMu   sync.Mutex     // protects cache operations
	startTime time.Time      // time Fs was started
	dlCache   *downloadCache // shared download cache for temp files
	forceSync bool           // force next sync regardless of throttle
}

// Object describes a Google Photos media item
type Object struct {
	fs       *Fs       // parent Fs
	remote   string    // remote path
	media    MediaItem // cached media item info
	hasMedia bool      // whether media info is populated
}

// ------------------------------------------------------------
// Fs interface methods
// ------------------------------------------------------------

// Name of the remote
func (f *Fs) Name() string { return f.name }

// Root of the remote
func (f *Fs) Root() string { return f.root }

// String converts this Fs to a string
func (f *Fs) String() string {
	return fmt.Sprintf("Google Photos Mobile path %q", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features { return f.features }

// Precision of the remote
func (f *Fs) Precision() time.Duration { return time.Second }

// Hashes returns the supported hash sets
func (f *Fs) Hashes() hash.Set { return hash.Set(hash.SHA1) }

// NewFs constructs an Fs from the path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse config: %w", err)
	}

	authData := opt.AuthData
	if authData == "" {
		return nil, errors.New("auth_data is required - set it via config or GP_AUTH_DATA env var")
	}

	root = strings.Trim(path.Clean(root), "/")
	if root == "." || root == "/" {
		root = ""
	}

	api := NewMobileAPI(ctx, authData, opt.DeviceModel, opt.DeviceMake)

	// Determine cache path
	cachePath := opt.CacheDBPath
	if cachePath == "" {
		email := parseEmail(authData)
		if email == "" {
			email = "default"
		}
		cachePath = defaultCachePath(email)
	}

	cache, err := NewCache(cachePath)
	if err != nil {
		return nil, fmt.Errorf("couldn't open cache database: %w", err)
	}

	f := &Fs{
		name:      name,
		root:      root,
		opt:       *opt,
		api:       api,
		cache:     cache,
		startTime: time.Now(),
		dlCache:   newDownloadCache(api),
	}

	f.features = (&fs.Features{
		ReadMimeType:   false,
		WriteMimeType:  false,
		DuplicateFiles: true,
	}).Fill(ctx, f)

	// Check if root points to a file
	if root != "" {
		_, leaf := path.Split(root)
		_, err := f.NewObject(ctx, leaf)
		if err == nil {
			// Root is a file
			f.root = path.Dir(root)
			if f.root == "." {
				f.root = ""
			}
			return f, fs.ErrorIsFile
		}
	}

	return f, nil
}

// NewObject finds the Object at remote
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	// Ensure cache is up to date
	if err := f.ensureCachePopulated(ctx); err != nil {
		return nil, fmt.Errorf("cache sync failed: %w", err)
	}

	// Look up by filename in cache
	dir, fileName := path.Split(remote)
	dir = strings.Trim(dir, "/")

	// Build the full path for lookup
	fullRemote := remote
	if f.root != "" {
		fullRemote = f.root + "/" + remote
	}

	// Search in the cache by filename
	item, err := f.cache.GetByFileName(fileName)
	if err != nil {
		return nil, fs.ErrorObjectNotFound
	}

	// Verify the item matches the path context
	_ = fullRemote // In flat mode, we just match by filename
	_ = dir

	o := &Object{
		fs:       f,
		remote:   remote,
		media:    *item,
		hasMedia: true,
	}
	return o, nil
}

// List the objects and directories in dir into entries
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	// Ensure cache is populated
	err = f.ensureCachePopulated(ctx)
	if err != nil {
		return nil, fmt.Errorf("cache sync failed: %w", err)
	}

	fullDir := dir
	if f.root != "" {
		if dir != "" {
			fullDir = f.root + "/" + dir
		} else {
			fullDir = f.root
		}
	}

	// Virtual filesystem:
	// "" (root) -> show "media" and "album" dirs
	// "media" -> flat list of all media
	// "album" -> list of albums (not implemented yet)
	// "album/<name>" -> list of items in album

	if fullDir == "" {
		// Root: show virtual directories
		entries = append(entries, fs.NewDir("media", f.startTime))
		// entries = append(entries, fs.NewDir("album", f.startTime))
		return entries, nil
	}

	if fullDir == "media" {
		// List all media items from cache
		items, err := f.cache.ListAll()
		if err != nil {
			return nil, fmt.Errorf("cache list failed: %w", err)
		}

		seen := make(map[string]int)
		for _, item := range items {
			if item.FileName == "" {
				continue
			}
			// Handle duplicates by adding media_key suffix
			fileName := item.FileName
			seen[fileName]++
			if seen[fileName] > 1 {
				ext := path.Ext(fileName)
				base := fileName[:len(fileName)-len(ext)]
				fileName = fmt.Sprintf("%s_%s%s", base, item.MediaKey[:8], ext)
			}

			// Remote path is relative to f.root
			// If f.root is "media", dir will be "" and remote should be just the filename
			// If f.root is "", dir will be "media" and remote should be "media/filename"
			remote := fileName
			if dir != "" {
				remote = dir + "/" + fileName
			}

			o := &Object{
				fs:       f,
				remote:   remote,
				media:    item,
				hasMedia: true,
			}
			entries = append(entries, o)
		}
		return entries, nil
	}

	return nil, fs.ErrorDirNotFound
}

// Put uploads a file
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	remote := src.Remote()

	// Read the file into memory for hashing and upload
	data, err := io.ReadAll(in)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	// Calculate SHA1
	sha1Bytes, sha1B64 := calculateSHA1(data)

	// Check if already exists
	mediaKey, err := f.api.FindRemoteMediaByHash(sha1Bytes)
	if err == nil && mediaKey != "" {
		fs.Debugf(f, "File %q already exists with media key %s", remote, mediaKey)
	} else {
		// Upload
		uploadToken, err := f.api.GetUploadToken(sha1B64, int64(len(data)))
		if err != nil {
			return nil, fmt.Errorf("failed to get upload token: %w", err)
		}

		uploadResp, err := f.api.UploadFile(data, uploadToken)
		if err != nil {
			return nil, fmt.Errorf("upload failed: %w", err)
		}

		_, fileName := path.Split(remote)
		mediaKey, err = f.api.CommitUpload(uploadResp, fileName, sha1Bytes)
		if err != nil {
			return nil, fmt.Errorf("commit upload failed: %w", err)
		}
	}

	// Force immediate cache sync so the new file appears in listings
	f.cacheMu.Lock()
	f.forceSync = true
	f.cacheMu.Unlock()
	_ = f.ensureCachePopulated(ctx)

	o := &Object{
		fs:     f,
		remote: remote,
		media: MediaItem{
			MediaKey:  mediaKey,
			FileName:  path.Base(remote),
			SizeBytes: int64(len(data)),
		},
		hasMedia: true,
	}
	return o, nil
}

// Mkdir creates the directory if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	// We support creating album directories in the future
	return nil
}

// Rmdir removes the directory
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return nil
}

// syncIntervalSeconds controls how often incremental syncs happen
const syncIntervalSeconds = 30

// ensureCachePopulated does a full init if needed, then an incremental
// sync if at least syncIntervalSeconds has passed since the last one.
// The last sync time is persisted in SQLite so it survives across rclone invocations.
func (f *Fs) ensureCachePopulated(ctx context.Context) error {
	f.cacheMu.Lock()
	defer f.cacheMu.Unlock()

	initComplete, err := f.cache.GetInitState()
	if err != nil {
		return err
	}

	if !initComplete {
		fs.Infof(f, "Initializing library cache (first run)...")
		err = f.fullCacheInit(ctx)
		if err != nil {
			return fmt.Errorf("cache init failed: %w", err)
		}
		err = f.cache.SetInitState(true)
		if err != nil {
			return err
		}
		_ = f.cache.SetLastSyncTime(time.Now().Unix())
		return nil
	}

	// Check persistent last sync time
	lastSync, err := f.cache.GetLastSyncTime()
	if err != nil {
		return err
	}

	now := time.Now().Unix()
	if f.forceSync || now-lastSync >= syncIntervalSeconds {
		fs.Debugf(f, "Performing incremental cache sync...")
		err = f.incrementalSync(ctx)
		if err != nil {
			return err
		}
		_ = f.cache.SetLastSyncTime(now)
		f.forceSync = false
	}

	return nil
}

// fullCacheInit does the initial full library sync
func (f *Fs) fullCacheInit(ctx context.Context) error {
	stateToken, pageToken, err := f.cache.GetStateTokens()
	if err != nil {
		return err
	}

	// If there's a saved page token, resume pagination
	if pageToken != "" {
		err = f.processInitPages(ctx, pageToken)
		if err != nil {
			return err
		}
	}

	// Get initial library state
	respData, err := f.api.GetLibraryState(stateToken)
	if err != nil {
		return fmt.Errorf("get library state failed: %w", err)
	}

	newStateToken, newPageToken, items, deletions := ParseDbUpdate(respData)

	err = f.cache.UpdateStateTokens(newStateToken, newPageToken)
	if err != nil {
		return err
	}
	err = f.cache.UpsertItems(items)
	if err != nil {
		return err
	}
	if len(deletions) > 0 {
		err = f.cache.DeleteItems(deletions)
		if err != nil {
			return err
		}
	}

	fs.Infof(f, "Init: %d items synced, %d deleted", len(items), len(deletions))

	if newPageToken != "" {
		err = f.processInitPages(ctx, newPageToken)
		if err != nil {
			return err
		}
	}

	return nil
}

// processInitPages processes paginated init responses
func (f *Fs) processInitPages(ctx context.Context, pageToken string) error {
	for pageToken != "" {
		respData, err := f.api.GetLibraryPageInit(pageToken)
		if err != nil {
			return fmt.Errorf("get library page init failed: %w", err)
		}

		_, nextPageToken, items, deletions := ParseDbUpdate(respData)

		err = f.cache.UpdateStateTokens("", nextPageToken)
		if err != nil {
			return err
		}
		err = f.cache.UpsertItems(items)
		if err != nil {
			return err
		}
		if len(deletions) > 0 {
			err = f.cache.DeleteItems(deletions)
			if err != nil {
				return err
			}
		}

		fs.Infof(f, "Init page: %d items synced, %d deleted", len(items), len(deletions))
		pageToken = nextPageToken
	}
	return nil
}

// incrementalSync does an incremental delta sync
func (f *Fs) incrementalSync(ctx context.Context) error {
	stateToken, _, err := f.cache.GetStateTokens()
	if err != nil {
		return err
	}

	if stateToken == "" {
		// No state token means we haven't done init yet
		return nil
	}

	respData, err := f.api.GetLibraryState(stateToken)
	if err != nil {
		return fmt.Errorf("incremental sync failed: %w", err)
	}

	newStateToken, newPageToken, items, deletions := ParseDbUpdate(respData)

	err = f.cache.UpdateStateTokens(newStateToken, newPageToken)
	if err != nil {
		return err
	}
	err = f.cache.UpsertItems(items)
	if err != nil {
		return err
	}
	if len(deletions) > 0 {
		err = f.cache.DeleteItems(deletions)
		if err != nil {
			return err
		}
	}

	if len(items) > 0 || len(deletions) > 0 {
		fs.Infof(f, "Incremental sync: %d updated, %d deleted", len(items), len(deletions))
	}

	// Process additional pages if needed
	if newPageToken != "" {
		return f.processPages(ctx, stateToken, newPageToken)
	}

	return nil
}

// processPages processes paginated delta responses
func (f *Fs) processPages(ctx context.Context, stateToken, pageToken string) error {
	for pageToken != "" {
		respData, err := f.api.GetLibraryPage(pageToken, stateToken)
		if err != nil {
			return fmt.Errorf("get library page failed: %w", err)
		}

		_, nextPageToken, items, deletions := ParseDbUpdate(respData)

		err = f.cache.UpdateStateTokens("", nextPageToken)
		if err != nil {
			return err
		}
		err = f.cache.UpsertItems(items)
		if err != nil {
			return err
		}
		if len(deletions) > 0 {
			err = f.cache.DeleteItems(deletions)
			if err != nil {
				return err
			}
		}

		fs.Infof(f, "Sync page: %d updated, %d deleted", len(items), len(deletions))
		pageToken = nextPageToken
	}
	return nil
}

// About gets quota information from the account
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	// Send a sync request with empty state token to get the full response
	// including account info at root.1.4. Incremental responses (with state
	// token) only include changed items and omit the account info block.
	respData, err := f.api.GetLibraryState("")
	if err != nil {
		return nil, fmt.Errorf("failed to get library state: %w", err)
	}

	root, err := DecodeRaw(respData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	field1, err := root.GetMessage(1)
	if err != nil {
		return nil, fmt.Errorf("no field 1 in response: %w", err)
	}

	usage := &fs.Usage{}

	// root.1.4 = account info
	if accountInfo, err := field1.GetMessage(4); err == nil {
		// root.1.4.5.1 = total item count
		if itemCountMsg, err := accountInfo.GetMessage(5); err == nil {
			count := itemCountMsg.GetVarint(1)
			usage.Objects = fs.NewUsageValue(count)
		}

		// root.1.4.8 = quota info
		if quotaInfo, err := accountInfo.GetMessage(8); err == nil {
			used := quotaInfo.GetVarint(1)  // bytes used against quota
			total := quotaInfo.GetVarint(2) // total quota bytes
			if total > 0 {
				usage.Total = fs.NewUsageValue(total)
				usage.Used = fs.NewUsageValue(used)
				usage.Free = fs.NewUsageValue(total - used)
			}
		}
	}

	return usage, nil
}

// ------------------------------------------------------------
// Object interface methods
// ------------------------------------------------------------

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info { return o.fs }

// Remote returns the remote path
func (o *Object) Remote() string { return o.remote }

// String returns a description of the Object
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// Size returns the size of the object
func (o *Object) Size() int64 {
	if !o.hasMedia {
		return -1
	}
	return o.media.SizeBytes
}

// ModTime returns the modification time of the object
func (o *Object) ModTime(ctx context.Context) time.Time {
	if !o.hasMedia {
		return time.Time{}
	}
	if o.media.UTCTimestamp > 0 {
		return time.Unix(o.media.UTCTimestamp/1000, (o.media.UTCTimestamp%1000)*int64(time.Millisecond))
	}
	return time.Time{}
}

// SetModTime sets the modification time
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	// Not supported by the API
	return nil
}

// Storable returns whether this object is storable
func (o *Object) Storable() bool { return true }

// Hash returns the SHA1 hash of the object as a lowercase hex string
func (o *Object) Hash(ctx context.Context, ty hash.Type) (string, error) {
	if ty != hash.SHA1 {
		return "", hash.ErrUnsupported
	}
	if !o.hasMedia {
		return "", nil
	}
	return o.media.SHA1Hash, nil
}

// Open opens the object for reading.
// Google Photos download URLs don't support HTTP Range requests,
// so we use a shared download cache that downloads each file once
// to a temp file in the background. Multiple Open() calls for the
// same file share the same download, and the returned reader
// implements fs.RangeSeeker for instant seeking within cached data.
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	if !o.hasMedia || o.media.MediaKey == "" {
		return nil, errors.New("no media key available")
	}

	// Get or start a shared download for this media key
	entry, err := o.fs.dlCache.getOrStart(o.media.MediaKey, o.media.SizeBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to start download: %w", err)
	}

	// Create a reader backed by the shared download
	reader := &cachedReader{
		entry:    entry,
		mediaKey: o.media.MediaKey,
		dc:       o.fs.dlCache,
	}

	// Apply initial seek from options
	for _, option := range options {
		switch opt := option.(type) {
		case *fs.RangeOption:
			if opt.Start >= 0 {
				reader.readPos = opt.Start
			}
		case *fs.SeekOption:
			reader.readPos = opt.Offset
		}
	}

	return reader, nil
}

// Update replaces the contents of the object
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	// Re-upload
	newObj, err := o.fs.Put(ctx, in, src, options...)
	if err != nil {
		return err
	}
	newO := newObj.(*Object)
	o.media = newO.media
	o.hasMedia = newO.hasMedia
	return nil
}

// Remove deletes the object (moves to trash)
func (o *Object) Remove(ctx context.Context) error {
	if !o.hasMedia || o.media.DedupKey == "" {
		return errors.New("no dedup key available for trash operation")
	}
	err := o.fs.api.MoveToTrash([]string{o.media.DedupKey})
	if err != nil {
		return err
	}
	// Force immediate cache sync so the deletion is reflected
	o.fs.cacheMu.Lock()
	o.fs.forceSync = true
	o.fs.cacheMu.Unlock()
	_ = o.fs.ensureCachePopulated(ctx)
	return nil
}

// ID returns the ID of the Object if known, or "" if not
func (o *Object) ID() string {
	return o.media.MediaKey
}

// Check the interfaces are satisfied
var (
	_ fs.Fs      = (*Fs)(nil)
	_ fs.Abouter = (*Fs)(nil)
	_ fs.Object  = (*Object)(nil)
	_ fs.IDer    = (*Object)(nil)
)
