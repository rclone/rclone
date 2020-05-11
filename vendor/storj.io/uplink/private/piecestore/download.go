// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"context"
	"fmt"
	"io"

	"github.com/zeebo/errs"

	"storj.io/common/errs2"
	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/signing"
	"storj.io/common/storj"
)

// Downloader is interface that can be used for downloading content.
// It matches signature of `io.ReadCloser`, with one extra function,
// GetHashAndLimit(), used for accessing information during GET_REPAIR.
type Downloader interface {
	Read([]byte) (int, error)
	Close() error
	GetHashAndLimit() (*pb.PieceHash, *pb.OrderLimit)
}

// Download implements downloading from a piecestore.
type Download struct {
	client     *Client
	limit      *pb.OrderLimit
	privateKey storj.PiecePrivateKey
	peer       *identity.PeerIdentity
	stream     downloadStream
	ctx        context.Context

	read         int64 // how much data we have read so far
	allocated    int64 // how far have we sent orders
	downloaded   int64 // how much data have we downloaded
	downloadSize int64 // how much do we want to download

	// what is the step we consider to upload
	allocationStep int64

	unread ReadBuffer

	// hash and originLimit are received in the event of a GET_REPAIR
	hash        *pb.PieceHash
	originLimit *pb.OrderLimit

	closed       bool
	closingError error
}

type downloadStream interface {
	CloseSend() error
	Send(*pb.PieceDownloadRequest) error
	Recv() (*pb.PieceDownloadResponse, error)
}

// Download starts a new download using the specified order limit at the specified offset and size.
func (client *Client) Download(ctx context.Context, limit *pb.OrderLimit, piecePrivateKey storj.PiecePrivateKey, offset, size int64) (_ Downloader, err error) {
	defer mon.Task()(&ctx)(&err)

	peer, err := client.conn.PeerIdentity()
	if err != nil {
		return nil, ErrInternal.Wrap(err)
	}

	stream, err := client.client.Download(ctx)
	if err != nil {
		return nil, err
	}

	err = stream.Send(&pb.PieceDownloadRequest{
		Limit: limit,
		Chunk: &pb.PieceDownloadRequest_Chunk{
			Offset:    offset,
			ChunkSize: size,
		},
	})
	if err != nil {
		_, recvErr := stream.Recv()
		return nil, ErrProtocol.Wrap(errs.Combine(err, recvErr))
	}

	download := &Download{
		client:     client,
		limit:      limit,
		privateKey: piecePrivateKey,
		peer:       peer,
		stream:     stream,
		ctx:        ctx,

		read: 0,

		allocated:    0,
		downloaded:   0,
		downloadSize: size,

		allocationStep: client.config.InitialStep,
	}

	if client.config.DownloadBufferSize <= 0 {
		return &LockingDownload{download: download}, nil
	}
	return &LockingDownload{
		download: NewBufferedDownload(download, int(client.config.DownloadBufferSize)),
	}, nil
}

