// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"bytes"
	"context"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"

	"storj.io/common/errs2"
	"storj.io/common/macaroon"
	"storj.io/common/pb"
	"storj.io/common/rpc"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/uplink/private/eestream"
)

var (
	mon = monkit.Package()

	// Error is the errs class of standard metainfo errors.
	Error = errs.Class("metainfo error")
)

// Client creates a grpcClient.
type Client struct {
	conn      *rpc.Conn
	client    pb.DRPCMetainfoClient
	apiKeyRaw []byte

	userAgent string
}

// ListItem is a single item in a listing.
type ListItem struct {
	Path     storj.Path
	Pointer  *pb.Pointer
	IsPrefix bool
}

// NewClient creates Metainfo API client.
func NewClient(client pb.DRPCMetainfoClient, apiKey *macaroon.APIKey, userAgent string) *Client {
	return &Client{
		client:    client,
		apiKeyRaw: apiKey.SerializeRaw(),

		userAgent: userAgent,
	}
}

// DialNodeURL dials to metainfo endpoint with the specified api key.
func DialNodeURL(ctx context.Context, dialer rpc.Dialer, nodeURL string, apiKey *macaroon.APIKey, userAgent string) (*Client, error) {
	url, err := storj.ParseNodeURL(nodeURL)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	if url.ID.IsZero() {
		return nil, Error.New("node ID is required in node URL %q", nodeURL)
	}

	conn, err := dialer.DialNodeURL(ctx, url)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &Client{
		conn:      conn,
		client:    pb.NewDRPCMetainfoClient(conn),
		apiKeyRaw: apiKey.SerializeRaw(),
		userAgent: userAgent,
	}, nil
}

// Close closes the dialed connection.
func (client *Client) Close() error {
	if client.conn != nil {
		return Error.Wrap(client.conn.Close())
	}
	return nil
}

func (client *Client) header() *pb.RequestHeader {
	return &pb.RequestHeader{
		ApiKey:    client.apiKeyRaw,
		UserAgent: []byte(client.userAgent),
	}
}

// GetProjectInfo gets the ProjectInfo for the api key associated with the metainfo client.
func (client *Client) GetProjectInfo(ctx context.Context) (resp *pb.ProjectInfoResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	return client.client.ProjectInfo(ctx, &pb.ProjectInfoRequest{
		Header: client.header(),
	})
}

// CreateBucketParams parameters for CreateBucket method.
type CreateBucketParams struct {
	Name                        []byte
	PathCipher                  storj.CipherSuite
	PartnerID                   []byte
	DefaultSegmentsSize         int64
	DefaultRedundancyScheme     storj.RedundancyScheme
	DefaultEncryptionParameters storj.EncryptionParameters
}

func (params *CreateBucketParams) toRequest(header *pb.RequestHeader) *pb.BucketCreateRequest {
	defaultRS := params.DefaultRedundancyScheme
	defaultEP := params.DefaultEncryptionParameters

	return &pb.BucketCreateRequest{
		Header:             header,
		Name:               params.Name,
		PathCipher:         pb.CipherSuite(params.PathCipher),
		PartnerId:          params.PartnerID,
		DefaultSegmentSize: params.DefaultSegmentsSize,
		DefaultRedundancyScheme: &pb.RedundancyScheme{
			Type:             pb.RedundancyScheme_SchemeType(defaultRS.Algorithm),
			MinReq:           int32(defaultRS.RequiredShares),
			Total:            int32(defaultRS.TotalShares),
			RepairThreshold:  int32(defaultRS.RepairShares),
			SuccessThreshold: int32(defaultRS.OptimalShares),
			ErasureShareSize: defaultRS.ShareSize,
		},
		DefaultEncryptionParameters: &pb.EncryptionParameters{
			CipherSuite: pb.CipherSuite(defaultEP.CipherSuite),
			BlockSize:   int64(defaultEP.BlockSize),
		},
	}
}

// BatchItem returns single item for batch request.
func (params *CreateBucketParams) BatchItem() *pb.BatchRequestItem {
	return &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_BucketCreate{
			BucketCreate: params.toRequest(nil),
		},
	}
}

// TODO potential names *Response/*Out/*Result

// CreateBucketResponse response for CreateBucket request.
type CreateBucketResponse struct {
	Bucket storj.Bucket
}

func newCreateBucketResponse(response *pb.BucketCreateResponse) (CreateBucketResponse, error) {
	bucket, err := convertProtoToBucket(response.Bucket)
	if err != nil {
		return CreateBucketResponse{}, err
	}
	return CreateBucketResponse{
		Bucket: bucket,
	}, nil
}

