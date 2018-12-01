package flect

import (
	"strings"
	"unicode"
)

// Underscore a string
//	bob dylan = bob_dylan
//	Nice to see you! = nice_to_see_you
//	widgetID = widget_id
func Underscore(s string) string {
	return New(s).Underscore().String()
}

// Underscore a string
//	bob dylan = bob_dylan
//	Nice to see you! = nice_to_see_you
//	widgetID = widget_id
func (i Ident) Underscore() Ident {
	var out []string
	for _, part := range i.Parts {
		var x string
		for _, c := range part {
			if unicode.IsLetter(c) || unicode.IsDigit(c) {
				x += string(c)
			}
		}
		if x != "" {
			out = append(out, x)
		}
	}
	return New(strings.ToLower(strings.Join(out, "_")))
}
