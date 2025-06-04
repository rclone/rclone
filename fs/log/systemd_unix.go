// Systemd interface for Unix variants only

//go:build unix

package log

import (
	"fmt"
	"log"
	"log/slog"
	"strconv"

	"github.com/coreos/go-systemd/v22/journal"
	"github.com/rclone/rclone/fs"
)

// Enables systemd logs if configured or if auto-detected
func startSystemdLog(handler *OutputHandler) bool {
	handler.clearFormatFlags(logFormatDate | logFormatTime | logFormatMicroseconds | logFormatUTC | logFormatLongFile | logFormatShortFile | logFormatPid)
	handler.setFormatFlags(logFormatNoLevel)
	// TODO: Use the native journal.Print approach rather than a custom implementation
	handler.SetOutput(func(level slog.Level, text string) {
		text = fmt.Sprintf("<%s>%-6s: %s", systemdLogPrefix(level), level, text)
		_ = log.Output(4, text)
	})
	return true
}

var slogLevelToSystemdPrefix = map[slog.Level]journal.Priority{
	fs.SlogLevelEmergency: journal.PriEmerg,
	fs.SlogLevelAlert:     journal.PriAlert,
	fs.SlogLevelCritical:  journal.PriCrit,
	slog.LevelError:       journal.PriErr,
	slog.LevelWarn:        journal.PriWarning,
	fs.SlogLevelNotice:    journal.PriNotice,
	slog.LevelInfo:        journal.PriInfo,
	slog.LevelDebug:       journal.PriDebug,
}

func systemdLogPrefix(l slog.Level) string {
	prio, ok := slogLevelToSystemdPrefix[l]
	if !ok {
		return ""
	}
	return strconv.Itoa(int(prio))
}

func isJournalStream() bool {
	if usingJournald, _ := journal.StderrIsJournalStream(); usingJournald {
		return true
	}
	return false
}
