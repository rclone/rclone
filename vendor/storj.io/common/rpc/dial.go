// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package rpc

import (
	"context"
	"crypto/tls"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/netutil"
	"storj.io/common/pb"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc/rpcpool"
	"storj.io/common/rpc/rpctracing"
	"storj.io/common/storj"
	"storj.io/drpc"
	"storj.io/drpc/drpcconn"
	"storj.io/drpc/drpcmanager"
	"storj.io/drpc/drpcstream"
)

// NewDefaultManagerOptions returns the default options we use for drpc managers.
func NewDefaultManagerOptions() drpcmanager.Options {
	return drpcmanager.Options{
		WriterBufferSize: 1024,
		Stream: drpcstream.Options{
			SplitSize: (4096 * 2) - 256,
		},
	}
}

// Transport is a type that creates net.Conns, given an address.
// net.Dialer implements this interface and is used by default.
type Transport interface {
	// DialContext is called to establish a connection.
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

// Dialer holds configuration for dialing.
type Dialer struct {
	// TLSOptions controls the tls options for dialing. If it is nil, only
	// insecure connections can be made.
	TLSOptions *tlsopts.Options

	// DialTimeout causes all the tcp dials to error if they take longer
	// than it if it is non-zero.
	DialTimeout time.Duration

	// DialLatency sleeps this amount if it is non-zero before every dial.
	// The timeout runs while the sleep is happening.
	DialLatency time.Duration

	// TransferRate limits all read/write operations to go slower than
	// the size per second if it is non-zero.
	TransferRate memory.Size

	// PoolOptions controls options for the connection pool.
	PoolOptions rpcpool.Options

	// ConnectionOptions controls the options that we pass to drpc connections.
	ConnectionOptions drpcconn.Options

	// TCPUserTimeout controls what setting to use for the TCP_USER_TIMEOUT
	// socket option on dialed connections. Only valid on linux. Only set
	// if positive.
	TCPUserTimeout time.Duration

	// Transport is how sockets are opened. If nil, net.Dialer is used.
	Transport Transport
}

// NewDefaultDialer returns a Dialer with default timeouts set.
func NewDefaultDialer(tlsOptions *tlsopts.Options) Dialer {
	return Dialer{
		TLSOptions:     tlsOptions,
		DialTimeout:    20 * time.Second,
		TCPUserTimeout: 15 * time.Minute,
		PoolOptions: rpcpool.Options{
			Capacity:       5,
			IdleExpiration: 2 * time.Minute,
		},
		ConnectionOptions: drpcconn.Options{
			Manager: NewDefaultManagerOptions(),
		},
	}
}

// dialContext does a raw tcp dial to the address and wraps the connection with the
// provided timeout.
func (d Dialer) dialContext(ctx context.Context, address string) (net.Conn, error) {
	if d.DialLatency > 0 {
		timer := time.NewTimer(d.DialLatency)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return nil, Error.Wrap(ctx.Err())
		}
	}

	dialer := d.Transport
	if dialer == nil {
		dialer = new(net.Dialer)
	}

	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		// N.B. this error is not wrapped on purpose! grpc code cares about inspecting
		// it and it's not smart enough to attempt to do any unwrapping. :( Additionally
		// DialContext does not return an error that can be inspected easily to see if it
		// came from the context being canceled. Thus, we do this racy thing where if the
		// context is canceled at this point, we return it, rather than return the error
		// from dialing. It's a slight lie, but arguably still correct because the cancel
		// must be racing with the dial anyway.
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			return nil, err
		}
	}

	if tcpconn, ok := conn.(*net.TCPConn); d.TCPUserTimeout > 0 && ok {
		if err := netutil.SetUserTimeout(tcpconn, d.TCPUserTimeout); err != nil {
			return nil, errs.Combine(Error.Wrap(err), Error.Wrap(conn.Close()))
		}
	}

	return &timedConn{
		Conn: netutil.TrackClose(conn),
		rate: d.TransferRate,
	}, nil
}

// DialNode creates an rpc connection to the specified node.
func (d Dialer) DialNode(ctx context.Context, node *pb.Node) (_ *Conn, err error) {
	if node == nil {
		return nil, Error.New("node is nil")
	}

	defer mon.Task()(&ctx, "node: "+node.Id.String()[0:8])(&err)

	if d.TLSOptions == nil {
		return nil, Error.New("tls options not set when required for this dial")
	}

	return d.dial(ctx, node.GetAddress().GetAddress(), d.TLSOptions.ClientTLSConfig(node.Id))
}

