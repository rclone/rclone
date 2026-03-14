package api

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"sync"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/rest"
)

// PhotosService manages iCloud Photos API interactions
type PhotosService struct {
	client    *Client
	endpoint  string
	mu        sync.Mutex
	libraries map[string]*Library
}

// Library represents a photo library in a specific zone
type Library struct {
	service *PhotosService
	zoneID  string
	mu      sync.Mutex
	albums  map[string]*Album
}

// Album represents a photo album with its metadata and query configuration
type Album struct {
	Name       string
	ObjectType string
	ListType   string
	Direction  string
	Filters    []Filter
	RecordName string
	Zone       string
	service    *PhotosService
	mu         sync.Mutex
	photoCache map[string]*Photo // keyed by filename
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
	IsVideo     bool
	DownloadURL string
}

// Filter represents a CloudKit query filter
type Filter struct {
	FieldName  string      `json:"fieldName"`
	Comparator string      `json:"comparator"`
	FieldValue interface{} `json:"fieldValue"`
}

// SmartAlbums defines the built-in smart album types available in iCloud Photos.
var SmartAlbums = map[string]*Album{
	"All Photos": {
		Name:       "All Photos",
		ObjectType: "CPLAssetByAssetDateWithoutHiddenOrDeleted",
		ListType:   "CPLAssetAndMasterByAssetDateWithoutHiddenOrDeleted",
		Direction:  "ASCENDING",
	},
	"Time-lapse": {
		Name:       "Time-lapse",
		ObjectType: "CPLAssetInSmartAlbumByAssetDate:Timelapse",
		ListType:   "CPLAssetAndMasterInSmartAlbumByAssetDate",
		Direction:  "ASCENDING",
		Filters: []Filter{
			{
				FieldName:  "smartAlbum",
				Comparator: "EQUALS",
				FieldValue: map[string]string{"type": "STRING", "value": "TIMELAPSE"},
			},
		},
	},
	"Videos": {
		Name:       "Videos",
		ObjectType: "CPLAssetInSmartAlbumByAssetDate:Video",
		ListType:   "CPLAssetAndMasterInSmartAlbumByAssetDate",
		Direction:  "ASCENDING",
		Filters: []Filter{
			{
				FieldName:  "smartAlbum",
				Comparator: "EQUALS",
				FieldValue: map[string]string{"type": "STRING", "value": "VIDEO"},
			},
		},
	},
	"Slo-mo": {
		Name:       "Slo-mo",
		ObjectType: "CPLAssetInSmartAlbumByAssetDate:Slomo",
		ListType:   "CPLAssetAndMasterInSmartAlbumByAssetDate",
		Direction:  "ASCENDING",
		Filters: []Filter{
			{
				FieldName:  "smartAlbum",
				Comparator: "EQUALS",
				FieldValue: map[string]string{"type": "STRING", "value": "SLOMO"},
			},
		},
	},
	"Bursts": {
		Name:       "Bursts",
		ObjectType: "CPLAssetBurstStackAssetByAssetDate",
		ListType:   "CPLBurstStackAssetAndMasterByAssetDate",
		Direction:  "ASCENDING",
	},
	"Favorites": {
		Name:       "Favorites",
		ObjectType: "CPLAssetInSmartAlbumByAssetDate:Favorite",
		ListType:   "CPLAssetAndMasterInSmartAlbumByAssetDate",
		Direction:  "ASCENDING",
		Filters: []Filter{
			{
				FieldName:  "smartAlbum",
				Comparator: "EQUALS",
				FieldValue: map[string]string{"type": "STRING", "value": "FAVORITE"},
			},
		},
	},
	"Panoramas": {
		Name:       "Panoramas",
		ObjectType: "CPLAssetInSmartAlbumByAssetDate:Panorama",
		ListType:   "CPLAssetAndMasterInSmartAlbumByAssetDate",
		Direction:  "ASCENDING",
		Filters: []Filter{
			{
				FieldName:  "smartAlbum",
				Comparator: "EQUALS",
				FieldValue: map[string]string{"type": "STRING", "value": "PANORAMA"},
			},
		},
	},
	"Screenshots": {
		Name:       "Screenshots",
		ObjectType: "CPLAssetInSmartAlbumByAssetDate:Screenshot",
		ListType:   "CPLAssetAndMasterInSmartAlbumByAssetDate",
		Direction:  "ASCENDING",
		Filters: []Filter{
			{
				FieldName:  "smartAlbum",
				Comparator: "EQUALS",
				FieldValue: map[string]string{"type": "STRING", "value": "SCREENSHOT"},
			},
		},
	},
	"Live": {
		Name:       "Live",
		ObjectType: "CPLAssetInSmartAlbumByAssetDate:Live",
		ListType:   "CPLAssetAndMasterInSmartAlbumByAssetDate",
		Direction:  "ASCENDING",
		Filters: []Filter{
			{
				FieldName:  "smartAlbum",
				Comparator: "EQUALS",
				FieldValue: map[string]string{"type": "STRING", "value": "LIVE"},
			},
		},
	},
	"Recently Deleted": {
		Name:       "Recently Deleted",
		ObjectType: "CPLAssetDeletedByExpungedDate",
		ListType:   "CPLAssetAndMasterDeletedByExpungedDate",
		Direction:  "ASCENDING",
	},
	"Hidden": {
		Name:       "Hidden",
		ObjectType: "CPLAssetHiddenByAssetDate",
		ListType:   "CPLAssetAndMasterHiddenByAssetDate",
		Direction:  "ASCENDING",
	},
}

