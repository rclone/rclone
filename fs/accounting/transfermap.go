package accounting

import (
	"context"
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
func (tm *transferMap) del(remote string) bool {
	tm.mu.Lock()
	_, exists := tm.items[remote]
	delete(tm.items, remote)
	tm.mu.Unlock()

	return exists
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
	// sort by time first and if equal by name.  Note that the relatively
	// low time resolution on Windows can cause equal times.
	sort.Slice(s, func(i, j int) bool {
		a, b := s[i], s[j]
		if a.startedAt.Before(b.startedAt) {
			return true
		} else if !a.startedAt.Equal(b.startedAt) {
			return false
		}
		return a.remote < b.remote
	})
	return s
}

// String returns string representation of map items excluding any in
// exclude (if set).
func (tm *transferMap) String(ctx context.Context, progress *inProgress, exclude *transferMap) string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	ci := fs.GetConfig(ctx)
	stringList := make([]string, 0, len(tm.items))
	for _, tr := range tm._sortedSlice() {
		var what = tr.what
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
			if what != "" {
				out += ", " + what
			}
		} else {
			if what == "" {
				what = tm.name
			}
			out = fmt.Sprintf("%*s: %s",
				ci.StatsFileNameLength,
				shortenName(tr.remote, ci.StatsFileNameLength),
				what,
			)
		}
		stringList = append(stringList, " * "+out)
	}
	return strings.Join(stringList, "\n")
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
		out := tr.rcStats() // basic stats
		if acc := progress.get(tr.remote); acc != nil {
			acc.rcStats(out) // add extended stats if have acc
		}
		t = append(t, out)
	}
	return t
}
