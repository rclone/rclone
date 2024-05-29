//go:build !windows

// Package driveletter returns whether a name is a valid drive letter
package driveletter

// IsDriveLetter returns a bool indicating whether name is a valid
// Windows drive letter
//
// On non windows platforms we don't have drive letters so we always
// return false
func IsDriveLetter(name string) bool {
	return false
}
