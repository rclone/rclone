package s3sizes

import (
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

// Constants
const (
	maxFileSize = 5 * 1024 * 1024 * 1024 * 1024 // largest possible upload file size
)

// PartSize returns a recommended partition size for S3 uploads, based on the
// totalSize of the file; specify a totalSize of -1 when the file size is unknown.
//
// The returned partition size is also recommended for calculating the S3 ETag hash.
func PartSize(totalSize int64) int64 {
	partSize := s3manager.MinUploadPartSize

	if totalSize == -1 {
		// Make parts as small as possible while still being able to upload to the
		// S3 file size limit. Rounded up to nearest MB.
		partSize = (((maxFileSize / s3manager.MaxUploadParts) >> 20) + 1) << 20
	} else if totalSize/partSize >= s3manager.MaxUploadParts {
		// Adjust PartSize until the number of parts is small enough.
		// Calculate partition size rounded up to the nearest MB
		partSize = (((totalSize / s3manager.MaxUploadParts) >> 20) + 1) << 20
	}

	return partSize
}
