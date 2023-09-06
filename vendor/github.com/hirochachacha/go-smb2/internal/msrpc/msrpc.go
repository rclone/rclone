package msrpc

import (
	"encoding/binary"
	"encoding/hex"

	"github.com/hirochachacha/go-smb2/internal/utf16le"
)

var le = binary.LittleEndian

func roundup(x, align int) int {
	return (x + (align - 1)) &^ (align - 1)
}

const (
	RPC_VERSION       = 5
	RPC_VERSION_MINOR = 0

	RPC_TYPE_REQUEST  = 0
	RPC_TYPE_RESPONSE = 2
	RPC_TYPE_BIND     = 11
	RPC_TYPE_BIND_ACK = 12

	RPC_PACKET_FLAG_FIRST = 0x01
	RPC_PACKET_FLAG_LAST  = 0x02

	SRVSVC_VERSION       = 3
	SRVSVC_VERSION_MINOR = 0

	NDR_VERSION = 2

	OP_NET_SHARE_ENUM = 15
)

var (
	SRVSVC_UUID = []byte("c84f324b7016d30112785a47bf6ee188")
	NDR_UUID    = []byte("045d888aeb1cc9119fe808002b104860")
)

type Bind struct {
	CallId uint32
}

func (r *Bind) Size() int {
	return 72
}

func (r *Bind) Encode(b []byte) {
	b[0] = RPC_VERSION
	b[1] = RPC_VERSION_MINOR
	b[2] = RPC_TYPE_BIND
	b[3] = RPC_PACKET_FLAG_FIRST | RPC_PACKET_FLAG_LAST

	// order = Little-Endian, float = IEEE, char = ASCII
	b[4] = 0x10
	b[5] = 0
	b[6] = 0
	b[7] = 0

	le.PutUint16(b[8:10], 72)        // frag length
	le.PutUint16(b[10:12], 0)        // auth length
	le.PutUint32(b[12:16], r.CallId) // call id
	le.PutUint16(b[16:18], 4280)     // max xmit frag
	le.PutUint16(b[18:20], 4280)     // max recv frag
	le.PutUint32(b[20:24], 0)        // assoc group
	le.PutUint32(b[24:28], 1)        // num ctx items
	le.PutUint16(b[28:30], 0)        // ctx item[1] .context id
	le.PutUint16(b[30:32], 1)        // ctx item[1] .num trans items

	hex.Decode(b[32:48], SRVSVC_UUID)
	le.PutUint16(b[48:50], SRVSVC_VERSION)
	le.PutUint16(b[50:52], SRVSVC_VERSION_MINOR)

	hex.Decode(b[52:68], NDR_UUID)
	le.PutUint32(b[68:72], NDR_VERSION)
}

type BindAckDecoder []byte

func (c BindAckDecoder) IsInvalid() bool {
	if len(c) < 24 {
		return true
	}
	if c.Version() != RPC_VERSION {
		return true
	}
	if c.VersionMinor() != RPC_VERSION_MINOR {
		return true
	}
	if c.PacketType() != RPC_TYPE_BIND_ACK {
		return true
	}
	return false
}

func (c BindAckDecoder) Version() uint8 {
	return c[0]
}

func (c BindAckDecoder) VersionMinor() uint8 {
	return c[1]
}

func (c BindAckDecoder) PacketType() uint8 {
	return c[2]
}

func (c BindAckDecoder) PacketFlags() uint8 {
	return c[3]
}

func (c BindAckDecoder) DataRepresentation() []byte {
	return c[4:8]
}

func (c BindAckDecoder) FragLength() uint16 {
	return le.Uint16(c[8:10])
}

func (c BindAckDecoder) AuthLength() uint16 {
	return le.Uint16(c[10:12])
}

func (c BindAckDecoder) CallId() uint32 {
	return le.Uint32(c[12:16])
}

