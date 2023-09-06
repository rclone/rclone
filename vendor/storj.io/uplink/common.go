// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"errors"
	"fmt"
	"io"
	"strings"
	_ "unsafe" // for go:linkname

	"github.com/jtolio/eventkit"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"

	"storj.io/common/encryption"
	"storj.io/common/errs2"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/uplink/private/metaclient"
	"storj.io/uplink/private/piecestore"
)

var mon = monkit.Package()
var evs = eventkit.Package()

// We use packageError.Wrap/New instead of plain errs.Wrap/New to add a prefix "uplink" to every error
// message emitted by the Uplink library.
// It is private because it's not intended to be part of the public API.
var packageError = errs.Class("uplink")

// ErrTooManyRequests is returned when user has sent too many requests in a given amount of time.
var ErrTooManyRequests = errors.New("too many requests")

// ErrBandwidthLimitExceeded is returned when project will exceeded bandwidth limit.
var ErrBandwidthLimitExceeded = errors.New("bandwidth limit exceeded")

// ErrStorageLimitExceeded is returned when project will exceeded storage limit.
var ErrStorageLimitExceeded = errors.New("storage limit exceeded")

// ErrSegmentsLimitExceeded is returned when project will exceeded segments limit.
var ErrSegmentsLimitExceeded = errors.New("segments limit exceeded")

// ErrPermissionDenied is returned when the request is denied due to invalid permissions.
var ErrPermissionDenied = errors.New("permission denied")

//go:linkname convertKnownErrors
func convertKnownErrors(err error, bucket, key string) error {
	switch {
	case errors.Is(err, io.EOF):
		return err
	case metaclient.ErrNoBucket.Has(err):
		return errwrapf("%w (%q)", ErrBucketNameInvalid, bucket)
	case metaclient.ErrNoPath.Has(err):
		return errwrapf("%w (%q)", ErrObjectKeyInvalid, key)
	case metaclient.ErrBucketNotFound.Has(err):
		return errwrapf("%w (%q)", ErrBucketNotFound, bucket)
	case metaclient.ErrObjectNotFound.Has(err):
		return errwrapf("%w (%q)", ErrObjectNotFound, key)
	case encryption.ErrMissingEncryptionBase.Has(err):
		return errwrapf("%w (%q)", ErrPermissionDenied, key)
	case encryption.ErrMissingDecryptionBase.Has(err):
		return errwrapf("%w (%q)", ErrPermissionDenied, key)
	case errs2.IsRPC(err, rpcstatus.ResourceExhausted):
		// TODO is a better way to do this?
		message := errs.Unwrap(err).Error()
		if strings.HasSuffix(message, "Exceeded Usage Limit") {
			return packageError.Wrap(rpcstatus.Wrap(rpcstatus.ResourceExhausted, ErrBandwidthLimitExceeded))
		} else if strings.HasSuffix(message, "Too Many Requests") {
			return packageError.Wrap(rpcstatus.Wrap(rpcstatus.ResourceExhausted, ErrTooManyRequests))
		} else if strings.Contains(message, "Exceeded Storage Limit") {
			// contains used to have some flexibility in constructing error message on server-side
			return packageError.Wrap(rpcstatus.Wrap(rpcstatus.ResourceExhausted, ErrStorageLimitExceeded))
		} else if strings.Contains(message, "Exceeded Segments Limit") {
			// contains used to have some flexibility in constructing error message on server-side
			return packageError.Wrap(rpcstatus.Wrap(rpcstatus.ResourceExhausted, ErrSegmentsLimitExceeded))
		}
	case errs2.IsRPC(err, rpcstatus.NotFound):
		const (
			bucketNotFoundPrefix = "bucket not found"
			objectNotFoundPrefix = "object not found"
		)

		message := errs.Unwrap(err).Error()
		if strings.HasPrefix(message, bucketNotFoundPrefix) {
			// remove error prefix + ": " from message
			bucket := strings.TrimPrefix(message[len(bucketNotFoundPrefix):], ": ")
			return errwrapf("%w (%q)", ErrBucketNotFound, bucket)
		} else if strings.HasPrefix(message, objectNotFoundPrefix) {
			return errwrapf("%w (%q)", ErrObjectNotFound, key)
		}
	case errs2.IsRPC(err, rpcstatus.PermissionDenied):
		originalErr := err
		wrappedErr := errwrapf("%w (%v)", ErrPermissionDenied, originalErr)
		// TODO: once we have confirmed nothing downstream
		// is using errs2.IsRPC(err, rpcstatus.PermissionDenied), we should
		// just return wrappedErr instead of this.
		return &joinedErr{main: wrappedErr, alt: originalErr, code: rpcstatus.PermissionDenied}
	}

	return packageError.Wrap(err)
}

func errwrapf(format string, err error, args ...interface{}) error {
	var all []interface{}
	all = append(all, err)
	all = append(all, args...)
	return packageError.Wrap(fmt.Errorf(format, all...))
}

type joinedErr struct {
	main error
	alt  error
	code rpcstatus.StatusCode
}

func (err *joinedErr) Is(target error) bool {
	return errors.Is(err.main, target) || errors.Is(err.alt, target)
}

func (err *joinedErr) As(target interface{}) bool {
	if errors.As(err.main, target) {
		return true
	}
	if errors.As(err.alt, target) {
		return true
	}
	return false
}

func (err *joinedErr) Code() uint64 {
	return uint64(err.code)
}

func (err *joinedErr) Unwrap() error {
	return err.main
}

func (err *joinedErr) Error() string {
	return err.main.Error()
}

// Ungroup works with errs2.IsRPC and errs.IsFunc.
func (err *joinedErr) Ungroup() []error {
	return []error{err.main, err.alt}
}

var noiseVersion = func() int64 {
	if piecestore.NoiseEnabled {
		// this is a number that indicates what noise support exists so far.
		// 1 was our first implementation, but crucially had an errant round
		// trip on uploads and downloads. 2 has the round trip fixed for
		// downloads but not uploads. 3 has long tail cancelation for
		// downloads fixed. 4 has round trips reduced for small uploads.
		// we'll probably have future values here.
		// we'll want to compare the performance of these different cases.
		return 4
	}
	// 0 means no noise
	return 0
}()
