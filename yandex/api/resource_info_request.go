package src

import "encoding/json"

// ResourceInfoRequest struct
type ResourceInfoRequest struct {
	client      *Client
	HTTPRequest *HTTPRequest
}

// Request of ResourceInfoRequest
func (req *ResourceInfoRequest) Request() *HTTPRequest {
	return req.HTTPRequest
}

// NewResourceInfoRequest create new ResourceInfo Request
func (c *Client) NewResourceInfoRequest(path string, options ...ResourceInfoRequestOptions) *ResourceInfoRequest {
	return &ResourceInfoRequest{
		client:      c,
		HTTPRequest: createResourceInfoRequest(c, "/resources", path, options...),
	}
}

// Exec run ResourceInfo Request
func (req *ResourceInfoRequest) Exec() (*ResourceInfoResponse, error) {
	data, err := req.Request().run(req.client)
	if err != nil {
		return nil, err
	}

	var info ResourceInfoResponse
	err = json.Unmarshal(data, &info)
	if err != nil {
		return nil, err
	}
	if info.CustomProperties == nil {
		info.CustomProperties = make(map[string]interface{})
	}
	if info.Embedded != nil {
		if cap(info.Embedded.Items) == 0 {
			info.Embedded.Items = []ResourceInfoResponse{}
		}
	}
	return &info, nil
}
