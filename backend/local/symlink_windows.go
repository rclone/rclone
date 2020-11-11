// +build windows plan9 js

package local

import (
	"strings"
)

// isCircularSymlinkError checks if the current error code is because of a circular symlink
func isCircularSymlinkError(err error) bool {
	if err != nil {
		if strings.Contains(err.Error(), "The name of the file cannot be resolved by the system") {
			return true
		}
	}
	return false
}
