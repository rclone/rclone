// Store the parsing of file patterns
//
// The layout is a trimmed-down version of the googlephotos backend's
// pattern.go: GoPro Media Library has no album concept, only a flat,
// date-stamped library and (once uploaded) an upload/ directory of items
// rclone itself has created.

package gopro

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
)

// lister describes the subset of the interfaces on Fs needed for the
// file pattern parsing
type lister interface {
	listDir(ctx context.Context, prefix string, filter mediaFilter) (entries fs.DirEntries, err error)
	listUploads(ctx context.Context, dir string) (entries fs.DirEntries, err error)
	dirTime() time.Time
	startYear() int
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

// patterns describes the layout of the gopro backend file system.
//
// NB no trailing / on paths
var patterns = dirPatterns{
	{
		re: `^$`,
		toEntries: func(ctx context.Context, f lister, prefix string, match []string) (fs.DirEntries, error) {
			return fs.DirEntries{
				fs.NewDir(prefix+"media", f.dirTime()),
				fs.NewDir(prefix+"upload", f.dirTime()),
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
			return f.listDir(ctx, prefix, mediaFilter{})
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
			filter, err := yearMonthDayFilter(match)
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
			filter, err := yearMonthDayFilter(match)
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
			filter, err := yearMonthDayFilter(match)
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

// mediaFilter restricts a directory listing to media captured on a given
// year, month and/or day. A zero field means "any". Applied server-side
// via capturedRange(), with matches() as a client-side backstop in case
// the server-side bound is ever inexact.
type mediaFilter struct {
	year, month, day int
}

// matches reports whether t falls within the filter
func (mf mediaFilter) matches(t time.Time) bool {
	if mf.year != 0 && t.Year() != mf.year {
		return false
	}
	if mf.month != 0 && int(t.Month()) != mf.month {
		return false
	}
	if mf.day != 0 && t.Day() != mf.day {
		return false
	}
	return true
}

// capturedRange returns the inclusive UTC instant bounds of the filter, for
// use as the API's captured_range parameter. ok is false for an empty
// filter (nothing to bound).
//
// "day"/"month"/"year" here are UTC calendar dates, matching how captured_at
// is interpreted elsewhere in this backend (matches, and the by-day/by-month
// directory names themselves) - not the capturing camera's local time zone,
// which /media/search doesn't expose a way to filter on anyway.
func (mf mediaFilter) capturedRange() (start, end time.Time, ok bool) {
	if mf.year == 0 {
		return time.Time{}, time.Time{}, false
	}
	switch {
	case mf.day != 0:
		start = time.Date(mf.year, time.Month(mf.month), mf.day, 0, 0, 0, 0, time.UTC)
		end = start.AddDate(0, 0, 1)
	case mf.month != 0:
		start = time.Date(mf.year, time.Month(mf.month), 1, 0, 0, 0, 0, time.UTC)
		end = start.AddDate(0, 1, 0)
	default:
		start = time.Date(mf.year, 1, 1, 0, 0, 0, 0, time.UTC)
		end = start.AddDate(1, 0, 0)
	}
	return start, end.Add(-time.Millisecond), true
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

// yearMonthDayFilter builds a mediaFilter from the year[/month[/day]]
// captured by a by-year/by-month/by-day pattern
func yearMonthDayFilter(match []string) (mf mediaFilter, err error) {
	year, err := strconv.Atoi(match[1])
	if err != nil || year < 1000 || year > 3000 {
		return mf, fmt.Errorf("bad year %q", match[1])
	}
	mf.year = year
	if len(match) >= 3 {
		month, err := strconv.Atoi(match[2])
		if err != nil || month < 1 || month > 12 {
			return mf, fmt.Errorf("bad month %q", match[2])
		}
		mf.month = month
	}
	if len(match) >= 4 {
		day, err := strconv.Atoi(match[3])
		if err != nil || day < 1 || day > 31 {
			return mf, fmt.Errorf("bad day %q", match[3])
		}
		mf.day = day
	}
	return mf, nil
}
