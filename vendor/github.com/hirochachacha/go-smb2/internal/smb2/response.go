package smb2

import "github.com/hirochachacha/go-smb2/internal/utf16le"

// ----------------------------------------------------------------------------
// SMB2 Error Response
//

type ErrorResponse struct {
	PacketHeader

	ErrorData Encoder // ErrorContextListResponse | (SymbolicLinkErrorResponse | SmallBufferErrorResponse)
}

func (c *ErrorResponse) Header() *PacketHeader {
	return &c.PacketHeader
}

func (c *ErrorResponse) Size() int {
	if c.ErrorData == nil {
		return 64 + 8 + 1
	}
	return 64 + 8 + c.ErrorData.Size()
}

// it doesn't handle Command property, set it yourself
func (c *ErrorResponse) Encode(pkt []byte) {
	c.encodeHeader(pkt)

	res := pkt[64:]
	le.PutUint16(res[:2], 9) // StructureSize
	if c.ErrorData != nil {
		le.PutUint16(res[2:4], uint16(c.ErrorData.Size()))
		c.ErrorData.Encode(res[8:])

		if e, ok := c.ErrorData.(ErrorContextListResponse); ok {
			res[2] = uint8(len(e))
		}
	}
}

type ErrorResponseDecoder []byte

func (r ErrorResponseDecoder) IsInvalid() bool {
	if len(r) < 8 {
		return true
	}

	if r.StructureSize() != 9 {
		return true
	}

	if uint32(len(r)) < 8+r.ByteCount() {
		return true
	}

	return false
}

func (r ErrorResponseDecoder) StructureSize() uint16 {
	return le.Uint16(r[:2])
}

func (r ErrorResponseDecoder) ErrorContextCount() uint8 {
	return r[2]
}

func (r ErrorResponseDecoder) ByteCount() uint32 {
	return le.Uint32(r[4:8])
}

func (r ErrorResponseDecoder) ErrorData() []byte {
	return r[8 : 8+r.ByteCount()]
}

// ----------------------------------------------------------------------------
// SMB2 Error Context Response
//

// for SMB311

type ErrorContextListResponse []*ErrorContextResponse

func (c ErrorContextListResponse) Size() int {
	size := 0
	for _, ec := range c {
		size = Roundup(size, 8)
		size += ec.Size()
	}
	return size
}

func (c ErrorContextListResponse) Encode(p []byte) {
	off := 0
	for _, ec := range c {
		off = Roundup(off, 8)

		ec.Encode(p[off:])

		off += ec.Size()
	}
}

type ErrorContextResponse struct {
	ErrorId uint32

	ErrorData Encoder
}

func (c *ErrorContextResponse) Size() int {
	return 8 + c.ErrorData.Size()
}

func (c *ErrorContextResponse) Encode(p []byte) {
	le.PutUint32(p[:4], uint32(c.ErrorData.Size()))
	le.PutUint32(p[4:8], c.ErrorId)
	if c.ErrorData != nil {
		c.ErrorData.Encode(p[8:])
	}
}

type ErrorContextResponseDecoder []byte

func (ctx ErrorContextResponseDecoder) IsInvalid() bool {
	if len(ctx) < 8 {
		return true
	}

	if uint32(len(ctx)) < 8+ctx.ErrorDataLength() {
		return true
	}

	return false
}

func (ctx ErrorContextResponseDecoder) ErrorDataLength() uint32 {
	return le.Uint32(ctx[:4])
}

func (ctx ErrorContextResponseDecoder) ErrorId() uint32 {
	return le.Uint32(ctx[4:8])
}

func (ctx ErrorContextResponseDecoder) ErrorContextData() []byte {
	return ctx[8 : 8+ctx.ErrorDataLength()]
}

func (ctx ErrorContextResponseDecoder) Next() int {
	return 8 + Roundup(int(ctx.ErrorDataLength()), 8)
}

// ----------------------------------------------------------------------------
// SMB2 ErrorData formats
//

type SmallBufferErrorResponse struct {
	RequiredBufferLength uint32
}

func (c *SmallBufferErrorResponse) Size() int {
	return 4
}

func (c *SmallBufferErrorResponse) Encode(p []byte) {
	le.PutUint32(p[:4], c.RequiredBufferLength)
}

type SmallBufferErrorResponseDecoder []byte

func (r SmallBufferErrorResponseDecoder) IsInvalid() bool {
	return len(r) != 4
}

func (r SmallBufferErrorResponseDecoder) RequiredBufferLength() uint32 {
	return le.Uint32(r)
}

type SymbolicLinkErrorResponse struct {
	UnparsedPathLength uint16
	Flags              uint32
	SubstituteName     string
	PrintName          string
}

func (c *SymbolicLinkErrorResponse) Size() int {
	return 28 + utf16le.EncodedStringLen(c.SubstituteName) + utf16le.EncodedStringLen(c.PrintName)
}

