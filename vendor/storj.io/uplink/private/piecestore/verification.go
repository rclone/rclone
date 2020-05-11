// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"bytes"
	"context"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/signing"
)

const pieceHashExpiration = 24 * time.Hour

var (
	// ErrInternal is an error class for internal errors.
	ErrInternal = errs.Class("internal")
	// ErrProtocol is an error class for unexpected protocol sequence.
	ErrProtocol = errs.Class("protocol")
	// ErrVerifyUntrusted is an error in case there is a trust issue.
	ErrVerifyUntrusted = errs.Class("untrusted")
	// ErrStorageNodeInvalidResponse is an error when a storage node returns a response with invalid data
	ErrStorageNodeInvalidResponse = errs.Class("storage node has returned an invalid response")
)

// VerifyPieceHash verifies piece hash which is sent by peer.
func (client *Client) VerifyPieceHash(ctx context.Context, peer *identity.PeerIdentity, limit *pb.OrderLimit, hash *pb.PieceHash, expectedHash []byte) (err error) {
	defer mon.Task()(&ctx)(&err)
	if peer == nil || limit == nil || hash == nil || len(expectedHash) == 0 {
		return ErrProtocol.New("invalid arguments")
	}
	if limit.PieceId != hash.PieceId {
		return ErrProtocol.New("piece id changed") // TODO: report rpc status bad message
	}
	if !bytes.Equal(hash.Hash, expectedHash) {
		return ErrVerifyUntrusted.New("hashes don't match") // TODO: report rpc status bad message
	}

	if err := signing.VerifyPieceHashSignature(ctx, signing.SigneeFromPeerIdentity(peer), hash); err != nil {
		return ErrVerifyUntrusted.New("invalid hash signature: %v", err) // TODO: report rpc status bad message
	}

	if hash.Timestamp.Before(time.Now().Add(-pieceHashExpiration)) {
		return ErrStorageNodeInvalidResponse.New("piece has timestamp is too old (%v). Required to be not older than %s",
			hash.Timestamp, pieceHashExpiration,
		)
	}

	return nil
}
