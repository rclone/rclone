package genny

import (
	"strings"
)

// Replace search/replace in a file name
func Replace(search string, replace string) Transformer {
	return NewTransformer("*", func(f File) (File, error) {
		name := f.Name()
		name = strings.Replace(name, search, replace, -1)
		return NewFile(name, f), nil
	})
}

// Dot will convert -dot- in a file name to just a .
// example -dot-travis.yml becomes .travis.yml
func Dot() Transformer {
	return Replace("-dot-", ".")
}
