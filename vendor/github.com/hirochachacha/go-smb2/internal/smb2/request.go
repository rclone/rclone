package smb2

import "github.com/hirochachacha/go-smb2/internal/utf16le"

// ----------------------------------------------------------------------------
// SMB2 NEGOTIATE Request Packet
//

type NegotiateRequest struct {
	PacketHeader

	SecurityMode uint16
	Capabilities uint32
	ClientGuid   [16]byte
	Dialects     []uint16

	Contexts []Encoder
}

func (c *NegotiateRequest) Header() *PacketHeader {
	return &c.PacketHeader
}

func (c *NegotiateRequest) Size() int {
	size := 36 + len(c.Dialects)*2

	for _, cc := range c.Contexts {
		size = Roundup(size, 8)

		size += cc.Size()
	}

	return 64 + size
}

func (c *NegotiateRequest) Encode(pkt []byte) {
	c.Command = SMB2_NEGOTIATE
	c.encodeHeader(pkt)

	req := pkt[64:]
	le.PutUint16(req[:2], 36) // StructureSize
	le.PutUint16(req[4:6], c.SecurityMode)
	le.PutUint32(req[8:12], c.Capabilities)
	copy(req[12:28], c.ClientGuid[:])

	{
		bs := req[36:]
		for i, d := range c.Dialects {
			le.PutUint16(bs[2*i:2*i+2], d)
		}
		le.PutUint16(req[2:4], uint16(len(c.Dialects)))
	}

	off := 36 + len(c.Dialects)*2

	for i, cc := range c.Contexts {
		off = Roundup(off, 8)

		if i == 0 {
			le.PutUint32(req[28:32], uint32(off+64)) // NegotiateContextOffset
		}

		cc.Encode(req[off:])

		off += cc.Size()
	}

	le.PutUint16(req[32:34], uint16(len(c.Contexts))) // NegotiateContextCount
}

type NegotiateRequestDecoder []byte

func (r NegotiateRequestDecoder) IsInvalid() bool {
	if len(r) < 36 {
		return true
	}

	if r.StructureSize() != 36 {
		return true
	}

	noff := r.NegotiateContextOffset()

	if noff&7 != 0 {
		return true
	}

	if len(r) < int(noff)-36 {
		return true
	}

	return false
}

func (r NegotiateRequestDecoder) StructureSize() uint16 {
	return le.Uint16(r[:2])
}

func (r NegotiateRequestDecoder) DialectCount() uint16 {
	return le.Uint16(r[2:4])
}

func (r NegotiateRequestDecoder) SecurityMode() uint16 {
	return le.Uint16(r[4:6])
}

func (r NegotiateRequestDecoder) Capabilities() uint32 {
	return le.Uint32(r[8:12])
}

func (r NegotiateRequestDecoder) ClientGuid() []byte {
	return r[12:28]
}

func (r NegotiateRequestDecoder) ClientStartTime() []byte {
	return r[28:36]
}

func (r NegotiateRequestDecoder) Dialects() []uint16 {
	bs := r[36 : 36+2*r.DialectCount()]
	us := make([]uint16, len(bs)/2)
	for i := range us {
		us[i] = le.Uint16(bs[2*i : 2*i+2])
	}
	return us
}

// From SMB311

func (r NegotiateRequestDecoder) NegotiateContextOffset() uint32 {
	return le.Uint32(r[28:32])
}

func (r NegotiateRequestDecoder) NegotiateContextCount() uint16 {
	return le.Uint16(r[32:34])
}

func (r NegotiateRequestDecoder) NegotiateContextList() []byte {
	off := r.NegotiateContextOffset()
	if off < 36 {
		return nil
	}
	return r[off-36:]
}

// ----------------------------------------------------------------------------
// SMB2 SESSION_SETUP Request Packet
//

type SessionSetupRequest struct {
	PacketHeader

	Flags             uint8
	SecurityMode      uint8
	Capabilities      uint32
	Channel           uint32
	SecurityBuffer    []byte
	PreviousSessionId uint64
}

func (c *SessionSetupRequest) Header() *PacketHeader {
	return &c.PacketHeader
}

func (c *SessionSetupRequest) Size() int {
	if len(c.SecurityBuffer) == 0 {
		return 64 + 24 + 1
	}
	return 64 + 24 + len(c.SecurityBuffer)
}

