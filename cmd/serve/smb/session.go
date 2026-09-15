package smb

import (
	"crypto/rand"
	"strings"

	"github.com/rclone/rclone/fs"
)

// handleSessionSetup handles an SMB2 SESSION_SETUP request. NTLM authentication
// is a two round-trip handshake wrapped in SPNEGO:
//
//  1. client sends NTLMSSP NEGOTIATE (Type 1); we reply STATUS_MORE_PROCESSING_
//     REQUIRED with an NTLMSSP CHALLENGE (Type 2) carrying a server challenge,
//     and
//  2. client sends NTLMSSP AUTHENTICATE (Type 3); we validate it and reply
//     STATUS_SUCCESS (or STATUS_LOGON_FAILURE).
//
// When no --user is configured the server accepts any client as a guest, which
// tells the client not to sign its requests.
func (c *conn) handleSessionSetup(h header, body []byte) (status uint32, sessionID uint64, respBody []byte) {
	if len(body) < 24 {
		return statusInvalidParameter, h.sessionID, errorResponseBody()
	}
	secBuf := bufferAt(body, le.Uint16(body[12:14]), le.Uint16(body[14:16]))

	// The NTLMSSP token is delivered either wrapped in SPNEGO/GSS (Windows and
	// go-smb2) or as a bare NTLMSSP message (Linux cifs over SMB2/3). Detect
	// which form was used so we can reply in kind.
	rawNTLM := len(secBuf) >= 8 && string(secBuf[:8]) == string(ntlmSignature)
	var token []byte
	if rawNTLM {
		token = secBuf
	} else {
		token, _ = spnegoExtractNTLM(secBuf)
	}

	sessionID = h.sessionID
	if sessionID == 0 {
		sessionID = c.server.nextSessionID()
	}

	if ntlmMessageType(token) == ntlmTypeAuthenticate {
		sessionFlags, ok := c.authenticate(token)
		if !ok {
			return statusLogonFailure, sessionID, errorResponseBody()
		}
		if c.server.opt.User != "" {
			// Record the session as authenticated so later commands pass the auth
			// gate in handleCommand.
			c.authedSessions[sessionID] = struct{}{}
		}
		c.maybeWarnWindowsGuest(token)
		// Final reply: empty for raw NTLMSSP, an "accept completed" SPNEGO
		// token otherwise.
		var reply []byte
		if !rawNTLM {
			reply = buildNegTokenResp(negStateAcceptCompleted, false, nil)
		}
		return statusSuccess, sessionID, sessionSetupRespBody(sessionFlags, reply)
	}

	// First round trip: generate and remember a challenge, then send it back in
	// the same wrapping the client used.
	_, _ = rand.Read(c.authChallenge[:])
	challenge := buildNTLMChallenge(c.authChallenge)
	reply := challenge
	if !rawNTLM {
		reply = buildNegTokenResp(negStateAcceptIncomplete, true, challenge)
	}
	return statusMoreProcessingRequired, sessionID, sessionSetupRespBody(0, reply)
}

// authenticate validates an NTLMSSP AUTHENTICATE token. It returns the session
// flags to report and whether authentication succeeded. With no --user
// configured every client is accepted as a guest. On a successful
// authenticated logon it derives the message signing key for the session.
func (c *conn) authenticate(token []byte) (sessionFlags uint16, ok bool) {
	if c.server.opt.User == "" {
		return sessionFlagIsGuest, true
	}
	auth, ok := parseNTLMAuthenticate(token)
	if !ok {
		return 0, false
	}
	if !strings.EqualFold(auth.user, c.server.opt.User) {
		return 0, false
	}
	responseKeyNT := ntowfv2(c.server.opt.User, c.server.opt.Pass, auth.domain)
	if !validateNTLMv2(responseKeyNT, c.authChallenge, auth.ntResponse) {
		return 0, false
	}
	// Derive the SMB signing key for this session so responses can be signed.
	sessionKey := exportedSessionKey(responseKeyNT, auth.ntResponse[:16], auth.flags, auth.encSessionKey)
	switch c.dialect {
	case dialect300, dialect302:
		c.signKey = kdf(sessionKey, []byte("SMB2AESCMAC\x00"), []byte("SmbSign\x00"))
	default:
		c.signKey = sessionKey
	}
	return 0, true
}

// maybeWarnWindowsGuest logs a one-time hint when a Windows client connects in
// guest mode. Windows rejects guest sessions (they are unsigned and Windows
// requires SMB signing), so this points the user at --user/--pass. The client
// is recognised as Windows from the NTLMSSP OS Version major; raw-NTLM clients
// (e.g. Linux cifs) are never Windows and are skipped.
func (c *conn) maybeWarnWindowsGuest(token []byte) {
	if c.server.opt.User != "" || c.warnedWindowsGuest {
		return
	}
	// Windows 10/11 (NTLMSSP OS major version 10) are the clients that block guest;
	// Linux cifs reports major 6, so this avoids warning about clients that work.
	auth, ok := parseNTLMAuthenticate(token)
	if !ok || auth.osMajor < 10 {
		return
	}
	c.warnedWindowsGuest = true
	fs.Logf(c.server.vfs.Fs(), "SMB: a Windows client connected as guest and will likely reject the "+
		"session -- Windows requires SMB signing, which a guest session cannot provide. Use --user "+
		"and --pass for Windows clients.")
}

// sessionSetupRespBody builds the SESSION_SETUP response body ([MS-SMB2]
// 2.2.6) carrying the supplied session flags and SPNEGO security buffer.
func sessionSetupRespBody(sessionFlags uint16, secBuf []byte) []byte {
	body := make([]byte, 8+len(secBuf))
	le.PutUint16(body[0:2], 9) // StructureSize (fixed magic value)
	le.PutUint16(body[2:4], sessionFlags)
	le.PutUint16(body[4:6], smb2HeaderSize+8) // SecurityBufferOffset (=72)
	le.PutUint16(body[6:8], uint16(len(secBuf)))
	copy(body[8:], secBuf)
	return body
}

// bufferAt returns the slice of a request body referenced by an offset (from
// the start of the SMB2 header) and length, or nil if out of range.
func bufferAt(body []byte, fileOffset, length uint16) []byte {
	start := int(fileOffset) - smb2HeaderSize
	if start < 0 || length == 0 || start+int(length) > len(body) {
		return nil
	}
	return body[start : start+int(length)]
}
