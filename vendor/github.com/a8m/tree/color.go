package tree

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const Escape = "\x1b"
const (
	Reset int = 0
	// Not used, remove.
	Bold  int = 1
	Black int = iota + 28
	Red
	Green
	Yellow
	Blue
	Magenta
	Cyan
	White
)

// ANSIColor
func ANSIColor(node *Node, s string) string {
	var style string
	var mode = node.Mode()
	var ext = filepath.Ext(node.Name())
	switch {
	case contains([]string{".bat", ".btm", ".cmd", ".com", ".dll", ".exe"}, ext):
		style = "1;32"
	case contains([]string{".arj", ".bz2", ".deb", ".gz", ".lzh", ".rpm",
		".tar", ".taz", ".tb2", ".tbz2", ".tbz", ".tgz", ".tz", ".tz2", ".z",
		".zip", ".zoo"}, ext):
		style = "1;31"
	case contains([]string{".asf", ".avi", ".bmp", ".flac", ".gif", ".jpg",
		"jpeg", ".m2a", ".m2v", ".mov", ".mp3", ".mpeg", ".mpg", ".ogg", ".ppm",
		".rm", ".tga", ".tif", ".wav", ".wmv",
		".xbm", ".xpm"}, ext):
		style = "1;35"
	case node.IsDir() || mode&os.ModeDir != 0:
		style = "1;34"
	case mode&os.ModeNamedPipe != 0:
		style = "40;33"
	case mode&os.ModeSocket != 0:
		style = "40;1;35"
	case mode&os.ModeDevice != 0 || mode&os.ModeCharDevice != 0:
		style = "40;1;33"
	case mode&os.ModeSymlink != 0:
		if _, err := filepath.EvalSymlinks(node.path); err != nil {
			style = "40;1;31"
		} else {
			style = "1;36"
		}
	case mode&modeExecute != 0:
		style = "1;32"
	default:
		return s
	}
	return fmt.Sprintf("%s[%sm%s%s[%dm", Escape, style, s, Escape, Reset)
}

// case-insensitive contains helper
func contains(slice []string, str string) bool {
	for _, val := range slice {
		if val == strings.ToLower(str) {
			return true
		}
	}
	return false
}

// TODO: HTMLColor
