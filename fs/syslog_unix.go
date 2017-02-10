// Syslog interface for Unix variants only

// +build !windows,!nacl,!plan9

package fs

import (
	"log"
	"log/syslog"
	"os"
	"path"
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
	}
)

// Starts syslog
func startSysLog() bool {
	facility, ok := syslogFacilityMap[*syslogFacility]
	if !ok {
		log.Fatalf("Unknown syslog facility %q - man syslog for list", *syslogFacility)
	}
	Me := path.Base(os.Args[0])
	w, err := syslog.New(syslog.LOG_NOTICE|facility, Me)
	if err != nil {
		log.Fatalf("Failed to start syslog: %v", err)
	}
	log.SetFlags(0)
	log.SetOutput(w)
	logPrint = func(level LogLevel, text string) {
		switch level {
		case LogLevelEmergency:
			_ = w.Emerg(text)
		case LogLevelAlert:
			_ = w.Alert(text)
		case LogLevelCritical:
			_ = w.Crit(text)
		case LogLevelError:
			_ = w.Err(text)
		case LogLevelWarning:
			_ = w.Warning(text)
		case LogLevelNotice:
			_ = w.Notice(text)
		case LogLevelInfo:
			_ = w.Info(text)
		case LogLevelDebug:
			_ = w.Debug(text)
		}
	}
	return true
}
