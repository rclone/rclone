package resolver

import (
	"path/filepath"
	"runtime"
	"strings"
)

func Key(s string) string {
	s = strings.Replace(s, "\\", "/", -1)
	return strings.ToLower(s)
}

func OsPath(s string) string {
	if runtime.GOOS == "windows" {
		s = strings.Replace(s, "/", string(filepath.Separator), -1)
	} else {
		s = strings.Replace(s, "\\", string(filepath.Separator), -1)
	}
	return s
}
