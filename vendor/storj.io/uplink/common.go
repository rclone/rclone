// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"

	"storj.io/common/errs2"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/storj"
)

var mon = monkit.Package()

// Error is default error class for uplink.
var packageError = errs.Class("uplink")

// ErrTooManyRequests is returned when user has sent too many requests in a given amount of time.
var ErrTooManyRequests = errors.New("too many requests")

// ErrBandwidthLimitExceeded is returned when project will exceeded bandwidth limit.
var ErrBandwidthLimitExceeded = errors.New("bandwidth limit exceeded")

func convertKnownErrors(err error, bucket, key string) error {
	switch {
	case storj.ErrNoBucket.Has(err):
		return errwrapf("%w (%q)", ErrBucketNameInvalid, bucket)
	case storj.ErrNoPath.Has(err):
		return errwrapf("%w (%q)", ErrObjectKeyInvalid, key)
	case storj.ErrBucketNotFound.Has(err):
		return errwrapf("%w (%q)", ErrBucketNotFound, bucket)
	case storj.ErrObjectNotFound.Has(err):
		return errwrapf("%w (%q)", ErrObjectNotFound, key)
	case errs2.IsRPC(err, rpcstatus.ResourceExhausted):
		// TODO is a better way to do this?
		message := errs.Unwrap(err).Error()
		if message == "Exceeded Usage Limit" {
			return packageError.Wrap(ErrBandwidthLimitExceeded)
		} else if message == "Too Many Requests" {
			return packageError.Wrap(ErrTooManyRequests)
		}
	case errs2.IsRPC(err, rpcstatus.NotFound):
		message := errs.Unwrap(err).Error()
		if strings.HasPrefix(message, storj.ErrBucketNotFound.New("").Error()) {
			return errwrapf("%w (%q)", ErrBucketNotFound, bucket)
		} else if strings.HasPrefix(message, storj.ErrObjectNotFound.New("").Error()) {
			return errwrapf("%w (%q)", ErrObjectNotFound, key)
		}
	}

	return packageError.Wrap(err)
}

func errwrapf(format string, err error, args ...interface{}) error {
	var all []interface{}
	all = append(all, err)
	all = append(all, args...)
	return packageError.Wrap(fmt.Errorf(format, all...))
}
