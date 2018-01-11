package src

import "encoding/json"

// TrashResourceInfoRequest struct
type TrashResourceInfoRequest struct {
	client      *Client
	HTTPRequest *HTTPRequest
}

// Request of TrashResourceInfoRequest struct
func (req *TrashResourceInfoRequest) Request() *HTTPRequest {
	return req.HTTPRequest
}

// NewTrashResourceInfoRequest create new TrashResourceInfo Request
func (c *Client) NewTrashResourceInfoRequest(path string, options ...ResourceInfoRequestOptions) *TrashResourceInfoRequest {
	return &TrashResourceInfoRequest{
		client:      c,
		HTTPRequest: createResourceInfoRequest(c, "/trash/resources", path, options...),
	}
}

// Exec run TrashResourceInfo Request
func (req *TrashResourceInfoRequest) Exec() (*ResourceInfoResponse, error) {
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
