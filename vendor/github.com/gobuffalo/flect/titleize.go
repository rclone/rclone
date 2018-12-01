package flect

import (
	"strings"
	"unicode"
)

// Titleize will capitalize the start of each part
//	"Nice to see you!" = "Nice To See You!"
//	"i've read a book! have you?" = "I've Read A Book! Have You?"
//	"This is `code` ok" = "This Is `code` OK"
func Titleize(s string) string {
	return New(s).Titleize().String()
}

// Titleize will capitalize the start of each part
//	"Nice to see you!" = "Nice To See You!"
//	"i've read a book! have you?" = "I've Read A Book! Have You?"
//	"This is `code` ok" = "This Is `code` OK"
func (i Ident) Titleize() Ident {
	var parts []string
	for _, part := range i.Parts {
		var x string
		x = string(unicode.ToTitle(rune(part[0])))
		if len(part) > 1 {
			x += part[1:]
		}
		parts = append(parts, x)
	}
	return New(strings.Join(parts, " "))
}
