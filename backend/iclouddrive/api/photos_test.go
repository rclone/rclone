package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setTestCacheDir(t *testing.T) string {
	t.Helper()
	oldCacheDir := config.GetCacheDir()
	cacheDir := t.TempDir()
	require.NoError(t, config.SetCacheDir(cacheDir))
	t.Cleanup(func() {
		_ = config.SetCacheDir(oldCacheDir)
	})
	return cacheDir
}

func newHTTPTestPhotosService(t *testing.T, remoteName string, handler http.HandlerFunc) *PhotosService {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	session := NewSession()
	session.srv = rest.NewClient(server.Client())

	return &PhotosService{
		client: &Client{
			remoteName: remoteName,
			Session:    session,
		},
		endpoint:    server.URL + "/database/1/com.apple.photos.cloud/production",
		pacer:       fs.NewPacer(context.Background(), pacer.NewDefault()),
		shouldRetry: func(ctx context.Context, resp *http.Response, err error) (bool, error) { return false, err },
		libraries:   make(map[string]*Library),
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		panic(err)
	}
}

func readJSONBody(r *http.Request, out any) error {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

func testAlbumRecordJSON(name, recordName string, albumType int, parentID string) map[string]any {
	fields := map[string]any{
		"albumNameEnc": map[string]any{"value": base64.StdEncoding.EncodeToString([]byte(name))},
	}
	if albumType != 0 {
		fields["albumType"] = map[string]any{"value": albumType}
	}
	if parentID != "" {
		fields["parentId"] = map[string]any{"value": parentID}
	}
	return map[string]any{
		"recordName": recordName,
		"fields":     fields,
	}
}

func newUserAlbumForTest(lib *Library, name, recordName string) *Album {
	album := lib.newUserAlbum(name, recordName)
	album.lib = lib
	return album
}

func TestGetPhotos_DoesNotServeStaleCacheOnPagedDeltaFailure(t *testing.T) {
	setTestCacheDir(t)
	ctx := context.Background()
	var changesZoneCalls atomic.Int32

	ps := newHTTPTestPhotosService(t, "paged-delta-failure", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/database/1/com.apple.photos.cloud/production/private/changes/zone":
			if changesZoneCalls.Add(1) > 1 {
				http.Error(w, "paged delta failure", http.StatusInternalServerError)
				return
			}
			writeJSON(t, w, map[string]any{
				"zones": []map[string]any{{
					"zoneID":     map[string]any{"zoneName": "PrimarySync"},
					"records":    []map[string]any{{"recordName": "asset1-IN-album-record", "deleted": true}},
					"syncToken":  "mid-token",
					"moreComing": true,
				}},
			})
		default:
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	})

	lib := &Library{service: ps, zoneID: "PrimarySync", area: areaPrivate, albums: map[string]*Album{}}
	album := newUserAlbumForTest(lib, "Exported", "album-record")
	lib.albums[album.Name] = album
	album.saveDiskCache([]*Photo{{ID: "stale-id", Filename: "stale.jpg"}})
	require.NoError(t, os.WriteFile(filepath.Join(lib.zoneCacheDir(), "syncToken"), []byte("old-token"), 0600))

	photos, err := album.GetPhotos(ctx)
	assert.Error(t, err)
	assert.Nil(t, photos)
	assert.Nil(t, lib.pendingDelta, "failed delta apply must not leave pendingDelta stuck forever")
}

func TestGetLibraries_RediscoversZonesWhenCacheExists(t *testing.T) {
	setTestCacheDir(t)
	ctx := context.Background()
	var changesDatabaseCalls atomic.Int32

	ps := newHTTPTestPhotosService(t, "library-rediscovery", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/database/1/com.apple.photos.cloud/production/private/changes/database" {
			changesDatabaseCalls.Add(1)
			writeJSON(t, w, map[string]any{"zones": []map[string]any{{"zoneID": map[string]any{"zoneName": "PrimarySync", "ownerRecordName": "owner", "zoneType": "REGULAR_CUSTOM_ZONE"}}}})
			return
		}
		if r.URL.Path == "/database/1/com.apple.photos.cloud/production/shared/changes/database" {
			changesDatabaseCalls.Add(1)
			writeJSON(t, w, map[string]any{"zones": []map[string]any{{"zoneID": map[string]any{"zoneName": "SharedSync-test", "ownerRecordName": "other", "zoneType": "REGULAR_CUSTOM_ZONE"}}}})
			return
		}
		http.Error(w, "unexpected request", http.StatusInternalServerError)
	})

	saveJSONCache(ps.client.CacheDir(), "libraries.json", []cachedLibraryEntry{{ZoneName: "PrimarySync", Area: areaPrivate}})

	libs, err := ps.GetLibraries(ctx)
	require.NoError(t, err)
	assert.Contains(t, libs, "PrimarySync")
	assert.Contains(t, libs, "SharedSync-test")
	assert.GreaterOrEqual(t, changesDatabaseCalls.Load(), int32(2), "must rediscover zones even when cache exists")
}

func TestGetLibraries_RediscoveryPreservesBufferedDelta(t *testing.T) {
	setTestCacheDir(t)
	ctx := context.Background()

	ps := newHTTPTestPhotosService(t, "library-merge", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/database/1/com.apple.photos.cloud/production/private/changes/zone":
			writeJSON(t, w, map[string]any{
				"zones": []map[string]any{{
					"zoneID":     map[string]any{"zoneName": "PrimarySync"},
					"records":    []map[string]any{{"recordName": "asset123-IN-album-uuid", "deleted": true}},
					"syncToken":  "new-token",
					"moreComing": false,
				}},
			})
		case "/database/1/com.apple.photos.cloud/production/private/changes/database":
			writeJSON(t, w, map[string]any{"zones": []map[string]any{{"zoneID": map[string]any{"zoneName": "PrimarySync", "ownerRecordName": "owner", "zoneType": "REGULAR_CUSTOM_ZONE"}}}})
		case "/database/1/com.apple.photos.cloud/production/shared/changes/database":
			writeJSON(t, w, map[string]any{"zones": []map[string]any{{"zoneID": map[string]any{"zoneName": "SharedSync-test", "ownerRecordName": "other", "zoneType": "REGULAR_CUSTOM_ZONE"}}}})
		default:
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	})

	saveJSONCache(ps.client.CacheDir(), "libraries.json", []cachedLibraryEntry{{ZoneName: "PrimarySync", Area: areaPrivate}})
	require.NoError(t, os.MkdirAll(filepath.Join(ps.client.CacheDir(), "PrimarySync"), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(ps.client.CacheDir(), "PrimarySync", "syncToken"), []byte("cached-token"), 0600))

	libs, err := ps.GetLibraries(ctx)
	require.NoError(t, err)
	primary := libs["PrimarySync"]
	require.NotNil(t, primary)
	require.NotNil(t, primary.pendingDelta, "rediscovery must not discard pending delta buffered from cached zone state")
	assert.Contains(t, libs, "SharedSync-test")
}