// CreateBucket creates a new bucket.
func (client *Client) CreateBucket(ctx context.Context, params CreateBucketParams) (respBucket storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)

	response, err := client.client.CreateBucket(ctx, params.toRequest(client.header()))
	if err != nil {
		return storj.Bucket{}, Error.Wrap(err)
	}

	respBucket, err = convertProtoToBucket(response.Bucket)
	if err != nil {
		return storj.Bucket{}, Error.Wrap(err)
	}
	return respBucket, nil
}

// GetBucketParams parmaters for GetBucketParams method.
type GetBucketParams struct {
	Name []byte
}

func (params *GetBucketParams) toRequest(header *pb.RequestHeader) *pb.BucketGetRequest {
	return &pb.BucketGetRequest{
		Header: header,
		Name:   params.Name,
	}
}

// BatchItem returns single item for batch request.
func (params *GetBucketParams) BatchItem() *pb.BatchRequestItem {
	return &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_BucketGet{
			BucketGet: params.toRequest(nil),
		},
	}
}

// GetBucketResponse response for GetBucket request.
type GetBucketResponse struct {
	Bucket storj.Bucket
}

func newGetBucketResponse(response *pb.BucketGetResponse) (GetBucketResponse, error) {
	bucket, err := convertProtoToBucket(response.Bucket)
	if err != nil {
		return GetBucketResponse{}, err
	}
	return GetBucketResponse{
		Bucket: bucket,
	}, nil
}

// GetBucket returns a bucket.
func (client *Client) GetBucket(ctx context.Context, params GetBucketParams) (respBucket storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)

	resp, err := client.client.GetBucket(ctx, params.toRequest(client.header()))
	if err != nil {
		if errs2.IsRPC(err, rpcstatus.NotFound) {
			return storj.Bucket{}, storj.ErrBucketNotFound.Wrap(err)
		}
		return storj.Bucket{}, Error.Wrap(err)
	}

	respBucket, err = convertProtoToBucket(resp.Bucket)
	if err != nil {
		return storj.Bucket{}, Error.Wrap(err)
	}
	return respBucket, nil
}

// DeleteBucketParams parmaters for DeleteBucket method.
type DeleteBucketParams struct {
	Name []byte
}

func (params *DeleteBucketParams) toRequest(header *pb.RequestHeader) *pb.BucketDeleteRequest {
	return &pb.BucketDeleteRequest{
		Header: header,
		Name:   params.Name,
	}
}

// BatchItem returns single item for batch request.
func (params *DeleteBucketParams) BatchItem() *pb.BatchRequestItem {
	return &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_BucketDelete{
			BucketDelete: params.toRequest(nil),
		},
	}
}

// DeleteBucket deletes a bucket.
func (client *Client) DeleteBucket(ctx context.Context, params DeleteBucketParams) (_ storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)
	resp, err := client.client.DeleteBucket(ctx, params.toRequest(client.header()))
	if err != nil {
		if errs2.IsRPC(err, rpcstatus.NotFound) {
			return storj.Bucket{}, storj.ErrBucketNotFound.Wrap(err)
		}
		return storj.Bucket{}, Error.Wrap(err)
	}

	respBucket, err := convertProtoToBucket(resp.Bucket)
	if err != nil {
		return storj.Bucket{}, Error.Wrap(err)
	}
	return respBucket, nil
}

// ListBucketsParams parmaters for ListBucketsParams method.
type ListBucketsParams struct {
	ListOpts storj.BucketListOptions
}

func (params *ListBucketsParams) toRequest(header *pb.RequestHeader) *pb.BucketListRequest {
	return &pb.BucketListRequest{
		Header:    header,
		Cursor:    []byte(params.ListOpts.Cursor),
		Limit:     int32(params.ListOpts.Limit),
		Direction: int32(params.ListOpts.Direction),
	}
}

// BatchItem returns single item for batch request.
func (params *ListBucketsParams) BatchItem() *pb.BatchRequestItem {
	return &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_BucketList{
			BucketList: params.toRequest(nil),
		},
	}
}

// ListBucketsResponse response for ListBucket request.
type ListBucketsResponse struct {
	BucketList storj.BucketList
}

func newListBucketsResponse(response *pb.BucketListResponse) ListBucketsResponse {
	bucketList := storj.BucketList{
		More: response.More,
	}
	bucketList.Items = make([]storj.Bucket, len(response.Items))
	for i, item := range response.GetItems() {
		bucketList.Items[i] = storj.Bucket{
			Name:    string(item.Name),
			Created: item.CreatedAt,
		}
	}
	return ListBucketsResponse{
		BucketList: bucketList,
	}
}

