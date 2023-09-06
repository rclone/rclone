// Package blake3 provides an SSE4.1/AVX2 accelerated BLAKE3 implementation.
package blake3

import (
	"errors"

	"github.com/zeebo/blake3/internal/consts"
	"github.com/zeebo/blake3/internal/utils"
)

// Hasher is a hash.Hash for BLAKE3.
type Hasher struct {
	size int
	h    hasher
}

// New returns a new Hasher that has a digest size of 32 bytes.
//
// If you need more or less output bytes than that, use Digest method.
func New() *Hasher {
	return &Hasher{
		size: 32,
		h: hasher{
			key: consts.IV,
		},
	}
}

// NewKeyed returns a new Hasher that uses the 32 byte input key and has
// a digest size of 32 bytes.
//
// If you need more or less output bytes than that, use the Digest method.
func NewKeyed(key []byte) (*Hasher, error) {
	if len(key) != 32 {
		return nil, errors.New("invalid key size")
	}

	h := &Hasher{
		size: 32,
		h: hasher{
			flags: consts.Flag_Keyed,
		},
	}
	utils.KeyFromBytes(key, &h.h.key)

	return h, nil
}

// DeriveKey derives a key based on reusable key material of any
// length, in the given context. The key will be stored in out, using
// all of its current length.
//
// Context strings must be hardcoded constants, and the recommended
// format is "[application] [commit timestamp] [purpose]", e.g.,
// "example.com 2019-12-25 16:18:03 session tokens v1".
func DeriveKey(context string, material []byte, out []byte) {
	h := NewDeriveKey(context)
	_, _ = h.Write(material)
	_, _ = h.Digest().Read(out)
}

// NewDeriveKey returns a Hasher that is initialized with the context
// string. See DeriveKey for details. It has a digest size of 32 bytes.
//
// If you need more or less output bytes than that, use the Digest method.
func NewDeriveKey(context string) *Hasher {
	// hash the context string and use that instead of IV
	h := &Hasher{
		size: 32,
		h: hasher{
			key:   consts.IV,
			flags: consts.Flag_DeriveKeyContext,
		},
	}

	var buf [32]byte
	_, _ = h.WriteString(context)
	_, _ = h.Digest().Read(buf[:])

	h.Reset()
	utils.KeyFromBytes(buf[:], &h.h.key)
	h.h.flags = consts.Flag_DeriveKeyMaterial

	return h
}

// Write implements part of the hash.Hash interface. It never returns an error.
func (h *Hasher) Write(p []byte) (int, error) {
	h.h.update(p)
	return len(p), nil
}

// WriteString is like Write but specialized to strings to avoid allocations.
func (h *Hasher) WriteString(p string) (int, error) {
	h.h.updateString(p)
	return len(p), nil
}

// Reset implements part of the hash.Hash interface. It causes the Hasher to
// act as if it was newly created.
func (h *Hasher) Reset() {
	h.h.reset()
}

// Clone returns a new Hasher with the same internal state.
//
// Modifying the resulting Hasher will not modify the original Hasher, and vice versa.
func (h *Hasher) Clone() *Hasher {
	return &Hasher{size: h.size, h: h.h}
}

// Size implements part of the hash.Hash interface. It returns the number of
// bytes the hash will output in Sum.
func (h *Hasher) Size() int {
	return h.size
}

// BlockSize implements part of the hash.Hash interface. It returns the most
// natural size to write to the Hasher.
func (h *Hasher) BlockSize() int {
	return 64
}

// Sum implements part of the hash.Hash interface. It appends the digest of
// the Hasher to the provided buffer and returns it.
func (h *Hasher) Sum(b []byte) []byte {
	if top := len(b) + h.size; top <= cap(b) && top >= len(b) {
		h.h.finalize(b[len(b):top])
		return b[:top]
	}

	tmp := make([]byte, h.size)
	h.h.finalize(tmp)
	return append(b, tmp...)
}

// Digest takes a snapshot of the hash state and returns an object that can
// be used to read and seek through 2^64 bytes of digest output.
func (h *Hasher) Digest() *Digest {
	var d Digest
	h.h.finalizeDigest(&d)
	return &d
}

// Sum256 returns the first 256 bits of the unkeyed digest of the data.
func Sum256(data []byte) (sum [32]byte) {
	out := Sum512(data)
	copy(sum[:], out[:32])
	return sum
}

// Sum512 returns the first 512 bits of the unkeyed digest of the data.
func Sum512(data []byte) (sum [64]byte) {
	if len(data) <= consts.ChunkLen {
		var d Digest
		compressAll(&d, data, 0, consts.IV)
		_, _ = d.Read(sum[:])
		return sum
	} else {
		h := hasher{key: consts.IV}
		h.update(data)
		h.finalize(sum[:])
		return sum
	}
}
