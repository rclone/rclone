package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func membershipField(value any) map[string]any {
	return map[string]any{"value": value}
}

func membershipFixtureRecord(recordType, name string, fields map[string]any) map[string]any {
	return map[string]any{"recordType": recordType, "recordName": name, "fields": fields}
}

func membershipFixtureRelation(asset, album string) map[string]any {
	return membershipFixtureRecord("CPLContainerRelation", asset+"-IN-"+album, map[string]any{
		"containerId": membershipField(album), "itemId": membershipField(asset),
	})
}

func membershipFixtureAsset(name, master string) map[string]any {
	return membershipFixtureRecord("CPLAsset", name, map[string]any{
		"masterRef": membershipField(map[string]any{"recordName": master}),
		"assetDate": membershipField(int64(1700000000000)),
	})
}

// membershipFixtureMaster builds the CPLMaster record for a numbered photo
func membershipFixtureMaster(number int) map[string]any {
	return membershipFixtureMasterNamed(fmt.Sprintf("master-%03d", number), number)
}

func membershipFixtureMasterNamed(name string, number int) map[string]any {
	return membershipFixtureRecord("CPLMaster", name, map[string]any{
		"filenameEnc":    map[string]any{"type": "STRING", "value": fmt.Sprintf("IMG_%03d.HEIC", number)},
		"resOriginalRes": map[string]any{"value": map[string]any{"size": 1024, "downloadURL": "https://example.com/original"}},
	})
}

func membershipFixtureAlias(name, target, zone, owner string) map[string]any {
	return membershipFixtureRecord("CPLAsset", name, map[string]any{
		"isSparsePrivateRecord": membershipField(1),
		"linkedShareRecordName": membershipField(target),
		"linkedShareZoneName":   membershipField(zone),
		"linkedShareZoneOwner":  membershipField(map[string]any{"recordName": owner}),
	})
}

func membershipFixtureLookup(t *testing.T, w http.ResponseWriter, r *http.Request, records map[string]map[string]any, area string) {
	t.Helper()
	var body struct {
		ZoneID  map[string]string             `json:"zoneID"`
		Records []struct{ RecordName string } `json:"records"`
	}
	if err := readJSONBody(r, &body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	zone := body.ZoneID["zoneName"]
	if zone == "SharedSync-test" {
		assert.Contains(t, r.URL.Path, "/"+area+"/records/lookup")
		assert.Equal(t, "shared-owner", body.ZoneID["ownerRecordName"])
		assert.Equal(t, "REGULAR_CUSTOM_ZONE", body.ZoneID["zoneType"])
	}
	result := []map[string]any{}
	for _, record := range body.Records {
		// Unknown names return no entry, mirroring CloudKit's behaviour for
		// records deleted while a stale relation still references them.
		if found, ok := records[zone+"/"+record.RecordName]; ok {
			result = append(result, found)
		}
	}
	writeJSON(t, w, map[string]any{"records": result})
}

func TestCustomAlbumIncludesPersonalAndSharedAssets(t *testing.T) {
	for _, area := range []string{areaPrivate, areaShared} {
		t.Run(area, func(t *testing.T) {
			setTestCacheDir(t)
			feed := []map[string]any{}
			lookup := map[string]map[string]any{}
			for number := range 81 {
				name := fmt.Sprintf("asset-%03d", number)
				zone, master := "PrimarySync", fmt.Sprintf("master-%03d", number)
				asset := membershipFixtureAsset(name, master)
				if number >= 2 {
					zone = "SharedSync-test"
					asset = membershipFixtureAlias(name, name, zone, "shared-owner")
				}
				feed = append(feed, asset, membershipFixtureRelation(name, "album"))
				lookup[zone+"/"+name] = membershipFixtureAsset(name, master)
				lookup[zone+"/"+master] = membershipFixtureMaster(number)
				if number == 2 {
					lookup[zone+"/"+master]["fields"].(map[string]any)["resOriginalVidComplRes"] = membershipField(map[string]any{"size": 2048, "downloadURL": "https://example.com/live.mov"})
				}
			}
			ps := newHTTPTestPhotosService(t, "membership-union", func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/changes/zone") {
					writeJSON(t, w, map[string]any{"zones": []map[string]any{{"zoneID": map[string]any{"zoneName": "PrimarySync"}, "records": feed, "syncToken": "complete"}}})
					return
				}
				membershipFixtureLookup(t, w, r, lookup, area)
			})
			lib := &Library{service: ps, zoneID: "PrimarySync", area: areaPrivate, albums: map[string]*Album{}}
			ps.libraries[lib.zoneID] = lib
			ps.libraries["SharedSync-test"] = &Library{service: ps, zoneID: "SharedSync-test", area: area, ownerRecordName: "shared-owner", zoneType: "REGULAR_CUSTOM_ZONE"}
			photos, err := newUserAlbumForTest(lib, "3D", "album").GetPhotos(context.Background())
			require.NoError(t, err)
			require.Len(t, photos, 82)
			shared := 0
			companions := 0
			for _, photo := range photos {
				assert.NotEmpty(t, photo.AssetID)
				if photo.SourceZone == "SharedSync-test" {
					if photo.ResourceKey == "resOriginalRes" {
						shared++
					} else if photo.ResourceKey == "resOriginalVidComplRes" {
						companions++
						url, err := ps.LookupPhotoDownloadURL(context.Background(), photo)
						require.NoError(t, err)
						assert.Equal(t, "https://example.com/live.mov", url)
					}
					assert.Equal(t, area, photo.SourceArea)
					assert.Equal(t, "shared-owner", photo.SourceOwner)
				}
			}
			assert.Equal(t, 79, shared)
			assert.Equal(t, 1, companions)
		})
	}
}

