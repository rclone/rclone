package putio

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// ZipsService is the service manage zip streams.
type ZipsService struct {
	client *Client
}

// Get gives detailed information about the given zip file id.
func (z *ZipsService) Get(ctx context.Context, id int64) (Zip, error) {
	req, err := z.client.NewRequest(ctx, "GET", "/v2/zips/"+itoa(id), nil)
	if err != nil {
		return Zip{}, err
	}

	var r Zip
	_, err = z.client.Do(req, &r)
	if err != nil {
		return Zip{}, err
	}

	return r, nil
}

// List lists active zip files.
func (z *ZipsService) List(ctx context.Context) ([]Zip, error) {
	req, err := z.client.NewRequest(ctx, "GET", "/v2/zips/list", nil)
	if err != nil {
		return nil, err
	}

	var r struct {
		Zips []Zip
	}
	_, err = z.client.Do(req, &r)
	if err != nil {
		return nil, err
	}

	return r.Zips, nil
}

// Create creates zip files for given file IDs. If the operation is successful,
// a zip ID will be returned to keep track of zip process.
func (z *ZipsService) Create(ctx context.Context, fileIDs ...int64) (int64, error) {
	if len(fileIDs) == 0 {
		return 0, fmt.Errorf("no file id given")
	}

	var ids []string
	for _, id := range fileIDs {
		ids = append(ids, itoa(id))
	}

	params := url.Values{}
	params.Set("file_ids", strings.Join(ids, ","))

	req, err := z.client.NewRequest(ctx, "POST", "/v2/zips/create", strings.NewReader(params.Encode()))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var r struct {
		ID int64 `json:"zip_id"`
	}
	_, err = z.client.Do(req, &r)
	if err != nil {
		return 0, err
	}

	return r.ID, nil
}
