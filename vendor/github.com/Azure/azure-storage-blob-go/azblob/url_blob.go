package azblob

import (
	"context"
	"net/url"

	"github.com/Azure/azure-pipeline-go/pipeline"
)

// A BlobURL represents a URL to an Azure Storage blob; the blob may be a block blob, append blob, or page blob.
type BlobURL struct {
	blobClient blobClient
}

// NewBlobURL creates a BlobURL object using the specified URL and request policy pipeline.
func NewBlobURL(url url.URL, p pipeline.Pipeline) BlobURL {
	blobClient := newBlobClient(url, p)
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

func (b BlobURL) GetAccountInfo(ctx context.Context) (*BlobGetAccountInfoResponse, error) {
	return b.blobClient.GetAccountInfo(ctx)
}

// WithPipeline creates a new BlobURL object identical to the source but with the specified request policy pipeline.
func (b BlobURL) WithPipeline(p pipeline.Pipeline) BlobURL {
	return NewBlobURL(b.blobClient.URL(), p)
}

// WithSnapshot creates a new BlobURL object identical to the source but with the specified snapshot timestamp.
// Pass "" to remove the snapshot returning a URL to the base blob.
func (b BlobURL) WithSnapshot(snapshot string) BlobURL {
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

// DownloadBlob reads a range of bytes from a blob. The response also includes the blob's properties and metadata.
// Passing azblob.CountToEnd (0) for count will download the blob from the offset to the end.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/get-blob.
func (b BlobURL) Download(ctx context.Context, offset int64, count int64, ac BlobAccessConditions, rangeGetContentMD5 bool) (*DownloadResponse, error) {
	var xRangeGetContentMD5 *bool
	if rangeGetContentMD5 {
		xRangeGetContentMD5 = &rangeGetContentMD5
	}
	ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag := ac.ModifiedAccessConditions.pointers()
	dr, err := b.blobClient.Download(ctx, nil, nil,
		httpRange{offset: offset, count: count}.pointers(),
		ac.LeaseAccessConditions.pointers(), xRangeGetContentMD5, nil,
		nil, nil, EncryptionAlgorithmNone, // CPK
		ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag, nil)
	if err != nil {
		return nil, err
	}
	return &DownloadResponse{
		b:       b,
		r:       dr,
		ctx:     ctx,
		getInfo: HTTPGetterInfo{Offset: offset, Count: count, ETag: dr.ETag()},
	}, err
}

// DeleteBlob marks the specified blob or snapshot for deletion. The blob is later deleted during garbage collection.
// Note that deleting a blob also deletes all its snapshots.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/delete-blob.
func (b BlobURL) Delete(ctx context.Context, deleteOptions DeleteSnapshotsOptionType, ac BlobAccessConditions) (*BlobDeleteResponse, error) {
	ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag := ac.ModifiedAccessConditions.pointers()
	return b.blobClient.Delete(ctx, nil, nil, ac.LeaseAccessConditions.pointers(), deleteOptions,
		ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag, nil)
}

// Undelete restores the contents and metadata of a soft-deleted blob and any associated soft-deleted snapshots.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/undelete-blob.
func (b BlobURL) Undelete(ctx context.Context) (*BlobUndeleteResponse, error) {
	return b.blobClient.Undelete(ctx, nil, nil)
}

// SetTier operation sets the tier on a blob. The operation is allowed on a page
// blob in a premium storage account and on a block blob in a blob storage account (locally
// redundant storage only). A premium page blob's tier determines the allowed size, IOPS, and
// bandwidth of the blob. A block blob's tier determines Hot/Cool/Archive storage type. This operation
// does not update the blob's ETag.
// For detailed information about block blob level tiering see https://docs.microsoft.com/en-us/azure/storage/blobs/storage-blob-storage-tiers.
func (b BlobURL) SetTier(ctx context.Context, tier AccessTierType, lac LeaseAccessConditions) (*BlobSetTierResponse, error) {
	return b.blobClient.SetTier(ctx, tier, nil, RehydratePriorityNone, nil, lac.pointers())
}

// GetBlobProperties returns the blob's properties.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/get-blob-properties.
func (b BlobURL) GetProperties(ctx context.Context, ac BlobAccessConditions) (*BlobGetPropertiesResponse, error) {
	ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag := ac.ModifiedAccessConditions.pointers()
	return b.blobClient.GetProperties(ctx, nil, nil, ac.LeaseAccessConditions.pointers(),
		nil, nil, EncryptionAlgorithmNone, // CPK
		ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag, nil)
}

// SetBlobHTTPHeaders changes a blob's HTTP headers.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/set-blob-properties.
func (b BlobURL) SetHTTPHeaders(ctx context.Context, h BlobHTTPHeaders, ac BlobAccessConditions) (*BlobSetHTTPHeadersResponse, error) {
	ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag := ac.ModifiedAccessConditions.pointers()
	return b.blobClient.SetHTTPHeaders(ctx, nil,
		&h.CacheControl, &h.ContentType, h.ContentMD5, &h.ContentEncoding, &h.ContentLanguage,
		ac.LeaseAccessConditions.pointers(), ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag,
		&h.ContentDisposition, nil)
}

// SetBlobMetadata changes a blob's metadata.
// https://docs.microsoft.com/rest/api/storageservices/set-blob-metadata.
func (b BlobURL) SetMetadata(ctx context.Context, metadata Metadata, ac BlobAccessConditions) (*BlobSetMetadataResponse, error) {
	ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag := ac.ModifiedAccessConditions.pointers()
	return b.blobClient.SetMetadata(ctx, nil, metadata, ac.LeaseAccessConditions.pointers(),
		nil, nil, EncryptionAlgorithmNone, // CPK
		ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag, nil)
}

// CreateSnapshot creates a read-only snapshot of a blob.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/snapshot-blob.
func (b BlobURL) CreateSnapshot(ctx context.Context, metadata Metadata, ac BlobAccessConditions) (*BlobCreateSnapshotResponse, error) {
	// CreateSnapshot does NOT panic if the user tries to create a snapshot using a URL that already has a snapshot query parameter
	// because checking this would be a performance hit for a VERY unusual path and I don't think the common case should suffer this
	// performance hit.
	ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag := ac.ModifiedAccessConditions.pointers()
	return b.blobClient.CreateSnapshot(ctx, nil, metadata,
		nil, nil, EncryptionAlgorithmNone, // CPK
		ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag, ac.LeaseAccessConditions.pointers(), nil)
}

// AcquireLease acquires a lease on the blob for write and delete operations. The lease duration must be between
// 15 to 60 seconds, or infinite (-1).
// For more information, see https://docs.microsoft.com/rest/api/storageservices/lease-blob.
func (b BlobURL) AcquireLease(ctx context.Context, proposedID string, duration int32, ac ModifiedAccessConditions) (*BlobAcquireLeaseResponse, error) {
	ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag := ac.pointers()
	return b.blobClient.AcquireLease(ctx, nil, &duration, &proposedID,
		ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag, nil)
}

// RenewLease renews the blob's previously-acquired lease.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/lease-blob.
func (b BlobURL) RenewLease(ctx context.Context, leaseID string, ac ModifiedAccessConditions) (*BlobRenewLeaseResponse, error) {
	ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag := ac.pointers()
	return b.blobClient.RenewLease(ctx, leaseID, nil,
		ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag, nil)
}

// ReleaseLease releases the blob's previously-acquired lease.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/lease-blob.
func (b BlobURL) ReleaseLease(ctx context.Context, leaseID string, ac ModifiedAccessConditions) (*BlobReleaseLeaseResponse, error) {
	ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag := ac.pointers()
	return b.blobClient.ReleaseLease(ctx, leaseID, nil,
		ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag, nil)
}

// BreakLease breaks the blob's previously-acquired lease (if it exists). Pass the LeaseBreakDefault (-1)
// constant to break a fixed-duration lease when it expires or an infinite lease immediately.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/lease-blob.
func (b BlobURL) BreakLease(ctx context.Context, breakPeriodInSeconds int32, ac ModifiedAccessConditions) (*BlobBreakLeaseResponse, error) {
	ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag := ac.pointers()
	return b.blobClient.BreakLease(ctx, nil, leasePeriodPointer(breakPeriodInSeconds),
		ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag, nil)
}

// ChangeLease changes the blob's lease ID.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/lease-blob.
func (b BlobURL) ChangeLease(ctx context.Context, leaseID string, proposedID string, ac ModifiedAccessConditions) (*BlobChangeLeaseResponse, error) {
	ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag := ac.pointers()
	return b.blobClient.ChangeLease(ctx, leaseID, proposedID,
		nil, ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag, nil)
}

// LeaseBreakNaturally tells ContainerURL's or BlobURL's BreakLease method to break the lease using service semantics.
const LeaseBreakNaturally = -1

func leasePeriodPointer(period int32) (p *int32) {
	if period != LeaseBreakNaturally {
		p = &period
	}
	return nil
}

// StartCopyFromURL copies the data at the source URL to a blob.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/copy-blob.
func (b BlobURL) StartCopyFromURL(ctx context.Context, source url.URL, metadata Metadata, srcac ModifiedAccessConditions, dstac BlobAccessConditions) (*BlobStartCopyFromURLResponse, error) {
	srcIfModifiedSince, srcIfUnmodifiedSince, srcIfMatchETag, srcIfNoneMatchETag := srcac.pointers()
	dstIfModifiedSince, dstIfUnmodifiedSince, dstIfMatchETag, dstIfNoneMatchETag := dstac.ModifiedAccessConditions.pointers()
	dstLeaseID := dstac.LeaseAccessConditions.pointers()

	return b.blobClient.StartCopyFromURL(ctx, source.String(), nil, metadata,
		AccessTierNone, RehydratePriorityNone, srcIfModifiedSince, srcIfUnmodifiedSince,
		srcIfMatchETag, srcIfNoneMatchETag,
		dstIfModifiedSince, dstIfUnmodifiedSince,
		dstIfMatchETag, dstIfNoneMatchETag,
		dstLeaseID, nil)
}

// AbortCopyFromURL stops a pending copy that was previously started and leaves a destination blob with 0 length and metadata.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/abort-copy-blob.
func (b BlobURL) AbortCopyFromURL(ctx context.Context, copyID string, ac LeaseAccessConditions) (*BlobAbortCopyFromURLResponse, error) {
	return b.blobClient.AbortCopyFromURL(ctx, copyID, nil, ac.pointers(), nil)
}
