package walk

import "github.com/rclone/rclone/fs"

// Listing helpers used by backends

// ListRHelper is used in the implementation of ListR to accumulate DirEntries
type ListRHelper struct {
	callback fs.ListRCallback
	entries  fs.DirEntries
}

// NewListRHelper should be called from ListR with the callback passed in
func NewListRHelper(callback fs.ListRCallback) *ListRHelper {
	return &ListRHelper{
		callback: callback,
	}
}

// send sends the stored entries to the callback if there are >= max
// entries.
func (lh *ListRHelper) send(max int) (err error) {
	if len(lh.entries) >= max {
		err = lh.callback(lh.entries)
		lh.entries = lh.entries[:0]
	}
	return err
}

// Add an entry to the stored entries and send them if there are more
// than a certain amount
func (lh *ListRHelper) Add(entry fs.DirEntry) error {
	if entry == nil {
		return nil
	}
	lh.entries = append(lh.entries, entry)
	return lh.send(100)
}

// Flush the stored entries (if any) sending them to the callback
func (lh *ListRHelper) Flush() error {
	return lh.send(1)
}
