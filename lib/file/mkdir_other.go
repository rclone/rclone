//go:build !windows || go1.22

package file

import "os"

// MkdirAll just calls os.MkdirAll on non-Windows and with go1.22 or newer on Windows
func MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}
