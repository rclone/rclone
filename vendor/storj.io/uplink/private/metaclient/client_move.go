// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metaclient

import (
	"context"

	"storj.io/common/pb"
	"storj.io/common/storj"
)

// BeginMoveObjectParams parameters for BeginMoveObject method.
type BeginMoveObjectParams struct {
	Bucket                []byte
	EncryptedObjectKey    []byte
	NewBucket             []byte
	NewEncryptedObjectKey []byte
}

// BeginMoveObjectResponse response for BeginMoveObjectResponse request.
type BeginMoveObjectResponse struct {
	StreamID                  storj.StreamID
	EncryptedMetadataKeyNonce storj.Nonce
	EncryptedMetadataKey      []byte
	SegmentKeys               []EncryptedKeyAndNonce
}

func (params *BeginMoveObjectParams) toRequest(header *pb.RequestHeader) *pb.ObjectBeginMoveRequest {
	return &pb.ObjectBeginMoveRequest{
		Header:                header,
		Bucket:                params.Bucket,
		EncryptedObjectKey:    params.EncryptedObjectKey,
		NewBucket:             params.NewBucket,
		NewEncryptedObjectKey: params.NewEncryptedObjectKey,
	}
}

// BatchItem returns single item for batch request.
func (params *BeginMoveObjectParams) BatchItem() *pb.BatchRequestItem {
	return &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_ObjectBeginMove{
			ObjectBeginMove: params.toRequest(nil),
		},
	}
}

func newBeginMoveObjectResponse(response *pb.ObjectBeginMoveResponse) BeginMoveObjectResponse {
	keys := convertKeys(response.SegmentKeys)

	return BeginMoveObjectResponse{
		StreamID:                  response.StreamId,
		EncryptedMetadataKeyNonce: response.EncryptedMetadataKeyNonce,
		EncryptedMetadataKey:      response.EncryptedMetadataKey,
		SegmentKeys:               keys,
	}
}

// BeginMoveObject begins process of moving object from one key to another.
func (client *Client) BeginMoveObject(ctx context.Context, params BeginMoveObjectParams) (_ BeginMoveObjectResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	var response *pb.ObjectBeginMoveResponse
	err = WithRetry(ctx, func(ctx context.Context) error {
		response, err = client.client.BeginMoveObject(ctx, params.toRequest(client.header()))
		return err
	})
	if err != nil {
		return BeginMoveObjectResponse{}, Error.Wrap(err)
	}

	return newBeginMoveObjectResponse(response), nil
}

// FinishMoveObjectParams parameters for FinishMoveObject method.
type FinishMoveObjectParams struct {
	StreamID                     storj.StreamID
	NewBucket                    []byte
	NewEncryptedObjectKey        []byte
	NewEncryptedMetadataKeyNonce storj.Nonce
	NewEncryptedMetadataKey      []byte
	NewSegmentKeys               []EncryptedKeyAndNonce
}

func (params *FinishMoveObjectParams) toRequest(header *pb.RequestHeader) *pb.ObjectFinishMoveRequest {
	keys := make([]*pb.EncryptedKeyAndNonce, len(params.NewSegmentKeys))
	for i, keyAndNonce := range params.NewSegmentKeys {
		keys[i] = &pb.EncryptedKeyAndNonce{
			Position: &pb.SegmentPosition{
				PartNumber: keyAndNonce.Position.PartNumber,
				Index:      keyAndNonce.Position.Index,
			},
			EncryptedKeyNonce: keyAndNonce.EncryptedKeyNonce,
			EncryptedKey:      keyAndNonce.EncryptedKey,
		}
	}
	return &pb.ObjectFinishMoveRequest{
		Header:                       header,
		StreamId:                     params.StreamID,
		NewBucket:                    params.NewBucket,
		NewEncryptedObjectKey:        params.NewEncryptedObjectKey,
		NewEncryptedMetadataKeyNonce: params.NewEncryptedMetadataKeyNonce,
		NewEncryptedMetadataKey:      params.NewEncryptedMetadataKey,
		NewSegmentKeys:               keys,
	}
}

// BatchItem returns single item for batch request.
func (params *FinishMoveObjectParams) BatchItem() *pb.BatchRequestItem {
	return &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_ObjectFinishMove{
			ObjectFinishMove: params.toRequest(nil),
		},
	}
}

// FinishMoveObject finishes process of moving object from one key to another.
func (client *Client) FinishMoveObject(ctx context.Context, params FinishMoveObjectParams) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = WithRetry(ctx, func(ctx context.Context) error {
		_, err = client.client.FinishMoveObject(ctx, params.toRequest(client.header()))
		return err
	})
	return Error.Wrap(err)
}
