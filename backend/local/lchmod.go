//go:build windows || plan9 || js || linux

package local

import "os"

const haveLChmod = false

// lChmod changes the mode of the named file to mode. If the file is a symbolic
// link, it changes the link, not the target. If there is an error,
// it will be of type *PathError.
func lChmod(name string, mode os.FileMode) error {
	// Can't do this safely on this OS - chmoding a symlink always
	// changes the destination.
	return nil
}
