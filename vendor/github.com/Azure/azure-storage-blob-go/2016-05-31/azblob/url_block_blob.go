package azblob

import (
	"context"
	"io"
	"net/url"
	"time"

	"github.com/Azure/azure-pipeline-go/pipeline"
)

const (
	// BlockBlobMaxPutBlobBytes indicates the maximum number of bytes that can be sent in a call to PutBlob.
	BlockBlobMaxPutBlobBytes = 256 * 1024 * 1024 // 256MB

	// BlockBlobMaxPutBlockBytes indicates the maximum number of bytes that can be sent in a call to PutBlock.
	BlockBlobMaxPutBlockBytes = 100 * 1024 * 1024 // 100MB

	// BlockBlobMaxBlocks indicates the maximum number of blocks allowed in a block blob.
	BlockBlobMaxBlocks = 50000
)

// BlockBlobURL defines a set of operations applicable to block blobs.
type BlockBlobURL struct {
	BlobURL
	bbClient blockBlobsClient
}

// NewBlockBlobURL creates a BlockBlobURL object using the specified URL and request policy pipeline.
func NewBlockBlobURL(url url.URL, p pipeline.Pipeline) BlockBlobURL {
	if p == nil {
		panic("p can't be nil")
	}
	blobClient := newBlobsClient(url, p)
	bbClient := newBlockBlobsClient(url, p)
	return BlockBlobURL{BlobURL: BlobURL{blobClient: blobClient}, bbClient: bbClient}
}

// WithPipeline creates a new BlockBlobURL object identical to the source but with the specific request policy pipeline.
func (bb BlockBlobURL) WithPipeline(p pipeline.Pipeline) BlockBlobURL {
	return NewBlockBlobURL(bb.blobClient.URL(), p)
}

// WithSnapshot creates a new BlockBlobURL object identical to the source but with the specified snapshot timestamp.
// Pass time.Time{} to remove the snapshot returning a URL to the base blob.
func (bb BlockBlobURL) WithSnapshot(snapshot time.Time) BlockBlobURL {
	p := NewBlobURLParts(bb.URL())
	p.Snapshot = snapshot
	return NewBlockBlobURL(p.URL(), bb.blobClient.Pipeline())
}

// PutBlob creates a new block blob, or updates the content of an existing block blob.
// Updating an existing block blob overwrites any existing metadata on the blob. Partial updates are not
// supported with PutBlob; the content of the existing blob is overwritten with the new content. To
// perform a partial update of a block blob's, use PutBlock and PutBlockList. Note that the http client
// closes the body stream after the request is sent to the service.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/put-blob.
func (bb BlockBlobURL) PutBlob(ctx context.Context, body io.ReadSeeker, h BlobHTTPHeaders, metadata Metadata, ac BlobAccessConditions) (*BlobsPutResponse, error) {
	ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag := ac.HTTPAccessConditions.pointers()
	return bb.blobClient.Put(ctx, BlobBlockBlob, body, nil, nil,
		&h.ContentType, &h.ContentEncoding, &h.ContentLanguage, h.contentMD5Pointer(), &h.CacheControl,
		metadata, ac.LeaseAccessConditions.pointers(),
		&h.ContentDisposition, ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag, nil, nil, nil)
}

// GetBlockList returns the list of blocks that have been uploaded as part of a block blob using the specified block list filter.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/get-block-list.
func (bb BlockBlobURL) GetBlockList(ctx context.Context, listType BlockListType, ac LeaseAccessConditions) (*BlockList, error) {
	return bb.bbClient.GetBlockList(ctx, listType, nil, nil, ac.pointers(), nil)
}

// PutBlock uploads the specified block to the block blob's "staging area" to be later committed by a call to PutBlockList.
// Note that the http client closes the body stream after the request is sent to the service.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/put-block.
func (bb BlockBlobURL) PutBlock(ctx context.Context, base64BlockID string, body io.ReadSeeker, ac LeaseAccessConditions) (*BlockBlobsPutBlockResponse, error) {
	return bb.bbClient.PutBlock(ctx, base64BlockID, body, nil, ac.pointers(), nil)
}

// PutBlockList writes a blob by specifying the list of block IDs that make up the blob.
// In order to be written as part of a blob, a block must have been successfully written
// to the server in a prior PutBlock operation. You can call PutBlockList to update a blob
// by uploading only those blocks that have changed, then committing the new and existing
// blocks together. Any blocks not specified in the block list and permanently deleted.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/put-block-list.
func (bb BlockBlobURL) PutBlockList(ctx context.Context, base64BlockIDs []string, h BlobHTTPHeaders,
	metadata Metadata, ac BlobAccessConditions) (*BlockBlobsPutBlockListResponse, error) {
	ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag := ac.HTTPAccessConditions.pointers()
	return bb.bbClient.PutBlockList(ctx, BlockLookupList{Latest: base64BlockIDs}, nil,
		&h.CacheControl, &h.ContentType, &h.ContentEncoding, &h.ContentLanguage, h.contentMD5Pointer(),
		metadata, ac.LeaseAccessConditions.pointers(), &h.ContentDisposition,
		ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag, nil)
}
