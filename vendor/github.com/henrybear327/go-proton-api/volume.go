package proton

import (
	"context"

	"github.com/go-resty/resty/v2"
)

func (c *Client) ListVolumes(ctx context.Context) ([]Volume, error) {
	var res struct {
		Volumes []Volume
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetResult(&res).Get("/drive/volumes")
	}); err != nil {
		return nil, err
	}

	return res.Volumes, nil
}

func (c *Client) GetVolume(ctx context.Context, volumeID string) (Volume, error) {
	var res struct {
		Volume Volume
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetResult(&res).Get("/drive/volumes/" + volumeID)
	}); err != nil {
		return Volume{}, err
	}

	return res.Volume, nil
}
