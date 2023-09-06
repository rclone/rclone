// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package encryption

import (
	"bytes"
	"context"
	"errors"
	"io"

	"github.com/spacemonkeygo/monkit/v3"

	"storj.io/common/ranger"
	"storj.io/common/readcloser"
)

var mon = monkit.Package()

// A Transformer is a data transformation that may change the size of the blocks
// of data it operates on in a deterministic fashion.
type Transformer interface {
	InBlockSize() int  // The block size prior to transformation
	OutBlockSize() int // The block size after transformation
	Transform(out, in []byte, blockNum int64) ([]byte, error)
}

type transformedReader struct {
	r            io.ReadCloser
	t            Transformer
	blockNum     int64
	inbuf        []byte
	outbuf       []byte
	expectedSize int64
	bytesRead    int
}

// NoopTransformer is a dummy Transformer that passes data through without modifying it.
type NoopTransformer struct{}

// InBlockSize is 1.
func (t *NoopTransformer) InBlockSize() int {
	return 1
}

// OutBlockSize is 1.
func (t *NoopTransformer) OutBlockSize() int {
	return 1
}

// Transform returns the input without modification.
func (t *NoopTransformer) Transform(out, in []byte, blockNum int64) ([]byte, error) {
	return append(out, in...), nil
}

// TransformReader applies a Transformer to a Reader. startingBlockNum should
// probably be 0 unless you know you're already starting at a block offset.
func TransformReader(r io.ReadCloser, t Transformer,
	startingBlockNum int64) io.ReadCloser {
	return &transformedReader{
		r:        r,
		t:        t,
		blockNum: startingBlockNum,
		inbuf:    make([]byte, t.InBlockSize()),
		outbuf:   make([]byte, 0, t.OutBlockSize()),
	}
}

// TransformReaderSize creates a TransformReader with expected size,
// i.e. the number of bytes that is expected to be read from this reader.
// If less than the expected bytes are read, the reader will return
// io.ErrUnexpectedEOF instead of io.EOF.
func TransformReaderSize(r io.ReadCloser, t Transformer,
	startingBlockNum int64, expectedSize int64) io.ReadCloser {
	return &transformedReader{
		r:            r,
		t:            t,
		blockNum:     startingBlockNum,
		inbuf:        make([]byte, t.InBlockSize()),
		outbuf:       make([]byte, 0, t.OutBlockSize()),
		expectedSize: expectedSize,
	}
}

func (t *transformedReader) Read(p []byte) (n int, err error) {
	if len(t.outbuf) == 0 {
		// If there's no more buffered data left, let's fill the buffer with
		// the next block
		b, err := io.ReadFull(t.r, t.inbuf)
		t.bytesRead += b
		if errors.Is(err, io.EOF) && int64(t.bytesRead) < t.expectedSize {
			return 0, io.ErrUnexpectedEOF
		} else if err != nil {
			return 0, err
		}
		t.outbuf, err = t.t.Transform(t.outbuf, t.inbuf, t.blockNum)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return 0, err
			}
			return 0, Error.Wrap(err)
		}
		t.blockNum++
	}

	// return as much as we can from the current buffered block
	n = copy(p, t.outbuf)
	// slide the uncopied data to the beginning of the buffer
	copy(t.outbuf, t.outbuf[n:])
	// resize the buffer
	t.outbuf = t.outbuf[:len(t.outbuf)-n]
	return n, nil
}

func (t *transformedReader) Close() error {
	return t.r.Close()
}

type transformedRanger struct {
	rr ranger.Ranger
	t  Transformer
}

// Transform will apply a Transformer to a Ranger.
func Transform(rr ranger.Ranger, t Transformer) (ranger.Ranger, error) {
	if rr.Size()%int64(t.InBlockSize()) != 0 {
		return nil, Error.New("invalid transformer and range reader combination." +
			"the range reader size is not a multiple of the block size")
	}
	return &transformedRanger{rr: rr, t: t}, nil
}

func (t *transformedRanger) Size() int64 {
	blocks := t.rr.Size() / int64(t.t.InBlockSize())
	return blocks * int64(t.t.OutBlockSize())
}

