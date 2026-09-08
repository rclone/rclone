package smb

// IOCTL control codes ([MS-FSCC] 2.3).
const (
	// fsctlValidateNegotiateInfo is sent by Windows clients after connecting
	// over SMB 3.x with signing, to detect a tampered NEGOTIATE ([MS-SMB2]
	// 2.2.31.4). If we do not answer it correctly the client disconnects.
	fsctlValidateNegotiateInfo uint32 = 0x00140204
	// fsctlDfsGetReferrals is sent (by cifs) to resolve DFS paths. We are not a
	// DFS server, so we report no referral.
	fsctlDfsGetReferrals uint32 = 0x00060194
)

// handleIoctl handles an SMB2 IOCTL request ([MS-SMB2] 2.2.31). We implement
// FSCTL_VALIDATE_NEGOTIATE_INFO and report "not a DFS path" for DFS referrals;
// all other control codes are unsupported.
func (c *conn) handleIoctl(h header, body []byte) (uint32, []byte) {
	if len(body) < 56 {
		return statusInvalidParameter, errorResponseBody()
	}
	ctlCode := le.Uint32(body[4:8])
	if ctlCode == fsctlDfsGetReferrals {
		return statusNotFound, errorResponseBody()
	}
	if ctlCode != fsctlValidateNegotiateInfo {
		return statusNotSupported, errorResponseBody()
	}
	fileID := body[8:24]

	// VALIDATE_NEGOTIATE_INFO response ([MS-SMB2] 2.2.32.1): the values must
	// match what we sent in the NEGOTIATE response.
	out := make([]byte, 24)
	le.PutUint32(out[0:4], 0)                         // Capabilities
	copy(out[4:20], c.server.serverGUID[:])           // ServerGuid
	le.PutUint16(out[20:22], negotiateSigningEnabled) // SecurityMode
	le.PutUint16(out[22:24], c.dialect)               // Dialect

	resp := make([]byte, 48+len(out))
	le.PutUint16(resp[0:2], 49)                  // StructureSize (fixed magic value)
	le.PutUint32(resp[4:8], ctlCode)             // CtlCode
	copy(resp[8:24], fileID)                     // FileId (echoed)
	le.PutUint32(resp[32:36], smb2HeaderSize+48) // OutputOffset (=112)
	le.PutUint32(resp[36:40], uint32(len(out)))  // OutputCount
	copy(resp[48:], out)
	return statusSuccess, resp
}
