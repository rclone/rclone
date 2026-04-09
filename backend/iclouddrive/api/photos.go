package api

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/rclone/rclone/fs"

	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"

	"golang.org/x/text/unicode/norm"
)

const (
	// CloudKit returns CPLMaster+CPLAsset pairs, so 200 records = 100 photos
	// Server caps at 200 regardless of requested value (tested: 500-5000 all return 200)
	photosQueryLimit        = 200
	rootFolderRecord        = "----Root-Folder----"
	projectRootFolderRecord = "----Project-Root-Folder----"
	indexingStateReady      = "FINISHED"

	// cacheSubdir is the subdirectory under rclone's cache dir for all iCloud Photos state
	cacheSubdir = "iclouddrive-photos"

	// Album type constants from CloudKit CPLAlbum records
	albumTypeFolder = 3

	// CPLAsset.assetSubtype values
	subtypePanorama  = 1
	subtypeSloMo     = 100
	subtypeTimeLapse = 101

	// CPLAsset.assetSubtypeV2 values
	subtypeV2Live       = 2
	subtypeV2Screenshot = 3

	// CPLAsset.adjustmentRenderType bitmask values
	adjustPortrait     = 2
	adjustLongExposure = 4

	// CloudKit record type names
	recordTypeAlbum      = "CPLAlbumByPositionLive"
	recordTypeCountIndex = "HyperionIndexCountLookup"

	// CloudKit endpoint area names
	areaPrivate = "private"
	areaShared  = "shared"

	// Slo-mo adjustment type (metadata-only edit, no separate rendered resource)
	adjustSloMo = "com.apple.video.slomo"
)

// utiExtensions maps common Apple UTI descriptors to file extensions
// Used as fallback when filenameEnc is missing from a CPLMaster record
var utiExtensions = map[string]string{
	"public.jpeg":                 ".jpg",
	"public.png":                  ".png",
	"public.heic":                 ".heic",
	"public.heif":                 ".heif",
	"public.tiff":                 ".tiff",
	"public.mpeg-4":               ".mp4",
	"com.apple.quicktime-movie":   ".mov",
	"com.compuserve.gif":          ".gif",
	"com.adobe.raw-image":         ".dng",
	"com.canon.cr2-raw-image":     ".cr2",
	"com.canon.cr3-raw-image":     ".cr3",
	"com.nikon.raw-image":         ".nef",
	"com.sony.arw-raw-image":      ".arw",
	"public.avif":                 ".avif",
	"org.webmproject.webp":        ".webp",
	"public.mpeg-2-video":         ".m2v",
	"com.apple.m4v-video":         ".m4v",
	"public.avi":                  ".avi",
	"public.mp3":                  ".mp3",
	"com.apple.m4a-audio":         ".m4a",
	"public.image":                ".heic",
	"com.fuji.raw-image":          ".raf",
	"com.panasonic.rw2-raw-image": ".rw2",
	"com.olympus.raw-image":       ".orf",
	"com.pentax.raw-image":        ".pef",
	"com.nikon.nrw-raw-image":     ".nrw",
	"com.canon.crw-raw-image":     ".crw",
}

// extFromUTI returns the file extension for an Apple UTI string field,
// falling back to the provided default if the field is nil or unknown
func extFromUTI(field *ckStringField, fallback string) string {
	if field != nil {
		if mapped, ok := utiExtensions[field.Value]; ok {
			return mapped
		}
	}
	return fallback
}

// buildPhotoCache creates a filename-keyed map from a photo slice
func buildPhotoCache(photos []*Photo) map[string]*Photo {
	m := make(map[string]*Photo, len(photos))
	for _, p := range photos {
		if p.Filename != "" {
			m[p.Filename] = p
		}
	}
	return m
}

// saveJSONCache marshals v to JSON and writes it atomically to dir/filename
func saveJSONCache(dir, filename string, v any) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		fs.Debugf(nil, "iclouddrive: failed to create cache dir: %v", err)
		return
	}
	data, err := json.Marshal(v)
	if err != nil {
		fs.Debugf(nil, "iclouddrive: failed to marshal cache %s: %v", filename, err)
		return
	}
	if err := atomicWriteFile(filepath.Join(dir, filename), data); err != nil {
		fs.Debugf(nil, "iclouddrive: failed to write cache %s: %v", filename, err)
	}
}

// atomicWriteFile writes data to target via atomic temp+rename
func atomicWriteFile(target string, data []byte) error {
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// ShouldRetryFunc classifies whether an HTTP response/error is retryable
type ShouldRetryFunc func(ctx context.Context, resp *http.Response, err error) (bool, error)

// PhotosService manages iCloud Photos API interactions
type PhotosService struct {
	client      *Client
	endpoint    string
	pacer       *fs.Pacer
	shouldRetry ShouldRetryFunc
	mu          sync.Mutex
	libraries   map[string]*Library
}

type libraryDiscovery struct {
	libraries      map[string]*Library
	refreshedAreas map[string]bool
}

// FlushCaches clears all cached libraries, albums, and photos
func (ps *PhotosService) FlushCaches() {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	for _, lib := range ps.libraries {
		lib.deltaMu.Lock()
		lib.mu.Lock()
		lib.cacheValid.Store(false)
		lib.pendingDelta = nil
		lib.clearDiskCache()
		for _, album := range lib.albums {
			album.mu.Lock()
			album.photoCache = nil
			album.mu.Unlock()
		}
		lib.albums = make(map[string]*Album)
		lib.mu.Unlock()
		lib.deltaMu.Unlock()
	}
	ps.libraries = make(map[string]*Library)
	// Also remove the libraries cache file
	_ = os.Remove(filepath.Join(ps.client.CacheDir(), "libraries.json"))
}

// deltaPayload holds a buffered changes/zone response waiting to be applied
// after albums are populated
type deltaPayload struct {
	records    []json.RawMessage
	syncToken  string
	moreComing bool
}

// deltaContainsAlbumChanges checks if any record in the delta is a CPLAlbum
// (album create/rename/delete) requiring eager album cache invalidation
func deltaContainsAlbumChanges(records []json.RawMessage) bool {
	for _, raw := range records {
		var header struct {
			RecordType string `json:"recordType"`
		}
		if json.Unmarshal(raw, &header) == nil && header.RecordType == "CPLAlbum" {
			return true
		}
	}
	return false
}

// Library represents a photo library in a specific zone
type Library struct {
	service         *PhotosService
	zoneID          string
	area            string     // "private" or "shared" - determines API endpoint path
	ownerRecordName string     // zone owner's _UUID for full zoneID in requests
	zoneType        string     // "REGULAR_CUSTOM_ZONE" for full zoneID in requests
	mu              sync.Mutex // protects albums map
	albums          map[string]*Album
	deltaMu         sync.Mutex    // serializes delta checks+apply; lock order: ps.mu before deltaMu
	cacheValid      atomic.Bool   // true = album/photo disk caches are loadable
	pendingDelta    *deltaPayload // buffered delta waiting for albums to be populated, protected by deltaMu
	notifyToken     string        // separate changes/zone token for ChangeNotify polling (memory-only)
}

// zoneIDMap returns the full zoneID object for CloudKit API requests
func (lib *Library) zoneIDMap() map[string]any {
	m := map[string]any{"zoneName": lib.zoneID}
	if lib.ownerRecordName != "" {
		m["ownerRecordName"] = lib.ownerRecordName
	}
	if lib.zoneType != "" {
		m["zoneType"] = lib.zoneType
	}
	return m
}

// invalidateAlbumCache clears in-memory albums and removes disk cache
func (lib *Library) invalidateAlbumCache() {
	lib.mu.Lock()
	lib.albums = make(map[string]*Album)
	lib.mu.Unlock()
	_ = os.Remove(filepath.Join(lib.zoneCacheDir(), "albums.json"))
}

// bufferDelta stores a pending delta for later application, eagerly
// invalidating album cache if the delta contains CPLAlbum records
// Must be called under deltaMu
func (lib *Library) bufferDelta(records []json.RawMessage, syncToken string, moreComing bool) {
	lib.pendingDelta = &deltaPayload{records: records, syncToken: syncToken, moreComing: moreComing}
	lib.cacheValid.Store(true)
	fs.Debugf(nil, "iclouddrive photos: zone %s has pending changes, buffered for later application", lib.zoneID)
	if deltaContainsAlbumChanges(records) {
		lib.invalidateAlbumCache()
		fs.Debugf(nil, "iclouddrive photos: zone %s delta contains album changes, invalidated album cache", lib.zoneID)
	}
}

// request makes an API call routed to this library's area (private or shared)
func (lib *Library) request(ctx context.Context, endpoint string, data, response any) error {
	return lib.service.requestForArea(ctx, lib.area, endpoint, data, response)
}

func (lib *Library) isSharedLibrary() bool {
	return strings.HasPrefix(lib.zoneID, "SharedSync")
}

func isSharedAlbumIndexError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "HTTP error 400") &&
		strings.Contains(msg, "BAD_REQUEST") &&
		strings.Contains(msg, "Index has invalid data")
}

func isZoneNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "HTTP error 404") &&
		strings.Contains(msg, "ZONE_NOT_FOUND") &&
		strings.Contains(msg, "Zone does not exist")
}

