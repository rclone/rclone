// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcctx

import (
	"context"

	"storj.io/drpc"
)

// TransportKey is used to store the drpc.Transport with the context.
type TransportKey struct{}

// WithTransport associates the drpc.Transport as a value on the context.
func WithTransport(ctx context.Context, tr drpc.Transport) context.Context {
	return context.WithValue(ctx, TransportKey{}, tr)
}

// Transport returns the drpc.Transport associated with the context and a bool if it
// existed.
func Transport(ctx context.Context) (drpc.Transport, bool) {
	tr, ok := ctx.Value(TransportKey{}).(drpc.Transport)
	return tr, ok
}
