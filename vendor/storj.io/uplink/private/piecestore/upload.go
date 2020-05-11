// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"context"
	"hash"
	"io"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"

	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/pkcrypto"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/sync2"
)

var mon = monkit.Package()

// Uploader defines the interface for uploading a piece.
type Uploader interface {
	// Write uploads data to the storage node.
	Write([]byte) (int, error)
	// Cancel cancels the upload.
	Cancel(context.Context) error
	// Commit finalizes the upload.
	Commit(context.Context) (*pb.PieceHash, error)
}

// Upload implements uploading to the storage node.
type Upload struct {
	client     *Client
	limit      *pb.OrderLimit
	privateKey storj.PiecePrivateKey
	peer       *identity.PeerIdentity
	stream     uploadStream
	ctx        context.Context

	hash           hash.Hash // TODO: use concrete implementation
	offset         int64
	allocationStep int64

	// when there's a send error then it will automatically close
	finished  bool
	sendError error
}

type uploadStream interface {
	Context() context.Context
	CloseSend() error
	Send(*pb.PieceUploadRequest) error
	CloseAndRecv() (*pb.PieceUploadResponse, error)
}

// UploadReader uploads a reader to the storage node.
func (client *Client) UploadReader(ctx context.Context, limit *pb.OrderLimit, piecePrivateKey storj.PiecePrivateKey, data io.Reader) (hash *pb.PieceHash, err error) {
	// UploadReader is implemented using deprecated Upload until we can get everything
	// to switch to UploadReader directly.

	upload, err := client.Upload(ctx, limit, piecePrivateKey)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			err = errs.Combine(err, upload.Cancel(ctx))
			return
		}
		hash, err = upload.Commit(ctx)
	}()

	_, err = sync2.Copy(ctx, upload, data)
	return nil, err
}

// Upload is deprecated and will be removed. Please use UploadReader.
func (client *Client) Upload(ctx context.Context, limit *pb.OrderLimit, piecePrivateKey storj.PiecePrivateKey) (_ Uploader, err error) {
	defer mon.Task()(&ctx, "node: "+limit.StorageNodeId.String()[0:8])(&err)

	peer, err := client.conn.PeerIdentity()
	if err != nil {
		return nil, ErrInternal.Wrap(err)
	}

	stream, err := client.client.Upload(ctx)
	if err != nil {
		return nil, err
	}

	err = stream.Send(&pb.PieceUploadRequest{
		Limit: limit,
	})
	if err != nil {
		_, closeErr := stream.CloseAndRecv()
		switch {
		case err != io.EOF && closeErr != nil:
			err = ErrProtocol.Wrap(errs.Combine(err, closeErr))
		case closeErr != nil:
			err = ErrProtocol.Wrap(closeErr)
		}

		return nil, err
	}

	upload := &Upload{
		client:     client,
		limit:      limit,
		privateKey: piecePrivateKey,
		peer:       peer,
		stream:     stream,
		ctx:        ctx,

		hash:           pkcrypto.NewHash(),
		offset:         0,
		allocationStep: client.config.InitialStep,
	}

	if client.config.UploadBufferSize <= 0 {
		return &LockingUpload{upload: upload}, nil
	}
	return &LockingUpload{
		upload: NewBufferedUpload(upload, int(client.config.UploadBufferSize)),
	}, nil
}