func (lib *Library) probeZoneExists(ctx context.Context) (bool, error) {
	query := map[string]any{
		"query": map[string]any{
			"recordType": "CPLAssetAndMasterByAssetDateWithoutHiddenOrDeleted",
			"filterBy": []map[string]any{
				{
					"fieldName":  "startRank",
					"comparator": "EQUALS",
					"fieldValue": map[string]any{"type": "INT64", "value": 0},
				},
				{
					"fieldName":  "direction",
					"comparator": "EQUALS",
					"fieldValue": map[string]any{"type": "STRING", "value": "ASCENDING"},
				},
			},
		},
		"resultsLimit": 1,
		"desiredKeys":  []string{"masterRef"},
		"zoneID":       lib.zoneIDMap(),
	}
	var response struct {
		Records []json.RawMessage `json:"records"`
	}
	if err := lib.request(ctx, "records/query", query, &response); err != nil {
		if isZoneNotFoundError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// newUserAlbum creates a standard user album with CPLContainerRelation query config
func (lib *Library) newUserAlbum(name, recordName string) *Album {
	return &Album{
		Name:       name,
		ObjectType: fmt.Sprintf("CPLContainerRelationNotDeletedByAssetDate:%s", recordName),
		ListType:   "CPLContainerRelationLiveByAssetDate",
		Direction:  "ASCENDING",
		RecordName: recordName,
		lib:        lib,
		Filters: []Filter{{
			FieldName:  "parentId",
			Comparator: "EQUALS",
			FieldValue: map[string]string{"type": "STRING", "value": recordName},
		}},
	}
}

// Album represents a photo album with its metadata and query configuration
type Album struct {
	Name       string            `json:"name"`
	ObjectType string            `json:"objectType"`
	ListType   string            `json:"listType"`
	Direction  string            `json:"direction"`
	Filters    []Filter          `json:"filters,omitempty"`
	RecordName string            `json:"recordName,omitempty"`
	IsFolder   bool              `json:"isFolder,omitempty"`
	Children   map[string]*Album `json:"children,omitempty"`
	lib        *Library          `json:"-"`
	mu         sync.Mutex        `json:"-"`
	photoCache map[string]*Photo `json:"-"`
}

// Photo represents a photo or video with its metadata
type Photo struct {
	ID          string
	Filename    string
	Size        int64
	AssetDate   int64 // Unix timestamp in milliseconds
	AddedDate   int64
	Width       int
	Height      int
	IsFavorite  bool
	IsHidden    bool
	SmartAlbums []string // smart album names this photo belongs to (for delta sync routing)
	ResourceKey string   // CloudKit field name for download (default: resOriginalRes)
}

// companion creates a derivative Photo entry sharing metadata with the parent
// Used for Live Photo MOV, edited versions, and RAW alternatives
func (p *Photo) companion(id, filename, resourceKey string, size int64) *Photo {
	return &Photo{
		ID:          id,
		Filename:    filename,
		Size:        size,
		AssetDate:   p.AssetDate,
		AddedDate:   p.AddedDate,
		IsFavorite:  p.IsFavorite,
		IsHidden:    p.IsHidden,
		ResourceKey: resourceKey,
		SmartAlbums: p.SmartAlbums,
	}
}

// Filter represents a CloudKit query filter
type Filter struct {
	FieldName  string `json:"fieldName"`
	Comparator string `json:"comparator"`
	FieldValue any    `json:"fieldValue"`
}

// smartAlbumFilter defines a filter-based smart album generated from a short table
// Each entry maps: display name -> ObjectType suffix + filter tag
// All share ListType "CPLAssetAndMasterInSmartAlbumByAssetDate", direction ASCENDING,
// and a single smartAlbum EQUALS filter
type smartAlbumFilter struct {
	suffix string // appended to "CPLAssetInSmartAlbumByAssetDate:"
	tag    string // smartAlbum filter value (e.g. "VIDEO", "FAVORITE")
}

// smartAlbumFilters is the data table for the 10 filter-based smart albums
var smartAlbumFilters = map[string]smartAlbumFilter{
	"Time-lapse":    {suffix: "Timelapse", tag: "TIMELAPSE"},
	"Videos":        {suffix: "Video", tag: "VIDEO"},
	"Slo-mo":        {suffix: "Slomo", tag: "SLOMO"},
	"Favorites":     {suffix: "Favorite", tag: "FAVORITE"},
	"Panoramas":     {suffix: "Panorama", tag: "PANORAMA"},
	"Screenshots":   {suffix: "Screenshot", tag: "SCREENSHOT"},
	"Live":          {suffix: "Live", tag: "LIVE"},
	"Portrait":      {suffix: "Depth", tag: "DEPTH"},
	"Long Exposure": {suffix: "Exposure", tag: "EXPOSURE"},
	"Animated":      {suffix: "Animated", tag: "ANIMATED"},
	// SELFIE filter exists server-side but the index is never populated via web API -
	// selfie classification is on-device only (iOS reads LensModel EXIF for "front camera")
	// Apple's own icloud.com doesn't show it either - omitted to avoid an always-empty album
}

// SmartAlbums defines the built-in smart album types available in iCloud Photos
// 10 filter-based albums are generated from smartAlbumFilters; 4 special albums
// (All Photos, Bursts, Hidden, Recently Deleted) use unique recordTypes
var SmartAlbums = buildSmartAlbums()

func buildSmartAlbums() map[string]*Album {
	albums := map[string]*Album{
		"All Photos": {
			Name:       "All Photos",
			ObjectType: "CPLAssetByAssetDateWithoutHiddenOrDeleted",
			ListType:   "CPLAssetAndMasterByAssetDateWithoutHiddenOrDeleted",
			Direction:  "ASCENDING",
		},
		"Bursts": {
			Name:       "Bursts",
			ObjectType: "CPLAssetBurstStackAssetByAssetDate",
			ListType:   "CPLBurstStackAssetAndMasterByAssetDate",
			Direction:  "ASCENDING",
		},
		"Hidden": {
			Name:       "Hidden",
			ObjectType: "CPLAssetHiddenByAssetDate",
			ListType:   "CPLAssetAndMasterHiddenByAssetDate",
			Direction:  "ASCENDING",
		},
		"Recently Deleted": {
			Name:       "Recently Deleted",
			ObjectType: "CPLAssetDeletedByExpungedDate",
			ListType:   "CPLAssetAndMasterDeletedByExpungedDate",
			Direction:  "DESCENDING",
		},
	}
	for name, f := range smartAlbumFilters {
		albums[name] = &Album{
			Name:       name,
			ObjectType: "CPLAssetInSmartAlbumByAssetDate:" + f.suffix,
			ListType:   "CPLAssetAndMasterInSmartAlbumByAssetDate",
			Direction:  "ASCENDING",
			Filters: []Filter{{
				FieldName:  "smartAlbum",
				Comparator: "EQUALS",
				FieldValue: map[string]string{"type": "STRING", "value": f.tag},
			}},
		}
	}
	return albums
}

type errorRoundTripper struct{}

func (errorRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("test photos service has no network transport")
}

// NewTestPhotosService creates a PhotosService with pre-populated libraries for testing
func NewTestPhotosService(libs map[string]map[string]*Album) *PhotosService {
	httpClient := &http.Client{Transport: errorRoundTripper{}}
	session := &Session{srv: rest.NewClient(httpClient)}
	ps := &PhotosService{
		client: &Client{
			remoteName: "_test",
			Session:    session,
		},
		endpoint:    "http://test.invalid/database/1/com.apple.photos.cloud/production",
		pacer:       fs.NewPacer(context.Background(), pacer.NewDefault()),
		shouldRetry: func(ctx context.Context, resp *http.Response, err error) (bool, error) { return false, err },
		libraries:   make(map[string]*Library),
	}
	for zoneName, albums := range libs {
		lib := &Library{
			service: ps,
			zoneID:  zoneName,
			area:    areaPrivate,
			albums:  make(map[string]*Album),
		}
		lib.cacheValid.Store(true)
		for name, album := range albums {
			lib.restoreAlbumLinks(album)
			lib.albums[name] = album
		}
		ps.libraries[zoneName] = lib
	}
	return ps
}

// SetTestPhotoCache populates an album's photo cache for testing
func (album *Album) SetTestPhotoCache(cache map[string]*Photo) {
	album.mu.Lock()
	album.photoCache = cache
	album.mu.Unlock()
}

// NewPhotosService creates a new PhotosService instance
func NewPhotosService(ctx context.Context, client *Client, pacer *fs.Pacer, shouldRetry ShouldRetryFunc) (*PhotosService, error) {
	service, exists := client.Session.AccountInfo.Webservices["ckdatabasews"]
	if !exists || service.Status != "active" {
		return nil, fmt.Errorf("ckdatabasews service not available")
	}
	endpoint := fmt.Sprintf("%s/database/1/com.apple.photos.cloud/production", service.URL)

	ps := &PhotosService{
		client:      client,
		endpoint:    endpoint,
		pacer:       pacer,
		shouldRetry: shouldRetry,
		libraries:   make(map[string]*Library),
	}

	ps.checkIndexingState(ctx, "PrimarySync")

	return ps, nil
}

// GetLibraries returns all available photo libraries
func (ps *PhotosService) GetLibraries(ctx context.Context) (map[string]*Library, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if len(ps.libraries) > 0 {
		if discovered, err := ps.discoverLibraries(ctx); err == nil {
			ps.libraries = mergeDiscoveredLibraries(ctx, ps.libraries, discovered)
			ps.saveCachedLibraries()
			fs.Debugf(nil, "iclouddrive photos: refreshed %d in-memory libraries from API", len(ps.libraries))
		} else {
			fs.Debugf(nil, "iclouddrive photos: in-memory library rediscovery failed, using cached state: %v", err)
		}
		return ps.libraries, nil
	}

	// Try loading cached zone names from disk
	if cached := ps.loadCachedLibraries(); cached != nil {
		ps.batchCheckForChanges(ctx, cached)
		ps.libraries = cached
		if discovered, err := ps.discoverLibraries(ctx); err == nil {
			ps.libraries = mergeDiscoveredLibraries(ctx, cached, discovered)
			ps.saveCachedLibraries()
			fs.Debugf(nil, "iclouddrive photos: refreshed %d libraries from API", len(ps.libraries))
		} else {
			fs.Debugf(nil, "iclouddrive photos: library rediscovery failed, using cached zones: %v", err)
			fs.Debugf(nil, "iclouddrive photos: %d libraries from cache", len(cached))
		}
		return ps.libraries, nil
	}

	discovered, err := ps.discoverLibraries(ctx)
	if err != nil {
		return nil, err
	}
	ps.libraries = discovered.libraries
	ps.saveCachedLibraries()
	return ps.libraries, nil
}

func (ps *PhotosService) discoverLibraries(ctx context.Context) (*libraryDiscovery, error) {
	result := &libraryDiscovery{
		libraries:      make(map[string]*Library),
		refreshedAreas: make(map[string]bool),
	}

	// Discover zones from API - probe both private and shared databases
	// Private zones: owned by the current user (PrimarySync + owned SharedSync)
	// Shared zones: owned by another user (non-owner SharedSync)
	type zoneResponse struct {
		Zones []struct {
			ZoneID struct {
				ZoneName        string `json:"zoneName"`
				OwnerRecordName string `json:"ownerRecordName"`
				ZoneType        string `json:"zoneType"`
			} `json:"zoneID"`
			Deleted bool `json:"deleted,omitempty"`
		} `json:"zones"`
	}

	for _, area := range []string{areaPrivate, areaShared} {
		var response zoneResponse
		if err := ps.requestForArea(ctx, area, "changes/database", map[string]any{}, &response); err != nil {
			if area == areaShared {
				// Shared database may not exist for all accounts
				fs.Debugf(nil, "iclouddrive photos: shared zone discovery failed (expected if no shared library): %v", err)
				continue
			}
			return nil, fmt.Errorf("failed to discover zones: %w", err)
		}
		result.refreshedAreas[area] = true
		for _, zone := range response.Zones {
			if zone.Deleted {
				continue
			}
			name := zone.ZoneID.ZoneName
			// SharedSync-* found in private takes precedence over shared
			if _, exists := result.libraries[name]; exists {
				continue
			}
			result.libraries[name] = &Library{
				service:         ps,
				zoneID:          name,
				area:            area,
				ownerRecordName: zone.ZoneID.OwnerRecordName,
				zoneType:        zone.ZoneID.ZoneType,
				albums:          make(map[string]*Album),
			}
		}
	}
	return result, nil
}

func mergeDiscoveredLibraries(ctx context.Context, existing map[string]*Library, discovered *libraryDiscovery) map[string]*Library {
	merged := make(map[string]*Library, len(existing)+len(discovered.libraries))
	for name, cached := range existing {
		if !discovered.refreshedAreas[cached.area] {
			if cached.area == areaShared {
				exists, err := cached.probeZoneExists(ctx)
				if err != nil {
					fs.Debugf(nil, "iclouddrive photos: shared zone probe failed for %q, keeping cached zone: %v", cached.zoneID, err)
				} else if !exists {
					fs.Debugf(nil, "iclouddrive photos: dropping cached shared zone %q after authoritative ZONE_NOT_FOUND probe", cached.zoneID)
					continue
				}
			}
			merged[name] = cached
		}
	}
	for name, fresh := range discovered.libraries {
		if cached, ok := existing[name]; ok {
			cached.service = fresh.service
			cached.area = fresh.area
			cached.ownerRecordName = fresh.ownerRecordName
			cached.zoneType = fresh.zoneType
			merged[name] = cached
			continue
		}
		merged[name] = fresh
	}
	return merged
}

// albumRecord represents a CPLAlbum record from CloudKit
type albumRecord struct {
	RecordName string `json:"recordName"`
	Fields     struct {
		AlbumNameEnc *ckStringField `json:"albumNameEnc,omitempty"`
		AlbumType    *ckIntField    `json:"albumType,omitempty"`
		ParentID     *ckStringField `json:"parentId,omitempty"`
		IsDeleted    *ckBoolField   `json:"isDeleted,omitempty"`
	} `json:"fields"`
}

// albumQueryResponse wraps a paginated list of album records
type albumQueryResponse struct {
	Records            []albumRecord `json:"records"`
	ContinuationMarker string        `json:"continuationMarker"`
}

// cachedLibraryEntry stores zone metadata for disk cache persistence
type cachedLibraryEntry struct {
	ZoneName        string `json:"zoneName"`
	Area            string `json:"area,omitempty"`
	OwnerRecordName string `json:"ownerRecordName,omitempty"`
	ZoneType        string `json:"zoneType,omitempty"`
}

// loadCachedLibraries loads zone metadata from disk cache
func (ps *PhotosService) loadCachedLibraries() map[string]*Library {
	cacheFile := filepath.Join(ps.client.CacheDir(), "libraries.json")
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return nil
	}

	var entries []cachedLibraryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil
	}

	libs := make(map[string]*Library, len(entries))
	for _, entry := range entries {
		area := entry.Area
		if area == "" {
			area = areaPrivate
		}
		libs[entry.ZoneName] = &Library{
			service:         ps,
			zoneID:          entry.ZoneName,
			area:            area,
			ownerRecordName: entry.OwnerRecordName,
			zoneType:        entry.ZoneType,
			albums:          make(map[string]*Album),
		}
	}
	return libs
}

