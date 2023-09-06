// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"context"
	"crypto/rand"
	"strings"

	"github.com/zeebo/errs"

	"storj.io/common/encryption"
	"storj.io/common/storj"
	"storj.io/uplink/private/metaclient"
)

// MoveObjectOptions options for MoveObject method.
type MoveObjectOptions struct {
	// may contain StreamID and Version in the future
}

// MoveObject moves object to a different bucket or/and key.
func (project *Project) MoveObject(ctx context.Context, oldbucket, oldkey, newbucket, newkey string, options *MoveObjectOptions) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = validateMoveCopyInput(oldbucket, oldkey, newbucket, newkey)
	if err != nil {
		return packageError.Wrap(err)
	}

	oldEncKey, err := encryptPath(project, oldbucket, oldkey)
	if err != nil {
		return packageError.Wrap(err)
	}

	newEncKey, err := encryptPath(project, newbucket, newkey)
	if err != nil {
		return packageError.Wrap(err)
	}

	metainfoClient, err := project.dialMetainfoClient(ctx)
	if err != nil {
		return packageError.Wrap(err)
	}
	defer func() { err = errs.Combine(err, metainfoClient.Close()) }()

	response, err := metainfoClient.BeginMoveObject(ctx, metaclient.BeginMoveObjectParams{
		Bucket:                []byte(oldbucket),
		EncryptedObjectKey:    []byte(oldEncKey.Raw()),
		NewBucket:             []byte(newbucket),
		NewEncryptedObjectKey: []byte(newEncKey.Raw()),
	})
	if err != nil {
		return convertKnownErrors(err, oldbucket, oldkey)
	}

	oldDerivedKey, err := deriveContentKey(project, oldbucket, oldkey)
	if err != nil {
		return packageError.Wrap(err)
	}

	newDerivedKey, err := deriveContentKey(project, newbucket, newkey)
	if err != nil {
		return packageError.Wrap(err)
	}

	newMetadataEncryptedKey, newMetadataKeyNonce, err := project.reencryptMetadataKey(response.EncryptedMetadataKey, response.EncryptedMetadataKeyNonce, oldDerivedKey, newDerivedKey)
	if err != nil {
		return packageError.Wrap(err)
	}

	newKeys, err := project.reencryptKeys(response.SegmentKeys, oldDerivedKey, newDerivedKey)
	if err != nil {
		return packageError.Wrap(err)
	}

	err = metainfoClient.FinishMoveObject(ctx, metaclient.FinishMoveObjectParams{
		StreamID:                     response.StreamID,
		NewBucket:                    []byte(newbucket),
		NewEncryptedObjectKey:        []byte(newEncKey.Raw()),
		NewEncryptedMetadataKeyNonce: newMetadataKeyNonce,
		NewEncryptedMetadataKey:      newMetadataEncryptedKey,
		NewSegmentKeys:               newKeys,
	})
	return convertKnownErrors(err, oldbucket, oldkey)
}

func validateMoveCopyInput(oldbucket, oldkey, newbucket, newkey string) error {
	switch {
	case oldbucket == "":
		return errwrapf("%w (%q)", ErrBucketNameInvalid, oldbucket)
	case oldkey == "":
		return errwrapf("%w (%q)", ErrObjectKeyInvalid, oldkey)
	case strings.HasSuffix(oldkey, "/"):
		return packageError.New("oldkey cannot be a prefix")
	case newbucket == "": // TODO should we make this error different
		return errwrapf("%w (%q)", ErrBucketNameInvalid, newbucket)
	case newkey == "": // TODO should we make this error different
		return errwrapf("%w (%q)", ErrObjectKeyInvalid, newkey)
	case strings.HasSuffix(newkey, "/"):
		return packageError.New("newkey cannot be a prefix")
	}

	return nil
}

func (project *Project) reencryptKeys(keys []metaclient.EncryptedKeyAndNonce, oldDerivedKey, newDerivedKey *storj.Key) ([]metaclient.EncryptedKeyAndNonce, error) {
	cipherSuite := project.encryptionParameters.CipherSuite

	newKeys := make([]metaclient.EncryptedKeyAndNonce, len(keys))
	for i, oldKey := range keys {
		// decrypt old key
		contentKey, err := encryption.DecryptKey(oldKey.EncryptedKey, cipherSuite, oldDerivedKey, &oldKey.EncryptedKeyNonce)
		if err != nil {
			return nil, packageError.Wrap(err)
		}

		// create new random nonce and encrypt
		var newEncryptedKeyNonce storj.Nonce
		// generate random nonce for encrypting the content key
		_, err = rand.Read(newEncryptedKeyNonce[:])
		if err != nil {
			return nil, packageError.Wrap(err)
		}

		newEncryptedKey, err := encryption.EncryptKey(contentKey, cipherSuite, newDerivedKey, &newEncryptedKeyNonce)
		if err != nil {
			return nil, packageError.Wrap(err)
		}

		newKeys[i] = metaclient.EncryptedKeyAndNonce{
			Position:          oldKey.Position,
			EncryptedKeyNonce: newEncryptedKeyNonce,
			EncryptedKey:      newEncryptedKey,
		}
	}

	return newKeys, nil
}

func (project *Project) reencryptMetadataKey(encryptedMetadataKey []byte, encryptedMetadataKeyNonce storj.Nonce, oldDerivedKey, newDerivedKey *storj.Key) ([]byte, storj.Nonce, error) {
	if len(encryptedMetadataKey) == 0 {
		return nil, storj.Nonce{}, nil
	}

	cipherSuite := project.encryptionParameters.CipherSuite

	// decrypt old metadata key
	metadataContentKey, err := encryption.DecryptKey(encryptedMetadataKey, cipherSuite, oldDerivedKey, &encryptedMetadataKeyNonce)
	if err != nil {
		return nil, storj.Nonce{}, packageError.Wrap(err)
	}

	// encrypt metadata content key with new derived key and old nonce
	newMetadataKeyNonce := encryptedMetadataKeyNonce
	newMetadataEncryptedKey, err := encryption.EncryptKey(metadataContentKey, cipherSuite, newDerivedKey, &newMetadataKeyNonce)
	if err != nil {
		return nil, storj.Nonce{}, packageError.Wrap(err)
	}

	return newMetadataEncryptedKey, newMetadataKeyNonce, nil
}
