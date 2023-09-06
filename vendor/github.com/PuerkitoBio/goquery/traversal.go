package goquery

import "golang.org/x/net/html"

type siblingType int

// Sibling type, used internally when iterating over children at the same
// level (siblings) to specify which nodes are requested.
const (
	siblingPrevUntil siblingType = iota - 3
	siblingPrevAll
	siblingPrev
	siblingAll
	siblingNext
	siblingNextAll
	siblingNextUntil
	siblingAllIncludingNonElements
)

// Find gets the descendants of each element in the current set of matched
// elements, filtered by a selector. It returns a new Selection object
// containing these matched elements.
func (s *Selection) Find(selector string) *Selection {
	return pushStack(s, findWithMatcher(s.Nodes, compileMatcher(selector)))
}

// FindMatcher gets the descendants of each element in the current set of matched
// elements, filtered by the matcher. It returns a new Selection object
// containing these matched elements.
func (s *Selection) FindMatcher(m Matcher) *Selection {
	return pushStack(s, findWithMatcher(s.Nodes, m))
}

// FindSelection gets the descendants of each element in the current
// Selection, filtered by a Selection. It returns a new Selection object
// containing these matched elements.
func (s *Selection) FindSelection(sel *Selection) *Selection {
	if sel == nil {
		return pushStack(s, nil)
	}
	return s.FindNodes(sel.Nodes...)
}

// FindNodes gets the descendants of each element in the current
// Selection, filtered by some nodes. It returns a new Selection object
// containing these matched elements.
func (s *Selection) FindNodes(nodes ...*html.Node) *Selection {
	return pushStack(s, mapNodes(nodes, func(i int, n *html.Node) []*html.Node {
		if sliceContains(s.Nodes, n) {
			return []*html.Node{n}
		}
		return nil
	}))
}

// Contents gets the children of each element in the Selection,
// including text and comment nodes. It returns a new Selection object
// containing these elements.
func (s *Selection) Contents() *Selection {
	return pushStack(s, getChildrenNodes(s.Nodes, siblingAllIncludingNonElements))
}

// ContentsFiltered gets the children of each element in the Selection,
// filtered by the specified selector. It returns a new Selection
// object containing these elements. Since selectors only act on Element nodes,
// this function is an alias to ChildrenFiltered unless the selector is empty,
// in which case it is an alias to Contents.
func (s *Selection) ContentsFiltered(selector string) *Selection {
	if selector != "" {
		return s.ChildrenFiltered(selector)
	}
	return s.Contents()
}

// ContentsMatcher gets the children of each element in the Selection,
// filtered by the specified matcher. It returns a new Selection
// object containing these elements. Since matchers only act on Element nodes,
// this function is an alias to ChildrenMatcher.
func (s *Selection) ContentsMatcher(m Matcher) *Selection {
	return s.ChildrenMatcher(m)
}

// Children gets the child elements of each element in the Selection.
// It returns a new Selection object containing these elements.
func (s *Selection) Children() *Selection {
	return pushStack(s, getChildrenNodes(s.Nodes, siblingAll))
}

// ChildrenFiltered gets the child elements of each element in the Selection,
// filtered by the specified selector. It returns a new
// Selection object containing these elements.
func (s *Selection) ChildrenFiltered(selector string) *Selection {
	return filterAndPush(s, getChildrenNodes(s.Nodes, siblingAll), compileMatcher(selector))
}

// ChildrenMatcher gets the child elements of each element in the Selection,
// filtered by the specified matcher. It returns a new
// Selection object containing these elements.
func (s *Selection) ChildrenMatcher(m Matcher) *Selection {
	return filterAndPush(s, getChildrenNodes(s.Nodes, siblingAll), m)
}

// Parent gets the parent of each element in the Selection. It returns a
// new Selection object containing the matched elements.
func (s *Selection) Parent() *Selection {
	return pushStack(s, getParentNodes(s.Nodes))
}

// ParentFiltered gets the parent of each element in the Selection filtered by a
// selector. It returns a new Selection object containing the matched elements.
func (s *Selection) ParentFiltered(selector string) *Selection {
	return filterAndPush(s, getParentNodes(s.Nodes), compileMatcher(selector))
}

