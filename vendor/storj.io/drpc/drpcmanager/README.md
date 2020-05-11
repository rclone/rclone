# package drpcmanager

`import "storj.io/drpc/drpcmanager"`

Package drpcmanager reads packets from a transport to make streams.

## Usage

#### type Manager

```go
type Manager struct {
}
```

Manager handles the logic of managing a transport for a drpc client or server.
It ensures that the connection is always being read from, that it is closed in
the case that the manager is and forwarding drpc protocol messages to the
appropriate stream.

#### func  New

```go
func New(tr drpc.Transport) *Manager
```
New returns a new Manager for the transport.

#### func  NewWithOptions

```go
func NewWithOptions(tr drpc.Transport, opts Options) *Manager
```
NewWithOptions returns a new manager for the transport. It uses the provided
options to manage details of how it uses it.

#### func (*Manager) Close

```go
func (m *Manager) Close() error
```
Close closes the transport the manager is using.

#### func (*Manager) Closed

```go
func (m *Manager) Closed() bool
```
Closed returns if the manager has been closed.

#### func (*Manager) NewClientStream

```go
func (m *Manager) NewClientStream(ctx context.Context) (stream *drpcstream.Stream, err error)
```
NewClientStream starts a stream on the managed transport for use by a client.

#### func (*Manager) NewServerStream

```go
func (m *Manager) NewServerStream(ctx context.Context) (stream *drpcstream.Stream, rpc string, err error)
```
NewServerStream starts a stream on the managed transport for use by a server. It
does this by waiting for the client to issue an invoke message and returning the
details.

#### type Options

```go
type Options struct {
	// WriterBufferSize controls the size of the buffer that we will fill before
	// flushing. Normal writes to streams typically issue a flush explicitly.
	WriterBufferSize int

	// Stream are passed to any streams the manager creates.
	Stream drpcstream.Options
}
```

Options controls configuration settings for a manager.
