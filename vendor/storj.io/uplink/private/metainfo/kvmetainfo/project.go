// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kvmetainfo

import (
	"storj.io/uplink/private/metainfo"
	"storj.io/uplink/private/storage/streams"
)

// Project implements project management operations.
type Project struct {
	metainfo           metainfo.Client
	streams            streams.Store
	encryptedBlockSize int32
	segmentsSize       int64
}

// NewProject constructs a *Project.
func NewProject(streams streams.Store, encryptedBlockSize int32, segmentsSize int64, metainfo metainfo.Client) *Project {
	return &Project{
		metainfo:           metainfo,
		streams:            streams,
		encryptedBlockSize: encryptedBlockSize,
		segmentsSize:       segmentsSize,
	}
}
