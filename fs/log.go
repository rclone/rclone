// Logging for rclone

package fs

import (
	"fmt"
	"log"
	"os"
)

// LogLevel describes rclone's logs.  These are a subset of the syslog log levels.
type LogLevel byte

//go:generate stringer -type=LogLevel

// Log levels - a subset of the syslog logs
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

// Outside world interface

// DebugLogger - logs to Stdout
var DebugLogger = log.New(os.Stdout, "", log.LstdFlags)

// makeLog produces a log string from the arguments passed in
func makeLog(o interface{}, text string, args ...interface{}) string {
	out := fmt.Sprintf(text, args...)
	if o == nil {
		return out
	}
	return fmt.Sprintf("%v: %s", o, out)
}

// Errorf writes error log output for this Object or Fs.  It
// unconditionally logs a message regardless of Config.LogLevel
func Errorf(o interface{}, text string, args ...interface{}) {
	if Config.LogLevel >= LogLevelError {
		log.Print(makeLog(o, text, args...))
	}
}

// Logf writes log output for this Object or Fs.  This should be
// considered to be Info level logging.  It is the default level.  By
// default rclone should not log very much so only use this for
// important things the user should see.  The user can filter these
// out with the -q flag.
func Logf(o interface{}, text string, args ...interface{}) {
	if Config.LogLevel >= LogLevelNotice {
		log.Print(makeLog(o, text, args...))
	}
}

// Infof writes info on transfers for this Object or Fs.  Use this
// level for logging transfers, deletions and things which should
// appear with the -v flag.
func Infof(o interface{}, text string, args ...interface{}) {
	if Config.LogLevel >= LogLevelInfo {
		DebugLogger.Print(makeLog(o, text, args...))
	}
}

// Debugf writes debugging output for this Object or Fs.  Use this for
// debug only.  The user must have to specify -vv to see this.
func Debugf(o interface{}, text string, args ...interface{}) {
	if Config.LogLevel >= LogLevelDebug {
		DebugLogger.Print(makeLog(o, text, args...))
	}
}
