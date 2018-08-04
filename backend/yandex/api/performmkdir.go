package src

import (
	"io/ioutil"
	"net/http"

	"github.com/pkg/errors"
)

// PerformMkdir does the actual mkdir via PUT request.
func (c *Client) PerformMkdir(url string) (int, string, error) {
	req, err := http.NewRequest("PUT", url, nil)
	if err != nil {
		return 0, "", err
	}

	//set access token and headers
	c.setRequestScope(req)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return 0, "", err
	}

	if resp.StatusCode != 201 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return 0, "", err
		}
		//third parameter is the json error response body
		return resp.StatusCode, string(body), errors.Errorf("create folder error [%d]: %s", resp.StatusCode, string(body))
	}
	return resp.StatusCode, "", nil
}
