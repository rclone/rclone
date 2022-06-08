package googlephotos

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rclone/rclone/backend/googlephotos/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/dirtree"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/mockobject"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// time for directories
var startTime = fstest.Time("2019-06-24T15:53:05.999999999Z")

// mock Fs for testing patterns
type testLister struct {
	t        *testing.T
	albums   *albums
	names    []string
	uploaded dirtree.DirTree
}

// newTestLister makes a mock for testing
func newTestLister(t *testing.T) *testLister {
	return &testLister{
		t:        t,
		albums:   newAlbums(),
		uploaded: dirtree.New(),
	}
}

// mock listDir for testing
func (f *testLister) listDir(ctx context.Context, prefix string, filter api.SearchFilter) (entries fs.DirEntries, err error) {
	for _, name := range f.names {
		entries = append(entries, mockobject.New(prefix+name))
	}
	return entries, nil
}

// mock listAlbums for testing
func (f *testLister) listAlbums(ctx context.Context, shared bool) (all *albums, err error) {
	return f.albums, nil
}

// mock listUploads for testing
func (f *testLister) listUploads(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	entries = f.uploaded[dir]
	return entries, nil
}

// mock dirTime for testing
func (f *testLister) dirTime() time.Time {
	return startTime
}

// mock startYear for testing
func (f *testLister) startYear() int {
	return 2000
}

// mock includeArchived for testing
func (f *testLister) includeArchived() bool {
	return false
}

func TestPatternMatch(t *testing.T) {
	for testNumber, test := range []struct {
		// input
		root     string
		itemPath string
		isFile   bool
		// expected output
		wantMatch   []string
		wantPrefix  string
		wantPattern *dirPattern
	}{
		{
			root:        "",
			itemPath:    "",
			isFile:      false,
			wantMatch:   []string{""},
			wantPrefix:  "",
			wantPattern: &patterns[0],
		},
		{
			root:        "",
			itemPath:    "",
			isFile:      true,
			wantMatch:   nil,
			wantPrefix:  "",
			wantPattern: nil,
		},
		{
			root:        "upload",
			itemPath:    "",
			isFile:      false,
			wantMatch:   []string{"upload", ""},
			wantPrefix:  "",
			wantPattern: &patterns[1],
		},
		{
			root:        "upload/dir",
			itemPath:    "",
			isFile:      false,
			wantMatch:   []string{"upload/dir", "dir"},
			wantPrefix:  "",
			wantPattern: &patterns[1],
		},
		{
			root:        "upload/file.jpg",
			itemPath:    "",
			isFile:      true,
			wantMatch:   []string{"upload/file.jpg", "file.jpg"},
			wantPrefix:  "",
			wantPattern: &patterns[2],
		},
		{
			root:        "media",
			itemPath:    "",
			isFile:      false,
			wantMatch:   []string{"media"},
			wantPrefix:  "",
			wantPattern: &patterns[3],
		},
		{
			root:        "",
			itemPath:    "media",
			isFile:      false,
			wantMatch:   []string{"media"},
			wantPrefix:  "media/",
			wantPattern: &patterns[3],
		},
		{
			root:        "media/all",
			itemPath:    "",
			isFile:      false,
			wantMatch:   []string{"media/all"},
			wantPrefix:  "",
			wantPattern: &patterns[4],
		},
		{
			root:        "media",
			itemPath:    "all",
			isFile:      false,
			wantMatch:   []string{"media/all"},
			wantPrefix:  "all/",
			wantPattern: &patterns[4],
		},
		{
			root:        "media/all",
			itemPath:    "file.jpg",
			isFile:      true,
			wantMatch:   []string{"media/all/file.jpg", "file.jpg"},
			wantPrefix:  "file.jpg/",
			wantPattern: &patterns[5],
		},
		{
			root:        "",
			itemPath:    "feature",
			isFile:      false,
			wantMatch:   []string{"feature"},
			wantPrefix:  "feature/",
			wantPattern: &patterns[23],
		},
		{
			root:        "feature/favorites",
			itemPath:    "",
			isFile:      false,
			wantMatch:   []string{"feature/favorites"},
			wantPrefix:  "",
			wantPattern: &patterns[24],
		},
		{
			root:        "feature",
			itemPath:    "favorites",
			isFile:      false,
			wantMatch:   []string{"feature/favorites"},
			wantPrefix:  "favorites/",
			wantPattern: &patterns[24],
		},
		{
			root:        "feature/favorites",
			itemPath:    "file.jpg",
			isFile:      true,
			wantMatch:   []string{"feature/favorites/file.jpg", "file.jpg"},
			wantPrefix:  "file.jpg/",
			wantPattern: &patterns[25],
		},
	} {
		t.Run(fmt.Sprintf("#%d,root=%q,itemPath=%q,isFile=%v", testNumber, test.root, test.itemPath, test.isFile), func(t *testing.T) {
			gotMatch, gotPrefix, gotPattern := patterns.match(test.root, test.itemPath, test.isFile)
			assert.Equal(t, test.wantMatch, gotMatch)
			assert.Equal(t, test.wantPrefix, gotPrefix)
			assert.Equal(t, test.wantPattern, gotPattern)
		})
	}
}