// saveCachedLibraries persists zone metadata to disk via atomic rename
func (ps *PhotosService) saveCachedLibraries() {
	var entries []cachedLibraryEntry
	for _, lib := range ps.libraries {
		entries = append(entries, cachedLibraryEntry{
			ZoneName:        lib.zoneID,
			Area:            lib.area,
			OwnerRecordName: lib.ownerRecordName,
			ZoneType:        lib.zoneType,
		})
	}
	saveJSONCache(ps.client.CacheDir(), "libraries.json", entries)
}

// GetAlbums returns all albums for this library
func (lib *Library) GetAlbums(ctx context.Context) (map[string]*Album, error) {
	lib.mu.Lock()
	defer lib.mu.Unlock()

	if len(lib.albums) > 0 {
		return lib.albums, nil
	}

	// Try loading cached albums if zone is unchanged
	if lib.cacheValid.Load() {
		if cached := lib.loadCachedAlbums(); cached != nil {
			lib.albums = cached
			fs.Debugf(nil, "iclouddrive photos: %d albums from cache for zone %s", len(cached), lib.zoneID)
			return lib.albums, nil
		}
	}

	// Build albums into a local map first so that a transient user album
	// query failure leaves lib.albums empty (retried on next call) rather
	// than permanently caching the smart-album-only subset
	albums := make(map[string]*Album, len(SmartAlbums))

	// Add smart albums
	for name, template := range SmartAlbums {
		albums[name] = &Album{
			Name:       template.Name,
			ObjectType: template.ObjectType,
			ListType:   template.ListType,
			Direction:  template.Direction,
			Filters:    append([]Filter{}, template.Filters...),
			RecordName: template.RecordName,
			lib:        lib,
		}
	}

	// Add user albums and folders (paginated - CloudKit caps at 200 per page)
	var allRecords []albumRecord
	var continuationMarker string

	for {
		query := map[string]any{
			"query":       map[string]any{"recordType": recordTypeAlbum},
			"zoneID":      lib.zoneIDMap(),
			"desiredKeys": []string{"albumNameEnc", "albumType", "parentId", "isDeleted"},
		}
		if continuationMarker != "" {
			query["continuationMarker"] = continuationMarker
		}

		var response albumQueryResponse

		if err := lib.request(ctx, "records/query", query, &response); err != nil {
			// SharedSync libraries return BAD_REQUEST / "Index has invalid data"
			// on CPLAlbumByPositionLive in live probes, so fall back to smart
			// albums there while surfacing PrimarySync failures directly
			fs.Debugf(nil, "iclouddrive photos: user album query failed for zone %q: %v", lib.zoneID, err)
			if lib.isSharedLibrary() && isSharedAlbumIndexError(err) {
				return albums, nil
			}
			return nil, fmt.Errorf("query user albums for zone %q: %w", lib.zoneID, err)
		}

		allRecords = append(allRecords, response.Records...)

		if response.ContinuationMarker == "" {
			break
		}
		continuationMarker = response.ContinuationMarker
	}

	for _, record := range allRecords {
		if record.Fields.AlbumNameEnc == nil ||
			record.RecordName == rootFolderRecord ||
			record.RecordName == projectRootFolderRecord ||
			(record.Fields.IsDeleted != nil && record.Fields.IsDeleted.Value) {
			continue
		}

		nameBytes, err := base64.StdEncoding.DecodeString(record.Fields.AlbumNameEnc.Value)
		if err != nil {
			fs.Debugf(nil, "iclouddrive photos: skipping album %q: base64 decode: %v", record.RecordName, err)
			continue
		}

		isFolder := record.Fields.AlbumType != nil && record.Fields.AlbumType.Value == albumTypeFolder
		albumName := norm.NFC.String(string(nameBytes))

		// User album with same name as a smart album - smart album has special
		// server-side query semantics that can't be replicated by the user album,
		// so we keep the smart album and skip the user album with a warning
		if _, isSmart := SmartAlbums[albumName]; isSmart && !isFolder {
			fs.Logf(nil, "iclouddrive photos: user album %q shadows smart album, using smart album", albumName)
			continue
		}

		if isFolder {
			folder := &Album{
				Name:       albumName,
				RecordName: record.RecordName,
				lib:        lib,
				IsFolder:   true,
				Children:   make(map[string]*Album),
			}
			if err := lib.fetchFolderChildren(ctx, folder); err != nil {
				return nil, fmt.Errorf("fetch children of folder %q: %w", albumName, err)
			}
			albums[albumName] = folder
		} else {
			albums[albumName] = lib.newUserAlbum(albumName, record.RecordName)
		}
	}

	lib.albums = albums
	lib.saveCachedAlbums()
	return lib.albums, nil
}

// loadCachedAlbums loads album metadata from disk cache
func (lib *Library) loadCachedAlbums() map[string]*Album {
	cacheFile := filepath.Join(lib.zoneCacheDir(), "albums.json")
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return nil
	}
	var albums map[string]*Album
	if err := json.Unmarshal(data, &albums); err != nil {
		return nil
	}
	// Restore runtime-only fields after deserialization
	for _, a := range albums {
		lib.restoreAlbumLinks(a)
	}
	return albums
}

// restoreAlbumLinks sets runtime-only fields (lib pointer) on an album
// and its children after deserialization from disk cache
func (lib *Library) restoreAlbumLinks(a *Album) {
	a.lib = lib
	for _, child := range a.Children {
		lib.restoreAlbumLinks(child)
	}
}

func flattenAlbumTree(dst []*Album, albums map[string]*Album) []*Album {
	for _, album := range albums {
		dst = append(dst, album)
		if album.IsFolder {
			dst = flattenAlbumTree(dst, album.Children)
		}
	}
	return dst
}

// saveCachedAlbums persists album metadata to disk via atomic rename
func (lib *Library) saveCachedAlbums() {
	saveJSONCache(lib.zoneCacheDir(), "albums.json", lib.albums)
}