func (c *SymbolicLinkErrorResponse) Encode(p []byte) {
	slen := utf16le.EncodeString(p[24:], c.SubstituteName)
	plen := utf16le.EncodeString(p[24+slen:], c.PrintName)

	le.PutUint32(p[:4], uint32(len(p)-4)) // SymLinkLength
	le.PutUint32(p[4:8], 0x4c4d5953)
	le.PutUint32(p[8:12], IO_REPARSE_TAG_SYMLINK)
	le.PutUint16(p[14:16], c.UnparsedPathLength)
	le.PutUint32(p[24:28], c.Flags)
	le.PutUint16(p[12:14], uint16(len(p)-12)) // ReparseDataLength
	le.PutUint16(p[16:18], 0)                 // SubstituteNameOffset
	le.PutUint16(p[18:20], uint16(slen))      // SubstituteNameLength
	le.PutUint16(p[20:22], uint16(slen))      // PrintNameOffset
	le.PutUint16(p[22:24], uint16(plen))      // PrintNameLength
}

type SymbolicLinkErrorResponseDecoder []byte

func (r SymbolicLinkErrorResponseDecoder) IsInvalid() bool {
	if len(r) < 28 {
		return true
	}

	if r.SymLinkErrorTag() != 0x4c4d5953 {
		return true
	}

	if r.ReparseTag() != IO_REPARSE_TAG_SYMLINK {
		return true
	}

	tlen := int(r.SymLinkLength())
	rlen := int(r.ReparseDataLength())
	soff := int(r.SubstituteNameOffset())
	slen := int(r.SubstituteNameLength())
	poff := int(r.PrintNameOffset())
	plen := int(r.PrintNameLength())

	if (soff&1 | poff&1) != 0 {
		return true
	}

	if len(r) < 4+tlen {
		return true
	}

	if tlen < 12+rlen {
		return true
	}

	if rlen < 12+soff+slen || rlen < 12+poff+plen {
		return true
	}

	return false
}

func (r SymbolicLinkErrorResponseDecoder) SymLinkLength() uint32 {
	return le.Uint32(r[:4])
}

func (r SymbolicLinkErrorResponseDecoder) SymLinkErrorTag() uint32 {
	return le.Uint32(r[4:8])
}

func (r SymbolicLinkErrorResponseDecoder) ReparseTag() uint32 {
	return le.Uint32(r[8:12])
}

func (r SymbolicLinkErrorResponseDecoder) ReparseDataLength() uint16 {
	return le.Uint16(r[12:14])
}

func (r SymbolicLinkErrorResponseDecoder) UnparsedPathLength() uint16 {
	return le.Uint16(r[14:16])
}

func (r SymbolicLinkErrorResponseDecoder) SubstituteNameOffset() uint16 {
	return le.Uint16(r[16:18])
}

func (r SymbolicLinkErrorResponseDecoder) SubstituteNameLength() uint16 {
	return le.Uint16(r[18:20])
}

func (r SymbolicLinkErrorResponseDecoder) PrintNameOffset() uint16 {
	return le.Uint16(r[20:22])
}

func (r SymbolicLinkErrorResponseDecoder) PrintNameLength() uint16 {
	return le.Uint16(r[22:24])
}

func (r SymbolicLinkErrorResponseDecoder) Flags() uint32 {
	return le.Uint32(r[24:28])
}

func (r SymbolicLinkErrorResponseDecoder) PathBuffer() []byte {
	return r[28:]
}

func (r SymbolicLinkErrorResponseDecoder) SubstituteName() string {
	off := r.SubstituteNameOffset()
	len := r.SubstituteNameLength()
	return utf16le.DecodeToString(r.PathBuffer()[off : off+len])
}

func (r SymbolicLinkErrorResponseDecoder) PrintName() string {
	off := r.PrintNameOffset()
	len := r.PrintNameLength()
	return utf16le.DecodeToString(r.PathBuffer()[off : off+len])
}

func (r SymbolicLinkErrorResponseDecoder) SplitUnparsedPath(name string) (string, string) {
	ws := UTF16FromString(name)
	ulen := int(r.UnparsedPathLength())
	if ulen/2 > len(ws) {
		return "", ""
	}

	return UTF16ToString(ws[:len(ws)-ulen/2]), UTF16ToString(ws[len(ws)-ulen/2:])
}

// ----------------------------------------------------------------------------
// SMB2 NEGOTIATE Response
//

type NegotiateResponse struct {
	PacketHeader

	SecurityMode    uint16
	DialectRevision uint16
	ServerGuid      [16]byte
	Capabilities    uint32
	MaxTransactSize uint32
	MaxReadSize     uint32
	MaxWriteSize    uint32
	SystemTime      *Filetime
	ServerStartTime *Filetime
	SecurityBuffer  []byte

	Contexts []Encoder
}

func (c *NegotiateResponse) Header() *PacketHeader {
	return &c.PacketHeader
}

func (c *NegotiateResponse) Size() int {
	size := 64 + len(c.SecurityBuffer)

	for _, cc := range c.Contexts {
		size = Roundup(size, 8)

		size += cc.Size()
	}

	if size == 64 {
		return 64 + 64 + 1
	}

	return 64 + size
}

