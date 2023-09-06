// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// Package rpctracing implements tracing for rpc.
package rpctracing

import (
	"github.com/spacemonkeygo/monkit/v3"
)

const (
	// TraceID is the key we use to store trace id value into context.
	TraceID = "trace-id"
	// ParentID is the key we use to store parent's span id value into context.
	ParentID = "parent-id"
	// Sampled is the key we use to store sampled flag into context.
	Sampled = "sampled"
)

var mon = monkit.Package()
