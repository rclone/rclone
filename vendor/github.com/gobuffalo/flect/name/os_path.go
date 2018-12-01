package name

import (
	"path/filepath"
	"runtime"
	"strings"
)

func OsPath(s string) string {
	return New(s).OsPath().String()
}

func (i Ident) OsPath() Ident {
	s := i.String()
	if runtime.GOOS == "windows" {
		s = strings.Replace(s, "/", string(filepath.Separator), -1)
	} else {
		s = strings.Replace(s, "\\", string(filepath.Separator), -1)
	}
	return New(s)
}
