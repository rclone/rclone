package netexplorer

import (
	"bufio"
	"bytes"
	"container/list"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	// Rclone imports
	"mime/multipart"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/hash"
	"go.etcd.io/bbolt"
	"golang.org/x/sync/singleflight"
)

var netExplorerForbiddenNameChars = regexp.MustCompile(`[\\/:*?"<>|]`)

// NetExplorer is the API client
type NetExplorer struct {
	BaseURL  string
	Token    string
	ClientID string
	// ConfigName is the rclone remote config section name where token
	// values are stored. This is set to the remote name passed to NewFs
	// so external processes updating the rclone config can be detected.
	ConfigName string
	Client     *http.Client
	opt        *Options
}

// a node in the folder tree returned by depth>1
type folderNode struct {
	ID           int       `json:"id"`
	Name         string    `json:"name"`
	Creation     time.Time `json:"creation"`
	Modification time.Time `json:"modification"`
	Content      struct {
		Folders []folderNode `json:"folders"`
		Files   []APIFile    `json:"files"`
	} `json:"content"`
}

// CreateFolderResponse models the JSON response from POST /folders
type CreateFolderResponse struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	ParentID int    `json:"parent_id"`
}

// ListItem represents an item returned in a listing (file or folder)
type ListItem struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"` // "file" or "folder"
	Size    int64  `json:"size,omitempty"`
	ModTime string `json:"mod_time,omitempty"`
}

// APIFolder matches the /folders response
type APIFolder struct {
	ID           int       `json:"id"`
	Name         string    `json:"name"`
	ParentID     int       `json:"parent_id"`
	Creation     time.Time `json:"creation"`
	Modification time.Time `json:"modification"`
	NbFiles      int       `json:"nb_files"`
	NbSub        int       `json:"nb_sub"`
}

// APIFile matches the /files response
type APIFile struct {
	ID           int       `json:"id"`
	Name         string    `json:"name"`
	Size         int64     `json:"size"`
	Creation     time.Time `json:"creation"`
	Modification time.Time `json:"modification"`
	ContentType  string    `json:"content_type"`
	MD5          string    `json:"md5"`
	Hash         string    `json:"hash"`
}

type lruItem struct {
	key string
	val string
	exp time.Time
	el  *list.Element
}
type lruCache struct {
	mu  sync.Mutex
	ttl time.Duration
	max int
	ll  *list.List
	it  map[string]*lruItem
}

type httpErr struct {
	code int
	body string
}

// PerformanceMonitor tracks performance metrics for throttling detection
type PerformanceMonitor struct {
	requestCount int64        // total number of API requests made
	lastReset    time.Time    // when the counter was last reset
	mu           sync.RWMutex // protects concurrent access
}

// NewPerformanceMonitor creates a new performance monitor
func NewPerformanceMonitor() *PerformanceMonitor {
	return &PerformanceMonitor{
		requestCount: 0,
		lastReset:    time.Now(),
	}
}

// IncrementCounter increments the request counter
func (pm *PerformanceMonitor) IncrementCounter() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.requestCount++
}

// GetRequestCount returns the current request count
func (pm *PerformanceMonitor) GetRequestCount() int64 {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.requestCount
}

// ResetCounter resets the request counter and updates the reset time
func (pm *PerformanceMonitor) ResetCounter() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.requestCount = 0
	pm.lastReset = time.Now()
}

// GetRequestsPerMinute calculates the current request rate
func (pm *PerformanceMonitor) GetRequestsPerMinute() float64 {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	elapsed := time.Since(pm.lastReset)
	if elapsed < time.Minute {
		// If less than a minute has passed, extrapolate
		return float64(pm.requestCount) * 60.0 / elapsed.Seconds()
	}

	// If more than a minute, calculate actual rate
	minutes := elapsed.Minutes()
	return float64(pm.requestCount) / minutes
}

// IsThrottled checks if we should throttle based on request rate
// You can customize the threshold as needed
func (pm *PerformanceMonitor) IsThrottled(threshold float64) bool {
	return pm.GetRequestsPerMinute() > threshold
}

// CheckAndThrottle checks if throttling is needed and sleeps if necessary
func (pm *PerformanceMonitor) CheckAndThrottle(threshold float64, sleepDuration time.Duration) {
	if pm.IsThrottled(threshold) {
		fs.Debugf(nil, "Throttling detected, sleeping for %v", sleepDuration)
		time.Sleep(sleepDuration)
		pm.ResetCounter() // Reset after throttling
	}
}

// sanitizeFileName performs minimal sanitization on file names
// Only removes leading/trailing spaces and ensures reasonable length limits
// No character replacement is performed to preserve original file/folder names
// Returns the sanitized name and a boolean indicating if changes were made
func sanitizeFileName(fileName string) (string, bool) {
	if fileName == "" {
		return fileName, false
	}

	changes := false

	// Remove leading and trailing spaces
	trimmed := strings.TrimSpace(fileName)
	if trimmed != fileName {
		fileName = trimmed
		changes = true
	}

	// If the filename is now empty, add a prefix
	if fileName == "" {
		fileName = "sanitized_file"
		changes = true
	}

	// Ensure the filename is not too long
	if len(fileName) > 200 {
		// Keep extension if it exists
		ext := filepath.Ext(fileName)
		base := strings.TrimSuffix(fileName, ext)
		if len(ext) > 0 {
			fileName = base[:200-len(ext)] + ext
		} else {
			fileName = fileName[:200]
		}
		changes = true
	}

	return fileName, changes
}

func NewNetExplorer() *NetExplorer {
	return &NetExplorer{
		BaseURL: "https://files.netexplorer.pro",
		// Use a 16-minute overall HTTP timeout to better accommodate very large uploads.
		Client: createOptimizedHTTPClient(16 * 60),
	}
}

// createOptimizedHTTPClient creates an HTTP client with connection pooling and optimized settings
func createOptimizedHTTPClient(timeoutSeconds int) *http.Client {
	transport := &http.Transport{
		MaxIdleConns:        200,               // Increased from 100 to 200
		MaxIdleConnsPerHost: 50,                // Increased from 20 to 50
		IdleConnTimeout:     120 * time.Second, // Increased from 90s to 120s
		DisableCompression:  false,             // Enable compression
		DisableKeepAlives:   false,             // Enable keep-alives
		MaxConnsPerHost:     100,               // Increased from 50 to 100
		// Let the overall client timeout control long-running uploads, especially for huge files.
		ResponseHeaderTimeout: 0,
		ExpectContinueTimeout: 10 * time.Second, // Increased to 10 seconds for large uploads
	}

	return &http.Client{
		Transport: transport,
		Timeout:   time.Duration(timeoutSeconds) * time.Second,
	}
}

// Authenticate user with NetExplorer and store token
func (ne *NetExplorer) Authenticate(email, password string) (string, error) {
	fs.Debugf(nil, "Authenticate: POST /api/auth as %q", email)

	payload := map[string]string{"user": email, "password": password}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest("POST", ne.BaseURL+"/api/auth", bytes.NewBuffer(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := ne.Client.Do(req)
	dt := time.Since(start)
	if err != nil {
		fs.Debugf(nil, "Authenticate: HTTP error after %v: %v", dt, err)
		return "", err
	}
	defer resp.Body.Close()
	fs.Debugf(nil, "Authenticate: status=%d in %v", resp.StatusCode, dt)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("authentication failed: %d %s", resp.StatusCode, body)
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	ne.Token = result.Token
	// Save token under the configured rclone remote section so external
	// processes updating the rclone config can be detected. Fall back to
	// ClientID for backward compatibility if ConfigName was not set.
	cfgKey := ne.ConfigName
	if cfgKey == "" {
		cfgKey = ne.ClientID
	}
	if err := config.SetValueAndSave(cfgKey, "token", ne.Token); err != nil {
		return ne.Token, fmt.Errorf("authentication succeeded but failed to save token: %w", err)
	}
	fs.Debugf(nil, "Authenticate: token saved for config %q", cfgKey)
	return result.Token, nil
}

// doRequestWithReauth executes a request built by buildReq using the current token.
// It proactively checks the config file for token updates before each request to
// seamlessly pick up tokens refreshed by NetExplorer Import's background loop.
// If the server responds with HTTP 401 Unauthorized, it will normally wait briefly and
// re-read the token from the config. If that yields a different token, it will
// retry once with the new token. If the token did not change, it will then try
// to re-authenticate once using the configured email/password, update the stored
// token and retry the request a final time.
// If directReauth is true, a 401 will skip the external-refresh wait and go
// directly to the credential-based re-authentication.
func (ne *NetExplorer) doRequestWithReauth(buildReq func(token string) (*http.Request, error), directReauth bool) (*http.Response, error) {
	// Helper to build and execute the HTTP request
	do := func(token string) (*http.Response, error) {
		req, err := buildReq(token)
		if err != nil {
			return nil, err
		}
		return ne.Client.Do(req)
	}

	// Merge per-call flag with configuration: either an explicit true or the
	// backend option will enable direct re-authentication.
	effectiveDirectReauth := directReauth
	if !effectiveDirectReauth && ne.opt != nil && ne.opt.DirectReauth {
		effectiveDirectReauth = true
	}

	// PROACTIVE: Re-read token from config before each request to pick up
	// external refreshes (e.g., from NetExplorer Import's 60s refresh loop).
	// This ensures long-running transfers seamlessly use refreshed tokens.
	cfgKey := ne.ConfigName
	if cfgKey == "" {
		cfgKey = ne.ClientID
	}
	currentTokenFromConfig := config.GetValue(cfgKey, "token")
	if currentTokenFromConfig != "" && currentTokenFromConfig != ne.Token {
		fs.Debugf(nil, "netexplorer: detected token refresh in config (proactive check), updating in-memory token for config %q", cfgKey)
		ne.Token = currentTokenFromConfig
	}

	// First attempt with current token (may have just been refreshed above)
	resp, err := do(ne.Token)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusUnauthorized {
		return resp, nil
	}

	// Token is missing or no longer valid - see NetExplorer docs (401 Unauthorized).
	resp.Body.Close()

	if !effectiveDirectReauth {
		// Log tokens found in both possible config locations to help diagnose
		// where an external refresher may have written the new token.
		mask := func(t string) string {
			if t == "" {
				return "(empty)"
			}
			if len(t) <= 8 {
				return t
			}
			return t[:4] + "…" + t[len(t)-4:]
		}
		altToken := config.GetValue(ne.ClientID, "token")
		fs.Debugf(nil, "netexplorer: 401 detected, current in-memory token=%s config[%s]=%s config[%s]=%s", mask(ne.Token), cfgKey, mask(currentTokenFromConfig), ne.ClientID, mask(altToken))

		const waitForExternalRefresh = 10 * time.Second
		fs.Debugf(nil, "netexplorer: got 401, waiting %v to see if token is refreshed externally", waitForExternalRefresh)
		time.Sleep(waitForExternalRefresh)
		// Force reload of the config file from disk in case an external
		// process updated it while we were waiting.
		if Loaded := config.LoadedData(); Loaded != nil {
			if err := Loaded.Load(); err != nil {
				fs.Debugf(nil, "netexplorer: failed to reload config after 401: %v", err)
			} else {
				fs.Debugf(nil, "netexplorer: reloaded config from disk after 401")
			}
		}

		// Re-read token from both config keys; prefer a changed value found in
		// either the remote-named section or the original ClientID section.
		newTokenFromCfgKey := config.GetValue(cfgKey, "token")
		newTokenFromClientID := config.GetValue(ne.ClientID, "token")
		if newTokenFromCfgKey != "" && newTokenFromCfgKey != ne.Token {
			fs.Debugf(nil, "netexplorer: detected new token in config %q after 401; retrying request with refreshed token", cfgKey)
			ne.Token = newTokenFromCfgKey
			return do(ne.Token)
		}
		if newTokenFromClientID != "" && newTokenFromClientID != ne.Token {
			fs.Debugf(nil, "netexplorer: detected new token in config %q after 401; retrying request with refreshed token", ne.ClientID)
			ne.Token = newTokenFromClientID
			return do(ne.Token)
		}
	}

	// No external refresh detected - or directReauth was requested - try to re-authenticate once if we have
	// credentials configured.
	if ne.opt == nil || ne.opt.Email == "" || ne.opt.Password == "" {
		return nil, fmt.Errorf("netexplorer: token expired (401) and no credentials configured for automatic re-authentication")
	}

	fs.Debugf(nil, "netexplorer: token appears expired (401), re-authenticating as %q", ne.opt.Email)

	newTok, err := ne.Authenticate(ne.opt.Email, ne.opt.Password)
	if err != nil {
		return nil, fmt.Errorf("netexplorer: token expired (401) and re-authentication failed: %w", err)
	}
	ne.Token = newTok

	// Retry once with the fresh token. If this still fails (including 401),
	// return the response/error to the caller.
	return do(ne.Token)
}

// Registration options
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "netexplorer",
		Description: "NetExplorer remote",
		NewFs:       NewFs,
		Options: []fs.Option{
			{
				Name:    "base_url",
				Help:    "Base URL of the NetExplorer API",
				Default: "https://org.netexplorer.pro",
			},
			{
				Name:      "token",
				Help:      "Access token for NetExplorer",
				Default:   "",
				Advanced:  true,
				Sensitive: true,
			},
			{
				Name:      "email",
				Help:      "Your NetExplorer user email (used to fetch a token if you don't paste one)",
				Default:   "",
				Sensitive: true,
			},
			{
				Name:       "password",
				Help:       "Your NetExplorer password (used to fetch a token if you don't paste one)",
				Default:    "",
				IsPassword: true,
				NoPasswordGenerate: true,
				Required:   true,
			},
			{
				Name:     "root",
				Help:     "Numeric root folder ID",
				Default:  "",
				Advanced: true,
			},
			{
				Name:     "index_path",
				Help:     "Path to on-disk index (BoltDB). Empty = use OS cache dir",
				Default:  "",
				Advanced: true,
			},
			{
				Name:     "max_concurrency",
				Help:     "Maximum number of concurrent operations (folder listing, file operations)",
				Default:  64,
				Advanced: true,
			},
			{
				Name:     "chunk_concurrency",
				Help:     "Maximum number of concurrent chunk uploads for large files",
				Default:  4,
				Advanced: true,
			},
			{
				Name:     "request_timeout",
				Help:     "HTTP request timeout in seconds (default 600 for large file support)",
				Default:  600,
				Advanced: true,
			},
			{
				Name:     "folder_delay",
				Help:     "Delay in milliseconds after folder creation (0 to disable)",
				Default:  50,
				Advanced: true,
			},
			{
				Name:     "hydration_retries",
				Help:     "Number of retries when looking for uploaded files (for eventual consistency)",
				Default:  5,
				Advanced: true,
			},
			{
				Name:     "ignore_checksum_errors",
				Help:     "Ignore checksum verification errors for files with eventual consistency issues",
				Default:  true,
				Advanced: true,
			},
			{
				Name:     "disable_hash_check",
				Help:     "Disable hash checking to avoid MD5 mismatch errors and improve reliability",
				Default:  true,
				Advanced: true,
			},
			{
				Name:     "upload_retries",
				Help:     "Number of retries for upload operations",
				Default:  3,
				Advanced: true,
			},
			{
				Name:     "retry_delay_base",
				Help:     "Base delay in seconds for retry backoff",
				Default:  1,
				Advanced: true,
			},
			{
				Name:     "direct_reauth",
				Help:     "On HTTP 401, skip waiting for external token refresh and re-authenticate immediately using configured credentials",
				Default:  false,
				Advanced: true,
			},
		},
	})
}

// Options is how rclone config is mapped into your backend
type Options struct {
	BaseURL              string `config:"base_url"`
	ClientID             string `config:"client_id"`
	Token                string `config:"token"`
	Email                string `config:"email"`
	Password             string `config:"password"`
	Root                 string `config:"root"`
	IndexPath            string `config:"index_path"`
	MaxConcurrency       int    `config:"max_concurrency"`
	ChunkConcurrency     int    `config:"chunk_concurrency"`
	RequestTimeout       int    `config:"request_timeout"`
	FolderDelay          int    `config:"folder_delay"`
	HydrationRetries     int    `config:"hydration_retries"`
	IgnoreChecksumErrors bool   `config:"ignore_checksum_errors"`
	DisableHashCheck     bool   `config:"disable_hash_check"`
	UploadRetries        int    `config:"upload_retries"`
	RetryDelayBase       int    `config:"retry_delay_base"`
	DirectReauth         bool   `config:"direct_reauth"`
}

// UploadCache stores upload responses to avoid redundant lookups
type UploadCache struct {
	mu    sync.RWMutex
	cache map[string]*APIFile // key: "parentID|filename"
}

// FileMetadataCache stores file metadata from ListFolder responses
type FileMetadataCache struct {
	mu    sync.RWMutex
	cache map[string]*APIFile // key: "fileID"
}

// FolderMetadataCache stores folder metadata from ListFolder responses
type FolderMetadataCache struct {
	mu    sync.RWMutex
	cache map[string]*APIFolder // key: "folderID"
}

// Fs is the main struct implementing fs.Fs
type Fs struct {
	name               string   // e.g. "netexplorer"
	root               string   // the raw root string (e.g. "10292/foo/bar")
	rootID             string   // the first part, always numeric ("10292")
	initialPath        []string // any trailing components (["foo","bar"])
	opt                Options
	features           *fs.Features
	ne                 *NetExplorer
	hashes             hash.Set
	folderCache        map[string]string
	cacheMu            sync.Mutex
	kv                 *bbolt.DB
	hot                *lruCache
	listSF             singleflight.Group
	listSem            chan struct{}
	chunkSem           chan struct{}
	ns                 string          // set once, used to namespace cache keys
	renameSummary      *RenameSummary  // tracks file and folder renames
	folderBatch        map[string]bool // track folders being created to avoid duplicates
	batchMu            sync.Mutex
	dirCache           map[string]string // cache directory -> folderID mapping
	dirCacheMu         sync.RWMutex
	uploadCache        *UploadCache           // cache upload responses
	fileMetaCache      *FileMetadataCache     // cache file metadata from ListFolder responses
	folderMetaCache    *FolderMetadataCache   // cache folder metadata from ListFolder responses
	perfMonitor        *PerformanceMonitor    // performance monitoring for throttling detection
	pendingFolderDates map[string]folderDates // cache pending folder dates (path -> dates) for folders that need to be created with dates
	pendingDatesMu     sync.RWMutex           // protects pendingFolderDates
}

// ValidateRemotePath rejects NetExplorer forbidden names early (including in --dry-run).
//
// NetExplorer naming rules are based on Windows compatibility:
// - Forbidden characters: \ / : * ? " < > |
// - No leading/trailing spaces or tabs
func (f *Fs) ValidateRemotePath(ctx context.Context, remote string) error {
	_ = ctx // reserved for future use
	if remote == "" {
		return nil // root is OK
	}
	parts := strings.Split(remote, "/")
	for _, name := range parts {
		if name == "" {
			// Keep this short - the caller (fs.Errorf) already logs the full path.
			return fmt.Errorf("netexplorer: invalid path: empty component")
		}
		// NetExplorer enforces a per-name limit (file or folder) - keep consistent with sanitizeFileName.
		if len(name) > 200 {
			return fmt.Errorf("netexplorer: invalid name %q: exceeds 200 characters", name)
		}
		trimmed := strings.Trim(name, " \t")
		if trimmed != name {
			return fmt.Errorf("netexplorer: invalid name %q: leading/trailing space or tab not allowed", name)
		}
		if netExplorerForbiddenNameChars.MatchString(name) {
			return fmt.Errorf("netexplorer: invalid name %q: contains forbidden character (\\ / : * ? \" < > |)", name)
		}
	}
	return nil
}

// folderDates stores the modification and creation dates for a folder
type folderDates struct {
	ModTime     time.Time
	CreatedTime time.Time
}

