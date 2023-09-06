package goquery

import (
	"strings"

	"golang.org/x/net/html"
)

// After applies the selector from the root document and inserts the matched elements
// after the elements in the set of matched elements.
//
// If one of the matched elements in the selection is not currently in the
// document, it's impossible to insert nodes after it, so it will be ignored.
//
// This follows the same rules as Selection.Append.
func (s *Selection) After(selector string) *Selection {
	return s.AfterMatcher(compileMatcher(selector))
}

// AfterMatcher applies the matcher from the root document and inserts the matched elements
// after the elements in the set of matched elements.
//
// If one of the matched elements in the selection is not currently in the
// document, it's impossible to insert nodes after it, so it will be ignored.
//
// This follows the same rules as Selection.Append.
func (s *Selection) AfterMatcher(m Matcher) *Selection {
	return s.AfterNodes(m.MatchAll(s.document.rootNode)...)
}

// AfterSelection inserts the elements in the selection after each element in the set of matched
// elements.
//
// This follows the same rules as Selection.Append.
func (s *Selection) AfterSelection(sel *Selection) *Selection {
	return s.AfterNodes(sel.Nodes...)
}

// AfterHtml parses the html and inserts it after the set of matched elements.
//
// This follows the same rules as Selection.Append.
func (s *Selection) AfterHtml(htmlStr string) *Selection {
	return s.eachNodeHtml(htmlStr, true, func(node *html.Node, nodes []*html.Node) {
		nextSibling := node.NextSibling
		for _, n := range nodes {
			if node.Parent != nil {
				node.Parent.InsertBefore(n, nextSibling)
			}
		}
	})
}

// AfterNodes inserts the nodes after each element in the set of matched elements.
//
// This follows the same rules as Selection.Append.
func (s *Selection) AfterNodes(ns ...*html.Node) *Selection {
	return s.manipulateNodes(ns, true, func(sn *html.Node, n *html.Node) {
		if sn.Parent != nil {
			sn.Parent.InsertBefore(n, sn.NextSibling)
		}
	})
}

// Append appends the elements specified by the selector to the end of each element
// in the set of matched elements, following those rules:
//
// 1) The selector is applied to the root document.
//
// 2) Elements that are part of the document will be moved to the new location.
//
// 3) If there are multiple locations to append to, cloned nodes will be
// appended to all target locations except the last one, which will be moved
// as noted in (2).
func (s *Selection) Append(selector string) *Selection {
	return s.AppendMatcher(compileMatcher(selector))
}

// AppendMatcher appends the elements specified by the matcher to the end of each element
// in the set of matched elements.
//
// This follows the same rules as Selection.Append.
func (s *Selection) AppendMatcher(m Matcher) *Selection {
	return s.AppendNodes(m.MatchAll(s.document.rootNode)...)
}

// AppendSelection appends the elements in the selection to the end of each element
// in the set of matched elements.
//
// This follows the same rules as Selection.Append.
func (s *Selection) AppendSelection(sel *Selection) *Selection {
	return s.AppendNodes(sel.Nodes...)
}

// AppendHtml parses the html and appends it to the set of matched elements.
func (s *Selection) AppendHtml(htmlStr string) *Selection {
	return s.eachNodeHtml(htmlStr, false, func(node *html.Node, nodes []*html.Node) {
		for _, n := range nodes {
			node.AppendChild(n)
		}
	})
}

// AppendNodes appends the specified nodes to each node in the set of matched elements.
//
// This follows the same rules as Selection.Append.
func (s *Selection) AppendNodes(ns ...*html.Node) *Selection {
	return s.manipulateNodes(ns, false, func(sn *html.Node, n *html.Node) {
		sn.AppendChild(n)
	})
}

// Before inserts the matched elements before each element in the set of matched elements.
//
// This follows the same rules as Selection.Append.
func (s *Selection) Before(selector string) *Selection {
	return s.BeforeMatcher(compileMatcher(selector))
}

// BeforeMatcher inserts the matched elements before each element in the set of matched elements.
//
// This follows the same rules as Selection.Append.
func (s *Selection) BeforeMatcher(m Matcher) *Selection {
	return s.BeforeNodes(m.MatchAll(s.document.rootNode)...)
}

// BeforeSelection inserts the elements in the selection before each element in the set of matched
// elements.
//
// This follows the same rules as Selection.Append.
func (s *Selection) BeforeSelection(sel *Selection) *Selection {
	return s.BeforeNodes(sel.Nodes...)
}