func (c *NegotiateResponse) Encode(pkt []byte) {
	c.Command = SMB2_NEGOTIATE
	c.encodeHeader(pkt)

	res := pkt[64:]
	le.PutUint16(res[:2], 65) // StructureSize
	le.PutUint16(res[2:4], c.SecurityMode)
	le.PutUint16(res[4:6], c.DialectRevision)
	copy(res[8:24], c.ServerGuid[:])
	le.PutUint32(res[24:28], c.Capabilities)
	le.PutUint32(res[28:32], c.MaxTransactSize)
	le.PutUint32(res[32:36], c.MaxReadSize)
	le.PutUint32(res[36:40], c.MaxWriteSize)
	c.SystemTime.Encode(res[40:48])
	c.ServerStartTime.Encode(res[48:56])

	// SecurityBuffer
	{
		copy(res[64:], c.SecurityBuffer)
		le.PutUint16(res[56:58], 64+64)                         // SecurityBufferOffset
		le.PutUint16(res[58:60], uint16(len(c.SecurityBuffer))) // SecurityBufferLength
	}

	off := 64 + len(c.SecurityBuffer)

	for i, cc := range c.Contexts {
		off = Roundup(off, 8)

		if i == 0 {
			le.PutUint32(res[60:64], uint32(off+64)) // NegotiateContextOffset
		}

		cc.Encode(res[off:])

		off += cc.Size()
	}

	le.PutUint16(res[6:8], uint16(len(c.Contexts))) // NegotiateContextCount
}

type NegotiateResponseDecoder []byte

func (r NegotiateResponseDecoder) IsInvalid() bool {
	if len(r) < 64 {
		return true
	}

	if r.StructureSize() != 65 {
		return true
	}

	if len(r) < int(r.SecurityBufferOffset()+r.SecurityBufferLength())-64 {
		return true
	}

	if r.DialectRevision() == SMB311 {
		noff := r.NegotiateContextOffset()

		if noff&7 != 0 {
			return true
		}

		if len(r) < int(noff)-64 {
			return true
		}
	}

	return false
}

func (r NegotiateResponseDecoder) StructureSize() uint16 {
	return le.Uint16(r[:2])
}

func (r NegotiateResponseDecoder) SecurityMode() uint16 {
	return le.Uint16(r[2:4])
}

func (r NegotiateResponseDecoder) DialectRevision() uint16 {
	return le.Uint16(r[4:6])
}

func (r NegotiateResponseDecoder) ServerGuid() []byte {
	return r[8:24]
}

func (r NegotiateResponseDecoder) Capabilities() uint32 {
	return le.Uint32(r[24:28])
}

func (r NegotiateResponseDecoder) MaxTransactSize() uint32 {
	return le.Uint32(r[28:32])
}

func (r NegotiateResponseDecoder) MaxReadSize() uint32 {
	return le.Uint32(r[32:36])
}

func (r NegotiateResponseDecoder) MaxWriteSize() uint32 {
	return le.Uint32(r[36:40])
}

func (r NegotiateResponseDecoder) SystemTime() FiletimeDecoder {
	return FiletimeDecoder(r[40:48])
}

func (r NegotiateResponseDecoder) ServerStartTime() FiletimeDecoder {
	return FiletimeDecoder(r[48:56])
}

func (r NegotiateResponseDecoder) SecurityBufferOffset() uint16 {
	return le.Uint16(r[56:58])
}

func (r NegotiateResponseDecoder) SecurityBufferLength() uint16 {
	return le.Uint16(r[58:60])
}

// func (r NegotiateResponseDecoder) Buffer() []byte {
// return r[64:]
// }

func (r NegotiateResponseDecoder) SecurityBuffer() []byte {
	off := r.SecurityBufferOffset()
	if off < 64+64 {
		return nil
	}
	off -= 64
	len := r.SecurityBufferLength()
	return r[off : off+len]
}

// From SMB311

func (r NegotiateResponseDecoder) NegotiateContextCount() uint16 {
	return le.Uint16(r[6:8])
}

func (r NegotiateResponseDecoder) NegotiateContextOffset() uint32 {
	return le.Uint32(r[60:64])
}

func (r NegotiateResponseDecoder) NegotiateContextList() []byte {
	off := r.NegotiateContextOffset()
	if off < 64 {
		return nil
	}
	return r[off-64:]
}

// ----------------------------------------------------------------------------
// SMB2 SESSION_SETUP Response
//

type SessionSetupResponse struct {
	PacketHeader

	SessionFlags   uint16
	SecurityBuffer []byte
}

func (c *SessionSetupResponse) Header() *PacketHeader {
	return &c.PacketHeader
}

func (c *SessionSetupResponse) Size() int {
	if len(c.SecurityBuffer) == 0 {
		return 64 + 8 + 1
	}

	return 64 + 8 + len(c.SecurityBuffer)
}

