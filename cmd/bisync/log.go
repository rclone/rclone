package bisync

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"

	"github.com/rclone/rclone/fs"
)

func (b *bisyncRun) indentf(tag, file, format string, args ...interface{}) {
	b.indent(tag, file, fmt.Sprintf(format, args...))
}

func (b *bisyncRun) indent(tag, file, msg string) {
	logf := fs.Infof
	switch {
	case tag == "ERROR":
		tag = ""
		logf = fs.Errorf
	case tag == "INFO":
		tag = ""
	case strings.HasPrefix(tag, "!"):
		tag = tag[1:]
		logf = fs.Logf
	}
	logf(nil, "- %-9s%-35s - %s", tag, msg, escapePath(file, false))
}

// escapePath will escape control characters in path.
// It won't quote just due to backslashes on Windows.
func escapePath(path string, forceQuotes bool) string {
	test := path
	if runtime.GOOS == "windows" {
		test = strings.ReplaceAll(path, "\\", "/")
	}
	if strconv.Quote(test) != `"`+test+`"` {
		return strconv.Quote(path)
	}
	if forceQuotes {
		return `"` + path + `"`
	}
	return path
}

func quotePath(path string) string {
	return escapePath(path, true)
}
