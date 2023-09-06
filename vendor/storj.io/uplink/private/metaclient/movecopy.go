// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package metaclient

import (
	"storj.io/common/pb"
	"storj.io/common/storj"
)

// EncryptedKeyAndNonce holds single segment encrypted key.
type EncryptedKeyAndNonce struct {
	Position          SegmentPosition
	EncryptedKeyNonce storj.Nonce
	EncryptedKey      []byte
}

func convertKeys(input []*pb.EncryptedKeyAndNonce) []EncryptedKeyAndNonce {
	keys := make([]EncryptedKeyAndNonce, len(input))
	for i, key := range input {
		keys[i] = EncryptedKeyAndNonce{
			EncryptedKeyNonce: key.EncryptedKeyNonce,
			EncryptedKey:      key.EncryptedKey,
		}
		if key.Position != nil {
			keys[i].Position = SegmentPosition{
				PartNumber: key.Position.PartNumber,
				Index:      key.Position.Index,
			}
		}
	}

	return keys
}
