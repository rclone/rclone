// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// Package rpctracing implements tracing for rpc.
package rpctracing

import (
	"context"

	"github.com/spacemonkeygo/monkit/v3"

	"storj.io/drpc"
	"storj.io/drpc/drpcmetadata"
)

type streamWrapper struct {
	drpc.Stream
	ctx context.Context
}

func (s *streamWrapper) Context() context.Context { return s.ctx }

// ExtractorFunc extracts from some metadata the trace information.
type ExtractorFunc func(metadata map[string]string) (trace *monkit.Trace, parentID int64)

func defaultExtractorFunc(metadata map[string]string) (*monkit.Trace, int64) {
	return monkit.NewTrace(monkit.NewId()), monkit.NewId()
}

// Handler implements drpc handler interface and takes in a callback function
// to extract the trace information from some metadata.
type Handler struct {
	handler drpc.Handler
	cb      ExtractorFunc
}

// NewHandler returns a new instance of Handler. If the callback is nil, a default
// one is used.
func NewHandler(handler drpc.Handler, cb ExtractorFunc) *Handler {
	if cb == nil {
		cb = defaultExtractorFunc
	}
	return &Handler{
		handler: handler,
		cb:      cb,
	}
}

// HandleRPC adds tracing metadata onto server stream.
func (handler *Handler) HandleRPC(stream drpc.Stream, rpc string) (err error) {
	streamCtx := stream.Context()

	metadata, ok := drpcmetadata.Get(streamCtx)
	if ok {
		trace, parentID := handler.cb(metadata)
		defer mon.FuncNamed(rpc).RemoteTrace(&streamCtx, parentID, trace)(&err)
	} else {
		defer mon.FuncNamed(rpc).ResetTrace(&streamCtx)(&err)
	}

	return handler.handler.HandleRPC(&streamWrapper{Stream: stream, ctx: streamCtx}, rpc)
}
