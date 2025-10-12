package file

import "os"

// MkdirAll now just calls os.MkdirAll
func MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}