// BeforeHtml parses the html and inserts it before the set of matched elements.
//
// This follows the same rules as Selection.Append.
func (s *Selection) BeforeHtml(htmlStr string) *Selection {
	return s.eachNodeHtml(htmlStr, true, func(node *html.Node, nodes []*html.Node) {
		for _, n := range nodes {
			if node.Parent != nil {
				node.Parent.InsertBefore(n, node)
			}
		}
	})
}

// BeforeNodes inserts the nodes before each element in the set of matched elements.
//
// This follows the same rules as Selection.Append.
func (s *Selection) BeforeNodes(ns ...*html.Node) *Selection {
	return s.manipulateNodes(ns, false, func(sn *html.Node, n *html.Node) {
		if sn.Parent != nil {
			sn.Parent.InsertBefore(n, sn)
		}
	})
}

// Clone creates a deep copy of the set of matched nodes. The new nodes will not be
// attached to the document.
func (s *Selection) Clone() *Selection {
	ns := newEmptySelection(s.document)
	ns.Nodes = cloneNodes(s.Nodes)
	return ns
}

// Empty removes all children nodes from the set of matched elements.
// It returns the children nodes in a new Selection.
func (s *Selection) Empty() *Selection {
	var nodes []*html.Node

	for _, n := range s.Nodes {
		for c := n.FirstChild; c != nil; c = n.FirstChild {
			n.RemoveChild(c)
			nodes = append(nodes, c)
		}
	}

	return pushStack(s, nodes)
}

// Prepend prepends the elements specified by the selector to each element in
// the set of matched elements, following the same rules as Append.
func (s *Selection) Prepend(selector string) *Selection {
	return s.PrependMatcher(compileMatcher(selector))
}

// PrependMatcher prepends the elements specified by the matcher to each
// element in the set of matched elements.
//
// This follows the same rules as Selection.Append.
func (s *Selection) PrependMatcher(m Matcher) *Selection {
	return s.PrependNodes(m.MatchAll(s.document.rootNode)...)
}

// PrependSelection prepends the elements in the selection to each element in
// the set of matched elements.
//
// This follows the same rules as Selection.Append.
func (s *Selection) PrependSelection(sel *Selection) *Selection {
	return s.PrependNodes(sel.Nodes...)
}

// PrependHtml parses the html and prepends it to the set of matched elements.
func (s *Selection) PrependHtml(htmlStr string) *Selection {
	return s.eachNodeHtml(htmlStr, false, func(node *html.Node, nodes []*html.Node) {
		firstChild := node.FirstChild
		for _, n := range nodes {
			node.InsertBefore(n, firstChild)
		}
	})
}

// PrependNodes prepends the specified nodes to each node in the set of
// matched elements.
//
// This follows the same rules as Selection.Append.
func (s *Selection) PrependNodes(ns ...*html.Node) *Selection {
	return s.manipulateNodes(ns, true, func(sn *html.Node, n *html.Node) {
		// sn.FirstChild may be nil, in which case this functions like
		// sn.AppendChild()
		sn.InsertBefore(n, sn.FirstChild)
	})
}

// Remove removes the set of matched elements from the document.
// It returns the same selection, now consisting of nodes not in the document.
func (s *Selection) Remove() *Selection {
	for _, n := range s.Nodes {
		if n.Parent != nil {
			n.Parent.RemoveChild(n)
		}
	}

	return s
}

// RemoveFiltered removes from the current set of matched elements those that
// match the selector filter. It returns the Selection of removed nodes.
//
// For example if the selection s contains "<h1>", "<h2>" and "<h3>"
// and s.RemoveFiltered("h2") is called, only the "<h2>" node is removed
// (and returned), while "<h1>" and "<h3>" are kept in the document.
func (s *Selection) RemoveFiltered(selector string) *Selection {
	return s.RemoveMatcher(compileMatcher(selector))
}

// RemoveMatcher removes from the current set of matched elements those that
// match the Matcher filter. It returns the Selection of removed nodes.
// See RemoveFiltered for additional information.
func (s *Selection) RemoveMatcher(m Matcher) *Selection {
	return s.FilterMatcher(m).Remove()
}

// ReplaceWith replaces each element in the set of matched elements with the
// nodes matched by the given selector.
// It returns the removed elements.
//
// This follows the same rules as Selection.Append.
func (s *Selection) ReplaceWith(selector string) *Selection {
	return s.ReplaceWithMatcher(compileMatcher(selector))
}