func TestGetLibraries_RefreshesInMemoryLibraries(t *testing.T) {
	setTestCacheDir(t)
	ctx := context.Background()
	var changesDatabaseCalls atomic.Int32

	ps := newHTTPTestPhotosService(t, "library-refresh-memory", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/database/1/com.apple.photos.cloud/production/private/changes/database":
			changesDatabaseCalls.Add(1)
			writeJSON(t, w, map[string]any{"zones": []map[string]any{{"zoneID": map[string]any{"zoneName": "PrimarySync", "ownerRecordName": "owner", "zoneType": "REGULAR_CUSTOM_ZONE"}}}})
		case "/database/1/com.apple.photos.cloud/production/shared/changes/database":
			changesDatabaseCalls.Add(1)
			writeJSON(t, w, map[string]any{"zones": []map[string]any{{"zoneID": map[string]any{"zoneName": "SharedSync-live", "ownerRecordName": "other", "zoneType": "REGULAR_CUSTOM_ZONE"}}}})
		default:
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	})

	ps.libraries = map[string]*Library{
		"PrimarySync": {service: ps, zoneID: "PrimarySync", area: areaPrivate, albums: make(map[string]*Album)},
	}

	libs, err := ps.GetLibraries(ctx)
	require.NoError(t, err)
	assert.Contains(t, libs, "PrimarySync")
	assert.Contains(t, libs, "SharedSync-live")
	assert.GreaterOrEqual(t, changesDatabaseCalls.Load(), int32(2), "in-memory libraries must still rediscover zones")
}

func TestGetLibraries_KeepsCachedSharedZonesOnTransientSharedRediscoveryFailure(t *testing.T) {
	setTestCacheDir(t)
	ctx := context.Background()

	ps := newHTTPTestPhotosService(t, "library-shared-preserve", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/database/1/com.apple.photos.cloud/production/private/changes/database":
			writeJSON(t, w, map[string]any{"zones": []map[string]any{{"zoneID": map[string]any{"zoneName": "PrimarySync", "ownerRecordName": "owner", "zoneType": "REGULAR_CUSTOM_ZONE"}}}})
		case "/database/1/com.apple.photos.cloud/production/shared/changes/database":
			http.Error(w, "transient shared discovery failure", http.StatusInternalServerError)
		case "/database/1/com.apple.photos.cloud/production/shared/records/query":
			http.Error(w, "transient per-zone probe failure", http.StatusInternalServerError)
		default:
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	})

	saveJSONCache(ps.client.CacheDir(), "libraries.json", []cachedLibraryEntry{{ZoneName: "PrimarySync", Area: areaPrivate}, {ZoneName: "SharedSync-cached", Area: areaShared}})

	libs, err := ps.GetLibraries(ctx)
	require.NoError(t, err)
	assert.Contains(t, libs, "PrimarySync")
	assert.Contains(t, libs, "SharedSync-cached")

	data, err := os.ReadFile(filepath.Join(ps.client.CacheDir(), "libraries.json"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "SharedSync-cached")
}

func TestGetLibraries_DropsCachedSharedZoneOnZoneNotFoundProbe(t *testing.T) {
	setTestCacheDir(t)
	ctx := context.Background()

	ps := newHTTPTestPhotosService(t, "library-shared-drop", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/database/1/com.apple.photos.cloud/production/private/changes/database":
			writeJSON(t, w, map[string]any{"zones": []map[string]any{{"zoneID": map[string]any{"zoneName": "PrimarySync", "ownerRecordName": "owner", "zoneType": "REGULAR_CUSTOM_ZONE"}}}})
		case "/database/1/com.apple.photos.cloud/production/shared/changes/database":
			http.Error(w, "transient shared discovery failure", http.StatusInternalServerError)
		case "/database/1/com.apple.photos.cloud/production/shared/records/query":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{
				"serverErrorCode": "ZONE_NOT_FOUND",
				"reason": "Zone does not exist",
				"errorClass": "ZONE_NOT_FOUND",
				"error": "ZONE_NOT_FOUND"
			}`))
		default:
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	})

	saveJSONCache(ps.client.CacheDir(), "libraries.json", []cachedLibraryEntry{{ZoneName: "PrimarySync", Area: areaPrivate}, {ZoneName: "SharedSync-cached", Area: areaShared, OwnerRecordName: "owner", ZoneType: "REGULAR_CUSTOM_ZONE"}})

	libs, err := ps.GetLibraries(ctx)
	require.NoError(t, err)
	assert.Contains(t, libs, "PrimarySync")
	assert.NotContains(t, libs, "SharedSync-cached")

	data, err := os.ReadFile(filepath.Join(ps.client.CacheDir(), "libraries.json"))
	require.NoError(t, err)
	assert.NotContains(t, string(data), "SharedSync-cached")
}

func TestGetAlbums_RetriesAfterTransientUserAlbumQueryFailure(t *testing.T) {
	setTestCacheDir(t)
	ctx := context.Background()
	var topQueries atomic.Int32

	ps := newHTTPTestPhotosService(t, "albums-retry-top", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/database/1/com.apple.photos.cloud/production/private/records/query" {
			http.Error(w, "unexpected request", http.StatusInternalServerError)
			return
		}
		var body struct {
			Query struct {
				FilterBy []struct {
					FieldName  string `json:"fieldName"`
					Comparator string `json:"comparator"`
					FieldValue struct {
						Value string `json:"value"`
					} `json:"fieldValue"`
				} `json:"filterBy"`
			} `json:"query"`
		}
		if err := readJSONBody(r, &body); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if len(body.Query.FilterBy) != 0 {
			http.Error(w, "unexpected child query", http.StatusInternalServerError)
			return
		}
		if topQueries.Add(1) == 1 {
			http.Error(w, "transient top-level album failure", http.StatusInternalServerError)
			return
		}
		writeJSON(t, w, map[string]any{
			"records": []map[string]any{testAlbumRecordJSON("User Album", "user-record", 0, "")},
		})
	})

	lib := &Library{service: ps, zoneID: "PrimarySync", area: areaPrivate, albums: map[string]*Album{}}
	_, err := lib.GetAlbums(ctx)
	assert.Error(t, err, "private library failure should surface as error")
	albums, err := lib.GetAlbums(ctx)
	require.NoError(t, err)
	assert.Contains(t, albums, "User Album")
	assert.GreaterOrEqual(t, topQueries.Load(), int32(2), "second GetAlbums call must retry transient top-level failures")
}

func TestGetAlbums_SharedSyncQueryFailureReturnsSmartAlbumsOnly(t *testing.T) {
	setTestCacheDir(t)
	ctx := context.Background()

	ps := newHTTPTestPhotosService(t, "albums-shared-fallback", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{
			"serverErrorCode": "BAD_REQUEST",
			"reason": "Index has invalid data",
			"errorClass": "BAD_REQUEST",
			"error": "BAD_REQUEST"
		}`))
	})

	lib := &Library{service: ps, zoneID: "SharedSync-test", area: areaPrivate, albums: map[string]*Album{}}
	albums, err := lib.GetAlbums(ctx)
	require.NoError(t, err)
	assert.Contains(t, albums, "All Photos")
	assert.NotContains(t, albums, "User Album")
	assert.Empty(t, lib.albums, "SharedSync fallback should not cache partial album results")
}

func TestGetAlbums_SharedSyncUnexpectedFailureReturnsError(t *testing.T) {
	setTestCacheDir(t)
	ctx := context.Background()

	ps := newHTTPTestPhotosService(t, "albums-shared-error", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unexpected shared failure", http.StatusInternalServerError)
	})

	lib := &Library{service: ps, zoneID: "SharedSync-test", area: areaPrivate, albums: map[string]*Album{}}
	_, err := lib.GetAlbums(ctx)
	assert.Error(t, err)
	assert.Empty(t, lib.albums, "SharedSync unexpected failures must not cache partial album results")
}

