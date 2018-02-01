package accounting

import (
	"sort"
	"strings"
)

// stringSet holds a set of strings
type stringSet map[string]struct{}

// Strings returns all the strings in the stringSet
func (ss stringSet) Strings() []string {
	strings := make([]string, 0, len(ss))
	for name := range ss {
		var out string
		if acc := Stats.inProgress.get(name); acc != nil {
			out = acc.String()
		} else {
			out = name
		}
		strings = append(strings, " * "+out)
	}
	sorted := sort.StringSlice(strings)
	sorted.Sort()
	return sorted
}

// String returns all the file names in the stringSet joined by newline
func (ss stringSet) String() string {
	return strings.Join(ss.Strings(), "\n")
}
