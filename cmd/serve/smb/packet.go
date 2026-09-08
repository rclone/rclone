package smb

import (
	"time"
	"unicode/utf16"
)

// SMB2 protocol constants. See [MS-SMB2] for the authoritative reference.
const (
	smb2HeaderSize = 64
	smb2Magic      = "\xfeSMB"
)

// SMB2 command codes ([MS-SMB2] 2.2.1).
const (
	cmdNegotiate      uint16 = 0x0000
	cmdSessionSetup   uint16 = 0x0001
	cmdLogoff         uint16 = 0x0002
	cmdTreeConnect    uint16 = 0x0003
	cmdTreeDisconnect uint16 = 0x0004
	cmdCreate         uint16 = 0x0005
	cmdClose          uint16 = 0x0006
	cmdFlush          uint16 = 0x0007
	cmdRead           uint16 = 0x0008
	cmdWrite          uint16 = 0x0009
	cmdLock           uint16 = 0x000A
	cmdIoctl          uint16 = 0x000B
	cmdCancel         uint16 = 0x000C
	cmdEcho           uint16 = 0x000D
	cmdQueryDirectory uint16 = 0x000E
	cmdChangeNotify   uint16 = 0x000F
	cmdQueryInfo      uint16 = 0x0010
	cmdSetInfo        uint16 = 0x0011
	cmdOplockBreak    uint16 = 0x0012
)

// SMB2 header flags ([MS-SMB2] 2.2.1.2).
const (
	flagsServerToRedir uint32 = 0x00000001
	flagsAsyncCommand  uint32 = 0x00000002
	flagsRelatedOps    uint32 = 0x00000004
	flagsSigned        uint32 = 0x00000008
)

// SMB2 dialect revisions ([MS-SMB2] 2.2.3).
const (
	dialect202      uint16 = 0x0202
	dialect210      uint16 = 0x0210
	dialect300      uint16 = 0x0300
	dialect302      uint16 = 0x0302
	dialect311      uint16 = 0x0311
	dialectWildcard uint16 = 0x02FF // SMB2 wildcard, used to upgrade from an SMB1 negotiate
)

// Negotiate SecurityMode flags.
const (
	negotiateSigningEnabled  uint16 = 0x0001
	negotiateSigningRequired uint16 = 0x0002
)

// SESSION_SETUP response SessionFlags.
const (
	sessionFlagIsGuest     uint16 = 0x0001
	sessionFlagIsNull      uint16 = 0x0002
	sessionFlagEncryptData uint16 = 0x0004
)

// TREE_CONNECT response ShareType.
const (
	shareTypeDisk byte = 0x01
	shareTypePipe byte = 0x02
)

// NTSTATUS codes ([MS-ERREF] 2.3.1) that we use.
const (
	statusSuccess                uint32 = 0x00000000
	statusPending                uint32 = 0x00000103
	statusNoMoreFiles            uint32 = 0x80000006
	statusMoreProcessingRequired uint32 = 0xC0000016
	statusNoSuchFile             uint32 = 0xC000000F
	statusObjectNameNotFound     uint32 = 0xC0000034
	statusObjectNameCollision    uint32 = 0xC0000035
	statusObjectPathNotFound     uint32 = 0xC000003A
	statusAccessDenied           uint32 = 0xC0000022
	statusBadNetworkName         uint32 = 0xC00000CC
	statusLogonFailure           uint32 = 0xC000006D
	statusUserSessionDeleted     uint32 = 0xC0000203
	statusInsufficientResources  uint32 = 0xC000009A
	statusNotSupported           uint32 = 0xC00000BB
	statusNotFound               uint32 = 0xC0000225
	statusNotImplemented         uint32 = 0xC0000002
	statusInvalidParameter       uint32 = 0xC000000D
	statusEndOfFile              uint32 = 0xC0000011
	statusFileIsADirectory       uint32 = 0xC00000BA
	statusNotADirectory          uint32 = 0xC0000103
	statusInvalidInfoClass       uint32 = 0xC0000003
	statusSharingViolation       uint32 = 0xC0000043
	statusDirectoryNotEmpty      uint32 = 0xC0000101
	statusMediaWriteProtected    uint32 = 0xC00000A2
	statusUnsuccessful           uint32 = 0xC0000001
)

