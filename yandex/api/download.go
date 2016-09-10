package src

import (
	"encoding/json"
	"io"
	"net/url"
)

// DownloadResponse struct is returned by the API for Download request.
type DownloadResponse struct {
	HRef      string `json:"href"`
	Method    string `json:"method"`
	Templated bool   `json:"templated"`
}

// Download will get specified data from Yandex.Disk supplying the extra headers
func (c *Client) Download(remotePath string, headers map[string]string) (io.ReadCloser, error) { //io.Writer
	ur, err := c.DownloadRequest(remotePath)
	if err != nil {
		return nil, err
	}
	return c.PerformDownload(ur.HRef, headers)
}

// DownloadRequest will make an download request and return a URL to download data to.
func (c *Client) DownloadRequest(remotePath string) (ur *DownloadResponse, err error) {
	values := url.Values{}
	values.Add("path", remotePath)

	req, err := c.scopedRequest("GET", "/v1/disk/resources/download?"+values.Encode(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	if err := CheckAPIError(resp); err != nil {
		return nil, err
	}
	defer CheckClose(resp.Body, &err)

	ur, err = ParseDownloadResponse(resp.Body)
	if err != nil {
		return nil, err
	}

	return ur, nil
}

// ParseDownloadResponse tries to read and parse DownloadResponse struct.
func ParseDownloadResponse(data io.Reader) (*DownloadResponse, error) {
	dec := json.NewDecoder(data)
	var ur DownloadResponse

	if err := dec.Decode(&ur); err == io.EOF {
		// ok
	} else if err != nil {
		return nil, err
	}

	// TODO: check if there is any trash data after JSON and crash if there is.

	return &ur, nil
}
