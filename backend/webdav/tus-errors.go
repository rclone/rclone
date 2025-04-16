package webdav

import (
	"errors"
	"fmt"
)

var (
	// ErrChunkSize is returned when the chunk size is zero
	ErrChunkSize = errors.New("tus chunk size must be greater than zero")
	// ErrNilLogger is returned when the logger is nil
	ErrNilLogger = errors.New("tus logger can't be nil")
	// ErrNilStore is returned when the store is nil
	ErrNilStore = errors.New("tus store can't be nil if resume is enable")
	// ErrNilUpload is returned when the upload is nil
	ErrNilUpload = errors.New("tus upload can't be nil")
	// ErrLargeUpload is returned when the upload body is to large
	ErrLargeUpload = errors.New("tus upload body is to large")
	// ErrVersionMismatch is returned when the tus protocol version is mismatching
	ErrVersionMismatch = errors.New("tus protocol version mismatch")
	// ErrOffsetMismatch is returned when the tus upload offset is mismatching
	ErrOffsetMismatch = errors.New("tus upload offset mismatch")
	// ErrUploadNotFound is returned when the tus upload is not found
	ErrUploadNotFound = errors.New("tus upload not found")
	// ErrResumeNotEnabled is returned when the tus resuming is not enabled
	ErrResumeNotEnabled = errors.New("tus resuming not enabled")
	// ErrFingerprintNotSet is returned when the tus fingerprint is not set
	ErrFingerprintNotSet = errors.New("tus fingerprint not set")
)

// ClientError represents an error state of a client
type ClientError struct {
	Code int
	Body []byte
}

// Error returns an error string containing the client error code
func (c ClientError) Error() string {
	return fmt.Sprintf("unexpected status code: %d", c.Code)
}
