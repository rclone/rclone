package src

import "strings"

func createResourceInfoRequest(c *Client,
	apiPath string,
	path string,
	options ...ResourceInfoRequestOptions) *HTTPRequest {
	var parameters = make(map[string]interface{})
	parameters["path"] = path
	if len(options) > 0 {
		opt := options[0]
		if opt.SortMode != nil {
			parameters["sort"] = opt.SortMode.String()
		}
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
	}
	return createGetRequest(c, apiPath, parameters)
}
