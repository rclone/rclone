// Systemd interface for Unix variants only

//go:build unix

package log

import (
	"log/slog"

	"github.com/coreos/go-systemd/v22/journal"
	"github.com/rclone/rclone/fs"
)

// Enables systemd logs if configured or if auto-detected
func startSystemdLog(handler *OutputHandler) bool {
	handler.clearFormatFlags(logFormatDate | logFormatTime | logFormatMicroseconds | logFormatUTC | logFormatLongFile | logFormatShortFile | logFormatPid)
	handler.setFormatFlags(logFormatNoLevel)
	handler.SetOutput(func(level slog.Level, text string) {
		_ = journal.Print(slogLevelToSystemdPriority(level), "%-6s: %s\n", level, text)
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

func slogLevelToSystemdPriority(l slog.Level) journal.Priority {
	prio, ok := slogLevelToSystemdPrefix[l]
	if !ok {
		return journal.PriInfo
	}
	return prio
}

func isJournalStream() bool {
	if usingJournald, _ := journal.StderrIsJournalStream(); usingJournald {
		return true
	}
	return false
}
