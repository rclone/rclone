//go:build !windows
// +build !windows

package file

import "os"

// MkdirAll just calls os.MkdirAll on non-Windows.
func MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}
