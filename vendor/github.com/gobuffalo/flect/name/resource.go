package name

import (
	"strings"
)

// Resource version of a name
func (n Ident) Resource() Ident {
	name := n.Underscore().String()
	x := strings.FieldsFunc(name, func(r rune) bool {
		return r == '_' || r == '/'
	})

	for i, w := range x {
		if i == len(x)-1 {
			x[i] = New(w).Pluralize().Pascalize().String()
			continue
		}

		x[i] = New(w).Pascalize().String()
	}

	return New(strings.Join(x, ""))
}
