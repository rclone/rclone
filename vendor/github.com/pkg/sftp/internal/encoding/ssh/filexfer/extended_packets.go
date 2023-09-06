package filexfer

import (
	"encoding"
	"sync"
)

// ExtendedData aliases the untyped interface composition of encoding.BinaryMarshaler and encoding.BinaryUnmarshaler.
type ExtendedData = interface {
	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler
}

// ExtendedDataConstructor defines a function that returns a new(ArbitraryExtendedPacket).
type ExtendedDataConstructor func() ExtendedData

var extendedPacketTypes = struct {
	mu           sync.RWMutex
	constructors map[string]ExtendedDataConstructor
}{
	constructors: make(map[string]ExtendedDataConstructor),
}

// RegisterExtendedPacketType defines a specific ExtendedDataConstructor for the given extension string.
func RegisterExtendedPacketType(extension string, constructor ExtendedDataConstructor) {
	extendedPacketTypes.mu.Lock()
	defer extendedPacketTypes.mu.Unlock()

	if _, exist := extendedPacketTypes.constructors[extension]; exist {
		panic("encoding/ssh/filexfer: multiple registration of extended packet type " + extension)
	}

	extendedPacketTypes.constructors[extension] = constructor
}

func newExtendedPacket(extension string) ExtendedData {
	extendedPacketTypes.mu.RLock()
	defer extendedPacketTypes.mu.RUnlock()

	if f := extendedPacketTypes.constructors[extension]; f != nil {
		return f()
	}

	return new(Buffer)
}

// ExtendedPacket defines the SSH_FXP_CLOSE packet.
type ExtendedPacket struct {
	ExtendedRequest string

	Data ExtendedData
}

// Type returns the SSH_FXP_xy value associated with this packet type.
func (p *ExtendedPacket) Type() PacketType {
	return PacketTypeExtended
}

// MarshalPacket returns p as a two-part binary encoding of p.
//
// The Data is marshaled into binary, and returned as the payload.
func (p *ExtendedPacket) MarshalPacket(reqid uint32, b []byte) (header, payload []byte, err error) {
	buf := NewBuffer(b)
	if buf.Cap() < 9 {
		size := 4 + len(p.ExtendedRequest) // string(extended-request)
		buf = NewMarshalBuffer(size)
	}

	buf.StartPacket(PacketTypeExtended, reqid)
	buf.AppendString(p.ExtendedRequest)

	if p.Data != nil {
		payload, err = p.Data.MarshalBinary()
		if err != nil {
			return nil, nil, err
		}
	}

	return buf.Packet(payload)
}

// UnmarshalPacketBody unmarshals the packet body from the given Buffer.
// It is assumed that the uint32(request-id) has already been consumed.
//
// If p.Data is nil, and the extension has been registered, a new type will be made from the registration.
// If the extension has not been registered, then a new Buffer will be allocated.
// Then the request-specific-data will be unmarshaled from the rest of the buffer.
func (p *ExtendedPacket) UnmarshalPacketBody(buf *Buffer) (err error) {
	if p.ExtendedRequest, err = buf.ConsumeString(); err != nil {
		return err
	}

	if p.Data == nil {
		p.Data = newExtendedPacket(p.ExtendedRequest)
	}

	return p.Data.UnmarshalBinary(buf.Bytes())
}

// ExtendedReplyPacket defines the SSH_FXP_CLOSE packet.
type ExtendedReplyPacket struct {
	Data ExtendedData
}

// Type returns the SSH_FXP_xy value associated with this packet type.
func (p *ExtendedReplyPacket) Type() PacketType {
	return PacketTypeExtendedReply
}

// MarshalPacket returns p as a two-part binary encoding of p.
//
// The Data is marshaled into binary, and returned as the payload.
func (p *ExtendedReplyPacket) MarshalPacket(reqid uint32, b []byte) (header, payload []byte, err error) {
	buf := NewBuffer(b)
	if buf.Cap() < 9 {
		buf = NewMarshalBuffer(0)
	}

	buf.StartPacket(PacketTypeExtendedReply, reqid)

	if p.Data != nil {
		payload, err = p.Data.MarshalBinary()
		if err != nil {
			return nil, nil, err
		}
	}

	return buf.Packet(payload)
}

// UnmarshalPacketBody unmarshals the packet body from the given Buffer.
// It is assumed that the uint32(request-id) has already been consumed.
//
// If p.Data is nil, and there is request-specific-data,
// then the request-specific-data will be wrapped in a Buffer and assigned to p.Data.
func (p *ExtendedReplyPacket) UnmarshalPacketBody(buf *Buffer) (err error) {
	if p.Data == nil {
		p.Data = new(Buffer)
	}

	return p.Data.UnmarshalBinary(buf.Bytes())
}
