package cryptomator

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"unsafe"
)

type FileHeader struct {
	Nonce      []byte
	Reserved   []byte
	ContentKey []byte
}

const (
	HeaderContentKeySize        = 32
	HeaderReservedSize          = 8
	HeaderPayloadSize           = HeaderContentKeySize + HeaderReservedSize
	HeaderReservedValue  uint64 = 0xFFFFFFFFFFFFFFFF
)

func (c *Cryptor) NewHeader() (header FileHeader, err error) {
	header.Nonce = make([]byte, c.NonceSize())
	header.ContentKey = make([]byte, HeaderContentKeySize)
	header.Reserved = make([]byte, HeaderReservedSize)

	if _, err = rand.Read(header.Nonce); err != nil {
		return
	}

	if _, err = rand.Read(header.ContentKey); err != nil {
		return
	}

	binary.BigEndian.PutUint64(header.Reserved, HeaderReservedValue)

	return
}

type headerPayload struct {
	Reserved   [HeaderReservedSize]byte
	ContentKey [HeaderContentKeySize]byte
}

var _ [0]struct{} = [unsafe.Sizeof(headerPayload{}) - HeaderPayloadSize]struct{}{}

func copySameLength(dst, src []byte, name string) error {
	if len(dst) != len(src) {
		return fmt.Errorf("incorrect length of %s: expected %d got %d", name, len(dst), len(src))
	}
	copy(dst, src)
	return nil
}

func (c *Cryptor) MarshalHeader(w io.Writer, h FileHeader) (err error) {
	var payload headerPayload
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
	encPayload := c.EncryptChunk(encBuffer.Bytes(), h.Nonce, nil)
	w.Write(encPayload)
	return
}

func (c *Cryptor) UnmarshalHeader(r io.Reader) (header FileHeader, err error) {
	encHeader := make([]byte, c.NonceSize()+HeaderPayloadSize+c.TagSize())
	_, err = io.ReadFull(r, encHeader)
	if err != nil {
		return
	}
	nonce := encHeader[:c.NonceSize()]
	encHeader, err = c.DecryptChunk(encHeader, nil)
	if err != nil {
		return
	}

	var payload headerPayload
	if err = binary.Read(bytes.NewReader(encHeader), binary.BigEndian, &payload); err != nil {
		return
	}
	header.Nonce = nonce
	header.ContentKey = payload.ContentKey[:]
	header.Reserved = payload.Reserved[:]
	return
}
