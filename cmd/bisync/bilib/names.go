package bilib

import (
	"bytes"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Names comprises a set of file names
type Names map[string]interface{}

// ToNames converts string slice to a set of names
func ToNames(list []string) Names {
	ns := Names{}
	for _, f := range list {
		ns.Add(f)
	}
	return ns
}

// Add adds new file name to the set
func (ns Names) Add(name string) {
	ns[name] = nil
}

// Has checks whether given name is present in the set
func (ns Names) Has(name string) bool {
	_, ok := ns[name]
	return ok
}

// NotEmpty checks whether set is not empty
func (ns Names) NotEmpty() bool {
	return len(ns) > 0
}

// ToList converts name set to string slice
func (ns Names) ToList() []string {
	list := []string{}
	for file := range ns {
		list = append(list, file)
	}
	sort.Strings(list)
	return list
}

// Save saves name set in a text file
func (ns Names) Save(path string) error {
	return SaveList(ns.ToList(), path)
}

// SaveList saves file name list in a text file
func SaveList(list []string, path string) error {
	buf := &bytes.Buffer{}
	for _, s := range list {
		_, _ = buf.WriteString(strconv.Quote(s))
		_ = buf.WriteByte('\n')
	}
	return os.WriteFile(path, buf.Bytes(), PermSecure)
}

// AliasMap comprises a pair of names that are not equal but treated as equal for comparison purposes
// For example, when normalizing unicode and casing
// This helps reduce repeated normalization functions, which really slow things down
type AliasMap map[string]string

// Add adds new pair to the set, in both directions
func (am AliasMap) Add(name1, name2 string) {
	if name1 != name2 {
		am[name1] = name2
		am[name2] = name1
	}
}

// Alias returns the alternate version, if any, else the original.
func (am AliasMap) Alias(name1 string) string {
	// note: we don't need to check normalization settings, because we already did it in March.
	// the AliasMap will only exist if March paired up two unequal filenames.
	name2, ok := am[name1]
	if ok {
		return name2
	}
	return name1
}

// ParseGlobs determines whether a string contains {brackets}
// and returns the substring (including both brackets) for replacing
// substring is first opening bracket to last closing bracket --
// good for {{this}} but not {this}{this}
func ParseGlobs(s string) (hasGlobs bool, substring string) {
	open := strings.Index(s, "{")
	close := strings.LastIndex(s, "}")
	if open >= 0 && close > open {
		return true, s[open : close+1]
	}
	return false, ""
}

// TrimBrackets converts {{this}} to this
func TrimBrackets(s string) string {
	return strings.Trim(s, "{}")
}

// TimeFormat converts a user-supplied string to a Go time constant, if possible
func TimeFormat(timeFormat string) string {
	switch timeFormat {
	case "Layout":
		timeFormat = time.Layout
	case "ANSIC":
		timeFormat = time.ANSIC
	case "UnixDate":
		timeFormat = time.UnixDate
	case "RubyDate":
		timeFormat = time.RubyDate
	case "RFC822":
		timeFormat = time.RFC822
	case "RFC822Z":
		timeFormat = time.RFC822Z
	case "RFC850":
		timeFormat = time.RFC850
	case "RFC1123":
		timeFormat = time.RFC1123
	case "RFC1123Z":
		timeFormat = time.RFC1123Z
	case "RFC3339":
		timeFormat = time.RFC3339
	case "RFC3339Nano":
		timeFormat = time.RFC3339Nano
	case "Kitchen":
		timeFormat = time.Kitchen
	case "Stamp":
		timeFormat = time.Stamp
	case "StampMilli":
		timeFormat = time.StampMilli
	case "StampMicro":
		timeFormat = time.StampMicro
	case "StampNano":
		timeFormat = time.StampNano
	case "DateTime":
		// timeFormat = time.DateTime // missing in go1.19
		timeFormat = "2006-01-02 15:04:05"
	case "DateOnly":
		// timeFormat = time.DateOnly // missing in go1.19
		timeFormat = "2006-01-02"
	case "TimeOnly":
		// timeFormat = time.TimeOnly // missing in go1.19
		timeFormat = "15:04:05"
	case "MacFriendlyTime", "macfriendlytime", "mac":
		timeFormat = "2006-01-02 0304PM" // not actually a Go constant -- but useful as macOS filenames can't have colons
	}
	return timeFormat
}

// AppyTimeGlobs converts "myfile-{DateOnly}.txt" to "myfile-2006-01-02.txt"
func AppyTimeGlobs(s string, t time.Time) string {
	hasGlobs, substring := ParseGlobs(s)
	if !hasGlobs {
		return s
	}
	timeString := t.Local().Format(TimeFormat(TrimBrackets(substring)))
	return strings.ReplaceAll(s, substring, timeString)
}
