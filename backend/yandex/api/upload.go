package src

//from yadisk

import (
	"encoding/json"
	"io"
	"net/url"
	"strconv"
)

// UploadResponse struct is returned by the API for upload request.
type UploadResponse struct {
	HRef      string `json:"href"`
	Method    string `json:"method"`
	Templated bool   `json:"templated"`
}

// Upload will put specified data to Yandex.Disk.
func (c *Client) Upload(data io.Reader, remotePath string, overwrite bool, contentType string) error {
	ur, err := c.UploadRequest(remotePath, overwrite)
	if err != nil {
		return err
	}

	return c.PerformUpload(ur.HRef, data, contentType)
}

// UploadRequest will make an upload request and return a URL to upload data to.
func (c *Client) UploadRequest(remotePath string, overwrite bool) (ur *UploadResponse, err error) {
	values := url.Values{}
	values.Add("path", remotePath)
	values.Add("overwrite", strconv.FormatBool(overwrite))

	req, err := c.scopedRequest("GET", "/v1/disk/resources/upload?"+values.Encode(), nil)
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

	ur, err = ParseUploadResponse(resp.Body)
	if err != nil {
		return nil, err
	}

	return ur, nil
}

// ParseUploadResponse tries to read and parse UploadResponse struct.
func ParseUploadResponse(data io.Reader) (*UploadResponse, error) {
	dec := json.NewDecoder(data)
	var ur UploadResponse

	if err := dec.Decode(&ur); err == io.EOF {
		// ok
	} else if err != nil {
		return nil, err
	}

	// TODO: check if there is any trash data after JSON and crash if there is.

	return &ur, nil
}
