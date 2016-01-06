package fs

import (
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
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
	return HashStreamTypes(r, SupportedHashes)
}

// HashStreamTypes will calculate hashes of the requested hash types.
func HashStreamTypes(r io.Reader, set HashSet) (map[HashType]string, error) {
	hashers, err := hashFromTypes(set)
	if err != nil {
		return nil, err
	}

	_, err = io.Copy(hashToMultiWriter(hashers), r)
	if err != nil {
		return nil, err
	}
	var ret = make(map[HashType]string)
	for k, v := range hashers {
		ret[k] = hex.EncodeToString(v.Sum(nil))
	}
	return ret, nil
}

// hashFromTypes will return hashers for all the requested types.
// The types must be a subset of SupportedHashes,
// and this function must support all types.
func hashFromTypes(set HashSet) (map[HashType]hash.Hash, error) {
	if !set.SubsetOf(SupportedHashes) {
		return nil, fmt.Errorf("Requested set %08x contains unknown hash types", int(set))
	}
	var hashers = make(map[HashType]hash.Hash)
	types := set.Array()
	for _, t := range types {
		switch t {
		case HashMD5:
			hashers[t] = md5.New()
		case HashSHA1:
			hashers[t] = sha1.New()
		default:
			panic("internal error: Unsupported hash type")
		}
	}
	return hashers, nil
}

// hashToMultiWriter will return a set of hashers into a
// single multiwriter, where one write will update all
// the hashers.
func hashToMultiWriter(h map[HashType]hash.Hash) io.Writer {
	// Convert to to slice
	var w = make([]io.Writer, 0, len(h))
	for _, v := range h {
		w = append(w, v)
	}
	return io.MultiWriter(w...)
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
	h, err := NewMultiHasherTypes(SupportedHashes)
	if err != nil {
		panic("internal error: could not create multihasher")
	}
	return h
}

// NewMultiHasherTypes will return a hash writer that will write
// the requested hash types.
func NewMultiHasherTypes(set HashSet) (*MultiHasher, error) {
	hashers, err := hashFromTypes(set)
	if err != nil {
		return nil, err
	}
	m := MultiHasher{h: hashers, Writer: hashToMultiWriter(hashers)}
	return &m, nil
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

// SubsetOf will return true if all types of h
// is present in the set c
func (h HashSet) SubsetOf(c HashSet) bool {
	return int(h)|int(c) == int(c)
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
	if int(h) == 0 {
		return 0
	}
	// credit: https://code.google.com/u/arnehormann/
	x := uint64(h)
	x -= (x >> 1) & 0x5555555555555555
	x = (x>>2)&0x3333333333333333 + x&0x3333333333333333
	x += x >> 4
	x &= 0x0f0f0f0f0f0f0f0f
	x *= 0x0101010101010101
	return int(x >> 56)
}
