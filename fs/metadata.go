package fs

import "context"

// Metadata represents Object metadata in a standardised form
//
// See docs/content/metadata.md for the interpretation of the keys
type Metadata map[string]string

// MetadataHelp represents help for a bit of system metadata
type MetadataHelp struct {
	Help     string
	Type     string
	Example  string
	ReadOnly bool
}

// MetadataInfo is help for the whole metadata for this backend.
type MetadataInfo struct {
	System map[string]MetadataHelp
	Help   string
}

// Set k to v on m
//
// If m is nil, then it will get made
func (m *Metadata) Set(k, v string) {
	if *m == nil {
		*m = make(Metadata, 1)
	}
	(*m)[k] = v
}

// Merge other into m
//
// If m is nil, then it will get made
func (m *Metadata) Merge(other Metadata) {
	for k, v := range other {
		if *m == nil {
			*m = make(Metadata, len(other))
		}
		(*m)[k] = v
	}
}

// MergeOptions gets any Metadata from the options passed in and
// stores it in m (which may be nil).
//
// If there is no m then metadata will be nil
func (m *Metadata) MergeOptions(options []OpenOption) {
	for _, opt := range options {
		if metadataOption, ok := opt.(MetadataOption); ok {
			m.Merge(Metadata(metadataOption))
		}
	}
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

// GetMetadataOptions from an ObjectInfo and merge it with any in options
//
// If --metadata isn't in use it will return nil
//
// If the object has no metadata then metadata will be nil
func GetMetadataOptions(ctx context.Context, o ObjectInfo, options []OpenOption) (metadata Metadata, err error) {
	ci := GetConfig(ctx)
	if !ci.Metadata {
		return nil, nil
	}
	metadata, err = GetMetadata(ctx, o)
	if err != nil {
		return nil, err
	}
	metadata.MergeOptions(options)
	return metadata, nil
}
