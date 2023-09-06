// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package rpcpool

import (
	"context"
	"crypto/tls"
	"runtime"
	"time"

	"github.com/spacemonkeygo/monkit/v3"

	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc/rpccache"
)

// Options controls the options for a connection pool.
type Options struct {
	// Capacity is how many connections to keep open.
	Capacity int

	// KeyCapacity is the number of connections to keep open per cache key.
	KeyCapacity int

	// IdleExpiration is how long a connection in the pool is allowed to be
	// kept idle. If zero, connections do not expire.
	IdleExpiration time.Duration

	// Name is used to differentiate pools in monkit stat.
	Name string
}

// Pool is a wrapper around a cache of connections that allows one to get or
// create new cached connections.
type Pool struct {
	cache *rpccache.Cache
	name  string
}

// New constructs a new Pool with the Options.
func New(opts Options) *Pool {
	p := &Pool{
		name: opts.Name,
		cache: rpccache.New(rpccache.Options{
			Expiration:  opts.IdleExpiration,
			Capacity:    opts.Capacity,
			KeyCapacity: opts.KeyCapacity,
			Close: func(pv interface{}) error {
				return pv.(*poolValue).conn.Close()
			},
			Stale: func(pv interface{}) bool {
				select {
				case <-pv.(*poolValue).conn.Closed():
					return true
				default:
					return false
				}
			},
			Unblocked: func(pv interface{}) bool {
				select {
				case <-pv.(*poolValue).conn.Unblocked():
					return true
				default:
					return false
				}
			},
		})}

	// As much as I dislike finalizers, especially for cases where it handles
	// file descriptors, I think it's important to add one here at least until
	// a full audit of all of the uses of the rpc.Dialer type and ensuring they
	// all get closed.
	runtime.SetFinalizer(p, func(p *Pool) {
		mon.Event("pool_leaked")
		_ = p.Close()
	})

	return p
}

// poolKey is the type of keys in the cache.
type poolKey struct {
	key        string
	tlsOptions *tlsopts.Options
}

// poolValue is the type of values in the cache.
type poolValue struct {
	conn  RawConn
	state *tls.ConnectionState
}

// Dialer is the type of function to create a new connection.
type Dialer = func(context.Context) (RawConn, *tls.ConnectionState, error)

// Close closes all of the cached connections. It is safe to call on a nil receiver.
func (p *Pool) Close() error {
	if p == nil {
		return nil
	}

	runtime.SetFinalizer(p, nil)
	return p.cache.Close()
}

// put puts back the pool key and value into the cache.
func (p *Pool) put(pk poolKey, pv *poolValue) {
	if p != nil {
		p.cache.Put(pk, pv)
	}
}

// get returns a drpc connection from the cache if possible, dialing if necessary.
func (p *Pool) get(ctx context.Context, pk poolKey, dial Dialer) (pv *poolValue, err error) {
	defer mon.Task()(&ctx)(&err)

	var tags []monkit.SeriesTag
	if p != nil && p.name != "" {
		tags = append(tags, monkit.NewSeriesTag("name", p.name))
	}
	if p != nil {
		pv, ok := p.cache.Take(pk).(*poolValue)
		if ok {
			mon.Event("connection_from_cache", tags...)
			return pv, nil
		}
	}

	mon.Event("connection_dialed", tags...)
	conn, state, err := dial(ctx)
	if err != nil {
		return nil, err
	}

	return &poolValue{
		conn:  conn,
		state: state,
	}, nil
}

// Get looks up a connection with the same key and TLS options and returns it if it
// exists. If it does not exist, it calls the dial function to create one. It is safe
// to call on a nil receiver, and if so, always returns a dialed connection.
func (p *Pool) Get(ctx context.Context, key string, tlsOptions *tlsopts.Options, dial Dialer) (conn Conn, err error) {
	defer mon.Task()(&ctx)(&err)

	pk := poolKey{
		key:        key,
		tlsOptions: tlsOptions,
	}

	// if there is no pool, each conn gets it's own pool so that it doesn't
	// need to dial for every rpc.
	ownsPool := false
	if p == nil {
		p = New(Options{Capacity: 1, KeyCapacity: 1})
		ownsPool = true
	}

	if ctx.Value(forceDialKey{}) != nil {
		pv, err := p.get(ctx, pk, dial)
		if err != nil {
			return nil, err
		}
		p.put(pk, pv)

		return &poolConn{
			closedChan: make(chan struct{}),
			pk:         pk,
			dial:       dial,
			state:      pv.state,

			ownsPool: ownsPool,
			pool:     p,
		}, nil
	}

	return &poolConn{
		closedChan: make(chan struct{}),
		pk:         pk,
		dial:       dial,

		ownsPool: ownsPool,
		pool:     p,
	}, nil
}

type forceDialKey struct{}

// WithForceDial returns a context that when used to Get a conn will force a dial.
func WithForceDial(ctx context.Context) context.Context {
	return context.WithValue(ctx, forceDialKey{}, struct{}{})
}