// DialAddressID dials to the specified address and asserts it has the given node id.
func (d Dialer) DialAddressID(ctx context.Context, address string, id storj.NodeID) (_ *Conn, err error) {
	defer mon.Task()(&ctx, "node: "+id.String()[0:8])(&err)

	if d.TLSOptions == nil {
		return nil, Error.New("tls options not set when required for this dial")
	}

	return d.dial(ctx, address, d.TLSOptions.ClientTLSConfig(id))
}

// DialNodeURL dials to the specified node url and asserts it has the given node id.
func (d Dialer) DialNodeURL(ctx context.Context, nodeURL storj.NodeURL) (_ *Conn, err error) {
	defer mon.Task()(&ctx, "node: "+nodeURL.ID.String()[0:8])(&err)

	if d.TLSOptions == nil {
		return nil, Error.New("tls options not set when required for this dial")
	}

	return d.dial(ctx, nodeURL.Address, d.TLSOptions.ClientTLSConfig(nodeURL.ID))
}

// DialAddressInsecureBestEffort is like DialAddressInsecure but tries to dial a node securely if
// it can.
//
// nodeURL is like a storj.NodeURL but (a) requires an address and (b) does not require a
// full node id and will work with just a node prefix. The format is either:
//  * node_host:node_port
//  * node_id_prefix@node_host:node_port
// Examples:
//  * 33.20.0.1:7777
//  * [2001:db8:1f70::999:de8:7648:6e8]:7777
//  * 12vha9oTFnerx@33.20.0.1:7777
//  * 12vha9oTFnerx@[2001:db8:1f70::999:de8:7648:6e8]:7777
//
// DialAddressInsecureBestEffort:
//  * will use a node id if provided in the nodeURL paramenter
//  * will otherwise look up the node address in a known map of node address to node ids and use
// 		the remembered node id.
//  * will otherwise dial insecurely
func (d Dialer) DialAddressInsecureBestEffort(ctx context.Context, nodeURL string) (_ *Conn, err error) {
	defer mon.Task()(&ctx)(&err)

	if d.TLSOptions == nil {
		return nil, Error.New("tls options not set when required for this dial")
	}

	var nodeIDPrefix, nodeAddress string
	parts := strings.Split(nodeURL, "@")
	switch len(parts) {
	default:
		return nil, Error.New("malformed node url: %q", nodeURL)
	case 1:
		nodeAddress = parts[0]
	case 2:
		nodeIDPrefix, nodeAddress = parts[0], parts[1]
	}

	if len(nodeIDPrefix) > 0 {
		return d.dial(ctx, nodeAddress, d.TLSOptions.ClientTLSConfigPrefix(nodeIDPrefix))
	}

	if nodeID, found := KnownNodeID(nodeAddress); found {
		return d.dial(ctx, nodeAddress, d.TLSOptions.ClientTLSConfig(nodeID))
	}

	zap.L().Warn(`Unknown node id for address. Specify node id in the form "node_id@node_host:node_port" for added security`,
		zap.String("Address", nodeAddress),
	)
	return d.dial(ctx, nodeAddress, d.TLSOptions.UnverifiedClientTLSConfig())
}

// DialAddressInsecure dials to the specified address and does not check the node id.
func (d Dialer) DialAddressInsecure(ctx context.Context, address string) (_ *Conn, err error) {
	defer mon.Task()(&ctx)(&err)

	if d.TLSOptions == nil {
		return nil, Error.New("tls options not set when required for this dial")
	}

	return d.dial(ctx, address, d.TLSOptions.UnverifiedClientTLSConfig())
}

// DialAddressUnencrypted dials to the specified address without tls.
func (d Dialer) DialAddressUnencrypted(ctx context.Context, address string) (_ *Conn, err error) {
	defer mon.Task()(&ctx)(&err)

	return d.dialUnencrypted(ctx, address)
}

// drpcHeader is the first bytes we send on a connection so that the remote
// knows to expect drpc on the wire instead of grpc.
const drpcHeader = "DRPC!!!1"