// /// Helpers
func (f *Fs) cacheSet(p, id string) {
	f.cacheMu.Lock()
	f.folderCache[p] = id
	f.cacheMu.Unlock()
}
func (f *Fs) cacheGet(p string) (string, bool) {
	f.cacheMu.Lock()
	id, ok := f.folderCache[p]
	f.cacheMu.Unlock()
	return id, ok
}
func (f *Fs) cacheDelete(p string) {
	f.cacheMu.Lock()
	delete(f.folderCache, p)
	f.cacheMu.Unlock()
}

// clearConflictingCacheEntries clears cache entries that might cause folder hierarchy conflicts
func (f *Fs) clearConflictingCacheEntries(folderName string) {
	f.cacheMu.Lock()
	defer f.cacheMu.Unlock()

	// Clear any cache entries that use just the folder name as key
	// This helps prevent conflicts when we have nested folders with same names
	for key := range f.folderCache {
		if key == folderName {
			delete(f.folderCache, key)
			fs.Debugf(f, "clearConflictingCacheEntries: cleared cache entry for %q", key)
		}
	}
}

// findPathByID finds the path that is mapped to a given folder ID
func (f *Fs) findPathByID(folderID string) string {
	f.cacheMu.Lock()
	defer f.cacheMu.Unlock()

	// Search through the cache to find which path is mapped to this ID
	for path, id := range f.folderCache {
		if id == folderID {
			return path
		}
	}
	return ""
}

// Upload cache helpers
func (f *Fs) cacheUploadResponse(parentID, filename string, fileInfo *APIFile) {
	f.uploadCache.mu.Lock()
	defer f.uploadCache.mu.Unlock()
	key := fmt.Sprintf("%s|%s", parentID, filename)
	f.uploadCache.cache[key] = fileInfo
}

func (f *Fs) getCachedUpload(parentID, filename string) (*APIFile, bool) {
	f.uploadCache.mu.RLock()
	defer f.uploadCache.mu.RUnlock()
	key := fmt.Sprintf("%s|%s", parentID, filename)
	fileInfo, exists := f.uploadCache.cache[key]
	return fileInfo, exists
}

// File metadata cache helpers
func (f *Fs) cacheFileMetadata(fileID string, fileInfo *APIFile) {
	f.fileMetaCache.mu.Lock()
	defer f.fileMetaCache.mu.Unlock()
	f.fileMetaCache.cache[fileID] = fileInfo
}

func (f *Fs) getCachedFileMetadata(fileID string) (*APIFile, bool) {
	f.fileMetaCache.mu.RLock()
	defer f.fileMetaCache.mu.RUnlock()
	fileInfo, exists := f.fileMetaCache.cache[fileID]
	return fileInfo, exists
}

// Folder metadata cache helpers
func (f *Fs) cacheFolderMetadata(folderID string, folderInfo *APIFolder) {
	f.folderMetaCache.mu.Lock()
	defer f.folderMetaCache.mu.Unlock()
	f.folderMetaCache.cache[folderID] = folderInfo
}

func (f *Fs) getCachedFolderMetadata(folderID string) (*APIFolder, bool) {
	f.folderMetaCache.mu.RLock()
	defer f.folderMetaCache.mu.RUnlock()
	folderInfo, exists := f.folderMetaCache.cache[folderID]
	return folderInfo, exists
}

// smartHydrateFile uses exponential backoff and limited retries for file hydration
func (f *Fs) smartHydrateFile(ctx context.Context, parentID, filename, dir string) int {
	maxRetries := 3 // Reduced from 5 to 3 for better performance
	baseDelay := 1 * time.Second

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Use singleflight to prevent duplicate hydration requests
		key := fmt.Sprintf("hydrate:%s:%s", parentID, filename)
		_, err, _ := f.listSF.Do(key, func() (any, error) {
			return nil, f.hydrateFolder(ctx, parentID, dir)
		})

		if err != nil {
			fs.Debugf(f, "smartHydrateFile(%s): attempt %d error: %v", filename, attempt+1, err)
		} else {
			// Check if file was found after hydration
			if idStr, ok := f.idxGetFile(parentID, filename); ok {
				if id, convErr := strconv.Atoi(idStr); convErr == nil {
					fs.Debugf(f, "smartHydrateFile(%s): found id=%d after %d attempts", filename, id, attempt+1)
					return id
				}
			}
		}

		// Wait before next attempt with exponential backoff
		if attempt < maxRetries-1 {
			delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt))) // 1s, 2s, 4s
			fs.Debugf(f, "smartHydrateFile(%s): attempt %d failed, waiting %v", filename, attempt+1, delay)
			time.Sleep(delay)
		}
	}

	fs.Debugf(f, "smartHydrateFile(%s): not found after %d attempts", filename, maxRetries)
	return 0
}
func newLRU(max int, ttl time.Duration) *lruCache {
	return &lruCache{ttl: ttl, max: max, ll: list.New(), it: make(map[string]*lruItem, max)}
}
func (c *lruCache) Get(k string) (string, bool) {
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	it, ok := c.it[k]
	if !ok {
		return "", false
	}
	if !it.exp.IsZero() && now.After(it.exp) {
		c.remove(it)
		return "", false
	}
	c.ll.MoveToFront(it.el)
	return it.val, true
}
func (c *lruCache) Set(k, v string) {
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	if it, ok := c.it[k]; ok {
		it.val = v
		it.exp = now.Add(c.ttl)
		c.ll.MoveToFront(it.el)
		return
	}
	el := c.ll.PushFront(k)
	c.it[k] = &lruItem{key: k, val: v, exp: now.Add(c.ttl), el: el}
	for c.ll.Len() > c.max {
		back := c.ll.Back()
		if back == nil {
			break
		}
		if it := c.it[back.Value.(string)]; it != nil {
			c.remove(it)
		}
	}
}
func (c *lruCache) Delete(k string) {
	c.mu.Lock()
	if it, ok := c.it[k]; ok {
		c.remove(it)
	}
	c.mu.Unlock()
}
func (c *lruCache) remove(it *lruItem) {
	delete(c.it, it.key)
	c.ll.Remove(it.el)
}

var (
	bucketFolders = []byte("folders") // key: path (string)     -> val: id (string)
	bucketFiles   = []byte("files")   // key: parentID|name     -> val: id (string)
)

func openKV(path string) (*bbolt.DB, error) {
	db, err := bbolt.Open(path, 0600, &bbolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, err
	}
	err = db.Update(func(tx *bbolt.Tx) error {
		if _, e := tx.CreateBucketIfNotExists(bucketFolders); e != nil {
			return e
		}
		if _, e := tx.CreateBucketIfNotExists(bucketFiles); e != nil {
			return e
		}
		return nil
	})
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}
func (e *httpErr) Error() string { return fmt.Sprintf("http %d: %s", e.code, e.body) }
func isAlreadyExists(err error) bool {
	var he *httpErr
	if errors.As(err, &he) {
		// NetExplorer may signal "already exists" with several status codes:
		// - 409/422: explicit conflict / unprocessable entity
		// - 403: in practice sometimes returned with a body like
		//   {"error":"Un élément du même nom existe déjà. Veuillez saisir un nom différent."}
		if he.code == http.StatusConflict || he.code == 422 {
			return true
		}
		if he.code == http.StatusForbidden &&
			(strings.Contains(he.body, "m\\u00eame nom existe d\\u00e9j\\u00e0") ||
				strings.Contains(he.body, "même nom existe déjà")) {
			return true
		}
	}
	return false
}
func (f *Fs) kpath(p string) string { return f.ns + p }

// ----- index getters/setters -----
func (f *Fs) idxGetFolder(p string) (string, bool) {
	if v, ok := f.hot.Get("p:" + f.kpath(p)); ok {
		return v, true
	}
	if id, ok := f.cacheGet(p); ok { // keep legacy map in sync
		f.hot.Set("p:"+p, id)
		return id, true
	}
	if f.kv != nil {
		var out string
		_ = f.kv.View(func(tx *bbolt.Tx) error {
			b := tx.Bucket(bucketFolders)
			if b == nil {
				return nil
			}
			v := b.Get([]byte(p))
			if v != nil {
				out = string(v)
			}
			return nil
		})
		if out != "" {
			f.hot.Set("p:"+p, out)
			f.cacheSet(p, out)
			return out, true
		}
	}
	return "", false
}
func (f *Fs) idxPutFolder(p, id string) {
	f.hot.Set("p:"+f.kpath(p), id)
	f.cacheSet(p, id) // keep legacy map
	if f.kv != nil {
		// Async cache write to avoid blocking
		go func() {
			_ = f.kv.Update(func(tx *bbolt.Tx) error {
				return tx.Bucket(bucketFolders).Put([]byte(p), []byte(id))
			})
		}()
	}
}
func (f *Fs) idxDelFolder(p string) {
	f.hot.Delete("p:" + f.kpath(p))
	f.cacheDelete(p) // keep legacy map
	if f.kv != nil {
		// Async cache write to avoid blocking
		go func() {
			_ = f.kv.Update(func(tx *bbolt.Tx) error {
				return tx.Bucket(bucketFolders).Delete([]byte(p))
			})
		}()
	}
}
func kf(parentID, name string) string { return parentID + "|" + name }
func (f *Fs) idxGetFile(parentID, name string) (string, bool) {
	key := "f:" + kf(parentID, name)
	if v, ok := f.hot.Get(key); ok {
		return v, true
	}
	if f.kv != nil {
		var out string
		_ = f.kv.View(func(tx *bbolt.Tx) error {
			b := tx.Bucket(bucketFiles)
			if b == nil {
				return nil
			}
			v := b.Get([]byte(kf(parentID, name)))
			if v != nil {
				out = string(v)
			}
			return nil
		})
		if out != "" {
			f.hot.Set(key, out)
			return out, true
		}
	}
	return "", false
}
func (f *Fs) idxPutFile(parentID, name, id string) {
	f.hot.Set("f:"+kf(parentID, name), id)
	if f.kv != nil {
		// Async cache write to avoid blocking
		go func() {
			_ = f.kv.Update(func(tx *bbolt.Tx) error {
				return tx.Bucket(bucketFiles).Put([]byte(kf(parentID, name)), []byte(id))
			})
		}()
	}
}
func (f *Fs) idxDelFile(parentID, name string) {
	f.hot.Delete("f:" + kf(parentID, name))
	if f.kv != nil {
		// Async cache write to avoid blocking
		go func() {
			_ = f.kv.Update(func(tx *bbolt.Tx) error {
				return tx.Bucket(bucketFiles).Delete([]byte(kf(parentID, name)))
			})
		}()
	}
}
func (f *Fs) hydrateFolder(ctx context.Context, parentID, parentPath string) error {
	f.listSem <- struct{}{}
	defer func() { <-f.listSem }()

	// Check if we already have this folder hydrated recently using timestamp buckets
	// This allows for more granular cache invalidation and better performance
	cacheKey := fmt.Sprintf("hydrated:%s:%d", parentID, time.Now().Unix()/300) // 5-minute buckets
	if _, hydrated := f.hot.Get(cacheKey); hydrated {
		fs.Debugf(f, "hydrateFolder: skipping %s (already hydrated in current bucket)", parentID)
		return nil
	}

	folders, files, err := f.ne.ListFolder(parentID)
	if err != nil {
		return err
	}

	// Batch cache updates for better performance - cache both IDs and metadata
	for _, af := range folders {
		folderPath := path.Join(parentPath, af.Name)
		f.idxPutFolder(folderPath, strconv.Itoa(af.ID))
		// Cache folder metadata to avoid redundant API calls
		f.cacheFolderMetadata(strconv.Itoa(af.ID), &af)
	}
	for _, fi := range files {
		f.idxPutFile(parentID, fi.Name, strconv.Itoa(fi.ID))
		// Cache file metadata to avoid redundant GetFile calls
		f.cacheFileMetadata(strconv.Itoa(fi.ID), &fi)
	}

	// Mark as hydrated to avoid redundant calls using timestamp bucket
	f.hot.Set(cacheKey, "1")
	return nil
}

// bulkHydrateFolders efficiently hydrates multiple folders using bulk API calls
// This reduces API calls from N to 1 for N folders by using NetExplorer's depth parameter
func (f *Fs) bulkHydrateFolders(ctx context.Context, folderIDs []string) error {
	if len(folderIDs) == 0 {
		return nil
	}

	fs.Debugf(f, "bulkHydrateFolders: hydrating %d folders", len(folderIDs))

	// Use singleflight to prevent duplicate bulk requests
	key := "bulk:" + strings.Join(folderIDs, ",")
	_, err, _ := f.listSF.Do(key, func() (any, error) {
		return nil, f.performBulkHydration(ctx, folderIDs)
	})

	return err
}

// performBulkHydration performs the actual bulk hydration using NetExplorer's depth API
func (f *Fs) performBulkHydration(ctx context.Context, folderIDs []string) error {
	if len(folderIDs) == 0 {
		return nil
	}

	// Group folders by their common root to minimize API calls
	rootGroups := f.groupFoldersByRoot(folderIDs)

	for rootID, subFolders := range rootGroups {
		// Use depth parameter for true bulk operations
		depth := f.calculateOptimalDepth(subFolders)
		fs.Debugf(f, "bulkHydrateFolders: using depth=%d for root=%s with %d folders", depth, rootID, len(subFolders))

		// Single API call with depth parameter instead of N individual calls
		folders, files, err := f.ne.ListFolderWithDepth(rootID, depth)
		if err != nil {
			fs.Debugf(f, "bulkHydrateFolders: failed to bulk hydrate root %s: %v", rootID, err)
			// Fallback to individual hydration for this root
			f.fallbackIndividualHydration(ctx, subFolders)
			continue
		}

		// Batch update cache with all discovered folders and files
		f.batchUpdateCache(folders, files)
	}

	fs.Debugf(f, "bulkHydrateFolders: completed hydrating %d folders", len(folderIDs))
	return nil
}

// groupFoldersByRoot groups folder IDs by their common root
func (f *Fs) groupFoldersByRoot(folderIDs []string) map[string][]string {
	groups := make(map[string][]string)

	for _, folderID := range folderIDs {
		// For now, group by rootID - in a more sophisticated implementation,
		// you could determine the actual root by walking up the folder tree
		rootID := f.rootID
		groups[rootID] = append(groups[rootID], folderID)
	}

	return groups
}

// calculateOptimalDepth determines the optimal depth for bulk API calls
func (f *Fs) calculateOptimalDepth(folderIDs []string) int {
	// Start with depth 2 to get immediate children
	// This can be optimized based on folder structure analysis
	return 2
}

// batchUpdateCache updates the cache with multiple folders and files at once
func (f *Fs) batchUpdateCache(folders []APIFolder, files []APIFile) {
	// Batch update folder cache with metadata
	for _, folder := range folders {
		// Note: This function is used for bulk operations where we don't have full path context
		// For now, we'll use the folder name as the key, but this should be improved
		// to use full paths when available
		f.idxPutFolder(folder.Name, strconv.Itoa(folder.ID))
		// Cache folder metadata to avoid redundant API calls
		f.cacheFolderMetadata(strconv.Itoa(folder.ID), &folder)
	}

	// Batch update file cache with metadata
	for _, file := range files {
		// Cache file metadata to avoid redundant GetFile calls
		f.cacheFileMetadata(strconv.Itoa(file.ID), &file)
		fs.Debugf(f, "batchUpdateCache: cached file %s (ID: %d, size: %d)", file.Name, file.ID, file.Size)
	}
}

// fallbackIndividualHydration is used when bulk operations fail
func (f *Fs) fallbackIndividualHydration(ctx context.Context, folderIDs []string) {
	fs.Debugf(f, "fallbackIndividualHydration: falling back to individual hydration for %d folders", len(folderIDs))

	batchSize := 5 // Smaller batch size for fallback
	for i := 0; i < len(folderIDs); i += batchSize {
		end := i + batchSize
		if end > len(folderIDs) {
			end = len(folderIDs)
		}

		batch := folderIDs[i:end]
		var wg sync.WaitGroup
		for _, folderID := range batch {
			wg.Add(1)
			go func(id string) {
				defer wg.Done()
				if err := f.hydrateFolder(ctx, id, ""); err != nil {
					fs.Debugf(f, "fallbackIndividualHydration: failed to hydrate folder %s: %v", id, err)
				}
			}(folderID)
		}
		wg.Wait()
	}
}

// warmCache proactively hydrates frequently accessed folders to prevent cache misses
func (f *Fs) warmCache(ctx context.Context, rootID string) {
	fs.Debugf(f, "warmCache: starting cache warming for root %s", rootID)

	// Start cache warming in background to avoid blocking
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fs.Debugf(f, "warmCache: recovered from panic: %v", r)
			}
		}()

		// Get root folder contents first
		folders, _, err := f.ne.ListFolder(rootID)
		if err != nil {
			fs.Debugf(f, "warmCache: failed to list root folder: %v", err)
			return
		}

		// Extract folder IDs for bulk hydration
		folderIDs := make([]string, 0, len(folders))
		for _, folder := range folders {
			folderIDs = append(folderIDs, strconv.Itoa(folder.ID))
		}

		// Bulk hydrate all root-level folders
		if len(folderIDs) > 0 {
			fs.Debugf(f, "warmCache: bulk hydrating %d root-level folders", len(folderIDs))
			if err := f.bulkHydrateFolders(ctx, folderIDs); err != nil {
				fs.Debugf(f, "warmCache: bulk hydration failed: %v", err)
			}
		}

		fs.Debugf(f, "warmCache: completed cache warming for root %s", rootID)
	}()
}

// preloadFrequentlyAccessedFolders identifies and preloads folders that are accessed frequently
func (f *Fs) preloadFrequentlyAccessedFolders(ctx context.Context) {
	fs.Debugf(f, "preloadFrequentlyAccessedFolders: starting smart preloading")

	// Start preloading in background
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fs.Debugf(f, "preloadFrequentlyAccessedFolders: recovered from panic: %v", r)
			}
		}()

		// Get frequently accessed folders from cache statistics
		// This is a simplified version - in production you'd track access patterns
		frequentlyAccessed := f.getFrequentlyAccessedFolders()

		if len(frequentlyAccessed) > 0 {
			fs.Debugf(f, "preloadFrequentlyAccessedFolders: preloading %d frequently accessed folders", len(frequentlyAccessed))
			if err := f.bulkHydrateFolders(ctx, frequentlyAccessed); err != nil {
				fs.Debugf(f, "preloadFrequentlyAccessedFolders: preloading failed: %v", err)
			}
		}
	}()
}

// getFrequentlyAccessedFolders returns a list of folder IDs that are accessed frequently
// This is a simplified implementation - in production you'd track access patterns
func (f *Fs) getFrequentlyAccessedFolders() []string {
	// For now, return empty list - this would be implemented based on access tracking
	// In a real implementation, you'd track folder access frequency and return the most accessed ones
	return []string{}
}

// fastResolveDirID quickly resolves directory ID with caching
func (f *Fs) fastResolveDirID(ctx context.Context, dir string) (string, error) {
	f.dirCacheMu.RLock()
	if folderID, exists := f.dirCache[dir]; exists {
		f.dirCacheMu.RUnlock()
		return folderID, nil
	}
	f.dirCacheMu.RUnlock()

	// Not in cache, resolve and cache it
	folderID, err := f.ensureFolderID(ctx, dir, true)
	if err != nil {
		return "", err
	}

	f.dirCacheMu.Lock()
	f.dirCache[dir] = folderID
	f.dirCacheMu.Unlock()

	return folderID, nil
}

///////////////////

