// Package gcalfs provides the Google Calendar filesystem pattern matching.
package gcalfs

import (
	"context"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
)

// dirPattern describes a single directory pattern
type dirPattern struct {
	re        string
	match     *regexp.Regexp
	isFile    bool
	toEntries func(ctx context.Context, f lister, prefix string, match []string) (fs.DirEntries, error)
}

// dirPatterns is a slice of dirPattern
type dirPatterns []dirPattern

// lister is the interface the pattern system needs
type lister interface {
	listCalendars(ctx context.Context, prefix string) (fs.DirEntries, error)
	listYears(ctx context.Context, prefix, calendarID string) (fs.DirEntries, error)
	listMonths(ctx context.Context, prefix, calendarID, year string) (fs.DirEntries, error)
	listDays(ctx context.Context, prefix, calendarID, year, month string) (fs.DirEntries, error)
	listEvents(ctx context.Context, prefix, calendarID, year, month, day string) (fs.DirEntries, error)
	calendarIDForName(name string) (string, bool)
	dirTime() time.Time
	startYear() int
}

// mustCompile compiles all regexps
func (ds dirPatterns) mustCompile() dirPatterns {
	for i := range ds {
		ds[i].match = regexp.MustCompile(ds[i].re)
	}
	return ds
}

// match finds a pattern
func (ds dirPatterns) match(root, itemPath string, isFile bool) ([]string, string, *dirPattern) {
	itemPath = strings.Trim(itemPath, "/")
	absPath := path.Join(root, itemPath)
	prefix := strings.Trim(absPath[len(root):], "/")
	if prefix != "" {
		prefix += "/"
	}
	for i := range ds {
		p := &ds[i]
		if p.isFile != isFile {
			continue
		}
		m := p.match.FindStringSubmatch(absPath)
		if m != nil {
			return m, prefix, p
		}
	}
	return nil, "", nil
}

// patterns is the compiled pattern set.
var patterns = dirPatterns{
	{ // root → list calendars
		re: `^$`,
		toEntries: func(ctx context.Context, f lister, prefix string, match []string) (fs.DirEntries, error) {
			return f.listCalendars(ctx, prefix)
		},
	},
	{ // calendar → list years: "CalendarName"
		re: `^([^/]+)$`,
		toEntries: func(ctx context.Context, f lister, prefix string, match []string) (fs.DirEntries, error) {
			calID, ok := f.calendarIDForName(match[1])
			if !ok {
				return nil, fs.ErrorDirNotFound
			}
			return f.listYears(ctx, prefix, calID)
		},
	},
	{ // year → list months: "CalendarName/2024"
		re: `^([^/]+)/(\d{4})$`,
		toEntries: func(ctx context.Context, f lister, prefix string, match []string) (fs.DirEntries, error) {
			calID, ok := f.calendarIDForName(match[1])
			if !ok {
				return nil, fs.ErrorDirNotFound
			}
			return f.listMonths(ctx, prefix, calID, match[2])
		},
	},
	{ // month → list days: "CalendarName/2024/2024-01"
		re: `^([^/]+)/(\d{4})/(\d{4}-\d{2})$`,
		toEntries: func(ctx context.Context, f lister, prefix string, match []string) (fs.DirEntries, error) {
			calID, ok := f.calendarIDForName(match[1])
			if !ok {
				return nil, fs.ErrorDirNotFound
			}
			return f.listDays(ctx, prefix, calID, match[2], match[3])
		},
	},
	{ // day → list events: "CalendarName/2024/2024-01/2024-01-15"
		re: `^([^/]+)/(\d{4})/(\d{4}-\d{2})/(\d{4}-\d{2}-\d{2})$`,
		toEntries: func(ctx context.Context, f lister, prefix string, match []string) (fs.DirEntries, error) {
			calID, ok := f.calendarIDForName(match[1])
			if !ok {
				return nil, fs.ErrorDirNotFound
			}
			parts := strings.Split(match[4], "-")
			return f.listEvents(ctx, prefix, calID, match[2], parts[1], parts[2])
		},
	},
	{ // .ics file: "CalendarName/2024/2024-01/2024-01-15/evtID — Summary.ics"
		re:     `^([^/]+)/(\d{4})/(\d{4}-\d{2})/(\d{4}-\d{2}-\d{2})/([^/]+\.ics)$`,
		isFile: true,
	},
}.mustCompile()
