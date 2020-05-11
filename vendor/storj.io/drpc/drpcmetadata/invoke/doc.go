// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// Package invoke defines the proto messages exposed by drpc for
// sending metadata across the wire.
package invoke

//go:generate bash -c "go install storj.io/drpc/cmd/protoc-gen-drpc && protoc --drpc_out=plugins=drpc:. metadata.proto"
