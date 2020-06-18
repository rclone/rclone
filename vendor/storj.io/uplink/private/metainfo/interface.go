// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"
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
func (create CreateObject) Object(bucket storj.Bucket, path storj.Path) storj.Object {
	return storj.Object{
		Bucket:      bucket,
		Path:        path,
		Metadata:    create.Metadata,
		ContentType: create.ContentType,
		Expires:     create.Expires,
		Stream: storj.Stream{
			Size:             -1,  // unknown
			Checksum:         nil, // unknown
			SegmentCount:     -1,  // unknown
			FixedSegmentSize: -1,  // unknown

			RedundancyScheme:     create.RedundancyScheme,
			EncryptionParameters: create.EncryptionParameters,
		},
	}
}

// ReadOnlyStream is an interface for reading segment information.
type ReadOnlyStream interface {
	Info() storj.Object

	// SegmentsAt returns the segment that contains the byteOffset and following segments.
	// Limit specifies how much to return at most.
	SegmentsAt(ctx context.Context, byteOffset int64, limit int64) (infos []storj.Segment, more bool, err error)
	// Segments returns the segment at index.
	// Limit specifies how much to return at most.
	Segments(ctx context.Context, index int64, limit int64) (infos []storj.Segment, more bool, err error)
}

// MutableObject is an interface for manipulating creating/deleting object stream.
type MutableObject interface {
	// Info gets the current information about the object.
	Info() storj.Object

	// CreateStream creates a new stream for the object.
	CreateStream(ctx context.Context) (MutableStream, error)
	// ContinueStream starts to continue a partially uploaded stream.
	ContinueStream(ctx context.Context) (MutableStream, error)
	// DeleteStream deletes any information about this objects stream.
	DeleteStream(ctx context.Context) error

	// Commit commits the changes to the database.
	Commit(ctx context.Context) error
}

// MutableStream is an interface for manipulating stream information.
type MutableStream interface {
	BucketName() string
	Path() string

	Expires() time.Time
	Metadata() ([]byte, error)

	// AddSegments adds segments to the stream.
	AddSegments(ctx context.Context, segments ...storj.Segment) error
	// UpdateSegments updates information about segments.
	UpdateSegments(ctx context.Context, segments ...storj.Segment) error
}
