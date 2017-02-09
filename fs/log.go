// Logging for rclone

package fs

import (
	"fmt"
	"log"
	"os"
)

// LogLevel describes rclone's logs.  These are a subset of the syslog log levels.
type LogLevel byte

// Log levels - a subset of the syslog logs
const (
	LogLevelEmergency = iota
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
// unconditionally logs a message regardless of Config.Quiet or
// Config.Verbose.
func Errorf(o interface{}, text string, args ...interface{}) {
	log.Print(makeLog(o, text, args...))
}

// Logf writes log output for this Object or Fs.  This should be
// considered to be Info level logging.
func Logf(o interface{}, text string, args ...interface{}) {
	if !Config.Quiet {
		log.Print(makeLog(o, text, args...))
	}
}

// Infof writes info on transfers for this Object or Fs
func Infof(o interface{}, text string, args ...interface{}) {
	if Config.Verbose {
		DebugLogger.Print(makeLog(o, text, args...))
	}
}

// Debugf writes debugging output for this Object or Fs
func Debugf(o interface{}, text string, args ...interface{}) {
	if Config.Verbose {
		DebugLogger.Print(makeLog(o, text, args...))
	}
}
