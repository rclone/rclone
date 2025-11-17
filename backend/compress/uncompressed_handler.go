package compress

import (
	"context"
	"fmt"
	"io"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/chunkedreader"
)

// uncompressedModeHandler implements compressionModeHandler for uncompressed files
type uncompressedModeHandler struct{}

// isCompressible checks the compression ratio of the provided data and returns true if the ratio exceeds
// the configured threshold
func (u *uncompressedModeHandler) isCompressible(r io.Reader, compressionMode int) (bool, error) {
	return false, nil
}

// newObjectGetOriginalSize returns the original file size from the metadata
func (u *uncompressedModeHandler) newObjectGetOriginalSize(meta *ObjectMetadata) (int64, error) {
	return 0, nil
}

// openGetReadCloser opens a compressed object and returns a ReadCloser in the Open method
func (u *uncompressedModeHandler) openGetReadCloser(
	ctx context.Context,
	o *Object,
	offset int64,
	limit int64,
	cr chunkedreader.ChunkedReader,
	closer io.Closer,
	options ...fs.OpenOption,
) (rc io.ReadCloser, err error) {
	return o.Object.Open(ctx, options...)
}

// processFileNameGetFileExtension returns the file extension for the given compression mode
func (u *uncompressedModeHandler) processFileNameGetFileExtension(compressionMode int) string {
	return ""
}

// putCompress compresses the input data and uploads it to the remote, returning the new object and its metadata
func (u *uncompressedModeHandler) putCompress(
	ctx context.Context,
	f *Fs,
	in io.Reader,
	src fs.ObjectInfo,
	options []fs.OpenOption,
	mimeType string,
) (fs.Object, *ObjectMetadata, error) {
	return nil, nil, fmt.Errorf("unsupported compression mode %d", f.mode)
}

// putUncompressGetNewMetadata returns metadata in the putUncompress method for a specific compression algorithm
func (u *uncompressedModeHandler) putUncompressGetNewMetadata(o fs.Object, mode int, md5 string, mimeType string, sum []byte) (fs.Object, *ObjectMetadata, error) {
	return nil, nil, fmt.Errorf("unsupported compression mode %d", Uncompressed)
}

// This function generates a metadata object for sgzip.GzipMetadata or SzstdMetadata.
// Warning: This function panics if cmeta is not of the expected type.
func (u *uncompressedModeHandler) newMetadata(size int64, mode int, cmeta any, md5 string, mimeType string) *ObjectMetadata {
	return nil
}
