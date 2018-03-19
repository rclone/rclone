// Copyright 2018 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// AUTO-GENERATED CODE. DO NOT EDIT.

package dlp

import (
	"math"
	"time"

	"cloud.google.com/go/internal/version"
	gax "github.com/googleapis/gax-go"
	"golang.org/x/net/context"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/api/transport"
	dlppb "google.golang.org/genproto/googleapis/privacy/dlp/v2beta2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
)

// CallOptions contains the retry settings for each method of Client.
type CallOptions struct {
	InspectContent           []gax.CallOption
	RedactImage              []gax.CallOption
	DeidentifyContent        []gax.CallOption
	ReidentifyContent        []gax.CallOption
	InspectDataSource        []gax.CallOption
	AnalyzeDataSourceRisk    []gax.CallOption
	ListInfoTypes            []gax.CallOption
	CreateInspectTemplate    []gax.CallOption
	UpdateInspectTemplate    []gax.CallOption
	GetInspectTemplate       []gax.CallOption
	ListInspectTemplates     []gax.CallOption
	DeleteInspectTemplate    []gax.CallOption
	CreateDeidentifyTemplate []gax.CallOption
	UpdateDeidentifyTemplate []gax.CallOption
	GetDeidentifyTemplate    []gax.CallOption
	ListDeidentifyTemplates  []gax.CallOption
	DeleteDeidentifyTemplate []gax.CallOption
	ListDlpJobs              []gax.CallOption
	GetDlpJob                []gax.CallOption
	DeleteDlpJob             []gax.CallOption
	CancelDlpJob             []gax.CallOption
}

func defaultClientOptions() []option.ClientOption {
	return []option.ClientOption{
		option.WithEndpoint("dlp.googleapis.com:443"),
		option.WithScopes(DefaultAuthScopes()...),
	}
}

func defaultCallOptions() *CallOptions {
	retry := map[[2]string][]gax.CallOption{
		{"default", "idempotent"}: {
			gax.WithRetry(func() gax.Retryer {
				return gax.OnCodes([]codes.Code{
					codes.DeadlineExceeded,
					codes.Unavailable,
				}, gax.Backoff{
					Initial:    100 * time.Millisecond,
					Max:        60000 * time.Millisecond,
					Multiplier: 1.3,
				})
			}),
		},
	}
	return &CallOptions{
		InspectContent:           retry[[2]string{"default", "idempotent"}],
		RedactImage:              retry[[2]string{"default", "idempotent"}],
		DeidentifyContent:        retry[[2]string{"default", "idempotent"}],
		ReidentifyContent:        retry[[2]string{"default", "idempotent"}],
		InspectDataSource:        retry[[2]string{"default", "non_idempotent"}],
		AnalyzeDataSourceRisk:    retry[[2]string{"default", "non_idempotent"}],
		ListInfoTypes:            retry[[2]string{"default", "idempotent"}],
		CreateInspectTemplate:    retry[[2]string{"default", "non_idempotent"}],
		UpdateInspectTemplate:    retry[[2]string{"default", "non_idempotent"}],
		GetInspectTemplate:       retry[[2]string{"default", "idempotent"}],
		ListInspectTemplates:     retry[[2]string{"default", "idempotent"}],
		DeleteInspectTemplate:    retry[[2]string{"default", "idempotent"}],
		CreateDeidentifyTemplate: retry[[2]string{"default", "non_idempotent"}],
		UpdateDeidentifyTemplate: retry[[2]string{"default", "non_idempotent"}],
		GetDeidentifyTemplate:    retry[[2]string{"default", "idempotent"}],
		ListDeidentifyTemplates:  retry[[2]string{"default", "idempotent"}],
		DeleteDeidentifyTemplate: retry[[2]string{"default", "idempotent"}],
		ListDlpJobs:              retry[[2]string{"default", "idempotent"}],
		GetDlpJob:                retry[[2]string{"default", "idempotent"}],
		DeleteDlpJob:             retry[[2]string{"default", "idempotent"}],
		CancelDlpJob:             retry[[2]string{"default", "non_idempotent"}],
	}
}

// Client is a client for interacting with DLP API.
type Client struct {
	// The connection to the service.
	conn *grpc.ClientConn

	// The gRPC API client.
	client dlppb.DlpServiceClient

	// The call options for this service.
	CallOptions *CallOptions

	// The x-goog-* metadata to be sent with each request.
	xGoogMetadata metadata.MD
}

