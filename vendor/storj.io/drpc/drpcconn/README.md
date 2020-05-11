# package drpcconn

`import "storj.io/drpc/drpcconn"`

Package drpcconn creates a drpc client connection from a transport.

## Usage

#### type Conn

```go
type Conn struct {
}
```

Conn is a drpc client connection.

#### func  New

```go
func New(tr drpc.Transport) *Conn
```
New returns a conn that uses the transport for reads and writes.

#### func  NewWithOptions

```go
func NewWithOptions(tr drpc.Transport, opts Options) *Conn
```
NewWithOptions returns a conn that uses the transport for reads and writes. The
Options control details of how the conn operates.

#### func (*Conn) Close

```go
func (c *Conn) Close() (err error)
```
Close closes the connection.

#### func (*Conn) Closed

```go
func (c *Conn) Closed() bool
```
Closed returns true if the connection is already closed.

#### func (*Conn) Invoke

```go
func (c *Conn) Invoke(ctx context.Context, rpc string, in, out drpc.Message) (err error)
```
Invoke issues the rpc on the transport serializing in, waits for a response, and
deserializes it into out. Only one Invoke or Stream may be open at a time.

#### func (*Conn) NewStream

```go
func (c *Conn) NewStream(ctx context.Context, rpc string) (_ drpc.Stream, err error)
```
NewStream begins a streaming rpc on the connection. Only one Invoke or Stream
may be open at a time.

#### func (*Conn) Transport

```go
func (c *Conn) Transport() drpc.Transport
```
Transport returns the transport the conn is using.

#### type Options

```go
type Options struct {
	// Manager controls the options we pass to the manager of this conn.
	Manager drpcmanager.Options
}
```

Options controls configuration settings for a conn.
