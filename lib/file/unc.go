//go:build !windows

package file

// UNCPath converts an absolute Windows path to a UNC long path.
//
// It does nothing on non windows platforms
func UNCPath(l string) string {
	return l
}
