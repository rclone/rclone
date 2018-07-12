package azblob

import (
	"context"
	"net/url"
	"strings"

	"github.com/Azure/azure-pipeline-go/pipeline"
)

const (
	// ContainerNameRoot is the special Azure Storage name used to identify a storage account's root container.
	ContainerNameRoot = "$root"

	// ContainerNameLogs is the special Azure Storage name used to identify a storage account's logs container.
	ContainerNameLogs = "$logs"
)

// A ServiceURL represents a URL to the Azure Storage Blob service allowing you to manipulate blob containers.
type ServiceURL struct {
	client serviceClient
}

// NewServiceURL creates a ServiceURL object using the specified URL and request policy pipeline.
func NewServiceURL(primaryURL url.URL, p pipeline.Pipeline) ServiceURL {
	if p == nil {
		panic("p can't be nil")
	}
	client := newServiceClient(primaryURL, p)
	return ServiceURL{client: client}
}

// URL returns the URL endpoint used by the ServiceURL object.
func (s ServiceURL) URL() url.URL {
	return s.client.URL()
}

// String returns the URL as a string.
func (s ServiceURL) String() string {
	u := s.URL()
	return u.String()
}

// WithPipeline creates a new ServiceURL object identical to the source but with the specified request policy pipeline.
func (s ServiceURL) WithPipeline(p pipeline.Pipeline) ServiceURL {
	return NewServiceURL(s.URL(), p)
}

// NewContainerURL creates a new ContainerURL object by concatenating containerName to the end of
// ServiceURL's URL. The new ContainerURL uses the same request policy pipeline as the ServiceURL.
// To change the pipeline, create the ContainerURL and then call its WithPipeline method passing in the
// desired pipeline object. Or, call this package's NewContainerURL instead of calling this object's
// NewContainerURL method.
func (s ServiceURL) NewContainerURL(containerName string) ContainerURL {
	containerURL := appendToURLPath(s.URL(), containerName)
	return NewContainerURL(containerURL, s.client.Pipeline())
}

// appendToURLPath appends a string to the end of a URL's path (prefixing the string with a '/' if required)
func appendToURLPath(u url.URL, name string) url.URL {
	// e.g. "https://ms.com/a/b/?k1=v1&k2=v2#f"
	// When you call url.Parse() this is what you'll get:
	//     Scheme: "https"
	//     Opaque: ""
	//       User: nil
	//       Host: "ms.com"
	//       Path: "/a/b/"	This should start with a / and it might or might not have a trailing slash
	//    RawPath: ""
	// ForceQuery: false
	//   RawQuery: "k1=v1&k2=v2"
	//   Fragment: "f"
	if len(u.Path) == 0 || u.Path[len(u.Path)-1] != '/' {
		u.Path += "/" // Append "/" to end before appending name
	}
	u.Path += name
	return u
}

// ListContainersFlatSegment returns a single segment of containers starting from the specified Marker. Use an empty
// Marker to start enumeration from the beginning. Container names are returned in lexicographic order.
// After getting a segment, process it, and then call ListContainersFlatSegment again (passing the the
// previously-returned Marker) to get the next segment. For more information, see
// https://docs.microsoft.com/rest/api/storageservices/list-containers2.
func (s ServiceURL) ListContainersSegment(ctx context.Context, marker Marker, o ListContainersSegmentOptions) (*ListContainersResponse, error) {
	prefix, include, maxResults := o.pointers()
	return s.client.ListContainersSegment(ctx, prefix, marker.val, maxResults, include, nil, nil)
}

// ListContainersOptions defines options available when calling ListContainers.
type ListContainersSegmentOptions struct {
	Detail     ListContainersDetail // No IncludeType header is produced if ""
	Prefix     string                   // No Prefix header is produced if ""
	MaxResults int32                    // 0 means unspecified
	// TODO: update swagger to generate this type?
}

func (o *ListContainersSegmentOptions) pointers() (prefix *string, include ListContainersIncludeType, maxResults *int32) {
	if o.Prefix != "" {
		prefix = &o.Prefix
	}
	if o.MaxResults != 0 {
		if o.MaxResults < 0 {
			panic("MaxResults must be >= 0")
		}
		maxResults = &o.MaxResults
	}
	include = ListContainersIncludeType(o.Detail.string())
	return
}

// ListContainersFlatDetail indicates what additional information the service should return with each container.
type ListContainersDetail struct {
	// Tells the service whether to return metadata for each container.
	Metadata bool
}

// string produces the Include query parameter's value.
func (d *ListContainersDetail) string() string {
	items := make([]string, 0, 1)
	// NOTE: Multiple strings MUST be appended in alphabetic order or signing the string for authentication fails!
	if d.Metadata {
		items = append(items, string(ListContainersIncludeMetadata))
	}
	if len(items) > 0 {
		return strings.Join(items, ",")
	}
	return string(ListContainersIncludeNone)
}

func (bsu ServiceURL) GetProperties(ctx context.Context) (*StorageServiceProperties, error) {
	return bsu.client.GetProperties(ctx, nil, nil)
}

func (bsu ServiceURL) SetProperties(ctx context.Context, properties StorageServiceProperties) (*ServiceSetPropertiesResponse, error) {
	return bsu.client.SetProperties(ctx, properties, nil, nil)
}

func (bsu ServiceURL) GetStatistics(ctx context.Context) (*StorageServiceStats, error) {
	return bsu.client.GetStatistics(ctx, nil, nil)
}
