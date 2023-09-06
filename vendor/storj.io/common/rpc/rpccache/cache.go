// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package rpccache

import (
	"sync"
	"time"

	"github.com/zeebo/errs"
)

// implementation note: the cache has some methods that could
// potentially be quadratic in the worst case. specifically
// when there are many stale entries in the list of values.
// while we could do a single pass filtering stale entries, the
// logic is a bit harder to follow and ensure is correct. instead
// we have a helper to remove a single entry from a list without
// knowing where it came from. since we can possibly call that
// to remove every element from a list, it's quadratic in the
// maximum size of that list. since this cache is intended to
// be used with small key capacities (like 5), the decision was
// made to accept that quadratic worst case for the benefit of
// having as simple an implementation as possible.

//
// options
//

// Options contains the options to configure a cache.
type Options struct {
	// Expiration will remove any values from the Cache after the
	// value passes. Zero means no expiration.
	Expiration time.Duration

	// Capacity is the maximum number of values the Cache can store.
	// Zero means unlimited. Negative means no values.
	Capacity int

	// KeyCapacity is like Capacity except it is per key. Zero means
	// the Cache holds unlimited for any single key. Negative means
	// no values for any single key.
	//
	// Implementation note: The cache is potentially quadratic in the
	// size of this parameter, so it is intended for small values, like
	// 5 or so.
	KeyCapacity int

	// Stale is optionally called on values before they are returned
	// to see if they should be discarded. Nil means no check is made.
	Stale func(interface{}) bool

	// Close is optionally called on any value removed from the Cache.
	Close func(interface{}) error

	// Unblocked is optional and called on values before they are returned
	// to see if they are available to be used. Nil means no check is made.
	Unblocked func(interface{}) bool
}

func (c Options) close(val interface{}) error {
	if c.Close == nil {
		return nil
	}
	return c.Close(val)
}

func (c Options) stale(val interface{}) bool {
	if c.Stale == nil {
		return false
	}
	return c.Stale(val)
}

func (c Options) unblocked(val interface{}) bool {
	if c.Unblocked == nil {
		return true
	}
	return c.Unblocked(val)
}

//
// cache
//

type entry struct {
	key interface{}
	val interface{}
	exp *time.Timer
}

// Cache is an expiring, stale-checking LRU of keys to multiple values.
type Cache struct {
	opts    Options
	mu      sync.Mutex
	entries map[interface{}][]*entry
	order   []*entry
	closed  bool
}

// New constructs a new Cache with the Options.
func New(opts Options) *Cache {
	return &Cache{
		opts:    opts,
		entries: make(map[interface{}][]*entry),
	}
}

//
// helpers
//

// closeEntry ensures the timer and connection are closed, returning
// any errors.
func (c *Cache) closeEntry(ent *entry) error {
	if ent.exp == nil || ent.exp.Stop() {
		return c.opts.close(ent.val)
	}
	return nil
}

// filterEntry is a helper to remove a specific cache entry from
// a slice of entries.
func filterEntry(entries []*entry, ent *entry) []*entry {
	for i := range entries {
		if entries[i] == ent {
			copy(entries[i:], entries[i+1:])
			return entries[:len(entries)-1]
		}
	}
	return entries
}

//
// filter functions to remove entries
//

// filterEntryLocked removes the entry from the map, deleting the
// map key if necessary.
//
// It should only be called with the mutex held.
func (c *Cache) filterEntryLocked(ent *entry) {
	entries := c.entries[ent.key]
	if len(entries) <= 1 {
		delete(c.entries, ent.key)
	} else {
		c.entries[ent.key] = filterEntry(entries, ent)
	}
	c.order = filterEntry(c.order, ent)
}

// filterCacheKey removes any closed or expired conns from the list
// of entries for the key, deleting the key from the entries map if
// necessary.
func (c *Cache) filterCacheKey(key interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, ent := range c.entries[key] {
		if c.opts.stale(ent.val) {
			c.filterEntryLocked(ent)
		}
	}
}

//
// internal accessors to get entries we care about
//

// oldestEntryLocked returns the oldest Put entry from the Cache or nil
// if one does not exist.
func (c *Cache) oldestEntryLocked() *entry {
	if len(c.order) == 0 {
		return nil
	}
	return c.order[0]
}

//
// exported api
//

// Close removes every value from the cache and closes it if necessary.
func (c *Cache) Close() (err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, entries := range c.entries {
		for _, ent := range entries {
			err = errs.Combine(err, c.closeEntry(ent))
		}
	}

	c.entries = make(map[interface{}][]*entry)
	c.order = nil
	c.closed = true

	return err
}

// Take acquires a value from the cache if one exists. It returns
// nil if one does not.
func (c *Cache) Take(key interface{}) interface{} {
	c.mu.Lock()
	defer c.mu.Unlock()

	entries := c.entries[key]
	for i := len(entries) - 1; i >= 0; i-- {
		ent := entries[i]

		// if the entry is not unblocked, then skip considering it.
		if !c.opts.unblocked(ent.val) {
			continue
		}
		c.filterEntryLocked(ent)

		// if we can't stop the timer or the conn is closed, try again.
		if ent.exp != nil && !ent.exp.Stop() {
			continue
		} else if c.opts.stale(ent.val) {
			// we choose to ignore this error because it is not actionable.
			_ = c.opts.close(ent.val)
			continue
		}

		return ent.val
	}

	return nil
}

// Put places the connection in to the cache with the provided key. It
// returns any errors closing any connections necessary to do the job.
func (c *Cache) Put(key, val interface{}) {
	if c.opts.Capacity < 0 || c.opts.KeyCapacity < 0 || c.opts.stale(val) {
		_ = c.opts.close(val)
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		_ = c.opts.close(val)
		return
	}

	// ensure we have enough capacity in the key
	for {
		entries := c.entries[key]
		if c.opts.KeyCapacity == 0 || len(entries) < c.opts.KeyCapacity {
			break
		}

		ent := entries[0]
		_ = c.closeEntry(ent)
		c.filterEntryLocked(ent)
	}

	// ensure we have enough overall capacity
	for {
		if c.opts.Capacity == 0 || len(c.order) < c.opts.Capacity {
			break
		}

		ent := c.oldestEntryLocked()
		_ = c.closeEntry(ent)
		c.filterEntryLocked(ent)
	}

	// create and push the new entry into the map and order list
	ent := &entry{key: key, val: val}
	c.entries[key] = append(c.entries[key], ent)
	c.order = append(c.order, ent)

	// we set expiration last so that the connection is already in
	// the data structure so we don't have to worry about filterCacheKey
	// even though it's protected by the mutex. defensive.
	if c.opts.Expiration > 0 {
		ent.exp = time.AfterFunc(c.opts.Expiration, func() {
			_ = c.opts.close(val)
			c.filterCacheKey(key)
		})
	}
}
