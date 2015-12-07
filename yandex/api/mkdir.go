package src

import (
	"net/url"
)

// Mkdir will make specified folder on Yandex Disk
func (c *Client) Mkdir(remotePath string) (int, string, error) {

	values := url.Values{}
	values.Add("path", remotePath) // only one current folder will be created. Not all the folders in the path.
	urlPath := "/v1/disk/resources?" + values.Encode()
	fullURL := RootAddr
	if urlPath[:1] != "/" {
		fullURL += "/" + urlPath
	} else {
		fullURL += urlPath
	}

	return c.PerformMkdir(fullURL)
}
