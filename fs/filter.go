// Control the filtering of files

package fs

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// Global
var (
	// Flags
	deleteExcluded = BoolP("delete-excluded", "", false, "Delete files on dest excluded from sync")
	filterRule     = StringArrayP("filter", "f", nil, "Add a file-filtering rule")
	filterFrom     = StringArrayP("filter-from", "", nil, "Read filtering patterns from a file")
	excludeRule    = StringArrayP("exclude", "", nil, "Exclude files matching pattern")
	excludeFrom    = StringArrayP("exclude-from", "", nil, "Read exclude patterns from file")
	excludeFile    = StringP("exclude-if-present", "", "", "Exclude directories if filename is present")
	includeRule    = StringArrayP("include", "", nil, "Include files matching pattern")
	includeFrom    = StringArrayP("include-from", "", nil, "Read include patterns from file")
	filesFrom      = StringArrayP("files-from", "", nil, "Read list of source-file names from file")
	minAge         = StringP("min-age", "", "", "Don't transfer any file younger than this in s or suffix ms|s|m|h|d|w|M|y")
	maxAge         = StringP("max-age", "", "", "Don't transfer any file older than this in s or suffix ms|s|m|h|d|w|M|y")
	minSize        = SizeSuffix(-1)
	maxSize        = SizeSuffix(-1)
	//cvsExclude     = BoolP("cvs-exclude", "C", false, "Exclude files in the same way CVS does")
)