// ParentMatcher gets the parent of each element in the Selection filtered by a
// matcher. It returns a new Selection object containing the matched elements.
func (s *Selection) ParentMatcher(m Matcher) *Selection {
	return filterAndPush(s, getParentNodes(s.Nodes), m)
}

// Closest gets the first element that matches the selector by testing the
// element itself and traversing up through its ancestors in the DOM tree.
func (s *Selection) Closest(selector string) *Selection {
	cs := compileMatcher(selector)
	return s.ClosestMatcher(cs)
}

// ClosestMatcher gets the first element that matches the matcher by testing the
// element itself and traversing up through its ancestors in the DOM tree.
func (s *Selection) ClosestMatcher(m Matcher) *Selection {
	return pushStack(s, mapNodes(s.Nodes, func(i int, n *html.Node) []*html.Node {
		// For each node in the selection, test the node itself, then each parent
		// until a match is found.
		for ; n != nil; n = n.Parent {
			if m.Match(n) {
				return []*html.Node{n}
			}
		}
		return nil
	}))
}

// ClosestNodes gets the first element that matches one of the nodes by testing the
// element itself and traversing up through its ancestors in the DOM tree.
func (s *Selection) ClosestNodes(nodes ...*html.Node) *Selection {
	set := make(map[*html.Node]bool)
	for _, n := range nodes {
		set[n] = true
	}
	return pushStack(s, mapNodes(s.Nodes, func(i int, n *html.Node) []*html.Node {
		// For each node in the selection, test the node itself, then each parent
		// until a match is found.
		for ; n != nil; n = n.Parent {
			if set[n] {
				return []*html.Node{n}
			}
		}
		return nil
	}))
}

// ClosestSelection gets the first element that matches one of the nodes in the
// Selection by testing the element itself and traversing up through its ancestors
// in the DOM tree.
func (s *Selection) ClosestSelection(sel *Selection) *Selection {
	if sel == nil {
		return pushStack(s, nil)
	}
	return s.ClosestNodes(sel.Nodes...)
}

// Parents gets the ancestors of each element in the current Selection. It
// returns a new Selection object with the matched elements.
func (s *Selection) Parents() *Selection {
	return pushStack(s, getParentsNodes(s.Nodes, nil, nil))
}

// ParentsFiltered gets the ancestors of each element in the current
// Selection. It returns a new Selection object with the matched elements.
func (s *Selection) ParentsFiltered(selector string) *Selection {
	return filterAndPush(s, getParentsNodes(s.Nodes, nil, nil), compileMatcher(selector))
}

// ParentsMatcher gets the ancestors of each element in the current
// Selection. It returns a new Selection object with the matched elements.
func (s *Selection) ParentsMatcher(m Matcher) *Selection {
	return filterAndPush(s, getParentsNodes(s.Nodes, nil, nil), m)
}

// ParentsUntil gets the ancestors of each element in the Selection, up to but
// not including the element matched by the selector. It returns a new Selection
// object containing the matched elements.
func (s *Selection) ParentsUntil(selector string) *Selection {
	return pushStack(s, getParentsNodes(s.Nodes, compileMatcher(selector), nil))
}

// ParentsUntilMatcher gets the ancestors of each element in the Selection, up to but
// not including the element matched by the matcher. It returns a new Selection
// object containing the matched elements.
func (s *Selection) ParentsUntilMatcher(m Matcher) *Selection {
	return pushStack(s, getParentsNodes(s.Nodes, m, nil))
}

// ParentsUntilSelection gets the ancestors of each element in the Selection,
// up to but not including the elements in the specified Selection. It returns a
// new Selection object containing the matched elements.
func (s *Selection) ParentsUntilSelection(sel *Selection) *Selection {
	if sel == nil {
		return s.Parents()
	}
	return s.ParentsUntilNodes(sel.Nodes...)
}

