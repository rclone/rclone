// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pb

// The following aliases are provided to prevent a build break due to the
// renaming of the request/response types for the metainfo service. They can be
// removed once the uplink and storj repositories have transitioned away from
// the old types.
type (
	// BucketCreateRequest is an alias for CreateBucketRequest and should not be used in new code.
	BucketCreateRequest = CreateBucketRequest
	// BucketCreateResponse is an alias for CreateBucketResponse and should not be used in new code.
	BucketCreateResponse = CreateBucketResponse
	// BucketGetRequest is an alias for GetBucketRequest and should not be used in new code.
	BucketGetRequest = GetBucketRequest
	// BucketGetResponse is an alias for GetBucketResponse and should not be used in new code.
	BucketGetResponse = GetBucketResponse
	// BucketDeleteRequest is an alias for DeleteBucketRequest and should not be used in new code.
	BucketDeleteRequest = DeleteBucketRequest
	// BucketDeleteResponse is an alias for DeleteBucketResponse and should not be used in new code.
	BucketDeleteResponse = DeleteBucketResponse
	// BucketListRequest is an alias for ListBucketsRequest and should not be used in new code.
	BucketListRequest = ListBucketsRequest
	// BucketListResponse is an alias for ListBucketsResponse and should not be used in new code.
	BucketListResponse = ListBucketsResponse
	// ObjectBeginRequest is an alias for BeginObjectRequest and should not be used in new code.
	ObjectBeginRequest = BeginObjectRequest
	// ObjectBeginResponse is an alias for BeginObjectResponse and should not be used in new code.
	ObjectBeginResponse = BeginObjectResponse
	// ObjectCommitRequest is an alias for CommitObjectRequest and should not be used in new code.
	ObjectCommitRequest = CommitObjectRequest
	// ObjectCommitResponse is an alias for CommitObjectResponse and should not be used in new code.
	ObjectCommitResponse = CommitObjectResponse
	// ObjectGetRequest is an alias for GetObjectRequest and should not be used in new code.
	ObjectGetRequest = GetObjectRequest
	// ObjectGetResponse is an alias for GetObjectResponse and should not be used in new code.
	ObjectGetResponse = GetObjectResponse
	// ObjectListRequest is an alias for ListObjectsRequest and should not be used in new code.
	ObjectListRequest = ListObjectsRequest
	// ObjectListResponse is an alias for ListObjectsResponse and should not be used in new code.
	ObjectListResponse = ListObjectsResponse
	// ObjectBeginDeleteRequest is an alias for BeginDeleteObjectRequest and should not be used in new code.
	ObjectBeginDeleteRequest = BeginDeleteObjectRequest
	// ObjectBeginDeleteResponse is an alias for BeginDeleteObjectResponse and should not be used in new code.
	ObjectBeginDeleteResponse = BeginDeleteObjectResponse
	// ObjectFinishDeleteRequest is an alias for FinishDeleteObjectRequest and should not be used in new code.
	ObjectFinishDeleteRequest = FinishDeleteObjectRequest
	// ObjectFinishDeleteResponse is an alias for FinishDeleteObjectResponse and should not be used in new code.
	ObjectFinishDeleteResponse = FinishDeleteObjectResponse
	// ObjectGetIPsRequest is an alias for GetObjectIPsRequest and should not be used in new code.
	ObjectGetIPsRequest = GetObjectIPsRequest
	// ObjectGetIPsResponse is an alias for GetObjectIPsResponse and should not be used in new code.
	ObjectGetIPsResponse = GetObjectIPsResponse
	// ObjectListPendingStreamsRequest is an alias for ListPendingObjectStreamsRequest and should not be used in new code.
	ObjectListPendingStreamsRequest = ListPendingObjectStreamsRequest
	// ObjectListPendingStreamsResponse is an alias for ListPendingObjectStreamsResponse and should not be used in new code.
	ObjectListPendingStreamsResponse = ListPendingObjectStreamsResponse
	// ObjectDownloadRequest is an alias for DownloadObjectRequest and should not be used in new code.
	ObjectDownloadRequest = DownloadObjectRequest
	// ObjectDownloadResponse is an alias for DownloadObjectResponse and should not be used in new code.
	ObjectDownloadResponse = DownloadObjectResponse
	// ObjectUpdateMetadataRequest is an alias for UpdateObjectMetadataRequest and should not be used in new code.
	ObjectUpdateMetadataRequest = UpdateObjectMetadataRequest
	// ObjectUpdateMetadataResponse is an alias for UpdateObjectMetadataResponse and should not be used in new code.
	ObjectUpdateMetadataResponse = UpdateObjectMetadataResponse
	// SegmentBeginRequest is an alias for BeginSegmentRequest and should not be used in new code.
	SegmentBeginRequest = BeginSegmentRequest
	// SegmentBeginResponse is an alias for BeginSegmentResponse and should not be used in new code.
	SegmentBeginResponse = BeginSegmentResponse
	// SegmentCommitRequest is an alias for CommitSegmentRequest and should not be used in new code.
	SegmentCommitRequest = CommitSegmentRequest
	// SegmentCommitResponse is an alias for CommitSegmentResponse and should not be used in new code.
	SegmentCommitResponse = CommitSegmentResponse
	// SegmentMakeInlineRequest is an alias for MakeInlineSegmentRequest and should not be used in new code.
	SegmentMakeInlineRequest = MakeInlineSegmentRequest
	// SegmentMakeInlineResponse is an alias for MakeInlineSegmentResponse and should not be used in new code.
	SegmentMakeInlineResponse = MakeInlineSegmentResponse
	// SegmentBeginDeleteRequest is an alias for BeginDeleteSegmentRequest and should not be used in new code.
	SegmentBeginDeleteRequest = BeginDeleteSegmentRequest
	// SegmentBeginDeleteResponse is an alias for BeginDeleteSegmentResponse and should not be used in new code.
	SegmentBeginDeleteResponse = BeginDeleteSegmentResponse
	// SegmentFinishDeleteRequest is an alias for FinishDeleteSegmentRequest and should not be used in new code.
	SegmentFinishDeleteRequest = FinishDeleteSegmentRequest
	// SegmentFinishDeleteResponse is an alias for FinishDeleteSegmentResponse and should not be used in new code.
	SegmentFinishDeleteResponse = FinishDeleteSegmentResponse
	// SegmentListRequest is an alias for ListSegmentsRequest and should not be used in new code.
	SegmentListRequest = ListSegmentsRequest
	// SegmentListResponse is an alias for ListSegmentsResponse and should not be used in new code.
	SegmentListResponse = ListSegmentsResponse
	// SegmentDownloadRequest is an alias for DownloadSegmentRequest and should not be used in new code.
	SegmentDownloadRequest = DownloadSegmentRequest
	// SegmentDownloadResponse is an alias for DownloadSegmentResponse and should not be used in new code.
	SegmentDownloadResponse = DownloadSegmentResponse
	// PartDeleteRequest is an alias for DeletePartRequest and should not be used in new code.
	PartDeleteRequest = DeletePartRequest
	// PartDeleteResponse is an alias for DeletePartResponse and should not be used in new code.
	PartDeleteResponse = DeletePartResponse
	// ObjectBeginMoveRequest is an alias for BeginMoveObjectRequest and should not be used in new code.
	ObjectBeginMoveRequest = BeginMoveObjectRequest
	// ObjectBeginMoveResponse is an alias for BeginMoveObjectResponse and should not be used in new code.
	ObjectBeginMoveResponse = BeginMoveObjectResponse
	// ObjectFinishMoveRequest is an alias for FinishMoveObjectRequest and should not be used in new code.
	ObjectFinishMoveRequest = FinishMoveObjectRequest
	// ObjectFinishMoveResponse is an alias for FinishMoveObjectResponse and should not be used in new code.
	ObjectFinishMoveResponse = FinishMoveObjectResponse
	// ObjectBeginCopyRequest is an alias for BeginCopyObjectRequest and should not be used in new code.
	ObjectBeginCopyRequest = BeginCopyObjectRequest
	// ObjectBeginCopyResponse is an alias for BeginCopyObjectResponse and should not be used in new code.
	ObjectBeginCopyResponse = BeginCopyObjectResponse
	// ObjectFinishCopyRequest is an alias for FinishCopyObjectRequest and should not be used in new code.
	ObjectFinishCopyRequest = FinishCopyObjectRequest
	// ObjectFinishCopyResponse is an alias for FinishCopyObjectResponse and should not be used in new code.
	ObjectFinishCopyResponse = FinishCopyObjectResponse
)