func init() {
	VarP(&minSize, "min-size", "", "Don't transfer any file smaller than this in k or suffix b|k|M|G")
	VarP(&maxSize, "max-size", "", "Don't transfer any file larger than this in k or suffix b|k|M|G")
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

// rules is a slice of rules
type rules struct {
	rules    []rule
	existing map[string]struct{}
}

// add adds a rule if it doesn't exist already
func (rs *rules) add(Include bool, re *regexp.Regexp) {
	if rs.existing == nil {
		rs.existing = make(map[string]struct{})
	}
	newRule := rule{
		Include: Include,
		Regexp:  re,
	}
	newRuleString := newRule.String()
	if _, ok := rs.existing[newRuleString]; ok {
		return // rule already exists
	}
	rs.rules = append(rs.rules, newRule)
	rs.existing[newRuleString] = struct{}{}
}

// clear clears all the rules
func (rs *rules) clear() {
	rs.rules = nil
	rs.existing = nil
}

// len returns the number of rules
func (rs *rules) len() int {
	return len(rs.rules)
}

// FilesMap describes the map of files to transfer
type FilesMap map[string]struct{}

// Filter describes any filtering in operation
type Filter struct {
	DeleteExcluded bool
	MinSize        int64
	MaxSize        int64
	ModTimeFrom    time.Time
	ModTimeTo      time.Time
	fileRules      rules
	dirRules       rules
	ExcludeFile    string
	files          FilesMap // files if filesFrom
	dirs           FilesMap // dirs from filesFrom
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
	addImplicitExclude := false

	if includeRule != nil {
		for _, rule := range *includeRule {
			err = f.Add(true, rule)
			if err != nil {
				return nil, err
			}
			addImplicitExclude = true
		}
	}
	if includeFrom != nil {
		for _, rule := range *includeFrom {
			err := forEachLine(rule, func(line string) error {
				return f.Add(true, line)
			})
			if err != nil {
				return nil, err
			}
			addImplicitExclude = true
		}
	}
	if excludeRule != nil {
		for _, rule := range *excludeRule {
			err = f.Add(false, rule)
			if err != nil {
				return nil, err
			}
		}
	}
	if excludeFrom != nil {
		for _, rule := range *excludeFrom {
			err := forEachLine(rule, func(line string) error {
				return f.Add(false, line)
			})
			if err != nil {
				return nil, err
			}
		}
	}
	if filterRule != nil {
		for _, rule := range *filterRule {
			err = f.AddRule(rule)
			if err != nil {
				return nil, err
			}
		}
	}
	if filterFrom != nil {
		for _, rule := range *filterFrom {
			err := forEachLine(rule, f.AddRule)
			if err != nil {
				return nil, err
			}
		}
	}
	if filesFrom != nil {
		for _, rule := range *filesFrom {
			f.initAddFile() // init to show --files-from set even if no files within
			err := forEachLine(rule, func(line string) error {
				return f.AddFile(line)
			})
			if err != nil {
				return nil, err
			}
		}
	}
	f.ExcludeFile = *excludeFile
	if addImplicitExclude {
		err = f.Add(false, "/**")
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
		Debugf(nil, "--min-age %v to %v", duration, f.ModTimeTo)
	}
	if *maxAge != "" {
		duration, err := ParseDuration(*maxAge)
		if err != nil {
			return nil, err
		}
		f.ModTimeFrom = time.Now().Add(-duration)
		if !f.ModTimeTo.IsZero() && f.ModTimeTo.Before(f.ModTimeFrom) {
			return nil, errors.New("argument --min-age can't be larger than --max-age")
		}
		Debugf(nil, "--max-age %v to %v", duration, f.ModTimeFrom)
	}
	if Config.Dump&DumpFilters != 0 {
		fmt.Println("--- start filters ---")
		fmt.Println(f.DumpFilters())
		fmt.Println("--- end filters ---")
	}
	return f, nil
}

// addDirGlobs adds directory globs from the file glob passed in
func (f *Filter) addDirGlobs(Include bool, glob string) error {
	for _, dirGlob := range globToDirGlobs(glob) {
		// Don't add "/" as we always include the root
		if dirGlob == "/" {
			continue
		}
		dirRe, err := globToRegexp(dirGlob)
		if err != nil {
			return err
		}
		f.dirRules.add(Include, dirRe)
	}
	return nil
}

// Add adds a filter rule with include or exclude status indicated
func (f *Filter) Add(Include bool, glob string) error {
	isDirRule := strings.HasSuffix(glob, "/")
	isFileRule := !isDirRule
	if strings.Contains(glob, "**") {
		isDirRule, isFileRule = true, true
	}
	re, err := globToRegexp(glob)
	if err != nil {
		return err
	}
	if isFileRule {
		f.fileRules.add(Include, re)
		// If include rule work out what directories are needed to scan
		// if exclude rule, we can't rule anything out
		// Unless it is `*` which matches everything
		// NB ** and /** are DirRules
		if Include || glob == "*" {
			err = f.addDirGlobs(Include, glob)
			if err != nil {
				return err
			}
		}
	}
	if isDirRule {
		f.dirRules.add(Include, re)
	}
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
	return errors.Errorf("malformed rule %q", rule)
}

// initAddFile creates f.files and f.dirs
func (f *Filter) initAddFile() {
	if f.files == nil {
		f.files = make(FilesMap)
		f.dirs = make(FilesMap)
	}
}

// AddFile adds a single file to the files from list
func (f *Filter) AddFile(file string) error {
	f.initAddFile()
	file = strings.Trim(file, "/")
	f.files[file] = struct{}{}
	// Put all the parent directories into f.dirs
	for {
		file = path.Dir(file)
		if file == "." {
			break
		}
		if _, found := f.dirs[file]; found {
			break
		}
		f.dirs[file] = struct{}{}
	}
	return nil
}

// Files returns all the files from the `--files-from` list
//
// It may be nil if the list is empty
func (f *Filter) Files() FilesMap {
	return f.files
}

// Clear clears all the filter rules
func (f *Filter) Clear() {
	f.fileRules.clear()
	f.dirRules.clear()
}

// InActive returns false if any filters are active
func (f *Filter) InActive() bool {
	return (f.files == nil &&
		f.ModTimeFrom.IsZero() &&
		f.ModTimeTo.IsZero() &&
		f.MinSize < 0 &&
		f.MaxSize < 0 &&
		f.fileRules.len() == 0 &&
		f.dirRules.len() == 0 &&
		len(f.ExcludeFile) == 0)
}

// includeRemote returns whether this remote passes the filter rules.
func (f *Filter) includeRemote(remote string) bool {
	for _, rule := range f.fileRules.rules {
		if rule.Match(remote) {
			return rule.Include
		}
	}
	return true
}

// ListContainsExcludeFile checks if exclude file is present in the list.
func (f *Filter) ListContainsExcludeFile(entries DirEntries) bool {
	if len(f.ExcludeFile) == 0 {
		return false
	}
	for _, entry := range entries {
		obj, ok := entry.(Object)
		if ok {
			basename := path.Base(obj.Remote())
			if basename == f.ExcludeFile {
				return true
			}
		}
	}
	return false
}

// IncludeDirectory returns a function which checks whether this
// directory should be included in the sync or not.
func (f *Filter) IncludeDirectory(fs Fs) func(string) (bool, error) {
	return func(remote string) (bool, error) {
		remote = strings.Trim(remote, "/")
		// first check if we need to remove directory based on
		// the exclude file
		excl, err := f.DirContainsExcludeFile(fs, remote)
		if err != nil {
			return false, err
		}
		if excl {
			return false, nil
		}

		// filesFrom takes precedence
		if f.files != nil {
			_, include := f.dirs[remote]
			return include, nil
		}
		remote += "/"
		for _, rule := range f.dirRules.rules {
			if rule.Match(remote) {
				return rule.Include, nil
			}
		}

		return true, nil
	}
}

// DirContainsExcludeFile checks if exclude file is present in a
// directroy. If fs is nil, it works properly if ExcludeFile is an
// empty string (for testing).
func (f *Filter) DirContainsExcludeFile(fs Fs, remote string) (bool, error) {
	if len(Config.Filter.ExcludeFile) > 0 {
		exists, err := FileExists(fs, path.Join(remote, Config.Filter.ExcludeFile))
		if err != nil {
			return false, err
		}
		if exists {
			return true, nil
		}
	}
	return false, nil
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
	if f.MinSize >= 0 && size < f.MinSize {
		return false
	}
	if f.MaxSize >= 0 && size > f.MaxSize {
		return false
	}
	return f.includeRemote(remote)
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
	rules = append(rules, "--- File filter rules ---")
	for _, rule := range f.fileRules.rules {
		rules = append(rules, rule.String())
	}
	rules = append(rules, "--- Directory filter rules ---")
	for _, dirRule := range f.dirRules.rules {
		rules = append(rules, dirRule.String())
	}
	return strings.Join(rules, "\n")
}
