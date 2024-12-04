package webdav

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

// Metadata is a typedef for a string to string map to hold metadata
type Metadata map[string]string

// Upload is a struct containing the file status during upload
type Upload struct {
	stream io.ReadSeeker
	size   int64
	offset int64

	Fingerprint string
	Metadata    Metadata
}

// Updates the Upload information based on offset.
func (u *Upload) updateProgress(offset int64) {
	u.offset = offset
}

// Finished returns whether this upload is finished or not.
func (u *Upload) Finished() bool {
	return u.offset >= u.size
}

// Progress returns the progress in a percentage.
func (u *Upload) Progress() int64 {
	return (u.offset * 100) / u.size
}

// Offset returns the current upload offset.
func (u *Upload) Offset() int64 {
	return u.offset
}

// Size returns the size of the upload body.
func (u *Upload) Size() int64 {
	return u.size
}

// EncodedMetadata encodes the upload metadata.
func (u *Upload) EncodedMetadata() string {
	var encoded []string

	for k, v := range u.Metadata {
		encoded = append(encoded, fmt.Sprintf("%s %s", k, b64encode(v)))
	}

	return strings.Join(encoded, ",")
}

func b64encode(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

// NewUpload creates a new upload from an io.Reader.
func NewUpload(reader io.Reader, size int64, metadata Metadata, fingerprint string) *Upload {
	stream, ok := reader.(io.ReadSeeker)

	if !ok {
		buf := new(bytes.Buffer)
		_, err := buf.ReadFrom(reader)
		if err != nil {
			return nil
		}
		stream = bytes.NewReader(buf.Bytes())
	}

	if metadata == nil {
		metadata = make(Metadata)
	}

	return &Upload{
		stream: stream,
		size:   size,

		Fingerprint: fingerprint,
		Metadata:    metadata,
	}
}
