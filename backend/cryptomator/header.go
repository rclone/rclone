package cryptomator

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"unsafe"
)

// fileHeader is the header of an encrypted Cryptomator file.
type fileHeader struct {
	// The nonce used to encrypt the file header. Each file content chunk, while encrypted with its own nonce, also mixes the file header nonce into the MAC.
	Nonce    []byte
	Reserved []byte
	// The AES key used to encrypt the file contents.
	ContentKey []byte
}

const (
	// headerContentKeySize is the size of the ContentKey in the FileHeader.
	headerContentKeySize = 32
	// headerReservedSize is the size of the Reserved data in the FileHeader.
	headerReservedSize = 8
	// headerPayloadSize is the size of the encrypted part of the file header.
	headerPayloadSize = headerContentKeySize + headerReservedSize
	// headerReservedValue is the expected value of the Reserved data.
	headerReservedValue uint64 = 0xFFFFFFFFFFFFFFFF
)

// NewHeader creates a new randomly initialized FileHeader
func (c *cryptor) NewHeader() (header fileHeader, err error) {
	header.Nonce = make([]byte, c.nonceSize())
	header.ContentKey = make([]byte, headerContentKeySize)
	header.Reserved = make([]byte, headerReservedSize)

	if _, err = rand.Read(header.Nonce); err != nil {
		return
	}

	if _, err = rand.Read(header.ContentKey); err != nil {
		return
	}

	binary.BigEndian.PutUint64(header.Reserved, headerReservedValue)

	return
}

type headerPayload struct {
	Reserved   [headerReservedSize]byte
	ContentKey [headerContentKeySize]byte
}

var _ [0]struct{} = [unsafe.Sizeof(headerPayload{}) - headerPayloadSize]struct{}{}

func copySameLength(dst, src []byte, name string) error {
	if len(dst) != len(src) {
		return fmt.Errorf("incorrect length of %s: expected %d got %d", name, len(dst), len(src))
	}
	copy(dst, src)
	return nil
}

// marshalHeader encrypts the header and writes it in encrypted form to the writer.
func (c *cryptor) marshalHeader(w io.Writer, h fileHeader) (err error) {
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
	encPayload := c.encryptChunk(encBuffer.Bytes(), h.Nonce, nil)
	_, err = w.Write(encPayload)
	return
}

// unmarshalHeader reads an encrypted header from the reader and decrypts it.
func (c *cryptor) unmarshalHeader(r io.Reader) (header fileHeader, err error) {
	encHeader := make([]byte, c.nonceSize()+headerPayloadSize+c.tagSize())
	_, err = io.ReadFull(r, encHeader)
	if err != nil {
		return
	}
	nonce := encHeader[:c.nonceSize()]
	encHeader, err = c.decryptChunk(encHeader, nil)
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
