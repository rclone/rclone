// Syslog interface for Unix variants only

//go:build !windows && !nacl && !plan9

package log

import (
	"log"
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
func startSysLog() bool {
	facility, ok := syslogFacilityMap[Opt.SyslogFacility]
	if !ok {
		fs.Fatalf(nil, "Unknown syslog facility %q - man syslog for list", Opt.SyslogFacility)
	}
	Me := path.Base(os.Args[0])
	w, err := syslog.New(syslog.LOG_NOTICE|facility, Me)
	if err != nil {
		fs.Fatalf(nil, "Failed to start syslog: %v", err)
	}
	log.SetFlags(0)
	log.SetOutput(w)
	fs.LogOutput = func(level fs.LogLevel, text string) {
		switch level {
		case fs.LogLevelEmergency:
			_ = w.Emerg(text)
		case fs.LogLevelAlert:
			_ = w.Alert(text)
		case fs.LogLevelCritical:
			_ = w.Crit(text)
		case fs.LogLevelError:
			_ = w.Err(text)
		case fs.LogLevelWarning:
			_ = w.Warning(text)
		case fs.LogLevelNotice:
			_ = w.Notice(text)
		case fs.LogLevelInfo:
			_ = w.Info(text)
		case fs.LogLevelDebug:
			_ = w.Debug(text)
		}
	}
	return true
}
