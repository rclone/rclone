// Package logger provides support for logging to stdout and stderr.
package logger

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/pengsrc/go-shared/convert"
	"github.com/pengsrc/go-shared/reopen"
)

// LogFormatter is used to format log entry.
type LogFormatter struct{}

// Format formats a given log entry, returns byte slice and error.
func (c *LogFormatter) Format(entry *log.Entry) ([]byte, error) {
	level := strings.ToUpper(entry.Level.String())
	if level == "WARNING" {
		level = "WARN"
	}
	if len(level) < 5 {
		level = strings.Repeat(" ", 5-len(level)) + level
	}

	return []byte(
		fmt.Sprintf(
			"[%s #%d] %s -- : %s\n",
			convert.TimeToString(time.Now(), convert.ISO8601Milli),
			os.Getpid(),
			level,
			entry.Message,
		),
	), nil
}

// NewLogFormatter creates a new log formatter.
func NewLogFormatter() *LogFormatter {
	return &LogFormatter{}
}

// ErrorHook presents error hook.
type ErrorHook struct {
	levels []log.Level

	out       io.Writer
	formatter log.Formatter
}

// Levels returns error log levels.
func (eh *ErrorHook) Levels() []log.Level {
	return eh.levels
}

// Fire triggers before logging.
func (eh *ErrorHook) Fire(entry *log.Entry) error {
	formatted, err := eh.formatter.Format(entry)
	if err != nil {
		return err
	}
	_, err = eh.out.Write(formatted)
	if err != nil {
		return err
	}
	return nil
}

// NewErrorHook creates new error hook.
func NewErrorHook(out io.Writer) *ErrorHook {
	return &ErrorHook{
		levels: []log.Level{
			log.WarnLevel,
			log.ErrorLevel,
			log.FatalLevel,
			log.PanicLevel,
		},
		out:       out,
		formatter: NewLogFormatter(),
	}
}

// Logger presents a logger.
type Logger struct {
	origLogger *log.Logger

	out    io.Writer
	errOut io.Writer

	bufferedOut    Flusher
	bufferedErrOut Flusher
}

// Flusher defines a interface with Flush() method.
type Flusher interface {
	Flush()
}

// GetLevel get the log level string.
func (l *Logger) GetLevel() string {
	return l.origLogger.Level.String()
}

// SetLevel sets the log level. Valid levels are "debug", "info", "warn", "error", and "fatal".
func (l *Logger) SetLevel(level string) {
	lvl, err := log.ParseLevel(level)
	if err != nil {
		l.Fatal(fmt.Sprintf(`log level not valid: "%s"`, level))
	}
	l.origLogger.Level = lvl
}

// Flush writes buffered logs.
func (l *Logger) Flush() {
	if l.bufferedOut != nil {
		l.bufferedOut.Flush()
	}
	if l.bufferedErrOut != nil {
		l.bufferedErrOut.Flush()
	}
}

// Debug logs a message with severity DEBUG.
func (l *Logger) Debug(message string) {
	l.output(l.origLogger.Debug, message)
}

// Info logs a message with severity INFO.
func (l *Logger) Info(message string) {
	l.output(l.origLogger.Info, message)
}

// Warn logs a message with severity WARN.
func (l *Logger) Warn(message string) {
	l.output(l.origLogger.Warn, message)
}

// Error logs a message with severity ERROR.
func (l *Logger) Error(message string) {
	l.output(l.origLogger.Error, message)
}

// Fatal logs a message with severity ERROR followed by a call to os.Exit().
func (l *Logger) Fatal(message string) {
	l.output(l.origLogger.Fatal, message)
}

// Debugf logs a message with severity DEBUG in format.
func (l *Logger) Debugf(format string, v ...interface{}) {
	l.output(l.origLogger.Debug, format, v...)
}

// Infof logs a message with severity INFO in format.
func (l *Logger) Infof(format string, v ...interface{}) {
	l.output(l.origLogger.Info, format, v...)
}

// Warnf logs a message with severity WARN in format.
func (l *Logger) Warnf(format string, v ...interface{}) {
	l.output(l.origLogger.Warn, format, v...)
}

// Errorf logs a message with severity ERROR in format.
func (l *Logger) Errorf(format string, v ...interface{}) {
	l.output(l.origLogger.Error, format, v...)
}

