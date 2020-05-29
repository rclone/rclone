// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package signing

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/common/pb"
	"storj.io/common/storj"
)

// Error is the default error class for signing package.
var Error = errs.Class("signing")

// Signer is able to sign data and verify own signature belongs.
type Signer interface {
	ID() storj.NodeID
	HashAndSign(ctx context.Context, data []byte) ([]byte, error)
	HashAndVerifySignature(ctx context.Context, data, signature []byte) error
}

// SignOrderLimit signs the order limit using the specified signer.
// Signer is a satellite.
func SignOrderLimit(ctx context.Context, satellite Signer, unsigned *pb.OrderLimit) (_ *pb.OrderLimit, err error) {
	defer mon.Task()(&ctx)(&err)
	bytes, err := EncodeOrderLimit(ctx, unsigned)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	signed := *unsigned
	signed.SatelliteSignature, err = satellite.HashAndSign(ctx, bytes)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &signed, nil
}

// SignUplinkOrder signs the order using the specified signer.
// Signer is an uplink.
func SignUplinkOrder(ctx context.Context, privateKey storj.PiecePrivateKey, unsigned *pb.Order) (_ *pb.Order, err error) {
	defer mon.Task()(&ctx)(&err)
	bytes, err := EncodeOrder(ctx, unsigned)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	signed := *unsigned
	signed.UplinkSignature, err = privateKey.Sign(bytes)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return &signed, nil
}

// SignPieceHash signs the piece hash using the specified signer.
// Signer is either uplink or storage node.
func SignPieceHash(ctx context.Context, signer Signer, unsigned *pb.PieceHash) (_ *pb.PieceHash, err error) {
	defer mon.Task()(&ctx)(&err)
	bytes, err := EncodePieceHash(ctx, unsigned)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	signed := *unsigned
	signed.Signature, err = signer.HashAndSign(ctx, bytes)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &signed, nil
}

// SignUplinkPieceHash signs the piece hash using the specified signer.
// Signer is either uplink or storage node.
func SignUplinkPieceHash(ctx context.Context, privateKey storj.PiecePrivateKey, unsigned *pb.PieceHash) (_ *pb.PieceHash, err error) {
	defer mon.Task()(&ctx)(&err)
	bytes, err := EncodePieceHash(ctx, unsigned)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	signed := *unsigned
	signed.Signature, err = privateKey.Sign(bytes)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return &signed, nil
}

// SignStreamID signs the stream ID using the specified signer.
// Signer is a satellite.
func SignStreamID(ctx context.Context, signer Signer, unsigned *pb.SatStreamID) (_ *pb.SatStreamID, err error) {
	defer mon.Task()(&ctx)(&err)
	bytes, err := EncodeStreamID(ctx, unsigned)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	signed := *unsigned
	signed.SatelliteSignature, err = signer.HashAndSign(ctx, bytes)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &signed, nil
}

// SignSegmentID signs the segment ID using the specified signer.
// Signer is a satellite.
func SignSegmentID(ctx context.Context, signer Signer, unsigned *pb.SatSegmentID) (_ *pb.SatSegmentID, err error) {
	defer mon.Task()(&ctx)(&err)
	bytes, err := EncodeSegmentID(ctx, unsigned)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	signed := *unsigned
	signed.SatelliteSignature, err = signer.HashAndSign(ctx, bytes)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &signed, nil
}

// SignExitCompleted signs the ExitCompleted using the specified signer.
// Signer is a satellite.
func SignExitCompleted(ctx context.Context, signer Signer, unsigned *pb.ExitCompleted) (_ *pb.ExitCompleted, err error) {
	defer mon.Task()(&ctx)(&err)
	bytes, err := EncodeExitCompleted(ctx, unsigned)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	signed := *unsigned
	signed.ExitCompleteSignature, err = signer.HashAndSign(ctx, bytes)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &signed, nil
}

// SignExitFailed signs the ExitFailed using the specified signer.
// Signer is a satellite.
func SignExitFailed(ctx context.Context, signer Signer, unsigned *pb.ExitFailed) (_ *pb.ExitFailed, err error) {
	defer mon.Task()(&ctx)(&err)
	bytes, err := EncodeExitFailed(ctx, unsigned)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	signed := *unsigned
	signed.ExitFailureSignature, err = signer.HashAndSign(ctx, bytes)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &signed, nil
}