// NewPhotosService creates a new PhotosService instance
func NewPhotosService(ctx context.Context, client *Client) (*PhotosService, error) {
	service, exists := client.Session.AccountInfo.Webservices["ckdatabasews"]
	if !exists || service.Status != "active" {
		return nil, fmt.Errorf("ckdatabasews service not available")
	}
	endpoint := fmt.Sprintf("%s/database/1/com.apple.photos.cloud/production/private", service.URL)

	ps := &PhotosService{
		client:    client,
		endpoint:  endpoint,
		libraries: make(map[string]*Library),
	}

	// Verify primary zone is ready
	if err := ps.checkIndexingState(ctx, "PrimarySync"); err != nil {
		return nil, err
	}

	return ps, nil
}

// GetLibraries returns all available photo libraries
func (ps *PhotosService) GetLibraries(ctx context.Context) (map[string]*Library, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if len(ps.libraries) > 0 {
		return ps.libraries, nil
	}

	// Discover zones
	var response struct {
		Zones []struct {
			ZoneID struct {
				ZoneName string `json:"zoneName"`
			} `json:"zoneID"`
			Deleted bool `json:"deleted,omitempty"`
		} `json:"zones"`
	}

	if err := ps.request(ctx, "changes/database", map[string]interface{}{}, &response); err != nil {
		return nil, fmt.Errorf("failed to discover zones: %w", err)
	}

	for _, zone := range response.Zones {
		if !zone.Deleted {
			library := &Library{
				service: ps,
				zoneID:  zone.ZoneID.ZoneName,
				albums:  make(map[string]*Album),
			}
			ps.libraries[zone.ZoneID.ZoneName] = library
		}
	}

	return ps.libraries, nil
}

