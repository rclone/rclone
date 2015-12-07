package src

import (
	"fmt"
	"io/ioutil"
	"net/http"
)

// PerformDelete does the actual delete via DELETE request.
func (c *Client) PerformDelete(url string) error {
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}

	//set access token and headers
	c.setRequestScope(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	//204 - resource deleted.
	//202 - folder not empty, content will be deleted soon (async delete).
	if resp.StatusCode != 204 && resp.StatusCode != 202 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("delete error [%d]: %s", resp.StatusCode, string(body[:]))
	}
	return nil
}
