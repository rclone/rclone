//go:build !plan9 && !js
// +build !plan9,!js

package cache

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	cache "github.com/patrickmn/go-cache"
	"github.com/rclone/rclone/fs"
)

// Memory is a wrapper of transient storage for a go-cache store
type Memory struct {
	db *cache.Cache
}

// NewMemory builds this cache storage
// defaultExpiration will set the expiry time of chunks in this storage
func NewMemory(defaultExpiration time.Duration) *Memory {
	mem := &Memory{}
	err := mem.Connect(defaultExpiration)
	if err != nil {
		fs.Errorf("cache", "can't open ram connection: %v", err)
	}

	return mem
}

// Connect will create a connection for the storage
func (m *Memory) Connect(defaultExpiration time.Duration) error {
	m.db = cache.New(defaultExpiration, -1)
	return nil
}

// HasChunk confirms the existence of a single chunk of an object
func (m *Memory) HasChunk(cachedObject *Object, offset int64) bool {
	key := cachedObject.abs() + "-" + strconv.FormatInt(offset, 10)
	_, found := m.db.Get(key)
	return found
}

// GetChunk will retrieve a single chunk which belongs to a cached object or an error if it doesn't find it
func (m *Memory) GetChunk(cachedObject *Object, offset int64) ([]byte, error) {
	key := cachedObject.abs() + "-" + strconv.FormatInt(offset, 10)
	var data []byte

	if x, found := m.db.Get(key); found {
		data = x.([]byte)
		return data, nil
	}

	return nil, fmt.Errorf("couldn't get cached object data at offset %v", offset)
}

// AddChunk adds a new chunk of a cached object
func (m *Memory) AddChunk(fp string, data []byte, offset int64) error {
	return m.AddChunkAhead(fp, data, offset, time.Second)
}

// AddChunkAhead adds a new chunk of a cached object
func (m *Memory) AddChunkAhead(fp string, data []byte, offset int64, t time.Duration) error {
	key := fp + "-" + strconv.FormatInt(offset, 10)
	m.db.Set(key, data, cache.DefaultExpiration)

	return nil
}

// CleanChunksByAge will cleanup on a cron basis
func (m *Memory) CleanChunksByAge(chunkAge time.Duration) {
	m.db.DeleteExpired()
}

// CleanChunksByNeed will cleanup chunks after the FS passes a specific chunk
func (m *Memory) CleanChunksByNeed(offset int64) {
	for key := range m.db.Items() {
		sepIdx := strings.LastIndex(key, "-")
		keyOffset, err := strconv.ParseInt(key[sepIdx+1:], 10, 64)
		if err != nil {
			fs.Errorf("cache", "couldn't parse offset entry %v", key)
			continue
		}

		if keyOffset < offset {
			m.db.Delete(key)
		}
	}
}

// CleanChunksBySize will cleanup chunks after the total size passes a certain point
func (m *Memory) CleanChunksBySize(maxSize int64) {
	// NOOP
}