// ListBuckets lists buckets.
func (client *Client) ListBuckets(ctx context.Context, params ListBucketsParams) (_ storj.BucketList, err error) {
	defer mon.Task()(&ctx)(&err)

	resp, err := client.client.ListBuckets(ctx, params.toRequest(client.header()))
	if err != nil {
		return storj.BucketList{}, Error.Wrap(err)
	}
	resultBucketList := storj.BucketList{
		More: resp.GetMore(),
	}
	resultBucketList.Items = make([]storj.Bucket, len(resp.GetItems()))
	for i, item := range resp.GetItems() {
		resultBucketList.Items[i] = storj.Bucket{
			Name:    string(item.GetName()),
			Created: item.GetCreatedAt(),
		}
	}
	return resultBucketList, nil
}

func convertProtoToBucket(pbBucket *pb.Bucket) (bucket storj.Bucket, err error) {
	if pbBucket == nil {
		return storj.Bucket{}, nil
	}

	defaultRS := pbBucket.GetDefaultRedundancyScheme()
	defaultEP := pbBucket.GetDefaultEncryptionParameters()

	var partnerID uuid.UUID
	err = partnerID.UnmarshalJSON(pbBucket.GetPartnerId())
	if err != nil && !partnerID.IsZero() {
		return bucket, errs.New("Invalid uuid")
	}

	return storj.Bucket{
		Name:                string(pbBucket.GetName()),
		PartnerID:           partnerID,
		PathCipher:          storj.CipherSuite(pbBucket.GetPathCipher()),
		Created:             pbBucket.GetCreatedAt(),
		DefaultSegmentsSize: pbBucket.GetDefaultSegmentSize(),
		DefaultRedundancyScheme: storj.RedundancyScheme{
			Algorithm:      storj.RedundancyAlgorithm(defaultRS.GetType()),
			ShareSize:      defaultRS.GetErasureShareSize(),
			RequiredShares: int16(defaultRS.GetMinReq()),
			RepairShares:   int16(defaultRS.GetRepairThreshold()),
			OptimalShares:  int16(defaultRS.GetSuccessThreshold()),
			TotalShares:    int16(defaultRS.GetTotal()),
		},
		DefaultEncryptionParameters: storj.EncryptionParameters{
			CipherSuite: storj.CipherSuite(defaultEP.CipherSuite),
			BlockSize:   int32(defaultEP.BlockSize),
		},
	}, nil
}

// BeginObjectParams parmaters for BeginObject method.
type BeginObjectParams struct {
	Bucket               []byte
	EncryptedPath        []byte
	Version              int32
	Redundancy           storj.RedundancyScheme
	EncryptionParameters storj.EncryptionParameters
	ExpiresAt            time.Time
}

func (params *BeginObjectParams) toRequest(header *pb.RequestHeader) *pb.ObjectBeginRequest {
	return &pb.ObjectBeginRequest{
		Header:        header,
		Bucket:        params.Bucket,
		EncryptedPath: params.EncryptedPath,
		Version:       params.Version,
		ExpiresAt:     params.ExpiresAt,
		RedundancyScheme: &pb.RedundancyScheme{
			Type:             pb.RedundancyScheme_SchemeType(params.Redundancy.Algorithm),
			ErasureShareSize: params.Redundancy.ShareSize,
			MinReq:           int32(params.Redundancy.RequiredShares),
			RepairThreshold:  int32(params.Redundancy.RepairShares),
			SuccessThreshold: int32(params.Redundancy.OptimalShares),
			Total:            int32(params.Redundancy.TotalShares),
		},
		EncryptionParameters: &pb.EncryptionParameters{
			CipherSuite: pb.CipherSuite(params.EncryptionParameters.CipherSuite),
			BlockSize:   int64(params.EncryptionParameters.BlockSize),
		},
	}
}

// BatchItem returns single item for batch request.
func (params *BeginObjectParams) BatchItem() *pb.BatchRequestItem {
	return &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_ObjectBegin{
			ObjectBegin: params.toRequest(nil),
		},
	}
}

// BeginObjectResponse response for BeginObject request.
type BeginObjectResponse struct {
	StreamID             storj.StreamID
	RedundancyStrategy   eestream.RedundancyStrategy
	EncryptionParameters storj.EncryptionParameters
}

func newBeginObjectResponse(response *pb.ObjectBeginResponse, redundancyStrategy eestream.RedundancyStrategy) BeginObjectResponse {
	ep := storj.EncryptionParameters{}
	if response.EncryptionParameters != nil {
		ep = storj.EncryptionParameters{
			CipherSuite: storj.CipherSuite(response.EncryptionParameters.CipherSuite),
			BlockSize:   int32(response.EncryptionParameters.BlockSize),
		}
	}

	return BeginObjectResponse{
		StreamID:             response.StreamId,
		RedundancyStrategy:   redundancyStrategy,
		EncryptionParameters: ep,
	}
}

