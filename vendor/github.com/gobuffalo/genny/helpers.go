package genny

import (
	"path/filepath"
	"strings"

	"github.com/gobuffalo/envy"
)

func exts(f File) []string {
	var exts []string

	name := f.Name()
	ext := filepath.Ext(name)

	for ext != "" {
		exts = append([]string{ext}, exts...)
		name = strings.TrimSuffix(name, ext)
		ext = filepath.Ext(name)
	}
	return exts
}

// HasExt checks if a file has ANY of the
// extensions passed in. If no extensions
// are given then `true` is returned
func HasExt(f File, ext ...string) bool {
	if len(ext) == 0 || ext == nil {
		return true
	}
	for _, xt := range ext {
		xt = strings.TrimSpace(xt)
		if xt == "*" || xt == "*.*" {
			return true
		}
		for _, x := range exts(f) {
			if x == xt {
				return true
			}
		}
	}
	return false
}

// StripExt from a File and return a new one
func StripExt(f File, ext string) File {
	name := f.Name()
	name = strings.Replace(name, ext, "", -1)
	return NewFile(name, f)
}

func GoBin() string {
	return envy.Get("GO_BIN", "go")
}
