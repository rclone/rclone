// Systemd interface for Unix variants only

//go:build !windows && !nacl && !plan9
// +build !windows,!nacl,!plan9

package log

import (
	"fmt"
	"log"
	"strings"

	sysdjournald "github.com/iguanesolutions/go-systemd/v5/journald"
	"github.com/rclone/rclone/fs"
)

// Enables systemd logs if configured or if auto detected
func startSystemdLog() bool {
	flagsStr := "," + Opt.Format + ","
	var flags int
	if strings.Contains(flagsStr, ",longfile,") {
		flags |= log.Llongfile
	}
	if strings.Contains(flagsStr, ",shortfile,") {
		flags |= log.Lshortfile
	}
	log.SetFlags(flags)
	fs.LogPrint = func(level fs.LogLevel, text string) {
		text = fmt.Sprintf("%s%-6s: %s", systemdLogPrefix(level), level, text)
		_ = log.Output(4, text)
	}
	return true
}

var logLevelToSystemdPrefix = []string{
	fs.LogLevelEmergency: sysdjournald.EmergPrefix,
	fs.LogLevelAlert:     sysdjournald.AlertPrefix,
	fs.LogLevelCritical:  sysdjournald.CritPrefix,
	fs.LogLevelError:     sysdjournald.ErrPrefix,
	fs.LogLevelWarning:   sysdjournald.WarningPrefix,
	fs.LogLevelNotice:    sysdjournald.NoticePrefix,
	fs.LogLevelInfo:      sysdjournald.InfoPrefix,
	fs.LogLevelDebug:     sysdjournald.DebugPrefix,
}

func systemdLogPrefix(l fs.LogLevel) string {
	if l >= fs.LogLevel(len(logLevelToSystemdPrefix)) {
		return ""
	}
	return logLevelToSystemdPrefix[l]
}
