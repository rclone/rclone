package log

import (
	"io"
	"time"

	"github.com/pengsrc/go-shared/buffer"
	"github.com/pengsrc/go-shared/convert"
)

// LevelWriter defines as interface a writer may implement in order
// to receive level information with payload.
type LevelWriter interface {
	io.Writer
	WriteLevel(level Level, message []byte) (n int, err error)
}

// Flusher defines a interface with Flush() method.
type Flusher interface {
	Flush() error
}

// StandardWriter implements io.Writer{} and LevelWriter{} interface.
type StandardWriter struct {
	w  io.Writer
	ew io.Writer // Writer for WARN, ERROR, FATAL, PANIC

	dl  Level // Default level
	pid int
}

// Write implements the io.Writer{} interface.
func (sw *StandardWriter) Write(p []byte) (n int, err error) {
	return sw.WriteLevel(sw.dl, p)
}

// WriteLevel implements the LevelWriter{} interface.
func (sw *StandardWriter) WriteLevel(level Level, message []byte) (n int, err error) {
	levelString := level.String()
	if len(levelString) == 4 {
		levelString = " " + levelString
	}

	buf := buffer.GlobalBytesPool().Get()
	defer buf.Free()

	buf.AppendString("[")
	buf.AppendTime(time.Now().UTC(), convert.ISO8601Milli)
	buf.AppendString(" #")
	buf.AppendInt(int64(sw.pid))
	buf.AppendString("] ")
	buf.AppendString(levelString)
	buf.AppendString(" -- : ")
	buf.AppendBytes(message)
	buf.AppendString("\n")

	if sw.ew != nil {
		if level > MuteLevel && level <= WarnLevel {
			n, err = sw.ew.Write(buf.Bytes())
			if err != nil {
				return
			}
		}
	}
	return sw.w.Write(buf.Bytes())
}

// Flush implements the Flusher{} interface.
func (sw *StandardWriter) Flush() (err error) {
	if flusher, ok := sw.w.(Flusher); ok {
		err = flusher.Flush()
		if err != nil {
			return err
		}
	}
	if sw.ew != nil {
		if flusher, ok := sw.ew.(Flusher); ok {
			err = flusher.Flush()
			return err
		}
	}
	return nil
}
