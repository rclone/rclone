package gopro

import (
	"context"
	"testing"
	"time"

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
	names    []string
	uploaded dirtree.DirTree
}

func newTestLister() *testLister {
	return &testLister{uploaded: dirtree.New()}
}

func (f *testLister) listDir(ctx context.Context, prefix string, filter mediaFilter) (entries fs.DirEntries, err error) {
	for _, name := range f.names {
		entries = append(entries, mockobject.New(prefix+name))
	}
	return entries, nil
}

func (f *testLister) listUploads(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	return f.uploaded[dir], nil
}

func (f *testLister) dirTime() time.Time { return startTime }
func (f *testLister) startYear() int     { return 2015 }

// find returns the pattern matching re, failing the test if there isn't
// exactly one
func findPattern(t *testing.T, re string) *dirPattern {
	t.Helper()
	for i := range patterns {
		if patterns[i].re == re {
			return &patterns[i]
		}
	}
	require.Failf(t, "pattern not found", "re=%q", re)
	return nil
}

func TestPatternMatch(t *testing.T) {
	for _, test := range []struct {
		name        string
		root        string
		itemPath    string
		isFile      bool
		wantMatch   []string
		wantPrefix  string
		wantPattern string // re of the expected pattern, "" for nil
	}{
		{
			name:        "root dir",
			root:        "",
			itemPath:    "",
			isFile:      false,
			wantMatch:   []string{""},
			wantPattern: `^$`,
		},
		{
			name:        "root as file",
			root:        "",
			itemPath:    "",
			isFile:      true,
			wantPattern: "",
		},
		{
			name:        "upload dir at root",
			root:        "upload",
			itemPath:    "",
			isFile:      false,
			wantMatch:   []string{"upload", ""},
			wantPattern: `^upload(?:/(.*))?$`,
		},
		{
			name:        "upload subdir",
			root:        "upload/dir",
			itemPath:    "",
			isFile:      false,
			wantMatch:   []string{"upload/dir", "dir"},
			wantPattern: `^upload(?:/(.*))?$`,
		},
		{
			name:        "upload file",
			root:        "upload/file.jpg",
			itemPath:    "",
			isFile:      true,
			wantMatch:   []string{"upload/file.jpg", "file.jpg"},
			wantPattern: `^upload/(.*)$`,
		},
		{
			name:        "media dir",
			root:        "",
			itemPath:    "media",
			isFile:      false,
			wantMatch:   []string{"media"},
			wantPrefix:  "media/",
			wantPattern: `^media$`,
		},
		{
			name:        "media/all",
			root:        "",
			itemPath:    "media/all",
			isFile:      false,
			wantMatch:   []string{"media/all"},
			wantPrefix:  "media/all/",
			wantPattern: `^media/all$`,
		},
		{
			name:        "media/all file",
			root:        "",
			itemPath:    "media/all/GX010123.MP4",
			isFile:      true,
			wantMatch:   []string{"media/all/GX010123.MP4", "GX010123.MP4"},
			wantPrefix:  "media/all/GX010123.MP4/",
			wantPattern: `^media/all/([^/]+)$`,
		},
		{
			name:        "media/by-year",
			root:        "",
			itemPath:    "media/by-year",
			isFile:      false,
			wantMatch:   []string{"media/by-year"},
			wantPrefix:  "media/by-year/",
			wantPattern: `^media/by-year$`,
		},
		{
			name:        "media/by-year/2026",
			root:        "",
			itemPath:    "media/by-year/2026",
			isFile:      false,
			wantMatch:   []string{"media/by-year/2026", "2026"},
			wantPrefix:  "media/by-year/2026/",
			wantPattern: `^media/by-year/(\d{4})$`,
		},
		{
			name:        "media/by-month/2026/2026-08",
			root:        "",
			itemPath:    "media/by-month/2026/2026-08",
			isFile:      false,
			wantMatch:   []string{"media/by-month/2026/2026-08", "2026", "08"},
			wantPrefix:  "media/by-month/2026/2026-08/",
			wantPattern: `^media/by-month/\d{4}/(\d{4})-(\d{2})$`,
		},
		{
			name:        "media/by-day/2026/2026-08-31",
			root:        "",
			itemPath:    "media/by-day/2026/2026-08-31",
			isFile:      false,
			wantMatch:   []string{"media/by-day/2026/2026-08-31", "2026", "08", "31"},
			wantPrefix:  "media/by-day/2026/2026-08-31/",
			wantPattern: `^media/by-day/\d{4}/(\d{4})-(\d{2})-(\d{2})$`,
		},
		{
			name:        "media/by-day file",
			root:        "",
			itemPath:    "media/by-day/2026/2026-08-31/GX010123.MP4",
			isFile:      true,
			wantMatch:   []string{"media/by-day/2026/2026-08-31/GX010123.MP4", "2026", "08", "31", "GX010123.MP4"},
			wantPrefix:  "media/by-day/2026/2026-08-31/GX010123.MP4/",
			wantPattern: `^media/by-day/\d{4}/(\d{4})-(\d{2})-(\d{2})/([^/]+)$`,
		},
		{
			name:        "nonsense path",
			root:        "",
			itemPath:    "album/foo",
			isFile:      false,
			wantPattern: "",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			match, prefix, pattern := patterns.match(test.root, test.itemPath, test.isFile)
			assert.Equal(t, test.wantMatch, match)
			assert.Equal(t, test.wantPrefix, prefix)
			if test.wantPattern == "" {
				assert.Nil(t, pattern)
			} else {
				require.NotNil(t, pattern)
				assert.Equal(t, test.wantPattern, pattern.re)
			}
		})
	}
}