func (c *SessionSetupRequest) Encode(pkt []byte) {
	c.Command = SMB2_SESSION_SETUP
	c.encodeHeader(pkt)

	req := pkt[64:]
	le.PutUint16(req[:2], 25)
	req[2] = c.Flags
	req[3] = c.SecurityMode
	le.PutUint32(req[4:8], c.Capabilities)
	le.PutUint32(req[8:12], c.Channel)
	le.PutUint64(req[16:24], c.PreviousSessionId)

	// SecurityBuffer
	{
		copy(req[24:], c.SecurityBuffer)
		le.PutUint16(req[12:14], 64+24)                         // SecurityBufferOffset
		le.PutUint16(req[14:16], uint16(len(c.SecurityBuffer))) // SecurityBufferLength
	}
}

type SessionSetupRequestDecoder []byte

func (r SessionSetupRequestDecoder) IsInvalid() bool {
	if len(r) < 24 {
		return true
	}

	if r.StructureSize() != 25 {
		return true
	}

	if len(r) < int(r.SecurityBufferOffset()+r.SecurityBufferLength())-64 {
		return true
	}

	return false
}

func (r SessionSetupRequestDecoder) StructureSize() uint16 {
	return le.Uint16(r[:2])
}

func (r SessionSetupRequestDecoder) Flags() uint8 {
	return r[2]
}

func (r SessionSetupRequestDecoder) SecurityMode() uint8 {
	return r[3]
}

func (r SessionSetupRequestDecoder) Capabilities() uint32 {
	return le.Uint32(r[4:8])
}

func (r SessionSetupRequestDecoder) Channel() uint32 {
	return le.Uint32(r[8:12])
}

func (r SessionSetupRequestDecoder) PreviousSessionId() uint64 {
	return le.Uint64(r[16:24])
}

func (r SessionSetupRequestDecoder) SecurityBufferOffset() uint16 {
	return le.Uint16(r[12:14])
}

func (r SessionSetupRequestDecoder) SecurityBufferLength() uint16 {
	return le.Uint16(r[14:16])
}

func (r SessionSetupRequestDecoder) SecurityBuffer() []byte {
	off := r.SecurityBufferOffset()
	if off < 64+24 {
		return nil
	}
	off -= 64
	len := r.SecurityBufferLength()
	return r[off : off+len]
}

// ----------------------------------------------------------------------------
// SMB2 LOGOFF Request Packet
//

type LogoffRequest struct {
	PacketHeader
}

func (c *LogoffRequest) Header() *PacketHeader {
	return &c.PacketHeader
}

func (c *LogoffRequest) Size() int {
	return 64 + 4
}

func (c *LogoffRequest) Encode(pkt []byte) {
	c.Command = SMB2_LOGOFF
	c.encodeHeader(pkt)

	req := pkt[64:]
	le.PutUint16(req[:2], 4) // StructureSize
}

type LogoffRequestDecoder []byte

func (r LogoffRequestDecoder) IsInvalid() bool {
	if len(r) < 4 {
		return true
	}

	if r.StructureSize() != 4 {
		return true
	}

	return false
}

func (r LogoffRequestDecoder) StructureSize() uint16 {
	return le.Uint16(r[:2])
}

// ----------------------------------------------------------------------------
// SMB2 TREE_CONNECT Request Packet
//

type TreeConnectRequest struct {
	PacketHeader

	Flags uint16
	Path  string
}

func (c *TreeConnectRequest) Header() *PacketHeader {
	return &c.PacketHeader
}

func (c *TreeConnectRequest) Size() int {
	if len(c.Path) == 0 {
		return 64 + 8 + 1
	}

	return 64 + 8 + utf16le.EncodedStringLen(c.Path)
}

func (c *TreeConnectRequest) Encode(pkt []byte) {
	c.Command = SMB2_TREE_CONNECT
	c.encodeHeader(pkt)

	req := pkt[64:]
	le.PutUint16(req[:2], 9) // StructureSize
	le.PutUint16(req[2:4], c.Flags)

	// Path
	{
		plen := utf16le.EncodeString(req[8:], c.Path)

		le.PutUint16(req[4:6], 8+64)         // PathOffset
		le.PutUint16(req[6:8], uint16(plen)) // PathLength
	}
}

type TreeConnectRequestDecoder []byte

func (r TreeConnectRequestDecoder) IsInvalid() bool {
	if len(r) < 8 {
		return true
	}

	if r.StructureSize() != 9 {
		return true
	}

	if len(r) < int(r.PathOffset()+r.PathLength())-64 {
		return true
	}

	return false
}

func (r TreeConnectRequestDecoder) StructureSize() uint16 {
	return le.Uint16(r[:2])
}

