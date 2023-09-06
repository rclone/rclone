// Package log provides support for logging to stdout, stderr and file.
package log

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path"
	"syscall"
	"time"

	"github.com/pengsrc/go-shared/check"
	"github.com/pengsrc/go-shared/reopen"
)

// Logger presents a logger.
// The only way to initialize a logger is using the convention construct
// functions like NewLogger().
type Logger struct {
	level Level
	lw    LevelWriter

	// Interested context keys.
	ctxKeys    []interface{}
	ctxKeysMap map[interface{}]string

	// isCallerEnabled sets whether to annotating logs with the calling
	// function's file name and line number. By default, all logs are annotated.
	isCallerEnabled bool
}

// GetLevel get the log level string.
func (l *Logger) GetLevel() string {
	return l.level.String()
}

// SetLevel sets the log level.
// Valid levels are "debug", "info", "warn", "error", and "fatal".
func (l *Logger) SetLevel(level string) (err error) {
	levelFlag, err := ParseLevel(level)
	if err == nil {
		l.level = levelFlag
	}
	return err
}

// SetInterestContextKeys sets the contexts keys that the logger should be
// interested in. Value of the interested context key will extract and print as
// newEvent filed.
func (l *Logger) SetInterestContextKeys(keys []interface{}) {
	l.ctxKeys = keys

	l.ctxKeysMap = make(map[interface{}]string)
	for _, key := range l.ctxKeys {
		l.ctxKeysMap[key] = fmt.Sprintf("%v", key)
	}
}

// SetCallerFlag sets whether to annotating logs with the caller.
func (l *Logger) SetCallerFlag(isEnabled bool) {
	l.isCallerEnabled = isEnabled
}

// Flush writes buffered logs.
func (l *Logger) Flush() {
	if flusher, ok := l.lw.(Flusher); ok {
		flusher.Flush()
	}
}

// Fatal logs a message with severity FATAL followed by a call to os.Exit(1).
func (l *Logger) Fatal(ctx context.Context, v ...interface{}) {
	l.event(ctx, FatalLevel).write("%v", v...)
}

// Panic logs a message with severity PANIC followed by a call to panic().
func (l *Logger) Panic(ctx context.Context, v ...interface{}) {
	l.event(ctx, PanicLevel).write("%v", v...)
}

// Error logs a message with severity ERROR.
func (l *Logger) Error(ctx context.Context, v ...interface{}) {
	l.event(ctx, ErrorLevel).write("%v", v...)
}

// Warn logs a message with severity WARN.
func (l *Logger) Warn(ctx context.Context, v ...interface{}) {
	l.event(ctx, WarnLevel).write("%v", v...)
}

// Info logs a message with severity INFO.
func (l *Logger) Info(ctx context.Context, v ...interface{}) {
	l.event(ctx, InfoLevel).write("%v", v...)
}

// Debug logs a message with severity DEBUG.
func (l *Logger) Debug(ctx context.Context, v ...interface{}) {
	l.event(ctx, DebugLevel).write("%v", v...)
}

// Fatalf logs a message with severity FATAL in format followed by a call to
// os.Exit(1).
func (l *Logger) Fatalf(ctx context.Context, format string, v ...interface{}) {
	l.event(ctx, FatalLevel).write(format, v...)
}

// Panicf logs a message with severity PANIC in format followed by a call to
// panic().
func (l *Logger) Panicf(ctx context.Context, format string, v ...interface{}) {
	l.event(ctx, PanicLevel).write(format, v...)
}

// Errorf logs a message with severity ERROR in format.
func (l *Logger) Errorf(ctx context.Context, format string, v ...interface{}) {
	l.event(ctx, ErrorLevel).write(format, v...)
}

// Warnf logs a message with severity WARN in format.
func (l *Logger) Warnf(ctx context.Context, format string, v ...interface{}) {
	l.event(ctx, WarnLevel).write(format, v...)
}

// Infof logs a message with severity INFO in format.
func (l *Logger) Infof(ctx context.Context, format string, v ...interface{}) {
	l.event(ctx, InfoLevel).write(format, v...)
}

// Debugf logs a message with severity DEBUG in format.
func (l *Logger) Debugf(ctx context.Context, format string, v ...interface{}) {
	l.event(ctx, DebugLevel).write(format, v...)
}

// FatalEvent returns a log event with severity FATAL.
func (l *Logger) FatalEvent(ctx context.Context) *Event {
	return l.event(ctx, FatalLevel)
}

// PanicEvent returns a log event with severity PANIC.
func (l *Logger) PanicEvent(ctx context.Context) *Event {
	return l.event(ctx, PanicLevel)
}

// ErrorEvent returns a log event with severity ERROR.
func (l *Logger) ErrorEvent(ctx context.Context) *Event {
	return l.event(ctx, ErrorLevel)
}

// WarnEvent returns a log event with severity WARN.
func (l *Logger) WarnEvent(ctx context.Context) *Event {
	return l.event(ctx, WarnLevel)
}

// InfoEvent returns a log event with severity INFO.
func (l *Logger) InfoEvent(ctx context.Context) *Event {
	return l.event(ctx, InfoLevel)
}

