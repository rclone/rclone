package ntlm

import (
	"bytes"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/rc4"
	"errors"
	"strings"

	"github.com/hirochachacha/go-smb2/internal/utf16le"
)

// NTLM v2 server
type Server struct {
	targetName string
	accounts   map[string]string // User: Password

	nmsg    []byte
	cmsg    []byte
	session *Session
}

func NewServer(targetName string) *Server {
	return &Server{
		targetName: targetName,
		accounts:   make(map[string]string),
	}
}

func (s *Server) AddAccount(user, password string) {
	s.accounts[user] = password
}

func (s *Server) Challenge(nmsg []byte) (cmsg []byte, err error) {
	//        NegotiateMessage
	//   0-8: Signature
	//  8-12: MessageType
	// 12-16: NegotiateFlags
	// 16-24: DomainNameFields
	// 24-32: WorkstationFields
	// 32-40: Version
	//   40-: Payload

	s.nmsg = nmsg

	if len(nmsg) < 32 {
		return nil, errors.New("message length is too short")
	}

	if !bytes.Equal(nmsg[:8], signature) {
		return nil, errors.New("invalid signature")
	}

	if le.Uint32(nmsg[8:12]) != NtLmNegotiate {
		return nil, errors.New("invalid message type")
	}

	flags := le.Uint32(nmsg[12:16]) & defaultFlags

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

	off := 48

	if flags&NTLMSSP_NEGOTIATE_VERSION != 0 {
		off += 8
	}

	targetName := utf16le.EncodeStringToBytes(s.targetName)

	cmsg = make([]byte, off+len(targetName)+4)

	copy(cmsg[:8], signature)
	le.PutUint32(cmsg[8:12], NtLmChallenge)
	le.PutUint32(cmsg[20:24], flags)

	if targetName != nil && flags&NTLMSSP_REQUEST_TARGET != 0 {
		len := copy(cmsg[off:], targetName)
		le.PutUint16(cmsg[12:14], uint16(len))
		le.PutUint16(cmsg[14:16], uint16(len))
		le.PutUint32(cmsg[16:20], uint32(off))
		off += len
	}

	if flags&NTLMSSP_NEGOTIATE_TARGET_INFO != 0 {
		len := copy(cmsg[off:], []byte{0x00, 0x00, 0x00, 0x00}) // AvId: MsvAvEOL, AvLen: 0
		le.PutUint16(cmsg[40:42], uint16(len))
		le.PutUint16(cmsg[42:44], uint16(len))
		le.PutUint32(cmsg[44:48], uint32(off))
		off += len
	}

	_, err = rand.Read(cmsg[24:32])
	if err != nil {
		return nil, err
	}

	if flags&NTLMSSP_NEGOTIATE_VERSION != 0 {
		copy(cmsg[48:56], version)
	}

	s.cmsg = cmsg

	return cmsg, nil
}