func TestCustomAlbumMembershipBootstrapResumesAndAppliesDeletion(t *testing.T) {
	setTestCacheDir(t)
	var fail atomic.Bool
	fail.Store(true)
	var starts atomic.Int32
	ps := newHTTPTestPhotosService(t, "membership-resume", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Zones []struct{ SyncToken string } `json:"zones"`
		}
		if err := readJSONBody(r, &body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		token := body.Zones[0].SyncToken
		result := map[string]any{"zoneID": map[string]any{"zoneName": "PrimarySync"}, "syncToken": "done"}
		switch token {
		case "":
			starts.Add(1)
			result["records"] = []map[string]any{membershipFixtureRelation("one", "album")}
			result["syncToken"], result["moreComing"] = "page-two", true
		case "page-two":
			if fail.Load() {
				http.Error(w, "interrupted", http.StatusInternalServerError)
				return
			}
			result["records"] = []map[string]any{membershipFixtureRelation("two", "album")}
		case "done":
			result["records"] = []map[string]any{{"recordName": "one-IN-album", "deleted": true}}
			result["syncToken"] = "deleted"
		}
		writeJSON(t, w, map[string]any{"zones": []map[string]any{result}})
	})
	lib := &Library{service: ps, zoneID: "PrimarySync", area: areaPrivate}
	_, err := lib.refreshAlbumMembership(context.Background())
	require.Error(t, err)
	fail.Store(false)
	state, err := lib.refreshAlbumMembership(context.Background())
	require.NoError(t, err)
	assert.True(t, state.Complete)
	assert.Len(t, state.Records, 2)
	assert.Equal(t, int32(1), starts.Load())
	state, err = lib.refreshAlbumMembership(context.Background())
	require.NoError(t, err)
	assert.Len(t, state.Records, 1)
	assert.Contains(t, state.Records, "two-IN-album")
	permissions, err := os.Stat(filepath.Join(lib.zoneCacheDir(), albumMembershipCacheFile))
	require.NoError(t, err)
	if runtime.GOOS != "windows" {
		assert.Equal(t, os.FileMode(0600), permissions.Mode().Perm())
	}
}

func TestCustomAlbumSkipsDeletedMemberButKeepsLiveOnes(t *testing.T) {
	setTestCacheDir(t)
	// Relation references an asset whose record is absent from the lookup
	// response entirely: the album must still list its live members.
	lookup := map[string]map[string]any{
		"PrimarySync/live":        membershipFixtureAsset("live", "master-live"),
		"PrimarySync/master-live": membershipFixtureMasterNamed("master-live", 1),
	}
	ps := newHTTPTestPhotosService(t, "stale-relation", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/changes/zone") {
			writeJSON(t, w, map[string]any{"zones": []map[string]any{{"zoneID": map[string]any{"zoneName": "PrimarySync"}, "records": []map[string]any{
				membershipFixtureRelation("live", "album"),
				membershipFixtureRelation("gone", "album"),
			}, "syncToken": "complete"}}})
			return
		}
		membershipFixtureLookup(t, w, r, lookup, areaPrivate)
	})
	lib := &Library{service: ps, zoneID: "PrimarySync", area: areaPrivate, albums: map[string]*Album{}}
	ps.libraries[lib.zoneID] = lib
	photos, err := newUserAlbumForTest(lib, "3D", "album").GetPhotos(context.Background())
	require.NoError(t, err)
	require.Len(t, photos, 1)
	assert.Equal(t, "IMG_001.HEIC", photos[0].Filename)
}

