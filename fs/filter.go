// Control the filtering of files

package fs

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/pflag"
)

// Global
var (
	// Flags
	deleteExcluded = pflag.BoolP("delete-excluded", "", false, "Delete files on dest excluded from sync")
	filterRule     = pflag.StringP("filter", "f", "", "Add a file-filtering rule")
	filterFrom     = pflag.StringP("filter-from", "", "", "Read filtering patterns from a file")
	excludeRule    = pflag.StringP("exclude", "", "", "Exclude files matching pattern")
	excludeFrom    = pflag.StringP("exclude-from", "", "", "Read exclude patterns from file")
	includeRule    = pflag.StringP("include", "", "", "Include files matching pattern")
	includeFrom    = pflag.StringP("include-from", "", "", "Read include patterns from file")
	filesFrom      = pflag.StringP("files-from", "", "", "Read list of source-file names from file")
	minAge         = pflag.StringP("min-age", "", "", "Don't transfer any file younger than this in s or suffix ms|s|m|h|d|w|M|y")
	maxAge         = pflag.StringP("max-age", "", "", "Don't transfer any file older than this in s or suffix ms|s|m|h|d|w|M|y")
	minSize        SizeSuffix
	maxSize        SizeSuffix
	dumpFilters    = pflag.BoolP("dump-filters", "", false, "Dump the filters to the output")
	//cvsExclude     = pflag.BoolP("cvs-exclude", "C", false, "Exclude files in the same way CVS does")
)

func init() {
	pflag.VarP(&minSize, "min-size", "", "Don't transfer any file smaller than this in k or suffix k|M|G")
	pflag.VarP(&maxSize, "max-size", "", "Don't transfer any file larger than this in k or suffix k|M|G")
}

// rule is one filter rule
type rule struct {
	Include bool
	Regexp  *regexp.Regexp
}

// Match returns true if rule matches path
func (r *rule) Match(path string) bool {
	return r.Regexp.MatchString(path)
}

// String the rule
func (r *rule) String() string {
	c := "-"
	if r.Include {
		c = "+"
	}
	return fmt.Sprintf("%s %s", c, r.Regexp.String())
}

// filesMap describes the map of files to transfer
type filesMap map[string]struct{}

// Filter describes any filtering in operation
type Filter struct {
	DeleteExcluded bool
	MinSize        int64
	MaxSize        int64
	ModTimeFrom    time.Time
	ModTimeTo      time.Time
	rules          []rule
	files          filesMap
}

// We use time conventions
var ageSuffixes = []struct {
	Suffix     string
	Multiplier time.Duration
}{
	{Suffix: "ms", Multiplier: time.Millisecond},
	{Suffix: "s", Multiplier: time.Second},
	{Suffix: "m", Multiplier: time.Minute},
	{Suffix: "h", Multiplier: time.Hour},
	{Suffix: "d", Multiplier: time.Hour * 24},
	{Suffix: "w", Multiplier: time.Hour * 24 * 7},
	{Suffix: "M", Multiplier: time.Hour * 24 * 30},
	{Suffix: "y", Multiplier: time.Hour * 24 * 365},

	// Default to second
	{Suffix: "", Multiplier: time.Second},
}

// ParseDuration parses a duration string. Accept ms|s|m|h|d|w|M|y suffixes. Defaults to second if not provided
func ParseDuration(age string) (time.Duration, error) {
	var period float64

	for _, ageSuffix := range ageSuffixes {
		if strings.HasSuffix(age, ageSuffix.Suffix) {
			numberString := age[:len(age)-len(ageSuffix.Suffix)]
			var err error
			period, err = strconv.ParseFloat(numberString, 64)
			if err != nil {
				return time.Duration(0), err
			}
			period *= float64(ageSuffix.Multiplier)
			break
		}
	}

	return time.Duration(period), nil
}

