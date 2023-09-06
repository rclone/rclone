// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package splitter

import (
	"storj.io/common/encryption"
	"storj.io/common/storj"
	"storj.io/uplink/private/metaclient"
)

// TODO: move it to separate package?
func encryptETag(etag []byte, cipherSuite storj.CipherSuite, contentKey *storj.Key) ([]byte, error) {
	etagKey, err := encryption.DeriveKey(contentKey, "storj-etag-v1")
	if err != nil {
		return nil, err
	}

	encryptedETag, err := encryption.Encrypt(etag, cipherSuite, etagKey, &storj.Nonce{})
	if err != nil {
		return nil, err
	}

	return encryptedETag, nil
}

func nonceForPosition(position metaclient.SegmentPosition) (storj.Nonce, error) {
	var nonce storj.Nonce
	inc := (int64(position.PartNumber) << 32) | (int64(position.Index) + 1)
	_, err := encryption.Increment(&nonce, inc)
	return nonce, err
}
