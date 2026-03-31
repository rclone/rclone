//go:build !plan9 && !solaris

package iclouddrive

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/rclone/rclone/backend/iclouddrive/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestPhotosObject() *PhotosObject {
	return &PhotosObject{
		fs:       &PhotosFs{name: "test", root: ""},
		remote:   "PrimarySync/All Photos/IMG_0001.JPG",
		size:     12345,
		modTime:  time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
		masterID: "test-master-001",
		zone:     "PrimarySync",
	}
}

func TestPhotosObject_Storable(t *testing.T) {
	o := newTestPhotosObject()
	assert.True(t, o.Storable())
}

func TestPhotosObject_SetModTime(t *testing.T) {
	o := newTestPhotosObject()
	err := o.SetModTime(context.Background(), time.Now())
	assert.ErrorIs(t, err, fs.ErrorCantSetModTime)
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

func TestPhotosObject_ReadOnly(t *testing.T) {
	o := newTestPhotosObject()
	err := o.Update(context.Background(), nil, nil)
	assert.ErrorIs(t, err, fs.ErrorNotImplemented)
	err = o.Remove(context.Background())
	assert.ErrorIs(t, err, fs.ErrorNotImplemented)
}

func TestPhotosObject_NilString(t *testing.T) {
	var o *PhotosObject
	assert.Equal(t, "<nil>", o.String())
}

func newTestPhotosFs() *PhotosFs {
	return &PhotosFs{
		name: "test-icp",
		root: "",
		opt:  Options{},
	}
}

func TestPhotosFs_BasicFields(t *testing.T) {
	f := newTestPhotosFs()
	assert.Equal(t, "test-icp", f.Name())
	assert.Equal(t, time.Second, f.Precision())
	assert.Equal(t, hash.Set(hash.None), f.Hashes())
}

func TestPhotosFs_ReadOnly(t *testing.T) {
	f := newTestPhotosFs()
	_, err := f.Put(context.Background(), nil, nil)
	assert.ErrorIs(t, err, fs.ErrorNotImplemented)
	err = f.Mkdir(context.Background(), "test")
	assert.ErrorIs(t, err, fs.ErrorNotImplemented)
	err = f.Rmdir(context.Background(), "test")
	assert.ErrorIs(t, err, fs.ErrorNotImplemented)
}

func TestPhotosFs_String(t *testing.T) {
	f := newTestPhotosFs()
	f.root = "PrimarySync"
	assert.Equal(t, "iCloud Photos root 'PrimarySync'", f.String())
}

func TestPhotosFs_Root(t *testing.T) {
	f := newTestPhotosFs()
	f.root = "PrimarySync/All Photos"
	r := f.Root()
	assert.Equal(t, "PrimarySync/All Photos", r)
}

func TestSmartAlbumDefinitions(t *testing.T) {
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
		"Portrait":         {"CPLAssetInSmartAlbumByAssetDate:Depth", "CPLAssetAndMasterInSmartAlbumByAssetDate"},
		"Long Exposure":    {"CPLAssetInSmartAlbumByAssetDate:Exposure", "CPLAssetAndMasterInSmartAlbumByAssetDate"},
		"Animated":         {"CPLAssetInSmartAlbumByAssetDate:Animated", "CPLAssetAndMasterInSmartAlbumByAssetDate"},
	}

	for name, exp := range expected {
		t.Run(name, func(t *testing.T) {
			album, exists := api.SmartAlbums[name]
			require.True(t, exists, "smart album %q must be defined", name)
			assert.Equal(t, exp.ObjectType, album.ObjectType)
			assert.Equal(t, exp.ListType, album.ListType)
		})
	}

	assert.Equal(t, len(expected), len(api.SmartAlbums))
}

