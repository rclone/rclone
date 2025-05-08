// Windows event logging

//go:build windows

package log

import (
	"fmt"
	"log/slog"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/atexit"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc/eventlog"
)

const (
	errorID    = uint32(windows.ERROR_INTERNAL_ERROR)
	infoID     = uint32(windows.ERROR_SUCCESS)
	sourceName = "rclone"
)

var (
	windowsEventLog *eventlog.Log
)

func startWindowsEventLog(handler *OutputHandler) error {
	// Don't install Windows event log if it is disabled.
	if Opt.WindowsEventLogLevel == fs.LogLevelOff {
		return nil
	}

	// Install the event source - we don't care if this fails as Windows has sensible fallbacks.
	_ = eventlog.InstallAsEventCreate(sourceName, eventlog.Info|eventlog.Warning|eventlog.Error)

	// Open the event log
	// If sourceName didn't get registered then Windows will use "Application" instead which is fine.
	// Though in my tests it seemsed to use sourceName regardless.
	elog, err := eventlog.Open(sourceName)
	if err != nil {
		return fmt.Errorf("open event log: %w", err)
	}

	// Set the global for the handler
	windowsEventLog = elog

	// Close it on exit
	atexit.Register(func() {
		err := elog.Close()
		if err != nil {
			fs.Errorf(nil, "Failed to close Windows event log: %v", err)
		}
	})

	// Add additional JSON logging to the eventLog handler.
	handler.AddOutput(true, eventLog)

	fs.Infof(nil, "Logging to Windows event log at level %v", Opt.WindowsEventLogLevel)
	return nil
}

// We use levels ERROR, NOTICE, INFO, DEBUG
// Need to map to ERROR, WARNING, INFO
func eventLog(level slog.Level, text string) {
	// Check to see if this level is required
	if level < fs.LogLevelToSlog(Opt.WindowsEventLogLevel) {
		return
	}

	// Now log to windows eventLog
	switch level {
	case fs.SlogLevelEmergency, fs.SlogLevelAlert, fs.SlogLevelCritical, slog.LevelError:
		_ = windowsEventLog.Error(errorID, text)
	case slog.LevelWarn:
		_ = windowsEventLog.Warning(infoID, text)
	case fs.SlogLevelNotice, slog.LevelInfo, slog.LevelDebug:
		_ = windowsEventLog.Info(infoID, text)
	}
}
