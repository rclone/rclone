// Package gphotosmobile provides an interface to Google Photos via the mobile API.
//
// # Library sync protocol
//
// Google Photos uses a delta-sync protocol similar to Google Drive's changes
// API, but over protobuf. The key concepts:
//
//   - state_token: An opaque string representing a point in the library's
//     change history. Sending a state_token returns only changes SINCE that
//     point. Sending an empty state_token returns the full library.
//   - page_token: For large responses, the server paginates. A non-empty
//     page_token means "there are more results; send this token to get
//     the next page."
//
// # Initial sync (first run)
//
// On first run, the cache is empty and init_complete=false in SQLite:
//
//  1. Call GetLibraryState("") — empty state_token means "give me everything."
//  2. Server returns: a batch of items, a new state_token, and possibly a
//     page_token if the library is large.
//  3. Save items + tokens to SQLite. If page_token is set, call
//     GetLibraryPageInit(page_token) repeatedly until page_token is empty.
//  4. Mark init_complete=true. The state_token is now our bookmark.
//
// For a 35K-item library, initial sync takes ~30 seconds and involves
// 10-20 paginated requests (each returns ~3000 items).
//
// # Incremental sync (subsequent runs)
//
// On subsequent runs, init_complete=true and we have a saved state_token:
//
//  1. Call GetLibraryState(saved_state_token).
//  2. Server returns only items changed/added/deleted since that token.
//  3. Upsert changed items, delete removed ones, save new state_token.
//  4. If page_token is set, call GetLibraryPage(page_token, state_token)
//     repeatedly until exhausted.
//
// Incremental syncs are very fast — typically 0 items if nothing changed,
// or a handful of items for recent uploads/deletes.
//
// # Sync throttling
//
// To avoid hammering the API, incremental syncs are throttled to at most
// once every 30 seconds (syncIntervalSeconds). The last sync time is
// persisted in SQLite so it survives across rclone invocations within
// the same cache database. A forceSync flag bypasses the throttle after
// uploads/deletes so changes appear immediately.
//
// # Virtual filesystem
//
// The backend presents a virtual directory structure:
//
//	/           → shows "media" directory
//	/media/     → flat listing of all non-trashed items from the cache
//
// Albums are not yet implemented. All items appear directly under /media/
// regardless of any album membership.
//
// # Duplicate filenames
//
// Google Photos allows multiple items with the same filename (e.g. two
// different photos both named "IMG_0001.jpg"). When listing, if a filename
// appears more than once, ALL copies get a deterministic suffix:
//
//	IMG_0001_<dedup_key>.jpg
//
// where dedup_key is a URL-safe base64-encoded SHA1 hash. This ensures
// every item has a unique filename and the names are stable across syncs.
package gphotosmobile

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/encoder"
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
				_, err := api.bearerToken(ctx)
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
			Help:     "Path to SQLite cache database.\n\nThe media library index is cached locally in SQLite for fast listing.\nThe default location is inside rclone's cache directory\n(see --cache-dir) at gphotosmobile/<remote-name>.db.\n\nThe initial sync downloads the full library index and may take a few\nminutes for large libraries. Subsequent runs use fast incremental sync.\nIf this file is deleted, rclone will re-sync the full library on next run.",
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
		}, {
			Name:     "download_cache",
			Help:     "Enable download cache for opened files.\n\nGoogle Photos download URLs do not support HTTP Range requests.\nWhen enabled, each opened file is downloaded once to a local temp\nfile and shared across concurrent readers, with instant seeking.\nThis is useful for rclone mount where the VFS layer needs to seek.\n\nWhen disabled (the default), Open() returns the raw HTTP stream\ndirectly with no temp files. Use --vfs-cache-mode full for mount.",
			Default:  false,
			Advanced: true,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			Default: (encoder.Base |
				encoder.EncodeCrLf |
				encoder.EncodeInvalidUtf8),
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	AuthData      string               `config:"auth_data"`
	CacheDBPath   string               `config:"cache_db_path"`
	DeviceModel   string               `config:"device_model"`
	DeviceMake    string               `config:"device_make"`
	DownloadCache bool                 `config:"download_cache"`
	Enc           encoder.MultiEncoder `config:"encoding"`
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
		cachePath = defaultCachePath(name)
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
	}
	if opt.DownloadCache {
		f.dlCache = newDownloadCache(api)
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

	// Decode the filename back to the original name for cache lookup
	fileName = f.opt.Enc.ToStandardName(fileName)

	// Build the full path for lookup
	fullRemote := remote
	if f.root != "" {
		fullRemote = f.root + "/" + remote
	}

	// Search in the cache by filename
	item, err := f.cache.GetByFileName(fileName)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fs.ErrorObjectNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("cache lookup failed: %w", err)
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

		// First pass: count filename occurrences to detect duplicates
		nameCount := make(map[string]int)
		for _, item := range items {
			if item.FileName != "" {
				nameCount[item.FileName]++
			}
		}

		for _, item := range items {
			if item.FileName == "" {
				continue
			}
			// If filename has duplicates, append dedup_key suffix to ALL of them
			// for deterministic naming regardless of database ordering
			fileName := item.FileName
			if nameCount[fileName] > 1 {
				ext := path.Ext(fileName)
				base := fileName[:len(fileName)-len(ext)]
				suffix := item.DedupKey
				if suffix == "" {
					suffix = item.MediaKey
				}
				fileName = fmt.Sprintf("%s_%s%s", base, suffix, ext)
			}

			// Encode the filename for rclone's virtual filesystem
			fileName = f.opt.Enc.FromStandardName(fileName)

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
	size := src.Size()

	// Stream the input through SHA1 hasher into a temp file
	// so we never hold the entire file in memory
	tmpFile, err := os.CreateTemp("", "gphotosmobile_upload_*.tmp")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
	}()

	hasher := sha1.New()
	tee := io.TeeReader(in, hasher)
	written, err := io.Copy(tmpFile, tee)
	if err != nil {
		return nil, fmt.Errorf("failed to write temp file: %w", err)
	}
	if size < 0 {
		size = written
	}

	sha1Bytes := hasher.Sum(nil)
	sha1B64 := base64.StdEncoding.EncodeToString(sha1Bytes)

	// Check if already exists
	mediaKey, err := f.api.FindRemoteMediaByHash(ctx, sha1Bytes)
	if err == nil && mediaKey != "" {
		fs.Debugf(f, "File %q already exists with media key %s", remote, mediaKey)
	} else {
		// Upload
		uploadToken, err := f.api.GetUploadToken(ctx, sha1B64, size)
		if err != nil {
			return nil, fmt.Errorf("failed to get upload token: %w", err)
		}

		// Seek back to start of temp file for upload
		if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
			return nil, fmt.Errorf("failed to seek temp file: %w", err)
		}

		uploadResp, err := f.api.UploadFile(ctx, tmpFile, size, uploadToken)
		if err != nil {
			return nil, fmt.Errorf("upload failed: %w", err)
		}

		_, fileName := path.Split(remote)
		fileName = f.opt.Enc.ToStandardName(fileName)
		mediaKey, err = f.api.CommitUpload(ctx, uploadResp, fileName, sha1Bytes)
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
			SizeBytes: size,
		},
		hasMedia: true,
	}
	return o, nil
}

