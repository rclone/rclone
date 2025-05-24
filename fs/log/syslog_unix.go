// Syslog interface for Unix variants only

//go:build !windows && !nacl && !plan9

package log

import (
	"log/slog"
	"log/syslog"
	"os"
	"path"

	"github.com/rclone/rclone/fs"
)

var (
	syslogFacilityMap = map[string]syslog.Priority{
		"KERN":     syslog.LOG_KERN,
		"USER":     syslog.LOG_USER,
		"MAIL":     syslog.LOG_MAIL,
		"DAEMON":   syslog.LOG_DAEMON,
		"AUTH":     syslog.LOG_AUTH,
		"SYSLOG":   syslog.LOG_SYSLOG,
		"LPR":      syslog.LOG_LPR,
		"NEWS":     syslog.LOG_NEWS,
		"UUCP":     syslog.LOG_UUCP,
		"CRON":     syslog.LOG_CRON,
		"AUTHPRIV": syslog.LOG_AUTHPRIV,
		"FTP":      syslog.LOG_FTP,
		"LOCAL0":   syslog.LOG_LOCAL0,
		"LOCAL1":   syslog.LOG_LOCAL1,
		"LOCAL2":   syslog.LOG_LOCAL2,
		"LOCAL3":   syslog.LOG_LOCAL3,
		"LOCAL4":   syslog.LOG_LOCAL4,
		"LOCAL5":   syslog.LOG_LOCAL5,
		"LOCAL6":   syslog.LOG_LOCAL6,
		"LOCAL7":   syslog.LOG_LOCAL7,
	}
)

// Starts syslog
func startSysLog(handler *OutputHandler) bool {
	facility, ok := syslogFacilityMap[Opt.SyslogFacility]
	if !ok {
		fs.Fatalf(nil, "Unknown syslog facility %q - man syslog for list", Opt.SyslogFacility)
	}
	Me := path.Base(os.Args[0])
	w, err := syslog.New(syslog.LOG_NOTICE|facility, Me)
	if err != nil {
		fs.Fatalf(nil, "Failed to start syslog: %v", err)
	}
	handler.clearFormatFlags(logFormatDate | logFormatTime | logFormatMicroseconds | logFormatUTC | logFormatLongFile | logFormatShortFile | logFormatPid)
	handler.setFormatFlags(logFormatNoLevel)
	handler.SetOutput(func(level slog.Level, text string) {
		switch level {
		case fs.SlogLevelEmergency:
			_ = w.Emerg(text)
		case fs.SlogLevelAlert:
			_ = w.Alert(text)
		case fs.SlogLevelCritical:
			_ = w.Crit(text)
		case slog.LevelError:
			_ = w.Err(text)
		case slog.LevelWarn:
			_ = w.Warning(text)
		case fs.SlogLevelNotice:
			_ = w.Notice(text)
		case slog.LevelInfo:
			_ = w.Info(text)
		case slog.LevelDebug:
			_ = w.Debug(text)
		}
	})
	return true
}
