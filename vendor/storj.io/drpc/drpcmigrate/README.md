# package drpcmigrate

`import "storj.io/drpc/drpcmigrate"`

Package drpcmigrate provides tools to support drpc concurrently alongside gRPC
on the same ports. See the grpc_and_drpc example in the examples folder for
expected usage.

## Usage

```go
var Closed = errs.New("listener closed")
```
Closed is returned by routed listeners when the mux is closed.

```go
var DRPCHeader = "DRPC!!!1"
```
DRPCHeader is a header for DRPC connections to use. This is designed to not
conflict with a headerless gRPC, HTTP, or TLS request.

#### func  DialWithHeader

```go
func DialWithHeader(ctx context.Context, network, address string, header string) (net.Conn, error)
```
DialWithHeader is like net.Dial, but uses HeaderConns with the provided header.

#### type HeaderConn

```go
type HeaderConn struct {
	net.Conn
}
```

HeaderConn fulfills the net.Conn interface. On the first call to Write it will
write the Header.

#### func  NewHeaderConn

```go
func NewHeaderConn(conn net.Conn, header string) *HeaderConn
```
NewHeaderConn returns a new *HeaderConn that writes the provided header as part
of the first Write.

#### func (*HeaderConn) Write

```go
func (d *HeaderConn) Write(buf []byte) (n int, err error)
```
Write will write buf to the underlying conn. If this is the first time Write is
called it will prepend the Header to the beginning of the write.

#### type HeaderDialer

```go
type HeaderDialer struct {
	net.Dialer
	Header string
}
```

HeaderDialer is a net.Dialer-like that prefixes all conns with the provided
header.

#### func (*HeaderDialer) Dial

```go
func (d *HeaderDialer) Dial(network, address string) (net.Conn, error)
```
Dial will dial the address on the named network, creating a connection that will
write the configured Header on the first user-requested write.

#### func (*HeaderDialer) DialContext

```go
func (d *HeaderDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error)
```
DialContext will dial the address on the named network, creating a connection
that will write the configured Header on the first user-requested write.

#### type ListenMux

```go
type ListenMux struct {
}
```

ListenMux lets one multiplex a listener into different listeners based on the
first bytes sent on the connection.

#### func  NewListenMux

```go
func NewListenMux(base net.Listener, prefixLen int) *ListenMux
```
NewListenMux creates a ListenMux that reads the prefixLen bytes from any
connections Accepted by the passed in listener and dispatches to the appropriate
route.

#### func (*ListenMux) Default

```go
func (m *ListenMux) Default() net.Listener
```
Default returns the net.Listener that is used if no route matches.

#### func (*ListenMux) Route

```go
func (m *ListenMux) Route(prefix string) net.Listener
```
Route returns a listener that will be used if the first bytes are the given
prefix. The length of the prefix must match the original passed in prefixLen.

#### func (*ListenMux) Run

```go
func (m *ListenMux) Run(ctx context.Context) error
```
Run calls listen on the provided listener and passes connections to the routed
listeners.
