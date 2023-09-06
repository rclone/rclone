package ntlm

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/rc4"
	"encoding/binary"
	"hash"
	"hash/crc32"

	"golang.org/x/crypto/md4"
)

var zero [16]byte

var version = []byte{
	0: WINDOWS_MAJOR_VERSION_10,
	1: WINDOWS_MINOR_VERSION_0,
	7: NTLMSSP_REVISION_W2K3,
}

const defaultFlags = NTLMSSP_NEGOTIATE_56 | NTLMSSP_NEGOTIATE_KEY_EXCH | NTLMSSP_NEGOTIATE_128 | NTLMSSP_NEGOTIATE_TARGET_INFO | NTLMSSP_NEGOTIATE_EXTENDED_SESSIONSECURITY | NTLMSSP_NEGOTIATE_ALWAYS_SIGN | NTLMSSP_NEGOTIATE_NTLM | NTLMSSP_NEGOTIATE_SIGN | NTLMSSP_REQUEST_TARGET | NTLMSSP_NEGOTIATE_UNICODE | NTLMSSP_NEGOTIATE_VERSION

var le = binary.LittleEndian

const (
	NtLmNegotiate    = 0x00000001
	NtLmChallenge    = 0x00000002
	NtLmAuthenticate = 0x00000003
)

const (
	NTLMSSP_NEGOTIATE_UNICODE = 1 << iota
	NTLM_NEGOTIATE_OEM
	NTLMSSP_REQUEST_TARGET
	_
	NTLMSSP_NEGOTIATE_SIGN
	NTLMSSP_NEGOTIATE_SEAL
	NTLMSSP_NEGOTIATE_DATAGRAM
	NTLMSSP_NEGOTIATE_LM_KEY
	_
	NTLMSSP_NEGOTIATE_NTLM
	_
	NTLMSSP_ANONYMOUS
	NTLMSSP_NEGOTIATE_OEM_DOMAIN_SUPPLIED
	NTLMSSP_NEGOTIATE_OEM_WORKSTATION_SUPPLIED
	_
	NTLMSSP_NEGOTIATE_ALWAYS_SIGN
	NTLMSSP_TARGET_TYPE_DOMAIN
	NTLMSSP_TARGET_TYPE_SERVER
	_
	NTLMSSP_NEGOTIATE_EXTENDED_SESSIONSECURITY
	NTLMSSP_NEGOTIATE_IDENTIFY
	_
	NTLMSSP_REQUEST_NON_NT_SESSION_KEY
	NTLMSSP_NEGOTIATE_TARGET_INFO
	_
	NTLMSSP_NEGOTIATE_VERSION
	_
	_
	_
	NTLMSSP_NEGOTIATE_128
	NTLMSSP_NEGOTIATE_KEY_EXCH
	NTLMSSP_NEGOTIATE_56
)

const (
	MsvAvEOL = iota
	MsvAvNbComputerName
	MsvAvNbDomainName
	MsvAvDnsComputerName
	MsvAvDnsDomainName
	MsvAvDnsTreeName
	MsvAvFlags
	MsvAvTimestamp
	MsvAvSingleHost
	MsvAvTargetName
	MsvAvChannelBindings
)

type addr struct {
	typ uint32
	val []byte
}

// channelBindings represents gss_channel_bindings_struct
type channelBindings struct {
	InitiatorAddress addr
	AcceptorAddress  addr
	AppData          []byte
}

var signature = []byte("NTLMSSP\x00")

//      Version
// 0-1: ProductMajorVersion
// 1-2: ProductMinorVersion
// 2-4: ProductBuild
// 4-7: Reserved
// 7-8: NTLMRevisionCurrent

const (
	WINDOWS_MAJOR_VERSION_5  = 0x05
	WINDOWS_MAJOR_VERSION_6  = 0x06
	WINDOWS_MAJOR_VERSION_10 = 0x0a
)

const (
	WINDOWS_MINOR_VERSION_0 = 0x00
	WINDOWS_MINOR_VERSION_1 = 0x01
	WINDOWS_MINOR_VERSION_2 = 0x02
	WINDOWS_MINOR_VERSION_3 = 0x03
)

const (
	NTLMSSP_REVISION_W2K3 = 0x0f
)

