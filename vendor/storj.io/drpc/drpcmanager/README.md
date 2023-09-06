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
func (m *Manager) Closed() <-chan struct{}
```
Closed returns a channel that is closed once the manager is closed.

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

#### func (*Manager) String

```go
func (m *Manager) String() string
```
String returns a string representation of the manager.

#### func (*Manager) Unblocked

```go
func (m *Manager) Unblocked() <-chan struct{}
```
Unblocked returns a channel that is closed when the manager is no longer blocked
from creating a new stream due to a previous stream's soft cancel. It should not
be called concurrently with NewClientStream or NewServerStream and the return
result is only valid until the next call to NewClientStream or NewServerStream.

#### type Options

```go
type Options struct {
	// WriterBufferSize controls the size of the buffer that we will fill before
	// flushing. Normal writes to streams typically issue a flush explicitly.
	WriterBufferSize int

	// Reader are passed to any readers the manager creates.
	Reader drpcwire.ReaderOptions

	// Stream are passed to any streams the manager creates.
	Stream drpcstream.Options

	// SoftCancel controls if a context cancel will cause the transport to be
	// closed or, if true, a soft cancel message will be attempted if possible.
	// A soft cancel can reduce the amount of closed and dialed connections at
	// the potential cost of higher latencies if there is latent data still being
	// flushed when the cancel happens.
	SoftCancel bool

	// InactivityTimeout is the amount of time the manager will wait when creating
	// a NewServerStream. It only includes the time it is reading packets from the
	// remote client. In other words, it only includes the time that the client
	// could delay before invoking an RPC. If zero or negative, no timeout is used.
	InactivityTimeout time.Duration
}
```

Options controls configuration settings for a manager.
