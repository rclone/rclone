// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"storj.io/common/encryption"
	"storj.io/common/paths"
	"storj.io/common/pb"
	"storj.io/common/storj"
)

// encryptionAccess represents an encryption access context. It holds
// information about how various buckets and objects should be
// encrypted and decrypted.
type encryptionAccess struct {
	store *encryption.Store
}

// newEncryptionAccess creates an encryption access context.
func newEncryptionAccess() *encryptionAccess {
	store := encryption.NewStore()
	return &encryptionAccess{store: store}
}

// newEncryptionAccessWithDefaultKey creates an encryption access context with
// a default key set.
// Use (*Project).SaltedKeyFromPassphrase to generate a default key.
func newEncryptionAccessWithDefaultKey(defaultKey *storj.Key) *encryptionAccess {
	ec := newEncryptionAccess()
	ec.setDefaultKey(defaultKey)
	return ec
}

// Store returns the underlying encryption store for the access context.
func (s *encryptionAccess) Store() *encryption.Store {
	return s.store
}

// setDefaultKey sets the default key for the encryption access context.
// Use (*Project).SaltedKeyFromPassphrase to generate a default key.
func (s *encryptionAccess) setDefaultKey(defaultKey *storj.Key) {
	s.store.SetDefaultKey(defaultKey)
}

func (s *encryptionAccess) setDefaultPathCipher(defaultPathCipher storj.CipherSuite) {
	s.store.SetDefaultPathCipher(defaultPathCipher)
}

func (s *encryptionAccess) toProto() (*pb.EncryptionAccess, error) {
	var storeEntries []*pb.EncryptionAccess_StoreEntry
	err := s.store.IterateWithCipher(func(bucket string, unenc paths.Unencrypted, enc paths.Encrypted, key storj.Key, pathCipher storj.CipherSuite) error {
		storeEntries = append(storeEntries, &pb.EncryptionAccess_StoreEntry{
			Bucket:          []byte(bucket),
			UnencryptedPath: []byte(unenc.Raw()),
			EncryptedPath:   []byte(enc.Raw()),
			Key:             key[:],
			PathCipher:      pb.CipherSuite(pathCipher),
		})
		return nil
	})
	if err != nil {
		return nil, packageError.Wrap(err)
	}

	var defaultKey []byte
	if key := s.store.GetDefaultKey(); key != nil {
		defaultKey = key[:]
	}

	return &pb.EncryptionAccess{
		DefaultKey:        defaultKey,
		StoreEntries:      storeEntries,
		DefaultPathCipher: pb.CipherSuite(s.store.GetDefaultPathCipher()),
	}, nil
}

func parseEncryptionAccessFromProto(p *pb.EncryptionAccess) (*encryptionAccess, error) {
	access := newEncryptionAccess()
	if len(p.DefaultKey) > 0 {
		if len(p.DefaultKey) != len(storj.Key{}) {
			return nil, packageError.New("invalid default key in encryption access")
		}
		var defaultKey storj.Key
		copy(defaultKey[:], p.DefaultKey)
		access.setDefaultKey(&defaultKey)
	}

	access.setDefaultPathCipher(storj.CipherSuite(p.DefaultPathCipher))
	// Unspecified cipher suite means that most probably access was serialized
	// before path cipher was moved to encryption access
	if p.DefaultPathCipher == pb.CipherSuite_ENC_UNSPECIFIED {
		access.setDefaultPathCipher(storj.EncAESGCM)
	}

	for _, entry := range p.StoreEntries {
		if len(entry.Key) != len(storj.Key{}) {
			return nil, packageError.New("invalid key in encryption access entry")
		}
		var key storj.Key
		copy(key[:], entry.Key)

		err := access.store.AddWithCipher(
			string(entry.Bucket),
			paths.NewUnencrypted(string(entry.UnencryptedPath)),
			paths.NewEncrypted(string(entry.EncryptedPath)),
			key,
			storj.CipherSuite(entry.PathCipher),
		)
		if err != nil {
			return nil, packageError.New("invalid encryption access entry: %v", err)
		}
	}

	return access, nil
}
