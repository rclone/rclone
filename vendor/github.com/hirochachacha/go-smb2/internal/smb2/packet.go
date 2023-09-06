package smb2

// ----------------------------------------------------------------------------
// SMB2 Packet Header
//

type PacketHeader struct {
	CreditCharge          uint16
	ChannelSequence       uint16
	Status                uint32
	Command               uint16
	CreditRequestResponse uint16
	Flags                 uint32
	MessageId             uint64
	AsyncId               uint64
	TreeId                uint32
	SessionId             uint64
}

func (hdr *PacketHeader) encodeHeader(pkt []byte) {
	p := PacketCodec(pkt)

	p.SetProtocolId()
	p.SetStructureSize()
	p.SetCreditCharge(hdr.CreditCharge)

	switch {
	case hdr.ChannelSequence != 0:
		p.SetChannelSequence(hdr.ChannelSequence)
	case hdr.Status != 0:
		p.SetStatus(hdr.Status)
	}

	p.SetCommand(hdr.Command)
	p.SetCreditRequest(hdr.CreditRequestResponse)
	p.SetFlags(hdr.Flags)
	p.SetMessageId(hdr.MessageId)

	switch {
	case hdr.TreeId != 0:
		p.SetTreeId(hdr.TreeId)
	case hdr.AsyncId != 0:
		p.SetAsyncId(hdr.AsyncId)
	}

	p.SetSessionId(hdr.SessionId)
}

// ----------------------------------------------------------------------------
// SMB2 Packet Interface
//

type Packet interface {
	Encoder

	Header() *PacketHeader
}

// ----------------------------------------------------------------------------
// SMB2 Packet Header
//

type PacketCodec []byte

func (p PacketCodec) IsInvalid() bool {
	if len(p) < 64 {
		return true
	}

	magic := p.ProtocolId()
	if magic[0] != 0xfe {
		return true
	}
	if magic[1] != 'S' {
		return true
	}
	if magic[2] != 'M' {
		return true
	}
	if magic[3] != 'B' {
		return true
	}

	if p.StructureSize() != 64 {
		return true
	}

	if p.NextCommand()&7 != 0 {
		return true
	}

	return false
}

func (p PacketCodec) ProtocolId() []byte {
	return p[:4]
}

func (p PacketCodec) SetProtocolId() {
	copy(p, MAGIC)
}

func (p PacketCodec) StructureSize() uint16 {
	return le.Uint16(p[4:6])
}

func (p PacketCodec) SetStructureSize() {
	le.PutUint16(p[4:6], 64)
}

func (p PacketCodec) CreditCharge() uint16 {
	return le.Uint16(p[6:8])
}

func (p PacketCodec) SetCreditCharge(u uint16) {
	le.PutUint16(p[6:8], u)
}

func (p PacketCodec) Status() uint32 {
	return le.Uint32(p[8:12])
}

func (p PacketCodec) SetStatus(u uint32) {
	le.PutUint32(p[8:12], u)
}

func (p PacketCodec) Command() uint16 {
	return le.Uint16(p[12:14])
}

func (p PacketCodec) SetCommand(u uint16) {
	le.PutUint16(p[12:14], u)
}

func (p PacketCodec) CreditRequest() uint16 {
	return le.Uint16(p[14:16])
}

func (p PacketCodec) SetCreditRequest(u uint16) {
	le.PutUint16(p[14:16], u)
}

func (p PacketCodec) CreditResponse() uint16 {
	return le.Uint16(p[14:16])
}

func (p PacketCodec) SetCreditResponse(u uint16) {
	le.PutUint16(p[14:16], u)
}

func (p PacketCodec) Flags() uint32 {
	return le.Uint32(p[16:20])
}

func (p PacketCodec) SetFlags(u uint32) {
	le.PutUint32(p[16:20], u)
}

func (p PacketCodec) NextCommand() uint32 {
	return le.Uint32(p[20:24])
}

func (p PacketCodec) SetNextCommand(u uint32) {
	le.PutUint32(p[20:24], u)
}

func (p PacketCodec) MessageId() uint64 {
	return le.Uint64(p[24:32])
}

func (p PacketCodec) SetMessageId(u uint64) {
	le.PutUint64(p[24:32], u)
}

func (p PacketCodec) AsyncId() uint64 {
	return le.Uint64(p[32:40])
}

func (p PacketCodec) SetAsyncId(u uint64) {
	le.PutUint64(p[32:40], u)
}

func (p PacketCodec) TreeId() uint32 {
	return le.Uint32(p[36:40])
}

func (p PacketCodec) SetTreeId(u uint32) {
	le.PutUint32(p[36:40], u)
}

func (p PacketCodec) SessionId() uint64 {
	return le.Uint64(p[40:48])
}

func (p PacketCodec) SetSessionId(u uint64) {
	le.PutUint64(p[40:48], u)
}

func (p PacketCodec) Signature() []byte {
	return p[48:64]
}

func (p PacketCodec) SetSignature(bs []byte) {
	copy(p[48:64], bs)
}

func (p PacketCodec) Data() []byte {
	return p[64:]
}

// From SMB3

func (p PacketCodec) ChannelSequence() uint16 {
	return le.Uint16(p[8:10])
}

func (p PacketCodec) SetChannelSequence(u uint16) {
	le.PutUint16(p[8:10], u)
}

// ----------------------------------------------------------------------------
// SMB2 TRANSFORM_HEADER
//

// From SMB3

type TransformCodec []byte

func (p TransformCodec) IsInvalid() bool {
	if len(p) < 52 {
		return true
	}

	magic := p.ProtocolId()
	if magic[0] != 0xfd {
		return true
	}
	if magic[1] != 'S' {
		return true
	}
	if magic[2] != 'M' {
		return true
	}
	if magic[3] != 'B' {
		return true
	}

	return false
}

func (p TransformCodec) ProtocolId() []byte {
	return p[:4]
}

func (p TransformCodec) SetProtocolId() {
	copy(p[:4], MAGIC2)
}

func (p TransformCodec) Signature() []byte {
	return p[4:20]
}

func (p TransformCodec) SetSignature(bs []byte) {
	copy(p[4:20], bs)
}

func (p TransformCodec) Nonce() []byte {
	return p[20:36]
}

func (p TransformCodec) SetNonce(bs []byte) {
	copy(p[20:36], bs)
}

func (p TransformCodec) OriginalMessageSize() uint32 {
	return le.Uint32(p[36:40])
}

func (p TransformCodec) SetOriginalMessageSize(u uint32) {
	le.PutUint32(p[36:40], u)
}

func (p TransformCodec) EncryptionAlgorithm() uint16 {
	return le.Uint16(p[42:44])
}

func (p TransformCodec) SetEncryptionAlgorithm(u uint16) {
	le.PutUint16(p[42:44], u)
}

func (p TransformCodec) SessionId() uint64 {
	return le.Uint64(p[44:52])
}

func (p TransformCodec) SetSessionId(u uint64) {
	le.PutUint64(p[44:52], u)
}

func (p TransformCodec) AssociatedData() []byte {
	return p[20:52]
}

func (p TransformCodec) EncryptedData() []byte {
	return p[52:]
}

// From SMB311

func (t TransformCodec) Flags() uint16 {
	return le.Uint16(t[42:44])
}

func (t TransformCodec) SetFlags(u uint16) {
	le.PutUint16(t[42:44], u)
}