func ntowfv2(USER, password, domain []byte) []byte {
	h := md4.New()
	h.Write(password)
	hash := h.Sum(nil)
	return ntowfv2Hash(USER, hash, domain)
}

func ntowfv2Hash(USER, hash, domain []byte) []byte {
	hm := hmac.New(md5.New, hash)
	hm.Write(USER)
	hm.Write(domain)
	return hm.Sum(nil)
}

func encodeNtlmv2Response(dst []byte, h hash.Hash, serverChallenge, clientChallenge, timeStamp []byte, targetInfo encoder) {
	//        NTLMv2Response
	//  0-16: Response
	//   16-: NTLMv2ClientChallenge

	ntlmv2ClientChallenge := dst[16:]

	//        NTLMv2ClientChallenge
	//   0-1: RespType
	//   1-2: HiRespType
	//   2-4: _
	//   4-8: _
	//  8-16: TimeStamp
	// 16-24: ChallengeFromClient
	// 24-28: _
	//   28-: AvPairs

	ntlmv2ClientChallenge[0] = 1
	ntlmv2ClientChallenge[1] = 1
	copy(ntlmv2ClientChallenge[8:16], timeStamp)
	copy(ntlmv2ClientChallenge[16:24], clientChallenge)
	targetInfo.encode(ntlmv2ClientChallenge[28:])

	h.Write(serverChallenge)
	h.Write(ntlmv2ClientChallenge)
	h.Sum(dst[:0]) // ntChallengeResponse.Response
}

type encoder interface {
	size() int
	encode(bs []byte)
}

type bytesEncoder []byte

func (b bytesEncoder) size() int {
	return len(b)
}

func (b bytesEncoder) encode(bs []byte) {
	copy(bs, b)
}

type targetInfoEncoder struct {
	Info    []byte
	SPN     []byte
	InfoMap map[uint16][]byte
}

func newTargetInfoEncoder(info, spn []byte) *targetInfoEncoder {
	infoMap, ok := parseAvPairs(info)
	if !ok {
		return nil
	}
	return &targetInfoEncoder{
		Info:    info,
		SPN:     spn,
		InfoMap: infoMap,
	}
}

func (i *targetInfoEncoder) size() int {
	size := len(i.Info)
	if _, ok := i.InfoMap[MsvAvFlags]; !ok {
		size += 8
	}
	size += 20
	if len(i.SPN) != 0 {
		size += 4 + len(i.SPN)
	}
	return size
}

func (i *targetInfoEncoder) encode(dst []byte) {
	var off int

	if flags, ok := i.InfoMap[MsvAvFlags]; ok {
		le.PutUint32(flags, le.Uint32(flags)|0x02)

		off = copy(dst, i.Info[:len(i.Info)-4])
	} else {
		off = copy(dst, i.Info[:len(i.Info)-4])

		le.PutUint16(dst[off:off+2], MsvAvFlags)
		le.PutUint16(dst[off+2:off+4], 4)
		le.PutUint32(dst[off+4:off+8], 0x02)

		off += 8
	}

	le.PutUint16(dst[off:off+2], MsvAvChannelBindings)
	le.PutUint16(dst[off+2:off+4], 16)

	off += 20

	if len(i.SPN) != 0 {
		le.PutUint16(dst[off:off+2], MsvAvTargetName)
		le.PutUint16(dst[off+2:off+4], uint16(len(i.SPN)))
		copy(dst[off+4:], i.SPN)

		off += 4 + len(i.SPN)
	}

	le.PutUint16(dst[off:off+2], MsvAvEOL)
	le.PutUint16(dst[off+2:off+4], 0)

	off += 4
}