// BeginObject begins object creation.
func (client *Client) BeginObject(ctx context.Context, params BeginObjectParams) (_ BeginObjectResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	response, err := client.client.BeginObject(ctx, params.toRequest(client.header()))
	if err != nil {
		return BeginObjectResponse{}, Error.Wrap(err)
	}

	rs := eestream.RedundancyStrategy{}
	if response.RedundancyScheme != nil {
		rs, err = eestream.NewRedundancyStrategyFromProto(response.RedundancyScheme)
		if err != nil {
			return BeginObjectResponse{}, Error.Wrap(err)
		}
	}

	return newBeginObjectResponse(response, rs), nil
}

// CommitObjectParams parmaters for CommitObject method.
type CommitObjectParams struct {
	StreamID storj.StreamID

	EncryptedMetadataNonce storj.Nonce
	EncryptedMetadata      []byte
}

func (params *CommitObjectParams) toRequest(header *pb.RequestHeader) *pb.ObjectCommitRequest {
	return &pb.ObjectCommitRequest{
		Header:                 header,
		StreamId:               params.StreamID,
		EncryptedMetadataNonce: params.EncryptedMetadataNonce,
		EncryptedMetadata:      params.EncryptedMetadata,
	}
}

// BatchItem returns single item for batch request.
func (params *CommitObjectParams) BatchItem() *pb.BatchRequestItem {
	return &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_ObjectCommit{
			ObjectCommit: params.toRequest(nil),
		},
	}
}

// CommitObject commits a created object.
func (client *Client) CommitObject(ctx context.Context, params CommitObjectParams) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = client.client.CommitObject(ctx, params.toRequest(client.header()))

	return Error.Wrap(err)
}

// GetObjectParams parameters for GetObject method.
type GetObjectParams struct {
	Bucket        []byte
	EncryptedPath []byte
	Version       int32
}

func (params *GetObjectParams) toRequest(header *pb.RequestHeader) *pb.ObjectGetRequest {
	return &pb.ObjectGetRequest{
		Header:        header,
		Bucket:        params.Bucket,
		EncryptedPath: params.EncryptedPath,
		Version:       params.Version,
	}
}

// BatchItem returns single item for batch request.
func (params *GetObjectParams) BatchItem() *pb.BatchRequestItem {
	return &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_ObjectGet{
			ObjectGet: params.toRequest(nil),
		},
	}
}

// GetObjectResponse response for GetObject request.
type GetObjectResponse struct {
	Info storj.ObjectInfo
}

func newGetObjectResponse(response *pb.ObjectGetResponse) GetObjectResponse {
	return GetObjectResponse{
		Info: newObjectInfo(response.Object),
	}
}

func newObjectInfo(object *pb.Object) storj.ObjectInfo {
	if object == nil {
		return storj.ObjectInfo{}
	}

	info := storj.ObjectInfo{
		Bucket: string(object.Bucket),
		Path:   storj.Path(object.EncryptedPath),

		StreamID: object.StreamId,

		Created:  object.CreatedAt,
		Modified: object.CreatedAt,
		Expires:  object.ExpiresAt,
		Metadata: object.EncryptedMetadata,
		Stream: storj.Stream{
			Size: object.TotalSize,
			EncryptionParameters: storj.EncryptionParameters{
				CipherSuite: storj.CipherSuite(object.EncryptionParameters.CipherSuite),
				BlockSize:   int32(object.EncryptionParameters.BlockSize),
			},
		},
	}

	pbRS := object.RedundancyScheme
	if pbRS != nil {
		info.Stream.RedundancyScheme = storj.RedundancyScheme{
			Algorithm:      storj.RedundancyAlgorithm(pbRS.Type),
			ShareSize:      pbRS.ErasureShareSize,
			RequiredShares: int16(pbRS.MinReq),
			RepairShares:   int16(pbRS.RepairThreshold),
			OptimalShares:  int16(pbRS.SuccessThreshold),
			TotalShares:    int16(pbRS.Total),
		}
	}
	return info
}

// GetObject gets single object.
func (client *Client) GetObject(ctx context.Context, params GetObjectParams) (_ storj.ObjectInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	response, err := client.client.GetObject(ctx, params.toRequest(client.header()))

	if err != nil {
		if errs2.IsRPC(err, rpcstatus.NotFound) {
			return storj.ObjectInfo{}, storj.ErrObjectNotFound.Wrap(err)
		}
		return storj.ObjectInfo{}, Error.Wrap(err)
	}

	getResponse := newGetObjectResponse(response)
	return getResponse.Info, nil
}

// BeginDeleteObjectParams parameters for BeginDeleteObject method.
type BeginDeleteObjectParams struct {
	Bucket        []byte
	EncryptedPath []byte
	Version       int32
}