func (c *SessionSetupResponse) Encode(pkt []byte) {
	c.Command = SMB2_SESSION_SETUP
	c.encodeHeader(pkt)

	res := pkt[64:]
	le.PutUint16(res[:2], 9) // StructureSize
	le.PutUint16(res[2:4], c.SessionFlags)

	if len(c.SecurityBuffer) != 0 {
		le.PutUint16(res[4:6], 8+64) // SecurityBufferOffset

		copy(res[8:], c.SecurityBuffer)

		le.PutUint16(res[6:8], uint16(len(c.SecurityBuffer)))
	}
}

type SessionSetupResponseDecoder []byte

func (r SessionSetupResponseDecoder) IsInvalid() bool {
	if len(r) < 8 {
		return true
	}

	if r.StructureSize() != 9 {
		return true
	}

	if len(r) < int(r.SecurityBufferOffset()+r.SecurityBufferLength())-64 {
		return true
	}

	return false
}

func (r SessionSetupResponseDecoder) StructureSize() uint16 {
	return le.Uint16(r[:2])
}

func (r SessionSetupResponseDecoder) SessionFlags() uint16 {
	return le.Uint16(r[2:4])
}

func (r SessionSetupResponseDecoder) SecurityBufferOffset() uint16 {
	return le.Uint16(r[4:6])
}

func (r SessionSetupResponseDecoder) SecurityBufferLength() uint16 {
	return le.Uint16(r[6:8])
}

// func (req SessionSetupResponseDecoder) Buffer() []byte {
// return req[8:]
// }

func (r SessionSetupResponseDecoder) SecurityBuffer() []byte {
	off := r.SecurityBufferOffset()
	if off < 8+64 {
		return nil
	}
	off -= 64
	len := r.SecurityBufferLength()
	return r[off : off+len]
}

// ----------------------------------------------------------------------------
// SMB2 LOGOFF Response
//

type LogoffResponse struct {
	PacketHeader
}

func (c *LogoffResponse) Header() *PacketHeader {
	return &c.PacketHeader
}

func (c *LogoffResponse) Size() int {
	return 64 + 4
}

func (c *LogoffResponse) Encode(pkt []byte) {
	c.Command = SMB2_LOGOFF
	c.encodeHeader(pkt)

	res := pkt[64:]
	le.PutUint16(res[:2], 4) // StructureSize
}

type LogoffResponseDecoder []byte

func (r LogoffResponseDecoder) IsInvalid() bool {
	if len(r) < 4 {
		return true
	}

	if r.StructureSize() != 4 {
		return true
	}

	return false
}

func (r LogoffResponseDecoder) StructureSize() uint16 {
	return le.Uint16(r[:2])
}

// ----------------------------------------------------------------------------
// SMB2 TREE_CONNECT Response
//

type TreeConnectResponse struct {
	PacketHeader

	ShareType     uint8
	ShareFlags    uint32
	Capabilities  uint32
	MaximalAccess uint32
}

func (c *TreeConnectResponse) Header() *PacketHeader {
	return &c.PacketHeader
}

func (c *TreeConnectResponse) Size() int {
	return 64 + 16
}

func (c *TreeConnectResponse) Encode(pkt []byte) {
	c.Command = SMB2_TREE_CONNECT
	c.encodeHeader(pkt)

	res := pkt[64:]
	le.PutUint16(res[:2], 16) // StructureSize
	res[2] = c.ShareType
	le.PutUint32(res[4:8], c.ShareFlags)
	le.PutUint32(res[8:12], c.Capabilities)
	le.PutUint32(res[12:16], c.MaximalAccess)
}

type TreeConnectResponseDecoder []byte

func (r TreeConnectResponseDecoder) IsInvalid() bool {
	if len(r) < 16 {
		return true
	}

	if r.StructureSize() != 16 {
		return true
	}

	return false
}

func (r TreeConnectResponseDecoder) StructureSize() uint16 {
	return le.Uint16(r[:2])
}

func (r TreeConnectResponseDecoder) ShareType() uint8 {
	return r[2]
}

func (r TreeConnectResponseDecoder) ShareFlags() uint32 {
	return le.Uint32(r[4:8])
}

func (r TreeConnectResponseDecoder) Capabilities() uint32 {
	return le.Uint32(r[8:12])
}

func (r TreeConnectResponseDecoder) MaximalAccess() uint32 {
	return le.Uint32(r[12:16])
}

// ----------------------------------------------------------------------------
// SMB2 TREE_DISCONNECT Response
//

type TreeDisconnectResponse struct {
	PacketHeader
}

func (c *TreeDisconnectResponse) Header() *PacketHeader {
	return &c.PacketHeader
}

func (c *TreeDisconnectResponse) Size() int {
	return 4
}

func (c *TreeDisconnectResponse) Encode(pkt []byte) {
	c.Command = SMB2_TREE_DISCONNECT
	c.encodeHeader(pkt)

	res := pkt[64:]
	le.PutUint16(res[:2], 4) // StructureSize
}

type TreeDisconnectResponseDecoder []byte

