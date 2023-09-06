// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"context"
	"io"
	_ "unsafe" // for go:linkname

	"storj.io/common/encryption"
	"storj.io/common/paths"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/uplink/private/eestream"
	"storj.io/uplink/private/metaclient"
)

// dialMetainfoClient is exposing project.dialMetainfoClient method.
//
// NB: this is used with linkname in private/multipart.
// It needs to be updated when this is updated.
//
//lint:ignore U1000, used with linkname
//nolint:deadcode,unused
//go:linkname dialMetainfoClient
func dialMetainfoClient(ctx context.Context, project *Project) (_ *metaclient.Client, err error) {
	return project.dialMetainfoClient(ctx)
}

// dialMetainfoClient is exposing project encryptionParameters field.
//
// NB: this is used with linkname in private/multipart.
// It needs to be updated when this is updated.
//
//lint:ignore U1000, used with linkname
//nolint:deadcode,unused
//go:linkname encryptionParameters
func encryptionParameters(project *Project) storj.EncryptionParameters {
	return project.encryptionParameters
}

// segmentSize is exposing project segmentSize field.
//
// NB: this is used with linkname in private/multipart.
// It needs to be updated when this is updated.
//
//lint:ignore U1000, used with linkname
//nolint:deadcode,unused
//go:linkname segmentSize
func segmentSize(project *Project) int64 {
	return project.segmentSize
}

// encryptPath is exposing helper method to encrypt path with project internals.
//
// NB: this is used with linkname in private/multipart.
// It needs to be updated when this is updated.
//
//lint:ignore U1000, used with linkname
//nolint:unused
//go:linkname encryptPath
func encryptPath(project *Project, bucket, key string) (paths.Encrypted, error) {
	encStore := project.access.encAccess.Store
	encPath, err := encryption.EncryptPathWithStoreCipher(bucket, paths.NewUnencrypted(key), encStore)
	return encPath, err
}

// deriveContentKey is exposing helper method to derive content key with project internals.
//
// NB: this is used with linkname in private/multipart.
// It needs to be updated when this is updated.
//
//lint:ignore U1000, used with linkname
//nolint:unused
//go:linkname deriveContentKey
func deriveContentKey(project *Project, bucket, key string) (*storj.Key, error) {
	encStore := project.access.encAccess.Store
	derivedKey, err := encryption.DeriveContentKey(bucket, paths.NewUnencrypted(key), encStore)
	return derivedKey, err
}

// ecPutSingleResult is exposing ec client PutSingleResult method.
//
// NB: this is used with linkname in private/multipart.
// It needs to be updated when this is updated.
//
//lint:ignore U1000, used with linkname
//nolint:deadcode,unused
//go:linkname ecPutSingleResult
func ecPutSingleResult(ctx context.Context, project *Project, limits []*pb.AddressedOrderLimit, privateKey storj.PiecePrivateKey,
	rs eestream.RedundancyStrategy, data io.Reader) (results []*pb.SegmentPieceUploadResult, err error) {
	return project.ec.PutSingleResult(ctx, limits, privateKey, rs, data)
}

// dialMetainfoDB is exposing project.dialMetainfoDB method.
//
// NB: this is used with linkname in private/multipart.
// It needs to be updated when this is updated.
//
//lint:ignore U1000, used with linkname
//nolint:unused
//go:linkname dialMetainfoDB
func dialMetainfoDB(ctx context.Context, project *Project) (_ *metaclient.DB, err error) {
	return project.dialMetainfoDB(ctx)
}
