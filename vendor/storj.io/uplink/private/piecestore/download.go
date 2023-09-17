// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/context2"
	"storj.io/common/errs2"
	"storj.io/common/pb"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/sync2"
)

// Download implements downloading from a piecestore.
type Download struct {
	client     *Client
	limit      *pb.OrderLimit
	privateKey storj.PiecePrivateKey
	stream     downloadStream
	ctx        context.Context
	cancelCtx  func(error)

	offset       int64 // the offset into the piece
	read         int64 // how much data we have read so far
	allocated    int64 // how far have we sent orders
	downloaded   int64 // how much data have we downloaded
	downloadSize int64 // how much do we want to download

	downloadRequestSent bool

	// what is the step we consider to upload
	orderStep int64

	unread ReadBuffer

	// hash and originLimit are received in the event of a GET_REPAIR
	hash        *pb.PieceHash
	originLimit *pb.OrderLimit

	close        sync.Once
	closingError syncError
}

type downloadStream interface {
	Close() error
	Send(*pb.PieceDownloadRequest) error
	Recv() (*pb.PieceDownloadResponse, error)
}

var monClientDownloadTask = mon.Task()

// Download starts a new download using the specified order limit at the specified offset and size.
func (client *Client) Download(ctx context.Context, limit *pb.OrderLimit, piecePrivateKey storj.PiecePrivateKey, offset, size int64) (_ *Download, err error) {
	defer monClientDownloadTask(&ctx)(&err)

	ctx, cancel := context2.WithCustomCancel(ctx)

	var underlyingStream downloadStream
	sync2.WithTimeout(client.config.MessageTimeout, func() {
		if client.replaySafe != nil {
			underlyingStream, err = client.replaySafe.Download(ctx)
		} else {
			underlyingStream, err = client.client.Download(ctx)
		}
	}, func() {
		cancel(errMessageTimeout)
	})
	if err != nil {
		cancel(context.Canceled)
		return nil, err
	}
	stream := &timedDownloadStream{
		timeout: client.config.MessageTimeout,
		stream:  underlyingStream,
		cancel:  cancel,
	}

	return &Download{
		client:     client,
		limit:      limit,
		privateKey: piecePrivateKey,
		stream:     stream,
		ctx:        ctx,
		cancelCtx:  cancel,

		offset: offset,
		read:   0,

		allocated:    0,
		downloaded:   0,
		downloadSize: size,

		orderStep: client.config.InitialStep,
	}, nil
}

