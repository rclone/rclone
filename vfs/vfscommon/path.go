package vfscommon

import "path/filepath"

// FindParent returns the parent directory of name, or "" for the root
func FindParent(name string) string {
	parent := filepath.Dir(name)
	if parent == "." || parent == "/" {
		parent = ""
	}
	return parent
}
