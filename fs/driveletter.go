// +build !windows

package fs

// isDriveLetter returns a bool indicating whether name is a valid
// Windows drive letter
//
// On non windows platforms we don't have drive letters so we always
// return false
func isDriveLetter(name string) bool {
	return false
}
