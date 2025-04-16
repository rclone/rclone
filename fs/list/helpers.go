package list

import (
	"context"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
)

// Listing helpers used by backends

// Helper is used in the implementation of ListR to accumulate DirEntries
type Helper struct {
	callback fs.ListRCallback
	entries  fs.DirEntries
}

// NewHelper should be called from ListR with the callback passed in
func NewHelper(callback fs.ListRCallback) *Helper {
	return &Helper{
		callback: callback,
	}
}

// send sends the stored entries to the callback if there are >= max
// entries.
func (lh *Helper) send(max int) (err error) {
	if len(lh.entries) >= max {
		err = lh.callback(lh.entries)
		lh.entries = lh.entries[:0]
	}
	return err
}

// Add an entry to the stored entries and send them if there are more
// than a certain amount
func (lh *Helper) Add(entry fs.DirEntry) error {
	if entry == nil {
		return nil
	}
	lh.entries = append(lh.entries, entry)
	return lh.send(100)
}

// Flush the stored entries (if any) sending them to the callback
func (lh *Helper) Flush() error {
	return lh.send(1)
}

// WithListP implements the List interface with ListP
//
// It should be used in backends which support ListP to implement
// List.
func WithListP(ctx context.Context, dir string, list fs.ListPer) (entries fs.DirEntries, err error) {
	err = list.ListP(ctx, dir, func(newEntries fs.DirEntries) error {
		accounting.Stats(ctx).Listed(int64(len(newEntries)))
		entries = append(entries, newEntries...)
		return nil
	})
	return entries, err
}
