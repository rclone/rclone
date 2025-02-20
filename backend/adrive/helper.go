package adrive

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/rclone/rclone/backend/adrive/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
)

// Client contains the info for the Aliyun Drive API
type AdriveClient struct {
	mu           sync.RWMutex // Protecting read/writes
	c            *rest.Client // The REST client
	rootURL      string       // API root URL
	errorHandler func(resp *http.Response) error
	pacer        *fs.Pacer // To pace the API calls
}

// NewClient takes an http.Client and makes a new api instance
func NewAdriveClient(c *http.Client, rootURL string) *AdriveClient {
	client := &AdriveClient{
		c:       rest.NewClient(c),
		rootURL: rootURL,
	}
	client.c.SetErrorHandler(errorHandler)
	client.c.SetRoot(rootURL)

	// Create a pacer using rclone's default exponential backoff
	client.pacer = fs.NewPacer(
		context.Background(),
		pacer.NewDefault(
			pacer.MinSleep(minSleep),
			pacer.MaxSleep(maxSleep),
			pacer.DecayConstant(decayConstant),
		),
	)

	return client
}

// Call makes a call to the API using the params passed in
func (c *AdriveClient) Call(ctx context.Context, opts *rest.Opts) (resp *http.Response, err error) {
	return c.CallWithPacer(ctx, opts, c.pacer)
}

// CallWithPacer makes a call to the API using the params passed in using the pacer passed in
func (c *AdriveClient) CallWithPacer(ctx context.Context, opts *rest.Opts, pacer *fs.Pacer) (resp *http.Response, err error) {
	err = pacer.Call(func() (bool, error) {
		resp, err = c.c.Call(ctx, opts)
		return shouldRetry(ctx, resp, err)
	})
	return resp, err
}

// CallJSON makes an API call and decodes the JSON return packet into response
func (c *AdriveClient) CallJSON(ctx context.Context, opts *rest.Opts, request interface{}, response interface{}) (resp *http.Response, err error) {
	return c.CallJSONWithPacer(ctx, opts, c.pacer, request, response)
}

// CallJSONWithPacer makes an API call and decodes the JSON return packet into response using the pacer passed in
func (c *AdriveClient) CallJSONWithPacer(ctx context.Context, opts *rest.Opts, pacer *fs.Pacer, request interface{}, response interface{}) (resp *http.Response, err error) {
	err = pacer.Call(func() (bool, error) {
		resp, err = c.c.CallJSON(ctx, opts, request, response)
		return shouldRetry(ctx, resp, err)
	})
	return resp, err
}

var retryErrorCodes = []int{
	403,
	404,
	429, // Too Many Requests
	500, // Internal Server Error
	502, // Bad Gateway
	503, // Service Unavailable
	504, // Gateway Timeout
	509, // Bandwidth Limit Exceeded
}

// shouldRetry returns true if err is nil, or if it's a retryable error
func shouldRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if err != nil {
		// Check for context cancellation first
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		// Retry network errors
		if fserrors.ShouldRetry(err) {
			return true, err
		}
		return false, err
	}

	if resp == nil {
		return false, nil
	}

	// Check if we have an API error
	if resp.StatusCode >= 400 {
		apiErr := new(api.Error)
		decodeErr := rest.DecodeJSON(resp, &apiErr)
		if decodeErr != nil {
			fs.Debugf(nil, "Failed to decode error response: %v", decodeErr)
			// If we can't decode the error, retry server errors
			return resp.StatusCode >= 500, fmt.Errorf("HTTP error %s", resp.Status)
		}
		return apiErr.ShouldRetry(1), apiErr
	}

	return false, nil
}

// errorHandler parses a non 2xx error response into an error
func errorHandler(resp *http.Response) error {
	// Decode error response
	apiErr := new(api.Error)
	err := rest.DecodeJSON(resp, &apiErr)
	if err != nil {
		fs.Debugf(nil, "Failed to decode error response: %v", err)
		// If we can't decode the error response, create a basic error
		apiErr.Code = resp.StatusCode
		apiErr.Message = resp.Status
		return apiErr
	}

	// Ensure we have an error code and message
	if apiErr.Code == 0 {
		apiErr.Code = resp.StatusCode
	}
	if apiErr.Message == "" {
		apiErr.Message = resp.Status
	}

	// Add response body as details if present
	if body, err := io.ReadAll(resp.Body); err == nil && len(body) > 0 {
		apiErr.Details = string(body)
	}

	return apiErr
}
