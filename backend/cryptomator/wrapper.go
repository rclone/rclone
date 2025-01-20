package cryptomator

import (
	"context"
	"errors"
	"time"

	"github.com/rclone/rclone/fs"
)

var (
	errorNotSupportedByUnderlyingRemote = errors.New("not supported by underlying remote")
)

// UnWrap returns the Fs that this Fs is wrapping
func (f *Fs) UnWrap() fs.Fs { return f.wrapped }

// WrapFs returns the Fs that is wrapping this Fs
func (f *Fs) WrapFs() fs.Fs { return f.wrapper }

// SetWrapper sets the Fs that is wrapping this Fs
func (f *Fs) SetWrapper(wrapper fs.Fs) { f.wrapper = wrapper }

// DirCacheFlush resets the directory cache - used in testing
// as an optional interface
func (f *Fs) DirCacheFlush() {
	do := f.wrapper.Features().DirCacheFlush
	if do != nil {
		do()
	}
	f.dirCache.Flush()
}

// PublicLink generates a public link to the remote path (usually readable by anyone)
func (f *Fs) PublicLink(ctx context.Context, remote string, expire fs.Duration, unlink bool) (string, error) {
	do := f.wrapped.Features().PublicLink
	if do == nil {
		return "", errorNotSupportedByUnderlyingRemote
	}
	leaf, dirID, err := f.dirCache.FindPath(ctx, remote, false)
	if err != nil {
		return "", err
	}
	encryptedPath := f.leafPath(leaf, dirID)
	return do(ctx, encryptedPath, expire, unlink)
}

// DirSetModTime sets the directory modtime for dir
func (f *Fs) DirSetModTime(ctx context.Context, dir string, modTime time.Time) error {
	do := f.wrapped.Features().DirSetModTime
	if do == nil {
		return errorNotSupportedByUnderlyingRemote
	}
	dirID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return err
	}
	return do(ctx, f.dirIDPath(dirID), modTime)
}

// CleanUp the trash in the Fs
func (f *Fs) CleanUp(ctx context.Context) error {
	do := f.wrapped.Features().CleanUp
	if do == nil {
		return errorNotSupportedByUnderlyingRemote
	}
	return do(ctx)
}

// About gets quota information from the Fs
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	do := f.wrapped.Features().About
	if do == nil {
		return nil, errorNotSupportedByUnderlyingRemote
	}
	return do(ctx)
}

// UserInfo returns info about the connected user
func (f *Fs) UserInfo(ctx context.Context) (map[string]string, error) {
	do := f.wrapped.Features().UserInfo
	if do == nil {
		return nil, errorNotSupportedByUnderlyingRemote
	}
	return do(ctx)
}

// Disconnect the current user
func (f *Fs) Disconnect(ctx context.Context) error {
	do := f.wrapped.Features().Disconnect
	if do == nil {
		return errorNotSupportedByUnderlyingRemote
	}
	return do(ctx)
}

// Shutdown the backend, closing any background tasks and any
// cached connections.
func (f *Fs) Shutdown(ctx context.Context) error {
	do := f.wrapped.Features().Shutdown
	if do == nil {
		return nil
	}
	return do(ctx)
}

// MimeType returns the content type of the Object if
// known, or "" if not
//
// This is deliberately unsupported so we don't leak mime type info by
// default.
func (o *DecryptingObject) MimeType(ctx context.Context) string {
	return ""
}

// ID returns the ID of the Object if known, or "" if not
func (o *DecryptingObject) ID() string {
	do, ok := o.Object.(fs.IDer)
	if !ok {
		return ""
	}
	return do.ID()
}

// ParentID returns the ID of the parent directory if known or nil if not
func (o *DecryptingObject) ParentID() string {
	do, ok := o.Object.(fs.ParentIDer)
	if !ok {
		return ""
	}
	return do.ParentID()
}

// UnWrap returns the Object that this Object is wrapping or
// nil if it isn't wrapping anything
func (o *DecryptingObject) UnWrap() fs.Object { return o.Object }

// SetTier performs changing storage tier of the Object if
// multiple storage classes supported
func (o *DecryptingObject) SetTier(tier string) error {
	do, ok := o.Object.(fs.SetTierer)
	if !ok {
		return errorNotSupportedByUnderlyingRemote
	}
	return do.SetTier(tier)
}

// GetTier returns storage tier or class of the Object
func (o *DecryptingObject) GetTier() string {
	do, ok := o.Object.(fs.GetTierer)
	if !ok {
		return ""
	}
	return do.GetTier()
}

// Metadata returns metadata for an object
//
// It should return nil if there is no Metadata
func (o *DecryptingObject) Metadata(ctx context.Context) (fs.Metadata, error) {
	do, ok := o.Object.(fs.Metadataer)
	if !ok {
		return nil, nil
	}
	return do.Metadata(ctx)
}

// SetMetadata sets metadata for an Object
//
// It should return fs.ErrorNotImplemented if it can't set metadata
func (o *DecryptingObject) SetMetadata(ctx context.Context, metadata fs.Metadata) error {
	do, ok := o.Object.(fs.SetMetadataer)
	if !ok {
		return fs.ErrorNotImplemented
	}
	return do.SetMetadata(ctx, metadata)
}

// UnWrap returns the Object that this Object is wrapping or
// nil if it isn't wrapping anything
func (i *EncryptingObjectInfo) UnWrap() fs.Object {
	return fs.UnWrapObjectInfo(i.ObjectInfo)
}

// MimeType returns the content type of the Object if
// known, or "" if not
//
// This is deliberately unsupported so we don't leak mime type info by
// default.
func (i *EncryptingObjectInfo) MimeType(ctx context.Context) string {
	return ""
}

// ID returns the ID of the Object if known, or "" if not
func (i *EncryptingObjectInfo) ID() string {
	do, ok := i.ObjectInfo.(fs.IDer)
	if !ok {
		return ""
	}
	return do.ID()
}

// GetTier returns storage tier or class of the Object
func (i *EncryptingObjectInfo) GetTier() string {
	do, ok := i.ObjectInfo.(fs.GetTierer)
	if !ok {
		return ""
	}
	return do.GetTier()
}

// Metadata returns metadata for an object
//
// It should return nil if there is no Metadata
func (i *EncryptingObjectInfo) Metadata(ctx context.Context) (fs.Metadata, error) {
	do, ok := i.ObjectInfo.(fs.Metadataer)
	if !ok {
		return nil, nil
	}
	return do.Metadata(ctx)
}

// Check the interfaces are satisfied
var (
	_ fs.UnWrapper       = (*Fs)(nil)
	_ fs.Wrapper         = (*Fs)(nil)
	_ fs.DirCacheFlusher = (*Fs)(nil)
	_ fs.PublicLinker    = (*Fs)(nil)
	_ fs.DirSetModTimer  = (*Fs)(nil)
	_ fs.CleanUpper      = (*Fs)(nil)
	_ fs.Abouter         = (*Fs)(nil)
	_ fs.UserInfoer      = (*Fs)(nil)
	_ fs.Disconnecter    = (*Fs)(nil)
	_ fs.Shutdowner      = (*Fs)(nil)

	_ fs.FullObject     = (*DecryptingObject)(nil)
	_ fs.FullObjectInfo = (*EncryptingObjectInfo)(nil)
	_ fs.FullDirectory  = (*Directory)(nil)
)
