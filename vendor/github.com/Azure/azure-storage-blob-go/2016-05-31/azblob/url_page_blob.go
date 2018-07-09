package azblob

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"time"

	"github.com/Azure/azure-pipeline-go/pipeline"
)

const (
	// PageBlobPageBytes indicates the number of bytes in a page (512).
	PageBlobPageBytes = 512

	// PageBlobMaxPutPagesBytes indicates the maximum number of bytes that can be sent in a call to PutPage.
	PageBlobMaxPutPagesBytes = 4 * 1024 * 1024 // 4MB
)

// PageBlobURL defines a set of operations applicable to page blobs.
type PageBlobURL struct {
	BlobURL
	pbClient pageBlobsClient
}

// NewPageBlobURL creates a PageBlobURL object using the specified URL and request policy pipeline.
func NewPageBlobURL(url url.URL, p pipeline.Pipeline) PageBlobURL {
	if p == nil {
		panic("p can't be nil")
	}
	blobClient := newBlobsClient(url, p)
	pbClient := newPageBlobsClient(url, p)
	return PageBlobURL{BlobURL: BlobURL{blobClient: blobClient}, pbClient: pbClient}
}

// WithPipeline creates a new PageBlobURL object identical to the source but with the specific request policy pipeline.
func (pb PageBlobURL) WithPipeline(p pipeline.Pipeline) PageBlobURL {
	return NewPageBlobURL(pb.blobClient.URL(), p)
}

// WithSnapshot creates a new PageBlobURL object identical to the source but with the specified snapshot timestamp.
// Pass time.Time{} to remove the snapshot returning a URL to the base blob.
func (pb PageBlobURL) WithSnapshot(snapshot time.Time) PageBlobURL {
	p := NewBlobURLParts(pb.URL())
	p.Snapshot = snapshot
	return NewPageBlobURL(p.URL(), pb.blobClient.Pipeline())
}

// Create creates a page blob of the specified length. Call PutPage to upload data data to a page blob.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/put-blob.
func (pb PageBlobURL) Create(ctx context.Context, size int64, sequenceNumber int64, h BlobHTTPHeaders, metadata Metadata, ac BlobAccessConditions) (*BlobsPutResponse, error) {
	if sequenceNumber < 0 {
		panic("sequenceNumber must be greater than or equal to 0")
	}
	ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag := ac.HTTPAccessConditions.pointers()
	return pb.blobClient.Put(ctx, BlobPageBlob, nil, nil, nil,
		&h.ContentType, &h.ContentEncoding, &h.ContentLanguage, h.contentMD5Pointer(), &h.CacheControl,
		metadata, ac.LeaseAccessConditions.pointers(),
		&h.ContentDisposition, ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag, &size, &sequenceNumber, nil)
}

// PutPages writes 1 or more pages to the page blob. The start and end offsets must be a multiple of 512.
// Note that the http client closes the body stream after the request is sent to the service.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/put-page.
func (pb PageBlobURL) PutPages(ctx context.Context, pr PageRange, body io.ReadSeeker, ac BlobAccessConditions) (*PageBlobsPutPageResponse, error) {
	ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag := ac.HTTPAccessConditions.pointers()
	ifSequenceNumberLessThanOrEqual, ifSequenceNumberLessThan, ifSequenceNumberEqual := ac.PageBlobAccessConditions.pointers()
	return pb.pbClient.PutPage(ctx, PageWriteUpdate, body, nil, pr.pointers(), ac.LeaseAccessConditions.pointers(),
		ifSequenceNumberLessThanOrEqual, ifSequenceNumberLessThan, ifSequenceNumberEqual,
		ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag, nil)
}