// NewClient creates a new dlp service client.
//
// The DLP API is a service that allows clients
// to detect the presence of Personally Identifiable Information (PII) and other
// privacy-sensitive data in user-supplied, unstructured data streams, like text
// blocks or images.
// The service also includes methods for sensitive data redaction and
// scheduling of data scans on Google Cloud Platform based data sets.
func NewClient(ctx context.Context, opts ...option.ClientOption) (*Client, error) {
	conn, err := transport.DialGRPC(ctx, append(defaultClientOptions(), opts...)...)
	if err != nil {
		return nil, err
	}
	c := &Client{
		conn:        conn,
		CallOptions: defaultCallOptions(),

		client: dlppb.NewDlpServiceClient(conn),
	}
	c.setGoogleClientInfo()
	return c, nil
}

// Connection returns the client's connection to the API service.
func (c *Client) Connection() *grpc.ClientConn {
	return c.conn
}

// Close closes the connection to the API service. The user should invoke this when
// the client is no longer required.
func (c *Client) Close() error {
	return c.conn.Close()
}

// setGoogleClientInfo sets the name and version of the application in
// the `x-goog-api-client` header passed on each request. Intended for
// use by Google-written clients.
func (c *Client) setGoogleClientInfo(keyval ...string) {
	kv := append([]string{"gl-go", version.Go()}, keyval...)
	kv = append(kv, "gapic", version.Repo, "gax", gax.Version, "grpc", grpc.Version)
	c.xGoogMetadata = metadata.Pairs("x-goog-api-client", gax.XGoogHeader(kv...))
}

