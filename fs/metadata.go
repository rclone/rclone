package fs

import "context"

// Metadata represents Object metadata in a standardised form
//
// See docs/content/metadata.md for the interpretation of the keys
type Metadata map[string]string

// Set k to v on m
//
// If m is nil, then it will get made
func (m *Metadata) Set(k, v string) {
	if *m == nil {
		*m = make(Metadata, 1)
	}
	(*m)[k] = v
}

// GetMetadata from an ObjectInfo
//
// If the object has no metadata then metadata will be nil
func GetMetadata(ctx context.Context, o ObjectInfo) (metadata Metadata, err error) {
	do, ok := o.(Metadataer)
	if !ok {
		return nil, nil
	}
	return do.Metadata(ctx)
}
