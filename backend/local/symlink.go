//go:build !windows && !plan9 && !js

package local

import (
	"os"
	"syscall"
)

// isCircularSymlinkError checks if the current error code is because of a circular symlink
func isCircularSymlinkError(err error) bool {
	if err != nil {
		if newerr, ok := err.(*os.PathError); ok {
			if errcode, ok := newerr.Err.(syscall.Errno); ok {
				if errcode == syscall.ELOOP {
					return true
				}
			}
		}
	}
	return false
}