func TestPatternMatchToEntries(t *testing.T) {
	ctx := context.Background()
	f := newTestLister(t)
	f.names = []string{"file.jpg"}
	f.albums.add(&api.Album{
		ID:    "1",
		Title: "sub/one",
	})
	f.albums.add(&api.Album{
		ID:    "2",
		Title: "sub",
	})
	f.uploaded.AddEntry(mockobject.New("upload/file1.jpg"))
	f.uploaded.AddEntry(mockobject.New("upload/dir/file2.jpg"))

	for testNumber, test := range []struct {
		// input
		root     string
		itemPath string
		// expected output
		wantMatch  []string
		wantPrefix string
		remotes    []string
	}{
		{
			root:       "",
			itemPath:   "",
			wantMatch:  []string{""},
			wantPrefix: "",
			remotes:    []string{"media/", "album/", "shared-album/", "upload/"},
		},
		{
			root:       "upload",
			itemPath:   "",
			wantMatch:  []string{"upload", ""},
			wantPrefix: "",
			remotes:    []string{"upload/file1.jpg", "upload/dir/"},
		},
		{
			root:       "upload",
			itemPath:   "dir",
			wantMatch:  []string{"upload/dir", "dir"},
			wantPrefix: "dir/",
			remotes:    []string{"upload/dir/file2.jpg"},
		},
		{
			root:       "media",
			itemPath:   "",
			wantMatch:  []string{"media"},
			wantPrefix: "",
			remotes:    []string{"all/", "by-year/", "by-month/", "by-day/"},
		},
		{
			root:       "media/all",
			itemPath:   "",
			wantMatch:  []string{"media/all"},
			wantPrefix: "",
			remotes:    []string{"file.jpg"},
		},
		{
			root:       "media",
			itemPath:   "all",
			wantMatch:  []string{"media/all"},
			wantPrefix: "all/",
			remotes:    []string{"all/file.jpg"},
		},
		{
			root:       "media/by-year",
			itemPath:   "",
			wantMatch:  []string{"media/by-year"},
			wantPrefix: "",
			remotes:    []string{"2000/", "2001/", "2002/", "2003/"},
		},
		{
			root:       "media/by-year/2000",
			itemPath:   "",
			wantMatch:  []string{"media/by-year/2000", "2000"},
			wantPrefix: "",
			remotes:    []string{"file.jpg"},
		},
		{
			root:       "media/by-month",
			itemPath:   "",
			wantMatch:  []string{"media/by-month"},
			wantPrefix: "",
			remotes:    []string{"2000/", "2001/", "2002/", "2003/"},
		},
		{
			root:       "media/by-month/2001",
			itemPath:   "",
			wantMatch:  []string{"media/by-month/2001", "2001"},
			wantPrefix: "",
			remotes:    []string{"2001-01/", "2001-02/", "2001-03/", "2001-04/"},
		},
		{
			root:       "media/by-month/2001/2001-01",
			itemPath:   "",
			wantMatch:  []string{"media/by-month/2001/2001-01", "2001", "01"},
			wantPrefix: "",
			remotes:    []string{"file.jpg"},
		},
		{
			root:       "media/by-day",
			itemPath:   "",
			wantMatch:  []string{"media/by-day"},
			wantPrefix: "",
			remotes:    []string{"2000/", "2001/", "2002/", "2003/"},
		},
		{
			root:       "media/by-day/2001",
			itemPath:   "",
			wantMatch:  []string{"media/by-day/2001", "2001"},
			wantPrefix: "",
			remotes:    []string{"2001-01-01/", "2001-01-02/", "2001-01-03/", "2001-01-04/"},
		},
		{
			root:       "media/by-day/2001/2001-01-02",
			itemPath:   "",
			wantMatch:  []string{"media/by-day/2001/2001-01-02", "2001", "01", "02"},
			wantPrefix: "",
			remotes:    []string{"file.jpg"},
		},
		{
			root:       "album",
			itemPath:   "",
			wantMatch:  []string{"album"},
			wantPrefix: "",
			remotes:    []string{"sub/"},
		},
		{
			root:       "album/sub",
			itemPath:   "",
			wantMatch:  []string{"album/sub", "sub"},
			wantPrefix: "",
			remotes:    []string{"one/", "file.jpg"},
		},
		{
			root:       "album/sub/one",
			itemPath:   "",
			wantMatch:  []string{"album/sub/one", "sub/one"},
			wantPrefix: "",
			remotes:    []string{"file.jpg"},
		},
		{
			root:       "shared-album",
			itemPath:   "",
			wantMatch:  []string{"shared-album"},
			wantPrefix: "",
			remotes:    []string{"sub/"},
		},
		{
			root:       "shared-album/sub",
			itemPath:   "",
			wantMatch:  []string{"shared-album/sub", "sub"},
			wantPrefix: "",
			remotes:    []string{"one/", "file.jpg"},
		},
		{
			root:       "shared-album/sub/one",
			itemPath:   "",
			wantMatch:  []string{"shared-album/sub/one", "sub/one"},
			wantPrefix: "",
			remotes:    []string{"file.jpg"},
		},
	} {
		t.Run(fmt.Sprintf("#%d,root=%q,itemPath=%q", testNumber, test.root, test.itemPath), func(t *testing.T) {
			match, prefix, pattern := patterns.match(test.root, test.itemPath, false)
			assert.Equal(t, test.wantMatch, match)
			assert.Equal(t, test.wantPrefix, prefix)
			assert.NotNil(t, pattern)
			assert.NotNil(t, pattern.toEntries)

			entries, err := pattern.toEntries(ctx, f, prefix, match)
			assert.NoError(t, err)
			var remotes = []string{}
			for _, entry := range entries {
				remote := entry.Remote()
				if _, isDir := entry.(fs.Directory); isDir {
					remote += "/"
				}
				remotes = append(remotes, remote)
				if len(remotes) >= 4 {
					break // only test first 4 entries
				}
			}
			assert.Equal(t, test.remotes, remotes)
		})
	}
}

