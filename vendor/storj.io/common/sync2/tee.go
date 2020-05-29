// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package sync2

import (
	"io"
	"sync"
	"sync/atomic"

	"github.com/calebcase/tmpfile"
)

type tee struct {
	noCopy noCopy // nolint: structcheck

	buffer ReadAtWriteAtCloser
	open   *int64

	mu       sync.Mutex
	nodata   sync.Cond
	noreader sync.Cond

	maxRead int64
	write   int64

	writerDone bool
	writerErr  error
}

// NewTeeFile returns a tee that uses file-system to offload memory.
func NewTeeFile(readers int, tempdir string) ([]PipeReader, PipeWriter, error) {
	file, err := tmpfile.New(tempdir, "tee")
	if err != nil {
		return nil, nil, err
	}

	handles := int64(readers + 1) // +1 for the writer

	buffer := &offsetFile{
		file: file,
		open: &handles,
	}

	return newTee(buffer, readers, &handles)
}

// NewTeeInmemory returns a tee that uses inmemory.
func NewTeeInmemory(readers int, allocMemory int64) ([]PipeReader, PipeWriter, error) {
	handles := int64(readers + 1) // +1 for the writer
	memory := memory(make([]byte, allocMemory))
	return newTee(memory, readers, &handles)
}

func newTee(buffer ReadAtWriteAtCloser, readers int, open *int64) ([]PipeReader, PipeWriter, error) {
	tee := &tee{
		buffer: buffer,
		open:   open,
	}
	tee.nodata.L = &tee.mu
	tee.noreader.L = &tee.mu

	teeReaders := make([]PipeReader, readers)
	for i := 0; i < readers; i++ {
		teeReaders[i] = &teeReader{tee: tee}
	}

	return teeReaders, &teeWriter{tee}, nil
}

type teeReader struct {
	tee    *tee
	pos    int64
	closed int32
}

type teeWriter struct{ tee *tee }

// Read reads from the tee returning io.EOF when writer is closed or bufSize is reached.
//
// It will block if the writer has not provided the data yet.
func (reader *teeReader) Read(data []byte) (n int, err error) {
	tee := reader.tee
	tee.mu.Lock()

	// fail fast on writer error
	if tee.writerErr != nil && tee.writerErr != io.EOF {
		tee.mu.Unlock()
		return 0, tee.writerErr
	}

	toRead := int64(len(data))
	end := reader.pos + toRead

	if end > tee.maxRead {
		tee.maxRead = end
		tee.noreader.Broadcast()
	}

	// wait until we have any data to read
	for reader.pos >= tee.write {
		// has the writer finished?
		if tee.writerDone {
			tee.mu.Unlock()
			return 0, tee.writerErr
		}

		// ok, let's wait
		tee.nodata.Wait()
	}

	// how much there's available for reading
	canRead := tee.write - reader.pos
	if toRead > canRead {
		toRead = canRead
	}
	readAt := reader.pos
	tee.mu.Unlock()

	// read data
	readAmount, err := tee.buffer.ReadAt(data[:toRead], readAt)
	reader.pos += int64(readAmount)

	return readAmount, err
}

// Write writes to the buffer returning io.ErrClosedPipe when limit is reached.
//
// It will block until at least one reader require the data.
func (writer *teeWriter) Write(data []byte) (n int, err error) {
	tee := writer.tee
	tee.mu.Lock()

	// have we closed already
	if tee.writerDone {
		tee.mu.Unlock()
		return 0, io.ErrClosedPipe
	}

	for tee.write > tee.maxRead {
		// are all readers already closed?
		if atomic.LoadInt64(tee.open) <= 1 {
			tee.mu.Unlock()
			return 0, io.ErrClosedPipe
		}
		// wait until new data is required by any reader
		tee.noreader.Wait()
	}

	writeAt := tee.write
	tee.mu.Unlock()

	// write data to buffer
	writeAmount, err := tee.buffer.WriteAt(data, writeAt)

	tee.mu.Lock()
	// update writing head
	tee.write += int64(writeAmount)
	// wake up reader
	tee.nodata.Broadcast()
	tee.mu.Unlock()

	return writeAmount, err
}

// Close implements io.Reader Close.
func (reader *teeReader) Close() error { return reader.CloseWithError(nil) }

// Close implements io.Writer Close.
func (writer *teeWriter) Close() error { return writer.CloseWithError(nil) }

// CloseWithError implements closing with error.
func (reader *teeReader) CloseWithError(reason error) (err error) {
	tee := reader.tee
	if atomic.CompareAndSwapInt32(&reader.closed, 0, 1) {
		err = tee.buffer.Close()
	}

	tee.mu.Lock()
	tee.noreader.Broadcast()
	tee.mu.Unlock()

	return err
}

// CloseWithError implements closing with error.
func (writer *teeWriter) CloseWithError(reason error) error {
	if reason == nil {
		reason = io.EOF
	}

	tee := writer.tee
	tee.mu.Lock()
	if tee.writerDone {
		tee.mu.Unlock()
		return io.ErrClosedPipe
	}
	tee.writerDone = true
	tee.writerErr = reason
	tee.nodata.Broadcast()
	tee.mu.Unlock()

	return tee.buffer.Close()
}
