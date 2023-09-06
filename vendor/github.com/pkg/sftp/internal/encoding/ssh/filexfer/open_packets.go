package filexfer

// SSH_FXF_* flags.
const (
	FlagRead      = 1 << iota // SSH_FXF_READ
	FlagWrite                 // SSH_FXF_WRITE
	FlagAppend                // SSH_FXF_APPEND
	FlagCreate                // SSH_FXF_CREAT
	FlagTruncate              // SSH_FXF_TRUNC
	FlagExclusive             // SSH_FXF_EXCL
)

// OpenPacket defines the SSH_FXP_OPEN packet.
type OpenPacket struct {
	Filename string
	PFlags   uint32
	Attrs    Attributes
}

// Type returns the SSH_FXP_xy value associated with this packet type.
func (p *OpenPacket) Type() PacketType {
	return PacketTypeOpen
}

// MarshalPacket returns p as a two-part binary encoding of p.
func (p *OpenPacket) MarshalPacket(reqid uint32, b []byte) (header, payload []byte, err error) {
	buf := NewBuffer(b)
	if buf.Cap() < 9 {
		// string(filename) + uint32(pflags) + ATTRS(attrs)
		size := 4 + len(p.Filename) + 4 + p.Attrs.Len()
		buf = NewMarshalBuffer(size)
	}

	buf.StartPacket(PacketTypeOpen, reqid)
	buf.AppendString(p.Filename)
	buf.AppendUint32(p.PFlags)

	p.Attrs.MarshalInto(buf)

	return buf.Packet(payload)
}

// UnmarshalPacketBody unmarshals the packet body from the given Buffer.
// It is assumed that the uint32(request-id) has already been consumed.
func (p *OpenPacket) UnmarshalPacketBody(buf *Buffer) (err error) {
	if p.Filename, err = buf.ConsumeString(); err != nil {
		return err
	}

	if p.PFlags, err = buf.ConsumeUint32(); err != nil {
		return err
	}

	return p.Attrs.UnmarshalFrom(buf)
}

// OpenDirPacket defines the SSH_FXP_OPENDIR packet.
type OpenDirPacket struct {
	Path string
}

// Type returns the SSH_FXP_xy value associated with this packet type.
func (p *OpenDirPacket) Type() PacketType {
	return PacketTypeOpenDir
}

// MarshalPacket returns p as a two-part binary encoding of p.
func (p *OpenDirPacket) MarshalPacket(reqid uint32, b []byte) (header, payload []byte, err error) {
	buf := NewBuffer(b)
	if buf.Cap() < 9 {
		size := 4 + len(p.Path) // string(path)
		buf = NewMarshalBuffer(size)
	}

	buf.StartPacket(PacketTypeOpenDir, reqid)
	buf.AppendString(p.Path)

	return buf.Packet(payload)
}

// UnmarshalPacketBody unmarshals the packet body from the given Buffer.
// It is assumed that the uint32(request-id) has already been consumed.
func (p *OpenDirPacket) UnmarshalPacketBody(buf *Buffer) (err error) {
	if p.Path, err = buf.ConsumeString(); err != nil {
		return err
	}

	return nil
}
