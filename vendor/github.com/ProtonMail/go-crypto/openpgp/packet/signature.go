// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package packet

import (
	"bytes"
	"crypto"
	"crypto/dsa"
	"encoding/binary"
	"hash"
	"io"
	"strconv"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp/ecdsa"
	"github.com/ProtonMail/go-crypto/openpgp/eddsa"
	"github.com/ProtonMail/go-crypto/openpgp/errors"
	"github.com/ProtonMail/go-crypto/openpgp/internal/algorithm"
	"github.com/ProtonMail/go-crypto/openpgp/internal/encoding"
)

const (
	// See RFC 4880, section 5.2.3.21 for details.
	KeyFlagCertify = 1 << iota
	KeyFlagSign
	KeyFlagEncryptCommunications
	KeyFlagEncryptStorage
	KeyFlagSplitKey
	KeyFlagAuthenticate
	_
	KeyFlagGroupKey
)

// Signature represents a signature. See RFC 4880, section 5.2.
type Signature struct {
	Version    int
	SigType    SignatureType
	PubKeyAlgo PublicKeyAlgorithm
	Hash       crypto.Hash

	// HashSuffix is extra data that is hashed in after the signed data.
	HashSuffix []byte
	// HashTag contains the first two bytes of the hash for fast rejection
	// of bad signed data.
	HashTag [2]byte

	// Metadata includes format, filename and time, and is protected by v5
	// signatures of type 0x00 or 0x01. This metadata is included into the hash
	// computation; if nil, six 0x00 bytes are used instead. See section 5.2.4.
	Metadata *LiteralData

	CreationTime time.Time

	RSASignature         encoding.Field
	DSASigR, DSASigS     encoding.Field
	ECDSASigR, ECDSASigS encoding.Field
	EdDSASigR, EdDSASigS encoding.Field

	// rawSubpackets contains the unparsed subpackets, in order.
	rawSubpackets []outputSubpacket

	// The following are optional so are nil when not included in the
	// signature.

	SigLifetimeSecs, KeyLifetimeSecs                        *uint32
	PreferredSymmetric, PreferredHash, PreferredCompression []uint8
	PreferredCipherSuites                                   [][2]uint8
	IssuerKeyId                                             *uint64
	IssuerFingerprint                                       []byte
	SignerUserId                                            *string
	IsPrimaryId                                             *bool
	Notations                                               []*Notation

	// TrustLevel and TrustAmount can be set by the signer to assert that
	// the key is not only valid but also trustworthy at the specified
	// level.
	// See RFC 4880, section 5.2.3.13 for details.
	TrustLevel  TrustLevel
	TrustAmount TrustAmount

	// TrustRegularExpression can be used in conjunction with trust Signature
	// packets to limit the scope of the trust that is extended.
	// See RFC 4880, section 5.2.3.14 for details.
	TrustRegularExpression *string

	// PolicyURI can be set to the URI of a document that describes the
	// policy under which the signature was issued. See RFC 4880, section
	// 5.2.3.20 for details.
	PolicyURI string

	// FlagsValid is set if any flags were given. See RFC 4880, section
	// 5.2.3.21 for details.
	FlagsValid                                                                                                         bool
	FlagCertify, FlagSign, FlagEncryptCommunications, FlagEncryptStorage, FlagSplitKey, FlagAuthenticate, FlagGroupKey bool

	// RevocationReason is set if this signature has been revoked.
	// See RFC 4880, section 5.2.3.23 for details.
	RevocationReason     *ReasonForRevocation
	RevocationReasonText string

	// In a self-signature, these flags are set there is a features subpacket
	// indicating that the issuer implementation supports these features
	// see https://datatracker.ietf.org/doc/html/draft-ietf-openpgp-crypto-refresh#features-subpacket
	SEIPDv1, SEIPDv2 bool

	// EmbeddedSignature, if non-nil, is a signature of the parent key, by
	// this key. This prevents an attacker from claiming another's signing
	// subkey as their own.
	EmbeddedSignature *Signature

	outSubpackets []outputSubpacket
}

