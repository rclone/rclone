package src

// LastUploadedResourceListResponse struct
type LastUploadedResourceListResponse struct {
	Items []ResourceInfoResponse `json:"items"`
	Limit *uint64                `json:"limit"`
}