func TestSmartAlbumFilters(t *testing.T) {
	filtered := map[string]string{
		"Time-lapse":    "TIMELAPSE",
		"Videos":        "VIDEO",
		"Slo-mo":        "SLOMO",
		"Favorites":     "FAVORITE",
		"Panoramas":     "PANORAMA",
		"Screenshots":   "SCREENSHOT",
		"Live":          "LIVE",
		"Portrait":      "DEPTH",
		"Long Exposure": "EXPOSURE",
		"Animated":      "ANIMATED",
	}

	for name, filterVal := range filtered {
		t.Run(name, func(t *testing.T) {
			album := api.SmartAlbums[name]
			require.NotEmpty(t, album.Filters, "album %q must have filters", name)
			assert.Equal(t, "smartAlbum", album.Filters[0].FieldName)
			assert.Equal(t, "EQUALS", album.Filters[0].Comparator)
			fv, ok := album.Filters[0].FieldValue.(map[string]string)
			require.True(t, ok, "filter value must be map[string]string")
			assert.Equal(t, filterVal, fv["value"])
		})
	}

	unfiltered := []string{"All Photos", "Bursts", "Recently Deleted", "Hidden"}
	for _, name := range unfiltered {
		t.Run(name+"_no_filter", func(t *testing.T) {
			album := api.SmartAlbums[name]
			assert.Empty(t, album.Filters, "album %q must not have filters", name)
		})
	}
}

func TestParseAlbumDirID(t *testing.T) {
	lib, album, ok := parseAlbumDirID("album:PrimarySync:Favorites")
	assert.True(t, ok)
	assert.Equal(t, "PrimarySync", lib)
	assert.Equal(t, "Favorites", album)

	// Colon in album name
	lib, album, ok = parseAlbumDirID("album:PrimarySync:My:Album")
	assert.True(t, ok)
	assert.Equal(t, "PrimarySync", lib)
	assert.Equal(t, "My:Album", album)

	// Missing album
	_, _, ok = parseAlbumDirID("album:PrimarySync")
	assert.False(t, ok)

	// Empty
	_, _, ok = parseAlbumDirID("album:")
	assert.False(t, ok)
}

func TestPhotosFs_Features(t *testing.T) {
	f := newTestPhotosFs()
	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
		PartialUploads:          false,
		ReadMimeType:            false,
		ReadMetadata:            true,
	}).Fill(context.Background(), f)
	features := f.Features()
	require.NotNil(t, features)
	assert.True(t, features.CanHaveEmptyDirectories)
	assert.False(t, features.PartialUploads)
	assert.False(t, features.ReadMimeType)
	assert.True(t, features.ReadMetadata)
}

func TestPhotosObject_Metadata(t *testing.T) {
	o := &PhotosObject{
		fs:         &PhotosFs{name: "test"},
		remote:     "PrimarySync/All Photos/IMG_0001.HEIC",
		size:       4200000,
		modTime:    time.Date(2025, 6, 15, 14, 30, 0, 0, time.UTC),
		masterID:   "master-001",
		zone:       "PrimarySync",
		width:      4032,
		height:     3024,
		addedDate:  1718459400000, // 2024-06-15T13:50:00Z in millis
		isFavorite: true,
		isHidden:   false,
	}

	metadata, err := o.Metadata(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "4032", metadata["width"])
	assert.Equal(t, "3024", metadata["height"])
	assert.Equal(t, "2024-06-15T13:50:00Z", metadata["added-time"])
	assert.Equal(t, "true", metadata["favorite"])
	assert.Equal(t, "false", metadata["hidden"])
}

func TestPhotosObject_MetadataZeroDimensions(t *testing.T) {
	// Live Photo .MOV companion - no width/height, not favorite
	o := &PhotosObject{
		fs:     &PhotosFs{name: "test"},
		remote: "PrimarySync/Live/IMG_3031.MOV",
	}

	metadata, err := o.Metadata(context.Background())
	require.NoError(t, err)
	_, hasWidth := metadata["width"]
	_, hasHeight := metadata["height"]
	assert.False(t, hasWidth, "zero width should be omitted")
	assert.False(t, hasHeight, "zero height should be omitted")
	assert.Equal(t, "false", metadata["favorite"])
	assert.Equal(t, "false", metadata["hidden"])
}