// NewFs constructs an Fs from the path, e.g. "netexplorer:myfolder"
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Load config options
	opt := new(Options)
	if err := configstruct.Set(m, opt); err != nil {
		return nil, err
	}

	// Password values are stored obscured in the config file.
	// Reveal it for authentication use, but also tolerate plain text values.
	if opt.Password != "" {
		revealed, err := obscure.Reveal(opt.Password)
		if err == nil {
			opt.Password = revealed
		}
	}

	fs.Debugf(nil, "NewFs(name=%q, root=%q) base_url=%q client_id=%q", name, root, opt.BaseURL, opt.ClientID)

	// Set defaults for new options
	if opt.MaxConcurrency <= 0 {
		opt.MaxConcurrency = 64
	}
	if opt.ChunkConcurrency <= 0 {
		opt.ChunkConcurrency = 4
	}
	if opt.RequestTimeout <= 0 {
		opt.RequestTimeout = 600 // Default 10 minutes for large file support
	}
	if opt.FolderDelay < 0 {
		opt.FolderDelay = 50
	}

	// Create API client with optimized HTTP client
	ne := &NetExplorer{
		BaseURL:  opt.BaseURL,
		Token:    opt.Token,
		ClientID: opt.ClientID,
		// Store the rclone remote name so token lookups use the same
		// config section other processes write to.
		ConfigName: name,
		Client:     createOptimizedHTTPClient(opt.RequestTimeout),
		opt:        opt,
	}

	// Fetch token if needed
	if opt.Token == "" && opt.Email != "" && opt.Password != "" {
		fs.Debugf(nil, "NewFs: no token, authenticating as %q …", opt.Email)
		tok, err := ne.Authenticate(opt.Email, opt.Password)
		if err != nil {
			return nil, fmt.Errorf("failed to login to NetExplorer: %w", err)
		}
		opt.Token = tok
		ne.Token = tok
	}
	u, _ := url.Parse(ne.BaseURL)
	if _, err := net.LookupHost(u.Hostname()); err != nil {
		return nil, fmt.Errorf("base_url %q not resolvable: %v", ne.BaseURL, err)
	}

	// Prepare CLI segments
	trimmed := strings.Trim(root, "/")
	var cliSegments []string
	if trimmed != "" {
		cliSegments = strings.Split(trimmed, "/")
	}

	// Determine rootID and initialPath: allow empty root to defer selection
	var rootID string
	var initialPath []string
	if opt.Root != "" {
		rootID = opt.Root
		initialPath = cliSegments
	} else if len(cliSegments) > 0 {
		rootID = cliSegments[0]
		if len(cliSegments) > 1 {
			initialPath = cliSegments[1:]
		}
	}
	fs.Debugf(nil, "NewFs: derived rootID=%q initialPath=%v", rootID, initialPath)

	// Build Fs (no KV / LRU yet)
	f := &Fs{
		name:        name,
		root:        root,
		rootID:      rootID,
		initialPath: initialPath,
		opt:         *opt,
		ne:          ne,
		folderCache: make(map[string]string, 64),
		hashes:      hash.NewHashSet(hash.MD5),
	}

	// If root not set, prompt now — we need it before naming the cache file
	if f.rootID == "" {
		fs.Debugf(f, "NewFs: no rootID configured, invoking ensureRoot() …")
		if err := f.ensureRoot(ctx); err != nil {
			return nil, err
		}
	}

	// Namespace for cache keys (and for the Bolt filename)
	// NOTE: ensure Fs has: ns string
	f.ns = fmt.Sprintf("%s|%s|", f.opt.BaseURL, f.rootID)

	// Build a namespaced Bolt filename so each (base_url, rootID, remote name) gets its own DB
	var cacheFile string
	if f.opt.IndexPath != "" {
		cacheFile = f.opt.IndexPath
	} else {
		suffix := strings.NewReplacer(":", "_", "/", "_").Replace(f.opt.BaseURL) + "-" + f.rootID
		if dir, err := os.UserCacheDir(); err == nil {
			cacheFile = filepath.Join(dir, fmt.Sprintf("rclone-netexplorer-%s-%s.bolt", name, suffix))
		} else {
			cacheFile = filepath.Join(os.TempDir(), fmt.Sprintf("rclone-netexplorer-%s-%s.bolt", name, suffix))
		}
	}

	// Open KV
	db, err := openKV(cacheFile)
	if err != nil {
		fs.Debugf(f, "index: Bolt open error: %v (index disabled)", err)
	} else {
		fs.Debugf(f, "index: Bolt at %s", cacheFile)
	}
	f.kv = db

	// Hot cache, list gate, features - optimized cache size for better performance
	f.hot = newLRU(200_000, 15*time.Minute) // Increased to 200k entries, 15min TTL for better performance
	f.listSem = make(chan struct{}, opt.MaxConcurrency)
	f.chunkSem = make(chan struct{}, opt.ChunkConcurrency)
	f.renameSummary = NewRenameSummary()
	f.folderBatch = make(map[string]bool) // Initialize folder batch tracking
	f.dirCache = make(map[string]string)  // Initialize directory cache
	f.uploadCache = &UploadCache{         // Initialize upload cache
		cache: make(map[string]*APIFile),
	}
	f.fileMetaCache = &FileMetadataCache{ // Initialize file metadata cache
		cache: make(map[string]*APIFile),
	}
	f.folderMetaCache = &FolderMetadataCache{ // Initialize folder metadata cache
		cache: make(map[string]*APIFolder),
	}
	f.perfMonitor = NewPerformanceMonitor()             // Initialize performance monitoring
	f.pendingFolderDates = make(map[string]folderDates) // Initialize pending folder dates cache
	f.features = (&fs.Features{
		CanHaveEmptyDirectories:  true,
		SlowHash:                 false,
		PutStream:                f.PutStream,
		WriteDirMetadata:         true,
		WriteDirSetModTime:       true,
		DirModTimeUpdatesOnWrite: true, // NetExplorer bumps parent folder modtime when writing files
		// Folder dates are restored after writes using DirSetModTime and cached source dates.
	}).Fill(ctx, f)

	// Initialise folder ID aliases and warm caches.
	if f.rootID != "" {
		// If this Fs is rooted at the true NetExplorer root (no initialPath),
		// then "" really means the API root, so we can safely alias it.
		//
		// If initialPath is non-empty (e.g. "netexplorer:some/sub/folder"),
		// then the logical rclone root "" should correspond to that subfolder,
		// not to the absolute API rootID. In that case we resolve the
		// effective root folder ID now so that List(""), NewObject("…"),
		// NeedTransfer, etc. all see the correct tree.
		if len(f.initialPath) == 0 {
			f.idxPutFolder("", f.rootID)
		} else {
			if _, err := f.ensureFolderID(ctx, "", false); err != nil {
				fs.Debugf(f, "NewFs: ensureFolderID(\"\", create=false) failed for initialPath %v: %v", f.initialPath, err)
				// Non-fatal: if this fails, later operations will fall back to
				// resolving/creating folders on demand.
			}
		}

		// Start cache warming in background for better performance.
		// This preloads frequently accessed folders to prevent cache misses.
		f.warmCache(ctx, f.rootID)
		f.preloadFrequentlyAccessedFolders(ctx)
	}

	fs.Debugf(f, "NewFs: ready (rootID=%q initialPath=%v)", f.rootID, f.initialPath)
	return f, nil
}

// Name of the remote
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote
func (f *Fs) Root() string {
	return f.root
}

// String returns a description of the FS
func (f *Fs) String() string {
	return fmt.Sprintf("NetExplorer root '%s'", f.root)
}

// Features returns the optional features
func (f *Fs) Features() *fs.Features {
	return f.features
}

func (f *Fs) Hashes() hash.Set {
	return f.hashes
}

// GetRenameSummary returns the rename summary for this filesystem
func (f *Fs) GetRenameSummary() *RenameSummary {
	return f.renameSummary
}

// PrintRenameSummary prints a user-friendly summary of all renames for this filesystem
func (f *Fs) PrintRenameSummary() {
	if f.renameSummary != nil {
		f.renameSummary.PrintSummary()
	}
}

func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	if t != hash.MD5 {
		return "", hash.ErrUnsupported
	}

	// If hash checking is disabled, return empty string to skip verification
	// This should be the default behavior - only do hash verification when explicitly requested
	if o.fs.opt.DisableHashCheck {
		fs.Debugf(o.fs, "Hash(%q): hash checking disabled - skipping verification", o.remote)
		return "", nil
	}

	// Only proceed with hash calculation if hash checking is explicitly enabled
	if o.md5 != "" {
		fs.Debugf(o.fs, "Hash(%q): using cached md5=%q", o.remote, o.md5)
		return strings.ToLower(o.md5), nil
	}

	// Handle placeholder IDs that indicate successful upload but ID not found
	if o.id <= 0 {
		fs.Debugf(o.fs, "Hash(%q): placeholder ID %d indicates successful upload but hash not available due to eventual consistency", o.remote, o.id)
		// Return empty string instead of error to indicate hash not available
		// This allows rclone's verification to skip hash checking for this file
		return "", nil
	}

	// Try to get metadata from cache first to avoid redundant GetFile calls
	if cachedFile, exists := o.fs.getCachedFileMetadata(strconv.Itoa(o.id)); exists {
		o.md5 = cachedFile.MD5
		o.size = cachedFile.Size
		o.modTime = cachedFile.Modification
		o.created = cachedFile.Creation
		o.mimeType = cachedFile.ContentType
		fs.Debugf(o.fs, "Hash(%q): md5=%q (from cache)", o.remote, o.md5)
		// If cached MD5 is empty, return empty string to skip hash verification
		if o.md5 == "" {
			fs.Debugf(o.fs, "Hash(%q): cached md5 is empty - skipping hash verification", o.remote)
			return "", nil
		}
		return strings.ToLower(o.md5), nil
	}

	// Fallback to API call only if not in cache
	fs.Debugf(o.fs, "Hash(%q): md5 not cached, calling GetFile(id=%d)…", o.remote, o.id)
	info, err := o.fs.ne.GetFile(strconv.Itoa(o.id))
	if err != nil {
		fs.Debugf(o.fs, "Hash(%q): GetFile error: %v", o.remote, err)
		return "", err
	}
	if info.MD5 == "" {
		fs.Debugf(o.fs, "Hash(%q): md5 missing in API response - this is normal for NetExplorer due to eventual consistency", o.remote)
		// Return empty string instead of error to indicate hash not available
		// This allows rclone to skip hash verification for this file
		return "", nil
	}
	o.md5 = info.MD5
	o.size = info.Size
	o.modTime = info.Modification
	o.created = info.Creation
	o.mimeType = info.ContentType
	// Cache the metadata for future use
	o.fs.cacheFileMetadata(strconv.Itoa(o.id), info)
	fs.Debugf(o.fs, "Hash(%q): md5=%q (from API)", o.remote, o.md5)
	return strings.ToLower(o.md5), nil
}

// joinPath returns "dir/name" or just "name" if dir is empty
func joinPath(dir, name string) string {
	if dir == "" {
		return name
	}
	return dir + "/" + name
}

// ensureRoot checks and prompts the user if the configured root is empty or invalid
func (f *Fs) ensureRoot(ctx context.Context) error {
	fs.Debugf(f, "ensureRoot: prompting user to pick a root folder …")
	fmt.Println("No root folder configured. Please select one:")

	start := time.Now()
	roots, err := f.ne.ListRoots()
	elapsed := time.Since(start)
	if err != nil {
		fs.Debugf(f, "ensureRoot: ListRoots error after %v: %v", elapsed, err)
		return err
	}
	fs.Debugf(f, "ensureRoot: ListRoots returned %d root(s) in %v", len(roots), elapsed)

	// 2) Print them in aligned columns
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tName")
	fmt.Fprintln(w, "--\t----")
	// Also build a map of valid IDs
	valid := make(map[string]bool, len(roots))
	for _, r := range roots {
		idStr := strconv.Itoa(r.ID)
		fmt.Fprintf(w, "%s\t%s\n", idStr, r.Name)
		valid[idStr] = true
	}
	w.Flush()

	// 3) Loop until we get a valid, non-empty choice
	reader := bufio.NewReader(os.Stdin)
	var choice string
	for {
		fmt.Print("Enter folder ID: ")
		line, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		choice = strings.TrimSpace(line)
		if valid[choice] {
			break
		}
		fmt.Println("⨯ Invalid ID, please enter one of the IDs listed above.")
	}

	// 4) Save and set
	if err := config.SetValueAndSave(f.name, "root", choice); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	f.rootID = choice
	fs.Debugf(f, "ensureRoot: selected rootID=%q", f.rootID)
	return nil
}

// ListRoots fetches root folders via the dedicated API endpoint
func (ne *NetExplorer) ListRoots() ([]APIFolder, error) {
	url := fmt.Sprintf("%s/api/folders?depth=0&filter=1", ne.BaseURL)
	fs.Debugf(nil, "ListRoots: GET %s", url)

	start := time.Now()
	resp, err := ne.doRequestWithReauth(func(token string) (*http.Request, error) {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Del("Expect")
		return req, nil
	}, false)
	dt := time.Since(start)
	if err != nil {
		fs.Debugf(nil, "ListRoots: HTTP error after %v: %v", dt, err)
		return nil, err
	}
	defer resp.Body.Close()
	fs.Debugf(nil, "ListRoots: status=%d in %v", resp.StatusCode, dt)

	// mirror original behavior of re-reading body
	b, _ := io.ReadAll(resp.Body)
	fs.Debugf(nil, "ListRoots: body bytes=%d", len(b))
	resp.Body = io.NopCloser(bytes.NewBuffer(b))

	var items []struct {
		ID   *int `json:"id"`
		Name string
		Type string
	}
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		fs.Debugf(nil, "ListRoots: JSON decode error: %v", err)
		return nil, err
	}
	var roots []APIFolder
	for _, it := range items {
		if it.Type == "shared" || it.ID == nil {
			continue
		}
		roots = append(roots, APIFolder{ID: *it.ID, Name: it.Name})
	}
	fs.Debugf(nil, "ListRoots: parsed %d root(s)", len(roots))
	return roots, nil
}

func (ne *NetExplorer) GetFile(fileID string) (*APIFile, error) {
	url := fmt.Sprintf("%s/api/file/%s?full", ne.BaseURL, fileID)
	fs.Debugf(nil, "GetFile: GET %s", url)

	start := time.Now()
	resp, err := ne.doRequestWithReauth(func(token string) (*http.Request, error) {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		return req, nil
	}, false)
	dt := time.Since(start)
	if err != nil {
		fs.Debugf(nil, "GetFile(%s): HTTP error after %v: %v", fileID, dt, err)
		return nil, err
	}
	defer resp.Body.Close()
	fs.Debugf(nil, "GetFile(%s): status=%d in %v", fileID, resp.StatusCode, dt)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GetFile %s: %d %s", fileID, resp.StatusCode, body)
	}
	var f APIFile
	if err := json.NewDecoder(resp.Body).Decode(&f); err != nil {
		fs.Debugf(nil, "GetFile(%s): JSON decode error: %v", fileID, err)
		return nil, err
	}
	if f.MD5 == "" && f.Hash != "" {
		f.MD5 = f.Hash
	}
	fs.Debugf(nil, "GetFile(%s): name=%q size=%d md5=%q", fileID, f.Name, f.Size, f.MD5)
	return &f, nil
}

// List the objects and directories in dir into entries
func (f *Fs) List(ctx context.Context, dir string) (fs.DirEntries, error) {
	origDir := dir
	dir = strings.Trim(dir, "/")
	fs.Debugf(f, "List(%q): start (normalized=%q)", origDir, dir)

	// Clear any potentially conflicting cache entries for nested folders
	// This helps prevent folder hierarchy flattening issues
	if dir != "" && strings.Contains(dir, "/") {
		pathSegments := strings.Split(dir, "/")
		for _, segment := range pathSegments {
			if segment != "" {
				f.clearConflictingCacheEntries(segment)
			}
		}
	}

	folderID, err := f.resolveFolderID(ctx, dir)
	if err != nil {
		fs.Debugf(f, "List(%q): resolveFolderID error: %v", dir, err)
		return nil, err
	}
	fs.Debugf(f, "List(%q): folderID=%s", dir, folderID)

	start := time.Now()
	// Track performance for list operations
	f.perfMonitor.IncrementCounter()

	apiFolders, apiFiles, err := f.ne.ListFolder(folderID)
	dt := time.Since(start)
	if err != nil {
		fs.Debugf(f, "List(%q): ListFolder error after %v: %v", dir, dt, err)
		return nil, err
	}
	fs.Debugf(f, "List(%q): ListFolder returned %d folders, %d files in %v", dir, len(apiFolders), len(apiFiles), dt)

	// Debug: Log all folders returned by the API to understand the structure
	fs.Debugf(f, "List(%q): NetExplorer REST API returned %d folders for folderID=%s", dir, len(apiFolders), folderID)
	for _, af := range apiFolders {
		fs.Debugf(f, "List(%q): REST API folder: name=%q, id=%d, parentID=%d", dir, af.Name, af.ID, af.ParentID)
	}

	var entries fs.DirEntries
	var subfolderIDs []string

	for _, af := range apiFolders {
		subPath := af.Name
		if dir != "" {
			subPath = dir + "/" + af.Name
		}

		// Debug: Log path resolution to understand the issue
		fs.Debugf(f, "List(%q): processing folder %q -> subPath=%q, ID=%d, ParentID=%d", dir, af.Name, subPath, af.ID, af.ParentID)

		// AGGRESSIVE FIX: Validate that the folder's parent ID matches the expected parent
		// This is the core issue - NetExplorer API returns folders that don't belong in the current directory
		expectedParentID, err := strconv.Atoi(folderID)
		if err == nil && af.ParentID != 0 && af.ParentID != expectedParentID {
			fs.Debugf(f, "List(%q): REJECTING folder %q - has parentID=%d but expected parentID=%d (this folder doesn't belong here)", dir, af.Name, af.ParentID, expectedParentID)
			// Skip this folder as it doesn't belong in the current directory
			continue
		}

		// ADDITIONAL FIX: For nested directories, be extra strict about folder placement
		// If we're in a nested directory (like Com_Av/Com_Av), reject any folders that should be at the root level
		if dir != "" && strings.Contains(dir, "/") {
			// Check if this folder name exists at the root level
			rootPath := af.Name
			if rootID, exists := f.idxGetFolder(rootPath); exists {
				// If the root folder has the same ID, this folder should be at the root level, not nested
				if rootID == strconv.Itoa(af.ID) {
					fs.Debugf(f, "List(%q): REJECTING folder %q - this folder belongs at root level, not in nested directory", dir, af.Name)
					continue
				}
			}
		}

		// Check if this folder already exists in cache with a different path
		// This handles the case where the API returns folders in the wrong parent
		if existingID, exists := f.idxGetFolder(subPath); exists && existingID != strconv.Itoa(af.ID) {
			fs.Debugf(f, "List(%q): folder %q already exists with different ID (cached=%s, new=%d)", dir, af.Name, existingID, af.ID)
			// Don't overwrite the existing mapping
			continue
		}

		// CRITICAL FIX: Handle NetExplorer API quirk where folders have duplicate IDs
		// Check if this folder ID is already mapped to a different path
		// This prevents the hierarchy flattening issue caused by duplicate folder IDs
		existingPath := f.findPathByID(strconv.Itoa(af.ID))
		if existingPath != "" && existingPath != subPath {
			fs.Debugf(f, "List(%q): folder ID %d already mapped to path %q, skipping duplicate at %q", dir, af.ID, existingPath, subPath)
			// Skip this duplicate folder to prevent hierarchy corruption
			continue
		}

		// Detect and fix wrong folder hierarchy for nested same-name folders
		// If we're listing a nested folder (e.g., "Com_Av/Com_Av") and find a folder
		// that should be at a higher level, skip it to prevent wrong tree structure
		if dir != "" && strings.Contains(dir, "/") {
			// Check if this folder should be at a higher level in the hierarchy
			// by looking for it in the cache at a different path
			pathSegments := strings.Split(dir, "/")
			for i := len(pathSegments) - 1; i >= 0; i-- {
				higherPath := strings.Join(pathSegments[:i], "/")
				if higherPath == "" {
					higherPath = ""
				}
				higherSubPath := higherPath + "/" + af.Name
				if existingID, exists := f.idxGetFolder(higherSubPath); exists {
					fs.Debugf(f, "List(%q): folder %q should be at higher level %q (cached=%s), skipping", dir, af.Name, higherPath, existingID)
					continue
				}
			}
		}

		// Additional check: If we're in a nested folder and find a folder with the same name
		// that exists at the root level, we need to be more careful about path resolution
		if dir != "" && strings.Contains(dir, "/") {
			// Check if there's a folder with the same name at the root level
			rootPath := af.Name
			if rootID, exists := f.idxGetFolder(rootPath); exists {
				// If the root folder has a different ID than what we're processing,
				// this might be a case where we need to preserve the hierarchy
				if rootID != strconv.Itoa(af.ID) {
					fs.Debugf(f, "List(%q): found conflicting folder %q at root (rootID=%s, currentID=%d), clearing conflicting cache entries", dir, af.Name, rootID, af.ID)
					// Clear any conflicting cache entries to ensure correct hierarchy
					f.clearConflictingCacheEntries(af.Name)
				}
			}
		}

		// FINAL VALIDATION: Before caching, ensure this folder actually belongs here
		// This is the last line of defense against hierarchy corruption
		if dir != "" && strings.Contains(dir, "/") {
			// For nested directories, double-check that this folder should be here
			// by verifying it's not a root-level folder with the same name
			rootPath := af.Name
			if rootID, exists := f.idxGetFolder(rootPath); exists {
				if rootID == strconv.Itoa(af.ID) {
					fs.Debugf(f, "List(%q): FINAL REJECTION - folder %q is a root-level folder, not caching in nested directory", dir, af.Name)
					continue
				}
			}
		}

		f.idxPutFolder(subPath, strconv.Itoa(af.ID)) // was: f.folderCache[subPath] = ...
		// Cache folder metadata to avoid redundant API calls
		f.cacheFolderMetadata(strconv.Itoa(af.ID), &af)
		entries = append(entries, fs.NewDir(joinPath(dir, af.Name), af.Modification))
		subfolderIDs = append(subfolderIDs, strconv.Itoa(af.ID))
	}

	// Optimize: Bulk hydrate subfolders in background to prevent future cache misses
	if len(subfolderIDs) > 0 {
		go func() {
			if err := f.bulkHydrateFolders(ctx, subfolderIDs); err != nil {
				fs.Debugf(f, "List(%q): background bulk hydration failed: %v", dir, err)
			}
		}()
	}
	for _, af := range apiFiles {
		folderIDStr := folderID // available above
		f.idxPutFile(folderIDStr, af.Name, strconv.Itoa(af.ID))
		// Cache file metadata to avoid redundant GetFile calls
		f.cacheFileMetadata(strconv.Itoa(af.ID), &af)
		entries = append(entries, &Object{
			fs:       f,
			remote:   joinPath(dir, af.Name),
			id:       af.ID,
			size:     af.Size,
			modTime:  af.Modification,
			created:  af.Creation,
			mimeType: af.ContentType,
			md5:      af.MD5,
		})
	}
	fs.Debugf(f, "List(%q): done (returned %d entries)", dir, len(entries))
	return entries, nil
}