// fetchFolderChildren queries child albums inside a folder by parentId
func (lib *Library) fetchFolderChildren(ctx context.Context, folder *Album) error {
	query := map[string]any{
		"query": map[string]any{
			"recordType": recordTypeAlbum,
			"filterBy": []map[string]any{{
				"fieldName":  "parentId",
				"comparator": "EQUALS",
				"fieldValue": map[string]string{"type": "STRING", "value": folder.RecordName},
			}},
		},
		"zoneID":      lib.zoneIDMap(),
		"desiredKeys": []string{"albumNameEnc", "albumType", "isDeleted"},
	}

	var continuationMarker string
	for {
		if continuationMarker != "" {
			query["continuationMarker"] = continuationMarker
		}

		var response albumQueryResponse

		if err := lib.request(ctx, "records/query", query, &response); err != nil {
			return err
		}

		for _, record := range response.Records {
			if record.Fields.AlbumNameEnc == nil ||
				(record.Fields.IsDeleted != nil && record.Fields.IsDeleted.Value) {
				continue
			}

			nameBytes, err := base64.StdEncoding.DecodeString(record.Fields.AlbumNameEnc.Value)
			if err != nil {
				fs.Debugf(nil, "iclouddrive photos: skipping child album %q: base64 decode: %v", record.RecordName, err)
				continue
			}

			childName := norm.NFC.String(string(nameBytes))
			isFolder := record.Fields.AlbumType != nil && record.Fields.AlbumType.Value == albumTypeFolder

			if isFolder {
				childFolder := &Album{
					Name:       childName,
					RecordName: record.RecordName,
					lib:        lib,
					IsFolder:   true,
					Children:   make(map[string]*Album),
				}
				if err := lib.fetchFolderChildren(ctx, childFolder); err != nil {
					return err
				}
				folder.Children[childName] = childFolder
			} else {
				folder.Children[childName] = lib.newUserAlbum(childName, record.RecordName)
			}
		}

		if response.ContinuationMarker == "" {
			break
		}
		continuationMarker = response.ContinuationMarker
	}

	return nil
}

// albumCacheKey returns a stable filename-safe key for an album's disk cache
func albumCacheKey(objectType string) string {
	h := sha256.Sum256([]byte(objectType))
	return hex.EncodeToString(h[:8])
}

// zoneCacheDir returns the disk cache directory for this zone
// Path follows rclone convention: <cacheDir>/<backend>/<remoteName>/<zone>/
func (lib *Library) zoneCacheDir() string {
	return filepath.Join(lib.service.client.CacheDir(), lib.zoneID)
}

// checkForChanges detects whether the zone has been modified since the last
// sync. If unchanged (0 records), sets cacheValid=true. If changed, buffers
// the first page as pendingDelta and still sets cacheValid=true (album disk
// cache is valid under the old token - delta hasn't been applied yet)
// The buffered delta is consumed later by applyPendingDelta when albums exist
func (lib *Library) checkForChanges(ctx context.Context) {
	lib.deltaMu.Lock()
	defer lib.deltaMu.Unlock()

	// Already have a buffered delta waiting to be applied
	if lib.pendingDelta != nil {
		return
	}

	token := lib.readSyncToken()
	if token == "" {
		return
	}

	var response changesZoneResponse
	if err := lib.request(ctx, "changes/zone", lib.changesZoneBody(token), &response); err != nil {
		fs.Debugf(nil, "iclouddrive photos: delta check failed for zone %s: %v", lib.zoneID, err)
		return
	}
	if len(response.Zones) == 0 {
		return
	}

	zone := response.Zones[0]

	if len(zone.Records) == 0 && !zone.MoreComing {
		// No changes - advance token, all caches valid
		fs.Debugf(nil, "iclouddrive photos: zone %s unchanged, using cached listings", lib.zoneID)
		lib.saveSyncToken(zone.SyncToken)
		lib.cacheValid.Store(true)
		return
	}

	// Buffer the delta for later application (after albums are populated)
	// Album disk cache is still valid under the old token
	lib.bufferDelta(zone.Records, zone.SyncToken, zone.MoreComing)
}

// zoneEntry pairs a changes/zone request body with its library for batched zone operations
type zoneEntry struct {
	zone map[string]any
	lib  *Library
}

// flattenZoneEntries extracts the zone request bodies and builds a zoneID→Library lookup
func flattenZoneEntries(entries []zoneEntry) ([]map[string]any, map[string]*Library) {
	zones := make([]map[string]any, len(entries))
	libByZone := make(map[string]*Library, len(entries))
	for i, e := range entries {
		zones[i] = e.zone
		libByZone[e.lib.zoneID] = e.lib
	}
	return zones, libByZone
}

// batchCheckForChanges checks all zones for changes in a single API call
// Each zone with a syncToken gets checked; zones without tokens are skipped
func (ps *PhotosService) batchCheckForChanges(ctx context.Context, libs map[string]*Library) {
	// Group zones by area for separate API calls (private and shared use different endpoints)
	byArea := make(map[string][]zoneEntry)
	for _, lib := range libs {
		lib.deltaMu.Lock()
		hasPending := lib.pendingDelta != nil
		lib.deltaMu.Unlock()
		if hasPending {
			continue
		}
		token := lib.readSyncToken()
		if token == "" {
			continue
		}
		zone := map[string]any{
			"zoneID":      lib.zoneIDMap(),
			"desiredKeys": changesZoneDesiredKeys,
			"syncToken":   token,
		}
		byArea[lib.area] = append(byArea[lib.area], zoneEntry{zone: zone, lib: lib})
	}

	for area, entries := range byArea {
		zones, libByZone := flattenZoneEntries(entries)
		var response changesZoneResponse
		if err := ps.requestForArea(ctx, area, "changes/zone", map[string]any{"zones": zones}, &response); err != nil {
			fs.Debugf(nil, "iclouddrive photos: batch delta check (%s) failed: %v", area, err)
			continue
		}
		for _, zone := range response.Zones {
			lib := libByZone[zone.ZoneID.ZoneName]
			if lib == nil {
				continue
			}
			lib.deltaMu.Lock()
			if len(zone.Records) == 0 && !zone.MoreComing {
				fs.Debugf(nil, "iclouddrive photos: zone %s unchanged, using cached listings", lib.zoneID)
				lib.saveSyncToken(zone.SyncToken)
				lib.cacheValid.Store(true)
			} else {
				lib.bufferDelta(zone.Records, zone.SyncToken, zone.MoreComing)
			}
			lib.deltaMu.Unlock()
		}
	}
}

// PollForChanges checks all zones for changes in a single API call using
// separate notification tokens, returns zone names that have been modified
// Used by ChangeNotify - does not consume or interfere with listing delta sync
func (ps *PhotosService) PollForChanges(ctx context.Context) []string {
	ps.mu.Lock()
	libs := make([]*Library, 0, len(ps.libraries))
	for _, lib := range ps.libraries {
		libs = append(libs, lib)
	}
	ps.mu.Unlock()

	// Group zones by area for separate API calls
	byArea := make(map[string][]zoneEntry)
	for _, lib := range libs {
		lib.deltaMu.Lock()
		token := lib.notifyToken
		lib.deltaMu.Unlock()
		if token == "" {
			token = lib.readSyncToken()
		}
		if token == "" {
			continue
		}
		byArea[lib.area] = append(byArea[lib.area], zoneEntry{
			zone: map[string]any{
				"zoneID":      lib.zoneIDMap(),
				"desiredKeys": changesZoneDesiredKeys,
				"syncToken":   token,
			},
			lib: lib,
		})
	}

	var changed []string
	for area, entries := range byArea {
		zones, libByZone := flattenZoneEntries(entries)
		var response changesZoneResponse
		if err := ps.requestForArea(ctx, area, "changes/zone", map[string]any{"zones": zones}, &response); err != nil {
			continue
		}

		for _, zone := range response.Zones {
			lib := libByZone[zone.ZoneID.ZoneName]
			if lib == nil {
				continue
			}
			lib.deltaMu.Lock()
			lib.notifyToken = zone.SyncToken
			lib.deltaMu.Unlock()

			if len(zone.Records) == 0 && !zone.MoreComing {
				continue
			}

			// Drain remaining pages per zone individually
			for zone.MoreComing {
				lib.deltaMu.Lock()
				token := lib.notifyToken
				lib.deltaMu.Unlock()
				var next changesZoneResponse
				body := map[string]any{"zones": []map[string]any{{
					"zoneID":      lib.zoneIDMap(),
					"desiredKeys": changesZoneDesiredKeys,
					"syncToken":   token,
				}}}
				if err := lib.request(ctx, "changes/zone", body, &next); err != nil {
					break
				}
				if len(next.Zones) == 0 {
					break
				}
				lib.deltaMu.Lock()
				lib.notifyToken = next.Zones[0].SyncToken
				lib.deltaMu.Unlock()
				zone.MoreComing = next.Zones[0].MoreComing
			}

			changed = append(changed, lib.zoneID)
			fs.Debugf(nil, "iclouddrive photos: ChangeNotify detected changes in zone %s", lib.zoneID)
		}
	}
	return changed
}

