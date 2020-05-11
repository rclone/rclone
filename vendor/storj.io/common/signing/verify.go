// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package signing

import (
	"context"

	"storj.io/common/pb"
	"storj.io/common/storj"
)

// Signee is able to verify that the data signature belongs to the signee.
type Signee interface {
	ID() storj.NodeID
	HashAndVerifySignature(ctx context.Context, data, signature []byte) error
}

// VerifyOrderLimitSignature verifies that the signature inside order limit is valid and  belongs to the satellite.
func VerifyOrderLimitSignature(ctx context.Context, satellite Signee, signed *pb.OrderLimit) (err error) {
	defer mon.Task()(&ctx)(&err)
	bytes, err := EncodeOrderLimit(ctx, signed)
	if err != nil {
		return Error.Wrap(err)
	}

	return satellite.HashAndVerifySignature(ctx, bytes, signed.SatelliteSignature)
}

// VerifyOrderSignature verifies that the signature inside order is valid and belongs to the uplink.
func VerifyOrderSignature(ctx context.Context, uplink Signee, signed *pb.Order) (err error) {
	defer mon.Task()(&ctx)(&err)
	bytes, err := EncodeOrder(ctx, signed)
	if err != nil {
		return Error.Wrap(err)
	}

	return uplink.HashAndVerifySignature(ctx, bytes, signed.UplinkSignature)
}

// VerifyUplinkOrderSignature verifies that the signature inside order is valid and belongs to the uplink.
func VerifyUplinkOrderSignature(ctx context.Context, publicKey storj.PiecePublicKey, signed *pb.Order) (err error) {
	defer mon.Task()(&ctx)(&err)
	bytes, err := EncodeOrder(ctx, signed)
	if err != nil {
		return Error.Wrap(err)
	}

	return Error.Wrap(publicKey.Verify(bytes, signed.UplinkSignature))
}

// VerifyPieceHashSignature verifies that the signature inside piece hash is valid and belongs to the signer, which is either uplink or storage node.
func VerifyPieceHashSignature(ctx context.Context, signee Signee, signed *pb.PieceHash) (err error) {
	defer mon.Task()(&ctx)(&err)
	bytes, err := EncodePieceHash(ctx, signed)
	if err != nil {
		return Error.Wrap(err)
	}

	return signee.HashAndVerifySignature(ctx, bytes, signed.Signature)
}

// VerifyUplinkPieceHashSignature verifies that the signature inside piece hash is valid and belongs to the signer, which is either uplink or storage node.
func VerifyUplinkPieceHashSignature(ctx context.Context, publicKey storj.PiecePublicKey, signed *pb.PieceHash) (err error) {
	defer mon.Task()(&ctx)(&err)

	bytes, err := EncodePieceHash(ctx, signed)
	if err != nil {
		return Error.Wrap(err)
	}

	return Error.Wrap(publicKey.Verify(bytes, signed.Signature))
}

// VerifyStreamID verifies that the signature inside stream ID belongs to the satellite
func VerifyStreamID(ctx context.Context, satellite Signee, signed *pb.SatStreamID) (err error) {
	defer mon.Task()(&ctx)(&err)
	bytes, err := EncodeStreamID(ctx, signed)
	if err != nil {
		return Error.Wrap(err)
	}

	return satellite.HashAndVerifySignature(ctx, bytes, signed.SatelliteSignature)
}

// VerifySegmentID verifies that the signature inside segment ID belongs to the satellite
func VerifySegmentID(ctx context.Context, satellite Signee, signed *pb.SatSegmentID) (err error) {
	defer mon.Task()(&ctx)(&err)
	bytes, err := EncodeSegmentID(ctx, signed)
	if err != nil {
		return Error.Wrap(err)
	}

	return satellite.HashAndVerifySignature(ctx, bytes, signed.SatelliteSignature)
}

// VerifyExitCompleted verifies that the signature inside ExitCompleted belongs to the satellite
func VerifyExitCompleted(ctx context.Context, satellite Signee, signed *pb.ExitCompleted) (err error) {
	defer mon.Task()(&ctx)(&err)
	bytes, err := EncodeExitCompleted(ctx, signed)
	if err != nil {
		return Error.Wrap(err)
	}

	return Error.Wrap(satellite.HashAndVerifySignature(ctx, bytes, signed.ExitCompleteSignature))
}

// VerifyExitFailed verifies that the signature inside ExitFailed belongs to the satellite
func VerifyExitFailed(ctx context.Context, satellite Signee, signed *pb.ExitFailed) (err error) {
	defer mon.Task()(&ctx)(&err)
	bytes, err := EncodeExitFailed(ctx, signed)
	if err != nil {
		return Error.Wrap(err)
	}

	return Error.Wrap(satellite.HashAndVerifySignature(ctx, bytes, signed.ExitFailureSignature))
}
