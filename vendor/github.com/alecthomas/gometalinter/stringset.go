package main

type stringSet struct {
	items map[string]struct{}
}

func newStringSet(items ...string) *stringSet {
	setItems := make(map[string]struct{}, len(items))
	for _, item := range items {
		setItems[item] = struct{}{}
	}
	return &stringSet{items: setItems}
}

func (s *stringSet) add(item string) {
	s.items[item] = struct{}{}
}

func (s *stringSet) asSlice() []string {
	items := []string{}
	for item := range s.items {
		items = append(items, item)
	}
	return items
}

func (s *stringSet) size() int {
	return len(s.items)
}
