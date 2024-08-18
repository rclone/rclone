package fs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/sirupsen/logrus"
)

// LogLevel describes rclone's logs.  These are a subset of the syslog log levels.
type LogLevel = Enum[logLevelChoices]

// Log levels.  These are the syslog levels of which we only use a
// subset.
//
//	LOG_EMERG      system is unusable
//	LOG_ALERT      action must be taken immediately
//	LOG_CRIT       critical conditions
//	LOG_ERR        error conditions
//	LOG_WARNING    warning conditions
//	LOG_NOTICE     normal, but significant, condition
//	LOG_INFO       informational message
//	LOG_DEBUG      debug-level message
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

type logLevelChoices struct{}

func (logLevelChoices) Choices() []string {
	return []string{
		LogLevelEmergency: "EMERGENCY",
		LogLevelAlert:     "ALERT",
		LogLevelCritical:  "CRITICAL",
		LogLevelError:     "ERROR",
		LogLevelWarning:   "WARNING",
		LogLevelNotice:    "NOTICE",
		LogLevelInfo:      "INFO",
		LogLevelDebug:     "DEBUG",
	}
}

func (logLevelChoices) Type() string {
	return "LogLevel"
}

// LogPrintPid enables process pid in log
var LogPrintPid = false

// InstallJSONLogger is a hook that --use-json-log calls
var InstallJSONLogger = func(logLevel LogLevel) {}

// LogOutput sends the text to the logger of level
var LogOutput = func(level LogLevel, text string) {
	text = fmt.Sprintf("%-6s: %s", level, text)
	if LogPrintPid {
		text = fmt.Sprintf("[%d] %s", os.Getpid(), text)
	}
	_ = log.Output(4, text)
}

// LogValueItem describes keyed item for a JSON log entry
type LogValueItem struct {
	key    string
	value  interface{}
	render bool
}

// LogValue should be used as an argument to any logging calls to
// augment the JSON output with more structured information.
//
// key is the dictionary parameter used to store value.
func LogValue(key string, value interface{}) LogValueItem {
	return LogValueItem{key: key, value: value, render: true}
}

// LogValueHide should be used as an argument to any logging calls to
// augment the JSON output with more structured information.
//
// key is the dictionary parameter used to store value.
//
// String() will return a blank string - this is useful to put items
// in which don't print into the log.
func LogValueHide(key string, value interface{}) LogValueItem {
	return LogValueItem{key: key, value: value, render: false}
}

// String returns the representation of value. If render is fals this
// is an empty string so LogValueItem entries won't show in the
// textual representation of logs.
func (j LogValueItem) String() string {
	if !j.render {
		return ""
	}
	if do, ok := j.value.(fmt.Stringer); ok {
		return do.String()
	}
	return fmt.Sprint(j.value)
}

func logLogrus(level LogLevel, text string, fields logrus.Fields) {
	switch level {
	case LogLevelDebug:
		logrus.WithFields(fields).Debug(text)
	case LogLevelInfo:
		logrus.WithFields(fields).Info(text)
	case LogLevelNotice, LogLevelWarning:
		logrus.WithFields(fields).Warn(text)
	case LogLevelError:
		logrus.WithFields(fields).Error(text)
	case LogLevelCritical:
		logrus.WithFields(fields).Fatal(text)
	case LogLevelEmergency, LogLevelAlert:
		logrus.WithFields(fields).Panic(text)
	}
}

func logLogrusWithObject(level LogLevel, o interface{}, text string, fields logrus.Fields) {
	if o != nil {
		if fields == nil {
			fields = logrus.Fields{}
		}
		fields["object"] = fmt.Sprintf("%+v", o)
		fields["objectType"] = fmt.Sprintf("%T", o)
	}
	logLogrus(level, text, fields)
}

func logJSON(level LogLevel, o interface{}, text string) {
	logLogrusWithObject(level, o, text, nil)
}

func logJSONf(level LogLevel, o interface{}, text string, args ...interface{}) {
	text = fmt.Sprintf(text, args...)
	fields := logrus.Fields{}
	for _, arg := range args {
		if item, ok := arg.(LogValueItem); ok {
			fields[item.key] = item.value
		}
	}
	logLogrusWithObject(level, o, text, fields)
}

func logPlain(level LogLevel, o interface{}, text string) {
	if o != nil {
		text = fmt.Sprintf("%v: %s", o, text)
	}
	LogOutput(level, text)
}

func logPlainf(level LogLevel, o interface{}, text string, args ...interface{}) {
	logPlain(level, o, fmt.Sprintf(text, args...))
}

// LogPrint produces a log string from the arguments passed in
func LogPrint(level LogLevel, o interface{}, text string) {
	if GetConfig(context.TODO()).UseJSONLog {
		logJSON(level, o, text)
	} else {
		logPlain(level, o, text)
	}
}

// LogPrintf produces a log string from the arguments passed in
func LogPrintf(level LogLevel, o interface{}, text string, args ...interface{}) {
	if GetConfig(context.TODO()).UseJSONLog {
		logJSONf(level, o, text, args...)
	} else {
		logPlainf(level, o, text, args...)
	}
}

