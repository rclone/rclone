package webdav

import (
	"errors"
	"fmt"
)

var (
	ErrChuckSize         = errors.New("tus chunk size must be greater than zero")
	ErrNilLogger         = errors.New("tus logger can't be nil")
	ErrNilStore          = errors.New("tus store can't be nil if Resume is enable")
	ErrNilUpload         = errors.New("tus upload can't be nil")
	ErrLargeUpload       = errors.New("tus upload body is to large")
	ErrVersionMismatch   = errors.New("tus protocol version mismatch")
	ErrOffsetMismatch    = errors.New("tus upload offset mismatch")
	ErrUploadNotFound    = errors.New("tus upload not found")
	ErrResumeNotEnabled  = errors.New("tus resuming not enabled")
	ErrFingerprintNotSet = errors.New("tus fingerprint not set")
)

type ClientError struct {
	Code int
	Body []byte
}

func (c ClientError) Error() string {
	return fmt.Sprintf("unexpected status code: %d", c.Code)
}
