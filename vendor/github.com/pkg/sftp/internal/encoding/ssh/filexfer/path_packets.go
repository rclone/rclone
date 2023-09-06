package filexfer

// LStatPacket defines the SSH_FXP_LSTAT packet.
type LStatPacket struct {
	Path string
}

// Type returns the SSH_FXP_xy value associated with this packet type.
func (p *LStatPacket) Type() PacketType {
	return PacketTypeLStat
}

// MarshalPacket returns p as a two-part binary encoding of p.
func (p *LStatPacket) MarshalPacket(reqid uint32, b []byte) (header, payload []byte, err error) {
	buf := NewBuffer(b)
	if buf.Cap() < 9 {
		size := 4 + len(p.Path) // string(path)
		buf = NewMarshalBuffer(size)
	}

	buf.StartPacket(PacketTypeLStat, reqid)
	buf.AppendString(p.Path)

	return buf.Packet(payload)
}

// UnmarshalPacketBody unmarshals the packet body from the given Buffer.
// It is assumed that the uint32(request-id) has already been consumed.
func (p *LStatPacket) UnmarshalPacketBody(buf *Buffer) (err error) {
	if p.Path, err = buf.ConsumeString(); err != nil {
		return err
	}

	return nil
}

// SetstatPacket defines the SSH_FXP_SETSTAT packet.
type SetstatPacket struct {
	Path  string
	Attrs Attributes
}

// Type returns the SSH_FXP_xy value associated with this packet type.
func (p *SetstatPacket) Type() PacketType {
	return PacketTypeSetstat
}

// MarshalPacket returns p as a two-part binary encoding of p.
func (p *SetstatPacket) MarshalPacket(reqid uint32, b []byte) (header, payload []byte, err error) {
	buf := NewBuffer(b)
	if buf.Cap() < 9 {
		size := 4 + len(p.Path) + p.Attrs.Len() // string(path) + ATTRS(attrs)
		buf = NewMarshalBuffer(size)
	}

	buf.StartPacket(PacketTypeSetstat, reqid)
	buf.AppendString(p.Path)

	p.Attrs.MarshalInto(buf)

	return buf.Packet(payload)
}

// UnmarshalPacketBody unmarshals the packet body from the given Buffer.
// It is assumed that the uint32(request-id) has already been consumed.
func (p *SetstatPacket) UnmarshalPacketBody(buf *Buffer) (err error) {
	if p.Path, err = buf.ConsumeString(); err != nil {
		return err
	}

	return p.Attrs.UnmarshalFrom(buf)
}

// RemovePacket defines the SSH_FXP_REMOVE packet.
type RemovePacket struct {
	Path string
}

// Type returns the SSH_FXP_xy value associated with this packet type.
func (p *RemovePacket) Type() PacketType {
	return PacketTypeRemove
}

// MarshalPacket returns p as a two-part binary encoding of p.
func (p *RemovePacket) MarshalPacket(reqid uint32, b []byte) (header, payload []byte, err error) {
	buf := NewBuffer(b)
	if buf.Cap() < 9 {
		size := 4 + len(p.Path) // string(path)
		buf = NewMarshalBuffer(size)
	}

	buf.StartPacket(PacketTypeRemove, reqid)
	buf.AppendString(p.Path)

	return buf.Packet(payload)
}

// UnmarshalPacketBody unmarshals the packet body from the given Buffer.
// It is assumed that the uint32(request-id) has already been consumed.
func (p *RemovePacket) UnmarshalPacketBody(buf *Buffer) (err error) {
	if p.Path, err = buf.ConsumeString(); err != nil {
		return err
	}

	return nil
}

// MkdirPacket defines the SSH_FXP_MKDIR packet.
type MkdirPacket struct {
	Path  string
	Attrs Attributes
}

// Type returns the SSH_FXP_xy value associated with this packet type.
func (p *MkdirPacket) Type() PacketType {
	return PacketTypeMkdir
}

