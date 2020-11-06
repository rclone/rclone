//+build windows

package file

import (
	"os"
)

// FindUnusedDriveLetter searches mounted drive list on the system
// (starting from Z: and ending at D:) for unused drive letter.
// Returns the letter found (like 'Z') or zero value.
func FindUnusedDriveLetter() (driveLetter uint8) {
	// Do not use A: and B:, because they are reserved for floppy drive.
	// Do not use C:, because it is normally used for main drive.
	for l := uint8('Z'); l >= uint8('D'); l-- {
		_, err := os.Stat(string(l) + ":" + string(os.PathSeparator))
		if os.IsNotExist(err) {
			return l
		}
	}
	return 0
}
