package api

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestIsSharedFolderChildID covers the classification used to decide whether an
// item lives inside a folder shared by another Apple ID and therefore must be
// addressed through the docws endpoints by item_id rather than by drivewsid.
func TestIsSharedFolderChildID(t *testing.T) {
	for _, tc := range []struct {
		name      string
		drivewsid string
		want      bool
	}{
		{"empty is a shared child (docws-only item)", "", true},
		{"folder in shared folder", "FOLDER_IN_SHARED_FOLDER::com.apple.CloudDocs::ABC-123", true},
		{"file in shared folder", "FILE_IN_SHARED_FOLDER::com.apple.CloudDocs::ABC-123", true},
		{"own-zone folder", "FOLDER::com.apple.CloudDocs::ABC-123", false},
		{"own-zone file", "FILE::com.apple.CloudDocs::ABC-123", false},
		{"share root folder is addressable", "SHARED_FOLDER::com.apple.CloudDocs::ABC-123", false},
		{"garbage without separators", "garbage", false},
		// HasPrefix matches the docType only after DeconstructDriveID has confirmed
		// there are three "::"-separated components; a bare prefix without the zone
		// and doc id deconstructs to an empty docType and is therefore not a child.
		{"prefix without full triple", "FOLDER_IN_SHARED_FOLDER::zone", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, IsSharedFolderChildID(tc.drivewsid))
		})
	}
}

// TestDriveIDRoundTrip checks Construct/Deconstruct/GetDocIDFromDriveID agree.
func TestDriveIDRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		docType, zone, docid string
	}{
		{"FOLDER", "com.apple.CloudDocs", "ABC-123"},
		{"FILE", "com.apple.CloudDocs", "doc-with-dashes-456"},
		{"FOLDER_IN_SHARED_FOLDER", "com.apple.CloudDocs", "789"},
	} {
		t.Run(tc.docType, func(t *testing.T) {
			id := ConstructDriveID(tc.docid, tc.zone, tc.docType)
			gotType, gotZone, gotDoc := DeconstructDriveID(id)
			assert.Equal(t, tc.docType, gotType)
			assert.Equal(t, tc.zone, gotZone)
			assert.Equal(t, tc.docid, gotDoc)
			assert.Equal(t, tc.docid, GetDocIDFromDriveID(id))
		})
	}
}

// TestDeconstructDriveIDShort documents the fallback for malformed IDs: fewer
// than three components yields empty type/zone and the whole string as the docid.
func TestDeconstructDriveIDShort(t *testing.T) {
	docType, zone, docid := DeconstructDriveID("not-a-drive-id")
	assert.Equal(t, "", docType)
	assert.Equal(t, "", zone)
	assert.Equal(t, "not-a-drive-id", docid)

	// GetDocIDFromDriveID always returns the trailing component.
	assert.Equal(t, "not-a-drive-id", GetDocIDFromDriveID("not-a-drive-id"))
}

// TestSplitName covers the name/extension splitting used when converting raw
// docws enumerate items into DriveItems.
func TestSplitName(t *testing.T) {
	for _, tc := range []struct {
		in, name, ext string
	}{
		{"report.txt", "report", "txt"},
		{"archive.tar.gz", "archive.tar", "gz"},
		{"noext", "noext", ""},
		{"trailingdot.", "trailingdot.", ""},
		{".hidden", "", "hidden"},
		{"", "", ""},
	} {
		t.Run(tc.in, func(t *testing.T) {
			d := &DriveItemRaw{ItemInfo: &DriveItemRawInfo{Name: tc.in}}
			name, ext := d.SplitName()
			assert.Equal(t, tc.name, name)
			assert.Equal(t, tc.ext, ext)
		})
	}
}

// TestIntoDriveItem checks the deterministic DriveItemRaw -> DriveItem conversion
// (as used for items listed via the docws enumerate endpoint).
func TestIntoDriveItem(t *testing.T) {
	raw := &DriveItemRaw{
		ItemID: "ITEM-1",
		ItemInfo: &DriveItemRawInfo{
			Name:       "photo.jpeg",
			Type:       "FILE",
			Version:    "etag-v2",
			Size:       4096,
			ModifiedAt: "1700000000000",
			CreatedAt:  "1600000000000",
		},
	}
	raw.ItemInfo.Urls.URLDownload = "https://example.invalid/download"

	item := raw.IntoDriveItem()
	assert.Equal(t, "ITEM-1", item.Itemid)
	assert.Equal(t, "photo", item.Name)
	assert.Equal(t, "jpeg", item.Extension)
	assert.Equal(t, "photo.jpeg", item.FullName())
	assert.Equal(t, "FILE", item.Type)
	assert.Equal(t, "etag-v2", item.Etag)
	assert.Equal(t, int64(4096), item.Size)
	assert.Equal(t, "https://example.invalid/download", item.DownloadURL())
	assert.False(t, item.IsFolder())
	assert.Equal(t, time.UnixMilli(1700000000000), item.DateModified)
	assert.Equal(t, time.UnixMilli(1600000000000), item.DateCreated)
}

// TestIntoDriveItemBadTimes verifies unparseable timestamps degrade to the zero time.
func TestIntoDriveItemBadTimes(t *testing.T) {
	raw := &DriveItemRaw{
		ItemID:   "ITEM-2",
		ItemInfo: &DriveItemRawInfo{Name: "dir", Type: "FOLDER", ModifiedAt: "", CreatedAt: "nope"},
	}
	item := raw.IntoDriveItem()
	assert.True(t, item.DateModified.IsZero())
	assert.True(t, item.DateCreated.IsZero())
	assert.True(t, item.IsFolder())
}

// TestDocumentDriveID checks the zone defaulting in Document.DriveID.
func TestDocumentDriveID(t *testing.T) {
	withZone := &Document{Type: "FILE", Zone: "owner.zone", DocumentID: "D1"}
	assert.Equal(t, "FILE::owner.zone::D1", withZone.DriveID())

	noZone := &Document{Type: "FILE", DocumentID: "D2"}
	assert.Equal(t, "FILE::com.apple.CloudDocs::D2", noZone.DriveID())
}

// TestDriveItemShareID checks that drivews shared-root metadata preserves the
// CloudDocs share record name needed to address files directly in a shared root.
func TestDriveItemShareID(t *testing.T) {
	var item DriveItem
	err := json.Unmarshal([]byte(`{
		"drivewsid": "SHARED_FOLDER::com.apple.CloudDocs::D1",
		"shareID": {
			"shareName": "SHARE-1",
			"recordName": "SHARE-1",
			"zoneID": {
				"zoneName": "com.apple.CloudDocs",
				"ownerRecordName": "_owner",
				"zoneType": "REGULAR_CUSTOM_ZONE"
			}
		}
	}`), &item)
	assert.NoError(t, err)
	assert.NotNil(t, item.ShareID)
	assert.Equal(t, "SHARE-1", item.ShareID.RecordName)
	assert.Equal(t, "_owner", item.ShareID.ZoneID.OwnerRecordName)
}
