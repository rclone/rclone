// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

// Package rpcpool implements connection pooling for rpc.
package rpcpool

import (
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
)

var mon = monkit.Package()

// Error is the class of errors returned by this package.
var Error = errs.Class("rpcpool")
