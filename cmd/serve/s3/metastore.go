package s3

import (
	"strings"
	"sync"
)

// metadataStore is the interface for storing S3 object metadata.
// Keys are "bucket/objectName" paths matching the existing sync.Map usage.
type metadataStore interface {
	Load(fp string) (map[string]string, bool)
	Store(fp string, meta map[string]string)
	Delete(fp string)
	DeleteAll(bucket string)
	Close() error
}

// memoryMetaStore stores metadata in memory using sync.Map.
type memoryMetaStore struct {
	m sync.Map
}

func newMemoryMetaStore() *memoryMetaStore {
	return &memoryMetaStore{}
}

func (s *memoryMetaStore) Load(fp string) (map[string]string, bool) {
	val, ok := s.m.Load(fp)
	if !ok {
		return nil, false
	}
	meta, ok := val.(map[string]string)
	return meta, ok
}

func (s *memoryMetaStore) Store(fp string, meta map[string]string) {
	s.m.Store(fp, meta)
}

func (s *memoryMetaStore) Delete(fp string) {
	s.m.Delete(fp)
}

func (s *memoryMetaStore) DeleteAll(bucket string) {
	prefix := bucket + "/"
	s.m.Range(func(k, v any) bool {
		if ks, ok := k.(string); ok && strings.HasPrefix(ks, prefix) {
			s.m.Delete(k)
		}
		return true
	})
}

func (s *memoryMetaStore) Close() error {
	return nil
}
