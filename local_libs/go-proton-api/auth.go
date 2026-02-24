package proton

import (
	"context"

	"github.com/go-resty/resty/v2"
)

func (c *Client) Auth2FA(ctx context.Context, req Auth2FAReq) error {
	return c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetBody(req).Post("/auth/v4/2fa")
	})
}

func (c *Client) AuthDelete(ctx context.Context) error {
	return c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.Delete("/auth/v4")
	})
}

func (c *Client) AuthSessions(ctx context.Context) ([]AuthSession, error) {
	var res struct {
		Sessions []AuthSession
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetResult(&res).Get("/auth/v4/sessions")
	}); err != nil {
		return nil, err
	}

	return res.Sessions, nil
}

func (c *Client) AuthRevoke(ctx context.Context, authUID string) error {
	return c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.Delete("/auth/v4/sessions/" + authUID)
	})
}

func (c *Client) AuthRevokeAll(ctx context.Context) error {
	return c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.Delete("/auth/v4/sessions")
	})
}