func (r TreeConnectRequestDecoder) Flags() uint16 {
	return le.Uint16(r[2:4])
}

func (r TreeConnectRequestDecoder) PathOffset() uint16 {
	return le.Uint16(r[4:6])
}

func (r TreeConnectRequestDecoder) PathLength() uint16 {
	return le.Uint16(r[6:8])
}

func (r TreeConnectRequestDecoder) Path() string {
	off := r.PathOffset()
	if off < 64+8 {
		return ""
	}
	off -= 64
	len := r.PathLength()
	return utf16le.DecodeToString(r[off : off+len])
}

// ----------------------------------------------------------------------------
// SMB2 TREE_DISCONNECT Request Packet
//

type TreeDisconnectRequest struct {
	PacketHeader
}

func (c *TreeDisconnectRequest) Header() *PacketHeader {
	return &c.PacketHeader
}

func (c *TreeDisconnectRequest) Size() int {
	return 64 + 4
}

func (c *TreeDisconnectRequest) Encode(pkt []byte) {
	c.Command = SMB2_TREE_DISCONNECT
	c.encodeHeader(pkt)

	req := pkt[64:]
	le.PutUint16(req[:2], 4) // StructureSize
}

type TreeDisconnectRequestDecoder []byte

func (r TreeDisconnectRequestDecoder) IsInvalid() bool {
	if len(r) < 4 {
		return true
	}

	if r.StructureSize() != 4 {
		return true
	}

	return false
}

func (r TreeDisconnectRequestDecoder) StructureSize() uint16 {
	return le.Uint16(r[:2])
}

// ----------------------------------------------------------------------------
// SMB2 CREATE Request Packet
//

type CreateRequest struct {
	PacketHeader

	SecurityFlags        uint8
	RequestedOplockLevel uint8
	ImpersonationLevel   uint32
	SmbCreateFlags       uint64
	DesiredAccess        uint32
	FileAttributes       uint32
	ShareAccess          uint32
	CreateDisposition    uint32
	CreateOptions        uint32
	Name                 string

	Contexts []Encoder
}

func (c *CreateRequest) Header() *PacketHeader {
	return &c.PacketHeader
}

func (c *CreateRequest) Size() int {
	if len(c.Name) == 0 && len(c.Contexts) == 0 {
		return 64 + 56 + 1
	}

	size := 64 + 56 + utf16le.EncodedStringLen(c.Name)

	for _, ctx := range c.Contexts {
		size = Roundup(size, 8)
		size += ctx.Size()
	}

	return size
}

func (c *CreateRequest) Encode(pkt []byte) {
	c.Command = SMB2_CREATE
	c.encodeHeader(pkt)

	req := pkt[64:]
	le.PutUint16(req[:2], 57) // StructureSize
	req[2] = c.SecurityFlags
	req[3] = c.RequestedOplockLevel
	le.PutUint32(req[4:8], c.ImpersonationLevel)
	le.PutUint64(req[8:16], c.SmbCreateFlags)
	le.PutUint32(req[24:28], c.DesiredAccess)
	le.PutUint32(req[28:32], c.FileAttributes)
	le.PutUint32(req[32:36], c.ShareAccess)
	le.PutUint32(req[36:40], c.CreateDisposition)
	le.PutUint32(req[40:44], c.CreateOptions)

	// Name
	nlen := utf16le.EncodeString(req[56:], c.Name)

	le.PutUint16(req[44:46], 56+64)
	le.PutUint16(req[46:48], uint16(nlen))

	off := 56 + nlen

	var ctx []byte
	var next int

	for i, c := range c.Contexts {
		off = Roundup(off, 8)

		if i == 0 {
			le.PutUint32(req[48:52], uint32(64+off)) // CreateContextsOffset
		} else {
			le.PutUint32(ctx[:4], uint32(next)) // Next
		}

		ctx = req[off:]

		c.Encode(ctx)

		next = c.Size()

		off += next
	}

	le.PutUint32(req[52:56], uint32(off-(56+nlen))) // CreateContextsLength
}

type CreateRequestDecoder []byte

func (r CreateRequestDecoder) IsInvalid() bool {
	if len(r) < 56 {
		return true
	}

	if r.StructureSize() != 57 {
		return true
	}

	noff := r.NameOffset()

	if noff&7 != 0 {
		return true
	}

	if len(r) < int(noff+r.NameLength())-64 {
		return true
	}

	coff := r.CreateContextsOffset()

	if coff&7 != 0 {
		return true
	}

	if len(r) < int(coff+r.CreateContextsLength())-64 {
		return true
	}

	return false
}

