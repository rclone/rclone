package accounting

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/rc"
)

// transferMap holds name to transfer map
type transferMap struct {
	mu    sync.RWMutex
	items map[string]*Transfer
	name  string
}

// newTransferMap creates a new empty transfer map of capacity size
func newTransferMap(size int, name string) *transferMap {
	return &transferMap{
		items: make(map[string]*Transfer, size),
		name:  name,
	}
}

// add adds a new transfer to the map
func (tm *transferMap) add(tr *Transfer) {
	tm.mu.Lock()
	tm.items[tr.remote] = tr
	tm.mu.Unlock()
}

// del removes a transfer from the map by name
func (tm *transferMap) del(remote string) {
	tm.mu.Lock()
	delete(tm.items, remote)
	tm.mu.Unlock()
}

// merge adds items from another map
func (tm *transferMap) merge(m *transferMap) {
	tm.mu.Lock()
	m.mu.Lock()
	for name, tr := range m.items {
		tm.items[name] = tr
	}
	m.mu.Unlock()
	tm.mu.Unlock()
}

// empty returns whether the map has any items
func (tm *transferMap) empty() bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return len(tm.items) == 0
}

// count returns the number of items in the map
func (tm *transferMap) count() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return len(tm.items)
}

// _sortedSlice returns all transfers sorted by start time
//
// Call with mu.Rlock held
func (tm *transferMap) _sortedSlice() []*Transfer {
	s := make([]*Transfer, 0, len(tm.items))
	for _, tr := range tm.items {
		s = append(s, tr)
	}
	sort.Slice(s, func(i, j int) bool {
		return s[i].startedAt.Before(s[j].startedAt)
	})
	return s
}

// String returns string representation of map items excluding any in
// exclude (if set).
func (tm *transferMap) String(progress *inProgress, exclude *transferMap) string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	strngs := make([]string, 0, len(tm.items))
	for _, tr := range tm._sortedSlice() {
		if exclude != nil {
			exclude.mu.RLock()
			_, found := exclude.items[tr.remote]
			exclude.mu.RUnlock()
			if found {
				continue
			}
		}
		var out string
		if acc := progress.get(tr.remote); acc != nil {
			out = acc.String()
		} else {
			out = fmt.Sprintf("%*s: %s",
				fs.Config.StatsFileNameLength,
				shortenName(tr.remote, fs.Config.StatsFileNameLength),
				tm.name,
			)
		}
		strngs = append(strngs, " * "+out)
	}
	return strings.Join(strngs, "\n")
}

// progress returns total bytes read as well as the size.
func (tm *transferMap) progress(stats *StatsInfo) (totalBytes, totalSize int64) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	for name := range tm.items {
		if acc := stats.inProgress.get(name); acc != nil {
			bytes, size := acc.progress()
			if size >= 0 && bytes >= 0 {
				totalBytes += bytes
				totalSize += size
			}
		}
	}
	return totalBytes, totalSize
}

// remotes returns a []string of the remote names for the transferMap
func (tm *transferMap) remotes() (c []string) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	for _, tr := range tm._sortedSlice() {
		c = append(c, tr.remote)
	}
	return c
}

// rcStats returns a []rc.Params of the stats for the transferMap
func (tm *transferMap) rcStats(progress *inProgress) (t []rc.Params) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	for _, tr := range tm._sortedSlice() {
		if acc := progress.get(tr.remote); acc != nil {
			t = append(t, acc.rcStats())
		} else {
			t = append(t, tr.rcStats())
		}
	}
	return t
}
