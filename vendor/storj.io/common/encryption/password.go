// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package encryption

import (
	"crypto/hmac"
	"crypto/sha256"

	"github.com/zeebo/errs"
	"golang.org/x/crypto/argon2"

	"storj.io/common/memory"
	"storj.io/common/storj"
)

func sha256hmac(key, data []byte) ([]byte, error) {
	h := hmac.New(sha256.New, key)
	if _, err := h.Write(data); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

// DeriveRootKey derives a root key for some path using the salt for the bucket and
// a password from the user. See the password key derivation design doc.
func DeriveRootKey(password, salt []byte, path storj.Path, argon2Threads uint8) (*storj.Key, error) {
	mixedSalt, err := sha256hmac(password, salt)
	if err != nil {
		return nil, err
	}

	pathSalt, err := sha256hmac(mixedSalt, []byte(path))
	if err != nil {
		return nil, err
	}

	// use a time of 1, 64MB of ram, and all of the cores.
	keyData := argon2.IDKey(password, pathSalt, 1, uint32(64*memory.MiB/memory.KiB), argon2Threads, 32)
	if len(keyData) != len(storj.Key{}) {
		return nil, errs.New("invalid output from argon2id")
	}

	var key storj.Key
	copy(key[:], keyData)
	return &key, nil
}
