package proton

import (
	"context"

	"github.com/go-resty/resty/v2"
)

func (c *Client) GetLatestVolumeEventID(ctx context.Context, volumeID string) (string, error) {
	var res struct {
		EventID string
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetResult(&res).Get("/drive/volumes/" + volumeID + "/events/latest")
	}); err != nil {
		return "", err
	}

	return res.EventID, nil
}

func (c *Client) GetLatestShareEventID(ctx context.Context, shareID string) (string, error) {
	var res struct {
		EventID string
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetResult(&res).Get("/drive/shares/" + shareID + "/events/latest")
	}); err != nil {
		return "", err
	}

	return res.EventID, nil
}

func (c *Client) GetVolumeEvent(ctx context.Context, volumeID, eventID string) (DriveEvent, error) {
	event, more, err := c.getVolumeEvent(ctx, volumeID, eventID)
	if err != nil {
		return DriveEvent{}, err
	}

	for more {
		var next DriveEvent

		next, more, err = c.getVolumeEvent(ctx, volumeID, event.EventID)
		if err != nil {
			return DriveEvent{}, err
		}

		event.Events = append(event.Events, next.Events...)
	}

	return event, nil
}

func (c *Client) GetShareEvent(ctx context.Context, shareID, eventID string) (DriveEvent, error) {
	event, more, err := c.getShareEvent(ctx, shareID, eventID)
	if err != nil {
		return DriveEvent{}, err
	}

	for more {
		var next DriveEvent

		next, more, err = c.getShareEvent(ctx, shareID, event.EventID)
		if err != nil {
			return DriveEvent{}, err
		}

		event.Events = append(event.Events, next.Events...)
	}

	return event, nil
}

func (c *Client) getVolumeEvent(ctx context.Context, volumeID, eventID string) (DriveEvent, bool, error) {
	var res struct {
		DriveEvent

		More Bool
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetResult(&res).Get("/drive/volumes/" + volumeID + "/events/" + eventID)
	}); err != nil {
		return DriveEvent{}, false, err
	}

	return res.DriveEvent, bool(res.More), nil
}

func (c *Client) getShareEvent(ctx context.Context, shareID, eventID string) (DriveEvent, bool, error) {
	var res struct {
		DriveEvent

		More Bool
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetResult(&res).Get("/drive/shares/" + shareID + "/events/" + eventID)
	}); err != nil {
		return DriveEvent{}, false, err
	}

	return res.DriveEvent, bool(res.More), nil
}