func TestPatternYears(t *testing.T) {
	f := newTestLister(t)
	entries, err := years(context.Background(), f, "potato/", nil)
	require.NoError(t, err)

	year := 2000
	for _, entry := range entries {
		assert.Equal(t, "potato/"+fmt.Sprint(year), entry.Remote())
		year++
	}
}

func TestPatternMonths(t *testing.T) {
	f := newTestLister(t)
	entries, err := months(context.Background(), f, "potato/", []string{"", "2020"})
	require.NoError(t, err)

	assert.Equal(t, 12, len(entries))
	for i, entry := range entries {
		assert.Equal(t, fmt.Sprintf("potato/2020-%02d", i+1), entry.Remote())
	}
}

func TestPatternDays(t *testing.T) {
	f := newTestLister(t)
	entries, err := days(context.Background(), f, "potato/", []string{"", "2020"})
	require.NoError(t, err)

	assert.Equal(t, 366, len(entries))
	assert.Equal(t, "potato/2020-01-01", entries[0].Remote())
	assert.Equal(t, "potato/2020-12-31", entries[len(entries)-1].Remote())
}

func TestPatternYearMonthDayFilter(t *testing.T) {
	ctx := context.Background()
	f := newTestLister(t)

	// Years
	sf, err := yearMonthDayFilter(ctx, f, []string{"", "2000"})
	require.NoError(t, err)
	assert.Equal(t, api.SearchFilter{
		Filters: &api.Filters{
			DateFilter: &api.DateFilter{
				Dates: []api.Date{
					{
						Year: 2000,
					},
				},
			},
		},
	}, sf)

	_, err = yearMonthDayFilter(ctx, f, []string{"", "potato"})
	require.Error(t, err)
	_, err = yearMonthDayFilter(ctx, f, []string{"", "999"})
	require.Error(t, err)
	_, err = yearMonthDayFilter(ctx, f, []string{"", "4000"})
	require.Error(t, err)

	// Months
	sf, err = yearMonthDayFilter(ctx, f, []string{"", "2000", "01"})
	require.NoError(t, err)
	assert.Equal(t, api.SearchFilter{
		Filters: &api.Filters{
			DateFilter: &api.DateFilter{
				Dates: []api.Date{
					{
						Month: 1,
						Year:  2000,
					},
				},
			},
		},
	}, sf)

	_, err = yearMonthDayFilter(ctx, f, []string{"", "2000", "potato"})
	require.Error(t, err)
	_, err = yearMonthDayFilter(ctx, f, []string{"", "2000", "0"})
	require.Error(t, err)
	_, err = yearMonthDayFilter(ctx, f, []string{"", "2000", "13"})
	require.Error(t, err)

	// Days
	sf, err = yearMonthDayFilter(ctx, f, []string{"", "2000", "01", "02"})
	require.NoError(t, err)
	assert.Equal(t, api.SearchFilter{
		Filters: &api.Filters{
			DateFilter: &api.DateFilter{
				Dates: []api.Date{
					{
						Day:   2,
						Month: 1,
						Year:  2000,
					},
				},
			},
		},
	}, sf)

	_, err = yearMonthDayFilter(ctx, f, []string{"", "2000", "01", "potato"})
	require.Error(t, err)
	_, err = yearMonthDayFilter(ctx, f, []string{"", "2000", "01", "0"})
	require.Error(t, err)
	_, err = yearMonthDayFilter(ctx, f, []string{"", "2000", "01", "32"})
	require.Error(t, err)
}

func TestPatternAlbumsToEntries(t *testing.T) {
	f := newTestLister(t)
	ctx := context.Background()

	_, err := albumsToEntries(ctx, f, false, "potato/", "sub")
	assert.Equal(t, fs.ErrorDirNotFound, err)

	f.albums.add(&api.Album{
		ID:    "1",
		Title: "sub/one",
	})

	entries, err := albumsToEntries(ctx, f, false, "potato/", "sub")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(entries))
	assert.Equal(t, "potato/one", entries[0].Remote())
	_, ok := entries[0].(fs.Directory)
	assert.Equal(t, true, ok)

	f.albums.add(&api.Album{
		ID:    "1",
		Title: "sub",
	})
	f.names = []string{"file.jpg"}

	entries, err = albumsToEntries(ctx, f, false, "potato/", "sub")
	assert.NoError(t, err)
	assert.Equal(t, 2, len(entries))
	assert.Equal(t, "potato/one", entries[0].Remote())
	_, ok = entries[0].(fs.Directory)
	assert.Equal(t, true, ok)
	assert.Equal(t, "potato/file.jpg", entries[1].Remote())
	_, ok = entries[1].(fs.Object)
	assert.Equal(t, true, ok)

}