func mac(dst []byte, negotiateFlags uint32, handle *rc4.Cipher, signingKey []byte, seqNum uint32, msg []byte) ([]byte, uint32) {
	ret, tag := sliceForAppend(dst, 16)
	if negotiateFlags&NTLMSSP_NEGOTIATE_EXTENDED_SESSIONSECURITY == 0 {
		//        NtlmsspMessageSignature
		//   0-4: Version
		//   4-8: RandomPad
		//  8-12: Checksum
		// 12-16: SeqNum

		le.PutUint32(tag[:4], 0x00000001)
		le.PutUint32(tag[8:12], crc32.ChecksumIEEE(msg))
		handle.XORKeyStream(tag[4:8], tag[4:8])
		handle.XORKeyStream(tag[8:12], tag[8:12])
		handle.XORKeyStream(tag[12:16], tag[12:16])
		tag[12] ^= byte(seqNum)
		tag[13] ^= byte(seqNum >> 8)
		tag[14] ^= byte(seqNum >> 16)
		tag[15] ^= byte(seqNum >> 24)
		if negotiateFlags&NTLMSSP_NEGOTIATE_DATAGRAM == 0 {
			seqNum++
		}
		tag[4] = 0
		tag[5] = 0
		tag[6] = 0
		tag[7] = 0
	} else {
		//        NtlmsspMessageSignatureExt
		//   0-4: Version
		//  4-12: Checksum
		// 12-16: SeqNum

		le.PutUint32(tag[:4], 0x00000001)
		le.PutUint32(tag[12:16], seqNum)
		h := hmac.New(md5.New, signingKey)
		h.Write(tag[12:16])
		h.Write(msg)
		copy(tag[4:12], h.Sum(nil))
		if negotiateFlags&NTLMSSP_NEGOTIATE_KEY_EXCH != 0 {
			handle.XORKeyStream(tag[4:12], tag[4:12])
		}
		seqNum++
	}

	return ret, seqNum
}

func signKey(negotiateFlags uint32, randomSessionKey []byte, fromClient bool) []byte {
	if negotiateFlags&NTLMSSP_NEGOTIATE_EXTENDED_SESSIONSECURITY != 0 {
		h := md5.New()
		h.Write(randomSessionKey)
		if fromClient {
			h.Write([]byte("session key to client-to-server signing key magic constant\x00"))
		} else {
			h.Write([]byte("session key to server-to-client signing key magic constant\x00"))
		}
		return h.Sum(nil)
	}
	return nil
}

func sealKey(negotiateFlags uint32, randomSessionKey []byte, fromClient bool) []byte {
	if negotiateFlags&NTLMSSP_NEGOTIATE_EXTENDED_SESSIONSECURITY != 0 {
		h := md5.New()
		switch {
		case negotiateFlags&NTLMSSP_NEGOTIATE_128 != 0:
			h.Write(randomSessionKey)
		case negotiateFlags&NTLMSSP_NEGOTIATE_56 != 0:
			h.Write(randomSessionKey[:7])
		default:
			h.Write(randomSessionKey[:5])
		}
		if fromClient {
			h.Write([]byte("session key to client-to-server sealing key magic constant\x00"))
		} else {
			h.Write([]byte("session key to server-to-client sealing key magic constant\x00"))
		}
		return h.Sum(nil)
	}

	if negotiateFlags&NTLMSSP_NEGOTIATE_LM_KEY != 0 {
		sealingKey := make([]byte, 8)
		if negotiateFlags&NTLMSSP_NEGOTIATE_56 != 0 {
			copy(sealingKey, randomSessionKey[:7])
			sealingKey[7] = 0xa0
		} else {
			copy(sealingKey, randomSessionKey[:5])
			sealingKey[5] = 0xe5
			sealingKey[6] = 0x38
			sealingKey[7] = 0xb0
		}
		return sealingKey
	}

	return randomSessionKey
}

func parseAvPairs(bs []byte) (pairs map[uint16][]byte, ok bool) {
	//        AvPair
	//   0-2: AvId
	//   2-4: AvLen
	//    4-: Value

	if len(bs) < 4 {
		return nil, false
	}

	// check MsvAvEOL
	for _, c := range bs[len(bs)-4:] {
		if c != 0x00 {
			return nil, false
		}
	}

	pairs = make(map[uint16][]byte)

	for len(bs) > 0 {
		if len(bs) < 4 {
			return nil, false
		}

		id := le.Uint16(bs[:2])
		// if _, dup := pairs[id]; dup {
		// return nil, false
		// }

		n := int(le.Uint16(bs[2:4]))
		if len(bs) < 4+n {
			return nil, false
		}

		pairs[id] = bs[4 : 4+n]

		bs = bs[4+n:]
	}

	return pairs, true
}

func sliceForAppend(in []byte, n int) (head, tail []byte) {
	if total := len(in) + n; cap(in) >= total {
		head = in[:total]
	} else {
		head = make([]byte, total)
		copy(head, in)
	}
	tail = head[len(in):]
	return
}
