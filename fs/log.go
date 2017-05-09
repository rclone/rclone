// Logging for rclone

package fs

import (
	"fmt"
	"log"
	"os"
	"reflect"
	"runtime"
	"strings"
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

// Flags
var (
	logFile        = StringP("log-file", "", "", "Log everything to this file")
	useSyslog      = BoolP("syslog", "", false, "Use Syslog for logging")
	syslogFacility = StringP("syslog-facility", "", "DAEMON", "Facility for syslog, eg KERN,USER,...")
)

// logPrint sends the text to the logger of level
var logPrint = func(level LogLevel, text string) {
	text = fmt.Sprintf("%-6s: %s", level, text)
	log.Print(text)
}

// logPrintf produces a log string from the arguments passed in
func logPrintf(level LogLevel, o interface{}, text string, args ...interface{}) {
	out := fmt.Sprintf(text, args...)
	if o != nil {
		out = fmt.Sprintf("%v: %s", o, out)
	}
	logPrint(level, out)
}

// Errorf writes error log output for this Object or Fs.  It
// should always be seen by the user.
func Errorf(o interface{}, text string, args ...interface{}) {
	if Config.LogLevel >= LogLevelError {
		logPrintf(LogLevelError, o, text, args...)
	}
}

// Logf writes log output for this Object or Fs.  This should be
// considered to be Info level logging.  It is the default level.  By
// default rclone should not log very much so only use this for
// important things the user should see.  The user can filter these
// out with the -q flag.
func Logf(o interface{}, text string, args ...interface{}) {
	if Config.LogLevel >= LogLevelNotice {
		logPrintf(LogLevelNotice, o, text, args...)
	}
}

// Infof writes info on transfers for this Object or Fs.  Use this
// level for logging transfers, deletions and things which should
// appear with the -v flag.
func Infof(o interface{}, text string, args ...interface{}) {
	if Config.LogLevel >= LogLevelInfo {
		logPrintf(LogLevelInfo, o, text, args...)
	}
}

// Debugf writes debugging output for this Object or Fs.  Use this for
// debug only.  The user must have to specify -vv to see this.
func Debugf(o interface{}, text string, args ...interface{}) {
	if Config.LogLevel >= LogLevelDebug {
		logPrintf(LogLevelDebug, o, text, args...)
	}
}

// fnName returns the name of the calling +2 function
func fnName() string {
	pc, _, _, ok := runtime.Caller(2)
	name := "*Unknown*"
	if ok {
		name = runtime.FuncForPC(pc).Name()
		dot := strings.LastIndex(name, ".")
		if dot >= 0 {
			name = name[dot+1:]
		}
	}
	return name
}

// Trace debugs the entry and exit of the calling function
//
// It is designed to be used in a defer statement so it returns a
// function that logs the exit parameters.
//
// Any pointers in the exit function will be dereferenced
func Trace(o interface{}, format string, a ...interface{}) func(string, ...interface{}) {
	if Config.LogLevel < LogLevelDebug {
		return func(format string, a ...interface{}) {}
	}
	name := fnName()
	logPrintf(LogLevelDebug, o, name+": "+format, a...)
	return func(format string, a ...interface{}) {
		for i := range a {
			// read the values of the pointed to items
			typ := reflect.TypeOf(a[i])
			if typ.Kind() == reflect.Ptr {
				value := reflect.ValueOf(a[i])
				if value.IsNil() {
					a[i] = nil
				} else {
					pointedToValue := reflect.Indirect(value)
					a[i] = pointedToValue.Interface()
				}
			}
		}
		logPrintf(LogLevelDebug, o, ">"+name+": "+format, a...)
	}
}

// InitLogging start the logging as per the command line flags
func InitLogging() {
	// Log file output
	if *logFile != "" {
		f, err := os.OpenFile(*logFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0640)
		if err != nil {
			log.Fatalf("Failed to open log file: %v", err)
		}
		_, err = f.Seek(0, os.SEEK_END)
		if err != nil {
			Errorf(nil, "Failed to seek log file to end: %v", err)
		}
		log.SetOutput(f)
		redirectStderr(f)
	}

	// Syslog output
	if *useSyslog {
		if *logFile != "" {
			log.Fatalf("Can't use --syslog and --log-file together")
		}
		startSysLog()
	}
}
