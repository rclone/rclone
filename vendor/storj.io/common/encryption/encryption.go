// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package encryption

import (
	"crypto/hmac"
	"crypto/sha512"

	"storj.io/common/storj"
)

const (
	// AESGCMNonceSize is the size of an AES-GCM nonce
	AESGCMNonceSize = 12
	// unit32Size is the number of bytes in the uint32 type
	uint32Size = 4
)

// AESGCMNonce represents the nonce used by the AES-GCM protocol
type AESGCMNonce [AESGCMNonceSize]byte

// ToAESGCMNonce returns the nonce as a AES-GCM nonce
func ToAESGCMNonce(nonce *storj.Nonce) *AESGCMNonce {
	aes := new(AESGCMNonce)
	copy((*aes)[:], nonce[:AESGCMNonceSize])
	return aes
}

// Increment increments the nonce with the given amount
func Increment(nonce *storj.Nonce, amount int64) (truncated bool, err error) {
	return incrementBytes(nonce[:], amount)
}

// Encrypt encrypts data with the given cipher, key and nonce
func Encrypt(data []byte, cipher storj.CipherSuite, key *storj.Key, nonce *storj.Nonce) (cipherData []byte, err error) {
	// Don't encrypt empty slice
	if len(data) == 0 {
		return []byte{}, nil
	}

	switch cipher {
	case storj.EncNull:
		return data, nil
	case storj.EncAESGCM:
		return EncryptAESGCM(data, key, ToAESGCMNonce(nonce))
	case storj.EncSecretBox:
		return EncryptSecretBox(data, key, nonce)
	case storj.EncNullBase64URL:
		return nil, ErrInvalidConfig.New("base64 encoding not supported for this operation")
	default:
		return nil, ErrInvalidConfig.New("encryption type %d is not supported", cipher)
	}
}

// Decrypt decrypts cipherData with the given cipher, key and nonce
func Decrypt(cipherData []byte, cipher storj.CipherSuite, key *storj.Key, nonce *storj.Nonce) (data []byte, err error) {
	// Don't decrypt empty slice
	if len(cipherData) == 0 {
		return []byte{}, nil
	}

	switch cipher {
	case storj.EncNull:
		return cipherData, nil
	case storj.EncAESGCM:
		return DecryptAESGCM(cipherData, key, ToAESGCMNonce(nonce))
	case storj.EncSecretBox:
		return DecryptSecretBox(cipherData, key, nonce)
	case storj.EncNullBase64URL:
		return nil, ErrInvalidConfig.New("base64 encoding not supported for this operation")
	default:
		return nil, ErrInvalidConfig.New("encryption type %d is not supported", cipher)
	}
}

// NewEncrypter creates a Transformer using the given cipher, key and nonce to encrypt data passing through it
func NewEncrypter(cipher storj.CipherSuite, key *storj.Key, startingNonce *storj.Nonce, encryptedBlockSize int) (Transformer, error) {
	switch cipher {
	case storj.EncNull:
		return &NoopTransformer{}, nil
	case storj.EncAESGCM:
		return NewAESGCMEncrypter(key, ToAESGCMNonce(startingNonce), encryptedBlockSize)
	case storj.EncSecretBox:
		return NewSecretboxEncrypter(key, startingNonce, encryptedBlockSize)
	case storj.EncNullBase64URL:
		return nil, ErrInvalidConfig.New("base64 encoding not supported for this operation")
	default:
		return nil, ErrInvalidConfig.New("encryption type %d is not supported", cipher)
	}
}

// NewDecrypter creates a Transformer using the given cipher, key and nonce to decrypt data passing through it
func NewDecrypter(cipher storj.CipherSuite, key *storj.Key, startingNonce *storj.Nonce, encryptedBlockSize int) (Transformer, error) {
	switch cipher {
	case storj.EncNull:
		return &NoopTransformer{}, nil
	case storj.EncAESGCM:
		return NewAESGCMDecrypter(key, ToAESGCMNonce(startingNonce), encryptedBlockSize)
	case storj.EncSecretBox:
		return NewSecretboxDecrypter(key, startingNonce, encryptedBlockSize)
	case storj.EncNullBase64URL:
		return nil, ErrInvalidConfig.New("base64 encoding not supported for this operation")
	default:
		return nil, ErrInvalidConfig.New("encryption type %d is not supported", cipher)
	}
}

// EncryptKey encrypts keyToEncrypt with the given cipher, key and nonce
func EncryptKey(keyToEncrypt *storj.Key, cipher storj.CipherSuite, key *storj.Key, nonce *storj.Nonce) (storj.EncryptedPrivateKey, error) {
	return Encrypt(keyToEncrypt[:], cipher, key, nonce)
}

// DecryptKey decrypts keyToDecrypt with the given cipher, key and nonce
func DecryptKey(keyToDecrypt storj.EncryptedPrivateKey, cipher storj.CipherSuite, key *storj.Key, nonce *storj.Nonce) (*storj.Key, error) {
	plainData, err := Decrypt(keyToDecrypt, cipher, key, nonce)
	if err != nil {
		return nil, err
	}

	var decryptedKey storj.Key
	copy(decryptedKey[:], plainData)

	return &decryptedKey, nil
}

// DeriveKey derives new key from the given key and message using HMAC-SHA512
func DeriveKey(key *storj.Key, message string) (*storj.Key, error) {
	mac := hmac.New(sha512.New, key[:])
	_, err := mac.Write([]byte(message))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	derived := new(storj.Key)
	copy(derived[:], mac.Sum(nil))

	return derived, nil
}

// CalcEncryptedSize calculates what would be the size of the cipher data after
// encrypting data with dataSize using a Transformer with the given encryption
// parameters.
func CalcEncryptedSize(dataSize int64, parameters storj.EncryptionParameters) (int64, error) {
	transformer, err := NewEncrypter(parameters.CipherSuite, new(storj.Key), new(storj.Nonce), int(parameters.BlockSize))
	if err != nil {
		return 0, err
	}

	inBlockSize := int64(transformer.InBlockSize())
	blocks := (dataSize + uint32Size + inBlockSize - 1) / inBlockSize

	encryptedSize := blocks * int64(transformer.OutBlockSize())

	return encryptedSize, nil
}