// header holds the fields of an SMB2 message header that we need.
type header struct {
	creditCharge  uint16
	command       uint16
	creditReqResp uint16
	flags         uint32
	nextCommand   uint32
	messageID     uint64
	treeID        uint32
	sessionID     uint64
}

// parseHeader parses the 64-byte SMB2 header at the start of b.
func parseHeader(b []byte) (h header, ok bool) {
	if len(b) < smb2HeaderSize || string(b[0:4]) != smb2Magic {
		return header{}, false
	}
	h = header{
		creditCharge:  le.Uint16(b[6:8]),
		command:       le.Uint16(b[12:14]),
		creditReqResp: le.Uint16(b[14:16]),
		flags:         le.Uint32(b[16:20]),
		nextCommand:   le.Uint32(b[20:24]),
		messageID:     le.Uint64(b[24:32]),
		treeID:        le.Uint32(b[36:40]),
		sessionID:     le.Uint64(b[40:48]),
	}
	return h, true
}

// buildResponse assembles a full SMB2 response message (a 64-byte header
// followed by body). The reply echoes the request's MessageId and carries the
// supplied status, session and tree ids and granted credits.
func buildResponse(reqH header, command uint16, status uint32, sessionID uint64, treeID uint32, credits uint16, body []byte) []byte {
	out := make([]byte, smb2HeaderSize+len(body))
	copy(out[0:4], smb2Magic)
	le.PutUint16(out[4:6], smb2HeaderSize)    // StructureSize (always 64)
	le.PutUint16(out[6:8], reqH.creditCharge) // echo the request's CreditCharge (2.1+)
	le.PutUint32(out[8:12], status)
	le.PutUint16(out[12:14], command)
	le.PutUint16(out[14:16], credits) // CreditResponse
	le.PutUint32(out[16:20], flagsServerToRedir)
	// NextCommand [20:24] = 0 (we do not compound responses)
	le.PutUint64(out[24:32], reqH.messageID)
	// Reserved [32:36] = 0
	le.PutUint32(out[36:40], treeID)
	le.PutUint64(out[40:48], sessionID)
	// Signature [48:64] = 0 (we do not sign guest sessions)
	copy(out[smb2HeaderSize:], body)
	return out
}

// errorResponseBody returns the minimal SMB2 ERROR Response body
// ([MS-SMB2] 2.2.2): StructureSize 9, no error context data.
func errorResponseBody() []byte {
	b := make([]byte, 9)
	le.PutUint16(b[0:2], 9)
	return b
}

// filetimeEpochDelta is the number of 100-nanosecond intervals between the
// FILETIME epoch (1601-01-01) and the Unix epoch (1970-01-01).
const filetimeEpochDelta = 116444736000000000

// timeToFiletime converts a time.Time to a Windows FILETIME.
func timeToFiletime(t time.Time) uint64 {
	if t.IsZero() {
		return 0
	}
	return uint64(t.UnixNano()/100 + filetimeEpochDelta)
}

// filetimeToTime converts a Windows FILETIME to a time.Time.
func filetimeToTime(ft uint64) time.Time {
	if ft == 0 {
		return time.Time{}
	}
	return time.Unix(0, (int64(ft)-filetimeEpochDelta)*100)
}

// utf16leToString decodes a little-endian UTF-16 byte slice to a string.
func utf16leToString(b []byte) string {
	if len(b)%2 != 0 {
		b = b[:len(b)-1]
	}
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = le.Uint16(b[2*i:])
	}
	return string(utf16.Decode(u))
}

// stringToUTF16le encodes a string as a little-endian UTF-16 byte slice.
func stringToUTF16le(s string) []byte {
	u := utf16.Encode([]rune(s))
	b := make([]byte, 2*len(u))
	for i, v := range u {
		le.PutUint16(b[2*i:], v)
	}
	return b
}
