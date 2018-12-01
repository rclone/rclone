package flect

import (
	"strings"
	"unicode"
)

// Camelize returns a camelize version of a string
//	bob dylan = bobDylan
//	widget_id = widgetID
//	WidgetID = widgetID
func Camelize(s string) string {
	return New(s).Camelize().String()
}

// Camelize returns a camelize version of a string
//	bob dylan = bobDylan
//	widget_id = widgetID
//	WidgetID = widgetID
func (i Ident) Camelize() Ident {
	var out []string
	for i, part := range i.Parts {
		var x string
		var capped bool
		if strings.ToLower(part) == "id" {
			out = append(out, "ID")
			continue
		}
		for _, c := range part {
			if unicode.IsLetter(c) || unicode.IsDigit(c) {
				if i == 0 {
					x += string(unicode.ToLower(c))
					continue
				}
				if !capped {
					capped = true
					x += string(unicode.ToUpper(c))
					continue
				}
				x += string(c)
			}
		}
		if x != "" {
			out = append(out, x)
		}
	}
	return New(strings.Join(out, ""))
}
