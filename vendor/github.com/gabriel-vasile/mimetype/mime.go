// Package mimetype uses magic number signatures
// to detect the MIME type and extension of a file.
package mimetype

import (
	"io"
	"os"

	"github.com/gabriel-vasile/mimetype/internal/matchers"
)

// Detect returns the MIME type and extension of the provided byte slice.
//
// mime is always a valid MIME type, with application/octet-stream as fallback.
// extension is empty string if detected file format does not have an extension.
func Detect(in []byte) (mime, extension string) {
	if len(in) == 0 {
		return "inode/x-empty", ""
	}
	n := root.match(in, root)
	return n.mime, n.extension
}

// DetectReader returns the MIME type and extension
// of the byte slice read from the provided reader.
//
// mime is always a valid MIME type, with application/octet-stream as fallback.
// extension is empty string if detection failed with an error or
// detected file format does not have an extension.
func DetectReader(r io.Reader) (mime, extension string, err error) {
	in := make([]byte, matchers.ReadLimit)
	n, err := r.Read(in)
	if err != nil && err != io.EOF {
		return root.mime, root.extension, err
	}
	in = in[:n]

	mime, extension = Detect(in)
	return mime, extension, nil
}

// DetectFile returns the MIME type and extension of the provided file.
//
// mime is always a valid MIME type, with application/octet-stream as fallback.
// extension is empty string if detection failed with an error or
// detected file format does not have an extension.
func DetectFile(file string) (mime, extension string, err error) {
	f, err := os.Open(file)
	if err != nil {
		return root.mime, root.extension, err
	}
	defer f.Close()

	return DetectReader(f)
}
