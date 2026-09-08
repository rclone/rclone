package vfscommon

import (
	"path"
	"path/filepath"
	"strings"
)

// NormalizePath returns the cleaned version of name for use in the VFS cache
//
// name should be a remote path not an osPath. It removes leading slashes
// and cleans the path using path.Clean.
func NormalizePath(name string) string {
	name = strings.Trim(name, "/")
	name = path.Clean(name)
	if name == "." || name == "/" {
		name = ""
	}
	return name
}

// OSFindParent returns the parent directory of name, or "" for the
// root for OS native paths.
func OSFindParent(name string) string {
	parent := filepath.Dir(name)
	if parent == "." || (len(parent) == 1 && parent[0] == filepath.Separator) {
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