func (sig *Signature) parse(r io.Reader) (err error) {
	// RFC 4880, section 5.2.3
	var buf [5]byte
	_, err = readFull(r, buf[:1])
	if err != nil {
		return
	}
	if buf[0] != 4 && buf[0] != 5 {
		err = errors.UnsupportedError("signature packet version " + strconv.Itoa(int(buf[0])))
		return
	}
	sig.Version = int(buf[0])
	_, err = readFull(r, buf[:5])
	if err != nil {
		return
	}
	sig.SigType = SignatureType(buf[0])
	sig.PubKeyAlgo = PublicKeyAlgorithm(buf[1])
	switch sig.PubKeyAlgo {
	case PubKeyAlgoRSA, PubKeyAlgoRSASignOnly, PubKeyAlgoDSA, PubKeyAlgoECDSA, PubKeyAlgoEdDSA:
	default:
		err = errors.UnsupportedError("public key algorithm " + strconv.Itoa(int(sig.PubKeyAlgo)))
		return
	}

	var ok bool

	if sig.Version < 5 {
		sig.Hash, ok = algorithm.HashIdToHashWithSha1(buf[2])
	} else {
		sig.Hash, ok = algorithm.HashIdToHash(buf[2])
	}

	if !ok {
		return errors.UnsupportedError("hash function " + strconv.Itoa(int(buf[2])))
	}

	hashedSubpacketsLength := int(buf[3])<<8 | int(buf[4])
	hashedSubpackets := make([]byte, hashedSubpacketsLength)
	_, err = readFull(r, hashedSubpackets)
	if err != nil {
		return
	}
	err = sig.buildHashSuffix(hashedSubpackets)
	if err != nil {
		return
	}

	err = parseSignatureSubpackets(sig, hashedSubpackets, true)
	if err != nil {
		return
	}

	_, err = readFull(r, buf[:2])
	if err != nil {
		return
	}
	unhashedSubpacketsLength := int(buf[0])<<8 | int(buf[1])
	unhashedSubpackets := make([]byte, unhashedSubpacketsLength)
	_, err = readFull(r, unhashedSubpackets)
	if err != nil {
		return
	}
	err = parseSignatureSubpackets(sig, unhashedSubpackets, false)
	if err != nil {
		return
	}

	_, err = readFull(r, sig.HashTag[:2])
	if err != nil {
		return
	}

	switch sig.PubKeyAlgo {
	case PubKeyAlgoRSA, PubKeyAlgoRSASignOnly:
		sig.RSASignature = new(encoding.MPI)
		_, err = sig.RSASignature.ReadFrom(r)
	case PubKeyAlgoDSA:
		sig.DSASigR = new(encoding.MPI)
		if _, err = sig.DSASigR.ReadFrom(r); err != nil {
			return
		}

		sig.DSASigS = new(encoding.MPI)
		_, err = sig.DSASigS.ReadFrom(r)
	case PubKeyAlgoECDSA:
		sig.ECDSASigR = new(encoding.MPI)
		if _, err = sig.ECDSASigR.ReadFrom(r); err != nil {
			return
		}

		sig.ECDSASigS = new(encoding.MPI)
		_, err = sig.ECDSASigS.ReadFrom(r)
	case PubKeyAlgoEdDSA:
		sig.EdDSASigR = new(encoding.MPI)
		if _, err = sig.EdDSASigR.ReadFrom(r); err != nil {
			return
		}

		sig.EdDSASigS = new(encoding.MPI)
		if _, err = sig.EdDSASigS.ReadFrom(r); err != nil {
			return
		}
	default:
		panic("unreachable")
	}
	return
}

// parseSignatureSubpackets parses subpackets of the main signature packet. See
// RFC 4880, section 5.2.3.1.
func parseSignatureSubpackets(sig *Signature, subpackets []byte, isHashed bool) (err error) {
	for len(subpackets) > 0 {
		subpackets, err = parseSignatureSubpacket(sig, subpackets, isHashed)
		if err != nil {
			return
		}
	}

	if sig.CreationTime.IsZero() {
		err = errors.StructuralError("no creation time in signature")
	}

	return
}

type signatureSubpacketType uint8

const (
	creationTimeSubpacket        signatureSubpacketType = 2
	signatureExpirationSubpacket signatureSubpacketType = 3
	trustSubpacket               signatureSubpacketType = 5
	regularExpressionSubpacket   signatureSubpacketType = 6
	keyExpirationSubpacket       signatureSubpacketType = 9
	prefSymmetricAlgosSubpacket  signatureSubpacketType = 11
	issuerSubpacket              signatureSubpacketType = 16
	notationDataSubpacket        signatureSubpacketType = 20
	prefHashAlgosSubpacket       signatureSubpacketType = 21
	prefCompressionSubpacket     signatureSubpacketType = 22
	primaryUserIdSubpacket       signatureSubpacketType = 25
	policyUriSubpacket           signatureSubpacketType = 26
	keyFlagsSubpacket            signatureSubpacketType = 27
	signerUserIdSubpacket        signatureSubpacketType = 28
	reasonForRevocationSubpacket signatureSubpacketType = 29
	featuresSubpacket            signatureSubpacketType = 30
	embeddedSignatureSubpacket   signatureSubpacketType = 32
	issuerFingerprintSubpacket   signatureSubpacketType = 33
	prefCipherSuitesSubpacket    signatureSubpacketType = 39
)

