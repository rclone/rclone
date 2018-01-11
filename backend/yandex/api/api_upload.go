package src

//from  yadisk

import (
	"io"
	"net/http"
)

//RootAddr is the base URL for Yandex Disk API.
const RootAddr = "https://cloud-api.yandex.com" //also https://cloud-api.yandex.net and https://cloud-api.yandex.ru

func (c *Client) setRequestScope(req *http.Request) {
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "OAuth "+c.token)
}

func (c *Client) scopedRequest(method, urlPath string, body io.Reader) (*http.Request, error) {
	fullURL := RootAddr
	if urlPath[:1] != "/" {
		fullURL += "/" + urlPath
	} else {
		fullURL += urlPath
	}

	req, err := http.NewRequest(method, fullURL, body)
	if err != nil {
		return req, err
	}

	c.setRequestScope(req)
	return req, nil
}
