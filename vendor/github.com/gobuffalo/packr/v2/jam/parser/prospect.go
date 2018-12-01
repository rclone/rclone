package parser

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/gobuffalo/packr/v2/file/resolver"
	"github.com/gobuffalo/packr/v2/plog"
)

var DefaultIgnoredFolders = []string{".", "_", "vendor", "node_modules", "_fixtures"}

func IsProspect(path string, ignore ...string) (status bool) {
	// plog.Debug("parser", "IsProspect", "path", path, "ignore", ignore)
	defer func() {
		if status {
			plog.Debug("parser", "IsProspect (TRUE)", "path", path, "status", status)
		}
	}()
	if path == "." {
		return true
	}

	ext := filepath.Ext(path)
	dir := filepath.Dir(path)

	fi, _ := os.Stat(path)
	if fi != nil {
		if fi.IsDir() {
			dir = filepath.Base(path)
		} else {
			if len(ext) > 0 {
				dir = filepath.Base(filepath.Dir(path))
			}
		}
	}

	path = strings.ToLower(path)
	dir = strings.ToLower(dir)

	if strings.HasSuffix(path, "-packr.go") {
		return false
	}

	if strings.HasSuffix(path, "_test.go") {
		return false
	}

	ignore = append(ignore, DefaultIgnoredFolders...)
	for i, x := range ignore {
		ignore[i] = strings.TrimSpace(strings.ToLower(x))
	}

	parts := strings.Split(resolver.OsPath(path), string(filepath.Separator))
	if len(parts) == 0 {
		return false
	}

	for _, i := range ignore {
		for _, p := range parts {
			if strings.HasPrefix(p, i) {
				return false
			}
		}
	}

	un := filepath.Base(path)
	if len(ext) != 0 {
		un = filepath.Base(filepath.Dir(path))
	}
	if strings.HasPrefix(un, "_") {
		return false
	}

	return ext == ".go"
}
