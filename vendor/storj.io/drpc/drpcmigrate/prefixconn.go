// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcmigrate

import (
	"bytes"
	"io"
	"net"
)

type prefixConn struct {
	io.Reader
	net.Conn
}

func newPrefixConn(data []byte, conn net.Conn) *prefixConn {
	return &prefixConn{
		Reader: io.MultiReader(bytes.NewReader(data), conn),
		Conn:   conn,
	}
}

func (pc *prefixConn) Read(p []byte) (n int, err error) {
	return pc.Reader.Read(p)
}
