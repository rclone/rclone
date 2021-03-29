// Package cache implements a simple cache where the entries are
// expired after a given time (5 minutes of disuse by default).
package cache

import (
	"strings"
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

// SetExpireDuration sets the interval at which things expire
//
// If it is less than or equal to 0 then things are never cached
func (c *Cache) SetExpireDuration(d time.Duration) *Cache {
	c.expireDuration = d
	return c
}

// returns true if we aren't to cache anything
func (c *Cache) noCache() bool {
	return c.expireDuration <= 0
}

// SetExpireInterval sets the interval at which the cache expiry runs
//
// Set to 0 or a -ve number to disable
func (c *Cache) SetExpireInterval(d time.Duration) *Cache {
	if d <= 0 {
		d = 100 * 365 * 24 * time.Hour
	}
	c.expireInterval = d
	return c
}

// cacheEntry is stored in the cache
type cacheEntry struct {
	value    interface{} // cached item
	err      error       // creation error
	key      string      // key
	lastUsed time.Time   // time used for expiry
	pinCount int         // non zero if the entry should not be removed
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
		if !c.noCache() {
			c.cache[key] = entry
		}
	}
	defer c.mu.Unlock()
	c.used(entry)
	return entry.value, entry.err
}

func (c *Cache) addPin(key string, count int) {
	c.mu.Lock()
	entry, ok := c.cache[key]
	if ok {
		entry.pinCount += count
		c.used(entry)
	}
	c.mu.Unlock()
}

// Pin a value in the cache if it exists
func (c *Cache) Pin(key string) {
	c.addPin(key, 1)
}

// Unpin a value in the cache if it exists
func (c *Cache) Unpin(key string) {
	c.addPin(key, -1)
}

// Put puts a value named key into the cache
func (c *Cache) Put(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.noCache() {
		return
	}
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

// Delete the entry passed in
//
// Returns true if the entry was found
func (c *Cache) Delete(key string) bool {
	c.mu.Lock()
	_, found := c.cache[key]
	delete(c.cache, key)
	c.mu.Unlock()
	return found
}

// DeletePrefix deletes all entries with the given prefix
//
// Returns number of entries deleted
func (c *Cache) DeletePrefix(prefix string) (deleted int) {
	c.mu.Lock()
	for k := range c.cache {
		if strings.HasPrefix(k, prefix) {
			delete(c.cache, k)
			deleted++
		}
	}
	c.mu.Unlock()
	return deleted
}

// Rename renames the item at oldKey to newKey.
//
// If there was an existing item at newKey then it takes precedence
// and is returned otherwise the item (if any) at oldKey is returned.
func (c *Cache) Rename(oldKey, newKey string) (value interface{}, found bool) {
	c.mu.Lock()
	if newEntry, newFound := c.cache[newKey]; newFound {
		// If new entry is found use that
		delete(c.cache, oldKey)
		value, found = newEntry.value, newFound
		c.used(newEntry)
	} else if oldEntry, oldFound := c.cache[oldKey]; oldFound {
		// If old entry is found rename it to new and use that
		c.cache[newKey] = oldEntry
		delete(c.cache, oldKey)
		c.used(oldEntry)
		value, found = oldEntry.value, oldFound
	}
	c.mu.Unlock()
	return value, found
}

// cacheExpire expires any entries that haven't been used recently
func (c *Cache) cacheExpire() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	for key, entry := range c.cache {
		if entry.pinCount <= 0 && now.Sub(entry.lastUsed) > c.expireDuration {
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

// Clear removes everything from the cache
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
