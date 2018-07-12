package azblob

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strconv"

	"github.com/Azure/azure-pipeline-go/pipeline"
)

const (
	// PageBlobPageBytes indicates the number of bytes in a page (512).
	PageBlobPageBytes = 512

	// PageBlobMaxPutPagesBytes indicates the maximum number of bytes that can be sent in a call to PutPage.
	PageBlobMaxUploadPagesBytes = 4 * 1024 * 1024 // 4MB
)

// PageBlobURL defines a set of operations applicable to page blobs.
type PageBlobURL struct {
	BlobURL
	pbClient pageBlobClient
}

// NewPageBlobURL creates a PageBlobURL object using the specified URL and request policy pipeline.
func NewPageBlobURL(url url.URL, p pipeline.Pipeline) PageBlobURL {
	if p == nil {
		panic("p can't be nil")
	}
	blobClient := newBlobClient(url, p)
	pbClient := newPageBlobClient(url, p)
	return PageBlobURL{BlobURL: BlobURL{blobClient: blobClient}, pbClient: pbClient}
}

// WithPipeline creates a new PageBlobURL object identical to the source but with the specific request policy pipeline.
func (pb PageBlobURL) WithPipeline(p pipeline.Pipeline) PageBlobURL {
	return NewPageBlobURL(pb.blobClient.URL(), p)
}

// WithSnapshot creates a new PageBlobURL object identical to the source but with the specified snapshot timestamp.
// Pass "" to remove the snapshot returning a URL to the base blob.
func (pb PageBlobURL) WithSnapshot(snapshot string) PageBlobURL {
	p := NewBlobURLParts(pb.URL())
	p.Snapshot = snapshot
	return NewPageBlobURL(p.URL(), pb.blobClient.Pipeline())
}

// CreatePageBlob creates a page blob of the specified length. Call PutPage to upload data data to a page blob.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/put-blob.
func (pb PageBlobURL) Create(ctx context.Context, size int64, sequenceNumber int64, h BlobHTTPHeaders, metadata Metadata, ac BlobAccessConditions) (*PageBlobCreateResponse, error) {
	if sequenceNumber < 0 {
		panic("sequenceNumber must be greater than or equal to 0")
	}
	ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag := ac.HTTPAccessConditions.pointers()
	return pb.pbClient.Create(ctx, 0, nil,
		&h.ContentType, &h.ContentEncoding, &h.ContentLanguage, h.ContentMD5, &h.CacheControl,
		metadata, ac.LeaseAccessConditions.pointers(),
		&h.ContentDisposition, ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag, &size, &sequenceNumber, nil)
}

// UploadPages writes 1 or more pages to the page blob. The start offset and the stream size must be a multiple of 512 bytes.
// This method panics if the stream is not at position 0.
// Note that the http client closes the body stream after the request is sent to the service.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/put-page.
func (pb PageBlobURL) UploadPages(ctx context.Context, offset int64, body io.ReadSeeker, ac BlobAccessConditions) (*PageBlobUploadPagesResponse, error) {
	count := validateSeekableStreamAt0AndGetCount(body)
	ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag := ac.HTTPAccessConditions.pointers()
	ifSequenceNumberLessThanOrEqual, ifSequenceNumberLessThan, ifSequenceNumberEqual := ac.PageBlobAccessConditions.pointers()
	return pb.pbClient.UploadPages(ctx, body, count, nil,
		PageRange{Start: offset, End: offset + count - 1}.pointers(),
		ac.LeaseAccessConditions.pointers(),
		ifSequenceNumberLessThanOrEqual, ifSequenceNumberLessThan, ifSequenceNumberEqual,
		ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag, nil)
}

// ClearPages frees the specified pages from the page blob.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/put-page.
func (pb PageBlobURL) ClearPages(ctx context.Context, offset int64, count int64, ac BlobAccessConditions) (*PageBlobClearPagesResponse, error) {
	ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag := ac.HTTPAccessConditions.pointers()
	ifSequenceNumberLessThanOrEqual, ifSequenceNumberLessThan, ifSequenceNumberEqual := ac.PageBlobAccessConditions.pointers()
	return pb.pbClient.ClearPages(ctx, 0, nil,
		PageRange{Start: offset, End: offset + count - 1}.pointers(),
		ac.LeaseAccessConditions.pointers(),
		ifSequenceNumberLessThanOrEqual, ifSequenceNumberLessThan,
		ifSequenceNumberEqual, ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag, nil)
}

// GetPageRanges returns the list of valid page ranges for a page blob or snapshot of a page blob.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/get-page-ranges.
func (pb PageBlobURL) GetPageRanges(ctx context.Context, offset int64, count int64, ac BlobAccessConditions) (*PageList, error) {
	ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag := ac.HTTPAccessConditions.pointers()
	return pb.pbClient.GetPageRanges(ctx, nil, nil,
		httpRange{offset: offset, count: count}.pointers(),
		ac.LeaseAccessConditions.pointers(),
		ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag, nil)
}

