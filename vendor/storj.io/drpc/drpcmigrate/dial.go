// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcmigrate

import (
	"context"
	"net"
)

// HeaderDialer is a net.Dialer-like that prefixes all conns with the provided
// header.
type HeaderDialer struct {
	net.Dialer
	Header string
}

// Dial will dial the address on the named network, creating a connection
// that will write the configured Header on the first user-requested write.
func (d *HeaderDialer) Dial(network, address string) (net.Conn, error) {
	return d.DialContext(context.Background(), network, address)
}

// DialContext will dial the address on the named network, creating a connection
// that will write the configured Header on the first user-requested write.
func (d *HeaderDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	conn, err := d.Dialer.DialContext(ctx, network, address)
	if err != nil {
		return nil, err
	}
	return NewHeaderConn(conn, d.Header), nil
}

// DialWithHeader is like net.Dial, but uses HeaderConns with the provided header.
func DialWithHeader(ctx context.Context, network, address string, header string) (net.Conn, error) {
	conn, err := (&HeaderDialer{Header: header}).DialContext(ctx, network, address)
	if err != nil {
		return nil, err
	}
	return conn, nil
}
