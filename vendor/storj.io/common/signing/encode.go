// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package signing

import (
	"context"

	"storj.io/common/pb"
	"storj.io/common/tracing"
)

var encodeOrderLimitTask = mon.Task()

// EncodeOrderLimit encodes order limit into bytes for signing. Removes signature from serialized limit.
func EncodeOrderLimit(ctx context.Context, limit *pb.OrderLimit) (_ []byte, err error) {
	defer encodeOrderLimitTask(&ctx)(&err)

	// protobuf has problems with serializing types with nullable=false
	// this uses a different message for signing, such that the rest of the code
	// doesn't have to deal with pointers for those particular fields.

	signing := pb.OrderLimitSigning{}
	signing.SerialNumber = limit.SerialNumber
	signing.SatelliteId = limit.SatelliteId
	if limit.DeprecatedUplinkId != nil && !limit.DeprecatedUplinkId.IsZero() {
		signing.DeprecatedUplinkId = limit.DeprecatedUplinkId
	}
	if !limit.UplinkPublicKey.IsZero() {
		signing.UplinkPublicKey = &limit.UplinkPublicKey
	}
	signing.StorageNodeId = limit.StorageNodeId
	signing.PieceId = limit.PieceId
	signing.Limit = limit.Limit
	signing.Action = limit.Action
	if !limit.PieceExpiration.IsZero() {
		signing.PieceExpiration = &limit.PieceExpiration
	}
	if !limit.OrderExpiration.IsZero() {
		signing.OrderExpiration = &limit.OrderExpiration
	}
	if !limit.OrderCreation.IsZero() {
		signing.OrderCreation = &limit.OrderCreation
	}

	signing.EncryptedMetadataKeyId = limit.EncryptedMetadataKeyId
	signing.EncryptedMetadata = limit.EncryptedMetadata

	signing.DeprecatedSatelliteAddress = limit.DeprecatedSatelliteAddress
	signing.XXX_unrecognized = limit.XXX_unrecognized

	return pb.Marshal(&signing)
}

var monEncodeOrderTask = mon.Task()

// EncodeOrder encodes order into bytes for signing. Removes signature from serialized order.
func EncodeOrder(ctx context.Context, order *pb.Order) (_ []byte, err error) {
	ctx = tracing.WithoutDistributedTracing(ctx)
	defer monEncodeOrderTask(&ctx)(&err)

	// protobuf has problems with serializing types with nullable=false
	// this uses a different message for signing, such that the rest of the code
	// doesn't have to deal with pointers for those particular fields.

	signing := pb.OrderSigning{}
	signing.SerialNumber = order.SerialNumber
	signing.Amount = order.Amount
	signing.XXX_unrecognized = order.XXX_unrecognized

	return pb.Marshal(&signing)
}

// EncodePieceHash encodes piece hash into bytes for signing. Removes signature from serialized hash.
func EncodePieceHash(ctx context.Context, hash *pb.PieceHash) (_ []byte, err error) {
	defer mon.Task()(&ctx)(&err)

	// protobuf has problems with serializing types with nullable=false
	// this uses a different message for signing, such that the rest of the code
	// doesn't have to deal with pointers for those particular fields.

	signing := pb.PieceHashSigning{}
	signing.PieceId = hash.PieceId
	signing.Hash = hash.Hash
	signing.PieceSize = hash.PieceSize
	signing.HashAlgorithm = hash.HashAlgorithm
	if !hash.Timestamp.IsZero() {
		signing.Timestamp = &hash.Timestamp
	}
	signing.XXX_unrecognized = hash.XXX_unrecognized

	return pb.Marshal(&signing)
}

// EncodeExitCompleted encodes ExitCompleted into bytes for signing.
func EncodeExitCompleted(ctx context.Context, exitCompleted *pb.ExitCompleted) (_ []byte, err error) {
	defer mon.Task()(&ctx)(&err)
	signature := exitCompleted.ExitCompleteSignature
	exitCompleted.ExitCompleteSignature = nil
	out, err := pb.Marshal(exitCompleted)
	exitCompleted.ExitCompleteSignature = signature

	return out, err
}

// EncodeExitFailed encodes ExitFailed into bytes for signing.
func EncodeExitFailed(ctx context.Context, exitFailed *pb.ExitFailed) (_ []byte, err error) {
	defer mon.Task()(&ctx)(&err)
	signature := exitFailed.ExitFailureSignature
	exitFailed.ExitFailureSignature = nil
	out, err := pb.Marshal(exitFailed)
	exitFailed.ExitFailureSignature = signature

	return out, err
}
