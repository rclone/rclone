package xpan

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/rclone/rclone/backend/xpan/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/rest"
	"golang.org/x/time/rate"
)

const (
	xPanServerRootURL = "https://pan.baidu.com"
)

// rateLimiterClient a wrapper for rest.Client
// ccntrol API request rate
type rateLimiterClient struct {
	*rest.Client
	limiter *rate.Limiter
}

func (c *rateLimiterClient) CallJSON(ctx context.Context, opts *rest.Opts, request interface{}, response interface{}) (resp *http.Response, err error) {
	if err = c.rateLimit(ctx, opts); err != nil {
		return
	}
	return c.Client.CallJSON(ctx, opts, request, response)
}

func (c *rateLimiterClient) Call(ctx context.Context, opts *rest.Opts) (resp *http.Response, err error) {
	if err = c.rateLimit(ctx, opts); err != nil {
		return
	}
	return c.Client.Call(ctx, opts)
}

func (c *rateLimiterClient) rateLimit(ctx context.Context, opts *rest.Opts) (err error) {
	if err = c.limiter.Wait(ctx); err != nil {
		return
	}
	fs.Debugf(c, "Querys remain tokens: %f, burst: %d", c.limiter.Tokens(), c.limiter.Burst())
	fs.Debugf(c, "Call: %s, params: %s", opts.Path, opts.Parameters.Encode())
	return
}

func errorHandler(resp *http.Response) error {
	body, err := rest.ReadBody(resp)
	if err != nil {
		return fmt.Errorf("error reading error out of body: %w", err)
	}
	var response api.Response
	if err = json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("HTTP error %v (%v) returned body: %q", resp.StatusCode, resp.Status, body)
	}
	if response.ErrorNumber != 0 {
		return api.Err(response.ErrorNumber)
	}
	if response.ErrorCode != 0 {
		return api.Err(response.ErrorCode)
	}
	return fmt.Errorf("HTTP error %v (%v) returned body: %q", resp.StatusCode, resp.Status, body)
}

func newRatelimiterClient(httpClient *http.Client, queryPerMinute int) *rateLimiterClient {
	return &rateLimiterClient{
		Client:  rest.NewClient(httpClient).SetRoot(xPanServerRootURL).SetErrorHandler(errorHandler),
		limiter: rate.NewLimiter(rate.Limit(float64(queryPerMinute)/60.0), 16),
	}
}
