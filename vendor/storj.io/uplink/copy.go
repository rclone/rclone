// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/uplink/private/metaclient"
)

// CopyObjectOptions options for CopyObject method.
type CopyObjectOptions struct {
	// may contain additional options in the future
}

// CopyObject atomically copies object to a different bucket or/and key.
func (project *Project) CopyObject(ctx context.Context, oldBucket, oldKey, newBucket, newKey string, options *CopyObjectOptions) (_ *Object, err error) {
	defer mon.Task()(&ctx)(&err)

	err = validateMoveCopyInput(oldBucket, oldKey, newBucket, newKey)
	if err != nil {
		return nil, packageError.Wrap(err)
	}

	oldEncKey, err := encryptPath(project, oldBucket, oldKey)
	if err != nil {
		return nil, packageError.Wrap(err)
	}

	newEncKey, err := encryptPath(project, newBucket, newKey)
	if err != nil {
		return nil, packageError.Wrap(err)
	}

	metainfoClient, err := project.dialMetainfoClient(ctx)
	if err != nil {
		return nil, packageError.Wrap(err)
	}
	defer func() { err = errs.Combine(err, metainfoClient.Close()) }()

	response, err := metainfoClient.BeginCopyObject(ctx, metaclient.BeginCopyObjectParams{
		Bucket:                []byte(oldBucket),
		EncryptedObjectKey:    []byte(oldEncKey.Raw()),
		NewBucket:             []byte(newBucket),
		NewEncryptedObjectKey: []byte(newEncKey.Raw()),
	})
	if err != nil {
		return nil, convertKnownErrors(err, oldBucket, oldKey)
	}

	oldDerivedKey, err := deriveContentKey(project, oldBucket, oldKey)
	if err != nil {
		return nil, packageError.Wrap(err)
	}

	newDerivedKey, err := deriveContentKey(project, newBucket, newKey)
	if err != nil {
		return nil, packageError.Wrap(err)
	}

	newMetadataEncryptedKey, newMetadataKeyNonce, err := project.reencryptMetadataKey(response.EncryptedMetadataKey, response.EncryptedMetadataKeyNonce, oldDerivedKey, newDerivedKey)
	if err != nil {
		return nil, packageError.Wrap(err)
	}

	newKeys, err := project.reencryptKeys(response.SegmentKeys, oldDerivedKey, newDerivedKey)
	if err != nil {
		return nil, packageError.Wrap(err)
	}

	obj, err := metainfoClient.FinishCopyObject(ctx, metaclient.FinishCopyObjectParams{
		StreamID:                     response.StreamID,
		NewBucket:                    []byte(newBucket),
		NewEncryptedObjectKey:        []byte(newEncKey.Raw()),
		NewEncryptedMetadataKeyNonce: newMetadataKeyNonce,
		NewEncryptedMetadataKey:      newMetadataEncryptedKey,
		NewSegmentKeys:               newKeys,
	})
	if err != nil {
		return nil, packageError.Wrap(err)
	}

	db, err := project.dialMetainfoDB(ctx)
	if err != nil {
		return nil, packageError.Wrap(err)
	}
	defer func() { err = errs.Combine(err, db.Close()) }()

	info, err := db.ObjectFromRawObjectItem(ctx, newBucket, newKey, obj.Info)
	if err != nil {
		return nil, packageError.Wrap(err)
	}

	return convertObject(&info), convertKnownErrors(err, oldBucket, oldKey)
}
