// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// Package rpctracing implements tracing for rpc.
package rpctracing

import (
	"context"

	"github.com/spacemonkeygo/monkit/v3"

	"storj.io/drpc"
	"storj.io/drpc/drpcmetadata"
	"storj.io/drpc/drpcmux"
)

type streamWrapper struct {
	drpc.Stream
	ctx context.Context
}

func (s *streamWrapper) Context() context.Context { return s.ctx }

type handlerFunc func(metadata map[string]string) (trace *monkit.Trace, spanID int64)

func defaultHandlerFunc(metadata map[string]string) (*monkit.Trace, int64) {
	return monkit.NewTrace(monkit.NewId()), monkit.NewId()
}

// Handler implements drpc handler interface and takes in a callback function.
type Handler struct {
	mux *drpcmux.Mux
	cb  handlerFunc
}

// NewHandler returns a new instance of Handler.
func NewHandler(mux *drpcmux.Mux, cb handlerFunc) *Handler {
	if cb == nil {
		cb = defaultHandlerFunc
	}
	return &Handler{
		mux: mux,
		cb:  cb,
	}
}

// HandleRPC adds tracing metadata onto server stream.
func (handler *Handler) HandleRPC(stream drpc.Stream, rpc string) (err error) {
	streamCtx := stream.Context()
	metadata, ok := drpcmetadata.Get(streamCtx)
	if ok {
		trace, spanID := handler.cb(metadata)
		defer mon.FuncNamed(rpc).RemoteTrace(&streamCtx, spanID, trace)(&err)
	}

	return handler.mux.HandleRPC(&streamWrapper{Stream: stream, ctx: streamCtx}, rpc)
}
