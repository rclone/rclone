package proton

import (
	"context"

	"github.com/go-resty/resty/v2"
)

func (c *Client) GetCalendars(ctx context.Context) ([]Calendar, error) {
	var res struct {
		Calendars []Calendar
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetResult(&res).Get("/calendar/v1")
	}); err != nil {
		return nil, err
	}

	return res.Calendars, nil
}

func (c *Client) GetCalendar(ctx context.Context, calendarID string) (Calendar, error) {
	var res struct {
		Calendar Calendar
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetResult(&res).Get("/calendar/v1/" + calendarID)
	}); err != nil {
		return Calendar{}, err
	}

	return res.Calendar, nil
}

func (c *Client) GetCalendarKeys(ctx context.Context, calendarID string) (CalendarKeys, error) {
	var res struct {
		Keys CalendarKeys
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetResult(&res).Get("/calendar/v1/" + calendarID + "/keys")
	}); err != nil {
		return nil, err
	}

	return res.Keys, nil
}

func (c *Client) GetCalendarMembers(ctx context.Context, calendarID string) ([]CalendarMember, error) {
	var res struct {
		Members []CalendarMember
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetResult(&res).Get("/calendar/v1/" + calendarID + "/members")
	}); err != nil {
		return nil, err
	}

	return res.Members, nil
}

func (c *Client) GetCalendarPassphrase(ctx context.Context, calendarID string) (CalendarPassphrase, error) {
	var res struct {
		Passphrase CalendarPassphrase
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetResult(&res).Get("/calendar/v1/" + calendarID + "/passphrase")
	}); err != nil {
		return CalendarPassphrase{}, err
	}

	return res.Passphrase, nil
}