func (r TreeDisconnectResponseDecoder) IsInvalid() bool {
	if len(r) < 4 {
		return true
	}

	if r.StructureSize() != 4 {
		return true
	}

	return false
}

func (r TreeDisconnectResponseDecoder) StructureSize() uint16 {
	return le.Uint16(r[:2])
}

// ----------------------------------------------------------------------------
// SMB2 CREATE Response
//

type CreateResponse struct {
	PacketHeader

	OplockLevel    uint8
	Flags          uint8
	CreateAction   uint32
	CreationTime   *Filetime
	LastAccessTime *Filetime
	LastWriteTime  *Filetime
	ChangeTime     *Filetime
	AllocationSize int64
	EndofFile      int64
	FileAttributes uint32
	FileId         *FileId

	Contexts []Encoder
}

func (c *CreateResponse) Header() *PacketHeader {
	return &c.PacketHeader
}

func (c *CreateResponse) Size() int {
	if len(c.Contexts) == 0 {
		return 64 + 88 + 1
	}

	size := 64 + 88

	for _, ctx := range c.Contexts {
		size = Roundup(size, 8)
		size += ctx.Size()
	}

	return size
}

func (c *CreateResponse) Encode(pkt []byte) {
	c.Command = SMB2_CREATE
	c.encodeHeader(pkt)

	res := pkt[64:]
	le.PutUint16(res[:2], 89) // StructureSize
	res[2] = c.OplockLevel
	res[3] = c.Flags
	le.PutUint32(res[4:8], c.CreateAction)
	c.CreationTime.Encode(res[8:16])
	c.LastAccessTime.Encode(res[16:24])
	c.LastWriteTime.Encode(res[24:32])
	c.ChangeTime.Encode(res[32:40])
	le.PutUint64(res[40:48], uint64(c.AllocationSize))
	le.PutUint64(res[48:56], uint64(c.EndofFile))
	le.PutUint32(res[56:60], c.FileAttributes)
	c.FileId.Encode(res[64:80])

	off := 88

	var ctx []byte
	var next int

	for i, c := range c.Contexts {
		off = Roundup(off, 8)

		if i == 0 {
			le.PutUint32(res[80:84], uint32(64+off)) // CreateContextsOffset
		} else {
			le.PutUint32(ctx[:4], uint32(next)) // Next
		}

		ctx = res[off:]

		c.Encode(ctx)

		next = c.Size()

		off += next
	}

	le.PutUint32(res[84:88], uint32(off-88)) // CreateContextsLength
}

type CreateResponseDecoder []byte