// newTestPhotosService builds a PhotosService with pre-populated libraries and albums
// for testing resolveAlbum and FindLeaf without HTTP calls
func newTestPhotosService() *api.PhotosService {
	return api.NewTestPhotosService(map[string]map[string]*api.Album{
		"PrimarySync": {
			"All Photos": {Name: "All Photos", ObjectType: "CPLAssetByAssetDateWithoutHiddenOrDeleted", Zone: "PrimarySync"},
			"Videos":     {Name: "Videos", ObjectType: "CPLAssetInSmartAlbumByAssetDate:Video", Zone: "PrimarySync"},
			"MyAlbum":    {Name: "MyAlbum", ObjectType: "CPLContainerRelationNotDeletedByAssetDate:rec1", Zone: "PrimarySync", RecordName: "rec1"},
			"Darkroom": {
				Name: "Darkroom", Zone: "PrimarySync", RecordName: "folder1", IsFolder: true,
				Children: map[string]*api.Album{
					"Rejected": {Name: "Rejected", ObjectType: "CPLContainerRelationNotDeletedByAssetDate:rec2", Zone: "PrimarySync", RecordName: "rec2"},
					"Nested": {
						Name: "Nested", Zone: "PrimarySync", RecordName: "folder2", IsFolder: true,
						Children: map[string]*api.Album{
							"Deep": {Name: "Deep", ObjectType: "CPLContainerRelationNotDeletedByAssetDate:rec3", Zone: "PrimarySync", RecordName: "rec3"},
						},
					},
				},
			},
		},
	})
}

func TestResolveAlbum(t *testing.T) {
	f := newTestPhotosFs()
	f.photos = newTestPhotosService()
	ctx := context.Background()

	t.Run("simple album", func(t *testing.T) {
		album, err := f.resolveAlbum(ctx, f.photos, "PrimarySync", "Videos")
		require.NoError(t, err)
		assert.Equal(t, "Videos", album.Name)
	})

	t.Run("nested folder child", func(t *testing.T) {
		album, err := f.resolveAlbum(ctx, f.photos, "PrimarySync", "Darkroom/Rejected")
		require.NoError(t, err)
		assert.Equal(t, "Rejected", album.Name)
	})

	t.Run("two-level nesting", func(t *testing.T) {
		album, err := f.resolveAlbum(ctx, f.photos, "PrimarySync", "Darkroom/Nested/Deep")
		require.NoError(t, err)
		assert.Equal(t, "Deep", album.Name)
	})

	t.Run("folder itself", func(t *testing.T) {
		album, err := f.resolveAlbum(ctx, f.photos, "PrimarySync", "Darkroom")
		require.NoError(t, err)
		assert.True(t, album.IsFolder)
	})

	t.Run("missing album", func(t *testing.T) {
		_, err := f.resolveAlbum(ctx, f.photos, "PrimarySync", "NoSuchAlbum")
		assert.ErrorIs(t, err, fs.ErrorDirNotFound)
	})

	t.Run("missing library", func(t *testing.T) {
		_, err := f.resolveAlbum(ctx, f.photos, "NoSuchLib", "Videos")
		assert.ErrorIs(t, err, fs.ErrorDirNotFound)
	})

	t.Run("traverse into non-folder", func(t *testing.T) {
		_, err := f.resolveAlbum(ctx, f.photos, "PrimarySync", "Videos/SubAlbum")
		assert.ErrorIs(t, err, fs.ErrorDirNotFound)
	})
}

