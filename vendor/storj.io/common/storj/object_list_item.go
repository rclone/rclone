// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

import (
	"time"
)

// ObjectListItem represents listed object
type ObjectListItem struct {
	EncryptedPath          []byte
	Version                int32
	Status                 int32
	CreatedAt              time.Time
	StatusAt               time.Time
	ExpiresAt              time.Time
	EncryptedMetadataNonce Nonce
	EncryptedMetadata      []byte
	IsPrefix               bool
}
