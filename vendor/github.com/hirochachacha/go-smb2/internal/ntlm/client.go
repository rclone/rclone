package ntlm

import (
	"bytes"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/rc4"
	"errors"
	"hash"
	"strings"
	"time"

	"github.com/hirochachacha/go-smb2/internal/utf16le"
)

// NTLM v2 client
type Client struct {
	User        string
	Password    string
	Hash        []byte
	Domain      string // e.g "WORKGROUP", "MicrosoftAccount"
	Workstation string // e.g "localhost", "HOME-PC"

	TargetSPN       string           // SPN ::= "service/hostname[:port]"; e.g "cifs/remotehost:1020"
	channelBindings *channelBindings // reserved for future implementation

	nmsg    []byte
	session *Session
}

func (c *Client) Negotiate() (nmsg []byte, err error) {
	//        NegotiateMessage
	//   0-8: Signature
	//  8-12: MessageType
	// 12-16: NegotiateFlags
	// 16-24: DomainNameFields
	// 24-32: WorkstationFields
	// 32-40: Version
	//   40-: Payload

	off := 32 + 8

	nmsg = make([]byte, off)

	copy(nmsg[:8], signature)
	le.PutUint32(nmsg[8:12], NtLmNegotiate)
	le.PutUint32(nmsg[12:16], defaultFlags)

	copy(nmsg[32:], version)

	c.nmsg = nmsg

	return nmsg, nil
}