func TestFindLeaf(t *testing.T) {
	f := newTestPhotosFs()
	f.photos = newTestPhotosService()
	ctx := context.Background()

	t.Run("root to library", func(t *testing.T) {
		id, found, err := f.FindLeaf(ctx, rootID, "PrimarySync")
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, "lib:PrimarySync", id)
	})

	t.Run("root to missing library", func(t *testing.T) {
		_, found, err := f.FindLeaf(ctx, rootID, "NoSuchLib")
		require.NoError(t, err)
		assert.False(t, found)
	})

	t.Run("library to album", func(t *testing.T) {
		id, found, err := f.FindLeaf(ctx, "lib:PrimarySync", "Videos")
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, "album:PrimarySync:Videos", id)
	})

	t.Run("library to missing album", func(t *testing.T) {
		_, found, err := f.FindLeaf(ctx, "lib:PrimarySync", "NoSuchAlbum")
		require.NoError(t, err)
		assert.False(t, found)
	})

	t.Run("missing library returns ErrorDirNotFound", func(t *testing.T) {
		_, _, err := f.FindLeaf(ctx, "lib:NoSuchLib", "Videos")
		assert.ErrorIs(t, err, fs.ErrorDirNotFound)
	})

	t.Run("folder to child album", func(t *testing.T) {
		id, found, err := f.FindLeaf(ctx, "album:PrimarySync:Darkroom", "Rejected")
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, "album:PrimarySync:Darkroom/Rejected", id)
	})

	t.Run("non-folder album returns not found", func(t *testing.T) {
		_, found, err := f.FindLeaf(ctx, "album:PrimarySync:Videos", "SubAlbum")
		require.NoError(t, err)
		assert.False(t, found)
	})

	t.Run("unknown prefix returns not found", func(t *testing.T) {
		_, found, err := f.FindLeaf(ctx, "unknown:prefix", "leaf")
		require.NoError(t, err)
		assert.False(t, found)
	})
}

func TestResolveAlbum_EmptyPath(t *testing.T) {
	f := newTestPhotosFs()
	f.photos = newTestPhotosService()
	ctx := context.Background()

	// parseAlbumDirID("album:PrimarySync:") returns ("PrimarySync", "", true)
	// This empty album path feeds into resolveAlbum
	_, err := f.resolveAlbum(ctx, f.photos, "PrimarySync", "")
	assert.ErrorIs(t, err, fs.ErrorDirNotFound, "empty album path should return ErrorDirNotFound")
}

func TestNewObject_ErrorPaths(t *testing.T) {
	f := newTestPhotosFs()
	f.photos = newTestPhotosService()
	ctx := context.Background()

	// Pre-populate photoCache on the "Videos" album so GetPhotoByName works
	// without HTTP. Access via the test PhotosService's internal structure
	libs, _ := f.photos.GetLibraries(ctx)
	albums, _ := libs["PrimarySync"].GetAlbums(ctx)
	videos := albums["Videos"]
	videos.SetTestPhotoCache(map[string]*api.Photo{
		"existing.mp4": {ID: "m1", Filename: "existing.mp4", Size: 1024, ResourceKey: "resOriginalRes"},
	})

	// NewObject depends on dircache.FindDir which requires FindRoot + HTTP
	// Test the reachable paths through resolveAlbum and GetPhotoByName

	// Test resolveAlbum → folder → GetPhotoByName on folder (should fail)
	album, err := f.resolveAlbum(ctx, f.photos, "PrimarySync", "Darkroom")
	require.NoError(t, err)
	assert.True(t, album.IsFolder, "Darkroom should be a folder")
	// NewObject would return ErrorObjectNotFound for a folder path

	// Test GetPhotoByName on album with populated cache
	photo, err := videos.GetPhotoByName(ctx, "existing.mp4")
	require.NoError(t, err)
	assert.Equal(t, "m1", photo.ID)

	// Test GetPhotoByName cache miss
	_, err = videos.GetPhotoByName(ctx, "nonexistent.mp4")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent.mp4")
}

func TestPhotosFs_CreateDir(t *testing.T) {
	f := newTestPhotosFs()
	_, err := f.CreateDir(context.Background(), "lib:PrimarySync", "NewAlbum")
	assert.ErrorIs(t, err, fs.ErrorNotImplemented)
}

