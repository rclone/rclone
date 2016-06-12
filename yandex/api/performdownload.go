package src

import (
	"io"
	"io/ioutil"
	"net/http"

	"github.com/pkg/errors"
)

// PerformDownload does the actual download via unscoped PUT request.
func (c *Client) PerformDownload(url string) (out io.ReadCloser, err error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	//c.setRequestScope(req)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		defer CheckClose(resp.Body, &err)
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return nil, errors.Errorf("download error [%d]: %s", resp.StatusCode, string(body[:]))
	}
	return resp.Body, err
}
