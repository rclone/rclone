// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package experiment

import (
	"context"

	"storj.io/common/rpc/rpcpool"
	"storj.io/drpc"
	"storj.io/drpc/drpcmetadata"
)

// Wrapper wraps a Conn with experimental feature flag information.
type Wrapper struct {
	rpcpool.Conn
}

// NewConnWrapper creates a new instance of the wrapper.
func NewConnWrapper(conn rpcpool.Conn) *Wrapper {
	return &Wrapper{
		conn,
	}
}

// Invoke implements drpc.Conn's Invoke method with feature flag information injected into the context.
func (c *Wrapper) Invoke(ctx context.Context, rpc string, enc drpc.Encoding, in drpc.Message, out drpc.Message) (err error) {
	return c.Conn.Invoke(c.trace(ctx), rpc, enc, in, out)
}

// NewStream implements drpc.Conn's NewStream method with experiment feature flag injected into the context.
func (c *Wrapper) NewStream(ctx context.Context, rpc string, enc drpc.Encoding) (_ drpc.Stream, err error) {
	return c.Conn.NewStream(c.trace(ctx), rpc, enc)
}

// trace injects tracing related information into the context.
func (c *Wrapper) trace(ctx context.Context) context.Context {
	if exp := ctx.Value(contextKey); exp != nil {
		if exps, ok := exp.(string); ok {
			return drpcmetadata.Add(ctx, drpcKey, exps)
		}
	}
	return ctx
}