// readSyncToken loads the sync token from disk
func (lib *Library) readSyncToken() string {
	data, err := os.ReadFile(filepath.Join(lib.zoneCacheDir(), "syncToken"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// changesZoneResponse is the structure returned by changes/zone
type changesZoneResponse struct {
	Zones []changesZoneResult `json:"zones"`
}

// changesZoneResult is a single zone entry within a changesZoneResponse
type changesZoneResult struct {
	ZoneID struct {
		ZoneName string `json:"zoneName"`
	} `json:"zoneID"`
	Records    []json.RawMessage `json:"records"`
	SyncToken  string            `json:"syncToken"`
	MoreComing bool              `json:"moreComing"`
}

// changesZoneDesiredKeys are the fields requested from changes/zone for
// delta sync classification into smart albums
var changesZoneDesiredKeys = []string{
	// CPLMaster fields
	"filenameEnc", "itemType", "resOriginalRes", "resOriginalWidth", "resOriginalHeight",
	"resOriginalFileType", "resOriginalVidComplRes",
	"resOriginalAltRes", "resOriginalAltFileType",
	// CPLAsset classification + metadata fields
	"masterRef", "assetDate", "addedDate",
	"isFavorite", "isHidden", "isDeleted", "assetSubtype", "assetSubtypeV2", "burstId",
	"adjustmentRenderType",
	"adjustmentType", "resJPEGFullRes", "resJPEGFullFileType",
	"resVidFullRes", "resVidFullFileType",
	// CPLContainerRelation field for user album membership invalidation
	"containerId",
}

// changesZoneBody builds the request body for a changes/zone call
func (lib *Library) changesZoneBody(syncToken string) map[string]any {
	zone := map[string]any{
		"zoneID":      lib.zoneIDMap(),
		"desiredKeys": changesZoneDesiredKeys,
	}
	if syncToken != "" {
		zone["syncToken"] = syncToken
	}
	return map[string]any{"zones": []map[string]any{zone}}
}

// deltaParseResult holds the classified output of parseDeltaRecords
type deltaParseResult struct {
	deletedIDs           map[string]bool
	newMasters           map[string]*photoRecord
	newAssets            map[string]*photoRecord
	changedAlbumRecords  map[string]bool
	albumMetadataChanged bool
	hasAssetOnlyUpdates  bool
}

func relationAlbumRecordFromRecordName(recordName string) (string, bool) {
	parts := strings.SplitN(recordName, "-IN-", 2)
	if len(parts) != 2 || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}

// shouldInvalidate returns true if an album's cache should be invalidated
// rather than incrementally updated from this delta
func (r *deltaParseResult) shouldInvalidate(recordName string, isSmart bool) bool {
	return (!isSmart && r.changedAlbumRecords[recordName]) ||
		(isSmart && r.hasAssetOnlyUpdates)
}

// parseDeltaRecords classifies raw delta records from changes/zone into
// deletions, new masters/assets, album membership changes, and metadata flags
func parseDeltaRecords(records []json.RawMessage) *deltaParseResult {
	r := &deltaParseResult{
		deletedIDs:          map[string]bool{},
		newMasters:          map[string]*photoRecord{},
		newAssets:           map[string]*photoRecord{},
		changedAlbumRecords: map[string]bool{},
	}

	for _, raw := range records {
		var header struct {
			RecordName string `json:"recordName"`
			RecordType string `json:"recordType"`
			Deleted    bool   `json:"deleted"`
		}
		if err := json.Unmarshal(raw, &header); err != nil {
			continue
		}
		if header.Deleted && header.RecordType == "" {
			if albumRecord, ok := relationAlbumRecordFromRecordName(header.RecordName); ok {
				r.changedAlbumRecords[albumRecord] = true
				continue
			}
		}
		switch header.RecordType {
		case "CPLMaster":
			if header.Deleted {
				r.deletedIDs[header.RecordName] = true
			} else {
				var rec photoRecord
				if err := json.Unmarshal(raw, &rec); err == nil {
					r.newMasters[rec.RecordName] = &rec
				}
			}
		case "CPLAsset":
			if header.Deleted {
				// Asset deletion - mark both the asset itself and its master for removal
				// The asset ID is needed because edited entries use asset.RecordName as ID
				// (not master), so filtering by master alone leaves ghost -edited entries
				r.deletedIDs[header.RecordName] = true
				var rec photoRecord
				if err := json.Unmarshal(raw, &rec); err == nil && rec.Fields.MasterRef != nil {
					r.deletedIDs[rec.Fields.MasterRef.Value.RecordName] = true
				}
			} else {
				var rec photoRecord
				if err := json.Unmarshal(raw, &rec); err == nil && rec.Fields.MasterRef != nil {
					r.newAssets[rec.Fields.MasterRef.Value.RecordName] = &rec
				}
			}
		case "CPLAlbum":
			r.albumMetadataChanged = true
		case "CPLContainerRelation":
			var rel struct {
				Fields struct {
					ContainerID *struct {
						Value string `json:"value"`
					} `json:"containerId"`
				} `json:"fields"`
			}
			if err := json.Unmarshal(raw, &rel); err == nil {
				if rel.Fields.ContainerID != nil && rel.Fields.ContainerID.Value != "" {
					r.changedAlbumRecords[rel.Fields.ContainerID.Value] = true
					continue
				}
			}
			// Deleted relation records and some changes/zone entries omit fields,
			// the deterministic recordName still encodes the target album record
			if albumRecord, ok := relationAlbumRecordFromRecordName(header.RecordName); ok {
				r.changedAlbumRecords[albumRecord] = true
			} else if header.Deleted {
				fs.Debugf(nil, "iclouddrive photos: deleted CPLContainerRelation %q has unexpected recordName format", header.RecordName)
			}
		}
	}

	// Detect asset-only metadata updates (favorite/hide/soft-delete toggle)
	for masterID := range r.newAssets {
		if _, hasMaster := r.newMasters[masterID]; !hasMaster && !r.deletedIDs[masterID] {
			r.hasAssetOnlyUpdates = true
			break
		}
	}

	return r
}

// applyPendingDelta consumes a buffered delta and applies it to album caches
// Called from GetPhotos after albums are guaranteed populated
// Returns true if cache is current (no pending delta, or delta applied successfully)
func (lib *Library) applyPendingDelta(ctx context.Context) bool {
	lib.deltaMu.Lock()
	defer lib.deltaMu.Unlock()

	pending := lib.pendingDelta
	if pending == nil {
		return true // nothing pending, cache is current
	}
	failPendingDelta := func() bool {
		lib.pendingDelta = nil
		lib.cacheValid.Store(false)
		return false
	}

	// Verify albums are populated - if not (e.g. eager album invalidation
	// cleared the map before GetAlbums ran), clear pendingDelta so
	// checkForChanges can re-detect it on the next call. SyncToken was not
	// advanced so the same delta will be returned by changes/zone
	lib.mu.Lock()
	hasAlbums := len(lib.albums) > 0
	lib.mu.Unlock()
	if !hasAlbums {
		lib.pendingDelta = nil
		return false
	}

	// Collect all delta records (first page from buffer + remaining pages from API)
	allRecords := pending.records
	syncToken := pending.syncToken
	moreComing := pending.moreComing
	for moreComing {
		var response changesZoneResponse
		if err := lib.request(ctx, "changes/zone", lib.changesZoneBody(syncToken), &response); err != nil {
			return failPendingDelta()
		}
		if len(response.Zones) == 0 {
			return failPendingDelta()
		}
		allRecords = append(allRecords, response.Zones[0].Records...)
		syncToken = response.Zones[0].SyncToken
		moreComing = response.Zones[0].MoreComing
	}

	result := parseDeltaRecords(allRecords)

	// Build new Photo entries from delta master+asset pairs
	var addedPhotos []*Photo
	for masterID, master := range result.newMasters {
		built := buildPhotos(master, result.newAssets[masterID])
		addedPhotos = append(addedPhotos, built...)
	}

	fs.Debugf(nil, "iclouddrive photos: zone %s delta: %d deleted, %d added, %d album membership changes from %d records",
		lib.zoneID, len(result.deletedIDs), len(addedPhotos), len(result.changedAlbumRecords), len(allRecords))

	// Apply delta to each album's disk cache
	// Pre-resolve cache dir from lib to avoid re-acquiring ps.mu
	// under deltaMu (lock ordering: deltaMu must not precede ps.mu)
	cacheDir := lib.zoneCacheDir()
	lib.mu.Lock()
	albums := make([]*Album, 0, len(lib.albums))
	for _, album := range flattenAlbumTree(nil, lib.albums) {
		if album.ObjectType != "" {
			albums = append(albums, album)
		}
	}
	lib.mu.Unlock()

	for _, album := range albums {
		cached, ok := album.loadDiskCacheFrom(cacheDir)
		if !ok {
			continue
		}

		_, isSmart := SmartAlbums[album.Name]

		// Skip albums that will be invalidated below (avoids stale-data
		// window for concurrent readers and wasted disk I/O)
		if result.shouldInvalidate(album.RecordName, isSmart) {
			continue
		}

		// Remove deleted/changed entries
		filtered := make([]*Photo, 0, len(cached))
		for _, p := range cached {
			if !result.deletedIDs[p.ID] {
				filtered = append(filtered, p)
			}
		}

		// Route new photos to smart albums based on classifySmartAlbums()
		if isSmart {
			for _, p := range addedPhotos {
				for _, sa := range p.SmartAlbums {
					if sa == album.Name {
						filtered = append(filtered, p)
						break
					}
				}
			}
		}

		album.saveDiskCacheTo(cacheDir, filtered)

		// Update in-memory cache if populated
		album.mu.Lock()
		if album.photoCache != nil {
			// Deep copy before dedup so shared *Photo pointers across albums
			// don't get cross-contaminated by filename suffix mutations
			deduped := make([]*Photo, len(filtered))
			for i, p := range filtered {
				cp := *p
				deduped[i] = &cp
			}
			deduplicateFilenames(deduped)
			album.photoCache = buildPhotoCache(deduped)
		}
		album.mu.Unlock()
	}

	// Invalidate caches for albums affected by membership or metadata changes
	if len(result.changedAlbumRecords) > 0 || result.hasAssetOnlyUpdates {
		lib.mu.Lock()
		var invalidated []*Album
		for _, album := range flattenAlbumTree(nil, lib.albums) {
			_, isSmart := SmartAlbums[album.Name]
			if result.shouldInvalidate(album.RecordName, isSmart) {
				invalidated = append(invalidated, album)
			}
		}
		lib.mu.Unlock()
		for _, album := range invalidated {
			album.mu.Lock()
			album.photoCache = nil
			album.mu.Unlock()
			if album.ObjectType != "" {
				_ = os.Remove(filepath.Join(cacheDir, albumCacheKey(album.ObjectType)+".json"))
			}
			fs.Debugf(nil, "iclouddrive photos: invalidated album %q cache", album.Name)
		}
	}

	// Album metadata change (CPLAlbum created/renamed/deleted) - clear album
	// list so GetAlbums re-fetches from API on next call
	if result.albumMetadataChanged {
		lib.invalidateAlbumCache()
		fs.Debugf(nil, "iclouddrive photos: zone %s album metadata changed, will re-fetch album list", lib.zoneID)
	}

	lib.pendingDelta = nil
	lib.saveSyncToken(syncToken)
	return true
}

// saveSyncToken persists the zone sync token to disk via atomic rename
func (lib *Library) saveSyncToken(token string) {
	dir := lib.zoneCacheDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		fs.Debugf(nil, "iclouddrive photos: failed to create cache dir: %v", err)
		return
	}
	if err := atomicWriteFile(filepath.Join(dir, "syncToken"), []byte(token)); err != nil {
		fs.Debugf(nil, "iclouddrive photos: failed to write sync token: %v", err)
	}
}

// clearDiskCache removes all cached album data and sync token for this zone
func (lib *Library) clearDiskCache() {
	dir := lib.zoneCacheDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		_ = os.Remove(filepath.Join(dir, e.Name()))
	}
}

// request makes an API call routed through the album's library area
func (album *Album) request(ctx context.Context, endpoint string, data, response any) error {
	if album.lib != nil {
		return album.lib.request(ctx, endpoint, data, response)
	}
	return fmt.Errorf("album %q has no library", album.Name)
}

// zoneIDMap returns the full zoneID for this album's zone
func (album *Album) zoneIDMap() map[string]any {
	if album.lib == nil {
		return map[string]any{}
	}
	return album.lib.zoneIDMap()
}

// loadDiskCacheFrom loads cached photo data from a specific cache directory
func (album *Album) loadDiskCacheFrom(cacheDir string) ([]*Photo, bool) {
	cacheFile := filepath.Join(cacheDir, albumCacheKey(album.ObjectType)+".json")
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return nil, false
	}
	var photos []*Photo
	if err := json.Unmarshal(data, &photos); err != nil {
		return nil, false
	}
	return photos, true
}

