// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package encryption

import (
	"golang.org/x/crypto/nacl/secretbox"

	"storj.io/common/storj"
)

type secretboxEncrypter struct {
	blockSize     int
	key           *storj.Key
	startingNonce *storj.Nonce
}

// NewSecretboxEncrypter returns a Transformer that encrypts the data passing
// through with key.
//
// startingNonce is treated as a big-endian encoded unsigned
// integer, and as blocks pass through, their block number and the starting
// nonce is added together to come up with that block's nonce. Encrypting
// different data with the same key and the same nonce is a huge security
// issue. It's safe to always encode new data with a random key and random
// startingNonce. The monotonically-increasing nonce (that rolls over) is to
// protect against data reordering.
//
// When in doubt, generate a new key from crypto/rand and a startingNonce
// from crypto/rand as often as possible.
func NewSecretboxEncrypter(key *storj.Key, startingNonce *storj.Nonce, encryptedBlockSize int) (Transformer, error) {
	if encryptedBlockSize <= secretbox.Overhead {
		return nil, ErrInvalidConfig.New("encrypted block size %d too small", encryptedBlockSize)
	}
	return &secretboxEncrypter{
		blockSize:     encryptedBlockSize - secretbox.Overhead,
		key:           key,
		startingNonce: startingNonce,
	}, nil
}

func (s *secretboxEncrypter) InBlockSize() int {
	return s.blockSize
}

func (s *secretboxEncrypter) OutBlockSize() int {
	return s.blockSize + secretbox.Overhead
}

func calcNonce(startingNonce *storj.Nonce, blockNum int64) (rv *storj.Nonce, err error) {
	rv = new(storj.Nonce)
	if copy(rv[:], (*startingNonce)[:]) != len(rv) {
		return rv, Error.New("didn't copy memory?!")
	}
	_, err = incrementBytes(rv[:], blockNum)
	return rv, err
}

func (s *secretboxEncrypter) Transform(out, in []byte, blockNum int64) ([]byte, error) {
	nonce, err := calcNonce(s.startingNonce, blockNum)
	if err != nil {
		return nil, err
	}
	return secretbox.Seal(out, in, nonce.Raw(), s.key.Raw()), nil
}

type secretboxDecrypter struct {
	blockSize     int
	key           *storj.Key
	startingNonce *storj.Nonce
}

// NewSecretboxDecrypter returns a Transformer that decrypts the data passing
// through with key. See the comments for NewSecretboxEncrypter about
// startingNonce.
func NewSecretboxDecrypter(key *storj.Key, startingNonce *storj.Nonce, encryptedBlockSize int) (Transformer, error) {
	if encryptedBlockSize <= secretbox.Overhead {
		return nil, ErrInvalidConfig.New("encrypted block size %d too small", encryptedBlockSize)
	}
	return &secretboxDecrypter{
		blockSize:     encryptedBlockSize - secretbox.Overhead,
		key:           key,
		startingNonce: startingNonce,
	}, nil
}

func (s *secretboxDecrypter) InBlockSize() int {
	return s.blockSize + secretbox.Overhead
}

func (s *secretboxDecrypter) OutBlockSize() int {
	return s.blockSize
}

func (s *secretboxDecrypter) Transform(out, in []byte, blockNum int64) ([]byte, error) {
	nonce, err := calcNonce(s.startingNonce, blockNum)
	if err != nil {
		return nil, err
	}
	rv, success := secretbox.Open(out, in, nonce.Raw(), s.key.Raw())
	if !success {
		return nil, ErrDecryptFailed.New("")
	}
	return rv, nil
}

// EncryptSecretBox encrypts byte data with a key and nonce. The cipher data is returned.
func EncryptSecretBox(data []byte, key *storj.Key, nonce *storj.Nonce) (cipherData []byte, err error) {
	return secretbox.Seal(nil, data, nonce.Raw(), key.Raw()), nil
}

// DecryptSecretBox decrypts byte data with a key and nonce. The plain data is returned.
func DecryptSecretBox(cipherData []byte, key *storj.Key, nonce *storj.Nonce) (data []byte, err error) {
	data, success := secretbox.Open(nil, cipherData, nonce.Raw(), key.Raw())
	if !success {
		return nil, ErrDecryptFailed.New("")
	}
	return data, nil
}