func (c BindAckDecoder) MaxXmitFrag() uint16 {
	return le.Uint16(c[16:18])
}

func (c BindAckDecoder) MaxRecvFrag() uint16 {
	return le.Uint16(c[18:20])
}

func (c BindAckDecoder) AssocGroupId() uint32 {
	return le.Uint32(c[20:24])
}

type NetShareEnumAllRequest struct {
	CallId     uint32
	ServerName string
	Level      uint32
}

func (r *NetShareEnumAllRequest) Size() int {
	off := 40 + utf16le.EncodedStringLen(r.ServerName) + 2
	off = roundup(off, 4)
	off += 24
	off += 4
	return off
}

func (r *NetShareEnumAllRequest) Encode(b []byte) {
	b[0] = RPC_VERSION
	b[1] = RPC_VERSION_MINOR
	b[2] = RPC_TYPE_REQUEST
	b[3] = RPC_PACKET_FLAG_FIRST | RPC_PACKET_FLAG_LAST

	// order = Little-Endian, float = IEEE, char = ASCII
	b[4] = 0x10
	b[5] = 0
	b[6] = 0
	b[7] = 0

	le.PutUint16(b[10:12], 0)                 // auth length
	le.PutUint32(b[12:16], r.CallId)          // call id
	le.PutUint16(b[20:22], 0)                 // context id
	le.PutUint16(b[22:24], OP_NET_SHARE_ENUM) // opnum

	// follwing parts will change if we use NDR64 instead of NDR

	// pointer to server unc

	le.PutUint32(b[24:28], 0x20000) // referent ID

	count := utf16le.EncodedStringLen(r.ServerName)/2 + 1

	le.PutUint32(b[28:32], uint32(count)) // max count
	le.PutUint32(b[32:36], 0)             // offset
	le.PutUint32(b[36:40], uint32(count)) // actual count

	utf16le.EncodeString(b[40:], r.ServerName) // server unc

	off := 40 + count*2
	off = roundup(off, 4)

	// pointer level

	le.PutUint32(b[off:off+4], r.Level)

	// pointer to ctr (srvsvc_NetShareCtr)

	le.PutUint32(b[off+4:off+8], 1)            // ctr
	le.PutUint32(b[off+8:off+12], 0x20004)     // referent ID
	le.PutUint32(b[off+12:off+16], 0)          // ctr1.count
	le.PutUint32(b[off+16:off+20], 0)          // ctr1.pointer
	le.PutUint32(b[off+20:off+24], 0xffffffff) // max buffer

	off += 24

	// pointer to resume handle

	le.PutUint32(b[off:off+4], 0) // null pointer
	// le.PutUint32(b[off:off+4], 0x20008) // referent ID
	// le.PutUint32(b[off+4:off+8], 0)     // resume handle

	off += 4

	le.PutUint16(b[8:10], uint16(off))     // frag length
	le.PutUint32(b[16:20], uint32(off-24)) // alloc hint
}

type NetShareEnumAllResponseDecoder []byte

func (c NetShareEnumAllResponseDecoder) IsInvalid() bool {
	if len(c) < 24 {
		return true
	}
	if c.Version() != RPC_VERSION {
		return true
	}
	if c.VersionMinor() != RPC_VERSION_MINOR {
		return true
	}
	if c.PacketType() != RPC_TYPE_RESPONSE {
		return true
	}

	return false
}

func (c NetShareEnumAllResponseDecoder) Version() uint8 {
	return c[0]
}

func (c NetShareEnumAllResponseDecoder) VersionMinor() uint8 {
	return c[1]
}

func (c NetShareEnumAllResponseDecoder) PacketType() uint8 {
	return c[2]
}

func (c NetShareEnumAllResponseDecoder) PacketFlags() uint8 {
	return c[3]
}

func (c NetShareEnumAllResponseDecoder) DataRepresentation() []byte {
	return c[4:8]
}

