package proton

import (
	"encoding/base64"
	"fmt"

	"github.com/ProtonMail/gluon/rfc822"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
)

type EncryptionScheme int

const (
	InternalScheme EncryptionScheme = 1 << iota
	EncryptedOutsideScheme
	ClearScheme
	PGPInlineScheme
	PGPMIMEScheme
	ClearMIMEScheme
)

type SignatureType int

const (
	NoSignature SignatureType = iota
	DetachedSignature
	AttachedSignature
)

type MessageRecipient struct {
	Type      EncryptionScheme
	Signature SignatureType

	BodyKeyPacket        string            `json:",omitempty"`
	AttachmentKeyPackets map[string]string `json:",omitempty"`
}

type MessagePackage struct {
	Addresses map[string]*MessageRecipient
	MIMEType  rfc822.MIMEType
	Type      EncryptionScheme
	Body      string

	BodyKey        *SessionKey            `json:",omitempty"`
	AttachmentKeys map[string]*SessionKey `json:",omitempty"`
}

func newMessagePackage(mimeType rfc822.MIMEType, encBodyData []byte) *MessagePackage {
	return &MessagePackage{
		Addresses: make(map[string]*MessageRecipient),
		MIMEType:  mimeType,
		Body:      base64.StdEncoding.EncodeToString(encBodyData),

		AttachmentKeys: make(map[string]*SessionKey),
	}
}

type SessionKey struct {
	Key       string
	Algorithm string
}

func newSessionKey(key *crypto.SessionKey) *SessionKey {
	return &SessionKey{
		Key:       key.GetBase64Key(),
		Algorithm: key.Algo,
	}
}

type SendPreferences struct {
	// Encrypt indicates whether the email should be encrypted or not.
	// If it's encrypted, we need to know which public key to use.
	Encrypt bool

	// PubKey contains an OpenPGP key that can be used for encryption.
	PubKey *crypto.KeyRing

	// SignatureType indicates how the email should be signed.
	SignatureType SignatureType

	// EncryptionScheme indicates if we should encrypt body and attachments separately and
	// what MIME format to give the final encrypted email. The two standard PGP
	// schemes are PGP/MIME and PGP/Inline. However we use a custom scheme for
	// internal emails (including the so-called encrypted-to-outside emails,
	// which even though meant for external users, they don't really get out of
	// our platform). If the email is sent unencrypted, no PGP scheme is needed.
	EncryptionScheme EncryptionScheme

	// MIMEType is the MIME type to use for formatting the body of the email
	// (before encryption/after decryption). The standard possibilities are the
	// enriched HTML format, text/html, and plain text, text/plain. But it's
	// also possible to have a multipart/mixed format, which is typically used
	// for PGP/MIME encrypted emails, where attachments go into the body too.
	// Because of this, this option is sometimes called MIME format.
	MIMEType rfc822.MIMEType
}

type SendDraftReq struct {
	Packages []*MessagePackage
}

func (req *SendDraftReq) AddMIMEPackage(
	kr *crypto.KeyRing,
	mimeBody string,
	prefs map[string]SendPreferences,
) error {
	for _, prefs := range prefs {
		if prefs.MIMEType != rfc822.MultipartMixed {
			return fmt.Errorf("invalid MIME type for MIME package: %s", prefs.MIMEType)
		}
	}

	pkg, err := newMIMEPackage(kr, mimeBody, prefs)
	if err != nil {
		return err
	}

	req.Packages = append(req.Packages, pkg)

	return nil
}

func (req *SendDraftReq) AddTextPackage(
	kr *crypto.KeyRing,
	body string,
	mimeType rfc822.MIMEType,
	prefs map[string]SendPreferences,
	attKeys map[string]*crypto.SessionKey,
) error {
	pkg, err := newTextPackage(kr, body, mimeType, prefs, attKeys)
	if err != nil {
		return err
	}

	req.Packages = append(req.Packages, pkg)

	return nil
}

