package name

import (
	"go/build"
	"path/filepath"
	"strings"
)

// Package will attempt to return a package version of the name
//	$GOPATH/src/foo/bar = foo/bar
//	$GOPATH\src\foo\bar = foo/bar
//	foo/bar = foo/bar
func Package(s string) string {
	return New(s).Package().String()
}

// Package will attempt to return a package version of the name
//	$GOPATH/src/foo/bar = foo/bar
//	$GOPATH\src\foo\bar = foo/bar
//	foo/bar = foo/bar
func (i Ident) Package() Ident {
	c := build.Default

	s := i.Original

	for _, src := range c.SrcDirs() {
		s = strings.TrimPrefix(s, src)
		s = strings.TrimPrefix(s, filepath.Dir(src)) // encase there's no /src prefix
	}

	s = strings.TrimPrefix(s, string(filepath.Separator))
	s = strings.Replace(s, "\\", "/", -1)
	s = strings.Replace(s, "_", "", -1)
	return Ident{New(s).ToLower()}
}