func (c NetShareEnumAllResponseDecoder) FragLength() uint16 {
	return le.Uint16(c[8:10])
}

func (c NetShareEnumAllResponseDecoder) AuthLength() uint16 {
	return le.Uint16(c[10:12])
}

func (c NetShareEnumAllResponseDecoder) CallId() uint32 {
	return le.Uint32(c[12:16])
}

func (c NetShareEnumAllResponseDecoder) AllocHint() uint32 {
	return le.Uint32(c[16:20])
}

func (c NetShareEnumAllResponseDecoder) ContextId() uint16 {
	return le.Uint16(c[20:22])
}

func (c NetShareEnumAllResponseDecoder) CancelCount() uint8 {
	return c[22]
}

func (c NetShareEnumAllResponseDecoder) IsIncomplete() bool {
	if len(c) < 48 {
		return true
	}

	level := le.Uint32(c[24:28])

	count := int(le.Uint32(c[36:40]))

	switch level {
	case 0:
		offset := 48 + count*4 // name pointer
		if len(c) < offset {
			return true
		}

		for i := 0; i < count; i++ {
			if len(c) < offset+12 {
				return true
			}

			noff := int(le.Uint32(c[offset+4 : offset+8]))    // offset
			nlen := int(le.Uint32(c[offset+8:offset+12])) * 2 // actual count
			offset = roundup(offset+12+noff+nlen, 4)

			if len(c) < offset {
				return true
			}
		}
	case 1:
		offset := 48 + count*12
		if len(c) < offset {
			return true
		}

		for i := 0; i < count; i++ {
			{ // name
				if len(c) < offset+12 {
					return true
				}

				noff := int(le.Uint32(c[offset+4 : offset+8]))    // offset
				nlen := int(le.Uint32(c[offset+8:offset+12])) * 2 // actual count
				offset = roundup(offset+12+noff+nlen, 4)

				if len(c) < offset {
					return true
				}
			}

			{ // comment
				if len(c) < offset+12 {
					return true
				}

				coff := int(le.Uint32(c[offset+4 : offset+8]))    // offset
				clen := int(le.Uint32(c[offset+8:offset+12])) * 2 // actual count
				offset = roundup(offset+12+coff+clen, 4)

				if len(c) < offset {
					return true
				}
			}
		}
	default:
		// TODO not supported yet
		return true
	}

	return false
}

func (c NetShareEnumAllResponseDecoder) Buffer() []byte {
	return c[24:]
}

func (c NetShareEnumAllResponseDecoder) ShareNameList() []string {
	level := le.Uint32(c[24:28])

	count := int(le.Uint32(c[36:40]))

	ss := make([]string, count)

	switch level {
	case 0:
		offset := 48 + count*4 // name pointer
		for i := 0; i < count; i++ {
			noff := int(le.Uint32(c[offset+4 : offset+8]))    // offset
			nlen := int(le.Uint32(c[offset+8:offset+12])) * 2 // actual count

			ss[i] = utf16le.DecodeToString(c[offset+12+noff : offset+12+noff+nlen])

			offset = roundup(offset+12+noff+nlen, 4)
		}
	case 1:
		offset := 48 + count*12
		for i := 0; i < count; i++ {
			{ // name
				noff := int(le.Uint32(c[offset+4 : offset+8]))    // offset
				nlen := int(le.Uint32(c[offset+8:offset+12])) * 2 // actual count

				ss[i] = utf16le.DecodeToString(c[offset+12+noff : offset+12+noff+nlen])

				offset = roundup(offset+12+noff+nlen, 4)
			}

			{ // comment
				coff := int(le.Uint32(c[offset+4 : offset+8]))    // offset
				clen := int(le.Uint32(c[offset+8:offset+12])) * 2 // actual count
				offset = roundup(offset+12+coff+clen, 4)
			}
		}
	default:
		// TODO not supported yet
		return nil
	}

	return ss
}