// parseSignatureSubpacket parses a single subpacket. len(subpacket) is >= 1.
func parseSignatureSubpacket(sig *Signature, subpacket []byte, isHashed bool) (rest []byte, err error) {
	// RFC 4880, section 5.2.3.1
	var (
		length     uint32
		packetType signatureSubpacketType
		isCritical bool
	)
	if len(subpacket) == 0 {
		err = errors.StructuralError("zero length signature subpacket")
		return
	}
	switch {
	case subpacket[0] < 192:
		length = uint32(subpacket[0])
		subpacket = subpacket[1:]
	case subpacket[0] < 255:
		if len(subpacket) < 2 {
			goto Truncated
		}
		length = uint32(subpacket[0]-192)<<8 + uint32(subpacket[1]) + 192
		subpacket = subpacket[2:]
	default:
		if len(subpacket) < 5 {
			goto Truncated
		}
		length = uint32(subpacket[1])<<24 |
			uint32(subpacket[2])<<16 |
			uint32(subpacket[3])<<8 |
			uint32(subpacket[4])
		subpacket = subpacket[5:]
	}
	if length > uint32(len(subpacket)) {
		goto Truncated
	}
	rest = subpacket[length:]
	subpacket = subpacket[:length]
	if len(subpacket) == 0 {
		err = errors.StructuralError("zero length signature subpacket")
		return
	}
	packetType = signatureSubpacketType(subpacket[0] & 0x7f)
	isCritical = subpacket[0]&0x80 == 0x80
	subpacket = subpacket[1:]
	sig.rawSubpackets = append(sig.rawSubpackets, outputSubpacket{isHashed, packetType, isCritical, subpacket})
	if !isHashed &&
		packetType != issuerSubpacket &&
		packetType != issuerFingerprintSubpacket &&
		packetType != embeddedSignatureSubpacket {
		return
	}
	switch packetType {
	case creationTimeSubpacket:
		if len(subpacket) != 4 {
			err = errors.StructuralError("signature creation time not four bytes")
			return
		}
		t := binary.BigEndian.Uint32(subpacket)
		sig.CreationTime = time.Unix(int64(t), 0)
	case signatureExpirationSubpacket:
		// Signature expiration time, section 5.2.3.10
		if len(subpacket) != 4 {
			err = errors.StructuralError("expiration subpacket with bad length")
			return
		}
		sig.SigLifetimeSecs = new(uint32)
		*sig.SigLifetimeSecs = binary.BigEndian.Uint32(subpacket)
	case trustSubpacket:
		if len(subpacket) != 2 {
			err = errors.StructuralError("trust subpacket with bad length")
			return
		}
		// Trust level and amount, section 5.2.3.13
		sig.TrustLevel = TrustLevel(subpacket[0])
		sig.TrustAmount = TrustAmount(subpacket[1])
	case regularExpressionSubpacket:
		if len(subpacket) == 0 {
			err = errors.StructuralError("regexp subpacket with bad length")
			return
		}
		// Trust regular expression, section 5.2.3.14
		// RFC specifies the string should be null-terminated; remove a null byte from the end
		if subpacket[len(subpacket)-1] != 0x00 {
			err = errors.StructuralError("expected regular expression to be null-terminated")
			return
		}
		trustRegularExpression := string(subpacket[:len(subpacket)-1])
		sig.TrustRegularExpression = &trustRegularExpression
	case keyExpirationSubpacket:
		// Key expiration time, section 5.2.3.6
		if len(subpacket) != 4 {
			err = errors.StructuralError("key expiration subpacket with bad length")
			return
		}
		sig.KeyLifetimeSecs = new(uint32)
		*sig.KeyLifetimeSecs = binary.BigEndian.Uint32(subpacket)
	case prefSymmetricAlgosSubpacket:
		// Preferred symmetric algorithms, section 5.2.3.7
		sig.PreferredSymmetric = make([]byte, len(subpacket))
		copy(sig.PreferredSymmetric, subpacket)
	case issuerSubpacket:
		// Issuer, section 5.2.3.5
		if sig.Version > 4 {
			err = errors.StructuralError("issuer subpacket found in v5 key")
			return
		}
		if len(subpacket) != 8 {
			err = errors.StructuralError("issuer subpacket with bad length")
			return
		}
		sig.IssuerKeyId = new(uint64)
		*sig.IssuerKeyId = binary.BigEndian.Uint64(subpacket)
	case notationDataSubpacket:
		// Notation data, section 5.2.3.16
		if len(subpacket) < 8 {
			err = errors.StructuralError("notation data subpacket with bad length")
			return
		}

		nameLength := uint32(subpacket[4])<<8 | uint32(subpacket[5])
		valueLength := uint32(subpacket[6])<<8 | uint32(subpacket[7])
		if len(subpacket) != int(nameLength)+int(valueLength)+8 {
			err = errors.StructuralError("notation data subpacket with bad length")
			return
		}

		notation := Notation{
			IsHumanReadable: (subpacket[0] & 0x80) == 0x80,
			Name:            string(subpacket[8:(nameLength + 8)]),
			Value:           subpacket[(nameLength + 8):(valueLength + nameLength + 8)],
			IsCritical:      isCritical,
		}

		sig.Notations = append(sig.Notations, &notation)
	case prefHashAlgosSubpacket:
		// Preferred hash algorithms, section 5.2.3.8
		sig.PreferredHash = make([]byte, len(subpacket))
		copy(sig.PreferredHash, subpacket)
	case prefCompressionSubpacket:
		// Preferred compression algorithms, section 5.2.3.9
		sig.PreferredCompression = make([]byte, len(subpacket))
		copy(sig.PreferredCompression, subpacket)
	case primaryUserIdSubpacket:
		// Primary User ID, section 5.2.3.19
		if len(subpacket) != 1 {
			err = errors.StructuralError("primary user id subpacket with bad length")
			return
		}
		sig.IsPrimaryId = new(bool)
		if subpacket[0] > 0 {
			*sig.IsPrimaryId = true
		}
	case keyFlagsSubpacket:
		// Key flags, section 5.2.3.21
		if len(subpacket) == 0 {
			err = errors.StructuralError("empty key flags subpacket")
			return
		}
		sig.FlagsValid = true
		if subpacket[0]&KeyFlagCertify != 0 {
			sig.FlagCertify = true
		}
		if subpacket[0]&KeyFlagSign != 0 {
			sig.FlagSign = true
		}
		if subpacket[0]&KeyFlagEncryptCommunications != 0 {
			sig.FlagEncryptCommunications = true
		}
		if subpacket[0]&KeyFlagEncryptStorage != 0 {
			sig.FlagEncryptStorage = true
		}
		if subpacket[0]&KeyFlagSplitKey != 0 {
			sig.FlagSplitKey = true
		}
		if subpacket[0]&KeyFlagAuthenticate != 0 {
			sig.FlagAuthenticate = true
		}
		if subpacket[0]&KeyFlagGroupKey != 0 {
			sig.FlagGroupKey = true
		}
	case signerUserIdSubpacket:
		userId := string(subpacket)
		sig.SignerUserId = &userId
	case reasonForRevocationSubpacket:
		// Reason For Revocation, section 5.2.3.23
		if len(subpacket) == 0 {
			err = errors.StructuralError("empty revocation reason subpacket")
			return
		}
		sig.RevocationReason = new(ReasonForRevocation)
		*sig.RevocationReason = ReasonForRevocation(subpacket[0])
		sig.RevocationReasonText = string(subpacket[1:])
	case featuresSubpacket:
		// Features subpacket, section 5.2.3.24 specifies a very general
		// mechanism for OpenPGP implementations to signal support for new
		// features.
		if len(subpacket) > 0 {
			if subpacket[0]&0x01 != 0 {
				sig.SEIPDv1 = true
			}
			// 0x02 and 0x04 are reserved
			if subpacket[0]&0x08 != 0 {
				sig.SEIPDv2 = true
			}
		}
	case embeddedSignatureSubpacket:
		// Only usage is in signatures that cross-certify
		// signing subkeys. section 5.2.3.26 describes the
		// format, with its usage described in section 11.1
		if sig.EmbeddedSignature != nil {
			err = errors.StructuralError("Cannot have multiple embedded signatures")
			return
		}
		sig.EmbeddedSignature = new(Signature)
		// Embedded signatures are required to be v4 signatures see
		// section 12.1. However, we only parse v4 signatures in this
		// file anyway.
		if err := sig.EmbeddedSignature.parse(bytes.NewBuffer(subpacket)); err != nil {
			return nil, err
		}
		if sigType := sig.EmbeddedSignature.SigType; sigType != SigTypePrimaryKeyBinding {
			return nil, errors.StructuralError("cross-signature has unexpected type " + strconv.Itoa(int(sigType)))
		}
	case policyUriSubpacket:
		// Policy URI, section 5.2.3.20
		sig.PolicyURI = string(subpacket)
	case issuerFingerprintSubpacket:
		if len(subpacket) == 0 {
			err = errors.StructuralError("empty issuer fingerprint subpacket")
			return
		}
		v, l := subpacket[0], len(subpacket[1:])
		if v == 5 && l != 32 || v != 5 && l != 20 {
			return nil, errors.StructuralError("bad fingerprint length")
		}
		sig.IssuerFingerprint = make([]byte, l)
		copy(sig.IssuerFingerprint, subpacket[1:])
		sig.IssuerKeyId = new(uint64)
		if v == 5 {
			*sig.IssuerKeyId = binary.BigEndian.Uint64(subpacket[1:9])
		} else {
			*sig.IssuerKeyId = binary.BigEndian.Uint64(subpacket[13:21])
		}
	case prefCipherSuitesSubpacket:
		// Preferred AEAD cipher suites
		// See https://www.ietf.org/archive/id/draft-ietf-openpgp-crypto-refresh-07.html#name-preferred-aead-ciphersuites
		if len(subpacket)%2 != 0 {
			err = errors.StructuralError("invalid aead cipher suite length")
			return
		}

		sig.PreferredCipherSuites = make([][2]byte, len(subpacket)/2)

		for i := 0; i < len(subpacket)/2; i++ {
			sig.PreferredCipherSuites[i] = [2]uint8{subpacket[2*i], subpacket[2*i+1]}
		}
	default:
		if isCritical {
			err = errors.UnsupportedError("unknown critical signature subpacket type " + strconv.Itoa(int(packetType)))
			return
		}
	}
	return

Truncated:
	err = errors.StructuralError("signature subpacket truncated")
	return
}

