package cryptomator

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/miscreant/miscreant.go"
)

const (
	CipherComboSivCtrMac = "SIV_CTRMAC"
	CipherComboSivGcm    = "SIV_GCM"
)

type Cryptor struct {
	siv *miscreant.Cipher
	contentCryptor
}

type contentCryptor interface {
	MarshalHeader(w io.Writer, h FileHeader) (err error)
	UnmarshalHeader(r io.Reader) (header FileHeader, err error)

	HeaderNonceSize() int
	HeaderTagSize() int
}

// TODO: support both cipher combos.

func NewCryptor(key MasterKey, cipherCombo string) (c Cryptor, err error) {
	c.siv, err = miscreant.NewAESCMACSIV(append(key.MacKey, key.EncryptKey...))
	if err != nil {
		return
	}

	aes, err := aes.NewCipher(key.EncryptKey)
	if err != nil {
		return
	}

	switch cipherCombo {
	default:
		err = fmt.Errorf("unsupported cipher combo %q", cipherCombo)
		return

	case CipherComboSivGcm:
		aesGcm, err := cipher.NewGCM(aes)
		if err != nil {
			return c, err
		}
		c.contentCryptor = &gcmCryptor{aesGcm}

	case CipherComboSivCtrMac:
		c.contentCryptor = &ctrMacCryptor{aes: aes, hmacKey: key.MacKey}
	}

	return
}

func (c *Cryptor) EncryptDirID(dirID string) (string, error) {
	ciphertext, err := c.siv.Seal(nil, []byte(dirID))
	if err != nil {
		return "", err
	}
	hash := sha1.Sum(ciphertext)
	return base32.StdEncoding.EncodeToString(hash[:]), nil
}

func (c *Cryptor) EncryptFilename(filename string, dirID string) (string, error) {
	ciphertext, err := c.siv.Seal(nil, []byte(filename), []byte(dirID))
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(ciphertext), nil
}

func (c *Cryptor) DecryptFilename(filename string, dirID string) (string, error) {
	filenameBytes, err := base64.URLEncoding.DecodeString(filename)
	if err != nil {
		return "", err
	}
	plaintext, err := c.siv.Open(nil, filenameBytes, []byte(dirID))
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

type FileHeader struct {
	Nonce      []byte
	Reserved   []byte
	ContentKey []byte
}

const (
	HeaderContentKeySize        = 32
	HeaderReservedSize          = 8
	HeaderReservedValue  uint64 = 0xFFFFFFFFFFFFFFFF
)

func (c *Cryptor) NewHeader() (header FileHeader, err error) {
	header.Nonce = make([]byte, c.HeaderNonceSize())
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