func (c *Client) Authenticate(cmsg []byte) (amsg []byte, err error) {
	//        ChallengeMessage
	//   0-8: Signature
	//  8-12: MessageType
	// 12-20: TargetNameFields
	// 20-24: NegotiateFlags
	// 24-32: ServerChallenge
	// 32-40: _
	// 40-48: TargetInfoFields
	// 48-56: Version
	//   56-: Payload

	if len(cmsg) < 48 {
		return nil, errors.New("message length is too short")
	}

	if !bytes.Equal(cmsg[:8], signature) {
		return nil, errors.New("invalid signature")
	}

	if le.Uint32(cmsg[8:12]) != NtLmChallenge {
		return nil, errors.New("invalid message type")
	}

	flags := le.Uint32(c.nmsg[12:16]) & le.Uint32(cmsg[20:24])

	if flags&NTLMSSP_REQUEST_TARGET == 0 {
		return nil, errors.New("invalid negotiate flags")
	}

	targetNameLen := le.Uint16(cmsg[12:14])    // cmsg.TargetNameLen
	targetNameMaxLen := le.Uint16(cmsg[14:16]) // cmsg.TargetNameMaxLen
	if targetNameMaxLen < targetNameLen {
		return nil, errors.New("invalid target name format")
	}
	targetNameBufferOffset := le.Uint32(cmsg[16:20]) // cmsg.TargetNameBufferOffset
	if len(cmsg) < int(targetNameBufferOffset+uint32(targetNameLen)) {
		return nil, errors.New("invalid target name format")
	}
	targetName := cmsg[targetNameBufferOffset : targetNameBufferOffset+uint32(targetNameLen)] // cmsg.TargetName

	if flags&NTLMSSP_NEGOTIATE_TARGET_INFO == 0 {
		return nil, errors.New("invalid negotiate flags")
	}

	targetInfoLen := le.Uint16(cmsg[40:42])    // cmsg.TargetInfoLen
	targetInfoMaxLen := le.Uint16(cmsg[42:44]) // cmsg.TargetInfoMaxLen
	if targetInfoMaxLen < targetInfoLen {
		return nil, errors.New("invalid target info format")
	}
	targetInfoBufferOffset := le.Uint32(cmsg[44:48]) // cmsg.TargetInfoBufferOffset
	if len(cmsg) < int(targetInfoBufferOffset+uint32(targetInfoLen)) {
		return nil, errors.New("invalid target info format")
	}
	targetInfo := cmsg[targetInfoBufferOffset : targetInfoBufferOffset+uint32(targetInfoLen)] // cmsg.TargetInfo
	info := newTargetInfoEncoder(targetInfo, utf16le.EncodeStringToBytes(c.TargetSPN))
	if info == nil {
		return nil, errors.New("invalid target info format")
	}

	//        AuthenticateMessage
	//   0-8: Signature
	//  8-12: MessageType
	// 12-20: LmChallengeResponseFields
	// 20-28: NtChallengeResponseFields
	// 28-36: DomainNameFields
	// 36-44: UserNameFields
	// 44-52: WorkstationFields
	// 52-60: EncryptedRandomSessionKeyFields
	// 60-64: NegotiateFlags
	// 64-72: Version
	// 72-88: MIC
	//   88-: Payload

	off := 64 + 8 + 16

	domain := utf16le.EncodeStringToBytes(c.Domain)
	user := utf16le.EncodeStringToBytes(c.User)
	workstation := utf16le.EncodeStringToBytes(c.Workstation)

	if domain == nil {
		domain = targetName
	}

	// LmChallengeResponseLen = 24
	// NtChallengeResponseLen =
	//   len(Response) = 16
	//	 len(NTLMv2ClientChallenge) =
	//     min len size = 28
	//     target info size
	//     padding = 4
	// len(EncryptedRandomSessionKey) = 0 or 16

	amsg = make([]byte, off+len(domain)+len(user)+len(workstation)+
		24+
		(16+(28+info.size()+4))+
		16)

	copy(amsg[:8], signature)
	le.PutUint32(amsg[8:12], NtLmAuthenticate)

	if domain != nil {
		len := copy(amsg[off:], domain)
		le.PutUint16(amsg[28:30], uint16(len))
		le.PutUint16(amsg[30:32], uint16(len))
		le.PutUint32(amsg[32:36], uint32(off))
		off += len
	}

	if user != nil {
		len := copy(amsg[off:], user)
		le.PutUint16(amsg[36:38], uint16(len))
		le.PutUint16(amsg[38:40], uint16(len))
		le.PutUint32(amsg[40:44], uint32(off))
		off += len
	}

	if workstation != nil {
		len := copy(amsg[off:], workstation)
		le.PutUint16(amsg[44:46], uint16(len))
		le.PutUint16(amsg[46:48], uint16(len))
		le.PutUint32(amsg[48:52], uint32(off))
		off += len
	}

	if c.User != "" || c.Password != "" || c.Hash != nil {
		var err error
		var h hash.Hash

		if c.Hash != nil {
			USER := utf16le.EncodeStringToBytes(strings.ToUpper(c.User))

			h = hmac.New(md5.New, ntowfv2Hash(USER, c.Hash, domain))
		} else {
			USER := utf16le.EncodeStringToBytes(strings.ToUpper(c.User))
			password := utf16le.EncodeStringToBytes(c.Password)

			h = hmac.New(md5.New, ntowfv2(USER, password, domain))
		}

		//        LMv2Response
		//  0-16: Response
		// 16-24: ChallengeFromClient

		lmChallengeResponse := amsg[off : off+24]
		{
			le.PutUint16(amsg[12:14], uint16(len(lmChallengeResponse)))
			le.PutUint16(amsg[14:16], uint16(len(lmChallengeResponse)))
			le.PutUint32(amsg[16:20], uint32(off))

			off += 24
		}

		//        NTLMv2Response
		//  0-16: Response
		//   16-: NTLMv2ClientChallenge

		ntChallengeResponse := amsg[off : len(amsg)-16]
		{
			ntlmv2ClientChallenge := ntChallengeResponse[16:]

			//        NTLMv2ClientChallenge
			//   0-1: RespType
			//   1-2: HiRespType
			//   2-4: _
			//   4-8: _
			//  8-16: TimeStamp
			// 16-24: ChallengeFromClient
			// 24-28: _
			//   28-: AvPairs

			serverChallenge := cmsg[24:32]

			clientChallenge := ntlmv2ClientChallenge[16:24]

			_, err := rand.Read(clientChallenge)
			if err != nil {
				return nil, err
			}

			timeStamp, ok := info.InfoMap[MsvAvTimestamp]
			if !ok {
				timeStamp = ntlmv2ClientChallenge[8:16]
				le.PutUint64(timeStamp, uint64((time.Now().UnixNano()/100)+116444736000000000))
			}

			encodeNtlmv2Response(ntChallengeResponse, h, serverChallenge, clientChallenge, timeStamp, info)

			le.PutUint16(amsg[20:22], uint16(len(ntChallengeResponse)))
			le.PutUint16(amsg[22:24], uint16(len(ntChallengeResponse)))
			le.PutUint32(amsg[24:28], uint32(off))

			off = len(amsg) - 16
		}

		session := new(Session)

		session.isClientSide = true

		session.user = c.User
		session.negotiateFlags = flags
		session.infoMap = info.InfoMap

		h.Reset()
		h.Write(ntChallengeResponse[:16])
		sessionBaseKey := h.Sum(nil)

		keyExchangeKey := sessionBaseKey // if ntlm version == 2

		if flags&NTLMSSP_NEGOTIATE_KEY_EXCH != 0 {
			session.exportedSessionKey = make([]byte, 16)
			_, err := rand.Read(session.exportedSessionKey)
			if err != nil {
				return nil, err
			}
			cipher, err := rc4.NewCipher(keyExchangeKey)
			if err != nil {
				return nil, err
			}
			encryptedRandomSessionKey := amsg[off:]
			cipher.XORKeyStream(encryptedRandomSessionKey, session.exportedSessionKey)

			le.PutUint16(amsg[52:54], 16)          // amsg.EncryptedRandomSessionKeyLen
			le.PutUint16(amsg[54:56], 16)          // amsg.EncryptedRandomSessionKeyMaxLen
			le.PutUint32(amsg[56:60], uint32(off)) // amsg.EncryptedRandomSessionKeyBufferOffset
		} else {
			session.exportedSessionKey = keyExchangeKey
		}

		le.PutUint32(amsg[60:64], flags)

		copy(amsg[64:], version)
		h = hmac.New(md5.New, session.exportedSessionKey)
		h.Write(c.nmsg)
		h.Write(cmsg)
		h.Write(amsg)
		h.Sum(amsg[:72]) // amsg.MIC

		{
			session.clientSigningKey = signKey(flags, session.exportedSessionKey, true)
			session.serverSigningKey = signKey(flags, session.exportedSessionKey, false)

			session.clientHandle, err = rc4.NewCipher(sealKey(flags, session.exportedSessionKey, true))
			if err != nil {
				return nil, err
			}

			session.serverHandle, err = rc4.NewCipher(sealKey(flags, session.exportedSessionKey, false))
			if err != nil {
				return nil, err
			}
		}

		c.session = session
	}

	return amsg, nil
}

func (c *Client) Session() *Session {
	return c.session
}
