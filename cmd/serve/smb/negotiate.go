package smb

import "time"

// supportedDialects lists the SMB dialects we implement, highest preference
// first. We negotiate up to SMB 3.0.2; 3.1.1 (which adds negotiate contexts and
// pre-auth integrity) is not offered.
var supportedDialects = []uint16{dialect302, dialect300, dialect210, dialect202}

// handleNegotiate handles an SMB2 NEGOTIATE request. It selects the highest
// dialect that both the client and we support. SMB 3.x guest sessions are
// unsigned and unencrypted, like 2.x.
func (c *conn) handleNegotiate(h header, body []byte) (uint32, []byte) {
	dialect, ok := chooseDialect(body)
	if !ok {
		// No dialect we implement was offered; fail cleanly rather than forcing
		// 2.0.2 on a client that never offered it (e.g. a 3.1.1-only client).
		return statusNotSupported, errorResponseBody()
	}
	c.dialect = dialect
	return statusSuccess, negotiateRespBody(c.dialect, c.server.serverGUID)
}

// negotiateRespBody builds the NEGOTIATE response body for the given dialect.
func negotiateRespBody(dialect uint16, guid [16]byte) []byte {
	resp := make([]byte, 64)                              // fixed portion; no security buffer payload
	le.PutUint16(resp[0:2], 65)                           // StructureSize (fixed magic value)
	le.PutUint16(resp[2:4], negotiateSigningEnabled)      // SecurityMode (enabled, not required)
	le.PutUint16(resp[4:6], dialect)                      // DialectRevision
	le.PutUint16(resp[6:8], 0)                            // NegotiateContextCount
	copy(resp[8:24], guid[:])                             // ServerGuid
	le.PutUint32(resp[24:28], 0)                          // Capabilities
	le.PutUint32(resp[28:32], maxIOSize)                  // MaxTransactSize
	le.PutUint32(resp[32:36], maxIOSize)                  // MaxReadSize
	le.PutUint32(resp[36:40], maxIOSize)                  // MaxWriteSize
	le.PutUint64(resp[40:48], timeToFiletime(time.Now())) // SystemTime
	le.PutUint64(resp[48:56], 0)                          // ServerStartTime
	le.PutUint16(resp[56:58], smb2HeaderSize+64)          // SecurityBufferOffset (=128)
	le.PutUint16(resp[58:60], 0)                          // SecurityBufferLength
	le.PutUint32(resp[60:64], 0)                          // NegotiateContextOffset
	return resp
}

// smb1NegotiateResponse builds an SMB2 NEGOTIATE response to a legacy SMB1
// multi-protocol negotiate request. It advertises the SMB2 wildcard dialect
// (0x02FF) so the client re-negotiates using SMB2.
func (c *conn) smb1NegotiateResponse() []byte {
	body := negotiateRespBody(dialectWildcard, c.server.serverGUID)
	out := make([]byte, smb2HeaderSize+len(body))
	copy(out[0:4], smb2Magic)
	le.PutUint16(out[4:6], smb2HeaderSize)       // StructureSize
	le.PutUint16(out[12:14], cmdNegotiate)       // Command
	le.PutUint16(out[14:16], 1)                  // CreditResponse
	le.PutUint32(out[16:20], flagsServerToRedir) // Flags
	copy(out[smb2HeaderSize:], body)
	return out
}

// chooseDialect parses the dialects offered in a NEGOTIATE request body and
// returns the highest one we support. ok is false when the body is malformed or
// offers no dialect we implement.
func chooseDialect(body []byte) (dialect uint16, ok bool) {
	if len(body) < 36 {
		return 0, false
	}
	count := int(le.Uint16(body[2:4]))
	offered := make(map[uint16]bool, count)
	for i := 0; i < count; i++ {
		off := 36 + i*2
		if off+2 > len(body) {
			break
		}
		offered[le.Uint16(body[off:off+2])] = true
	}
	for _, d := range supportedDialects {
		if offered[d] {
			return d, true
		}
	}
	return 0, false
}
