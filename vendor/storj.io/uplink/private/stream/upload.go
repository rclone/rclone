// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stream

import (
	"context"
	"io"
	"sync"

	"github.com/zeebo/errs"
	"golang.org/x/sync/errgroup"

	"storj.io/uplink/private/metaclient"
	"storj.io/uplink/private/storage/streams"
)

// Upload implements Writer and Closer for writing to stream.
type Upload struct {
	ctx      context.Context
	stream   *metaclient.MutableStream
	streams  *streams.Store
	writer   *io.PipeWriter
	errgroup errgroup.Group

	// mu protects closed
	mu     sync.Mutex
	closed bool

	// metamu protects meta
	metamu sync.RWMutex
	meta   *streams.Meta
}

// NewUpload creates new stream upload.
func NewUpload(ctx context.Context, stream *metaclient.MutableStream, streamsStore *streams.Store) *Upload {
	reader, writer := io.Pipe()

	upload := Upload{
		ctx:     ctx,
		stream:  stream,
		streams: streamsStore,
		writer:  writer,
	}

	upload.errgroup.Go(func() error {
		m, err := streamsStore.Put(ctx, stream.BucketName(), stream.Path(), reader, stream, stream.Expires())
		if err != nil {
			err = Error.Wrap(err)
			return errs.Combine(err, reader.CloseWithError(err))
		}

		upload.metamu.Lock()
		upload.meta = &m
		upload.metamu.Unlock()

		return nil
	})

	return &upload
}

// close transitions the upload to being closed and returns an error
// if it is already closed.
func (upload *Upload) close() error {
	upload.mu.Lock()
	defer upload.mu.Unlock()

	if upload.closed {
		return Error.New("already closed")
	}

	upload.closed = true
	return nil
}

// isClosed returns true if the upload is already closed.
func (upload *Upload) isClosed() bool {
	upload.mu.Lock()
	defer upload.mu.Unlock()

	return upload.closed
}

// Write writes len(data) bytes from data to the underlying data stream.
//
// See io.Writer for more details.
func (upload *Upload) Write(data []byte) (n int, err error) {
	if upload.isClosed() {
		return 0, Error.New("already closed")
	}
	return upload.writer.Write(data)
}

// Commit closes the stream and releases the underlying resources.
func (upload *Upload) Commit() error {
	if err := upload.close(); err != nil {
		return err
	}

	// Wait for our launched goroutine to return.
	return errs.Combine(
		upload.writer.Close(),
		upload.errgroup.Wait(),
	)
}

// Abort closes the stream with an error so that it does not successfully commit and
// releases the underlying resources.
func (upload *Upload) Abort() error {
	if err := upload.close(); err != nil {
		return err
	}

	// Wait for our launched goroutine to return. We do not need any of the
	// errors from the abort because they will just be things stating that
	// it was aborted.
	_ = upload.writer.CloseWithError(Error.New("aborted"))
	_ = upload.errgroup.Wait()

	return nil
}

// Meta returns the metadata of the uploaded object.
//
// Will return nil if the upload is still in progress.
func (upload *Upload) Meta() *streams.Meta {
	upload.metamu.RLock()
	defer upload.metamu.RUnlock()

	// we can safely return the pointer because it doesn't change after the
	// upload finishes
	return upload.meta
}