// ReplaceWithMatcher replaces each element in the set of matched elements with
// the nodes matched by the given Matcher.
// It returns the removed elements.
//
// This follows the same rules as Selection.Append.
func (s *Selection) ReplaceWithMatcher(m Matcher) *Selection {
	return s.ReplaceWithNodes(m.MatchAll(s.document.rootNode)...)
}

// ReplaceWithSelection replaces each element in the set of matched elements with
// the nodes from the given Selection.
// It returns the removed elements.
//
// This follows the same rules as Selection.Append.
func (s *Selection) ReplaceWithSelection(sel *Selection) *Selection {
	return s.ReplaceWithNodes(sel.Nodes...)
}

// ReplaceWithHtml replaces each element in the set of matched elements with
// the parsed HTML.
// It returns the removed elements.
//
// This follows the same rules as Selection.Append.
func (s *Selection) ReplaceWithHtml(htmlStr string) *Selection {
	s.eachNodeHtml(htmlStr, true, func(node *html.Node, nodes []*html.Node) {
		nextSibling := node.NextSibling
		for _, n := range nodes {
			if node.Parent != nil {
				node.Parent.InsertBefore(n, nextSibling)
			}
		}
	})
	return s.Remove()
}

// ReplaceWithNodes replaces each element in the set of matched elements with
// the given nodes.
// It returns the removed elements.
//
// This follows the same rules as Selection.Append.
func (s *Selection) ReplaceWithNodes(ns ...*html.Node) *Selection {
	s.AfterNodes(ns...)
	return s.Remove()
}

// SetHtml sets the html content of each element in the selection to
// specified html string.
func (s *Selection) SetHtml(htmlStr string) *Selection {
	for _, context := range s.Nodes {
		for c := context.FirstChild; c != nil; c = context.FirstChild {
			context.RemoveChild(c)
		}
	}
	return s.eachNodeHtml(htmlStr, false, func(node *html.Node, nodes []*html.Node) {
		for _, n := range nodes {
			node.AppendChild(n)
		}
	})
}

// SetText sets the content of each element in the selection to specified content.
// The provided text string is escaped.
func (s *Selection) SetText(text string) *Selection {
	return s.SetHtml(html.EscapeString(text))
}

// Unwrap removes the parents of the set of matched elements, leaving the matched
// elements (and their siblings, if any) in their place.
// It returns the original selection.
func (s *Selection) Unwrap() *Selection {
	s.Parent().Each(func(i int, ss *Selection) {
		// For some reason, jquery allows unwrap to remove the <head> element, so
		// allowing it here too. Same for <html>. Why it allows those elements to
		// be unwrapped while not allowing body is a mystery to me.
		if ss.Nodes[0].Data != "body" {
			ss.ReplaceWithSelection(ss.Contents())
		}
	})

	return s
}

// Wrap wraps each element in the set of matched elements inside the first
// element matched by the given selector. The matched child is cloned before
// being inserted into the document.
//
// It returns the original set of elements.
func (s *Selection) Wrap(selector string) *Selection {
	return s.WrapMatcher(compileMatcher(selector))
}

// WrapMatcher wraps each element in the set of matched elements inside the
// first element matched by the given matcher. The matched child is cloned
// before being inserted into the document.
//
// It returns the original set of elements.
func (s *Selection) WrapMatcher(m Matcher) *Selection {
	return s.wrapNodes(m.MatchAll(s.document.rootNode)...)
}

// WrapSelection wraps each element in the set of matched elements inside the
// first element in the given Selection. The element is cloned before being
// inserted into the document.
//
// It returns the original set of elements.
func (s *Selection) WrapSelection(sel *Selection) *Selection {
	return s.wrapNodes(sel.Nodes...)
}

// WrapHtml wraps each element in the set of matched elements inside the inner-
// most child of the given HTML.
//
// It returns the original set of elements.
func (s *Selection) WrapHtml(htmlStr string) *Selection {
	nodesMap := make(map[string][]*html.Node)
	for _, context := range s.Nodes {
		var parent *html.Node
		if context.Parent != nil {
			parent = context.Parent
		} else {
			parent = &html.Node{Type: html.ElementNode}
		}
		nodes, found := nodesMap[nodeName(parent)]
		if !found {
			nodes = parseHtmlWithContext(htmlStr, parent)
			nodesMap[nodeName(parent)] = nodes
		}
		newSingleSelection(context, s.document).wrapAllNodes(cloneNodes(nodes)...)
	}
	return s
}

