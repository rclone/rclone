package proton

import (
	"context"
	"net/url"
	"strconv"

	"github.com/go-resty/resty/v2"
)

func (c *Client) CountCalendarEvents(ctx context.Context, calendarID string) (int, error) {
	var res struct {
		Total int
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetResult(&res).Get("/calendar/v1/" + calendarID + "/events")
	}); err != nil {
		return 0, err
	}

	return res.Total, nil
}

// TODO: For now, the query params are partially constant -- should they be configurable?
func (c *Client) GetCalendarEvents(ctx context.Context, calendarID string, page, pageSize int, filter url.Values) ([]CalendarEvent, error) {
	var res struct {
		Events []CalendarEvent
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetQueryParams(map[string]string{
			"Page":     strconv.Itoa(page),
			"PageSize": strconv.Itoa(pageSize),
		}).SetQueryParamsFromValues(filter).SetResult(&res).Get("/calendar/v1/" + calendarID + "/events")
	}); err != nil {
		return nil, err
	}

	return res.Events, nil
}

func (c *Client) GetAllCalendarEvents(ctx context.Context, calendarID string, filter url.Values) ([]CalendarEvent, error) {
	total, err := c.CountCalendarEvents(ctx, calendarID)
	if err != nil {
		return nil, err
	}

	return fetchPaged(ctx, total, maxPageSize, c, func(ctx context.Context, page, pageSize int) ([]CalendarEvent, error) {
		return c.GetCalendarEvents(ctx, calendarID, page, pageSize, filter)
	})
}

func (c *Client) GetCalendarEvent(ctx context.Context, calendarID, eventID string) (CalendarEvent, error) {
	var res struct {
		Event CalendarEvent
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetResult(&res).Get("/calendar/v1/" + calendarID + "/events/" + eventID)
	}); err != nil {
		return CalendarEvent{}, err
	}

	return res.Event, nil
}
