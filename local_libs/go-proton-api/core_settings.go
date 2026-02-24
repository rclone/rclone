package proton

import (
	"context"

	"github.com/go-resty/resty/v2"
)

func (c *Client) GetUserSettings(ctx context.Context) (UserSettings, error) {
	var res struct {
		UserSettings UserSettings
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetResult(&res).Get("/core/v4/settings")
	}); err != nil {
		return UserSettings{}, err
	}

	return res.UserSettings, nil
}

func (c *Client) SetUserSettingsTelemetry(ctx context.Context, req SetTelemetryReq) (UserSettings, error) {
	var res struct {
		UserSettings UserSettings
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetBody(req).SetResult(&res).Put("/core/v4/settings/telemetry")
	}); err != nil {
		return UserSettings{}, err
	}

	return res.UserSettings, nil
}

func (c *Client) SetUserSettingsCrashReports(ctx context.Context, req SetCrashReportReq) (UserSettings, error) {
	var res struct {
		UserSettings UserSettings
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetBody(req).SetResult(&res).Put("/core/v4/settings/crashreports")
	}); err != nil {
		return UserSettings{}, err
	}

	return res.UserSettings, nil
}
