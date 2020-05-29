// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"
	"time"

	"storj.io/common/errs2"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/storj"
	"storj.io/common/uuid"
)

// CreateSegmentOld requests the order limits for creating a new segment.
func (client *Client) CreateSegmentOld(ctx context.Context, bucket string, path storj.Path, segmentIndex int64, redundancy *pb.RedundancyScheme, maxEncryptedSegmentSize int64, expiration time.Time) (limits []*pb.AddressedOrderLimit, rootPieceID storj.PieceID, piecePrivateKey storj.PiecePrivateKey, err error) {
	defer mon.Task()(&ctx)(&err)

	response, err := client.client.CreateSegmentOld(ctx, &pb.SegmentWriteRequestOld{
		Header:                  client.header(),
		Bucket:                  []byte(bucket),
		Path:                    []byte(path),
		Segment:                 segmentIndex,
		Redundancy:              redundancy,
		MaxEncryptedSegmentSize: maxEncryptedSegmentSize,
		Expiration:              expiration,
	})
	if err != nil {
		return nil, rootPieceID, piecePrivateKey, Error.Wrap(err)
	}

	return response.GetAddressedLimits(), response.RootPieceId, response.PrivateKey, nil
}

// CommitSegmentOld requests to store the pointer for the segment.
func (client *Client) CommitSegmentOld(ctx context.Context, bucket string, path storj.Path, segmentIndex int64, pointer *pb.Pointer, originalLimits []*pb.OrderLimit) (savedPointer *pb.Pointer, err error) {
	defer mon.Task()(&ctx)(&err)

	response, err := client.client.CommitSegmentOld(ctx, &pb.SegmentCommitRequestOld{
		Header:         client.header(),
		Bucket:         []byte(bucket),
		Path:           []byte(path),
		Segment:        segmentIndex,
		Pointer:        pointer,
		OriginalLimits: originalLimits,
	})
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return response.GetPointer(), nil
}

// SegmentInfoOld requests the pointer of a segment.
func (client *Client) SegmentInfoOld(ctx context.Context, bucket string, path storj.Path, segmentIndex int64) (pointer *pb.Pointer, err error) {
	defer mon.Task()(&ctx)(&err)

	response, err := client.client.SegmentInfoOld(ctx, &pb.SegmentInfoRequestOld{
		Header:  client.header(),
		Bucket:  []byte(bucket),
		Path:    []byte(path),
		Segment: segmentIndex,
	})
	if err != nil {
		if errs2.IsRPC(err, rpcstatus.NotFound) {
			return nil, storj.ErrObjectNotFound.Wrap(err)
		}
		return nil, Error.Wrap(err)
	}

	return response.GetPointer(), nil
}

// ReadSegmentOld requests the order limits for reading a segment.
func (client *Client) ReadSegmentOld(ctx context.Context, bucket string, path storj.Path, segmentIndex int64) (pointer *pb.Pointer, limits []*pb.AddressedOrderLimit, piecePrivateKey storj.PiecePrivateKey, err error) {
	defer mon.Task()(&ctx)(&err)

	response, err := client.client.DownloadSegmentOld(ctx, &pb.SegmentDownloadRequestOld{
		Header:  client.header(),
		Bucket:  []byte(bucket),
		Path:    []byte(path),
		Segment: segmentIndex,
	})
	if err != nil {
		if errs2.IsRPC(err, rpcstatus.NotFound) {
			return nil, nil, piecePrivateKey, storj.ErrObjectNotFound.Wrap(err)
		}
		return nil, nil, piecePrivateKey, Error.Wrap(err)
	}

	return response.GetPointer(), sortLimits(response.GetAddressedLimits(), response.GetPointer()), response.PrivateKey, nil
}

// sortLimits sorts order limits and fill missing ones with nil values.
func sortLimits(limits []*pb.AddressedOrderLimit, pointer *pb.Pointer) []*pb.AddressedOrderLimit {
	sorted := make([]*pb.AddressedOrderLimit, pointer.GetRemote().GetRedundancy().GetTotal())
	for _, piece := range pointer.GetRemote().GetRemotePieces() {
		sorted[piece.GetPieceNum()] = getLimitByStorageNodeID(limits, piece.NodeId)
	}
	return sorted
}

func getLimitByStorageNodeID(limits []*pb.AddressedOrderLimit, storageNodeID storj.NodeID) *pb.AddressedOrderLimit {
	for _, limit := range limits {
		if limit.GetLimit().StorageNodeId == storageNodeID {
			return limit
		}
	}
	return nil
}

// DeleteSegmentOld requests the order limits for deleting a segment.
func (client *Client) DeleteSegmentOld(ctx context.Context, bucket string, path storj.Path, segmentIndex int64) (limits []*pb.AddressedOrderLimit, piecePrivateKey storj.PiecePrivateKey, err error) {
	defer mon.Task()(&ctx)(&err)

	response, err := client.client.DeleteSegmentOld(ctx, &pb.SegmentDeleteRequestOld{
		Header:  client.header(),
		Bucket:  []byte(bucket),
		Path:    []byte(path),
		Segment: segmentIndex,
	})
	if err != nil {
		if errs2.IsRPC(err, rpcstatus.NotFound) {
			return nil, piecePrivateKey, storj.ErrObjectNotFound.Wrap(err)
		}
		return nil, piecePrivateKey, Error.Wrap(err)
	}

	return response.GetAddressedLimits(), response.PrivateKey, nil
}

// ListSegmentsOld lists the available segments.
func (client *Client) ListSegmentsOld(ctx context.Context, bucket string, prefix, startAfter, ignoredEndBefore storj.Path, recursive bool, limit int32, metaFlags uint32) (items []ListItem, more bool, err error) {
	defer mon.Task()(&ctx)(&err)

	response, err := client.client.ListSegmentsOld(ctx, &pb.ListSegmentsRequestOld{
		Header:     client.header(),
		Bucket:     []byte(bucket),
		Prefix:     []byte(prefix),
		StartAfter: []byte(startAfter),
		Recursive:  recursive,
		Limit:      limit,
		MetaFlags:  metaFlags,
	})
	if err != nil {
		return nil, false, Error.Wrap(err)
	}

	list := response.GetItems()
	items = make([]ListItem, len(list))
	for i, item := range list {
		items[i] = ListItem{
			Path:     storj.Path(item.GetPath()),
			Pointer:  item.GetPointer(),
			IsPrefix: item.IsPrefix,
		}
	}

	return items, response.GetMore(), nil
}

// SetAttributionOld tries to set the attribution information on the bucket.
func (client *Client) SetAttributionOld(ctx context.Context, bucket string, partnerID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = client.client.SetAttributionOld(ctx, &pb.SetAttributionRequestOld{
		Header:     client.header(),
		PartnerId:  partnerID[:], // TODO: implement storj.UUID that can be sent using pb
		BucketName: []byte(bucket),
	})

	return Error.Wrap(err)
}
