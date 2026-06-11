//go:build !plan9 && !solaris

package iclouddrive

import (
	"testing"

	"github.com/rclone/rclone/backend/iclouddrive/api"
	"github.com/stretchr/testify/assert"
)

// The directory-cache ID helpers below are pure: they never touch the network or
// any Fs configuration, so an empty *Fs is enough to exercise them.

// TestParseSharedItemID checks extraction of the embedded item_id. Own-zone IDs
// are stored as `drivewsid#etag` and yield ""; shared-folder children are stored
// as `drivewsid#etag#itemID`.
func TestParseSharedItemID(t *testing.T) {
	f := &Fs{}
	for _, tc := range []struct {
		name, rid, want string
	}{
		{"own-zone id has no item_id", "FOLDER::z::doc#etag", ""},
		{"shared child carries item_id", "FOLDER_IN_SHARED_FOLDER::z::doc#etag#ITEM-9", "ITEM-9"},
		{"bare id", "FOLDER::z::doc", ""},
		{"empty drivewsid with item_id", "#etag#ITEM-7", "ITEM-7"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, f.parseSharedItemID(tc.rid))
		})
	}
}

// TestParseNormalizedID checks the drivewsid/etag split, including the 3-part
// shared-child form where only the first two components are returned.
func TestParseNormalizedID(t *testing.T) {
	f := &Fs{}
	for _, tc := range []struct {
		rid, wantID, wantEtag string
	}{
		{"FOLDER::z::doc#etag", "FOLDER::z::doc", "etag"},
		{"FOLDER::z::doc", "FOLDER::z::doc", ""},
		{"FOLDER_IN_SHARED_FOLDER::z::doc#etag#ITEM-9", "FOLDER_IN_SHARED_FOLDER::z::doc", "etag"},
	} {
		t.Run(tc.rid, func(t *testing.T) {
			id, etag := f.parseNormalizedID(tc.rid)
			assert.Equal(t, tc.wantID, id)
			assert.Equal(t, tc.wantEtag, etag)
		})
	}
}

// TestIDJoin checks joining and the replace-existing-etag behaviour.
func TestIDJoin(t *testing.T) {
	f := &Fs{}
	assert.Equal(t, "FOLDER::z::doc#etag", f.IDJoin("FOLDER::z::doc", "etag"))
	// An ID that already carries an etag has it replaced, not appended.
	assert.Equal(t, "FOLDER::z::doc#new", f.IDJoin("FOLDER::z::doc#old", "new"))
}

// TestFolderID checks that the normalized cache ID embeds the item_id only for
// shared-folder children, and is identical to IDJoin for own-zone items.
func TestFolderID(t *testing.T) {
	f := &Fs{}
	for _, tc := range []struct {
		name string
		item *api.DriveItem
		want string
	}{
		{
			"own-zone folder is unchanged (no item_id appended)",
			&api.DriveItem{Drivewsid: "FOLDER::z::doc", Etag: "e1", Itemid: "ITEM-1"},
			"FOLDER::z::doc#e1",
		},
		{
			"shared child embeds item_id",
			&api.DriveItem{Drivewsid: "FOLDER_IN_SHARED_FOLDER::z::doc", Etag: "e2", Itemid: "ITEM-2"},
			"FOLDER_IN_SHARED_FOLDER::z::doc#e2#ITEM-2",
		},
		{
			"shared child without item_id stays as drivewsid#etag",
			&api.DriveItem{Drivewsid: "FOLDER_IN_SHARED_FOLDER::z::doc", Etag: "e3", Itemid: ""},
			"FOLDER_IN_SHARED_FOLDER::z::doc#e3",
		},
		{
			"empty drivewsid (docws-only) embeds item_id",
			&api.DriveItem{Drivewsid: "", Etag: "e4", Itemid: "ITEM-4"},
			"#e4#ITEM-4",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := f.folderID(tc.item)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestFolderIDRoundTrip checks the cache ID built by folderID is parsed back into
// its components by the parse helpers consistently.
func TestFolderIDRoundTrip(t *testing.T) {
	f := &Fs{}
	item := &api.DriveItem{Drivewsid: "FOLDER_IN_SHARED_FOLDER::z::doc", Etag: "etag", Itemid: "ITEM-5"}
	jid := f.folderID(item)

	id, etag := f.parseNormalizedID(jid)
	assert.Equal(t, item.Drivewsid, id)
	assert.Equal(t, item.Etag, etag)
	assert.Equal(t, item.Itemid, f.parseSharedItemID(jid))
}

// TestIsSharedFolderRootID checks the shared-root classifier used when selecting
// where a temporary shared-subfolder upload should first land.
func TestIsSharedFolderRootID(t *testing.T) {
	assert.True(t, isSharedFolderRootID("SHARED_FOLDER::z::share-root"))
	assert.False(t, isSharedFolderRootID("FOLDER_IN_SHARED_FOLDER::z::child"))
	assert.False(t, isSharedFolderRootID("FOLDER::z::own"))
	assert.False(t, isSharedFolderRootID(""))
}

// TestSharedUploadTempName verifies that temporary upload names are collision-free
// at the share root while preserving the final extension for content type handling.
func TestSharedUploadTempName(t *testing.T) {
	assert.Equal(t, ".rclone-upload-token.txt", sharedUploadTempName("report.txt", "token"))
	assert.Equal(t, ".rclone-upload-token.gz", sharedUploadTempName("archive.tar.gz", "token"))
	assert.Equal(t, ".rclone-upload-token", sharedUploadTempName("noext", "token"))
	assert.Equal(t, ".rclone-upload-token", sharedUploadTempName("trailingdot.", "token"))
	assert.Equal(t, ".rclone-upload-token", sharedUploadTempName(".hidden", "token"))
}
