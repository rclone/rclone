//go:build windows && !go1.22

package file

import (
	"os"
	"path/filepath"
	"syscall"
)

// MkdirAll creates a directory named path, along with any necessary parents.
//
// Improves os.MkdirAll by avoiding trying to create a folder `\\?` when the
// volume of a given extended length path does not exist, and `\\?\UNC` when
// a network host name does not exist.
//
// Based on source code from golang's os.MkdirAll
// (https://github.com/golang/go/blob/master/src/os/path.go)
func MkdirAll(path string, perm os.FileMode) error {
	// Fast path: if we can tell whether path is a directory or file, stop with success or error.
	dir, err := os.Stat(path)
	if err == nil {
		if dir.IsDir() {
			return nil
		}
		return &os.PathError{
			Op:   "mkdir",
			Path: path,
			Err:  syscall.ENOTDIR,
		}
	}

	// Slow path: make sure parent exists and then call Mkdir for path.
	i := len(path)
	for i > 0 && os.IsPathSeparator(path[i-1]) { // Skip trailing path separator.
		i--
	}
	if i > 0 {
		path = path[:i]
		if path == filepath.VolumeName(path) {
			// Make reference to a drive's root directory include the trailing slash.
			// In extended-length form without trailing slash ("\\?\C:"), os.Stat
			// and os.Mkdir both fails. With trailing slash ("\\?\C:\") works,
			// and regular paths with or without it ("C:" and "C:\") both works.
			path += string(os.PathSeparator)
		} else {
			// See if there is a parent to be created first.
			// Not when path refer to a drive's root directory, because we don't
			// want to return noninformative error trying to create \\?.
			j := i
			for j > 0 && !os.IsPathSeparator(path[j-1]) { // Scan backward over element.
				j--
			}
			if j > 1 {
				if path[:j-1] != `\\?\UNC` && path[:j-1] != `\\?` {
					// Create parent.
					err = MkdirAll(path[:j-1], perm)
					if err != nil {
						return err
					}
				}
			}
		}
	}

	// Parent now exists; invoke Mkdir and use its result.
	err = os.Mkdir(path, perm)
	if err != nil {
		// Handle arguments like "foo/." by
		// double-checking that directory doesn't exist.
		dir, err1 := os.Lstat(path)
		if err1 == nil && dir.IsDir() {
			return nil
		}
		return err
	}
	return nil
}
