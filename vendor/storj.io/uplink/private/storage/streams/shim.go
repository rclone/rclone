// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package streams

import (
	"context"
	"io"
	"time"

	"storj.io/common/encryption"
	"storj.io/common/ranger"
	"storj.io/common/storj"
	"storj.io/uplink/private/metainfo"
	"storj.io/uplink/private/storage/segments"
)

// Metadata interface returns the latest metadata for an object.
type Metadata interface {
	Metadata() ([]byte, error)
}

// Store interface methods for streams to satisfy to be a store.
type Store interface {
	Get(ctx context.Context, path storj.Path, object storj.Object) (ranger.Ranger, error)
	Put(ctx context.Context, path storj.Path, data io.Reader, metadata Metadata, expiration time.Time) (Meta, error)
}

type shimStore struct {
	store typedStore
}

// NewStreamStore constructs a Store.
func NewStreamStore(metainfo *metainfo.Client, segments segments.Store, segmentSize int64, encStore *encryption.Store, encBlockSize int, cipher storj.CipherSuite, inlineThreshold int, maxEncryptedSegmentSize int64) (Store, error) {
	typedStore, err := newTypedStreamStore(metainfo, segments, segmentSize, encStore, encBlockSize, cipher, inlineThreshold, maxEncryptedSegmentSize)
	if err != nil {
		return nil, err
	}
	return &shimStore{store: typedStore}, nil
}

// Get parses the passed in path and dispatches to the typed store.
func (s *shimStore) Get(ctx context.Context, path storj.Path, object storj.Object) (_ ranger.Ranger, err error) {
	defer mon.Task()(&ctx)(&err)

	return s.store.Get(ctx, ParsePath(path), object)
}

// Put parses the passed in path and dispatches to the typed store.
func (s *shimStore) Put(ctx context.Context, path storj.Path, data io.Reader, metadata Metadata, expiration time.Time) (_ Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	return s.store.Put(ctx, ParsePath(path), data, metadata, expiration)
}
