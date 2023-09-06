package proton_api_bridge

import (
	"context"

	"github.com/henrybear327/go-proton-api"
)

func getAllShares(ctx context.Context, c *proton.Client) ([]proton.ShareMetadata, error) {
	shares, err := c.ListShares(ctx, true)
	if err != nil {
		return nil, err
	}

	return shares, nil
}

func getShareByID(ctx context.Context, c *proton.Client, shareID string) (*proton.Share, error) {
	share, err := c.GetShare(ctx, shareID)
	if err != nil {
		return nil, err
	}

	return &share, nil
}
