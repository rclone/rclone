package src

import (
	"encoding/json"
	"strings"
)

// LastUploadedResourceListRequest struct
type LastUploadedResourceListRequest struct {
	client      *Client
	HTTPRequest *HTTPRequest
}

// LastUploadedResourceListRequestOptions struct
type LastUploadedResourceListRequestOptions struct {
	MediaType   []MediaType
	Limit       *uint32
	Fields      []string
	PreviewSize *PreviewSize
	PreviewCrop *bool
}

// Request return request
func (req *LastUploadedResourceListRequest) Request() *HTTPRequest {
	return req.HTTPRequest
}

// NewLastUploadedResourceListRequest create new LastUploadedResourceList Request
func (c *Client) NewLastUploadedResourceListRequest(options ...LastUploadedResourceListRequestOptions) *LastUploadedResourceListRequest {
	var parameters = make(map[string]interface{})
	if len(options) > 0 {
		opt := options[0]
		if opt.Limit != nil {
			parameters["limit"] = opt.Limit
		}
		if opt.Fields != nil {
			parameters["fields"] = strings.Join(opt.Fields, ",")
		}
		if opt.PreviewSize != nil {
			parameters["preview_size"] = opt.PreviewSize.String()
		}
		if opt.PreviewCrop != nil {
			parameters["preview_crop"] = opt.PreviewCrop
		}
		if opt.MediaType != nil {
			var strMediaTypes = make([]string, len(opt.MediaType))
			for i, t := range opt.MediaType {
				strMediaTypes[i] = t.String()
			}
			parameters["media_type"] = strings.Join(strMediaTypes, ",")
		}
	}
	return &LastUploadedResourceListRequest{
		client:      c,
		HTTPRequest: createGetRequest(c, "/resources/last-uploaded", parameters),
	}
}

// Exec run LastUploadedResourceList Request
func (req *LastUploadedResourceListRequest) Exec() (*LastUploadedResourceListResponse, error) {
	data, err := req.Request().run(req.client)
	if err != nil {
		return nil, err
	}
	var info LastUploadedResourceListResponse
	err = json.Unmarshal(data, &info)
	if err != nil {
		return nil, err
	}
	if cap(info.Items) == 0 {
		info.Items = []ResourceInfoResponse{}
	}
	return &info, nil
}
