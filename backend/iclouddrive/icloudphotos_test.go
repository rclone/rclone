//go:build !plan9 && !solaris

package iclouddrive

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/rclone/rclone/backend/iclouddrive/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestPhotosFs() *PhotosFs {
	return &PhotosFs{
		name: "test-icp",
		root: "",
		opt:  Options{},
	}
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
			"All Photos": {Name: "All Photos", ObjectType: "CPLAssetByAssetDateWithoutHiddenOrDeleted"},
			"Videos":     {Name: "Videos", ObjectType: "CPLAssetInSmartAlbumByAssetDate:Video"},
			"UserAlbum":  {Name: "UserAlbum", ObjectType: "CPLContainerRelationNotDeletedByAssetDate:rec1", RecordName: "rec1"},
			"Folder": {
				Name: "Folder", RecordName: "folder1", IsFolder: true,
				Children: map[string]*api.Album{
					"ChildAlbum": {Name: "ChildAlbum", ObjectType: "CPLContainerRelationNotDeletedByAssetDate:rec2", RecordName: "rec2"},
					"NestedFolder": {
						Name: "NestedFolder", RecordName: "folder2", IsFolder: true,
						Children: map[string]*api.Album{
							"LeafAlbum": {Name: "LeafAlbum", ObjectType: "CPLContainerRelationNotDeletedByAssetDate:rec3", RecordName: "rec3"},
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
		album, err := f.resolveAlbum(ctx, f.photos, "PrimarySync", "Folder/ChildAlbum")
		require.NoError(t, err)
		assert.Equal(t, "ChildAlbum", album.Name)
	})

	t.Run("two-level nesting", func(t *testing.T) {
		album, err := f.resolveAlbum(ctx, f.photos, "PrimarySync", "Folder/NestedFolder/LeafAlbum")
		require.NoError(t, err)
		assert.Equal(t, "LeafAlbum", album.Name)
	})

	t.Run("folder itself", func(t *testing.T) {
		album, err := f.resolveAlbum(ctx, f.photos, "PrimarySync", "Folder")
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
		id, found, err := f.FindLeaf(ctx, "album:PrimarySync:Folder", "ChildAlbum")
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, "album:PrimarySync:Folder/ChildAlbum", id)
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
	album, err := f.resolveAlbum(ctx, f.photos, "PrimarySync", "Folder")
	require.NoError(t, err)
	assert.True(t, album.IsFolder, "Folder should be a folder")
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
	assert.Contains(t, dirs, "PrimarySync/Folder")
	assert.Contains(t, dirs, "PrimarySync/Folder/ChildAlbum")
	assert.Contains(t, dirs, "PrimarySync/Folder/NestedFolder")
	assert.Contains(t, dirs, "PrimarySync/Folder/NestedFolder/LeafAlbum")
}

func TestListR_FolderLevel(t *testing.T) {
	f := &PhotosFs{
		name:      "test",
		root:      "PrimarySync/Folder",
		opt:       Options{},
		startTime: time.Now(),
	}
	f.features = (&fs.Features{}).Fill(context.Background(), f)
	f.photos = newTestPhotosService()
	f.dirCache = dircache.New("PrimarySync/Folder", rootID, f)
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

	// When rooted at a folder, should see children recursively
	assert.Contains(t, dirs, "ChildAlbum")
	assert.Contains(t, dirs, "NestedFolder")
	assert.Contains(t, dirs, "NestedFolder/LeafAlbum")
}

func TestNotifyZoneChange(t *testing.T) {
	ctx := context.Background()
	ps := newTestPhotosService()

	tests := []struct {
		name string
		root string
		zone string
		want []string
	}{
		{
			name: "top level root",
			root: "",
			zone: "PrimarySync",
			want: []string{"PrimarySync"},
		},
		{
			name: "library root",
			root: "PrimarySync",
			zone: "PrimarySync",
			want: []string{"", "All Photos", "Folder", "Folder/ChildAlbum", "Folder/NestedFolder", "Folder/NestedFolder/LeafAlbum", "UserAlbum", "Videos"},
		},
		{
			name: "folder root",
			root: "PrimarySync/Folder",
			zone: "PrimarySync",
			want: []string{"", "ChildAlbum", "NestedFolder", "NestedFolder/LeafAlbum"},
		},
		{
			name: "nested folder root",
			root: "PrimarySync/Folder/NestedFolder",
			zone: "PrimarySync",
			want: []string{"", "LeafAlbum"},
		},
		{
			name: "leaf album root",
			root: "PrimarySync/Folder/NestedFolder/LeafAlbum",
			zone: "PrimarySync",
			want: []string{""},
		},
		{
			name: "missing nested album still invalidates root",
			root: "PrimarySync/Folder/NoSuchAlbum",
			zone: "PrimarySync",
			want: []string{""},
		},
		{
			name: "different zone ignored",
			root: "PrimarySync/Folder/NestedFolder",
			zone: "SharedSync",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newTestPhotosFs()
			f.root = tt.root

			var got []string
			f.notifyZoneChange(ctx, ps, tt.zone, func(remote string, entryType fs.EntryType) {
				assert.Equal(t, fs.EntryDirectory, entryType)
				got = append(got, remote)
			})

			sort.Strings(got)
			want := append([]string(nil), tt.want...)
			sort.Strings(want)
			assert.Equal(t, want, got)
		})
	}
}