func (r CreateRequestDecoder) StructureSize() uint16 {
	return le.Uint16(r[:2])
}

func (r CreateRequestDecoder) SecurityFlags() uint8 {
	return r[2]
}

func (r CreateRequestDecoder) RequestedOplockLevel() uint8 {
	return r[3]
}

func (r CreateRequestDecoder) ImpersonationLevel() uint32 {
	return le.Uint32(r[4:8])
}

func (r CreateRequestDecoder) SmbCreateFlags() uint64 {
	return le.Uint64(r[8:16])
}

func (r CreateRequestDecoder) DesiredAccess() uint32 {
	return le.Uint32(r[24:28])
}

func (r CreateRequestDecoder) FileAttributes() uint32 {
	return le.Uint32(r[28:32])
}

func (r CreateRequestDecoder) ShareAccess() uint32 {
	return le.Uint32(r[32:36])
}

func (r CreateRequestDecoder) CreateDisposition() uint32 {
	return le.Uint32(r[36:40])
}

func (r CreateRequestDecoder) CreateOptions() uint32 {
	return le.Uint32(r[40:44])
}

func (r CreateRequestDecoder) NameOffset() uint16 {
	return le.Uint16(r[44:46])
}

func (r CreateRequestDecoder) NameLength() uint16 {
	return le.Uint16(r[46:48])
}

func (r CreateRequestDecoder) CreateContextsOffset() uint32 {
	return le.Uint32(r[48:52])
}

func (r CreateRequestDecoder) CreateContextsLength() uint32 {
	return le.Uint32(r[52:56])
}

// ----------------------------------------------------------------------------
// SMB2 CLOSE Request Packet
//

type CloseRequest struct {
	PacketHeader

	Flags  uint16
	FileId *FileId
}

func (c *CloseRequest) Header() *PacketHeader {
	return &c.PacketHeader
}

func (c *CloseRequest) Size() int {
	return 64 + 24
}

func (c *CloseRequest) Encode(pkt []byte) {
	c.Command = SMB2_CLOSE
	c.encodeHeader(pkt)

	req := pkt[64:]
	le.PutUint16(req[:2], 24) // StructureSize
	le.PutUint16(req[2:4], c.Flags)
	c.FileId.Encode(req[8:24])
}

type CloseRequestDecoder []byte

func (r CloseRequestDecoder) IsInvalid() bool {
	if len(r) < 24 {
		return true
	}

	if r.StructureSize() != 24 {
		return true
	}

	return false
}

func (r CloseRequestDecoder) StructureSize() uint16 {
	return le.Uint16(r[:2])
}

func (r CloseRequestDecoder) Flags() uint16 {
	return le.Uint16(r[2:4])
}

func (r CloseRequestDecoder) FileId() FileIdDecoder {
	return FileIdDecoder(r[8:24])
}

// ----------------------------------------------------------------------------
// SMB2 FLUSH Request Packet
//

type FlushRequest struct {
	PacketHeader

	FileId *FileId
}

func (c *FlushRequest) Header() *PacketHeader {
	return &c.PacketHeader
}

func (c *FlushRequest) Size() int {
	return 64 + 24
}

func (c *FlushRequest) Encode(pkt []byte) {
	c.Command = SMB2_FLUSH
	c.encodeHeader(pkt)

	req := pkt[64:]
	le.PutUint16(req[:2], 24) // StructureSize
	c.FileId.Encode(req[8:24])
}

type FlushRequestDecoder []byte

func (r FlushRequestDecoder) IsInvalid() bool {
	if len(r) < 24 {
		return true
	}

	if r.StructureSize() != 24 {
		return true
	}

	return false
}

func (r FlushRequestDecoder) StructureSize() uint16 {
	return le.Uint16(r[:2])
}

func (r FlushRequestDecoder) FileId() FileIdDecoder {
	return FileIdDecoder(r[8:24])
}

// ----------------------------------------------------------------------------
// SMB2 READ Request Packet
//

type ReadRequest struct {
	PacketHeader

	Padding         uint8
	Flags           uint8
	Length          uint32
	Offset          uint64
	FileId          *FileId
	MinimumCount    uint32
	Channel         uint32
	RemainingBytes  uint32
	ReadChannelInfo []Encoder
}

func (c *ReadRequest) Header() *PacketHeader {
	return &c.PacketHeader
}

func (c *ReadRequest) Size() int {
	if len(c.ReadChannelInfo) == 0 {
		return 64 + 48 + 1
	}

	size := 64 + 48
	for _, r := range c.ReadChannelInfo {
		size += r.Size()
	}
	return size
}

