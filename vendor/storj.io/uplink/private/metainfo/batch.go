// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"github.com/zeebo/errs"

	"storj.io/common/pb"
	"storj.io/uplink/private/eestream"
)

var (
	// ErrInvalidType error for inalid response type casting.
	ErrInvalidType = errs.New("invalid response type")
)

// BatchItem represents single request in batch.
type BatchItem interface {
	BatchItem() *pb.BatchRequestItem
}

// BatchResponse single response from batch call.
type BatchResponse struct {
	pbRequest  interface{}
	pbResponse interface{}
}

// CreateBucket returns BatchResponse for CreateBucket request.
func (resp *BatchResponse) CreateBucket() (CreateBucketResponse, error) {
	item, ok := resp.pbResponse.(*pb.BatchResponseItem_BucketCreate)
	if !ok {
		return CreateBucketResponse{}, ErrInvalidType
	}

	createResponse, err := newCreateBucketResponse(item.BucketCreate)
	if err != nil {
		return CreateBucketResponse{}, err
	}
	return createResponse, nil
}

// GetBucket returns response for GetBucket request.
func (resp *BatchResponse) GetBucket() (GetBucketResponse, error) {
	item, ok := resp.pbResponse.(*pb.BatchResponseItem_BucketGet)
	if !ok {
		return GetBucketResponse{}, ErrInvalidType
	}
	getResponse, err := newGetBucketResponse(item.BucketGet)
	if err != nil {
		return GetBucketResponse{}, err
	}
	return getResponse, nil
}

// ListBuckets returns response for ListBuckets request.
func (resp *BatchResponse) ListBuckets() (ListBucketsResponse, error) {
	item, ok := resp.pbResponse.(*pb.BatchResponseItem_BucketList)
	if !ok {
		return ListBucketsResponse{}, ErrInvalidType
	}
	return newListBucketsResponse(item.BucketList), nil
}

// BeginObject returns response for BeginObject request.
func (resp *BatchResponse) BeginObject() (BeginObjectResponse, error) {
	item, ok := resp.pbResponse.(*pb.BatchResponseItem_ObjectBegin)
	if !ok {
		return BeginObjectResponse{}, ErrInvalidType
	}

	rs, err := eestream.NewRedundancyStrategyFromProto(item.ObjectBegin.RedundancyScheme)
	if err != nil {
		return BeginObjectResponse{}, Error.Wrap(err)
	}

	return newBeginObjectResponse(item.ObjectBegin, rs), nil
}

// BeginDeleteObject returns response for BeginDeleteObject request.
func (resp *BatchResponse) BeginDeleteObject() (BeginDeleteObjectResponse, error) {
	item, ok := resp.pbResponse.(*pb.BatchResponseItem_ObjectBeginDelete)
	if !ok {
		return BeginDeleteObjectResponse{}, ErrInvalidType
	}
	return newBeginDeleteObjectResponse(item.ObjectBeginDelete), nil
}

// GetObject returns response for GetObject request.
func (resp *BatchResponse) GetObject() (GetObjectResponse, error) {
	item, ok := resp.pbResponse.(*pb.BatchResponseItem_ObjectGet)
	if !ok {
		return GetObjectResponse{}, ErrInvalidType
	}
	return newGetObjectResponse(item.ObjectGet), nil
}

// ListObjects returns response for ListObjects request.
func (resp *BatchResponse) ListObjects() (ListObjectsResponse, error) {
	item, ok := resp.pbResponse.(*pb.BatchResponseItem_ObjectList)
	if !ok {
		return ListObjectsResponse{}, ErrInvalidType
	}

	requestItem, ok := resp.pbRequest.(*pb.BatchRequestItem_ObjectList)
	if !ok {
		return ListObjectsResponse{}, ErrInvalidType
	}

	return newListObjectsResponse(item.ObjectList, requestItem.ObjectList.EncryptedPrefix, requestItem.ObjectList.Recursive), nil
}

// BeginSegment returns response for BeginSegment request.
func (resp *BatchResponse) BeginSegment() (BeginSegmentResponse, error) {
	item, ok := resp.pbResponse.(*pb.BatchResponseItem_SegmentBegin)
	if !ok {
		return BeginSegmentResponse{}, ErrInvalidType
	}

	return newBeginSegmentResponse(item.SegmentBegin), nil
}

// BeginDeleteSegment returns response for BeginDeleteSegment request.
func (resp *BatchResponse) BeginDeleteSegment() (BeginDeleteSegmentResponse, error) {
	item, ok := resp.pbResponse.(*pb.BatchResponseItem_SegmentBeginDelete)
	if !ok {
		return BeginDeleteSegmentResponse{}, ErrInvalidType
	}

	return newBeginDeleteSegmentResponse(item.SegmentBeginDelete), nil
}

// ListSegment returns response for ListSegment request.
func (resp *BatchResponse) ListSegment() (ListSegmentsResponse, error) {
	item, ok := resp.pbResponse.(*pb.BatchResponseItem_SegmentList)
	if !ok {
		return ListSegmentsResponse{}, ErrInvalidType
	}
	return newListSegmentsResponse(item.SegmentList), nil
}

// DownloadSegment returns response for DownloadSegment request.
func (resp *BatchResponse) DownloadSegment() (DownloadSegmentResponse, error) {
	item, ok := resp.pbResponse.(*pb.BatchResponseItem_SegmentDownload)
	if !ok {
		return DownloadSegmentResponse{}, ErrInvalidType
	}
	return newDownloadSegmentResponse(item.SegmentDownload), nil
}