// subpacketLengthLength returns the length, in bytes, of an encoded length value.
func subpacketLengthLength(length int) int {
	if length < 192 {
		return 1
	}
	if length < 16320 {
		return 2
	}
	return 5
}

func (sig *Signature) CheckKeyIdOrFingerprint(pk *PublicKey) bool {
	if sig.IssuerFingerprint != nil && len(sig.IssuerFingerprint) >= 20 {
		return bytes.Equal(sig.IssuerFingerprint, pk.Fingerprint)
	}
	return sig.IssuerKeyId != nil && *sig.IssuerKeyId == pk.KeyId
}

// serializeSubpacketLength marshals the given length into to.
func serializeSubpacketLength(to []byte, length int) int {
	// RFC 4880, Section 4.2.2.
	if length < 192 {
		to[0] = byte(length)
		return 1
	}
	if length < 16320 {
		length -= 192
		to[0] = byte((length >> 8) + 192)
		to[1] = byte(length)
		return 2
	}
	to[0] = 255
	to[1] = byte(length >> 24)
	to[2] = byte(length >> 16)
	to[3] = byte(length >> 8)
	to[4] = byte(length)
	return 5
}

// subpacketsLength returns the serialized length, in bytes, of the given
// subpackets.
func subpacketsLength(subpackets []outputSubpacket, hashed bool) (length int) {
	for _, subpacket := range subpackets {
		if subpacket.hashed == hashed {
			length += subpacketLengthLength(len(subpacket.contents) + 1)
			length += 1 // type byte
			length += len(subpacket.contents)
		}
	}
	return
}

