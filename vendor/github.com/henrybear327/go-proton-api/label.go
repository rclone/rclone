package proton

import (
	"context"
	"errors"
	"strconv"

	"github.com/go-resty/resty/v2"
)

var ErrNoSuchLabel = errors.New("no such label")

func (c *Client) GetLabel(ctx context.Context, labelID string, labelTypes ...LabelType) (Label, error) {
	labels, err := c.GetLabels(ctx, labelTypes...)
	if err != nil {
		return Label{}, err
	}

	for _, label := range labels {
		if label.ID == labelID {
			return label, nil
		}
	}

	return Label{}, ErrNoSuchLabel
}

func (c *Client) GetLabels(ctx context.Context, labelTypes ...LabelType) ([]Label, error) {
	var labels []Label

	for _, labelType := range labelTypes {
		labelType := labelType

		var res struct {
			Labels []Label
		}

		if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
			return r.SetQueryParam("Type", strconv.Itoa(int(labelType))).SetResult(&res).Get("/core/v4/labels")
		}); err != nil {
			return nil, err
		}

		labels = append(labels, res.Labels...)
	}

	return labels, nil
}

func (c *Client) CreateLabel(ctx context.Context, req CreateLabelReq) (Label, error) {
	var res struct {
		Label Label
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetBody(req).SetResult(&res).Post("/core/v4/labels")
	}); err != nil {
		return Label{}, err
	}

	return res.Label, nil
}

func (c *Client) DeleteLabel(ctx context.Context, labelID string) error {
	return c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.Delete("/core/v4/labels/" + labelID)
	})
}

func (c *Client) UpdateLabel(ctx context.Context, labelID string, req UpdateLabelReq) (Label, error) {
	var res struct {
		Label Label
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetBody(req).SetResult(&res).Put("/core/v4/labels/" + labelID)
	}); err != nil {
		return Label{}, err
	}

	return res.Label, nil
}