func TestGetAlbums_RetriesAfterTransientFolderChildQueryFailure(t *testing.T) {
	setTestCacheDir(t)
	ctx := context.Background()
	var childQueries atomic.Int32

	ps := newHTTPTestPhotosService(t, "albums-retry-folder", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/database/1/com.apple.photos.cloud/production/private/records/query" {
			http.Error(w, "unexpected request", http.StatusInternalServerError)
			return
		}
		var body struct {
			Query struct {
				FilterBy []struct {
					FieldName  string `json:"fieldName"`
					Comparator string `json:"comparator"`
					FieldValue struct {
						Value string `json:"value"`
					} `json:"fieldValue"`
				} `json:"filterBy"`
			} `json:"query"`
		}
		if err := readJSONBody(r, &body); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		parentID := ""
		for _, filter := range body.Query.FilterBy {
			if filter.FieldName == "parentId" {
				parentID = filter.FieldValue.Value
			}
		}
		switch parentID {
		case "":
			writeJSON(t, w, map[string]any{
				"records": []map[string]any{testAlbumRecordJSON("Folder", "folder-record", albumTypeFolder, "")},
			})
		case "folder-record":
			if childQueries.Add(1) == 1 {
				http.Error(w, "transient child query failure", http.StatusInternalServerError)
				return
			}
			writeJSON(t, w, map[string]any{
				"records": []map[string]any{testAlbumRecordJSON("Child", "child-record", 0, "folder-record")},
			})
		default:
			http.Error(w, "unexpected parentId", http.StatusInternalServerError)
		}
	})

	lib := &Library{service: ps, zoneID: "PrimarySync", area: areaPrivate, albums: map[string]*Album{}}
	_, _ = lib.GetAlbums(ctx)
	albums, err := lib.GetAlbums(ctx)
	require.NoError(t, err)
	folder, ok := albums["Folder"]
	require.True(t, ok)
	require.True(t, folder.IsFolder)
	assert.Contains(t, folder.Children, "Child")
	assert.GreaterOrEqual(t, childQueries.Load(), int32(2), "second GetAlbums call must retry transient child query failures")
}

