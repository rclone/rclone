package flect

import "unicode"

// Capitalize will cap the first letter of string
//	user = User
//	bob dylan = Bob dylan
//	widget_id = Widget_id
func Capitalize(s string) string {
	return New(s).Capitalize().String()
}

// Capitalize will cap the first letter of string
//	user = User
//	bob dylan = Bob dylan
//	widget_id = Widget_id
func (i Ident) Capitalize() Ident {
	var x string
	if len(i.Parts) == 0 {
		return New("")
	}
	x = string(unicode.ToTitle(rune(i.Original[0])))
	if len(i.Original) > 1 {
		x += i.Original[1:]
	}
	return New(x)
}
