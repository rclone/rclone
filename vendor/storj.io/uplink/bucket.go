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
	"storj.io/common/storj"
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
	defer mon.Func().RestartTrace(&ctx)(&err)

	b, err := project.db.GetBucket(ctx, bucket)
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
	defer mon.Func().RestartTrace(&ctx)(&err)

	b, err := project.db.CreateBucket(ctx, bucket)

	if err != nil {
		if storj.ErrNoBucket.Has(err) {
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
	defer mon.Func().RestartTrace(&ctx)(&err)

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
	defer mon.Func().RestartTrace(&ctx)(&err)

	existing, err := project.db.DeleteBucket(ctx, bucket)
	if err != nil {
		if errs2.IsRPC(err, rpcstatus.FailedPrecondition) {
			return nil, errwrapf("%w (%q)", ErrBucketNotEmpty, bucket)
		}
		return nil, convertKnownErrors(err, bucket, "")
	}

	if existing == (storj.Bucket{}) {
		return nil, nil
	}

	return &Bucket{
		Name:    existing.Name,
		Created: existing.Created,
	}, nil
}
