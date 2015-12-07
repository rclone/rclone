package src

// ResourceListResponse struct
type ResourceListResponse struct {
	Sort      *SortMode              `json:"sort"`
	PublicKey string                 `json:"public_key"`
	Items     []ResourceInfoResponse `json:"items"`
	Path      string                 `json:"path"`
	Limit     *uint64                `json:"limit"`
	Offset    *uint64                `json:"offset"`
	Total     *uint64                `json:"total"`
}