func (params *BeginDeleteObjectParams) toRequest(header *pb.RequestHeader) *pb.ObjectBeginDeleteRequest {
	return &pb.ObjectBeginDeleteRequest{
		Header:        header,
		Bucket:        params.Bucket,
		EncryptedPath: params.EncryptedPath,
		Version:       params.Version,
	}
}

// BatchItem returns single item for batch request.
func (params *BeginDeleteObjectParams) BatchItem() *pb.BatchRequestItem {
	return &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_ObjectBeginDelete{
			ObjectBeginDelete: params.toRequest(nil),
		},
	}
}

// BeginDeleteObjectResponse response for BeginDeleteObject request.
type BeginDeleteObjectResponse struct {
	StreamID storj.StreamID
}

func newBeginDeleteObjectResponse(response *pb.ObjectBeginDeleteResponse) BeginDeleteObjectResponse {
	return BeginDeleteObjectResponse{
		StreamID: response.StreamId,
	}
}

// BeginDeleteObject begins object deletion process.
func (client *Client) BeginDeleteObject(ctx context.Context, params BeginDeleteObjectParams) (_ storj.StreamID, _ storj.ObjectInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	response, err := client.client.BeginDeleteObject(ctx, params.toRequest(client.header()))
	if err != nil {
		if errs2.IsRPC(err, rpcstatus.NotFound) {
			return storj.StreamID{}, storj.ObjectInfo{}, storj.ErrObjectNotFound.Wrap(err)
		}
		return storj.StreamID{}, storj.ObjectInfo{}, Error.Wrap(err)
	}

	return response.StreamId, newObjectInfo(response.Object), nil
}

// FinishDeleteObjectParams parameters for FinishDeleteObject method.
type FinishDeleteObjectParams struct {
	StreamID storj.StreamID
}

func (params *FinishDeleteObjectParams) toRequest(header *pb.RequestHeader) *pb.ObjectFinishDeleteRequest {
	return &pb.ObjectFinishDeleteRequest{
		Header:   header,
		StreamId: params.StreamID,
	}
}

// BatchItem returns single item for batch request.
func (params *FinishDeleteObjectParams) BatchItem() *pb.BatchRequestItem {
	return &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_ObjectFinishDelete{
			ObjectFinishDelete: params.toRequest(nil),
		},
	}
}

// FinishDeleteObject finishes object deletion process.
func (client *Client) FinishDeleteObject(ctx context.Context, params FinishDeleteObjectParams) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = client.client.FinishDeleteObject(ctx, params.toRequest(client.header()))

	return Error.Wrap(err)
}

// ListObjectsParams parameters for ListObjects method.
type ListObjectsParams struct {
	Bucket          []byte
	EncryptedPrefix []byte
	EncryptedCursor []byte
	Limit           int32
	IncludeMetadata bool
	Recursive       bool
}

func (params *ListObjectsParams) toRequest(header *pb.RequestHeader) *pb.ObjectListRequest {
	return &pb.ObjectListRequest{
		Header:          header,
		Bucket:          params.Bucket,
		EncryptedPrefix: params.EncryptedPrefix,
		EncryptedCursor: params.EncryptedCursor,
		Limit:           params.Limit,
		ObjectIncludes: &pb.ObjectListItemIncludes{
			Metadata: params.IncludeMetadata,
		},
		Recursive: params.Recursive,
	}
}

// BatchItem returns single item for batch request.
func (params *ListObjectsParams) BatchItem() *pb.BatchRequestItem {
	return &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_ObjectList{
			ObjectList: params.toRequest(nil),
		},
	}
}

// ListObjectsResponse response for ListObjects request.
type ListObjectsResponse struct {
	Items []storj.ObjectListItem
	More  bool
}

func newListObjectsResponse(response *pb.ObjectListResponse, encryptedPrefix []byte, recursive bool) ListObjectsResponse {
	objects := make([]storj.ObjectListItem, len(response.Items))
	for i, object := range response.Items {
		encryptedPath := object.EncryptedPath
		isPrefix := false
		if !recursive && len(encryptedPath) != 0 && encryptedPath[len(encryptedPath)-1] == '/' && !bytes.Equal(encryptedPath, encryptedPrefix) {
			isPrefix = true
		}

		objects[i] = storj.ObjectListItem{
			EncryptedPath:          object.EncryptedPath,
			Version:                object.Version,
			Status:                 int32(object.Status),
			StatusAt:               object.StatusAt,
			CreatedAt:              object.CreatedAt,
			ExpiresAt:              object.ExpiresAt,
			EncryptedMetadataNonce: object.EncryptedMetadataNonce,
			EncryptedMetadata:      object.EncryptedMetadata,

			IsPrefix: isPrefix,
		}
	}

	return ListObjectsResponse{
		Items: objects,
		More:  response.More,
	}
}