// Read downloads data from the storage node allocating as necessary.
func (client *Download) Read(data []byte) (read int, err error) {
	ctx := client.ctx
	defer mon.Task()(&ctx, "node: "+client.limit.StorageNodeId.String()[0:8])(&err)

	if client.closingError.IsSet() {
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
		notYetReceived := client.allocated - client.downloaded
		if notYetReceived < client.orderStep {
			newAllocation := client.orderStep

			// If we have downloaded more than we have allocated
			// due to a generous storagenode include this in the next allocation.
			if notYetReceived < 0 {
				newAllocation += -notYetReceived
			}

			// Ensure we don't allocate more than we intend to read.
			if client.allocated+newAllocation > client.downloadSize {
				newAllocation = client.downloadSize - client.allocated
			}

			// send an order
			if newAllocation > 0 {
				order, err := signing.SignUplinkOrder(ctx, client.privateKey, &pb.Order{
					SerialNumber: client.limit.SerialNumber,
					Amount:       client.allocated + newAllocation,
				})
				if err != nil {
					// we are signing so we shouldn't propagate this into close,
					// however we should include this as a read error
					client.unread.IncludeError(err)
					client.closeWithError(nil)
					return read, nil
				}

				err = func() error {
					if client.downloadRequestSent {
						return client.stream.Send(&pb.PieceDownloadRequest{
							Order: order,
						})
					}
					client.downloadRequestSent = true

					if client.client.NodeURL().NoiseInfo.Proto != storj.NoiseProto_Unset {
						// all nodes that have noise support also support
						// combining the order and the piece download request
						// into one protobuf.
						return client.stream.Send(&pb.PieceDownloadRequest{
							Limit: client.limit,
							Chunk: &pb.PieceDownloadRequest_Chunk{
								Offset:    client.offset,
								ChunkSize: client.downloadSize,
							},
							Order:            order,
							MaximumChunkSize: client.client.config.MaximumChunkSize,
						})
					}

					// nodes that don't support noise don't necessarily
					// support these combined messages, but also don't
					// benefit much from them being combined.
					err := client.stream.Send(&pb.PieceDownloadRequest{
						Limit: client.limit,
						Chunk: &pb.PieceDownloadRequest_Chunk{
							Offset:    client.offset,
							ChunkSize: client.downloadSize,
						},
						MaximumChunkSize: client.client.config.MaximumChunkSize,
					})
					if err != nil {
						return err
					}
					return client.stream.Send(&pb.PieceDownloadRequest{
						Order: order,
					})
				}()
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
				client.allocated += newAllocation
				client.orderStep = client.client.nextOrderStep(client.orderStep)
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
	client.close.Do(func() {
		client.closingError.Set(err)
		// ensure we close the connection
		_ = client.stream.Close()
	})
}

// closeWithError is used when we include the err in the closing error and also close the stream.
func (client *Download) closeWithError(err error) {
	client.close.Do(func() {
		err := errs.Combine(err, client.stream.Close())
		client.closingError.Set(err)
	})
}

// closeAndTryFetchError closes the stream and also tries to fetch the actual error from the stream.
func (client *Download) closeAndTryFetchError() {
	client.close.Do(func() {
		err := client.stream.Close()
		if err == nil || errors.Is(err, io.EOF) {
			// note, although, we close the stream, we'll try to fetch the error
			// from the current buffer.
			_, err = client.stream.Recv()
		}
		client.closingError.Set(err)
	})
}

// Close closes the downloading.
func (client *Download) Close() error {
	client.closeWithError(nil)
	client.cancelCtx(context.Canceled)

	err := client.closingError.Get()
	if err != nil {
		details := errs.Class(fmt.Sprintf("(Node ID: %s, Piece ID: %s)", client.limit.StorageNodeId.String(), client.limit.PieceId.String()))
		err = details.Wrap(Error.Wrap(err))
	}

	return err
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

// timedDownloadStream wraps downloadStream and adds timeouts
// to all operations.
type timedDownloadStream struct {
	timeout time.Duration
	stream  downloadStream
	cancel  func(error)
}

func (stream *timedDownloadStream) cancelTimeout() {
	stream.cancel(errMessageTimeout)
}

func (stream *timedDownloadStream) Close() (err error) {
	sync2.WithTimeout(stream.timeout, func() {
		err = stream.stream.Close()
	}, stream.cancelTimeout)
	return CloseError.Wrap(err)
}

func (stream *timedDownloadStream) Send(req *pb.PieceDownloadRequest) (err error) {
	sync2.WithTimeout(stream.timeout, func() {
		err = stream.stream.Send(req)
	}, stream.cancelTimeout)
	return err
}

func (stream *timedDownloadStream) Recv() (resp *pb.PieceDownloadResponse, err error) {
	sync2.WithTimeout(stream.timeout, func() {
		resp, err = stream.stream.Recv()
	}, stream.cancelTimeout)
	return resp, err
}

// syncError synchronizes access to an error and keeps
// track whether it has been set, even to nil.
type syncError struct {
	mu  sync.Mutex
	set bool
	err error
}

// IsSet returns whether `Set` has been called.
func (s *syncError) IsSet() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.set
}

// Set sets the error.
func (s *syncError) Set(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.set {
		return
	}
	s.set = true
	s.err = err
}

// Get gets the error.
func (s *syncError) Get() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}
