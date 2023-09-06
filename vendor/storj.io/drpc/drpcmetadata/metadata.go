// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcmetadata

import (
	"context"

	"github.com/zeebo/errs"
)

// AddPairs attaches metadata onto a context and return the context.
func AddPairs(ctx context.Context, metadata map[string]string) context.Context {
	for key, val := range metadata {
		ctx = Add(ctx, key, val)
	}
	return ctx
}

// Encode generates byte form of the metadata and appends it onto the passed in buffer.
func Encode(buf []byte, metadata map[string]string) ([]byte, error) {
	for key, value := range metadata {
		buf = appendEntry(buf, key, value)
	}
	return buf, nil
}

// Decode translate byte form of metadata into key/value metadata.
func Decode(buf []byte) (map[string]string, error) {
	var out map[string]string
	var key, value []byte
	var ok bool
	var err error

	for len(buf) > 0 {
		buf, key, value, ok, err = readEntry(buf)
		if err != nil {
			return nil, err
		} else if !ok {
			return nil, errs.New("invalid data")
		}
		if out == nil {
			out = make(map[string]string)
		}
		out[string(key)] = string(value)
	}

	return out, nil
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
