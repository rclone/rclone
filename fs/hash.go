package fs

import (
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"strings"

	"github.com/ncw/rclone/backend/dropbox/dbhash"
	"github.com/pkg/errors"
	"github.com/spf13/pflag"
)

// HashType indicates a standard hashing algorithm
type HashType int

// ErrHashUnsupported should be returned by filesystem,
// if it is requested to deliver an unsupported hash type.
var ErrHashUnsupported = errors.New("hash type not supported")

const (
	// HashMD5 indicates MD5 support
	HashMD5 HashType = 1 << iota

	// HashSHA1 indicates SHA-1 support
	HashSHA1

	// HashDropbox indicates Dropbox special hash
	// https://www.dropbox.com/developers/reference/content-hash
	HashDropbox

	// HashNone indicates no hashes are supported
	HashNone HashType = 0
)

// SupportedHashes returns a set of all the supported hashes by
// HashStream and MultiHasher.
var SupportedHashes = NewHashSet(HashMD5, HashSHA1, HashDropbox)

// HashWidth returns the width in characters for any HashType
var HashWidth = map[HashType]int{
	HashMD5:     32,
	HashSHA1:    40,
	HashDropbox: 64,
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

// String returns a string representation of the hash type.
// The function will panic if the hash type is unknown.
func (h HashType) String() string {
	switch h {
	case HashNone:
		return "None"
	case HashMD5:
		return "MD5"
	case HashSHA1:
		return "SHA-1"
	case HashDropbox:
		return "DropboxHash"
	default:
		err := fmt.Sprintf("internal error: unknown hash type: 0x%x", int(h))
		panic(err)
	}
}

// Set a HashType from a flag
func (h *HashType) Set(s string) error {
	switch s {
	case "None":
		*h = HashNone
	case "MD5":
		*h = HashMD5
	case "SHA-1":
		*h = HashSHA1
	case "DropboxHash":
		*h = HashDropbox
	default:
		return errors.Errorf("Unknown hash type %q", s)
	}
	return nil
}

// Type of the value
func (h HashType) Type() string {
	return "string"
}

// Check it satisfies the interface
var _ pflag.Value = (*HashType)(nil)

// hashFromTypes will return hashers for all the requested types.
// The types must be a subset of SupportedHashes,
// and this function must support all types.
func hashFromTypes(set HashSet) (map[HashType]hash.Hash, error) {
	if !set.SubsetOf(SupportedHashes) {
		return nil, errors.Errorf("requested set %08x contains unknown hash types", int(set))
	}
	var hashers = make(map[HashType]hash.Hash)
	types := set.Array()
	for _, t := range types {
		switch t {
		case HashMD5:
			hashers[t] = md5.New()
		case HashSHA1:
			hashers[t] = sha1.New()
		case HashDropbox:
			hashers[t] = dbhash.New()
		default:
			err := fmt.Sprintf("internal error: Unsupported hash type %v", t)
			panic(err)
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
	w    io.Writer
	size int64
	h    map[HashType]hash.Hash // Hashes
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
	m := MultiHasher{h: hashers, w: hashToMultiWriter(hashers)}
	return &m, nil
}

func (m *MultiHasher) Write(p []byte) (n int, err error) {
	n, err = m.w.Write(p)
	m.size += int64(n)
	return n, err
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

// Size returns the number of bytes written
func (m *MultiHasher) Size() int64 {
	return m.size
}

// A HashSet Indicates one or more hash types.
type HashSet int

// NewHashSet will create a new hash set with the hash types supplied
func NewHashSet(t ...HashType) HashSet {
	h := HashSet(HashNone)
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

// GetOne will return a hash type.
// Currently the first is returned, but it could be
// improved to return the strongest.
func (h HashSet) GetOne() HashType {
	v := int(h)
	i := uint(0)
	for v != 0 {
		if v&1 != 0 {
			return HashType(1 << i)
		}
		i++
		v >>= 1
	}
	return HashType(HashNone)
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

// String returns a string representation of the hash set.
// The function will panic if it contains an unknown type.
func (h HashSet) String() string {
	a := h.Array()
	var r []string
	for _, v := range a {
		r = append(r, v.String())
	}
	return "[" + strings.Join(r, ", ") + "]"
}
