// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package errs2

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/common/rpc/rpcstatus"
)

// IsCanceled returns true, when the error is a cancellation.
func IsCanceled(err error) bool {
	return errs.IsFunc(err, func(err error) bool {
		return err == context.Canceled ||
			rpcstatus.Code(err) == rpcstatus.Canceled
	})
}

// IgnoreCanceled returns nil, when the operation was about canceling.
func IgnoreCanceled(err error) error {
	if IsCanceled(err) {
		return nil
	}
	return err
}