// NewFilter parses the command line options and creates a Filter object
func NewFilter() (f *Filter, err error) {
	f = &Filter{
		DeleteExcluded: *deleteExcluded,
		MinSize:        int64(minSize),
		MaxSize:        int64(maxSize),
	}

	if *includeRule != "" {
		err = f.Add(true, *includeRule)
		if err != nil {
			return nil, err
		}
		// Add implicit exclude
		err = f.Add(false, "*")
		if err != nil {
			return nil, err
		}
	}
	if *includeFrom != "" {
		err := forEachLine(*includeFrom, func(line string) error {
			return f.Add(true, line)
		})
		if err != nil {
			return nil, err
		}
		// Add implicit exclude
		err = f.Add(false, "*")
		if err != nil {
			return nil, err
		}
	}
	if *excludeRule != "" {
		err = f.Add(false, *excludeRule)
		if err != nil {
			return nil, err
		}
	}
	if *excludeFrom != "" {
		err := forEachLine(*excludeFrom, func(line string) error {
			return f.Add(false, line)
		})
		if err != nil {
			return nil, err
		}
	}
	if *filterRule != "" {
		err = f.AddRule(*filterRule)
		if err != nil {
			return nil, err
		}
	}
	if *filterFrom != "" {
		err := forEachLine(*filterFrom, f.AddRule)
		if err != nil {
			return nil, err
		}
	}
	if *filesFrom != "" {
		err := forEachLine(*filesFrom, func(line string) error {
			return f.AddFile(line)
		})
		if err != nil {
			return nil, err
		}
	}
	if *minAge != "" {
		duration, err := ParseDuration(*minAge)
		if err != nil {
			return nil, err
		}
		f.ModTimeTo = time.Now().Add(-duration)
		Debug(nil, "--min-age %v to %v", duration, f.ModTimeTo)
	}
	if *maxAge != "" {
		duration, err := ParseDuration(*maxAge)
		if err != nil {
			return nil, err
		}
		f.ModTimeFrom = time.Now().Add(-duration)
		if !f.ModTimeTo.IsZero() && f.ModTimeTo.Before(f.ModTimeFrom) {
			return nil, fmt.Errorf("Argument --min-age can't be larger than --max-age")
		}
		Debug(nil, "--max-age %v to %v", duration, f.ModTimeFrom)
	}
	if *dumpFilters {
		fmt.Println("--- start filters ---")
		fmt.Println(f.DumpFilters())
		fmt.Println("--- end filters ---")
	}
	return f, nil
}

// Add adds a filter rule with include or exclude status indicated
func (f *Filter) Add(Include bool, glob string) error {
	re, err := globToRegexp(glob)
	if err != nil {
		return err
	}
	rule := rule{
		Include: Include,
		Regexp:  re,
	}
	f.rules = append(f.rules, rule)
	return nil
}

// AddRule adds a filter rule with include/exclude indicated by the prefix
//
// These are
//
//   + glob
//   - glob
//   !
//
// '+' includes the glob, '-' excludes it and '!' resets the filter list
//
// Line comments may be introduced with '#' or ';'
func (f *Filter) AddRule(rule string) error {
	switch {
	case rule == "!":
		f.Clear()
		return nil
	case strings.HasPrefix(rule, "- "):
		return f.Add(false, rule[2:])
	case strings.HasPrefix(rule, "+ "):
		return f.Add(true, rule[2:])
	}
	return fmt.Errorf("Malformed rule %q", rule)
}

// AddFile adds a single file to the files from list
func (f *Filter) AddFile(file string) error {
	if f.files == nil {
		f.files = make(filesMap)
	}
	file = strings.Trim(file, "/")
	f.files[file] = struct{}{}
	return nil
}

// Clear clears all the filter rules
func (f *Filter) Clear() {
	f.rules = nil
}

// Include returns whether this object should be included into the
// sync or not
func (f *Filter) Include(remote string, size int64, modTime time.Time) bool {
	// filesFrom takes precedence
	if f.files != nil {
		_, include := f.files[remote]
		return include
	}
	if !f.ModTimeFrom.IsZero() && modTime.Before(f.ModTimeFrom) {
		return false
	}
	if !f.ModTimeTo.IsZero() && modTime.After(f.ModTimeTo) {
		return false
	}
	if f.MinSize != 0 && size < f.MinSize {
		return false
	}
	if f.MaxSize != 0 && size > f.MaxSize {
		return false
	}
	for _, rule := range f.rules {
		if rule.Match(remote) {
			return rule.Include
		}
	}
	return true
}

// IncludeObject returns whether this object should be included into
// the sync or not. This is a convenience function to avoid calling
// o.ModTime(), which is an expensive operation.
func (f *Filter) IncludeObject(o Object) bool {
	var modTime time.Time

	if !f.ModTimeFrom.IsZero() || !f.ModTimeTo.IsZero() {
		modTime = o.ModTime()
	} else {
		modTime = time.Unix(0, 0)
	}

	return f.Include(o.Remote(), o.Size(), modTime)
}

// forEachLine calls fn on every line in the file pointed to by path
//
// It ignores empty lines and lines starting with '#' or ';'
func forEachLine(path string, fn func(string) error) (err error) {
	in, err := os.Open(path)
	if err != nil {
		return err
	}
	defer CheckClose(in, &err)
	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		if len(line) == 0 || line[0] == '#' || line[0] == ';' {
			continue
		}
		err := fn(line)
		if err != nil {
			return err
		}
	}
	return scanner.Err()
}

// DumpFilters dumps the filters in textual form, 1 per line
func (f *Filter) DumpFilters() string {
	rules := []string{}
	if !f.ModTimeFrom.IsZero() {
		rules = append(rules, fmt.Sprintf("Last-modified date must be equal or greater than: %s", f.ModTimeFrom.String()))
	}
	if !f.ModTimeTo.IsZero() {
		rules = append(rules, fmt.Sprintf("Last-modified date must be equal or less than: %s", f.ModTimeTo.String()))
	}
	for _, rule := range f.rules {
		rules = append(rules, rule.String())
	}
	return strings.Join(rules, "\n")
}
