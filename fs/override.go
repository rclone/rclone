package fs

import "context"

// OverrideRemote is a wrapper to override the Remote for an
// ObjectInfo
type OverrideRemote struct {
	ObjectInfo
	remote string
}

// NewOverrideRemote returns an OverrideRemoteObject which will
// return the remote specified
func NewOverrideRemote(oi ObjectInfo, remote string) *OverrideRemote {
	// re-wrap an OverrideRemote
	if or, ok := oi.(*OverrideRemote); ok {
		return &OverrideRemote{
			ObjectInfo: or.ObjectInfo,
			remote:     remote,
		}
	}
	return &OverrideRemote{
		ObjectInfo: oi,
		remote:     remote,
	}
}

// Remote returns the overridden remote name
func (o *OverrideRemote) Remote() string {
	return o.remote
}

// String returns the overridden remote name
func (o *OverrideRemote) String() string {
	return o.remote
}

// MimeType returns the mime type of the underlying object or "" if it
// can't be worked out
func (o *OverrideRemote) MimeType(ctx context.Context) string {
	if do, ok := o.ObjectInfo.(MimeTyper); ok {
		return do.MimeType(ctx)
	}
	return ""
}

// ID returns the ID of the Object if known, or "" if not
func (o *OverrideRemote) ID() string {
	if do, ok := o.ObjectInfo.(IDer); ok {
		return do.ID()
	}
	return ""
}

// UnWrap returns the Object that this Object is wrapping or nil if it
// isn't wrapping anything
func (o *OverrideRemote) UnWrap() Object {
	if o, ok := o.ObjectInfo.(Object); ok {
		return o
	}
	return nil
}

// GetTier returns storage tier or class of the Object
func (o *OverrideRemote) GetTier() string {
	if do, ok := o.ObjectInfo.(GetTierer); ok {
		return do.GetTier()
	}
	return ""
}

// Metadata returns metadata for an object
//
// It should return nil if there is no Metadata
func (o *OverrideRemote) Metadata(ctx context.Context) (Metadata, error) {
	if do, ok := o.ObjectInfo.(Metadataer); ok {
		return do.Metadata(ctx)
	}
	return nil, nil
}
