package src

// FilesResourceListResponse struct is returned by the API for requests.
type FilesResourceListResponse struct {
	Items  []ResourceInfoResponse `json:"items"`
	Limit  *uint64                `json:"limit"`
	Offset *uint64                `json:"offset"`
}
