//go:build !windows

package file

// FindUnusedDriveLetter does nothing except on Windows.
func FindUnusedDriveLetter() (driveLetter uint8) {
	return 0
}