func (c *ReadRequest) Encode(pkt []byte) {
	c.Command = SMB2_READ
	c.encodeHeader(pkt)

	req := pkt[64:]
	le.PutUint16(req[:2], 49)
	req[2] = c.Padding
	req[3] = c.Flags
	le.PutUint32(req[4:8], c.Length)
	le.PutUint64(req[8:16], c.Offset)
	c.FileId.Encode(req[16:32])
	le.PutUint32(req[32:36], c.MinimumCount)
	le.PutUint32(req[36:40], c.Channel)
	le.PutUint32(req[40:44], c.RemainingBytes)

	off := 48

	for i, r := range c.ReadChannelInfo {
		if i == 0 {
			le.PutUint16(req[44:46], uint16(64+off)) // ReadChannelInfoOffset
		}

		r.Encode(req[off:])

		off += r.Size()
	}

	le.PutUint16(req[46:48], uint16(off-48)) // ReadChannelInfoLength
}

type ReadRequestDecoder []byte

func (r ReadRequestDecoder) IsInvalid() bool {
	if len(r) < 48 {
		return true
	}

	if r.StructureSize() != 49 {
		return true
	}

	if len(r) < int(r.ReadChannelInfoOffset()+r.ReadChannelInfoLength()) {
		return true
	}

	return false
}

func (r ReadRequestDecoder) StructureSize() uint16 {
	return le.Uint16(r[:2])
}

func (r ReadRequestDecoder) Padding() uint8 {
	return r[2]
}

func (r ReadRequestDecoder) Flags() uint8 {
	return r[3]
}

func (r ReadRequestDecoder) Length() uint32 {
	return le.Uint32(r[4:8])
}

func (r ReadRequestDecoder) Offset() uint64 {
	return le.Uint64(r[8:16])
}

func (r ReadRequestDecoder) FileId() FileIdDecoder {
	return FileIdDecoder(r[16:32])
}

func (r ReadRequestDecoder) MinimumCount() uint32 {
	return le.Uint32(r[32:36])
}

func (r ReadRequestDecoder) Channel() uint32 {
	return le.Uint32(r[36:40])
}

func (r ReadRequestDecoder) RemainingBytes() uint32 {
	return le.Uint32(r[40:44])
}

func (r ReadRequestDecoder) ReadChannelInfoOffset() uint16 {
	return le.Uint16(r[44:46])
}

func (r ReadRequestDecoder) ReadChannelInfoLength() uint16 {
	return le.Uint16(r[46:48])
}

// ----------------------------------------------------------------------------
// SMB2 WRITE Request Packet
//

type WriteRequest struct {
	PacketHeader

	FileId           *FileId
	Flags            uint32
	Channel          uint32
	RemainingBytes   uint32
	Offset           uint64
	WriteChannelInfo []Encoder
	Data             []byte
}

func (c *WriteRequest) Header() *PacketHeader {
	return &c.PacketHeader
}

func (c *WriteRequest) Size() int {
	if len(c.Data) == 0 && len(c.WriteChannelInfo) == 0 {
		return 64 + 48 + 1
	}

	off := 64 + 48

	for _, w := range c.WriteChannelInfo {
		off += w.Size()
	}

	off += len(c.Data)

	return off
}

func (c *WriteRequest) Encode(pkt []byte) {
	c.Command = SMB2_WRITE
	c.encodeHeader(pkt)

	req := pkt[64:]
	le.PutUint16(req[:2], 49) // StructureSize
	le.PutUint64(req[8:16], c.Offset)
	c.FileId.Encode(req[16:32])
	le.PutUint32(req[32:36], c.Channel)
	le.PutUint32(req[36:40], c.RemainingBytes)
	le.PutUint32(req[44:48], c.Flags)

	off := 48

	for i, w := range c.WriteChannelInfo {
		if i == 0 {
			le.PutUint16(req[40:42], uint16(64+off)) // WriteChannelInfoOffset
		}

		w.Encode(req[off:])

		off += w.Size()
	}

	le.PutUint16(req[42:44], uint16(off-48)) // WriteChannelInfoLength

	le.PutUint16(req[2:4], uint16(64+off)) // DataOffset

	copy(req[off:], c.Data)

	le.PutUint32(req[4:8], uint32(len(c.Data))) // Length
}

type WriteRequestDecoder []byte