// serializeSubpackets marshals the given subpackets into to.
func serializeSubpackets(to []byte, subpackets []outputSubpacket, hashed bool) {
	for _, subpacket := range subpackets {
		if subpacket.hashed == hashed {
			n := serializeSubpacketLength(to, len(subpacket.contents)+1)
			to[n] = byte(subpacket.subpacketType)
			if subpacket.isCritical {
				to[n] |= 0x80
			}
			to = to[1+n:]
			n = copy(to, subpacket.contents)
			to = to[n:]
		}
	}
	return
}

// SigExpired returns whether sig is a signature that has expired or is created
// in the future.
func (sig *Signature) SigExpired(currentTime time.Time) bool {
	if sig.CreationTime.After(currentTime) {
		return true
	}
	if sig.SigLifetimeSecs == nil || *sig.SigLifetimeSecs == 0 {
		return false
	}
	expiry := sig.CreationTime.Add(time.Duration(*sig.SigLifetimeSecs) * time.Second)
	return currentTime.After(expiry)
}

// buildHashSuffix constructs the HashSuffix member of sig in preparation for signing.
func (sig *Signature) buildHashSuffix(hashedSubpackets []byte) (err error) {
	var hashId byte
	var ok bool

	if sig.Version < 5 {
		hashId, ok = algorithm.HashToHashIdWithSha1(sig.Hash)
	} else {
		hashId, ok = algorithm.HashToHashId(sig.Hash)
	}

	if !ok {
		sig.HashSuffix = nil
		return errors.InvalidArgumentError("hash cannot be represented in OpenPGP: " + strconv.Itoa(int(sig.Hash)))
	}

	hashedFields := bytes.NewBuffer([]byte{
		uint8(sig.Version),
		uint8(sig.SigType),
		uint8(sig.PubKeyAlgo),
		uint8(hashId),
		uint8(len(hashedSubpackets) >> 8),
		uint8(len(hashedSubpackets)),
	})
	hashedFields.Write(hashedSubpackets)

	var l uint64 = uint64(6 + len(hashedSubpackets))
	if sig.Version == 5 {
		hashedFields.Write([]byte{0x05, 0xff})
		hashedFields.Write([]byte{
			uint8(l >> 56), uint8(l >> 48), uint8(l >> 40), uint8(l >> 32),
			uint8(l >> 24), uint8(l >> 16), uint8(l >> 8), uint8(l),
		})
	} else {
		hashedFields.Write([]byte{0x04, 0xff})
		hashedFields.Write([]byte{
			uint8(l >> 24), uint8(l >> 16), uint8(l >> 8), uint8(l),
		})
	}
	sig.HashSuffix = make([]byte, hashedFields.Len())
	copy(sig.HashSuffix, hashedFields.Bytes())
	return
}

func (sig *Signature) signPrepareHash(h hash.Hash) (digest []byte, err error) {
	hashedSubpacketsLen := subpacketsLength(sig.outSubpackets, true)
	hashedSubpackets := make([]byte, hashedSubpacketsLen)
	serializeSubpackets(hashedSubpackets, sig.outSubpackets, true)
	err = sig.buildHashSuffix(hashedSubpackets)
	if err != nil {
		return
	}
	if sig.Version == 5 && (sig.SigType == 0x00 || sig.SigType == 0x01) {
		sig.AddMetadataToHashSuffix()
	}

	h.Write(sig.HashSuffix)
	digest = h.Sum(nil)
	copy(sig.HashTag[:], digest)
	return
}

