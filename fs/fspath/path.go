// Package fspath contains routines for fspath manipulation
package fspath

import (
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ncw/rclone/fs/driveletter"
)

// Matcher is a pattern to match an rclone URL
var Matcher = regexp.MustCompile(`^(:?[\w_ -]+):(.*)$`)

// Parse deconstructs a remote path into configName and fsPath
//
// If the path is a local path then configName will be returned as "".
//
// So "remote:path/to/dir" will return "remote", "path/to/dir"
// and "/path/to/local" will return ("", "/path/to/local")
//
// Note that this will turn \ into / in the fsPath on Windows
func Parse(path string) (configName, fsPath string) {
	parts := Matcher.FindStringSubmatch(path)
	configName, fsPath = "", path
	if parts != nil && !driveletter.IsDriveLetter(parts[1]) {
		configName, fsPath = parts[1], parts[2]
	}
	// change native directory separators to / if there are any
	fsPath = filepath.ToSlash(fsPath)
	return configName, fsPath
}

// Split splits a remote into a parent and a leaf
//
// if it returns leaf as an empty string then remote is a directory
//
// if it returns parent as an empty string then that means the current directory
//
// The returned values have the property that parent + leaf == remote
// (except under Windows where \ will be translated into /)
func Split(remote string) (parent string, leaf string) {
	remoteName, remotePath := Parse(remote)
	if remoteName != "" {
		remoteName += ":"
	}
	// Construct new remote name without last segment
	parent, leaf = path.Split(remotePath)
	return remoteName + parent, leaf
}

// JoinRootPath joins any number of path elements into a single path, adding a
// separating slash if necessary. The result is Cleaned; in particular,
// all empty strings are ignored.
// If the first non empty element has a leading "//" this is preserved.
func JoinRootPath(elem ...string) string {
	for i, e := range elem {
		if e != "" {
			if strings.HasPrefix(e, "//") {
				return "/" + path.Clean(strings.Join(elem[i:], "/"))
			}
			return path.Clean(strings.Join(elem[i:], "/"))
		}
	}
	return ""
}