// ListObjects lists objects according to specific parameters.
func (client *Client) ListObjects(ctx context.Context, params ListObjectsParams) (_ []storj.ObjectListItem, more bool, err error) {
	defer mon.Task()(&ctx)(&err)

	response, err := client.client.ListObjects(ctx, params.toRequest(client.header()))
	if err != nil {
		return []storj.ObjectListItem{}, false, Error.Wrap(err)
	}

	listResponse := newListObjectsResponse(response, params.EncryptedPrefix, params.Recursive)
	return listResponse.Items, listResponse.More, Error.Wrap(err)
}

// BeginSegmentParams parameters for BeginSegment method.
type BeginSegmentParams struct {
	StreamID      storj.StreamID
	Position      storj.SegmentPosition
	MaxOrderLimit int64
}

func (params *BeginSegmentParams) toRequest(header *pb.RequestHeader) *pb.SegmentBeginRequest {
	return &pb.SegmentBeginRequest{
		Header:   header,
		StreamId: params.StreamID,
		Position: &pb.SegmentPosition{
			PartNumber: params.Position.PartNumber,
			Index:      params.Position.Index,
		},
		MaxOrderLimit: params.MaxOrderLimit,
	}
}

// BatchItem returns single item for batch request.
func (params *BeginSegmentParams) BatchItem() *pb.BatchRequestItem {
	return &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_SegmentBegin{
			SegmentBegin: params.toRequest(nil),
		},
	}
}

// BeginSegmentResponse response for BeginSegment request.
type BeginSegmentResponse struct {
	SegmentID       storj.SegmentID
	Limits          []*pb.AddressedOrderLimit
	PiecePrivateKey storj.PiecePrivateKey
}

func newBeginSegmentResponse(response *pb.SegmentBeginResponse) BeginSegmentResponse {
	return BeginSegmentResponse{
		SegmentID:       response.SegmentId,
		Limits:          response.AddressedLimits,
		PiecePrivateKey: response.PrivateKey,
	}
}

// BeginSegment begins a segment upload.
func (client *Client) BeginSegment(ctx context.Context, params BeginSegmentParams) (_ storj.SegmentID, limits []*pb.AddressedOrderLimit, piecePrivateKey storj.PiecePrivateKey, err error) {
	defer mon.Task()(&ctx)(&err)

	response, err := client.client.BeginSegment(ctx, params.toRequest(client.header()))
	if err != nil {
		return storj.SegmentID{}, nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	return response.SegmentId, response.AddressedLimits, response.PrivateKey, nil
}

// CommitSegmentParams parameters for CommitSegment method.
type CommitSegmentParams struct {
	SegmentID         storj.SegmentID
	Encryption        storj.SegmentEncryption
	SizeEncryptedData int64

	UploadResult []*pb.SegmentPieceUploadResult
}

func (params *CommitSegmentParams) toRequest(header *pb.RequestHeader) *pb.SegmentCommitRequest {
	return &pb.SegmentCommitRequest{
		Header:    header,
		SegmentId: params.SegmentID,

		EncryptedKeyNonce: params.Encryption.EncryptedKeyNonce,
		EncryptedKey:      params.Encryption.EncryptedKey,
		SizeEncryptedData: params.SizeEncryptedData,
		UploadResult:      params.UploadResult,
	}
}

// BatchItem returns single item for batch request.
func (params *CommitSegmentParams) BatchItem() *pb.BatchRequestItem {
	return &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_SegmentCommit{
			SegmentCommit: params.toRequest(nil),
		},
	}
}

// CommitSegment commits an uploaded segment.
func (client *Client) CommitSegment(ctx context.Context, params CommitSegmentParams) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = client.client.CommitSegment(ctx, params.toRequest(client.header()))

	return Error.Wrap(err)
}

// MakeInlineSegmentParams parameters for MakeInlineSegment method.
type MakeInlineSegmentParams struct {
	StreamID            storj.StreamID
	Position            storj.SegmentPosition
	Encryption          storj.SegmentEncryption
	EncryptedInlineData []byte
}

func (params *MakeInlineSegmentParams) toRequest(header *pb.RequestHeader) *pb.SegmentMakeInlineRequest {
	return &pb.SegmentMakeInlineRequest{
		Header:   header,
		StreamId: params.StreamID,
		Position: &pb.SegmentPosition{
			PartNumber: params.Position.PartNumber,
			Index:      params.Position.Index,
		},
		EncryptedKeyNonce:   params.Encryption.EncryptedKeyNonce,
		EncryptedKey:        params.Encryption.EncryptedKey,
		EncryptedInlineData: params.EncryptedInlineData,
	}
}

