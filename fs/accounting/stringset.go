package accounting

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/ncw/rclone/fs"
)

// stringSet holds a set of strings
type stringSet struct {
	mu    sync.RWMutex
	items map[string]struct{}
	name  string
}

// newStringSet creates a new empty string set of capacity size
func newStringSet(size int, name string) *stringSet {
	return &stringSet{
		items: make(map[string]struct{}, size),
		name:  name,
	}
}

// add adds remote to the set
func (ss *stringSet) add(remote string) {
	ss.mu.Lock()
	ss.items[remote] = struct{}{}
	ss.mu.Unlock()
}

// del removes remote from the set
func (ss *stringSet) del(remote string) {
	ss.mu.Lock()
	delete(ss.items, remote)
	ss.mu.Unlock()
}

// empty returns whether the set has any items
func (ss *stringSet) empty() bool {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	return len(ss.items) == 0
}

// count returns the number of items in the set
func (ss *stringSet) count() int {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	return len(ss.items)
}

// Strings returns all the strings in the stringSet
func (ss *stringSet) Strings() []string {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	strings := make([]string, 0, len(ss.items))
	for name := range ss.items {
		var out string
		if acc := Stats.inProgress.get(name); acc != nil {
			out = acc.String()
		} else {
			out = fmt.Sprintf("%*s: %s",
				fs.Config.StatsFileNameLength,
				shortenName(name, fs.Config.StatsFileNameLength),
				ss.name,
			)
		}
		strings = append(strings, " * "+out)
	}
	sorted := sort.StringSlice(strings)
	sorted.Sort()
	return sorted
}

// String returns all the file names in the stringSet joined by newline
func (ss *stringSet) String() string {
	return strings.Join(ss.Strings(), "\n")
}

// progress returns total bytes read as well as the size.
func (ss *stringSet) progress() (totalBytes, totalSize int64) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	for name := range ss.items {
		if acc := Stats.inProgress.get(name); acc != nil {
			bytes, size := acc.progress()
			if size >= 0 && bytes >= 0 {
				totalBytes += bytes
				totalSize += size
			}
		}
	}
	return totalBytes, totalSize
}
