// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcmetadata

import (
	"context"

	"github.com/gogo/protobuf/proto"

	"storj.io/drpc/drpcmetadata/invoke"
)

// AddPairs attaches metadata onto a context and return the context.
func AddPairs(ctx context.Context, metadata map[string]string) context.Context {
	for key, val := range metadata {
		ctx = Add(ctx, key, val)
	}
	return ctx
}

// Encode generates byte form of the metadata and appends it onto the passed in buffer.
func Encode(buffer []byte, metadata map[string]string) ([]byte, error) {
	data, err := proto.Marshal(&invoke.Metadata{Data: metadata})
	if err != nil {
		return buffer, err
	}
	return append(buffer, data...), nil
}

// Decode translate byte form of metadata into key/value metadata.
func Decode(data []byte) (map[string]string, error) {
	var md invoke.Metadata
	err := proto.Unmarshal(data, &md)
	if err != nil {
		return nil, err
	}
	return md.Data, nil
}

type metadataKey struct{}

// Add associates a key/value pair on the context.
func Add(ctx context.Context, key, value string) context.Context {
	metadata, ok := Get(ctx)
	if !ok {
		metadata = make(map[string]string)
		ctx = context.WithValue(ctx, metadataKey{}, metadata)
	}
	metadata[key] = value
	return ctx
}

// Get returns all key/value pairs on the given context.
func Get(ctx context.Context) (map[string]string, bool) {
	metadata, ok := ctx.Value(metadataKey{}).(map[string]string)
	return metadata, ok
}
