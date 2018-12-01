package name

import "unicode"

// Char returns the first letter, lowered
//	"" = "x"
//	"foo" = "f"
//	"123d456" = "d"
func Char(s string) string {
	return New(s).Char().String()
}

// Char returns the first letter, lowered
//	"" = "x"
//	"foo" = "f"
//	"123d456" = "d"
func (i Ident) Char() Ident {
	for _, c := range i.Original {
		if unicode.IsLetter(c) {
			return New(string(unicode.ToLower(c)))
		}
	}
	return New("x")
}
