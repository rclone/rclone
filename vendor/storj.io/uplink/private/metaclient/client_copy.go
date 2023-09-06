// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package metaclient

import (
	"context"

	"storj.io/common/pb"
	"storj.io/common/storj"
)

// BeginCopyObjectParams parameters for BeginCopyObject method.
type BeginCopyObjectParams struct {
	Bucket                []byte
	EncryptedObjectKey    []byte
	NewBucket             []byte
	NewEncryptedObjectKey []byte
}

// BeginCopyObjectResponse response for BeginCopyObjectResponse request.
type BeginCopyObjectResponse struct {
	StreamID                  storj.StreamID
	EncryptedMetadataKeyNonce storj.Nonce
	EncryptedMetadataKey      []byte
	SegmentKeys               []EncryptedKeyAndNonce
}

func (params *BeginCopyObjectParams) toRequest(header *pb.RequestHeader) *pb.ObjectBeginCopyRequest {
	return &pb.ObjectBeginCopyRequest{
		Header:                header,
		Bucket:                params.Bucket,
		EncryptedObjectKey:    params.EncryptedObjectKey,
		NewBucket:             params.NewBucket,
		NewEncryptedObjectKey: params.NewEncryptedObjectKey,
	}
}

// BatchItem returns single item for batch request.
func (params *BeginCopyObjectParams) BatchItem() *pb.BatchRequestItem {
	return &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_ObjectBeginCopy{
			ObjectBeginCopy: params.toRequest(nil),
		},
	}
}

func newBeginCopyObjectResponse(response *pb.ObjectBeginCopyResponse) BeginCopyObjectResponse {
	keys := make([]EncryptedKeyAndNonce, len(response.SegmentKeys))
	for i, key := range response.SegmentKeys {
		keys[i] = EncryptedKeyAndNonce{
			EncryptedKeyNonce: key.EncryptedKeyNonce,
			EncryptedKey:      key.EncryptedKey,
		}
		if key.Position != nil {
			keys[i].Position = SegmentPosition{
				PartNumber: key.Position.PartNumber,
				Index:      key.Position.Index,
			}
		}
	}

	return BeginCopyObjectResponse{
		StreamID:                  response.StreamId,
		EncryptedMetadataKeyNonce: response.EncryptedMetadataKeyNonce,
		EncryptedMetadataKey:      response.EncryptedMetadataKey,
		SegmentKeys:               keys,
	}
}

// BeginCopyObject requests data needed to copy an object from one key to another.
func (client *Client) BeginCopyObject(ctx context.Context, params BeginCopyObjectParams) (_ BeginCopyObjectResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	var response *pb.ObjectBeginCopyResponse
	err = WithRetry(ctx, func(ctx context.Context) error {
		response, err = client.client.BeginCopyObject(ctx, params.toRequest(client.header()))
		return err
	})
	if err != nil {
		return BeginCopyObjectResponse{}, Error.Wrap(err)
	}
	return newBeginCopyObjectResponse(response), nil
}

// FinishCopyObjectParams parameters for FinishCopyObject method.
type FinishCopyObjectParams struct {
	StreamID                     storj.StreamID
	NewBucket                    []byte
	NewEncryptedObjectKey        []byte
	NewEncryptedMetadataKeyNonce storj.Nonce
	NewEncryptedMetadataKey      []byte
	NewSegmentKeys               []EncryptedKeyAndNonce
}

func (params *FinishCopyObjectParams) toRequest(header *pb.RequestHeader) *pb.ObjectFinishCopyRequest {
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
	return &pb.ObjectFinishCopyRequest{
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
func (params *FinishCopyObjectParams) BatchItem() *pb.BatchRequestItem {
	return &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_ObjectFinishCopy{
			ObjectFinishCopy: params.toRequest(nil),
		},
	}
}

// FinishCopyObjectResponse response for FinishCopyObjectResponse request.
type FinishCopyObjectResponse struct {
	Info RawObjectItem
}

// FinishCopyObject finishes process of copying object from one key to another.
func (client *Client) FinishCopyObject(ctx context.Context, params FinishCopyObjectParams) (_ FinishCopyObjectResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	var response *pb.ObjectFinishCopyResponse
	err = WithRetry(ctx, func(ctx context.Context) error {
		response, err = client.client.FinishCopyObject(ctx, params.toRequest(client.header()))
		return err
	})
	if err != nil {
		return FinishCopyObjectResponse{}, Error.Wrap(err)
	}

	return newFinishCopyObjectResponse(response), nil
}

func newFinishCopyObjectResponse(response *pb.ObjectFinishCopyResponse) FinishCopyObjectResponse {
	info := newObjectInfo(response.Object)

	return FinishCopyObjectResponse{Info: info}
}
