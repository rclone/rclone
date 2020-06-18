// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stream

import (
	"context"
	"io"
	"sync"

	"github.com/zeebo/errs"
	"golang.org/x/sync/errgroup"

	"storj.io/common/storj"
	"storj.io/uplink/private/metainfo"
	"storj.io/uplink/private/storage/streams"
)

// Upload implements Writer and Closer for writing to stream.
type Upload struct {
	ctx      context.Context
	stream   metainfo.MutableStream
	streams  streams.Store
	writer   io.WriteCloser
	closed   bool
	errgroup errgroup.Group

	// mu protects meta
	mu   sync.RWMutex
	meta *streams.Meta
}

// NewUpload creates new stream upload.
func NewUpload(ctx context.Context, stream metainfo.MutableStream, streams streams.Store) *Upload {
	reader, writer := io.Pipe()

	upload := Upload{
		ctx:     ctx,
		stream:  stream,
		streams: streams,
		writer:  writer,
	}

	upload.errgroup.Go(func() error {
		m, err := streams.Put(ctx, storj.JoinPaths(stream.BucketName(), stream.Path()), reader, stream, stream.Expires())
		if err != nil {
			return errs.Combine(err, reader.CloseWithError(err))
		}

		upload.mu.Lock()
		upload.meta = &m
		upload.mu.Unlock()

		return nil
	})

	return &upload
}

// Write writes len(data) bytes from data to the underlying data stream.
//
// See io.Writer for more details.
func (upload *Upload) Write(data []byte) (n int, err error) {
	if upload.closed {
		return 0, Error.New("already closed")
	}

	return upload.writer.Write(data)
}

// Close closes the stream and releases the underlying resources.
func (upload *Upload) Close() error {
	if upload.closed {
		return Error.New("already closed")
	}

	upload.closed = true

	err := upload.writer.Close()

	// Wait for streams.Put to commit the upload to the PointerDB
	return errs.Combine(err, upload.errgroup.Wait())
}

// Meta returns the metadata of the uploaded object.
//
// Will return nil if the upload is still in progress.
func (upload *Upload) Meta() *streams.Meta {
	upload.mu.RLock()
	defer upload.mu.RUnlock()

	// we can safely return the pointer because it doesn't change after the
	// upload finishes
	return upload.meta
}
