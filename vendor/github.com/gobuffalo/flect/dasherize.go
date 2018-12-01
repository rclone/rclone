package flect

import (
	"strings"
	"unicode"
)

// Dasherize returns an alphanumeric, lowercased, dashed string
//	Donald E. Knuth = donald-e-knuth
//	Test with + sign = test-with-sign
//	admin/WidgetID = admin-widget-id
func Dasherize(s string) string {
	return New(s).Dasherize().String()
}

// Dasherize returns an alphanumeric, lowercased, dashed string
//	Donald E. Knuth = donald-e-knuth
//	Test with + sign = test-with-sign
//	admin/WidgetID = admin-widget-id
func (i Ident) Dasherize() Ident {
	var parts []string

	for _, part := range i.Parts {
		var x string
		for _, c := range part {
			if unicode.IsLetter(c) || unicode.IsDigit(c) {
				x += string(c)
			}
		}
		parts = xappend(parts, x)
	}

	return New(strings.ToLower(strings.Join(parts, "-")))
}
