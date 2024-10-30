package webdav

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

type Metadata map[string]string

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

// Returns whether this upload is finished or not.
func (u *Upload) Finished() bool {
	return u.offset >= u.size
}

// Returns the progress in a percentage.
func (u *Upload) Progress() int64 {
	return (u.offset * 100) / u.size
}

// Returns the current upload offset.
func (u *Upload) Offset() int64 {
	return u.offset
}

// Returns the size of the upload body.
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
		buf.ReadFrom(reader)
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
