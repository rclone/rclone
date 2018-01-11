package src

import "encoding/json"

//DiskInfoRequest type
type DiskInfoRequest struct {
	client      *Client
	HTTPRequest *HTTPRequest
}

func (req *DiskInfoRequest) request() *HTTPRequest {
	return req.HTTPRequest
}

//DiskInfoResponse struct is returned by the API for DiskInfo request.
type DiskInfoResponse struct {
	TrashSize     uint64            `json:"TrashSize"`
	TotalSpace    uint64            `json:"TotalSpace"`
	UsedSpace     uint64            `json:"UsedSpace"`
	SystemFolders map[string]string `json:"SystemFolders"`
}

//NewDiskInfoRequest create new DiskInfo Request
func (c *Client) NewDiskInfoRequest() *DiskInfoRequest {
	return &DiskInfoRequest{
		client:      c,
		HTTPRequest: createGetRequest(c, "/", nil),
	}
}

//Exec run DiskInfo Request
func (req *DiskInfoRequest) Exec() (*DiskInfoResponse, error) {
	data, err := req.request().run(req.client)
	if err != nil {
		return nil, err
	}

	var info DiskInfoResponse
	err = json.Unmarshal(data, &info)
	if err != nil {
		return nil, err
	}
	if info.SystemFolders == nil {
		info.SystemFolders = make(map[string]string)
	}

	return &info, nil
}
