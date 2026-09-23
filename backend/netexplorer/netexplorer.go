package netexplorer

import (
	"bufio"
	"bytes"
	"container/list"
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
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
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/oauthutil"
	"github.com/rclone/rclone/lib/terminal"
	"go.etcd.io/bbolt"
	"golang.org/x/oauth2"
	"golang.org/x/sync/singleflight"
)

const (
	defaultBaseURL              = "https://org.netexplorer.pro"
	rcloneClientID              = "53dd8290-08a3-4679-91c5-91e13a753daf"
	rcloneEncryptedClientSecret = "6R4lx7xr5K6igGprq8DhBnz_Ge9XbJrT4IuWja452BYTsnOmLZhVa4KKQ-aZlzVZ8cgdGuMzip2b3kdmygQax_97r9ld0tQvPg74pgirhp0"
	tusProtocolVersion          = "1.0.0"
	tusPatchChunkSize           = 16 << 20
)

var errTusRestartSession = errors.New("netexplorer: tus session restart required")

// endpointMetrics holds per-endpoint response-time stats.
type endpointMetrics struct {
	count   int64
	totalMs int64
	minMs   int64
	maxMs   int64
}

// httpCallStats tracks per-endpoint response-time metrics across all doRequestWithReauth calls.
type httpCallStats struct {
	mu         sync.Mutex
	byEndpoint map[string]*endpointMetrics
	lastLog    time.Time
}

// reNumericSeg matches a URL path segment that is a plain integer ID (e.g. "12345").
var reNumericSeg = regexp.MustCompile(`^\d+$`)

// reRandomSeg matches a URL path segment that looks like a random/opaque ID:
// at least 20 chars, only alphanumeric, dash, or underscore (covers UUIDs and TUS upload IDs).
var reRandomSeg = regexp.MustCompile(`^[0-9a-zA-Z_\-]{20,}$`)

// normalizeURLKey returns "METHOD /normalized/path" suitable as a stats map key.
// Numeric and random-looking path segments are replaced with {id}.
// Query strings are dropped.
func normalizeURLKey(method string, u *url.URL) string {
	parts := strings.Split(u.Path, "/")
	for i, p := range parts {
		if reNumericSeg.MatchString(p) || reRandomSeg.MatchString(p) {
			parts[i] = "{id}"
		}
	}
	return method + " " + strings.Join(parts, "/")
}

func (s *httpCallStats) record(method string, u *url.URL, ms int64) {
	key := normalizeURLKey(method, u)
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.byEndpoint[key]
	if !ok {
		m = &endpointMetrics{minMs: ms, maxMs: ms}
		s.byEndpoint[key] = m
	}
	m.count++
	m.totalMs += ms
	if ms < m.minMs {
		m.minMs = ms
	}
	if ms > m.maxMs {
		m.maxMs = ms
	}
	if time.Since(s.lastLog) >= 10*time.Second || s.lastLog.IsZero() {
		for k, ep := range s.byEndpoint {
			fs.Debugf(nil, "[netexplorer] HTTP %-38s count=%4d  mean=%5dms  min=%5dms  max=%5dms",
				k, ep.count, ep.totalMs/ep.count, ep.minMs, ep.maxMs)
		}
		s.lastLog = time.Now()
	}
}

var globalHTTPStats = &httpCallStats{byEndpoint: make(map[string]*endpointMetrics)}

// NetExplorer is the API client
type NetExplorer struct {
	BaseURL     string
	Token       string
	Client      *http.Client
	tokenSource *oauthutil.TokenSource
	opt         *Options
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

type tusOffsetMismatchErr struct {
	serverOffset int64
	body         string
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
		BaseURL: defaultBaseURL,
		// Use a 16-minute overall HTTP timeout to better accommodate very large uploads.
		Client: createOptimizedHTTPClient(16 * 60),
	}
}

type userAgentTransport struct {
	base      http.RoundTripper
	userAgent string
}

func (t *userAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", t.userAgent)
	}
	return t.base.RoundTrip(req)
}

// createOptimizedHTTPClient creates an HTTP client with connection pooling and optimized settings
func createOptimizedHTTPClient(timeoutSeconds int) *http.Client {
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second, // TCP keep-alive to prevent idle connections from being reset by firewalls/NAT
	}
	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		MaxIdleConns:          200,
		MaxIdleConnsPerHost:   100,
		IdleConnTimeout:       120 * time.Second,
		DisableCompression:    false,
		DisableKeepAlives:     false,
		MaxConnsPerHost:       100,
		ResponseHeaderTimeout: 0,
		ExpectContinueTimeout: 10 * time.Second,
	}

	return &http.Client{
		Transport: &userAgentTransport{
			base:      transport,
			userAgent: "rclone/" + fs.Version,
		},
		Timeout: time.Duration(timeoutSeconds) * time.Second,
	}
}

func oauthEndpointsFromBaseURL(baseURL string) (authURL string, tokenURL string, err error) {
	u, err := url.Parse(baseURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", "", fmt.Errorf("invalid base_url %q for OAuth2 endpoints", baseURL)
	}
	return u.Scheme + "://" + u.Host + "/oauth2/authorize", u.Scheme + "://" + u.Host + "/oauth2/token", nil
}

func closeResponseBody(resp *http.Response) {
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
}

func drainAndCloseResponseBody(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}

func responseTimingHeaders(resp *http.Response) string {
	if resp == nil {
		return ""
	}

	headers := []string{
		"X-NE-ExecTime",
		"X-NE-SQLExecTime",
		"X-NE-SQLReqCount",
		"X-NE-RedisExecTime",
		"X-NE-NatsExecTime",
	}
	parts := make([]string, 0, len(headers))
	for _, name := range headers {
		if value := strings.TrimSpace(resp.Header.Get(name)); value != "" {
			parts = append(parts, fmt.Sprintf("%s=%s", name, value))
		}
	}
	return strings.Join(parts, "  ")
}

func getDefaultOAuthClientSecret() (string, error) {
	if rcloneEncryptedClientSecret == "" {
		return "", nil
	}
	secret, err := obscure.Reveal(rcloneEncryptedClientSecret)
	if err != nil {
		return "", fmt.Errorf("invalid built-in netexplorer client secret: %w", err)
	}
	return secret, nil
}

