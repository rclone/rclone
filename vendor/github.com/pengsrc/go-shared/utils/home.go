package utils

import (
	"os"
	"runtime"
)

// GetHome returns the home directory.
func GetHome() string {
	if runtime.GOOS == "windows" {
		home := os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		if home == "" {
			home = os.Getenv("USERPROFILE")
		}
	}
	return os.Getenv("HOME")
}
