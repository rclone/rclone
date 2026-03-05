//go:build !plan9 && !solaris

package iclouddrive

import (
	"context"
	"testing"
	"time"

	"github.com/rclone/rclone/backend/iclouddrive/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// 1. COMPILE-TIME INTERFACE COMPLIANCE
// These fail at compile time if interfaces are not satisfied.
// =============================================================================

var (
	_ fs.Fs     = (*PhotosFs)(nil)
	_ fs.Object = (*PhotosObject)(nil)
)

// =============================================================================
// 2. UNIT TESTS FOR PhotosObject
// =============================================================================

func newTestPhotosObject() *PhotosObject {
	return &PhotosObject{
		fs:          &PhotosFs{name: "test", root: ""},
		remote:      "PrimarySync/All Photos/IMG_0001.JPG",
		size:        12345,
		modTime:     time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
		downloadURL: "https://example.com/photo.jpg",
	}
}

// BUG-1: Storable() must return true.
// Storable()==false means rclone skips the object during sync/copy.
// A read-only backend still needs Storable()==true so files can be
// transferred TO other backends.
// Reference: iclouddrive.go Object.Storable() returns true.
func TestPhotosObject_Storable(t *testing.T) {
	o := newTestPhotosObject()
	assert.True(t, o.Storable(), "Storable() must return true; false makes photos un-syncable")
}

// BUG-7: SetModTime must return fs.ErrorCantSetModTime, not fs.ErrorNotImplemented.
// ErrorNotImplemented signals a transient/fixable situation.
// ErrorCantSetModTime tells rclone this backend fundamentally cannot set modtime.
// Reference: iclouddrive.go Object.SetModTime() returns fs.ErrorCantSetModTime.
func TestPhotosObject_SetModTime(t *testing.T) {
	o := newTestPhotosObject()
	err := o.SetModTime(context.Background(), time.Now())
	assert.ErrorIs(t, err, fs.ErrorCantSetModTime,
		"SetModTime must return fs.ErrorCantSetModTime, not fs.ErrorNotImplemented")
}

func TestPhotosObject_Hash(t *testing.T) {
	o := newTestPhotosObject()
	h, err := o.Hash(context.Background(), hash.MD5)
	assert.Equal(t, "", h)
	assert.ErrorIs(t, err, hash.ErrUnsupported)
}

func TestPhotosObject_BasicFields(t *testing.T) {
	o := newTestPhotosObject()
	assert.Equal(t, "PrimarySync/All Photos/IMG_0001.JPG", o.Remote())
	assert.Equal(t, int64(12345), o.Size())
	assert.Equal(t, time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC), o.ModTime(context.Background()))
	assert.Equal(t, "PrimarySync/All Photos/IMG_0001.JPG", o.String())
	assert.NotNil(t, o.Fs())
}

// Read-only: Update and Remove must return errors.
func TestPhotosObject_ReadOnly(t *testing.T) {
	o := newTestPhotosObject()
	err := o.Update(context.Background(), nil, nil)
	assert.Error(t, err, "Update on read-only backend must return error")
	err = o.Remove(context.Background())
	assert.Error(t, err, "Remove on read-only backend must return error")
}

// =============================================================================
// 3. UNIT TESTS FOR PhotosFs
// =============================================================================

func newTestPhotosFs() *PhotosFs {
	return &PhotosFs{
		name: "test-icp",
		root: "",
		opt:  PhotosOptions{},
	}
}

func TestPhotosFs_BasicFields(t *testing.T) {
	f := newTestPhotosFs()
	assert.Equal(t, "test-icp", f.Name())
	assert.Equal(t, time.Second, f.Precision())
	assert.Equal(t, hash.Set(hash.None), f.Hashes())
}

// Read-only: Put, Mkdir, Rmdir must return errors.
func TestPhotosFs_ReadOnly(t *testing.T) {
	f := newTestPhotosFs()
	_, err := f.Put(context.Background(), nil, nil)
	assert.Error(t, err, "Put on read-only backend must return error")
	err = f.Mkdir(context.Background(), "test")
	assert.Error(t, err, "Mkdir on read-only backend must return error")
	err = f.Rmdir(context.Background(), "test")
	assert.Error(t, err, "Rmdir on read-only backend must return error")
}

// STYLE-2: Root() must apply encoding, matching Drive backend convention.
// When Enc is zero-value (no encoding), Root() should still return root unchanged.
func TestPhotosFs_Root(t *testing.T) {
	f := newTestPhotosFs()
	f.root = "PrimarySync/All Photos"
	// With zero-value encoder, should return root as-is (no panic, no empty string)
	r := f.Root()
	assert.Equal(t, "PrimarySync/All Photos", r)
}

// =============================================================================
// 4. SMART ALBUM DEFINITIONS MATCH PYICLOUD REFERENCE
// Verified against pyicloud services/photos.py SMART_ALBUMS dict.
// =============================================================================

