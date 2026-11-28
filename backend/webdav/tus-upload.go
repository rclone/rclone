package webdav

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"
)

// Metadata is a typedef for a string to string map to hold metadata
type Metadata map[string]string

// Upload is a struct containing the file status during upload
type Upload struct {
	stream io.ReadSeeker
	size   int64
	offset int64
	file   *os.File

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
func NewUpload(reader io.Reader, size int64, metadata Metadata, fingerprint string) (*Upload, error) {
	stream, ok := reader.(io.ReadSeeker)
	var file *os.File

	if !ok {
		var err error
		file, err = os.CreateTemp("", "rclone-tus")
		if err != nil {
			return nil, fmt.Errorf("failed to create tus upload spool: %w", err)
		}
		if _, err = io.Copy(file, reader); err != nil {
			_ = file.Close()
			_ = os.Remove(file.Name())
			return nil, fmt.Errorf("failed to spool tus upload: %w", err)
		}
		if _, err = file.Seek(0, io.SeekStart); err != nil {
			_ = file.Close()
			_ = os.Remove(file.Name())
			return nil, fmt.Errorf("failed to rewind tus upload spool: %w", err)
		}
		stream = file
	}

	if metadata == nil {
		metadata = make(Metadata)
	}

	return &Upload{
		stream: stream,
		size:   size,
		file:   file,

		Fingerprint: fingerprint,
		Metadata:    metadata,
	}, nil
}

// Close removes the temporary spool file for u, if one was created.
func (u *Upload) Close() error {
	if u.file == nil {
		return nil
	}
	name := u.file.Name()
	err := u.file.Close()
	removeErr := os.Remove(name)
	u.file = nil
	if err != nil {
		return err
	}
	return removeErr
}
