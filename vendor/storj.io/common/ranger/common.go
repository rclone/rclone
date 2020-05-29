// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package ranger

import (
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
)

// Error is the errs class of standard Ranger errors.
var Error = errs.Class("ranger error")

var mon = monkit.Package()
