package azblob

import (
	"context"
	"io"
	"net/url"
	"time"

	"github.com/Azure/azure-pipeline-go/pipeline"
)

const (
	// AppendBlobMaxAppendBlockBytes indicates the maximum number of bytes that can be sent in a call to AppendBlock.
	AppendBlobMaxAppendBlockBytes = 4 * 1024 * 1024 // 4MB

	// AppendBlobMaxBlocks indicates the maximum number of blocks allowed in an append blob.
	AppendBlobMaxBlocks = 50000
)

// AppendBlobURL defines a set of operations applicable to append blobs.
type AppendBlobURL struct {
	BlobURL
	abClient appendBlobsClient
}

// NewAppendBlobURL creates an AppendBlobURL object using the specified URL and request policy pipeline.
func NewAppendBlobURL(url url.URL, p pipeline.Pipeline) AppendBlobURL {
	blobClient := newBlobsClient(url, p)
	abClient := newAppendBlobsClient(url, p)
	return AppendBlobURL{BlobURL: BlobURL{blobClient: blobClient}, abClient: abClient}
}

// WithPipeline creates a new AppendBlobURL object identical to the source but with the specific request policy pipeline.
func (ab AppendBlobURL) WithPipeline(p pipeline.Pipeline) AppendBlobURL {
	return NewAppendBlobURL(ab.blobClient.URL(), p)
}

// WithSnapshot creates a new AppendBlobURL object identical to the source but with the specified snapshot timestamp.
// Pass time.Time{} to remove the snapshot returning a URL to the base blob.
func (ab AppendBlobURL) WithSnapshot(snapshot time.Time) AppendBlobURL {
	p := NewBlobURLParts(ab.URL())
	p.Snapshot = snapshot
	return NewAppendBlobURL(p.URL(), ab.blobClient.Pipeline())
}

// Create creates a 0-length append blob. Call AppendBlock to append data to an append blob.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/put-blob.
func (ab AppendBlobURL) Create(ctx context.Context, h BlobHTTPHeaders, metadata Metadata, ac BlobAccessConditions) (*BlobsPutResponse, error) {
	ifModifiedSince, ifUnmodifiedSince, ifMatch, ifNoneMatch := ac.HTTPAccessConditions.pointers()
	return ab.blobClient.Put(ctx, BlobAppendBlob, nil, nil, nil,
		&h.ContentType, &h.ContentEncoding, &h.ContentLanguage, h.contentMD5Pointer(), &h.CacheControl,
		metadata, ac.LeaseAccessConditions.pointers(),
		&h.ContentDisposition,
		ifModifiedSince, ifUnmodifiedSince, ifMatch, ifNoneMatch, nil, nil, nil)

}

// AppendBlock commits a new block of data to the end of the existing append blob.
// Note that the http client closes the body stream after the request is sent to the service.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/append-block.
func (ab AppendBlobURL) AppendBlock(ctx context.Context, body io.ReadSeeker, ac BlobAccessConditions) (*AppendBlobsAppendBlockResponse, error) {
	ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag := ac.HTTPAccessConditions.pointers()
	ifAppendPositionEqual, ifMaxSizeLessThanOrEqual := ac.AppendBlobAccessConditions.pointers()
	return ab.abClient.AppendBlock(ctx, body, nil, ac.LeaseAccessConditions.pointers(),
		ifMaxSizeLessThanOrEqual, ifAppendPositionEqual,
		ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag, nil)
}

// AppendBlobAccessConditions identifies append blob-specific access conditions which you optionally set.
type AppendBlobAccessConditions struct {
	// IfAppendPositionEqual ensures that the AppendBlock operation succeeds
	// only if the append position is equal to a value.
	// IfAppendPositionEqual=0 means no 'IfAppendPositionEqual' header specified.
	// IfAppendPositionEqual>0 means 'IfAppendPositionEqual' header specified with its value
	// IfAppendPositionEqual==-1 means IfAppendPositionEqual' header specified with a value of 0
	IfAppendPositionEqual int32

	// IfMaxSizeLessThanOrEqual ensures that the AppendBlock operation succeeds
	// only if the append blob's size is less than or equal to a value.
	// IfMaxSizeLessThanOrEqual=0 means no 'IfMaxSizeLessThanOrEqual' header specified.
	// IfMaxSizeLessThanOrEqual>0 means 'IfMaxSizeLessThanOrEqual' header specified with its value
	// IfMaxSizeLessThanOrEqual==-1 means 'IfMaxSizeLessThanOrEqual' header specified with a value of 0
	IfMaxSizeLessThanOrEqual int32
}

// pointers is for internal infrastructure. It returns the fields as pointers.
func (ac AppendBlobAccessConditions) pointers() (iape *int32, imsltoe *int32) {
	if ac.IfAppendPositionEqual < -1 {
		panic("IfAppendPositionEqual can't be less than -1")
	}
	if ac.IfMaxSizeLessThanOrEqual < -1 {
		panic("IfMaxSizeLessThanOrEqual can't be less than -1")
	}
	var zero int32 // defaults to 0
	switch ac.IfAppendPositionEqual {
	case -1:
		iape = &zero
	case 0:
		iape = nil
	default:
		iape = &ac.IfAppendPositionEqual
	}

	switch ac.IfMaxSizeLessThanOrEqual {
	case -1:
		imsltoe = &zero
	case 0:
		imsltoe = nil
	default:
		imsltoe = &ac.IfMaxSizeLessThanOrEqual
	}
	return
}
