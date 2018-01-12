// +build windows

package driveletter

// IsDriveLetter returns a bool indicating whether name is a valid
// Windows drive letter
func IsDriveLetter(name string) bool {
	if len(name) != 1 {
		return false
	}
	c := name[0]
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}
