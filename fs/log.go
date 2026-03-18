package fs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"

	"github.com/rclone/rclone/lib/caller"
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
	LogLevelOff
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
		LogLevelOff:       "OFF",
	}
}

func (logLevelChoices) Type() string {
	return "LogLevel"
}

// slogLevel definitions defined as slog.Level constants.
// The integer values determine severity for filtering.
// Lower values are less severe (e.g., Debug), higher values are more severe (e.g., Emergency).
// We fit our extra values into slog's scale.
const (
	// slog.LevelDebug   slog.Level = -4
	// slog.LevelInfo    slog.Level = 0
	SlogLevelNotice = slog.Level(2) // Between Info (0) and Warn (4)
	// slog.LevelWarn    slog.Level = 4
	// slog.LevelError   slog.Level = 8
	SlogLevelCritical  = slog.Level(12) // More severe than Error
	SlogLevelAlert     = slog.Level(16) // More severe than Critical
	SlogLevelEmergency = slog.Level(20) // Most severe
	SlogLevelOff       = slog.Level(24) // A very high value
)

// Map our level numbers to slog level numbers
var levelToSlog = []slog.Level{
	LogLevelEmergency: SlogLevelEmergency,
	LogLevelAlert:     SlogLevelAlert,
	LogLevelCritical:  SlogLevelCritical,
	LogLevelError:     slog.LevelError,
	LogLevelWarning:   slog.LevelWarn,
	LogLevelNotice:    SlogLevelNotice,
	LogLevelInfo:      slog.LevelInfo,
	LogLevelDebug:     slog.LevelDebug,
	LogLevelOff:       SlogLevelOff,
}

// LogValueItem describes keyed item for a JSON log entry
type LogValueItem struct {
	key    string
	value  any
	render bool
}

// LogValue should be used as an argument to any logging calls to
// augment the JSON output with more structured information.
//
// key is the dictionary parameter used to store value.
func LogValue(key string, value any) LogValueItem {
	return LogValueItem{key: key, value: value, render: true}
}

// LogValueHide should be used as an argument to any logging calls to
// augment the JSON output with more structured information.
//
// key is the dictionary parameter used to store value.
//
// String() will return a blank string - this is useful to put items
// in which don't print into the log.
func LogValueHide(key string, value any) LogValueItem {
	return LogValueItem{key: key, value: value, render: false}
}

// String returns the representation of value. If render is false this
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

// LogLevelToSlog converts an rclone log level to log/slog log level.
func LogLevelToSlog(level LogLevel) slog.Level {
	slogLevel := slog.LevelError
	// NB level is unsigned so we don't check < 0 here
	if int(level) < len(levelToSlog) {
		slogLevel = levelToSlog[level]
	}
	return slogLevel
}

func logSlog(level LogLevel, text string, attrs []any) {
	slog.Log(context.Background(), LogLevelToSlog(level), text, attrs...)
}

func logSlogWithObject(level LogLevel, o any, text string, attrs []any) {
	if o != nil {
		attrs = slices.Concat(attrs, []any{
			"object", fmt.Sprintf("%+v", o),
			"objectType", fmt.Sprintf("%T", o),
		})
	}
	logSlog(level, text, attrs)
}

// LogPrint produces a log string from the arguments passed in
func LogPrint(level LogLevel, o any, text string) {
	logSlogWithObject(level, o, text, nil)
}

// LogPrintf produces a log string from the arguments passed in
func LogPrintf(level LogLevel, o any, text string, args ...any) {
	text = fmt.Sprintf(text, args...)
	var fields []any
	for _, arg := range args {
		if item, ok := arg.(LogValueItem); ok {
			fields = append(fields, item.key, item.value)
		}
	}
	logSlogWithObject(level, o, text, fields)
}

// LogLevelPrint writes logs at the given level
func LogLevelPrint(level LogLevel, o any, text string) {
	if GetConfig(context.TODO()).LogLevel >= level {
		LogPrint(level, o, text)
	}
}

// LogLevelPrintf writes logs at the given level
func LogLevelPrintf(level LogLevel, o any, text string, args ...any) {
	if GetConfig(context.TODO()).LogLevel >= level {
		LogPrintf(level, o, text, args...)
	}
}

// Panic writes alert log output for this Object or Fs and calls panic().
// It should always be seen by the user.
func Panic(o any, text string) {
	if GetConfig(context.TODO()).LogLevel >= LogLevelAlert {
		LogPrint(LogLevelAlert, o, text)
	}
	panic(text)
}