func (s *Server) Authenticate(amsg []byte) (err error) {
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

	if len(amsg) < 64 {
		return errors.New("message length is too short")
	}

	if !bytes.Equal(amsg[:8], signature) {
		return errors.New("invalid signature")
	}

	if le.Uint32(amsg[8:12]) != NtLmAuthenticate {
		return errors.New("invalid message type")
	}

	flags := le.Uint32(amsg[60:64])

	ntChallengeResponseLen := le.Uint16(amsg[20:22])    // amsg.NtChallengeResponseLen
	ntChallengeResponseMaxLen := le.Uint16(amsg[22:24]) // amsg.NtChallengeResponseMaxLen
	if ntChallengeResponseMaxLen < ntChallengeResponseLen {
		return errors.New("invalid LM challenge format")
	}
	ntChallengeResponseBufferOffset := le.Uint32(amsg[24:28]) // amsg.NtChallengeResponseBufferOffset
	if len(amsg) < int(ntChallengeResponseBufferOffset+uint32(ntChallengeResponseLen)) {
		return errors.New("invalid LM challenge format")
	}
	ntChallengeResponse := amsg[ntChallengeResponseBufferOffset : ntChallengeResponseBufferOffset+uint32(ntChallengeResponseLen)] // amsg.NtChallengeResponse

	domainNameLen := le.Uint16(amsg[28:30])    // amsg.DomainNameLen
	domainNameMaxLen := le.Uint16(amsg[30:32]) // amsg.DomainNameMaxLen
	if domainNameMaxLen < domainNameLen {
		return errors.New("invalid domain name format")
	}
	domainNameBufferOffset := le.Uint32(amsg[32:36]) // amsg.DomainNameBufferOffset
	if len(amsg) < int(domainNameBufferOffset+uint32(domainNameLen)) {
		return errors.New("invalid domain name format")
	}
	domainName := amsg[domainNameBufferOffset : domainNameBufferOffset+uint32(domainNameLen)] // amsg.DomainName

	userNameLen := le.Uint16(amsg[36:38])    // amsg.UserNameLen
	userNameMaxLen := le.Uint16(amsg[38:40]) // amsg.UserNameMaxLen
	if userNameMaxLen < userNameLen {
		return errors.New("invalid user name format")
	}
	userNameBufferOffset := le.Uint32(amsg[40:44]) // amsg.UserNameBufferOffset
	if len(amsg) < int(userNameBufferOffset+uint32(userNameLen)) {
		return errors.New("invalid user name format")
	}
	userName := amsg[userNameBufferOffset : userNameBufferOffset+uint32(userNameLen)] // amsg.UserName

	encryptedRandomSessionKeyLen := le.Uint16(amsg[52:54])    // amsg.EncryptedRandomSessionKeyLen
	encryptedRandomSessionKeyMaxLen := le.Uint16(amsg[54:56]) // amsg.EncryptedRandomSessionKeyMaxLen
	if encryptedRandomSessionKeyMaxLen < encryptedRandomSessionKeyLen {
		return errors.New("invalid user name format")
	}
	encryptedRandomSessionKeyBufferOffset := le.Uint32(amsg[56:60]) // amsg.EncryptedRandomSessionKeyBufferOffset
	if len(amsg) < int(encryptedRandomSessionKeyBufferOffset+uint32(encryptedRandomSessionKeyLen)) {
		return errors.New("invalid user name format")
	}
	encryptedRandomSessionKey := amsg[encryptedRandomSessionKeyBufferOffset : encryptedRandomSessionKeyBufferOffset+uint32(encryptedRandomSessionKeyLen)] // amsg.EncryptedRandomSessionKey

	if len(userName) != 0 || len(ntChallengeResponse) != 0 {
		user := utf16le.DecodeToString(userName)
		expectedNtChallengeResponse := make([]byte, len(ntChallengeResponse))
		ntlmv2ClientChallenge := ntChallengeResponse[16:]
		USER := utf16le.EncodeStringToBytes(strings.ToUpper(user))
		password := utf16le.EncodeStringToBytes(s.accounts[user])
		h := hmac.New(md5.New, ntowfv2(USER, password, domainName))
		serverChallenge := s.cmsg[24:32]
		timeStamp := ntlmv2ClientChallenge[8:16]
		clientChallenge := ntlmv2ClientChallenge[16:24]
		targetInfo := ntlmv2ClientChallenge[28:]
		encodeNtlmv2Response(expectedNtChallengeResponse, h, serverChallenge, clientChallenge, timeStamp, bytesEncoder(targetInfo))
		if !bytes.Equal(ntChallengeResponse, expectedNtChallengeResponse) {
			return errors.New("login failure")
		}

		session := new(Session)

		session.isClientSide = false

		session.user = user
		session.negotiateFlags = flags

		h.Reset()
		h.Write(ntChallengeResponse[:16])
		sessionBaseKey := h.Sum(nil)

		keyExchangeKey := sessionBaseKey // if ntlm version == 2

		if flags&NTLMSSP_NEGOTIATE_KEY_EXCH != 0 {
			session.exportedSessionKey = make([]byte, 16)
			cipher, err := rc4.NewCipher(keyExchangeKey)
			if err != nil {
				return err
			}
			cipher.XORKeyStream(session.exportedSessionKey, encryptedRandomSessionKey)
		} else {
			session.exportedSessionKey = keyExchangeKey
		}

		if infoMap, ok := parseAvPairs(targetInfo); ok {
			if avFlags, ok := infoMap[MsvAvFlags]; ok && le.Uint32(avFlags)&0x02 != 0 {
				MIC := make([]byte, 16)
				if flags&NTLMSSP_NEGOTIATE_VERSION != 0 {
					copy(MIC, amsg[72:88])
					copy(amsg[72:88], zero[:])
				} else {
					copy(MIC, amsg[64:80])
					copy(amsg[64:80], zero[:])
				}
				h = hmac.New(md5.New, session.exportedSessionKey)
				h.Write(s.nmsg)
				h.Write(s.cmsg)
				h.Write(amsg)
				if !bytes.Equal(MIC, h.Sum(nil)) {
					return errors.New("login failure")
				}
			}
		}

		{
			session.clientSigningKey = signKey(flags, session.exportedSessionKey, true)
			session.serverSigningKey = signKey(flags, session.exportedSessionKey, false)

			session.clientHandle, err = rc4.NewCipher(sealKey(flags, session.exportedSessionKey, true))
			if err != nil {
				return err
			}

			session.serverHandle, err = rc4.NewCipher(sealKey(flags, session.exportedSessionKey, false))
			if err != nil {
				return err
			}
		}

		s.session = session

		return nil
	}

	return errors.New("credential is empty")
}

func (s *Server) Session() *Session {
	return s.session
}
