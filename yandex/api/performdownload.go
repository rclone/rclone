package src

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
)

// PerformDownload does the actual download via unscoped PUT request.
func (c *Client) PerformDownload(url string) (io.ReadCloser, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	//c.setRequestScope(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		defer CheckClose(resp.Body, &err)
		return nil, fmt.Errorf("download error [%d]: %s", resp.StatusCode, string(body[:]))
	}
	return resp.Body, err
}
