package log

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/rclone/rclone/fs"
	"github.com/sirupsen/logrus"
)

var loggerInstalled = false

// InstallJSONLogger installs the JSON logger at the specified log level
func InstallJSONLogger(logLevel fs.LogLevel) {
	if !loggerInstalled {
		logrus.AddHook(NewCallerHook())
		loggerInstalled = true
	}
	logrus.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: "2006-01-02T15:04:05.999999-07:00",
	})
	logrus.SetLevel(logrus.DebugLevel)
	switch logLevel {
	case fs.LogLevelEmergency, fs.LogLevelAlert:
		logrus.SetLevel(logrus.PanicLevel)
	case fs.LogLevelCritical:
		logrus.SetLevel(logrus.FatalLevel)
	case fs.LogLevelError:
		logrus.SetLevel(logrus.ErrorLevel)
	case fs.LogLevelWarning, fs.LogLevelNotice:
		logrus.SetLevel(logrus.WarnLevel)
	case fs.LogLevelInfo:
		logrus.SetLevel(logrus.InfoLevel)
	case fs.LogLevelDebug:
		logrus.SetLevel(logrus.DebugLevel)
	}
}

// install hook in fs to call to avoid circular dependency
func init() {
	fs.InstallJSONLogger = InstallJSONLogger
}

// CallerHook for log the calling file and line of the fine
type CallerHook struct {
	Field  string
	Skip   int
	levels []logrus.Level
}

// NewCallerHook use to make a hook
func NewCallerHook(levels ...logrus.Level) logrus.Hook {
	hook := CallerHook{
		Field:  "source",
		Skip:   7,
		levels: levels,
	}
	if len(hook.levels) == 0 {
		hook.levels = logrus.AllLevels
	}
	return &hook
}

// Levels implement applied hook to which levels
func (h *CallerHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// Fire logs the information of context (filename and line)
func (h *CallerHook) Fire(entry *logrus.Entry) error {
	entry.Data[h.Field] = findCaller(h.Skip)
	return nil
}

// findCaller ignores the caller relevant to logrus or fslog then find out the exact caller
func findCaller(skip int) string {
	file := ""
	line := 0
	for i := 0; i < 10; i++ {
		file, line = getCaller(skip + i)
		if !strings.HasPrefix(file, "logrus") && !strings.Contains(file, "log.go") {
			break
		}
	}
	return fmt.Sprintf("%s:%d", file, line)
}

func getCaller(skip int) (string, int) {
	_, file, line, ok := runtime.Caller(skip)
	// fmt.Println(file,":",line)
	if !ok {
		return "", 0
	}
	n := 0
	for i := len(file) - 1; i > 0; i-- {
		if file[i] == '/' {
			n++
			if n >= 2 {
				file = file[i+1:]
				break
			}
		}
	}
	return file, line
}
