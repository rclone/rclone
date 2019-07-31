// Package cache implements a simple cache where the entries are
// expired after a given time (5 minutes of disuse by default).
package cache

import (
	"sync"
	"time"
)

// Cache holds values indexed by string, but expired after a given (5
// minutes by default).
type Cache struct {
	mu             sync.Mutex
	cache          map[string]*cacheEntry
	expireRunning  bool
	expireDuration time.Duration // expire the cache entry when it is older than this
	expireInterval time.Duration // interval to run the cache expire
}

// New creates a new cache with the default expire duration and interval
func New() *Cache {
	return &Cache{
		cache:          map[string]*cacheEntry{},
		expireRunning:  false,
		expireDuration: 300 * time.Second,
		expireInterval: 60 * time.Second,
	}
}

// cacheEntry is stored in the cache
type cacheEntry struct {
	value    interface{} // cached item
	err      error       // creation error
	key      string      // key
	lastUsed time.Time   // time used for expiry
}

// CreateFunc is called to create new values.  If the create function
// returns an error it will be cached if ok is true, otherwise the
// error will just be returned, allowing negative caching if required.
type CreateFunc func(key string) (value interface{}, ok bool, error error)

// used marks an entry as accessed now and kicks the expire timer off
// should be called with the lock held
func (c *Cache) used(entry *cacheEntry) {
	entry.lastUsed = time.Now()
	if !c.expireRunning {
		time.AfterFunc(c.expireInterval, c.cacheExpire)
		c.expireRunning = true
	}
}

// Get gets a value named key either from the cache or creates it
// afresh with the create function.
func (c *Cache) Get(key string, create CreateFunc) (value interface{}, err error) {
	c.mu.Lock()
	entry, ok := c.cache[key]
	if !ok {
		c.mu.Unlock() // Unlock in case Get is called recursively
		value, ok, err = create(key)
		if err != nil && !ok {
			return value, err
		}
		entry = &cacheEntry{
			value: value,
			key:   key,
			err:   err,
		}
		c.mu.Lock()
		c.cache[key] = entry
	}
	defer c.mu.Unlock()
	c.used(entry)
	return entry.value, entry.err
}

// Put puts an value named key into the cache
func (c *Cache) Put(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry := &cacheEntry{
		value: value,
		key:   key,
	}
	c.used(entry)
	c.cache[key] = entry
}

// GetMaybe returns the key and true if found, nil and false if not
func (c *Cache) GetMaybe(key string) (value interface{}, found bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, found := c.cache[key]
	if !found {
		return nil, found
	}
	c.used(entry)
	return entry.value, found
}

// cacheExpire expires any entries that haven't been used recently
func (c *Cache) cacheExpire() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	for key, entry := range c.cache {
		if now.Sub(entry.lastUsed) > c.expireDuration {
			delete(c.cache, key)
		}
	}
	if len(c.cache) != 0 {
		time.AfterFunc(c.expireInterval, c.cacheExpire)
		c.expireRunning = true
	} else {
		c.expireRunning = false
	}
}

// Clear removes everything from the cahce
func (c *Cache) Clear() {
	c.mu.Lock()
	for k := range c.cache {
		delete(c.cache, k)
	}
	c.mu.Unlock()
}

// Entries returns the number of entries in the cache
func (c *Cache) Entries() int {
	c.mu.Lock()
	entries := len(c.cache)
	c.mu.Unlock()
	return entries
}
