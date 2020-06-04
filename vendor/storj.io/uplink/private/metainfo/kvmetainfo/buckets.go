// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kvmetainfo

import (
	"context"

	"storj.io/common/storj"
	"storj.io/uplink/private/metainfo"
)

// CreateBucket creates a new bucket.
func (db *Project) CreateBucket(ctx context.Context, bucketName string) (newBucket storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)

	if bucketName == "" {
		return storj.Bucket{}, storj.ErrNoBucket.New("")
	}

	newBucket, err = db.metainfo.CreateBucket(ctx, metainfo.CreateBucketParams{
		Name: []byte(bucketName),
	})
	return newBucket, storj.ErrBucket.Wrap(err)
}

// DeleteBucket deletes bucket.
func (db *Project) DeleteBucket(ctx context.Context, bucketName string) (bucket storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)

	if bucketName == "" {
		return storj.Bucket{}, storj.ErrNoBucket.New("")
	}

	bucket, err = db.metainfo.DeleteBucket(ctx, metainfo.DeleteBucketParams{
		Name: []byte(bucketName),
	})
	return bucket, storj.ErrBucket.Wrap(err)
}

// GetBucket gets bucket information.
func (db *Project) GetBucket(ctx context.Context, bucketName string) (bucket storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)

	if bucketName == "" {
		return storj.Bucket{}, storj.ErrNoBucket.New("")
	}

	bucket, err = db.metainfo.GetBucket(ctx, metainfo.GetBucketParams{
		Name: []byte(bucketName),
	})
	return bucket, storj.ErrBucket.Wrap(err)
}

// ListBuckets lists buckets.
func (db *Project) ListBuckets(ctx context.Context, listOpts storj.BucketListOptions) (bucketList storj.BucketList, err error) {
	defer mon.Task()(&ctx)(&err)

	bucketList, err = db.metainfo.ListBuckets(ctx, metainfo.ListBucketsParams{
		ListOpts: listOpts,
	})
	return bucketList, storj.ErrBucket.Wrap(err)
}