// GetPageRangesDiff gets the collection of page ranges that differ between a specified snapshot and this page blob.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/get-page-ranges.
func (pb PageBlobURL) GetPageRangesDiff(ctx context.Context, offset int64, count int64, prevSnapshot string, ac BlobAccessConditions) (*PageList, error) {
	ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag := ac.HTTPAccessConditions.pointers()
	return pb.pbClient.GetPageRangesDiff(ctx, nil, nil, &prevSnapshot,
		httpRange{offset: offset, count: count}.pointers(),
		ac.LeaseAccessConditions.pointers(),
		ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag,
		nil)
}

// Resize resizes the page blob to the specified size (which must be a multiple of 512).
// For more information, see https://docs.microsoft.com/rest/api/storageservices/set-blob-properties.
func (pb PageBlobURL) Resize(ctx context.Context, size int64, ac BlobAccessConditions) (*PageBlobResizeResponse, error) {
	if size%PageBlobPageBytes != 0 {
		panic("Size must be a multiple of PageBlobPageBytes (512)")
	}
	ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag := ac.HTTPAccessConditions.pointers()
	return pb.pbClient.Resize(ctx, size, nil, ac.LeaseAccessConditions.pointers(),
		ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag, nil)
}

// SetSequenceNumber sets the page blob's sequence number.
func (pb PageBlobURL) UpdateSequenceNumber(ctx context.Context, action SequenceNumberActionType, sequenceNumber int64,
	ac BlobAccessConditions) (*PageBlobUpdateSequenceNumberResponse, error) {
	if sequenceNumber < 0 {
		panic("sequenceNumber must be greater than or equal to 0")
	}
	sn := &sequenceNumber
	if action == SequenceNumberActionIncrement {
		sn = nil
	}
	ifModifiedSince, ifUnmodifiedSince, ifMatch, ifNoneMatch := ac.HTTPAccessConditions.pointers()
	return pb.pbClient.UpdateSequenceNumber(ctx, action, nil,
		ac.LeaseAccessConditions.pointers(), ifModifiedSince, ifUnmodifiedSince, ifMatch, ifNoneMatch,
		sn, nil)
}

// StartIncrementalCopy begins an operation to start an incremental copy from one page blob's snapshot to this page blob.
// The snapshot is copied such that only the differential changes between the previously copied snapshot are transferred to the destination.
// The copied snapshots are complete copies of the original snapshot and can be read or copied from as usual.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/incremental-copy-blob and
// https://docs.microsoft.com/en-us/azure/virtual-machines/windows/incremental-snapshots.
func (pb PageBlobURL) StartCopyIncremental(ctx context.Context, source url.URL, snapshot string, ac BlobAccessConditions) (*PageBlobCopyIncrementalResponse, error) {
	ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag := ac.HTTPAccessConditions.pointers()
	qp := source.Query()
	qp.Set("snapshot", snapshot)
	source.RawQuery = qp.Encode()
	return pb.pbClient.CopyIncremental(ctx, source.String(), nil, nil,
		ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag, nil)
}

func (pr PageRange) pointers() *string {
	if pr.Start < 0 {
		panic("PageRange's Start value must be greater than or equal to 0")
	}
	if pr.End <= 0 {
		panic("PageRange's End value must be greater than 0")
	}
	if pr.Start%PageBlobPageBytes != 0 {
		panic("PageRange's Start value must be a multiple of 512")
	}
	if pr.End%PageBlobPageBytes != (PageBlobPageBytes - 1) {
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
	IfSequenceNumberLessThan int64

	// IfSequenceNumberLessThanOrEqual ensures that the page blob operation succeeds
	// only if the blob's sequence number is less than or equal to a value.
	// IfSequenceNumberLessThanOrEqual=0 means no 'IfSequenceNumberLessThanOrEqual' header specified.
	// IfSequenceNumberLessThanOrEqual>0 means 'IfSequenceNumberLessThanOrEqual' header specified with its value
	// IfSequenceNumberLessThanOrEqual=-1 means 'IfSequenceNumberLessThanOrEqual' header specified with a value of 0
	IfSequenceNumberLessThanOrEqual int64

	// IfSequenceNumberEqual ensures that the page blob operation succeeds
	// only if the blob's sequence number is equal to a value.
	// IfSequenceNumberEqual=0 means no 'IfSequenceNumberEqual' header specified.
	// IfSequenceNumberEqual>0 means 'IfSequenceNumberEqual' header specified with its value
	// IfSequenceNumberEqual=-1 means 'IfSequenceNumberEqual' header specified with a value of 0
	IfSequenceNumberEqual int64
}

// pointers is for internal infrastructure. It returns the fields as pointers.
func (ac PageBlobAccessConditions) pointers() (snltoe *int64, snlt *int64, sne *int64) {
	if ac.IfSequenceNumberLessThan < -1 {
		panic("Ifsequencenumberlessthan can't be less than -1")
	}
	if ac.IfSequenceNumberLessThanOrEqual < -1 {
		panic("IfSequenceNumberLessThanOrEqual can't be less than -1")
	}
	if ac.IfSequenceNumberEqual < -1 {
		panic("IfSequenceNumberEqual can't be less than -1")
	}

	var zero int64 // Defaults to 0
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
