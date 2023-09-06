package proton_api_bridge

import (
	"context"

	"github.com/henrybear327/go-proton-api"
)

func listAllVolumes(ctx context.Context, c *proton.Client) ([]proton.Volume, error) {
	volumes, err := c.ListVolumes(ctx)
	if err != nil {
		return nil, err
	}

	return volumes, nil
}