// saveDiskCache persists photo data to disk for delta sync via atomic rename
func (album *Album) saveDiskCache(photos []*Photo) {
	if album.ObjectType == "" || album.lib == nil {
		return
	}
	album.saveDiskCacheTo(album.lib.zoneCacheDir(), photos)
}

// saveDiskCacheTo persists photo data to a specific cache directory
func (album *Album) saveDiskCacheTo(dir string, photos []*Photo) {
	saveJSONCache(dir, albumCacheKey(album.ObjectType)+".json", photos)
}

// countQuery builds a HyperionIndexCountLookup query for a single object type
func countQuery(objectType string, zoneID map[string]any) map[string]any {
	return map[string]any{
		"resultsLimit": 1,
		"query": map[string]any{
			"filterBy": map[string]any{
				"fieldName":  "indexCountID",
				"fieldValue": map[string]any{"type": "STRING_LIST", "value": []string{objectType}},
				"comparator": "IN",
			},
			"recordType": recordTypeCountIndex,
		},
		"zoneWide": true,
		"zoneID":   zoneID,
	}
}

// countBatchResponse is the response shape for batched count queries
type countBatchResponse struct {
	Batch []struct {
		Records []struct {
			Fields struct {
				ItemCount struct {
					Value int64 `json:"value"`
				} `json:"itemCount"`
			} `json:"fields"`
		} `json:"records"`
	} `json:"batch"`
}

// toCounts maps an ordered list of names to their counts from the batch response
func (r *countBatchResponse) toCounts(names []string) map[string]int64 {
	counts := make(map[string]int64, len(names))
	for i, name := range names {
		if i < len(r.Batch) && len(r.Batch[i].Records) > 0 {
			counts[name] = r.Batch[i].Records[0].Fields.ItemCount.Value
		}
	}
	return counts
}

// GetLibraryAlbumCounts returns the album count for each library in a single
// batched request, keyed by zone name (e.g. "PrimarySync")
func (ps *PhotosService) GetLibraryAlbumCounts(ctx context.Context) (map[string]int64, error) {
	ps.mu.Lock()
	type libEntry struct {
		name string
		lib  *Library
	}
	byArea := make(map[string][]libEntry)
	for name, lib := range ps.libraries {
		byArea[lib.area] = append(byArea[lib.area], libEntry{name: name, lib: lib})
	}
	ps.mu.Unlock()

	counts := make(map[string]int64)
	for area, entries := range byArea {
		var batch []map[string]any
		var order []string
		for _, e := range entries {
			order = append(order, e.name)
			batch = append(batch, countQuery(recordTypeAlbum, e.lib.zoneIDMap()))
		}
		var response countBatchResponse
		if err := ps.requestForArea(ctx, area, "internal/records/query/batch", map[string]any{"batch": batch}, &response); err != nil {
			return nil, fmt.Errorf("failed to get library album counts: %w", err)
		}
		for k, v := range response.toCounts(order) {
			counts[k] = v
		}
	}
	return counts, nil
}

// CloudKit field types for record deserialization
type ckStringField struct {
	Value string `json:"value"`
	Type  string `json:"type,omitempty"` // present on filenameEnc (ENCRYPTED_BYTES vs STRING)
}

type ckIntField struct {
	Value int `json:"value"`
}

type ckTimestampField struct {
	Value int64 `json:"value"`
}

type ckResourceField struct {
	Value struct {
		Size        int64  `json:"size"`
		DownloadURL string `json:"downloadURL"`
	} `json:"value"`
}

type ckBoolField struct {
	Value bool `json:"value"`
}

type ckReferenceField struct {
	Value struct {
		RecordName string `json:"recordName"`
	} `json:"value"`
}

// photoRecord represents a CloudKit record (CPLAsset or CPLMaster)
type photoRecord struct {
	RecordName string `json:"recordName"`
	RecordType string `json:"recordType"`
	Fields     struct {
		FilenameEnc            *ckStringField    `json:"filenameEnc,omitempty"`
		ItemType               *ckStringField    `json:"itemType,omitempty"`
		ResOriginalRes         *ckResourceField  `json:"resOriginalRes,omitempty"`
		ResOriginalWidth       *ckIntField       `json:"resOriginalWidth,omitempty"`
		ResOriginalHeight      *ckIntField       `json:"resOriginalHeight,omitempty"`
		ResOriginalFileType    *ckStringField    `json:"resOriginalFileType,omitempty"`
		ResOriginalVidComplRes *ckResourceField  `json:"resOriginalVidComplRes,omitempty"`
		ResOriginalAltRes      *ckResourceField  `json:"resOriginalAltRes,omitempty"`
		ResOriginalAltFileType *ckStringField    `json:"resOriginalAltFileType,omitempty"`
		MasterRef              *ckReferenceField `json:"masterRef,omitempty"`
		AssetDate              *ckTimestampField `json:"assetDate,omitempty"`
		AddedDate              *ckTimestampField `json:"addedDate,omitempty"`
		IsFavorite             *ckIntField       `json:"isFavorite,omitempty"`
		IsHidden               *ckIntField       `json:"isHidden,omitempty"`
		AssetSubtype           *ckIntField       `json:"assetSubtype,omitempty"`
		AssetSubtypeV2         *ckIntField       `json:"assetSubtypeV2,omitempty"`
		BurstID                *ckStringField    `json:"burstId,omitempty"`
		AdjustmentRenderType   *ckIntField       `json:"adjustmentRenderType,omitempty"`
		IsDeleted              *ckIntField       `json:"isDeleted,omitempty"`
		AdjustmentType         *ckStringField    `json:"adjustmentType,omitempty"`
		ResJPEGFullRes         *ckResourceField  `json:"resJPEGFullRes,omitempty"`
		ResJPEGFullFileType    *ckStringField    `json:"resJPEGFullFileType,omitempty"`
		ResVidFullRes          *ckResourceField  `json:"resVidFullRes,omitempty"`
		ResVidFullFileType     *ckStringField    `json:"resVidFullFileType,omitempty"`
	} `json:"fields"`
}

// classifySmartAlbums determines which smart albums a photo belongs to
// based on CPLMaster and CPLAsset fields from CloudKit
func classifySmartAlbums(master *photoRecord, asset *photoRecord) []string {
	isVideo := false
	if master.Fields.ResOriginalFileType != nil {
		uti := master.Fields.ResOriginalFileType.Value
		isVideo = uti == "public.mpeg-4" || uti == "com.apple.quicktime-movie"
	}

	var subtype, subtypeV2, favorite, hidden, deleted int
	if asset != nil {
		if asset.Fields.AssetSubtype != nil {
			subtype = asset.Fields.AssetSubtype.Value
		}
		if asset.Fields.AssetSubtypeV2 != nil {
			subtypeV2 = asset.Fields.AssetSubtypeV2.Value
		}
		if asset.Fields.IsFavorite != nil {
			favorite = asset.Fields.IsFavorite.Value
		}
		if asset.Fields.IsHidden != nil {
			hidden = asset.Fields.IsHidden.Value
		}
		if asset.Fields.IsDeleted != nil {
			deleted = asset.Fields.IsDeleted.Value
		}
	}

	// Soft-deleted assets (isDeleted=1) go to Recently Deleted only
	if deleted == 1 {
		return []string{"Recently Deleted"}
	}

	var albums []string
	if hidden == 0 {
		albums = append(albums, "All Photos")
	}
	if hidden == 1 {
		albums = append(albums, "Hidden")
	}
	if favorite == 1 {
		albums = append(albums, "Favorites")
	}
	if isVideo && subtype == 0 {
		albums = append(albums, "Videos")
	}
	if subtype == subtypeSloMo {
		albums = append(albums, "Slo-mo")
	}
	if subtype == subtypeTimeLapse {
		albums = append(albums, "Time-lapse")
	}
	if subtype == subtypePanorama {
		albums = append(albums, "Panoramas")
	}
	if subtypeV2 == subtypeV2Live {
		albums = append(albums, "Live")
	}
	if subtypeV2 == subtypeV2Screenshot {
		albums = append(albums, "Screenshots")
	}
	if asset != nil && asset.Fields.BurstID != nil && asset.Fields.BurstID.Value != "" {
		albums = append(albums, "Bursts")
	}
	// adjustmentRenderType is a bitmask: PORTRAIT=2, LONG_EXPOSURE=4
	if asset != nil && asset.Fields.AdjustmentRenderType != nil {
		art := asset.Fields.AdjustmentRenderType.Value
		if art&adjustPortrait != 0 {
			albums = append(albums, "Portrait")
		}
		if art&adjustLongExposure != 0 {
			albums = append(albums, "Long Exposure")
		}
	}
	// Animated (GIFs) detected by file type on master record
	if master.Fields.ResOriginalFileType != nil && master.Fields.ResOriginalFileType.Value == "com.compuserve.gif" {
		albums = append(albums, "Animated")
	}
	// Selfies: no reliable field available from delta records - server query handles it
	return albums
}

// deduplicateFilenames renames ALL photos with colliding filenames by appending
// the full masterID (CloudKit recordName) before the extension. Every duplicate
// gets the suffix so filenames are stable when photos are added or removed
// Unique filenames are untouched. Collision-free by construction since CloudKit
// recordNames are unique. Same pattern as googlephotos which embeds the full
// media item ID ({55+ chars}) in every filename
func deduplicateFilenames(photos []*Photo) {
	counts := make(map[string]int, len(photos))
	for _, p := range photos {
		counts[p.Filename]++
	}
	for _, p := range photos {
		if counts[p.Filename] <= 1 {
			continue
		}
		ext := path.Ext(p.Filename)
		base := strings.TrimSuffix(p.Filename, ext)
		p.Filename = base + "_" + p.ID + ext
	}
}

