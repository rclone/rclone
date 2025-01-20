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
	// CipherComboSivGcm uses AES-SIV for filenames and AES-GCM for contents. It is the current Cryptomator default.
	CipherComboSivGcm = "SIV_GCM"
	// CipherComboSivCtrMac uses AES-SIV for filenames and AES-CTR plus an HMAC for contents. It was the default until Cryptomator 1.7.
	CipherComboSivCtrMac = "SIV_CTRMAC"
)

// Cryptor implements encryption operations for Cryptomator vaults.
type Cryptor struct {
	masterKey   MasterKey
	sivKey      []byte
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

// NewCryptor creates a new cryptor from vault configuration.
func NewCryptor(key MasterKey, cipherCombo string) (c Cryptor, err error) {
	c.masterKey = key
	c.sivKey = append(key.MacKey, key.EncryptKey...)
	c.cipherCombo = cipherCombo
	c.contentCryptor, err = c.newContentCryptor(key.EncryptKey)
	if err != nil {
		return
	}
	return
}

func (c *Cryptor) newSIV() *miscreant.Cipher {
	siv, err := miscreant.NewAESCMACSIV(c.sivKey)
	if err != nil {
		panic(err)
	}
	return siv
}

// EncryptDirID encrypts a directory ID.
func (c *Cryptor) EncryptDirID(dirID string) string {
	ciphertext, err := c.newSIV().Seal(nil, []byte(dirID))
	if err != nil {
		// Seal can only actually fail if you pass in more than 126 associated data items.
		panic(err)
	}
	hash := sha1.Sum(ciphertext)
	return base32.StdEncoding.EncodeToString(hash[:])
}

// EncryptFilename encrypts a filename.
func (c *Cryptor) EncryptFilename(filename string, dirID string) string {
	ciphertext, err := c.newSIV().Seal(nil, []byte(filename), []byte(dirID))
	if err != nil {
		// Seal can only actually fail if you pass in more than 126 associated data items.
		panic(err)
	}
	return base64.URLEncoding.EncodeToString(ciphertext)
}

// DecryptFilename decrypts a filename.
func (c *Cryptor) DecryptFilename(filename string, dirID string) (string, error) {
	filenameBytes, err := base64.URLEncoding.DecodeString(filename)
	if err != nil {
		return "", err
	}
	plaintext, err := c.newSIV().Open(nil, filenameBytes, []byte(dirID))
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

// EncryptionOverhead returns how much longer a payload of the given size would be made by EncryptChunk.
func (c *Cryptor) EncryptionOverhead() int {
	return c.NonceSize() + c.TagSize()
}

type gcmCryptor struct {
	aesGcm cipher.AEAD
}

func (*gcmCryptor) NonceSize() int { return 12 }
func (*gcmCryptor) TagSize() int   { return 16 }

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
	_ = binary.Write(&buf, binary.BigEndian, chunkNr)
	buf.Write(fileNonce)
	return buf.Bytes()
}

type ctrMacCryptor struct {
	aes     cipher.Block
	hmacKey []byte
}

func (*ctrMacCryptor) NonceSize() int { return 16 }
func (*ctrMacCryptor) TagSize() int   { return 32 }

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
	_ = binary.Write(&buf, binary.BigEndian, chunkNr)
	return buf.Bytes()
}
