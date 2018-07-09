package azblob

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/Azure/azure-pipeline-go/pipeline"
)

// A ContainerURL represents a URL to the Azure Storage container allowing you to manipulate its blobs.
type ContainerURL struct {
	client containerClient
}

// NewContainerURL creates a ContainerURL object using the specified URL and request policy pipeline.
func NewContainerURL(url url.URL, p pipeline.Pipeline) ContainerURL {
	if p == nil {
		panic("p can't be nil")
	}
	client := newContainerClient(url, p)
	return ContainerURL{client: client}
}

// URL returns the URL endpoint used by the ContainerURL object.
func (c ContainerURL) URL() url.URL {
	return c.client.URL()
}

// String returns the URL as a string.
func (c ContainerURL) String() string {
	u := c.URL()
	return u.String()
}

// WithPipeline creates a new ContainerURL object identical to the source but with the specified request policy pipeline.
func (c ContainerURL) WithPipeline(p pipeline.Pipeline) ContainerURL {
	return NewContainerURL(c.URL(), p)
}

// NewBlobURL creates a new BlobURL object by concatenating blobName to the end of
// ContainerURL's URL. The new BlobURL uses the same request policy pipeline as the ContainerURL.
// To change the pipeline, create the BlobURL and then call its WithPipeline method passing in the
// desired pipeline object. Or, call this package's NewBlobURL instead of calling this object's
// NewBlobURL method.
func (c ContainerURL) NewBlobURL(blobName string) BlobURL {
	blobURL := appendToURLPath(c.URL(), blobName)
	return NewBlobURL(blobURL, c.client.Pipeline())
}

// NewAppendBlobURL creates a new AppendBlobURL object by concatenating blobName to the end of
// ContainerURL's URL. The new AppendBlobURL uses the same request policy pipeline as the ContainerURL.
// To change the pipeline, create the AppendBlobURL and then call its WithPipeline method passing in the
// desired pipeline object. Or, call this package's NewAppendBlobURL instead of calling this object's
// NewAppendBlobURL method.
func (c ContainerURL) NewAppendBlobURL(blobName string) AppendBlobURL {
	blobURL := appendToURLPath(c.URL(), blobName)
	return NewAppendBlobURL(blobURL, c.client.Pipeline())
}

// NewBlockBlobURL creates a new BlockBlobURL object by concatenating blobName to the end of
// ContainerURL's URL. The new BlockBlobURL uses the same request policy pipeline as the ContainerURL.
// To change the pipeline, create the BlockBlobURL and then call its WithPipeline method passing in the
// desired pipeline object. Or, call this package's NewBlockBlobURL instead of calling this object's
// NewBlockBlobURL method.
func (c ContainerURL) NewBlockBlobURL(blobName string) BlockBlobURL {
	blobURL := appendToURLPath(c.URL(), blobName)
	return NewBlockBlobURL(blobURL, c.client.Pipeline())
}

// NewPageBlobURL creates a new PageBlobURL object by concatenating blobName to the end of
// ContainerURL's URL. The new PageBlobURL uses the same request policy pipeline as the ContainerURL.
// To change the pipeline, create the PageBlobURL and then call its WithPipeline method passing in the
// desired pipeline object. Or, call this package's NewPageBlobURL instead of calling this object's
// NewPageBlobURL method.
func (c ContainerURL) NewPageBlobURL(blobName string) PageBlobURL {
	blobURL := appendToURLPath(c.URL(), blobName)
	return NewPageBlobURL(blobURL, c.client.Pipeline())
}

// Create creates a new container within a storage account. If a container with the same name already exists, the operation fails.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/create-container.
func (c ContainerURL) Create(ctx context.Context, metadata Metadata, publicAccessType PublicAccessType) (*ContainerCreateResponse, error) {
	return c.client.Create(ctx, nil, metadata, publicAccessType, nil)
}

// Delete marks the specified container for deletion. The container and any blobs contained within it are later deleted during garbage collection.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/delete-container.
func (c ContainerURL) Delete(ctx context.Context, ac ContainerAccessConditions) (*ContainerDeleteResponse, error) {
	if ac.IfMatch != ETagNone || ac.IfNoneMatch != ETagNone {
		panic("the IfMatch and IfNoneMatch access conditions must have their default values because they are ignored by the service")
	}

	ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag := ac.HTTPAccessConditions.pointers()
	return c.client.Delete(ctx, nil, nil, ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag, nil)
}

