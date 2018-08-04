package src

//from yadisk

import (
	"io"
	"io/ioutil"
	"net/http"

	"github.com/pkg/errors"
)

// PerformUpload does the actual upload via unscoped PUT request.
func (c *Client) PerformUpload(url string, data io.Reader, contentType string) (err error) {
	req, err := http.NewRequest("PUT", url, data)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)

	//c.setRequestScope(req)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer CheckClose(resp.Body, &err)

	if resp.StatusCode != 201 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		return errors.Errorf("upload error [%d]: %s", resp.StatusCode, string(body))
	}
	return nil
}