func (r WriteRequestDecoder) IsInvalid() bool {
	if len(r) < 48 {
		return true
	}

	if r.StructureSize() != 49 {
		return true
	}

	if len(r) < int(r.WriteChannelInfoOffset()+r.WriteChannelInfoLength())-64 {
		return true
	}

	if len(r) < int(uint32(r.DataOffset())+r.Length())-64 {
		return true
	}

	return false
}

func (r WriteRequestDecoder) StructureSize() uint16 {
	return le.Uint16(r[:2])
}

func (r WriteRequestDecoder) DataOffset() uint16 {
	return le.Uint16(r[2:4])
}

func (r WriteRequestDecoder) Length() uint32 {
	return le.Uint32(r[4:8])
}

func (r WriteRequestDecoder) Offset() uint64 {
	return le.Uint64(r[8:16])
}

func (r WriteRequestDecoder) FileId() FileIdDecoder {
	return FileIdDecoder(r[16:32])
}

func (r WriteRequestDecoder) Channel() uint32 {
	return le.Uint32(r[32:36])
}

func (r WriteRequestDecoder) RemainingBytes() uint32 {
	return le.Uint32(r[36:40])
}

func (r WriteRequestDecoder) WriteChannelInfoOffset() uint16 {
	return le.Uint16(r[40:42])
}

func (r WriteRequestDecoder) WriteChannelInfoLength() uint16 {
	return le.Uint16(r[42:44])
}

func (r WriteRequestDecoder) Flags() uint32 {
	return le.Uint32(r[44:48])
}

// ----------------------------------------------------------------------------
// SMB2 OPLOCK_BREAK Acknowledgement
//

// ----------------------------------------------------------------------------
// SMB2 LOCK Request Packet
//

// ----------------------------------------------------------------------------
// SMB2 ECHO Request Packet
//

// ----------------------------------------------------------------------------
// SMB2 CANCEL Request Packet
//

type CancelRequest struct {
	PacketHeader
}

func (c *CancelRequest) Header() *PacketHeader {
	return &c.PacketHeader
}

func (c *CancelRequest) Size() int {
	return 64 + 4
}

func (c *CancelRequest) Encode(pkt []byte) {
	c.Command = SMB2_CANCEL
	c.encodeHeader(pkt)

	req := pkt[64:]
	le.PutUint16(req[:2], 4) // StructureSize
}

type CancelRequestDecoder []byte

func (r CancelRequestDecoder) IsInvalid() bool {
	if len(r) < 4 {
		return true
	}

	if r.StructureSize() != 4 {
		return true
	}

	return false
}

func (r CancelRequestDecoder) StructureSize() uint16 {
	return le.Uint16(r[:2])
}

// ----------------------------------------------------------------------------
// SMB2 IOCTL Request Packet
//

type IoctlRequest struct {
	PacketHeader

	CtlCode           uint32
	FileId            *FileId
	OutputOffset      uint32
	OutputCount       uint32
	MaxInputResponse  uint32
	MaxOutputResponse uint32
	Flags             uint32
	Input             Encoder
}

func (c *IoctlRequest) Header() *PacketHeader {
	return &c.PacketHeader
}

func (c *IoctlRequest) Size() int {
	if c.Input == nil {
		return 64 + 56 + 1
	}

	return 64 + 56 + c.Input.Size()
}

func (c *IoctlRequest) Encode(pkt []byte) {
	c.Command = SMB2_IOCTL
	c.encodeHeader(pkt)

	req := pkt[64:]
	le.PutUint16(req[:2], 57) // StructureSize
	le.PutUint32(req[4:8], c.CtlCode)
	c.FileId.Encode(req[8:24])
	le.PutUint32(req[32:36], c.MaxInputResponse)
	le.PutUint32(req[36:40], c.OutputOffset)
	le.PutUint32(req[40:44], c.OutputCount)
	le.PutUint32(req[44:48], c.MaxOutputResponse)
	le.PutUint32(req[48:52], c.Flags)

	off := 56

	if c.Input != nil {
		le.PutUint32(req[24:28], uint32(off+64)) // InputOffset

		c.Input.Encode(req[off:])

		le.PutUint32(req[28:32], uint32(c.Input.Size())) // InputCount
	}
}

type IoctlRequestDecoder []byte

func (r IoctlRequestDecoder) IsInvalid() bool {
	if len(r) < 56 {
		return true
	}

	if r.StructureSize() != 57 {
		return true
	}

	if len(r) < int(r.InputOffset()+r.InputCount())-64 {
		return true
	}

	return false
}

func (r IoctlRequestDecoder) StructureSize() uint16 {
	return le.Uint16(r[:2])
}