// ClearPages frees the specified pages from the page blob.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/put-page.
func (pb PageBlobURL) ClearPages(ctx context.Context, pr PageRange, ac BlobAccessConditions) (*PageBlobsPutPageResponse, error) {
	ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag := ac.HTTPAccessConditions.pointers()
	ifSequenceNumberLessThanOrEqual, ifSequenceNumberLessThan, ifSequenceNumberEqual := ac.PageBlobAccessConditions.pointers()
	return pb.pbClient.PutPage(ctx, PageWriteClear, nil, nil, pr.pointers(), ac.LeaseAccessConditions.pointers(),
		ifSequenceNumberLessThanOrEqual, ifSequenceNumberLessThan,
		ifSequenceNumberEqual, ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag, nil)
}

// GetPageRanges returns the list of valid page ranges for a page blob or snapshot of a page blob.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/get-page-ranges.
func (pb PageBlobURL) GetPageRanges(ctx context.Context, br BlobRange, ac BlobAccessConditions) (*PageList, error) {
	ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag := ac.HTTPAccessConditions.pointers()
	return pb.pbClient.GetPageRanges(ctx, nil, nil, nil, br.pointers(), ac.LeaseAccessConditions.pointers(),
		ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag, nil)
}

// GetPageRangesDiff gets the collection of page ranges that differ between a specified snapshot and this page blob.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/get-page-ranges.
func (pb PageBlobURL) GetPageRangesDiff(ctx context.Context, br BlobRange, prevSnapshot time.Time, ac BlobAccessConditions) (*PageList, error) {
	ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag := ac.HTTPAccessConditions.pointers()
	return pb.pbClient.GetPageRanges(ctx, nil, nil, &prevSnapshot, br.pointers(),
		ac.LeaseAccessConditions.pointers(), ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag, nil)
}

// Resize resizes the page blob to the specified size (which must be a multiple of 512).
// For more information, see https://docs.microsoft.com/rest/api/storageservices/set-blob-properties.
func (pb PageBlobURL) Resize(ctx context.Context, size int64, ac BlobAccessConditions) (*BlobsSetPropertiesResponse, error) {
	if size%PageBlobPageBytes != 0 {
		panic("Size must be a multiple of PageBlobPageBytes (512)")
	}
	ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag := ac.HTTPAccessConditions.pointers()
	return pb.blobClient.SetProperties(ctx, nil, nil, nil, nil, nil, nil, ac.LeaseAccessConditions.pointers(),
		ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag, nil, &size, SequenceNumberActionNone, nil, nil)
}

// SetSequenceNumber sets the page blob's sequence number.
func (pb PageBlobURL) SetSequenceNumber(ctx context.Context, action SequenceNumberActionType, sequenceNumber int64,
	h BlobHTTPHeaders, ac BlobAccessConditions) (*BlobsSetPropertiesResponse, error) {
	if sequenceNumber < 0 {
		panic("sequenceNumber must be greater than or equal to 0")
	}
	sn := &sequenceNumber
	if action == SequenceNumberActionIncrement {
		sn = nil
	}
	ifModifiedSince, ifUnmodifiedSince, ifMatch, ifNoneMatch := ac.HTTPAccessConditions.pointers()
	return pb.blobClient.SetProperties(ctx, nil, &h.CacheControl, &h.ContentType, h.contentMD5Pointer(), &h.ContentEncoding, &h.ContentLanguage,
		ac.LeaseAccessConditions.pointers(),
		ifModifiedSince, ifUnmodifiedSince, ifMatch, ifNoneMatch, &h.ContentDisposition, nil, action, sn, nil)
}

// StartIncrementalCopy begins an operation to start an incremental copy from one page blob's snapshot to this page blob.
// The snapshot is copied such that only the differential changes between the previously copied snapshot are transferred to the destination.
// The copied snapshots are complete copies of the original snapshot and can be read or copied from as usual.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/incremental-copy-blob and
// https://docs.microsoft.com/en-us/azure/virtual-machines/windows/incremental-snapshots.
func (pb PageBlobURL) StartIncrementalCopy(ctx context.Context, source url.URL, snapshot time.Time, ac BlobAccessConditions) (*PageBlobsIncrementalCopyResponse, error) {
	ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag := ac.HTTPAccessConditions.pointers()
	qp := source.Query()
	qp.Set("snapshot", snapshot.Format(snapshotTimeFormat))
	source.RawQuery = qp.Encode()
	return pb.pbClient.IncrementalCopy(ctx, source.String(), nil, nil,
		ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag, nil)
}

