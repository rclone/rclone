package name

import "path/filepath"

func FilePathJoin(names ...string) string {
	var ni = make([]Ident, len(names))
	for i, n := range names {
		ni[i] = New(n)
	}
	base := New("")
	return base.FilePathJoin(ni...).String()
}

func (i Ident) FilePathJoin(ni ...Ident) Ident {
	var s = make([]string, len(ni))
	for i, n := range ni {
		s[i] = n.OsPath().String()
	}
	return New(filepath.Join(s...))
}