func TestCustomAlbumFailsClosedForUnresolvedAlias(t *testing.T) {
	setTestCacheDir(t)
	ps := newHTTPTestPhotosService(t, "membership-unresolved", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{"zones": []map[string]any{{"zoneID": map[string]any{"zoneName": "PrimarySync"}, "syncToken": "done", "records": []map[string]any{
			membershipFixtureAlias("missing", "missing", "SharedSync-unknown", "owner"), membershipFixtureRelation("missing", "album"),
		}}}})
	})
	lib := &Library{service: ps, zoneID: "PrimarySync", area: areaPrivate}
	photos, err := newUserAlbumForTest(lib, "3D", "album").GetPhotos(context.Background())
	require.Error(t, err)
	assert.Nil(t, photos)
}

func TestCustomAlbumFollowsRemapAndPreservesSiblingAssets(t *testing.T) {
	setTestCacheDir(t)
	remap := membershipFixtureRecord("CPLAsset", "old", map[string]any{"remappedRef": membershipField(map[string]any{
		"recordName": "one", "zoneID": map[string]any{"zoneName": "PrimarySync"},
	})})
	feed := []map[string]any{remap, membershipFixtureAsset("one", "master-001"), membershipFixtureAsset("two", "master-001"), membershipFixtureRelation("old", "album"), membershipFixtureRelation("two", "album")}
	lookup := map[string]map[string]any{"PrimarySync/one": membershipFixtureAsset("one", "master-001"), "PrimarySync/two": membershipFixtureAsset("two", "master-001"), "PrimarySync/master-001": membershipFixtureMaster(1)}
	ps := newHTTPTestPhotosService(t, "membership-siblings", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/changes/zone") {
			writeJSON(t, w, map[string]any{"zones": []map[string]any{{"zoneID": map[string]any{"zoneName": "PrimarySync"}, "records": feed, "syncToken": "done"}}})
			return
		}
		membershipFixtureLookup(t, w, r, lookup, areaPrivate)
	})
	lib := &Library{service: ps, zoneID: "PrimarySync", area: areaPrivate}
	ps.libraries[lib.zoneID] = lib
	photos, err := newUserAlbumForTest(lib, "3D", "album").GetPhotos(context.Background())
	require.NoError(t, err)
	require.Len(t, photos, 2)
	assert.NotEqual(t, photos[0].AssetID, photos[1].AssetID)
	assert.NotEqual(t, photos[0].Filename, photos[1].Filename)
	assert.Equal(t, "master-001", photos[0].ID)
	assert.Equal(t, "master-001", photos[1].ID)
	encoded, err := json.Marshal(photos)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "downloadURL")
}

func TestAlbumMembershipOmitsDeletedAssets(t *testing.T) {
	state := &albumMembershipState{Records: make(map[string]membershipRecord)}
	asset := membershipFixtureAsset("soft-deleted", "master")
	asset["fields"].(map[string]any)["isDeleted"] = membershipField(1)
	changes := []map[string]any{membershipFixtureAsset("removed", "master"), asset,
		membershipFixtureRelation("removed", "album"), membershipFixtureRelation("soft-deleted", "album"),
		{"recordName": "removed", "deleted": true}}
	var records []json.RawMessage
	for _, change := range changes {
		encoded, err := json.Marshal(change)
		require.NoError(t, err)
		records = append(records, encoded)
	}
	require.NoError(t, applyAlbumMembershipRecords(state, records))
	members, err := albumMemberIDs(state, "album")
	require.NoError(t, err)
	assert.Empty(t, members)
}

func TestAlbumMembershipMatchesNativeSnapshot(t *testing.T) {
	snapshotPath, nativePath := os.Getenv("RCLONE_ICLOUD_MEMBERSHIP_SNAPSHOT"), os.Getenv("RCLONE_ICLOUD_NATIVE_SELECTION")
	if snapshotPath == "" || nativePath == "" {
		t.Skip("private CloudKit and native Photos comparison fixtures are not configured")
	}
	data, err := os.ReadFile(snapshotPath)
	require.NoError(t, err)
	var snapshot struct {
		Complete bool
		Records  []json.RawMessage
	}
	require.NoError(t, json.Unmarshal(data, &snapshot))
	require.True(t, snapshot.Complete, "a partial source cannot establish album completeness")
	data, err = os.ReadFile(nativePath)
	require.NoError(t, err)
	var native []struct {
		AlbumID string `json:"album_cloud_guid"`
		AssetID string `json:"cloud_asset_guid"`
	}
	require.NoError(t, json.Unmarshal(data, &native))
	require.NotEmpty(t, native)
	state := &albumMembershipState{Records: make(map[string]membershipRecord)}
	require.NoError(t, applyAlbumMembershipRecords(state, snapshot.Records))
	members, err := albumMemberIDs(state, native[0].AlbumID)
	require.NoError(t, err)
	expected := make([]string, len(native))
	for index, asset := range native {
		expected[index] = asset.AssetID
	}
	require.ElementsMatch(t, expected, members)
	t.Logf("native assets=%d; complete feed members=%d; exact identifiers match", len(native), len(members))
}