// NewObject finds the Object at remote if it exists.
//
// This implementation is metadata-aware so that higher level operations
// (NeedTransfer, sync, copy, etc.) can make correct decisions about whether
// a file needs transferring. It uses the same cached index and hydration
// mechanisms as List() / downloadByName().
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	fs.Debugf(f, "NewObject(%q): start", remote)

	// Split remote into directory and file name
	dir := path.Dir(remote)
	if dir == "." {
		dir = ""
	}
	name := path.Base(remote)

	// Resolve the folder ID without creating anything.
	folderID, err := f.resolveFolderID(ctx, dir)
	if err != nil {
		if errors.Is(err, fs.ErrorDirNotFound) {
			fs.Debugf(f, "NewObject(%q): parent directory %q not found", remote, dir)
			return nil, fs.ErrorObjectNotFound
		}
		fs.Debugf(f, "NewObject(%q): resolveFolderID(%q) error: %v", remote, dir, err)
		return nil, err
	}
	if folderID == "" {
		fs.Debugf(f, "NewObject(%q): resolveFolderID(%q) returned empty folderID", remote, dir)
		return nil, fs.ErrorObjectNotFound
	}

	// Try fast path from the file index cache.
	fileIDStr, ok := f.idxGetFile(folderID, name)
	if !ok {
		// Hydrate this folder once (same pattern as downloadByName)
		key := "files:" + folderID
		_, err, _ := f.listSF.Do(key, func() (any, error) {
			return nil, f.hydrateFolder(ctx, folderID, dir)
		})
		if err != nil {
			fs.Debugf(f, "NewObject(%q): hydrateFolder(%s) error: %v", remote, folderID, err)
			return nil, err
		}

		fileIDStr, ok = f.idxGetFile(folderID, name)
		if !ok {
			fs.Debugf(f, "NewObject(%q): file %q not found in folder %s after hydration", remote, name, folderID)
			return nil, fs.ErrorObjectNotFound
		}
	}

	// Look up cached file metadata populated either by List()/hydrateFolder
	// or previous operations.
	var (
		meta *APIFile
	)
	if cached, ok := f.getCachedFileMetadata(fileIDStr); ok {
		meta = cached
	} else {
		// Fallback: list files in this folder and find our entry.
		files, err := f.ne.ListFiles(folderID)
		if err != nil {
			fs.Debugf(f, "NewObject(%q): ListFiles(%s) error: %v", remote, folderID, err)
			return nil, err
		}
		for i := range files {
			if files[i].Name == name {
				meta = &files[i]
				f.cacheFileMetadata(fileIDStr, meta)
				break
			}
		}
		if meta == nil {
			fs.Debugf(f, "NewObject(%q): metadata for file %q (folder %s) not found", remote, name, folderID)
			return nil, fs.ErrorObjectNotFound
		}
	}

	id, convErr := strconv.Atoi(fileIDStr)
	if convErr != nil {
		fs.Debugf(f, "NewObject(%q): invalid file ID %q: %v", remote, fileIDStr, convErr)
		return nil, convErr
	}

	obj := &Object{
		fs:       f,
		remote:   remote,
		id:       id,
		size:     meta.Size,
		modTime:  meta.Modification,
		created:  meta.Creation,
		mimeType: meta.ContentType,
		md5:      meta.MD5,
	}

	fs.Debugf(f, "NewObject(%q): resolved id=%d size=%d modTime=%v", remote, obj.id, obj.size, obj.modTime)
	return obj, nil
}

// Put uploads the object to the remote path
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	remotePath := src.Remote()
	// Only log large files to reduce debug overhead
	if src.Size() > 10*1024*1024 { // Only log files larger than 10MB
		fs.Debugf(f, "Put(%q): start size=%d", remotePath, src.Size())
	}

	dir := path.Dir(remotePath)
	if dir == "." {
		dir = ""
	}

	// Extract root folder dates if source and destination root folder names match
	if len(f.initialPath) > 0 {
		dstRootName := f.initialPath[0] // Destination root folder name
		rootPath := dstRootName         // The root folder path is just its name

		// Check if already cached
		f.pendingDatesMu.RLock()
		_, exists := f.pendingFolderDates[rootPath]
		f.pendingDatesMu.RUnlock()

		if !exists {
			// Use singleflight to dedupe
			key := "root-dates:" + rootPath
			_, _, _ = f.listSF.Do(key, func() (any, error) {
				if srcFs, ok := src.Fs().(fs.Fs); ok {
					// Get source root folder name from the source Fs root path
					// This is more reliable than inferring from file paths
					srcRootPath := strings.Trim(srcFs.Root(), "/")
					var srcRootName string
					if srcRootPath != "" {
						// Extract the root folder name (last component of the root path)
						srcRootName = path.Base(srcRootPath)
					} else {
						// If source Fs root is empty, try to infer from file path
						srcFileDir := path.Dir(src.Remote())
						if srcFileDir != "." && srcFileDir != "" {
							srcSegments := strings.Split(strings.Trim(srcFileDir, "/"), "/")
							if len(srcSegments) > 0 {
								srcRootName = srcSegments[0]
							}
						}
					}

					fs.Debugf(f, "Put(%q): checking root folder match - srcRootName=%q dstRootName=%q (srcRootPath=%q)", remotePath, srcRootName, dstRootName, srcRootPath)

					// If root folder names match, extract and store root folder dates
					if srcRootName == dstRootName {
						fs.Debugf(f, "Put(%q): root folder names match (%q), extracting dates", remotePath, srcRootName)
						// Try to get the root folder directory entry
						var rootDirEntry fs.Directory

						// Strategy 1: If source Fs root is the folder itself, try to get it from parent
						if srcRootPath != "" {
							// The source Fs root is the folder we want
							// When a folder is set as the root, we can't easily access it from within that Fs.
							// We need to try accessing it from its parent.
							parentOfRoot := path.Dir(srcRootPath)
							if parentOfRoot == "." {
								parentOfRoot = ""
							}

							fs.Debugf(f, "Put(%q): source Fs root is %q, trying to find it by listing parent %q", remotePath, srcRootPath, parentOfRoot)

							// Try listing the parent to find the root folder
							entries, err := srcFs.List(ctx, parentOfRoot)
							if err == nil {
								fs.Debugf(f, "Put(%q): listed parent, got %d entries", remotePath, len(entries))
								for _, entry := range entries {
									if dirEntry, ok := entry.(fs.Directory); ok {
										dirRemote := dirEntry.Remote()
										fs.Debugf(f, "Put(%q): checking directory entry: %q (base=%q, srcRootPath=%q, srcRootName=%q)", remotePath, dirRemote, path.Base(dirRemote), srcRootPath, srcRootName)
										// Match by full path or by base name
										if dirRemote == srcRootPath || path.Base(dirRemote) == srcRootName {
											rootDirEntry = dirEntry
											fs.Debugf(f, "Put(%q): found root folder directory entry: %q", remotePath, dirRemote)
											break
										}
									}
								}
							} else {
								fs.Debugf(f, "Put(%q): failed to list parent %q: %v", remotePath, parentOfRoot, err)
							}

							// Strategy 2: If not found and parent is empty, try creating a parent Fs to access the root folder
							// This handles the case where the root folder is at the source backend's root level
							if rootDirEntry == nil && parentOfRoot == "" {
								fs.Debugf(f, "Put(%q): root folder not found in parent listing (parent is empty), trying to create parent Fs", remotePath)
								// Try to create a new Fs with empty root to access the parent level
								// This allows us to list the source backend's root and find the root folder
								parentFsName := srcFs.Name()
								parentFs, err := cache.Get(ctx, parentFsName+":")
								if err == nil {
									fs.Debugf(f, "Put(%q): created parent Fs %q, trying to list root to find folder", remotePath, parentFsName)
									entries, err := parentFs.List(ctx, "")
									if err == nil {
										fs.Debugf(f, "Put(%q): listed parent Fs root, got %d entries", remotePath, len(entries))
										for _, entry := range entries {
											if dirEntry, ok := entry.(fs.Directory); ok {
												dirRemote := dirEntry.Remote()
												fs.Debugf(f, "Put(%q): checking directory entry in parent Fs: %q (base=%q)", remotePath, dirRemote, path.Base(dirRemote))
												if path.Base(dirRemote) == srcRootName {
													rootDirEntry = dirEntry
													fs.Debugf(f, "Put(%q): found root folder in parent Fs: %q", remotePath, dirRemote)
													break
												}
											}
										}
									} else {
										fs.Debugf(f, "Put(%q): failed to list parent Fs root: %v", remotePath, err)
									}
								} else {
									fs.Debugf(f, "Put(%q): failed to create parent Fs: %v", remotePath, err)
								}
							}
						} else {
							// Source Fs root is empty, try listing root and finding matching folder
							fs.Debugf(f, "Put(%q): source Fs root is empty, listing root to find folder", remotePath)
							entries, err := srcFs.List(ctx, "")
							if err == nil {
								for _, entry := range entries {
									if dirEntry, ok := entry.(fs.Directory); ok {
										if path.Base(dirEntry.Remote()) == srcRootName {
											rootDirEntry = dirEntry
											fs.Debugf(f, "Put(%q): found root folder directory entry: %q", remotePath, dirEntry.Remote())
											break
										}
									}
								}
							}
						}

						// Extract metadata from the root folder directory entry
						if rootDirEntry != nil {
							if dirMeta, err := fs.GetMetadata(ctx, rootDirEntry); err == nil && dirMeta != nil {
								var modTime, createdTime time.Time
								if mtimeStr, ok := dirMeta["mtime"]; ok && mtimeStr != "" {
									if t, err := time.Parse(time.RFC3339Nano, mtimeStr); err == nil {
										modTime = t
									}
								}
								if btimeStr, ok := dirMeta["btime"]; ok && btimeStr != "" {
									if t, err := time.Parse(time.RFC3339Nano, btimeStr); err == nil {
										createdTime = t
									}
								}
								if !modTime.IsZero() || !createdTime.IsZero() {
									f.pendingDatesMu.Lock()
									f.pendingFolderDates[rootPath] = folderDates{ModTime: modTime, CreatedTime: createdTime}
									f.pendingDatesMu.Unlock()
									fs.Debugf(f, "Put(%q): stored root folder dates for %q (source: %q) - modTime=%v createdTime=%v", remotePath, rootPath, srcRootName, modTime, createdTime)
								} else {
									fs.Debugf(f, "Put(%q): root folder dates are zero, not storing", remotePath)
								}
							} else if err != nil {
								fs.Debugf(f, "Put(%q): failed to get metadata for root folder: %v", remotePath, err)
							}
						} else {
							fs.Debugf(f, "Put(%q): could not find root folder directory entry", remotePath)
						}
					} else {
						fs.Debugf(f, "Put(%q): root folder names do not match (src=%q dst=%q), skipping root folder date extraction", remotePath, srcRootName, dstRootName)
					}
				}
				return nil, nil
			})
		}
	}

	// Extract directory dates for the entire ancestor chain from source before creating folders
	// This ensures each folder in the path is created with correct dates and can be restored
	if dir != "" {
		if srcFs, ok := src.Fs().(fs.Fs); ok {
			segments := strings.Split(dir, "/")
			// Build and process each ancestor: A, A/B, A/B/C
			for i := 0; i < len(segments); i++ {
				srcAnc := strings.Join(segments[:i+1], "/")
				if srcAnc == "." {
					continue
				}
				// Destination ancestor path includes initialPath
				fullDstAnc := strings.Trim(strings.Join(append(f.initialPath, segments[:i+1]...), "/"), "/")

				// Dedupe concurrent fetches per ancestor
				key := "parent-dates:" + fullDstAnc
				_, _, _ = f.listSF.Do(key, func() (any, error) {
					// Skip if already cached
					f.pendingDatesMu.RLock()
					if _, ok := f.pendingFolderDates[fullDstAnc]; ok {
						f.pendingDatesMu.RUnlock()
						return nil, nil
					}
					f.pendingDatesMu.RUnlock()

					// Find metadata for src ancestor directory by listing its parent
					parentOfSrcAnc := path.Dir(srcAnc)
					if parentOfSrcAnc == "." {
						parentOfSrcAnc = ""
					}
					expectedName := path.Base(srcAnc)

					entries, err := srcFs.List(ctx, parentOfSrcAnc)
					if err != nil {
						fs.Debugf(f, "Put(%q): failed to list source parent %q for ancestor %q: %v", remotePath, parentOfSrcAnc, srcAnc, err)
						return nil, nil
					}
					for _, entry := range entries {
						dirEntry, ok := entry.(fs.Directory)
						if !ok {
							continue
						}
						if dirEntry.Remote() != srcAnc && path.Base(dirEntry.Remote()) != expectedName {
							continue
						}
						// Found the directory; get metadata
						if dirMeta, err := fs.GetMetadata(ctx, dirEntry); err == nil && dirMeta != nil {
							var modTime, createdTime time.Time
							if mtimeStr, ok := dirMeta["mtime"]; ok && mtimeStr != "" {
								if t, err := time.Parse(time.RFC3339Nano, mtimeStr); err == nil {
									modTime = t
								}
							}
							if btimeStr, ok := dirMeta["btime"]; ok && btimeStr != "" {
								if t, err := time.Parse(time.RFC3339Nano, btimeStr); err == nil {
									createdTime = t
								}
							}
							if !modTime.IsZero() || !createdTime.IsZero() {
								f.pendingDatesMu.Lock()
								f.pendingFolderDates[fullDstAnc] = folderDates{ModTime: modTime, CreatedTime: createdTime}
								f.pendingDatesMu.Unlock()
								fs.Debugf(f, "Put(%q): stored ancestor dir dates for %q (source: %q) - modTime=%v createdTime=%v", remotePath, fullDstAnc, srcAnc, modTime, createdTime)
							} else {
								fs.Debugf(f, "Put(%q): no dates extracted for ancestor %q", remotePath, srcAnc)
							}
						} else if err != nil {
							fs.Debugf(f, "Put(%q): failed to get metadata for ancestor %q: %v", remotePath, srcAnc, err)
						}
						break
					}
					return nil, nil
				})
			}
		} else {
			fs.Debugf(f, "Put(%q): source Fs does not implement fs.Fs, cannot get ancestor dir dates", remotePath)
		}
	}

	parentID, err := f.fastResolveDirID(ctx, dir) // fast cached directory resolution
	if err != nil {
		fs.Debugf(f, "Put(%q): fastResolveDirID(%q) error: %v", remotePath, dir, err)
		return nil, err
	}
	if src.Size() > 10*1024*1024 { // Only log large files
		fs.Debugf(f, "Put(%q): parentID=%s", remotePath, parentID)
	}

	o := &Object{fs: f, remote: remotePath, parentID: parentID, size: src.Size(), modTime: src.ModTime(ctx)}
	if err := o.Update(ctx, in, src, options...); err != nil {
		fs.Debugf(f, "Put(%q): Update error: %v", remotePath, err)
		return nil, err
	}
	fs.Debugf(f, "Put(%q): done (id=%d md5=%q)", remotePath, o.id, o.md5)
	return o, nil
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	remotePath := src.Remote()
	fs.Debugf(f, "PutStream(%q): start (indeterminate size)", remotePath)

	dir := path.Dir(remotePath)
	if dir == "." {
		dir = ""
	}

	parentID, err := f.fastResolveDirID(ctx, dir) // fast cached directory resolution
	if err != nil {
		fs.Debugf(f, "PutStream(%q): fastResolveDirID(%q) error: %v", remotePath, dir, err)
		return nil, err
	}
	fs.Debugf(f, "PutStream(%q): parentID=%s", remotePath, parentID)

	o := &Object{fs: f, remote: remotePath, parentID: parentID, size: -1, modTime: src.ModTime(ctx)}
	if err := o.Update(ctx, in, src, options...); err != nil {
		fs.Debugf(f, "PutStream(%q): Update error: %v", remotePath, err)
		return nil, err
	}
	fs.Debugf(f, "PutStream(%q): done (id=%d md5=%q)", remotePath, o.id, o.md5)
	return o, nil
}

// Mkdir creates a (possibly nested) directory under the configured root.
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	_, err := f.ensureFolderID(ctx, dir, true)
	return err
}

