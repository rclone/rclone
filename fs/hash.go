package fs

import (
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"hash"
	"io"
)

// HashType indicates a standard hashing algorithm
type HashType int

const (
	// HashMD5 indicates MD5 support
	HashMD5 HashType = 1 << iota

	// HashSHA1 indicates SHA-1 support
	HashSHA1
)

// SupportedHashes returns a set of all the supported hashes by
// HashStream and MultiHasher.
var SupportedHashes = NewHashSet(HashMD5, HashSHA1)

// The HashedFs interface indicates that the filesystem
// supports one or more file hashes.
type HashedFs interface {
	Hashes() HashSet
}

// HashStream will calculate hashes of all supported hash types.
func HashStream(r io.Reader) (map[HashType]string, error) {
	md5w := md5.New()
	sha1w := sha1.New()
	w := io.MultiWriter(md5w, sha1w)
	_, err := io.Copy(w, r)
	if err != nil {
		return nil, err
	}
	return map[HashType]string{
		HashMD5:  hex.EncodeToString(md5w.Sum(nil)),
		HashSHA1: hex.EncodeToString(sha1w.Sum(nil)),
	}, nil
}

// A MultiHasher will construct various hashes on
// all incoming writes.
type MultiHasher struct {
	io.Writer
	h map[HashType]hash.Hash // Hashes
}

// NewMultiHasher will return a hash writer that will write all
// supported hash types.
func NewMultiHasher() *MultiHasher {
	m := &MultiHasher{h: make(map[HashType]hash.Hash)}
	m.h[HashMD5] = md5.New()
	m.h[HashSHA1] = sha1.New()
	m.Writer = io.MultiWriter(m.h[HashMD5], m.h[HashSHA1])
	return m
}

// Sums returns the sums of all accumulated hashes as hex encoded
// strings.
func (m *MultiHasher) Sums() map[HashType]string {
	dst := make(map[HashType]string)
	for k, v := range m.h {
		dst[k] = hex.EncodeToString(v.Sum(nil))
	}
	return dst
}

// A HashSet Indicates one or more hash types.
type HashSet int

// NewHashSet will create a new hash set with the hash types supplied
func NewHashSet(t ...HashType) HashSet {
	h := HashSet(0)
	return h.Add(t...)
}

// Add one or more hash types to the set.
// Returns the modified hash set.
func (h *HashSet) Add(t ...HashType) HashSet {
	for _, v := range t {
		*h |= HashSet(v)
	}
	return *h
}

// Contains returns true if the
func (h HashSet) Contains(t HashType) bool {
	return int(h)&int(t) != 0
}

// Overlap returns the overlapping hash types
func (h HashSet) Overlap(t HashSet) HashSet {
	return HashSet(int(h) & int(t))
}

// Array returns an array of all hash types in the set
func (h HashSet) Array() (ht []HashType) {
	v := int(h)
	i := uint(0)
	for v != 0 {
		if v&1 != 0 {
			ht = append(ht, HashType(1<<i))
		}
		i++
		v >>= 1
	}
	return ht
}

// Count returns the number of hash types in the set
func (h HashSet) Count() int {
	// credit: https://code.google.com/u/arnehormann/
	x := uint64(h)
	x -= (x >> 1) & 0x5555555555555555
	x = (x>>2)&0x3333333333333333 + x&0x3333333333333333
	x += x >> 4
	x &= 0x0f0f0f0f0f0f0f0f
	x *= 0x0101010101010101
	return int(x >> 56)
}