// InspectContent finds potentially sensitive info in content.
// This method has limits on input size, processing time, and output size.
// How-to guide for text (at /dlp/docs/inspecting-text), How-to guide for
// images (at /dlp/docs/inspecting-images)
func (c *Client) InspectContent(ctx context.Context, req *dlppb.InspectContentRequest, opts ...gax.CallOption) (*dlppb.InspectContentResponse, error) {
	ctx = insertMetadata(ctx, c.xGoogMetadata)
	opts = append(c.CallOptions.InspectContent[0:len(c.CallOptions.InspectContent):len(c.CallOptions.InspectContent)], opts...)
	var resp *dlppb.InspectContentResponse
	err := gax.Invoke(ctx, func(ctx context.Context, settings gax.CallSettings) error {
		var err error
		resp, err = c.client.InspectContent(ctx, req, settings.GRPC...)
		return err
	}, opts...)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// RedactImage redacts potentially sensitive info from an image.
// This method has limits on input size, processing time, and output size.
// How-to guide (at /dlp/docs/redacting-sensitive-data-images)
func (c *Client) RedactImage(ctx context.Context, req *dlppb.RedactImageRequest, opts ...gax.CallOption) (*dlppb.RedactImageResponse, error) {
	ctx = insertMetadata(ctx, c.xGoogMetadata)
	opts = append(c.CallOptions.RedactImage[0:len(c.CallOptions.RedactImage):len(c.CallOptions.RedactImage)], opts...)
	var resp *dlppb.RedactImageResponse
	err := gax.Invoke(ctx, func(ctx context.Context, settings gax.CallSettings) error {
		var err error
		resp, err = c.client.RedactImage(ctx, req, settings.GRPC...)
		return err
	}, opts...)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// DeidentifyContent de-identifies potentially sensitive info from a ContentItem.
// This method has limits on input size and output size.
// How-to guide (at /dlp/docs/deidentify-sensitive-data)
func (c *Client) DeidentifyContent(ctx context.Context, req *dlppb.DeidentifyContentRequest, opts ...gax.CallOption) (*dlppb.DeidentifyContentResponse, error) {
	ctx = insertMetadata(ctx, c.xGoogMetadata)
	opts = append(c.CallOptions.DeidentifyContent[0:len(c.CallOptions.DeidentifyContent):len(c.CallOptions.DeidentifyContent)], opts...)
	var resp *dlppb.DeidentifyContentResponse
	err := gax.Invoke(ctx, func(ctx context.Context, settings gax.CallSettings) error {
		var err error
		resp, err = c.client.DeidentifyContent(ctx, req, settings.GRPC...)
		return err
	}, opts...)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// ReidentifyContent re-identify content that has been de-identified.
func (c *Client) ReidentifyContent(ctx context.Context, req *dlppb.ReidentifyContentRequest, opts ...gax.CallOption) (*dlppb.ReidentifyContentResponse, error) {
	ctx = insertMetadata(ctx, c.xGoogMetadata)
	opts = append(c.CallOptions.ReidentifyContent[0:len(c.CallOptions.ReidentifyContent):len(c.CallOptions.ReidentifyContent)], opts...)
	var resp *dlppb.ReidentifyContentResponse
	err := gax.Invoke(ctx, func(ctx context.Context, settings gax.CallSettings) error {
		var err error
		resp, err = c.client.ReidentifyContent(ctx, req, settings.GRPC...)
		return err
	}, opts...)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// InspectDataSource schedules a job scanning content in a Google Cloud Platform data
// repository. How-to guide (at /dlp/docs/inspecting-storage)
func (c *Client) InspectDataSource(ctx context.Context, req *dlppb.InspectDataSourceRequest, opts ...gax.CallOption) (*dlppb.DlpJob, error) {
	ctx = insertMetadata(ctx, c.xGoogMetadata)
	opts = append(c.CallOptions.InspectDataSource[0:len(c.CallOptions.InspectDataSource):len(c.CallOptions.InspectDataSource)], opts...)
	var resp *dlppb.DlpJob
	err := gax.Invoke(ctx, func(ctx context.Context, settings gax.CallSettings) error {
		var err error
		resp, err = c.client.InspectDataSource(ctx, req, settings.GRPC...)
		return err
	}, opts...)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// AnalyzeDataSourceRisk schedules a job to compute risk analysis metrics over content in a Google
// Cloud Platform repository. [How-to guide}(/dlp/docs/compute-risk-analysis)
func (c *Client) AnalyzeDataSourceRisk(ctx context.Context, req *dlppb.AnalyzeDataSourceRiskRequest, opts ...gax.CallOption) (*dlppb.DlpJob, error) {
	ctx = insertMetadata(ctx, c.xGoogMetadata)
	opts = append(c.CallOptions.AnalyzeDataSourceRisk[0:len(c.CallOptions.AnalyzeDataSourceRisk):len(c.CallOptions.AnalyzeDataSourceRisk)], opts...)
	var resp *dlppb.DlpJob
	err := gax.Invoke(ctx, func(ctx context.Context, settings gax.CallSettings) error {
		var err error
		resp, err = c.client.AnalyzeDataSourceRisk(ctx, req, settings.GRPC...)
		return err
	}, opts...)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// ListInfoTypes returns sensitive information types DLP supports.
func (c *Client) ListInfoTypes(ctx context.Context, req *dlppb.ListInfoTypesRequest, opts ...gax.CallOption) (*dlppb.ListInfoTypesResponse, error) {
	ctx = insertMetadata(ctx, c.xGoogMetadata)
	opts = append(c.CallOptions.ListInfoTypes[0:len(c.CallOptions.ListInfoTypes):len(c.CallOptions.ListInfoTypes)], opts...)
	var resp *dlppb.ListInfoTypesResponse
	err := gax.Invoke(ctx, func(ctx context.Context, settings gax.CallSettings) error {
		var err error
		resp, err = c.client.ListInfoTypes(ctx, req, settings.GRPC...)
		return err
	}, opts...)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// CreateInspectTemplate creates an inspect template for re-using frequently used configuration
// for inspecting content, images, and storage.
func (c *Client) CreateInspectTemplate(ctx context.Context, req *dlppb.CreateInspectTemplateRequest, opts ...gax.CallOption) (*dlppb.InspectTemplate, error) {
	ctx = insertMetadata(ctx, c.xGoogMetadata)
	opts = append(c.CallOptions.CreateInspectTemplate[0:len(c.CallOptions.CreateInspectTemplate):len(c.CallOptions.CreateInspectTemplate)], opts...)
	var resp *dlppb.InspectTemplate
	err := gax.Invoke(ctx, func(ctx context.Context, settings gax.CallSettings) error {
		var err error
		resp, err = c.client.CreateInspectTemplate(ctx, req, settings.GRPC...)
		return err
	}, opts...)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// UpdateInspectTemplate updates the inspect template.
func (c *Client) UpdateInspectTemplate(ctx context.Context, req *dlppb.UpdateInspectTemplateRequest, opts ...gax.CallOption) (*dlppb.InspectTemplate, error) {
	ctx = insertMetadata(ctx, c.xGoogMetadata)
	opts = append(c.CallOptions.UpdateInspectTemplate[0:len(c.CallOptions.UpdateInspectTemplate):len(c.CallOptions.UpdateInspectTemplate)], opts...)
	var resp *dlppb.InspectTemplate
	err := gax.Invoke(ctx, func(ctx context.Context, settings gax.CallSettings) error {
		var err error
		resp, err = c.client.UpdateInspectTemplate(ctx, req, settings.GRPC...)
		return err
	}, opts...)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// GetInspectTemplate gets an inspect template.
func (c *Client) GetInspectTemplate(ctx context.Context, req *dlppb.GetInspectTemplateRequest, opts ...gax.CallOption) (*dlppb.InspectTemplate, error) {
	ctx = insertMetadata(ctx, c.xGoogMetadata)
	opts = append(c.CallOptions.GetInspectTemplate[0:len(c.CallOptions.GetInspectTemplate):len(c.CallOptions.GetInspectTemplate)], opts...)
	var resp *dlppb.InspectTemplate
	err := gax.Invoke(ctx, func(ctx context.Context, settings gax.CallSettings) error {
		var err error
		resp, err = c.client.GetInspectTemplate(ctx, req, settings.GRPC...)
		return err
	}, opts...)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// ListInspectTemplates lists inspect templates.
func (c *Client) ListInspectTemplates(ctx context.Context, req *dlppb.ListInspectTemplatesRequest, opts ...gax.CallOption) *InspectTemplateIterator {
	ctx = insertMetadata(ctx, c.xGoogMetadata)
	opts = append(c.CallOptions.ListInspectTemplates[0:len(c.CallOptions.ListInspectTemplates):len(c.CallOptions.ListInspectTemplates)], opts...)
	it := &InspectTemplateIterator{}
	it.InternalFetch = func(pageSize int, pageToken string) ([]*dlppb.InspectTemplate, string, error) {
		var resp *dlppb.ListInspectTemplatesResponse
		req.PageToken = pageToken
		if pageSize > math.MaxInt32 {
			req.PageSize = math.MaxInt32
		} else {
			req.PageSize = int32(pageSize)
		}
		err := gax.Invoke(ctx, func(ctx context.Context, settings gax.CallSettings) error {
			var err error
			resp, err = c.client.ListInspectTemplates(ctx, req, settings.GRPC...)
			return err
		}, opts...)
		if err != nil {
			return nil, "", err
		}
		return resp.InspectTemplates, resp.NextPageToken, nil
	}
	fetch := func(pageSize int, pageToken string) (string, error) {
		items, nextPageToken, err := it.InternalFetch(pageSize, pageToken)
		if err != nil {
			return "", err
		}
		it.items = append(it.items, items...)
		return nextPageToken, nil
	}
	it.pageInfo, it.nextFunc = iterator.NewPageInfo(fetch, it.bufLen, it.takeBuf)
	return it
}

// DeleteInspectTemplate deletes inspect templates.
func (c *Client) DeleteInspectTemplate(ctx context.Context, req *dlppb.DeleteInspectTemplateRequest, opts ...gax.CallOption) error {
	ctx = insertMetadata(ctx, c.xGoogMetadata)
	opts = append(c.CallOptions.DeleteInspectTemplate[0:len(c.CallOptions.DeleteInspectTemplate):len(c.CallOptions.DeleteInspectTemplate)], opts...)
	err := gax.Invoke(ctx, func(ctx context.Context, settings gax.CallSettings) error {
		var err error
		_, err = c.client.DeleteInspectTemplate(ctx, req, settings.GRPC...)
		return err
	}, opts...)
	return err
}

// CreateDeidentifyTemplate creates an Deidentify template for re-using frequently used configuration
// for Deidentifying content, images, and storage.
func (c *Client) CreateDeidentifyTemplate(ctx context.Context, req *dlppb.CreateDeidentifyTemplateRequest, opts ...gax.CallOption) (*dlppb.DeidentifyTemplate, error) {
	ctx = insertMetadata(ctx, c.xGoogMetadata)
	opts = append(c.CallOptions.CreateDeidentifyTemplate[0:len(c.CallOptions.CreateDeidentifyTemplate):len(c.CallOptions.CreateDeidentifyTemplate)], opts...)
	var resp *dlppb.DeidentifyTemplate
	err := gax.Invoke(ctx, func(ctx context.Context, settings gax.CallSettings) error {
		var err error
		resp, err = c.client.CreateDeidentifyTemplate(ctx, req, settings.GRPC...)
		return err
	}, opts...)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// UpdateDeidentifyTemplate updates the inspect template.
func (c *Client) UpdateDeidentifyTemplate(ctx context.Context, req *dlppb.UpdateDeidentifyTemplateRequest, opts ...gax.CallOption) (*dlppb.DeidentifyTemplate, error) {
	ctx = insertMetadata(ctx, c.xGoogMetadata)
	opts = append(c.CallOptions.UpdateDeidentifyTemplate[0:len(c.CallOptions.UpdateDeidentifyTemplate):len(c.CallOptions.UpdateDeidentifyTemplate)], opts...)
	var resp *dlppb.DeidentifyTemplate
	err := gax.Invoke(ctx, func(ctx context.Context, settings gax.CallSettings) error {
		var err error
		resp, err = c.client.UpdateDeidentifyTemplate(ctx, req, settings.GRPC...)
		return err
	}, opts...)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// GetDeidentifyTemplate gets an inspect template.
func (c *Client) GetDeidentifyTemplate(ctx context.Context, req *dlppb.GetDeidentifyTemplateRequest, opts ...gax.CallOption) (*dlppb.DeidentifyTemplate, error) {
	ctx = insertMetadata(ctx, c.xGoogMetadata)
	opts = append(c.CallOptions.GetDeidentifyTemplate[0:len(c.CallOptions.GetDeidentifyTemplate):len(c.CallOptions.GetDeidentifyTemplate)], opts...)
	var resp *dlppb.DeidentifyTemplate
	err := gax.Invoke(ctx, func(ctx context.Context, settings gax.CallSettings) error {
		var err error
		resp, err = c.client.GetDeidentifyTemplate(ctx, req, settings.GRPC...)
		return err
	}, opts...)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// ListDeidentifyTemplates lists inspect templates.
func (c *Client) ListDeidentifyTemplates(ctx context.Context, req *dlppb.ListDeidentifyTemplatesRequest, opts ...gax.CallOption) *DeidentifyTemplateIterator {
	ctx = insertMetadata(ctx, c.xGoogMetadata)
	opts = append(c.CallOptions.ListDeidentifyTemplates[0:len(c.CallOptions.ListDeidentifyTemplates):len(c.CallOptions.ListDeidentifyTemplates)], opts...)
	it := &DeidentifyTemplateIterator{}
	it.InternalFetch = func(pageSize int, pageToken string) ([]*dlppb.DeidentifyTemplate, string, error) {
		var resp *dlppb.ListDeidentifyTemplatesResponse
		req.PageToken = pageToken
		if pageSize > math.MaxInt32 {
			req.PageSize = math.MaxInt32
		} else {
			req.PageSize = int32(pageSize)
		}
		err := gax.Invoke(ctx, func(ctx context.Context, settings gax.CallSettings) error {
			var err error
			resp, err = c.client.ListDeidentifyTemplates(ctx, req, settings.GRPC...)
			return err
		}, opts...)
		if err != nil {
			return nil, "", err
		}
		return resp.DeidentifyTemplates, resp.NextPageToken, nil
	}
	fetch := func(pageSize int, pageToken string) (string, error) {
		items, nextPageToken, err := it.InternalFetch(pageSize, pageToken)
		if err != nil {
			return "", err
		}
		it.items = append(it.items, items...)
		return nextPageToken, nil
	}
	it.pageInfo, it.nextFunc = iterator.NewPageInfo(fetch, it.bufLen, it.takeBuf)
	return it
}

// DeleteDeidentifyTemplate deletes inspect templates.
func (c *Client) DeleteDeidentifyTemplate(ctx context.Context, req *dlppb.DeleteDeidentifyTemplateRequest, opts ...gax.CallOption) error {
	ctx = insertMetadata(ctx, c.xGoogMetadata)
	opts = append(c.CallOptions.DeleteDeidentifyTemplate[0:len(c.CallOptions.DeleteDeidentifyTemplate):len(c.CallOptions.DeleteDeidentifyTemplate)], opts...)
	err := gax.Invoke(ctx, func(ctx context.Context, settings gax.CallSettings) error {
		var err error
		_, err = c.client.DeleteDeidentifyTemplate(ctx, req, settings.GRPC...)
		return err
	}, opts...)
	return err
}

// ListDlpJobs lists DlpJobs that match the specified filter in the request.
func (c *Client) ListDlpJobs(ctx context.Context, req *dlppb.ListDlpJobsRequest, opts ...gax.CallOption) *DlpJobIterator {
	ctx = insertMetadata(ctx, c.xGoogMetadata)
	opts = append(c.CallOptions.ListDlpJobs[0:len(c.CallOptions.ListDlpJobs):len(c.CallOptions.ListDlpJobs)], opts...)
	it := &DlpJobIterator{}
	it.InternalFetch = func(pageSize int, pageToken string) ([]*dlppb.DlpJob, string, error) {
		var resp *dlppb.ListDlpJobsResponse
		req.PageToken = pageToken
		if pageSize > math.MaxInt32 {
			req.PageSize = math.MaxInt32
		} else {
			req.PageSize = int32(pageSize)
		}
		err := gax.Invoke(ctx, func(ctx context.Context, settings gax.CallSettings) error {
			var err error
			resp, err = c.client.ListDlpJobs(ctx, req, settings.GRPC...)
			return err
		}, opts...)
		if err != nil {
			return nil, "", err
		}
		return resp.Jobs, resp.NextPageToken, nil
	}
	fetch := func(pageSize int, pageToken string) (string, error) {
		items, nextPageToken, err := it.InternalFetch(pageSize, pageToken)
		if err != nil {
			return "", err
		}
		it.items = append(it.items, items...)
		return nextPageToken, nil
	}
	it.pageInfo, it.nextFunc = iterator.NewPageInfo(fetch, it.bufLen, it.takeBuf)
	return it
}

// GetDlpJob gets the latest state of a long-running DlpJob.
func (c *Client) GetDlpJob(ctx context.Context, req *dlppb.GetDlpJobRequest, opts ...gax.CallOption) (*dlppb.DlpJob, error) {
	ctx = insertMetadata(ctx, c.xGoogMetadata)
	opts = append(c.CallOptions.GetDlpJob[0:len(c.CallOptions.GetDlpJob):len(c.CallOptions.GetDlpJob)], opts...)
	var resp *dlppb.DlpJob
	err := gax.Invoke(ctx, func(ctx context.Context, settings gax.CallSettings) error {
		var err error
		resp, err = c.client.GetDlpJob(ctx, req, settings.GRPC...)
		return err
	}, opts...)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// DeleteDlpJob deletes a long-running DlpJob. This method indicates that the client is
// no longer interested in the DlpJob result. The job will be cancelled if
// possible.
func (c *Client) DeleteDlpJob(ctx context.Context, req *dlppb.DeleteDlpJobRequest, opts ...gax.CallOption) error {
	ctx = insertMetadata(ctx, c.xGoogMetadata)
	opts = append(c.CallOptions.DeleteDlpJob[0:len(c.CallOptions.DeleteDlpJob):len(c.CallOptions.DeleteDlpJob)], opts...)
	err := gax.Invoke(ctx, func(ctx context.Context, settings gax.CallSettings) error {
		var err error
		_, err = c.client.DeleteDlpJob(ctx, req, settings.GRPC...)
		return err
	}, opts...)
	return err
}

// CancelDlpJob starts asynchronous cancellation on a long-running DlpJob.  The server
// makes a best effort to cancel the DlpJob, but success is not
// guaranteed.
func (c *Client) CancelDlpJob(ctx context.Context, req *dlppb.CancelDlpJobRequest, opts ...gax.CallOption) error {
	ctx = insertMetadata(ctx, c.xGoogMetadata)
	opts = append(c.CallOptions.CancelDlpJob[0:len(c.CallOptions.CancelDlpJob):len(c.CallOptions.CancelDlpJob)], opts...)
	err := gax.Invoke(ctx, func(ctx context.Context, settings gax.CallSettings) error {
		var err error
		_, err = c.client.CancelDlpJob(ctx, req, settings.GRPC...)
		return err
	}, opts...)
	return err
}

// DeidentifyTemplateIterator manages a stream of *dlppb.DeidentifyTemplate.
type DeidentifyTemplateIterator struct {
	items    []*dlppb.DeidentifyTemplate
	pageInfo *iterator.PageInfo
	nextFunc func() error

	// InternalFetch is for use by the Google Cloud Libraries only.
	// It is not part of the stable interface of this package.
	//
	// InternalFetch returns results from a single call to the underlying RPC.
	// The number of results is no greater than pageSize.
	// If there are no more results, nextPageToken is empty and err is nil.
	InternalFetch func(pageSize int, pageToken string) (results []*dlppb.DeidentifyTemplate, nextPageToken string, err error)
}

// PageInfo supports pagination. See the google.golang.org/api/iterator package for details.
func (it *DeidentifyTemplateIterator) PageInfo() *iterator.PageInfo {
	return it.pageInfo
}

// Next returns the next result. Its second return value is iterator.Done if there are no more
// results. Once Next returns Done, all subsequent calls will return Done.
func (it *DeidentifyTemplateIterator) Next() (*dlppb.DeidentifyTemplate, error) {
	var item *dlppb.DeidentifyTemplate
	if err := it.nextFunc(); err != nil {
		return item, err
	}
	item = it.items[0]
	it.items = it.items[1:]
	return item, nil
}

func (it *DeidentifyTemplateIterator) bufLen() int {
	return len(it.items)
}

func (it *DeidentifyTemplateIterator) takeBuf() interface{} {
	b := it.items
	it.items = nil
	return b
}

// DlpJobIterator manages a stream of *dlppb.DlpJob.
type DlpJobIterator struct {
	items    []*dlppb.DlpJob
	pageInfo *iterator.PageInfo
	nextFunc func() error

	// InternalFetch is for use by the Google Cloud Libraries only.
	// It is not part of the stable interface of this package.
	//
	// InternalFetch returns results from a single call to the underlying RPC.
	// The number of results is no greater than pageSize.
	// If there are no more results, nextPageToken is empty and err is nil.
	InternalFetch func(pageSize int, pageToken string) (results []*dlppb.DlpJob, nextPageToken string, err error)
}

// PageInfo supports pagination. See the google.golang.org/api/iterator package for details.
func (it *DlpJobIterator) PageInfo() *iterator.PageInfo {
	return it.pageInfo
}

// Next returns the next result. Its second return value is iterator.Done if there are no more
// results. Once Next returns Done, all subsequent calls will return Done.
func (it *DlpJobIterator) Next() (*dlppb.DlpJob, error) {
	var item *dlppb.DlpJob
	if err := it.nextFunc(); err != nil {
		return item, err
	}
	item = it.items[0]
	it.items = it.items[1:]
	return item, nil
}

func (it *DlpJobIterator) bufLen() int {
	return len(it.items)
}

func (it *DlpJobIterator) takeBuf() interface{} {
	b := it.items
	it.items = nil
	return b
}

// InspectTemplateIterator manages a stream of *dlppb.InspectTemplate.
type InspectTemplateIterator struct {
	items    []*dlppb.InspectTemplate
	pageInfo *iterator.PageInfo
	nextFunc func() error

	// InternalFetch is for use by the Google Cloud Libraries only.
	// It is not part of the stable interface of this package.
	//
	// InternalFetch returns results from a single call to the underlying RPC.
	// The number of results is no greater than pageSize.
	// If there are no more results, nextPageToken is empty and err is nil.
	InternalFetch func(pageSize int, pageToken string) (results []*dlppb.InspectTemplate, nextPageToken string, err error)
}

// PageInfo supports pagination. See the google.golang.org/api/iterator package for details.
func (it *InspectTemplateIterator) PageInfo() *iterator.PageInfo {
	return it.pageInfo
}

// Next returns the next result. Its second return value is iterator.Done if there are no more
// results. Once Next returns Done, all subsequent calls will return Done.
func (it *InspectTemplateIterator) Next() (*dlppb.InspectTemplate, error) {
	var item *dlppb.InspectTemplate
	if err := it.nextFunc(); err != nil {
		return item, err
	}
	item = it.items[0]
	it.items = it.items[1:]
	return item, nil
}

func (it *InspectTemplateIterator) bufLen() int {
	return len(it.items)
}

func (it *InspectTemplateIterator) takeBuf() interface{} {
	b := it.items
	it.items = nil
	return b
}