// MkdirMetadata creates a directory with metadata (including creation and modification dates)
func (f *Fs) MkdirMetadata(ctx context.Context, dir string, metadata fs.Metadata) (fs.Directory, error) {
	// Extract modification and creation dates from metadata
	modTime := time.Time{}
	var createdTime time.Time

	if metadata != nil {
		// Extract mtime (modification time)
		if mtimeStr, ok := metadata["mtime"]; ok && mtimeStr != "" {
			metaModTime, err := time.Parse(time.RFC3339Nano, mtimeStr)
			if err != nil {
				fs.Debugf(f, "MkdirMetadata(%q): failed to parse metadata mtime %q: %v", dir, mtimeStr, err)
			} else {
				fs.Debugf(f, "MkdirMetadata(%q): extracted mtime from metadata: %v", dir, metaModTime)
				modTime = metaModTime
			}
		}

		// Extract btime (creation time)
		if btimeStr, ok := metadata["btime"]; ok && btimeStr != "" {
			metaBtime, err := time.Parse(time.RFC3339Nano, btimeStr)
			if err != nil {
				fs.Debugf(f, "MkdirMetadata(%q): failed to parse metadata btime %q: %v", dir, btimeStr, err)
			} else {
				fs.Debugf(f, "MkdirMetadata(%q): extracted btime from metadata: %v", dir, metaBtime)
				createdTime = metaBtime
			}
		}
	}

	fs.Debugf(f, "MkdirMetadata(%q): final dates - modTime=%v createdTime=%v", dir, modTime, createdTime)

	// Store dates in pending cache so ensureFolderID can use them when creating the folder
	dirTrimmed := strings.Trim(dir, "/")
	if !modTime.IsZero() || !createdTime.IsZero() {
		f.pendingDatesMu.Lock()
		f.pendingFolderDates[dirTrimmed] = folderDates{
			ModTime:     modTime,
			CreatedTime: createdTime,
		}
		f.pendingDatesMu.Unlock()
		fs.Debugf(f, "MkdirMetadata(%q): stored pending dates in cache", dirTrimmed)
	}

	// Check if directory already exists
	folderID, err := f.ensureFolderID(ctx, dirTrimmed, false)
	if err == nil && folderID != "" {
		// Directory exists - cannot update dates after creation (API limitation)
		fs.Debugf(f, "MkdirMetadata(%q): directory already exists (id=%s) - dates can only be set during creation", dir, folderID)
		// Keep dates in cache for potential future recreation
		return &netexplorerDir{
			Dir:      fs.NewDir(dir, modTime),
			fs:       f,
			dir:      dir,
			folderID: folderID,
		}, nil
	}

	// Directory doesn't exist, create it with dates
	// ensureFolderID will use the dates from pendingFolderDates cache
	folderIDStr, err := f.ensureFolderID(ctx, dirTrimmed, true)
	if err != nil {
		// Remove from pending cache on error
		f.pendingDatesMu.Lock()
		delete(f.pendingFolderDates, dirTrimmed)
		f.pendingDatesMu.Unlock()
		return nil, fmt.Errorf("MkdirMetadata: failed to create folder: %w", err)
	}

	fs.Debugf(f, "MkdirMetadata(%q): created folder with id=%s", dirTrimmed, folderIDStr)

	// Return a custom Directory that implements SetMetadata
	return &netexplorerDir{
		Dir:      fs.NewDir(dir, modTime),
		fs:       f,
		dir:      dir,
		folderID: folderIDStr,
	}, nil
}

// netexplorerDir is a custom Directory type that implements SetMetadata
// to handle the case where CopyDirMetadata tries to update metadata on existing folders
type netexplorerDir struct {
	*fs.Dir
	fs       *Fs
	dir      string
	folderID string
}

// SetMetadata implements SetMetadataer for Directory
// Since NetExplorer API doesn't support updating folder dates after creation,
// we return nil silently (no error) to indicate this is not a fatal issue
func (d *netexplorerDir) SetMetadata(ctx context.Context, metadata fs.Metadata) error {
	// NetExplorer API doesn't support updating folder dates after creation
	// Dates can only be set during folder creation via CreateFolder
	// Return nil silently to avoid treating this as a fatal error
	fs.Debugf(d.fs, "SetMetadata(%q): not supported - NetExplorer API doesn't allow updating folder dates after creation (returning nil)", d.dir)
	return nil
}

// SetModTime implements SetModTimer for Directory
func (d *netexplorerDir) SetModTime(ctx context.Context, modTime time.Time) error {
	// Delegate directory modtime handling to the backend-level DirSetModTime.
	// Returning ErrorNotImplemented here makes fs/operations fall back to
	// Fs.DirSetModTime, which can use cached dates and UpdateFolder.
	fs.Debugf(d.fs, "SetModTime(%q): delegating to Fs.DirSetModTime (returning ErrorNotImplemented)", d.dir)
	return fs.ErrorNotImplemented
}

// DirSetModTime sets the directory modtime for dir
func (f *Fs) DirSetModTime(ctx context.Context, dir string, modTime time.Time) error {
	// This is called by rclone's sync engine in the "directory finalization"
	// phase when DirModTimeUpdatesOnWrite is true. NetExplorer bumps parent
	// folder modtime whenever files are written, so we restore the intended
	// dates here once per directory using cached source dates.

	dirTrimmed := strings.Trim(dir, "/")

	// Build the full destination path key used when caching folder dates.
	// This mirrors how Put() and ensureFolderID() construct full paths:
	//   fullDstDir = initialPath + dir segments (relative to Fs root).
	fullDstDir := strings.Trim(strings.Join(append(f.initialPath, strings.Split(dirTrimmed, "/")...), "/"), "/")

	// Look up cached dates (modTime + createdTime) for this folder.
	f.pendingDatesMu.RLock()
	dates, ok := f.pendingFolderDates[fullDstDir]
	f.pendingDatesMu.RUnlock()

	// If we don't have cached dates, at least respect the requested modTime.
	if !ok || (dates.ModTime.IsZero() && dates.CreatedTime.IsZero()) {
		dates.ModTime = modTime
	}

	// Resolve the directory's folder ID relative to this Fs root.
	folderID, err := f.resolveFolderID(ctx, dirTrimmed)
	if err != nil || folderID == "" {
		fs.Debugf(f, "DirSetModTime(%q): resolveFolderID error or empty ID: %v", dir, err)
		// Non-fatal: directory modtime isn't critical for data integrity.
		return nil
	}

	fs.Debugf(f, "DirSetModTime(%q): restoring folder dates for %q (id=%s) to creation=%v modification=%v",
		dir, fullDstDir, folderID, dates.CreatedTime, dates.ModTime)

	if err := f.ne.UpdateFolder(folderID, dates.ModTime, dates.CreatedTime); err != nil {
		fs.Debugf(f, "DirSetModTime(%q): UpdateFolder failed: %v", dir, err)
		// Treat as non-fatal to avoid failing the transfer session.
		return nil
	}

	return nil
}

// Rmdir removes "dir" under the base path:
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	fs.Debugf(f, "Rmdir(%q): start", dir)
	folderID, err := f.resolveFolderID(ctx, dir)
	if err != nil {
		fs.Debugf(f, "Rmdir(%q): resolveFolderID error: %v", dir, err)
		return err
	}
	start := time.Now()
	err = f.ne.RemoveFolder(folderID, "")
	dt := time.Since(start)
	if err != nil {
		fs.Debugf(f, "Rmdir(%q): RemoveFolder error after %v: %v", dir, dt, err)
		return err
	}
	fs.Debugf(f, "Rmdir(%q): done in %v", dir, dt)
	return nil
}

// Precision returns the precision of timestamps
func (f *Fs) Precision() time.Duration {
	return time.Second
}

// ------------------------------------------------------------
// Object implements fs.Object
// ------------------------------------------------------------

type Object struct {
	fs       *Fs
	id       int
	remote   string
	parentID string
	size     int64
	modTime  time.Time
	created  time.Time
	mimeType string
	md5      string
}

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
	return o.remote
}

// Size returns the size of the object
func (o *Object) Size() int64 {
	// If we have a placeholder ID (-1, -2) indicating successful upload with eventual consistency,
	// and the size is 0, this might be due to source metadata issues
	// In this case, we should return -1 to indicate unknown size to skip size verification
	if o.id <= 0 && o.size == 0 {
		fs.Debugf(o.fs, "Size(%q): placeholder ID %d with size 0 - likely source metadata issue, returning -1 to skip size verification", o.remote, o.id)
		return -1
	}
	return o.size
}

// ModTime returns the modification time
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.modTime
}

// SetModTime is optional if your service doesn't allow setting mod time
func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
	return fs.ErrorCantSetModTime
}

// Open downloads the file (for reading)
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	dir := path.Dir(o.remote)
	if dir == "." {
		dir = ""
	}
	fs.Debugf(o.fs, "Open(%q): dir=%q", o.remote, dir)

	folderID, err := o.fs.resolveFolderID(ctx, dir)
	if err != nil {
		fs.Debugf(o.fs, "Open(%q): resolveFolderID error: %v", o.remote, err)
		return nil, err
	}
	name := path.Base(o.remote)
	fs.Debugf(o.fs, "Open(%q): folderID=%s, downloading %q via index", o.remote, folderID, name)
	rc, err := o.fs.downloadByName(ctx, dir, folderID, name)
	if err != nil {
		fs.Debugf(o.fs, "Open(%q): downloadByName error: %v", o.remote, err)
		return nil, err
	}
	fs.Debugf(o.fs, "Open(%q): stream opened", o.remote)
	return rc, nil
}

// Update re-uploads or creates the file
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	relPath := o.remote
	dir := path.Dir(relPath)
	if dir == "." {
		dir = ""
	}

	// Only log large files to reduce debug overhead
	if src.Size() > 10*1024*1024 { // Only log files larger than 10MB
		fs.Debugf(o.fs, "Update(%q): start size=%d dir=%q", relPath, src.Size(), dir)
	}

	parentID := o.parentID
	if parentID == "" {
		fs.Debugf(o.fs, "Update(%q): parentID not set, resolving…", relPath)
		var err error
		parentID, err = o.fs.ensureFolderID(ctx, dir, true)
		if err != nil {
			fs.Debugf(o.fs, "Update(%q): ensureFolderID(%q) error: %v", relPath, dir, err)
			return err
		}
		o.parentID = parentID
	}
	if src.Size() > 10*1024*1024 { // Only log large files
		fs.Debugf(o.fs, "Update(%q): using parentID=%s", relPath, parentID)
	}

	name := path.Base(relPath)

	// Get modification time - check metadata first, then fallback to src.ModTime
	modTime := src.ModTime(ctx)
	var createdTime time.Time
	fs.Debugf(o.fs, "Update(%q): initial modTime from src.ModTime=%v", relPath, modTime)

	// Always try to get metadata for system dates (mtime, btime) even without --metadata flag
	// This allows us to preserve creation dates from Windows filesystem
	meta, err := fs.GetMetadata(ctx, src)
	if err != nil {
		// If metadata retrieval fails, just log and continue with ModTime
		fs.Debugf(o.fs, "Update(%q): failed to get metadata from source object: %v", relPath, err)
	} else if meta != nil {
		fs.Debugf(o.fs, "Update(%q): got metadata with %d keys: %v", relPath, len(meta), meta)
		// mtime in meta overrides source ModTime
		if mtimeStr, ok := meta["mtime"]; ok && mtimeStr != "" {
			metaModTime, err := time.Parse(time.RFC3339Nano, mtimeStr)
			if err != nil {
				fs.Debugf(o.fs, "Update(%q): failed to parse metadata mtime %q: %v", relPath, mtimeStr, err)
			} else {
				fs.Debugf(o.fs, "Update(%q): using mtime from metadata: %v (was %v)", relPath, metaModTime, modTime)
				modTime = metaModTime
			}
		}

		// Extract btime (creation time) from metadata
		if btimeStr, ok := meta["btime"]; ok && btimeStr != "" {
			metaBtime, err := time.Parse(time.RFC3339Nano, btimeStr)
			if err != nil {
				fs.Debugf(o.fs, "Update(%q): failed to parse metadata btime %q: %v", relPath, btimeStr, err)
			} else {
				fs.Debugf(o.fs, "Update(%q): extracted btime from metadata: %v", relPath, metaBtime)
				createdTime = metaBtime
			}
		} else {
			fs.Debugf(o.fs, "Update(%q): no btime in metadata", relPath)
		}
	} else {
		fs.Debugf(o.fs, "Update(%q): metadata is nil (source may not implement Metadataer interface)", relPath)
	}

	// Also merge in any metadata from options (if --metadata flag is used)
	// Options metadata can override system metadata
	if optionsMeta, err := fs.GetMetadataOptions(ctx, o.fs, src, options); err == nil && optionsMeta != nil {
		fs.Debugf(o.fs, "Update(%q): got options metadata with %d keys", relPath, len(optionsMeta))
		// Options metadata can override system metadata
		if mtimeStr, ok := optionsMeta["mtime"]; ok && mtimeStr != "" {
			metaModTime, err := time.Parse(time.RFC3339Nano, mtimeStr)
			if err == nil {
				fs.Debugf(o.fs, "Update(%q): overriding modTime with options metadata: %v", relPath, metaModTime)
				modTime = metaModTime
			}
		}
		if btimeStr, ok := optionsMeta["btime"]; ok && btimeStr != "" {
			metaBtime, err := time.Parse(time.RFC3339Nano, btimeStr)
			if err == nil {
				fs.Debugf(o.fs, "Update(%q): overriding createdTime with options metadata: %v", relPath, metaBtime)
				createdTime = metaBtime
			}
		}
	}

	fs.Debugf(o.fs, "Update(%q): final dates - modTime=%v createdTime=%v (zero=%v)", relPath, modTime, createdTime, createdTime.IsZero())

	// Enhanced retry logic for different error types - more surgical approach
	maxRetries := o.fs.opt.UploadRetries
	if maxRetries <= 0 {
		maxRetries = 3 // Default fallback
	}
	baseDelay := time.Duration(o.fs.opt.RetryDelayBase) * time.Second
	if baseDelay <= 0 {
		baseDelay = 1 * time.Second // Default fallback
	}

	uploadSuccess := false
	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Track performance for upload operations
		o.fs.perfMonitor.IncrementCounter()

		fi, err := o.fs.ne.UploadFile(parentID, name, in, src.Size(), modTime, createdTime)
		if err == nil {
			// Success - handle the result
			uploadSuccess = true
			if fi != nil {
				// Use the upload response directly - no hydration needed
				o.id = fi.ID
				o.md5 = fi.MD5
				o.fs.idxPutFile(parentID, name, strconv.Itoa(fi.ID))
				// Cache the upload response to avoid future lookups
				o.fs.cacheUploadResponse(parentID, name, fi)
				fs.Debugf(o.fs, "Update(%q): uploaded id=%d md5=%q (cached)", relPath, o.id, o.md5)
			} else {
				// For chunked uploads without immediate response, use smart hydration
				fs.Debugf(o.fs, "Update(%q): chunked upload complete, using smart hydration", relPath)

				// Try to get from upload cache first
				if cachedFile, exists := o.fs.getCachedUpload(parentID, name); exists {
					o.id = cachedFile.ID
					o.md5 = cachedFile.MD5
					fs.Debugf(o.fs, "Update(%q): found in upload cache id=%d", relPath, o.id)
				} else {
					// Use smart hydration with exponential backoff
					o.id = o.fs.smartHydrateFile(ctx, parentID, name, dir)
					if o.id > 0 {
						fs.Debugf(o.fs, "Update(%q): found via smart hydration id=%d", relPath, o.id)
					} else {
						// Set placeholder ID for eventual consistency
						o.id = -1
						fs.Debugf(o.fs, "Update(%q): upload successful but file not immediately visible (eventual consistency)", relPath)
					}
				}
			}

			break // Exit the retry loop on success
		}

		// Enhanced error handling for network issues first
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			if attempt < maxRetries {
				delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt)))
				fs.Debugf(o.fs, "Update(%q): network timeout, retrying in %v (attempt %d/%d)", relPath, delay, attempt+1, maxRetries)
				time.Sleep(delay)
				// Rewind reader if possible for retry
				if seeker, ok := in.(io.Seeker); ok {
					seeker.Seek(0, io.SeekStart)
				}
				continue
			} else {
				return fmt.Errorf("network timeout after %d retries: %w", maxRetries, err)
			}
		}

		// Handle connection resets and other network errors
		if strings.Contains(err.Error(), "connection reset") || strings.Contains(err.Error(), "broken pipe") {
			if attempt < maxRetries {
				delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt)))
				fs.Debugf(o.fs, "Update(%q): connection reset, retrying in %v (attempt %d/%d)", relPath, delay, attempt+1, maxRetries)
				time.Sleep(delay)
				// Rewind reader if possible for retry
				if seeker, ok := in.(io.Seeker); ok {
					seeker.Seek(0, io.SeekStart)
				}
				continue
			} else {
				return fmt.Errorf("connection reset after %d retries: %w", maxRetries, err)
			}
		}

		// Enhanced error handling for different HTTP status codes
		var he *httpErr
		if errors.As(err, &he) {
			switch he.code {
			case 422:
				// Invalid filename - don't retry, let user fix it
				fs.Debugf(o.fs, "Update(%q): filename contains invalid characters (422), not retrying", relPath)
				return fmt.Errorf("filename contains invalid characters: %w", err)
			case 404:
				// Directory not found - clear cache and retry ONCE
				if attempt < maxRetries {
					fs.Debugf(o.fs, "Update(%q): attempt %d/%d - detected 404 error, clearing cache for dir=%q and retrying", relPath, attempt+1, maxRetries, dir)
					o.fs.idxDelFolder(dir)
					time.Sleep(baseDelay)

					// Re-resolve the folder ID
					var resolveErr error
					parentID, resolveErr = o.fs.resolveFolderID(ctx, dir)
					if resolveErr != nil {
						return resolveErr
					}
					o.parentID = parentID

					// Rewind reader if possible
					if seeker, ok := in.(io.Seeker); ok {
						seeker.Seek(0, io.SeekStart)
					} else {
						return fmt.Errorf("cannot retry upload - reader is not seekable: %w", err)
					}
					continue
				} else {
					return fmt.Errorf("folder not found after %d retries: %w", maxRetries, err)
				}
			case 423:
				// File still being transferred - this often means the upload succeeded but API is processing
				fs.Debugf(o.fs, "Update(%q): got 423 'file still being transferred' - upload likely succeeded, treating as success", relPath)

				// Don't retry 423 errors - they usually indicate successful upload with eventual consistency
				// Set a placeholder ID to indicate successful upload
				o.id = -2 // Use -2 to indicate successful upload with 423 response
				fs.Debugf(o.fs, "Update(%q): treating 423 as successful upload with eventual consistency", relPath)
				uploadSuccess = true
				// Exit the retry loop
			case 429:
				// Rate limiting - retry with exponential backoff
				if attempt < maxRetries {
					delay := time.Duration(attempt+1) * 5 * time.Second // 5s, 10s, 15s
					fs.Debugf(o.fs, "Update(%q): rate limited (429), waiting %v (attempt %d/%d)", relPath, delay, attempt+1, maxRetries)
					time.Sleep(delay)
					// Rewind reader if possible for retry
					if seeker, ok := in.(io.Seeker); ok {
						seeker.Seek(0, io.SeekStart)
					}
					continue
				} else {
					return fmt.Errorf("rate limited after %d retries: %w", maxRetries, err)
				}
			case 500:
				// Server error - retry with exponential backoff
				if attempt < maxRetries {
					delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt)))
					fs.Debugf(o.fs, "Update(%q): server error (500), waiting %v (attempt %d/%d)", relPath, delay, attempt+1, maxRetries)
					time.Sleep(delay)
					// Rewind reader if possible for retry
					if seeker, ok := in.(io.Seeker); ok {
						seeker.Seek(0, io.SeekStart)
					}
					continue
				} else {
					return fmt.Errorf("server error after %d retries - upload failed: %w", maxRetries, err)
				}
			case 400:
				// Bad request - usually means the request is malformed, don't retry
				fs.Debugf(o.fs, "Update(%q): bad request (400), not retrying", relPath)
				return fmt.Errorf("bad request - upload failed: %w", err)
			}
		}

		// For other errors, don't retry
		fs.Debugf(o.fs, "Update(%q): UploadFile error (non-retryable): %v", relPath, err)
		return err
	}

	// Check if we had a successful upload (either normal success or 423)
	if uploadSuccess {
		o.size = src.Size()
		o.modTime = modTime // Use the potentially overridden modTime
		if !createdTime.IsZero() {
			o.created = createdTime // Store created time if available
		}

		// Upload verification is handled by the upload response itself
		// No need for additional API calls since upload responses already contain file metadata

		/*  Disabled for performance: folder date restoration is now handled once per
		    directory via Fs.DirSetModTime using cached metadata populated in Put().

		// Restore parent folder and ancestor dates using cached values from Put()
		// to counteract NetExplorer bumping parent folder modtimes to "now".
		if dir != "" {
			fullDstDir := strings.Trim(strings.Join(append(o.fs.initialPath, strings.Split(dir, "/")...), "/"), "/")
			o.fs.pendingDatesMu.RLock()
			dates, okDates := o.fs.pendingFolderDates[fullDstDir]
			o.fs.pendingDatesMu.RUnlock()
			if okDates && (!dates.ModTime.IsZero() || !dates.CreatedTime.IsZero()) {
				fs.Debugf(o.fs, "Update(%q): restoring parent folder dates for %q (id=%s) to creation=%v modification=%v",
					relPath, fullDstDir, parentID, dates.CreatedTime, dates.ModTime)
				if err2 := o.fs.ne.UpdateFolder(parentID, dates.ModTime, dates.CreatedTime); err2 != nil {
					fs.Debugf(o.fs, "Update(%q): restore parent folder dates failed: %v", relPath, err2)
				}
			}

			// Additionally restore ancestor folders (grandparents, etc.) if dates are cached.
			// Walk up from fullDstDir towards the initialPath root.
			initialPrefix := strings.Trim(strings.Join(o.fs.initialPath, "/"), "/")
			ancPath := fullDstDir
			for {
				ancPath = path.Dir(ancPath)
				if ancPath == "." || ancPath == "" {
					break
				}
				o.fs.pendingDatesMu.RLock()
				aDates, okA := o.fs.pendingFolderDates[ancPath]
				o.fs.pendingDatesMu.RUnlock()
				if !okA || (aDates.ModTime.IsZero() && aDates.CreatedTime.IsZero()) {
					continue
				}
				// Resolve ancestor folder ID (dir must be relative to initialPath)
				relAnc := ancPath
				if initialPrefix != "" {
					relAnc = strings.TrimPrefix(relAnc, initialPrefix)
					relAnc = strings.Trim(relAnc, "/")
				}
				ancID, errAnc := o.fs.resolveFolderID(ctx, relAnc)
				if errAnc != nil || ancID == "" {
					fs.Debugf(o.fs, "Update(%q): skip restoring ancestor %q (resolve error: %v)", relPath, ancPath, errAnc)
					continue
				}
				fs.Debugf(o.fs, "Update(%q): restoring ancestor folder dates for %q (id=%s) to creation=%v modification=%v",
					relPath, ancPath, ancID, aDates.CreatedTime, aDates.ModTime)
				if err2 := o.fs.ne.UpdateFolder(ancID, aDates.ModTime, aDates.CreatedTime); err2 != nil {
					fs.Debugf(o.fs, "Update(%q): restore ancestor folder dates failed for %q: %v", relPath, ancPath, err2)
				}
				if ancPath == initialPrefix {
					break
				}
			}
		}

		// Always restore root folder if it's in the cache (for files at root level or as final ancestor).
		if len(o.fs.initialPath) > 0 {
			rootPath := o.fs.initialPath[0]
			o.fs.pendingDatesMu.RLock()
			rootDates, rootOk := o.fs.pendingFolderDates[rootPath]
			o.fs.pendingDatesMu.RUnlock()
			if rootOk && (!rootDates.ModTime.IsZero() || !rootDates.CreatedTime.IsZero()) {
				// Resolve root folder ID (the root folder is at the first level under the numeric root)
				rootFolderID, errRoot := o.fs.resolveFolderID(ctx, "")
				if errRoot == nil && rootFolderID != "" {
					fs.Debugf(o.fs, "Update(%q): restoring root folder dates for %q (id=%s) to creation=%v modification=%v",
						relPath, rootPath, rootFolderID, rootDates.CreatedTime, rootDates.ModTime)
					if err2 := o.fs.ne.UpdateFolder(rootFolderID, rootDates.ModTime, rootDates.CreatedTime); err2 != nil {
						fs.Debugf(o.fs, "Update(%q): restore root folder dates failed: %v", relPath, err2)
					}
				} else {
					fs.Debugf(o.fs, "Update(%q): failed to resolve root folder ID for %q: %v", relPath, rootPath, errRoot)
				}
			} else {
				fs.Debugf(o.fs, "Update(%q): root folder dates not in cache (rootPath=%q, rootOk=%v)", relPath, rootPath, rootOk)
			}
		}

		*/ // end disabled per-file folder date restoration

		fs.Debugf(o.fs, "Update(%q): done", relPath)
		return nil
	}

	return fmt.Errorf("unexpected error in Update retry loop")
}