func TestParseAlbumDirID_Exhaustive(t *testing.T) {
	tests := []struct {
		input      string
		lib, album string
		ok         bool
	}{
		{"album:PrimarySync:Videos", "PrimarySync", "Videos", true},
		{"album:PrimarySync:Folder/Child", "PrimarySync", "Folder/Child", true},
		{"album:PrimarySync:Name:With:Colons", "PrimarySync", "Name:With:Colons", true},
		{"album:PrimarySync:", "PrimarySync", "", true},
		{"album:", "", "", false},
		{"notalbum:foo:bar", "notalbum", "foo:bar", true}, // strips "album:" prefix literally, "notalbum:" stays
		{"", "", "", false},
	}

	for _, tt := range tests {
		lib, album, ok := parseAlbumDirID(tt.input)
		if tt.ok {
			assert.True(t, ok, "input=%q", tt.input)
			assert.Equal(t, tt.lib, lib, "input=%q", tt.input)
			assert.Equal(t, tt.album, album, "input=%q", tt.input)
		} else {
			assert.False(t, ok, "input=%q", tt.input)
		}
	}
}

// setEmptyPhotoCaches recursively sets empty photo caches on all leaf albums
// so GetPhotos returns without HTTP (service=nil test fast path)
func setEmptyPhotoCaches(albums map[string]*api.Album) {
	for _, album := range albums {
		if album.IsFolder {
			setEmptyPhotoCaches(album.Children)
		} else {
			album.SetTestPhotoCache(map[string]*api.Photo{})
		}
	}
}

func TestListR_NestedFolderRecursion(t *testing.T) {
	f := newTestPhotosFs()
	f.photos = newTestPhotosService()
	f.dirCache = dircache.New("", rootID, f)
	f.features = (&fs.Features{}).Fill(context.Background(), f)
	f.startTime = time.Now()
	ctx := context.Background()

	err := f.dirCache.FindRoot(ctx, false)
	require.NoError(t, err)

	// Pre-populate empty photo caches on all leaf albums
	libs, err := f.photos.GetLibraries(ctx)
	require.NoError(t, err)
	for _, lib := range libs {
		albums, err := lib.GetAlbums(ctx)
		require.NoError(t, err)
		setEmptyPhotoCaches(albums)
	}

	var dirs []string
	err = f.ListR(ctx, "", func(entries fs.DirEntries) error {
		for _, entry := range entries {
			if _, ok := entry.(fs.Directory); ok {
				dirs = append(dirs, entry.Remote())
			}
		}
		return nil
	})
	require.NoError(t, err)
	sort.Strings(dirs)

	// All directories must be present, including deeply nested ones
	assert.Contains(t, dirs, "PrimarySync")
	assert.Contains(t, dirs, "PrimarySync/All Photos")
	assert.Contains(t, dirs, "PrimarySync/Videos")
	assert.Contains(t, dirs, "PrimarySync/Darkroom")
	assert.Contains(t, dirs, "PrimarySync/Darkroom/Rejected")
	assert.Contains(t, dirs, "PrimarySync/Darkroom/Nested")
	assert.Contains(t, dirs, "PrimarySync/Darkroom/Nested/Deep")
}

func TestListR_FolderLevel(t *testing.T) {
	f := &PhotosFs{
		name:      "test",
		root:      "PrimarySync/Darkroom",
		opt:       Options{},
		startTime: time.Now(),
	}
	f.features = (&fs.Features{}).Fill(context.Background(), f)
	f.photos = newTestPhotosService()
	f.dirCache = dircache.New("PrimarySync/Darkroom", rootID, f)
	ctx := context.Background()

	err := f.dirCache.FindRoot(ctx, false)
	require.NoError(t, err)

	libs, _ := f.photos.GetLibraries(ctx)
	for _, lib := range libs {
		albums, _ := lib.GetAlbums(ctx)
		setEmptyPhotoCaches(albums)
	}

	var dirs []string
	err = f.ListR(ctx, "", func(entries fs.DirEntries) error {
		for _, entry := range entries {
			if _, ok := entry.(fs.Directory); ok {
				dirs = append(dirs, entry.Remote())
			}
		}
		return nil
	})
	require.NoError(t, err)

	// When rooted at Darkroom folder, should see children recursively
	assert.Contains(t, dirs, "Rejected")
	assert.Contains(t, dirs, "Nested")
	assert.Contains(t, dirs, "Nested/Deep")
}
