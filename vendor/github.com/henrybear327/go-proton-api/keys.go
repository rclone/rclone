package proton

import (
	"context"

	"github.com/go-resty/resty/v2"
)

func (c *Client) GetPublicKeys(ctx context.Context, address string) (PublicKeys, RecipientType, error) {
	var res struct {
		Keys          []PublicKey
		RecipientType RecipientType
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetResult(&res).SetQueryParam("Email", address).Get("/core/v4/keys")
	}); err != nil {
		return nil, RecipientTypeExternal, err
	}

	return res.Keys, res.RecipientType, nil
}

func (c *Client) CreateAddressKey(ctx context.Context, req CreateAddressKeyReq) (Key, error) {
	var res struct {
		Key Key
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetBody(req).SetResult(&res).Post("/core/v4/keys/address")
	}); err != nil {
		return Key{}, err
	}

	return res.Key, nil
}

func (c *Client) CreateLegacyAddressKey(ctx context.Context, req CreateAddressKeyReq) (Key, error) {
	var res struct {
		Key Key
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetBody(req).SetResult(&res).Post("/core/v4/keys")
	}); err != nil {
		return Key{}, err
	}

	return res.Key, nil
}

func (c *Client) MakeAddressKeyPrimary(ctx context.Context, keyID string, keyList KeyList) error {
	return c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetBody(struct{ SignedKeyList KeyList }{SignedKeyList: keyList}).Put("/core/v4/keys/" + keyID + "/primary")
	})
}

func (c *Client) DeleteAddressKey(ctx context.Context, keyID string, keyList KeyList) error {
	return c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetBody(struct{ SignedKeyList KeyList }{SignedKeyList: keyList}).Post("/core/v4/keys/" + keyID + "/delete")
	})
}
