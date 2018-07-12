package azblob

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/Azure/azure-pipeline-go/pipeline"
)

// A BlobURL represents a URL to an Azure Storage blob; the blob may be a block blob, append blob, or page blob.
type BlobURL struct {
	blobClient blobsClient
}

// NewBlobURL creates a BlobURL object using the specified URL and request policy pipeline.
func NewBlobURL(url url.URL, p pipeline.Pipeline) BlobURL {
	if p == nil {
		panic("p can't be nil")
	}
	blobClient := newBlobsClient(url, p)
	return BlobURL{blobClient: blobClient}
}

// URL returns the URL endpoint used by the BlobURL object.
func (b BlobURL) URL() url.URL {
	return b.blobClient.URL()
}

// String returns the URL as a string.
func (b BlobURL) String() string {
	u := b.URL()
	return u.String()
}

// WithPipeline creates a new BlobURL object identical to the source but with the specified request policy pipeline.
func (b BlobURL) WithPipeline(p pipeline.Pipeline) BlobURL {
	if p == nil {
		panic("p can't be nil")
	}
	return NewBlobURL(b.blobClient.URL(), p)
}

// WithSnapshot creates a new BlobURL object identical to the source but with the specified snapshot timestamp.
// Pass time.Time{} to remove the snapshot returning a URL to the base blob.
func (b BlobURL) WithSnapshot(snapshot time.Time) BlobURL {
	p := NewBlobURLParts(b.URL())
	p.Snapshot = snapshot
	return NewBlobURL(p.URL(), b.blobClient.Pipeline())
}

// ToAppendBlobURL creates an AppendBlobURL using the source's URL and pipeline.
func (b BlobURL) ToAppendBlobURL() AppendBlobURL {
	return NewAppendBlobURL(b.URL(), b.blobClient.Pipeline())
}

// ToBlockBlobURL creates a BlockBlobURL using the source's URL and pipeline.
func (b BlobURL) ToBlockBlobURL() BlockBlobURL {
	return NewBlockBlobURL(b.URL(), b.blobClient.Pipeline())
}

// ToPageBlobURL creates a PageBlobURL using the source's URL and pipeline.
func (b BlobURL) ToPageBlobURL() PageBlobURL {
	return NewPageBlobURL(b.URL(), b.blobClient.Pipeline())
}

// StartCopy copies the data at the source URL to a blob.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/copy-blob.
func (b BlobURL) StartCopy(ctx context.Context, source url.URL, metadata Metadata, srcac BlobAccessConditions, dstac BlobAccessConditions) (*BlobsCopyResponse, error) {
	srcIfModifiedSince, srcIfUnmodifiedSince, srcIfMatchETag, srcIfNoneMatchETag := srcac.HTTPAccessConditions.pointers()
	dstIfModifiedSince, dstIfUnmodifiedSince, dstIfMatchETag, dstIfNoneMatchETag := dstac.HTTPAccessConditions.pointers()
	srcLeaseID := srcac.LeaseAccessConditions.pointers()
	dstLeaseID := dstac.LeaseAccessConditions.pointers()

	return b.blobClient.Copy(ctx, source.String(), nil, metadata,
		srcIfModifiedSince, srcIfUnmodifiedSince,
		srcIfMatchETag, srcIfNoneMatchETag,
		dstIfModifiedSince, dstIfUnmodifiedSince,
		dstIfMatchETag, dstIfNoneMatchETag,
		dstLeaseID, srcLeaseID, nil)
}

// AbortCopy stops a pending copy that was previously started and leaves a destination blob with 0 length and metadata.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/abort-copy-blob.
func (b BlobURL) AbortCopy(ctx context.Context, copyID string, ac LeaseAccessConditions) (*BlobsAbortCopyResponse, error) {
	return b.blobClient.AbortCopy(ctx, copyID, "abort", nil, ac.pointers(), nil)
}

// BlobRange defines a range of bytes within a blob, starting at Offset and ending
// at Offset+Count. Use a zero-value BlobRange to indicate the entire blob.
type BlobRange struct {
	Offset int64
	Count  int64
}

func (dr *BlobRange) pointers() *string {
	if dr.Offset < 0 {
		panic("The blob's range Offset must be >= 0")
	}
	if dr.Count < 0 {
		panic("The blob's range Count must be >= 0")
	}
	if dr.Offset == 0 && dr.Count == 0 {
		return nil
	}
	endRange := ""
	if dr.Count > 0 {
		endRange = strconv.FormatInt((dr.Offset+dr.Count)-1, 10)
	}
	dataRange := fmt.Sprintf("bytes=%v-%s", dr.Offset, endRange)
	return &dataRange
}

// GetBlob reads a range of bytes from a blob. The response also includes the blob's properties and metadata.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/get-blob.
func (b BlobURL) GetBlob(ctx context.Context, blobRange BlobRange, ac BlobAccessConditions, rangeGetContentMD5 bool) (*GetResponse, error) {
	var xRangeGetContentMD5 *bool
	if rangeGetContentMD5 {
		xRangeGetContentMD5 = &rangeGetContentMD5
	}
	ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag := ac.HTTPAccessConditions.pointers()
	return b.blobClient.Get(ctx, nil, nil, blobRange.pointers(), ac.LeaseAccessConditions.pointers(), xRangeGetContentMD5,
		ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag, nil)
}

