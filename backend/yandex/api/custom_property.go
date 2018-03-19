package src

import (
	"bytes"
	"encoding/json"
	"io"
	"net/url"
)

//CustomPropertyResponse struct we send and is returned by the API for CustomProperty request.
type CustomPropertyResponse struct {
	CustomProperties map[string]interface{} `json:"custom_properties"`
}

//SetCustomProperty will set specified data from Yandex Disk
func (c *Client) SetCustomProperty(remotePath string, property string, value string) error {
	rcm := map[string]interface{}{
		property: value,
	}
	cpr := CustomPropertyResponse{rcm}
	data, _ := json.Marshal(cpr)
	body := bytes.NewReader(data)
	err := c.SetCustomPropertyRequest(remotePath, body)
	if err != nil {
		return err
	}
	return err
}

//SetCustomPropertyRequest will make an CustomProperty request and return a URL to CustomProperty data to.
func (c *Client) SetCustomPropertyRequest(remotePath string, body io.Reader) (err error) {
	values := url.Values{}
	values.Add("path", remotePath)
	req, err := c.scopedRequest("PATCH", "/v1/disk/resources?"+values.Encode(), body)
	if err != nil {
		return err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	if err := CheckAPIError(resp); err != nil {
		return err
	}
	defer CheckClose(resp.Body, &err)

	//If needed we can read response and check if custom_property is set.

	return nil
}
