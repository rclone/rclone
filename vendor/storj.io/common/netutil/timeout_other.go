// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build !linux

package netutil

import (
	"net"
	"time"
)

// SetUserTimeout sets the TCP_USER_TIMEOUT setting on the provided conn.
func SetUserTimeout(conn *net.TCPConn, timeout time.Duration) error {
	return nil
}
