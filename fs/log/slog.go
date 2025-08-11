// Interfaces for the slog package

package log

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
)

// Handler is the standard handler for the logging.
var Handler = defaultHandler()

// Create the default OutputHandler
//
// This logs to stderr with standard go logger format at level INFO.
//
// This will be adjusted by InitLogging to be the configured levels
// but it is important we have a logger running regardless of whether
// InitLogging has been called yet or not.
func defaultHandler() *OutputHandler {
	// Default options for default handler
	opts := &slog.HandlerOptions{
		Level: fs.LogLevelToSlog(fs.InitialLogLevel()),
	}

	// Create our handler
	h := NewOutputHandler(os.Stderr, opts, logFormatDate|logFormatTime)

	// Set the slog default handler
	slog.SetDefault(slog.New(h))

	// Make log.Printf logs at level Notice
	slog.SetLogLoggerLevel(fs.SlogLevelNotice)

	return h
}

// Map slog level names to string
var slogNames = map[slog.Level]string{
	slog.LevelDebug:       "DEBUG",
	slog.LevelInfo:        "INFO",
	fs.SlogLevelNotice:    "NOTICE",
	slog.LevelWarn:        "WARNING",
	slog.LevelError:       "ERROR",
	fs.SlogLevelCritical:  "CRITICAL",
	fs.SlogLevelAlert:     "ALERT",
	fs.SlogLevelEmergency: "EMERGENCY",
}

// Convert a slog level to string using rclone's extra levels
func slogLevelToString(level slog.Level) string {
	levelStr := slogNames[level]
	if levelStr == "" {
		levelStr = level.String()
	}
	return levelStr
}

// ReplaceAttr function to customize the level key's string value in logs
func mapLogLevelNames(groups []string, a slog.Attr) slog.Attr {
	if a.Key == slog.LevelKey {
		level, ok := a.Value.Any().(slog.Level)
		if !ok {
			return a
		}
		levelStr := strings.ToLower(slogLevelToString(level))
		a.Value = slog.StringValue(levelStr)
	}
	return a
}

// get the file and line number of the caller skipping skip levels
func getCaller(skip int) string {
	var pc [64]uintptr
	n := runtime.Callers(skip, pc[:])
	if n == 0 {
		return ""
	}
	frames := runtime.CallersFrames(pc[:n])
	more := true
	var frame runtime.Frame
	for more {
		frame, more = frames.Next()

		file := frame.File
		if strings.Contains(file, "/log/") || strings.HasSuffix(file, "log.go") {
			continue
		}
		line := frame.Line

		// shorten file name
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
		return fmt.Sprintf("%s:%d", file, line)
	}
	return ""
}

// OutputHandler is a slog.Handler that writes log records in a format
// identical to the standard library's `log` package (e.g., "YYYY/MM/DD HH:MM:SS message").
//
// It can also write logs in JSON format identical to logrus.
type OutputHandler struct {
	opts        slog.HandlerOptions
	levelVar    slog.LevelVar
	writer      io.Writer
	mu          sync.Mutex
	output      []outputFn    // log to writer if empty or the last item
	outputExtra []outputExtra // log to all these additional places
	format      logFormat
	jsonBuf     bytes.Buffer
	jsonHandler *slog.JSONHandler
}

// Records the type and function pointer for extra logging output.
type outputExtra struct {
	json   bool
	output outputFn
}

// Define the type of the override logger
type outputFn func(level slog.Level, text string)

// NewOutputHandler creates a new OutputHandler with the specified flags.
//
// This is designed to use log/slog but produce output which is
// backwards compatible with previous rclone versions.
//
// If opts is nil, default options are used, with Level set to
// slog.LevelInfo.
func NewOutputHandler(out io.Writer, opts *slog.HandlerOptions, format logFormat) *OutputHandler {
	h := &OutputHandler{
		writer: out,
		format: format,
	}
	if opts != nil {
		h.opts = *opts
	}
	if h.opts.Level == nil {
		h.opts.Level = slog.LevelInfo
	}
	// Set the level var with the configured level
	h.levelVar.Set(h.opts.Level.Level())
	// And use it from now on
	h.opts.Level = &h.levelVar

	// Create the JSON logger in case we need it
	jsonOpts := slog.HandlerOptions{
		Level:       h.opts.Level,
		ReplaceAttr: mapLogLevelNames,
	}
	h.jsonHandler = slog.NewJSONHandler(&h.jsonBuf, &jsonOpts)
	return h
}

// SetOutput sets a new output handler for the log output.
//
// This is for temporarily overriding the output.
func (h *OutputHandler) SetOutput(fn outputFn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.output = append(h.output, fn)
}

// ResetOutput resets the log output to what is was.
func (h *OutputHandler) ResetOutput() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.output) > 0 {
		h.output = h.output[:len(h.output)-1]
	}
}

// AddOutput adds an additional logging destination of the type specified.
func (h *OutputHandler) AddOutput(json bool, fn outputFn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.outputExtra = append(h.outputExtra, outputExtra{
		json:   json,
		output: fn,
	})
}

// SetLevel sets a new log level, returning the old one.
func (h *OutputHandler) SetLevel(level slog.Level) slog.Level {
	h.mu.Lock()
	defer h.mu.Unlock()
	oldLevel := h.levelVar.Level()
	h.levelVar.Set(level)
	return oldLevel
}

// Set the writer for the log to that passed.
func (h *OutputHandler) setWriter(writer io.Writer) {
	h.writer = writer
}

// Set the format flags to that passed in.
func (h *OutputHandler) setFormat(format logFormat) {
	h.format = format
}

