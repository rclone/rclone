package reopen

import (
	"bufio"
	"io"
	"os"
	"sync"
	"time"
)

// Reopener interface defines something that can be reopened.
type Reopener interface {
	Reopen() error
}

// Writer is a writer that also can be reopened.
type Writer interface {
	Reopener
	io.Writer
}

// WriteCloser is a io.WriteCloser that can also be reopened.
type WriteCloser interface {
	Reopener
	io.WriteCloser
}

// FileWriter that can also be reopened.
type FileWriter struct {
	// Ensures close/reopen/write are not called at the same time, protects f
	mu   sync.Mutex
	f    *os.File
	mode os.FileMode
	name string
}

// Close calls the under lying File.Close().
func (f *FileWriter) Close() error {
	f.mu.Lock()
	err := f.f.Close()
	f.mu.Unlock()
	return err
}

// Reopen the file.
func (f *FileWriter) Reopen() error {
	f.mu.Lock()
	err := f.reopen()
	f.mu.Unlock()
	return err
}

// Write implements the stander io.Writer interface.
func (f *FileWriter) Write(p []byte) (int, error) {
	f.mu.Lock()
	n, err := f.f.Write(p)
	f.mu.Unlock()
	return n, err
}

// reopen with mutex free.
func (f *FileWriter) reopen() error {
	if f.f != nil {
		f.f.Close()
		f.f = nil
	}
	ff, err := os.OpenFile(f.name, os.O_WRONLY|os.O_APPEND|os.O_CREATE, f.mode)
	if err != nil {
		f.f = nil
		return err
	}
	f.f = ff

	return nil
}

// NewFileWriter opens a file for appending and writing and can be reopened.
// It is a ReopenWriteCloser...
func NewFileWriter(name string) (*FileWriter, error) {
	// Standard default mode
	return NewFileWriterMode(name, 0644)
}

// NewFileWriterMode opens a Reopener file with a specific permission.
func NewFileWriterMode(name string, mode os.FileMode) (*FileWriter, error) {
	writer := FileWriter{
		f:    nil,
		name: name,
		mode: mode,
	}
	err := writer.reopen()
	if err != nil {
		return nil, err
	}
	return &writer, nil
}

// BufferedFileWriter is buffer writer than can be reopened.
type BufferedFileWriter struct {
	mu         sync.Mutex
	OrigWriter *FileWriter
	BufWriter  *bufio.Writer
}

// Reopen implement Reopener.
func (bw *BufferedFileWriter) Reopen() error {
	bw.mu.Lock()
	bw.BufWriter.Flush()

	// Use non-mutex version since we are using this one.
	err := bw.OrigWriter.reopen()

	bw.BufWriter.Reset(io.Writer(bw.OrigWriter))
	bw.mu.Unlock()

	return err
}

// Close flushes the internal buffer and closes the destination file.
func (bw *BufferedFileWriter) Close() error {
	bw.mu.Lock()
	bw.BufWriter.Flush()
	bw.OrigWriter.f.Close()
	bw.mu.Unlock()
	return nil
}

// Write implements io.Writer (and reopen.Writer).
func (bw *BufferedFileWriter) Write(p []byte) (int, error) {
	bw.mu.Lock()
	n, err := bw.BufWriter.Write(p)

	// Special Case... if the used space in the buffer is LESS than
	// the input, then we did a flush in the middle of the line
	// and the full log line was not sent on its way.
	if bw.BufWriter.Buffered() < len(p) {
		bw.BufWriter.Flush()
	}

	bw.mu.Unlock()
	return n, err
}

// Flush flushes the buffer.
func (bw *BufferedFileWriter) Flush() (err error) {
	bw.mu.Lock()
	defer bw.mu.Unlock()

	if err = bw.BufWriter.Flush(); err != nil {
		return err
	}
	if err = bw.OrigWriter.f.Sync(); err != nil {
		return err
	}
	return
}

// flushDaemon periodically flushes the log file buffers.
func (bw *BufferedFileWriter) flushDaemon(interval time.Duration) {
	for range time.NewTicker(interval).C {
		bw.Flush()
	}
}

// NewBufferedFileWriter opens a buffered file that is periodically flushed.
func NewBufferedFileWriter(w *FileWriter) *BufferedFileWriter {
	return NewBufferedFileWriterSize(w, bufferSize, flushInterval)
}

// NewBufferedFileWriterSize opens a buffered file with the given size that is periodically
// flushed on the given interval.
func NewBufferedFileWriterSize(w *FileWriter, size int, flush time.Duration) *BufferedFileWriter {
	bw := BufferedFileWriter{
		OrigWriter: w,
		BufWriter:  bufio.NewWriterSize(w, size),
	}
	go bw.flushDaemon(flush)
	return &bw
}

const bufferSize = 256 * 1024
const flushInterval = 30 * time.Second