// MarshalPacket returns p as a two-part binary encoding of p.
func (p *MkdirPacket) MarshalPacket(reqid uint32, b []byte) (header, payload []byte, err error) {
	buf := NewBuffer(b)
	if buf.Cap() < 9 {
		size := 4 + len(p.Path) + p.Attrs.Len() // string(path) + ATTRS(attrs)
		buf = NewMarshalBuffer(size)
	}

	buf.StartPacket(PacketTypeMkdir, reqid)
	buf.AppendString(p.Path)

	p.Attrs.MarshalInto(buf)

	return buf.Packet(payload)
}

// UnmarshalPacketBody unmarshals the packet body from the given Buffer.
// It is assumed that the uint32(request-id) has already been consumed.
func (p *MkdirPacket) UnmarshalPacketBody(buf *Buffer) (err error) {
	if p.Path, err = buf.ConsumeString(); err != nil {
		return err
	}

	return p.Attrs.UnmarshalFrom(buf)
}

// RmdirPacket defines the SSH_FXP_RMDIR packet.
type RmdirPacket struct {
	Path string
}

// Type returns the SSH_FXP_xy value associated with this packet type.
func (p *RmdirPacket) Type() PacketType {
	return PacketTypeRmdir
}

// MarshalPacket returns p as a two-part binary encoding of p.
func (p *RmdirPacket) MarshalPacket(reqid uint32, b []byte) (header, payload []byte, err error) {
	buf := NewBuffer(b)
	if buf.Cap() < 9 {
		size := 4 + len(p.Path) // string(path)
		buf = NewMarshalBuffer(size)
	}

	buf.StartPacket(PacketTypeRmdir, reqid)
	buf.AppendString(p.Path)

	return buf.Packet(payload)
}

// UnmarshalPacketBody unmarshals the packet body from the given Buffer.
// It is assumed that the uint32(request-id) has already been consumed.
func (p *RmdirPacket) UnmarshalPacketBody(buf *Buffer) (err error) {
	if p.Path, err = buf.ConsumeString(); err != nil {
		return err
	}

	return nil
}

// RealPathPacket defines the SSH_FXP_REALPATH packet.
type RealPathPacket struct {
	Path string
}

// Type returns the SSH_FXP_xy value associated with this packet type.
func (p *RealPathPacket) Type() PacketType {
	return PacketTypeRealPath
}

// MarshalPacket returns p as a two-part binary encoding of p.
func (p *RealPathPacket) MarshalPacket(reqid uint32, b []byte) (header, payload []byte, err error) {
	buf := NewBuffer(b)
	if buf.Cap() < 9 {
		size := 4 + len(p.Path) // string(path)
		buf = NewMarshalBuffer(size)
	}

	buf.StartPacket(PacketTypeRealPath, reqid)
	buf.AppendString(p.Path)

	return buf.Packet(payload)
}

// UnmarshalPacketBody unmarshals the packet body from the given Buffer.
// It is assumed that the uint32(request-id) has already been consumed.
func (p *RealPathPacket) UnmarshalPacketBody(buf *Buffer) (err error) {
	if p.Path, err = buf.ConsumeString(); err != nil {
		return err
	}

	return nil
}

// StatPacket defines the SSH_FXP_STAT packet.
type StatPacket struct {
	Path string
}

// Type returns the SSH_FXP_xy value associated with this packet type.
func (p *StatPacket) Type() PacketType {
	return PacketTypeStat
}

// MarshalPacket returns p as a two-part binary encoding of p.
func (p *StatPacket) MarshalPacket(reqid uint32, b []byte) (header, payload []byte, err error) {
	buf := NewBuffer(b)
	if buf.Cap() < 9 {
		size := 4 + len(p.Path) // string(path)
		buf = NewMarshalBuffer(size)
	}

	buf.StartPacket(PacketTypeStat, reqid)
	buf.AppendString(p.Path)

	return buf.Packet(payload)
}

// UnmarshalPacketBody unmarshals the packet body from the given Buffer.
// It is assumed that the uint32(request-id) has already been consumed.
func (p *StatPacket) UnmarshalPacketBody(buf *Buffer) (err error) {
	if p.Path, err = buf.ConsumeString(); err != nil {
		return err
	}

	return nil
}

