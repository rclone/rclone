package goquery

import "golang.org/x/net/html"

// Filter reduces the set of matched elements to those that match the selector string.
// It returns a new Selection object for this subset of matching elements.
func (s *Selection) Filter(selector string) *Selection {
	return s.FilterMatcher(compileMatcher(selector))
}

// FilterMatcher reduces the set of matched elements to those that match
// the given matcher. It returns a new Selection object for this subset
// of matching elements.
func (s *Selection) FilterMatcher(m Matcher) *Selection {
	return pushStack(s, winnow(s, m, true))
}

// Not removes elements from the Selection that match the selector string.
// It returns a new Selection object with the matching elements removed.
func (s *Selection) Not(selector string) *Selection {
	return s.NotMatcher(compileMatcher(selector))
}

// NotMatcher removes elements from the Selection that match the given matcher.
// It returns a new Selection object with the matching elements removed.
func (s *Selection) NotMatcher(m Matcher) *Selection {
	return pushStack(s, winnow(s, m, false))
}

// FilterFunction reduces the set of matched elements to those that pass the function's test.
// It returns a new Selection object for this subset of elements.
func (s *Selection) FilterFunction(f func(int, *Selection) bool) *Selection {
	return pushStack(s, winnowFunction(s, f, true))
}

// NotFunction removes elements from the Selection that pass the function's test.
// It returns a new Selection object with the matching elements removed.
func (s *Selection) NotFunction(f func(int, *Selection) bool) *Selection {
	return pushStack(s, winnowFunction(s, f, false))
}

// FilterNodes reduces the set of matched elements to those that match the specified nodes.
// It returns a new Selection object for this subset of elements.
func (s *Selection) FilterNodes(nodes ...*html.Node) *Selection {
	return pushStack(s, winnowNodes(s, nodes, true))
}

// NotNodes removes elements from the Selection that match the specified nodes.
// It returns a new Selection object with the matching elements removed.
func (s *Selection) NotNodes(nodes ...*html.Node) *Selection {
	return pushStack(s, winnowNodes(s, nodes, false))
}

// FilterSelection reduces the set of matched elements to those that match a
// node in the specified Selection object.
// It returns a new Selection object for this subset of elements.
func (s *Selection) FilterSelection(sel *Selection) *Selection {
	if sel == nil {
		return pushStack(s, winnowNodes(s, nil, true))
	}
	return pushStack(s, winnowNodes(s, sel.Nodes, true))
}

// NotSelection removes elements from the Selection that match a node in the specified
// Selection object. It returns a new Selection object with the matching elements removed.
func (s *Selection) NotSelection(sel *Selection) *Selection {
	if sel == nil {
		return pushStack(s, winnowNodes(s, nil, false))
	}
	return pushStack(s, winnowNodes(s, sel.Nodes, false))
}

// Intersection is an alias for FilterSelection.
func (s *Selection) Intersection(sel *Selection) *Selection {
	return s.FilterSelection(sel)
}

// Has reduces the set of matched elements to those that have a descendant
// that matches the selector.
// It returns a new Selection object with the matching elements.
func (s *Selection) Has(selector string) *Selection {
	return s.HasSelection(s.document.Find(selector))
}

// HasMatcher reduces the set of matched elements to those that have a descendant
// that matches the matcher.
// It returns a new Selection object with the matching elements.
func (s *Selection) HasMatcher(m Matcher) *Selection {
	return s.HasSelection(s.document.FindMatcher(m))
}

// HasNodes reduces the set of matched elements to those that have a
// descendant that matches one of the nodes.
// It returns a new Selection object with the matching elements.
func (s *Selection) HasNodes(nodes ...*html.Node) *Selection {
	return s.FilterFunction(func(_ int, sel *Selection) bool {
		// Add all nodes that contain one of the specified nodes
		for _, n := range nodes {
			if sel.Contains(n) {
				return true
			}
		}
		return false
	})
}

// HasSelection reduces the set of matched elements to those that have a
// descendant that matches one of the nodes of the specified Selection object.
// It returns a new Selection object with the matching elements.
func (s *Selection) HasSelection(sel *Selection) *Selection {
	if sel == nil {
		return s.HasNodes()
	}
	return s.HasNodes(sel.Nodes...)
}

// End ends the most recent filtering operation in the current chain and
// returns the set of matched elements to its previous state.
func (s *Selection) End() *Selection {
	if s.prevSel != nil {
		return s.prevSel
	}
	return newEmptySelection(s.document)
}

// Filter based on the matcher, and the indicator to keep (Filter) or
// to get rid of (Not) the matching elements.
func winnow(sel *Selection, m Matcher, keep bool) []*html.Node {
	// Optimize if keep is requested
	if keep {
		return m.Filter(sel.Nodes)
	}
	// Use grep
	return grep(sel, func(i int, s *Selection) bool {
		return !m.Match(s.Get(0))
	})
}

// Filter based on an array of nodes, and the indicator to keep (Filter) or
// to get rid of (Not) the matching elements.
func winnowNodes(sel *Selection, nodes []*html.Node, keep bool) []*html.Node {
	if len(nodes)+len(sel.Nodes) < minNodesForSet {
		return grep(sel, func(i int, s *Selection) bool {
			return isInSlice(nodes, s.Get(0)) == keep
		})
	}

	set := make(map[*html.Node]bool)
	for _, n := range nodes {
		set[n] = true
	}
	return grep(sel, func(i int, s *Selection) bool {
		return set[s.Get(0)] == keep
	})
}

// Filter based on a function test, and the indicator to keep (Filter) or
// to get rid of (Not) the matching elements.
func winnowFunction(sel *Selection, f func(int, *Selection) bool, keep bool) []*html.Node {
	return grep(sel, func(i int, s *Selection) bool {
		return f(i, s) == keep
	})
}
