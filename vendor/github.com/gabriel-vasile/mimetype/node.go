package mimetype

type (
	// node represents a vertex in the matchers tree structure.
	// It holds the mime type, the extension and the function
	// to check whether a byte slice has the mime type.
	node struct {
		mime      string
		extension string
		matchFunc func([]byte) bool
		children  []*node
	}
)

func newNode(mime, extension string, matchFunc func([]byte) bool, children ...*node) *node {
	return &node{
		mime:      mime,
		extension: extension,
		matchFunc: matchFunc,
		children:  children,
	}
}

// match does a depth-first search on the matchers tree.
// it returns the deepest successful matcher for which all the children fail.
func (n *node) match(in []byte, deepestMatch *node) *node {
	for _, c := range n.children {
		if c.matchFunc(in) {
			return c.match(in, c)
		}
	}

	return deepestMatch
}

func (n *node) flatten() []*node {
	out := []*node{n}
	for _, c := range n.children {
		out = append(out, c.flatten()...)
	}

	return out
}