// Write sends data to the storagenode allocating as necessary.
func (client *Upload) Write(data []byte) (written int, err error) {
	ctx := client.ctx
	defer mon.Task()(&ctx, "node: "+client.peer.ID.String()[0:8])(&err)

	if client.finished {
		return 0, io.EOF
	}
	// if we already encountered an error, keep returning it
	if client.sendError != nil {
		return 0, client.sendError
	}

	fullData := data
	defer func() {
		// write the hash of the data sent to the server
		// guaranteed not to return error
		_, _ = client.hash.Write(fullData[:written])
	}()

	for len(data) > 0 {
		// pick a data chunk to send
		var sendData []byte
		if client.allocationStep < int64(len(data)) {
			sendData, data = data[:client.allocationStep], data[client.allocationStep:]
		} else {
			sendData, data = data, nil
		}

		// create a signed order for the next chunk
		order, err := signing.SignUplinkOrder(ctx, client.privateKey, &pb.Order{
			SerialNumber: client.limit.SerialNumber,
			Amount:       client.offset + int64(len(sendData)),
		})
		if err != nil {
			return written, ErrInternal.Wrap(err)
		}

		// send signed order + data
		err = client.stream.Send(&pb.PieceUploadRequest{
			Order: order,
			Chunk: &pb.PieceUploadRequest_Chunk{
				Offset: client.offset,
				Data:   sendData,
			},
		})
		if err != nil {
			_, closeErr := client.stream.CloseAndRecv()
			switch {
			case err != io.EOF && closeErr != nil:
				err = ErrProtocol.Wrap(errs.Combine(err, closeErr))
			case closeErr != nil:
				err = ErrProtocol.Wrap(closeErr)
			}

			client.sendError = err
			return written, err
		}

		// update our offset
		client.offset += int64(len(sendData))
		written += len(sendData)

		// update allocation step, incrementally building trust
		client.allocationStep = client.client.nextAllocationStep(client.allocationStep)
	}

	return written, nil
}

// Cancel cancels the uploading.
func (client *Upload) Cancel(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	if client.finished {
		return io.EOF
	}
	client.finished = true
	return Error.Wrap(client.stream.CloseSend())
}

// Commit finishes uploading by sending the piece-hash and retrieving the piece-hash.
func (client *Upload) Commit(ctx context.Context) (_ *pb.PieceHash, err error) {
	defer mon.Task()(&ctx, "node: "+client.peer.ID.String()[0:8])(&err)
	if client.finished {
		return nil, io.EOF
	}
	client.finished = true

	if client.sendError != nil {
		// something happened during sending, try to figure out what exactly
		// since sendError was already reported, we don't need to rehandle it.
		_, closeErr := client.stream.CloseAndRecv()
		return nil, Error.Wrap(closeErr)
	}

	// sign the hash for storage node
	uplinkHash, err := signing.SignUplinkPieceHash(ctx, client.privateKey, &pb.PieceHash{
		PieceId:   client.limit.PieceId,
		PieceSize: client.offset,
		Hash:      client.hash.Sum(nil),
		Timestamp: client.limit.OrderCreation,
	})
	if err != nil {
		// failed to sign, let's close the sending side, no need to wait for a response
		closeErr := client.stream.CloseSend()
		// closeErr being io.EOF doesn't inform us about anything
		return nil, Error.Wrap(errs.Combine(err, ignoreEOF(closeErr)))
	}

	// exchange signed piece hashes
	// 1. send our piece hash
	sendErr := client.stream.Send(&pb.PieceUploadRequest{
		Done: uplinkHash,
	})

	// 2. wait for a piece hash as a response
	response, closeErr := client.stream.CloseAndRecv()
	if response == nil || response.Done == nil {
		// combine all the errors from before
		// sendErr is io.EOF when failed to send, so don't care
		// closeErr is io.EOF when storage node closed before sending us a response
		return nil, errs.Combine(ErrProtocol.New("expected piece hash"), ignoreEOF(sendErr), ignoreEOF(closeErr))
	}

	// verification
	verifyErr := client.client.VerifyPieceHash(client.stream.Context(), client.peer, client.limit, response.Done, uplinkHash.Hash)

	// combine all the errors from before
	// sendErr is io.EOF when we failed to send
	// closeErr is io.EOF when storage node closed properly
	return response.Done, errs.Combine(verifyErr, ignoreEOF(sendErr), ignoreEOF(closeErr))
}
