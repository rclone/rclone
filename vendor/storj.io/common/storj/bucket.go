// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

import (
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/uuid"
)

var (
	// ErrBucket is an error class for general bucket errors
	ErrBucket = errs.Class("bucket")

	// ErrNoBucket is an error class for using empty bucket name
	ErrNoBucket = errs.Class("no bucket specified")

	// ErrBucketNotFound is an error class for non-existing bucket
	ErrBucketNotFound = errs.Class("bucket not found")
)

// Bucket contains information about a specific bucket
type Bucket struct {
	ID                          uuid.UUID
	Name                        string
	ProjectID                   uuid.UUID
	PartnerID                   uuid.UUID
	Created                     time.Time
	PathCipher                  CipherSuite
	DefaultSegmentsSize         int64
	DefaultRedundancyScheme     RedundancyScheme
	DefaultEncryptionParameters EncryptionParameters
}
