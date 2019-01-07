// Package file provides a version of os.OpenFile, the handles of
// which can be renamed and deleted under Windows.
package file

import "os"

// Open opens the named file for reading. If successful, methods on
// the returned file can be used for reading; the associated file
// descriptor has mode O_RDONLY.
// If there is an error, it will be of type *PathError.
func Open(name string) (*os.File, error) {
	return OpenFile(name, os.O_RDONLY, 0)
}

// Create creates the named file with mode 0666 (before umask), truncating
// it if it already exists. If successful, methods on the returned
// File can be used for I/O; the associated file descriptor has mode
// O_RDWR.
// If there is an error, it will be of type *PathError.
func Create(name string) (*os.File, error) {
	return OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
}