// ParentsUntilNodes gets the ancestors of each element in the Selection,
// up to but not including the specified nodes. It returns a
// new Selection object containing the matched elements.
func (s *Selection) ParentsUntilNodes(nodes ...*html.Node) *Selection {
	return pushStack(s, getParentsNodes(s.Nodes, nil, nodes))
}

// ParentsFilteredUntil is like ParentsUntil, with the option to filter the
// results based on a selector string. It returns a new Selection
// object containing the matched elements.
func (s *Selection) ParentsFilteredUntil(filterSelector, untilSelector string) *Selection {
	return filterAndPush(s, getParentsNodes(s.Nodes, compileMatcher(untilSelector), nil), compileMatcher(filterSelector))
}

// ParentsFilteredUntilMatcher is like ParentsUntilMatcher, with the option to filter the
// results based on a matcher. It returns a new Selection object containing the matched elements.
func (s *Selection) ParentsFilteredUntilMatcher(filter, until Matcher) *Selection {
	return filterAndPush(s, getParentsNodes(s.Nodes, until, nil), filter)
}

// ParentsFilteredUntilSelection is like ParentsUntilSelection, with the
// option to filter the results based on a selector string. It returns a new
// Selection object containing the matched elements.
func (s *Selection) ParentsFilteredUntilSelection(filterSelector string, sel *Selection) *Selection {
	return s.ParentsMatcherUntilSelection(compileMatcher(filterSelector), sel)
}

// ParentsMatcherUntilSelection is like ParentsUntilSelection, with the
// option to filter the results based on a matcher. It returns a new
// Selection object containing the matched elements.
func (s *Selection) ParentsMatcherUntilSelection(filter Matcher, sel *Selection) *Selection {
	if sel == nil {
		return s.ParentsMatcher(filter)
	}
	return s.ParentsMatcherUntilNodes(filter, sel.Nodes...)
}

// ParentsFilteredUntilNodes is like ParentsUntilNodes, with the
// option to filter the results based on a selector string. It returns a new
// Selection object containing the matched elements.
func (s *Selection) ParentsFilteredUntilNodes(filterSelector string, nodes ...*html.Node) *Selection {
	return filterAndPush(s, getParentsNodes(s.Nodes, nil, nodes), compileMatcher(filterSelector))
}

// ParentsMatcherUntilNodes is like ParentsUntilNodes, with the
// option to filter the results based on a matcher. It returns a new
// Selection object containing the matched elements.
func (s *Selection) ParentsMatcherUntilNodes(filter Matcher, nodes ...*html.Node) *Selection {
	return filterAndPush(s, getParentsNodes(s.Nodes, nil, nodes), filter)
}

// Siblings gets the siblings of each element in the Selection. It returns
// a new Selection object containing the matched elements.
func (s *Selection) Siblings() *Selection {
	return pushStack(s, getSiblingNodes(s.Nodes, siblingAll, nil, nil))
}

// SiblingsFiltered gets the siblings of each element in the Selection
// filtered by a selector. It returns a new Selection object containing the
// matched elements.
func (s *Selection) SiblingsFiltered(selector string) *Selection {
	return filterAndPush(s, getSiblingNodes(s.Nodes, siblingAll, nil, nil), compileMatcher(selector))
}

// SiblingsMatcher gets the siblings of each element in the Selection
// filtered by a matcher. It returns a new Selection object containing the
// matched elements.
func (s *Selection) SiblingsMatcher(m Matcher) *Selection {
	return filterAndPush(s, getSiblingNodes(s.Nodes, siblingAll, nil, nil), m)
}

// Next gets the immediately following sibling of each element in the
// Selection. It returns a new Selection object containing the matched elements.
func (s *Selection) Next() *Selection {
	return pushStack(s, getSiblingNodes(s.Nodes, siblingNext, nil, nil))
}

// NextFiltered gets the immediately following sibling of each element in the
// Selection filtered by a selector. It returns a new Selection object
// containing the matched elements.
func (s *Selection) NextFiltered(selector string) *Selection {
	return filterAndPush(s, getSiblingNodes(s.Nodes, siblingNext, nil, nil), compileMatcher(selector))
}

