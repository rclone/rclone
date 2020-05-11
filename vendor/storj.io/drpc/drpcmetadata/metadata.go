// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcmetadata

import (
	"context"

	"github.com/gogo/protobuf/proto"

	"storj.io/drpc/drpcmetadata/invoke"
)

// AddPairs attaches metadata onto a context and return the context.
func AddPairs(ctx context.Context, md map[string]string) context.Context {
	if len(md) < 1 {
		return ctx
	}

	for key, val := range md {
		ctx = Add(ctx, key, val)
	}

	return ctx
}

// Encode generates byte form of the metadata and appends it onto the passed in buffer.
func Encode(buffer []byte, md map[string]string) ([]byte, error) {
	if len(md) < 1 {
		return buffer, nil
	}

	msg := invoke.Metadata{
		Data: md,
	}

	msgBytes, err := proto.Marshal(&msg)
	if err != nil {
		return buffer, err
	}

	buffer = append(buffer, msgBytes...)

	return buffer, nil
}

// Decode translate byte form of metadata into key/value metadata.
func Decode(data []byte) (map[string]string, error) {
	if len(data) < 1 {
		return map[string]string{}, nil
	}

	msg := invoke.Metadata{}
	err := proto.Unmarshal(data, &msg)
	if err != nil {
		return nil, err
	}

	return msg.Data, nil
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
