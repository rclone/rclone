// Store the parsing of file patterns

package googlephotos

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rclone/rclone/backend/googlephotos/api"
	"github.com/rclone/rclone/fs"
)

// lister describes the subset of the interfaces on Fs needed for the
// file pattern parsing
type lister interface {
	listDir(ctx context.Context, prefix string, filter api.SearchFilter) (entries fs.DirEntries, err error)
	listAlbums(ctx context.Context, shared bool) (all *albums, err error)
	listUploads(ctx context.Context, dir string) (entries fs.DirEntries, err error)
	dirTime() time.Time
	startYear() int
	includeArchived() bool
}

// dirPattern describes a single directory pattern
type dirPattern struct {
	re        string         // match for the path
	match     *regexp.Regexp // compiled match
	canUpload bool           // true if can upload here
	canMkdir  bool           // true if can make a directory here
	isFile    bool           // true if this is a file
	isUpload  bool           // true if this is the upload directory
	// function to turn a match into DirEntries
	toEntries func(ctx context.Context, f lister, prefix string, match []string) (fs.DirEntries, error)
}

// dirPatterns is a slice of all the directory patterns
type dirPatterns []dirPattern

// patterns describes the layout of the google photos backend file system.
//
// NB no trailing / on paths
var patterns = dirPatterns{
	{
		re: `^$`,
		toEntries: func(ctx context.Context, f lister, prefix string, match []string) (fs.DirEntries, error) {
			return fs.DirEntries{
				fs.NewDir(prefix+"media", f.dirTime()),
				fs.NewDir(prefix+"album", f.dirTime()),
				fs.NewDir(prefix+"shared-album", f.dirTime()),
				fs.NewDir(prefix+"upload", f.dirTime()),
				fs.NewDir(prefix+"feature", f.dirTime()),
			}, nil
		},
	},
	{
		re: `^upload(?:/(.*))?$`,
		toEntries: func(ctx context.Context, f lister, prefix string, match []string) (fs.DirEntries, error) {
			return f.listUploads(ctx, match[0])
		},
		canUpload: true,
		canMkdir:  true,
		isUpload:  true,
	},
	{
		re:        `^upload/(.*)$`,
		isFile:    true,
		canUpload: true,
		isUpload:  true,
	},
	{
		re: `^media$`,
		toEntries: func(ctx context.Context, f lister, prefix string, match []string) (fs.DirEntries, error) {
			return fs.DirEntries{
				fs.NewDir(prefix+"all", f.dirTime()),
				fs.NewDir(prefix+"by-year", f.dirTime()),
				fs.NewDir(prefix+"by-month", f.dirTime()),
				fs.NewDir(prefix+"by-day", f.dirTime()),
			}, nil
		},
	},
	{
		re: `^media/all$`,
		toEntries: func(ctx context.Context, f lister, prefix string, match []string) (fs.DirEntries, error) {
			return f.listDir(ctx, prefix, api.SearchFilter{})
		},
	},
	{
		re:     `^media/all/([^/]+)$`,
		isFile: true,
	},
	{
		re:        `^media/by-year$`,
		toEntries: years,
	},
	{
		re: `^media/by-year/(\d{4})$`,
		toEntries: func(ctx context.Context, f lister, prefix string, match []string) (fs.DirEntries, error) {
			filter, err := yearMonthDayFilter(ctx, f, match)
			if err != nil {
				return nil, err
			}
			return f.listDir(ctx, prefix, filter)
		},
	},
	{
		re:     `^media/by-year/(\d{4})/([^/]+)$`,
		isFile: true,
	},
	{
		re:        `^media/by-month$`,
		toEntries: years,
	},
	{
		re:        `^media/by-month/(\d{4})$`,
		toEntries: months,
	},
	{
		re: `^media/by-month/\d{4}/(\d{4})-(\d{2})$`,
		toEntries: func(ctx context.Context, f lister, prefix string, match []string) (fs.DirEntries, error) {
			filter, err := yearMonthDayFilter(ctx, f, match)
			if err != nil {
				return nil, err
			}
			return f.listDir(ctx, prefix, filter)
		},
	},
	{
		re:     `^media/by-month/\d{4}/(\d{4})-(\d{2})/([^/]+)$`,
		isFile: true,
	},
	{
		re:        `^media/by-day$`,
		toEntries: years,
	},
	{
		re:        `^media/by-day/(\d{4})$`,
		toEntries: days,
	},
	{
		re: `^media/by-day/\d{4}/(\d{4})-(\d{2})-(\d{2})$`,
		toEntries: func(ctx context.Context, f lister, prefix string, match []string) (fs.DirEntries, error) {
			filter, err := yearMonthDayFilter(ctx, f, match)
			if err != nil {
				return nil, err
			}
			return f.listDir(ctx, prefix, filter)
		},
	},
	{
		re:     `^media/by-day/\d{4}/(\d{4})-(\d{2})-(\d{2})/([^/]+)$`,
		isFile: true,
	},
	{
		re: `^album$`,
		toEntries: func(ctx context.Context, f lister, prefix string, match []string) (entries fs.DirEntries, err error) {
			return albumsToEntries(ctx, f, false, prefix, "")
		},
	},
	{
		re:       `^album/(.+)$`,
		canMkdir: true,
		toEntries: func(ctx context.Context, f lister, prefix string, match []string) (entries fs.DirEntries, err error) {
			return albumsToEntries(ctx, f, false, prefix, match[1])

		},
	},
	{
		re:        `^album/(.+?)/([^/]+)$`,
		canUpload: true,
		isFile:    true,
	},
	{
		re: `^shared-album$`,
		toEntries: func(ctx context.Context, f lister, prefix string, match []string) (entries fs.DirEntries, err error) {
			return albumsToEntries(ctx, f, true, prefix, "")
		},
	},
	{
		re: `^shared-album/(.+)$`,
		toEntries: func(ctx context.Context, f lister, prefix string, match []string) (entries fs.DirEntries, err error) {
			return albumsToEntries(ctx, f, true, prefix, match[1])

		},
	},
	{
		re:     `^shared-album/(.+?)/([^/]+)$`,
		isFile: true,
	},
	{
		re: `^feature$`,
		toEntries: func(ctx context.Context, f lister, prefix string, match []string) (entries fs.DirEntries, err error) {
			return fs.DirEntries{
				fs.NewDir(prefix+"favorites", f.dirTime()),
			}, nil
		},
	},
	{
		re: `^feature/favorites$`,
		toEntries: func(ctx context.Context, f lister, prefix string, match []string) (entries fs.DirEntries, err error) {
			filter := featureFilter(ctx, f, match)
			if err != nil {
				return nil, err
			}
			return f.listDir(ctx, prefix, filter)
		},
	},
	{
		re:     `^feature/favorites/([^/]+)$`,
		isFile: true,
	},
}.mustCompile()