// NextMatcher gets the immediately following sibling of each element in the
// Selection filtered by a matcher. It returns a new Selection object
// containing the matched elements.
func (s *Selection) NextMatcher(m Matcher) *Selection {
	return filterAndPush(s, getSiblingNodes(s.Nodes, siblingNext, nil, nil), m)
}

// NextAll gets all the following siblings of each element in the
// Selection. It returns a new Selection object containing the matched elements.
func (s *Selection) NextAll() *Selection {
	return pushStack(s, getSiblingNodes(s.Nodes, siblingNextAll, nil, nil))
}

// NextAllFiltered gets all the following siblings of each element in the
// Selection filtered by a selector. It returns a new Selection object
// containing the matched elements.
func (s *Selection) NextAllFiltered(selector string) *Selection {
	return filterAndPush(s, getSiblingNodes(s.Nodes, siblingNextAll, nil, nil), compileMatcher(selector))
}

// NextAllMatcher gets all the following siblings of each element in the
// Selection filtered by a matcher. It returns a new Selection object
// containing the matched elements.
func (s *Selection) NextAllMatcher(m Matcher) *Selection {
	return filterAndPush(s, getSiblingNodes(s.Nodes, siblingNextAll, nil, nil), m)
}

// Prev gets the immediately preceding sibling of each element in the
// Selection. It returns a new Selection object containing the matched elements.
func (s *Selection) Prev() *Selection {
	return pushStack(s, getSiblingNodes(s.Nodes, siblingPrev, nil, nil))
}

// PrevFiltered gets the immediately preceding sibling of each element in the
// Selection filtered by a selector. It returns a new Selection object
// containing the matched elements.
func (s *Selection) PrevFiltered(selector string) *Selection {
	return filterAndPush(s, getSiblingNodes(s.Nodes, siblingPrev, nil, nil), compileMatcher(selector))
}

// PrevMatcher gets the immediately preceding sibling of each element in the
// Selection filtered by a matcher. It returns a new Selection object
// containing the matched elements.
func (s *Selection) PrevMatcher(m Matcher) *Selection {
	return filterAndPush(s, getSiblingNodes(s.Nodes, siblingPrev, nil, nil), m)
}

// PrevAll gets all the preceding siblings of each element in the
// Selection. It returns a new Selection object containing the matched elements.
func (s *Selection) PrevAll() *Selection {
	return pushStack(s, getSiblingNodes(s.Nodes, siblingPrevAll, nil, nil))
}

// PrevAllFiltered gets all the preceding siblings of each element in the
// Selection filtered by a selector. It returns a new Selection object
// containing the matched elements.
func (s *Selection) PrevAllFiltered(selector string) *Selection {
	return filterAndPush(s, getSiblingNodes(s.Nodes, siblingPrevAll, nil, nil), compileMatcher(selector))
}

// PrevAllMatcher gets all the preceding siblings of each element in the
// Selection filtered by a matcher. It returns a new Selection object
// containing the matched elements.
func (s *Selection) PrevAllMatcher(m Matcher) *Selection {
	return filterAndPush(s, getSiblingNodes(s.Nodes, siblingPrevAll, nil, nil), m)
}

// NextUntil gets all following siblings of each element up to but not
// including the element matched by the selector. It returns a new Selection
// object containing the matched elements.
func (s *Selection) NextUntil(selector string) *Selection {
	return pushStack(s, getSiblingNodes(s.Nodes, siblingNextUntil,
		compileMatcher(selector), nil))
}

// NextUntilMatcher gets all following siblings of each element up to but not
// including the element matched by the matcher. It returns a new Selection
// object containing the matched elements.
func (s *Selection) NextUntilMatcher(m Matcher) *Selection {
	return pushStack(s, getSiblingNodes(s.Nodes, siblingNextUntil,
		m, nil))
}

// NextUntilSelection gets all following siblings of each element up to but not
// including the element matched by the Selection. It returns a new Selection
// object containing the matched elements.
func (s *Selection) NextUntilSelection(sel *Selection) *Selection {
	if sel == nil {
		return s.NextAll()
	}
	return s.NextUntilNodes(sel.Nodes...)
}

