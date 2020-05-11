// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// Package grpchook exists to avoid introducing a dependency to
// grpc unless pb/pbgrpc is imported in other packages.
package grpchook

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
)

// ErrNotHooked is returned from funcs when pb/pbgrpc hasn't been imported.
var ErrNotHooked = errors.New("grpc not hooked")

// HookedErrServerStopped is the grpc.ErrServerStopped when initialized.
var HookedErrServerStopped error

// IsErrServerStopped returns when err == grpc.ErrServerStopped and
// pb/pbgrpc has been imported.
func IsErrServerStopped(err error) bool {
	if HookedErrServerStopped == nil {
		return false
	}

	return HookedErrServerStopped == err
}

// HookedInternalFromContext returns grpc peer information from context.
var HookedInternalFromContext func(ctx context.Context) (addr net.Addr, state tls.ConnectionState, err error)

// InternalFromContext returns the peer that was previously associated by NewContext using grpc.
func InternalFromContext(ctx context.Context) (addr net.Addr, state tls.ConnectionState, err error) {
	if HookedInternalFromContext == nil {
		return nil, tls.ConnectionState{}, ErrNotHooked
	}

	return HookedInternalFromContext(ctx)
}

// StatusCode is rpcstatus.Code, however it cannot use it directly without introducing a
// circular dependency.
type StatusCode uint64

// HookedErrorWrap is the func to wrap a status code.
var HookedErrorWrap func(code StatusCode, err error) error

// HookedConvertToStatusCode tries to convert grpc error status to rpcstatus.StatusCode
var HookedConvertToStatusCode func(err error) (StatusCode, bool)
