package smb

import "time"

// This file implements just enough SPNEGO/GSS-API and NTLMSSP to complete an
// SMB2 SESSION_SETUP. The security buffer in SESSION_SETUP is a SPNEGO token
// (DER encoded). We need to:
//
//   - extract the inner NTLMSSP message from the client's tokens, and
//   - build our own SPNEGO negTokenResp wrapping an NTLMSSP CHALLENGE.
//
// For guest (no auth) we do not validate the NTLMSSP messages; we only use the
// NTLMSSP MessageType to tell the two SESSION_SETUP round trips apart.

// oidNLMP is the full DER TLV (tag 0x06) for the NTLMSSP mechanism OID
// 1.3.6.1.4.1.311.2.2.10.
var oidNLMP = []byte{0x06, 0x0A, 0x2b, 0x06, 0x01, 0x04, 0x01, 0x82, 0x37, 0x02, 0x02, 0x0A}

// SPNEGO negotiation states.
const (
	negStateAcceptCompleted  byte = 0
	negStateAcceptIncomplete byte = 1
)

// NTLMSSP message signature and the message types we care about.
var ntlmSignature = []byte("NTLMSSP\x00")

const (
	ntlmTypeNegotiate    uint32 = 1
	ntlmTypeChallenge    uint32 = 2
	ntlmTypeAuthenticate uint32 = 3
)

// ntlmDefaultFlags are the NTLMSSP negotiate flags we advertise in the
// CHALLENGE message (matches the common reference set used by SMB clients).
const ntlmDefaultFlags uint32 = 0xE2898215

// ntlmNegotiateTargetTypeServer marks the CHALLENGE target name as a server name.
const ntlmNegotiateTargetTypeServer uint32 = 0x00020000

// derLen encodes a DER length.
func derLen(n int) []byte {
	switch {
	case n < 0x80:
		return []byte{byte(n)}
	case n < 0x100:
		return []byte{0x81, byte(n)}
	default:
		return []byte{0x82, byte(n >> 8), byte(n)}
	}
}

// derTLV encodes a DER tag-length-value.
func derTLV(tag byte, content []byte) []byte {
	out := make([]byte, 0, 1+len(content)+3)
	out = append(out, tag)
	out = append(out, derLen(len(content))...)
	out = append(out, content...)
	return out
}

// derReadTLV reads a single DER tag-length-value from the front of b. It
// returns the tag, the contents, and the bytes following the element.
func derReadTLV(b []byte) (tag byte, content, rest []byte, ok bool) {
	if len(b) < 2 {
		return 0, nil, nil, false
	}
	tag = b[0]
	n := int(b[1])
	i := 2
	if n&0x80 != 0 {
		nbytes := n & 0x7f
		if nbytes == 0 || nbytes > 4 || len(b) < 2+nbytes {
			return 0, nil, nil, false
		}
		n = 0
		for j := 0; j < nbytes; j++ {
			n = n<<8 | int(b[2+j])
		}
		i = 2 + nbytes
	}
	if n < 0 || len(b) < i+n {
		return 0, nil, nil, false
	}
	return tag, b[i : i+n], b[i+n:], true
}

// spnegoExtractNTLM extracts the inner NTLMSSP token from a SPNEGO security
// buffer. It handles both negTokenInit (GSS-API APPLICATION tag 0x60, used in
// the first SESSION_SETUP request) and negTokenResp (tag 0xA1, used in
// subsequent requests).
func spnegoExtractNTLM(buf []byte) ([]byte, bool) {
	tag, content, _, ok := derReadTLV(buf)
	if !ok {
		return nil, false
	}
	var seq []byte
	switch tag {
	case 0x60: // GSS-API negTokenInit: OID + [0] NegTokenInit
		_, _, afterOID, ok := derReadTLV(content) // skip SPNEGO OID
		if !ok {
			return nil, false
		}
		t, c, _, ok := derReadTLV(afterOID) // [0]
		if !ok || t != 0xA0 {
			return nil, false
		}
		t, c, _, ok = derReadTLV(c) // SEQUENCE
		if !ok || t != 0x30 {
			return nil, false
		}
		seq = c
	case 0xA1: // negTokenResp: [1] NegTokenResp -> SEQUENCE
		t, c, _, ok := derReadTLV(content)
		if !ok || t != 0x30 {
			return nil, false
		}
		seq = c
	default:
		return nil, false
	}
	// Walk the SEQUENCE looking for [2] (mechToken / responseToken).
	for len(seq) > 0 {
		t, c, next, ok := derReadTLV(seq)
		if !ok {
			return nil, false
		}
		if t == 0xA2 {
			t2, c2, _, ok := derReadTLV(c) // OCTET STRING
			if !ok || t2 != 0x04 {
				return nil, false
			}
			return c2, true
		}
		seq = next
	}
	return nil, false
}