// Fatalf logs a message with severity ERROR in format followed by a call to
// os.Exit().
func (l *Logger) Fatalf(format string, v ...interface{}) {
	l.output(l.origLogger.Fatal, format, v...)
}

func (l *Logger) output(origin func(...interface{}), formatOrMessage string, v ...interface{}) {
	if len(v) > 0 {
		origin(fmt.Sprintf(formatOrMessage, v...))
	} else {
		origin(formatOrMessage)
	}
}

// CheckLevel checks whether the log level is valid.
func CheckLevel(level string) error {
	if _, err := log.ParseLevel(level); err != nil {
		return fmt.Errorf(`log level not valid: "%s"`, level)
	}
	return nil
}

// NewFileLogger creates a logger that write into file.
func NewFileLogger(filePath string, level ...string) (*Logger, error) {
	return NewFileLoggerWithErr(filePath, "", level...)
}

// NewFileLoggerWithErr creates a logger that write into files.
func NewFileLoggerWithErr(filePath, errFilePath string, level ...string) (*Logger, error) {
	if err := checkDir(path.Dir(filePath)); err != nil {
		return nil, err
	}
	if errFilePath != "" {
		if err := checkDir(path.Dir(errFilePath)); err != nil {
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
		return NewLoggerWithErr(out, nil, level...)
	}
	return NewLoggerWithErr(out, errOut, level...)
}

// NewBufferedFileLogger creates a logger that write into file with buffer.
func NewBufferedFileLogger(filePath string, level ...string) (*Logger, error) {
	return NewBufferedFileLoggerWithErr(filePath, "", level...)
}

// NewBufferedFileLoggerWithErr creates a logger that write into files with buffer.
func NewBufferedFileLoggerWithErr(filePath, errFilePath string, level ...string) (*Logger, error) {
	if err := checkDir(path.Dir(filePath)); err != nil {
		return nil, err
	}
	if errFilePath != "" {
		if err := checkDir(path.Dir(errFilePath)); err != nil {
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
			case <-time.After(10 * time.Second):
				bufferedOut.Flush()
				if bufferedErrOut != nil {
					bufferedErrOut.Flush()
				}
			}
		}
	}()
	signal.Notify(c, syscall.SIGHUP)

	if bufferedErrOut == nil {
		return NewLoggerWithErr(bufferedOut, nil, level...)
	}
	return NewLoggerWithErr(bufferedOut, bufferedErrOut, level...)
}

// NewTerminalLogger creates a logger that write into terminal.
func NewTerminalLogger(level ...string) (*Logger, error) {
	return NewLogger(os.Stdout, level...)
}

// NewTerminalLoggerWithErr creates a logger that write into terminal.
func NewTerminalLoggerWithErr(level ...string) (*Logger, error) {
	return NewLoggerWithErr(os.Stdout, os.Stderr, level...)
}

// NewLogger creates a new logger for given out and level, and the level is
// optional.
func NewLogger(out io.Writer, level ...string) (*Logger, error) {
	return NewLoggerWithErr(out, nil, level...)
}

// NewLoggerWithErr creates a new logger for given out, err out, level, and the
// err out can be nil, and the level is optional.
func NewLoggerWithErr(out, errOut io.Writer, level ...string) (*Logger, error) {
	if out == nil {
		return nil, errors.New(`must specify the output for logger`)
	}
	l := &Logger{
		origLogger: &log.Logger{
			Out:       out,
			Formatter: NewLogFormatter(),
			Hooks:     log.LevelHooks{},
			Level:     log.WarnLevel,
		},
		out:    out,
		errOut: errOut,
	}

	if errOut != nil {
		l.origLogger.Hooks.Add(NewErrorHook(l.errOut))
	}

	if len(level) == 1 {
		if err := CheckLevel(level[0]); err != nil {
			return nil, err
		}
		l.SetLevel(level[0])
	}

	return l, nil
}

func checkDir(dir string) error {
	if info, err := os.Stat(dir); err != nil {
		return fmt.Errorf(`directory not exists: %s`, dir)
	} else if !info.IsDir() {
		return fmt.Errorf(`path is not directory: %s`, dir)
	}
	return nil
}
