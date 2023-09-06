// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package rpc

import (
	"context"
	"crypto/tls"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/memory"
	"storj.io/common/netutil"
	"storj.io/common/rpc/multidial"
	"storj.io/common/socket"
	"storj.io/drpc/drpcmigrate"
)

var (
	// STORJ_RPC_SET_LINGER_0 defaults to false below.
	enableSetLinger0, _ = strconv.ParseBool(os.Getenv("STORJ_RPC_SET_LINGER_0"))
)

type ctxKeyTCPFastOpenMultidial struct{}
type ctxKeyBackgroundQoS struct{}

// ConnectorConn is a type that creates a connection and establishes a tls
// session.
type ConnectorConn interface {
	net.Conn
	ConnectionState() tls.ConnectionState
}

// Connector is a type that creates a ConnectorConn, given an address and
// a tls configuration.
type Connector interface {
	// DialContext is called to establish a encrypted connection using tls.
	DialContext(ctx context.Context, tlsconfig *tls.Config, address string) (ConnectorConn, error)
}

// DialFunc represents a dialer that can establish a net.Conn.
type DialFunc func(ctx context.Context, network, address string) (net.Conn, error)

// TCPConnector implements a dialer that creates an encrypted connection using tls.
type TCPConnector struct {
	// TCPUserTimeout controls what setting to use for the TCP_USER_TIMEOUT
	// socket option on dialed connections. Only valid on linux. Only set
	// if positive.
	TCPUserTimeout time.Duration

	// TransferRate limits all read/write operations to go slower than
	// the size per second if it is non-zero.
	TransferRate memory.Size

	// SendDRPCMuxHeader caused the connector to send a preamble after TCP handshake
	// but before the TLS handshake.
	// This was used to migrate from gRPC to DRPC.
	// This needs to be false when connecting through a TLS termination proxy.
	SendDRPCMuxHeader bool

	providedDialer DialFunc
}

// NewDefaultTCPConnector creates a new TCPConnector instance with provided tcp dialer.
// If no dialer is predefined, net.Dialer is used by default.
//
// Deprecated: Use NewHybridConnector wherever possible instead.
func NewDefaultTCPConnector(dialer DialFunc) *TCPConnector {
	return &TCPConnector{
		TCPUserTimeout:    15 * time.Minute,
		SendDRPCMuxHeader: true,
		providedDialer:    dialer,
	}
}

// DialContext creates a encrypted tcp connection using tls.
func (t *TCPConnector) DialContext(ctx context.Context, tlsConfig *tls.Config, address string) (_ ConnectorConn, err error) {
	defer mon.Task()(&ctx)(&err)

	rawConn, err := t.DialContextUnencrypted(ctx, address)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	conn := tls.Client(rawConn, tlsConfig)
	err = conn.HandshakeContext(ctx)
	if err != nil {
		_ = rawConn.Close()
		return nil, Error.Wrap(err)
	}

	return &tlsConnWrapper{
		Conn:       conn,
		underlying: rawConn,
	}, nil
}

// DialContextUnencrypted creates a raw tcp connection.
func (t *TCPConnector) DialContextUnencrypted(ctx context.Context, address string) (_ net.Conn, err error) {
	defer mon.Task()(&ctx)(&err)

	conn, err := t.DialContextUnencryptedUnprefixed(ctx, address)
	if err != nil {
		return nil, err
	}

	if t.SendDRPCMuxHeader {
		conn = drpcmigrate.NewHeaderConn(conn, drpcmigrate.DRPCHeader)
	}

	return conn, nil
}

// DialContextUnencryptedUnprefixed creates a raw TCP connection without any prefixes.
func (t *TCPConnector) DialContextUnencryptedUnprefixed(ctx context.Context, address string) (_ net.Conn, err error) {
	defer mon.Task()(&ctx)(&err)

	if t.providedDialer != nil {
		return t.lowLevelDial(ctx, t.providedDialer, "provided", address)
	}

	getCtxBool := func(key interface{}) bool {
		v, ok := ctx.Value(key).(bool)
		return v && ok
	}

	dialer := socket.ExtendedDialer{
		LowPrioCongestionControl: getCtxBool(ctxKeyBackgroundQoS{}),
		LowEffortQoS:             getCtxBool(ctxKeyBackgroundQoS{}),
	}

	if !socket.TCPFastOpenConnectSupported || !getCtxBool(ctxKeyTCPFastOpenMultidial{}) {
		return t.lowLevelDial(ctx, dialer.DialContext, "standard", address)
	}

	return multidial.NewMultidialer(
		func(ctx context.Context, network, address string) (net.Conn, error) {
			standard := dialer
			standard.TCPFastOpenConnect = false
			return t.lowLevelDial(ctx, standard.DialContext, "standard", address)
		},
		func(ctx context.Context, network, address string) (net.Conn, error) {
			fastopen := dialer
			fastopen.TCPFastOpenConnect = true
			return t.lowLevelDial(ctx, fastopen.DialContext, "fastopen", address)
		},
	).DialContext(ctx, "", address)
}

