package bisync

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/terminal"
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

	tag = Color(terminal.BlueFg, tag)
	msg = Color(terminal.MagentaFg, msg)
	file = Color(terminal.CyanFg, escapePath(file, false))
	logf(nil, "- %-18s%-43s - %s", tag, msg, file)
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

// Color handles terminal colors for bisync
func Color(style string, s string) string {
	terminal.Start()
	return style + s + terminal.Reset
}
