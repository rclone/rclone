// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stream

import (
	"context"
	"io"

	"storj.io/common/storj"
	"storj.io/uplink/private/metainfo"
	"storj.io/uplink/private/storage/streams"
)

// Download implements Reader, Seeker and Closer for reading from stream.
type Download struct {
	ctx     context.Context
	stream  metainfo.ReadOnlyStream
	streams streams.Store
	reader  io.ReadCloser
	offset  int64
	limit   int64
	closed  bool
}

// NewDownload creates new stream download.
func NewDownload(ctx context.Context, stream metainfo.ReadOnlyStream, streams streams.Store) *Download {
	return &Download{
		ctx:     ctx,
		stream:  stream,
		streams: streams,
		limit:   stream.Info().Size,
	}
}

// NewDownloadRange creates new stream range download with range from offset to offset+limit.
func NewDownloadRange(ctx context.Context, stream metainfo.ReadOnlyStream, streams streams.Store, offset, limit int64) *Download {
	size := stream.Info().Size
	if offset > size {
		offset = size
	}
	if limit < 0 || limit+offset > size {
		limit = size - offset
	}
	return &Download{
		ctx:     ctx,
		stream:  stream,
		streams: streams,
		offset:  offset,
		limit:   limit,
	}
}

// Read reads up to len(data) bytes into data.
//
// If this is the first call it will read from the beginning of the stream.
// Use Seek to change the current offset for the next Read call.
//
// See io.Reader for more details.
func (download *Download) Read(data []byte) (n int, err error) {
	if download.closed {
		return 0, Error.New("already closed")
	}

	if download.reader == nil {
		err = download.resetReader()
		if err != nil {
			return 0, err
		}
	}

	if download.limit <= 0 {
		return 0, io.EOF
	}
	if download.limit < int64(len(data)) {
		data = data[:download.limit]
	}
	n, err = download.reader.Read(data)
	download.limit -= int64(n)
	download.offset += int64(n)

	return n, err
}

// Close closes the stream and releases the underlying resources.
func (download *Download) Close() error {
	if download.closed {
		return Error.New("already closed")
	}

	download.closed = true

	if download.reader == nil {
		return nil
	}

	return download.reader.Close()
}

func (download *Download) resetReader() error {
	if download.reader != nil {
		err := download.reader.Close()
		if err != nil {
			return err
		}
	}

	obj := download.stream.Info()

	rr, err := download.streams.Get(download.ctx, storj.JoinPaths(obj.Bucket.Name, obj.Path), obj)
	if err != nil {
		return err
	}

	download.reader, err = rr.Range(download.ctx, download.offset, download.limit)
	if err != nil {
		return err
	}

	return nil
}