func (pr PageRange) pointers() *string {
	if pr.Start < 0 {
		panic("PageRange's Start value must be greater than or equal to 0")
	}
	if pr.End <= 0 {
		panic("PageRange's End value must be greater than 0")
	}
	if pr.Start%512 != 0 {
		panic("PageRange's Start value must be a multiple of 512")
	}
	if pr.End%512 != 511 {
		panic("PageRange's End value must be 1 less than a multiple of 512")
	}
	if pr.End <= pr.Start {
		panic("PageRange's End value must be after the start")
	}
	endOffset := strconv.FormatInt(int64(pr.End), 10)
	asString := fmt.Sprintf("bytes=%v-%s", pr.Start, endOffset)
	return &asString
}

// PageBlobAccessConditions identifies page blob-specific access conditions which you optionally set.
type PageBlobAccessConditions struct {
	// IfSequenceNumberLessThan ensures that the page blob operation succeeds
	// only if the blob's sequence number is less than a value.
	// IfSequenceNumberLessThan=0 means no 'IfSequenceNumberLessThan' header specified.
	// IfSequenceNumberLessThan>0 means 'IfSequenceNumberLessThan' header specified with its value
	// IfSequenceNumberLessThan==-1 means 'IfSequenceNumberLessThan' header specified with a value of 0
	IfSequenceNumberLessThan int32

	// IfSequenceNumberLessThanOrEqual ensures that the page blob operation succeeds
	// only if the blob's sequence number is less than or equal to a value.
	// IfSequenceNumberLessThanOrEqual=0 means no 'IfSequenceNumberLessThanOrEqual' header specified.
	// IfSequenceNumberLessThanOrEqual>0 means 'IfSequenceNumberLessThanOrEqual' header specified with its value
	// IfSequenceNumberLessThanOrEqual=-1 means 'IfSequenceNumberLessThanOrEqual' header specified with a value of 0
	IfSequenceNumberLessThanOrEqual int32

	// IfSequenceNumberEqual ensures that the page blob operation succeeds
	// only if the blob's sequence number is equal to a value.
	// IfSequenceNumberEqual=0 means no 'IfSequenceNumberEqual' header specified.
	// IfSequenceNumberEqual>0 means 'IfSequenceNumberEqual' header specified with its value
	// IfSequenceNumberEqual=-1 means 'IfSequenceNumberEqual' header specified with a value of 0
	IfSequenceNumberEqual int32
}

// pointers is for internal infrastructure. It returns the fields as pointers.
func (ac PageBlobAccessConditions) pointers() (snltoe *int32, snlt *int32, sne *int32) {
	if ac.IfSequenceNumberLessThan < -1 {
		panic("Ifsequencenumberlessthan can't be less than -1")
	}
	if ac.IfSequenceNumberLessThanOrEqual < -1 {
		panic("IfSequenceNumberLessThanOrEqual can't be less than -1")
	}
	if ac.IfSequenceNumberEqual < -1 {
		panic("IfSequenceNumberEqual can't be less than -1")
	}

	var zero int32 // Defaults to 0
	switch ac.IfSequenceNumberLessThan {
	case -1:
		snlt = &zero
	case 0:
		snlt = nil
	default:
		snlt = &ac.IfSequenceNumberLessThan
	}

	switch ac.IfSequenceNumberLessThanOrEqual {
	case -1:
		snltoe = &zero
	case 0:
		snltoe = nil
	default:
		snltoe = &ac.IfSequenceNumberLessThanOrEqual
	}
	switch ac.IfSequenceNumberEqual {
	case -1:
		sne = &zero
	case 0:
		sne = nil
	default:
		sne = &ac.IfSequenceNumberEqual
	}
	return
}
