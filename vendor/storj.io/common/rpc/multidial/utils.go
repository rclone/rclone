// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package multidial

import (
	"net"
	"sync/atomic"
)

type addr struct {
	network string
	address string
}

func (a addr) Network() string { return a.network }
func (a addr) String() string  { return a.address }

type atomicConn struct {
	conn atomic.Value
}

func (c *atomicConn) Store(conn net.Conn) {
	c.conn.Store(conn)
}

func (c *atomicConn) Load() (net.Conn, bool) {
	conn, ok := c.conn.Load().(net.Conn)
	return conn, ok
}
