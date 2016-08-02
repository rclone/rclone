// +build linux

package local

import (
	"github.com/davecheney/xattr"
)

// listxattr lists the extended attributes on the file
func listxattr(path string) ([]string, error) {
	return xattr.Listxattr(path)
}

// getxattr returns a specific attribute on a file
func getxattr(path, name string) ([]byte, error) {
	return xattr.Getxattr(path, name)
}

// setxattr sets a specific extended attribute on a file
func setxattr(path, name string, data []byte) error {
	return xattr.Setxattr(path, name, data)
}
