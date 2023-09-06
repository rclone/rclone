// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package rpcpool

import (
	"context"
	"crypto/tls"
	"sync"

	"github.com/zeebo/errs"

	"storj.io/drpc"
)

// RawConn is the type of connections the dialer must return.
type RawConn interface {
	drpc.Conn
	Unblocked() <-chan struct{}
}

// Conn is the type for connections returned from the pool.
type Conn interface {
	drpc.Conn
	State() *tls.ConnectionState
	ForceState(ctx context.Context) error
}

// poolConn grabs a connection from the pool for every invoke/stream.
type poolConn struct {
	pk   poolKey
	dial Dialer

	ownsPool bool
	pool     *Pool

	closedOnce sync.Once
	closedChan chan struct{}

	stateMu sync.Mutex
	state   *tls.ConnectionState
}

// Close marks the poolConn as closed and will not allow future calls to Invoke or NewStream
// to proceed. It does not stop any ongoing calls to Invoke or NewStream.
func (c *poolConn) Close() (err error) {
	c.closedOnce.Do(func() {
		// if c.ownsPool is true, there is no other poolConn that has a reference to
		// this pool. thus, we must close the pool when the conn is closed.
		if c.ownsPool {
			err = c.pool.Close()
		}
		close(c.closedChan)
	})
	return nil
}

// Closed returns true if the poolConn is closed.
func (c *poolConn) Closed() <-chan struct{} {
	return c.closedChan
}

// State returns the current best known tls.ConnectionState. It is nil if it is unknown.
func (c *poolConn) State() *tls.ConnectionState {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()

	return c.state
}

// ForceState forces a dial so that the tls connection state is filled in. It is
// a no-op if the state is already filled in.
func (c *poolConn) ForceState(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	select {
	case <-c.closedChan:
		return errs.New("connection closed")
	default:
	}

	if c.State() != nil {
		return nil
	}

	pv, err := c.pool.get(ctx, c.pk, c.dial)
	if err != nil {
		return err
	}
	defer c.pool.put(c.pk, pv)

	c.stateMu.Lock()
	if c.state == nil {
		c.state = pv.state
	}
	c.stateMu.Unlock()

	return nil
}

// Invoke acquires a connection from the pool, dialing if necessary, and issues the Invoke on that
// connection. The connection is replaced into the pool after the invoke finishes.
func (c *poolConn) Invoke(ctx context.Context, rpc string, enc drpc.Encoding, in, out drpc.Message) (err error) {
	defer mon.Task()(&ctx)(&err)

	select {
	case <-c.closedChan:
		return errs.New("connection closed")
	default:
	}

	pv, err := c.pool.get(ctx, c.pk, c.dial)
	if err != nil {
		return err
	}
	defer c.pool.put(c.pk, pv)

	c.stateMu.Lock()
	if c.state == nil {
		c.state = pv.state
	}
	c.stateMu.Unlock()

	return pv.conn.Invoke(ctx, rpc, enc, in, out)
}

// NewStream acquires a connection from the pool, dialing if necessary, and issues the NewStream on
// that connection. The connection is replaced into the pool after the stream is finished.
func (c *poolConn) NewStream(ctx context.Context, rpc string, enc drpc.Encoding) (_ drpc.Stream, err error) {
	defer mon.Task()(&ctx)(&err)

	select {
	case <-c.closedChan:
		return nil, errs.New("connection closed")
	default:
	}

	pv, err := c.pool.get(ctx, c.pk, c.dial)
	if err != nil {
		return nil, err
	}

	c.stateMu.Lock()
	if c.state == nil {
		c.state = pv.state
	}
	c.stateMu.Unlock()

	stream, err := pv.conn.NewStream(ctx, rpc, enc)
	if err != nil {
		return nil, err
	}

	// the stream's done channel is closed when we're sure no reads/writes are
	// coming in for that stream anymore. it has been fully terminated.
	go func() {
		<-stream.Context().Done()
		c.pool.put(c.pk, pv)
	}()

	return stream, nil
}