// WrapNode wraps each element in the set of matched elements inside the inner-
// most child of the given node. The given node is copied before being inserted
// into the document.
//
// It returns the original set of elements.
func (s *Selection) WrapNode(n *html.Node) *Selection {
	return s.wrapNodes(n)
}

func (s *Selection) wrapNodes(ns ...*html.Node) *Selection {
	s.Each(func(i int, ss *Selection) {
		ss.wrapAllNodes(ns...)
	})

	return s
}

// WrapAll wraps a single HTML structure, matched by the given selector, around
// all elements in the set of matched elements. The matched child is cloned
// before being inserted into the document.
//
// It returns the original set of elements.
func (s *Selection) WrapAll(selector string) *Selection {
	return s.WrapAllMatcher(compileMatcher(selector))
}

// WrapAllMatcher wraps a single HTML structure, matched by the given Matcher,
// around all elements in the set of matched elements. The matched child is
// cloned before being inserted into the document.
//
// It returns the original set of elements.
func (s *Selection) WrapAllMatcher(m Matcher) *Selection {
	return s.wrapAllNodes(m.MatchAll(s.document.rootNode)...)
}

// WrapAllSelection wraps a single HTML structure, the first node of the given
// Selection, around all elements in the set of matched elements. The matched
// child is cloned before being inserted into the document.
//
// It returns the original set of elements.
func (s *Selection) WrapAllSelection(sel *Selection) *Selection {
	return s.wrapAllNodes(sel.Nodes...)
}

// WrapAllHtml wraps the given HTML structure around all elements in the set of
// matched elements. The matched child is cloned before being inserted into the
// document.
//
// It returns the original set of elements.
func (s *Selection) WrapAllHtml(htmlStr string) *Selection {
	var context *html.Node
	var nodes []*html.Node
	if len(s.Nodes) > 0 {
		context = s.Nodes[0]
		if context.Parent != nil {
			nodes = parseHtmlWithContext(htmlStr, context)
		} else {
			nodes = parseHtml(htmlStr)
		}
	}
	return s.wrapAllNodes(nodes...)
}

func (s *Selection) wrapAllNodes(ns ...*html.Node) *Selection {
	if len(ns) > 0 {
		return s.WrapAllNode(ns[0])
	}
	return s
}

// WrapAllNode wraps the given node around the first element in the Selection,
// making all other nodes in the Selection children of the given node. The node
// is cloned before being inserted into the document.
//
// It returns the original set of elements.
func (s *Selection) WrapAllNode(n *html.Node) *Selection {
	if s.Size() == 0 {
		return s
	}

	wrap := cloneNode(n)

	first := s.Nodes[0]
	if first.Parent != nil {
		first.Parent.InsertBefore(wrap, first)
		first.Parent.RemoveChild(first)
	}

	for c := getFirstChildEl(wrap); c != nil; c = getFirstChildEl(wrap) {
		wrap = c
	}

	newSingleSelection(wrap, s.document).AppendSelection(s)

	return s
}

// WrapInner wraps an HTML structure, matched by the given selector, around the
// content of element in the set of matched elements. The matched child is
// cloned before being inserted into the document.
//
// It returns the original set of elements.
func (s *Selection) WrapInner(selector string) *Selection {
	return s.WrapInnerMatcher(compileMatcher(selector))
}

// WrapInnerMatcher wraps an HTML structure, matched by the given selector,
// around the content of element in the set of matched elements. The matched
// child is cloned before being inserted into the document.
//
// It returns the original set of elements.
func (s *Selection) WrapInnerMatcher(m Matcher) *Selection {
	return s.wrapInnerNodes(m.MatchAll(s.document.rootNode)...)
}

// WrapInnerSelection wraps an HTML structure, matched by the given selector,
// around the content of element in the set of matched elements. The matched
// child is cloned before being inserted into the document.
//
// It returns the original set of elements.
func (s *Selection) WrapInnerSelection(sel *Selection) *Selection {
	return s.wrapInnerNodes(sel.Nodes...)
}

// WrapInnerHtml wraps an HTML structure, matched by the given selector, around
// the content of element in the set of matched elements. The matched child is
// cloned before being inserted into the document.
//
// It returns the original set of elements.
func (s *Selection) WrapInnerHtml(htmlStr string) *Selection {
	nodesMap := make(map[string][]*html.Node)
	for _, context := range s.Nodes {
		nodes, found := nodesMap[nodeName(context)]
		if !found {
			nodes = parseHtmlWithContext(htmlStr, context)
			nodesMap[nodeName(context)] = nodes
		}
		newSingleSelection(context, s.document).wrapInnerNodes(cloneNodes(nodes)...)
	}
	return s
}

