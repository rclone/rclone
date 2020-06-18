// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/common/encryption"
	"storj.io/common/storj"
)

var errClass = errs.Class("metainfo")

const defaultSegmentLimit = 8 // TODO

// DB implements metainfo database.
type DB struct {
	metainfo *Client

	encStore *encryption.Store
}

// New creates a new metainfo database.
func New(metainfo *Client, encStore *encryption.Store) *DB {
	return &DB{
		metainfo: metainfo,
		encStore: encStore,
	}
}

// CreateBucket creates a new bucket with the specified information.
func (db *DB) CreateBucket(ctx context.Context, bucketName string) (newBucket storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)

	if bucketName == "" {
		return storj.Bucket{}, storj.ErrNoBucket.New("")
	}

	newBucket, err = db.metainfo.CreateBucket(ctx, CreateBucketParams{
		Name: []byte(bucketName),
	})
	return newBucket, storj.ErrBucket.Wrap(err)
}

// DeleteBucket deletes bucket.
func (db *DB) DeleteBucket(ctx context.Context, bucketName string) (bucket storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)

	if bucketName == "" {
		return storj.Bucket{}, storj.ErrNoBucket.New("")
	}

	bucket, err = db.metainfo.DeleteBucket(ctx, DeleteBucketParams{
		Name: []byte(bucketName),
	})
	return bucket, storj.ErrBucket.Wrap(err)
}

// GetBucket gets bucket information.
func (db *DB) GetBucket(ctx context.Context, bucketName string) (bucket storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)

	if bucketName == "" {
		return storj.Bucket{}, storj.ErrNoBucket.New("")
	}

	bucket, err = db.metainfo.GetBucket(ctx, GetBucketParams{
		Name: []byte(bucketName),
	})
	return bucket, storj.ErrBucket.Wrap(err)
}

// ListBuckets lists buckets.
func (db *DB) ListBuckets(ctx context.Context, options storj.BucketListOptions) (bucketList storj.BucketList, err error) {
	defer mon.Task()(&ctx)(&err)

	bucketList, err = db.metainfo.ListBuckets(ctx, ListBucketsParams{
		ListOpts: options,
	})
	return bucketList, storj.ErrBucket.Wrap(err)
}
