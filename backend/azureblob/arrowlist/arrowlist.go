//go:build !plan9 && !solaris && !js

// Package arrowlist implements the experimental "Blob Listing with Apache
// Arrow" feature on top of the released Azure azblob SDK.
//
// The Azure SDK for Go has support for Arrow listing on the unreleased
// feature/storage/bifrost branch (commit c6fa341ca22b) where the listing
// pagers decode Arrow IPC stream responses into the ordinary ListBlobs
// response types. Until that ships in a tagged azblob release this package
// fills the gap: it exposes the same options and pager interface as the
// experimental SDK, built only on the public API of the released SDK plus
// copies of two small internal auth policies (see sharedkey.go and
// challenge_policy.go).
//
// This package is temporary. When Arrow listing ships in a released azblob
// the whole package should be deleted and its callers pointed back at
// container.Client.NewListBlobsHierarchyPager - the option and response
// types here are deliberately source compatible with the SDK's to make that
// a mechanical change.
package arrowlist

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
)

const (
	// moduleName and moduleVersion identify this package in the azcore
	// telemetry policy's User-Agent fragment.
	moduleName    = "github.com/rclone/rclone/backend/azureblob/arrowlist"
	moduleVersion = "v0.0.1"

	// serviceVersion is the x-ms-version sent with listing requests. Arrow
	// responses and the startFrom/endBefore parameters need at least
	// 2026-06-06.
	serviceVersion = "2026-06-06"

	// tokenScope is the OAuth scope for Azure Storage, as used by the SDK's
	// own clients.
	tokenScope = "https://storage.azure.com/.default"
)

// ErrEndBeforeXMLFallback is returned by the pager when the server answered
// with XML instead of Arrow while EndBefore was set. The service only honours
// endBefore on the Arrow listing path, so an XML page could silently contain
// blobs outside the requested range and cannot be used.
var ErrEndBeforeXMLFallback = errors.New("arrow listing fell back to XML with endBefore set")

// ListBlobsHierarchyOptions provides the configuration for
// Client.NewListBlobsHierarchyPager.
//
// It mirrors the experimental SDK's container.ListBlobsHierarchyOptions:
// the released SDK's options are embedded (providing Include, Marker,
// MaxResults, Prefix and StartFrom by promotion) with the two fields the
// released SDK lacks added alongside.
type ListBlobsHierarchyOptions struct {
	container.ListBlobsHierarchyOptions
	// EndBefore limits listing to blobs whose full container path is
	// lexically before it (exclusive). Only honoured on Arrow responses.
	EndBefore *string
	// UseArrowFormat requests the response as an Apache Arrow IPC stream.
	UseArrowFormat *bool
}

// ClientOptions contains the optional parameters for client creation.
type ClientOptions struct {
	azcore.ClientOptions
}

// Client issues Arrow listing requests to a single container.
type Client struct {
	endpoint string
	azClient *azcore.Client
}

// newClient creates a Client for the container at containerURL sending
// requests through a pipeline with the given auth policy (which may be nil
// for anonymous or SAS access).
func newClient(containerURL string, authPolicy policy.Policy, options *ClientOptions) (*Client, error) {
	if options == nil {
		options = &ClientOptions{}
	}
	plOpts := runtime.PipelineOptions{}
	if authPolicy != nil {
		plOpts.PerRetry = []policy.Policy{authPolicy}
	}
	azClient, err := azcore.NewClient(moduleName, moduleVersion, plOpts, &options.ClientOptions)
	if err != nil {
		return nil, err
	}
	return &Client{
		endpoint: containerURL,
		azClient: azClient,
	}, nil
}

// NewClient creates a Client authenticating with a token credential.
func NewClient(containerURL string, cred azcore.TokenCredential, options *ClientOptions) (*Client, error) {
	return newClient(containerURL, NewStorageChallengePolicy(cred, tokenScope, false), options)
}

// NewClientWithNoCredential creates a Client with no authentication, for
// anonymous access or a containerURL carrying a SAS token in its query.
func NewClientWithNoCredential(containerURL string, options *ClientOptions) (*Client, error) {
	return newClient(containerURL, nil, options)
}

// NewClientWithSharedKeyCredential creates a Client signing requests with the
// account's shared key.
func NewClientWithSharedKeyCredential(containerURL string, cred *SharedKeyCredential, options *ClientOptions) (*Client, error) {
	return newClient(containerURL, NewSharedKeyCredPolicy(cred), options)
}