// ntlmMessageType returns the NTLMSSP MessageType of token, or 0 if token is
// not a valid NTLMSSP message.
func ntlmMessageType(token []byte) uint32 {
	if len(token) < 12 || string(token[0:8]) != string(ntlmSignature) {
		return 0
	}
	return le.Uint32(token[8:12])
}

// buildNegTokenResp builds a SPNEGO negTokenResp. supportedMech adds the
// NTLMSSP mechanism OID (required in the server's first reply); token, when
// non-nil, is wrapped as the responseToken.
func buildNegTokenResp(state byte, supportedMech bool, token []byte) []byte {
	var seq []byte
	seq = append(seq, derTLV(0xA0, derTLV(0x0A, []byte{state}))...) // [0] negState
	if supportedMech {
		seq = append(seq, derTLV(0xA1, oidNLMP)...) // [1] supportedMech
	}
	if token != nil {
		seq = append(seq, derTLV(0xA2, derTLV(0x04, token))...) // [2] responseToken
	}
	return derTLV(0xA1, derTLV(0x30, seq))
}

// avPair encodes an NTLMSSP AV_PAIR ([MS-NLMP] 2.2.2.1).
func avPair(id uint16, value []byte) []byte {
	b := make([]byte, 4+len(value))
	le.PutUint16(b[0:2], id)
	le.PutUint16(b[2:4], uint16(len(value)))
	copy(b[4:], value)
	return b
}

// buildNTLMChallenge builds an NTLMSSP CHALLENGE (Type 2) message ([MS-NLMP]
// 2.2.1.2) carrying the given 8-byte server challenge plus a target name and a
// full target-info block (computer/domain names and a timestamp). Windows
// requires these AV pairs to compute its NTLMv2 response.
func buildNTLMChallenge(serverChallenge [8]byte) []byte {
	const computer = "RCLONE"
	const domain = "WORKGROUP"
	nbComputer := stringToUTF16le(computer)
	nbDomain := stringToUTF16le(domain)
	targetName := nbDomain

	timestamp := make([]byte, 8)
	le.PutUint64(timestamp, timeToFiletime(time.Now()))

	var targetInfo []byte
	targetInfo = append(targetInfo, avPair(2, nbDomain)...)   // MsvAvNbDomainName
	targetInfo = append(targetInfo, avPair(1, nbComputer)...) // MsvAvNbComputerName
	targetInfo = append(targetInfo, avPair(4, nbDomain)...)   // MsvAvDnsDomainName
	targetInfo = append(targetInfo, avPair(3, nbComputer)...) // MsvAvDnsComputerName
	targetInfo = append(targetInfo, avPair(7, timestamp)...)  // MsvAvTimestamp
	targetInfo = append(targetInfo, avPair(0, nil)...)        // MsvAvEOL

	const fixedLen = 56 // includes the 8-byte Version field
	targetNameOff := fixedLen
	targetInfoOff := targetNameOff + len(targetName)

	b := make([]byte, fixedLen+len(targetName)+len(targetInfo))
	copy(b[0:8], ntlmSignature)
	le.PutUint32(b[8:12], ntlmTypeChallenge)
	le.PutUint16(b[12:14], uint16(len(targetName)))
	le.PutUint16(b[14:16], uint16(len(targetName)))
	le.PutUint32(b[16:20], uint32(targetNameOff))
	le.PutUint32(b[20:24], ntlmDefaultFlags|ntlmNegotiateTargetTypeServer)
	copy(b[24:32], serverChallenge[:])
	// Reserved [32:40] = 0
	le.PutUint16(b[40:42], uint16(len(targetInfo)))
	le.PutUint16(b[42:44], uint16(len(targetInfo)))
	le.PutUint32(b[44:48], uint32(targetInfoOff))
	// Version [48:56]: Windows-style {major 10, minor 0, build 0, NTLM rev 15}
	b[48] = 10
	b[55] = 0x0f
	copy(b[targetNameOff:], targetName)
	copy(b[targetInfoOff:], targetInfo)
	return b
}
