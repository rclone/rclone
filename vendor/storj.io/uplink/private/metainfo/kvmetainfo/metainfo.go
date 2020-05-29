// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kvmetainfo

import (
	"context"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"

	"storj.io/common/encryption"
	"storj.io/common/storj"
	"storj.io/uplink/private/metainfo"
	"storj.io/uplink/private/storage/segments"
	"storj.io/uplink/private/storage/streams"
)

var mon = monkit.Package()

var errClass = errs.Class("kvmetainfo")

const defaultSegmentLimit = 8 // TODO

// DB implements metainfo database.
type DB struct {
	project *Project

	metainfo *metainfo.Client

	streams  streams.Store
	segments segments.Store

	encStore *encryption.Store
}

// New creates a new metainfo database.
func New(project *Project, metainfo *metainfo.Client, streams streams.Store, segments segments.Store, encStore *encryption.Store) *DB {
	return &DB{
		project:  project,
		metainfo: metainfo,
		streams:  streams,
		segments: segments,
		encStore: encStore,
	}
}

// CreateBucket creates a new bucket with the specified information.
func (db *DB) CreateBucket(ctx context.Context, bucketName string, info *storj.Bucket) (bucketInfo storj.Bucket, err error) {
	return db.project.CreateBucket(ctx, bucketName, info)
}

// DeleteBucket deletes bucket.
func (db *DB) DeleteBucket(ctx context.Context, bucketName string) (_ storj.Bucket, err error) {
	return db.project.DeleteBucket(ctx, bucketName)
}

// GetBucket gets bucket information.
func (db *DB) GetBucket(ctx context.Context, bucketName string) (bucketInfo storj.Bucket, err error) {
	return db.project.GetBucket(ctx, bucketName)
}

// ListBuckets lists buckets.
func (db *DB) ListBuckets(ctx context.Context, options storj.BucketListOptions) (list storj.BucketList, err error) {
	return db.project.ListBuckets(ctx, options)
}
