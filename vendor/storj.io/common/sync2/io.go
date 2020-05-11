// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package sync2

import (
	"io"
	"os"
	"sync/atomic"
)

// ReadAtWriteAtCloser implements all io.ReaderAt, io.WriterAt and io.Closer
type ReadAtWriteAtCloser interface {
	io.ReaderAt
	io.WriterAt
	io.Closer
}

// PipeWriter allows closing the writer with an error
type PipeWriter interface {
	io.WriteCloser
	CloseWithError(reason error) error
}

// PipeReader allows closing the reader with an error
type PipeReader interface {
	io.ReadCloser
	CloseWithError(reason error) error
}

// memory implements ReadAtWriteAtCloser on a memory buffer
type memory []byte

// Size returns size of memory buffer
func (memory memory) Size() int { return len(memory) }

// ReadAt implements io.ReaderAt methods
func (memory memory) ReadAt(data []byte, at int64) (amount int, err error) {
	if at > int64(len(memory)) {
		return 0, io.ErrClosedPipe
	}
	amount = copy(data, memory[at:])
	return amount, nil
}

// WriteAt implements io.WriterAt methods
func (memory memory) WriteAt(data []byte, at int64) (amount int, err error) {
	if at > int64(len(memory)) {
		return 0, io.ErrClosedPipe
	}
	amount = copy(memory[at:], data)
	return amount, nil
}

// Close implements io.Closer implementation
func (memory memory) Close() error { return nil }

// offsetFile implements ReadAt, WriteAt offset to the file with reference counting
type offsetFile struct {
	file   *os.File
	offset int64
	open   *int64 // number of handles open
}

// ReadAt implements io.ReaderAt methods
func (file offsetFile) ReadAt(data []byte, at int64) (amount int, err error) {
	return file.file.ReadAt(data, file.offset+at)
}

// WriteAt implements io.WriterAt methods
func (file offsetFile) WriteAt(data []byte, at int64) (amount int, err error) {
	return file.file.WriteAt(data, file.offset+at)
}

// Close implements io.Closer methods
func (file offsetFile) Close() error {
	if atomic.AddInt64(file.open, -1) == 0 {
		return file.file.Close()
	}
	return nil
}