// GetPropertiesAndMetadata returns the container's metadata and system properties.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/get-container-metadata.
func (c ContainerURL) GetPropertiesAndMetadata(ctx context.Context, ac LeaseAccessConditions) (*ContainerGetPropertiesResponse, error) {
	// NOTE: GetMetadata actually calls GetProperties internally because GetProperties returns the metadata AND the properties.
	// This allows us to not expose a GetProperties method at all simplifying the API.
	return c.client.GetProperties(ctx, nil, ac.pointers(), nil)
}

// SetMetadata sets the container's metadata.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/set-container-metadata.
func (c ContainerURL) SetMetadata(ctx context.Context, metadata Metadata, ac ContainerAccessConditions) (*ContainerSetMetadataResponse, error) {
	if !ac.IfUnmodifiedSince.IsZero() || ac.IfMatch != ETagNone || ac.IfNoneMatch != ETagNone {
		panic("the IfUnmodifiedSince, IfMatch, and IfNoneMatch must have their default values because they are ignored by the blob service")
	}
	ifModifiedSince, _, _, _ := ac.HTTPAccessConditions.pointers()
	return c.client.SetMetadata(ctx, nil, ac.LeaseAccessConditions.pointers(), metadata, ifModifiedSince, nil)
}

// GetPermissions returns the container's permissions. The permissions indicate whether container's blobs may be accessed publicly.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/get-container-acl.
func (c ContainerURL) GetPermissions(ctx context.Context, ac LeaseAccessConditions) (*SignedIdentifiers, error) {
	return c.client.GetACL(ctx, nil, ac.pointers(), nil)
}

// The AccessPolicyPermission type simplifies creating the permissions string for a container's access policy.
// Initialize an instance of this type and then call its String method to set AccessPolicy's Permission field.
type AccessPolicyPermission struct {
	Read, Add, Create, Write, Delete, List bool
}

// String produces the access policy permission string for an Azure Storage container.
// Call this method to set AccessPolicy's Permission field.
func (p AccessPolicyPermission) String() string {
	var b bytes.Buffer
	if p.Read {
		b.WriteRune('r')
	}
	if p.Add {
		b.WriteRune('a')
	}
	if p.Create {
		b.WriteRune('c')
	}
	if p.Write {
		b.WriteRune('w')
	}
	if p.Delete {
		b.WriteRune('d')
	}
	if p.List {
		b.WriteRune('l')
	}
	return b.String()
}

// Parse initializes the AccessPolicyPermission's fields from a string.
func (p *AccessPolicyPermission) Parse(s string) error {
	*p = AccessPolicyPermission{} // Clear the flags
	for _, r := range s {
		switch r {
		case 'r':
			p.Read = true
		case 'a':
			p.Add = true
		case 'c':
			p.Create = true
		case 'w':
			p.Write = true
		case 'd':
			p.Delete = true
		case 'l':
			p.List = true
		default:
			return fmt.Errorf("Invalid permission: '%v'", r)
		}
	}
	return nil
}

// SetPermissions sets the container's permissions. The permissions indicate whether blobs in a container may be accessed publicly.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/set-container-acl.
func (c ContainerURL) SetPermissions(ctx context.Context, accessType PublicAccessType, permissions []SignedIdentifier,
	ac ContainerAccessConditions) (*ContainerSetACLResponse, error) {
	if ac.IfMatch != ETagNone || ac.IfNoneMatch != ETagNone {
		panic("the IfMatch and IfNoneMatch access conditions must have their default values because they are ignored by the service")
	}
	ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag := ac.HTTPAccessConditions.pointers()
	return c.client.SetACL(ctx, permissions, nil, nil, accessType, ifModifiedSince, ifUnmodifiedSince, ifMatchETag, ifNoneMatchETag, nil)
}

// AcquireLease acquires a lease on the container for delete operations. The lease duration must be between 15 to 60 seconds, or infinite (-1).
// For more information, see https://docs.microsoft.com/rest/api/storageservices/lease-container.
func (c ContainerURL) AcquireLease(ctx context.Context, proposedID string, duration int32, ac HTTPAccessConditions) (*ContainerLeaseResponse, error) {
	ifModifiedSince, ifUnmodifiedSince, _, _ := ac.pointers()
	return c.client.Lease(ctx, LeaseActionAcquire, nil, nil, nil, &duration, &proposedID,
		ifModifiedSince, ifUnmodifiedSince, nil)
}

