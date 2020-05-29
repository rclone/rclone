// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package drpccache

import (
	"context"
	"sync"
)

type cacheKey struct{}

// Cache is a per stream cache.
type Cache struct {
	mu     sync.Mutex
	values map[interface{}]interface{}
}

// New returns a new cache.
func New() *Cache { return &Cache{} }

// FromContext returns a cache from a context.
//
// Example usage:
//
// 	cache := drpccache.FromContext(stream.Context())
// 	if cache != nil {
// 	       value := cache.LoadOrCreate("initialized", func() (interface{}) {
// 	               return 42
// 	       })
// 	}
func FromContext(ctx context.Context) *Cache {
	value := ctx.Value(cacheKey{})
	if value == nil {
		return nil
	}

	cache, ok := value.(*Cache)
	if !ok {
		return nil
	}

	return cache
}

// WithContext returns a context with the value cache associated with the context.
func WithContext(parent context.Context, cache *Cache) context.Context {
	return context.WithValue(parent, cacheKey{}, cache)
}

// init ensures that the values map exist.
func (cache *Cache) init() {
	if cache.values == nil {
		cache.values = map[interface{}]interface{}{}
	}
}

// Clear clears the cache.
func (cache *Cache) Clear() {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	cache.values = nil
}

// Store sets the value at a key.
func (cache *Cache) Store(key, value interface{}) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	cache.init()
	cache.values[key] = value
}

// Load returns the value with the given key.
func (cache *Cache) Load(key interface{}) interface{} {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	if cache.values == nil {
		return nil
	}

	return cache.values[key]
}

// LoadOrCreate returns the value with the given key.
func (cache *Cache) LoadOrCreate(key interface{}, fn func() interface{}) interface{} {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	cache.init()

	value, ok := cache.values[key]
	if !ok {
		value = fn()
		cache.values[key] = value
	}

	return value
}
