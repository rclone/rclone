// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"context"
	"errors"
	"hash"
	"io"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"

	"storj.io/common/context2"
	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/drpc"
)

var mon = monkit.Package()

// Upload implements uploading to the storage node.
type upload struct {
	client     *Client
	limit      *pb.OrderLimit
	privateKey storj.PiecePrivateKey
	nodeID     storj.NodeID
	stream     uploadStream

	nextRequest *pb.PieceUploadRequest

	hash          hash.Hash // TODO: use concrete implementation
	hashAlgorithm pb.PieceHashAlgorithm
	offset        int64
	orderStep     int64

	// when there's a send error then it will automatically close
	finished bool
}

type uploadStream interface {
	Context() context.Context
	Close() error
	Send(*pb.PieceUploadRequest) error
	CloseAndRecv() (*pb.PieceUploadResponse, error)
}

// UploadReader uploads to the storage node.
func (client *Client) UploadReader(ctx context.Context, limit *pb.OrderLimit, piecePrivateKey storj.PiecePrivateKey, data io.Reader) (hash *pb.PieceHash, err error) {
	defer mon.Task()(&ctx, "node: "+limit.StorageNodeId.String()[0:8])(&err)

	ctx, cancel := context2.WithCustomCancel(ctx)
	defer cancel(context.Canceled)

	var underlyingStream uploadStream
	sync2.WithTimeout(client.config.MessageTimeout, func() {
		if client.replaySafe != nil {
			underlyingStream, err = client.replaySafe.Upload(ctx)
		} else {
			underlyingStream, err = client.client.Upload(ctx)
		}
	}, func() { cancel(errMessageTimeout) })
	if err != nil {
		return nil, err
	}
	defer func() { _ = underlyingStream.Close() }()

	stream := &timedUploadStream{
		timeout: client.config.MessageTimeout,
		stream:  underlyingStream,
		cancel:  cancel,
	}

	nextRequest := &pb.PieceUploadRequest{
		Limit:         limit,
		HashAlgorithm: client.UploadHashAlgo,
	}
	if client.NodeURL().DebounceLimit > 0 {
		// in this case, the storage node is running code late enough that it will be able to handle
		// aggregated requests entirely. this is the best case and we don't need to use drpc stream
		// corking. this is because storage nodes that advertise their debounce limit
		// also have the change that support aggregated request limits.

		// leave nextRequest alone, so nothing to do!
	} else {
		// okay, let's see if we can do drpc stream corking.
		if streamGetter, ok := underlyingStream.(interface {
			GetStream() drpc.Stream
		}); ok {
			if flusher, ok := streamGetter.GetStream().(interface {
				SetManualFlush(bool)
			}); ok {
				// we can. let's send the next request and empty the nextRequest variable.
				flusher.SetManualFlush(true)
				err = stream.Send(nextRequest)
				flusher.SetManualFlush(false)
				nextRequest = nil
				// err checking below.
			}
		}
		if nextRequest != nil {
			// okay here, we are not in the DebounceLimit > 0 case, but we did not discover
			// we could do stream corking, so, give up I guess, just send as-is.
			err = stream.Send(nextRequest)
			nextRequest = nil
			// err checking below.
		}

		if err != nil {
			_, closeErr := stream.CloseAndRecv()
			switch {
			case !errors.Is(err, io.EOF) && closeErr != nil:
				err = ErrProtocol.Wrap(errs.Combine(err, closeErr))
			case closeErr != nil:
				err = ErrProtocol.Wrap(closeErr)
			}

			return nil, err
		}
	}

	upload := &upload{
		client:        client,
		limit:         limit,
		privateKey:    piecePrivateKey,
		nodeID:        limit.StorageNodeId,
		stream:        stream,
		hash:          pb.NewHashFromAlgorithm(client.UploadHashAlgo),
		hashAlgorithm: client.UploadHashAlgo,
		offset:        0,
		orderStep:     client.config.InitialStep,
		nextRequest:   nextRequest,
	}

	return upload.write(ctx, data)
}

// write sends all data to the storagenode allocating as necessary.
func (client *upload) write(ctx context.Context, data io.Reader) (hash *pb.PieceHash, err error) {
	defer mon.Task()(&ctx, "node: "+client.nodeID.String()[0:8])(&err)

	defer func() {
		if err != nil {
			err = errs.Combine(err, client.cancel(ctx))
			return
		}
	}()

	// write the hash of the data sent to the server
	data = io.TeeReader(data, client.hash)

	// Some facts about uploads
	//  * Signing orders are CPU intensive, so we don't want to do them too often.
	//  * Buffering data in RAM is resource intensive, so we don't want to buffer
	//    much.
	//  * We don't pay for upload bandwidth, so there's not a ton of benefit to
	//    even having upload orders other than making sure we can measure
	//    user bandwidth usage well.
	//  So, to address these things, we're going to read in a small increment
	// (maybe config.InitialStep I guess) consistently, throughout the entire
	// operation. We're going to keep track of how much we've written, and if
	// the current write requires us to send an order with a larger amount in
	// it, only then will we sign. Most writes won't include an order.

	backingArray := make([]byte, client.client.config.UploadBufferSize)

	var orderedSoFar int64

	done := false
	for !done {
		// read the next amount
		sendData := backingArray
		n, readErr := tryReadFull(ctx, data, sendData)
		if readErr != nil {
			if !errors.Is(readErr, io.EOF) {
				return nil, ErrInternal.Wrap(readErr)
			}
			done = true
		}
		if n <= 0 {
			continue
		}
		sendData = sendData[:n]

		req := client.nextRequest
		client.nextRequest = nil
		if req == nil {
			req = &pb.PieceUploadRequest{}
		}
		req.Chunk = &pb.PieceUploadRequest_Chunk{
			Offset: client.offset,
			Data:   sendData,
		}

		if client.offset+int64(len(sendData)) > orderedSoFar {
			// okay, create the next signed order.
			// Note: it might be larger than we need! in the worst
			// case, if there's only one byte here and we're at the
			// max order step, we will overshoot by
			// MaximumStepSize - 1.
			// But that's okay. Upload bandwidth is free.

			orderedSoFar = min(client.offset+client.orderStep, client.limit.Limit)

			order, err := signing.SignUplinkOrder(ctx, client.privateKey, &pb.Order{
				SerialNumber: client.limit.SerialNumber,
				Amount:       orderedSoFar,
			})
			if err != nil {
				return nil, ErrInternal.Wrap(err)
			}
			req.Order = order
			// update order step, incrementally building trust
			client.orderStep = client.client.nextOrderStep(client.orderStep)
		}

		// update our offset
		client.offset += int64(len(sendData))

		if done {
			// combine the last request with the closing data.
			return client.commit(ctx, req)
		}

		// send signed order + data
		err = client.stream.Send(req)
		if err != nil {
			_, closeErr := client.stream.CloseAndRecv()
			switch {
			case !errors.Is(err, io.EOF) && closeErr != nil:
				err = ErrProtocol.Wrap(errs.Combine(err, closeErr))
			case closeErr != nil:
				err = ErrProtocol.Wrap(closeErr)
			}

			return nil, err
		}
	}

	return client.commit(ctx, &pb.PieceUploadRequest{})
}

