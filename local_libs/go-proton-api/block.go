package proton

import (
	"bytes"
	"context"
	"io"
	"math/rand/v2"
	"time"

	"github.com/go-resty/resty/v2"
)

func (c *Client) GetBlock(ctx context.Context, bareURL, token string) (io.ReadCloser, error) {
	res, err := c.doRes(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetHeader("pm-storage-token", token).SetDoNotParseResponse(true).Get(bareURL)
	})
	if err != nil {
		return nil, err
	}

	return res.RawBody(), nil
}

func (c *Client) RequestBlockUpload(ctx context.Context, req BlockUploadReq) ([]BlockUploadLink, error) {
	var res struct {
		UploadLinks []BlockUploadLink
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetResult(&res).SetBody(req).Post("/drive/blocks")
	}); err != nil {
		return nil, err
	}

	return res.UploadLinks, nil
}

func (c *Client) UploadBlock(ctx context.Context, bareURL, token string, block io.Reader) error {
	// Buffer the block data so we can retry on transient failures.
	data, err := io.ReadAll(block)
	if err != nil {
		return err
	}

	const maxRetries = 5

	for attempt := 0; ; attempt++ {
		err = c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
			return r.
				SetHeader("pm-storage-token", token).
				SetMultipartField("Block", "blob", "application/octet-stream", bytes.NewReader(data)).
				Post(bareURL)
		})
		if err == nil {
			return nil
		}
		if attempt >= maxRetries-1 {
			return err
		}

		// Exponential backoff with jitter: 1s, 2s, 4s, 8s base
		base := time.Duration(1<<uint(attempt)) * time.Second
		jitter := time.Duration(rand.Int64N(int64(base / 2)))
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(base + jitter):
		}
	}
}