// Delete marks the specified blob or snapshot for deletion. The blob is later deleted during garbage collection.
// Note that deleting a blob also deletes all its snapshots.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/delete-blob.
func (b BlobURL) Delete(ctx context.Context, deleteOptions DeleteSnapshotsOptionType, ac BlobAccessConditions) (*BlobsDeleteResponse, error) {
	ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag := ac.HTTPAccessConditions.pointers()
	return b.blobClient.Delete(ctx, nil, nil, ac.LeaseAccessConditions.pointers(), deleteOptions,
		ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag, nil)
}

// GetPropertiesAndMetadata returns the blob's metadata and properties.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/get-blob-properties.
func (b BlobURL) GetPropertiesAndMetadata(ctx context.Context, ac BlobAccessConditions) (*BlobsGetPropertiesResponse, error) {
	ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag := ac.HTTPAccessConditions.pointers()
	return b.blobClient.GetProperties(ctx, nil, nil, ac.LeaseAccessConditions.pointers(),
		ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag, nil)
	// NOTE: GetMetadata actually calls GetProperties since this returns a the properties AND the metadata
}

// SetProperties changes a blob's HTTP header properties.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/set-blob-properties.
func (b BlobURL) SetProperties(ctx context.Context, h BlobHTTPHeaders, ac BlobAccessConditions) (*BlobsSetPropertiesResponse, error) {
	ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag := ac.HTTPAccessConditions.pointers()
	return b.blobClient.SetProperties(ctx, nil,
		&h.CacheControl, &h.ContentType, h.contentMD5Pointer(), &h.ContentEncoding, &h.ContentLanguage,
		ac.LeaseAccessConditions.pointers(), ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag,
		&h.ContentDisposition, nil, SequenceNumberActionNone, nil, nil)
}

// SetMetadata changes a blob's metadata.
// https://docs.microsoft.com/rest/api/storageservices/set-blob-metadata.
func (b BlobURL) SetMetadata(ctx context.Context, metadata Metadata, ac BlobAccessConditions) (*BlobsSetMetadataResponse, error) {
	ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag := ac.HTTPAccessConditions.pointers()
	return b.blobClient.SetMetadata(ctx, nil, metadata, ac.LeaseAccessConditions.pointers(),
		ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag, nil)
}

// CreateSnapshot creates a read-only snapshot of a blob.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/snapshot-blob.
func (b BlobURL) CreateSnapshot(ctx context.Context, metadata Metadata, ac BlobAccessConditions) (*BlobsTakeSnapshotResponse, error) {
	// CreateSnapshot does NOT panic if the user tries to create a snapshot using a URL that already has a snapshot query parameter
	// because checking this would be a performance hit for a VERY unusual path and I don't think the common case should suffer this
	// performance hit.
	ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag := ac.HTTPAccessConditions.pointers()
	return b.blobClient.TakeSnapshot(ctx, nil, metadata, ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag, ac.LeaseAccessConditions.pointers(), nil)
}

// AcquireLease acquires a lease on the blob for write and delete operations. The lease duration must be between
// 15 to 60 seconds, or infinite (-1).
// For more information, see https://docs.microsoft.com/rest/api/storageservices/lease-blob.
func (b BlobURL) AcquireLease(ctx context.Context, proposedID string, duration int32, ac HTTPAccessConditions) (*BlobsLeaseResponse, error) {
	ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag := ac.pointers()
	return b.blobClient.Lease(ctx, LeaseActionAcquire, nil, nil, nil, &duration, &proposedID,
		ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag, nil)
}

// RenewLease renews the blob's previously-acquired lease.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/lease-blob.
func (b BlobURL) RenewLease(ctx context.Context, leaseID string, ac HTTPAccessConditions) (*BlobsLeaseResponse, error) {
	ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag := ac.pointers()
	return b.blobClient.Lease(ctx, LeaseActionRenew, nil, &leaseID, nil, nil, nil,
		ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag, nil)
}

// ReleaseLease releases the blob's previously-acquired lease.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/lease-blob.
func (b BlobURL) ReleaseLease(ctx context.Context, leaseID string, ac HTTPAccessConditions) (*BlobsLeaseResponse, error) {
	ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag := ac.pointers()
	return b.blobClient.Lease(ctx, LeaseActionRelease, nil, &leaseID, nil, nil, nil,
		ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag, nil)
}

// BreakLease breaks the blob's previously-acquired lease (if it exists). Pass the LeaseBreakDefault (-1)
// constant to break a fixed-duration lease when it expires or an infinite lease immediately.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/lease-blob.
func (b BlobURL) BreakLease(ctx context.Context, breakPeriodInSeconds int32, ac HTTPAccessConditions) (*BlobsLeaseResponse, error) {
	ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag := ac.pointers()
	return b.blobClient.Lease(ctx, LeaseActionBreak, nil, nil, leasePeriodPointer(breakPeriodInSeconds), nil, nil,
		ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag, nil)
}

// ChangeLease changes the blob's lease ID.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/lease-blob.
func (b BlobURL) ChangeLease(ctx context.Context, leaseID string, proposedID string, ac HTTPAccessConditions) (*BlobsLeaseResponse, error) {
	ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag := ac.pointers()
	return b.blobClient.Lease(ctx, LeaseActionChange, nil, &leaseID, nil, nil, &proposedID,
		ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag, nil)
}

// LeaseBreakNaturally tells ContainerURL's or BlobURL's BreakLease method to break the lease using service semantics.
const LeaseBreakNaturally = -1

func leasePeriodPointer(period int32) (p *int32) {
	if period != LeaseBreakNaturally {
		p = &period
	}
	return nil
}
