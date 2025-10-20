// VFS Item integration for the VFS layer
//
// This file contains the implementation of the VFS cache item
// which is used to track the state of files in the cache.

package vfscache

import (
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/vfs/vfscache/downloaders"
	"github.com/rclone/rclone/vfs/vfscache/writeback"
	"github.com/rclone/rclone/vfs/vfscommon"
)

// Item is a wrapper around a cache.Item with VFS specific functionality
type Item struct {
	mu           sync.Mutex // protects all below
	c            *Cache     // cache this is part of
	info         *Info      // information about the file
	downloaders  *downloaders.Downloaders
	writeBackID  writeback.Handle // if 0 not writing back
	pendingWrite writeback.Handle // if 0 no pending write
	name         string           // from c.name
	o            fs.Object        // object currently in the file
	used         time.Time        // time this was last used
	err          error            // last error on this item
}

// Info represents the information about a cached file
type Info struct {
	Name     string                 // name of the file
	Size     int64                  // size of the file
	ModTime  time.Time              // modification time of the file
	Rs       *downloaders.RangeSpec // range specification
	Dirty    bool                   // if set then the file has been modified
	Pinned   bool                   // if set then the file is pinned in the cache
	Metadata vfscommon.Metadata     // metadata for the file
}

// NewItem creates a new Item
func NewItem(c *Cache, name string, info *Info) *Item {
	item := &Item{
		c:    c,
		name: name,
		info: info,
	}
	// fs.Debugf(name, "NewItem(%q)", name)
	return item
}

// present returns true if the file is present in the cache
func (item *Item) present() bool {
	item.mu.Lock()
	defer item.mu.Unlock()
	return item._present()
}

// _present returns true if the file is present in the cache
// Call with lock held
func (item *Item) _present() bool {
	if item.info == nil {
		return false
	}
	// If we have a range spec then we only have part of the file
	if item.info.Rs == nil {
		return true
	}
	// If we have a range spec then we only have part of the file
	// so we need to check if we have the whole file
	return item.info.Rs.Present()
}

// VFSStatusCache returns the cache status of the file, which can be "FULL", "PARTIAL", "NONE", "DIRTY", "UPLOADING", or "ERROR".
func (item *Item) VFSStatusCache() string {
	status, _ := item.VFSStatusCacheWithPercentage()
	return status
}

// _vfsStatusCacheWithPercentage is the implementation of VFSStatusCacheWithPercentage but without the lock
//
// Must be called with the lock held
func (item *Item) _vfsStatusCacheWithPercentage() (string, int) {
	// Check if item.info is nil to prevent panic
	if item.info == nil {
		return "NONE", 0
	}

	// Check if item is being uploaded
	if item.writeBackID != 0 {
		if item.c.writeback != nil {
			// Check upload status
			isUploading := item.c.writeback.IsUploading(item.writeBackID)
			if isUploading {
				return "UPLOADING", 100
			}
		}
	}

	// Check if item is dirty (modified but not uploaded yet)
	if item.info.Dirty {
		return "DIRTY", 100
	}

	// Check cache status
	if item._present() {
		return "FULL", 100
	}

	var cachedSize int64
	if item.info.Rs != nil {
		cachedSize = item.info.Rs.Size()
	}
	totalSize := item.info.Size

	if totalSize <= 0 {
		if cachedSize > 0 {
			return "PARTIAL", 100
		}
		return "NONE", 0
	}

	if cachedSize >= totalSize {
		return "FULL", 100
	}

	if cachedSize > 0 {
		percentage := int((cachedSize * 100) / totalSize)
		return "PARTIAL", percentage
	}

	return "NONE", 0
}

// VFSStatusCacheWithPercentage returns the cache status of the file along with percentage cached.
// Returns status string and percentage (0-100).
func (item *Item) VFSStatusCacheWithPercentage() (string, int) {
	item.mu.Lock()
	defer item.mu.Unlock()
	return item._vfsStatusCacheWithPercentage()
}

// VFSStatusCacheDetailed returns detailed cache status information for the file.
// Returns status string, percentage (0-100), total size, cached size, and dirty flag.
func (item *Item) VFSStatusCacheDetailed() (string, int, int64, int64, bool) {
	item.mu.Lock()
	defer item.mu.Unlock()

	// Get basic status and percentage
	status, percentage := item._vfsStatusCacheWithPercentage()

	// Get size information
	totalSize := item.info.Size
	var cachedSize int64
	if status == "FULL" || status == "DIRTY" || status == "UPLOADING" {
		cachedSize = totalSize
	} else if item.info.Rs != nil {
		cachedSize = item.info.Rs.Size()
	}

	// Get dirty flag
	dirty := item.info.Dirty

	return status, percentage, totalSize, cachedSize, dirty
}

// GetAggregateStats returns aggregate cache statistics for all items in the cache
func (c *Cache) GetAggregateStats() AggregateStats {
	c.mu.Lock()
	defer c.mu.Unlock()

	stats := AggregateStats{
		TotalFiles: len(c.item),
	}

	if stats.TotalFiles == 0 {
		return stats
	}

	var totalPercentage int

	for _, item := range c.item {
		status, percentage := item.VFSStatusCacheWithPercentage()

		switch status {
		case "FULL":
			stats.FullCount++
		case "PARTIAL":
			stats.PartialCount++
		case "NONE":
			stats.NoneCount++
		case "DIRTY":
			stats.DirtyCount++
		case "UPLOADING":
			stats.UploadingCount++
		}

		stats.TotalCachedBytes += item.getDiskSize()
		totalPercentage += percentage
	}

	// Calculate average percentage
	if stats.TotalFiles > 0 {
		stats.AverageCachePercentage = totalPercentage / stats.TotalFiles
	}

	return stats
}

// AggregateStats holds aggregate cache statistics
type AggregateStats struct {
	TotalFiles             int   `json:"totalFiles"`
	FullCount              int   `json:"fullCount"`
	PartialCount           int   `json:"partialCount"`
	NoneCount              int   `json:"noneCount"`
	DirtyCount             int   `json:"dirtyCount"`
	UploadingCount         int   `json:"uploadingCount"`
	TotalCachedBytes       int64 `json:"totalCachedBytes"`
	AverageCachePercentage int   `json:"averageCachePercentage"`
}

// getDiskSize returns the size of the file on disk
func (item *Item) getDiskSize() int64 {
	item.mu.Lock()
	defer item.mu.Unlock()
	if item.info == nil {
		return 0
	}
	// Return cached size if available
	if item.info.Rs != nil {
		return item.info.Rs.Size()
	}
	// Return total size if file is fully cached
	return item.info.Size
}
