// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metaclient

import (
	"bytes"
	"context"
	"sync"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"

	"storj.io/common/errs2"
	"storj.io/common/macaroon"
	"storj.io/common/pb"
	"storj.io/common/rpc"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/storj"
	"storj.io/uplink/private/eestream"
)

var (
	mon = monkit.Package()

	// Error is the errs class of standard metainfo errors.
	Error = errs.Class("metaclient")
)

// Client creates a grpcClient.
type Client struct {
	mu        sync.Mutex
	conn      *rpc.Conn
	client    pb.DRPCMetainfoClient
	apiKeyRaw []byte

	userAgent string
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

	conn, err := dialer.DialNode(ctx, url, rpc.DialOptions{ForceTCPFastOpenMultidialSupport: true})
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
	client.mu.Lock()
	defer client.mu.Unlock()

	if client.conn != nil {
		err := client.conn.Close()
		client.conn = nil
		return Error.Wrap(err)
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
func (client *Client) GetProjectInfo(ctx context.Context) (response *pb.ProjectInfoResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	err = WithRetry(ctx, func(ctx context.Context) error {
		response, err = client.client.ProjectInfo(ctx, &pb.ProjectInfoRequest{
			Header: client.header(),
		})
		return err
	})
	return response, err
}

// CreateBucketParams parameters for CreateBucket method.
type CreateBucketParams struct {
	Name []byte
}

func (params *CreateBucketParams) toRequest(header *pb.RequestHeader) *pb.BucketCreateRequest {
	return &pb.BucketCreateRequest{
		Header: header,
		Name:   params.Name,
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

// CreateBucketResponse response for CreateBucket request.
type CreateBucketResponse struct {
	Bucket Bucket
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
func (client *Client) CreateBucket(ctx context.Context, params CreateBucketParams) (respBucket Bucket, err error) {
	defer mon.Task()(&ctx)(&err)

	var response *pb.BucketCreateResponse
	err = WithRetry(ctx, func(ctx context.Context) error {
		response, err = client.client.CreateBucket(ctx, params.toRequest(client.header()))
		return err
	})
	if err != nil {
		return Bucket{}, Error.Wrap(err)
	}

	respBucket, err = convertProtoToBucket(response.Bucket)
	if err != nil {
		return Bucket{}, Error.Wrap(err)
	}
	return respBucket, nil
}

// GetBucketParams parameters for GetBucketParams method.
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
	Bucket Bucket
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
func (client *Client) GetBucket(ctx context.Context, params GetBucketParams) (respBucket Bucket, err error) {
	defer mon.Task()(&ctx)(&err)

	var response *pb.BucketGetResponse
	err = WithRetry(ctx, func(ctx context.Context) error {
		// TODO(moby) make sure bucket not found is properly handled
		response, err = client.client.GetBucket(ctx, params.toRequest(client.header()))
		return err
	})
	if err != nil {
		if errs2.IsRPC(err, rpcstatus.NotFound) {
			return Bucket{}, ErrBucketNotFound.Wrap(err)
		}
		return Bucket{}, Error.Wrap(err)
	}

	respBucket, err = convertProtoToBucket(response.Bucket)
	if err != nil {
		return Bucket{}, Error.Wrap(err)
	}
	return respBucket, nil
}

// GetBucketLocationParams parameters for GetBucketLocation method.
type GetBucketLocationParams struct {
	Name []byte
}

func (params *GetBucketLocationParams) toRequest(header *pb.RequestHeader) *pb.GetBucketLocationRequest {
	return &pb.GetBucketLocationRequest{
		Header: header,
		Name:   params.Name,
	}
}

// BatchItem returns single item for batch request.
func (params *GetBucketLocationParams) BatchItem() *pb.BatchRequestItem {
	return &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_BucketGetLocation{
			BucketGetLocation: params.toRequest(nil),
		},
	}
}

// GetBucketLocationResponse response for GetBucketLocation request.
type GetBucketLocationResponse struct {
	Location []byte
}

// GetBucketLocation returns a bucket location.
func (client *Client) GetBucketLocation(ctx context.Context, params GetBucketLocationParams) (_ GetBucketLocationResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	var response *pb.GetBucketLocationResponse
	err = WithRetry(ctx, func(ctx context.Context) error {
		response, err = client.client.GetBucketLocation(ctx, params.toRequest(client.header()))
		return err
	})
	if err != nil {
		return GetBucketLocationResponse{}, Error.Wrap(err)
	}

	return GetBucketLocationResponse{
		Location: response.Location,
	}, nil
}

// DeleteBucketParams parameters for DeleteBucket method.
type DeleteBucketParams struct {
	Name      []byte
	DeleteAll bool
}

func (params *DeleteBucketParams) toRequest(header *pb.RequestHeader) *pb.BucketDeleteRequest {
	return &pb.BucketDeleteRequest{
		Header:    header,
		Name:      params.Name,
		DeleteAll: params.DeleteAll,
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
func (client *Client) DeleteBucket(ctx context.Context, params DeleteBucketParams) (_ Bucket, err error) {
	defer mon.Task()(&ctx)(&err)

	var response *pb.BucketDeleteResponse
	err = WithRetry(ctx, func(ctx context.Context) error {
		// TODO(moby) make sure bucket not found is properly handled
		response, err = client.client.DeleteBucket(ctx, params.toRequest(client.header()))
		return err
	})
	if err != nil {
		if errs2.IsRPC(err, rpcstatus.NotFound) {
			return Bucket{}, ErrBucketNotFound.Wrap(err)
		}
		return Bucket{}, Error.Wrap(err)
	}

	respBucket, err := convertProtoToBucket(response.Bucket)
	if err != nil {
		return Bucket{}, Error.Wrap(err)
	}
	return respBucket, nil
}

// ListBucketsParams parameters for ListBucketsParams method.
type ListBucketsParams struct {
	ListOpts BucketListOptions
}

func (params *ListBucketsParams) toRequest(header *pb.RequestHeader) *pb.BucketListRequest {
	return &pb.BucketListRequest{
		Header:    header,
		Cursor:    []byte(params.ListOpts.Cursor),
		Limit:     int32(params.ListOpts.Limit),
		Direction: params.ListOpts.Direction,
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
	BucketList BucketList
}

func newListBucketsResponse(response *pb.BucketListResponse) ListBucketsResponse {
	bucketList := BucketList{
		More: response.More,
	}
	bucketList.Items = make([]Bucket, len(response.Items))
	for i, item := range response.GetItems() {
		bucketList.Items[i] = Bucket{
			Name:    string(item.Name),
			Created: item.CreatedAt,
		}
	}
	return ListBucketsResponse{
		BucketList: bucketList,
	}
}

// ListBuckets lists buckets.
func (client *Client) ListBuckets(ctx context.Context, params ListBucketsParams) (_ BucketList, err error) {
	defer mon.Task()(&ctx)(&err)

	var response *pb.BucketListResponse
	err = WithRetry(ctx, func(ctx context.Context) error {
		response, err = client.client.ListBuckets(ctx, params.toRequest(client.header()))
		return err
	})
	if err != nil {
		return BucketList{}, Error.Wrap(err)
	}

	resultBucketList := BucketList{
		More: response.GetMore(),
	}
	resultBucketList.Items = make([]Bucket, len(response.GetItems()))
	for i, item := range response.GetItems() {
		resultBucketList.Items[i] = Bucket{
			Name:        string(item.GetName()),
			Created:     item.GetCreatedAt(),
			Attribution: string(item.GetUserAgent()),
		}
	}
	return resultBucketList, nil
}

func convertProtoToBucket(pbBucket *pb.Bucket) (bucket Bucket, err error) {
	if pbBucket == nil {
		return Bucket{}, nil
	}

	return Bucket{
		Name:    string(pbBucket.GetName()),
		Created: pbBucket.GetCreatedAt(),
	}, nil
}

// BeginObjectParams parameters for BeginObject method.
type BeginObjectParams struct {
	Bucket               []byte
	EncryptedObjectKey   []byte
	Version              int32
	Redundancy           storj.RedundancyScheme
	EncryptionParameters storj.EncryptionParameters
	ExpiresAt            time.Time

	EncryptedMetadata             []byte
	EncryptedMetadataEncryptedKey []byte
	EncryptedMetadataNonce        storj.Nonce
}

func (params *BeginObjectParams) toRequest(header *pb.RequestHeader) *pb.ObjectBeginRequest {
	return &pb.ObjectBeginRequest{
		Header:             header,
		Bucket:             params.Bucket,
		EncryptedObjectKey: params.EncryptedObjectKey,
		Version:            params.Version,
		ExpiresAt:          params.ExpiresAt,
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

		EncryptedMetadata:             params.EncryptedMetadata,
		EncryptedMetadataEncryptedKey: params.EncryptedMetadataEncryptedKey,
		EncryptedMetadataNonce:        params.EncryptedMetadataNonce,
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
	StreamID storj.StreamID
}

func newBeginObjectResponse(response *pb.ObjectBeginResponse) BeginObjectResponse {
	return BeginObjectResponse{
		StreamID: response.StreamId,
	}
}

// BeginObject begins object creation.
func (client *Client) BeginObject(ctx context.Context, params BeginObjectParams) (_ BeginObjectResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	var response *pb.ObjectBeginResponse
	err = WithRetry(ctx, func(ctx context.Context) error {
		response, err = client.client.BeginObject(ctx, params.toRequest(client.header()))
		return err
	})
	if err != nil {
		return BeginObjectResponse{}, Error.Wrap(err)
	}

	return newBeginObjectResponse(response), nil
}

// CommitObjectParams parameters for CommitObject method.
type CommitObjectParams struct {
	StreamID storj.StreamID

	EncryptedMetadataNonce        storj.Nonce
	EncryptedMetadata             []byte
	EncryptedMetadataEncryptedKey []byte
}

func (params *CommitObjectParams) toRequest(header *pb.RequestHeader) *pb.ObjectCommitRequest {
	return &pb.ObjectCommitRequest{
		Header:                        header,
		StreamId:                      params.StreamID,
		EncryptedMetadataNonce:        params.EncryptedMetadataNonce,
		EncryptedMetadata:             params.EncryptedMetadata,
		EncryptedMetadataEncryptedKey: params.EncryptedMetadataEncryptedKey,
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

	err = WithRetry(ctx, func(ctx context.Context) error {
		_, err = client.client.CommitObject(ctx, params.toRequest(client.header()))
		return err
	})
	return Error.Wrap(err)
}

// GetObjectParams parameters for GetObject method.
type GetObjectParams struct {
	Bucket             []byte
	EncryptedObjectKey []byte
	Version            int32

	RedundancySchemePerSegment bool
}

func (params *GetObjectParams) toRequest(header *pb.RequestHeader) *pb.ObjectGetRequest {
	return &pb.ObjectGetRequest{
		Header:                     header,
		Bucket:                     params.Bucket,
		EncryptedObjectKey:         params.EncryptedObjectKey,
		Version:                    params.Version,
		RedundancySchemePerSegment: params.RedundancySchemePerSegment,
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
	Info RawObjectItem
}

func newGetObjectResponse(response *pb.ObjectGetResponse) GetObjectResponse {
	return GetObjectResponse{
		Info: newObjectInfo(response.Object),
	}
}

func newObjectInfo(object *pb.Object) RawObjectItem {
	if object == nil {
		return RawObjectItem{}
	}

	info := RawObjectItem{
		Bucket:             string(object.Bucket),
		EncryptedObjectKey: object.EncryptedObjectKey,
		Version:            uint32(object.Version),

		StreamID: object.StreamId,

		Created:                       object.CreatedAt,
		Modified:                      object.CreatedAt,
		PlainSize:                     object.PlainSize,
		Expires:                       object.ExpiresAt,
		EncryptedMetadata:             object.EncryptedMetadata,
		EncryptedMetadataNonce:        object.EncryptedMetadataNonce,
		EncryptedMetadataEncryptedKey: object.EncryptedMetadataEncryptedKey,

		EncryptionParameters: storj.EncryptionParameters{
			CipherSuite: storj.CipherSuite(object.EncryptionParameters.CipherSuite),
			BlockSize:   int32(object.EncryptionParameters.BlockSize),
		},
	}

	pbRS := object.RedundancyScheme
	if pbRS != nil {
		info.RedundancyScheme = storj.RedundancyScheme{
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
func (client *Client) GetObject(ctx context.Context, params GetObjectParams) (_ RawObjectItem, err error) {
	defer mon.Task()(&ctx)(&err)

	var response *pb.ObjectGetResponse
	err = WithRetry(ctx, func(ctx context.Context) error {
		response, err = client.client.GetObject(ctx, params.toRequest(client.header()))
		return err
	})
	if err != nil {
		if errs2.IsRPC(err, rpcstatus.NotFound) {
			return RawObjectItem{}, ErrObjectNotFound.Wrap(err)
		}
		return RawObjectItem{}, Error.Wrap(err)
	}

	getResponse := newGetObjectResponse(response)
	return getResponse.Info, nil
}

// GetObjectIPsParams are params for the GetObjectIPs request.
type GetObjectIPsParams struct {
	Bucket             []byte
	EncryptedObjectKey []byte
	Version            int32
}

// GetObjectIPsResponse is the response from GetObjectIPs.
type GetObjectIPsResponse struct {
	IPPorts            [][]byte
	SegmentCount       int64
	PieceCount         int64
	ReliablePieceCount int64
}

func (params *GetObjectIPsParams) toRequest(header *pb.RequestHeader) *pb.ObjectGetIPsRequest {
	return &pb.ObjectGetIPsRequest{
		Header:             header,
		Bucket:             params.Bucket,
		EncryptedObjectKey: params.EncryptedObjectKey,
		Version:            params.Version,
	}
}

// GetObjectIPs returns the IP addresses of the nodes which hold the object.
func (client *Client) GetObjectIPs(ctx context.Context, params GetObjectIPsParams) (r *GetObjectIPsResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	var response *pb.ObjectGetIPsResponse
	err = WithRetry(ctx, func(ctx context.Context) error {
		response, err = client.client.GetObjectIPs(ctx, params.toRequest(client.header()))
		return err
	})
	if err != nil {
		if errs2.IsRPC(err, rpcstatus.NotFound) {
			return nil, ErrObjectNotFound.Wrap(err)
		}
		return nil, Error.Wrap(err)
	}

	return &GetObjectIPsResponse{
		IPPorts:            response.Ips,
		SegmentCount:       response.SegmentCount,
		PieceCount:         response.PieceCount,
		ReliablePieceCount: response.ReliablePieceCount,
	}, nil
}

// UpdateObjectMetadataParams are params for the UpdateObjectMetadata request.
type UpdateObjectMetadataParams struct {
	Bucket             []byte
	EncryptedObjectKey []byte
	Version            int32
	StreamID           storj.StreamID

	EncryptedMetadataNonce        storj.Nonce
	EncryptedMetadata             []byte
	EncryptedMetadataEncryptedKey []byte
}

func (params *UpdateObjectMetadataParams) toRequest(header *pb.RequestHeader) *pb.ObjectUpdateMetadataRequest {
	return &pb.ObjectUpdateMetadataRequest{
		Header:                        header,
		Bucket:                        params.Bucket,
		EncryptedObjectKey:            params.EncryptedObjectKey,
		Version:                       params.Version,
		StreamId:                      params.StreamID,
		EncryptedMetadataNonce:        params.EncryptedMetadataNonce,
		EncryptedMetadata:             params.EncryptedMetadata,
		EncryptedMetadataEncryptedKey: params.EncryptedMetadataEncryptedKey,
	}
}

// UpdateObjectMetadata replaces objects metadata.
func (client *Client) UpdateObjectMetadata(ctx context.Context, params UpdateObjectMetadataParams) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = WithRetry(ctx, func(ctx context.Context) error {
		_, err = client.client.UpdateObjectMetadata(ctx, params.toRequest(client.header()))
		return err
	})
	if err != nil {
		if errs2.IsRPC(err, rpcstatus.NotFound) {
			return ErrObjectNotFound.Wrap(err)
		}
	}

	return Error.Wrap(err)
}

// BeginDeleteObjectParams parameters for BeginDeleteObject method.
type BeginDeleteObjectParams struct {
	Bucket             []byte
	EncryptedObjectKey []byte
	Version            int32
	StreamID           storj.StreamID
	Status             int32
}

func (params *BeginDeleteObjectParams) toRequest(header *pb.RequestHeader) *pb.ObjectBeginDeleteRequest {
	return &pb.ObjectBeginDeleteRequest{
		Header:             header,
		Bucket:             params.Bucket,
		EncryptedObjectKey: params.EncryptedObjectKey,
		Version:            params.Version,
		StreamId:           &params.StreamID,
		Status:             params.Status,
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
}

func newBeginDeleteObjectResponse(response *pb.ObjectBeginDeleteResponse) BeginDeleteObjectResponse {
	return BeginDeleteObjectResponse{}
}

// BeginDeleteObject begins object deletion process.
func (client *Client) BeginDeleteObject(ctx context.Context, params BeginDeleteObjectParams) (_ RawObjectItem, err error) {
	defer mon.Task()(&ctx)(&err)

	var response *pb.ObjectBeginDeleteResponse
	err = WithRetry(ctx, func(ctx context.Context) error {
		// response.StreamID is not processed because satellite will always return nil
		response, err = client.client.BeginDeleteObject(ctx, params.toRequest(client.header()))
		return err
	})
	if err != nil {
		if errs2.IsRPC(err, rpcstatus.NotFound) {
			return RawObjectItem{}, ErrObjectNotFound.Wrap(err)
		}
		return RawObjectItem{}, Error.Wrap(err)
	}

	return newObjectInfo(response.Object), nil
}

// ListObjectsParams parameters for ListObjects method.
type ListObjectsParams struct {
	Bucket                []byte
	EncryptedPrefix       []byte
	EncryptedCursor       []byte
	Limit                 int32
	IncludeCustomMetadata bool
	IncludeSystemMetadata bool
	Recursive             bool
	Status                int32
}

func (params *ListObjectsParams) toRequest(header *pb.RequestHeader) *pb.ObjectListRequest {
	return &pb.ObjectListRequest{
		Header:          header,
		Bucket:          params.Bucket,
		EncryptedPrefix: params.EncryptedPrefix,
		EncryptedCursor: params.EncryptedCursor,
		Limit:           params.Limit,
		ObjectIncludes: &pb.ObjectListItemIncludes{
			Metadata:              params.IncludeCustomMetadata,
			ExcludeSystemMetadata: !params.IncludeSystemMetadata,
		},
		UseObjectIncludes: true,
		Recursive:         params.Recursive,
		Status:            pb.Object_Status(params.Status),
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
	Items []RawObjectListItem
	More  bool
}

func newListObjectsResponse(response *pb.ObjectListResponse, encryptedPrefix []byte, recursive bool) ListObjectsResponse {
	objects := make([]RawObjectListItem, len(response.Items))
	for i, object := range response.Items {
		encryptedObjectKey := object.EncryptedObjectKey
		isPrefix := false
		if !recursive && len(encryptedObjectKey) != 0 && encryptedObjectKey[len(encryptedObjectKey)-1] == '/' && !bytes.Equal(encryptedObjectKey, encryptedPrefix) {
			isPrefix = true
		}

		objects[i] = RawObjectListItem{
			EncryptedObjectKey:            object.EncryptedObjectKey,
			Version:                       object.Version,
			Status:                        int32(object.Status),
			StatusAt:                      object.StatusAt,
			CreatedAt:                     object.CreatedAt,
			ExpiresAt:                     object.ExpiresAt,
			PlainSize:                     object.PlainSize,
			EncryptedMetadataNonce:        object.EncryptedMetadataNonce,
			EncryptedMetadataEncryptedKey: object.EncryptedMetadataEncryptedKey,
			EncryptedMetadata:             object.EncryptedMetadata,

			IsPrefix: isPrefix,
		}

		if object.StreamId != nil {
			objects[i].StreamID = *object.StreamId
		}
	}

	return ListObjectsResponse{
		Items: objects,
		More:  response.More,
	}
}

// ListObjects lists objects according to specific parameters.
func (client *Client) ListObjects(ctx context.Context, params ListObjectsParams) (_ []RawObjectListItem, more bool, err error) {
	defer mon.Task()(&ctx)(&err)

	var response *pb.ObjectListResponse
	err = WithRetry(ctx, func(ctx context.Context) error {
		response, err = client.client.ListObjects(ctx, params.toRequest(client.header()))
		return err
	})
	if err != nil {
		return []RawObjectListItem{}, false, Error.Wrap(err)
	}

	listResponse := newListObjectsResponse(response, params.EncryptedPrefix, params.Recursive)
	return listResponse.Items, listResponse.More, Error.Wrap(err)
}

// ListPendingObjectStreamsParams parameters for ListPendingObjectStreams method.
type ListPendingObjectStreamsParams struct {
	Bucket             []byte
	EncryptedObjectKey []byte
	EncryptedCursor    []byte
	Limit              int32
}

func (params *ListPendingObjectStreamsParams) toRequest(header *pb.RequestHeader) *pb.ObjectListPendingStreamsRequest {
	return &pb.ObjectListPendingStreamsRequest{
		Header:             header,
		Bucket:             params.Bucket,
		EncryptedObjectKey: params.EncryptedObjectKey,
		StreamIdCursor:     params.EncryptedCursor,
		Limit:              params.Limit,
	}
}

// BatchItem returns single item for batch request.
func (params *ListPendingObjectStreamsParams) BatchItem() *pb.BatchRequestItem {
	return &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_ObjectListPendingStreams{
			ObjectListPendingStreams: params.toRequest(nil),
		},
	}
}

// ListPendingObjectStreamsResponse response for ListPendingObjectStreams request.
type ListPendingObjectStreamsResponse struct {
	Items []RawObjectListItem
	More  bool
}

func newListPendingObjectStreamsResponse(response *pb.ObjectListPendingStreamsResponse) ListPendingObjectStreamsResponse {
	objects := make([]RawObjectListItem, len(response.Items))
	for i, object := range response.Items {

		objects[i] = RawObjectListItem{
			EncryptedObjectKey:     object.EncryptedObjectKey,
			Version:                object.Version,
			Status:                 int32(object.Status),
			StatusAt:               object.StatusAt,
			CreatedAt:              object.CreatedAt,
			ExpiresAt:              object.ExpiresAt,
			PlainSize:              object.PlainSize,
			EncryptedMetadataNonce: object.EncryptedMetadataNonce,
			EncryptedMetadata:      object.EncryptedMetadata,

			IsPrefix: false,
		}

		if object.StreamId != nil {
			objects[i].StreamID = *object.StreamId
		}
	}

	return ListPendingObjectStreamsResponse{
		Items: objects,
		More:  response.More,
	}
}

// ListPendingObjectStreams lists pending objects with the specified object key in the specified bucket.
func (client *Client) ListPendingObjectStreams(ctx context.Context, params ListPendingObjectStreamsParams) (_ ListPendingObjectStreamsResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	var response *pb.ObjectListPendingStreamsResponse
	err = WithRetry(ctx, func(ctx context.Context) error {
		response, err = client.client.ListPendingObjectStreams(ctx, params.toRequest(client.header()))
		return err
	})
	if err != nil {
		return ListPendingObjectStreamsResponse{}, Error.Wrap(err)
	}

	return newListPendingObjectStreamsResponse(response), nil
}

// SegmentListItem represents listed segment.
type SegmentListItem struct {
	Position          SegmentPosition
	PlainSize         int64
	PlainOffset       int64
	CreatedAt         time.Time
	EncryptedETag     []byte
	EncryptedKeyNonce storj.Nonce
	EncryptedKey      []byte
}

// ListSegmentsParams parameters for ListSegments method.
type ListSegmentsParams struct {
	StreamID []byte
	Cursor   SegmentPosition
	Limit    int32
	Range    StreamRange
}

func (params *ListSegmentsParams) toRequest(header *pb.RequestHeader) *pb.SegmentListRequest {
	return &pb.SegmentListRequest{
		Header:   header,
		StreamId: params.StreamID,
		CursorPosition: &pb.SegmentPosition{
			PartNumber: params.Cursor.PartNumber,
			Index:      params.Cursor.Index,
		},
		Limit: params.Limit,
		Range: params.Range.toProto(),
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

// ListSegmentsResponse response for ListSegments request.
type ListSegmentsResponse struct {
	Items                []SegmentListItem
	More                 bool
	EncryptionParameters storj.EncryptionParameters
}

func newListSegmentsResponse(response *pb.SegmentListResponse) ListSegmentsResponse {
	segments := make([]SegmentListItem, len(response.Items))
	for i, segment := range response.Items {
		segments[i] = SegmentListItem{
			Position: SegmentPosition{
				PartNumber: segment.Position.PartNumber,
				Index:      segment.Position.Index,
			},
			PlainSize:         segment.PlainSize,
			PlainOffset:       segment.PlainOffset,
			CreatedAt:         segment.CreatedAt,
			EncryptedETag:     segment.EncryptedETag,
			EncryptedKeyNonce: segment.EncryptedKeyNonce,
			EncryptedKey:      segment.EncryptedKey,
		}
	}

	ep := storj.EncryptionParameters{}
	if response.EncryptionParameters != nil {
		ep = storj.EncryptionParameters{
			CipherSuite: storj.CipherSuite(response.EncryptionParameters.CipherSuite),
			BlockSize:   int32(response.EncryptionParameters.BlockSize),
		}
	}

	return ListSegmentsResponse{
		Items:                segments,
		More:                 response.More,
		EncryptionParameters: ep,
	}
}

// ListSegments lists segments according to specific parameters.
func (client *Client) ListSegments(ctx context.Context, params ListSegmentsParams) (_ ListSegmentsResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	var response *pb.SegmentListResponse
	err = WithRetry(ctx, func(ctx context.Context) error {
		response, err = client.client.ListSegments(ctx, params.toRequest(client.header()))
		return err
	})
	if err != nil {
		return ListSegmentsResponse{}, Error.Wrap(err)
	}

	return newListSegmentsResponse(response), nil
}

// BeginSegmentParams parameters for BeginSegment method.
type BeginSegmentParams struct {
	StreamID      storj.StreamID
	Position      SegmentPosition
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
	SegmentID          storj.SegmentID
	Limits             []*pb.AddressedOrderLimit
	PiecePrivateKey    storj.PiecePrivateKey
	RedundancyStrategy eestream.RedundancyStrategy
}

func newBeginSegmentResponse(response *pb.SegmentBeginResponse) (BeginSegmentResponse, error) {
	var rs eestream.RedundancyStrategy
	var err error
	if response.RedundancyScheme != nil {
		rs, err = eestream.NewRedundancyStrategyFromProto(response.RedundancyScheme)
		if err != nil {
			return BeginSegmentResponse{}, err
		}
	}
	return BeginSegmentResponse{
		SegmentID:          response.SegmentId,
		Limits:             response.AddressedLimits,
		PiecePrivateKey:    response.PrivateKey,
		RedundancyStrategy: rs,
	}, nil
}

// BeginSegment begins a segment upload.
func (client *Client) BeginSegment(ctx context.Context, params BeginSegmentParams) (_ BeginSegmentResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	var response *pb.SegmentBeginResponse
	err = WithRetry(ctx, func(ctx context.Context) error {
		response, err = client.client.BeginSegment(ctx, params.toRequest(client.header()))
		return err
	})
	if err != nil {
		return BeginSegmentResponse{}, Error.Wrap(err)
	}

	return newBeginSegmentResponse(response)
}

// RetryBeginSegmentPiecesParams parameters for RetryBeginSegmentPieces method.
type RetryBeginSegmentPiecesParams struct {
	SegmentID         storj.SegmentID
	RetryPieceNumbers []int
}

func (params *RetryBeginSegmentPiecesParams) toRequest(header *pb.RequestHeader) *pb.RetryBeginSegmentPiecesRequest {
	retryPieceNumbers := make([]int32, len(params.RetryPieceNumbers))
	for i, pieceNumber := range params.RetryPieceNumbers {
		retryPieceNumbers[i] = int32(pieceNumber)
	}
	return &pb.RetryBeginSegmentPiecesRequest{
		Header:            header,
		SegmentId:         params.SegmentID,
		RetryPieceNumbers: retryPieceNumbers,
	}
}

// RetryBeginSegmentPiecesResponse response for RetryBeginSegmentPieces request.
type RetryBeginSegmentPiecesResponse struct {
	SegmentID storj.SegmentID
	Limits    []*pb.AddressedOrderLimit
}

func newRetryBeginSegmentPiecesResponse(response *pb.RetryBeginSegmentPiecesResponse) (RetryBeginSegmentPiecesResponse, error) {
	return RetryBeginSegmentPiecesResponse{
		SegmentID: response.SegmentId,
		Limits:    response.AddressedLimits,
	}, nil
}

// RetryBeginSegmentPieces exchanges piece orders.
func (client *Client) RetryBeginSegmentPieces(ctx context.Context, params RetryBeginSegmentPiecesParams) (_ RetryBeginSegmentPiecesResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	var response *pb.RetryBeginSegmentPiecesResponse
	err = WithRetry(ctx, func(ctx context.Context) error {
		response, err = client.client.RetryBeginSegmentPieces(ctx, params.toRequest(client.header()))
		return err
	})
	if err != nil {
		return RetryBeginSegmentPiecesResponse{}, Error.Wrap(err)
	}

	return newRetryBeginSegmentPiecesResponse(response)
}

// CommitSegmentParams parameters for CommitSegment method.
type CommitSegmentParams struct {
	SegmentID         storj.SegmentID
	Encryption        SegmentEncryption
	SizeEncryptedData int64
	PlainSize         int64
	EncryptedTag      []byte

	UploadResult []*pb.SegmentPieceUploadResult
}

func (params *CommitSegmentParams) toRequest(header *pb.RequestHeader) *pb.SegmentCommitRequest {
	return &pb.SegmentCommitRequest{
		Header:    header,
		SegmentId: params.SegmentID,

		EncryptedKeyNonce: params.Encryption.EncryptedKeyNonce,
		EncryptedKey:      params.Encryption.EncryptedKey,
		SizeEncryptedData: params.SizeEncryptedData,
		PlainSize:         params.PlainSize,
		EncryptedETag:     params.EncryptedTag,
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

	err = WithRetry(ctx, func(ctx context.Context) error {
		_, err = client.client.CommitSegment(ctx, params.toRequest(client.header()))
		return err
	})

	return Error.Wrap(err)
}

// MakeInlineSegmentParams parameters for MakeInlineSegment method.
type MakeInlineSegmentParams struct {
	StreamID            storj.StreamID
	Position            SegmentPosition
	Encryption          SegmentEncryption
	EncryptedInlineData []byte
	PlainSize           int64
	EncryptedTag        []byte
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
		PlainSize:           params.PlainSize,
		EncryptedETag:       params.EncryptedTag,
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

	err = WithRetry(ctx, func(ctx context.Context) error {
		_, err = client.client.MakeInlineSegment(ctx, params.toRequest(client.header()))
		return err
	})

	return Error.Wrap(err)
}

// DownloadObjectParams parameters for DownloadSegment method.
type DownloadObjectParams struct {
	Bucket             []byte
	EncryptedObjectKey []byte

	Range StreamRange
}

// StreamRange contains range specification.
type StreamRange struct {
	Mode   StreamRangeMode
	Start  int64
	Limit  int64
	Suffix int64
}

// StreamRangeMode contains different modes for range.
type StreamRangeMode byte

const (
	// StreamRangeAll selects all.
	StreamRangeAll StreamRangeMode = iota
	// StreamRangeStart selects starting from range.Start.
	StreamRangeStart
	// StreamRangeStartLimit selects starting from range.Start to range.End (inclusive).
	StreamRangeStartLimit
	// StreamRangeSuffix selects last range.Suffix bytes.
	StreamRangeSuffix
)

func (streamRange StreamRange) toProto() *pb.Range {
	switch streamRange.Mode {
	case StreamRangeAll:
	case StreamRangeStart:
		return &pb.Range{
			Range: &pb.Range_Start{
				Start: &pb.RangeStart{
					PlainStart: streamRange.Start,
				},
			},
		}
	case StreamRangeStartLimit:
		return &pb.Range{
			Range: &pb.Range_StartLimit{
				StartLimit: &pb.RangeStartLimit{
					PlainStart: streamRange.Start,
					PlainLimit: streamRange.Limit,
				},
			},
		}
	case StreamRangeSuffix:
		return &pb.Range{
			Range: &pb.Range_Suffix{
				Suffix: &pb.RangeSuffix{
					PlainSuffix: streamRange.Suffix,
				},
			},
		}
	}
	return nil
}

// Normalize converts the range to a StreamRangeStartLimit or StreamRangeAll.
func (streamRange StreamRange) Normalize(plainSize int64) StreamRange {
	switch streamRange.Mode {
	case StreamRangeAll:
		streamRange.Start = 0
		streamRange.Limit = plainSize
	case StreamRangeStart:
		streamRange.Mode = StreamRangeStartLimit
		streamRange.Limit = plainSize
	case StreamRangeStartLimit:
	case StreamRangeSuffix:
		streamRange.Mode = StreamRangeStartLimit
		streamRange.Start = plainSize - streamRange.Suffix
		streamRange.Limit = plainSize
	}

	if streamRange.Start < 0 {
		streamRange.Start = 0
	}
	if streamRange.Limit > plainSize {
		streamRange.Limit = plainSize
	}
	streamRange.Suffix = 0

	return streamRange
}

func (params *DownloadObjectParams) toRequest(header *pb.RequestHeader) *pb.ObjectDownloadRequest {
	return &pb.ObjectDownloadRequest{
		Header:             header,
		Bucket:             params.Bucket,
		EncryptedObjectKey: params.EncryptedObjectKey,
		Range:              params.Range.toProto(),
	}
}

// BatchItem returns single item for batch request.
func (params *DownloadObjectParams) BatchItem() *pb.BatchRequestItem {
	return &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_ObjectDownload{
			ObjectDownload: params.toRequest(nil),
		},
	}
}

// DownloadObjectResponse response for DownloadSegment request.
type DownloadObjectResponse struct {
	Object             RawObjectItem
	DownloadedSegments []DownloadSegmentWithRSResponse
	ListSegments       ListSegmentsResponse
}

func newDownloadObjectResponse(response *pb.ObjectDownloadResponse) DownloadObjectResponse {
	downloadedSegments := make([]DownloadSegmentWithRSResponse, 0, len(response.SegmentDownload))
	for _, segmentDownload := range response.SegmentDownload {
		downloadedSegments = append(downloadedSegments, newDownloadSegmentResponseWithRS(segmentDownload))
	}
	return DownloadObjectResponse{
		Object:             newObjectInfo(response.Object),
		DownloadedSegments: downloadedSegments,
		ListSegments:       newListSegmentsResponse(response.SegmentList),
	}
}

// DownloadObject gets object information, lists segments and downloads the first segment.
func (client *Client) DownloadObject(ctx context.Context, params DownloadObjectParams) (_ DownloadObjectResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	var response *pb.ObjectDownloadResponse
	err = WithRetry(ctx, func(ctx context.Context) error {
		response, err = client.client.DownloadObject(ctx, params.toRequest(client.header()))
		return err
	})
	if err != nil {
		if errs2.IsRPC(err, rpcstatus.NotFound) {
			return DownloadObjectResponse{}, ErrObjectNotFound.Wrap(err)
		}
		return DownloadObjectResponse{}, Error.Wrap(err)
	}

	return newDownloadObjectResponse(response), nil
}

// DownloadSegmentParams parameters for DownloadSegment method.
type DownloadSegmentParams struct {
	StreamID storj.StreamID
	Position SegmentPosition
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
	Info SegmentDownloadResponseInfo

	Limits []*pb.AddressedOrderLimit
}

func newDownloadSegmentResponse(response *pb.SegmentDownloadResponse) DownloadSegmentResponse {
	info := SegmentDownloadResponseInfo{
		SegmentID:           response.SegmentId,
		EncryptedSize:       response.SegmentSize,
		EncryptedInlineData: response.EncryptedInlineData,
		PiecePrivateKey:     response.PrivateKey,
		SegmentEncryption: SegmentEncryption{
			EncryptedKeyNonce: response.EncryptedKeyNonce,
			EncryptedKey:      response.EncryptedKey,
		},
	}
	if response.Next != nil {
		info.Next = SegmentPosition{
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

// DownloadSegmentWithRSResponse contains information for downloading remote segment or data from an inline segment.
type DownloadSegmentWithRSResponse struct {
	Info   SegmentDownloadInfo
	Limits []*pb.AddressedOrderLimit
}

// SegmentDownloadInfo represents information necessary for downloading segment (inline and remote).
type SegmentDownloadInfo struct {
	SegmentID           storj.SegmentID
	PlainOffset         int64
	PlainSize           int64
	EncryptedSize       int64
	EncryptedInlineData []byte
	PiecePrivateKey     storj.PiecePrivateKey
	SegmentEncryption   SegmentEncryption
	RedundancyScheme    storj.RedundancyScheme
	Position            *SegmentPosition
}

func newDownloadSegmentResponseWithRS(response *pb.SegmentDownloadResponse) DownloadSegmentWithRSResponse {
	info := SegmentDownloadInfo{
		SegmentID:           response.SegmentId,
		PlainOffset:         response.PlainOffset,
		PlainSize:           response.PlainSize,
		EncryptedSize:       response.SegmentSize,
		EncryptedInlineData: response.EncryptedInlineData,
		PiecePrivateKey:     response.PrivateKey,
		SegmentEncryption: SegmentEncryption{
			EncryptedKeyNonce: response.EncryptedKeyNonce,
			EncryptedKey:      response.EncryptedKey,
		},
	}

	if response.Position != nil {
		info.Position = &SegmentPosition{
			PartNumber: response.Position.PartNumber,
			Index:      response.Position.Index,
		}
	}

	if response.RedundancyScheme != nil {
		info.RedundancyScheme = storj.RedundancyScheme{
			Algorithm:      storj.RedundancyAlgorithm(response.RedundancyScheme.Type),
			ShareSize:      response.RedundancyScheme.ErasureShareSize,
			RequiredShares: int16(response.RedundancyScheme.MinReq),
			RepairShares:   int16(response.RedundancyScheme.RepairThreshold),
			OptimalShares:  int16(response.RedundancyScheme.SuccessThreshold),
			TotalShares:    int16(response.RedundancyScheme.Total),
		}
	}

	for i := range response.AddressedLimits {
		if response.AddressedLimits[i].Limit == nil {
			response.AddressedLimits[i] = nil
		}
	}
	return DownloadSegmentWithRSResponse{
		Info:   info,
		Limits: response.AddressedLimits,
	}
}

// TODO replace DownloadSegment with DownloadSegmentWithRS in batch

// DownloadSegmentWithRS gets information for downloading remote segment or data from an inline segment.
func (client *Client) DownloadSegmentWithRS(ctx context.Context, params DownloadSegmentParams) (_ DownloadSegmentWithRSResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	var response *pb.SegmentDownloadResponse
	err = WithRetry(ctx, func(ctx context.Context) error {
		response, err = client.client.DownloadSegment(ctx, params.toRequest(client.header()))
		return err
	})
	if err != nil {
		if errs2.IsRPC(err, rpcstatus.NotFound) {
			return DownloadSegmentWithRSResponse{}, ErrObjectNotFound.Wrap(err)
		}
		return DownloadSegmentWithRSResponse{}, Error.Wrap(err)
	}

	return newDownloadSegmentResponseWithRS(response), nil
}

// RevokeAPIKey revokes the APIKey provided in the params.
func (client *Client) RevokeAPIKey(ctx context.Context, params RevokeAPIKeyParams) (err error) {
	defer mon.Task()(&ctx)(&err)
	err = WithRetry(ctx, func(ctx context.Context) error {
		_, err = client.client.RevokeAPIKey(ctx, params.toRequest(client.header()))
		return err
	})
	return Error.Wrap(err)
}

// RevokeAPIKeyParams contain params for a RevokeAPIKey request.
type RevokeAPIKeyParams struct {
	APIKey []byte
}

func (r RevokeAPIKeyParams) toRequest(header *pb.RequestHeader) *pb.RevokeAPIKeyRequest {
	return &pb.RevokeAPIKeyRequest{
		Header: header,
		ApiKey: r.APIKey,
	}
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
		resp[i] = MakeBatchResponse(batchItems[i], response)
	}

	return resp, nil
}

// SetRawAPIKey sets the client's raw API key. Mainly used for testing.
func (client *Client) SetRawAPIKey(key []byte) {
	client.apiKeyRaw = key
}
