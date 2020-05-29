# package drpc

`import "storj.io/drpc"`

Package drpc is a light replacement for gprc.

## Usage

```go
var (
	Error         = errs.Class("drpc")
	InternalError = errs.Class("internal error")
	ProtocolError = errs.Class("protocol error")
)
```
These error classes represent some common errors that drpc generates.

#### type Conn

```go
type Conn interface {
	// Close closes the connection.
	Close() error

	// Closed returns true if the connection is definitely closed.
	Closed() bool

	// Transport returns the transport the connection is using.
	Transport() Transport

	// Invoke issues a unary rpc to the remote. Only one Invoke or Stream may be
	// open at once.
	Invoke(ctx context.Context, rpc string, in, out Message) error

	// NewStream starts a stream with the remote. Only one Invoke or Stream may be
	// open at once.
	NewStream(ctx context.Context, rpc string) (Stream, error)
}
```

Conn represents a client connection to a server.

#### type Description

```go
type Description interface {
	// NumMethods returns the number of methods available.
	NumMethods() int

	// Method returns the information about the nth method along with a handler
	// to invoke it. The method interface that it returns is expected to be
	// a method expression like `(*Type).HandlerName`.
	Method(n int) (rpc string, receiver Receiver, method interface{}, ok bool)
}
```

Description is the interface implemented by things that can be registered by a
Server.

#### type Handler

```go
type Handler interface {
	HandleRPC(stream Stream, rpc string) (err error)
}
```

Handler handles streams and rpcs dispatched to it by a Server.

#### type Message

```go
type Message interface {
	Reset()
	String() string
	ProtoMessage()
}
```

Message is a protobuf message, just here so protobuf isn't necessary to import
or be exposed in the types.

#### type Mux

```go
type Mux interface {
	Register(srv interface{}, desc Description) error
}
```

Mux is a type that can have an implementation and a Description registered with
it.

#### type Receiver

```go
type Receiver = func(srv interface{}, ctx context.Context, in1, in2 interface{}) (out Message, err error)
```

Receiver is invoked by a server for a given rpc.

#### type Stream

```go
type Stream interface {
	// Context returns the context associated with the stream. It is canceled
	// when the Stream is closed and no more messages will ever be sent or
	// received on it.
	Context() context.Context

	// MsgSend sends the Message to the remote.
	MsgSend(msg Message) error

	// MsgRecv receives a Message from the remote.
	MsgRecv(msg Message) error

	// CloseSend signals to the remote that we will no longer send any messages.
	CloseSend() error

	// Close closes the stream.
	Close() error
}
```

Stream is a bi-directional stream of messages to some other party.

#### type Transport

```go
type Transport interface {
	io.Reader
	io.Writer
	io.Closer
}
```

Transport is an interface describing what is required for a drpc connection.
