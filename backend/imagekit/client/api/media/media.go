package media

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/rclone/rclone/backend/imagekit/client/api"
	"github.com/rclone/rclone/backend/imagekit/client/config"
)

// API is the main struct for media
type API struct {
	Config     config.Configuration
	HttpClient *http.Client
}

func (m *API) post(ctx context.Context, url string, data interface{}, ms api.MetaSetter) (*http.Response, error) {
	url = api.BuildPath(m.Config.Prefix, url)
	var err error
	var body []byte

	if data != nil {
		if body, err = json.Marshal(data); err != nil {
			return nil, fmt.Errorf("post:marshal data: %w", err)
		}
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))

	if err != nil {
		return nil, fmt.Errorf("post:http request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "rclone/imagekit")
	req.SetBasicAuth(m.Config.PrivateKey, "")

	resp, err := m.Config.HttpClient.Do(req.WithContext(ctx))
	defer api.DeferredBodyClose(resp)

	if err != nil {
		err = fmt.Errorf("client.Do %w", err)
	}
	api.SetResponseMeta(resp, ms)
	return resp, err
}

func (m *API) get(ctx context.Context, url string, ms api.MetaSetter) (*http.Response, error) {
	url = api.BuildPath(m.Config.Prefix, url)
	req, err := http.NewRequest(http.MethodGet, url, nil)

	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "rclone/imagekit")
	req.SetBasicAuth(m.Config.PrivateKey, "")

	resp, err := m.HttpClient.Do(req.WithContext(ctx))
	defer api.DeferredBodyClose(resp)

	api.SetResponseMeta(resp, ms)

	return resp, err
}

func (m *API) delete(ctx context.Context, url string, data interface{}, ms api.MetaSetter) (*http.Response, error) {
	var err error
	url = api.BuildPath(m.Config.Prefix, url)
	var body []byte

	if data != nil {
		if body, err = json.Marshal(data); err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequest(http.MethodDelete, url, bytes.NewBuffer(body))
	req.Header.Set("User-Agent", "rclone/imagekit")
	if data != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(m.Config.PrivateKey, "")

	resp, err := m.HttpClient.Do(req.WithContext(ctx))
	defer api.DeferredBodyClose(resp)

	api.SetResponseMeta(resp, ms)

	return resp, err
}