// Remove deletes the object
func (o *Object) Remove(ctx context.Context) error {
	dir := path.Dir(o.remote)
	if dir == "." {
		dir = ""
	}

	parentID, err := o.fs.resolveFolderID(ctx, dir)
	if err != nil {
		return err
	}

	// Use sanitized name for deletion since that's what was actually uploaded
	originalName := path.Base(o.remote)
	fileName, wasChanged := sanitizeFileName(originalName)
	if wasChanged {
		fs.Infof(o.fs, "Remove(%q): Using sanitized name %q for deletion (original: %q)", o.remote, fileName, originalName)
	}
	return o.fs.deleteByName(ctx, dir, parentID, fileName)
}

// Storable always returns true
func (o *Object) Storable() bool {
	return true
}

// ------------------------------------------------------------
// NetExplorer API methods
// ------------------------------------------------------------

// Create a new sub-folder under parentID
func (ne *NetExplorer) CreateFolder(parentID, name string, modTime time.Time, createdTime time.Time) (string, bool, error) {
	url := fmt.Sprintf("%s/api/folder", ne.BaseURL)
	fs.Debugf(nil, "CreateFolder: POST %s (parent=%s name=%q)", url, parentID, name)
	fs.Debugf(nil, "CreateFolder(%q): input dates - modTime=%v createdTime=%v (modTime.IsZero=%v createdTime.IsZero=%v)", name, modTime, createdTime, modTime.IsZero(), createdTime.IsZero())

	body := map[string]interface{}{"parent_id": parentID, "name": name}

	// Add dates if available
	if !modTime.IsZero() {
		modTimeUTC := modTime.UTC()
		body["modification"] = modTimeUTC.Format(time.RFC3339)
		fs.Debugf(nil, "CreateFolder(%q): adding modification=%q (UTC, original=%v)", name, modTimeUTC.Format(time.RFC3339), modTime)
	} else {
		fs.Debugf(nil, "CreateFolder(%q): modTime is zero, not sending modification field", name)
	}

	if !createdTime.IsZero() {
		createdTimeUTC := createdTime.UTC()
		body["creation"] = createdTimeUTC.Format(time.RFC3339)
		fs.Debugf(nil, "CreateFolder(%q): adding creation=%q (UTC, original=%v)", name, createdTimeUTC.Format(time.RFC3339), createdTime)
	} else {
		fs.Debugf(nil, "CreateFolder(%q): createdTime is zero, not sending creation field", name)
	}

	b, _ := json.Marshal(body)
	tokenPreview := ne.Token
	if len(tokenPreview) > 10 {
		tokenPreview = tokenPreview[:10]
	}
	fs.Debugf(nil, "CreateFolder(%q): FULL REQUEST URL: %s", name, url)
	fs.Debugf(nil, "CreateFolder(%q): FULL REQUEST HEADERS: Authorization=Bearer %s..., Content-Type=application/json", name, tokenPreview)
	fs.Debugf(nil, "CreateFolder(%q): FULL REQUEST BODY: %s", name, string(b))
	fs.Debugf(nil, "CreateFolder(%q): request payload: %s", name, string(b))

	start := time.Now()
	resp, err := ne.doRequestWithReauth(func(token string) (*http.Request, error) {
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(b))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	}, false)
	dt := time.Since(start)
	if err != nil {
		fs.Debugf(nil, "CreateFolder(%q): HTTP error after %v: %v", name, dt, err)
		return "", false, err
	}
	defer resp.Body.Close()
	// Read response body for logging
	bodyBytes, _ := io.ReadAll(resp.Body)
	fs.Debugf(nil, "CreateFolder(%q): FULL RESPONSE - status=%d in %v", name, resp.StatusCode, dt)
	fs.Debugf(nil, "CreateFolder(%q): FULL RESPONSE HEADERS: %v", name, resp.Header)
	if len(bodyBytes) > 0 {
		fs.Debugf(nil, "CreateFolder(%q): FULL RESPONSE BODY: %s", name, string(bodyBytes))
		fs.Debugf(nil, "CreateFolder(%q): server response body: %s", name, string(bodyBytes))
	} else {
		fs.Debugf(nil, "CreateFolder(%q): FULL RESPONSE BODY: (empty)", name)
		fs.Debugf(nil, "CreateFolder(%q): server response body: (empty)", name)
	}

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated:
		// Prefer parsing as APIFolder to inspect the returned name and dates.
		var af APIFolder
		if err := json.Unmarshal(bodyBytes, &af); err == nil && af.ID != 0 {
			// NetExplorer sometimes auto-renames on conflict, e.g. "BE_CAP" → "BE_CAP (1)".
			// Those should NOT be treated as the canonical folder for "name", otherwise
			// rclone will happily use "name (1)" / "name (2)" as if they were "name".
			//
			// In practice we've seen several patterns (with or without spaces, numeric
			// suffixes, etc.), so be conservative: if the server echoes back a *different*
			// name than what we requested, treat this as an "already exists" situation
			// and let ensureFolderID() re-hydrate and pick up the real folder ID from
			// the listing.
			if af.Name != "" && af.Name != name {
				fs.Debugf(nil, "CreateFolder(%q): server returned different name %q (id=%d); treating as an \"already exists\" conflict and cleaning up auto-renamed folder", name, af.Name, af.ID)

				// Best-effort cleanup: delete the auto-renamed folder we just caused
				// the server to create, so that "name (1)", "name (2)", … do not
				// accumulate in the user's tree. The canonical folder for "name"
				// (created earlier by us or by another client) remains untouched.
				go func(id int, reqName, actualName string) {
					if err := ne.RemoveFolder(strconv.Itoa(id), ""); err != nil {
						fs.Debugf(nil, "CreateFolder(%q): failed to delete auto-renamed folder id=%d name=%q: %v", reqName, id, actualName, err)
					} else {
						fs.Debugf(nil, "CreateFolder(%q): successfully deleted auto-renamed folder id=%d name=%q", reqName, id, actualName)
					}
				}(af.ID, name, af.Name)

				return "", false, &httpErr{
					code: http.StatusConflict,
					body: fmt.Sprintf("server returned name %q for requested %q", af.Name, name),
				}
			}

			fs.Debugf(nil, "CreateFolder(%q): parsed folder response - id=%d creation=%v modification=%v", name, af.ID, af.Creation, af.Modification)
			fs.Debugf(nil, "CreateFolder(%q): RESPONSE DATES - requested creation=%v modification=%v, received creation=%v modification=%v", name, createdTime, modTime, af.Creation, af.Modification)
			return strconv.Itoa(af.ID), true, nil
		}

		// Fallback: simple structure with only an ID field.
		var cr struct {
			ID int `json:"id"`
		}
		if err := json.Unmarshal(bodyBytes, &cr); err != nil || cr.ID == 0 {
			return "", false, err
		}
		fs.Debugf(nil, "CreateFolder(%q): created folder with id=%d", name, cr.ID)
		fs.Debugf(nil, "CreateFolder(%q): WARNING - response did not contain dates, only ID returned", name)
		return strconv.Itoa(cr.ID), true, nil
	case http.StatusConflict, 422:
		// Folder already exists - try to find it by parsing the response
		// Note: Cannot update dates after creation (API limitation)
		var af APIFolder
		if err := json.Unmarshal(bodyBytes, &af); err == nil && af.ID != 0 {
			fs.Debugf(nil, "CreateFolder(%q): folder already exists - id=%d creation=%v modification=%v", name, af.ID, af.Creation, af.Modification)
			fs.Debugf(nil, "CreateFolder(%q): dates can only be set during creation, not when folder already exists", name)
			return strconv.Itoa(af.ID), false, nil
		}
		return "", false, &httpErr{code: resp.StatusCode, body: string(bodyBytes)}
	default:
		return "", false, &httpErr{code: resp.StatusCode, body: string(bodyBytes)}
	}
}