// Mkdir creates the directory if it doesn't exist.
// Only the virtual "media" directory is accepted.
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	fullDir := dir
	if f.root != "" {
		if dir != "" {
			fullDir = f.root + "/" + dir
		} else {
			fullDir = f.root
		}
	}
	switch fullDir {
	case "", "media":
		// These virtual directories always exist
		return nil
	}
	return fs.ErrorDirNotFound
}

// Shutdown closes the SQLite cache database and cleans up any download
// cache temp files. Called by rclone when the Fs is no longer needed.
func (f *Fs) Shutdown(ctx context.Context) error {
	if f.dlCache != nil {
		f.dlCache.shutdown()
	}
	return f.cache.Close()
}

// Rmdir removes the directory.
// Virtual directories cannot be removed.
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	fullDir := dir
	if f.root != "" {
		if dir != "" {
			fullDir = f.root + "/" + dir
		} else {
			fullDir = f.root
		}
	}
	switch fullDir {
	case "", "media":
		// Virtual directories cannot be removed
		return errors.New("can't remove virtual directory")
	}
	return fs.ErrorDirNotFound
}

// syncIntervalSeconds controls the minimum interval between incremental syncs.
// Library listing calls ensureCachePopulated on every List/NewObject, so this
// prevents excessive API calls when rclone makes many calls in quick succession
// (e.g. during a recursive listing).
const syncIntervalSeconds = 30