func (r CreateResponseDecoder) IsInvalid() bool {
	if len(r) < 88 {
		return true
	}

	if r.StructureSize() != 89 {
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

func (r CreateResponseDecoder) StructureSize() uint16 {
	return le.Uint16(r[:2])
}

func (r CreateResponseDecoder) OplockLevel() uint8 {
	return r[2]
}

func (r CreateResponseDecoder) Flags() uint8 {
	return r[3]
}

func (r CreateResponseDecoder) CreateAction() uint32 {
	return le.Uint32(r[4:8])
}

func (r CreateResponseDecoder) CreationTime() FiletimeDecoder {
	return FiletimeDecoder(r[8:16])
}

func (r CreateResponseDecoder) LastAccessTime() FiletimeDecoder {
	return FiletimeDecoder(r[16:24])
}

func (r CreateResponseDecoder) LastWriteTime() FiletimeDecoder {
	return FiletimeDecoder(r[24:32])
}

func (r CreateResponseDecoder) ChangeTime() FiletimeDecoder {
	return FiletimeDecoder(r[32:40])
}

func (r CreateResponseDecoder) AllocationSize() int64 {
	return int64(le.Uint64(r[40:48]))
}

func (r CreateResponseDecoder) EndofFile() int64 {
	return int64(le.Uint64(r[48:56]))
}

func (r CreateResponseDecoder) FileAttributes() uint32 {
	return le.Uint32(r[56:60])
}

func (r CreateResponseDecoder) FileId() FileIdDecoder {
	return FileIdDecoder(r[64:80])
}

func (r CreateResponseDecoder) CreateContextsOffset() uint32 {
	return le.Uint32(r[80:84])
}

func (r CreateResponseDecoder) CreateContextsLength() uint32 {
	return le.Uint32(r[84:88])
}

// func (r CreateResponseDecoder) Buffer() []byte {
// return r[88:]
// }

func (r CreateResponseDecoder) CreateContexts() []byte {
	off := r.CreateContextsOffset()
	if off < 88+64 {
		return nil
	}
	off -= 64
	len := r.CreateContextsLength()
	return r[off : off+len]
}

// ----------------------------------------------------------------------------
// SMB2 CLOSE Response
//

type CloseResponse struct {
	PacketHeader

	Flags          uint16
	CreationTime   *Filetime
	LastAccessTime *Filetime
	LastWriteTime  *Filetime
	ChangeTime     *Filetime
	AllocationSize int64
	EndofFile      int64
	FileAttributes uint32
}

func (c *CloseResponse) Header() *PacketHeader {
	return &c.PacketHeader
}

func (c *CloseResponse) Size() int {
	return 64 + 60
}

func (c *CloseResponse) Encode(pkt []byte) {
	c.Command = SMB2_CLOSE
	c.encodeHeader(pkt)

	res := pkt[64:]
	le.PutUint16(res[:2], 60) // StructureSize
	le.PutUint16(res[2:4], c.Flags)
	c.CreationTime.Encode(res[8:16])
	c.LastAccessTime.Encode(res[16:24])
	c.LastWriteTime.Encode(res[24:32])
	c.ChangeTime.Encode(res[32:40])
	le.PutUint64(res[40:48], uint64(c.AllocationSize))
	le.PutUint64(res[48:56], uint64(c.EndofFile))
	le.PutUint32(res[56:60], c.FileAttributes)
}

type CloseResponseDecoder []byte

func (r CloseResponseDecoder) IsInvalid() bool {
	if len(r) < 60 {
		return true
	}

	if r.StructureSize() != 60 {
		return true
	}

	return false
}

func (r CloseResponseDecoder) StructureSize() uint16 {
	return le.Uint16(r[:2])
}

func (r CloseResponseDecoder) Flags() uint16 {
	return le.Uint16(r[2:4])
}

func (r CloseResponseDecoder) CreationTime() FiletimeDecoder {
	return FiletimeDecoder(r[8:16])
}

func (r CloseResponseDecoder) LastAccessTime() FiletimeDecoder {
	return FiletimeDecoder(r[16:24])
}

func (r CloseResponseDecoder) LastWriteTime() FiletimeDecoder {
	return FiletimeDecoder(r[24:32])
}

func (r CloseResponseDecoder) ChangeTime() FiletimeDecoder {
	return FiletimeDecoder(r[32:40])
}

func (r CloseResponseDecoder) AllocationSize() int64 {
	return int64(le.Uint64(r[40:48]))
}

func (r CloseResponseDecoder) EndofFile() int64 {
	return int64(le.Uint64(r[48:56]))
}

func (r CloseResponseDecoder) FileAttributes() uint32 {
	return le.Uint32(r[56:60])
}

// ----------------------------------------------------------------------------
// SMB2 FLUSH Response
//

type FlushResponse struct {
	PacketHeader
}

func (c *FlushResponse) Header() *PacketHeader {
	return &c.PacketHeader
}

func (c *FlushResponse) Size() int {
	return 64 + 4
}

func (c *FlushResponse) Encode(pkt []byte) {
	c.Command = SMB2_FLUSH
	c.encodeHeader(pkt)

	res := pkt[64:]
	le.PutUint16(res[:2], 4) // StructureSize
}

type FlushResponseDecoder []byte

func (r FlushResponseDecoder) IsInvalid() bool {
	if len(r) < 4 {
		return true
	}

	if r.StructureSize() != 4 {
		return true
	}

	return false
}

func (r FlushResponseDecoder) StructureSize() uint16 {
	return le.Uint16(r[:2])
}

// ----------------------------------------------------------------------------
// SMB2 READ Response
//

type ReadResponse struct {
	PacketHeader

	Data          []byte
	DataRemaining uint32
}

func (c *ReadResponse) Header() *PacketHeader {
	return &c.PacketHeader
}

func (c *ReadResponse) Size() int {
	if len(c.Data) == 0 {
		return 64 + 16 + 1
	}
	return 64 + 16 + len(c.Data)
}

func (c *ReadResponse) Encode(pkt []byte) {
	c.Command = SMB2_READ
	c.encodeHeader(pkt)

	res := pkt[64:]
	le.PutUint16(res[:2], 17) // StructureSize
	res[2] = 16               // DataOffset
	copy(res[16:], c.Data)
	le.PutUint32(res[4:8], uint32(len(c.Data))) // DataLength
	le.PutUint32(res[8:12], c.DataRemaining)
}

type ReadResponseDecoder []byte

func (r ReadResponseDecoder) IsInvalid() bool {
	if len(r) < 16 {
		return true
	}

	if r.StructureSize() != 17 {
		return true
	}

	if len(r) < int(uint32(r.DataOffset())+r.DataLength())-64 {
		return true
	}

	return false
}

func (r ReadResponseDecoder) StructureSize() uint16 {
	return le.Uint16(r[:2])
}

func (r ReadResponseDecoder) DataOffset() uint8 {
	return r[2]
}

func (r ReadResponseDecoder) DataLength() uint32 {
	return le.Uint32(r[4:8])
}

func (r ReadResponseDecoder) DataRemaining() uint32 {
	return le.Uint32(r[8:12])
}

// func (r ReadResponseDecoder) Buffer() []byte {
// return r[16:]
// }

func (r ReadResponseDecoder) Data() []byte {
	off := r.DataOffset()
	if off < 16+64 {
		return nil
	}
	off -= 64
	len := r.DataLength()
	return r[off : uint32(off)+len]
}

// ----------------------------------------------------------------------------
// SMB2 WRITE Response
//

type WriteResponse struct {
	PacketHeader

	Count     uint32
	Remaining uint32
}

func (c *WriteResponse) Header() *PacketHeader {
	return &c.PacketHeader
}

func (c *WriteResponse) Size() int {
	return 64 + 16 + 1
}

func (c *WriteResponse) Encode(pkt []byte) {
	c.Command = SMB2_WRITE
	c.encodeHeader(pkt)

	res := pkt[64:]
	le.PutUint16(res[:2], 17) // StructureSize
	le.PutUint32(res[4:8], c.Count)
	le.PutUint32(res[8:12], c.Remaining)
}

type WriteResponseDecoder []byte

func (r WriteResponseDecoder) IsInvalid() bool {
	if len(r) < 16 {
		return true
	}

	if r.StructureSize() != 17 {
		return true
	}

	return false
}

func (r WriteResponseDecoder) StructureSize() uint16 {
	return le.Uint16(r[:2])
}

func (r WriteResponseDecoder) Count() uint32 {
	return le.Uint32(r[4:8])
}

func (r WriteResponseDecoder) Remaining() uint32 {
	return le.Uint32(r[8:12])
}

func (r WriteResponseDecoder) WriteChannelInfoOffset() uint16 {
	return le.Uint16(r[12:14])
}

func (r WriteResponseDecoder) WriteChannelInfoLength() uint16 {
	return le.Uint16(r[14:16])
}

// ----------------------------------------------------------------------------
// SMB2 OPLOCK_BREAK Notification and Response
//

// ----------------------------------------------------------------------------
// SMB2 LOCK Response
//

// ----------------------------------------------------------------------------
// SMB2 ECHO Response
//

// ----------------------------------------------------------------------------
// SMB2 IOCTL Response
//

type IoctlResponse struct {
	PacketHeader

	CtlCode uint32
	FileId  *FileId
	Flags   uint32
	Input   Encoder
	Output  Encoder
}

func (c *IoctlResponse) Header() *PacketHeader {
	return &c.PacketHeader
}

func (c *IoctlResponse) Size() int {
	if c.Input == nil && c.Output == nil {
		return 64 + 48 + 1
	}
	return 64 + 48 + c.Input.Size() + c.Output.Size()
}

func (c *IoctlResponse) Encode(pkt []byte) {
	c.Command = SMB2_IOCTL
	c.encodeHeader(pkt)

	res := pkt[64:]
	le.PutUint16(res[:2], 49) // StructureSize
	le.PutUint32(res[4:8], c.CtlCode)
	c.FileId.Encode(res[8:24])
	le.PutUint32(res[40:44], c.Flags)

	off := 48

	if c.Input != nil {
		le.PutUint32(res[24:28], uint32(off+64)) // InputOffset

		c.Input.Encode(res[off:])

		le.PutUint32(res[28:32], uint32(c.Input.Size())) // InputCount
	}

	if c.Output != nil {
		le.PutUint32(res[32:36], uint32(off+64)) // InputOffset

		c.Output.Encode(res[off:])

		le.PutUint32(res[36:40], uint32(c.Output.Size())) // InputCount
	}
}

type IoctlResponseDecoder []byte

func (r IoctlResponseDecoder) IsInvalid() bool {
	if len(r) < 48 {
		return true
	}

	if r.StructureSize() != 49 {
		return true
	}

	if len(r) < int(r.InputOffset()+r.InputCount())-64 {
		return true
	}

	if len(r) < int(r.OutputOffset()+r.OutputCount())-64 {
		return true
	}

	return false
}

func (r IoctlResponseDecoder) StructureSize() uint16 {
	return le.Uint16(r[:2])
}

func (r IoctlResponseDecoder) CtlCode() uint32 {
	return le.Uint32(r[4:8])
}

func (r IoctlResponseDecoder) FileId() FileIdDecoder {
	return FileIdDecoder(r[8:24])
}

func (r IoctlResponseDecoder) InputOffset() uint32 {
	return le.Uint32(r[24:28])
}

func (r IoctlResponseDecoder) InputCount() uint32 {
	return le.Uint32(r[28:32])
}

func (r IoctlResponseDecoder) OutputOffset() uint32 {
	return le.Uint32(r[32:36])
}

func (r IoctlResponseDecoder) OutputCount() uint32 {
	return le.Uint32(r[36:40])
}

func (r IoctlResponseDecoder) Flags() uint32 {
	return le.Uint32(r[40:44])
}

// func (r IoctlResponseDecoder) Buffer() []byte {
// return r[48:]
// }

func (r IoctlResponseDecoder) Input() []byte {
	off := r.InputOffset()
	if off < 64+48 {
		return nil
	}
	off -= 64
	len := r.InputCount()
	return r[off : off+len]
}

func (r IoctlResponseDecoder) Output() []byte {
	off := r.OutputOffset()
	if off < 64+48 {
		return nil
	}
	off -= 64
	len := r.OutputCount()
	return r[off : off+len]
}

// ----------------------------------------------------------------------------
// SMB2 QUERY_DIRECTORY Response
//

type QueryDirectoryResponse struct {
	PacketHeader

	Output Encoder
}

func (c *QueryDirectoryResponse) Header() *PacketHeader {
	return &c.PacketHeader
}

func (c *QueryDirectoryResponse) Size() int {
	if c.Output == nil {
		return 64 + 8 + 1
	}
	return 64 + 8 + c.Output.Size()
}

func (c *QueryDirectoryResponse) Encode(pkt []byte) {
	c.Command = SMB2_QUERY_DIRECTORY
	c.encodeHeader(pkt)

	res := pkt[64:]
	le.PutUint16(res[:2], 9) // StructureSize

	off := 8

	if c.Output != nil {
		le.PutUint16(res[2:4], uint16(off+64))
		c.Output.Encode(res[8:])
		le.PutUint32(res[4:8], uint32(c.Output.Size()))
	}
}

type QueryDirectoryResponseDecoder []byte

func (r QueryDirectoryResponseDecoder) IsInvalid() bool {
	if len(r) < 8 {
		return true
	}

	if r.StructureSize() != 9 {
		return true
	}

	if len(r) < int(uint32(r.OutputBufferOffset())+r.OutputBufferLength())-64 {
		return true
	}

	return false
}

func (r QueryDirectoryResponseDecoder) StructureSize() uint16 {
	return le.Uint16(r[:2])
}

func (r QueryDirectoryResponseDecoder) OutputBufferOffset() uint16 {
	return le.Uint16(r[2:4])
}

func (r QueryDirectoryResponseDecoder) OutputBufferLength() uint32 {
	return le.Uint32(r[4:8])
}

// func (r QueryDirectoryResponseDecoder) Buffer() []byte {
// return r[8:]
// }

func (r QueryDirectoryResponseDecoder) OutputBuffer() []byte {
	off := r.OutputBufferOffset()
	if off < 64+8 {
		return nil
	}
	off -= 64
	len := r.OutputBufferLength()
	return r[off : uint32(off)+len]
}

// ----------------------------------------------------------------------------
// SMB2 CHANGE_NOTIFY Response
//

// ----------------------------------------------------------------------------
// SMB2 QUERY_INFO Response
//

type QueryInfoResponse struct {
	PacketHeader

	Output Encoder
}

func (c *QueryInfoResponse) Header() *PacketHeader {
	return &c.PacketHeader
}

func (c *QueryInfoResponse) Size() int {
	if c.Output == nil {
		return 64 + 8 + 1
	}
	return 64 + 8 + c.Output.Size()
}

func (c *QueryInfoResponse) Encode(pkt []byte) {
	c.Command = SMB2_QUERY_INFO
	c.encodeHeader(pkt)

	res := pkt[64:]
	le.PutUint16(res[:2], 9) // StructureSize

	off := 8

	if c.Output != nil {
		le.PutUint16(res[2:4], uint16(off+64))
		c.Output.Encode(res[8:])
		le.PutUint32(res[4:8], uint32(c.Output.Size()))
	}
}

type QueryInfoResponseDecoder []byte

func (r QueryInfoResponseDecoder) IsInvalid() bool {
	if len(r) < 8 {
		return true
	}

	if r.StructureSize() != 9 {
		return true
	}

	if len(r) < int(uint32(r.OutputBufferOffset())+r.OutputBufferLength())-64 {
		return true
	}

	return false
}

func (r QueryInfoResponseDecoder) StructureSize() uint16 {
	return le.Uint16(r[:2])
}

func (r QueryInfoResponseDecoder) OutputBufferOffset() uint16 {
	return le.Uint16(r[2:4])
}

func (r QueryInfoResponseDecoder) OutputBufferLength() uint32 {
	return le.Uint32(r[4:8])
}

// func (r QueryInfoResponseDecoder) Buffer() []byte {
// return r[8:]
// }

func (r QueryInfoResponseDecoder) OutputBuffer() []byte {
	off := r.OutputBufferOffset()
	if off < 64+8 {
		return nil
	}
	off -= 64
	len := r.OutputBufferLength()
	return r[off : uint32(off)+len]
}

// ----------------------------------------------------------------------------
// SMB2 SET_INFO Response
//

type SetInfoResponse struct {
	PacketHeader
}

func (c *SetInfoResponse) Header() *PacketHeader {
	return &c.PacketHeader
}

func (c *SetInfoResponse) Size() int {
	return 64 + 2
}

func (c *SetInfoResponse) Encode(pkt []byte) {
	c.Command = SMB2_SET_INFO
	c.encodeHeader(pkt)

	res := pkt[64:]
	le.PutUint16(res[:2], 2) // StructureSize
}

type SetInfoResponseDecoder []byte

func (r SetInfoResponseDecoder) IsInvalid() bool {
	if len(r) < 2 {
		return true
	}

	if r.StructureSize() != 2 {
		return true
	}

	return false
}

func (r SetInfoResponseDecoder) StructureSize() uint16 {
	return le.Uint16(r[:2])
}
