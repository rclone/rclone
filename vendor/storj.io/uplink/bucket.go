// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"context"
	"errors"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/errs2"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/uplink/private/metaclient"
)

// ErrBucketNameInvalid is returned when the bucket name is invalid.
var ErrBucketNameInvalid = errors.New("bucket name invalid")

// ErrBucketAlreadyExists is returned when the bucket already exists during creation.
var ErrBucketAlreadyExists = errors.New("bucket already exists")

// ErrBucketNotEmpty is returned when the bucket is not empty during deletion.
var ErrBucketNotEmpty = errors.New("bucket not empty")

// ErrBucketNotFound is returned when the bucket is not found.
var ErrBucketNotFound = errors.New("bucket not found")

// Bucket contains information about the bucket.
type Bucket struct {
	Name    string
	Created time.Time
}

// StatBucket returns information about a bucket.
func (project *Project) StatBucket(ctx context.Context, bucket string) (info *Bucket, err error) {
	defer mon.Task()(&ctx)(&err)

	db, err := project.dialMetainfoDB(ctx)
	if err != nil {
		return nil, convertKnownErrors(err, bucket, "")
	}
	defer func() { err = errs.Combine(err, db.Close()) }()

	b, err := db.GetBucket(ctx, bucket)
	if err != nil {
		return nil, convertKnownErrors(err, bucket, "")
	}

	return &Bucket{
		Name:    b.Name,
		Created: b.Created,
	}, nil
}

// CreateBucket creates a new bucket.
//
// When bucket already exists it returns a valid Bucket and ErrBucketExists.
func (project *Project) CreateBucket(ctx context.Context, bucket string) (created *Bucket, err error) {
	defer mon.Task()(&ctx)(&err)

	db, err := project.dialMetainfoDB(ctx)
	if err != nil {
		return nil, convertKnownErrors(err, bucket, "")
	}
	defer func() { err = errs.Combine(err, db.Close()) }()

	b, err := db.CreateBucket(ctx, bucket)
	if err != nil {
		if metaclient.ErrNoBucket.Has(err) {
			return nil, errwrapf("%w (%q)", ErrBucketNameInvalid, bucket)
		}
		if errs2.IsRPC(err, rpcstatus.AlreadyExists) {
			// TODO: Ideally, the satellite should return the existing bucket when this error occurs.
			existing, err := project.StatBucket(ctx, bucket)
			if err != nil {
				return existing, errs.Combine(errwrapf("%w (%q)", ErrBucketAlreadyExists, bucket), convertKnownErrors(err, bucket, ""))
			}
			return existing, errwrapf("%w (%q)", ErrBucketAlreadyExists, bucket)
		}
		if errs2.IsRPC(err, rpcstatus.InvalidArgument) {
			return nil, errwrapf("%w (%q)", ErrBucketNameInvalid, bucket)
		}
		return nil, convertKnownErrors(err, bucket, "")
	}

	return &Bucket{
		Name:    b.Name,
		Created: b.Created,
	}, nil
}

// EnsureBucket ensures that a bucket exists or creates a new one.
//
// When bucket already exists it returns a valid Bucket and no error.
func (project *Project) EnsureBucket(ctx context.Context, bucket string) (ensured *Bucket, err error) {
	defer mon.Task()(&ctx)(&err)

	ensured, err = project.CreateBucket(ctx, bucket)
	if err != nil && !errors.Is(err, ErrBucketAlreadyExists) {
		return nil, convertKnownErrors(err, bucket, "")
	}

	return ensured, nil
}

// DeleteBucket deletes a bucket.
//
// When bucket is not empty it returns ErrBucketNotEmpty.
func (project *Project) DeleteBucket(ctx context.Context, bucket string) (deleted *Bucket, err error) {
	defer mon.Task()(&ctx)(&err)

	db, err := project.dialMetainfoDB(ctx)
	if err != nil {
		return nil, convertKnownErrors(err, bucket, "")
	}
	defer func() { err = errs.Combine(err, db.Close()) }()

	existing, err := db.DeleteBucket(ctx, bucket, false)
	if err != nil {
		if errs2.IsRPC(err, rpcstatus.FailedPrecondition) {
			return nil, errwrapf("%w (%q)", ErrBucketNotEmpty, bucket)
		}
		return nil, convertKnownErrors(err, bucket, "")
	}

	if len(existing.Name) == 0 {
		return &Bucket{Name: bucket}, nil
	}

	return &Bucket{
		Name:    existing.Name,
		Created: existing.Created,
	}, nil
}

// DeleteBucketWithObjects deletes a bucket and all objects within that bucket.
func (project *Project) DeleteBucketWithObjects(ctx context.Context, bucket string) (deleted *Bucket, err error) {
	defer mon.Task()(&ctx)(&err)

	db, err := project.dialMetainfoDB(ctx)
	if err != nil {
		return nil, convertKnownErrors(err, bucket, "")
	}
	defer func() { err = errs.Combine(err, db.Close()) }()

	existing, err := db.DeleteBucket(ctx, bucket, true)
	if err != nil {
		return nil, convertKnownErrors(err, bucket, "")
	}

	if len(existing.Name) == 0 {
		return &Bucket{Name: bucket}, nil
	}

	return &Bucket{
		Name:    existing.Name,
		Created: existing.Created,
	}, nil
}
