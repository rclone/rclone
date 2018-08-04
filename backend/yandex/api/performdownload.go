package src

import (
	"io"
	"io/ioutil"
	"net/http"

	"github.com/pkg/errors"
)

// PerformDownload does the actual download via unscoped GET request.
func (c *Client) PerformDownload(url string, headers map[string]string) (out io.ReadCloser, err error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Set any extra headers
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	//c.setRequestScope(req)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}

	_, isRanging := req.Header["Range"]
	if !(resp.StatusCode == http.StatusOK || (isRanging && resp.StatusCode == http.StatusPartialContent)) {
		defer CheckClose(resp.Body, &err)
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return nil, errors.Errorf("download error [%d]: %s", resp.StatusCode, string(body))
	}
	return resp.Body, err
}