// GetAlbums returns all albums for this library
func (lib *Library) GetAlbums(ctx context.Context) (map[string]*Album, error) {
	lib.mu.Lock()
	defer lib.mu.Unlock()

	if len(lib.albums) > 0 {
		return lib.albums, nil
	}

	// Add smart albums
	for name, template := range SmartAlbums {
		album := &Album{
			Name:       template.Name,
			ObjectType: template.ObjectType,
			ListType:   template.ListType,
			Direction:  template.Direction,
			Filters:    template.Filters,
			RecordName: template.RecordName,
			Zone:       lib.zoneID,
			service:    lib.service,
		}
		lib.albums[name] = album
	}

	// Add user albums
	query := map[string]interface{}{
		"query":  map[string]interface{}{"recordType": "CPLAlbumByPositionLive"},
		"zoneID": map[string]string{"zoneName": lib.zoneID},
	}

	var response struct {
		Records []struct {
			RecordName string `json:"recordName"`
			Fields     struct {
				AlbumNameEnc *struct {
					Value string `json:"value"`
				} `json:"albumNameEnc,omitempty"`
				IsDeleted *struct {
					Value bool `json:"value"`
				} `json:"isDeleted,omitempty"`
			} `json:"fields"`
		} `json:"records"`
	}

	if err := lib.service.request(ctx, "records/query", query, &response); err != nil {
		return nil, fmt.Errorf("failed to query user albums: %w", err)
	}
	for _, record := range response.Records {
		if record.Fields.AlbumNameEnc == nil ||
			record.RecordName == "----Root-Folder----" ||
			(record.Fields.IsDeleted != nil && record.Fields.IsDeleted.Value) {
			continue
		}

		nameBytes, err := base64.StdEncoding.DecodeString(record.Fields.AlbumNameEnc.Value)
		if err != nil {
			continue
		}

		albumName := string(nameBytes)
		lib.albums[albumName] = &Album{
			Name:       albumName,
			ObjectType: fmt.Sprintf("CPLContainerRelationNotDeletedByAssetDate:%s", record.RecordName),
			ListType:   "CPLContainerRelationLiveByAssetDate",
			Direction:  "ASCENDING",
			RecordName: record.RecordName,
			Zone:       lib.zoneID,
			service:    lib.service,
			Filters: []Filter{
				{
					FieldName:  "parentId",
					Comparator: "EQUALS",
					FieldValue: map[string]string{"type": "STRING", "value": record.RecordName},
				},
			},
		}
	}

	return lib.albums, nil
}