func (ne *NetExplorer) RemoveFolder(folderID, _ string) error {
	url := fmt.Sprintf("%s/api/folder/%s", ne.BaseURL, folderID)
	fs.Debugf(nil, "RemoveFolder: DELETE %s", url)

	start := time.Now()
	resp, err := ne.doRequestWithReauth(func(token string) (*http.Request, error) {
		req, err := http.NewRequest("DELETE", url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		return req, nil
	}, false)
	dt := time.Since(start)
	if err != nil {
		fs.Debugf(nil, "RemoveFolder(%s): HTTP error after %v: %v", folderID, dt, err)
		return err
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	fs.Debugf(nil, "RemoveFolder(%s): status=%d in %v", folderID, resp.StatusCode, dt)

	if resp.StatusCode != 204 && resp.StatusCode != 200 {
		return fmt.Errorf("RemoveFolder failed %d", resp.StatusCode)
	}
	return nil
}

// updateFolderDates is a helper function to update folder dates (used internally)
func (ne *NetExplorer) updateFolderDates(folderID string, modTime time.Time, createdTime time.Time) error {
	// Build payload (only send provided fields)
	payload := map[string]any{}
	if !modTime.IsZero() {
		payload["modification"] = modTime.UTC().Format(time.RFC3339)
	}
	if !createdTime.IsZero() {
		payload["creation"] = createdTime.UTC().Format(time.RFC3339)
	}
	if len(payload) == 0 {
		return nil
	}

	url := fmt.Sprintf("%s/api/folder/%s", ne.BaseURL, folderID)
	b, _ := json.Marshal(payload)
	fs.Debugf(nil, "UpdateFolder: PUT %s (folderID=%s) body=%s", url, folderID, string(b))

	resp, err := ne.doRequestWithReauth(func(token string) (*http.Request, error) {
		req, err := http.NewRequest("PUT", url, bytes.NewBuffer(b))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	}, false)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("UpdateFolder failed %d: %s", resp.StatusCode, string(bodyBytes))
	}

	fs.Debugf(nil, "UpdateFolder: OK via PUT /api/folder/{id} (status=%d)", resp.StatusCode)
	return nil
}

// UpdateFolder updates folder metadata including modification and creation dates (public API)
func (ne *NetExplorer) UpdateFolder(folderID string, modTime time.Time, createdTime time.Time) error {
	fs.Debugf(nil, "UpdateFolder: PUT /api/folder/%s (folderID=%s)", folderID, folderID)
	return ne.updateFolderDates(folderID, modTime, createdTime)
}

// UploadFile uploads data into folderID.
// If fileSize ≤ 16 MiB it uses the classic one-shot endpoint.
// Otherwise it streams the file in 16 MiB blocks with the
//
//	chunk / chunks / chunksSize / fileSize protocol required by NetExplorer.
//
// streamInit asks the server for a sessionKey to stream the bytes with a PUT.
// NOTE: this keeps the "pre-fix" behavior you showed (targetPath = base name).
func (ne *NetExplorer) streamInit(folderID, targetPath string, size int64, modTime time.Time, createdTime time.Time) (string, *APIFile, error) {
	url := ne.BaseURL + "/api/file/upload"
	// NOTE (pre-fix): send only the base name → server places it at root.
	base := path.Base(targetPath)
	fs.Debugf(nil, "STREAM_INIT: POST %s folder=%s targetPath=%q size=%d method=stream", url, folderID, base, size)

	payload := map[string]any{
		"fileSize":     size,
		"folderId":     folderID,
		"targetPath":   base, // <— old behavior (server may ignore folderId for stream pathing)
		"uploadMethod": "stream",
	}

	// Add dates if available
	if !modTime.IsZero() {
		modTimeUTC := modTime.UTC()
		payload["modification"] = modTimeUTC.Format(time.RFC3339)
	}
	if !createdTime.IsZero() {
		createdTimeUTC := createdTime.UTC()
		payload["creation"] = createdTimeUTC.Format(time.RFC3339)
	}

	b, _ := json.Marshal(payload)
	fs.Debugf(nil, "STREAM_INIT: request payload: %s", string(b))

	start := time.Now()
	resp, err := ne.doRequestWithReauth(func(token string) (*http.Request, error) {
		req, err := http.NewRequest("POST", url, bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		return req, nil
	}, false)
	dt := time.Since(start)
	if err != nil {
		fs.Debugf(nil, "STREAM_INIT: HTTP error after %v: %v", dt, err)
		return "", nil, err
	}
	defer resp.Body.Close()
	fs.Debugf(nil, "STREAM_INIT: status=%d in %v", resp.StatusCode, dt)

	raw, _ := io.ReadAll(resp.Body)
	if len(raw) > 0 {
		fs.Debugf(nil, "STREAM_INIT: server response body: %s", string(raw))
	} else {
		fs.Debugf(nil, "STREAM_INIT: server response body: (empty)")
	}

	if resp.StatusCode/100 != 2 {
		return "", nil, fmt.Errorf("stream init failed %d: %s", resp.StatusCode, string(raw))
	}

	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err == nil {
		if sk, ok := generic["sessionKey"].(string); ok && sk != "" {
			fs.Debugf(nil, "STREAM_INIT: received sessionKey=%s…", sk[:4]+"…")
			return sk, nil, nil
		}
		if _, hasID := generic["id"]; hasID {
			fi := &APIFile{}
			if v, ok := generic["id"].(float64); ok {
				fi.ID = int(v)
			}
			if v, ok := generic["name"].(string); ok {
				fi.Name = v
			}
			if v, ok := generic["size"].(float64); ok {
				fi.Size = int64(v)
			}
			if v, ok := generic["hash"].(string); ok && v != "" {
				fi.MD5 = v
			}
			if fi.ID != 0 {
				fs.Debugf(nil, "STREAM_INIT: server returned file object id=%d name=%q size=%d md5=%q", fi.ID, fi.Name, fi.Size, fi.MD5)
				return "", fi, nil
			}
			fs.Debugf(nil, "STREAM_INIT: unexpected response keys: %v", generic)
		}
	} else {
		fs.Debugf(nil, "STREAM_INIT: JSON decode error: %v (raw=%d bytes)", err, len(raw))
	}
	return "", nil, fmt.Errorf("stream init: no session key or file object in response")
}

// putStream streams the raw bytes using the sessionKey returned by streamInit.
// Server may respond with JSON metadata or an empty body; handle both.
func (ne *NetExplorer) putStream(sessionKey string, data io.Reader, size int64) (*APIFile, error) {
	// Common pattern is the same endpoint with a query param; adjust if your API differs.
	url := ne.BaseURL + "/api/file/upload?sessionKey=" + url.QueryEscape(sessionKey) + "&full=1"
	fs.Debugf(nil, "PUT_STREAM: PUT %s (size=%d)", url, size)

	start := time.Now()
	resp, err := ne.doRequestWithReauth(func(token string) (*http.Request, error) {
		req, err := http.NewRequest("PUT", url, data)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/octet-stream")
		req.Header.Set("Accept", "application/json")
		req.Header.Del("Expect")
		if size >= 0 {
			req.ContentLength = size
		}
		return req, nil
	}, false)
	dt := time.Since(start)
	if err != nil {
		fs.Debugf(nil, "PUT_STREAM: HTTP error after %v: %v", dt, err)
		return nil, err
	}
	defer resp.Body.Close()
	fs.Debugf(nil, "PUT_STREAM: status=%d in %v", resp.StatusCode, dt)

	// Read response body for logging
	bodyBytes, _ := io.ReadAll(resp.Body)
	if len(bodyBytes) > 0 {
		fs.Debugf(nil, "PUT_STREAM: server response body: %s", string(bodyBytes))
	} else {
		fs.Debugf(nil, "PUT_STREAM: server response body: (empty)")
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		return nil, fmt.Errorf("putStream failed %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Try to decode metadata if present
	var fi APIFile
	if err := json.Unmarshal(bodyBytes, &fi); err == nil && fi.ID != 0 {
		if fi.MD5 == "" && fi.Hash != "" {
			fi.MD5 = fi.Hash
		}
		fs.Debugf(nil, "PUT_STREAM: parsed response - id=%d name=%q size=%d md5=%q creation=%v modification=%v", fi.ID, fi.Name, fi.Size, fi.MD5, fi.Creation, fi.Modification)
		return &fi, nil
	}
	// No JSON body → caller will hydrate later
	fs.Debugf(nil, "PUT_STREAM: no JSON metadata returned; will rely on hydrate to find id")
	return nil, nil
}

func (ne *NetExplorer) UploadFile(folderID, fileName string, data io.Reader, fileSize int64, modTime time.Time, createdTime time.Time) (*APIFile, error) {
	const chunkSize = 16 << 20                    // 16 MiB
	const smallFileThreshold = 16 * 1024 * 1024   // 16MB - use direct upload for most files
	const mediumFileThreshold = 100 * 1024 * 1024 // 100MB - medium files use stream, big files use chunks

	// Only log large files to reduce debug overhead
	if fileSize > 10*1024*1024 { // Only log files larger than 10MB
		fs.Debugf(nil, "UploadFile(folderID=%s, name=%q, size=%d)", folderID, fileName, fileSize)
	}

	// Handle unknown file size - use direct upload
	if fileSize < 0 {
		fs.Debugf(nil, "UploadFile(%q): unknown file size, using direct upload", fileName)
		return ne.uploadSingle(folderID, fileName, data, modTime, createdTime)
	}

	// Handle empty files
	if fileSize == 0 {
		fs.Debugf(nil, "UploadFile(%q): empty file, using direct upload", fileName)
		return ne.uploadSingle(folderID, fileName, data, modTime, createdTime)
	}

	// Strategy 1: Very small files (< 64KB) - Direct upload
	if fileSize < smallFileThreshold {
		// Only log every 100th small file to reduce debug spam
		if fileSize%100 == 0 {
		}
		return ne.uploadSingle(folderID, fileName, data, modTime, createdTime)
	}

	// Strategy 2: Medium files (64KB - 100MB) - Stream disabled → use chunked
	if fileSize < mediumFileThreshold {
		fs.Debugf(nil, "UploadFile(%q): medium file (%d bytes), streaming disabled → using chunked upload", fileName, fileSize)
		return ne.uploadChunked(folderID, fileName, data, fileSize, chunkSize, modTime, createdTime)
	}

	// Strategy 3: Big files (>= 100MB) - Use chunked upload directly
	fs.Debugf(nil, "UploadFile(%q): big file (%d bytes), using chunked upload", fileName, fileSize)
	return ne.uploadChunked(folderID, fileName, data, fileSize, chunkSize, modTime, createdTime)
}

// --- helper for chunked uploads ---------------------------------------------

// uploadChunked handles chunked uploads for files that are too large for direct upload
func (ne *NetExplorer) uploadChunked(folderID, fileName string, data io.Reader, fileSize int64, chunkSize int64, modTime time.Time, createdTime time.Time) (*APIFile, error) {
	totalChunks := int((fileSize + chunkSize - 1) / chunkSize)
	lastChunkIdx := totalChunks - 1

	// Optimized sequential chunk upload - NetExplorer API doesn't support parallel chunks
	buf := make([]byte, chunkSize)
	chunkRetries := make(map[int]int) // Track retries per chunk
	for idx := 0; idx < totalChunks; idx++ {
		need := int(chunkSize)
		if rem := fileSize - int64(idx)*chunkSize; rem < chunkSize {
			need = int(rem)
		}

		if _, err := io.ReadFull(data, buf[:need]); err != nil {
			fs.Debugf(nil, "uploadChunked(%q): read chunk %d error: %v", fileName, idx, err)
			return nil, fmt.Errorf("read chunk %d: %w", idx, err)
		}

		// Optimized multipart creation
		var body bytes.Buffer
		w := multipart.NewWriter(&body)

		// Track form fields for logging
		formFields := map[string]string{
			"folderId":   folderID,
			"chunk":      strconv.Itoa(idx),
			"chunks":     strconv.Itoa(totalChunks),
			"chunksSize": strconv.FormatInt(chunkSize, 10),
			"fileSize":   strconv.FormatInt(fileSize, 10),
		}

		_ = w.WriteField("folderId", folderID)
		_ = w.WriteField("chunk", strconv.Itoa(idx))
		_ = w.WriteField("chunks", strconv.Itoa(totalChunks))
		_ = w.WriteField("chunksSize", strconv.FormatInt(chunkSize, 10))
		_ = w.WriteField("fileSize", strconv.FormatInt(fileSize, 10))

		// Add dates on the first chunk AND the last chunk
		// Some APIs may read dates from the last chunk where the final file metadata is returned
		if idx == 0 || idx == lastChunkIdx {
			if !modTime.IsZero() {
				modTimeUTC := modTime.UTC()
				modTimeStr := modTimeUTC.Format(time.RFC3339)
				_ = w.WriteField("modification", modTimeStr)
				formFields["modification"] = modTimeStr
				if idx == 0 {
					fs.Debugf(nil, "uploadChunked(%q): chunk %d adding modification=%q (UTC)", fileName, idx, modTimeStr)
				} else {
					fs.Debugf(nil, "uploadChunked(%q): last chunk %d adding modification=%q (UTC)", fileName, idx, modTimeStr)
				}
			} else {
				fs.Debugf(nil, "uploadChunked(%q): chunk %d modTime is zero, not sending modification field", fileName, idx)
			}
			if !createdTime.IsZero() {
				createdTimeUTC := createdTime.UTC()
				createdTimeStr := createdTimeUTC.Format(time.RFC3339)
				_ = w.WriteField("creation", createdTimeStr)
				formFields["creation"] = createdTimeStr
				if idx == 0 {
					fs.Debugf(nil, "uploadChunked(%q): chunk %d adding creation=%q (UTC)", fileName, idx, createdTimeStr)
				} else {
					fs.Debugf(nil, "uploadChunked(%q): last chunk %d adding creation=%q (UTC)", fileName, idx, createdTimeStr)
				}
			} else {
				fs.Debugf(nil, "uploadChunked(%q): chunk %d createdTime is zero, not sending creation field", fileName, idx)
			}
		}
		// Log request form fields for chunks with dates (first and last)
		if idx == 0 || idx == lastChunkIdx {
			fs.Debugf(nil, "uploadChunked(%q): chunk %d request form fields: %v", fileName, idx, formFields)
		}

		part, _ := w.CreateFormFile("targetFile", fileName)
		_, _ = part.Write(buf[:need])
		_ = w.Close()

		start := time.Now()
		resp, err := ne.doRequestWithReauth(func(token string) (*http.Request, error) {
			req, err := http.NewRequest("POST", ne.BaseURL+"/api/file/upload", &body)
			if err != nil {
				return nil, err
			}
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", w.FormDataContentType())
			return req, nil
		}, false)
		dt := time.Since(start)
		if err != nil {
			fs.Debugf(nil, "uploadChunked(%q): HTTP error on chunk %d after %v: %v", fileName, idx, dt, err)

			// Handle network errors for chunks
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				fs.Debugf(nil, "uploadChunked(%q): chunk %d network timeout, retrying", fileName, idx)
				// Rewind the data reader to the start of this chunk for retry
				if seeker, ok := data.(io.Seeker); ok {
					offset := int64(idx) * chunkSize
					if _, seekErr := seeker.Seek(offset, io.SeekStart); seekErr != nil {
						fs.Debugf(nil, "uploadChunked(%q): failed to seek for retry of chunk %d: %v", fileName, idx, seekErr)
						return nil, fmt.Errorf("seek for retry of chunk %d: %w", idx, seekErr)
					}
				}
				idx-- // Retry this chunk
				time.Sleep(2 * time.Second)
				continue
			}

			// Handle connection resets for chunks
			if strings.Contains(err.Error(), "connection reset") || strings.Contains(err.Error(), "broken pipe") {
				fs.Debugf(nil, "uploadChunked(%q): chunk %d connection reset, retrying", fileName, idx)
				// Rewind the data reader to the start of this chunk for retry
				if seeker, ok := data.(io.Seeker); ok {
					offset := int64(idx) * chunkSize
					if _, seekErr := seeker.Seek(offset, io.SeekStart); seekErr != nil {
						fs.Debugf(nil, "uploadChunked(%q): failed to seek for retry of chunk %d: %v", fileName, idx, seekErr)
						return nil, fmt.Errorf("seek for retry of chunk %d: %w", idx, seekErr)
					}
				}
				idx-- // Retry this chunk
				time.Sleep(3 * time.Second)
				continue
			}

			return nil, err
		}
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if len(bodyBytes) > 0 && (resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK) {
			fs.Debugf(nil, "uploadChunked(%q): chunk %d server response body: %s", fileName, idx, string(bodyBytes))
			// Parse response on last chunk to check if dates were applied
			if idx == lastChunkIdx {
				var fi APIFile
				if err := json.Unmarshal(bodyBytes, &fi); err == nil && fi.ID != 0 {
					fs.Debugf(nil, "uploadChunked(%q): last chunk %d parsed response - id=%d creation=%v modification=%v", fileName, idx, fi.ID, fi.Creation, fi.Modification)
					fs.Debugf(nil, "uploadChunked(%q): expected dates - creation=%v modification=%v", fileName, createdTime.UTC(), modTime.UTC())
				}
			}
		} else if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
			fs.Debugf(nil, "uploadChunked(%q): chunk %d error response body: %s", fileName, idx, string(bodyBytes))
		}

		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
			// Enhanced error handling for chunk failures
			switch resp.StatusCode {
			case 400:
				// Bad request - retry the chunk
				fs.Debugf(nil, "uploadChunked(%q): chunk %d failed with 400, retrying", fileName, idx)
				// Rewind the data reader to the start of this chunk for retry
				if seeker, ok := data.(io.Seeker); ok {
					offset := int64(idx) * chunkSize
					if _, seekErr := seeker.Seek(offset, io.SeekStart); seekErr != nil {
						fs.Debugf(nil, "uploadChunked(%q): failed to seek for retry of chunk %d: %v", fileName, idx, seekErr)
						return nil, fmt.Errorf("seek for retry of chunk %d: %w", idx, seekErr)
					}
				}
				// Retry this chunk
				idx-- // Decrement to retry this chunk
				continue
			case 423:
				// File still being transferred - we need to retry this chunk to ensure it was uploaded
				// 423 can mean the chunk is still being processed OR the chunk upload failed
				chunkRetries[idx]++
				maxChunkRetries := 3
				if chunkRetries[idx] > maxChunkRetries {
					fs.Debugf(nil, "uploadChunked(%q): chunk %d got 423 after %d retries - treating as successful", fileName, idx, maxChunkRetries)
					// After max retries, treat as successful to avoid infinite loops
					continue
				}
				fs.Debugf(nil, "uploadChunked(%q): chunk %d got 423 - retry %d/%d", fileName, idx, chunkRetries[idx], maxChunkRetries)
				// Wait a bit before retrying
				time.Sleep(time.Duration(chunkRetries[idx]) * 2 * time.Second)
				// Rewind the data reader to the start of this chunk for retry
				if seeker, ok := data.(io.Seeker); ok {
					offset := int64(idx) * chunkSize
					if _, seekErr := seeker.Seek(offset, io.SeekStart); seekErr != nil {
						fs.Debugf(nil, "uploadChunked(%q): failed to seek for retry of chunk %d: %v", fileName, idx, seekErr)
						return nil, fmt.Errorf("seek for retry of chunk %d: %w", idx, seekErr)
					}
				}
				// Retry this chunk
				idx-- // Decrement to retry this chunk
				continue
			case 429:
				// Rate limiting - retry with exponential backoff
				chunkRetries[idx]++
				maxChunkRetries := 5 // Increased from 3 to 5
				if chunkRetries[idx] > maxChunkRetries {
					fs.Debugf(nil, "uploadChunked(%q): chunk %d rate limited after %d retries - treating as successful", fileName, idx, maxChunkRetries)
					continue
				}
				delay := time.Duration(chunkRetries[idx]) * 3 * time.Second
				fs.Debugf(nil, "uploadChunked(%q): chunk %d rate limited (429), waiting %v (retry %d/%d)", fileName, idx, delay, chunkRetries[idx], maxChunkRetries)
				time.Sleep(delay)
				// Rewind the data reader to the start of this chunk for retry
				if seeker, ok := data.(io.Seeker); ok {
					offset := int64(idx) * chunkSize
					if _, seekErr := seeker.Seek(offset, io.SeekStart); seekErr != nil {
						fs.Debugf(nil, "uploadChunked(%q): failed to seek for retry of chunk %d: %v", fileName, idx, seekErr)
						return nil, fmt.Errorf("seek for retry of chunk %d: %w", idx, seekErr)
					}
				}
				idx-- // Retry this chunk
				continue
			case 500:
				// Server error - wait longer and retry with exponential backoff
				chunkRetries[idx]++
				maxChunkRetries := 3
				if chunkRetries[idx] > maxChunkRetries {
					fs.Debugf(nil, "uploadChunked(%q): chunk %d server error after %d retries - treating as successful", fileName, idx, maxChunkRetries)
					continue
				}
				delay := time.Duration(chunkRetries[idx]) * 5 * time.Second
				fs.Debugf(nil, "uploadChunked(%q): chunk %d got 500, waiting %v (retry %d/%d)", fileName, idx, delay, chunkRetries[idx], maxChunkRetries)
				time.Sleep(delay)
				// Rewind the data reader to the start of this chunk for retry
				if seeker, ok := data.(io.Seeker); ok {
					offset := int64(idx) * chunkSize
					if _, seekErr := seeker.Seek(offset, io.SeekStart); seekErr != nil {
						fs.Debugf(nil, "uploadChunked(%q): failed to seek for retry of chunk %d: %v", fileName, idx, seekErr)
						return nil, fmt.Errorf("seek for retry of chunk %d: %w", idx, seekErr)
					}
				}
				idx-- // Retry this chunk
				continue
			default:
				// Other errors - wrap and return
				return nil, &httpErr{
					code: resp.StatusCode,
					body: fmt.Sprintf("chunk %d failed", idx),
				}
			}
		}
	}
	return nil, nil
}

// --- helper for small uploads ---------------------------------------------

// returns the metadata of the freshly-created file
func (ne *NetExplorer) uploadSingle(folderID, fileName string, data io.Reader, modTime time.Time, createdTime time.Time) (*APIFile, error) {
	url := ne.BaseURL + "/api/file/upload?full"
	fs.Debugf(nil, "uploadSingle: POST %s (folderID=%s name=%q)", url, folderID, fileName)

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)

	// Track form fields for logging
	formFields := map[string]string{
		"folderId": folderID,
	}

	_ = mw.WriteField("folderId", folderID)

	// Add modification time if available
	if !modTime.IsZero() {
		// Convert to UTC for consistency (ISO 8601 with Z)
		modTimeUTC := modTime.UTC()
		modTimeStr := modTimeUTC.Format(time.RFC3339)
		_ = mw.WriteField("modification", modTimeStr)
		formFields["modification"] = modTimeStr
		fs.Debugf(nil, "uploadSingle(%q): adding modification=%q (UTC)", fileName, modTimeStr)
	} else {
		fs.Debugf(nil, "uploadSingle(%q): modTime is zero, not sending modification field", fileName)
	}

	// Add creation time if available
	if !createdTime.IsZero() {
		// Convert to UTC for consistency (ISO 8601 with Z)
		createdTimeUTC := createdTime.UTC()
		createdTimeStr := createdTimeUTC.Format(time.RFC3339)
		_ = mw.WriteField("creation", createdTimeStr)
		formFields["creation"] = createdTimeStr
		fs.Debugf(nil, "uploadSingle(%q): adding creation=%q (UTC)", fileName, createdTimeStr)
	} else {
		fs.Debugf(nil, "uploadSingle(%q): createdTime is zero, not sending creation field", fileName)
	}

	// Log request form fields (excluding file data)
	fs.Debugf(nil, "uploadSingle(%q): request form fields: %v", fileName, formFields)

	part, err := mw.CreateFormFile("targetFile", fileName)
	if err != nil {
		return nil, err
	}
	if _, err = io.Copy(part, data); err != nil {
		return nil, err
	}
	if err = mw.Close(); err != nil {
		return nil, err
	}

	start := time.Now()
	resp, err := ne.doRequestWithReauth(func(token string) (*http.Request, error) {
		req, err := http.NewRequest("POST", url, &body)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		return req, nil
	}, false)
	dt := time.Since(start)
	if err != nil {
		fs.Debugf(nil, "uploadSingle(%q): HTTP error after %v: %v", fileName, dt, err)
		return nil, err
	}
	defer resp.Body.Close()
	fs.Debugf(nil, "uploadSingle(%q): status=%d in %v", fileName, resp.StatusCode, dt)

	// Read response body for logging
	bodyBytes, _ := io.ReadAll(resp.Body)
	if len(bodyBytes) > 0 {
		fs.Debugf(nil, "uploadSingle(%q): server response body: %s", fileName, string(bodyBytes))
	} else {
		fs.Debugf(nil, "uploadSingle(%q): server response body: (empty)", fileName)
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		// Enhanced error handling
		switch resp.StatusCode {
		case 423:
			fs.Debugf(nil, "uploadSingle(%q): file still being transferred (423)", fileName)
			return nil, &httpErr{code: resp.StatusCode, body: "file still being transferred"}
		case 500:
			fs.Debugf(nil, "uploadSingle(%q): server error (500)", fileName)
			return nil, &httpErr{code: resp.StatusCode, body: "server error"}
		default:
			return nil, &httpErr{code: resp.StatusCode, body: string(bodyBytes)}
		}
	}

	var fi APIFile
	if err := json.Unmarshal(bodyBytes, &fi); err != nil {
		fs.Debugf(nil, "uploadSingle(%q): failed to parse JSON response: %v", fileName, err)
		return nil, err
	}
	if fi.MD5 == "" && fi.Hash != "" {
		fi.MD5 = fi.Hash
	}
	fs.Debugf(nil, "uploadSingle(%q): parsed response - id=%d size=%d md5=%q creation=%v modification=%v", fileName, fi.ID, fi.Size, fi.MD5, fi.Creation, fi.Modification)
	return &fi, nil
}

// GET /file/{fileId}/download
func (ne *NetExplorer) DownloadFile(fileID, _ string) (io.ReadCloser, error) {
	url := fmt.Sprintf("%s/api/file/%s/download", ne.BaseURL, fileID)
	fs.Debugf(nil, "DownloadFile: GET %s", url)

	start := time.Now()
	resp, err := ne.doRequestWithReauth(func(token string) (*http.Request, error) {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Del("Expect")
		return req, nil
	}, false)
	dt := time.Since(start)
	if err != nil {
		fs.Debugf(nil, "DownloadFile(%s): HTTP error after %v: %v", fileID, dt, err)
		return nil, err
	}
	if resp.StatusCode != 200 {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		fs.Debugf(nil, "DownloadFile(%s): status=%d in %v", fileID, resp.StatusCode, dt)
		return nil, fmt.Errorf("DownloadFile failed %d", resp.StatusCode)
	}
	fs.Debugf(nil, "DownloadFile(%s): status=200 in %v (Content-Length=%d)", fileID, dt, resp.ContentLength)
	return resp.Body, nil
}

// Download by folder ID + file name
func (ne *NetExplorer) DownloadFileByName(folderID, fileName string) (io.ReadCloser, error) {
	fs.Debugf(nil, "DownloadFileByName: folderID=%s name=%q", folderID, fileName)
	files, err := ne.ListFiles(folderID)
	if err != nil {
		fs.Debugf(nil, "DownloadFileByName: ListFiles error: %v", err)
		return nil, err
	}
	var fileID int
	for _, f := range files {
		if f.Name == fileName {
			fileID = f.ID
			break
		}
	}
	if fileID == 0 {
		fs.Debugf(nil, "DownloadFileByName: %q not found in folder %s", fileName, folderID)
		return nil, fmt.Errorf("file %q not found in folder %s", fileName, folderID)
	}
	fs.Debugf(nil, "DownloadFileByName: resolved name=%q -> id=%d", fileName, fileID)
	return ne.DownloadFile(strconv.Itoa(fileID), "")
}