// Sign signs a message with a private key. The hash, h, must contain
// the hash of the message to be signed and will be mutated by this function.
// On success, the signature is stored in sig. Call Serialize to write it out.
// If config is nil, sensible defaults will be used.
func (sig *Signature) Sign(h hash.Hash, priv *PrivateKey, config *Config) (err error) {
	if priv.Dummy() {
		return errors.ErrDummyPrivateKey("dummy key found")
	}
	sig.Version = priv.PublicKey.Version
	sig.IssuerFingerprint = priv.PublicKey.Fingerprint
	sig.outSubpackets, err = sig.buildSubpackets(priv.PublicKey)
	if err != nil {
		return err
	}
	digest, err := sig.signPrepareHash(h)
	if err != nil {
		return
	}
	switch priv.PubKeyAlgo {
	case PubKeyAlgoRSA, PubKeyAlgoRSASignOnly:
		// supports both *rsa.PrivateKey and crypto.Signer
		sigdata, err := priv.PrivateKey.(crypto.Signer).Sign(config.Random(), digest, sig.Hash)
		if err == nil {
			sig.RSASignature = encoding.NewMPI(sigdata)
		}
	case PubKeyAlgoDSA:
		dsaPriv := priv.PrivateKey.(*dsa.PrivateKey)

		// Need to truncate hashBytes to match FIPS 186-3 section 4.6.
		subgroupSize := (dsaPriv.Q.BitLen() + 7) / 8
		if len(digest) > subgroupSize {
			digest = digest[:subgroupSize]
		}
		r, s, err := dsa.Sign(config.Random(), dsaPriv, digest)
		if err == nil {
			sig.DSASigR = new(encoding.MPI).SetBig(r)
			sig.DSASigS = new(encoding.MPI).SetBig(s)
		}
	case PubKeyAlgoECDSA:
		sk := priv.PrivateKey.(*ecdsa.PrivateKey)
		r, s, err := ecdsa.Sign(config.Random(), sk, digest)

		if err == nil {
			sig.ECDSASigR = new(encoding.MPI).SetBig(r)
			sig.ECDSASigS = new(encoding.MPI).SetBig(s)
		}
	case PubKeyAlgoEdDSA:
		sk := priv.PrivateKey.(*eddsa.PrivateKey)
		r, s, err := eddsa.Sign(sk, digest)
		if err == nil {
			sig.EdDSASigR = encoding.NewMPI(r)
			sig.EdDSASigS = encoding.NewMPI(s)
		}
	default:
		err = errors.UnsupportedError("public key algorithm: " + strconv.Itoa(int(sig.PubKeyAlgo)))
	}

	return
}

// SignUserId computes a signature from priv, asserting that pub is a valid
// key for the identity id.  On success, the signature is stored in sig. Call
// Serialize to write it out.
// If config is nil, sensible defaults will be used.
func (sig *Signature) SignUserId(id string, pub *PublicKey, priv *PrivateKey, config *Config) error {
	if priv.Dummy() {
		return errors.ErrDummyPrivateKey("dummy key found")
	}
	h, err := userIdSignatureHash(id, pub, sig.Hash)
	if err != nil {
		return err
	}
	return sig.Sign(h, priv, config)
}

// CrossSignKey computes a signature from signingKey on pub hashed using hashKey. On success,
// the signature is stored in sig. Call Serialize to write it out.
// If config is nil, sensible defaults will be used.
func (sig *Signature) CrossSignKey(pub *PublicKey, hashKey *PublicKey, signingKey *PrivateKey,
	config *Config) error {
	h, err := keySignatureHash(hashKey, pub, sig.Hash)
	if err != nil {
		return err
	}
	return sig.Sign(h, signingKey, config)
}

// SignKey computes a signature from priv, asserting that pub is a subkey. On
// success, the signature is stored in sig. Call Serialize to write it out.
// If config is nil, sensible defaults will be used.
func (sig *Signature) SignKey(pub *PublicKey, priv *PrivateKey, config *Config) error {
	if priv.Dummy() {
		return errors.ErrDummyPrivateKey("dummy key found")
	}
	h, err := keySignatureHash(&priv.PublicKey, pub, sig.Hash)
	if err != nil {
		return err
	}
	return sig.Sign(h, priv, config)
}

// RevokeKey computes a revocation signature of pub using priv. On success, the signature is
// stored in sig. Call Serialize to write it out.
// If config is nil, sensible defaults will be used.
func (sig *Signature) RevokeKey(pub *PublicKey, priv *PrivateKey, config *Config) error {
	h, err := keyRevocationHash(pub, sig.Hash)
	if err != nil {
		return err
	}
	return sig.Sign(h, priv, config)
}

// RevokeSubkey computes a subkey revocation signature of pub using priv.
// On success, the signature is stored in sig. Call Serialize to write it out.
// If config is nil, sensible defaults will be used.
func (sig *Signature) RevokeSubkey(pub *PublicKey, priv *PrivateKey, config *Config) error {
	// Identical to a subkey binding signature
	return sig.SignKey(pub, priv, config)
}

// Serialize marshals sig to w. Sign, SignUserId or SignKey must have been
// called first.
func (sig *Signature) Serialize(w io.Writer) (err error) {
	if len(sig.outSubpackets) == 0 {
		sig.outSubpackets = sig.rawSubpackets
	}
	if sig.RSASignature == nil && sig.DSASigR == nil && sig.ECDSASigR == nil && sig.EdDSASigR == nil {
		return errors.InvalidArgumentError("Signature: need to call Sign, SignUserId or SignKey before Serialize")
	}

	sigLength := 0
	switch sig.PubKeyAlgo {
	case PubKeyAlgoRSA, PubKeyAlgoRSASignOnly:
		sigLength = int(sig.RSASignature.EncodedLength())
	case PubKeyAlgoDSA:
		sigLength = int(sig.DSASigR.EncodedLength())
		sigLength += int(sig.DSASigS.EncodedLength())
	case PubKeyAlgoECDSA:
		sigLength = int(sig.ECDSASigR.EncodedLength())
		sigLength += int(sig.ECDSASigS.EncodedLength())
	case PubKeyAlgoEdDSA:
		sigLength = int(sig.EdDSASigR.EncodedLength())
		sigLength += int(sig.EdDSASigS.EncodedLength())
	default:
		panic("impossible")
	}

	unhashedSubpacketsLen := subpacketsLength(sig.outSubpackets, false)
	length := len(sig.HashSuffix) - 6 /* trailer not included */ +
		2 /* length of unhashed subpackets */ + unhashedSubpacketsLen +
		2 /* hash tag */ + sigLength
	if sig.Version == 5 {
		length -= 4 // eight-octet instead of four-octet big endian
	}
	err = serializeHeader(w, packetTypeSignature, length)
	if err != nil {
		return
	}
	err = sig.serializeBody(w)
	if err != nil {
		return err
	}
	return
}

