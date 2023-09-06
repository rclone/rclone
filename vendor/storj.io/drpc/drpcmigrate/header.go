// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcmigrate

import (
	"net"
	"sync"
)

// DRPCHeader is a header for DRPC connections to use. This is designed
// to not conflict with a headerless gRPC, HTTP, or TLS request.
var DRPCHeader = "DRPC!!!1"

// HeaderConn fulfills the net.Conn interface. On the first call to Write
// it will write the Header.
type HeaderConn struct {
	net.Conn
	once   sync.Once
	header string
}

// NewHeaderConn returns a new *HeaderConn that writes the provided header
// as part of the first Write.
func NewHeaderConn(conn net.Conn, header string) *HeaderConn {
	return &HeaderConn{
		Conn:   conn,
		header: header,
	}
}

// Write will write buf to the underlying conn. If this is the first time Write
// is called it will prepend the Header to the beginning of the write.
func (d *HeaderConn) Write(buf []byte) (n int, err error) {
	var didOnce bool
	d.once.Do(func() {
		didOnce = true
		n, err = d.Conn.Write(append([]byte(d.header), buf...))
	})
	if didOnce {
		n -= len(d.header)
		if n < 0 {
			n = 0
		}
		return n, err
	}
	return d.Conn.Write(buf)
}
