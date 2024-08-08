// Package logrus provides abstraction for logrus for rclone
package logrus

import (
	"context"
	"fmt"
	"io"

	"github.com/sirupsen/logrus"

	"github.com/rclone/rclone/fs"
)

// LogPrintf produces a log string from the arguments passed in
var LogPrintf = func(level fs.LogLevel, o interface{}, text string, args ...interface{}) {
	out := fmt.Sprintf(text, args...)

	if fs.GetConfig(context.TODO()).UseJSONLog {
		fields := logrus.Fields{}
		if o != nil {
			fields = logrus.Fields{
				"object":     fmt.Sprintf("%+v", o),
				"objectType": fmt.Sprintf("%T", o),
			}
		}
		for _, arg := range args {
			if item, ok := arg.(fs.LogValueItem); ok {
				fields[item.Key] = item.Value
			}
		}
		switch level {
		case fs.LogLevelDebug:
			logrus.WithFields(fields).Debug(out)
		case fs.LogLevelInfo:
			logrus.WithFields(fields).Info(out)
		case fs.LogLevelNotice, fs.LogLevelWarning:
			logrus.WithFields(fields).Warn(out)
		case fs.LogLevelError:
			logrus.WithFields(fields).Error(out)
		case fs.LogLevelCritical:
			logrus.WithFields(fields).Fatal(out)
		case fs.LogLevelEmergency, fs.LogLevelAlert:
			logrus.WithFields(fields).Panic(out)
		}
	} else {
		if o != nil {
			out = fmt.Sprintf("%v: %s", o, out)
		}
		fs.LogPrint(level, out)
	}
}

// SetOutput sets output for logrus Standard Logger
func SetOutput(out io.Writer) {
	logrus.SetOutput(out)
}

// GetStandardLoggerWriter returns the Writer for the Standard Logger
func GetStandardLoggerWriter() *io.PipeWriter {
	return logrus.StandardLogger().Writer()
}
