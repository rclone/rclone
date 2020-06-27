package md5simd

import (
	"crypto/md5"
	"hash"
	"sync"
)

const (
	// The blocksize of MD5 in bytes.
	BlockSize = 64

	// The size of an MD5 checksum in bytes.
	Size = 16

	// internalBlockSize is the internal block size.
	internalBlockSize = 32 << 10
)

type Server interface {
	NewHash() Hasher
	Close()
}

type Hasher interface {
	hash.Hash
	Close()
}

// md5Wrapper is a wrapper around the builtin hasher.
type md5Wrapper struct {
	hash.Hash
}

var md5Pool = sync.Pool{New: func() interface{} {
	return md5.New()
}}

// fallbackServer - Fallback when no assembly is available.
type fallbackServer struct {
}

// NewHash -- return regular Golang md5 hashing from crypto
func (s *fallbackServer) NewHash() Hasher {
	return &md5Wrapper{Hash: md5Pool.New().(hash.Hash)}
}

func (s *fallbackServer) Close() {
}

func (m *md5Wrapper) Close() {
	if m.Hash != nil {
		m.Reset()
		md5Pool.Put(m.Hash)
		m.Hash = nil
	}
}