func TestBuildPhotos(t *testing.T) {
	filename := base64.StdEncoding.EncodeToString([]byte("IMG_0001.JPG"))

	master := &photoRecord{
		RecordName: "master123",
		RecordType: "CPLMaster",
	}
	master.Fields.FilenameEnc = &ckStringField{Value: filename, Type: "ENCRYPTED_BYTES"}
	master.Fields.ResOriginalRes = &ckResourceField{Value: struct {
		Size        int64  `json:"size"`
		DownloadURL string `json:"downloadURL"`
	}{Size: 1024, DownloadURL: "https://example.com/photo"}}
	master.Fields.ResOriginalWidth = &ckIntField{Value: 4032}
	master.Fields.ResOriginalHeight = &ckIntField{Value: 3024}

	asset := &photoRecord{}
	asset.Fields.AssetDate = &ckTimestampField{Value: 1700000000000}
	asset.Fields.AddedDate = &ckTimestampField{Value: 1700000001000}

	t.Run("basic photo", func(t *testing.T) {
		photos := buildPhotos(master, asset)
		require.Len(t, photos, 1)
		p := photos[0]
		assert.Equal(t, "master123", p.ID)
		assert.Equal(t, "IMG_0001.JPG", p.Filename)
		assert.Equal(t, int64(1024), p.Size)
		assert.Equal(t, 4032, p.Width)
		assert.Equal(t, 3024, p.Height)
		assert.Equal(t, int64(1700000000000), p.AssetDate)
		assert.Equal(t, int64(1700000001000), p.AddedDate)
		assert.Equal(t, "resOriginalRes", p.ResourceKey)
	})

	t.Run("live photo with MOV companion", func(t *testing.T) {
		liveMaster := *master // copy to avoid mutating shared fixture
		liveMaster.Fields.ResOriginalVidComplRes = &ckResourceField{Value: struct {
			Size        int64  `json:"size"`
			DownloadURL string `json:"downloadURL"`
		}{Size: 2048, DownloadURL: "https://example.com/video"}}

		photos := buildPhotos(&liveMaster, asset)
		require.Len(t, photos, 2)

		assert.Equal(t, "IMG_0001.JPG", photos[0].Filename)
		assert.Equal(t, "resOriginalRes", photos[0].ResourceKey)

		assert.Equal(t, "IMG_0001.MOV", photos[1].Filename)
		assert.Equal(t, "resOriginalVidComplRes", photos[1].ResourceKey)
		assert.Equal(t, int64(2048), photos[1].Size)
		assert.Equal(t, "master123", photos[1].ID)
	})

	t.Run("no download URL returns nil", func(t *testing.T) {
		noURL := &photoRecord{RecordName: "no-url"}
		noURL.Fields.FilenameEnc = master.Fields.FilenameEnc
		photos := buildPhotos(noURL, nil)
		assert.Nil(t, photos)
	})

	t.Run("no filename returns nil", func(t *testing.T) {
		noName := &photoRecord{RecordName: "no-name"}
		noName.Fields.ResOriginalRes = master.Fields.ResOriginalRes
		photos := buildPhotos(noName, nil)
		assert.Nil(t, photos)
	})

	t.Run("nil asset is handled", func(t *testing.T) {
		photos := buildPhotos(master, nil)
		require.Len(t, photos, 1)
		assert.Equal(t, int64(0), photos[0].AssetDate)
	})

	t.Run("invalid base64 filename returns nil", func(t *testing.T) {
		bad := &photoRecord{RecordName: "bad-b64"}
		bad.Fields.FilenameEnc = &ckStringField{Value: "!!!not-base64!!!", Type: "ENCRYPTED_BYTES"}
		bad.Fields.ResOriginalRes = master.Fields.ResOriginalRes
		photos := buildPhotos(bad, nil)
		assert.Nil(t, photos)
	})

	t.Run("STRING type filenameEnc used as-is", func(t *testing.T) {
		rec := &photoRecord{RecordName: "str-name"}
		rec.Fields.FilenameEnc = &ckStringField{Value: "plain_photo.heic", Type: "STRING"}
		rec.Fields.ResOriginalRes = master.Fields.ResOriginalRes
		photos := buildPhotos(rec, nil)
		require.Len(t, photos, 1)
		assert.Equal(t, "plain_photo.heic", photos[0].Filename)
	})

	t.Run("NFD filename normalized to NFC", func(t *testing.T) {
		// "Café.heic" in NFD: e + combining acute accent (U+0301)
		nfd := "Cafe\u0301.heic"
		nfc := "Caf\u00e9.heic"

		rec := &photoRecord{RecordName: "nfd-enc"}
		rec.Fields.FilenameEnc = &ckStringField{Value: base64.StdEncoding.EncodeToString([]byte(nfd)), Type: "ENCRYPTED_BYTES"}
		rec.Fields.ResOriginalRes = master.Fields.ResOriginalRes
		photos := buildPhotos(rec, nil)
		require.Len(t, photos, 1)
		assert.Equal(t, nfc, photos[0].Filename)

		rec2 := &photoRecord{RecordName: "nfd-str"}
		rec2.Fields.FilenameEnc = &ckStringField{Value: nfd, Type: "STRING"}
		rec2.Fields.ResOriginalRes = master.Fields.ResOriginalRes
		photos2 := buildPhotos(rec2, nil)
		require.Len(t, photos2, 1)
		assert.Equal(t, nfc, photos2[0].Filename)
	})

	t.Run("itemType fallback when filenameEnc missing", func(t *testing.T) {
		rec := &photoRecord{RecordName: "AaBbCcDd1234"}
		rec.Fields.ItemType = &ckStringField{Value: "public.heic"}
		rec.Fields.ResOriginalRes = master.Fields.ResOriginalRes
		photos := buildPhotos(rec, nil)
		require.Len(t, photos, 1)
		assert.Equal(t, "AaBbCcDd1234.heic", photos[0].Filename)
	})

	t.Run("unknown itemType no fallback returns nil", func(t *testing.T) {
		rec := &photoRecord{RecordName: "unknown-uti"}
		rec.Fields.ItemType = &ckStringField{Value: "com.unknown.weird-format"}
		rec.Fields.ResOriginalRes = master.Fields.ResOriginalRes
		photos := buildPhotos(rec, nil)
		assert.Nil(t, photos)
	})

	t.Run("edited photo version from adjustmentType", func(t *testing.T) {
		editAsset := &photoRecord{RecordName: "asset-edited-123"}
		editAsset.Fields.AssetDate = asset.Fields.AssetDate
		editAsset.Fields.AdjustmentType = &ckStringField{Value: "com.apple.photos"}
		editAsset.Fields.ResJPEGFullRes = &ckResourceField{Value: struct {
			Size        int64  `json:"size"`
			DownloadURL string `json:"downloadURL"`
		}{Size: 512, DownloadURL: "https://example.com/edited"}}
		editAsset.Fields.ResJPEGFullFileType = &ckStringField{Value: "public.jpeg"}

		photos := buildPhotos(master, editAsset)
		require.Len(t, photos, 2)
		assert.Equal(t, "IMG_0001.JPG", photos[0].Filename)
		assert.Equal(t, "resOriginalRes", photos[0].ResourceKey)
		assert.Equal(t, "IMG_0001-edited.jpg", photos[1].Filename)
		assert.Equal(t, "resJPEGFullRes", photos[1].ResourceKey)
		assert.Equal(t, int64(512), photos[1].Size)
		assert.Equal(t, "asset-edited-123", photos[1].ID)
	})

	t.Run("slo-mo skips edited version", func(t *testing.T) {
		slomoAsset := &photoRecord{RecordName: "asset-slomo"}
		slomoAsset.Fields.AssetDate = asset.Fields.AssetDate
		slomoAsset.Fields.AdjustmentType = &ckStringField{Value: "com.apple.video.slomo"}
		photos := buildPhotos(master, slomoAsset)
		require.Len(t, photos, 1)
		assert.Equal(t, "IMG_0001.JPG", photos[0].Filename)
	})

	t.Run("RAW alternative from resOriginalAltRes", func(t *testing.T) {
		rawMaster := &photoRecord{RecordName: "master-raw"}
		rawMaster.Fields.FilenameEnc = master.Fields.FilenameEnc
		rawMaster.Fields.ResOriginalRes = master.Fields.ResOriginalRes
		rawMaster.Fields.ResOriginalWidth = master.Fields.ResOriginalWidth
		rawMaster.Fields.ResOriginalHeight = master.Fields.ResOriginalHeight
		rawMaster.Fields.ResOriginalAltRes = &ckResourceField{Value: struct {
			Size        int64  `json:"size"`
			DownloadURL string `json:"downloadURL"`
		}{Size: 8192, DownloadURL: "https://example.com/raw"}}
		rawMaster.Fields.ResOriginalAltFileType = &ckStringField{Value: "com.canon.cr2-raw-image"}

		photos := buildPhotos(rawMaster, nil)
		require.Len(t, photos, 2)
		assert.Equal(t, "IMG_0001.JPG", photos[0].Filename)
		assert.Equal(t, "resOriginalRes", photos[0].ResourceKey)
		assert.Equal(t, "IMG_0001.cr2", photos[1].Filename)
		assert.Equal(t, "resOriginalAltRes", photos[1].ResourceKey)
		assert.Equal(t, int64(8192), photos[1].Size)
		assert.Equal(t, 4032, photos[1].Width, "RAW alt must inherit width from original")
		assert.Equal(t, 3024, photos[1].Height, "RAW alt must inherit height from original")
	})

	t.Run("RAW alt same extension gets -alt suffix", func(t *testing.T) {
		dupMaster := &photoRecord{RecordName: "master-dup-ext"}
		dupMaster.Fields.FilenameEnc = master.Fields.FilenameEnc
		dupMaster.Fields.ResOriginalRes = master.Fields.ResOriginalRes
		dupMaster.Fields.ResOriginalAltRes = &ckResourceField{Value: struct {
			Size        int64  `json:"size"`
			DownloadURL string `json:"downloadURL"`
		}{Size: 4096, DownloadURL: "https://example.com/alt"}}
		dupMaster.Fields.ResOriginalAltFileType = &ckStringField{Value: "public.jpeg"}

		photos := buildPhotos(dupMaster, nil)
		require.Len(t, photos, 2)
		assert.Equal(t, "IMG_0001.JPG", photos[0].Filename)
		// Same extension would collide, so -alt suffix added
		assert.Equal(t, "IMG_0001-alt.jpg", photos[1].Filename)
	})

	t.Run("edited video via resVidFullRes", func(t *testing.T) {
		vidAsset := &photoRecord{RecordName: "asset-vid-edit"}
		vidAsset.Fields.AssetDate = asset.Fields.AssetDate
		vidAsset.Fields.AdjustmentType = &ckStringField{Value: "com.apple.photos"}
		vidAsset.Fields.ResVidFullRes = &ckResourceField{Value: struct {
			Size        int64  `json:"size"`
			DownloadURL string `json:"downloadURL"`
		}{Size: 2048, DownloadURL: "https://example.com/vid-edit"}}
		vidAsset.Fields.ResVidFullFileType = &ckStringField{Value: "public.mpeg-4"}

		photos := buildPhotos(master, vidAsset)
		require.Len(t, photos, 2)
		assert.Equal(t, "IMG_0001.JPG", photos[0].Filename)
		assert.Equal(t, "IMG_0001-edited.mp4", photos[1].Filename)
		assert.Equal(t, "resVidFullRes", photos[1].ResourceKey)
	})

	t.Run("edited + RAW on same photo", func(t *testing.T) {
		rawMaster := &photoRecord{RecordName: "master-combo"}
		rawMaster.Fields.FilenameEnc = master.Fields.FilenameEnc
		rawMaster.Fields.ResOriginalRes = master.Fields.ResOriginalRes
		rawMaster.Fields.ResOriginalAltRes = &ckResourceField{Value: struct {
			Size        int64  `json:"size"`
			DownloadURL string `json:"downloadURL"`
		}{Size: 8192, DownloadURL: "https://example.com/raw"}}
		rawMaster.Fields.ResOriginalAltFileType = &ckStringField{Value: "com.nikon.raw-image"}

		comboAsset := &photoRecord{RecordName: "asset-combo"}
		comboAsset.Fields.AssetDate = asset.Fields.AssetDate
		comboAsset.Fields.AdjustmentType = &ckStringField{Value: "com.apple.photos"}
		comboAsset.Fields.ResJPEGFullRes = &ckResourceField{Value: struct {
			Size        int64  `json:"size"`
			DownloadURL string `json:"downloadURL"`
		}{Size: 512, DownloadURL: "https://example.com/edited"}}
		comboAsset.Fields.ResJPEGFullFileType = &ckStringField{Value: "public.jpeg"}

		photos := buildPhotos(rawMaster, comboAsset)
		require.Len(t, photos, 3)
		assert.Equal(t, "IMG_0001.JPG", photos[0].Filename)
		assert.Equal(t, "IMG_0001-edited.jpg", photos[1].Filename)
		assert.Equal(t, "IMG_0001.nef", photos[2].Filename)
	})

	t.Run("RAW alt with nil file type uses original extension", func(t *testing.T) {
		noTypeMaster := &photoRecord{RecordName: "master-notype"}
		noTypeMaster.Fields.FilenameEnc = master.Fields.FilenameEnc
		noTypeMaster.Fields.ResOriginalRes = master.Fields.ResOriginalRes
		noTypeMaster.Fields.ResOriginalAltRes = &ckResourceField{Value: struct {
			Size        int64  `json:"size"`
			DownloadURL string `json:"downloadURL"`
		}{Size: 4096, DownloadURL: "https://example.com/alt-notype"}}
		// No ResOriginalAltFileType set

		photos := buildPhotos(noTypeMaster, nil)
		require.Len(t, photos, 2)
		assert.Equal(t, "IMG_0001.JPG", photos[0].Filename)
		// Same extension collision triggers -alt suffix
		assert.Equal(t, "IMG_0001-alt.JPG", photos[1].Filename)
	})

	t.Run("extensionless filename gets .MOV companion", func(t *testing.T) {
		noExt := &photoRecord{RecordName: "no-ext"}
		noExt.Fields.FilenameEnc = &ckStringField{Value: base64.StdEncoding.EncodeToString([]byte("IMG_NOEXT")), Type: "ENCRYPTED_BYTES"}
		noExt.Fields.ResOriginalRes = master.Fields.ResOriginalRes
		noExt.Fields.ResOriginalVidComplRes = &ckResourceField{Value: struct {
			Size        int64  `json:"size"`
			DownloadURL string `json:"downloadURL"`
		}{Size: 512, DownloadURL: "https://example.com/v"}}
		photos := buildPhotos(noExt, nil)
		require.Len(t, photos, 2)
		assert.Equal(t, "IMG_NOEXT", photos[0].Filename)
		assert.Equal(t, "IMG_NOEXT.MOV", photos[1].Filename)
	})
}

