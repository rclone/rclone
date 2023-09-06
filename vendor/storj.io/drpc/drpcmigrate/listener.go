// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcmigrate

import (
	"net"
	"sync"
)

type listener struct {
	addr  net.Addr
	conns chan net.Conn
	once  sync.Once
	done  chan struct{}
	err   error
}

func newListener(addr net.Addr) *listener {
	return &listener{
		addr:  addr,
		conns: make(chan net.Conn),
		done:  make(chan struct{}),
	}
}

// Conns returns the channel of net.Conn that the listener reads from.
func (l *listener) Conns() chan net.Conn { return l.conns }

// Accept waits for and returns the next connection to the listener.
func (l *listener) Accept() (conn net.Conn, err error) {
	select {
	case <-l.done:
		return nil, l.err
	default:
	}
	select {
	case <-l.done:
		return nil, l.err
	case conn = <-l.conns:
		return conn, nil
	}
}

// Close closes the listener.
// Any blocked Accept operations will be unblocked and return errors.
func (l *listener) Close() error {
	l.once.Do(func() {
		l.err = Closed
		close(l.done)
	})
	return nil
}

// Addr returns the listener's network address.
func (l *listener) Addr() net.Addr {
	return l.addr
}
