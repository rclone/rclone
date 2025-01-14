package cryptomator

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"hash"

	"github.com/miscreant/miscreant.go"
)

const (
	CipherComboSivCtrMac = "SIV_CTRMAC"
	CipherComboSivGcm    = "SIV_GCM"
)

type Cryptor struct {
	masterKey   MasterKey
	siv         *miscreant.Cipher
	cipherCombo string
	contentCryptor
}

type contentCryptor interface {
	EncryptChunk(plaintext, nonce, additionalData []byte) (ciphertext []byte)
	DecryptChunk(ciphertext, additionalData []byte) ([]byte, error)
	fileAssociatedData(fileNonce []byte, chunkNr uint64) []byte

	NonceSize() int
	TagSize() int
}

func NewCryptor(key MasterKey, cipherCombo string) (c Cryptor, err error) {
	c.masterKey = key
	c.siv, err = miscreant.NewAESCMACSIV(append(key.MacKey, key.EncryptKey...))
	if err != nil {
		return
	}
	c.cipherCombo = cipherCombo
	c.contentCryptor, err = c.newContentCryptor(key.EncryptKey)
	if err != nil {
		return
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

func (c *Cryptor) newContentCryptor(key []byte) (contentCryptor, error) {
	aes, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	switch c.cipherCombo {
	default:
		return nil, fmt.Errorf("unsupported cipher combo %q", c.cipherCombo)

	case CipherComboSivGcm:
		aesGcm, err := cipher.NewGCM(aes)
		if err != nil {
			return nil, err
		}
		return &gcmCryptor{aesGcm}, nil

	case CipherComboSivCtrMac:
		return &ctrMacCryptor{aes: aes, hmacKey: c.masterKey.MacKey}, nil
	}
}

func EncryptedChunkSize(c contentCryptor, payloadSize int) int {
	return c.NonceSize() + payloadSize + c.TagSize()
}

type gcmCryptor struct {
	aesGcm cipher.AEAD
}

func (_ *gcmCryptor) NonceSize() int { return 12 }
func (_ *gcmCryptor) TagSize() int   { return 16 }

func (c *gcmCryptor) EncryptChunk(payload, nonce, additionalData []byte) (ciphertext []byte) {
	buf := bytes.Buffer{}
	buf.Write(nonce)
	buf.Write(c.aesGcm.Seal(nil, nonce, payload, additionalData))
	return buf.Bytes()
}

func (c *gcmCryptor) DecryptChunk(chunk, additionalData []byte) ([]byte, error) {
	nonce := chunk[:c.NonceSize()]
	return c.aesGcm.Open(nil, nonce, chunk[c.NonceSize():], additionalData)
}

func (c *gcmCryptor) fileAssociatedData(fileNonce []byte, chunkNr uint64) []byte {
	buf := bytes.Buffer{}
	binary.Write(&buf, binary.BigEndian, chunkNr)
	buf.Write(fileNonce)
	return buf.Bytes()
}

type ctrMacCryptor struct {
	aes     cipher.Block
	hmacKey []byte
}

func (_ *ctrMacCryptor) NonceSize() int { return 16 }
func (_ *ctrMacCryptor) TagSize() int   { return 32 }

func (c *ctrMacCryptor) newCTR(nonce []byte) cipher.Stream { return cipher.NewCTR(c.aes, nonce) }
func (c *ctrMacCryptor) newHMAC() hash.Hash                { return hmac.New(sha256.New, c.hmacKey) }

func (c *ctrMacCryptor) EncryptChunk(payload, nonce, additionalData []byte) (ciphertext []byte) {
	c.newCTR(nonce).XORKeyStream(payload, payload)
	buf := bytes.Buffer{}
	buf.Write(nonce)
	buf.Write(payload)
	hash := c.newHMAC()
	hash.Write(additionalData)
	hash.Write(buf.Bytes())
	buf.Write(hash.Sum(nil))
	return buf.Bytes()
}

func (c *ctrMacCryptor) DecryptChunk(chunk, additionalData []byte) ([]byte, error) {
	startMac := len(chunk) - c.TagSize()
	mac := chunk[startMac:]
	chunk = chunk[:startMac]

	hash := c.newHMAC()
	hash.Write(additionalData)
	hash.Write(chunk)
	if !hmac.Equal(mac, hash.Sum(nil)) {
		return nil, fmt.Errorf("hmac failed")
	}

	nonce := chunk[:c.NonceSize()]
	chunk = chunk[c.NonceSize():]
	c.newCTR(nonce).XORKeyStream(chunk, chunk)
	return chunk, nil
}

func (c *ctrMacCryptor) fileAssociatedData(fileNonce []byte, chunkNr uint64) []byte {
	buf := bytes.Buffer{}
	buf.Write(fileNonce)
	binary.Write(&buf, binary.BigEndian, chunkNr)
	return buf.Bytes()
}