// RenamePacket defines the SSH_FXP_RENAME packet.
type RenamePacket struct {
	OldPath string
	NewPath string
}

// Type returns the SSH_FXP_xy value associated with this packet type.
func (p *RenamePacket) Type() PacketType {
	return PacketTypeRename
}

// MarshalPacket returns p as a two-part binary encoding of p.
func (p *RenamePacket) MarshalPacket(reqid uint32, b []byte) (header, payload []byte, err error) {
	buf := NewBuffer(b)
	if buf.Cap() < 9 {
		// string(oldpath) + string(newpath)
		size := 4 + len(p.OldPath) + 4 + len(p.NewPath)
		buf = NewMarshalBuffer(size)
	}

	buf.StartPacket(PacketTypeRename, reqid)
	buf.AppendString(p.OldPath)
	buf.AppendString(p.NewPath)

	return buf.Packet(payload)
}

// UnmarshalPacketBody unmarshals the packet body from the given Buffer.
// It is assumed that the uint32(request-id) has already been consumed.
func (p *RenamePacket) UnmarshalPacketBody(buf *Buffer) (err error) {
	if p.OldPath, err = buf.ConsumeString(); err != nil {
		return err
	}

	if p.NewPath, err = buf.ConsumeString(); err != nil {
		return err
	}

	return nil
}

// ReadLinkPacket defines the SSH_FXP_READLINK packet.
type ReadLinkPacket struct {
	Path string
}

// Type returns the SSH_FXP_xy value associated with this packet type.
func (p *ReadLinkPacket) Type() PacketType {
	return PacketTypeReadLink
}

// MarshalPacket returns p as a two-part binary encoding of p.
func (p *ReadLinkPacket) MarshalPacket(reqid uint32, b []byte) (header, payload []byte, err error) {
	buf := NewBuffer(b)
	if buf.Cap() < 9 {
		size := 4 + len(p.Path) // string(path)
		buf = NewMarshalBuffer(size)
	}

	buf.StartPacket(PacketTypeReadLink, reqid)
	buf.AppendString(p.Path)

	return buf.Packet(payload)
}

// UnmarshalPacketBody unmarshals the packet body from the given Buffer.
// It is assumed that the uint32(request-id) has already been consumed.
func (p *ReadLinkPacket) UnmarshalPacketBody(buf *Buffer) (err error) {
	if p.Path, err = buf.ConsumeString(); err != nil {
		return err
	}

	return nil
}

// SymlinkPacket defines the SSH_FXP_SYMLINK packet.
//
// The order of the arguments to the SSH_FXP_SYMLINK method was inadvertently reversed.
// Unfortunately, the reversal was not noticed until the server was widely deployed.
// Covered in Section 4.1 of https://github.com/openssh/openssh-portable/blob/master/PROTOCOL
type SymlinkPacket struct {
	LinkPath   string
	TargetPath string
}

// Type returns the SSH_FXP_xy value associated with this packet type.
func (p *SymlinkPacket) Type() PacketType {
	return PacketTypeSymlink
}

// MarshalPacket returns p as a two-part binary encoding of p.
func (p *SymlinkPacket) MarshalPacket(reqid uint32, b []byte) (header, payload []byte, err error) {
	buf := NewBuffer(b)
	if buf.Cap() < 9 {
		// string(targetpath) + string(linkpath)
		size := 4 + len(p.TargetPath) + 4 + len(p.LinkPath)
		buf = NewMarshalBuffer(size)
	}

	buf.StartPacket(PacketTypeSymlink, reqid)

	// Arguments were inadvertently reversed.
	buf.AppendString(p.TargetPath)
	buf.AppendString(p.LinkPath)

	return buf.Packet(payload)
}

// UnmarshalPacketBody unmarshals the packet body from the given Buffer.
// It is assumed that the uint32(request-id) has already been consumed.
func (p *SymlinkPacket) UnmarshalPacketBody(buf *Buffer) (err error) {
	// Arguments were inadvertently reversed.
	if p.TargetPath, err = buf.ConsumeString(); err != nil {
		return err
	}

	if p.LinkPath, err = buf.ConsumeString(); err != nil {
		return err
	}

	return nil
}