func (t *TCPConnector) lowLevelDial(ctx context.Context, dial DialFunc, style, address string) (_ net.Conn, err error) {
	defer mon.Task()(&ctx, style)(&err)

	conn, err := dial(ctx, "tcp", address)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	if tcpconn, ok := conn.(*net.TCPConn); ok {
		/*
			The normal TCP termination sequence looks like this (simplified):

			We have two peers: A and B

			 1. A calls close()
				* A sends FIN to B
				* A goes into FIN_WAIT_1 state
			 2. B receives FIN
				* B sends ACK to A
				* B goes into CLOSE_WAIT state
			 3. A receives ACK
				* A goes into FIN_WAIT_2 state
			 4. B calls close()
				* B sends FIN to A
				* B goes into LAST_ACK state
			 5. A receives FIN
				* A sends ACK to B
				* A goes into TIME_WAIT state
			 6. B receives ACK
				* B goes to CLOSED state – i.e. is removed from the socket tables

			(From https://stackoverflow.com/a/13088864)

			Note that A doesn't leave TIME_WAIT! This is the normal operation of TCP connections,
			and indeed can be validated by building a toy client and server that do a small
			data exchange and then close their connections cleanly. The client will enter
			TIME_WAIT.

			What happens with TLS and Go (most times):

			However, this is not what happens with TLS and Go. The TLS spec mandates that
			close-notify messages are sent by both the client and the server upon closing
			the write side of the socket. https://www.rfc-editor.org/rfc/rfc5246#section-7.2.1
			Unfortunately, common usage of Go + TLS involves invoking Close, which closes
			both the read and the write side of the socket. The client OS puts this
			socket into TIME_WAIT.

			When Close is called on a TLS-using Go connection, it sends a close-notify
			message, then closes both the read and write sides of its socket. The server
			will receive the close notify, close its read side, then send a close-notify
			of its own, closing its write side.

			The client operating system receives the unexpected-to-it close-notify and
			freaks out, sending a RST packet, which then ends the socket's TIME_WAIT
			state.

			How this pertains to Storj:

			The Go + TLS situation has been the most common situation for us for
			some time, and so as a result, we have not been running our connections
			with clean TCP shut down. One side effect is we've been sending more
			connection teardown messages than we need (RST and abortive close),
			but the other side effect is that we haven't had many TIME_WAIT
			connections. Switching to a protocol with clean shutdown suddenly balloons
			the number of TIME_WAIT connections.

			We can fix this three ways:

			 (a) Make it so protocols continue to behave like TLS - e.g., continue
			     to have the server send a close notify, even though we know
			     the client has already shut down its read side, guaranteeing
			     an abortive close RST packet. This seems wasteful.
			 (b) Just tell the kernel that we mean to do an abortive close like
			     we've been doing, but without the additional packet waste.
			 (c) Make our firewalls and app servers okay with clean TCP shutdown.
			     Keeping track of a connection in TIME_WAIT should not require
			     many resources. We likely have something gravely misconfigured
			     that we can't handle clean connection closes.

			Setting SetLinger(0) is option b.
			enableSetLinger0 defaults to false so that we can pursue option c.
		*/
		if enableSetLinger0 {
			if err := tcpconn.SetLinger(0); err != nil {
				return nil, errs.Combine(Error.Wrap(err), Error.Wrap(conn.Close()))
			}
		}

		if t.TCPUserTimeout > 0 {
			if err := netutil.SetUserTimeout(tcpconn, t.TCPUserTimeout); err != nil {
				return nil, errs.Combine(Error.Wrap(err), Error.Wrap(conn.Close()))
			}
		}
	}

	return &timedConn{
		Conn: netutil.TrackClose(conn),
		rate: t.TransferRate,
	}, nil
}

// SetTransferRate sets the transfer rate member for this TCPConnector
// instance. This is mainly provided for interface compatibility with other
// connectors.
func (t *TCPConnector) SetTransferRate(rate memory.Size) {
	t.TransferRate = rate
}

// SetSendDRPCMuxHeader says whether we should send the DRPC mux header.
func (t *TCPConnector) SetSendDRPCMuxHeader(send bool) {
	t.SendDRPCMuxHeader = send
}