// WrapInnerNode wraps an HTML structure, matched by the given selector, around
// the content of element in the set of matched elements. The matched child is
// cloned before being inserted into the document.
//
// It returns the original set of elements.
func (s *Selection) WrapInnerNode(n *html.Node) *Selection {
	return s.wrapInnerNodes(n)
}

func (s *Selection) wrapInnerNodes(ns ...*html.Node) *Selection {
	if len(ns) == 0 {
		return s
	}

	s.Each(func(i int, s *Selection) {
		contents := s.Contents()

		if contents.Size() > 0 {
			contents.wrapAllNodes(ns...)
		} else {
			s.AppendNodes(cloneNode(ns[0]))
		}
	})

	return s
}

func parseHtml(h string) []*html.Node {
	// Errors are only returned when the io.Reader returns any error besides
	// EOF, but strings.Reader never will
	nodes, err := html.ParseFragment(strings.NewReader(h), &html.Node{Type: html.ElementNode})
	if err != nil {
		panic("goquery: failed to parse HTML: " + err.Error())
	}
	return nodes
}

func parseHtmlWithContext(h string, context *html.Node) []*html.Node {
	// Errors are only returned when the io.Reader returns any error besides
	// EOF, but strings.Reader never will
	nodes, err := html.ParseFragment(strings.NewReader(h), context)
	if err != nil {
		panic("goquery: failed to parse HTML: " + err.Error())
	}
	return nodes
}

// Get the first child that is an ElementNode
func getFirstChildEl(n *html.Node) *html.Node {
	c := n.FirstChild
	for c != nil && c.Type != html.ElementNode {
		c = c.NextSibling
	}
	return c
}

// Deep copy a slice of nodes.
func cloneNodes(ns []*html.Node) []*html.Node {
	cns := make([]*html.Node, 0, len(ns))

	for _, n := range ns {
		cns = append(cns, cloneNode(n))
	}

	return cns
}

// Deep copy a node. The new node has clones of all the original node's
// children but none of its parents or siblings.
func cloneNode(n *html.Node) *html.Node {
	nn := &html.Node{
		Type:     n.Type,
		DataAtom: n.DataAtom,
		Data:     n.Data,
		Attr:     make([]html.Attribute, len(n.Attr)),
	}

	copy(nn.Attr, n.Attr)
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		nn.AppendChild(cloneNode(c))
	}

	return nn
}

func (s *Selection) manipulateNodes(ns []*html.Node, reverse bool,
	f func(sn *html.Node, n *html.Node)) *Selection {

	lasti := s.Size() - 1

	// net.Html doesn't provide document fragments for insertion, so to get
	// things in the correct order with After() and Prepend(), the callback
	// needs to be called on the reverse of the nodes.
	if reverse {
		for i, j := 0, len(ns)-1; i < j; i, j = i+1, j-1 {
			ns[i], ns[j] = ns[j], ns[i]
		}
	}

	for i, sn := range s.Nodes {
		for _, n := range ns {
			if i != lasti {
				f(sn, cloneNode(n))
			} else {
				if n.Parent != nil {
					n.Parent.RemoveChild(n)
				}
				f(sn, n)
			}
		}
	}

	return s
}

// eachNodeHtml parses the given html string and inserts the resulting nodes in the dom with the mergeFn.
// The parsed nodes are inserted for each element of the selection.
// isParent can be used to indicate that the elements of the selection should be treated as the parent for the parsed html.
// A cache is used to avoid parsing the html multiple times should the elements of the selection result in the same context.
func (s *Selection) eachNodeHtml(htmlStr string, isParent bool, mergeFn func(n *html.Node, nodes []*html.Node)) *Selection {
	// cache to avoid parsing the html for the same context multiple times
	nodeCache := make(map[string][]*html.Node)
	var context *html.Node
	for _, n := range s.Nodes {
		if isParent {
			context = n.Parent
		} else {
			if n.Type != html.ElementNode {
				continue
			}
			context = n
		}
		if context != nil {
			nodes, found := nodeCache[nodeName(context)]
			if !found {
				nodes = parseHtmlWithContext(htmlStr, context)
				nodeCache[nodeName(context)] = nodes
			}
			mergeFn(n, cloneNodes(nodes))
		}
	}
	return s
}