// cancel cancels the uploading.
func (client *upload) cancel(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	if client.finished {
		return io.EOF
	}
	client.finished = true
	return Error.Wrap(client.stream.Close())
}

// commit finishes uploading by sending the piece-hash and retrieving the piece-hash.
func (client *upload) commit(ctx context.Context, req *pb.PieceUploadRequest) (_ *pb.PieceHash, err error) {
	defer mon.Task()(&ctx, "node: "+client.nodeID.String()[0:8])(&err)
	if client.finished {
		return nil, io.EOF
	}
	client.finished = true

	// sign the hash for storage node
	uplinkHash, err := signing.SignUplinkPieceHash(ctx, client.privateKey, &pb.PieceHash{
		PieceId:       client.limit.PieceId,
		PieceSize:     client.offset,
		Hash:          client.hash.Sum(nil),
		Timestamp:     client.limit.OrderCreation,
		HashAlgorithm: client.hashAlgorithm,
	})
	if err != nil {
		// failed to sign, let's close, no need to wait for a response
		closeErr := client.stream.Close()
		// closeErr being io.EOF doesn't inform us about anything
		return nil, Error.Wrap(errs.Combine(err, ignoreEOF(closeErr)))
	}

	// exchange signed piece hashes
	// 1. send our piece hash
	req.Done = uplinkHash
	sendErr := client.stream.Send(req)

	// 2. wait for a piece hash as a response
	response, closeErr := client.stream.CloseAndRecv()
	if response == nil || response.Done == nil {
		// combine all the errors from before
		// sendErr is io.EOF when failed to send, so don't care
		// closeErr is io.EOF when storage node closed before sending us a response
		return nil, errs.Combine(ErrProtocol.New("expected piece hash"), ignoreEOF(sendErr), ignoreEOF(closeErr))
	}

	var peer *identity.PeerIdentity
	if len(response.NodeCertchain) > 0 {
		peer, err = identity.DecodePeerIdentity(ctx, response.NodeCertchain)
	} else {
		peer, err = client.client.GetPeerIdentity()
	}
	if err != nil {
		return nil, errs.Combine(err, ignoreEOF(sendErr), ignoreEOF(closeErr))
	}
	if peer.ID != client.nodeID {
		return nil, errs.Combine(ErrProtocol.New("mismatch node ids"), ignoreEOF(sendErr), ignoreEOF(closeErr))
	}

	// verification
	verifyErr := client.client.VerifyPieceHash(ctx, peer, client.limit, response.Done, uplinkHash.Hash, uplinkHash.HashAlgorithm)

	// combine all the errors from before
	// sendErr is io.EOF when we failed to send
	// closeErr is io.EOF when storage node closed properly
	return response.Done, errs.Combine(verifyErr, ignoreEOF(sendErr), ignoreEOF(closeErr))
}

func tryReadFull(ctx context.Context, r io.Reader, buf []byte) (n int, err error) {
	total := len(buf)

	for n < total && err == nil {
		if ctx.Err() != nil {
			return n, ctx.Err()
		}
		var nn int
		nn, err = r.Read(buf[n:])
		n += nn
	}

	return n, err
}

// timedUploadStream wraps uploadStream and adds timeouts
// to all operations.
type timedUploadStream struct {
	timeout time.Duration
	stream  uploadStream
	cancel  func(error)
}

func (stream *timedUploadStream) Context() context.Context {
	return stream.stream.Context()
}

func (stream *timedUploadStream) cancelTimeout() {
	stream.cancel(errMessageTimeout)
}

func (stream *timedUploadStream) Close() (err error) {
	sync2.WithTimeout(stream.timeout, func() {
		err = stream.stream.Close()
	}, stream.cancelTimeout)
	return CloseError.Wrap(err)
}

func (stream *timedUploadStream) Send(req *pb.PieceUploadRequest) (err error) {
	sync2.WithTimeout(stream.timeout, func() {
		err = stream.stream.Send(req)
	}, stream.cancelTimeout)
	return err
}

func (stream *timedUploadStream) CloseAndRecv() (resp *pb.PieceUploadResponse, err error) {
	sync2.WithTimeout(stream.timeout, func() {
		resp, err = stream.stream.CloseAndRecv()
	}, stream.cancelTimeout)
	return resp, err
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
