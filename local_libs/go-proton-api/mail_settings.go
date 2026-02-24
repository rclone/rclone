package proton

import (
	"context"

	"github.com/go-resty/resty/v2"
)

func (c *Client) GetMailSettings(ctx context.Context) (MailSettings, error) {
	var res struct {
		MailSettings MailSettings
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetResult(&res).Get("/mail/v4/settings")
	}); err != nil {
		return MailSettings{}, err
	}

	return res.MailSettings, nil
}

func (c *Client) SetDisplayName(ctx context.Context, req SetDisplayNameReq) (MailSettings, error) {
	var res struct {
		MailSettings MailSettings
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetBody(req).SetResult(&res).Put("/mail/v4/settings/display")
	}); err != nil {
		return MailSettings{}, err
	}

	return res.MailSettings, nil
}

func (c *Client) SetSignature(ctx context.Context, req SetSignatureReq) (MailSettings, error) {
	var res struct {
		MailSettings MailSettings
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetBody(req).SetResult(&res).Put("/mail/v4/settings/signature")
	}); err != nil {
		return MailSettings{}, err
	}

	return res.MailSettings, nil
}

func (c *Client) SetDraftMIMEType(ctx context.Context, req SetDraftMIMETypeReq) (MailSettings, error) {
	var res struct {
		MailSettings MailSettings
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetBody(req).SetResult(&res).Put("/mail/v4/settings/drafttype")
	}); err != nil {
		return MailSettings{}, err
	}

	return res.MailSettings, nil
}

func (c *Client) SetAttachPublicKey(ctx context.Context, req SetAttachPublicKeyReq) (MailSettings, error) {
	var res struct {
		MailSettings MailSettings
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetBody(req).SetResult(&res).Put("/mail/v4/settings/attachpublic")
	}); err != nil {
		return MailSettings{}, err
	}

	return res.MailSettings, nil
}

func (c *Client) SetSignExternalMessages(ctx context.Context, req SetSignExternalMessagesReq) (MailSettings, error) {
	var res struct {
		MailSettings MailSettings
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetBody(req).SetResult(&res).Put("/mail/v4/settings/sign")
	}); err != nil {
		return MailSettings{}, err
	}

	return res.MailSettings, nil
}

func (c *Client) SetDefaultPGPScheme(ctx context.Context, req SetDefaultPGPSchemeReq) (MailSettings, error) {
	var res struct {
		MailSettings MailSettings
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetBody(req).SetResult(&res).Put("/mail/v4/settings/pgpscheme")
	}); err != nil {
		return MailSettings{}, err
	}

	return res.MailSettings, nil
}