// NextUntilNodes gets all following siblings of each element up to but not
// including the element matched by the nodes. It returns a new Selection
// object containing the matched elements.
func (s *Selection) NextUntilNodes(nodes ...*html.Node) *Selection {
	return pushStack(s, getSiblingNodes(s.Nodes, siblingNextUntil,
		nil, nodes))
}

// PrevUntil gets all preceding siblings of each element up to but not
// including the element matched by the selector. It returns a new Selection
// object containing the matched elements.
func (s *Selection) PrevUntil(selector string) *Selection {
	return pushStack(s, getSiblingNodes(s.Nodes, siblingPrevUntil,
		compileMatcher(selector), nil))
}

// PrevUntilMatcher gets all preceding siblings of each element up to but not
// including the element matched by the matcher. It returns a new Selection
// object containing the matched elements.
func (s *Selection) PrevUntilMatcher(m Matcher) *Selection {
	return pushStack(s, getSiblingNodes(s.Nodes, siblingPrevUntil,
		m, nil))
}

// PrevUntilSelection gets all preceding siblings of each element up to but not
// including the element matched by the Selection. It returns a new Selection
// object containing the matched elements.
func (s *Selection) PrevUntilSelection(sel *Selection) *Selection {
	if sel == nil {
		return s.PrevAll()
	}
	return s.PrevUntilNodes(sel.Nodes...)
}

// PrevUntilNodes gets all preceding siblings of each element up to but not
// including the element matched by the nodes. It returns a new Selection
// object containing the matched elements.
func (s *Selection) PrevUntilNodes(nodes ...*html.Node) *Selection {
	return pushStack(s, getSiblingNodes(s.Nodes, siblingPrevUntil,
		nil, nodes))
}

// NextFilteredUntil is like NextUntil, with the option to filter
// the results based on a selector string.
// It returns a new Selection object containing the matched elements.
func (s *Selection) NextFilteredUntil(filterSelector, untilSelector string) *Selection {
	return filterAndPush(s, getSiblingNodes(s.Nodes, siblingNextUntil,
		compileMatcher(untilSelector), nil), compileMatcher(filterSelector))
}

// NextFilteredUntilMatcher is like NextUntilMatcher, with the option to filter
// the results based on a matcher.
// It returns a new Selection object containing the matched elements.
func (s *Selection) NextFilteredUntilMatcher(filter, until Matcher) *Selection {
	return filterAndPush(s, getSiblingNodes(s.Nodes, siblingNextUntil,
		until, nil), filter)
}

// NextFilteredUntilSelection is like NextUntilSelection, with the
// option to filter the results based on a selector string. It returns a new
// Selection object containing the matched elements.
func (s *Selection) NextFilteredUntilSelection(filterSelector string, sel *Selection) *Selection {
	return s.NextMatcherUntilSelection(compileMatcher(filterSelector), sel)
}

// NextMatcherUntilSelection is like NextUntilSelection, with the
// option to filter the results based on a matcher. It returns a new
// Selection object containing the matched elements.
func (s *Selection) NextMatcherUntilSelection(filter Matcher, sel *Selection) *Selection {
	if sel == nil {
		return s.NextMatcher(filter)
	}
	return s.NextMatcherUntilNodes(filter, sel.Nodes...)
}

// NextFilteredUntilNodes is like NextUntilNodes, with the
// option to filter the results based on a selector string. It returns a new
// Selection object containing the matched elements.
func (s *Selection) NextFilteredUntilNodes(filterSelector string, nodes ...*html.Node) *Selection {
	return filterAndPush(s, getSiblingNodes(s.Nodes, siblingNextUntil,
		nil, nodes), compileMatcher(filterSelector))
}

// NextMatcherUntilNodes is like NextUntilNodes, with the
// option to filter the results based on a matcher. It returns a new
// Selection object containing the matched elements.
func (s *Selection) NextMatcherUntilNodes(filter Matcher, nodes ...*html.Node) *Selection {
	return filterAndPush(s, getSiblingNodes(s.Nodes, siblingNextUntil,
		nil, nodes), filter)
}

