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
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// Logger is the interface of SDK logger.
type Logger interface {
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
	Panicf(format string, args ...interface{})
}

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
		entry.Message),
	), nil
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
	if l, ok := instance.(*logrus.Logger); ok {
		return l.Level.String()
	}
	return "unknown"
}

// SetLevel sets the log level. Valid levels are "debug", "info", "warn", "error", and "fatal".
func SetLevel(level string) {
	if l, ok := instance.(*logrus.Logger); ok {
		lvl, err := logrus.ParseLevel(level)
		if err != nil {
			Fatalf(fmt.Sprintf(`log level not valid: "%s"`, level))
		}
		l.Level = lvl
	}
}

// SetLogger sets the a logger as SDK logger.
func SetLogger(l Logger) {
	instance = l
}

// Debugf logs a message with severity DEBUG.
func Debugf(format string, v ...interface{}) {
	instance.Debugf(format, v...)
}

// Infof logs a message with severity INFO.
func Infof(format string, v ...interface{}) {
	instance.Infof(format, v...)
}

// Warnf logs a message with severity WARN.
func Warnf(format string, v ...interface{}) {
	instance.Warnf(format, v...)
}

// Errorf logs a message with severity ERROR.
func Errorf(format string, v ...interface{}) {
	instance.Errorf(format, v...)
}

// Fatalf logs a message with severity ERROR followed by a call to os.Exit().
func Fatalf(format string, v ...interface{}) {
	instance.Fatalf(format, v...)
}

var instance Logger

func init() {
	l := logrus.New()
	l.Formatter = &LogFormatter{}
	l.Out = os.Stderr
	l.Level = logrus.WarnLevel

	instance = l
}