// dial performs the dialing to the drpc endpoint with tls.
func (d Dialer) dial(ctx context.Context, address string, tlsConfig *tls.Config) (_ *Conn, err error) {
	defer mon.Task()(&ctx)(&err)

	// include the timeout here so that it includes all aspects of the dial
	if d.DialTimeout > 0 {
		var cancel func()
		ctx, cancel = context.WithTimeout(ctx, d.DialTimeout)
		defer cancel()
	}

	pool := rpcpool.New(d.PoolOptions, func(ctx context.Context) (drpc.Transport, error) {
		return d.dialTransport(ctx, address, tlsConfig)
	})

	conn, err := d.dialTransport(ctx, address, tlsConfig)
	if err != nil {
		return nil, err
	}
	state := conn.ConnectionState()

	if err := pool.Put(drpcconn.New(conn)); err != nil {
		return nil, err
	}

	return &Conn{
		state: state,
		Conn:  rpctracing.NewTracingWrapper(pool),
	}, nil
}

// dialTransport performs dialing to the drpc endpoint with tls.
func (d Dialer) dialTransport(ctx context.Context, address string, tlsConfig *tls.Config) (_ *tlsConnWrapper, err error) {
	defer mon.Task()(&ctx)(&err)

	// open the tcp socket to the address
	rawConn, err := d.dialContext(ctx, address)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	rawConn = newDrpcHeaderConn(rawConn)

	// perform the handshake racing with the context closing. we use a buffer
	// of size 1 so that the handshake can proceed even if no one is reading.
	errCh := make(chan error, 1)
	conn := tls.Client(rawConn, tlsConfig)
	go func() { errCh <- conn.Handshake() }()

	// see which wins and close the raw conn if there was any error. we can't
	// close the tls connection concurrently with handshakes or it sometimes
	// will panic. cool, huh?
	select {
	case <-ctx.Done():
		err = ctx.Err()
	case err = <-errCh:
	}
	if err != nil {
		_ = rawConn.Close()
		return nil, Error.Wrap(err)
	}

	return &tlsConnWrapper{
		Conn:       conn,
		underlying: rawConn,
	}, nil
}

// dialUnencrypted performs dialing to the drpc endpoint with no tls.
func (d Dialer) dialUnencrypted(ctx context.Context, address string) (_ *Conn, err error) {
	defer mon.Task()(&ctx)(&err)

	// include the timeout here so that it includes all aspects of the dial
	if d.DialTimeout > 0 {
		var cancel func()
		ctx, cancel = context.WithTimeout(ctx, d.DialTimeout)
		defer cancel()
	}

	conn := rpcpool.New(d.PoolOptions, func(ctx context.Context) (drpc.Transport, error) {
		return d.dialTransportUnencrypted(ctx, address)
	})
	return &Conn{
		Conn: rpctracing.NewTracingWrapper(conn),
	}, nil
}

// dialTransportUnencrypted performs dialing to the drpc endpoint with no tls.
func (d Dialer) dialTransportUnencrypted(ctx context.Context, address string) (_ net.Conn, err error) {
	defer mon.Task()(&ctx)(&err)

	// open the tcp socket to the address
	conn, err := d.dialContext(ctx, address)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return newDrpcHeaderConn(conn), nil
}

// tlsConnWrapper is a wrapper around a *tls.Conn that calls Close on the
// underlying connection when closed rather than trying to send a
// notification to the other side which may block forever.
type tlsConnWrapper struct {
	*tls.Conn
	underlying net.Conn
}

// Close closes the underlying connection.
func (t *tlsConnWrapper) Close() error { return t.underlying.Close() }

// drpcHeaderConn fulfills the net.Conn interface. On the first call to Write
// it will write the drpcHeader.
type drpcHeaderConn struct {
	net.Conn
	once sync.Once
}

// newDrpcHeaderConn returns a new *drpcHeaderConn.
func newDrpcHeaderConn(conn net.Conn) *drpcHeaderConn {
	return &drpcHeaderConn{
		Conn: conn,
	}
}

// Write will write buf to the underlying conn. If this is the first time Write
// is called it will prepend the drpcHeader to the beginning of the write.
func (d *drpcHeaderConn) Write(buf []byte) (n int, err error) {
	var didOnce bool
	d.once.Do(func() {
		didOnce = true
		header := []byte(drpcHeader)
		n, err = d.Conn.Write(append(header, buf...))
	})
	if didOnce {
		n -= len(drpcHeader)
		if n < 0 {
			n = 0
		}
		return n, err
	}
	return d.Conn.Write(buf)
}
