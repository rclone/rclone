package azblob

import (
	"context"
	"io"
	"net/url"

	"encoding/base64"
	"encoding/binary"

	"github.com/Azure/azure-pipeline-go/pipeline"
)

const (
	// BlockBlobMaxPutBlobBytes indicates the maximum number of bytes that can be sent in a call to Upload.
	BlockBlobMaxUploadBlobBytes = 256 * 1024 * 1024 // 256MB

	// BlockBlobMaxStageBlockBytes indicates the maximum number of bytes that can be sent in a call to StageBlock.
	BlockBlobMaxStageBlockBytes = 100 * 1024 * 1024 // 100MB

	// BlockBlobMaxBlocks indicates the maximum number of blocks allowed in a block blob.
	BlockBlobMaxBlocks = 50000
)

// BlockBlobURL defines a set of operations applicable to block blobs.
type BlockBlobURL struct {
	BlobURL
	bbClient blockBlobClient
}

// NewBlockBlobURL creates a BlockBlobURL object using the specified URL and request policy pipeline.
func NewBlockBlobURL(url url.URL, p pipeline.Pipeline) BlockBlobURL {
	if p == nil {
		panic("p can't be nil")
	}
	blobClient := newBlobClient(url, p)
	bbClient := newBlockBlobClient(url, p)
	return BlockBlobURL{BlobURL: BlobURL{blobClient: blobClient}, bbClient: bbClient}
}

// WithPipeline creates a new BlockBlobURL object identical to the source but with the specific request policy pipeline.
func (bb BlockBlobURL) WithPipeline(p pipeline.Pipeline) BlockBlobURL {
	return NewBlockBlobURL(bb.blobClient.URL(), p)
}

// WithSnapshot creates a new BlockBlobURL object identical to the source but with the specified snapshot timestamp.
// Pass "" to remove the snapshot returning a URL to the base blob.
func (bb BlockBlobURL) WithSnapshot(snapshot string) BlockBlobURL {
	p := NewBlobURLParts(bb.URL())
	p.Snapshot = snapshot
	return NewBlockBlobURL(p.URL(), bb.blobClient.Pipeline())
}

// Upload creates a new block blob or overwrites an existing block blob.
// Updating an existing block blob overwrites any existing metadata on the blob. Partial updates are not
// supported with Upload; the content of the existing blob is overwritten with the new content. To
// perform a partial update of a block blob, use StageBlock and CommitBlockList.
// This method panics if the stream is not at position 0.
// Note that the http client closes the body stream after the request is sent to the service.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/put-blob.
func (bb BlockBlobURL) Upload(ctx context.Context, body io.ReadSeeker, h BlobHTTPHeaders, metadata Metadata, ac BlobAccessConditions) (*BlockBlobUploadResponse, error) {
	ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag := ac.HTTPAccessConditions.pointers()
	return bb.bbClient.Upload(ctx, body, validateSeekableStreamAt0AndGetCount(body), nil,
		&h.ContentType, &h.ContentEncoding, &h.ContentLanguage, h.ContentMD5,
		&h.CacheControl, metadata, ac.LeaseAccessConditions.pointers(),
		&h.ContentDisposition, ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag,
		nil)
}

// StageBlock uploads the specified block to the block blob's "staging area" to be later committed by a call to CommitBlockList.
// Note that the http client closes the body stream after the request is sent to the service.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/put-block.
func (bb BlockBlobURL) StageBlock(ctx context.Context, base64BlockID string, body io.ReadSeeker, ac LeaseAccessConditions) (*BlockBlobStageBlockResponse, error) {
	return bb.bbClient.StageBlock(ctx, base64BlockID, validateSeekableStreamAt0AndGetCount(body), body, nil, ac.pointers(), nil)
}

// StageBlockFromURL copies the specified block from a source URL to the block blob's "staging area" to be later committed by a call to CommitBlockList.
// If count is CountToEnd (0), then data is read from specified offset to the end.
// For more information, see https://docs.microsoft.com/en-us/rest/api/storageservices/put-block-from-url.
func (bb BlockBlobURL) StageBlockFromURL(ctx context.Context, base64BlockID string, sourceURL url.URL, offset int64, count int64, ac LeaseAccessConditions) (*BlockBlobStageBlockFromURLResponse, error) {
	sourceURLStr := sourceURL.String()
	return bb.bbClient.StageBlockFromURL(ctx, base64BlockID, 0, &sourceURLStr, httpRange{offset: offset, count: count}.pointers(), nil, nil, ac.pointers(), nil)
}

// CommitBlockList writes a blob by specifying the list of block IDs that make up the blob.
// In order to be written as part of a blob, a block must have been successfully written
// to the server in a prior PutBlock operation. You can call PutBlockList to update a blob
// by uploading only those blocks that have changed, then committing the new and existing
// blocks together. Any blocks not specified in the block list and permanently deleted.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/put-block-list.
func (bb BlockBlobURL) CommitBlockList(ctx context.Context, base64BlockIDs []string, h BlobHTTPHeaders,
	metadata Metadata, ac BlobAccessConditions) (*BlockBlobCommitBlockListResponse, error) {
	ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag := ac.HTTPAccessConditions.pointers()
	return bb.bbClient.CommitBlockList(ctx, BlockLookupList{Latest: base64BlockIDs}, nil,
		&h.CacheControl, &h.ContentType, &h.ContentEncoding, &h.ContentLanguage, h.ContentMD5,
		metadata, ac.LeaseAccessConditions.pointers(), &h.ContentDisposition,
		ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag, nil)
}

// GetBlockList returns the list of blocks that have been uploaded as part of a block blob using the specified block list filter.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/get-block-list.
func (bb BlockBlobURL) GetBlockList(ctx context.Context, listType BlockListType, ac LeaseAccessConditions) (*BlockList, error) {
	return bb.bbClient.GetBlockList(ctx, listType, nil, nil, ac.pointers(), nil)
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////

type BlockID [64]byte

func (blockID BlockID) ToBase64() string {
	return base64.StdEncoding.EncodeToString(blockID[:])
}

func (blockID *BlockID) FromBase64(s string) error {
	*blockID = BlockID{} // Zero out the block ID
	_, err := base64.StdEncoding.Decode(blockID[:], ([]byte)(s))
	return err
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////

type uuidBlockID BlockID

func (ubi uuidBlockID) UUID() uuid {
	u := uuid{}
	copy(u[:], ubi[:len(u)])
	return u
}

func (ubi uuidBlockID) Number() uint32 {
	return binary.BigEndian.Uint32(ubi[len(uuid{}):])
}

func newUuidBlockID(u uuid) uuidBlockID {
	ubi := uuidBlockID{}     // Create a new uuidBlockID
	copy(ubi[:len(u)], u[:]) // Copy the specified UUID into it
	// Block number defaults to 0
	return ubi
}

func (ubi *uuidBlockID) SetUUID(u uuid) *uuidBlockID {
	copy(ubi[:len(u)], u[:])
	return ubi
}

func (ubi uuidBlockID) WithBlockNumber(blockNumber uint32) uuidBlockID {
	binary.BigEndian.PutUint32(ubi[len(uuid{}):], blockNumber) // Put block number after UUID
	return ubi                                                 // Return the passed-in copy
}

func (ubi uuidBlockID) ToBase64() string {
	return BlockID(ubi).ToBase64()
}

func (ubi *uuidBlockID) FromBase64(s string) error {
	return (*BlockID)(ubi).FromBase64(s)
}
