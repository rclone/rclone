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
package logger

import (
	"context"
	"fmt"

	"github.com/pengsrc/go-shared/log"
)

// Logger is the interface of SDK logger.
type Logger interface {
	Debugf(ctx context.Context, format string, v ...interface{})
	Infof(ctx context.Context, format string, v ...interface{})
	Warnf(ctx context.Context, format string, v ...interface{})
	Errorf(ctx context.Context, format string, v ...interface{})
	Fatalf(ctx context.Context, format string, v ...interface{})
	Panicf(ctx context.Context, format string, v ...interface{})
}

// CheckLevel checks whether the log level is valid.
func CheckLevel(level string) error {
	if _, err := log.ParseLevel(level); err != nil {
		return fmt.Errorf(`log level not valid: "%s"`, level)
	}
	return nil
}

// GetLevel get the log level string.
func GetLevel() string {
	if l, ok := instance.(*log.Logger); ok {
		return l.GetLevel()
	}
	return "UNKNOWN"
}

// SetLevel sets the log level.
// Valid levels are "debug", "info", "warn", "error", and "fatal".
func SetLevel(level string) {
	if l, ok := instance.(*log.Logger); ok {
		err := l.SetLevel(level)
		if err != nil {
			Fatalf(nil, fmt.Sprintf(`log level not valid: "%s"`, level))
		}
	}
}

// SetLogger sets the a logger as SDK logger.
func SetLogger(l Logger) {
	instance = l
}

// Debugf logs a message with severity DEBUG.
func Debugf(ctx context.Context, format string, v ...interface{}) {
	instance.Debugf(ctx, format, v...)
}

// Infof logs a message with severity INFO.
func Infof(ctx context.Context, format string, v ...interface{}) {
	instance.Infof(ctx, format, v...)
}

// Warnf logs a message with severity WARN.
func Warnf(ctx context.Context, format string, v ...interface{}) {
	instance.Warnf(ctx, format, v...)
}

// Errorf logs a message with severity ERROR.
func Errorf(ctx context.Context, format string, v ...interface{}) {
	instance.Errorf(ctx, format, v...)
}

// Fatalf logs a message with severity ERROR followed by a call to os.Exit().
func Fatalf(ctx context.Context, format string, v ...interface{}) {
	instance.Fatalf(ctx, format, v...)
}

var instance Logger

func init() {
	l, err := log.NewTerminalLogger(log.WarnLevel.String())
	if err != nil {
		panic(fmt.Sprintf("failed to initialize QingStor SDK logger: %v", err))
	}
	instance = l
}
