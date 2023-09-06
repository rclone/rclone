// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package netutil

import (
	"net"
	"runtime"
)

// closeTrackingConn wraps a net.Conn and keeps track of if it was closed
// or if it was leaked (and closes it if it was leaked).
type closeTrackingConn struct {
	net.Conn
}

// TrackClose wraps the conn and sets a  finalizer on the returned value to
// close the conn and monitor that it was leaked.
func TrackClose(conn net.Conn) net.Conn {
	tracked := &closeTrackingConn{Conn: conn}
	runtime.SetFinalizer(tracked, (*closeTrackingConn).finalize)
	return tracked
}

// Close clears the finalizer and closes the connection.
func (c *closeTrackingConn) Close() error {
	runtime.SetFinalizer(c, nil)
	mon.Event("connection_closed")
	return c.Conn.Close()
}

// finalize monitors that a connection was leaked and closes the connection.
func (c *closeTrackingConn) finalize() {
	mon.Event("connection_leaked")
	_ = c.Conn.Close()
}
