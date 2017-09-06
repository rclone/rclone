// +-------------------------------------------------------------------------
// | Copyright (C) 2016 Yunify, Inc.
// +-------------------------------------------------------------------------
// | Licensed under the Apache License, Version 2.0 (the "License");
// | you may not use this work except in compliance with the License.
// | You may obtain a copy of the License in the LICENSE file, or at:
// |
// | http://www.apache.org/licenses/LICENSE-2.0
// |
// | Unless required by applicable law or agreed to in writing, software
// | distributed under the License is distributed on an "AS IS" BASIS,
// | WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// | See the License for the specific language governing permissions and
// | limitations under the License.
// +-------------------------------------------------------------------------

// Package logger provides support for logging to stdout and stderr.
// Log entries will be logged with format: $timestamp $hostname [$pid]: $severity $message.
package logger

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

var instance *logrus.Logger

// LogFormatter is used to format log entry.
type LogFormatter struct{}

// Format formats a given log entry, returns byte slice and error.
func (c *LogFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	level := strings.ToUpper(entry.Level.String())
	if level == "WARNING" {
		level = "WARN"
	}
	if len(level) < 5 {
		level = strings.Repeat(" ", 5-len(level)) + level
	}

	return []byte(fmt.Sprintf(
		"[%s #%d] %s -- : %s\n",
		time.Now().Format("2006-01-02T15:04:05.000Z"),
		os.Getpid(),
		level,
		entry.Message)), nil
}

// SetOutput set the destination for the log output
func SetOutput(out io.Writer) {
	instance.Out = out
}

// CheckLevel checks whether the log level is valid.
func CheckLevel(level string) error {
	if _, err := logrus.ParseLevel(level); err != nil {
		return fmt.Errorf(`log level not valid: "%s"`, level)
	}
	return nil
}

// GetLevel get the log level string.
func GetLevel() string {
	return instance.Level.String()
}

// SetLevel sets the log level. Valid levels are "debug", "info", "warn", "error", and "fatal".
func SetLevel(level string) {
	lvl, err := logrus.ParseLevel(level)
	if err != nil {
		Fatal(fmt.Sprintf(`log level not valid: "%s"`, level))
	}
	instance.Level = lvl
}

// Debug logs a message with severity DEBUG.
func Debug(format string, v ...interface{}) {
	output(instance.Debug, format, v...)
}

// Info logs a message with severity INFO.
func Info(format string, v ...interface{}) {
	output(instance.Info, format, v...)
}

// Warn logs a message with severity WARN.
func Warn(format string, v ...interface{}) {
	output(instance.Warn, format, v...)
}

// Error logs a message with severity ERROR.
func Error(format string, v ...interface{}) {
	output(instance.Error, format, v...)
}

// Fatal logs a message with severity ERROR followed by a call to os.Exit().
func Fatal(format string, v ...interface{}) {
	output(instance.Fatal, format, v...)
}

func output(origin func(...interface{}), format string, v ...interface{}) {
	if len(v) > 0 {
		origin(fmt.Sprintf(format, v...))
	} else {
		origin(format)
	}
}

func init() {
	instance = logrus.New()
	instance.Formatter = &LogFormatter{}
	instance.Out = os.Stderr
	instance.Level = logrus.WarnLevel
}