// RenewLease renews the container's previously-acquired lease.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/lease-container.
func (c ContainerURL) RenewLease(ctx context.Context, leaseID string, ac HTTPAccessConditions) (*ContainerLeaseResponse, error) {
	ifModifiedSince, ifUnmodifiedSince, _, _ := ac.pointers()
	return c.client.Lease(ctx, LeaseActionRenew, nil, &leaseID, nil, nil, nil, ifModifiedSince, ifUnmodifiedSince, nil)
}

// ReleaseLease releases the container's previously-acquired lease.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/lease-container.
func (c ContainerURL) ReleaseLease(ctx context.Context, leaseID string, ac HTTPAccessConditions) (*ContainerLeaseResponse, error) {
	ifModifiedSince, ifUnmodifiedSince, _, _ := ac.pointers()
	return c.client.Lease(ctx, LeaseActionRelease, nil, &leaseID, nil, nil, nil, ifModifiedSince, ifUnmodifiedSince, nil)
}

// BreakLease breaks the container's previously-acquired lease (if it exists).
// For more information, see https://docs.microsoft.com/rest/api/storageservices/lease-container.
func (c ContainerURL) BreakLease(ctx context.Context, period int32, ac HTTPAccessConditions) (*ContainerLeaseResponse, error) {
	ifModifiedSince, ifUnmodifiedSince, _, _ := ac.pointers()
	return c.client.Lease(ctx, LeaseActionBreak, nil, nil, leasePeriodPointer(period), nil, nil, ifModifiedSince, ifUnmodifiedSince, nil)
}

// ChangeLease changes the container's lease ID.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/lease-container.
func (c ContainerURL) ChangeLease(ctx context.Context, leaseID string, proposedID string, ac HTTPAccessConditions) (*ContainerLeaseResponse, error) {
	ifModifiedSince, ifUnmodifiedSince, _, _ := ac.pointers()
	return c.client.Lease(ctx, LeaseActionChange, nil, &leaseID, nil, nil, &proposedID, ifModifiedSince, ifUnmodifiedSince, nil)
}

// ListBlobs returns a single segment of blobs starting from the specified Marker. Use an empty
// Marker to start enumeration from the beginning. Blob names are returned in lexicographic order.
// After getting a segment, process it, and then call ListBlobs again (passing the the previously-returned
// Marker) to get the next segment.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/list-blobs.
func (c ContainerURL) ListBlobs(ctx context.Context, marker Marker, o ListBlobsOptions) (*ListBlobsResponse, error) {
	prefix, delimiter, include, maxResults := o.pointers()
	return c.client.ListBlobs(ctx, prefix, delimiter, marker.val, maxResults, include, nil, nil)
}

// ListBlobsOptions defines options available when calling ListBlobs.
type ListBlobsOptions struct {
	Details   BlobListingDetails // No IncludeType header is produced if ""
	Prefix    string             // No Prefix header is produced if ""
	Delimiter string

	// SetMaxResults sets the maximum desired results you want the service to return. Note, the
	// service may return fewer results than requested.
	// MaxResults=0 means no 'MaxResults' header specified.
	MaxResults int32
}

func (o *ListBlobsOptions) pointers() (prefix *string, delimiter *string, include ListBlobsIncludeType, maxResults *int32) {
	if o.Prefix != "" {
		prefix = &o.Prefix
	}
	if o.Delimiter != "" {
		delimiter = &o.Delimiter
	}
	include = ListBlobsIncludeType(o.Details.string())
	if o.MaxResults != 0 {
		if o.MaxResults < 0 {
			panic("MaxResults must be >= 0")
		}
		maxResults = &o.MaxResults
	}
	return
}

// BlobListingDetails indicates what additional information the service should return with each blob.
type BlobListingDetails struct {
	Copy, Metadata, Snapshots, UncommittedBlobs bool
}

// string produces the Include query parameter's value.
func (d *BlobListingDetails) string() string {
	items := make([]string, 0, 4)
	// NOTE: Multiple strings MUST be appended in alphabetic order or signing the string for authentication fails!
	if d.Copy {
		items = append(items, string(ListBlobsIncludeCopy))
	}
	if d.Metadata {
		items = append(items, string(ListBlobsIncludeMetadata))
	}
	if d.Snapshots {
		items = append(items, string(ListBlobsIncludeSnapshots))
	}
	if d.UncommittedBlobs {
		items = append(items, string(ListBlobsIncludeUncommittedblobs))
	}
	if len(items) > 0 {
		return strings.Join(items, ",")
	}
	return string(ListBlobsIncludeNone)
}
