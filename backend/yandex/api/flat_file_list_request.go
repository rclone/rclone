package src

import (
	"encoding/json"
	"strings"
)

// FlatFileListRequest struct client for FlatFileList Request
type FlatFileListRequest struct {
	client      *Client
	HTTPRequest *HTTPRequest
}

// FlatFileListRequestOptions struct - options for request
type FlatFileListRequestOptions struct {
	MediaType   []MediaType
	Limit       *uint32
	Offset      *uint32
	Fields      []string
	PreviewSize *PreviewSize
	PreviewCrop *bool
}

// Request get request
func (req *FlatFileListRequest) Request() *HTTPRequest {
	return req.HTTPRequest
}

// NewFlatFileListRequest create new FlatFileList Request
func (c *Client) NewFlatFileListRequest(options ...FlatFileListRequestOptions) *FlatFileListRequest {
	var parameters = make(map[string]interface{})
	if len(options) > 0 {
		opt := options[0]
		if opt.Limit != nil {
			parameters["limit"] = *opt.Limit
		}
		if opt.Offset != nil {
			parameters["offset"] = *opt.Offset
		}
		if opt.Fields != nil {
			parameters["fields"] = strings.Join(opt.Fields, ",")
		}
		if opt.PreviewSize != nil {
			parameters["preview_size"] = opt.PreviewSize.String()
		}
		if opt.PreviewCrop != nil {
			parameters["preview_crop"] = *opt.PreviewCrop
		}
		if opt.MediaType != nil {
			var strMediaTypes = make([]string, len(opt.MediaType))
			for i, t := range opt.MediaType {
				strMediaTypes[i] = t.String()
			}
			parameters["media_type"] = strings.Join(strMediaTypes, ",")
		}
	}
	return &FlatFileListRequest{
		client:      c,
		HTTPRequest: createGetRequest(c, "/resources/files", parameters),
	}
}

// Exec run FlatFileList Request
func (req *FlatFileListRequest) Exec() (*FilesResourceListResponse, error) {
	data, err := req.Request().run(req.client)
	if err != nil {
		return nil, err
	}
	var info FilesResourceListResponse
	err = json.Unmarshal(data, &info)
	if err != nil {
		return nil, err
	}
	if cap(info.Items) == 0 {
		info.Items = []ResourceInfoResponse{}
	}
	return &info, nil
}
