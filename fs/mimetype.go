package fs

import (
	"context"
	"mime"
	"path"
	"strings"
)

// MimeTypeFromName returns a guess at the mime type from the name
func MimeTypeFromName(remote string) (mimeType string) {
	mimeType = mime.TypeByExtension(path.Ext(remote))
	if !strings.ContainsRune(mimeType, '/') {
		mimeType = "application/octet-stream"
	}
	return mimeType
}

// MimeType returns the MimeType from the object, either by calling
// the MimeTyper interface or using MimeTypeFromName
func MimeType(ctx context.Context, o ObjectInfo) (mimeType string) {
	// Read the MimeType from the optional interface if available
	if do, ok := o.(MimeTyper); ok {
		mimeType = do.MimeType(ctx)
		// Debugf(o, "Read MimeType as %q", mimeType)
		if mimeType != "" {
			return mimeType
		}
	}
	return MimeTypeFromName(o.Remote())
}

// MimeTypeDirEntry returns the MimeType of a DirEntry
//
// It returns "inode/directory" for directories, or uses
// MimeType(Object)
func MimeTypeDirEntry(ctx context.Context, item DirEntry) string {
	switch x := item.(type) {
	case Object:
		return MimeType(ctx, x)
	case Directory:
		return "inode/directory"
	}
	return ""
}
