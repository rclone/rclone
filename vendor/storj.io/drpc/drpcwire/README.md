# package drpcwire

`import "storj.io/drpc/drpcwire"`

Package drpcwire provides low level helpers for the drpc wire protocol.

## Usage

#### func  AppendFrame

```go
func AppendFrame(buf []byte, fr Frame) []byte
```
AppendFrame appends a marshaled form of the frame to the provided buffer.

#### func  AppendVarint

```go
func AppendVarint(buf []byte, x uint64) []byte
```
AppendVarint appends the varint encoding of x to the buffer and returns it.

#### func  MarshalError

```go
func MarshalError(err error) []byte
```
MarshalError returns a byte form of the error with any error code incorporated.

#### func  ReadVarint

```go
func ReadVarint(buf []byte) (rem []byte, out uint64, ok bool, err error)
```
ReadVarint reads a varint encoded integer from the front of buf, returning the
remaining bytes, the value, and if there was a success. if ok is false, the
returned buffer is the same as the passed in buffer.

#### func  SplitData

```go
func SplitData(buf []byte, n int) (prefix, suffix []byte)
```
SplitData is used to split a buffer if it is larger than n bytes. If n is zero,
a reasonable default is used. If n is less than zero then it does not split.

#### func  SplitN

```go
func SplitN(pkt Packet, n int, cb func(fr Frame) error) error
```
SplitN splits the marshaled form of the Packet into a number of frames such that
each frame is at most n bytes. It calls the callback with every such frame. If n
is zero, a reasonable default is used.

#### func  UnmarshalError

```go
func UnmarshalError(data []byte) error
```
UnmarshalError unmarshals the marshaled error to one with a code.

#### type Frame

```go
type Frame struct {
	// Data is the payload of bytes.
	Data []byte

	// ID is used so that the frame can be reconstructed.
	ID ID

	// Kind is the kind of the payload.
	Kind Kind

	// Done is true if this is the last frame for the ID.
	Done bool

	// Control is true if the frame has the control bit set.
	Control bool
}
```

Frame is a split data frame on the wire.

#### func  ParseFrame

```go
func ParseFrame(buf []byte) (rem []byte, fr Frame, ok bool, err error)
```
ParseFrame attempts to parse a frame at the beginning of buf. If successful then
rem contains the unparsed data, fr contains the parsed frame, ok will be true,
and err will be nil. If there is not enough data for a frame, ok will be false
and err will be nil. If the data in the buf is malformed, then an error is
returned.

#### func (Frame) String

```go
func (fr Frame) String() string
```
String returns a human readable form of the packet.

#### type ID

```go
type ID struct {
	// Stream is the stream identifier.
	Stream uint64

	// Message is the message identifier.
	Message uint64
}
```

ID represents a packet id.

#### func (ID) Less

```go
func (i ID) Less(j ID) bool
```
Less returns true if the id is less than the provided one. An ID is less than
another if the Stream is less, and if the stream is equal, if the Message is
less.

#### func (ID) String

```go
func (i ID) String() string
```
String returns a human readable form of the ID.

#### type Kind

```go
type Kind uint8
```

Kind is the enumeration of all the different kinds of messages drpc sends.

```go
const (

	// KindInvoke is used to invoke an rpc. The body is the name of the rpc.
	KindInvoke Kind = 1

	// KindMessage is used to send messages. The body is an encoded message.
	KindMessage Kind = 2

	// KindError is used to inform that an error happened. The body is an error
	// with a code attached.
	KindError Kind = 3

	// KindCancel is sent to notify the remote that we have soft canceled.
	KindCancel Kind = 4

	// KindClose is used to inform that the rpc is dead. It has no body.
	KindClose Kind = 5

	// KindCloseSend is used to inform that no more messages will be sent.
	// It has no body.
	KindCloseSend Kind = 6 // body must be empty

	// KindInvokeMetadata includes metadata about the next Invoke packet.
	KindInvokeMetadata Kind = 7
)
```

#### func (Kind) String

```go
func (i Kind) String() string
```

#### type Packet

```go
type Packet struct {
	// Data is the payload of the packet.
	Data []byte

	// ID is the identifier for the packet.
	ID ID

	// Kind is the kind of the packet.
	Kind Kind

	// Control is set to true for packets that are
	// forwards compatible. Unknown or invalid packets
	// with the control bool set should be ignored
	// instead of triggering any errors.
	Control bool
}
```

Packet is a single message sent by drpc.

#### func (Packet) String

```go
func (p Packet) String() string
```
String returns a human readable form of the packet.

#### type Reader

```go
type Reader struct {
}
```

Reader reconstructs packets from frames read from an io.Reader.

#### func  NewReader

```go
func NewReader(r io.Reader) *Reader
```
NewReader constructs a Reader to read Packets from the io.Reader.

#### func  NewReaderWithOptions

```go
func NewReaderWithOptions(r io.Reader, opts ReaderOptions) *Reader
```
NewReaderWithOptions constructs a Reader to read Packets from the io.Reader. It
uses the provided options to manage buffering.

#### func (*Reader) ReadPacket

```go
func (r *Reader) ReadPacket() (pkt Packet, err error)
```
ReadPacket reads a packet from the io.Reader. It is equivalent to calling
ReadPacketUsing(nil).

#### func (*Reader) ReadPacketUsing

```go
func (r *Reader) ReadPacketUsing(buf []byte) (pkt Packet, err error)
```
ReadPacketUsing reads a packet from the io.Reader. IDs read from frames must be
monotonically increasing. When a new ID is read, the old data is discarded. This
allows for easier asynchronous interrupts. If the amount of data in the Packet
becomes too large, an error is returned. The returned packet's Data field is
constructed by appending to the provided buf after it has been resliced to be
zero length.

#### type ReaderOptions

```go
type ReaderOptions struct {
	// MaximumBufferSize controls the maximum size of buffered
	// packet data.
	MaximumBufferSize int
}
```

ReaderOptions controls configuration settings for a reader.

#### type Writer

```go
type Writer struct {
}
```

Writer is a helper to buffer and write packets and frames to an io.Writer.

#### func  NewWriter

```go
func NewWriter(w io.Writer, size int) *Writer
```
NewWriter returns a Writer that will attempt to buffer size data before sending
it to the io.Writer.

#### func (*Writer) Empty

```go
func (b *Writer) Empty() bool
```
Empty returns true if there are no bytes buffered in the writer.

#### func (*Writer) Flush

```go
func (b *Writer) Flush() (err error)
```
Flush forces a flush of any buffered data to the io.Writer. It is a no-op if
there is no data in the buffer.

#### func (*Writer) Reset

```go
func (b *Writer) Reset() *Writer
```
Reset clears any pending data in the buffer.

#### func (*Writer) WriteFrame

```go
func (b *Writer) WriteFrame(fr Frame) (err error)
```
WriteFrame appends the frame into the buffer, and if the buffer is larger than
the configured size, flushes it.

#### func (*Writer) WritePacket

```go
func (b *Writer) WritePacket(pkt Packet) (err error)
```
WritePacket writes the packet as a single frame, ignoring any size constraints.