func (sig *Signature) serializeBody(w io.Writer) (err error) {
	hashedSubpacketsLen := uint16(uint16(sig.HashSuffix[4])<<8) | uint16(sig.HashSuffix[5])
	fields := sig.HashSuffix[:6+hashedSubpacketsLen]
	_, err = w.Write(fields)
	if err != nil {
		return
	}

	unhashedSubpacketsLen := subpacketsLength(sig.outSubpackets, false)
	unhashedSubpackets := make([]byte, 2+unhashedSubpacketsLen)
	unhashedSubpackets[0] = byte(unhashedSubpacketsLen >> 8)
	unhashedSubpackets[1] = byte(unhashedSubpacketsLen)
	serializeSubpackets(unhashedSubpackets[2:], sig.outSubpackets, false)

	_, err = w.Write(unhashedSubpackets)
	if err != nil {
		return
	}
	_, err = w.Write(sig.HashTag[:])
	if err != nil {
		return
	}

	switch sig.PubKeyAlgo {
	case PubKeyAlgoRSA, PubKeyAlgoRSASignOnly:
		_, err = w.Write(sig.RSASignature.EncodedBytes())
	case PubKeyAlgoDSA:
		if _, err = w.Write(sig.DSASigR.EncodedBytes()); err != nil {
			return
		}
		_, err = w.Write(sig.DSASigS.EncodedBytes())
	case PubKeyAlgoECDSA:
		if _, err = w.Write(sig.ECDSASigR.EncodedBytes()); err != nil {
			return
		}
		_, err = w.Write(sig.ECDSASigS.EncodedBytes())
	case PubKeyAlgoEdDSA:
		if _, err = w.Write(sig.EdDSASigR.EncodedBytes()); err != nil {
			return
		}
		_, err = w.Write(sig.EdDSASigS.EncodedBytes())
	default:
		panic("impossible")
	}
	return
}

// outputSubpacket represents a subpacket to be marshaled.
type outputSubpacket struct {
	hashed        bool // true if this subpacket is in the hashed area.
	subpacketType signatureSubpacketType
	isCritical    bool
	contents      []byte
}