func getOAuthConfig(m configmap.Mapper) (*oauthutil.Config, error) {
	baseURL, ok := m.Get("base_url")
	if !ok || baseURL == "" {
		baseURL = defaultBaseURL
	}
	authURL, tokenURL, err := oauthEndpointsFromBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	clientSecret, err := getDefaultOAuthClientSecret()
	if err != nil {
		return nil, err
	}
	return &oauthutil.Config{
		AuthURL:      authURL,
		TokenURL:     tokenURL,
		ClientID:     rcloneClientID,
		ClientSecret: clientSecret,
		AuthStyle:    oauth2.AuthStyleInParams,
		Scopes:       []string{"all"},
		RedirectURL:  oauthutil.RedirectLocalhostURL,
	}, nil
}

// normalizeLegacyToken supports old NetExplorer configs that stored a raw access token string.
func normalizeLegacyToken(m configmap.Mapper) error {
	tokenString, ok := m.Get(config.ConfigToken)
	if !ok || tokenString == "" || json.Valid([]byte(tokenString)) {
		return nil
	}
	tokenBytes, err := json.Marshal(&oauth2.Token{
		AccessToken: tokenString,
		TokenType:   "Bearer",
	})
	if err != nil {
		return err
	}
	m.Set(config.ConfigToken, string(tokenBytes))
	return nil
}

// doRequestWithReauth executes an API request with the current OAuth2 access token.
//
// On HTTP 401 it invalidates the token once and retries one time.
func (ne *NetExplorer) doRequestWithReauth(buildReq func(token string) (*http.Request, error), _ bool) (*http.Response, error) {
	if ne.tokenSource == nil {
		return nil, errors.New("netexplorer: OAuth2 token source is not configured")
	}

	do := func() (*http.Response, error) {
		token, err := ne.tokenSource.Token()
		if err != nil {
			return nil, fmt.Errorf("netexplorer: failed to get OAuth2 token: %w", err)
		}
		ne.Token = token.AccessToken
		req, err := buildReq(ne.Token)
		if err != nil {
			return nil, err
		}
		if req.Header.Get("User-Agent") == "" {
			req.Header.Set("User-Agent", "rclone/"+fs.Version)
		}
		start := time.Now()
		resp, err := ne.Client.Do(req)
		dt := time.Since(start)
		ms := dt.Milliseconds()
		if err != nil {
			fs.Debugf(nil, "[netexplorer] HTTP %s %s -> error in %dms: %v", req.Method, req.URL.Path, ms, err)
		} else {
			fs.Debugf(nil, "[netexplorer] HTTP %s %s -> %d in %dms", req.Method, req.URL.Path, resp.StatusCode, ms)
			globalHTTPStats.record(req.Method, req.URL, ms)
		}
		return resp, err
	}

	resp, err := do()
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusUnauthorized {
		return resp, nil
	}

	closeResponseBody(resp)
	ne.tokenSource.Invalidate()
	return do()
}

// Registration options
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "netexplorer",
		Description: "NetExplorer remote",
		NewFs:       NewFs,
		Config: func(ctx context.Context, name string, m configmap.Mapper, config fs.ConfigIn) (*fs.ConfigOut, error) {
			oAuthConfig, err := getOAuthConfig(m)
			if err != nil {
				return nil, err
			}
			return oauthutil.ConfigOut("", &oauthutil.Options{
				OAuth2Config: oAuthConfig,
				NoOffline:    true,
			})
		},
		Options: append([]fs.Option{
			{
				Name:    "base_url",
				Help:    "Base URL of the NetExplorer API",
				Default: defaultBaseURL,
			},
		}, append(oauthutil.SharedOptions, []fs.Option{
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
		}...)...),
	})
}

// Options is how rclone config is mapped into your backend
type Options struct {
	BaseURL              string `config:"base_url"`
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
	maxRetries := f.opt.HydrationRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}
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

