// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcmigrate

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/zeebo/errs"
)

// Closed is returned by routed listeners when the mux is closed.
var Closed = errs.New("listener closed")

// ListenMux lets one multiplex a listener into different listeners based on the first
// bytes sent on the connection.
type ListenMux struct {
	base      net.Listener
	prefixLen int
	addr      net.Addr
	def       *listener

	mu     sync.Mutex
	routes map[string]*listener

	once sync.Once
	done chan struct{}
	err  error
}

// NewListenMux creates a ListenMux that reads the prefixLen bytes from any connections
// Accepted by the passed in listener and dispatches to the appropriate route.
func NewListenMux(base net.Listener, prefixLen int) *ListenMux {
	addr := base.Addr()
	return &ListenMux{
		base:      base,
		prefixLen: prefixLen,
		addr:      addr,
		def:       newListener(addr),

		routes: make(map[string]*listener),

		done: make(chan struct{}),
	}
}

//
// set up the routes
//

// Default returns the net.Listener that is used if no route matches.
func (m *ListenMux) Default() net.Listener { return m.def }

// Route returns a listener that will be used if the first bytes are the given prefix. The
// length of the prefix must match the original passed in prefixLen.
func (m *ListenMux) Route(prefix string) net.Listener {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(prefix) != m.prefixLen {
		panic(fmt.Sprintf("invalid prefix: has %d but needs %d bytes", len(prefix), m.prefixLen))
	}

	lis, ok := m.routes[prefix]
	if !ok {
		lis = newListener(m.addr)
		m.routes[prefix] = lis
		go m.monitorListener(prefix, lis)
	}
	return lis
}

//
// run the muxer
//

// Run calls listen on the provided listener and passes connections to the routed
// listeners.
func (m *ListenMux) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go m.monitorContext(ctx)
	go m.monitorBase()

	<-m.done

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, lis := range m.routes {
		<-lis.done
	}

	_ = m.def.Close()
	<-m.def.done

	return m.err
}

func (m *ListenMux) monitorContext(ctx context.Context) {
	<-ctx.Done()
	m.once.Do(func() {
		_ = m.base.Close() // TODO(jeff): do we care about this error?
		close(m.done)
	})
}

func (m *ListenMux) monitorBase() {
	for {
		conn, err := m.base.Accept()
		if err != nil {
			// TODO(jeff): temporary errors?
			m.once.Do(func() {
				m.err = err
				close(m.done)
			})
			return
		}
		go m.routeConn(conn)
	}
}

func (m *ListenMux) monitorListener(prefix string, lis *listener) {
	select {
	case <-m.done:
		lis.once.Do(func() {
			if m.err != nil {
				lis.err = m.err
			} else {
				lis.err = Closed
			}
			close(lis.done)
		})
	case <-lis.done:
	}
	m.mu.Lock()
	delete(m.routes, prefix)
	m.mu.Unlock()
}

func (m *ListenMux) routeConn(conn net.Conn) {
	buf := make([]byte, m.prefixLen)
	if _, err := io.ReadFull(conn, buf); err != nil {
		// TODO(jeff): how to handle these errors?
		_ = conn.Close()
		return
	}

	m.mu.Lock()
	lis, ok := m.routes[string(buf)]
	if !ok {
		lis = m.def
		conn = newPrefixConn(buf, conn)
	}
	m.mu.Unlock()

	// TODO(jeff): a timeout for the listener to get to the conn?

	select {
	case <-lis.done:
		// TODO(jeff): better way to signal to the caller the listener is closed?
		_ = conn.Close()
	case lis.Conns() <- conn:
	}
}
