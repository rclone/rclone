package fs

import (
	"fmt"
	"log"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// LogLevel describes rclone's logs.  These are a subset of the syslog log levels.
type LogLevel byte

// Log levels.  These are the syslog levels of which we only use a
// subset.
//
//    LOG_EMERG      system is unusable
//    LOG_ALERT      action must be taken immediately
//    LOG_CRIT       critical conditions
//    LOG_ERR        error conditions
//    LOG_WARNING    warning conditions
//    LOG_NOTICE     normal, but significant, condition
//    LOG_INFO       informational message
//    LOG_DEBUG      debug-level message
const (
	LogLevelEmergency LogLevel = iota
	LogLevelAlert
	LogLevelCritical
	LogLevelError // Error - can't be suppressed
	LogLevelWarning
	LogLevelNotice // Normal logging, -q suppresses
	LogLevelInfo   // Transfers, needs -v
	LogLevelDebug  // Debug level, needs -vv
)

var logLevelToString = []string{
	LogLevelEmergency: "EMERGENCY",
	LogLevelAlert:     "ALERT",
	LogLevelCritical:  "CRITICAL",
	LogLevelError:     "ERROR",
	LogLevelWarning:   "WARNING",
	LogLevelNotice:    "NOTICE",
	LogLevelInfo:      "INFO",
	LogLevelDebug:     "DEBUG",
}

// String turns a LogLevel into a string
func (l LogLevel) String() string {
	if l >= LogLevel(len(logLevelToString)) {
		return fmt.Sprintf("LogLevel(%d)", l)
	}
	return logLevelToString[l]
}

// Set a LogLevel
func (l *LogLevel) Set(s string) error {
	for n, name := range logLevelToString {
		if s != "" && name == s {
			*l = LogLevel(n)
			return nil
		}
	}
	return errors.Errorf("Unknown log level %q", s)
}

// Type of the value
func (l *LogLevel) Type() string {
	return "string"
}

// LogPrint sends the text to the logger of level
var LogPrint = func(level LogLevel, text string) {
	text = fmt.Sprintf("%-6s: %s", level, text)
	_ = log.Output(4, text)
}

// LogValueItem describes keyed item for a JSON log entry
type LogValueItem struct {
	key   string
	value interface{}
}

// LogValue should be used as an argument to any logging calls to
// augment the JSON output with more structured information.
//
// key is the dictionary parameter used to store value.
func LogValue(key string, value interface{}) LogValueItem {
	return LogValueItem{key: key, value: value}
}

// String returns an empty string so LogValueItem entries won't show
// in the textual representation of logs. They need to be put in so
// the number of parameters of the log call matches.
func (j LogValueItem) String() string {
	return ""
}

// LogPrintf produces a log string from the arguments passed in
func LogPrintf(level LogLevel, o interface{}, text string, args ...interface{}) {
	out := fmt.Sprintf(text, args...)

	if Config.UseJSONLog {
		fields := logrus.Fields{}
		if o != nil {
			fields = logrus.Fields{
				"object":     fmt.Sprintf("%+v", o),
				"objectType": fmt.Sprintf("%T", o),
			}
		}
		for _, arg := range args {
			if item, ok := arg.(LogValueItem); ok {
				fields[item.key] = item.value
			}
		}
		switch level {
		case LogLevelDebug:
			logrus.WithFields(fields).Debug(out)
		case LogLevelInfo:
			logrus.WithFields(fields).Info(out)
		case LogLevelNotice, LogLevelWarning:
			logrus.WithFields(fields).Warn(out)
		case LogLevelError:
			logrus.WithFields(fields).Error(out)
		case LogLevelCritical:
			logrus.WithFields(fields).Fatal(out)
		case LogLevelEmergency, LogLevelAlert:
			logrus.WithFields(fields).Panic(out)
		}
	} else {
		if o != nil {
			out = fmt.Sprintf("%v: %s", o, out)
		}
		LogPrint(level, out)
	}
}

// LogLevelPrintf writes logs at the given level
func LogLevelPrintf(level LogLevel, o interface{}, text string, args ...interface{}) {
	if Config.LogLevel >= level {
		LogPrintf(level, o, text, args...)
	}
}

// Errorf writes error log output for this Object or Fs.  It
// should always be seen by the user.
func Errorf(o interface{}, text string, args ...interface{}) {
	if Config.LogLevel >= LogLevelError {
		LogPrintf(LogLevelError, o, text, args...)
	}
}

// Logf writes log output for this Object or Fs.  This should be
// considered to be Info level logging.  It is the default level.  By
// default rclone should not log very much so only use this for
// important things the user should see.  The user can filter these
// out with the -q flag.
func Logf(o interface{}, text string, args ...interface{}) {
	if Config.LogLevel >= LogLevelNotice {
		LogPrintf(LogLevelNotice, o, text, args...)
	}
}

// Infof writes info on transfers for this Object or Fs.  Use this
// level for logging transfers, deletions and things which should
// appear with the -v flag.
func Infof(o interface{}, text string, args ...interface{}) {
	if Config.LogLevel >= LogLevelInfo {
		LogPrintf(LogLevelInfo, o, text, args...)
	}
}

// Debugf writes debugging output for this Object or Fs.  Use this for
// debug only.  The user must have to specify -vv to see this.
func Debugf(o interface{}, text string, args ...interface{}) {
	if Config.LogLevel >= LogLevelDebug {
		LogPrintf(LogLevelDebug, o, text, args...)
	}
}

// LogDirName returns an object for the logger, logging a root
// directory which would normally be "" as the Fs
func LogDirName(f Fs, dir string) interface{} {
	if dir != "" {
		return dir
	}
	return f
}