// GetAlbumCount returns the number of albums in this library
func (lib *Library) GetAlbumCount(ctx context.Context) (int64, error) {
	query := map[string]interface{}{
		"batch": []map[string]interface{}{
			{
				"resultsLimit": 1,
				"query": map[string]interface{}{
					"filterBy": map[string]interface{}{
						"fieldName": "indexCountID",
						"fieldValue": map[string]interface{}{
							"type":  "STRING_LIST",
							"value": []string{"CPLAlbumByPositionLive"},
						},
						"comparator": "IN",
					},
					"recordType": "HyperionIndexCountLookup",
				},
				"zoneWide": true,
				"zoneID":   map[string]interface{}{"zoneName": lib.zoneID},
			},
		},
	}

	var response struct {
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

	if err := lib.service.request(ctx, "internal/records/query/batch", query, &response); err != nil {
		return 0, fmt.Errorf("failed to get album count: %w", err)
	}

	if len(response.Batch) > 0 && len(response.Batch[0].Records) > 0 {
		return response.Batch[0].Records[0].Fields.ItemCount.Value, nil
	}

	return 0, nil
}

// GetPhotos retrieves photos from this album
func (album *Album) GetPhotos(ctx context.Context, limit int) ([]*Photo, error) {
	var photos []*Photo
	offset := 0

	for {
		// Build query
		filters := []map[string]interface{}{
			{
				"fieldName":  "startRank",
				"fieldValue": map[string]interface{}{"type": "INT64", "value": offset},
				"comparator": "EQUALS",
			},
			{
				"fieldName":  "direction",
				"fieldValue": map[string]interface{}{"type": "STRING", "value": album.Direction},
				"comparator": "EQUALS",
			},
		}

		for _, filter := range album.Filters {
			filters = append(filters, map[string]interface{}{
				"fieldName":  filter.FieldName,
				"comparator": filter.Comparator,
				"fieldValue": filter.FieldValue,
			})
		}

		query := map[string]interface{}{
			"query": map[string]interface{}{
				"filterBy":   filters,
				"recordType": album.ListType,
			},
			"resultsLimit": 200,
			"desiredKeys": []string{
				"resOriginalWidth", "resOriginalHeight", "resOriginalRes",
				"resVidFullRes", "filenameEnc", "assetDate", "addedDate",
				"masterRef", "recordName", "recordType",
			},
			"zoneID": map[string]interface{}{"zoneName": album.Zone},
		}

		// Execute query
		var response struct {
			Records []map[string]interface{} `json:"records"`
		}

		if err := album.service.request(ctx, "records/query", query, &response); err != nil {
			return nil, fmt.Errorf("failed to fetch photos: %w", err)
		}

		// Separate records
		assetMap := make(map[string]map[string]interface{})
		var masters []map[string]interface{}

		for _, record := range response.Records {
			if recordType, _ := record["recordType"].(string); recordType == "CPLAsset" {
				if masterRef := getNestedField(record, "fields", "masterRef", "value", "recordName"); masterRef != "" {
					assetMap[masterRef] = record
				}
			} else if recordType == "CPLMaster" {
				masters = append(masters, record)
			}
		}

		if len(masters) == 0 {
			break
		}

		// Create photos from records
		for _, master := range masters {
			recordName, ok := master["recordName"].(string)
			if !ok {
				continue
			}

			fields, ok := master["fields"].(map[string]interface{})
			if !ok {
				continue
			}

			photo := &Photo{ID: recordName}

			// Extract filename
			if filenameEnc, ok := getFieldValue(fields, "filenameEnc").(string); ok {
				if decoded, err := base64.StdEncoding.DecodeString(filenameEnc); err == nil {
					photo.Filename = string(decoded)
				}
			}

			// Extract size and download URL from original resource
			if resOriginal, ok := getFieldValue(fields, "resOriginalRes").(map[string]interface{}); ok {
				if size, ok := resOriginal["size"].(float64); ok {
					photo.Size = int64(size)
				}
				if url, ok := resOriginal["downloadURL"].(string); ok {
					photo.DownloadURL = url
				}
			}

			// Extract dimensions
			if width := getFieldValue(fields, "resOriginalWidth"); width != nil {
				if w, ok := width.(float64); ok {
					photo.Width = int(w)
				}
			}
			if height := getFieldValue(fields, "resOriginalHeight"); height != nil {
				if h, ok := height.(float64); ok {
					photo.Height = int(h)
				}
			}

			// Check if it's a video
			photo.IsVideo = fields["resVidFullRes"] != nil

			// Extract dates from asset record
			if assetRecord, exists := assetMap[recordName]; exists {
				if assetFields, ok := assetRecord["fields"].(map[string]interface{}); ok {
					if assetDate := getFieldValue(assetFields, "assetDate"); assetDate != nil {
						if date, ok := assetDate.(float64); ok {
							photo.AssetDate = int64(date)
						}
					}
					if addedDate := getFieldValue(assetFields, "addedDate"); addedDate != nil {
						if date, ok := addedDate.(float64); ok {
							photo.AddedDate = int64(date)
						}
					}
				}
			}

			// Only include photos with download URLs
			if photo.DownloadURL != "" {
				photos = append(photos, photo)
				if limit > 0 && len(photos) >= limit {
					return photos[:limit], nil
				}
			}
		}

		// Log pagination status
		fs.Debugf(nil, "iclouddrive: fetched %d photos", len(photos))
		offset += len(masters)
	}

	// Populate filename cache for NewObject lookups
	album.mu.Lock()
	album.photoCache = make(map[string]*Photo, len(photos))
	for _, photo := range photos {
		if photo.Filename != "" {
			album.photoCache[photo.Filename] = photo
		}
	}
	album.mu.Unlock()

	return photos, nil
}

// GetPhotoByName looks up a photo by filename, using cache if available.
// Falls back to fetching all photos if cache is empty.
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

	// Cache miss — fetch all photos to populate cache
	photos, err := album.GetPhotos(ctx, 0)
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

// GetAlbumCounts returns photo counts for all albums in a single batch request.
// Returns a map of album name → count.
func (lib *Library) GetAlbumCounts(ctx context.Context) (map[string]int64, error) {
	lib.mu.Lock()
	albums := lib.albums
	lib.mu.Unlock()

	if len(albums) == 0 {
		return nil, nil
	}

	// Build batch query with one entry per album
	var batch []map[string]interface{}
	var albumOrder []string
	for name, album := range albums {
		albumOrder = append(albumOrder, name)
		batch = append(batch, map[string]interface{}{
			"resultsLimit": 1,
			"query": map[string]interface{}{
				"filterBy": map[string]interface{}{
					"fieldName": "indexCountID",
					"fieldValue": map[string]interface{}{
						"type":  "STRING_LIST",
						"value": []string{album.ObjectType},
					},
					"comparator": "IN",
				},
				"recordType": "HyperionIndexCountLookup",
			},
			"zoneWide": true,
			"zoneID":   map[string]interface{}{"zoneName": album.Zone},
		})
	}

	var response struct {
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

	if err := lib.service.request(ctx, "internal/records/query/batch", map[string]interface{}{"batch": batch}, &response); err != nil {
		return nil, err
	}

	counts := make(map[string]int64, len(albums))
	for i, name := range albumOrder {
		if i < len(response.Batch) && len(response.Batch[i].Records) > 0 {
			counts[name] = response.Batch[i].Records[0].Fields.ItemCount.Value
		}
	}
	return counts, nil
}

// GetPhoto finds a photo by its path (libraryName/albumName/filename).
// Uses album photo cache when available (populated by GetPhotos during List).
func (ps *PhotosService) GetPhoto(ctx context.Context, libraryName, albumName, filename string) (*Photo, error) {
	libraries, err := ps.GetLibraries(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get libraries: %w", err)
	}

	library, exists := libraries[libraryName]
	if !exists {
		return nil, fmt.Errorf("library %q not found", libraryName)
	}

	albums, err := library.GetAlbums(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get albums: %w", err)
	}

	album, exists := albums[albumName]
	if !exists {
		return nil, fmt.Errorf("album %q not found in library %q", albumName, libraryName)
	}

	return album.GetPhotoByName(ctx, filename)
}

// checkIndexingState verifies that the iCloud Photo Library is ready
func (ps *PhotosService) checkIndexingState(ctx context.Context, zoneID string) error {
	query := map[string]interface{}{
		"query":  map[string]interface{}{"recordType": "CheckIndexingState"},
		"zoneID": map[string]string{"zoneName": zoneID},
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

	if err := ps.request(ctx, "records/query", query, &response); err != nil {
		return fmt.Errorf("failed to check indexing state: %w", err)
	}

	if len(response.Records) == 0 || response.Records[0].Fields.State.Value != "FINISHED" {
		return fmt.Errorf("iCloud Photo Library not ready for indexing")
	}

	return nil
}

// Helper functions for internal use

func (ps *PhotosService) request(ctx context.Context, endpoint string, data interface{}, response interface{}) error {
	params := url.Values{
		"remapEnums":          {"true"},
		"getCurrentSyncToken": {"true"},
	}

	opts := rest.Opts{
		Method:       "POST",
		RootURL:      fmt.Sprintf("%s/%s?%s", ps.endpoint, endpoint, params.Encode()),
		ExtraHeaders: ps.client.Session.GetHeaders(map[string]string{"Content-Type": "text/plain"}),
	}

	_, err := ps.client.Session.Request(ctx, opts, data, response)
	return err
}

func getFieldValue(fields map[string]interface{}, fieldName string) interface{} {
	if field, ok := fields[fieldName].(map[string]interface{}); ok {
		return field["value"]
	}
	return nil
}

func getNestedField(data map[string]interface{}, keys ...string) string {
	current := data
	for _, key := range keys[:len(keys)-1] {
		if next, ok := current[key].(map[string]interface{}); ok {
			current = next
		} else {
			return ""
		}
	}
	if value, ok := current[keys[len(keys)-1]].(string); ok {
		return value
	}
	return ""
}