func TestPatternRootAndUploadAreOnlyTopLevelDirs(t *testing.T) {
	f := newTestLister()
	_, _, pattern := patterns.match("", "", false)
	require.NotNil(t, pattern)
	entries, err := pattern.toEntries(context.Background(), f, "", nil)
	require.NoError(t, err)
	var names []string
	for _, e := range entries {
		names = append(names, e.Remote())
	}
	assert.ElementsMatch(t, []string{"media", "upload"}, names)
}

func TestPatternMediaSubdirs(t *testing.T) {
	f := newTestLister()
	_, _, pattern := patterns.match("", "media", false)
	require.NotNil(t, pattern)
	entries, err := pattern.toEntries(context.Background(), f, "media/", nil)
	require.NoError(t, err)
	var names []string
	for _, e := range entries {
		names = append(names, e.Remote())
	}
	assert.ElementsMatch(t, []string{"media/all", "media/by-year", "media/by-month", "media/by-day"}, names)
}

func TestPatternYears(t *testing.T) {
	f := newTestLister()
	entries, err := years(context.Background(), f, "", nil)
	require.NoError(t, err)
	require.NotEmpty(t, entries)
	assert.Equal(t, "2015", entries[0].Remote())
	assert.Equal(t, "2019", entries[len(entries)-1].Remote())
}

func TestPatternMonths(t *testing.T) {
	f := newTestLister()
	entries, err := months(context.Background(), f, "", []string{"", "2026"})
	require.NoError(t, err)
	require.Len(t, entries, 12)
	assert.Equal(t, "2026-01", entries[0].Remote())
	assert.Equal(t, "2026-12", entries[11].Remote())
}

func TestPatternDays(t *testing.T) {
	f := newTestLister()
	entries, err := days(context.Background(), f, "", []string{"", "2026"})
	require.NoError(t, err)
	require.Len(t, entries, 365) // 2026 is not a leap year
	assert.Equal(t, "2026-01-01", entries[0].Remote())
	assert.Equal(t, "2026-12-31", entries[len(entries)-1].Remote())
}

func TestPatternYearMonthDayFilter(t *testing.T) {
	mf, err := yearMonthDayFilter([]string{"", "2026"})
	require.NoError(t, err)
	assert.Equal(t, mediaFilter{year: 2026}, mf)

	mf, err = yearMonthDayFilter([]string{"", "2026", "08"})
	require.NoError(t, err)
	assert.Equal(t, mediaFilter{year: 2026, month: 8}, mf)

	mf, err = yearMonthDayFilter([]string{"", "2026", "08", "31"})
	require.NoError(t, err)
	assert.Equal(t, mediaFilter{year: 2026, month: 8, day: 31}, mf)

	_, err = yearMonthDayFilter([]string{"", "not-a-year"})
	assert.Error(t, err)

	_, err = yearMonthDayFilter([]string{"", "2026", "13"})
	assert.Error(t, err)

	_, err = yearMonthDayFilter([]string{"", "2026", "08", "32"})
	assert.Error(t, err)
}

func TestMediaFilterMatches(t *testing.T) {
	captured := fstest.Time("2026-08-31T10:00:00Z")
	assert.True(t, mediaFilter{}.matches(captured))
	assert.True(t, mediaFilter{year: 2026}.matches(captured))
	assert.False(t, mediaFilter{year: 2025}.matches(captured))
	assert.True(t, mediaFilter{year: 2026, month: 8}.matches(captured))
	assert.False(t, mediaFilter{year: 2026, month: 7}.matches(captured))
	assert.True(t, mediaFilter{year: 2026, month: 8, day: 31}.matches(captured))
	assert.False(t, mediaFilter{year: 2026, month: 8, day: 30}.matches(captured))
}

func TestPatternFindPatternHelper(t *testing.T) {
	// sanity check the test helper itself finds a known pattern
	p := findPattern(t, `^media/all$`)
	assert.False(t, p.isFile)
	assert.NotNil(t, p.toEntries)
}
