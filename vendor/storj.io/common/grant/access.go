// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package grant

import (
	"bytes"
	"errors"
	"fmt"

	"storj.io/common/base58"
	"storj.io/common/encryption"
	"storj.io/common/macaroon"
	"storj.io/common/paths"
	"storj.io/common/pb"
	"storj.io/common/storj"
)

// An Access Grant contains everything to access a project and specific buckets.
// It includes a potentially-restricted API Key, a potentially-restricted set
// of encryption information, and information about the Satellite responsible
// for the project's metadata.
type Access struct {
	SatelliteAddress string
	APIKey           *macaroon.APIKey
	EncAccess        *EncryptionAccess
}

// ParseAccess parses a serialized access grant string.
//
// This should be the main way to instantiate an access grant for opening a project.
// See the note on RequestAccessWithPassphrase.
func ParseAccess(access string) (*Access, error) {
	data, version, err := base58.CheckDecode(access)
	if err != nil || version != 0 {
		return nil, errors.New("invalid access grant format")
	}

	p := new(pb.Scope)
	if err := pb.Unmarshal(data, p); err != nil {
		return nil, fmt.Errorf("unable to unmarshal access grant: %w", err)
	}

	if len(p.SatelliteAddr) == 0 {
		return nil, errors.New("access grant is missing satellite address")
	}

	apiKey, err := macaroon.ParseRawAPIKey(p.ApiKey)
	if err != nil {
		return nil, fmt.Errorf("access grant has malformed api key: %w", err)
	}

	encAccess, err := parseEncryptionAccessFromProto(p.EncryptionAccess)
	if err != nil {
		return nil, fmt.Errorf("access grant has malformed encryption access: %w", err)
	}
	encAccess.LimitTo(apiKey)

	return &Access{
		SatelliteAddress: p.SatelliteAddr,
		APIKey:           apiKey,
		EncAccess:        encAccess,
	}, nil
}

// Serialize serializes an access grant such that it can be used later with
// ParseAccess or other tools.
func (access *Access) Serialize() (string, error) {
	switch {
	case len(access.SatelliteAddress) == 0:
		return "", errors.New("access grant is missing satellite address")
	case access.APIKey == nil:
		return "", errors.New("access grant is missing api key")
	case access.EncAccess == nil:
		return "", errors.New("access grant is missing encryption access")
	}

	enc, err := access.EncAccess.toProto()
	if err != nil {
		return "", err
	}

	data, err := pb.Marshal(&pb.Scope{
		SatelliteAddr:    access.SatelliteAddress,
		ApiKey:           access.APIKey.SerializeRaw(),
		EncryptionAccess: enc,
	})
	if err != nil {
		return "", fmt.Errorf("unable to marshal access grant: %w", err)
	}

	return base58.CheckEncode(data, 0), nil
}

// EncryptionAccess represents an encryption access context. It holds
// information about how various buckets and objects should be
// encrypted and decrypted.
type EncryptionAccess struct {
	Store *encryption.Store
}

// NewEncryptionAccess creates an encryption access context.
func NewEncryptionAccess() *EncryptionAccess {
	store := encryption.NewStore()
	return &EncryptionAccess{Store: store}
}

// NewEncryptionAccessWithDefaultKey creates an encryption access context with
// a default key set.
// Use (*Project).SaltedKeyFromPassphrase to generate a default key.
func NewEncryptionAccessWithDefaultKey(defaultKey *storj.Key) *EncryptionAccess {
	ec := NewEncryptionAccess()
	ec.SetDefaultKey(defaultKey)
	return ec
}

// SetDefaultKey sets the default key for the encryption access context.
// Use (*Project).SaltedKeyFromPassphrase to generate a default key.
func (s *EncryptionAccess) SetDefaultKey(defaultKey *storj.Key) {
	s.Store.SetDefaultKey(defaultKey)
}

// SetDefaultPathCipher sets which cipher suite to use by default.
func (s *EncryptionAccess) SetDefaultPathCipher(defaultPathCipher storj.CipherSuite) {
	s.Store.SetDefaultPathCipher(defaultPathCipher)
}

// LimitTo limits the data in the encryption access only to the paths that would be
// allowed by the api key. Any errors that happen due to the consistency of the api
// key cause no keys to be stored.
func (s *EncryptionAccess) LimitTo(apiKey *macaroon.APIKey) {
	store, err := s.limitTo(apiKey)
	if err != nil {
		store = encryption.NewStore()
	}
	s.Store = store
}

