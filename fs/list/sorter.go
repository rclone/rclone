package list

import (
	"cmp"
	"context"
	"slices"
	"sync"

	"github.com/rclone/rclone/fs"
)

// Sorter implements an efficient mechanism for sorting list entries.
//
// If there are a large number of entries, this may be done on disk
// instead of in memory.
//
// Supply entries with the Add method, call Send at the end to deliver
// the sorted entries and finalise with CleanUp regardless of whether
// you called Add or Send.
//
// Sorted entries are delivered to the callback supplied to NewSorter
// when the Send method is called.
type Sorter struct {
	ctx      context.Context
	mu       sync.Mutex
	callback fs.ListRCallback
	entries  fs.DirEntries
	keyFn    KeyFn
}

// KeyFn turns an entry into a sort key
type KeyFn func(entry fs.DirEntry) string

// identityKeyFn maps an entry to its Remote
func identityKeyFn(entry fs.DirEntry) string {
	return entry.Remote()
}

// NewSorter creates a new Sorter with callback for sorted entries to
// be delivered to. keyFn is used to process each entry to get a key
// function, if nil then it will just use entry.Remote()
func NewSorter(ctx context.Context, callback fs.ListRCallback, keyFn KeyFn) (*Sorter, error) {
	if keyFn == nil {
		keyFn = identityKeyFn
	}
	return &Sorter{
		ctx:      ctx,
		callback: callback,
		keyFn:    keyFn,
	}, nil
}

// Add entries to the list sorter.
//
// Does not call the callback.
//
// Safe to call from concurrent go routines
func (ls *Sorter) Add(entries fs.DirEntries) error {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	ls.entries = append(ls.entries, entries...)
	return nil
}

// Send the sorted entries to the callback.
func (ls *Sorter) Send() error {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	// Sort the directory entries by Remote
	//
	// We use a stable sort here just in case there are
	// duplicates. Assuming the remote delivers the entries in a
	// consistent order, this will give the best user experience
	// in syncing as it will use the first entry for the sync
	// comparison.
	slices.SortStableFunc(ls.entries, func(a, b fs.DirEntry) int {
		return cmp.Compare(ls.keyFn(a), ls.keyFn(b))
	})
	return ls.callback(ls.entries)
}

// CleanUp the Sorter, cleaning up any memory / files.
//
// It is safe and encouraged to call this regardless of whether you
// called Send or not.
//
// This does not call the callback
func (ls *Sorter) CleanUp() {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	ls.entries = nil
}

// SortToChan makes a callback for the Sorter which sends the output
// to the channel provided.
func SortToChan(out chan<- fs.DirEntry) fs.ListRCallback {
	return func(entries fs.DirEntries) error {
		for _, entry := range entries {
			out <- entry
		}
		return nil
	}
}