// buildPhotos creates Photo entries from a CPLMaster record and its paired CPLAsset
// Returns 1-2 entries: the photo itself, plus a .MOV companion for Live Photos
func buildPhotos(master *photoRecord, asset *photoRecord) []*Photo {
	photo := &Photo{ID: master.RecordName}

	if master.Fields.FilenameEnc != nil {
		if master.Fields.FilenameEnc.Type == "STRING" {
			photo.Filename = norm.NFC.String(master.Fields.FilenameEnc.Value)
		} else if decoded, err := base64.StdEncoding.DecodeString(master.Fields.FilenameEnc.Value); err == nil {
			photo.Filename = norm.NFC.String(string(decoded))
		}
	}
	// Fallback: synthesize filename from recordName + itemType UTI when filenameEnc is missing
	if photo.Filename == "" && master.Fields.ItemType != nil {
		if ext, ok := utiExtensions[master.Fields.ItemType.Value]; ok {
			photo.Filename = master.RecordName + ext
		}
	}

	if master.Fields.ResOriginalRes != nil {
		photo.Size = master.Fields.ResOriginalRes.Value.Size
	}

	if master.Fields.ResOriginalWidth != nil {
		photo.Width = master.Fields.ResOriginalWidth.Value
	}
	if master.Fields.ResOriginalHeight != nil {
		photo.Height = master.Fields.ResOriginalHeight.Value
	}

	var liveVideoSize int64
	var hasLiveVideo bool
	if master.Fields.ResOriginalVidComplRes != nil && master.Fields.ResOriginalVidComplRes.Value.DownloadURL != "" {
		liveVideoSize = master.Fields.ResOriginalVidComplRes.Value.Size
		hasLiveVideo = true
	}

	if asset != nil {
		if asset.Fields.AssetDate != nil {
			photo.AssetDate = asset.Fields.AssetDate.Value
		}
		if asset.Fields.AddedDate != nil {
			photo.AddedDate = asset.Fields.AddedDate.Value
		}
		photo.IsFavorite = asset.Fields.IsFavorite != nil && asset.Fields.IsFavorite.Value == 1
		photo.IsHidden = asset.Fields.IsHidden != nil && asset.Fields.IsHidden.Value == 1
	}

	photo.SmartAlbums = classifySmartAlbums(master, asset)

	hasDownloadURL := master.Fields.ResOriginalRes != nil && master.Fields.ResOriginalRes.Value.DownloadURL != ""
	if !hasDownloadURL || photo.Filename == "" {
		return nil
	}

	photo.ResourceKey = "resOriginalRes"
	result := []*Photo{photo}

	ext := path.Ext(photo.Filename)
	stem := strings.TrimSuffix(photo.Filename, ext)

	if hasLiveVideo {
		result = append(result, photo.companion(photo.ID, stem+".MOV", "resOriginalVidComplRes", liveVideoSize))
	}

	// Edited photo version (Photos.app adjustments)
	// Slo-mo edits are metadata-only (playback speed) with no separate rendered resource
	if asset != nil && asset.Fields.AdjustmentType != nil &&
		asset.Fields.AdjustmentType.Value != "" &&
		asset.Fields.AdjustmentType.Value != adjustSloMo {
		if asset.Fields.ResJPEGFullRes != nil && asset.Fields.ResJPEGFullRes.Value.DownloadURL != "" {
			editExt := extFromUTI(asset.Fields.ResJPEGFullFileType, ext)
			result = append(result, photo.companion(asset.RecordName, stem+"-edited"+editExt, "resJPEGFullRes", asset.Fields.ResJPEGFullRes.Value.Size))
		} else if asset.Fields.ResVidFullRes != nil && asset.Fields.ResVidFullRes.Value.DownloadURL != "" {
			editExt := extFromUTI(asset.Fields.ResVidFullFileType, ext)
			result = append(result, photo.companion(asset.RecordName, stem+"-edited"+editExt, "resVidFullRes", asset.Fields.ResVidFullRes.Value.Size))
		}
	}

	// RAW alternative (RAW+JPEG pairs where both originals are stored)
	if master.Fields.ResOriginalAltRes != nil && master.Fields.ResOriginalAltRes.Value.DownloadURL != "" {
		altExt := extFromUTI(master.Fields.ResOriginalAltFileType, ext)
		altFilename := stem + altExt
		if strings.EqualFold(altFilename, photo.Filename) {
			altFilename = stem + "-alt" + altExt
		}
		alt := photo.companion(master.RecordName, altFilename, "resOriginalAltRes", master.Fields.ResOriginalAltRes.Value.Size)
		alt.Width = photo.Width   // same sensor capture, same dimensions
		alt.Height = photo.Height // same sensor capture, same dimensions
		result = append(result, alt)
	}

	return result
}

// photosDesiredKeys are the fields requested for photo listing
var photosDesiredKeys = []string{
	"resOriginalRes", "resOriginalVidComplRes", "resOriginalFileType",
	"resOriginalWidth", "resOriginalHeight",
	"resOriginalAltRes", "resOriginalAltFileType",
	"filenameEnc", "itemType", "assetDate", "addedDate", "masterRef",
	"isFavorite", "isHidden", "isDeleted",
	"assetSubtype", "assetSubtypeV2", "burstId", "adjustmentRenderType",
	"adjustmentType", "resJPEGFullRes", "resJPEGFullFileType",
	"resVidFullRes", "resVidFullFileType",
}

// fetchPhotoCount returns the photo count for this album via HyperionIndexCountLookup
func (album *Album) fetchPhotoCount(ctx context.Context) (int64, error) {
	if album.ObjectType == "" {
		return 0, nil
	}
	var response struct {
		Records []struct {
			Fields struct {
				ItemCount struct {
					Value int64 `json:"value"`
				} `json:"itemCount"`
			} `json:"fields"`
		} `json:"records"`
	}
	if err := album.request(ctx, "records/query", countQuery(album.ObjectType, album.zoneIDMap()), &response); err != nil {
		return 0, err
	}
	if len(response.Records) > 0 {
		return response.Records[0].Fields.ItemCount.Value, nil
	}
	return 0, nil
}

// parsePhotoRecords extracts Photo entries from a batch of CloudKit records
func parsePhotoRecords(records []photoRecord) []*Photo {
	if len(records) == 0 {
		return nil
	}
	half := len(records)/2 + 1
	assetMap := make(map[string]*photoRecord, half)
	masters := make([]*photoRecord, 0, half)
	for i := range records {
		record := &records[i]
		switch record.RecordType {
		case "CPLAsset":
			if record.Fields.MasterRef != nil {
				assetMap[record.Fields.MasterRef.Value.RecordName] = record
			}
		case "CPLMaster":
			masters = append(masters, record)
		}
	}
	photos := make([]*Photo, 0, len(masters))
	for _, master := range masters {
		built := buildPhotos(master, assetMap[master.RecordName])
		photos = append(photos, built...)
	}
	return photos
}

// buildPartitionQuery constructs a CloudKit records/query body for a single
// startRank EQUALS partition, including album-specific direction and filters
func (album *Album) buildPartitionQuery(startRank int) map[string]any {
	filters := []map[string]any{
		{
			"fieldName":  "startRank",
			"comparator": "EQUALS",
			"fieldValue": map[string]any{"type": "INT64", "value": startRank},
		},
		{
			"fieldName":  "direction",
			"fieldValue": map[string]any{"type": "STRING", "value": album.Direction},
			"comparator": "EQUALS",
		},
	}
	for _, filter := range album.Filters {
		filters = append(filters, map[string]any{
			"fieldName":  filter.FieldName,
			"comparator": filter.Comparator,
			"fieldValue": filter.FieldValue,
		})
	}
	return map[string]any{
		"query": map[string]any{
			"filterBy":   filters,
			"recordType": album.ListType,
		},
		"resultsLimit": photosQueryLimit,
		"desiredKeys":  photosDesiredKeys,
		"zoneID":       album.zoneIDMap(),
	}
}

// fetchPhotosParallel fetches all photos using parallel startRank partitions
// Each partition is one API call with startRank EQUALS, no continuationMarker
// Stride = photosQueryLimit/2 photos per partition (200 records = 100 photos)
func (album *Album) fetchPhotosParallel(ctx context.Context, totalPhotos int64) ([]*Photo, string, error) {
	stride := photosQueryLimit / 2 // 100 photos per partition
	numPartitions := int((totalPhotos + int64(stride) - 1) / int64(stride))
	workers := fs.GetConfig(ctx).Checkers
	if workers < 1 {
		workers = 8
	}

	fs.Logf(nil, "iclouddrive photos: parallel cold listing %d photos in %d partitions (%d workers)",
		totalPhotos, numPartitions, workers)

	type partitionResult struct {
		photos      []*Photo
		syncToken   string
		recordCount int // raw record count to detect full pages
		err         error
	}

	// Cancel all remaining goroutines on first error
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make([]partitionResult, numPartitions)
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup

	for i := 0; i < numPartitions; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				results[idx] = partitionResult{err: ctx.Err()}
				return
			}
			defer func() { <-sem }()

			startRank := idx * stride
			query := album.buildPartitionQuery(startRank)

			var response struct {
				Records   []photoRecord `json:"records"`
				SyncToken string        `json:"syncToken"`
			}
			if err := album.request(ctx, "records/query", query, &response); err != nil {
				cancel() // stop remaining partitions
				results[idx] = partitionResult{err: fmt.Errorf("partition %d (rank=%d): %w", idx, startRank, err)}
				return
			}
			results[idx] = partitionResult{
				photos:      parsePhotoRecords(response.Records),
				syncToken:   response.SyncToken,
				recordCount: len(response.Records),
			}
		}(i)
	}
	wg.Wait()

	// Merge results in order
	var allPhotos []*Photo
	var lastSyncToken string
	var lastRecordCount int
	for _, r := range results {
		if r.err != nil {
			return nil, "", r.err
		}
		allPhotos = append(allPhotos, r.photos...)
		if r.syncToken != "" {
			lastSyncToken = r.syncToken
		}
		lastRecordCount = r.recordCount
	}

	// Completeness: keep fetching until a partition returns fewer than
	// resultsLimit records (partial page = last page). This handles stale
	// counts AND count=0 (count query failed - discover all photos here)
	needsTailCheck := lastRecordCount >= photosQueryLimit || numPartitions == 0
	nextRank := numPartitions * stride
	for needsTailCheck {
		fs.Debugf(nil, "iclouddrive photos: fetching tail partition at rank=%d", nextRank)
		query := album.buildPartitionQuery(nextRank)
		var response struct {
			Records   []photoRecord `json:"records"`
			SyncToken string        `json:"syncToken"`
		}
		if err := album.request(ctx, "records/query", query, &response); err != nil {
			return nil, "", fmt.Errorf("tail partition (rank=%d): %w", nextRank, err)
		}
		lastRecordCount = len(response.Records)
		allPhotos = append(allPhotos, parsePhotoRecords(response.Records)...)
		if response.SyncToken != "" {
			lastSyncToken = response.SyncToken
		}
		nextRank += stride
		needsTailCheck = lastRecordCount >= photosQueryLimit
	}

	fs.Logf(nil, "iclouddrive photos: parallel fetch complete, %d photos", len(allPhotos))
	return allPhotos, lastSyncToken, nil
}

