// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

// Package telemetryclient is internal package to support telemetry
// without introducing a direct dependency to the actual implementation.
package telemetryclient

import (
	"context"

	"go.uber.org/zap"
)

type contextKey int

const constructorKey contextKey = iota

// Constructor creates a new telemetry client.
type Constructor func(log *zap.Logger, satelliteAddress string) (Client, error)

// Client is the common interface for telemetry.
type Client interface {
	Run(ctx context.Context)
	Stop()
	Report(ctx context.Context) error
}

// WithConstructor specifies which telemetry to use.
func WithConstructor(ctx context.Context, ctor Constructor) context.Context {
	return context.WithValue(ctx, constructorKey, ctor)
}

// ConstructorFrom loads the telemetry client constructor from context.
func ConstructorFrom(ctx context.Context) (_ Constructor, ok bool) {
	v := ctx.Value(constructorKey)
	if v == nil {
		return nil, false
	}

	ctor, ok := v.(Constructor)
	return ctor, ok
}