func TestSmartAlbumDefinitions(t *testing.T) {
	// These are the 11 smart albums defined in pyicloud.
	// Our implementation must have all of them with correct ObjectType/ListType.
	expected := map[string]struct {
		ObjectType string
		ListType   string
	}{
		"All Photos":       {"CPLAssetByAssetDateWithoutHiddenOrDeleted", "CPLAssetAndMasterByAssetDateWithoutHiddenOrDeleted"},
		"Time-lapse":       {"CPLAssetInSmartAlbumByAssetDate:Timelapse", "CPLAssetAndMasterInSmartAlbumByAssetDate"},
		"Videos":           {"CPLAssetInSmartAlbumByAssetDate:Video", "CPLAssetAndMasterInSmartAlbumByAssetDate"},
		"Slo-mo":           {"CPLAssetInSmartAlbumByAssetDate:Slomo", "CPLAssetAndMasterInSmartAlbumByAssetDate"},
		"Bursts":           {"CPLAssetBurstStackAssetByAssetDate", "CPLBurstStackAssetAndMasterByAssetDate"},
		"Favorites":        {"CPLAssetInSmartAlbumByAssetDate:Favorite", "CPLAssetAndMasterInSmartAlbumByAssetDate"},
		"Panoramas":        {"CPLAssetInSmartAlbumByAssetDate:Panorama", "CPLAssetAndMasterInSmartAlbumByAssetDate"},
		"Screenshots":      {"CPLAssetInSmartAlbumByAssetDate:Screenshot", "CPLAssetAndMasterInSmartAlbumByAssetDate"},
		"Live":             {"CPLAssetInSmartAlbumByAssetDate:Live", "CPLAssetAndMasterInSmartAlbumByAssetDate"},
		"Recently Deleted": {"CPLAssetDeletedByExpungedDate", "CPLAssetAndMasterDeletedByExpungedDate"},
		"Hidden":           {"CPLAssetHiddenByAssetDate", "CPLAssetAndMasterHiddenByAssetDate"},
	}

	for name, exp := range expected {
		t.Run(name, func(t *testing.T) {
			album, exists := api.SmartAlbums[name]
			require.True(t, exists, "smart album %q must be defined", name)
			assert.Equal(t, exp.ObjectType, album.ObjectType,
				"ObjectType mismatch for %q", name)
			assert.Equal(t, exp.ListType, album.ListType,
				"ListType mismatch for %q", name)
		})
	}

	// Verify we have exactly the expected number (no extras, no missing)
	assert.Equal(t, len(expected), len(api.SmartAlbums),
		"smart album count mismatch: expected %d, got %d", len(expected), len(api.SmartAlbums))
}

// =============================================================================
// 5. SMART ALBUM FILTER VERIFICATION
// Albums with smartAlbum filters must have correct filter values.
// =============================================================================

func TestSmartAlbumFilters(t *testing.T) {
	// Albums that must have a smartAlbum filter
	filtered := map[string]string{
		"Time-lapse":  "TIMELAPSE",
		"Videos":      "VIDEO",
		"Slo-mo":      "SLOMO",
		"Favorites":   "FAVORITE",
		"Panoramas":   "PANORAMA",
		"Screenshots": "SCREENSHOT",
		"Live":        "LIVE",
	}

	for name, filterVal := range filtered {
		t.Run(name, func(t *testing.T) {
			album := api.SmartAlbums[name]
			require.NotEmpty(t, album.Filters, "album %q must have filters", name)
			assert.Equal(t, "smartAlbum", album.Filters[0].FieldName)
			assert.Equal(t, "EQUALS", album.Filters[0].Comparator)
			fv, ok := album.Filters[0].FieldValue.(map[string]string)
			require.True(t, ok, "filter value must be map[string]string")
			assert.Equal(t, filterVal, fv["value"],
				"filter value mismatch for %q", name)
		})
	}

	// Albums that must NOT have filters
	unfiltered := []string{"All Photos", "Bursts", "Recently Deleted", "Hidden"}
	for _, name := range unfiltered {
		t.Run(name+"_no_filter", func(t *testing.T) {
			album := api.SmartAlbums[name]
			assert.Empty(t, album.Filters, "album %q must not have filters", name)
		})
	}
}

// =============================================================================
// 6. PHOTO DATA STRUCTURE
// =============================================================================

func TestPhotoFields(t *testing.T) {
	p := &api.Photo{
		ID:          "ABC123",
		Filename:    "IMG_0001.HEIC",
		Size:        5_000_000,
		AssetDate:   1700000000000, // milliseconds
		DownloadURL: "https://cvws.icloud-content.com/...",
		Width:       4032,
		Height:      3024,
		IsVideo:     false,
	}

	assert.Equal(t, "ABC123", p.ID)
	assert.Equal(t, "IMG_0001.HEIC", p.Filename)
	assert.Equal(t, int64(5_000_000), p.Size)
	assert.Equal(t, "https://cvws.icloud-content.com/...", p.DownloadURL)
	assert.Equal(t, 4032, p.Width)
	assert.Equal(t, 3024, p.Height)
	assert.False(t, p.IsVideo)

	// AssetDate is milliseconds since epoch
	ts := time.Unix(p.AssetDate/1000, 0)
	assert.Equal(t, 2023, ts.Year(), "AssetDate must be milliseconds since epoch")
}

// =============================================================================
// 7. FEATURES AND DIRCACHE
// =============================================================================

func TestPhotosFs_Features(t *testing.T) {
	f := newTestPhotosFs()
	f.features = (&fs.Features{
		CanHaveEmptyDirectories: false,
		PartialUploads:          false,
		ReadMimeType:            false,
	}).Fill(context.Background(), f)
	features := f.Features()
	require.NotNil(t, features)
	assert.False(t, features.CanHaveEmptyDirectories)
	assert.False(t, features.PartialUploads)
	assert.False(t, features.ReadMimeType)
}