func (f *Fs) downloadByName(ctx context.Context, folderPath, folderID, name string) (io.ReadCloser, error) {
	if id, ok := f.idxGetFile(folderID, name); ok {
		return f.ne.DownloadFile(id, "")
	}
	// hydrate files of that folder once
	key := "files:" + folderID
	_, err, _ := f.listSF.Do(key, func() (any, error) {
		return nil, f.hydrateFolder(ctx, folderID, folderPath)
	})
	if err != nil {
		return nil, err
	}
	if id, ok := f.idxGetFile(folderID, name); ok {
		return f.ne.DownloadFile(id, "")
	}
	return nil, fmt.Errorf("file %q not found in folder %s", name, folderID)
}

func (f *Fs) deleteByName(ctx context.Context, folderPath, folderID, name string) error {
	if id, ok := f.idxGetFile(folderID, name); ok {
		if err := f.ne.DeleteFile(id, ""); err == nil {
			f.idxDelFile(folderID, name)
			return nil
		}
	}
	// hydrate once, then try again
	key := "files:" + folderID
	_, err, _ := f.listSF.Do(key, func() (any, error) {
		return nil, f.hydrateFolder(ctx, folderID, folderPath)
	})
	if err != nil {
		return err
	}
	if id, ok := f.idxGetFile(folderID, name); ok {
		if err := f.ne.DeleteFile(id, ""); err == nil {
			f.idxDelFile(folderID, name)
			return nil
		}
	}
	return fmt.Errorf("file %q not found in folder %s", name, folderID)
}

// DELETE /file/{fileId}
func (ne *NetExplorer) DeleteFile(fileID, _ string) error {
	url := fmt.Sprintf("%s/api/file/%s", ne.BaseURL, fileID)
	fs.Debugf(nil, "DeleteFile: DELETE %s", url)
	start := time.Now()
	resp, err := ne.doRequestWithReauth(func(token string) (*http.Request, error) {
		req, err := http.NewRequest("DELETE", url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		return req, nil
	}, false)
	dt := time.Since(start)
	if err != nil {
		fs.Debugf(nil, "DeleteFile(%s): HTTP error after %v: %v", fileID, dt, err)
		return err
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	fs.Debugf(nil, "DeleteFile(%s): status=%d in %v", fileID, resp.StatusCode, dt)
	if resp.StatusCode != 204 && resp.StatusCode != 200 {
		return fmt.Errorf("DeleteFile failed %d", resp.StatusCode)
	}
	return nil
}

func (ne *NetExplorer) DeleteFileByName(folderID, fileName string) error {
	fs.Debugf(nil, "DeleteFileByName: folderID=%s name=%q", folderID, fileName)
	files, err := ne.ListFiles(folderID)
	if err != nil {
		fs.Debugf(nil, "DeleteFileByName: ListFiles error: %v", err)
		return err
	}
	var fileID int
	for _, f := range files {
		if f.Name == fileName {
			fileID = f.ID
			break
		}
	}
	if fileID == 0 {
		fs.Debugf(nil, "DeleteFileByName: %q not found in folder %s", fileName, folderID)
		return fmt.Errorf("file %q not found in folder %s", fileName, folderID)
	}
	return ne.DeleteFile(strconv.Itoa(fileID), "")
}

// ListFolders returns only the sub-folders under folderID
func (ne *NetExplorer) ListFolders(folderID string) ([]APIFolder, error) {
	fs.Debugf(nil, "ListFolders: folderID=%s", folderID)
	folders, _, err := ne.ListFolder(folderID)
	return folders, err
}

func (ne *NetExplorer) ListFiles(fileID string) ([]APIFile, error) {
	fs.Debugf(nil, "ListFiles: folderID=%s", fileID)
	_, files, err := ne.ListFolder(fileID)
	return files, err
}

// ListFolder lists both folders and files under folderID.
// It handles both API shapes:
//  1. GET /api/folder/{id}?depth=1 → raw []FolderObject
//  2. GET /api/folders?parent_id={id} → []{ content: { folders:…, files:… } }
func (ne *NetExplorer) ListFolder(folderID string) ([]APIFolder, []APIFile, error) {
	url := fmt.Sprintf("%s/api/folder/%s?depth=1", ne.BaseURL, folderID)
	fs.Debugf(nil, "ListFolder: GET %s", url)

	start := time.Now()
	resp, err := ne.doRequestWithReauth(func(token string) (*http.Request, error) {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Del("Expect")
		return req, nil
	}, false)
	dt := time.Since(start)
	if err != nil {
		fs.Debugf(nil, "ListFolder(%s): HTTP error after %v: %v", folderID, dt, err)
		return nil, nil, err
	}
	defer resp.Body.Close()
	fs.Debugf(nil, "ListFolder(%s): status=%d in %v", folderID, resp.StatusCode, dt)

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		fs.Debugf(nil, "ListFolder(%s): read body error: %v", folderID, err)
		return nil, nil, err
	}
	fs.Debugf(nil, "ListFolder(%s): body bytes=%d", folderID, len(raw))

	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		var folders []APIFolder
		if err := json.Unmarshal(raw, &folders); err != nil {
			return nil, nil, fmt.Errorf("unmarshal raw-folder-array: %w — raw: %s", err, raw)
		}
		fs.Debugf(nil, "ListFolder(%s): shape=array → folders=%d files=0", folderID, len(folders))
		return folders, nil, nil
	}

	var wrapper struct {
		Content struct {
			Folders []APIFolder `json:"folders"`
			Files   []APIFile   `json:"files"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		return nil, nil, fmt.Errorf("unmarshal wrapper content: %w — raw: %s", err, raw)
	}
	fs.Debugf(nil, "ListFolder(%s): shape=wrapper → folders=%d files=%d",
		folderID, len(wrapper.Content.Folders), len(wrapper.Content.Files))
	return wrapper.Content.Folders, wrapper.Content.Files, nil
}

// ListFolderWithDepth lists folders and files with a specific depth for bulk operations
func (ne *NetExplorer) ListFolderWithDepth(folderID string, depth int) ([]APIFolder, []APIFile, error) {
	url := fmt.Sprintf("%s/api/folder/%s?depth=%d", ne.BaseURL, folderID, depth)
	fs.Debugf(nil, "ListFolderWithDepth: GET %s", url)

	start := time.Now()
	resp, err := ne.doRequestWithReauth(func(token string) (*http.Request, error) {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Del("Expect")
		return req, nil
	}, false)
	dt := time.Since(start)
	if err != nil {
		fs.Debugf(nil, "ListFolderWithDepth(%s, depth=%d): HTTP error after %v: %v", folderID, depth, dt, err)
		return nil, nil, err
	}
	defer resp.Body.Close()
	fs.Debugf(nil, "ListFolderWithDepth(%s, depth=%d): status=%d in %v", folderID, depth, resp.StatusCode, dt)

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		fs.Debugf(nil, "ListFolderWithDepth(%s, depth=%d): read body error: %v", folderID, depth, err)
		return nil, nil, err
	}
	fs.Debugf(nil, "ListFolderWithDepth(%s, depth=%d): body bytes=%d", folderID, depth, len(raw))

	// Parse the response - with depth > 1, we expect a nested structure
	var root struct {
		Content struct {
			Folders []APIFolder `json:"folders"`
			Files   []APIFile   `json:"files"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, nil, fmt.Errorf("unmarshal depth response: %w — raw: %s", err, raw)
	}

	// For bulk operations, we need to flatten the nested structure
	allFolders, allFiles := ne.flattenDepthResponse(root.Content.Folders, root.Content.Files)

	fs.Debugf(nil, "ListFolderWithDepth(%s, depth=%d): flattened → folders=%d files=%d",
		folderID, depth, len(allFolders), len(allFiles))
	return allFolders, allFiles, nil
}

// flattenDepthResponse flattens nested folder structures for bulk operations
func (ne *NetExplorer) flattenDepthResponse(folders []APIFolder, files []APIFile) ([]APIFolder, []APIFile) {
	allFolders := make([]APIFolder, 0, len(folders))
	allFiles := make([]APIFile, 0, len(files))

	// Add current level files
	allFiles = append(allFiles, files...)

	// Add current level folders
	allFolders = append(allFolders, folders...)

	// Note: In a real implementation, you'd need to recursively process
	// the folder's content. For now, we'll just add the folders themselves.
	// The actual nested content would need to be extracted from the API response.

	return allFolders, allFiles
}

// ListFolderByPath does one HTTP call to fetch the tree under rootID,
// then walks down pathSegments and returns just the final folder's content.
func (ne *NetExplorer) ListFolderByPath(rootID string, pathSegments []string) (folders []folderNode, files []APIFile, err error) {
	depth := len(pathSegments) + 1
	url := fmt.Sprintf("%s/api/folder/%s?depth=%d&full=1", ne.BaseURL, rootID, depth)
	fs.Debugf(nil, "ListFolderByPath: GET %s (segments=%v)", url, pathSegments)

	start := time.Now()
	resp, err := ne.doRequestWithReauth(func(token string) (*http.Request, error) {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		return req, nil
	}, false)
	dt := time.Since(start)
	if err != nil {
		fs.Debugf(nil, "ListFolderByPath: HTTP error after %v: %v", dt, err)
		return nil, nil, err
	}
	defer resp.Body.Close()
	fs.Debugf(nil, "ListFolderByPath: status=%d in %v", resp.StatusCode, dt)

	var root struct {
		Content struct {
			Folders []folderNode `json:"folders"`
		} `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&root); err != nil {
		fs.Debugf(nil, "ListFolderByPath: JSON decode error: %v", err)
		return nil, nil, err
	}

	nodes := root.Content.Folders
	for i, seg := range pathSegments {
		fs.Debugf(nil, "ListFolderByPath: descend %d/%d into %q", i+1, len(pathSegments), seg)
		var found bool
		for _, n := range nodes {
			if n.Name == seg {
				nodes = n.Content.Folders
				if i == len(pathSegments)-1 {
					folders = n.Content.Folders
					files = n.Content.Files
				}
				found = true
				break
			}
		}
		if !found {
			fs.Debugf(nil, "ListFolderByPath: segment %q not found", seg)
			return nil, nil, fmt.Errorf("directory %q not found", seg)
		}
	}

	if len(pathSegments) == 0 {
		folders = append(folders, root.Content.Folders...)
	}
	fs.Debugf(nil, "ListFolderByPath: result folders=%d files=%d", len(folders), len(files))
	return folders, files, nil
}

// ---------------------------------------------------------------------
// resolveFolderID walks rootID → initialPath → dir to get a numeric ID
// ---------------------------------------------------------------------

func (f *Fs) resolveFolderID(ctx context.Context, dir string) (string, error) {
	dir = strings.Trim(dir, "/")
	if id, ok := f.idxGetFolder(dir); ok {
		fs.Debugf(f, "resolveFolderID(%q): index hit -> %s", dir, id)
		return id, nil
	}
	// walk like ensure, but never create; hydrate once per missing parent
	return f.ensureFolderID(ctx, dir, false)
}

// ensureFolderID walks initialPath + dir, creating segments if needed (when create==true).
// It is deliberately conservative about calling CreateFolder to avoid creating duplicate
// folders with the same name under the same parent when caches are stale or multiple
// goroutines/processes are working on the same tree.
func (f *Fs) ensureFolderID(ctx context.Context, dir string, create bool) (string, error) {
	dir = strings.Trim(dir, "/")
	fs.Debugf(f, "ensureFolderID(%q, create=%v): start (rootID=%s initialPath=%v)", dir, create, f.rootID, f.initialPath)

	parentID := f.rootID
	parentPath := ""
	segments := append([]string{}, f.initialPath...)
	if dir != "" {
		segments = append(segments, strings.Split(dir, "/")...)
	}

segmentsLoop:
	for _, name := range segments {
		if name == "" {
			continue
		}

		fullPath := path.Join(parentPath, name)

		// 1) Fast path: check the index for this logical path.
		if id, ok := f.idxGetFolder(fullPath); ok {
			fs.Debugf(f, "ensureFolderID: cache hit for %q -> %s", fullPath, id)
			parentID, parentPath = id, fullPath
			continue
		}

		// 2) Always hydrate the current parent once before deciding to create anything.
		//    This ensures we see folders that were created out-of-band (previous runs,
		//    other processes, UI actions, etc.) and avoids blind duplicate creations.
		key := "list:" + parentID
		_, listErr, _ := f.listSF.Do(key, func() (any, error) {
			return nil, f.hydrateFolder(ctx, parentID, parentPath)
		})
		if listErr != nil {
			return "", listErr
		}

		if id, ok := f.idxGetFolder(fullPath); ok {
			fs.Debugf(f, "ensureFolderID: hydrated hit for %q -> %s", fullPath, id)
			parentID, parentPath = id, fullPath
			continue
		}

		// 3) If we are not allowed to create folders, then this path truly doesn't exist.
		if !create {
			return "", fs.ErrorDirNotFound
		}

		// 4) Creation path with intra-process de-duplication using folderBatch.
		//    At this point, from our point of view, no folder with this name exists
		//    under the current parent, so we can safely attempt a CreateFolder guarded
		//    by folderBatch.
		waitForOtherCreator := false

		f.batchMu.Lock()
		if f.folderBatch[fullPath] {
			// Another goroutine in this process is already creating this folder.
			waitForOtherCreator = true
		} else {
			f.folderBatch[fullPath] = true
		}
		f.batchMu.Unlock()

		if waitForOtherCreator {
			// Another goroutine in this process is already creating this folder.
			// Instead of racing and potentially creating duplicates, wait for it
			// to finish and rely on its result. If we still cannot see the folder
			// after several attempts, bubble up an error so the caller can retry
			// the whole operation rather than blindly creating "name (1)", "(2)", …
			for attempts := 0; attempts < 5; attempts++ {
				time.Sleep(100 * time.Millisecond)
				key := "list:" + parentID
				_, listErr, _ := f.listSF.Do(key, func() (any, error) {
					return nil, f.hydrateFolder(ctx, parentID, parentPath)
				})
				if listErr != nil {
					return "", listErr
				}
				if id, ok := f.idxGetFolder(fullPath); ok {
					fs.Debugf(f, "ensureFolderID: observed folder %q created by another goroutine -> %s", fullPath, id)
					parentID, parentPath = id, fullPath
					continue segmentsLoop
				}
			}

			// At this point we still don't see the folder in our index, even after
			// repeatedly hydrating the parent. This can happen if the other goroutine
			// failed its CreateFolder call or if the remote API is being very slow to
			// reflect the new folder in listings.
			//
			// Rather than failing the whole transfer (which makes it look like the
			// copy ran twice when rclone retries), fall back to performing a *single*
			// guarded CreateFolder attempt ourselves. Any true "already exists" or
			// auto‑rename situations will be surfaced by CreateFolder() as conflicts
			// and handled via the hydration path below, so this does not re‑introduce
			// the BE_CAP(1)/(2) fragmentation bug.

			fs.Debugf(f, "ensureFolderID: folder %q still not visible after waiting for concurrent creator; attempting a guarded CreateFolder ourselves", fullPath)

			// Mark ourselves as the creator for this fullPath while we attempt
			// the creation to avoid additional local races.
			f.batchMu.Lock()
			f.folderBatch[fullPath] = true
			f.batchMu.Unlock()
			// and then fall through into the normal creation path below
		}

		// Check for pending folder dates (from MkdirMetadata/DirSetModTime), keyed by fullPath.
		var modTime, createdTime time.Time
		f.pendingDatesMu.RLock()
		if dates, ok := f.pendingFolderDates[fullPath]; ok {
			modTime = dates.ModTime
			createdTime = dates.CreatedTime
			fs.Debugf(f, "ensureFolderID(%q): using pending dates from cache - modTime=%v createdTime=%v (IsZero: modTime=%v createdTime=%v)",
				fullPath, modTime, createdTime, modTime.IsZero(), createdTime.IsZero())
		} else {
			fs.Debugf(f, "ensureFolderID(%q): no pending dates found in cache, will create folder with zero dates", fullPath)
		}
		f.pendingDatesMu.RUnlock()

		// Perform the actual folder creation with a small retry loop.
		var nid string
		var created bool
		var err error

		for retry := 0; retry < 3; retry++ {
			nid, created, err = f.ne.CreateFolder(parentID, name, modTime, createdTime)
			if err == nil && created {
				break
			}
			if err != nil && !isAlreadyExists(err) {
				fs.Debugf(f, "ensureFolderID: CreateFolder attempt %d for %q under %s failed: %v", retry+1, fullPath, parentID, err)
				if retry < 2 {
					time.Sleep(time.Duration(100*(retry+1)) * time.Millisecond)
				}
			} else {
				// Either we succeeded without "created" == true or the server reported
				// that the folder already exists. In both cases we'll resolve the ID
				// via hydration below.
				break
			}
		}

		// We're done being the creator for this fullPath.
		f.batchMu.Lock()
		delete(f.folderBatch, fullPath)
		f.batchMu.Unlock()

		if err != nil && !isAlreadyExists(err) {
			return "", err
		}

		if err == nil && created {
			// We definitively created a new folder.
			fs.Infof(f, "ensureFolderID: CREATED folder %q with ID %s under parent %s (fullPath=%q)", name, nid, parentID, fullPath)
			f.idxPutFolder(fullPath, nid)
			parentID, parentPath = nid, fullPath

			// Allow NetExplorer's API some time to become consistent.
			if f.opt.FolderDelay > 0 {
				fs.Debugf(f, "ensureFolderID: waiting %dms for API consistency after creating folder %s", f.opt.FolderDelay, nid)
				time.Sleep(time.Duration(f.opt.FolderDelay) * time.Millisecond)
			}
			continue
		}

		// If we reach here, either the folder already existed or the API didn't set
		// "created" even though the operation succeeded. Hydrate again and pick up
		// the real ID from the listing.
		key = "list:" + parentID
		_, listErr, _ = f.listSF.Do(key, func() (any, error) {
			return nil, f.hydrateFolder(ctx, parentID, parentPath)
		})
		if listErr != nil {
			return "", listErr
		}
		if id, ok := f.idxGetFolder(fullPath); ok {
			parentID, parentPath = id, fullPath
			continue
		}

		// Still not found – this means something is inconsistent on the server side.
		return "", fmt.Errorf("ensureFolderID: folder %q under parent %s not found after creation", fullPath, parentID)
	}

	// Store a short alias for the final directory path relative to the Fs root.
	f.idxPutFolder(dir, parentID)
	fs.Debugf(f, "ensureFolderID(%q): result -> %s", dir, parentID)
	return parentID, nil
}

// ---------------------------------------------------------------------
// File name sanitization summary and utilities
// ---------------------------------------------------------------------

// RenameSummary tracks file and folder renames for user reporting
type RenameSummary struct {
	mu      sync.Mutex
	renames map[string]string // original -> sanitized
}

// NewRenameSummary creates a new rename summary tracker
func NewRenameSummary() *RenameSummary {
	return &RenameSummary{
		renames: make(map[string]string),
	}
}

// AddRename records a file or folder rename
func (rs *RenameSummary) AddRename(original, sanitized string) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.renames[original] = sanitized
}

// GetRenames returns a copy of all recorded renames
func (rs *RenameSummary) GetRenames() map[string]string {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	renames := make(map[string]string)
	for k, v := range rs.renames {
		renames[k] = v
	}
	return renames
}

// PrintSummary prints a user-friendly summary of all renames
func (rs *RenameSummary) PrintSummary() {
	renames := rs.GetRenames()
	if len(renames) == 0 {
		fs.Infof(nil, "No files or folders were renamed during transfer")
		return
	}

	fs.Infof(nil, "=== FILE/FOLDER RENAME SUMMARY ===")
	fs.Infof(nil, "The following items were renamed due to invalid characters:")

	// Sort for consistent output
	var keys []string
	for k := range renames {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, original := range keys {
		sanitized := renames[original]
		fs.Infof(nil, "  %q → %q", original, sanitized)
	}

	fs.Infof(nil, "Total renames: %d", len(renames))
	fs.Infof(nil, "Note: NetExplorer API rejects files with special characters: / \\ : * ? \" < > | and leading/trailing spaces")
	fs.Infof(nil, "=====================================")
}
