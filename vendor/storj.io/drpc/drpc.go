// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpc

import (
	"context"
	"io"

	"github.com/zeebo/errs"
)

// These error classes represent some common errors that drpc generates.
var (
	Error         = errs.Class("drpc")
	InternalError = errs.Class("internal error")
	ProtocolError = errs.Class("protocol error")
	ClosedError   = errs.Class("closed")
)

// Transport is an interface describing what is required for a drpc connection.
// Any net.Conn can be used as a Transport.
type Transport interface {
	io.Reader
	io.Writer
	io.Closer
}

// Message is a protobuf message. It is expected to be used with an Encoding.
// This exists so that one can use whatever protobuf library/runtime they want.
type Message interface{}

// Conn represents a client connection to a server.
type Conn interface {
	// Close closes the connection.
	Close() error

	// Closed returns a channel that is closed if the connection is definitely closed.
	Closed() <-chan struct{}

	// Invoke issues a unary RPC to the remote. Only one Invoke or Stream may be
	// open at once.
	Invoke(ctx context.Context, rpc string, enc Encoding, in, out Message) error

	// NewStream starts a stream with the remote. Only one Invoke or Stream may be
	// open at once.
	NewStream(ctx context.Context, rpc string, enc Encoding) (Stream, error)
}

// Stream is a bi-directional stream of messages to some other party.
type Stream interface {
	// Context returns the context associated with the stream. It is canceled
	// when the Stream is closed and no more messages will ever be sent or
	// received on it.
	Context() context.Context

	// MsgSend sends the Message to the remote.
	MsgSend(msg Message, enc Encoding) error

	// MsgRecv receives a Message from the remote.
	MsgRecv(msg Message, enc Encoding) error

	// CloseSend signals to the remote that we will no longer send any messages.
	CloseSend() error

	// Close closes the stream.
	Close() error
}

// Receiver is invoked by a server for a given RPC.
type Receiver = func(srv interface{}, ctx context.Context, in1, in2 interface{}) (out Message, err error)

// Description is the interface implemented by things that can be registered by
// a Server.
type Description interface {
	// NumMethods returns the number of methods available.
	NumMethods() int

	// Method returns the information about the nth method along with a handler
	// to invoke it. The method interface that it returns is expected to be
	// a method expression like `(*Type).HandlerName`.
	Method(n int) (rpc string, encoding Encoding, receiver Receiver, method interface{}, ok bool)
}

// Mux is a type that can have an implementation and a Description registered with it.
type Mux interface {
	// Register marks that the description should dispatch RPCs that it describes to
	// the provided srv.
	Register(srv interface{}, desc Description) error
}

// Handler handles streams and RPCs dispatched to it by a Server.
type Handler interface {
	// HandleRPC executes the RPC identified by the rpc string using the stream to
	// communicate with the remote.
	HandleRPC(stream Stream, rpc string) (err error)
}

// Encoding represents a way to marshal/unmarshal Message types.
type Encoding interface {
	// Marshal returns the encoded form of msg.
	Marshal(msg Message) ([]byte, error)

	// Unmarshal reads the encoded form of some Message into msg.
	// The buf is expected to contain only a single complete Message.
	Unmarshal(buf []byte, msg Message) error
}
