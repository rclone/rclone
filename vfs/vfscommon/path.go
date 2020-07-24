package vfscommon

import (
	"path"
	"path/filepath"
)

// OsFindParent returns the parent directory of name, or "" for the
// root for OS native paths.
func OsFindParent(name string) string {
	parent := filepath.Dir(name)
	if parent == "." || parent == "/" {
		parent = ""
	}
	return parent
}

// FindParent returns the parent directory of name, or "" for the root
// for rclone paths.
func FindParent(name string) string {
	parent := path.Dir(name)
	if parent == "." || parent == "/" {
		parent = ""
	}
	return parent
}
