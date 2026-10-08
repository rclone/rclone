// Package sanitize cleans untrusted input before use.
//
// The names handled here are rclone remote paths, which use "/" as
// the only path separator. A "\" is an ordinary character in a remote
// path and is kept as such - it is up to each backend to make names
// safe for its own storage (the local backend, for example, encodes
// it on Windows and rejects any path which escapes its root). As
// defence in depth Path does however refuse names in which "\" would
// form a ".." component if it were a separator.
package sanitize

import (
	"fmt"
	"path"
	"slices"
	"strings"
)

// Leaf checks that the untrusted name is safe to use as a single path
// component (a directory entry leaf), such as a name read from an
// archive's directory listing.
//
// It returns an error if the name is empty, ".", "..", or contains a
// "/". Such a name would otherwise fabricate hierarchy or escape its
// directory when joined onto a path.
func Leaf(name string) error {
	switch name {
	case "", ".", "..":
		return fmt.Errorf("unsafe path component %q", name)
	}
	if strings.Contains(name, "/") {
		return fmt.Errorf("path component %q contains a \"/\"", name)
	}
	return nil
}

// Path sanitizes the untrusted "/"-separated path name, such as an
// archive entry name, so that it is safe to use as an rclone remote
// path relative to some root.
//
// It returns the name cleaned with path.Clean and with any leading
// and trailing "/" removed, or "" if the name refers to the root
// directory (e.g. "", "/" or "./").
//
// It returns an error for any name with a ".." path component, which
// would otherwise escape the root when joined onto it (a path
// traversal, or "Zip Slip", attack). Both "/" and "\" are treated as
// separators when looking for ".." components.
func Path(name string) (string, error) {
	isSeparator := func(r rune) bool { return r == '/' || r == '\\' }
	if slices.Contains(strings.FieldsFunc(name, isSeparator), "..") {
		return "", fmt.Errorf("path %q has a %q component", name, "..")
	}
	cleaned := strings.Trim(path.Clean(name), "/")
	if cleaned == "." {
		return "", nil
	}
	return cleaned, nil
}