func TestParsePhotoRecords(t *testing.T) {
	filename := base64.StdEncoding.EncodeToString([]byte("IMG_0001.JPG"))

	records := []photoRecord{
		{RecordName: "master1", RecordType: "CPLMaster"},
		{RecordName: "asset1", RecordType: "CPLAsset"},
		{RecordName: "master2", RecordType: "CPLMaster"},
		{RecordName: "asset2", RecordType: "CPLAsset"},
	}
	// Set up master fields
	for i := range records {
		if records[i].RecordType == "CPLMaster" {
			records[i].Fields.FilenameEnc = &ckStringField{Value: filename, Type: "ENCRYPTED_BYTES"}
			records[i].Fields.ResOriginalRes = &ckResourceField{Value: struct {
				Size        int64  `json:"size"`
				DownloadURL string `json:"downloadURL"`
			}{Size: 1024, DownloadURL: "https://example.com/dl"}}
		}
	}
	// Link assets to masters
	records[1].Fields.MasterRef = &ckReferenceField{Value: struct {
		RecordName string `json:"recordName"`
	}{RecordName: "master1"}}
	records[3].Fields.MasterRef = &ckReferenceField{Value: struct {
		RecordName string `json:"recordName"`
	}{RecordName: "master2"}}

	photos := parsePhotoRecords(records)
	require.Len(t, photos, 2)
	assert.Equal(t, "master1", photos[0].ID)
	assert.Equal(t, "master2", photos[1].ID)
	assert.Equal(t, "IMG_0001.JPG", photos[0].Filename)

	t.Run("orphan master without asset", func(t *testing.T) {
		orphan := []photoRecord{
			{RecordName: "orphan-master", RecordType: "CPLMaster"},
		}
		orphan[0].Fields.FilenameEnc = &ckStringField{Value: filename, Type: "ENCRYPTED_BYTES"}
		orphan[0].Fields.ResOriginalRes = records[0].Fields.ResOriginalRes
		photos := parsePhotoRecords(orphan)
		require.Len(t, photos, 1)
		assert.Equal(t, "orphan-master", photos[0].ID)
	})

	t.Run("empty records", func(t *testing.T) {
		photos := parsePhotoRecords(nil)
		assert.Nil(t, photos)
	})
}

