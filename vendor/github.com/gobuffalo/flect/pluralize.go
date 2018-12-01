package flect

import (
	"strings"
	"sync"
)

var pluralMoot = &sync.RWMutex{}

// Pluralize returns a plural version of the string
//	user = users
//	person = people
//	datum = data
func Pluralize(s string) string {
	return New(s).Pluralize().String()
}

// Pluralize returns a plural version of the string
//	user = users
//	person = people
//	datum = data
func (i Ident) Pluralize() Ident {
	s := i.Original
	if len(s) == 0 {
		return New("")
	}

	pluralMoot.RLock()
	defer pluralMoot.RUnlock()

	ls := strings.ToLower(s)
	if _, ok := pluralToSingle[ls]; ok {
		return i
	}
	if p, ok := singleToPlural[ls]; ok {
		return New(p)
	}
	for _, r := range pluralRules {
		if strings.HasSuffix(ls, r.suffix) {
			return New(r.fn(s))
		}
	}

	if strings.HasSuffix(ls, "s") {
		return i
	}

	return New(i.String() + "s")
}
