// Package downloaders provides utilities for the VFS layer
package downloaders

import (
	"github.com/rclone/rclone/lib/ranges"
)

// RangeSpec represents a specification of ranges that are cached
type RangeSpec struct {
	rs ranges.Ranges
}

// NewRangeSpec creates a new RangeSpec
func NewRangeSpec() *RangeSpec {
	return &RangeSpec{
		rs: make(ranges.Ranges, 0),
	}
}

// Size returns the total size of all cached ranges
func (rs *RangeSpec) Size() int64 {
	return rs.rs.Size()
}

// Present returns true if the entire file [0, size) is present in the cache
func (rs *RangeSpec) Present(size int64) bool {
	if size <= 0 {
		return false
	}
	// Entire file is present iff there are no missing ranges in [0,size)
	missing := rs.rs.FindMissing(ranges.Range{Pos: 0, Size: size})
	return missing.IsEmpty()
}

// Insert adds a range to the RangeSpec
func (rs *RangeSpec) Insert(r ranges.Range) {
	rs.rs.Insert(r)
}

// FindMissing finds the ranges that are missing from the cache
func (rs *RangeSpec) FindMissing(r ranges.Range) ranges.Range {
	return rs.rs.FindMissing(r)
}

// HasRange returns true if the current ranges entirely include range
func (rs *RangeSpec) HasRange(r ranges.Range) bool {
	// Check if the range is fully covered by existing ranges
	_, next, present := rs.rs.Find(r)
	return present && next.IsEmpty()
}