// PrevFilteredUntil is like PrevUntil, with the option to filter
// the results based on a selector string.
// It returns a new Selection object containing the matched elements.
func (s *Selection) PrevFilteredUntil(filterSelector, untilSelector string) *Selection {
	return filterAndPush(s, getSiblingNodes(s.Nodes, siblingPrevUntil,
		compileMatcher(untilSelector), nil), compileMatcher(filterSelector))
}

// PrevFilteredUntilMatcher is like PrevUntilMatcher, with the option to filter
// the results based on a matcher.
// It returns a new Selection object containing the matched elements.
func (s *Selection) PrevFilteredUntilMatcher(filter, until Matcher) *Selection {
	return filterAndPush(s, getSiblingNodes(s.Nodes, siblingPrevUntil,
		until, nil), filter)
}

// PrevFilteredUntilSelection is like PrevUntilSelection, with the
// option to filter the results based on a selector string. It returns a new
// Selection object containing the matched elements.
func (s *Selection) PrevFilteredUntilSelection(filterSelector string, sel *Selection) *Selection {
	return s.PrevMatcherUntilSelection(compileMatcher(filterSelector), sel)
}

// PrevMatcherUntilSelection is like PrevUntilSelection, with the
// option to filter the results based on a matcher. It returns a new
// Selection object containing the matched elements.
func (s *Selection) PrevMatcherUntilSelection(filter Matcher, sel *Selection) *Selection {
	if sel == nil {
		return s.PrevMatcher(filter)
	}
	return s.PrevMatcherUntilNodes(filter, sel.Nodes...)
}

// PrevFilteredUntilNodes is like PrevUntilNodes, with the
// option to filter the results based on a selector string. It returns a new
// Selection object containing the matched elements.
func (s *Selection) PrevFilteredUntilNodes(filterSelector string, nodes ...*html.Node) *Selection {
	return filterAndPush(s, getSiblingNodes(s.Nodes, siblingPrevUntil,
		nil, nodes), compileMatcher(filterSelector))
}

// PrevMatcherUntilNodes is like PrevUntilNodes, with the
// option to filter the results based on a matcher. It returns a new
// Selection object containing the matched elements.
func (s *Selection) PrevMatcherUntilNodes(filter Matcher, nodes ...*html.Node) *Selection {
	return filterAndPush(s, getSiblingNodes(s.Nodes, siblingPrevUntil,
		nil, nodes), filter)
}

// Filter and push filters the nodes based on a matcher, and pushes the results
// on the stack, with the srcSel as previous selection.
func filterAndPush(srcSel *Selection, nodes []*html.Node, m Matcher) *Selection {
	// Create a temporary Selection with the specified nodes to filter using winnow
	sel := &Selection{nodes, srcSel.document, nil}
	// Filter based on matcher and push on stack
	return pushStack(srcSel, winnow(sel, m, true))
}

// Internal implementation of Find that return raw nodes.
func findWithMatcher(nodes []*html.Node, m Matcher) []*html.Node {
	// Map nodes to find the matches within the children of each node
	return mapNodes(nodes, func(i int, n *html.Node) (result []*html.Node) {
		// Go down one level, becausejQuery's Find selects only within descendants
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode {
				result = append(result, m.MatchAll(c)...)
			}
		}
		return
	})
}

// Internal implementation to get all parent nodes, stopping at the specified
// node (or nil if no stop).
func getParentsNodes(nodes []*html.Node, stopm Matcher, stopNodes []*html.Node) []*html.Node {
	return mapNodes(nodes, func(i int, n *html.Node) (result []*html.Node) {
		for p := n.Parent; p != nil; p = p.Parent {
			sel := newSingleSelection(p, nil)
			if stopm != nil {
				if sel.IsMatcher(stopm) {
					break
				}
			} else if len(stopNodes) > 0 {
				if sel.IsNodes(stopNodes...) {
					break
				}
			}
			if p.Type == html.ElementNode {
				result = append(result, p)
			}
		}
		return
	})
}

