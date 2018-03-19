package src

// ResourceInfoRequestOptions struct
type ResourceInfoRequestOptions struct {
	SortMode    *SortMode
	Limit       *uint32
	Offset      *uint32
	Fields      []string
	PreviewSize *PreviewSize
	PreviewCrop *bool
}
