package bisync

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strconv"
	"strings"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/encoder"
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

	if b.opt.DryRun {
		logf = fs.Logf
	}

	if tag == "Path1" {
		tag = Color(terminal.CyanFg, "Path1")
	} else {
		tag = Color(terminal.BlueFg, tag)
	}
	msg = Color(terminal.MagentaFg, msg)
	msg = strings.ReplaceAll(msg, "Queue copy to", Color(terminal.GreenFg, "Queue copy to"))
	msg = strings.ReplaceAll(msg, "Queue delete", Color(terminal.RedFg, "Queue delete"))
	file = Color(terminal.CyanFg, escapePath(file, false))
	logf(nil, "- %-18s%-43s - %s", tag, msg, file)
}

// escapePath will escape control characters in path.
// It won't quote just due to backslashes on Windows.
func escapePath(path string, forceQuotes bool) string {
	path = encode(path)
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

var Colors bool // Colors controls whether terminal colors are enabled

// Color handles terminal colors for bisync
func Color(style string, s string) string {
	if !Colors {
		return s
	}
	terminal.Start()
	return style + s + terminal.Reset
}

func encode(s string) string {
	return encoder.OS.ToStandardPath(encoder.OS.FromStandardPath(s))
}

// prettyprint formats JSON for improved readability in debug logs
func prettyprint(in any, label string, level fs.LogLevel) {
	inBytes, err := json.MarshalIndent(in, "", "\t")
	if err != nil {
		fs.Debugf(nil, "failed to marshal input: %v", err)
	}
	if level == fs.LogLevelDebug {
		fs.Debugf(nil, "%s: \n%s\n", label, string(inBytes))
	} else if level == fs.LogLevelInfo {
		fs.Infof(nil, "%s: \n%s\n", label, string(inBytes))
	}
}
