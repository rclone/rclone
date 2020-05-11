// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

// Package rpcpeer implements context.Context peer tagging.
package rpcpeer

import (
	"context"
	"crypto/tls"
	"net"

	"github.com/zeebo/errs"

	"storj.io/common/internal/grpchook"
	"storj.io/drpc/drpcctx"
)

// Error is the class of errors returned by this package.
var Error = errs.Class("rpcpeer")

// Peer represents an rpc peer.
type Peer struct {
	Addr  net.Addr
	State tls.ConnectionState
}

// peerKey is used as a unique value for context keys.
type peerKey struct{}

// NewContext returns a new context with the peer associated as a value.
func NewContext(ctx context.Context, peer *Peer) context.Context {
	return context.WithValue(ctx, peerKey{}, peer)
}

// FromContext returns the peer that was previously associated by NewContext.
func FromContext(ctx context.Context) (*Peer, error) {
	if peer, ok := ctx.Value(peerKey{}).(*Peer); ok {
		return peer, nil
	} else if peer, drpcErr := drpcInternalFromContext(ctx); drpcErr == nil {
		return peer, nil
	} else if addr, state, grpcErr := grpchook.InternalFromContext(ctx); grpcErr == nil {
		return &Peer{Addr: addr, State: state}, nil
	} else {
		if grpcErr == grpchook.ErrNotHooked {
			grpcErr = nil
		}
		return nil, errs.Combine(drpcErr, grpcErr)
	}
}

// drpcInternalFromContext returns a peer from the context using drpc.
func drpcInternalFromContext(ctx context.Context) (*Peer, error) {
	tr, ok := drpcctx.Transport(ctx)
	if !ok {
		return nil, Error.New("unable to get drpc peer from context")
	}

	conn, ok := tr.(interface {
		RemoteAddr() net.Addr
		ConnectionState() tls.ConnectionState
	})
	if !ok {
		return nil, Error.New("drpc transport does not have required methods")
	}

	return &Peer{
		Addr:  conn.RemoteAddr(),
		State: conn.ConnectionState(),
	}, nil
}
