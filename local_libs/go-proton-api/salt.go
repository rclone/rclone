package proton

import (
	"context"

	"github.com/go-resty/resty/v2"
)

func (c *Client) GetSalts(ctx context.Context) (Salts, error) {
	var res struct {
		KeySalts []Salt
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetResult(&res).Get("/core/v4/keys/salts")
	}); err != nil {
		return nil, err
	}

	return res.KeySalts, nil
}
