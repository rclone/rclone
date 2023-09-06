// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metaclient

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/common/encryption"
)

var errClass = errs.Class("metainfo")

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

// Close closes the underlying resources passed to the metainfo DB.
func (db *DB) Close() error {
	return db.metainfo.Close()
}

// CreateBucket creates a new bucket with the specified information.
func (db *DB) CreateBucket(ctx context.Context, bucketName string) (newBucket Bucket, err error) {
	defer mon.Task()(&ctx)(&err)

	if bucketName == "" {
		return Bucket{}, ErrNoBucket.New("")
	}

	newBucket, err = db.metainfo.CreateBucket(ctx, CreateBucketParams{
		Name: []byte(bucketName),
	})
	return newBucket, ErrBucket.Wrap(err)
}

// DeleteBucket deletes bucket.
func (db *DB) DeleteBucket(ctx context.Context, bucketName string, deleteAll bool) (bucket Bucket, err error) {
	defer mon.Task()(&ctx)(&err)

	if bucketName == "" {
		return Bucket{}, ErrNoBucket.New("")
	}

	bucket, err = db.metainfo.DeleteBucket(ctx, DeleteBucketParams{
		Name:      []byte(bucketName),
		DeleteAll: deleteAll,
	})
	return bucket, ErrBucket.Wrap(err)
}

// GetBucket gets bucket information.
func (db *DB) GetBucket(ctx context.Context, bucketName string) (bucket Bucket, err error) {
	defer mon.Task()(&ctx)(&err)

	if bucketName == "" {
		return Bucket{}, ErrNoBucket.New("")
	}

	bucket, err = db.metainfo.GetBucket(ctx, GetBucketParams{
		Name: []byte(bucketName),
	})
	return bucket, ErrBucket.Wrap(err)
}

// ListBuckets lists buckets.
func (db *DB) ListBuckets(ctx context.Context, options BucketListOptions) (bucketList BucketList, err error) {
	defer mon.Task()(&ctx)(&err)

	bucketList, err = db.metainfo.ListBuckets(ctx, ListBucketsParams{
		ListOpts: options,
	})
	return bucketList, ErrBucket.Wrap(err)
}

// IterateBucketsOptions buckets iterator options.
type IterateBucketsOptions struct {
	Cursor string
	Limit  int

	DialClientFunc func() (*Client, error)
}

// IterateBuckets returns iterator to go over buckets.
func IterateBuckets(ctx context.Context, options IterateBucketsOptions) *BucketIterator {
	defer mon.Task()(&ctx)(nil)

	opts := BucketListOptions{
		Direction: After,
		Cursor:    options.Cursor,
	}

	buckets := BucketIterator{
		ctx:            ctx,
		dialClientFunc: options.DialClientFunc,
		options:        opts,
	}

	return &buckets
}

// BucketIterator is an iterator over a collection of buckets.
type BucketIterator struct {
	ctx            context.Context
	dialClientFunc func() (*Client, error)
	options        BucketListOptions
	list           *BucketList
	position       int
	completed      bool
	err            error
}

// Next prepares next Bucket for reading.
// It returns false if the end of the iteration is reached and there are no more buckets, or if there is an error.
func (buckets *BucketIterator) Next() bool {
	if buckets.err != nil {
		buckets.completed = true
		return false
	}

	if buckets.list == nil {
		more := buckets.loadNext()
		buckets.completed = !more
		return more
	}

	if buckets.position >= len(buckets.list.Items)-1 {
		if !buckets.list.More {
			buckets.completed = true
			return false
		}
		more := buckets.loadNext()
		buckets.completed = !more
		return more
	}

	buckets.position++

	return true
}

func (buckets *BucketIterator) loadNext() bool {
	ok, err := buckets.tryLoadNext()
	if err != nil {
		buckets.err = err
		return false
	}
	return ok
}

func (buckets *BucketIterator) tryLoadNext() (ok bool, err error) {
	client, err := buckets.dialClientFunc()
	if err != nil {
		return false, err
	}
	defer func() { err = errs.Combine(err, client.Close()) }()

	list, err := client.ListBuckets(buckets.ctx, ListBucketsParams{
		ListOpts: buckets.options,
	})
	if err != nil {
		return false, err
	}
	buckets.list = &list
	if list.More {
		buckets.options = buckets.options.NextPage(list)
	}
	buckets.position = 0
	return len(list.Items) > 0, nil
}

// Err returns error, if one happened during iteration.
func (buckets *BucketIterator) Err() error {
	return buckets.err
}

// Item returns the current bucket in the iterator.
func (buckets *BucketIterator) Item() *Bucket {
	item := buckets.item()
	if item == nil {
		return nil
	}
	return &Bucket{
		Name:        item.Name,
		Created:     item.Created,
		Attribution: item.Attribution,
	}
}

func (buckets *BucketIterator) item() *Bucket {
	if buckets.completed {
		return nil
	}

	if buckets.err != nil {
		return nil
	}

	if buckets.list == nil {
		return nil
	}

	if len(buckets.list.Items) == 0 {
		return nil
	}

	return &buckets.list.Items[buckets.position]
}
