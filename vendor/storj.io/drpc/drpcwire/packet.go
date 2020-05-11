// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcwire

import "fmt"

//go:generate stringer -type=Kind -trimprefix=Kind_ -output=packet_string.go

// Kind is the enumeration of all the different kinds of messages drpc sends.
type Kind uint8

const (
	// kindReserved is saved for the future in case we need to extend.
	kindReserved Kind = 0

	// kindCancelDeprecated is a reminder that we once used this kind value.
	kindCancelDeprecated Kind = 4

	// KindInvoke is used to invoke an rpc. The body is the name of the rpc.
	KindInvoke Kind = 1

	// KindMessage is used to send messages. The body is a protobuf.
	KindMessage Kind = 2

	// KindError is used to inform that an error happened. The body is an error
	// with a code attached.
	KindError Kind = 3

	// KindClose is used to inform that the rpc is dead. It has no body.
	KindClose Kind = 5

	// KindCloseSend is used to inform that no more messages will be sent.
	// It has no body.
	KindCloseSend Kind = 6 // body must be empty

	// KindInvokeMetadata includes metadata about the next Invoke packet.
	KindInvokeMetadata Kind = 7
)

//
// packet id
//

// ID represents a packet id.
type ID struct {
	// Stream is the stream identifier.
	Stream uint64

	// Message is the message identifier.
	Message uint64
}

// Less returns true if the id is less than the provided one. An ID is less than
// another if the Stream is less, and if the stream is equal, if the Message
// is less.
func (i ID) Less(j ID) bool {
	return i.Stream < j.Stream || (i.Stream == j.Stream && i.Message < j.Message)
}

// String returns a human readable form of the ID.
func (i ID) String() string { return fmt.Sprintf("<%d,%d>", i.Stream, i.Message) }

//
// data frame
//

// Frame is a split data frame on the wire.
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

// ParseFrame attempts to parse a frame at the beginning of buf. If successful
// then rem contains the unparsed data, fr contains the parsed frame, ok will
// be true, and err will be nil. If there is not enough data for a frame, ok
// will be false and err will be nil. If the data in the buf is malformed, then
// an error is returned.
func ParseFrame(buf []byte) (rem []byte, fr Frame, ok bool, err error) {
	var length uint64
	var control byte
	if len(buf) < 4 {
		goto bad
	}

	rem, control = buf[1:], buf[0]
	fr.Done = (control & 0b00000001) > 0
	fr.Control = (control & 0b10000000) > 0
	fr.Kind = Kind((control & 0b01111110) >> 1)
	rem, fr.ID.Stream, ok, err = ReadVarint(rem)
	if !ok || err != nil {
		goto bad
	}
	rem, fr.ID.Message, ok, err = ReadVarint(rem)
	if !ok || err != nil {
		goto bad
	}
	rem, length, ok, err = ReadVarint(rem)
	if !ok || err != nil || length > uint64(len(rem)) {
		goto bad
	}
	rem, fr.Data = rem[length:], rem[:length]

	return rem, fr, true, nil
bad:
	return buf, fr, false, err
}

// AppendFrame appends a marshaled form of the frame to the provided buffer.
func AppendFrame(buf []byte, fr Frame) []byte {
	control := byte(fr.Kind << 1)
	if fr.Done {
		control |= 0b00000001
	}
	if fr.Control {
		control |= 0b10000000
	}

	out := buf
	out = append(out, control)
	out = AppendVarint(out, fr.ID.Stream)
	out = AppendVarint(out, fr.ID.Message)
	out = AppendVarint(out, uint64(len(fr.Data)))
	out = append(out, fr.Data...)
	return out
}

//
// packet
//

// Packet is a single message sent by drpc.
type Packet struct {
	// Data is the payload of the packet.
	Data []byte

	// ID is the identifier for the packet.
	ID ID

	// Kind is the kind of the packet.
	Kind Kind
}

// String returns a human readable form of the packet.
func (p Packet) String() string {
	return fmt.Sprintf("<s:%d m:%d kind:%s data:%d>",
		p.ID.Stream, p.ID.Message, p.Kind, len(p.Data))
}
