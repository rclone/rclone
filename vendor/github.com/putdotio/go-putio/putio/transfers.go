package putio

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// TransfersService is the service to operate on torrent transfers, such as
// adding a torrent or magnet link, retrying a current one etc.
type TransfersService struct {
	client *Client
}

// List lists all active transfers. If a transfer is completed, it will not be
// available in response.
func (t *TransfersService) List(ctx context.Context) ([]Transfer, error) {
	req, err := t.client.NewRequest(ctx, "GET", "/v2/transfers/list", nil)
	if err != nil {
		return nil, err
	}

	var r struct {
		Transfers []Transfer
	}
	_, err = t.client.Do(req, &r)
	if err != nil {
		return nil, err
	}

	return r.Transfers, nil
}

// Add creates a new transfer. A valid torrent or a magnet URL is expected.
// Parent is the folder where the new transfer is downloaded to. If a negative
// value is given, user's preferred download folder is used. CallbackURL is
// used to send a POST request after the transfer is finished downloading.
func (t *TransfersService) Add(ctx context.Context, urlStr string, parent int64, callbackURL string) (Transfer, error) {
	if urlStr == "" {
		return Transfer{}, fmt.Errorf("empty URL")
	}

	params := url.Values{}
	params.Set("url", urlStr)
	// negative values indicate user's preferred download folder. don't include
	// it in the request
	if parent >= 0 {
		params.Set("save_parent_id", itoa(parent))
	}
	if callbackURL != "" {
		params.Set("callback_url", callbackURL)
	}

	req, err := t.client.NewRequest(ctx, "POST", "/v2/transfers/add", strings.NewReader(params.Encode()))
	if err != nil {
		return Transfer{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var r struct {
		Transfer Transfer
	}
	_, err = t.client.Do(req, &r)
	if err != nil {
		return Transfer{}, err
	}

	return r.Transfer, nil
}

// Get returns the given transfer's properties.
func (t *TransfersService) Get(ctx context.Context, id int64) (Transfer, error) {
	req, err := t.client.NewRequest(ctx, "GET", "/v2/transfers/"+itoa(id), nil)
	if err != nil {
		return Transfer{}, err
	}

	var r struct {
		Transfer Transfer
	}
	_, err = t.client.Do(req, &r)
	if err != nil {
		return Transfer{}, err
	}

	return r.Transfer, nil
}

// Retry retries previously failed transfer.
func (t *TransfersService) Retry(ctx context.Context, id int64) (Transfer, error) {
	params := url.Values{}
	params.Set("id", itoa(id))

	req, err := t.client.NewRequest(ctx, "POST", "/v2/transfers/retry", strings.NewReader(params.Encode()))
	if err != nil {
		return Transfer{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var r struct {
		Transfer Transfer
	}
	_, err = t.client.Do(req, &r)
	if err != nil {
		return Transfer{}, err
	}

	return r.Transfer, nil
}

// Cancel deletes given transfers.
func (t *TransfersService) Cancel(ctx context.Context, ids ...int64) error {
	if len(ids) == 0 {
		return fmt.Errorf("no id given")
	}

	var transfers []string
	for _, id := range ids {
		transfers = append(transfers, itoa(id))
	}

	params := url.Values{}
	params.Set("transfer_ids", strings.Join(transfers, ","))

	req, err := t.client.NewRequest(ctx, "POST", "/v2/transfers/cancel", strings.NewReader(params.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	_, err = t.client.Do(req, &struct{}{})
	if err != nil {
		return err
	}

	return nil
}

// Clean removes completed transfers from the transfer list.
func (t *TransfersService) Clean(ctx context.Context) error {
	req, err := t.client.NewRequest(ctx, "POST", "/v2/transfers/clean", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	_, err = t.client.Do(req, &struct{}{})
	if err != nil {
		return err
	}

	return nil
}