// LogLevelPrint writes logs at the given level
func LogLevelPrint(level LogLevel, o interface{}, text string) {
	if GetConfig(context.TODO()).LogLevel >= level {
		LogPrint(level, o, text)
	}
}

// LogLevelPrintf writes logs at the given level
func LogLevelPrintf(level LogLevel, o interface{}, text string, args ...interface{}) {
	if GetConfig(context.TODO()).LogLevel >= level {
		LogPrintf(level, o, text, args...)
	}
}

// Panic writes alert log output for this Object or Fs and calls panic().
// It should always be seen by the user.
func Panic(o interface{}, text string) {
	if GetConfig(context.TODO()).LogLevel >= LogLevelAlert {
		LogPrint(LogLevelAlert, o, text)
	}
	panic(text)
}

// Panicf writes alert log output for this Object or Fs and calls panic().
// It should always be seen by the user.
func Panicf(o interface{}, text string, args ...interface{}) {
	if GetConfig(context.TODO()).LogLevel >= LogLevelAlert {
		LogPrintf(LogLevelAlert, o, text, args...)
	}
	panic(fmt.Sprintf(text, args...))
}

// Fatal writes critical log output for this Object or Fs and calls os.Exit(1).
// It should always be seen by the user.
func Fatal(o interface{}, text string) {
	if GetConfig(context.TODO()).LogLevel >= LogLevelCritical {
		LogPrint(LogLevelCritical, o, text)
	}
	os.Exit(1)
}

// Fatalf writes critical log output for this Object or Fs and calls os.Exit(1).
// It should always be seen by the user.
func Fatalf(o interface{}, text string, args ...interface{}) {
	if GetConfig(context.TODO()).LogLevel >= LogLevelCritical {
		LogPrintf(LogLevelCritical, o, text, args...)
	}
	os.Exit(1)
}

// Error writes error log output for this Object or Fs.  It
// should always be seen by the user.
func Error(o interface{}, text string) {
	LogLevelPrint(LogLevelError, o, text)
}

// Errorf writes error log output for this Object or Fs.  It
// should always be seen by the user.
func Errorf(o interface{}, text string, args ...interface{}) {
	LogLevelPrintf(LogLevelError, o, text, args...)
}

// Print writes log output for this Object or Fs, same as Logf.
func Print(o interface{}, text string) {
	LogLevelPrint(LogLevelNotice, o, text)
}

// Printf writes log output for this Object or Fs, same as Logf.
func Printf(o interface{}, text string, args ...interface{}) {
	LogLevelPrintf(LogLevelNotice, o, text, args...)
}

// Log writes log output for this Object or Fs.  This should be
// considered to be Notice level logging.  It is the default level.
// By default rclone should not log very much so only use this for
// important things the user should see.  The user can filter these
// out with the -q flag.
func Log(o interface{}, text string) {
	LogLevelPrint(LogLevelNotice, o, text)
}

// Logf writes log output for this Object or Fs.  This should be
// considered to be Notice level logging.  It is the default level.
// By default rclone should not log very much so only use this for
// important things the user should see.  The user can filter these
// out with the -q flag.
func Logf(o interface{}, text string, args ...interface{}) {
	LogLevelPrintf(LogLevelNotice, o, text, args...)
}

// Infoc writes info on transfers for this Object or Fs.  Use this
// level for logging transfers, deletions and things which should
// appear with the -v flag.
// There is name class on "Info", hence the name "Infoc", "c" for constant.
func Infoc(o interface{}, text string) {
	LogLevelPrint(LogLevelInfo, o, text)
}

// Infof writes info on transfers for this Object or Fs.  Use this
// level for logging transfers, deletions and things which should
// appear with the -v flag.
func Infof(o interface{}, text string, args ...interface{}) {
	LogLevelPrintf(LogLevelInfo, o, text, args...)
}

// Debug writes debugging output for this Object or Fs.  Use this for
// debug only.  The user must have to specify -vv to see this.
func Debug(o interface{}, text string) {
	LogLevelPrint(LogLevelDebug, o, text)
}

// Debugf writes debugging output for this Object or Fs.  Use this for
// debug only.  The user must have to specify -vv to see this.
func Debugf(o interface{}, text string, args ...interface{}) {
	LogLevelPrintf(LogLevelDebug, o, text, args...)
}

// LogDirName returns an object for the logger, logging a root
// directory which would normally be "" as the Fs
func LogDirName(f Fs, dir string) interface{} {
	if dir != "" {
		return dir
	}
	return f
}

// PrettyPrint formats JSON for improved readability in debug logs.
// If it can't Marshal JSON, it falls back to fmt.
func PrettyPrint(in any, label string, level LogLevel) {
	if GetConfig(context.TODO()).LogLevel < level {
		return
	}
	inBytes, err := json.MarshalIndent(in, "", "\t")
	if err != nil || string(inBytes) == "{}" || string(inBytes) == "[]" {
		LogPrintf(level, label, "\n%+v\n", in)
		return
	}
	LogPrintf(level, label, "\n%s\n", string(inBytes))
}