// Panicf writes alert log output for this Object or Fs and calls panic().
// It should always be seen by the user.
func Panicf(o any, text string, args ...any) {
	if GetConfig(context.TODO()).LogLevel >= LogLevelAlert {
		LogPrintf(LogLevelAlert, o, text, args...)
	}
	panic(fmt.Sprintf(text, args...))
}

// Panic if this called from an rc job.
//
// This means fatal errors get turned into panics which get caught by
// the rc job handler so they don't crash rclone.
//
// This detects if we are being called from an rc Job by looking for
// Job.run in the call stack.
//
// Ideally we would do this by passing a context about but we don't
// have one with the logging calls yet.
//
// This is tested in fs/rc/internal_job_test.go in TestInternalFatal.
func panicIfRcJob(o any, text string, args []any) {
	if !caller.Present("(*Job).run") {
		return
	}
	var errTxt strings.Builder
	_, _ = errTxt.WriteString("fatal error: ")
	if o != nil {
		_, _ = fmt.Fprintf(&errTxt, "%v: ", o)
	}
	if args != nil {
		_, _ = fmt.Fprintf(&errTxt, text, args...)
	} else {
		_, _ = errTxt.WriteString(text)
	}
	panic(errTxt.String())
}

// Fatal writes critical log output for this Object or Fs and calls os.Exit(1).
// It should always be seen by the user.
func Fatal(o any, text string) {
	if GetConfig(context.TODO()).LogLevel >= LogLevelCritical {
		LogPrint(LogLevelCritical, o, text)
	}
	panicIfRcJob(o, text, nil)
	os.Exit(1)
}

// Fatalf writes critical log output for this Object or Fs and calls os.Exit(1).
// It should always be seen by the user.
func Fatalf(o any, text string, args ...any) {
	if GetConfig(context.TODO()).LogLevel >= LogLevelCritical {
		LogPrintf(LogLevelCritical, o, text, args...)
	}
	panicIfRcJob(o, text, args)
	os.Exit(1)
}

// Error writes error log output for this Object or Fs.  It
// should always be seen by the user.
func Error(o any, text string) {
	LogLevelPrint(LogLevelError, o, text)
}

// Errorf writes error log output for this Object or Fs.  It
// should always be seen by the user.
func Errorf(o any, text string, args ...any) {
	LogLevelPrintf(LogLevelError, o, text, args...)
}

// Print writes log output for this Object or Fs, same as Logf.
func Print(o any, text string) {
	LogLevelPrint(LogLevelNotice, o, text)
}

// Printf writes log output for this Object or Fs, same as Logf.
func Printf(o any, text string, args ...any) {
	LogLevelPrintf(LogLevelNotice, o, text, args...)
}

// Log writes log output for this Object or Fs.  This should be
// considered to be Notice level logging.  It is the default level.
// By default rclone should not log very much so only use this for
// important things the user should see.  The user can filter these
// out with the -q flag.
func Log(o any, text string) {
	LogLevelPrint(LogLevelNotice, o, text)
}

// Logf writes log output for this Object or Fs.  This should be
// considered to be Notice level logging.  It is the default level.
// By default rclone should not log very much so only use this for
// important things the user should see.  The user can filter these
// out with the -q flag.
func Logf(o any, text string, args ...any) {
	LogLevelPrintf(LogLevelNotice, o, text, args...)
}

// Infoc writes info on transfers for this Object or Fs.  Use this
// level for logging transfers, deletions and things which should
// appear with the -v flag.
// There is name class on "Info", hence the name "Infoc", "c" for constant.
func Infoc(o any, text string) {
	LogLevelPrint(LogLevelInfo, o, text)
}

// Infof writes info on transfers for this Object or Fs.  Use this
// level for logging transfers, deletions and things which should
// appear with the -v flag.
func Infof(o any, text string, args ...any) {
	LogLevelPrintf(LogLevelInfo, o, text, args...)
}

// Debug writes debugging output for this Object or Fs.  Use this for
// debug only.  The user must have to specify -vv to see this.
func Debug(o any, text string) {
	LogLevelPrint(LogLevelDebug, o, text)
}

// Debugf writes debugging output for this Object or Fs.  Use this for
// debug only.  The user must have to specify -vv to see this.
func Debugf(o any, text string, args ...any) {
	LogLevelPrintf(LogLevelDebug, o, text, args...)
}

// LogDirName returns an object for the logger, logging a root
// directory which would normally be "" as the Fs
func LogDirName(f Fs, dir string) any {
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