func TestClassifySmartAlbums(t *testing.T) {
	makeMaster := func(fileType string) *photoRecord {
		m := &photoRecord{}
		if fileType != "" {
			m.Fields.ResOriginalFileType = &ckStringField{Value: fileType}
		}
		return m
	}
	makeAsset := func(subtype, subtypeV2, favorite, hidden int) *photoRecord {
		a := &photoRecord{}
		a.Fields.AssetSubtype = &ckIntField{Value: subtype}
		a.Fields.AssetSubtypeV2 = &ckIntField{Value: subtypeV2}
		a.Fields.IsFavorite = &ckIntField{Value: favorite}
		a.Fields.IsHidden = &ckIntField{Value: hidden}
		return a
	}

	tests := []struct {
		name     string
		master   *photoRecord
		asset    *photoRecord
		expected []string
	}{
		{"regular photo", makeMaster("public.jpeg"), makeAsset(0, 0, 0, 0), []string{"All Photos"}},
		{"video mpeg4", makeMaster("public.mpeg-4"), makeAsset(0, 0, 0, 0), []string{"All Photos", "Videos"}},
		{"video quicktime", makeMaster("com.apple.quicktime-movie"), makeAsset(0, 0, 0, 0), []string{"All Photos", "Videos"}},
		{"favorite photo", makeMaster("public.jpeg"), makeAsset(0, 0, 1, 0), []string{"All Photos", "Favorites"}},
		{"hidden photo", makeMaster("public.jpeg"), makeAsset(0, 0, 0, 1), []string{"Hidden"}},
		{"screenshot", makeMaster("public.jpeg"), makeAsset(0, 3, 0, 0), []string{"All Photos", "Screenshots"}},
		{"favorited screenshot", makeMaster("public.jpeg"), makeAsset(0, 3, 1, 0), []string{"All Photos", "Favorites", "Screenshots"}},
		{"slo-mo", makeMaster("public.mpeg-4"), makeAsset(100, 0, 0, 0), []string{"All Photos", "Slo-mo"}},
		{"time-lapse", makeMaster("public.mpeg-4"), makeAsset(101, 0, 0, 0), []string{"All Photos", "Time-lapse"}},
		{"panorama", makeMaster("public.jpeg"), makeAsset(1, 0, 0, 0), []string{"All Photos", "Panoramas"}},
		{"live photo", makeMaster("public.jpeg"), makeAsset(0, 2, 0, 0), []string{"All Photos", "Live"}},
		{"nil asset", makeMaster("public.jpeg"), nil, []string{"All Photos"}},
		{"no file type", makeMaster(""), makeAsset(0, 0, 0, 0), []string{"All Photos"}},
		{"hidden favorite", makeMaster("public.jpeg"), makeAsset(0, 0, 1, 1), []string{"Hidden", "Favorites"}},
		{"burst photo", makeMaster("public.jpeg"), func() *photoRecord {
			a := makeAsset(0, 0, 0, 0)
			a.Fields.BurstID = &ckStringField{Value: "B7A3F2E1-4D5C-6789-ABCD-EF0123456789"}
			return a
		}(), []string{"All Photos", "Bursts"}},
		{"portrait photo", makeMaster("public.jpeg"), func() *photoRecord {
			a := makeAsset(0, 0, 0, 0)
			a.Fields.AdjustmentRenderType = &ckIntField{Value: 2} // PORTRAIT bit
			return a
		}(), []string{"All Photos", "Portrait"}},
		{"long exposure", makeMaster("public.jpeg"), func() *photoRecord {
			a := makeAsset(0, 0, 0, 0)
			a.Fields.AdjustmentRenderType = &ckIntField{Value: 4} // LONG_EXPOSURE bit
			return a
		}(), []string{"All Photos", "Long Exposure"}},
		{"portrait + long exposure", makeMaster("public.jpeg"), func() *photoRecord {
			a := makeAsset(0, 0, 0, 0)
			a.Fields.AdjustmentRenderType = &ckIntField{Value: 6} // both bits
			return a
		}(), []string{"All Photos", "Portrait", "Long Exposure"}},
		{"animated gif", makeMaster("com.compuserve.gif"), makeAsset(0, 0, 0, 0), []string{"All Photos", "Animated"}},
		{"soft-deleted photo", makeMaster("public.jpeg"), func() *photoRecord {
			a := makeAsset(0, 0, 1, 0) // favorite=1 should be ignored when deleted
			a.Fields.IsDeleted = &ckIntField{Value: 1}
			return a
		}(), []string{"Recently Deleted"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifySmartAlbums(tt.master, tt.asset)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestGetPhotoByName_CacheHit(t *testing.T) {
	album := &Album{
		Name:       "Videos",
		ObjectType: "CPLAssetInSmartAlbumByAssetDate:Video",
	}
	album.photoCache = map[string]*Photo{
		"IMG_0001.JPG": {ID: "m1", Filename: "IMG_0001.JPG", Size: 1024},
		"IMG_0002.MOV": {ID: "m2", Filename: "IMG_0002.MOV", Size: 2048},
	}

	t.Run("found in cache", func(t *testing.T) {
		photo, err := album.GetPhotoByName(context.Background(), "IMG_0001.JPG")
		require.NoError(t, err)
		assert.Equal(t, "m1", photo.ID)
		assert.Equal(t, int64(1024), photo.Size)
	})

	t.Run("not in cache", func(t *testing.T) {
		// GetPhotoByName with a populated cache but missing filename
		// should return error (cache exists but name not found)
		_, err := album.GetPhotoByName(context.Background(), "MISSING.JPG")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "MISSING.JPG")
	})
}

func TestDeduplicateFilenames(t *testing.T) {
	t.Run("no duplicates unchanged", func(t *testing.T) {
		photos := []*Photo{
			{ID: "master1", Filename: "IMG_0001.JPG"},
			{ID: "master2", Filename: "IMG_0002.JPG"},
		}
		deduplicateFilenames(photos)
		assert.Equal(t, "IMG_0001.JPG", photos[0].Filename)
		assert.Equal(t, "IMG_0002.JPG", photos[1].Filename)
	})

	t.Run("all duplicates get full ID suffix", func(t *testing.T) {
		photos := []*Photo{
			{ID: "AQJ2Fq0Px7pGM", Filename: "camphoto_001.jpg"},
			{ID: "BRK3Gr1Qy8qHN", Filename: "camphoto_001.jpg"},
			{ID: "CSL4Hs2Rz9rIO", Filename: "camphoto_001.jpg"},
		}
		deduplicateFilenames(photos)
		assert.Equal(t, "camphoto_001_AQJ2Fq0Px7pGM.jpg", photos[0].Filename)
		assert.Equal(t, "camphoto_001_BRK3Gr1Qy8qHN.jpg", photos[1].Filename)
		assert.Equal(t, "camphoto_001_CSL4Hs2Rz9rIO.jpg", photos[2].Filename)
	})

	t.Run("no extension handled", func(t *testing.T) {
		photos := []*Photo{
			{ID: "masterA", Filename: "noext"},
			{ID: "masterB", Filename: "noext"},
		}
		deduplicateFilenames(photos)
		assert.Equal(t, "noext_masterA", photos[0].Filename)
		assert.Equal(t, "noext_masterB", photos[1].Filename)
	})

	t.Run("deterministic across runs", func(t *testing.T) {
		mk := func() []*Photo {
			return []*Photo{
				{ID: "AQJ2Fq0Px7pGM", Filename: "dup.jpg"},
				{ID: "BRK3Gr1Qy8qHN", Filename: "dup.jpg"},
			}
		}
		p1 := mk()
		p2 := mk()
		deduplicateFilenames(p1)
		deduplicateFilenames(p2)
		assert.Equal(t, p1[0].Filename, p2[0].Filename)
		assert.Equal(t, p1[1].Filename, p2[1].Filename)
	})

	t.Run("stable when new duplicate added", func(t *testing.T) {
		// Before: two duplicates
		before := []*Photo{
			{ID: "AQJ2Fq0Px7pGM", Filename: "dup.jpg"},
			{ID: "BRK3Gr1Qy8qHN", Filename: "dup.jpg"},
		}
		deduplicateFilenames(before)
		nameA := before[0].Filename
		nameB := before[1].Filename

		// After: third duplicate added - existing names must not change
		after := []*Photo{
			{ID: "AQJ2Fq0Px7pGM", Filename: "dup.jpg"},
			{ID: "BRK3Gr1Qy8qHN", Filename: "dup.jpg"},
			{ID: "CSL4Hs2Rz9rIO", Filename: "dup.jpg"},
		}
		deduplicateFilenames(after)
		assert.Equal(t, nameA, after[0].Filename, "existing file A must keep same name")
		assert.Equal(t, nameB, after[1].Filename, "existing file B must keep same name")
	})
}

func TestDeduplicateFilenamesSharedPointers(t *testing.T) {
	// Photos classified into multiple smart albums share *Photo pointers
	// deduplicateFilenames mutates Filename in place, so shared pointers
	// across albums get cross-contaminated unless deep-copied first
	shared := &Photo{ID: "master1", Filename: "dup.jpg"}
	other := &Photo{ID: "master2", Filename: "dup.jpg"}

	albumA := []*Photo{shared, other}
	albumB := []*Photo{shared} // same pointer, unique in this album

	// Simulate the fix: deep-copy before dedup (same as applyPendingDelta)
	dedupedA := make([]*Photo, len(albumA))
	for i, p := range albumA {
		cp := *p
		dedupedA[i] = &cp
	}
	deduplicateFilenames(dedupedA)

	// albumA's dedup copies got suffixes
	assert.Contains(t, dedupedA[0].Filename, "_master1")
	assert.Contains(t, dedupedA[1].Filename, "_master2")

	// Original shared pointer is untouched - albumB sees clean filename
	assert.Equal(t, "dup.jpg", albumB[0].Filename, "deep copy must prevent cross-album contamination")
	assert.Equal(t, "dup.jpg", shared.Filename, "original pointer must be unmodified")
}

func TestDeltaContainsAlbumChanges(t *testing.T) {
	t.Run("CPLAlbum record triggers invalidation", func(t *testing.T) {
		records := []json.RawMessage{
			json.RawMessage(`{"recordName":"master1","recordType":"CPLMaster"}`),
			json.RawMessage(`{"recordName":"asset1","recordType":"CPLAsset"}`),
			json.RawMessage(`{"recordName":"album1","recordType":"CPLAlbum"}`),
		}
		assert.True(t, deltaContainsAlbumChanges(records))
	})

	t.Run("no CPLAlbum does not trigger", func(t *testing.T) {
		records := []json.RawMessage{
			json.RawMessage(`{"recordName":"master1","recordType":"CPLMaster"}`),
			json.RawMessage(`{"recordName":"asset1","recordType":"CPLAsset"}`),
			json.RawMessage(`{"recordName":"rel1","recordType":"CPLContainerRelation"}`),
		}
		assert.False(t, deltaContainsAlbumChanges(records))
	})

	t.Run("empty records does not trigger", func(t *testing.T) {
		assert.False(t, deltaContainsAlbumChanges(nil))
	})

	t.Run("malformed JSON skipped", func(t *testing.T) {
		records := []json.RawMessage{
			json.RawMessage(`{invalid`),
			json.RawMessage(`{"recordName":"album1","recordType":"CPLAlbum"}`),
		}
		assert.True(t, deltaContainsAlbumChanges(records))
	})
}

func TestParseDeltaRecords(t *testing.T) {
	t.Run("deleted CPLAsset marks both asset and master IDs", func(t *testing.T) {
		// Edited entries use asset.RecordName as ID; ghost entries survive
		// if only masterID is in deletedIDs
		records := []json.RawMessage{
			json.RawMessage(`{"recordName":"master-AAA","recordType":"CPLMaster","deleted":true}`),
			json.RawMessage(`{"recordName":"asset-BBB","recordType":"CPLAsset","deleted":true,"fields":{"masterRef":{"value":{"recordName":"master-AAA"}}}}`),
		}
		r := parseDeltaRecords(records)
		assert.True(t, r.deletedIDs["master-AAA"], "master ID from CPLMaster deletion")
		assert.True(t, r.deletedIDs["asset-BBB"], "asset ID from CPLAsset deletion (for -edited entries)")
	})

	t.Run("deleted CPLAsset without masterRef still tracks asset ID", func(t *testing.T) {
		// CloudKit may strip fields from deleted records
		records := []json.RawMessage{
			json.RawMessage(`{"recordName":"asset-CCC","recordType":"CPLAsset","deleted":true}`),
		}
		r := parseDeltaRecords(records)
		assert.True(t, r.deletedIDs["asset-CCC"], "asset ID tracked even without masterRef")
		assert.Equal(t, 1, len(r.deletedIDs), "only asset ID, no master")
	})

	t.Run("deleted CPLContainerRelation extracts album from recordName", func(t *testing.T) {
		// Deleted relations lack fields; album parsed from "assetID-IN-albumRecordName"
		records := []json.RawMessage{
			json.RawMessage(`{"recordName":"assetXYZ-IN-album-uuid-456","recordType":"CPLContainerRelation","deleted":true}`),
		}
		r := parseDeltaRecords(records)
		assert.True(t, r.changedAlbumRecords["album-uuid-456"], "album extracted from deleted relation recordName")
	})

	t.Run("deleted relation without recordType still extracts album from recordName", func(t *testing.T) {
		// changes/zone omits recordType for deleted CPLContainerRelation records
		records := []json.RawMessage{
			json.RawMessage(`{"recordName":"assetXYZ-IN-album-uuid-654","deleted":true}`),
		}
		r := parseDeltaRecords(records)
		assert.True(t, r.changedAlbumRecords["album-uuid-654"], "album extracted from deleted relation with missing recordType")
	})

	t.Run("non-deleted CPLContainerRelation uses containerId STRING field", func(t *testing.T) {
		// CloudKit relation records expose containerId as the canonical album field
		records := []json.RawMessage{
			json.RawMessage(`{"recordName":"rel1","recordType":"CPLContainerRelation","fields":{"containerId":{"value":"album-uuid-789"}}}`),
		}
		r := parseDeltaRecords(records)
		assert.True(t, r.changedAlbumRecords["album-uuid-789"], "album extracted from containerId STRING field")
	})

	t.Run("non-deleted CPLContainerRelation falls back to recordName when fields missing", func(t *testing.T) {
		// changes/zone can return live relation records with empty fields
		records := []json.RawMessage{
			json.RawMessage(`{"recordName":"assetXYZ-IN-album-uuid-987","recordType":"CPLContainerRelation","fields":{},"deleted":false}`),
		}
		r := parseDeltaRecords(records)
		assert.True(t, r.changedAlbumRecords["album-uuid-987"], "album extracted from live relation recordName fallback")
	})

	t.Run("deleted relation with unexpected recordName format is safe", func(t *testing.T) {
		// No "-IN-" in recordName - must not panic or add garbage
		records := []json.RawMessage{
			json.RawMessage(`{"recordName":"weirdformat","recordType":"CPLContainerRelation","deleted":true}`),
		}
		r := parseDeltaRecords(records)
		assert.Empty(t, r.changedAlbumRecords, "no album invalidated for malformed recordName")
	})

	t.Run("CPLAlbum sets albumMetadataChanged", func(t *testing.T) {
		records := []json.RawMessage{
			json.RawMessage(`{"recordName":"album1","recordType":"CPLAlbum"}`),
		}
		r := parseDeltaRecords(records)
		assert.True(t, r.albumMetadataChanged)
	})

	t.Run("asset-only update detected", func(t *testing.T) {
		// CPLAsset changed but no CPLMaster in delta - triggers smart album invalidation
		records := []json.RawMessage{
			json.RawMessage(`{"recordName":"asset-DDD","recordType":"CPLAsset","fields":{"masterRef":{"value":{"recordName":"master-EEE"}}}}`),
		}
		r := parseDeltaRecords(records)
		assert.True(t, r.hasAssetOnlyUpdates, "asset without matching master = asset-only update")
	})

	t.Run("asset with matching master is not asset-only", func(t *testing.T) {
		records := []json.RawMessage{
			json.RawMessage(`{"recordName":"master-FFF","recordType":"CPLMaster","fields":{"resOriginalRes":{"value":{"downloadURL":"https://example.com"}}}}`),
			json.RawMessage(`{"recordName":"asset-GGG","recordType":"CPLAsset","fields":{"masterRef":{"value":{"recordName":"master-FFF"}}}}`),
		}
		r := parseDeltaRecords(records)
		assert.False(t, r.hasAssetOnlyUpdates)
		assert.Equal(t, 1, len(r.newMasters))
		assert.Equal(t, 1, len(r.newAssets))
	})

	t.Run("unknown record types are silently skipped", func(t *testing.T) {
		records := []json.RawMessage{
			json.RawMessage(`{"recordName":"x","recordType":"CPLSomethingNew"}`),
		}
		r := parseDeltaRecords(records)
		assert.Empty(t, r.deletedIDs)
		assert.Empty(t, r.newMasters)
		assert.Empty(t, r.newAssets)
		assert.Empty(t, r.changedAlbumRecords)
		assert.False(t, r.albumMetadataChanged)
	})

	t.Run("malformed JSON records skipped without panic", func(t *testing.T) {
		records := []json.RawMessage{
			json.RawMessage(`{invalid`),
			json.RawMessage(`{"recordName":"master1","recordType":"CPLMaster","deleted":true}`),
		}
		r := parseDeltaRecords(records)
		assert.True(t, r.deletedIDs["master1"], "valid record after malformed one still parsed")
	})

	t.Run("empty records produces empty result", func(t *testing.T) {
		r := parseDeltaRecords(nil)
		assert.Empty(t, r.deletedIDs)
		assert.False(t, r.albumMetadataChanged)
		assert.False(t, r.hasAssetOnlyUpdates)
	})
}

func TestApplyPendingDelta_EmptyAlbumsClearsPending(t *testing.T) {
	// When checkForChanges eagerly clears lib.albums (CPLAlbum delta),
	// applyPendingDelta must clear pendingDelta instead of leaving it stuck
	// Without this fix, pendingDelta stays non-nil forever and all future
	// checkForChanges calls skip (line 912: pendingDelta != nil)
	lib := &Library{
		albums: make(map[string]*Album), // empty - simulates eager clear
		pendingDelta: &deltaPayload{
			records:   []json.RawMessage{json.RawMessage(`{"recordName":"m1","recordType":"CPLMaster"}`)},
			syncToken: "old-token",
		},
	}
	result := lib.applyPendingDelta(context.Background())
	assert.False(t, result, "must return false when albums empty")
	assert.Nil(t, lib.pendingDelta, "must clear pendingDelta to avoid permanent stuck state")
}

func TestApplyPendingDelta_InvalidatesNestedAlbum(t *testing.T) {
	oldCacheDir := config.GetCacheDir()
	t.Cleanup(func() {
		_ = config.SetCacheDir(oldCacheDir)
	})
	require.NoError(t, config.SetCacheDir(t.TempDir()))

	ps := &PhotosService{client: &Client{remoteName: "delta-nested-test"}}
	lib := &Library{
		service: ps,
		zoneID:  "PrimarySync",
		area:    areaPrivate,
		albums:  make(map[string]*Album),
	}
	child := lib.newUserAlbum("Child", "child-record")
	child.SetTestPhotoCache(map[string]*Photo{
		"one.jpg": {ID: "master1", Filename: "one.jpg"},
	})
	child.saveDiskCache([]*Photo{{ID: "master1", Filename: "one.jpg"}})

	folder := &Album{
		Name:       "Folder",
		RecordName: "folder-record",
		lib:        lib,
		IsFolder:   true,
		Children: map[string]*Album{
			"Child": child,
		},
	}
	lib.albums[folder.Name] = folder
	lib.pendingDelta = &deltaPayload{
		records:   []json.RawMessage{json.RawMessage(`{"recordName":"asset123-IN-child-record","deleted":true}`)},
		syncToken: "next-token",
	}

	cacheFile := filepath.Join(lib.zoneCacheDir(), albumCacheKey(child.ObjectType)+".json")
	_, err := os.Stat(cacheFile)
	require.NoError(t, err, "nested child album cache file should exist before invalidation")

	result := lib.applyPendingDelta(context.Background())
	assert.True(t, result)
	assert.Nil(t, lib.pendingDelta)

	child.mu.Lock()
	assert.Nil(t, child.photoCache, "nested child album memory cache should be invalidated")
	child.mu.Unlock()
	_, err = os.Stat(cacheFile)
	assert.ErrorIs(t, err, os.ErrNotExist, "nested child album disk cache should be removed")
	data, err := os.ReadFile(filepath.Join(lib.zoneCacheDir(), "syncToken"))
	require.NoError(t, err)
	assert.Equal(t, "next-token", string(data))
}

func TestFlushCaches_NoPendingDeltaRace(t *testing.T) {
	// Verify FlushCaches and applyPendingDelta don't race on pendingDelta.
	// FlushCaches must acquire deltaMu before writing pendingDelta.
	ps := &PhotosService{
		client:    &Client{remoteName: "race-test"},
		libraries: make(map[string]*Library),
	}
	lib := &Library{
		service: ps,
		zoneID:  "PrimarySync",
		area:    areaPrivate,
		albums:  make(map[string]*Album),
	}
	ps.libraries["PrimarySync"] = lib

	// Run concurrent FlushCaches + applyPendingDelta
	// The race detector will flag unsynchronized access to pendingDelta
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 100; i++ {
			lib.deltaMu.Lock()
			lib.pendingDelta = &deltaPayload{
				records:   []json.RawMessage{json.RawMessage(`{"recordName":"m1","recordType":"CPLMaster"}`)},
				syncToken: "tok",
			}
			lib.deltaMu.Unlock()
			lib.applyPendingDelta(context.Background())
		}
	}()
	for i := 0; i < 100; i++ {
		ps.FlushCaches()
		// Re-add the library since FlushCaches clears it
		ps.mu.Lock()
		ps.libraries["PrimarySync"] = lib
		ps.mu.Unlock()
	}
	<-done
}

func TestBuildSmartAlbums(t *testing.T) {
	albums := buildSmartAlbums()

	// Must produce exactly 14 smart albums
	assert.Equal(t, 14, len(albums), "expected 14 smart albums")

	// All expected names present
	expected := []string{
		"All Photos", "Favorites", "Videos", "Screenshots", "Live",
		"Slo-mo", "Time-lapse", "Panoramas", "Portrait", "Long Exposure",
		"Animated", "Bursts", "Hidden", "Recently Deleted",
	}
	for _, name := range expected {
		a, ok := albums[name]
		require.True(t, ok, "missing smart album %q", name)
		assert.NotEmpty(t, a.ObjectType, "album %q has no ObjectType", name)
		assert.NotEmpty(t, a.ListType, "album %q has no ListType", name)
		assert.NotEmpty(t, a.Direction, "album %q has no Direction", name)
	}

	// Verify special albums have correct configuration
	assert.Equal(t, "DESCENDING", albums["Recently Deleted"].Direction)
	assert.Empty(t, albums["All Photos"].Filters, "All Photos should have no filters")
	assert.Empty(t, albums["Bursts"].Filters, "Bursts should have no filters")
	assert.Empty(t, albums["Hidden"].Filters, "Hidden should have no filters")

	// Verify filter-based albums have exactly one smartAlbum filter
	for _, name := range []string{"Favorites", "Videos", "Screenshots", "Live", "Slo-mo", "Time-lapse", "Panoramas", "Portrait", "Long Exposure", "Animated"} {
		a := albums[name]
		require.Len(t, a.Filters, 1, "album %q should have exactly one filter", name)
		assert.Equal(t, "smartAlbum", a.Filters[0].FieldName)
		assert.Equal(t, "EQUALS", a.Filters[0].Comparator)
	}
}

func TestAtomicWriteFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "test.json")

	// Successful write
	err := atomicWriteFile(target, []byte(`{"key":"value"}`))
	require.NoError(t, err)
	data, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, `{"key":"value"}`, string(data))

	// Overwrite existing
	err = atomicWriteFile(target, []byte(`{"key":"updated"}`))
	require.NoError(t, err)
	data, err = os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, `{"key":"updated"}`, string(data))

	// No .tmp file left behind
	_, err = os.Stat(target + ".tmp")
	assert.True(t, os.IsNotExist(err), "temp file should not persist")
}

func TestAlbumCacheKey(t *testing.T) {
	k1 := albumCacheKey("CPLAssetByAssetDateWithoutHiddenOrDeleted")
	k2 := albumCacheKey("CPLAssetInSmartAlbumByAssetDate:Video")
	k3 := albumCacheKey("CPLAssetByAssetDateWithoutHiddenOrDeleted")

	assert.Len(t, k1, 16, "cache key should be 16 hex chars")
	assert.Equal(t, k1, k3, "same input should produce same key")
	assert.NotEqual(t, k1, k2, "different input should produce different key")

	// Verify filename-safe
	for _, c := range k1 {
		assert.True(t, (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'), "key should be hex: %c", c)
	}
}
