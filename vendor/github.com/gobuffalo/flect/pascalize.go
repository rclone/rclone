package flect

import (
	"unicode"
)

// Pascalize returns a string with each segment capitalized
//	user = User
//	bob dylan = BobDylan
//	widget_id = WidgetID
func Pascalize(s string) string {
	return New(s).Pascalize().String()
}

// Pascalize returns a string with each segment capitalized
//	user = User
//	bob dylan = BobDylan
//	widget_id = WidgetID
func (i Ident) Pascalize() Ident {
	c := i.Camelize()
	if len(c.String()) == 0 {
		return c
	}
	return New(string(unicode.ToUpper(rune(c.Original[0]))) + c.Original[1:])
}