func newMIMEPackage(
	kr *crypto.KeyRing,
	mimeBody string,
	prefs map[string]SendPreferences,
) (*MessagePackage, error) {
	decBodyKey, encBodyData, err := encSplit(kr, mimeBody)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt MIME body: %w", err)
	}

	pkg := newMessagePackage(rfc822.MultipartMixed, encBodyData)

	for addr, prefs := range prefs {
		if prefs.MIMEType != rfc822.MultipartMixed {
			return nil, fmt.Errorf("invalid MIME type for MIME package: %s", prefs.MIMEType)
		}

		if prefs.SignatureType != DetachedSignature {
			return nil, fmt.Errorf("invalid signature type for MIME package: %d", prefs.SignatureType)
		}

		recipient := &MessageRecipient{
			Type:      prefs.EncryptionScheme,
			Signature: prefs.SignatureType,
		}

		switch prefs.EncryptionScheme {
		case PGPMIMEScheme:
			if prefs.PubKey == nil {
				return nil, fmt.Errorf("missing public key for %s", addr)
			}

			encBodyKey, err := prefs.PubKey.EncryptSessionKey(decBodyKey)
			if err != nil {
				return nil, fmt.Errorf("failed to encrypt session key: %w", err)
			}

			recipient.BodyKeyPacket = base64.StdEncoding.EncodeToString(encBodyKey)

		case ClearMIMEScheme:
			pkg.BodyKey = &SessionKey{
				Key:       decBodyKey.GetBase64Key(),
				Algorithm: decBodyKey.Algo,
			}

		default:
			return nil, fmt.Errorf("invalid encryption scheme for MIME package: %d", prefs.EncryptionScheme)
		}

		pkg.Addresses[addr] = recipient
		pkg.Type |= prefs.EncryptionScheme
	}

	return pkg, nil
}

func newTextPackage(
	kr *crypto.KeyRing,
	body string,
	mimeType rfc822.MIMEType,
	prefs map[string]SendPreferences,
	attKeys map[string]*crypto.SessionKey,
) (*MessagePackage, error) {
	if mimeType != rfc822.TextPlain && mimeType != rfc822.TextHTML {
		return nil, fmt.Errorf("invalid MIME type for package: %s", mimeType)
	}

	decBodyKey, encBodyData, err := encSplit(kr, body)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt message body: %w", err)
	}

	pkg := newMessagePackage(mimeType, encBodyData)

	for addr, prefs := range prefs {
		if prefs.MIMEType != mimeType {
			return nil, fmt.Errorf("invalid MIME type for package: %s", prefs.MIMEType)
		}

		if prefs.SignatureType == DetachedSignature && !prefs.Encrypt {
			if prefs.EncryptionScheme == PGPInlineScheme {
				return nil, fmt.Errorf("invalid encryption scheme for %s: %d", addr, prefs.EncryptionScheme)
			}

			if prefs.EncryptionScheme == ClearScheme && mimeType != rfc822.TextPlain {
				return nil, fmt.Errorf("invalid MIME type for clear package: %s", mimeType)
			}
		}

		if prefs.EncryptionScheme == InternalScheme && !prefs.Encrypt {
			return nil, fmt.Errorf("internal packages must be encrypted")
		}

		if prefs.EncryptionScheme == PGPInlineScheme && mimeType != rfc822.TextPlain {
			return nil, fmt.Errorf("invalid MIME type for PGP inline package: %s", mimeType)
		}

		switch prefs.EncryptionScheme {
		case ClearScheme:
			pkg.BodyKey = newSessionKey(decBodyKey)

			for attID, attKey := range attKeys {
				pkg.AttachmentKeys[attID] = newSessionKey(attKey)
			}

		case InternalScheme, PGPInlineScheme:
			// ...

		default:
			return nil, fmt.Errorf("invalid encryption scheme for package: %d", prefs.EncryptionScheme)
		}

		recipient := &MessageRecipient{
			Type:                 prefs.EncryptionScheme,
			Signature:            prefs.SignatureType,
			AttachmentKeyPackets: make(map[string]string),
		}

		if prefs.Encrypt {
			if prefs.PubKey == nil {
				return nil, fmt.Errorf("missing public key for %s", addr)
			}

			if prefs.SignatureType != DetachedSignature {
				return nil, fmt.Errorf("invalid signature type for package: %d", prefs.SignatureType)
			}

			encBodyKey, err := prefs.PubKey.EncryptSessionKey(decBodyKey)
			if err != nil {
				return nil, fmt.Errorf("failed to encrypt session key: %w", err)
			}

			recipient.BodyKeyPacket = base64.StdEncoding.EncodeToString(encBodyKey)

			for attID, attKey := range attKeys {
				encAttKey, err := prefs.PubKey.EncryptSessionKey(attKey)
				if err != nil {
					return nil, fmt.Errorf("failed to encrypt attachment key: %w", err)
				}

				recipient.AttachmentKeyPackets[attID] = base64.StdEncoding.EncodeToString(encAttKey)
			}
		}

		pkg.Addresses[addr] = recipient
		pkg.Type |= prefs.EncryptionScheme
	}

	return pkg, nil
}

func encSplit(kr *crypto.KeyRing, body string) (*crypto.SessionKey, []byte, error) {
	encBody, err := kr.Encrypt(crypto.NewPlainMessageFromString(body), kr)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to encrypt MIME body: %w", err)
	}

	splitEncBody, err := encBody.SplitMessage()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to split message: %w", err)
	}

	decBodyKey, err := kr.DecryptSessionKey(splitEncBody.GetBinaryKeyPacket())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decrypt session key: %w", err)
	}

	return decBodyKey, splitEncBody.GetBinaryDataPacket(), nil
}