// NewListBlobsHierarchyPager returns a pager over the container's blobs,
// requesting Arrow responses when o.UseArrowFormat is set and returning the
// same response type as container.Client.NewListBlobsHierarchyPager.
//
// If the service does not support Arrow listing (the feature not enabled on
// the account, or an HNS account) it answers with XML, which is decoded
// transparently - unless EndBefore is set, in which case the fetcher returns
// ErrEndBeforeXMLFallback (see there).
func (c *Client) NewListBlobsHierarchyPager(delimiter string, o *ListBlobsHierarchyOptions) *runtime.Pager[container.ListBlobsHierarchyResponse] {
	// Copy the options so advancing the Marker doesn't mutate the caller's struct.
	opts := ListBlobsHierarchyOptions{}
	if o != nil {
		opts = *o
	}
	useArrow := opts.UseArrowFormat != nil && *opts.UseArrowFormat
	return runtime.NewPager(runtime.PagingHandler[container.ListBlobsHierarchyResponse]{
		More: func(page container.ListBlobsHierarchyResponse) bool {
			return page.NextMarker != nil && len(*page.NextMarker) > 0
		},
		Fetcher: func(ctx context.Context, page *container.ListBlobsHierarchyResponse) (container.ListBlobsHierarchyResponse, error) {
			if page != nil {
				opts.Marker = page.NextMarker
			}
			req, err := c.listCreateRequest(ctx, delimiter, &opts, useArrow)
			if err != nil {
				return container.ListBlobsHierarchyResponse{}, err
			}
			resp, err := c.azClient.Pipeline().Do(req)
			if err != nil {
				return container.ListBlobsHierarchyResponse{}, err
			}
			if !runtime.HasStatusCode(resp, http.StatusOK) {
				return container.ListBlobsHierarchyResponse{}, runtime.NewResponseError(resp)
			}
			if useArrow && resp.Header.Get("Content-Type") == ArrowContentType {
				return HandleHierarchyListResponse(resp)
			}
			if opts.EndBefore != nil {
				return container.ListBlobsHierarchyResponse{}, ErrEndBeforeXMLFallback
			}
			return handleXMLResponse(resp)
		},
	})
}

// listCreateRequest builds the ListBlobs request, mirroring the generated
// request builders in the SDK.
func (c *Client) listCreateRequest(ctx context.Context, delimiter string, opts *ListBlobsHierarchyOptions, useArrow bool) (*policy.Request, error) {
	req, err := runtime.NewRequest(ctx, http.MethodGet, c.endpoint)
	if err != nil {
		return nil, err
	}
	// Reading the existing query first preserves any SAS token in the endpoint.
	reqQP := req.Raw().URL.Query()
	reqQP.Set("comp", "list")
	reqQP.Set("restype", "container")
	reqQP.Set("delimiter", delimiter)
	if include := formatInclude(opts.Include); include != "" {
		reqQP.Set("include", include)
	}
	if opts.Marker != nil {
		reqQP.Set("marker", *opts.Marker)
	}
	if opts.MaxResults != nil {
		reqQP.Set("maxresults", strconv.FormatInt(int64(*opts.MaxResults), 10))
	}
	if opts.Prefix != nil {
		reqQP.Set("prefix", *opts.Prefix)
	}
	if opts.StartFrom != nil {
		reqQP.Set("startFrom", *opts.StartFrom)
	}
	if useArrow && opts.EndBefore != nil {
		reqQP.Set("endBefore", *opts.EndBefore)
	}
	req.Raw().URL.RawQuery = strings.ReplaceAll(reqQP.Encode(), "+", "%20")
	if useArrow {
		// Stream the Arrow body rather than buffering it in the pipeline.
		runtime.SkipBodyDownload(req)
		req.Raw().Header["Accept"] = []string{ArrowAcceptHeader}
	} else {
		req.Raw().Header["Accept"] = []string{"application/xml"}
	}
	req.Raw().Header["x-ms-version"] = []string{serviceVersion}
	return req, nil
}

// handleXMLResponse decodes an XML ListBlobs response, mirroring the SDK's
// generated ListBlobHierarchySegmentHandleResponse.
func handleXMLResponse(resp *http.Response) (container.ListBlobsHierarchyResponse, error) {
	result := container.ListBlobsHierarchyResponse{}
	extractResponseHeaders(resp, &result.ClientRequestID, &result.ContentType, &result.Date, &result.RequestID, &result.Version)
	if err := runtime.UnmarshalAsXML(resp, &result.ListBlobsHierarchySegmentResponse); err != nil {
		return container.ListBlobsHierarchyResponse{}, err
	}
	return result, nil
}

// formatInclude renders the include datasets as the comma separated query
// parameter value, in the same order as the SDK's unexported
// ListBlobsInclude.format.
func formatInclude(l container.ListBlobsInclude) string {
	var include []string
	if l.Copy {
		include = append(include, "copy")
	}
	if l.Deleted {
		include = append(include, "deleted")
	}
	if l.DeletedWithVersions {
		include = append(include, "deletedwithversions")
	}
	if l.ImmutabilityPolicy {
		include = append(include, "immutabilitypolicy")
	}
	if l.LegalHold {
		include = append(include, "legalhold")
	}
	if l.Metadata {
		include = append(include, "metadata")
	}
	if l.Snapshots {
		include = append(include, "snapshots")
	}
	if l.Tags {
		include = append(include, "tags")
	}
	if l.UncommittedBlobs {
		include = append(include, "uncommittedblobs")
	}
	if l.Versions {
		include = append(include, "versions")
	}
	if l.Permissions {
		include = append(include, "permissions")
	}
	return strings.Join(include, ",")
}
