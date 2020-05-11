// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// TODO maybe there is better place for this

package fpath

import "context"

// The key type is unexported to prevent collisions with context keys defined in
// other packages.
type key int

// temp is the context key for temp struct
const tempKey key = 0

type temp struct {
	inmemory  bool
	directory string
}

// WithTempData creates context with information how store temporary data, in memory or on disk
func WithTempData(ctx context.Context, directory string, inmemory bool) context.Context {
	temp := temp{
		inmemory:  inmemory,
		directory: directory,
	}
	return context.WithValue(ctx, tempKey, temp)
}

// GetTempData returns if temporary data should be stored in memory or on disk
func GetTempData(ctx context.Context) (string, bool, bool) {
	tempValue, ok := ctx.Value(tempKey).(temp)
	if !ok {
		return "", false, false
	}
	return tempValue.directory, tempValue.inmemory, ok
}
