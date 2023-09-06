package goquery

// Each iterates over a Selection object, executing a function for each
// matched element. It returns the current Selection object. The function
// f is called for each element in the selection with the index of the
// element in that selection starting at 0, and a *Selection that contains
// only that element.
func (s *Selection) Each(f func(int, *Selection)) *Selection {
	for i, n := range s.Nodes {
		f(i, newSingleSelection(n, s.document))
	}
	return s
}

// EachWithBreak iterates over a Selection object, executing a function for each
// matched element. It is identical to Each except that it is possible to break
// out of the loop by returning false in the callback function. It returns the
// current Selection object.
func (s *Selection) EachWithBreak(f func(int, *Selection) bool) *Selection {
	for i, n := range s.Nodes {
		if !f(i, newSingleSelection(n, s.document)) {
			return s
		}
	}
	return s
}

// Map passes each element in the current matched set through a function,
// producing a slice of string holding the returned values. The function
// f is called for each element in the selection with the index of the
// element in that selection starting at 0, and a *Selection that contains
// only that element.
func (s *Selection) Map(f func(int, *Selection) string) (result []string) {
	for i, n := range s.Nodes {
		result = append(result, f(i, newSingleSelection(n, s.document)))
	}

	return result
}
