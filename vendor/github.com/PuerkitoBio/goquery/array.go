package goquery

import (
	"golang.org/x/net/html"
)

const (
	maxUint = ^uint(0)
	maxInt  = int(maxUint >> 1)

	// ToEnd is a special index value that can be used as end index in a call
	// to Slice so that all elements are selected until the end of the Selection.
	// It is equivalent to passing (*Selection).Length().
	ToEnd = maxInt
)

// First reduces the set of matched elements to the first in the set.
// It returns a new Selection object, and an empty Selection object if the
// the selection is empty.
func (s *Selection) First() *Selection {
	return s.Eq(0)
}

// Last reduces the set of matched elements to the last in the set.
// It returns a new Selection object, and an empty Selection object if
// the selection is empty.
func (s *Selection) Last() *Selection {
	return s.Eq(-1)
}

// Eq reduces the set of matched elements to the one at the specified index.
// If a negative index is given, it counts backwards starting at the end of the
// set. It returns a new Selection object, and an empty Selection object if the
// index is invalid.
func (s *Selection) Eq(index int) *Selection {
	if index < 0 {
		index += len(s.Nodes)
	}

	if index >= len(s.Nodes) || index < 0 {
		return newEmptySelection(s.document)
	}

	return s.Slice(index, index+1)
}

// Slice reduces the set of matched elements to a subset specified by a range
// of indices. The start index is 0-based and indicates the index of the first
// element to select. The end index is 0-based and indicates the index at which
// the elements stop being selected (the end index is not selected).
//
// The indices may be negative, in which case they represent an offset from the
// end of the selection.
//
// The special value ToEnd may be specified as end index, in which case all elements
// until the end are selected. This works both for a positive and negative start
// index.
func (s *Selection) Slice(start, end int) *Selection {
	if start < 0 {
		start += len(s.Nodes)
	}
	if end == ToEnd {
		end = len(s.Nodes)
	} else if end < 0 {
		end += len(s.Nodes)
	}
	return pushStack(s, s.Nodes[start:end])
}

// Get retrieves the underlying node at the specified index.
// Get without parameter is not implemented, since the node array is available
// on the Selection object.
func (s *Selection) Get(index int) *html.Node {
	if index < 0 {
		index += len(s.Nodes) // Negative index gets from the end
	}
	return s.Nodes[index]
}

// Index returns the position of the first element within the Selection object
// relative to its sibling elements.
func (s *Selection) Index() int {
	if len(s.Nodes) > 0 {
		return newSingleSelection(s.Nodes[0], s.document).PrevAll().Length()
	}
	return -1
}

// IndexSelector returns the position of the first element within the
// Selection object relative to the elements matched by the selector, or -1 if
// not found.
func (s *Selection) IndexSelector(selector string) int {
	if len(s.Nodes) > 0 {
		sel := s.document.Find(selector)
		return indexInSlice(sel.Nodes, s.Nodes[0])
	}
	return -1
}

// IndexMatcher returns the position of the first element within the
// Selection object relative to the elements matched by the matcher, or -1 if
// not found.
func (s *Selection) IndexMatcher(m Matcher) int {
	if len(s.Nodes) > 0 {
		sel := s.document.FindMatcher(m)
		return indexInSlice(sel.Nodes, s.Nodes[0])
	}
	return -1
}

// IndexOfNode returns the position of the specified node within the Selection
// object, or -1 if not found.
func (s *Selection) IndexOfNode(node *html.Node) int {
	return indexInSlice(s.Nodes, node)
}

// IndexOfSelection returns the position of the first node in the specified
// Selection object within this Selection object, or -1 if not found.
func (s *Selection) IndexOfSelection(sel *Selection) int {
	if sel != nil && len(sel.Nodes) > 0 {
		return indexInSlice(s.Nodes, sel.Nodes[0])
	}
	return -1
}
