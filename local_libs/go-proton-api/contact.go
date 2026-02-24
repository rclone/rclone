package proton

import (
	"context"
	"strconv"

	"github.com/go-resty/resty/v2"
)

func (c *Client) GetContact(ctx context.Context, contactID string) (Contact, error) {
	var res struct {
		Contact Contact
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetResult(&res).Get("/contacts/v4/" + contactID)
	}); err != nil {
		return Contact{}, err
	}

	return res.Contact, nil
}

func (c *Client) CountContacts(ctx context.Context) (int, error) {
	var res struct {
		Total int
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetResult(&res).Get("/contacts/v4")
	}); err != nil {
		return 0, err
	}

	return res.Total, nil
}

func (c *Client) CountContactEmails(ctx context.Context, email string) (int, error) {
	var res struct {
		Total int
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetResult(&res).SetQueryParam("Email", email).Get("/contacts/v4/emails")
	}); err != nil {
		return 0, err
	}

	return res.Total, nil
}

func (c *Client) GetContacts(ctx context.Context, page, pageSize int) ([]Contact, error) {
	var res struct {
		Contacts []Contact
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetQueryParams(map[string]string{
			"Page":     strconv.Itoa(page),
			"PageSize": strconv.Itoa(pageSize),
		}).SetResult(&res).Get("/contacts/v4")
	}); err != nil {
		return nil, err
	}

	return res.Contacts, nil
}

func (c *Client) GetAllContacts(ctx context.Context) ([]Contact, error) {
	total, err := c.CountContacts(ctx)
	if err != nil {
		return nil, err
	}

	return fetchPaged(ctx, total, maxPageSize, c, func(ctx context.Context, page, pageSize int) ([]Contact, error) {
		return c.GetContacts(ctx, page, pageSize)
	})
}

func (c *Client) GetContactEmails(ctx context.Context, email string, page, pageSize int) ([]ContactEmail, error) {
	var res struct {
		ContactEmails []ContactEmail
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetQueryParams(map[string]string{
			"Page":     strconv.Itoa(page),
			"PageSize": strconv.Itoa(pageSize),
			"Email":    email,
		}).SetResult(&res).Get("/contacts/v4/emails")
	}); err != nil {
		return nil, err
	}

	return res.ContactEmails, nil
}

func (c *Client) GetAllContactEmails(ctx context.Context, email string) ([]ContactEmail, error) {
	total, err := c.CountContactEmails(ctx, email)
	if err != nil {
		return nil, err
	}

	return fetchPaged(ctx, total, maxPageSize, c, func(ctx context.Context, page, pageSize int) ([]ContactEmail, error) {
		return c.GetContactEmails(ctx, email, page, pageSize)
	})
}

func (c *Client) CreateContacts(ctx context.Context, req CreateContactsReq) ([]CreateContactsRes, error) {
	var res struct {
		Responses []CreateContactsRes
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetBody(req).SetResult(&res).Post("/contacts/v4")
	}); err != nil {
		return nil, err
	}

	return res.Responses, nil
}

func (c *Client) UpdateContact(ctx context.Context, contactID string, req UpdateContactReq) (Contact, error) {
	var res struct {
		Contact Contact
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetBody(req).SetResult(&res).Put("/contacts/v4/" + contactID)
	}); err != nil {
		return Contact{}, err
	}

	return res.Contact, nil
}

func (c *Client) DeleteContacts(ctx context.Context, req DeleteContactsReq) error {
	return c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetBody(req).Put("/contacts/v4/delete")
	})
}
