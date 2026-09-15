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
	startYear(ctx context.Context) int
	showEmptyDirs() bool
	capturedDates(ctx context.Context) ([]time.Time, error)
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
			// prefix (trimmed, matching how Mkdir below builds the same
			// key), not match[0]: f.uploaded keys its entries by their own
			// (root-relative) Remote(), the same as everywhere else in
			// this backend - match[0] is always the absolute matched text
			// ("upload", "upload/foo", ...), which only equals the
			// root-relative path when this Fs happens to be rooted at ""
			// (true for every existing test, false for as ordinary an
			// invocation as "rclone copy file remote:upload", which roots
			// the Fs at "upload" itself) - confirmed live, using match[0]
			// there makes every listUploads lookup miss silently.
			return f.listUploads(ctx, strings.Trim(prefix, "/"))
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
// year, month and/or day. A zero field means "any" - see matches.
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

// Return the years from startYear to today - only the ones with at least
// one item captured in them, unless --gopro-show-empty-dirs is set
func years(ctx context.Context, f lister, prefix string, match []string) (entries fs.DirEntries, err error) {
	currentYear := f.dirTime().Year()
	startYear := f.startYear(ctx)
	present, err := yearsPresent(ctx, f)
	if err != nil {
		return nil, err
	}
	for year := startYear; year <= currentYear; year++ {
		if present != nil && !present[year] {
			continue
		}
		entries = append(entries, fs.NewDir(prefix+fmt.Sprint(year), f.dirTime()))
	}
	return entries, nil
}

// Return the months in a given year - only the ones with at least one item
// captured in them, unless --gopro-show-empty-dirs is set
func months(ctx context.Context, f lister, prefix string, match []string) (entries fs.DirEntries, err error) {
	year, err := strconv.Atoi(match[1])
	if err != nil {
		return nil, fmt.Errorf("bad year %q", match[1])
	}
	present, err := monthsPresent(ctx, f, year)
	if err != nil {
		return nil, err
	}
	for month := 1; month <= 12; month++ {
		if present != nil && !present[month] {
			continue
		}
		entries = append(entries, fs.NewDir(fmt.Sprintf("%s%s-%02d", prefix, match[1], month), f.dirTime()))
	}
	return entries, nil
}

// Return the days in a given year - only the ones with at least one item
// captured on them, unless --gopro-show-empty-dirs is set
func days(ctx context.Context, f lister, prefix string, match []string) (entries fs.DirEntries, err error) {
	year := match[1]
	current, err := time.Parse("2006", year)
	if err != nil {
		return nil, fmt.Errorf("bad year %q", match[1])
	}
	currentYear := current.Year()
	present, err := daysPresent(ctx, f, currentYear)
	if err != nil {
		return nil, err
	}
	for current.Year() == currentYear {
		if present == nil || present[current.YearDay()] {
			entries = append(entries, fs.NewDir(prefix+current.Format("2006-01-02"), f.dirTime()))
		}
		current = current.AddDate(0, 0, 1)
	}
	return entries, nil
}

// yearsPresent returns the set of years with at least one captured item, or
// nil if --gopro-show-empty-dirs is set (meaning "don't filter, show every
// year in range regardless of content").
func yearsPresent(ctx context.Context, f lister) (map[int]bool, error) {
	if f.showEmptyDirs() {
		return nil, nil
	}
	dates, err := f.capturedDates(ctx)
	if err != nil {
		return nil, err
	}
	present := map[int]bool{}
	for _, d := range dates {
		present[d.Year()] = true
	}
	return present, nil
}

// monthsPresent is yearsPresent's counterpart for the months (1-12) with at
// least one item captured in them within year.
func monthsPresent(ctx context.Context, f lister, year int) (map[int]bool, error) {
	if f.showEmptyDirs() {
		return nil, nil
	}
	dates, err := f.capturedDates(ctx)
	if err != nil {
		return nil, err
	}
	present := map[int]bool{}
	for _, d := range dates {
		if d.Year() == year {
			present[int(d.Month())] = true
		}
	}
	return present, nil
}

// daysPresent is yearsPresent's counterpart for the days of the year
// (1-366, i.e. time.Time.YearDay) with at least one item captured on them
// within year.
func daysPresent(ctx context.Context, f lister, year int) (map[int]bool, error) {
	if f.showEmptyDirs() {
		return nil, nil
	}
	dates, err := f.capturedDates(ctx)
	if err != nil {
		return nil, err
	}
	present := map[int]bool{}
	for _, d := range dates {
		if d.Year() == year {
			present[d.YearDay()] = true
		}
	}
	return present, nil
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
