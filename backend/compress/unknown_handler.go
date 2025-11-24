package compress

import (
	"context"
	"fmt"
	"io"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/chunkedreader"
)

// unknownModeHandler implements compressionModeHandler for unknown compression types
type unknownModeHandler struct{}

// isCompressible checks the compression ratio of the provided data and returns true if the ratio exceeds
// the configured threshold
func (unk *unknownModeHandler) isCompressible(r io.Reader, compressionMode int) (bool, error) {
	return false, fmt.Errorf("unknown compression mode %d", compressionMode)
}

// newObjectGetOriginalSize returns the original file size from the metadata
func (unk *unknownModeHandler) newObjectGetOriginalSize(meta *ObjectMetadata) (int64, error) {
	return 0, nil
}

// openGetReadCloser opens a compressed object and returns a ReadCloser in the Open method
func (unk *unknownModeHandler) openGetReadCloser(
	ctx context.Context,
	o *Object,
	offset int64,
	limit int64,
	cr chunkedreader.ChunkedReader,
	closer io.Closer,
	options ...fs.OpenOption,
) (rc io.ReadCloser, err error) {
	return nil, fmt.Errorf("unknown compression mode %d", o.meta.Mode)
}

// processFileNameGetFileExtension returns the file extension for the given compression mode
func (unk *unknownModeHandler) processFileNameGetFileExtension(compressionMode int) string {
	return ""
}

// putCompress compresses the input data and uploads it to the remote, returning the new object and its metadata
func (unk *unknownModeHandler) putCompress(
	ctx context.Context,
	f *Fs,
	in io.Reader,
	src fs.ObjectInfo,
	options []fs.OpenOption,
	mimeType string,
) (fs.Object, *ObjectMetadata, error) {
	return nil, nil, fmt.Errorf("unknown compression mode %d", f.mode)
}

// putUncompressGetNewMetadata returns metadata in the putUncompress method for a specific compression algorithm
func (unk *unknownModeHandler) putUncompressGetNewMetadata(o fs.Object, mode int, md5 string, mimeType string, sum []byte) (fs.Object, *ObjectMetadata, error) {
	return nil, nil, fmt.Errorf("unknown compression mode")
}

// This function generates a metadata object for sgzip.GzipMetadata or SzstdMetadata.
// Warning: This function panics if cmeta is not of the expected type.
func (unk *unknownModeHandler) newMetadata(size int64, mode int, cmeta any, md5 string, mimeType string) *ObjectMetadata {
	return nil
}
