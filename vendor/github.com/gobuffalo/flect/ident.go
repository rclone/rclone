package flect

import (
	"encoding"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Ident represents the string and it's parts
type Ident struct {
	Original string
	Parts    []string
}

// String implements fmt.Stringer and returns the original string
func (i Ident) String() string {
	return i.Original
}

// New creates a new Ident from the string
func New(s string) Ident {
	i := Ident{
		Original: s,
		Parts:    toParts(s),
	}

	return i
}

var splitRx = regexp.MustCompile("[^\\p{L}]")

func toParts(s string) []string {
	parts := []string{}
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return parts
	}
	if _, ok := baseAcronyms[strings.ToUpper(s)]; ok {
		return []string{strings.ToUpper(s)}
	}
	var prev rune
	var x string
	for _, c := range s {
		cs := string(c)
		// fmt.Println("### cs ->", cs)
		// fmt.Println("### unicode.IsControl(c) ->", unicode.IsControl(c))
		// fmt.Println("### unicode.IsDigit(c) ->", unicode.IsDigit(c))
		// fmt.Println("### unicode.IsGraphic(c) ->", unicode.IsGraphic(c))
		// fmt.Println("### unicode.IsLetter(c) ->", unicode.IsLetter(c))
		// fmt.Println("### unicode.IsLower(c) ->", unicode.IsLower(c))
		// fmt.Println("### unicode.IsMark(c) ->", unicode.IsMark(c))
		// fmt.Println("### unicode.IsPrint(c) ->", unicode.IsPrint(c))
		// fmt.Println("### unicode.IsPunct(c) ->", unicode.IsPunct(c))
		// fmt.Println("### unicode.IsSpace(c) ->", unicode.IsSpace(c))
		// fmt.Println("### unicode.IsTitle(c) ->", unicode.IsTitle(c))
		// fmt.Println("### unicode.IsUpper(c) ->", unicode.IsUpper(c))
		if !utf8.ValidRune(c) {
			continue
		}

		if isSpace(c) {
			parts = xappend(parts, x)
			x = cs
			prev = c
			continue
		}

		if unicode.IsUpper(c) && !unicode.IsUpper(prev) {
			parts = xappend(parts, x)
			x = cs
			prev = c
			continue
		}
		if unicode.IsUpper(c) && baseAcronyms[strings.ToUpper(x)] {
			parts = xappend(parts, x)
			x = cs
			prev = c
			continue
		}
		if unicode.IsLetter(c) || unicode.IsDigit(c) || unicode.IsPunct(c) || c == '`' {
			prev = c
			x += cs
			continue
		}

		parts = xappend(parts, x)
		x = ""
		prev = c
	}
	parts = xappend(parts, x)

	return parts
}

var _ encoding.TextUnmarshaler = &Ident{}
var _ encoding.TextMarshaler = &Ident{}

//UnmarshalText unmarshalls byte array into the Ident
func (i *Ident) UnmarshalText(data []byte) error {
	(*i) = New(string(data))
	return nil
}

//MarshalText marshals Ident into byte array
func (i Ident) MarshalText() ([]byte, error) {
	return []byte(i.Original), nil
}
