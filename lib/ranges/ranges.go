// Package ranges provides the Ranges type for keeping track of byte
// ranges which may or may not be present in an object.
package ranges

import (
	"sort"
)

// Range describes a single byte range
type Range struct {
	Pos  int64
	Size int64
}

// End returns the end of the Range
func (r Range) End() int64 {
	return r.Pos + r.Size
}

// IsEmpty true if the range has no size
func (r Range) IsEmpty() bool {
	return r.Size <= 0
}

// Clip ensures r.End() <= offset by modifying r.Size if necessary
//
// if r.Pos > offset then a Range{Pos:0, Size:0} will be returned.
func (r *Range) Clip(offset int64) {
	if r.End() <= offset {
		return
	}
	r.Size -= r.End() - offset
	if r.Size < 0 {
		r.Pos = 0
		r.Size = 0
	}
}

// Intersection returns the common Range for two Range~s
//
// If there is no intersection then the Range returned will have
// IsEmpty() true
func (r Range) Intersection(b Range) (intersection Range) {
	if (r.Pos >= b.Pos && r.Pos < b.End()) || (b.Pos >= r.Pos && b.Pos < r.End()) {
		intersection.Pos = max(r.Pos, b.Pos)
		intersection.Size = min(r.End(), b.End()) - intersection.Pos
	}
	return
}

// Ranges describes a number of Range segments. These should only be
// added with the Ranges.Insert function. The Ranges are kept sorted
// and coalesced to the minimum size.
type Ranges []Range

// merge the Range new into dest if possible
//
// dst.Pos must be >= src.Pos
//
// return true if merged
func merge(new, dst *Range) bool {
	if new.End() < dst.Pos {
		return false
	}
	if new.End() > dst.End() {
		dst.Size = new.Size
	} else {
		dst.Size += dst.Pos - new.Pos
	}
	dst.Pos = new.Pos
	return true
}

// coalesce ranges assuming an element has been inserted at i
func (rs *Ranges) coalesce(i int) {
	ranges := *rs
	var j int
	startChop := i
	endChop := i
	// look at previous element too
	if i > 0 && merge(&ranges[i-1], &ranges[i]) {
		startChop = i - 1
	}
	for j = i; j < len(ranges)-1; j++ {
		if !merge(&ranges[j], &ranges[j+1]) {
			break
		}
		endChop = j + 1
	}
	if endChop > startChop {
		// chop the unneeded ranges out
		copy(ranges[startChop:], ranges[endChop:])
		*rs = ranges[:len(ranges)-endChop+startChop]
	}
}

// search finds the first Range in rs that has Pos >= r.Pos
//
// The return takes on values 0..len(rs) so may point beyond the end
// of the slice.
func (rs Ranges) search(r Range) int {
	return sort.Search(len(rs), func(i int) bool {
		return rs[i].Pos >= r.Pos
	})
}

// Insert the new Range into a sorted and coalesced slice of
// Ranges. The result will be sorted and coalesced.
func (rs *Ranges) Insert(r Range) {
	if r.IsEmpty() {
		return
	}
	ranges := *rs
	if len(ranges) == 0 {
		ranges = append(ranges, r)
		*rs = ranges
		return
	}
	i := ranges.search(r)
	if i == len(ranges) || !merge(&r, &ranges[i]) {
		// insert into the range
		ranges = append(ranges, Range{})
		copy(ranges[i+1:], ranges[i:])
		ranges[i] = r
		*rs = ranges
	}
	rs.coalesce(i)
}

// Find searches for r in rs and returns the next present or absent
// Range. It returns:
//
// curr which is the Range found
// next is the Range which should be presented to Find next
// present shows whether curr is present or absent
//
// if !next.IsEmpty() then Find should be called again with r = next
// to retrieve the next Range.
//
// Note that r.Pos == curr.Pos always
func (rs Ranges) Find(r Range) (curr, next Range, present bool) {
	if r.IsEmpty() {
		return r, next, false
	}
	var intersection Range
	i := rs.search(r)
	if i > 0 {
		prev := rs[i-1]
		// we know prev.Pos < r.Pos so intersection.Pos == r.Pos
		intersection = prev.Intersection(r)
		if !intersection.IsEmpty() {
			r.Pos = intersection.End()
			r.Size -= intersection.Size
			return intersection, r, true
		}
	}
	if i >= len(rs) {
		return r, Range{}, false
	}
	found := rs[i]
	intersection = found.Intersection(r)
	if intersection.IsEmpty() {
		return r, Range{}, false
	}
	if r.Pos < intersection.Pos {
		curr = Range{
			Pos:  r.Pos,
			Size: intersection.Pos - r.Pos,
		}
		r.Pos = curr.End()
		r.Size -= curr.Size
		return curr, r, false
	}
	r.Pos = intersection.End()
	r.Size -= intersection.Size
	return intersection, r, true
}

// FoundRange is returned from FindAll
//
// It contains a Range and a boolean as to whether the range was
// Present or not.
type FoundRange struct {
	R       Range
	Present bool
}

// FindAll repeatedly calls Find searching for r in rs and returning
// present or absent ranges.
//
// It returns a slice of FoundRange. Each element has a range and an
// indication of whether it was present or not.
func (rs Ranges) FindAll(r Range) (frs []FoundRange) {
	for !r.IsEmpty() {
		var fr FoundRange
		fr.R, r, fr.Present = rs.Find(r)
		frs = append(frs, fr)
	}
	return frs
}

// Present returns whether r can be satisfied by rs
func (rs Ranges) Present(r Range) (present bool) {
	if r.IsEmpty() {
		return true
	}
	_, next, present := rs.Find(r)
	if !present {
		return false
	}
	if next.IsEmpty() {
		return true
	}
	return false
}

// Intersection works out which ranges out of rs are entirely
// contained within r and returns a new Ranges
func (rs Ranges) Intersection(r Range) (newRs Ranges) {
	if len(rs) == 0 {
		return rs
	}
	for !r.IsEmpty() {
		var curr Range
		var found bool
		curr, r, found = rs.Find(r)
		if found {
			newRs.Insert(curr)
		}
	}
	return newRs
}

// Equal returns true if rs == bs
func (rs Ranges) Equal(bs Ranges) bool {
	if len(rs) != len(bs) {
		return false
	}
	if rs == nil || bs == nil {
		return true
	}
	for i := range rs {
		if rs[i] != bs[i] {
			return false
		}
	}
	return true
}

// Size returns the total size of all the segments
func (rs Ranges) Size() (size int64) {
	for _, r := range rs {
		size += r.Size
	}
	return size
}

// FindMissing finds the initial part of r that is not in rs
//
// If r is entirely present in rs then r an empty block will be returned.
//
// If r is not present in rs then the block returned will have IsEmpty
// return true.
//
// If r is partially present in rs then a new block will be returned
// which starts with the first part of rs that isn't present in r. The
// End() for this block will be the same as originally passed in.
//
// For all returns rout.End() == r.End()
func (rs Ranges) FindMissing(r Range) (rout Range) {
	rout = r
	if r.IsEmpty() {
		return rout
	}
	curr, _, present := rs.Find(r)
	if !present {
		// Initial block is not present
		return rout
	}
	rout.Size -= curr.End() - rout.Pos
	rout.Pos = curr.End()
	return rout
}