// mustCompile compiles the regexps in the dirPatterns
func (ds dirPatterns) mustCompile() dirPatterns {
	for i := range ds {
		pattern := &ds[i]
		pattern.match = regexp.MustCompile(pattern.re)
	}
	return ds
}

// match finds the path passed in the matching structure and
// returns the parameters and a pointer to the match, or nil.
func (ds dirPatterns) match(root string, itemPath string, isFile bool) (match []string, prefix string, pattern *dirPattern) {
	itemPath = strings.Trim(itemPath, "/")
	absPath := path.Join(root, itemPath)
	prefix = strings.Trim(absPath[len(root):], "/")
	if prefix != "" {
		prefix += "/"
	}
	for i := range ds {
		pattern = &ds[i]
		if pattern.isFile != isFile {
			continue
		}
		match = pattern.match.FindStringSubmatch(absPath)
		if match != nil {
			return
		}
	}
	return nil, "", nil
}

// Return the years from startYear to today
func years(ctx context.Context, f lister, prefix string, match []string) (entries fs.DirEntries, err error) {
	currentYear := f.dirTime().Year()
	for year := f.startYear(); year <= currentYear; year++ {
		entries = append(entries, fs.NewDir(prefix+fmt.Sprint(year), f.dirTime()))
	}
	return entries, nil
}

