// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package multidial

import (
	"net"
	"sync"
	"time"

	"github.com/zeebo/errs"
)

type conn struct {
	mtx    sync.Mutex
	setup  *setup
	conn   net.Conn
	closed bool
}

var _ net.Conn = (*conn)(nil)

func (c *conn) LocalAddr() net.Addr {
	c.mtx.Lock()
	setup, conn := c.setup, c.conn
	c.mtx.Unlock()
	if conn != nil {
		return conn.LocalAddr()
	}
	return addr{network: setup.network, address: "127.0.0.1:0"}
}

func (c *conn) RemoteAddr() net.Addr {
	c.mtx.Lock()
	setup, conn := c.setup, c.conn
	c.mtx.Unlock()
	if conn != nil {
		return conn.RemoteAddr()
	}
	return addr{network: setup.network, address: setup.address}
}

func (c *conn) Read(p []byte) (n int, err error) {
	c.mtx.Lock()
	setup, conn := c.setup, c.conn
	c.mtx.Unlock()
	if conn != nil {
		return conn.Read(p)
	}
	n, conn, err = setup.Read(p)
	if conn != nil {
		c.mtx.Lock()
		if c.closed {
			c.mtx.Unlock()
			// conn.Close() might return nil, but we should return an error
			err = errs.Combine(errs.New("connection closed"), conn.Close())
			return 0, err
		}
		c.conn, c.setup = conn, nil
		c.mtx.Unlock()
		_ = setup.Close()
	}
	return n, err
}

func (c *conn) Write(p []byte) (n int, err error) {
	c.mtx.Lock()
	setup, conn := c.setup, c.conn
	c.mtx.Unlock()
	if conn != nil {
		return conn.Write(p)
	}
	return setup.Write(p)
}

func (c *conn) Close() error {
	c.mtx.Lock()
	setup, conn, closed := c.setup, c.conn, c.closed
	c.closed = true
	c.mtx.Unlock()
	if closed {
		return nil
	}
	var eg errs.Group
	if conn != nil {
		eg.Add(conn.Close())
	}
	if setup != nil {
		eg.Add(setup.Close())
	}
	return eg.Err()
}

func (c *conn) SetDeadline(t time.Time) error {
	c.mtx.Lock()
	setup, conn := c.setup, c.conn
	c.mtx.Unlock()
	if conn != nil {
		return conn.SetDeadline(t)
	}
	return setup.SetDeadline(t)
}

func (c *conn) SetReadDeadline(t time.Time) error {
	c.mtx.Lock()
	setup, conn := c.setup, c.conn
	c.mtx.Unlock()
	if conn != nil {
		return conn.SetReadDeadline(t)
	}
	return setup.SetReadDeadline(t)
}

func (c *conn) SetWriteDeadline(t time.Time) error {
	c.mtx.Lock()
	setup, conn := c.setup, c.conn
	c.mtx.Unlock()
	if conn != nil {
		return conn.SetWriteDeadline(t)
	}
	return setup.SetWriteDeadline(t)

}
