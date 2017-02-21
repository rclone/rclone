// +build !linux

package local

import (
	"github.com/pkg/errors"
)

// listxattr lists the extended attributes on the file
func listxattr(path string) ([]string, error) {
	return []string{}, errors.New("extended attributes not supported")
}

// getxattr returns a specific attribute on a file
func getxattr(path, name string) ([]byte, error) {
	return []byte{}, errors.New("extended attributes not supported")
}

// setxattr sets a specific extended attribute on a file
func setxattr(path, name string, data []byte) error {
	return errors.New("extended attributes not supported")
}
