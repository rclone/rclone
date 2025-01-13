package cryptomator

import (
	"bytes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"unsafe"
)

type ctrMacCryptor struct {
	aes     cipher.Block
	hmacKey []byte
}

func (_ *ctrMacCryptor) HeaderNonceSize() int { return headerCtrMacNonceSize }
func (_ *ctrMacCryptor) HeaderTagSize() int   { return headerCtrMacTagSize }

const (
	headerCtrMacNonceSize     = 16
	headerCtrMacPayloadSize   = HeaderContentKeySize + HeaderReservedSize
	headerCtrMacTagSize       = 32
	headerCtrMacEncryptedSize = headerCtrMacNonceSize + headerCtrMacPayloadSize + headerCtrMacTagSize
)

type headerCtrMacEncrypted struct {
	Nonce            [headerCtrMacNonceSize]byte
	EncryptedPayload [headerCtrMacPayloadSize]byte
	Mac              [headerCtrMacTagSize]byte
}

var _ [0]struct{} = [unsafe.Sizeof(headerCtrMacEncrypted{}) - headerCtrMacEncryptedSize]struct{}{}

type headerCtrMacPayload struct {
	Reserved   [HeaderReservedSize]byte
	ContentKey [HeaderContentKeySize]byte
}

var _ [0]struct{} = [unsafe.Sizeof(headerCtrMacPayload{}) - headerCtrMacPayloadSize]struct{}{}

func copySameLength(dst, src []byte, name string) error {
	if len(dst) != len(src) {
		return fmt.Errorf("incorrect length of %s: expected %d got %d", name, len(dst), len(src))
	}
	copy(dst, src)
	return nil
}

func (c *ctrMacCryptor) MarshalHeader(w io.Writer, h FileHeader) (err error) {
	var payload headerCtrMacPayload
	if err = copySameLength(payload.Reserved[:], h.Reserved, "Reserved"); err != nil {
		return
	}
	if err = copySameLength(payload.ContentKey[:], h.ContentKey, "ContentKey"); err != nil {
		return
	}

	ctr := cipher.NewCTR(c.aes, h.Nonce)
	var encBuffer bytes.Buffer
	ctrWriter := cipher.StreamWriter{S: ctr, W: &encBuffer}
	if err = binary.Write(ctrWriter, binary.BigEndian, &payload); err != nil {
		return
	}
	encPayload := encBuffer.Bytes()

	hash := hmac.New(sha256.New, c.hmacKey)
	hash.Write(h.Nonce)
	hash.Write(encPayload)
	mac := hash.Sum(nil)

	var encHeader headerCtrMacEncrypted
	if err = copySameLength(encHeader.Nonce[:], h.Nonce, "Nonce"); err != nil {
		return err
	}
	if err = copySameLength(encHeader.EncryptedPayload[:], encPayload, "encPayload"); err != nil {
		return err
	}
	if err = copySameLength(encHeader.Mac[:], mac, "mac"); err != nil {
		return err
	}

	err = binary.Write(w, binary.BigEndian, &encHeader)
	return
}

func (c *ctrMacCryptor) UnmarshalHeader(r io.Reader) (header FileHeader, err error) {
	var encHeader headerCtrMacEncrypted
	if err = binary.Read(r, binary.BigEndian, &encHeader); err != nil {
		return
	}

	header.Nonce = encHeader.Nonce[:]

	hash := hmac.New(sha256.New, c.hmacKey)
	hash.Write(encHeader.Nonce[:])
	hash.Write(encHeader.EncryptedPayload[:])
	expectedMac := hash.Sum(nil)
	if !hmac.Equal(expectedMac, encHeader.Mac[:]) {
		return header, fmt.Errorf("invalid hmac: wanted %s, got %s", expectedMac, encHeader.Mac)
	}

	ctr := cipher.NewCTR(c.aes, encHeader.Nonce[:])
	var payload headerCtrMacPayload
	ctrReader := cipher.StreamReader{S: ctr, R: bytes.NewReader(encHeader.EncryptedPayload[:])}
	if err = binary.Read(ctrReader, binary.BigEndian, &payload); err != nil {
		return
	}

	header.ContentKey = payload.ContentKey[:]
	header.Reserved = payload.Reserved[:]
	return
}
