package proton

import (
	"context"
	"github.com/go-resty/resty/v2"
)

func (c *Client) SendDataEvent(ctx context.Context, req SendStatsReq) error {
	return c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetBody(req).Post("/data/v1/stats")
	})
}

func (c *Client) SendDataEventMultiple(ctx context.Context, req SendStatsMultiReq) error {
	return c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetBody(req).Post("/data/v1/stats/multiple")
	})
}
