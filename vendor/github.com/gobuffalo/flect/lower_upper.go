package flect

import "strings"

// ToUpper is a convience wrapper for strings.ToUpper
func (i Ident) ToUpper() Ident {
	return New(strings.ToUpper(i.Original))
}

// ToLower is a convience wrapper for strings.ToLower
func (i Ident) ToLower() Ident {
	return New(strings.ToLower(i.Original))
}
