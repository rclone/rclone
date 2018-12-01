package name

import (
	"strings"
)

func Key(s string) string {
	return New(s).Key().String()
}

func (i Ident) Key() Ident {
	s := strings.Replace(i.String(), "\\", "/", -1)
	return New(strings.ToLower(s))
}