// Return the months in a given year
func months(ctx context.Context, f lister, prefix string, match []string) (entries fs.DirEntries, err error) {
	year := match[1]
	for month := 1; month <= 12; month++ {
		entries = append(entries, fs.NewDir(fmt.Sprintf("%s%s-%02d", prefix, year, month), f.dirTime()))
	}
	return entries, nil
}

// Return the days in a given year
func days(ctx context.Context, f lister, prefix string, match []string) (entries fs.DirEntries, err error) {
	year := match[1]
	current, err := time.Parse("2006", year)
	if err != nil {
		return nil, fmt.Errorf("bad year %q", match[1])
	}
	currentYear := current.Year()
	for current.Year() == currentYear {
		entries = append(entries, fs.NewDir(prefix+current.Format("2006-01-02"), f.dirTime()))
		current = current.AddDate(0, 0, 1)
	}
	return entries, nil
}

// This creates a search filter on year/month/day as provided
func yearMonthDayFilter(ctx context.Context, f lister, match []string) (sf api.SearchFilter, err error) {
	year, err := strconv.Atoi(match[1])
	if err != nil || year < 1000 || year > 3000 {
		return sf, fmt.Errorf("bad year %q", match[1])
	}
	sf = api.SearchFilter{
		Filters: &api.Filters{
			DateFilter: &api.DateFilter{
				Dates: []api.Date{
					{
						Year: year,
					},
				},
			},
		},
	}
	if len(match) >= 3 {
		month, err := strconv.Atoi(match[2])
		if err != nil || month < 1 || month > 12 {
			return sf, fmt.Errorf("bad month %q", match[2])
		}
		sf.Filters.DateFilter.Dates[0].Month = month
	}
	if len(match) >= 4 {
		day, err := strconv.Atoi(match[3])
		if err != nil || day < 1 || day > 31 {
			return sf, fmt.Errorf("bad day %q", match[3])
		}
		sf.Filters.DateFilter.Dates[0].Day = day
	}
	return sf, nil
}

// featureFilter creates a filter for the Feature enum
//
// The API only supports one feature, FAVORITES, so hardcode that feature.
//
// https://developers.google.com/photos/library/reference/rest/v1/mediaItems/search#FeatureFilter
func featureFilter(ctx context.Context, f lister, match []string) (sf api.SearchFilter) {
	sf = api.SearchFilter{
		Filters: &api.Filters{
			FeatureFilter: &api.FeatureFilter{
				IncludedFeatures: []string{
					"FAVORITES",
				},
			},
		},
	}
	return sf
}

// Turns an albumPath into entries
//
// These can either be synthetic directory entries if the album path
// is a prefix of another album, or actual files, or a combination of
// the two.
func albumsToEntries(ctx context.Context, f lister, shared bool, prefix string, albumPath string) (entries fs.DirEntries, err error) {
	albums, err := f.listAlbums(ctx, shared)
	if err != nil {
		return nil, err
	}
	// Put in the directories
	dirs, foundAlbumPath := albums.getDirs(albumPath)
	if foundAlbumPath {
		for _, dir := range dirs {
			d := fs.NewDir(prefix+dir, f.dirTime())
			dirPath := path.Join(albumPath, dir)
			// if this dir is an album add more special stuff
			album, ok := albums.get(dirPath)
			if ok {
				count, err := strconv.ParseInt(album.MediaItemsCount, 10, 64)
				if err != nil {
					fs.Debugf(f, "Error reading media count: %v", err)
				}
				d.SetID(album.ID).SetItems(count)
			}
			entries = append(entries, d)
		}
	}
	// if this is an album then return a filter to list it
	album, foundAlbum := albums.get(albumPath)
	if foundAlbum {
		filter := api.SearchFilter{AlbumID: album.ID}
		newEntries, err := f.listDir(ctx, prefix, filter)
		if err != nil {
			return nil, err
		}
		entries = append(entries, newEntries...)
	}
	if !foundAlbumPath && !foundAlbum && albumPath != "" {
		return nil, fs.ErrorDirNotFound
	}
	return entries, nil
}