// GetPhotos retrieves photos from this album using parallel partitions with disk cache
func (album *Album) GetPhotos(ctx context.Context) ([]*Photo, error) {
	// No library configured - return pre-populated cache (test path)
	if album.lib == nil {
		album.mu.Lock()
		defer album.mu.Unlock()
		result := make([]*Photo, 0, len(album.photoCache))
		for _, p := range album.photoCache {
			result = append(result, p)
		}
		return result, nil
	}

	// Check for changes, apply any buffered delta, serve from cache
	if album.ObjectType != "" {
		album.lib.checkForChanges(ctx)
		if album.lib.applyPendingDelta(ctx) && album.lib.cacheValid.Load() {
			// Serve from in-memory cache if populated (avoids disk I/O + JSON parse + dedup)
			album.mu.Lock()
			if album.photoCache != nil {
				result := make([]*Photo, 0, len(album.photoCache))
				for _, p := range album.photoCache {
					result = append(result, p)
				}
				album.mu.Unlock()
				return result, nil
			}
			album.mu.Unlock()
			// Fall back to disk cache
			if cached, ok := album.loadDiskCacheFrom(album.lib.zoneCacheDir()); ok {
				deduplicateFilenames(cached)
				album.mu.Lock()
				album.photoCache = buildPhotoCache(cached)
				album.mu.Unlock()
				fs.Debugf(nil, "iclouddrive photos: %d items from cache for %q", len(cached), album.Name)
				return cached, nil
			}
		}
	}

	// Fetch photo count for parallel partition calculation
	// If count unavailable, use 0 - the tail-fetch loop handles completeness
	var count int64
	if album.ObjectType != "" {
		count, _ = album.fetchPhotoCount(ctx)
	}
	photos, lastSyncToken, err := album.fetchPhotosParallel(ctx, count)
	if err != nil {
		return nil, err
	}

	// Persist original filenames for delta sync (dedup is applied on read)
	album.saveDiskCache(photos)

	deduplicateFilenames(photos)

	// Populate filename cache for NewObject lookups
	album.mu.Lock()
	album.photoCache = buildPhotoCache(photos)
	album.mu.Unlock()
	if lastSyncToken != "" && album.lib != nil {
		album.lib.saveSyncToken(lastSyncToken)
	}

	return photos, nil
}

// GetPhotoByName looks up a photo by filename, using cache if available
// CloudKit has no filterBy on filename fields - the only queryable fields
// are rank, date, smartAlbum, etc. Apple's own icloud.com UI paginates
// the full album and indexes client-side. On cache miss we must enumerate
// the entire album via GetPhotos before lookup
func (album *Album) GetPhotoByName(ctx context.Context, filename string) (*Photo, error) {
	album.mu.Lock()
	if album.photoCache != nil {
		photo, exists := album.photoCache[filename]
		album.mu.Unlock()
		if exists {
			return photo, nil
		}
		return nil, fmt.Errorf("photo %q not found in album %q", filename, album.Name)
	}
	album.mu.Unlock()

	// Cache miss - fetch all photos to populate cache
	photos, err := album.GetPhotos(ctx)
	if err != nil {
		return nil, err
	}
	for _, photo := range photos {
		if photo.Filename == filename {
			return photo, nil
		}
	}
	return nil, fmt.Errorf("photo %q not found in album %q", filename, album.Name)
}

// GetAlbumCounts returns photo counts for all albums in a single batch request
func (lib *Library) GetAlbumCounts(ctx context.Context) (map[string]int64, error) {
	// Snapshot under lock to avoid racing with GetAlbums
	type albumEntry struct {
		name       string
		objectType string
	}
	lib.mu.Lock()
	entries := make([]albumEntry, 0, len(lib.albums))
	for name, album := range lib.albums {
		if album.ObjectType == "" {
			continue // skip folders (albumType=3), they have no photo count
		}
		entries = append(entries, albumEntry{name: name, objectType: album.ObjectType})
	}
	lib.mu.Unlock()

	if len(entries) == 0 {
		return nil, nil
	}

	zoneIDAny := lib.zoneIDMap()

	var batch []map[string]any
	var albumOrder []string
	for _, entry := range entries {
		albumOrder = append(albumOrder, entry.name)
		batch = append(batch, countQuery(entry.objectType, zoneIDAny))
	}
	var response countBatchResponse
	if err := lib.request(ctx, "internal/records/query/batch", map[string]any{"batch": batch}, &response); err != nil {
		return nil, err
	}
	return response.toCounts(albumOrder), nil
}

// resolveZone returns the area and full zoneID for a zone name,
// using the library metadata if available or falling back to private
func (ps *PhotosService) resolveZone(zoneName string) (area string, zoneID map[string]any) {
	ps.mu.Lock()
	lib := ps.libraries[zoneName]
	ps.mu.Unlock()
	if lib != nil {
		return lib.area, lib.zoneIDMap()
	}
	return areaPrivate, map[string]any{"zoneName": zoneName}
}

// LookupDownloadURL fetches a fresh download URL for a record
// recordName is the CPLMaster or CPLAsset recordName depending on the resource
// resourceKey selects which resource to look up (e.g. "resOriginalRes",
// "resOriginalVidComplRes" for Live Photo video, "resJPEGFullRes" for edited)
func (ps *PhotosService) LookupDownloadURL(ctx context.Context, recordName, zone, resourceKey string) (string, error) {
	area, zoneID := ps.resolveZone(zone)

	query := map[string]any{
		"records": []map[string]any{
			{"recordName": recordName},
		},
		"zoneID": zoneID,
	}

	var response struct {
		Records []json.RawMessage `json:"records"`
	}

	if err := ps.requestForArea(ctx, area, "records/lookup", query, &response); err != nil {
		return "", fmt.Errorf("failed to look up record %q: %w", recordName, err)
	}

	if len(response.Records) == 0 {
		return "", fmt.Errorf("no records in lookup response for %q", recordName)
	}

	// Parse fields as raw JSON to extract the requested resource key
	var record struct {
		Fields map[string]json.RawMessage `json:"fields"`
	}
	if err := json.Unmarshal(response.Records[0], &record); err != nil {
		return "", fmt.Errorf("failed to parse lookup response: %w", err)
	}

	rawField, exists := record.Fields[resourceKey]
	if !exists {
		return "", fmt.Errorf("no %q field in record %q", resourceKey, recordName)
	}

	var res struct {
		Value struct {
			DownloadURL string `json:"downloadURL"`
		} `json:"value"`
	}
	if err := json.Unmarshal(rawField, &res); err != nil {
		return "", fmt.Errorf("failed to parse %q field: %w", resourceKey, err)
	}
	if res.Value.DownloadURL == "" {
		return "", fmt.Errorf("no download URL for %q in record %q", resourceKey, recordName)
	}

	return res.Value.DownloadURL, nil
}

// checkIndexingState warns if the iCloud Photo Library is still indexing
func (ps *PhotosService) checkIndexingState(ctx context.Context, zoneName string) {
	area, zoneID := ps.resolveZone(zoneName)

	query := map[string]any{
		"query":  map[string]any{"recordType": "CheckIndexingState"},
		"zoneID": zoneID,
	}

	var response struct {
		Records []struct {
			Fields struct {
				State struct {
					Value string `json:"value"`
				} `json:"state"`
			} `json:"fields"`
		} `json:"records"`
	}

	if err := ps.requestForArea(ctx, area, "records/query", query, &response); err != nil {
		fs.Logf(nil, "iclouddrive photos: could not check indexing state: %v", err)
		return
	}

	if len(response.Records) == 0 || response.Records[0].Fields.State.Value != indexingStateReady {
		fs.Logf(nil, "iclouddrive photos: library is still indexing, listings may be incomplete")
	}
}

// requestWithReauth makes a CloudKit request with pacer retry and reauth on 401/421
func (ps *PhotosService) requestWithReauth(ctx context.Context, makeOpts func() rest.Opts, data, response any) error {
	reauthDone := false
	return ps.pacer.Call(func() (bool, error) {
		resp, err := ps.client.Session.Request(ctx, makeOpts(), data, response)
		if !reauthDone && err != nil && resp != nil && (resp.StatusCode == 401 || resp.StatusCode == 421) {
			reauthDone = true
			if authErr := ps.client.Authenticate(ctx); authErr != nil {
				return false, authErr
			}
			if ps.client.Session.Requires2FA() {
				return false, errors.New("trust token expired, please reauth")
			}
			resp, err = ps.client.Session.Request(ctx, makeOpts(), data, response)
		}
		return ps.shouldRetry(ctx, resp, err)
	})
}

// requestForArea makes a request to the given area (private or shared) endpoint
func (ps *PhotosService) requestForArea(ctx context.Context, area, endpoint string, data, response any) error {
	rootURL := fmt.Sprintf("%s/%s/%s?remapEnums=true&getCurrentSyncToken=true", ps.endpoint, area, endpoint)

	return ps.requestWithReauth(ctx, func() rest.Opts {
		return rest.Opts{
			Method:       "POST",
			RootURL:      rootURL,
			ExtraHeaders: ps.client.Session.GetHeaders(map[string]string{"Content-Type": "text/plain"}), // text/plain matches icloud.com (CORS preflight bypass)
		}
	}, data, response)
}
