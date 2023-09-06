// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information

package sync2

import (
	"errors"
	"os"
	"sync/atomic"
)

// sharedFile implements Read, WriteAt offset to the file with reference counting.
type sharedFile struct {
	file  *os.File
	read  int64
	write int64
	open  *int64 // number of handles open
}

// ReadAt implements io.Reader methods.
func (buf *sharedFile) Read(data []byte) (amount int, err error) {
	amount, err = buf.file.ReadAt(data, buf.read)
	buf.read += int64(amount)
	return amount, err
}

// WriteAt implements io.Writer methods.
func (buf *sharedFile) Write(data []byte) (amount int, err error) {
	amount, err = buf.file.WriteAt(data, buf.write)
	buf.write += int64(amount)
	return amount, err
}

// Close implements io.Closer methods.
func (buf *sharedFile) Close() error {
	if atomic.AddInt64(buf.open, -1) == 0 {
		return buf.file.Close()
	}
	return nil
}

type memoryBlock struct {
	offset int
	data   []byte
	next   *memoryBlock
}

// blockReader implements io.ReadCloser on a memoryBlock.
type blockReader struct {
	current *memoryBlock
	read    int
}

// blockWriter implements io.WriteCloser on a memoryBlock.
type blockWriter struct {
	current *memoryBlock
	write   int
}

var (
	errReaderPassedWriter = errors.New("block reader passed writer")
	errWriterMissingBlock = errors.New("block writer is missing a block")
)

// Reader implements io.Reader method.
func (buf *blockReader) Read(data []byte) (amount int, err error) {
	into := data
	for len(into) > 0 {
		cur := buf.current
		// if we don't have a block, we've finished the data
		if cur == nil {
			return amount, errReaderPassedWriter
		}

		// check whether we should proceed to the next block
		if buf.read-cur.offset >= len(cur.data) {
			buf.current = cur.next
			continue
		}

		// copy as much as we can
		n := copy(into, cur.data[buf.read-cur.offset:])

		into = into[n:]
		amount += n
		buf.read += n
	}
	return amount, nil
}

// Write implements io.Writer method.
func (buf *blockWriter) Write(data []byte) (amount int, err error) {
	out := data
	for len(out) > 0 {
		cur := buf.current
		// if we don't have a block, there's an error
		if cur == nil {
			return amount, errWriterMissingBlock
		}

		// we've reached end of the current block
		if buf.write-cur.offset >= len(cur.data) {
			cur.next = &memoryBlock{
				offset: cur.offset + len(cur.data),
				data:   make([]byte, len(cur.data)),
			}
			buf.current = cur.next
			continue
		}

		// copy as much as we can
		n := copy(cur.data[buf.write-cur.offset:], out)

		out = out[n:]
		amount += n
		buf.write += n
	}
	return amount, nil
}

// Close implements io.Closer methods.
func (buf *blockReader) Close() error { return nil }

// Close implements io.Closer methods.
func (buf *blockWriter) Close() error { return nil }
