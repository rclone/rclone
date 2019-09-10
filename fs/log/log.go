// Package log provides logging for rclone
package log

import (
	"io"
	"log"
	"os"
	"reflect"
	"runtime"
	"strings"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/flags"
)

// Flags
var (
	logFile        = flags.StringP("log-file", "", "", "Log everything to this file")
	logFormat      = flags.StringP("log-format", "", "date,time", "Comma separated list of log format options")
	useSyslog      = flags.BoolP("syslog", "", false, "Use Syslog for logging")
	syslogFacility = flags.StringP("syslog-facility", "", "DAEMON", "Facility for syslog, eg KERN,USER,...")
)

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
	if fs.Config.LogLevel < fs.LogLevelDebug {
		return func(format string, a ...interface{}) {}
	}
	name := fnName()
	fs.LogPrintf(fs.LogLevelDebug, o, name+": "+format, a...)
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
		fs.LogPrintf(fs.LogLevelDebug, o, ">"+name+": "+format, a...)
	}
}

// InitLogging start the logging as per the command line flags
func InitLogging() {
	flagsStr := "," + *logFormat + ","
	var flags int
	if strings.Contains(flagsStr, ",date,") {
		flags |= log.Ldate
	}
	if strings.Contains(flagsStr, ",time,") {
		flags |= log.Ltime
	}
	if strings.Contains(flagsStr, ",microseconds,") {
		flags |= log.Lmicroseconds
	}
	if strings.Contains(flagsStr, ",longfile,") {
		flags |= log.Llongfile
	}
	if strings.Contains(flagsStr, ",shortfile,") {
		flags |= log.Lshortfile
	}
	if strings.Contains(flagsStr, ",UTC,") {
		flags |= log.LUTC
	}
	log.SetFlags(flags)

	// Log file output
	if *logFile != "" {
		f, err := os.OpenFile(*logFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0640)
		if err != nil {
			log.Fatalf("Failed to open log file: %v", err)
		}
		_, err = f.Seek(0, io.SeekEnd)
		if err != nil {
			fs.Errorf(nil, "Failed to seek log file to end: %v", err)
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

// Redirected returns true if the log has been redirected from stdout
func Redirected() bool {
	return *useSyslog || *logFile != ""
}