// clear format flags that this output type doesn't want
func (h *OutputHandler) clearFormatFlags(bitMask logFormat) {
	h.format &^= bitMask
}

// set format flags that this output type requires
func (h *OutputHandler) setFormatFlags(bitMask logFormat) {
	h.format |= bitMask
}

// Enabled returns whether this logger is enabled for this level.
func (h *OutputHandler) Enabled(_ context.Context, level slog.Level) bool {
	minLevel := slog.LevelInfo
	if h.opts.Level != nil {
		minLevel = h.opts.Level.Level()
	}
	return level >= minLevel
}

// Create a log header in Go standard log format.
func (h *OutputHandler) formatStdLogHeader(buf *bytes.Buffer, level slog.Level, t time.Time, object string, lineInfo string) {
	// Add time in Go standard format if requested
	if h.format&(logFormatDate|logFormatTime|logFormatMicroseconds) != 0 {
		if h.format&logFormatUTC != 0 {
			t = t.UTC()
		}
		if h.format&logFormatDate != 0 {
			year, month, day := t.Date()
			fmt.Fprintf(buf, "%04d/%02d/%02d ", year, month, day)
		}
		if h.format&(logFormatTime|logFormatMicroseconds) != 0 {
			hour, min, sec := t.Clock()
			fmt.Fprintf(buf, "%02d:%02d:%02d", hour, min, sec)
			if h.format&logFormatMicroseconds != 0 {
				fmt.Fprintf(buf, ".%06d", t.Nanosecond()/1e3)
			}
			buf.WriteByte(' ')
		}
	}
	// Add source code filename:line if requested
	if h.format&(logFormatShortFile|logFormatLongFile) != 0 && lineInfo != "" {
		buf.WriteString(lineInfo)
		buf.WriteByte(':')
		buf.WriteByte(' ')
	}
	// Add PID if requested
	if h.format&logFormatPid != 0 {
		fmt.Fprintf(buf, "[%d] ", os.Getpid())
	}
	// Add log level if required
	if h.format&logFormatNoLevel == 0 {
		levelStr := slogLevelToString(level)
		fmt.Fprintf(buf, "%-6s: ", levelStr)
	}
	// Add object if passed
	if object != "" {
		buf.WriteString(object)
		buf.WriteByte(':')
		buf.WriteByte(' ')
	}
}

// Create a log in standard Go log format into buf.
func (h *OutputHandler) textLog(ctx context.Context, buf *bytes.Buffer, r slog.Record) error {
	var lineInfo string
	if h.format&(logFormatShortFile|logFormatLongFile) != 0 {
		lineInfo = getCaller(2)
	}

	var object string
	r.Attrs(func(attr slog.Attr) bool {
		if attr.Key == "object" {
			object = attr.Value.String()
			return false
		}
		return true
	})

	h.formatStdLogHeader(buf, r.Level, r.Time, object, lineInfo)
	buf.WriteString(r.Message)
	if buf.Len() == 0 || buf.Bytes()[buf.Len()-1] != '\n' { // Ensure newline
		buf.WriteByte('\n')
	}
	return nil
}

// Create a log in JSON format into buf.
func (h *OutputHandler) jsonLog(ctx context.Context, buf *bytes.Buffer, r slog.Record) (err error) {
	// Call the JSON handler to create the JSON in buf
	r.AddAttrs(
		slog.String("source", getCaller(2)),
	)
	h.mu.Lock()
	err = h.jsonHandler.Handle(ctx, r)
	if err == nil {
		_, err = h.jsonBuf.WriteTo(buf)
	}
	h.mu.Unlock()
	return err
}

// Handle outputs a log in the current format
func (h *OutputHandler) Handle(ctx context.Context, r slog.Record) (err error) {
	var (
		bufJSON *bytes.Buffer
		bufText *bytes.Buffer
		buf     *bytes.Buffer
	)

	// Check whether we need to build Text or JSON logs or both
	needJSON := h.format&logFormatJSON != 0
	needText := !needJSON
	for _, out := range h.outputExtra {
		if out.json {
			needJSON = true
		} else {
			needText = true
		}
	}

	if needJSON {
		var bufJSONBack [256]byte
		bufJSON = bytes.NewBuffer(bufJSONBack[:0])
		err = h.jsonLog(ctx, bufJSON, r)
		if err != nil {
			return err
		}
	}

	if needText {
		var bufTextBack [256]byte
		bufText = bytes.NewBuffer(bufTextBack[:0])
		err = h.textLog(ctx, bufText, r)
		if err != nil {
			return err
		}
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Do the log, either to the default destination or to the alternate logging system
	if h.format&logFormatJSON != 0 {
		buf = bufJSON
	} else {
		buf = bufText
	}
	if len(h.output) > 0 {
		h.output[len(h.output)-1](r.Level, buf.String())
		err = nil
	} else {
		_, err = h.writer.Write(buf.Bytes())
	}

	// Log to any additional destinations required
	for _, out := range h.outputExtra {
		if out.json {
			out.output(r.Level, bufJSON.String())
		} else {
			out.output(r.Level, bufText.String())
		}
	}
	return err
}

// WithAttrs creates a new handler with the same writer, options, and flags.
// Attributes are ignored for the output format of this specific handler.
func (h *OutputHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return NewOutputHandler(h.writer, &h.opts, h.format)
}

// WithGroup creates a new handler with the same writer, options, and flags.
// Groups are ignored for the output format of this specific handler.
func (h *OutputHandler) WithGroup(name string) slog.Handler {
	return NewOutputHandler(h.writer, &h.opts, h.format)
}

// Check interface
var _ slog.Handler = (*OutputHandler)(nil)