func (sig *Signature) buildSubpackets(issuer PublicKey) (subpackets []outputSubpacket, err error) {
	creationTime := make([]byte, 4)
	binary.BigEndian.PutUint32(creationTime, uint32(sig.CreationTime.Unix()))
	subpackets = append(subpackets, outputSubpacket{true, creationTimeSubpacket, false, creationTime})

	if sig.IssuerKeyId != nil && sig.Version == 4 {
		keyId := make([]byte, 8)
		binary.BigEndian.PutUint64(keyId, *sig.IssuerKeyId)
		subpackets = append(subpackets, outputSubpacket{true, issuerSubpacket, false, keyId})
	}
	if sig.IssuerFingerprint != nil {
		contents := append([]uint8{uint8(issuer.Version)}, sig.IssuerFingerprint...)
		subpackets = append(subpackets, outputSubpacket{true, issuerFingerprintSubpacket, sig.Version == 5, contents})
	}
	if sig.SignerUserId != nil {
		subpackets = append(subpackets, outputSubpacket{true, signerUserIdSubpacket, false, []byte(*sig.SignerUserId)})
	}
	if sig.SigLifetimeSecs != nil && *sig.SigLifetimeSecs != 0 {
		sigLifetime := make([]byte, 4)
		binary.BigEndian.PutUint32(sigLifetime, *sig.SigLifetimeSecs)
		subpackets = append(subpackets, outputSubpacket{true, signatureExpirationSubpacket, true, sigLifetime})
	}

	// Key flags may only appear in self-signatures or certification signatures.

	if sig.FlagsValid {
		var flags byte
		if sig.FlagCertify {
			flags |= KeyFlagCertify
		}
		if sig.FlagSign {
			flags |= KeyFlagSign
		}
		if sig.FlagEncryptCommunications {
			flags |= KeyFlagEncryptCommunications
		}
		if sig.FlagEncryptStorage {
			flags |= KeyFlagEncryptStorage
		}
		if sig.FlagSplitKey {
			flags |= KeyFlagSplitKey
		}
		if sig.FlagAuthenticate {
			flags |= KeyFlagAuthenticate
		}
		if sig.FlagGroupKey {
			flags |= KeyFlagGroupKey
		}
		subpackets = append(subpackets, outputSubpacket{true, keyFlagsSubpacket, false, []byte{flags}})
	}

	for _, notation := range sig.Notations {
		subpackets = append(
			subpackets,
			outputSubpacket{
				true,
				notationDataSubpacket,
				notation.IsCritical,
				notation.getData(),
			})
	}

	// The following subpackets may only appear in self-signatures.

	var features = byte(0x00)
	if sig.SEIPDv1 {
		features |= 0x01
	}
	if sig.SEIPDv2 {
		features |= 0x08
	}

	if features != 0x00 {
		subpackets = append(subpackets, outputSubpacket{true, featuresSubpacket, false, []byte{features}})
	}

	if sig.TrustLevel != 0 {
		subpackets = append(subpackets, outputSubpacket{true, trustSubpacket, true, []byte{byte(sig.TrustLevel), byte(sig.TrustAmount)}})
	}

	if sig.TrustRegularExpression != nil {
		// RFC specifies the string should be null-terminated; add a null byte to the end
		subpackets = append(subpackets, outputSubpacket{true, regularExpressionSubpacket, true, []byte(*sig.TrustRegularExpression + "\000")})
	}

	if sig.KeyLifetimeSecs != nil && *sig.KeyLifetimeSecs != 0 {
		keyLifetime := make([]byte, 4)
		binary.BigEndian.PutUint32(keyLifetime, *sig.KeyLifetimeSecs)
		subpackets = append(subpackets, outputSubpacket{true, keyExpirationSubpacket, true, keyLifetime})
	}

	if sig.IsPrimaryId != nil && *sig.IsPrimaryId {
		subpackets = append(subpackets, outputSubpacket{true, primaryUserIdSubpacket, false, []byte{1}})
	}

	if len(sig.PreferredSymmetric) > 0 {
		subpackets = append(subpackets, outputSubpacket{true, prefSymmetricAlgosSubpacket, false, sig.PreferredSymmetric})
	}

	if len(sig.PreferredHash) > 0 {
		subpackets = append(subpackets, outputSubpacket{true, prefHashAlgosSubpacket, false, sig.PreferredHash})
	}

	if len(sig.PreferredCompression) > 0 {
		subpackets = append(subpackets, outputSubpacket{true, prefCompressionSubpacket, false, sig.PreferredCompression})
	}

	if len(sig.PolicyURI) > 0 {
		subpackets = append(subpackets, outputSubpacket{true, policyUriSubpacket, false, []uint8(sig.PolicyURI)})
	}

	if len(sig.PreferredCipherSuites) > 0 {
		serialized := make([]byte, len(sig.PreferredCipherSuites)*2)
		for i, cipherSuite := range sig.PreferredCipherSuites {
			serialized[2*i] = cipherSuite[0]
			serialized[2*i+1] = cipherSuite[1]
		}
		subpackets = append(subpackets, outputSubpacket{true, prefCipherSuitesSubpacket, false, serialized})
	}

	// Revocation reason appears only in revocation signatures and is serialized as per section 5.2.3.23.
	if sig.RevocationReason != nil {
		subpackets = append(subpackets, outputSubpacket{true, reasonForRevocationSubpacket, true,
			append([]uint8{uint8(*sig.RevocationReason)}, []uint8(sig.RevocationReasonText)...)})
	}

	// EmbeddedSignature appears only in subkeys capable of signing and is serialized as per section 5.2.3.26.
	if sig.EmbeddedSignature != nil {
		var buf bytes.Buffer
		err = sig.EmbeddedSignature.serializeBody(&buf)
		if err != nil {
			return
		}
		subpackets = append(subpackets, outputSubpacket{true, embeddedSignatureSubpacket, true, buf.Bytes()})
	}

	return
}

// AddMetadataToHashSuffix modifies the current hash suffix to include metadata
// (format, filename, and time). Version 5 keys protect this data including it
// in the hash computation. See section 5.2.4.
func (sig *Signature) AddMetadataToHashSuffix() {
	if sig == nil || sig.Version != 5 {
		return
	}
	if sig.SigType != 0x00 && sig.SigType != 0x01 {
		return
	}
	lit := sig.Metadata
	if lit == nil {
		// This will translate into six 0x00 bytes.
		lit = &LiteralData{}
	}

	// Extract the current byte count
	n := sig.HashSuffix[len(sig.HashSuffix)-8:]
	l := uint64(
		uint64(n[0])<<56 | uint64(n[1])<<48 | uint64(n[2])<<40 | uint64(n[3])<<32 |
			uint64(n[4])<<24 | uint64(n[5])<<16 | uint64(n[6])<<8 | uint64(n[7]))

	suffix := bytes.NewBuffer(nil)
	suffix.Write(sig.HashSuffix[:l])

	// Add the metadata
	var buf [4]byte
	buf[0] = lit.Format
	fileName := lit.FileName
	if len(lit.FileName) > 255 {
		fileName = fileName[:255]
	}
	buf[1] = byte(len(fileName))
	suffix.Write(buf[:2])
	suffix.Write([]byte(lit.FileName))
	binary.BigEndian.PutUint32(buf[:], lit.Time)
	suffix.Write(buf[:])

	// Update the counter and restore trailing bytes
	l = uint64(suffix.Len())
	suffix.Write([]byte{0x05, 0xff})
	suffix.Write([]byte{
		uint8(l >> 56), uint8(l >> 48), uint8(l >> 40), uint8(l >> 32),
		uint8(l >> 24), uint8(l >> 16), uint8(l >> 8), uint8(l),
	})
	sig.HashSuffix = suffix.Bytes()
}
