// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package errs2

import (
	"github.com/zeebo/errs"

	"storj.io/common/rpc/rpcstatus"
)

// IsRPC checks if err contains an RPC error with the given status code.
func IsRPC(err error, code rpcstatus.StatusCode) bool {
	return errs.IsFunc(err, func(err error) bool {
		return rpcstatus.Code(err) == code
	})
}