// ensureCachePopulated is the main entry point for keeping the local SQLite
// cache in sync with the remote Google Photos library. It is called by
// List, NewObject, and any method that reads from the cache.
//
// The logic is:
//  1. If init_complete is false → run fullCacheInit (downloads entire library).
//  2. If init_complete is true and ≥30s since last sync → run incrementalSync.
//  3. If forceSync is true (set after Put/Remove) → sync regardless of time.
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

// fullCacheInit performs the initial full library download.
//
// The sequence is:
//  1. Check if there's a saved page_token from a previous interrupted init.
//     If so, resume pagination from where we left off (crash recovery).
//  2. Call GetLibraryState("") to get the first batch + state_token + page_token.
//  3. Save everything to SQLite.
//  4. If page_token is non-empty, paginate via processInitPages until done.
//
// Each page returns ~3000 items. Progress is logged with item counts.
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
	respData, err := f.api.GetLibraryState(ctx, stateToken)
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
		respData, err := f.api.GetLibraryPageInit(ctx, pageToken)
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

// incrementalSync fetches only changes since the last saved state_token.
// This is the fast path used on all runs after the initial sync.
// Typically returns 0 items if nothing changed in the user's library.
func (f *Fs) incrementalSync(ctx context.Context) error {
	stateToken, _, err := f.cache.GetStateTokens()
	if err != nil {
		return err
	}

	if stateToken == "" {
		// No state token means we haven't done init yet
		return nil
	}

	respData, err := f.api.GetLibraryState(ctx, stateToken)
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
		respData, err := f.api.GetLibraryPage(ctx, pageToken, stateToken)
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

// About gets quota information from the account.
// It sends GetLibraryState("") (empty state token) specifically to get the
// account info block, which is only present in full-sync responses, not in
// incremental ones. The account info is at field path:
//
//	1.4.5.1  = total item count
//	1.4.8.1  = bytes used against quota
//	1.4.8.2  = total quota bytes
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	respData, err := f.api.GetLibraryState(ctx, "")
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
//
// When download_cache is enabled (default), Google Photos download URLs
// are fetched once into a shared temp file. Multiple Open() calls for
// the same file share the same download, and the returned reader
// implements fs.RangeSeeker for instant seeking within cached data.
//
// When download_cache is disabled, Open() streams the HTTP response body
// directly. This avoids temp file disk usage but does not support
// seeking — RangeOption/SeekOption are ignored and the caller must read
// sequentially. This mode is suitable for rclone copy/sync but NOT for
// rclone mount.
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	if !o.hasMedia || o.media.MediaKey == "" {
		return nil, errors.New("no media key available")
	}

	// When download cache is disabled, stream directly from the API
	if o.fs.dlCache == nil {
		downloadURL, err := o.fs.api.GetDownloadURL(ctx, o.media.MediaKey)
		if err != nil {
			return nil, fmt.Errorf("failed to get download URL: %w", err)
		}
		body, err := o.fs.api.DownloadFile(ctx, downloadURL)
		if err != nil {
			return nil, fmt.Errorf("failed to download file: %w", err)
		}
		return body, nil
	}

	// Get or start a shared download for this media key
	entry, err := o.fs.dlCache.getOrStart(ctx, o.media.MediaKey, o.media.SizeBytes)
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
	err := o.fs.api.MoveToTrash(ctx, []string{o.media.DedupKey})
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
	_ fs.Fs         = (*Fs)(nil)
	_ fs.Abouter    = (*Fs)(nil)
	_ fs.Shutdowner = (*Fs)(nil)
	_ fs.Object     = (*Object)(nil)
	_ fs.IDer       = (*Object)(nil)
)