// Internal implementation of sibling nodes that return a raw slice of matches.
func getSiblingNodes(nodes []*html.Node, st siblingType, untilm Matcher, untilNodes []*html.Node) []*html.Node {
	var f func(*html.Node) bool

	// If the requested siblings are ...Until, create the test function to
	// determine if the until condition is reached (returns true if it is)
	if st == siblingNextUntil || st == siblingPrevUntil {
		f = func(n *html.Node) bool {
			if untilm != nil {
				// Matcher-based condition
				sel := newSingleSelection(n, nil)
				return sel.IsMatcher(untilm)
			} else if len(untilNodes) > 0 {
				// Nodes-based condition
				sel := newSingleSelection(n, nil)
				return sel.IsNodes(untilNodes...)
			}
			return false
		}
	}

	return mapNodes(nodes, func(i int, n *html.Node) []*html.Node {
		return getChildrenWithSiblingType(n.Parent, st, n, f)
	})
}

// Gets the children nodes of each node in the specified slice of nodes,
// based on the sibling type request.
func getChildrenNodes(nodes []*html.Node, st siblingType) []*html.Node {
	return mapNodes(nodes, func(i int, n *html.Node) []*html.Node {
		return getChildrenWithSiblingType(n, st, nil, nil)
	})
}

// Gets the children of the specified parent, based on the requested sibling
// type, skipping a specified node if required.
func getChildrenWithSiblingType(parent *html.Node, st siblingType, skipNode *html.Node,
	untilFunc func(*html.Node) bool) (result []*html.Node) {

	// Create the iterator function
	var iter = func(cur *html.Node) (ret *html.Node) {
		// Based on the sibling type requested, iterate the right way
		for {
			switch st {
			case siblingAll, siblingAllIncludingNonElements:
				if cur == nil {
					// First iteration, start with first child of parent
					// Skip node if required
					if ret = parent.FirstChild; ret == skipNode && skipNode != nil {
						ret = skipNode.NextSibling
					}
				} else {
					// Skip node if required
					if ret = cur.NextSibling; ret == skipNode && skipNode != nil {
						ret = skipNode.NextSibling
					}
				}
			case siblingPrev, siblingPrevAll, siblingPrevUntil:
				if cur == nil {
					// Start with previous sibling of the skip node
					ret = skipNode.PrevSibling
				} else {
					ret = cur.PrevSibling
				}
			case siblingNext, siblingNextAll, siblingNextUntil:
				if cur == nil {
					// Start with next sibling of the skip node
					ret = skipNode.NextSibling
				} else {
					ret = cur.NextSibling
				}
			default:
				panic("Invalid sibling type.")
			}
			if ret == nil || ret.Type == html.ElementNode || st == siblingAllIncludingNonElements {
				return
			}
			// Not a valid node, try again from this one
			cur = ret
		}
	}

	for c := iter(nil); c != nil; c = iter(c) {
		// If this is an ...Until case, test before append (returns true
		// if the until condition is reached)
		if st == siblingNextUntil || st == siblingPrevUntil {
			if untilFunc(c) {
				return
			}
		}
		result = append(result, c)
		if st == siblingNext || st == siblingPrev {
			// Only one node was requested (immediate next or previous), so exit
			return
		}
	}
	return
}

// Internal implementation of parent nodes that return a raw slice of Nodes.
func getParentNodes(nodes []*html.Node) []*html.Node {
	return mapNodes(nodes, func(i int, n *html.Node) []*html.Node {
		if n.Parent != nil && n.Parent.Type == html.ElementNode {
			return []*html.Node{n.Parent}
		}
		return nil
	})
}

// Internal map function used by many traversing methods. Takes the source nodes
// to iterate on and the mapping function that returns an array of nodes.
// Returns an array of nodes mapped by calling the callback function once for
// each node in the source nodes.
func mapNodes(nodes []*html.Node, f func(int, *html.Node) []*html.Node) (result []*html.Node) {
	set := make(map[*html.Node]bool)
	for i, n := range nodes {
		if vals := f(i, n); len(vals) > 0 {
			result = appendWithoutDuplicates(result, vals, set)
		}
	}
	return result
}