// CalcEncompassingBlocks is a useful helper function that, given an offset,
// length, and blockSize, will tell you which blocks contain the requested
// offset and length.
func CalcEncompassingBlocks(offset, length int64, blockSize int) (
	firstBlock, blockCount int64) {
	firstBlock = offset / int64(blockSize)
	if length <= 0 {
		return firstBlock, 0
	}
	lastBlock := (offset + length) / int64(blockSize)
	if (offset+length)%int64(blockSize) == 0 {
		return firstBlock, lastBlock - firstBlock
	}
	return firstBlock, 1 + lastBlock - firstBlock
}

func (t *transformedRanger) Range(ctx context.Context, offset, length int64) (_ io.ReadCloser, err error) {
	defer mon.Task()(&ctx)(&err)

	// Range may not have been called for block-aligned offsets and lengths, so
	// let's figure out which blocks encompass the request
	firstBlock, blockCount := CalcEncompassingBlocks(
		offset, length, t.t.OutBlockSize())
	// If block count is 0, there is nothing to transform, so return a dumb
	// reader that will just return io.EOF on read
	if blockCount == 0 {
		return io.NopCloser(bytes.NewReader(nil)), nil
	}
	// okay, now let's get the range on the underlying ranger for those blocks
	// and then Transform it.
	r, err := t.rr.Range(ctx,
		firstBlock*int64(t.t.InBlockSize()),
		blockCount*int64(t.t.InBlockSize()))
	if err != nil {
		return nil, err
	}
	tr := TransformReaderSize(r, t.t, firstBlock, blockCount*int64(t.t.InBlockSize()))
	// the range we got potentially includes more than we wanted. if the
	// offset started past the beginning of the first block, we need to
	// swallow the first few bytes
	_, err = io.CopyN(io.Discard, tr,
		offset-firstBlock*int64(t.t.OutBlockSize()))
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, io.ErrUnexpectedEOF
		}
		return nil, Error.Wrap(err)
	}
	// the range might have been too long. only return what was requested
	return readcloser.LimitReadCloser(tr, length), nil
}

// TransformWriterPadded applies a Transformer to a Writer. It also applies
// padding to the output bytes.
func TransformWriterPadded(w io.Writer, t Transformer) io.WriteCloser {
	inbuf := make([]byte, t.InBlockSize())
	return &transformedWriter{
		w:      w,
		t:      t,
		inbuf:  inbuf,
		cursor: inbuf,
		outbuf: make([]byte, 0, t.OutBlockSize()),
	}
}

type transformedWriter struct {
	w        io.Writer
	t        Transformer
	blockNum int64
	inbuf    []byte
	cursor   []byte
	outbuf   []byte
	closed   bool
	err      error
}

func (t *transformedWriter) storeErr(err error) error {
	t.err = err
	return err
}

func (t *transformedWriter) Write(p []byte) (n int, err error) {
	if t.err != nil {
		return 0, t.err
	} else if t.closed {
		return 0, Error.New("write after closed")
	}

	for len(p) > 0 {
		cn := copy(t.cursor, p)
		p = p[cn:]
		t.cursor = t.cursor[cn:]
		n += cn

		if len(t.cursor) == 0 {
			t.outbuf, err = t.t.Transform(t.outbuf[:0], t.inbuf, t.blockNum)
			if err != nil {
				return n, t.storeErr(Error.Wrap(err))
			}
			if _, err := t.w.Write(t.outbuf); err != nil {
				return n, t.storeErr(Error.Wrap(err))
			}
			t.cursor = t.inbuf
			t.blockNum++
		}
	}

	return n, nil
}

func (t *transformedWriter) Close() error {
	if t.err != nil {
		return t.err
	} else if t.closed {
		return nil
	}
	padding := makePadding(int64(len(t.inbuf))-int64(len(t.cursor)), len(t.inbuf))
	if _, err := t.Write(padding); err != nil {
		return t.storeErr(Error.Wrap(err))
	} else if len(t.cursor) != len(t.inbuf) {
		return t.storeErr(Error.New("programmer error: padding didn't cause output buffer flush"))
	}
	t.closed = true
	return nil
}
