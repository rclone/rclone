//go:build !windows

package file

import "os"

// OpenFile is the generalized open call; most users will use Open or Create
// instead. It opens the named file with specified flag (O_RDONLY etc.) and
// perm (before umask), if applicable. If successful, methods on the returned
// File can be used for I/O. If there is an error, it will be of type
// *PathError.
//
// Under both Unix and Windows this will allow open files to be
// renamed and or deleted.
var OpenFile = os.OpenFile

// Rename renames (moves) oldpath to newpath, replacing newpath if it
// exists.
//
// On Unix this is os.Rename. On Windows it is a rename which can replace
// a destination which has open handles - see file_windows.go.
func Rename(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}

// IsReserved checks if path contains a reserved name
func IsReserved(path string) error {
	return nil
}
