package proton

import (
	"context"
	"fmt"
	"strconv"

	"github.com/bradenaw/juniper/xslices"
	"github.com/go-resty/resty/v2"
)

func (c *Client) ListChildren(ctx context.Context, shareID, linkID string, showAll bool) ([]Link, error) {
	var res struct {
		Links []Link
	}

	var links []Link

	for page := 0; ; page++ {
		if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
			return r.
				SetQueryParams(map[string]string{
					"Page":     strconv.Itoa(page),
					"PageSize": strconv.Itoa(maxPageSize),
					"ShowAll":  Bool(showAll).FormatURL(),
				}).
				SetResult(&res).
				Get("/drive/shares/" + shareID + "/folders/" + linkID + "/children")
		}); err != nil {
			return nil, err
		}

		if len(res.Links) == 0 {
			break
		}

		links = append(links, res.Links...)
	}

	return links, nil
}

func (c *Client) TrashChildren(ctx context.Context, shareID, linkID string, childIDs ...string) error {
	var res struct {
		Responses []struct {
			LinkID   string
			Response APIError
		}
	}

	for _, childIDs := range xslices.Chunk(childIDs, maxPageSize) {
		req := struct {
			LinkIDs []string
		}{
			LinkIDs: childIDs,
		}

		if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
			return r.SetBody(req).SetResult(&res).Post("/drive/shares/" + shareID + "/folders/" + linkID + "/trash_multiple")
		}); err != nil {
			return err
		}

		for _, res := range res.Responses {
			if res.Response.Code != SuccessCode {
				return fmt.Errorf("failed to trash child: %w", res.Response)
			}
		}
	}

	return nil
}

func (c *Client) EmptyTrash(ctx context.Context, shareID string) error {
	var res struct {
		APIError
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetResult(&res).Delete("/drive/shares/" + shareID + "/trash")
	}); err != nil {
		return err
	}

	return nil
}

func (c *Client) DeleteChildren(ctx context.Context, shareID, linkID string, childIDs ...string) error {
	var res struct {
		Responses []struct {
			LinkID   string
			Response APIError
		}
	}

	for _, childIDs := range xslices.Chunk(childIDs, maxPageSize) {
		req := struct {
			LinkIDs []string
		}{
			LinkIDs: childIDs,
		}

		if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
			return r.SetBody(req).SetResult(&res).Post("/drive/shares/" + shareID + "/folders/" + linkID + "/delete_multiple")
		}); err != nil {
			return err
		}

		for _, res := range res.Responses {
			if res.Response.Code != SuccessCode {
				return fmt.Errorf("failed to delete child: %w", res.Response)
			}
		}
	}

	return nil
}