// DebugEvent returns a log event with severity DEBUG.
func (l *Logger) DebugEvent(ctx context.Context) *Event {
	return l.event(ctx, DebugLevel)
}

func (l *Logger) event(ctx context.Context, level Level) (e *Event) {
	var ctxKeys *[]interface{}
	var ctxKeysMap *map[interface{}]string

	if len(l.ctxKeys) > 0 {
		ctxKeys = &l.ctxKeys
	}
	if len(l.ctxKeysMap) > 0 {
		ctxKeysMap = &l.ctxKeysMap
	}

	return newEvent(ctx, ctxKeys, ctxKeysMap, level, l.lw, level <= l.level, l.isCallerEnabled)
}

// NewLogger creates a new logger for given out and level, and the level is
// optional.
func NewLogger(out io.Writer, level ...string) (*Logger, error) {
	return NewLoggerWithError(out, nil, level...)
}

// NewLoggerWithError creates a new logger for given out, err out, level, and the
// err out can be nil, and the level is optional.
func NewLoggerWithError(out, errOut io.Writer, level ...string) (l *Logger, err error) {
	if out == nil {
		return nil, errors.New("logger output must specified")
	}

	sw := &StandardWriter{w: out, ew: errOut, pid: os.Getpid()}
	l = &Logger{lw: sw}

	if len(level) == 1 {
		if err = l.SetLevel(level[0]); err != nil {
			return nil, err
		}
	}

	return l, nil
}

// NewTerminalLogger creates a logger that write into terminal.
func NewTerminalLogger(level ...string) (*Logger, error) {
	return NewLogger(os.Stdout, level...)
}

// NewBufferedTerminalLogger creates a buffered logger that write into terminal.
func NewBufferedTerminalLogger(level ...string) (*Logger, error) {
	return NewLogger(bufio.NewWriter(os.Stdout), level...)
}

// NewFileLogger creates a logger that write into file.
func NewFileLogger(filePath string, level ...string) (*Logger, error) {
	return NewFileLoggerWithError(filePath, "", level...)
}

// NewFileLoggerWithError creates a logger that write into files.
func NewFileLoggerWithError(filePath, errFilePath string, level ...string) (*Logger, error) {
	if err := check.Dir(path.Dir(filePath)); err != nil {
		return nil, err
	}
	if errFilePath != "" {
		if err := check.Dir(path.Dir(errFilePath)); err != nil {
			return nil, err
		}
	}

	out, err := reopen.NewFileWriter(filePath)
	if err != nil {
		return nil, err
	}
	var errOut *reopen.FileWriter
	if errFilePath != "" {
		errOut, err = reopen.NewFileWriter(errFilePath)
		if err != nil {
			return nil, err
		}
	}

	c := make(chan os.Signal)
	go func() {
		for {
			select {
			case <-c:
				out.Reopen()
				if errOut != nil {
					errOut.Reopen()
				}
			}
		}
	}()
	signal.Notify(c, syscall.SIGHUP)

	if errOut == nil {
		return NewLoggerWithError(out, nil, level...)
	}
	return NewLoggerWithError(out, errOut, level...)
}

// NewBufferedFileLogger creates a logger that write into file with buffer.
// The flushSeconds's unit is second.
func NewBufferedFileLogger(filePath string, flushInterval int, level ...string) (*Logger, error) {
	return NewBufferedFileLoggerWithError(filePath, "", flushInterval, level...)
}

// NewBufferedFileLoggerWithError creates a logger that write into files with buffer.
// The flushSeconds's unit is second.
func NewBufferedFileLoggerWithError(filePath, errFilePath string, flushInterval int, level ...string) (*Logger, error) {
	if err := check.Dir(path.Dir(filePath)); err != nil {
		return nil, err
	}
	if errFilePath != "" {
		if err := check.Dir(path.Dir(errFilePath)); err != nil {
			return nil, err
		}
	}

	if flushInterval == 0 {
		flushInterval = 10
	}

	out, err := reopen.NewFileWriter(filePath)
	if err != nil {
		return nil, err
	}
	var errOut *reopen.FileWriter
	if errFilePath != "" {
		errOut, err = reopen.NewFileWriter(errFilePath)
		if err != nil {
			return nil, err
		}
	}

	bufferedOut := reopen.NewBufferedFileWriter(out)
	var bufferedErrOut *reopen.BufferedFileWriter
	if errOut != nil {
		bufferedErrOut = reopen.NewBufferedFileWriter(errOut)
	}

	c := make(chan os.Signal)
	go func() {
		for {
			select {
			case <-c:
				bufferedOut.Reopen()
				if bufferedErrOut != nil {
					bufferedErrOut.Reopen()
				}
			case <-time.After(time.Duration(flushInterval) * time.Second):
				bufferedOut.Flush()
				if bufferedErrOut != nil {
					bufferedErrOut.Flush()
				}
			}
		}
	}()
	signal.Notify(c, syscall.SIGHUP)

	if bufferedErrOut == nil {
		return NewLoggerWithError(bufferedOut, nil, level...)
	}
	return NewLoggerWithError(bufferedOut, bufferedErrOut, level...)
}