func (r IoctlRequestDecoder) CtlCode() uint32 {
	return le.Uint32(r[4:8])
}

func (r IoctlRequestDecoder) FileId() FileIdDecoder {
	return FileIdDecoder(r[8:24])
}

func (r IoctlRequestDecoder) InputOffset() uint32 {
	return le.Uint32(r[24:28])
}

func (r IoctlRequestDecoder) InputCount() uint32 {
	return le.Uint32(r[28:32])
}

func (r IoctlRequestDecoder) MaxInputResponse() uint32 {
	return le.Uint32(r[32:36])
}

func (r IoctlRequestDecoder) OutputOffset() uint32 {
	return le.Uint32(r[36:40])
}

func (r IoctlRequestDecoder) OutputCount() uint32 {
	return le.Uint32(r[40:44])
}

func (r IoctlRequestDecoder) MaxOutputResponse() uint32 {
	return le.Uint32(r[44:48])
}

func (r IoctlRequestDecoder) Flags() uint32 {
	return le.Uint32(r[48:52])
}

// ----------------------------------------------------------------------------
// SMB2 QUERY_DIRECTORY Request Packet
//

type QueryDirectoryRequest struct {
	PacketHeader

	FileInfoClass      uint8
	Flags              uint8
	FileIndex          uint32
	FileId             *FileId
	OutputBufferLength uint32
	FileName           string
}

func (c *QueryDirectoryRequest) Header() *PacketHeader {
	return &c.PacketHeader
}

func (c *QueryDirectoryRequest) Size() int {
	if len(c.FileName) == 0 {
		return 64 + 32 + 1
	}

	return 64 + 32 + utf16le.EncodedStringLen(c.FileName)
}

func (c *QueryDirectoryRequest) Encode(pkt []byte) {
	c.Command = SMB2_QUERY_DIRECTORY
	c.encodeHeader(pkt)

	req := pkt[64:]
	le.PutUint16(req[:2], 33) // StructureSize
	req[2] = c.FileInfoClass
	req[3] = c.Flags
	le.PutUint32(req[4:8], c.FileIndex)
	c.FileId.Encode(req[8:24])
	le.PutUint32(req[28:32], c.OutputBufferLength)

	off := 32

	le.PutUint16(req[24:26], uint16(off+64)) // FileNameOffset

	flen := utf16le.EncodeString(req[off:], c.FileName)

	le.PutUint16(req[26:28], uint16(flen)) // FileNameLength
}

type QueryDirectoryRequestDecoder []byte

func (r QueryDirectoryRequestDecoder) IsInvalid() bool {
	if len(r) < 32 {
		return true
	}

	if r.StructureSize() != 33 {
		return true
	}

	if len(r) < int(r.FileNameOffset()+r.FileNameLength())-64 {
		return true
	}

	return false
}

func (r QueryDirectoryRequestDecoder) StructureSize() uint16 {
	return le.Uint16(r[:2])
}

func (r QueryDirectoryRequestDecoder) FileInfoClass() uint8 {
	return r[2]
}

func (r QueryDirectoryRequestDecoder) Flags() uint8 {
	return r[3]
}

func (r QueryDirectoryRequestDecoder) FileIndex() uint32 {
	return le.Uint32(r[4:8])
}

func (r QueryDirectoryRequestDecoder) FileId() FileIdDecoder {
	return FileIdDecoder(r[8:24])
}

func (r QueryDirectoryRequestDecoder) FileNameOffset() uint16 {
	return le.Uint16(r[24:26])
}

func (r QueryDirectoryRequestDecoder) FileNameLength() uint16 {
	return le.Uint16(r[26:28])
}

func (r QueryDirectoryRequestDecoder) OutputBufferLength() uint32 {
	return le.Uint32(r[28:32])
}

// ----------------------------------------------------------------------------
// SMB2 CHANGE_NOTIFY Request Packet
//

// ----------------------------------------------------------------------------
// SMB2 QUERY_INFO Request Packet
//

type QueryInfoRequest struct {
	PacketHeader

	InfoType              uint8
	FileInfoClass         uint8
	OutputBufferLength    uint32
	AdditionalInformation uint32
	Flags                 uint32
	FileId                *FileId
	Input                 Encoder
}

func (c *QueryInfoRequest) Header() *PacketHeader {
	return &c.PacketHeader
}

func (c *QueryInfoRequest) Size() int {
	if c.Input == nil {
		return 64 + 40 + 1
	}

	return 64 + 40 + c.Input.Size()
}

