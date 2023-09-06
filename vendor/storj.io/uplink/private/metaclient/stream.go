// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metaclient

import (
	"time"

	"storj.io/common/pb"
)

// MutableStream is for manipulating stream information.
type MutableStream struct {
	info Object

	dynamic         bool
	dynamicMetadata SerializableMeta
	dynamicExpires  time.Time
}

// SerializableMeta is an interface for getting pb.SerializableMeta.
type SerializableMeta interface {
	Metadata() ([]byte, error)
}

// BucketName returns streams bucket name.
func (stream *MutableStream) BucketName() string { return stream.info.Bucket.Name }

// Path returns streams path.
func (stream *MutableStream) Path() string { return stream.info.Path }

// Info returns object info about the stream.
func (stream *MutableStream) Info() Object { return stream.info }

// Expires returns stream expiration time.
func (stream *MutableStream) Expires() time.Time {
	if stream.dynamic {
		return stream.dynamicExpires
	}
	return stream.info.Expires
}

// Metadata returns metadata associated with the stream.
func (stream *MutableStream) Metadata() ([]byte, error) {
	if stream.dynamic {
		return stream.dynamicMetadata.Metadata()
	}

	if stream.info.ContentType != "" {
		if stream.info.Metadata == nil {
			stream.info.Metadata = make(map[string]string)
			stream.info.Metadata[contentTypeKey] = stream.info.ContentType
		} else if _, found := stream.info.Metadata[contentTypeKey]; !found {
			stream.info.Metadata[contentTypeKey] = stream.info.ContentType
		}
	}
	if stream.info.Metadata == nil {
		return []byte{}, nil
	}
	return pb.Marshal(&pb.SerializableMeta{
		UserDefined: stream.info.Metadata,
	})
}
