// Package bilib provides common stuff for bisync and bisync_test
package bilib

import (
	"os"
	"regexp"
	"runtime"
	"strings"

	"github.com/rclone/rclone/fs"
)

// FsPath converts Fs to a suitable rclone argument
func FsPath(f fs.Fs) string {
	name, path, slash := f.Name(), f.Root(), "/"
	if name == "local" {
		slash = string(os.PathSeparator)
		if runtime.GOOS == "windows" {
			path = strings.ReplaceAll(path, "/", slash)
			path = strings.TrimPrefix(path, `\\?\`)
		}
	} else {
		path = name + ":" + path
	}
	if !strings.HasSuffix(path, slash) {
		path += slash
	}
	return path
}

// CanonicalPath converts a remote to a suitable base file name
func CanonicalPath(remote string) string {
	trimmed := strings.Trim(remote, `\/`)
	return nonCanonicalChars.ReplaceAllString(trimmed, "_")
}

var nonCanonicalChars = regexp.MustCompile(`[\s\\/:?*]`)

// SessionName makes a unique base name for the sync operation
func SessionName(fs1, fs2 fs.Fs) string {
	return CanonicalPath(FsPath(fs1)) + ".." + CanonicalPath(FsPath(fs2))
}