func (c *QueryInfoRequest) Encode(pkt []byte) {
	c.Command = SMB2_QUERY_INFO
	c.encodeHeader(pkt)

	req := pkt[64:]
	le.PutUint16(req[:2], 41) // StructureSize
	req[2] = c.InfoType
	req[3] = c.FileInfoClass
	le.PutUint32(req[4:8], c.OutputBufferLength)
	le.PutUint32(req[16:20], c.AdditionalInformation)
	le.PutUint32(req[20:24], c.Flags)
	c.FileId.Encode(req[24:40])

	off := 40

	if c.Input != nil {
		le.PutUint16(req[8:10], uint16(off+64)) // InputBufferOffset

		c.Input.Encode(req[off:])

		le.PutUint32(req[12:16], uint32(c.Input.Size())) // InputBufferLength
	}
}

type QueryInfoRequestDecoder []byte

func (r QueryInfoRequestDecoder) IsInvalid() bool {
	if len(r) < 40 {
		return true
	}

	if r.StructureSize() != 41 {
		return true
	}

	if len(r) < int(uint32(r.InputBufferOffset())+r.InputBufferLength())-64 {
		return true
	}

	return false
}

func (r QueryInfoRequestDecoder) StructureSize() uint16 {
	return le.Uint16(r[:2])
}

func (r QueryInfoRequestDecoder) InfoType() uint8 {
	return r[2]
}

func (r QueryInfoRequestDecoder) FileInfoClass() uint8 {
	return r[3]
}

func (r QueryInfoRequestDecoder) OutputBufferLength() uint32 {
	return le.Uint32(r[4:8])
}

func (r QueryInfoRequestDecoder) InputBufferOffset() uint16 {
	return le.Uint16(r[8:10])
}

func (r QueryInfoRequestDecoder) InputBufferLength() uint32 {
	return le.Uint32(r[12:16])
}

func (r QueryInfoRequestDecoder) AdditionalInformation() uint32 {
	return le.Uint32(r[16:20])
}

func (r QueryInfoRequestDecoder) Flags() uint32 {
	return le.Uint32(r[20:24])
}

func (r QueryInfoRequestDecoder) FileId() FileIdDecoder {
	return FileIdDecoder(r[24:40])
}

// ----------------------------------------------------------------------------
// SMB2 SET_INFO Request Packet
//

type SetInfoRequest struct {
	PacketHeader

	InfoType              uint8
	FileInfoClass         uint8
	AdditionalInformation uint32
	FileId                *FileId
	Input                 Encoder
}

func (c *SetInfoRequest) Header() *PacketHeader {
	return &c.PacketHeader
}

func (c *SetInfoRequest) Size() int {
	if c.Input == nil {
		return 64 + 32 + 1
	}

	return 64 + 32 + c.Input.Size()
}

func (c *SetInfoRequest) Encode(pkt []byte) {
	c.Command = SMB2_SET_INFO
	c.encodeHeader(pkt)

	req := pkt[64:]
	le.PutUint16(req[:2], 33) // StructureSize
	req[2] = c.InfoType
	req[3] = c.FileInfoClass
	le.PutUint32(req[12:16], c.AdditionalInformation)
	c.FileId.Encode(req[16:32])

	off := 32

	if c.Input != nil {
		le.PutUint16(req[8:10], uint16(off+64)) // BufferOffset

		c.Input.Encode(req[off:])

		le.PutUint32(req[4:8], uint32(c.Input.Size())) // BufferLength
	}
}

type SetInfoRequestDecoder []byte

func (r SetInfoRequestDecoder) IsInvalid() bool {
	if len(r) < 32 {
		return true
	}

	if r.StructureSize() != 33 {
		return true
	}

	if len(r) < int(uint32(r.BufferOffset())+r.BufferLength())-64 {
		return true
	}

	return false
}

func (r SetInfoRequestDecoder) StructureSize() uint16 {
	return le.Uint16(r[:2])
}

func (r SetInfoRequestDecoder) InfoType() uint8 {
	return r[2]
}

func (r SetInfoRequestDecoder) FileInfoClass() uint8 {
	return r[3]
}

func (r SetInfoRequestDecoder) BufferLength() uint32 {
	return le.Uint32(r[4:8])
}

func (r SetInfoRequestDecoder) BufferOffset() uint16 {
	return le.Uint16(r[8:10])
}

func (r SetInfoRequestDecoder) AdditionalInformation() uint32 {
	return le.Uint32(r[12:16])
}

func (r SetInfoRequestDecoder) FileId() FileIdDecoder {
	return FileIdDecoder(r[16:32])
}
