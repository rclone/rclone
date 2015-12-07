package src

import (
	"net/url"
	"strconv"
)

// Delete will remove specified file/folder from Yandex Disk
func (c *Client) Delete(remotePath string, permanently bool) error {

	values := url.Values{}
	values.Add("permanently", strconv.FormatBool(permanently))
	values.Add("path", remotePath)
	urlPath := "/v1/disk/resources?" + values.Encode()
	fullURL := RootAddr
	if urlPath[:1] != "/" {
		fullURL += "/" + urlPath
	} else {
		fullURL += urlPath
	}

	if err := c.PerformDelete(fullURL); err != nil {
		return err
	}
	return nil
}
