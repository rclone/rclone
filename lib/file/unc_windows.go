//go:build windows

package file

import (
	"regexp"
	"strings"
)

// Pattern to match a windows absolute path: "c:\" and similar
var isAbsWinDrive = regexp.MustCompile(`^[a-zA-Z]\:\\`)

// UNCPath converts an absolute Windows path to a UNC long path.
//
// It does nothing on non windows platforms
func UNCPath(l string) string {
	// If prefix is "\\", we already have a UNC path or server.
	if strings.HasPrefix(l, `\\`) {
		// If already long path, just keep it
		if strings.HasPrefix(l, `\\?\`) {
			return l
		}

		// Trim "\\" from path and add UNC prefix.
		return `\\?\UNC\` + strings.TrimPrefix(l, `\\`)
	}
	if isAbsWinDrive.MatchString(l) {
		return `\\?\` + l
	}
	return l
}
