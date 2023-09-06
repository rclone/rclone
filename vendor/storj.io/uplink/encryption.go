// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"storj.io/common/encryption"
	"storj.io/common/storj"
)

// EncryptionKey represents a key for encrypting and decrypting data.
type EncryptionKey struct {
	key *storj.Key
}

// DeriveEncryptionKey derives a salted encryption key for passphrase using the
// salt.
//
// This function is useful for deriving a salted encryption key for users when
// implementing multitenancy in a single app bucket. See the relevant section in
// the package documentation.
func DeriveEncryptionKey(passphrase string, salt []byte) (*EncryptionKey, error) {
	key, err := encryption.DeriveRootKey([]byte(passphrase), salt, "", 1)
	if err != nil {
		return nil, packageError.Wrap(err)
	}
	return &EncryptionKey{key: key}, nil
}