// BatchItem returns single item for batch request.
func (params *MakeInlineSegmentParams) BatchItem() *pb.BatchRequestItem {
	return &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_SegmentMakeInline{
			SegmentMakeInline: params.toRequest(nil),
		},
	}
}

// MakeInlineSegment creates an inline segment.
func (client *Client) MakeInlineSegment(ctx context.Context, params MakeInlineSegmentParams) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = client.client.MakeInlineSegment(ctx, params.toRequest(client.header()))

	return Error.Wrap(err)
}

// BeginDeleteSegmentParams parameters for BeginDeleteSegment method.
type BeginDeleteSegmentParams struct {
	StreamID storj.StreamID
	Position storj.SegmentPosition
}

func (params *BeginDeleteSegmentParams) toRequest(header *pb.RequestHeader) *pb.SegmentBeginDeleteRequest {
	return &pb.SegmentBeginDeleteRequest{
		Header:   header,
		StreamId: params.StreamID,
		Position: &pb.SegmentPosition{
			PartNumber: params.Position.PartNumber,
			Index:      params.Position.Index,
		},
	}
}

// BatchItem returns single item for batch request.
func (params *BeginDeleteSegmentParams) BatchItem() *pb.BatchRequestItem {
	return &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_SegmentBeginDelete{
			SegmentBeginDelete: params.toRequest(nil),
		},
	}
}

// BeginDeleteSegmentResponse response for BeginDeleteSegment request.
type BeginDeleteSegmentResponse struct {
	SegmentID       storj.SegmentID
	Limits          []*pb.AddressedOrderLimit
	PiecePrivateKey storj.PiecePrivateKey
}

func newBeginDeleteSegmentResponse(response *pb.SegmentBeginDeleteResponse) BeginDeleteSegmentResponse {
	return BeginDeleteSegmentResponse{
		SegmentID:       response.SegmentId,
		Limits:          response.AddressedLimits,
		PiecePrivateKey: response.PrivateKey,
	}
}

// BeginDeleteSegment begins segment deletion process.
func (client *Client) BeginDeleteSegment(ctx context.Context, params BeginDeleteSegmentParams) (_ storj.SegmentID, limits []*pb.AddressedOrderLimit, _ storj.PiecePrivateKey, err error) {
	defer mon.Task()(&ctx)(&err)

	response, err := client.client.BeginDeleteSegment(ctx, params.toRequest(client.header()))
	if err != nil {
		return storj.SegmentID{}, nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	return response.SegmentId, response.AddressedLimits, response.PrivateKey, nil
}

// FinishDeleteSegmentParams parameters for FinishDeleteSegment method.
type FinishDeleteSegmentParams struct {
	SegmentID storj.SegmentID

	DeleteResults []*pb.SegmentPieceDeleteResult
}

func (params *FinishDeleteSegmentParams) toRequest(header *pb.RequestHeader) *pb.SegmentFinishDeleteRequest {
	return &pb.SegmentFinishDeleteRequest{
		Header:    header,
		SegmentId: params.SegmentID,
		Results:   params.DeleteResults,
	}
}

// BatchItem returns single item for batch request.
func (params *FinishDeleteSegmentParams) BatchItem() *pb.BatchRequestItem {
	return &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_SegmentFinishDelete{
			SegmentFinishDelete: params.toRequest(nil),
		},
	}
}

// FinishDeleteSegment finishes segment upload process.
func (client *Client) FinishDeleteSegment(ctx context.Context, params FinishDeleteSegmentParams) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = client.client.FinishDeleteSegment(ctx, params.toRequest(client.header()))

	return Error.Wrap(err)
}

// DownloadSegmentParams parameters for DownloadSegment method.
type DownloadSegmentParams struct {
	StreamID storj.StreamID
	Position storj.SegmentPosition
}

func (params *DownloadSegmentParams) toRequest(header *pb.RequestHeader) *pb.SegmentDownloadRequest {
	return &pb.SegmentDownloadRequest{
		Header:   header,
		StreamId: params.StreamID,
		CursorPosition: &pb.SegmentPosition{
			PartNumber: params.Position.PartNumber,
			Index:      params.Position.Index,
		},
	}
}

// BatchItem returns single item for batch request.
func (params *DownloadSegmentParams) BatchItem() *pb.BatchRequestItem {
	return &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_SegmentDownload{
			SegmentDownload: params.toRequest(nil),
		},
	}
}

// DownloadSegmentResponse response for DownloadSegment request.
type DownloadSegmentResponse struct {
	Info storj.SegmentDownloadInfo

	Limits []*pb.AddressedOrderLimit
}

