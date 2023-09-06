// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metaclient

import (
	"time"

	"storj.io/common/storj"
)

// CreateObject has optional parameters that can be set.
type CreateObject struct {
	Metadata    map[string]string
	ContentType string
	Expires     time.Time

	storj.RedundancyScheme
	storj.EncryptionParameters
}

// Object converts the CreateObject to an object with unitialized values.
func (create CreateObject) Object(bucket Bucket, path string) Object {
	return Object{
		Bucket: Bucket{
			Name:    bucket.Name,
			Created: bucket.Created,
		},
		Path:        path,
		Metadata:    create.Metadata,
		ContentType: create.ContentType,
		Expires:     create.Expires,
		Stream: Stream{
			Size:             -1, // unknown
			SegmentCount:     -1, // unknown
			FixedSegmentSize: -1, // unknown

			RedundancyScheme:     create.RedundancyScheme,
			EncryptionParameters: create.EncryptionParameters,
		},
	}
}
