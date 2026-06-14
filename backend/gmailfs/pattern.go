// Store the parsing of file patterns

package gmailfs

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
)

// lister describes the subset of the interfaces on Fs needed for the
// file pattern parsing
type lister interface {
	listThreads(ctx context.Context, prefix, year, month, day string) (fs.DirEntries, error)
	listThread(ctx context.Context, prefix, threadDir string) (fs.DirEntries, error)
	listAttachments(ctx context.Context, prefix, threadDir string) (fs.DirEntries, error)
	dirTime() time.Time
	startYear() int
}

// dirPattern describes a single directory pattern
type dirPattern struct {
	re        string
	match     *regexp.Regexp
	isFile    bool
	toEntries func(ctx context.Context, f lister, prefix string, match []string) (fs.DirEntries, error)
}

// dirPatterns is a slice of dirPattern
type dirPatterns []dirPattern

// mustCompile compiles every pattern's regexp in place.
func (ds dirPatterns) mustCompile() dirPatterns {
	for i := range ds {
		ds[i].match = regexp.MustCompile(ds[i].re)
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

// listYears returns one dir per year from startYear to the current year.
func listYears(ctx context.Context, f lister, prefix string, match []string) (entries fs.DirEntries, err error) {
	currentYear := f.dirTime().Year()
	for year := f.startYear(); year <= currentYear; year++ {
		entries = append(entries, fs.NewDir(prefix+fmt.Sprint(year), f.dirTime()))
	}
	return entries, nil
}

// listMonths returns the twelve month dirs for a year.
func listMonths(ctx context.Context, f lister, prefix string, match []string) (entries fs.DirEntries, err error) {
	year := match[1]
	for month := 1; month <= 12; month++ {
		entries = append(entries, fs.NewDir(fmt.Sprintf("%s%s-%02d", prefix, year, month), f.dirTime()))
	}
	return entries, nil
}

// listDays returns one dir per day in the month identified by match[2] (YYYY-MM).
func listDays(ctx context.Context, f lister, prefix string, match []string) (entries fs.DirEntries, err error) {
	t, err := time.Parse("2006-01", match[2])
	if err != nil {
		return nil, err
	}
	month := t.Month()
	current := time.Date(t.Year(), month, 1, 0, 0, 0, 0, time.UTC)
	for current.Month() == month {
		entries = append(entries, fs.NewDir(prefix+current.Format("2006-01-02"), f.dirTime()))
		current = current.AddDate(0, 0, 1)
	}
	return entries, nil
}

// patterns describes the layout of the gmail backend file system.
//
// NB no trailing / on paths. More-specific (attachments) patterns come before
// the generic thread-dir pattern so they are not shadowed.
var patterns = dirPatterns{
	{ // root → years
		re:        `^$`,
		toEntries: listYears,
	},
	{ // year → months
		re:        `^(\d{4})$`,
		toEntries: listMonths,
	},
	{ // month → days
		re:        `^(\d{4})/(\d{4}-\d{2})$`,
		toEntries: listDays,
	},
	{ // day → threads
		re: `^(\d{4})/(\d{4}-\d{2})/(\d{4}-\d{2}-\d{2})$`,
		toEntries: func(ctx context.Context, f lister, prefix string, match []string) (fs.DirEntries, error) {
			parts := strings.Split(match[3], "-")
			return f.listThreads(ctx, prefix, parts[0], parts[1], parts[2])
		},
	},
	{ // attachments dir (before thread dir to avoid shadowing)
		re: `^(\d{4})/(\d{4}-\d{2})/(\d{4}-\d{2}-\d{2})/(.+)/attachments$`,
		toEntries: func(ctx context.Context, f lister, prefix string, match []string) (fs.DirEntries, error) {
			return f.listAttachments(ctx, prefix, match[4])
		},
	},
	{ // attachment file
		re:     `^(\d{4})/(\d{4}-\d{2})/(\d{4}-\d{2}-\d{2})/([^/]+)/attachments/([^/]+)$`,
		isFile: true,
	},
	{ // .eml file (before generic thread dir; must come before thread-dir dir pattern of same depth as files are matched separately)
		re:     `^(\d{4})/(\d{4}-\d{2})/(\d{4}-\d{2}-\d{2})/([^/]+)/([^/]+\.eml)$`,
		isFile: true,
	},
	{ // thread dir → messages + attachments dir
		re: `^(\d{4})/(\d{4}-\d{2})/(\d{4}-\d{2}-\d{2})/(.+)$`,
		toEntries: func(ctx context.Context, f lister, prefix string, match []string) (fs.DirEntries, error) {
			return f.listThread(ctx, prefix, match[4])
		},
	},
}.mustCompile()
