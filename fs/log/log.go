// Package log provides logging for rclone
package log

import (
	"context"
	"fmt"
	"io"
	"os"
	"reflect"
	"runtime"
	"strings"

	"github.com/rclone/rclone/fs"
)

// OptionsInfo descripts the Options in use
var OptionsInfo = fs.Options{{
	Name:    "log_file",
	Default: "",
	Help:    "Log everything to this file",
	Groups:  "Logging",
}, {
	Name:    "log_format",
	Default: logFormatDate | logFormatTime,
	Help:    "Comma separated list of log format options",
	Groups:  "Logging",
}, {
	Name:    "syslog",
	Default: false,
	Help:    "Use Syslog for logging",
	Groups:  "Logging",
}, {
	Name:    "syslog_facility",
	Default: "DAEMON",
	Help:    "Facility for syslog, e.g. KERN,USER",
	Groups:  "Logging",
}, {
	Name:    "log_systemd",
	Default: false,
	Help:    "Activate systemd integration for the logger",
	Groups:  "Logging",
}, {
	Name:    "windows_event_log_level",
	Default: fs.LogLevelOff,
	Help:    "Windows Event Log level DEBUG|INFO|NOTICE|ERROR|OFF",
	Groups:  "Logging",
	Hide: func() fs.OptionVisibility {
		if runtime.GOOS == "windows" {
			return 0
		}
		return fs.OptionHideBoth
	}(),
}}

// Options contains options for controlling the logging
type Options struct {
	File                 string      `config:"log_file"`        // Log everything to this file
	Format               logFormat   `config:"log_format"`      // Comma separated list of log format options
	UseSyslog            bool        `config:"syslog"`          // Use Syslog for logging
	SyslogFacility       string      `config:"syslog_facility"` // Facility for syslog, e.g. KERN,USER,...
	LogSystemdSupport    bool        `config:"log_systemd"`     // set if using systemd logging
	WindowsEventLogLevel fs.LogLevel `config:"windows_event_log_level"`
}

func init() {
	fs.RegisterGlobalOptions(fs.OptionsInfo{Name: "log", Opt: &Opt, Options: OptionsInfo})
}

// Opt is the options for the logger
var Opt Options

// enum for the log format
type logFormat = fs.Bits[logFormatChoices]

const (
	logFormatDate logFormat = 1 << iota
	logFormatTime
	logFormatMicroseconds
	logFormatUTC
	logFormatLongFile
	logFormatShortFile
	logFormatPid
	logFormatNoLevel
	logFormatJSON
)

type logFormatChoices struct{}

func (logFormatChoices) Choices() []fs.BitsChoicesInfo {
	return []fs.BitsChoicesInfo{
		{Bit: uint64(logFormatDate), Name: "date"},
		{Bit: uint64(logFormatTime), Name: "time"},
		{Bit: uint64(logFormatMicroseconds), Name: "microseconds"},
		{Bit: uint64(logFormatUTC), Name: "UTC"},
		{Bit: uint64(logFormatLongFile), Name: "longfile"},
		{Bit: uint64(logFormatShortFile), Name: "shortfile"},
		{Bit: uint64(logFormatPid), Name: "pid"},
		{Bit: uint64(logFormatNoLevel), Name: "nolevel"},
		{Bit: uint64(logFormatJSON), Name: "json"},
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
// It is designed to be used in a defer statement, so it returns a
// function that logs the exit parameters.
//
// Any pointers in the exit function will be dereferenced
func Trace(o any, format string, a ...any) func(string, ...any) {
	if fs.GetConfig(context.Background()).LogLevel < fs.LogLevelDebug {
		return func(format string, a ...any) {}
	}
	name := fnName()
	fs.LogPrintf(fs.LogLevelDebug, o, name+": "+format, a...)
	return func(format string, a ...any) {
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

// Stack logs a stack trace of callers with the o and info passed in
func Stack(o any, info string) {
	if fs.GetConfig(context.Background()).LogLevel < fs.LogLevelDebug {
		return
	}
	arr := [16 * 1024]byte{}
	buf := arr[:]
	n := runtime.Stack(buf, false)
	buf = buf[:n]
	fs.LogPrintf(fs.LogLevelDebug, o, "%s\nStack trace:\n%s", info, buf)
}

// This is called from fs when the config is reloaded
//
// The config should really be here but we can't move it as it is
// externally visible in the rc.
func logReload(ci *fs.ConfigInfo) error {
	Handler.SetLevel(fs.LogLevelToSlog(ci.LogLevel))

	if Opt.WindowsEventLogLevel != fs.LogLevelOff && Opt.WindowsEventLogLevel > ci.LogLevel {
		return fmt.Errorf("--windows-event-log-level %q must be >= --log-level %q", Opt.WindowsEventLogLevel, ci.LogLevel)
	}

	return nil
}

func init() {
	fs.LogReload = logReload
}

// InitLogging start the logging as per the command line flags
func InitLogging() {
	// Note that ci only has the defaults in at this point
	// We set real values in logReload
	ci := fs.GetConfig(context.Background())

	// Log file output
	if Opt.File != "" {
		f, err := os.OpenFile(Opt.File, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0640)
		if err != nil {
			fs.Fatalf(nil, "Failed to open log file: %v", err)
		}
		_, err = f.Seek(0, io.SeekEnd)
		if err != nil {
			fs.Errorf(nil, "Failed to seek log file to end: %v", err)
		}
		redirectStderr(f)
		Handler.setWriter(f)
	}

	// --use-json-log implies JSON formatting
	if ci.UseJSONLog {
		Opt.Format |= logFormatJSON
	}

	// Set slog level to initial log level
	Handler.SetLevel(fs.LogLevelToSlog(fs.InitialLogLevel()))

	// Set the format to the configured format
	Handler.setFormat(Opt.Format)

	// Syslog output
	if Opt.UseSyslog {
		if Opt.File != "" {
			fs.Fatalf(nil, "Can't use --syslog and --log-file together")
		}
		startSysLog(Handler)
	}

	// Activate systemd logger support if systemd invocation ID is
	// detected and output is going to stderr (not logging to a file or syslog)
	if !Redirected() {
		if isJournalStream() {
			Opt.LogSystemdSupport = true
		}
	}

	// Systemd logging output
	if Opt.LogSystemdSupport {
		startSystemdLog(Handler)
	}

	// Windows event logging
	if Opt.WindowsEventLogLevel != fs.LogLevelOff {
		err := startWindowsEventLog(Handler)
		if err != nil {
			fs.Fatalf(nil, "Failed to start windows event log: %v", err)
		}
	}
}

// Redirected returns true if the log has been redirected from stdout
func Redirected() bool {
	return Opt.UseSyslog || Opt.File != ""
}
