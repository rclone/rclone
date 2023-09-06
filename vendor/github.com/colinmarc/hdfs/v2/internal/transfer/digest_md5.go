package transfer

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/colinmarc/hdfs/v2/internal/sasl"
)

const (
	saslIntegrityPrefixLength = 4
	macDataLen                = 4
	macHMACLen                = 10
	macMsgTypeLen             = 2
	macSeqNumLen              = 4
)

var macMsgType = [2]byte{0x00, 0x01}

type digestMD5Conn interface {
	net.Conn
	decode(input []byte) ([]byte, error)
}

// digestMD5Handshake represents the negotiation state in a token-digestmd5
// authentication flow.
type digestMD5Handshake struct {
	authID   []byte
	passwd   string
	hostname string
	service  string

	token *sasl.Challenge

	cnonce string
	cipher string
}

// challengeStep1 implements step one of RFC 2831.
func (d *digestMD5Handshake) challengeStep1(challenge []byte) ([]byte, error) {
	var err error
	d.token, err = sasl.ParseChallenge(challenge)
	if err != nil {
		return nil, err
	}

	d.cnonce, err = genCnonce()
	if err != nil {
		return nil, err
	}

	d.cipher = chooseCipher(d.token.Cipher)
	rspdigest := d.compute(true)

	ret := fmt.Sprintf(`username="%s", realm="%s", nonce="%s", cnonce="%s", nc=%08x, qop=%s, digest-uri="%s/%s", response=%s, charset=utf-8`,
		d.authID, d.token.Realm, d.token.Nonce, d.cnonce, 1, d.token.Qop[0], d.service, d.hostname, rspdigest)

	if d.cipher != "" {
		ret += ", cipher=" + d.cipher
	}

	return []byte(ret), nil
}

// challengeStep2 implements step two of RFC 2831.
func (d *digestMD5Handshake) challengeStep2(challenge []byte) error {
	rspauth := strings.Split(string(challenge), "=")

	if rspauth[0] != "rspauth" {
		return fmt.Errorf("rspauth not in '%s'", string(challenge))
	}

	if rspauth[1] != d.compute(false) {
		return errors.New("rspauth did not match digest")
	}

	return nil
}

// compute implements the computation of md5 digest authentication per RFC 2831.
// The response value computation is defined as:
//
//     HEX(KD(HEX(H(A1)),
//       { nonce-value, ":", nc-value, ":", cnonce-value, ":", qop-value,
//         ":", HEX(H(A2)) }))
//     A1 = { H({ username-value, ":", realm-value, ":", passwd }),
//            ":", nonce-value, ":", cnonce-value }
//
//   If "qop" is "auth":
//
//		 A2 = { "AUTHENTICATE:", digest-uri-value }
//
//   If "qop" is "auth-int" or "auth-conf":
//
//       A2 = { "AUTHENTICATE:", digest-uri-value,
//              ":00000000000000000000000000000000" }
//
//   Where:
//
//     - { a, b, ... } is the concatenation of the octet strings a, b, ...
//     - H(s) is the 16 octet MD5 Hash [RFC1321] of the octet string s
//     - KD(k, s) is H({k, ":", s})
//     - HEX(n) is the representation of the 16 octet MD5 hash n as a string of
//       32 hex digits (with alphabetic characters in lower case)
func (d *digestMD5Handshake) compute(initial bool) string {
	x := hex.EncodeToString(h(d.a1()))
	y := strings.Join([]string{
		d.token.Nonce,
		fmt.Sprintf("%08x", 1),
		d.cnonce,
		d.token.Qop[0],
		hex.EncodeToString(h(d.a2(initial))),
	}, ":")
	return hex.EncodeToString(kd(x, y))
}

func (d *digestMD5Handshake) a1() string {
	x := h(strings.Join([]string{string(d.authID), d.token.Realm, d.passwd}, ":"))
	return strings.Join([]string{string(x[:]), d.token.Nonce, d.cnonce}, ":")

}

func (d *digestMD5Handshake) a2(initial bool) string {
	digestURI := d.service + "/" + d.hostname
	var a2 string

	// When validating the server's response-auth, we need to leave out the
	// 'AUTHENTICATE:' prefix.
	if initial {
		a2 = strings.Join([]string{"AUTHENTICATE", digestURI}, ":")
	} else {
		a2 = ":" + digestURI
	}

	if d.token.Qop[0] == sasl.QopPrivacy || d.token.Qop[0] == sasl.QopIntegrity {
		a2 = a2 + ":00000000000000000000000000000000"
	}

	return a2
}

// Defined this way for testing.
var genCnonce = func() (string, error) {
	ret := make([]byte, 12)
	if _, err := rand.Read(ret); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(ret), nil
}

func h(s string) []byte {
	hash := md5.Sum([]byte(s))
	return hash[:]
}

func kd(k, s string) []byte {
	return h(k + ":" + s)
}

func generateIntegrityKeys(a1 string) ([]byte, []byte) {
	clientIntMagicStr := []byte("Digest session key to client-to-server signing key magic constant")
	serverIntMagicStr := []byte("Digest session key to server-to-client signing key magic constant")

	sum := h(a1)
	kic := md5.Sum(append(sum[:], clientIntMagicStr...))
	kis := md5.Sum(append(sum[:], serverIntMagicStr...))

	return kic[:], kis[:]
}

func generatePrivacyKeys(a1 string, cipher string) ([]byte, []byte) {
	sum := h(a1)
	var n int
	switch cipher {
	case "rc4-40":
		n = 5
	case "rc4-56":
		n = 7
	default:
		n = md5.Size
	}

	kcc := md5.Sum(append(sum[:n],
		[]byte("Digest H(A1) to client-to-server sealing key magic constant")...))
	kcs := md5.Sum(append(sum[:n],
		[]byte("Digest H(A1) to server-to-client sealing key magic constant")...))

	return kcc[:], kcs[:]
}

func chooseCipher(options []string) string {
	s := make(map[string]bool)
	for _, c := range options {
		s[c] = true
	}

	// TODO: Support 3DES

	switch {
	case s["rc4"]:
		return "rc4"
	case s["rc4-56"]:
		return "rc4-56"
	case s["rc4-40"]:
		return "rc4-40"
	default:
		return ""
	}
}

func lenEncodeBytes(seqnum int) (out [4]byte) {
	out[0] = byte((seqnum >> 24) & 0xFF)
	out[1] = byte((seqnum >> 16) & 0xFF)
	out[2] = byte((seqnum >> 8) & 0xFF)
	out[3] = byte(seqnum & 0xFF)
	return
}