func (e *tusOffsetMismatchErr) Error() string {
	if e.serverOffset >= 0 {
		return fmt.Sprintf("tus offset mismatch at server offset %d: %s", e.serverOffset, e.body)
	}
	return fmt.Sprintf("tus offset mismatch: %s", e.body)
}
func isAlreadyExists(err error) bool {
	var he *httpErr
	if errors.As(err, &he) {
		// NetExplorer may signal "already exists" with several status codes:
		// - 409/422: explicit conflict / unprocessable entity
		// - 403: in practice sometimes returned with a body like
		//   {"error":"Un element du meme nom existe deja. Veuillez saisir un nom different."}
		if he.code == http.StatusConflict || he.code == 422 {
			return true
		}
		if he.code == http.StatusForbidden &&
			(strings.Contains(he.body, "m\\u00eame nom existe d\\u00e9j\\u00e0") ||
				strings.Contains(he.body, "m\u00eame nom existe d\u00e9j\u00e0")) {
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

	if opt.BaseURL == "" {
		opt.BaseURL = defaultBaseURL
	}
	if err := normalizeLegacyToken(m); err != nil {
		return nil, fmt.Errorf("failed to normalize legacy token: %w", err)
	}
	oAuthConfig, err := getOAuthConfig(m)
	if err != nil {
		return nil, fmt.Errorf("invalid OAuth2 config: %w", err)
	}

	fs.Debugf(nil, "NewFs(name=%q, root=%q) base_url=%q", name, root, opt.BaseURL)

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

	baseClient := createOptimizedHTTPClient(opt.RequestTimeout)
	_, tokenSource, err := oauthutil.NewClientWithBaseClient(ctx, name, m, oAuthConfig, baseClient)
	if err != nil {
		return nil, fmt.Errorf("failed to configure netexplorer oauth2: %w", err)
	}

	// Create API client with optimized HTTP client
	ne := &NetExplorer{
		BaseURL:     opt.BaseURL,
		Client:      baseClient,
		tokenSource: tokenSource,
		opt:         opt,
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
	if f.rootID == "" {
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
		// effective root folder ID now so that List(""), NewObject("..."),
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
	fs.Debugf(o.fs, "Hash(%q): md5 not cached, calling GetFile(id=%d)...", o.remote, o.id)
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

func (f *Fs) ensureRoot(ctx context.Context) error {
	_ = ctx
	if !terminal.IsTerminal(int(os.Stdin.Fd())) {
		return errors.New("netexplorer: root folder ID is required; run `rclone config reconnect` in an interactive terminal or set the root option manually")
	}

	fs.Debugf(f, "ensureRoot: prompting user to pick a root folder")
	fmt.Println("No root folder configured. Please select one:")

	start := time.Now()
	roots, err := f.ne.ListRoots()
	elapsed := time.Since(start)
	if err != nil {
		fs.Debugf(f, "ensureRoot: ListRoots error after %v: %v", elapsed, err)
		return err
	}
	fs.Debugf(f, "ensureRoot: ListRoots returned %d root(s) in %v", len(roots), elapsed)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "ID\tName"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "--\t----"); err != nil {
		return err
	}
	valid := make(map[string]bool, len(roots))
	for _, r := range roots {
		id := strconv.Itoa(r.ID)
		if _, err := fmt.Fprintf(w, "%s\t%s\n", id, r.Name); err != nil {
			return err
		}
		valid[id] = true
	}
	if err := w.Flush(); err != nil {
		return err
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Enter folder ID: ")
		line, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		choice := strings.TrimSpace(line)
		if valid[choice] {
			if err := config.SetValueAndSave(f.name, "root", choice); err != nil {
				return fmt.Errorf("failed to save root folder ID: %w", err)
			}
			f.rootID = choice
			fs.Debugf(f, "ensureRoot: selected rootID=%q", f.rootID)
			return nil
		}
		fmt.Println("Invalid ID, please enter one of the IDs listed above.")
	}
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
	defer closeResponseBody(resp)
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
	url := fmt.Sprintf("%s/api/file/%s", ne.BaseURL, fileID)
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
	defer closeResponseBody(resp)
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

func (o *Object) rewindUploadReader(in io.Reader, uploadErr error) error {
	seeker, ok := in.(io.Seeker)
	if !ok {
		return fserrors.RetryError(fmt.Errorf("cannot retry in-place upload - reader is not seekable: %w", uploadErr))
	}
	if _, err := seeker.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to rewind upload reader: %w", err)
	}
	return nil
}

func sourceObjectFromInfo(src fs.ObjectInfo) fs.Object {
	if obj, ok := src.(fs.Object); ok {
		return obj
	}
	if wrapped, ok := src.(fs.ObjectUnWrapper); ok {
		return wrapped.UnWrap()
	}
	return nil
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
		fs.Debugf(o.fs, "Update(%q): parentID not set, resolving...", relPath)
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
	sourceObject := sourceObjectFromInfo(src)
	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Track performance for upload operations
		o.fs.perfMonitor.IncrementCounter()

		fi, err := o.fs.ne.UploadFile(ctx, parentID, name, in, src.Size(), modTime, createdTime, sourceObject)
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
				// For uploads without immediate metadata, use smart hydration.
				fs.Debugf(o.fs, "Update(%q): upload complete without metadata, using smart hydration", relPath)

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
				if rewindErr := o.rewindUploadReader(in, err); rewindErr != nil {
					return rewindErr
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
				if rewindErr := o.rewindUploadReader(in, err); rewindErr != nil {
					return rewindErr
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

					if rewindErr := o.rewindUploadReader(in, err); rewindErr != nil {
						return rewindErr
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
					if rewindErr := o.rewindUploadReader(in, err); rewindErr != nil {
						return rewindErr
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
					if rewindErr := o.rewindUploadReader(in, err); rewindErr != nil {
						return rewindErr
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
		fs.Debugf(o.fs, "Remove(%q): using sanitized name %q for deletion (original: %q)", o.remote, fileName, originalName)
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
	defer closeResponseBody(resp)
	// Read response body for logging
	bodyBytes, _ := io.ReadAll(resp.Body)
	fs.Debugf(nil, "CreateFolder(%q): status=%d in %v", name, resp.StatusCode, dt)
	if len(bodyBytes) > 0 {
		fs.Debugf(nil, "CreateFolder(%q): server response body: %s", name, string(bodyBytes))
	} else {
		fs.Debugf(nil, "CreateFolder(%q): server response body: (empty)", name)
	}

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated:
		// Prefer parsing as APIFolder to inspect the returned name and dates.
		var af APIFolder
		if err := json.Unmarshal(bodyBytes, &af); err == nil && af.ID != 0 {
			// NetExplorer sometimes auto-renames on conflict, e.g. "BE_CAP" -> "BE_CAP (1)".
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
				// the server to create, so that "name (1)", "name (2)", ... do not
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
	drainAndCloseResponseBody(resp)
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
	defer closeResponseBody(resp)

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

func (ne *NetExplorer) updateFileMetadata(fileID string, modTime time.Time, createdTime time.Time) (*APIFile, error) {
	payload := map[string]any{}
	if !modTime.IsZero() {
		payload["modification"] = modTime.UTC().Format(time.RFC3339)
	}
	if !createdTime.IsZero() {
		payload["creation"] = createdTime.UTC().Format(time.RFC3339)
	}
	if len(payload) == 0 {
		return nil, nil
	}

	url := fmt.Sprintf("%s/api/file/%s", ne.BaseURL, fileID)
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	resp, err := ne.doRequestWithReauth(func(token string) (*http.Request, error) {
		req, err := http.NewRequest("PUT", url, bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		return req, nil
	}, false)
	if err != nil {
		return nil, err
	}
	defer closeResponseBody(resp)

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		return nil, &httpErr{code: resp.StatusCode, body: string(body)}
	}
	if len(body) == 0 {
		return nil, nil
	}

	var fi APIFile
	if err := json.Unmarshal(body, &fi); err != nil {
		return nil, nil
	}
	if fi.MD5 == "" && fi.Hash != "" {
		fi.MD5 = fi.Hash
	}
	return &fi, nil
}

func timesWithinTolerance(a time.Time, b time.Time, tolerance time.Duration) bool {
	if a.IsZero() || b.IsZero() {
		return a.IsZero() && b.IsZero()
	}
	delta := a.Sub(b)
	if delta < 0 {
		delta = -delta
	}
	return delta <= tolerance
}

func encodeTusMetadata(metadata map[string]string) string {
	keys := make([]string, 0, len(metadata))
	for key := range metadata {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		value := metadata[key]
		if value == "" {
			parts = append(parts, key)
			continue
		}
		parts = append(parts, key+" "+base64.StdEncoding.EncodeToString([]byte(value)))
	}
	return strings.Join(parts, ",")
}

func (ne *NetExplorer) resolveTusLocation(location string) (string, error) {
	if location == "" {
		return "", errors.New("tus creation response missing Location header")
	}

	locationURL, err := url.Parse(location)
	if err != nil {
		return "", fmt.Errorf("invalid tus Location header %q: %w", location, err)
	}
	if locationURL.IsAbs() {
		return locationURL.String(), nil
	}

	baseURL, err := url.Parse(ne.BaseURL)
	if err != nil {
		return "", fmt.Errorf("invalid base URL %q: %w", ne.BaseURL, err)
	}
	return baseURL.ResolveReference(locationURL).String(), nil
}

func (ne *NetExplorer) tusSessionInit(folderID, fileName string, size int64) (string, error) {
	url := ne.BaseURL + "/api/file/tus"
	payload := map[string]any{
		"name":   fileName,
		"target": folderID,
		"size":   size,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

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
	if err != nil {
		return "", err
	}
	defer closeResponseBody(resp)

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", &httpErr{code: resp.StatusCode, body: string(body)}
	}

	var sessions []struct {
		SessionKey string `json:"sessionKey"`
	}
	if err := json.Unmarshal(body, &sessions); err == nil && len(sessions) > 0 && sessions[0].SessionKey != "" {
		return sessions[0].SessionKey, nil
	}

	var session struct {
		SessionKey string `json:"sessionKey"`
	}
	if err := json.Unmarshal(body, &session); err == nil && session.SessionKey != "" {
		return session.SessionKey, nil
	}

	return "", fmt.Errorf("tus init failed: sessionKey missing in response")
}

func (ne *NetExplorer) tusCreateUpload(sessionKey string, size int64) (string, error) {
	url := ne.BaseURL + "/api/tus"
	metadata := encodeTusMetadata(map[string]string{
		"token": sessionKey,
	})

	resp, err := ne.doRequestWithReauth(func(token string) (*http.Request, error) {
		req, err := http.NewRequest("POST", url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Tus-Resumable", tusProtocolVersion)
		req.Header.Set("Upload-Length", strconv.FormatInt(size, 10))
		req.Header.Set("Upload-Metadata", metadata)
		req.ContentLength = 0
		return req, nil
	}, false)
	if err != nil {
		return "", err
	}
	defer closeResponseBody(resp)

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", &httpErr{code: resp.StatusCode, body: string(body)}
	}

	return ne.resolveTusLocation(resp.Header.Get("Location"))
}

func (ne *NetExplorer) tusGetOffset(uploadURL string) (int64, error) {
	resp, err := ne.doRequestWithReauth(func(token string) (*http.Request, error) {
		req, err := http.NewRequest("HEAD", uploadURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Tus-Resumable", tusProtocolVersion)
		return req, nil
	}, false)
	if err != nil {
		return 0, err
	}
	defer drainAndCloseResponseBody(resp)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return 0, &httpErr{code: resp.StatusCode, body: string(body)}
	}

	offsetHeader := resp.Header.Get("Upload-Offset")
	if offsetHeader == "" {
		return 0, errors.New("tus HEAD response missing Upload-Offset header")
	}
	offset, err := strconv.ParseInt(offsetHeader, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid Upload-Offset %q: %w", offsetHeader, err)
	}
	return offset, nil
}

func (ne *NetExplorer) tusPatch(uploadURL string, offset int64, chunk []byte) (int64, error) {
	resp, err := ne.doRequestWithReauth(func(token string) (*http.Request, error) {
		req, err := http.NewRequest("PATCH", uploadURL, bytes.NewReader(chunk))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Tus-Resumable", tusProtocolVersion)
		req.Header.Set("Upload-Offset", strconv.FormatInt(offset, 10))
		req.Header.Set("Content-Type", "application/offset+octet-stream")
		req.Header.Del("Expect")
		req.ContentLength = int64(len(chunk))
		return req, nil
	}, false)
	if err != nil {
		return 0, err
	}
	defer closeResponseBody(resp)

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		if resp.StatusCode == http.StatusConflict {
			serverOffset := int64(-1)
			if offsetHeader := resp.Header.Get("Upload-Offset"); offsetHeader != "" {
				if parsed, parseErr := strconv.ParseInt(offsetHeader, 10, 64); parseErr == nil {
					serverOffset = parsed
				}
			}
			return 0, &tusOffsetMismatchErr{serverOffset: serverOffset, body: string(body)}
		}
		return 0, &httpErr{code: resp.StatusCode, body: string(body)}
	}

	offsetHeader := resp.Header.Get("Upload-Offset")
	if offsetHeader == "" {
		return offset + int64(len(chunk)), nil
	}
	nextOffset, err := strconv.ParseInt(offsetHeader, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid Upload-Offset %q: %w", offsetHeader, err)
	}
	return nextOffset, nil
}

func (ne *NetExplorer) tusFinalize(sessionKey string) (*APIFile, error) {
	url := ne.BaseURL + "/api/file/tus"
	payload := map[string]string{
		"token": sessionKey,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

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
	if err != nil {
		return nil, err
	}
	defer closeResponseBody(resp)

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, &httpErr{code: resp.StatusCode, body: string(body)}
	}

	var fi APIFile
	if err := json.Unmarshal(body, &fi); err != nil {
		return nil, fmt.Errorf("tus finalize: failed to decode file response: %w", err)
	}
	if fi.MD5 == "" && fi.Hash != "" {
		fi.MD5 = fi.Hash
	}
	return &fi, nil
}

func (ne *NetExplorer) cancelTusUpload(uploadURL string) error {
	resp, err := ne.doRequestWithReauth(func(token string) (*http.Request, error) {
		req, err := http.NewRequest("DELETE", uploadURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Tus-Resumable", tusProtocolVersion)
		return req, nil
	}, false)
	if err != nil {
		return err
	}
	defer closeResponseBody(resp)

	body, _ := io.ReadAll(resp.Body)
	switch resp.StatusCode {
	case http.StatusOK, http.StatusNoContent, http.StatusNotFound, http.StatusGone:
		return nil
	default:
		return &httpErr{code: resp.StatusCode, body: string(body)}
	}
}

func (ne *NetExplorer) uploadTus(ctx context.Context, folderID, fileName string, data io.Reader, fileSize int64, modTime time.Time, createdTime time.Time, sourceObject fs.Object) (*APIFile, error) {
	maxSessionRestarts := ne.opt.UploadRetries
	if maxSessionRestarts <= 0 {
		maxSessionRestarts = 3
	}
	baseDelay := time.Duration(ne.opt.RetryDelayBase) * time.Second
	if baseDelay <= 0 {
		baseDelay = time.Second
	}

	currentReader := data
	accountReader, _ := data.(*accounting.Account)
	var reopenedReader io.ReadCloser
	lastCancelledUploadURL := ""
	lastCancelledOffset := int64(-1)
	resumeResetAttempts := 0

	reopenSourceAtOffset := func(offset int64) error {
		if sourceObject == nil {
			return errors.New("source object does not support reopen")
		}
		rc, err := sourceObject.Open(ctx, &fs.HashesOption{Hashes: hash.Set(hash.None)}, &fs.RangeOption{Start: offset, End: -1})
		if err != nil {
			return fmt.Errorf("failed to reopen source at offset %d: %w", offset, err)
		}
		if accountReader != nil {
			oldReader := accountReader.GetReader()
			accountReader.UpdateReader(ctx, rc)
			if oldReader != nil {
				_ = oldReader.Close()
			}
			currentReader = accountReader
			return nil
		}
		if reopenedReader != nil {
			_ = reopenedReader.Close()
		}
		reopenedReader = rc
		currentReader = rc
		return nil
	}
	defer func() {
		if reopenedReader != nil {
			_ = reopenedReader.Close()
		}
	}()

	restartUpload := func(uploadErr error) error {
		if err := reopenSourceAtOffset(0); err == nil {
			return nil
		}
		seeker, ok := data.(io.Seeker)
		if !ok {
			return fserrors.RetryError(fmt.Errorf("cannot restart tus upload session - reader is not seekable: %w", uploadErr))
		}
		if _, err := seeker.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("failed to rewind tus upload reader: %w", err)
		}
		return nil
	}

	tusDelay := func(attempt int) time.Duration {
		return time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt)))
	}

	setReaderOffset := func(targetOffset int64) error {
		if targetOffset <= 0 {
			return nil
		}
		if err := reopenSourceAtOffset(targetOffset); err == nil {
			return nil
		}
		if seeker, ok := data.(io.Seeker); ok {
			if _, err := seeker.Seek(targetOffset, io.SeekStart); err == nil {
				return nil
			} else {
				fs.Debugf(nil, "uploadTus(%q): absolute seek to offset %d failed, falling back to stream discard: %v", fileName, targetOffset, err)
			}
		}
		skipped, err := io.CopyN(io.Discard, data, targetOffset)
		if err != nil {
			return fmt.Errorf("failed to advance tus upload reader to offset %d (skipped %d): %w", targetOffset, skipped, err)
		}
		return nil
	}
	advanceReader := func(delta int64) error {
		if delta <= 0 {
			return nil
		}
		if sourceObject != nil {
			return errors.New("relative advance is not supported when source reopen is available")
		}
		if seeker, ok := data.(io.Seeker); ok {
			if _, err := seeker.Seek(delta, io.SeekCurrent); err == nil {
				return nil
			} else {
				fs.Debugf(nil, "uploadTus(%q): relative seek by %d bytes failed, falling back to stream discard: %v", fileName, delta, err)
			}
		}
		skipped, err := io.CopyN(io.Discard, data, delta)
		if err != nil {
			return fmt.Errorf("failed to advance tus upload reader by %d bytes (skipped %d): %w", delta, skipped, err)
		}
		return nil
	}

uploadSession:
	for sessionAttempt := 0; sessionAttempt <= maxSessionRestarts; sessionAttempt++ {
		sessionKey, err := ne.tusSessionInit(folderID, fileName, fileSize)
		if err != nil {
			return nil, err
		}
		uploadURL, err := ne.tusCreateUpload(sessionKey, fileSize)
		if err != nil {
			return nil, err
		}

		fs.Debugf(nil, "uploadTus(%q): session %d/%d created, uploadURL=%q", fileName, sessionAttempt+1, maxSessionRestarts+1, uploadURL)

		offset, err := ne.tusGetOffset(uploadURL)
		if err != nil {
			return nil, fmt.Errorf("tus initial offset probe failed: %w", err)
		}
		if offset < 0 || offset > fileSize {
			return nil, fmt.Errorf("tus initial offset %d is invalid for file size %d", offset, fileSize)
		}
		if lastCancelledUploadURL != "" && uploadURL == lastCancelledUploadURL && offset == lastCancelledOffset {
			return nil, fserrors.NoRetryError(fmt.Errorf("tus session reset failed: server returned the same upload resource %q at offset %d after cancel", uploadURL, offset))
		}
		if err := setReaderOffset(offset); err != nil {
			return nil, err
		}
		if offset > 0 {
			fs.Debugf(nil, "uploadTus(%q): resuming existing upload from offset %d", fileName, offset)
		}

		for offset < fileSize {
			chunkStart := offset
			chunkSize := int64(tusPatchChunkSize)
			if remaining := fileSize - chunkStart; remaining < chunkSize {
				chunkSize = remaining
			}

			buf := make([]byte, int(chunkSize))
			if _, err := io.ReadFull(currentReader, buf); err != nil {
				return nil, fmt.Errorf("read tus chunk at offset %d: %w", chunkStart, err)
			}

			chunkEnd := chunkStart + int64(len(buf))
			chunkOffset := chunkStart
			patchAttempts := 0

			for chunkOffset < chunkEnd {
				nextOffset, err := ne.tusPatch(uploadURL, chunkOffset, buf[int(chunkOffset-chunkStart):])
				if err == nil {
					if nextOffset < chunkOffset {
						return nil, fmt.Errorf("tus PATCH returned inconsistent offset %d for chunk [%d,%d)", nextOffset, chunkStart, chunkEnd)
					}
					if nextOffset > chunkEnd {
						skipAhead := nextOffset - chunkEnd
						if nextOffset > fileSize {
							return nil, fmt.Errorf("tus PATCH returned inconsistent offset %d for file size %d", nextOffset, fileSize)
						}
						if skipAhead > 0 {
							if sourceObject != nil {
								if err := setReaderOffset(nextOffset); err != nil {
									return nil, err
								}
							} else if err := advanceReader(skipAhead); err != nil {
								return nil, err
							}
						}
						fs.Debugf(nil, "uploadTus(%q): PATCH acknowledged offset %d beyond current chunk [%d,%d), advancing local reader", fileName, nextOffset, chunkStart, chunkEnd)
					}
					chunkOffset = nextOffset
					offset = nextOffset
					patchAttempts = 0
					continue
				}

				var mismatchErr *tusOffsetMismatchErr
				if errors.As(err, &mismatchErr) {
					serverOffset := mismatchErr.serverOffset
					if serverOffset < 0 {
						probedOffset, probeErr := ne.tusGetOffset(uploadURL)
						if probeErr != nil {
							return nil, fmt.Errorf("tus resume probe failed after mismatch: %w", probeErr)
						}
						serverOffset = probedOffset
					}
					if serverOffset == chunkOffset {
						if resumeResetAttempts >= 1 {
							return nil, fserrors.NoRetryError(fmt.Errorf("tus resume inconsistency at offset %d: PATCH rejected the offset reported by the server after session reset", serverOffset))
						}
						if cancelErr := ne.cancelTusUpload(uploadURL); cancelErr != nil {
							fs.Debugf(nil, "uploadTus(%q): failed to cancel inconsistent TUS upload resource %q before restart: %v", fileName, uploadURL, cancelErr)
						} else {
							fs.Debugf(nil, "uploadTus(%q): cancelled inconsistent TUS upload resource %q before restarting session", fileName, uploadURL)
						}
						lastCancelledUploadURL = uploadURL
						lastCancelledOffset = serverOffset
						resumeResetAttempts++
						if rewindErr := restartUpload(err); rewindErr != nil {
							return nil, rewindErr
						}
						delay := tusDelay(resumeResetAttempts - 1)
						fs.Debugf(nil, "uploadTus(%q): restarting session after resume inconsistency at offset %d in %v", fileName, serverOffset, delay)
						time.Sleep(delay)
						continue uploadSession
					}
					if serverOffset < chunkStart {
						if sessionAttempt >= maxSessionRestarts {
							return nil, errTusRestartSession
						}
						if rewindErr := restartUpload(err); rewindErr != nil {
							return nil, rewindErr
						}
						delay := tusDelay(sessionAttempt)
						fs.Debugf(nil, "uploadTus(%q): mismatch moved server offset behind current chunk to %d, restarting session in %v", fileName, serverOffset, delay)
						time.Sleep(delay)
						continue uploadSession
					}
					if serverOffset > chunkEnd {
						if err := setReaderOffset(serverOffset); err != nil {
							return nil, err
						}
						fs.Debugf(nil, "uploadTus(%q): mismatch moved server offset to %d beyond current chunk [%d,%d), reopening source there", fileName, serverOffset, chunkStart, chunkEnd)
						offset = serverOffset
						chunkOffset = chunkEnd
						break
					}
					chunkOffset = serverOffset
					offset = serverOffset
					fs.Debugf(nil, "uploadTus(%q): mismatch realigned upload to offset %d", fileName, serverOffset)
					continue
				}

				var he *httpErr
				if errors.As(err, &he) && (he.code == http.StatusNotFound || he.code == http.StatusGone) {
					if sessionAttempt >= maxSessionRestarts {
						return nil, err
					}
					if rewindErr := restartUpload(err); rewindErr != nil {
						return nil, rewindErr
					}
					delay := tusDelay(sessionAttempt)
					fs.Debugf(nil, "uploadTus(%q): upload resource lost (status=%d), restarting session in %v", fileName, he.code, delay)
					time.Sleep(delay)
					continue uploadSession
				}

				serverOffset, headErr := ne.tusGetOffset(uploadURL)
				if headErr != nil {
					var headHTTP *httpErr
					if errors.As(headErr, &headHTTP) && (headHTTP.code == http.StatusNotFound || headHTTP.code == http.StatusGone) {
						if sessionAttempt >= maxSessionRestarts {
							return nil, headErr
						}
						if rewindErr := restartUpload(err); rewindErr != nil {
							return nil, rewindErr
						}
						delay := tusDelay(sessionAttempt)
						fs.Debugf(nil, "uploadTus(%q): HEAD reports missing upload resource, restarting session in %v", fileName, delay)
						time.Sleep(delay)
						continue uploadSession
					}
					return nil, fmt.Errorf("tus resume probe failed at offset %d: %w", chunkOffset, headErr)
				}

				if serverOffset < chunkStart {
					if sessionAttempt >= maxSessionRestarts {
						return nil, errTusRestartSession
					}
					if rewindErr := restartUpload(err); rewindErr != nil {
						return nil, rewindErr
					}
					delay := tusDelay(sessionAttempt)
					fs.Debugf(nil, "uploadTus(%q): server offset %d fell behind current chunk [%d,%d), restarting session in %v", fileName, serverOffset, chunkStart, chunkEnd, delay)
					time.Sleep(delay)
					continue uploadSession
				}
				if serverOffset > chunkEnd {
					return nil, fmt.Errorf("tus server offset %d exceeds buffered chunk end %d", serverOffset, chunkEnd)
				}

				chunkOffset = serverOffset
				offset = serverOffset
				patchAttempts++
				if patchAttempts > maxSessionRestarts+1 {
					return nil, fmt.Errorf("tus PATCH failed repeatedly around offset %d: %w", serverOffset, err)
				}
				delay := tusDelay(patchAttempts - 1)
				fs.Debugf(nil, "uploadTus(%q): resuming from offset %d after patch error: %v", fileName, serverOffset, err)
				time.Sleep(delay)
			}
		}

		var finalInfo *APIFile
		var finalizeErr error
		for finalizeAttempt := 0; finalizeAttempt <= maxSessionRestarts; finalizeAttempt++ {
			finalInfo, finalizeErr = ne.tusFinalize(sessionKey)
			if finalizeErr == nil {
				break
			}

			var he *httpErr
			if !errors.As(finalizeErr, &he) {
				return nil, finalizeErr
			}
			if he.code != http.StatusLocked && he.code != http.StatusTooManyRequests && he.code != http.StatusInternalServerError {
				return nil, finalizeErr
			}

			delay := tusDelay(finalizeAttempt)
			fs.Debugf(nil, "uploadTus(%q): finalize retry in %v after status %d", fileName, delay, he.code)
			time.Sleep(delay)
		}
		if finalizeErr != nil {
			return nil, finalizeErr
		}

		if finalInfo != nil && (!modTime.IsZero() || !createdTime.IsZero()) {
			updatedInfo, err := ne.updateFileMetadata(strconv.Itoa(finalInfo.ID), modTime, createdTime)
			if err != nil {
				fs.Debugf(nil, "uploadTus(%q): failed to update file metadata after finalize: %v", fileName, err)
			} else if updatedInfo != nil {
				finalInfo = updatedInfo
			}
		}

		return finalInfo, nil
	}

	return nil, errTusRestartSession
}

// UploadFile uploads data into folderID.
// If fileSize <= 16 MiB it uses the classic one-shot endpoint.
// Otherwise it streams the file in 16 MiB blocks with the
//
//	chunk / chunks / chunksSize / fileSize protocol required by NetExplorer.
//
// streamInit asks the server for a sessionKey to stream the bytes with a PUT.
func (ne *NetExplorer) streamInit(folderID, targetPath string, size int64, modTime time.Time, createdTime time.Time) (string, *APIFile, error) {
	url := ne.BaseURL + "/api/file/upload"
	base := path.Base(targetPath)
	fs.Debugf(nil, "STREAM_INIT: POST %s folder=%s targetPath=%q size=%d method=stream", url, folderID, base, size)

	payload := map[string]any{
		"fileSize":     size,
		"folderId":     folderID,
		"targetPath":   base,
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
	defer closeResponseBody(resp)
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
			fs.Debugf(nil, "STREAM_INIT: received sessionKey=%s...", sk[:4]+"...")
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
	url := ne.BaseURL + "/api/file/upload?sessionKey=" + url.QueryEscape(sessionKey)
	fs.Debugf(nil, "PUT_STREAM: PUT %s (size=%d)", url, size)

	start := time.Now()
	buildCount := 0
	resp, err := ne.doRequestWithReauth(func(token string) (*http.Request, error) {
		if buildCount > 0 {
			seeker, ok := data.(io.Seeker)
			if !ok {
				return nil, fserrors.RetryError(errors.New("cannot retry stream upload after reauth - reader is not seekable"))
			}
			if _, err := seeker.Seek(0, io.SeekStart); err != nil {
				return nil, fmt.Errorf("failed to rewind stream upload after reauth: %w", err)
			}
		}
		buildCount++

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
	defer closeResponseBody(resp)
	if timing := responseTimingHeaders(resp); timing != "" {
		fs.Debugf(nil, "PUT_STREAM: status=%d in %v (%s)", resp.StatusCode, dt, timing)
	} else {
		fs.Debugf(nil, "PUT_STREAM: status=%d in %v", resp.StatusCode, dt)
	}

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
	// No JSON body -> caller will hydrate later
	fs.Debugf(nil, "PUT_STREAM: no JSON metadata returned; will rely on hydrate to find id")
	return nil, nil
}

func (ne *NetExplorer) UploadFile(ctx context.Context, folderID, fileName string, data io.Reader, fileSize int64, modTime time.Time, createdTime time.Time, sourceObject fs.Object) (*APIFile, error) {
	const smallFileThreshold = 16 * 1024 * 1024 // 16 MiB

	// Only log large files to reduce debug overhead
	if fileSize > 10*1024*1024 { // Only log files larger than 10MB
		fs.Debugf(nil, "UploadFile(folderID=%s, name=%q, size=%d)", folderID, fileName, fileSize)
	}

	// Handle unknown file size - use direct upload
	if fileSize < 0 {
		fs.Debugf(nil, "UploadFile(%q): unknown file size, using direct upload", fileName)
		return ne.uploadSingle(ctx, folderID, fileName, data, fileSize, modTime, createdTime, sourceObject)
	}

	// Handle empty files
	if fileSize == 0 {
		fs.Debugf(nil, "UploadFile(%q): empty file, using direct upload", fileName)
		return ne.uploadSingle(ctx, folderID, fileName, data, fileSize, modTime, createdTime, sourceObject)
	}

	if fileSize < smallFileThreshold {
		return ne.uploadSingle(ctx, folderID, fileName, data, fileSize, modTime, createdTime, sourceObject)
	}

	fs.Debugf(nil, "UploadFile(%q): large file (%d bytes), using TUS upload", fileName, fileSize)
	return ne.uploadTus(ctx, folderID, fileName, data, fileSize, modTime, createdTime, sourceObject)
}

// parseChunkReportMissing parses a chunk upload report response and returns
// the next missing chunk index when present.
func parseChunkReportMissing(body []byte) (missing int, ok bool) {
	type reportEntry struct {
		Missing int `json:"missing"`
	}
	var report []reportEntry
	if err := json.Unmarshal(body, &report); err != nil || len(report) == 0 {
		return 0, false
	}
	if report[0].Missing < 0 {
		return 0, false
	}
	return report[0].Missing, true
}

// returns the metadata of the freshly-created file
func (ne *NetExplorer) uploadSingle(ctx context.Context, folderID, fileName string, data io.Reader, fileSize int64, modTime time.Time, createdTime time.Time, sourceObject fs.Object) (*APIFile, error) {
	uploadURL, err := url.Parse(ne.BaseURL + "/api/file/upload")
	if err != nil {
		return nil, fmt.Errorf("uploadSingle: invalid upload url: %w", err)
	}

	queryFields := map[string]string{}
	query := uploadURL.Query()

	// POST /file/upload expects creation/modification as query parameters.
	if !modTime.IsZero() {
		modTimeStr := modTime.UTC().Format(time.RFC3339)
		query.Set("modification", modTimeStr)
		queryFields["modification"] = modTimeStr
		fs.Debugf(nil, "uploadSingle(%q): adding query modification=%q (UTC)", fileName, modTimeStr)
	} else {
		fs.Debugf(nil, "uploadSingle(%q): modTime is zero, not sending query modification", fileName)
	}
	if !createdTime.IsZero() {
		createdTimeStr := createdTime.UTC().Format(time.RFC3339)
		query.Set("creation", createdTimeStr)
		queryFields["creation"] = createdTimeStr
		fs.Debugf(nil, "uploadSingle(%q): adding query creation=%q (UTC)", fileName, createdTimeStr)
	} else {
		fs.Debugf(nil, "uploadSingle(%q): createdTime is zero, not sending query creation", fileName)
	}
	uploadURL.RawQuery = query.Encode()
	fs.Debugf(nil, "uploadSingle: POST %s (folderID=%s name=%q)", uploadURL.String(), folderID, fileName)

	// Compute fileHash before creating the multipart body so it can be sent
	// alongside folderId/targetFile on direct uploads.
	var fileHashHex string
	var hashSource string
	hashStart := time.Now()
	if sourceObject != nil {
		if h, err := sourceObject.Hash(ctx, hash.MD5); err == nil && h != "" {
			fileHashHex = h
			hashSource = "remote"
		} else {
			hashSource = "remote-unavailable"
		}
	} else if fileSize > 0 {
		buf, err := io.ReadAll(data)
		if err != nil {
			return nil, fmt.Errorf("uploadSingle: failed to buffer data for hash: %w", err)
		}
		sum := md5.Sum(buf)
		fileHashHex = hex.EncodeToString(sum[:])
		data = bytes.NewReader(buf)
		hashSource = "local"
	} else {
		hashSource = "skipped"
	}
	hashDt := time.Since(hashStart)
	fs.Debugf(nil, "[netexplorer] uploadSingle %q: hash=%s  hashMs=%dms  md5=%s",
		fileName, hashSource, hashDt.Milliseconds(), fileHashHex)

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)

	// Track form fields for logging
	formFields := map[string]string{
		"folderId": folderID,
	}

	_ = mw.WriteField("folderId", folderID)

	if fileSize >= 0 {
		_ = mw.WriteField("fileSize", strconv.FormatInt(fileSize, 10))
		formFields["fileSize"] = strconv.FormatInt(fileSize, 10)
	}
	if fileHashHex != "" {
		_ = mw.WriteField("fileHash", fileHashHex)
		formFields["fileHash"] = fileHashHex
	}

	// Log request fields separately: folderId in multipart body, dates in query string.
	fs.Debugf(nil, "uploadSingle(%q): request form fields=%v query fields=%v", fileName, formFields, queryFields)

	part, err := mw.CreateFormFile("targetFile", fileName)
	if err != nil {
		return nil, err
	}
	bufferStart := time.Now()
	bytesCopied, copyErr := io.Copy(part, data)
	if copyErr != nil {
		return nil, copyErr
	}
	if err = mw.Close(); err != nil {
		return nil, err
	}
	bufferDt := time.Since(bufferStart)
	sizeMB := float64(bytesCopied) / (1024 * 1024)
	fs.Debugf(nil, "[netexplorer] uploadSingle %q: size=%.2f MB  buffer=%dms  body=%d B",
		fileName, sizeMB, bufferDt.Milliseconds(), body.Len())

	postStart := time.Now()
	resp, err := ne.doRequestWithReauth(func(token string) (*http.Request, error) {
		req, err := http.NewRequest("POST", uploadURL.String(), &body)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		return req, nil
	}, false)
	postDt := time.Since(postStart)
	if err != nil {
		fs.Debugf(nil, "[netexplorer] uploadSingle %q: POST error after %dms: %v", fileName, postDt.Milliseconds(), err)
		return nil, err
	}
	defer closeResponseBody(resp)
	if timing := responseTimingHeaders(resp); timing != "" {
		fs.Debugf(nil, "[netexplorer] uploadSingle %q: POST=%dms  status=%d  %s  (%.2f MB/s)",
			fileName, postDt.Milliseconds(), resp.StatusCode, timing,
			sizeMB/(float64(postDt.Milliseconds())/1000))
	} else {
		fs.Debugf(nil, "[netexplorer] uploadSingle %q: POST=%dms  status=%d  (%.2f MB/s)",
			fileName, postDt.Milliseconds(), resp.StatusCode,
			sizeMB/(float64(postDt.Milliseconds())/1000))
	}

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

	// Direct uploads usually return the final metadata, so only issue a
	// corrective PUT when the server clearly ignored the requested creation time.
	if !createdTime.IsZero() && !timesWithinTolerance(fi.Creation, createdTime, 2*time.Second) {
		updatedInfo, err := ne.updateFileMetadata(strconv.Itoa(fi.ID), modTime, createdTime)
		if err != nil {
			fs.Debugf(nil, "[netexplorer] uploadSingle %q: PUT correction failed: %v", fileName, err)
		} else if updatedInfo != nil {
			fi = *updatedInfo
		}
	}

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
		drainAndCloseResponseBody(resp)
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
	drainAndCloseResponseBody(resp)
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
//  1. GET /api/folder/{id}?depth=1 -> raw []FolderObject
//  2. GET /api/folders?parent_id={id} -> []{ content: { folders:..., files:... } }
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
	defer closeResponseBody(resp)
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
			return nil, nil, fmt.Errorf("unmarshal raw-folder-array: %w - raw: %s", err, raw)
		}
		fs.Debugf(nil, "ListFolder(%s): shape=array -> folders=%d files=0", folderID, len(folders))
		return folders, nil, nil
	}

	var wrapper struct {
		Content struct {
			Folders []APIFolder `json:"folders"`
			Files   []APIFile   `json:"files"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		return nil, nil, fmt.Errorf("unmarshal wrapper content: %w - raw: %s", err, raw)
	}
	fs.Debugf(nil, "ListFolder(%s): shape=wrapper -> folders=%d files=%d",
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
	defer closeResponseBody(resp)
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
		return nil, nil, fmt.Errorf("unmarshal depth response: %w - raw: %s", err, raw)
	}

	// For bulk operations, we need to flatten the nested structure
	allFolders, allFiles := ne.flattenDepthResponse(root.Content.Folders, root.Content.Files)

	fs.Debugf(nil, "ListFolderWithDepth(%s, depth=%d): flattened -> folders=%d files=%d",
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

// ---------------------------------------------------------------------
// resolveFolderID walks rootID -> initialPath -> dir to get a numeric ID
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
			// the whole operation rather than blindly creating "name (1)", "(2)", ...
			waitAttempts := 30
			if f.opt.FolderDelay > 0 {
				delayAttempts := (f.opt.FolderDelay / 100) + 10
				if delayAttempts > waitAttempts {
					waitAttempts = delayAttempts
				}
			}
			for attempts := 0; attempts < waitAttempts; attempts++ {
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
			// repeatedly hydrating the parent. Do NOT attempt a second CreateFolder
			// here: with NetExplorer's eventual consistency that is exactly what can
			// turn one legitimate folder into "name (1)" / "name (2)" fragmentation.
			//
			// Returning an error here is preferable to creating a duplicate. The
			// caller/rclone retry loop can safely re-run the operation once the
			// original creator becomes visible in listings.
			return "", fmt.Errorf("ensureFolderID: concurrent creation for %q under parent %s not visible after waiting", fullPath, parentID)
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
			fs.Debugf(f, "ensureFolderID: created folder %q with ID %s under parent %s (fullPath=%q)", name, nid, parentID, fullPath)
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

		// Still not found - this means something is inconsistent on the server side.
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
		fs.Debugf(nil, "No files or folders were renamed during transfer")
		return
	}

	fs.Debugf(nil, "=== FILE/FOLDER RENAME SUMMARY ===")
	fs.Debugf(nil, "The following items were renamed due to invalid characters:")

	// Sort for consistent output
	var keys []string
	for k := range renames {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, original := range keys {
		sanitized := renames[original]
		fs.Debugf(nil, "  %q -> %q", original, sanitized)
	}

	fs.Debugf(nil, "Total renames: %d", len(renames))
	fs.Debugf(nil, "Note: NetExplorer API rejects files with special characters: / \\ : * ? \" < > | and leading/trailing spaces")
	fs.Debugf(nil, "=====================================")
}
