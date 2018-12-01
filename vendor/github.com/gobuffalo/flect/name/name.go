package name

import (
	"encoding"
	"strings"

	"github.com/gobuffalo/flect"
)

// Proper pascalizes and singularizes the string
//	person = Person
//	foo_bar = FooBar
//	admin/widgets = AdminWidget
func Proper(s string) string {
	return New(s).Proper().String()
}

// Proper pascalizes and singularizes the string
//	person = Person
//	foo_bar = FooBar
//	admin/widgets = AdminWidget
func (i Ident) Proper() Ident {
	return Ident{i.Singularize().Pascalize()}
}

// Group pascalizes and pluralizes the string
//	person = People
//	foo_bar = FooBars
//	admin/widget = AdminWidgets
func Group(s string) string {
	return New(s).Group().String()
}

// Group pascalizes and pluralizes the string
//	person = People
//	foo_bar = FooBars
//	admin/widget = AdminWidgets
func (i Ident) Group() Ident {
	var parts []string
	if len(i.Original) == 0 {
		return i
	}
	last := i.Parts[len(i.Parts)-1]
	for _, part := range i.Parts[:len(i.Parts)-1] {
		parts = append(parts, flect.Pascalize(part))
	}
	last = New(last).Pluralize().Pascalize().String()
	parts = append(parts, last)
	return New(strings.Join(parts, ""))
}

var _ encoding.TextUnmarshaler = &Ident{}
var _ encoding.TextMarshaler = &Ident{}

func (i *Ident) UnmarshalText(data []byte) error {
	(*i) = New(string(data))
	return nil
}

func (i Ident) MarshalText() ([]byte, error) {
	return []byte(i.Original), nil
}