func newDownloadSegmentResponse(response *pb.SegmentDownloadResponse) DownloadSegmentResponse {
	info := storj.SegmentDownloadInfo{
		SegmentID:           response.SegmentId,
		Size:                response.SegmentSize,
		EncryptedInlineData: response.EncryptedInlineData,
		PiecePrivateKey:     response.PrivateKey,
		SegmentEncryption: storj.SegmentEncryption{
			EncryptedKeyNonce: response.EncryptedKeyNonce,
			EncryptedKey:      response.EncryptedKey,
		},
	}
	if response.Next != nil {
		info.Next = storj.SegmentPosition{
			PartNumber: response.Next.PartNumber,
			Index:      response.Next.Index,
		}
	}

	for i := range response.AddressedLimits {
		if response.AddressedLimits[i].Limit == nil {
			response.AddressedLimits[i] = nil
		}
	}
	return DownloadSegmentResponse{
		Info:   info,
		Limits: response.AddressedLimits,
	}
}

// DownloadSegment gets information for downloading remote segment or data
// from an inline segment.
func (client *Client) DownloadSegment(ctx context.Context, params DownloadSegmentParams) (_ storj.SegmentDownloadInfo, _ []*pb.AddressedOrderLimit, err error) {
	defer mon.Task()(&ctx)(&err)

	response, err := client.client.DownloadSegment(ctx, params.toRequest(client.header()))
	if err != nil {
		if errs2.IsRPC(err, rpcstatus.NotFound) {
			return storj.SegmentDownloadInfo{}, nil, storj.ErrObjectNotFound.Wrap(err)
		}
		return storj.SegmentDownloadInfo{}, nil, Error.Wrap(err)
	}

	downloadResponse := newDownloadSegmentResponse(response)
	return downloadResponse.Info, downloadResponse.Limits, nil
}

// ListSegmentsParams parameters for ListSegment method.
type ListSegmentsParams struct {
	StreamID       storj.StreamID
	CursorPosition storj.SegmentPosition
	Limit          int32
}

// ListSegmentsResponse response for ListSegments request.
type ListSegmentsResponse struct {
	Items []storj.SegmentListItem
	More  bool
}

func (params *ListSegmentsParams) toRequest(header *pb.RequestHeader) *pb.SegmentListRequest {
	return &pb.SegmentListRequest{
		Header:   header,
		StreamId: params.StreamID,
		CursorPosition: &pb.SegmentPosition{
			PartNumber: params.CursorPosition.PartNumber,
			Index:      params.CursorPosition.Index,
		},
		Limit: params.Limit,
	}
}

// BatchItem returns single item for batch request.
func (params *ListSegmentsParams) BatchItem() *pb.BatchRequestItem {
	return &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_SegmentList{
			SegmentList: params.toRequest(nil),
		},
	}
}

func newListSegmentsResponse(response *pb.SegmentListResponse) ListSegmentsResponse {
	items := make([]storj.SegmentListItem, len(response.Items))
	for i, responseItem := range response.Items {
		items[i] = storj.SegmentListItem{
			Position: storj.SegmentPosition{
				PartNumber: responseItem.Position.PartNumber,
				Index:      responseItem.Position.Index,
			},
		}
	}
	return ListSegmentsResponse{
		Items: items,
		More:  response.More,
	}
}

// ListSegments lists object segments.
func (client *Client) ListSegments(ctx context.Context, params ListSegmentsParams) (_ []storj.SegmentListItem, more bool, err error) {
	defer mon.Task()(&ctx)(&err)

	response, err := client.client.ListSegments(ctx, params.toRequest(client.header()))
	if err != nil {
		if errs2.IsRPC(err, rpcstatus.NotFound) {
			return []storj.SegmentListItem{}, false, storj.ErrObjectNotFound.Wrap(err)
		}
		return []storj.SegmentListItem{}, false, Error.Wrap(err)
	}

	listResponse := newListSegmentsResponse(response)
	return listResponse.Items, listResponse.More, Error.Wrap(err)
}

// Batch sends multiple requests in one batch.
func (client *Client) Batch(ctx context.Context, requests ...BatchItem) (resp []BatchResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	batchItems := make([]*pb.BatchRequestItem, len(requests))
	for i, request := range requests {
		batchItems[i] = request.BatchItem()
	}
	response, err := client.client.Batch(ctx, &pb.BatchRequest{
		Header:   client.header(),
		Requests: batchItems,
	})
	if err != nil {
		return []BatchResponse{}, Error.Wrap(err)
	}

	resp = make([]BatchResponse, len(response.Responses))
	for i, response := range response.Responses {
		resp[i] = BatchResponse{
			pbRequest:  batchItems[i].Request,
			pbResponse: response.Response,
		}
	}

	return resp, nil
}

// SetRawAPIKey sets the client's raw API key. Mainly used for testing.
func (client *Client) SetRawAPIKey(key []byte) {
	client.apiKeyRaw = key
}