// collapsePrefixes collapses the caveat paths in a macaroon into a single list of valid
// prefixes. This function should move to be exported in the common repo at some point.
func collapsePrefixes(mac *macaroon.Macaroon) ([]*macaroon.Caveat_Path, bool, error) {
	isAllowedByGroup := func(cav *macaroon.Caveat_Path, group []*macaroon.Caveat_Path) bool {
		for _, other := range group {
			if bytes.Equal(cav.Bucket, other.Bucket) &&
				bytes.HasPrefix(cav.EncryptedPathPrefix, other.EncryptedPathPrefix) {
				return true
			}
		}
		return false
	}

	isAllowed := func(cav *macaroon.Caveat_Path, groups [][]*macaroon.Caveat_Path) bool {
		for _, group := range groups {
			if !isAllowedByGroup(cav, group) {
				return false
			}
		}
		return true
	}

	// load all of the groups and prefixes from the caveats
	var groups [][]*macaroon.Caveat_Path
	var prefixes []*macaroon.Caveat_Path
	for _, cavData := range mac.Caveats() {
		var cav macaroon.Caveat
		if err := cav.UnmarshalBinary(cavData); err != nil {
			return nil, false, err
		}
		if len(cav.AllowedPaths) > 0 {
			groups = append(groups, cav.AllowedPaths)
			prefixes = append(prefixes, cav.AllowedPaths...)
		}
	}

	// if we have no groups/prefixes, then there are no path restrictions.
	if len(groups) == 0 || len(prefixes) == 0 {
		return nil, false, nil
	}

	// filter the prefixes by if every group allows them
	j := 0
	for i, prefix := range prefixes {
		if !isAllowed(prefix, groups) {
			continue
		}
		prefixes[j] = prefixes[i]
		j++
	}
	prefixes = prefixes[:j]

	return prefixes, true, nil
}

// limitTo returns the store that the access should be limited to and any error that
// happened during processing. If there should be no limits, then it returns the
// pointer identical store that currently exists on the EncryptionAccess. Otherwise
// the returned store is a new value.
func (s *EncryptionAccess) limitTo(apiKey *macaroon.APIKey) (*encryption.Store, error) {
	// This is a bit hacky. We may want to export some stuff.
	data := apiKey.SerializeRaw()
	mac, err := macaroon.ParseMacaroon(data)
	if err != nil {
		return nil, err
	}

	// collapse the prefixes of the macaroon into a list of valid ones and if there were
	// any restrictions at all.
	prefixes, restricted, err := collapsePrefixes(mac)
	if err != nil {
		return nil, err
	}

	// if we have no restrictions, we're done. we can return the same store that we have.
	if !restricted {
		return s.Store, nil
	}

	// create the new store that we'll load into and carry some necessary defaults
	store := encryption.NewStore()
	store.SetDefaultPathCipher(s.Store.GetDefaultPathCipher()) // keep default path cipher

	// add the prefixes to the store, skipping any that fail for any reason
	for _, prefix := range prefixes {
		bucket := string(prefix.Bucket)
		encPath := paths.NewEncrypted(string(prefix.EncryptedPathPrefix))

		unencPath, err := encryption.DecryptPathWithStoreCipher(bucket, encPath, s.Store)
		if err != nil {
			continue
		}
		key, err := encryption.DerivePathKey(bucket, unencPath, s.Store)
		if err != nil {
			continue
		}

		// we have to unfortunately look up the cipher again because the Decrypt function
		// does not return it.
		_, _, base := s.Store.LookupEncrypted(bucket, encPath)
		if base == nil {
			continue // this should not happen given Decrypt succeeded, but whatever
		}

		if err := store.AddWithCipher(bucket, unencPath, encPath, *key, base.PathCipher); err != nil {
			continue
		}
	}

	return store, nil
}

func (s *EncryptionAccess) toProto() (*pb.EncryptionAccess, error) {
	var storeEntries []*pb.EncryptionAccess_StoreEntry
	err := s.Store.IterateWithCipher(func(bucket string, unenc paths.Unencrypted, enc paths.Encrypted, key storj.Key, pathCipher storj.CipherSuite) error {
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
		return nil, err
	}

	var defaultKey []byte
	if key := s.Store.GetDefaultKey(); key != nil {
		defaultKey = key[:]
	}

	return &pb.EncryptionAccess{
		DefaultKey:        defaultKey,
		StoreEntries:      storeEntries,
		DefaultPathCipher: pb.CipherSuite(s.Store.GetDefaultPathCipher()),
	}, nil
}

func parseEncryptionAccessFromProto(p *pb.EncryptionAccess) (*EncryptionAccess, error) {
	access := NewEncryptionAccess()
	if len(p.DefaultKey) > 0 {
		if len(p.DefaultKey) != len(storj.Key{}) {
			return nil, errors.New("invalid default key in encryption access")
		}
		var defaultKey storj.Key
		copy(defaultKey[:], p.DefaultKey)
		access.SetDefaultKey(&defaultKey)
	}

	access.SetDefaultPathCipher(storj.CipherSuite(p.DefaultPathCipher))
	// Unspecified cipher suite means that most probably access was serialized
	// before path cipher was moved to encryption access
	if p.DefaultPathCipher == pb.CipherSuite_ENC_UNSPECIFIED {
		access.SetDefaultPathCipher(storj.EncAESGCM)
	}

	for _, entry := range p.StoreEntries {
		if len(entry.Key) != len(storj.Key{}) {
			return nil, errors.New("invalid key in encryption access entry")
		}
		var key storj.Key
		copy(key[:], entry.Key)

		err := access.Store.AddWithCipher(
			string(entry.Bucket),
			paths.NewUnencrypted(string(entry.UnencryptedPath)),
			paths.NewEncrypted(string(entry.EncryptedPath)),
			key,
			storj.CipherSuite(entry.PathCipher),
		)
		if err != nil {
			return nil, fmt.Errorf("invalid encryption access entry: %w", err)
		}
	}

	return access, nil
}