// Read downloads data from the storage node allocating as necessary.
func (client *Download) Read(data []byte) (read int, err error) {
	ctx := client.ctx
	defer mon.Task()(&ctx, "node: "+client.peer.ID.String()[0:8])(&err)

	if client.closed {
		return 0, io.ErrClosedPipe
	}

	for client.read < client.downloadSize {
		// read from buffer
		n, err := client.unread.Read(data)
		client.read += int64(n)
		read += n

		// if we have an error return the error
		if err != nil {
			return read, err
		}
		// if we are pending for an error, avoid further requests, but try to finish what's in unread buffer.
		if client.unread.Errored() {
			return read, nil
		}

		// do we need to send a new order to storagenode
		if client.allocated-client.downloaded < client.allocationStep {
			newAllocation := client.allocationStep

			// have we downloaded more than we have allocated due to a generous storagenode?
			if client.allocated-client.downloaded < 0 {
				newAllocation += client.downloaded - client.allocated
			}

			// ensure we don't allocate more than we intend to read
			if client.allocated+newAllocation > client.downloadSize {
				newAllocation = client.downloadSize - client.allocated
			}

			// send an order
			if newAllocation > 0 {
				order, err := signing.SignUplinkOrder(ctx, client.privateKey, &pb.Order{
					SerialNumber: client.limit.SerialNumber,
					Amount:       newAllocation,
				})
				if err != nil {
					// we are signing so we shouldn't propagate this into close,
					// however we should include this as a read error
					client.unread.IncludeError(err)
					client.closeWithError(nil)
					return read, nil
				}

				err = client.stream.Send(&pb.PieceDownloadRequest{
					Order: order,
				})
				if err != nil {
					// other side doesn't want to talk to us anymore or network went down
					client.unread.IncludeError(err)
					// if it's a cancellation, then we'll just close with context.Canceled
					if errs2.IsCanceled(err) {
						client.closeWithError(err)
						return read, err
					}
					// otherwise, something else happened and we should try to ask the other side
					client.closeAndTryFetchError()
					return read, nil
				}

				// update our allocation step
				client.allocationStep = client.client.nextAllocationStep(client.allocationStep)
			}
		}

		// we have data, no need to wait for a chunk
		if read > 0 {
			return read, nil
		}

		// we don't have data, wait for a chunk from storage node
		response, err := client.stream.Recv()
		if response != nil && response.Chunk != nil {
			client.downloaded += int64(len(response.Chunk.Data))
			client.unread.Fill(response.Chunk.Data)
		}
		// This is a GET_REPAIR because we got a piece hash and the original order limit.
		if response != nil && response.Hash != nil && response.Limit != nil {
			client.hash = response.Hash
			client.originLimit = response.Limit
		}

		// we may have some data buffered, so we cannot immediately return the error
		// we'll queue the error and use the received error as the closing error
		if err != nil {
			client.unread.IncludeError(err)
			client.handleClosingError(err)
		}
	}

	// all downloaded
	if read == 0 {
		return 0, io.EOF
	}
	return read, nil
}

// handleClosingError should be used for an error that also closed the stream.
func (client *Download) handleClosingError(err error) {
	if client.closed {
		return
	}
	client.closed = true
	client.closingError = err
}

// closeWithError is used when we include the err in the closing error and also close the stream.
func (client *Download) closeWithError(err error) {
	if client.closed {
		return
	}
	client.closed = true
	client.closingError = errs.Combine(err, client.stream.CloseSend())
}

// closeAndTryFetchError closes the stream and also tries to fetch the actual error from the stream.
func (client *Download) closeAndTryFetchError() {
	if client.closed {
		return
	}
	client.closed = true

	client.closingError = client.stream.CloseSend()
	if client.closingError == nil || client.closingError == io.EOF {
		_, client.closingError = client.stream.Recv()
	}
}

// Close closes the downloading.
func (client *Download) Close() (err error) {
	defer func() {
		if err != nil {
			details := errs.Class(fmt.Sprintf("(Node ID: %s, Piece ID: %s)", client.peer.ID.String(), client.limit.PieceId.String()))
			err = details.Wrap(err)
			err = Error.Wrap(err)
		}
	}()

	client.closeWithError(nil)
	return client.closingError
}

// GetHashAndLimit gets the download's hash and original order limit.
func (client *Download) GetHashAndLimit() (*pb.PieceHash, *pb.OrderLimit) {
	return client.hash, client.originLimit
}

// ReadBuffer implements buffered reading with an error.
type ReadBuffer struct {
	data []byte
	err  error
}

// Error returns an error if it was encountered.
func (buffer *ReadBuffer) Error() error { return buffer.err }

// Errored returns whether the buffer contains an error.
func (buffer *ReadBuffer) Errored() bool { return buffer.err != nil }

// Empty checks whether buffer needs to be filled.
func (buffer *ReadBuffer) Empty() bool {
	return len(buffer.data) == 0 && buffer.err == nil
}

// IncludeError adds error at the end of the buffer.
func (buffer *ReadBuffer) IncludeError(err error) {
	buffer.err = errs.Combine(buffer.err, err)
}

// Fill fills the buffer with the specified bytes.
func (buffer *ReadBuffer) Fill(data []byte) {
	buffer.data = data
}

// Read reads from the buffer.
func (buffer *ReadBuffer) Read(data []byte) (n int, err error) {
	if len(buffer.data) > 0 {
		n = copy(data, buffer.data)
		buffer.data = buffer.data[n:]
		return n, nil
	}

	if buffer.err != nil {
		return 0, buffer.err
	}

	return 0, nil
}
