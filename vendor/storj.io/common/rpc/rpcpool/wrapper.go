// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package rpcpool

import "context"

type dialerWrapper struct{}

// DialerWrapper can create enhanced functionality.
type DialerWrapper = func(ctx context.Context, dialer Dialer) Dialer

// WithDialerWrapper creates context with DialerWrapper used by dial.Invoke.
func WithDialerWrapper(ctx context.Context, wrapper DialerWrapper) context.Context {
	return context.WithValue(ctx, dialerWrapper{}, wrapper)
}

// GetWrapper returns with the dialerWrapper if registered.
func GetWrapper(ctx context.Context) (DialerWrapper, bool) {
	wrapper, ok := ctx.Value(dialerWrapper{}).(DialerWrapper)
	return wrapper, ok
}

// WrapDialer returns with dialed which may be wrapped if wrapper is registered.
func WrapDialer(ctx context.Context, dialer Dialer) Dialer {
	wrapper, found := GetWrapper(ctx)
	if !found {
		return dialer
	}
	return wrapper(ctx, dialer)
}
