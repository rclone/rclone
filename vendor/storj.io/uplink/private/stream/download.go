// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stream

import (
	"context"
	"io"

	"storj.io/uplink/private/metaclient"
	"storj.io/uplink/private/storage/streams"
)

// Download implements Reader, Seeker and Closer for reading from stream.
type Download struct {
	ctx     context.Context
	info    metaclient.DownloadInfo
	streams *streams.Store
	reader  io.ReadCloser
	offset  int64
	length  int64
	closed  bool
}

// NewDownload creates new stream download.
func NewDownload(ctx context.Context, info metaclient.DownloadInfo, streams *streams.Store) *Download {
	return &Download{
		ctx:     ctx,
		info:    info,
		streams: streams,
		length:  info.Object.Size,
	}
}

// NewDownloadRange creates new stream range download with range from start to start+length.
func NewDownloadRange(ctx context.Context, info metaclient.DownloadInfo, streams *streams.Store, start, length int64) *Download {
	size := info.Object.Size
	if start > size {
		start = size
	}
	if length < 0 || length+start > size {
		length = size - start
	}
	return &Download{
		ctx:     ctx,
		info:    info,
		streams: streams,
		offset:  start,
		length:  length,
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

	if download.length <= 0 {
		return 0, io.EOF
	}
	if download.length < int64(len(data)) {
		data = data[:download.length]
	}
	n, err = download.reader.Read(data)
	download.length -= int64(n)
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

	obj := download.info.Object

	rr, err := download.streams.Get(download.ctx, obj.Bucket.Name, obj.Path, download.info)
	if err != nil {
		return err
	}

	download.reader, err = rr.Range(download.ctx, download.offset, download.length)
	if err != nil {
		return err
	}

	return nil
}
