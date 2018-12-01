package packd

import (
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

var CommonSkipPrefixes = []string{".", "_", "node_modules", "vendor"}

// SkipWalker will walk the Walker and call the WalkFunc for files who's directories
// do no match any of the skipPrefixes. If no skipPrefixes are passed, then
// CommonSkipPrefixes is used
func SkipWalker(walker Walker, skipPrefixes []string, wf WalkFunc) error {
	if len(skipPrefixes) == 0 {
		skipPrefixes = append(skipPrefixes, CommonSkipPrefixes...)
	}
	return walker.Walk(func(path string, file File) error {
		fi, err := file.FileInfo()
		if err != nil {
			return errors.WithStack(err)
		}

		path = strings.Replace(path, "\\", "/", -1)

		parts := strings.Split(path, "/")
		if !fi.IsDir() {
			parts = parts[:len(parts)-1]
		}

		for _, base := range parts {
			if base != "." {
				for _, skip := range skipPrefixes {
					skip = strings.ToLower(skip)
					lbase := strings.ToLower(base)
					if strings.HasPrefix(lbase, skip) {
						return filepath.SkipDir
					}
				}
			}
		}
		return wf(path, file)
	})
}
