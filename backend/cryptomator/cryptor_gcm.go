package cryptomator

import (
	"bytes"
	"crypto/cipher"
	"encoding/binary"
	"io"
	"unsafe"
)

type gcmCryptor struct {
	aesGcm cipher.AEAD
}

func (_ *gcmCryptor) HeaderNonceSize() int { return headerGcmNonceSize }
func (_ *gcmCryptor) HeaderTagSize() int   { return headerGcmTagSize }

const (
	headerGcmNonceSize     = 12
	headerGcmPayloadSize   = HeaderContentKeySize + HeaderReservedSize
	headerGcmTagSize       = 16
	headerGcmEncryptedSize = headerGcmNonceSize + headerGcmPayloadSize + headerGcmTagSize
)

type headerGcmEncrypted struct {
	Nonce            [headerGcmNonceSize]byte
	EncryptedPayload [headerGcmPayloadSize + headerGcmTagSize]byte
}

var _ [0]struct{} = [unsafe.Sizeof(headerGcmEncrypted{}) - headerGcmEncryptedSize]struct{}{}

type headerGcmPayload struct {
	Reserved   [HeaderReservedSize]byte
	ContentKey [HeaderContentKeySize]byte
}

var _ [0]struct{} = [unsafe.Sizeof(headerGcmPayload{}) - headerGcmPayloadSize]struct{}{}

func (c *gcmCryptor) MarshalHeader(w io.Writer, h FileHeader) (err error) {
	var payload headerGcmPayload
	if err = copySameLength(payload.Reserved[:], h.Reserved, "Reserved"); err != nil {
		return
	}
	if err = copySameLength(payload.ContentKey[:], h.ContentKey, "ContentKey"); err != nil {
		return
	}
	var encBuffer bytes.Buffer
	if err = binary.Write(&encBuffer, binary.BigEndian, &payload); err != nil {
		return
	}
	encPayload := encBuffer.Bytes()
	encPayload = c.aesGcm.Seal(encPayload[:0], h.Nonce, encPayload, nil)
	var encHeader headerGcmEncrypted
	if err = copySameLength(encHeader.Nonce[:], h.Nonce, "Nonce"); err != nil {
		return
	}
	if err = copySameLength(encHeader.EncryptedPayload[:], encPayload, "encPayload"); err != nil {
		return
	}

	err = binary.Write(w, binary.BigEndian, &encHeader)
	return
}

func (c *gcmCryptor) UnmarshalHeader(r io.Reader) (header FileHeader, err error) {
	var encHeader headerGcmEncrypted
	if err = binary.Read(r, binary.BigEndian, &encHeader); err != nil {
		return
	}

	header.Nonce = encHeader.Nonce[:]

	plaintext, err := c.aesGcm.Open(nil, header.Nonce, encHeader.EncryptedPayload[:], nil)
	if err != nil {
		return
	}
	var payload headerGcmPayload
	if err = binary.Read(bytes.NewReader(plaintext), binary.BigEndian, &payload); err != nil {
		return
	}
	header.ContentKey = payload.ContentKey[:]
	header.Reserved = payload.Reserved[:]
	return
}
