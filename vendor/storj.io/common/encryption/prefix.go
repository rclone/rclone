// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package encryption

import (
	"github.com/zeebo/errs"

	"storj.io/common/paths"
	"storj.io/common/storj"
)

// PrefixInfo is a helper type that contains all of the encrypted and unencrypted paths related
// to some path and its parent. It includes the cipher that was used to encrypt and decrypt
// the paths and what bucket it is in.
type PrefixInfo struct {
	Bucket string
	Cipher storj.CipherSuite

	PathUnenc paths.Unencrypted
	PathEnc   paths.Encrypted
	PathKey   storj.Key

	ParentUnenc paths.Unencrypted
	ParentEnc   paths.Encrypted
	ParentKey   storj.Key
}

// GetPrefixInfo returns the PrefixInfo for some unencrypted path inside of a bucket.
func GetPrefixInfo(bucket string, path paths.Unencrypted, store *Store) (pi *PrefixInfo, err error) {
	_, remaining, base := store.LookupUnencrypted(bucket, path)
	if base == nil {
		return nil, ErrMissingEncryptionBase.New("%q/%q", bucket, path)
	}

	if path.Valid() && remaining.Done() {
		return nil, ErrMissingEncryptionBase.New("no parent: %q/%q", bucket, path)
	}

	// if we're using the default base (meaning the default key), we need
	// to include the bucket name in the path derivation.
	key := &base.Key
	if base.Default {
		key, err = derivePathKeyComponent(key, bucket)
		if err != nil {
			return nil, errs.Wrap(err)
		}
	}

	var (
		pathUnenc   pathBuilder
		pathEnc     pathBuilder
		parentUnenc pathBuilder
		parentEnc   pathBuilder
	)

	pathKey := *key
	parentKey := *key

	if !base.Default && base.Encrypted.Valid() {
		pathUnenc.append(base.Unencrypted.Raw())
		pathEnc.append(base.Encrypted.Raw())
		parentUnenc.append(base.Unencrypted.Raw())
		parentEnc.append(base.Encrypted.Raw())
	}

	var componentUnenc string
	var componentEnc string

	for i := 0; !remaining.Done(); i++ {
		if i > 0 {
			parentKey = *key
			parentEnc.append(componentEnc)
			parentUnenc.append(componentUnenc)
		}

		componentUnenc = remaining.Next()

		componentEnc, err = encryptPathComponent(componentUnenc, base.PathCipher, key)
		if err != nil {
			return nil, errs.Wrap(err)
		}
		key, err = derivePathKeyComponent(key, componentUnenc)
		if err != nil {
			return nil, errs.Wrap(err)
		}

		pathKey = *key
		pathUnenc.append(componentUnenc)
		pathEnc.append(componentEnc)
	}

	return &PrefixInfo{
		Bucket: bucket,
		Cipher: base.PathCipher,

		PathKey:   pathKey,
		PathUnenc: pathUnenc.Unencrypted(),
		PathEnc:   pathEnc.Encrypted(),

		ParentKey:   parentKey,
		ParentUnenc: parentUnenc.Unencrypted(),
		ParentEnc:   parentEnc.Encrypted(),
	}, nil
}
