// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package experiment

import (
	"context"

	"storj.io/drpc"
	"storj.io/drpc/drpcmetadata"
)

type streamWrapper struct {
	drpc.Stream
	ctx context.Context
}

func (s *streamWrapper) Context() context.Context { return s.ctx }

// Handler implements drpc handler interface to extract experiment feature flag.
type Handler struct {
	handler drpc.Handler
}

// NewHandler returns a new instance of Handler.
func NewHandler(handler drpc.Handler) *Handler {
	return &Handler{
		handler: handler,
	}
}

// HandleRPC copies experiment feature flag from drpcmeta to context.
func (handler *Handler) HandleRPC(stream drpc.Stream, rpc string) (err error) {
	streamCtx := stream.Context()

	metadata, ok := drpcmetadata.Get(streamCtx)
	if ok {
		if exp, found := metadata[drpcKey]; found {
			streamCtx = context.WithValue(streamCtx, contextKey, exp)
		}
	}
	return handler.handler.HandleRPC(&streamWrapper{Stream: stream, ctx: streamCtx}, rpc)
}
