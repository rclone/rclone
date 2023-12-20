package fs

import (
	"context"
	"mime"
	"path"
	"strings"
)

// Add a minimal number of mime types to augment go's built in types
// for environments which don't have access to a mime.types file (e.g.
// Termux on android)
func init() {
	for _, t := range []struct {
		mimeType   string
		extensions string
	}{
		{"audio/flac", ".flac"},
		{"audio/mpeg", ".mpga,.mpega,.mp2,.mp3,.m4a"},
		{"audio/ogg", ".oga,.ogg,.opus,.spx"},
		{"audio/x-wav", ".wav"},
		{"image/tiff", ".tiff,.tif"},
		{"video/dv", ".dif,.dv"},
		{"video/fli", ".fli"},
		{"video/mpeg", ".mpeg,.mpg,.mpe"},
		{"video/MP2T", ".ts"},
		{"video/mp4", ".mp4"},
		{"video/quicktime", ".qt,.mov"},
		{"video/ogg", ".ogv"},
		{"video/webm", ".webm"},
		{"video/x-msvideo", ".avi"},
		{"video/x-matroska", ".mpv,.mkv"},
		{"application/x-subrip", ".srt"},
	} {
		for _, ext := range strings.Split(t.extensions, ",") {
			if mime.TypeByExtension(ext) == "" {
				err := mime.AddExtensionType(ext, t.mimeType)
				if err != nil {
					panic(err)
				}
			}
		}
	}
}

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
func MimeType(ctx context.Context, o DirEntry) (mimeType string) {
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
