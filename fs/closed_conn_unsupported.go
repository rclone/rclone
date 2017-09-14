// +build plan9

package fs

// isClosedConnErrorPlatform reports whether err is an error from use
// of a closed network connection using platform specific error codes.
func isClosedConnErrorPlatform(err error) bool {
	return false
}
