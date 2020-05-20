// Package bucket is contains utilities for managing bucket based backends
package bucket

import (
	"errors"
	"strings"
	"sync"
)

var (
	// ErrAlreadyDeleted is returned when an already deleted
	// bucket is passed to Remove
	ErrAlreadyDeleted = errors.New("bucket already deleted")
)

// Split takes an absolute path which includes the bucket and
// splits it into a bucket and a path in that bucket
// bucketPath
func Split(absPath string) (bucket, bucketPath string) {
	// No bucket
	if absPath == "" {
		return "", ""
	}
	slash := strings.IndexRune(absPath, '/')
	// Bucket but no path
	if slash < 0 {
		return absPath, ""
	}
	return absPath[:slash], absPath[slash+1:]
}

// Cache stores whether buckets are available and their IDs
type Cache struct {
	mu       sync.Mutex      // mutex to protect created and deleted
	status   map[string]bool // true if we have created the container, false if deleted
	createMu sync.Mutex      // mutex to protect against simultaneous Remove
	removeMu sync.Mutex      // mutex to protect against simultaneous Create
}

// NewCache creates an empty Cache
func NewCache() *Cache {
	return &Cache{
		status: make(map[string]bool, 1),
	}
}

// MarkOK marks the bucket as being present
func (c *Cache) MarkOK(bucket string) {
	if bucket != "" {
		c.mu.Lock()
		c.status[bucket] = true
		c.mu.Unlock()
	}
}

// MarkDeleted marks the bucket as being deleted
func (c *Cache) MarkDeleted(bucket string) {
	if bucket != "" {
		c.mu.Lock()
		c.status[bucket] = false
		c.mu.Unlock()
	}
}

type (
	// ExistsFn should be passed to Create to see if a bucket
	// exists or not
	ExistsFn func() (found bool, err error)

	// CreateFn should be passed to Create to make a bucket
	CreateFn func() error
)

// Create the bucket with create() if it doesn't exist
//
// If exists is set then if the bucket has been deleted it will call
// exists() to see if it still exists.
//
// If f returns an error we assume the bucket was not created
func (c *Cache) Create(bucket string, create CreateFn, exists ExistsFn) (err error) {
	// if we are at the root, then it is OK
	if bucket == "" {
		return nil
	}

	c.createMu.Lock()
	defer c.createMu.Unlock()
	c.mu.Lock()
	defer c.mu.Unlock()

	// if have exists function and bucket has been deleted, check
	// it still exists
	if created, ok := c.status[bucket]; ok && !created && exists != nil {
		found, err := exists()
		if err == nil {
			c.status[bucket] = found
		}
		if err != nil || found {
			return err
		}
	}

	// If bucket already exists then it is OK
	if created, ok := c.status[bucket]; ok && created {
		return nil
	}

	// Create the bucket
	c.mu.Unlock()
	err = create()
	c.mu.Lock()
	if err != nil {
		return err
	}

	// Mark OK if successful
	c.status[bucket] = true
	return nil
}

// Remove the bucket with f if it exists
//
// If f returns an error we assume the bucket was not removed
//
// If the bucket has already been deleted it returns ErrAlreadyDeleted
func (c *Cache) Remove(bucket string, f func() error) error {
	// if we are at the root, then it is OK
	if bucket == "" {
		return nil
	}

	c.removeMu.Lock()
	defer c.removeMu.Unlock()
	c.mu.Lock()
	defer c.mu.Unlock()

	// If bucket already deleted then it is OK
	if created, ok := c.status[bucket]; ok && !created {
		return ErrAlreadyDeleted
	}

	// Remove the bucket
	c.mu.Unlock()
	err := f()
	c.mu.Lock()
	if err != nil {
		return err
	}

	// Mark removed if successful
	c.status[bucket] = false
	return err
}

// IsDeleted returns true if the bucket has definitely been deleted by
// us, false otherwise.
func (c *Cache) IsDeleted(bucket string) bool {
	c.mu.Lock()
	created, ok := c.status[bucket]
	c.mu.Unlock()
	// if status unknown then return false
	if !ok {
		return false
	}
	return !created
}
