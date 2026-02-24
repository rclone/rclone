package proton

import (
	"context"

	"github.com/go-resty/resty/v2"
)

func (c *Client) ListShares(ctx context.Context, all bool) ([]ShareMetadata, error) {
	var res struct {
		Shares []ShareMetadata
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		if all {
			r.SetQueryParam("ShowAll", "1")
		}

		return r.SetResult(&res).Get("/drive/shares")
	}); err != nil {
		return nil, err
	}

	return res.Shares, nil
}

func (c *Client) GetShare(ctx context.Context, shareID string) (Share, error) {
	var res struct {
		Share
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetResult(&res).Get("/drive/shares/" + shareID)
	}); err != nil {
		return Share{}, err
	}

	return res.Share, nil
}
