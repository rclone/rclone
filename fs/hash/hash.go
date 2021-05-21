package hash

import (
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"hash"
	"hash/crc32"
	"io"
	"strings"

	"github.com/jzelinskie/whirlpool"
	"github.com/pkg/errors"
)

// Type indicates a standard hashing algorithm
type Type int

type hashDefinition struct {
	width    int
	name     string
	alias    string
	newFunc  func() hash.Hash
	hashType Type
}

var (
	type2hash  = map[Type]*hashDefinition{}
	name2hash  = map[string]*hashDefinition{}
	alias2hash = map[string]*hashDefinition{}
	supported  = []Type{}
)

// RegisterHash adds a new Hash to the list and returns it Type
func RegisterHash(name, alias string, width int, newFunc func() hash.Hash) Type {
	hashType := Type(1 << len(supported))
	supported = append(supported, hashType)

	definition := &hashDefinition{
		name:     name,
		alias:    alias,
		width:    width,
		newFunc:  newFunc,
		hashType: hashType,
	}

	type2hash[hashType] = definition
	name2hash[name] = definition
	alias2hash[alias] = definition

	return hashType
}

// ErrUnsupported should be returned by filesystem,
// if it is requested to deliver an unsupported hash type.
var ErrUnsupported = errors.New("hash type not supported")

var (
	// None indicates no hashes are supported
	None Type

	// MD5 indicates MD5 support
	MD5 Type

	// SHA1 indicates SHA-1 support
	SHA1 Type

	// Whirlpool indicates Whirlpool support
	Whirlpool Type

	// CRC32 indicates CRC-32 support
	CRC32 Type
)

func init() {
	MD5 = RegisterHash("md5", "MD5", 32, md5.New)
	SHA1 = RegisterHash("sha1", "SHA-1", 40, sha1.New)
	Whirlpool = RegisterHash("whirlpool", "Whirlpool", 128, whirlpool.New)
	CRC32 = RegisterHash("crc32", "CRC-32", 8, func() hash.Hash { return crc32.NewIEEE() })
}

// Supported returns a set of all the supported hashes by
// HashStream and MultiHasher.
func Supported() Set {
	return NewHashSet(supported...)
}

// Width returns the width in characters for any HashType
func Width(hashType Type) int {
	if hash := type2hash[hashType]; hash != nil {
		return hash.width
	}
	return 0
}

// Stream will calculate hashes of all supported hash types.
func Stream(r io.Reader) (map[Type]string, error) {
	return StreamTypes(r, Supported())
}

// StreamTypes will calculate hashes of the requested hash types.
func StreamTypes(r io.Reader, set Set) (map[Type]string, error) {
	hashers, err := fromTypes(set)
	if err != nil {
		return nil, err
	}

	_, err = io.Copy(toMultiWriter(hashers), r)
	if err != nil {
		return nil, err
	}
	var ret = make(map[Type]string)
	for k, v := range hashers {
		ret[k] = hex.EncodeToString(v.Sum(nil))
	}
	return ret, nil
}

// String returns a string representation of the hash type.
// The function will panic if the hash type is unknown.
func (h Type) String() string {
	if h == None {
		return "none"
	}
	if hash := type2hash[h]; hash != nil {
		return hash.name
	}
	panic(fmt.Sprintf("internal error: unknown hash type: 0x%x", int(h)))
}

// Set a Type from a flag.
// Both name and alias are accepted.
func (h *Type) Set(s string) error {
	if s == "none" || s == "None" {
		*h = None
		return nil
	}
	if hash := name2hash[strings.ToLower(s)]; hash != nil {
		*h = hash.hashType
		return nil
	}
	if hash := alias2hash[s]; hash != nil {
		*h = hash.hashType
		return nil
	}
	return errors.Errorf("Unknown hash type %q", s)
}

// Type of the value
func (h Type) Type() string {
	return "string"
}

// fromTypes will return hashers for all the requested types.
// The types must be a subset of SupportedHashes,
// and this function must support all types.
func fromTypes(set Set) (map[Type]hash.Hash, error) {
	if !set.SubsetOf(Supported()) {
		return nil, errors.Errorf("requested set %08x contains unknown hash types", int(set))
	}
	hashers := map[Type]hash.Hash{}

	for _, t := range set.Array() {
		hash := type2hash[t]
		if hash == nil {
			panic(fmt.Sprintf("internal error: Unsupported hash type %v", t))
		}
		hashers[t] = hash.newFunc()
	}

	return hashers, nil
}

// toMultiWriter will return a set of hashers into a
// single multiwriter, where one write will update all
// the hashers.
func toMultiWriter(h map[Type]hash.Hash) io.Writer {
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
	h    map[Type]hash.Hash // Hashes
}

// NewMultiHasher will return a hash writer that will write all
// supported hash types.
func NewMultiHasher() *MultiHasher {
	h, err := NewMultiHasherTypes(Supported())
	if err != nil {
		panic("internal error: could not create multihasher")
	}
	return h
}

// NewMultiHasherTypes will return a hash writer that will write
// the requested hash types.
func NewMultiHasherTypes(set Set) (*MultiHasher, error) {
	hashers, err := fromTypes(set)
	if err != nil {
		return nil, err
	}
	m := MultiHasher{h: hashers, w: toMultiWriter(hashers)}
	return &m, nil
}

func (m *MultiHasher) Write(p []byte) (n int, err error) {
	n, err = m.w.Write(p)
	m.size += int64(n)
	return n, err
}

// Sums returns the sums of all accumulated hashes as hex encoded
// strings.
func (m *MultiHasher) Sums() map[Type]string {
	dst := make(map[Type]string)
	for k, v := range m.h {
		dst[k] = hex.EncodeToString(v.Sum(nil))
	}
	return dst
}

// Sum returns the specified hash from the multihasher
func (m *MultiHasher) Sum(hashType Type) ([]byte, error) {
	h, ok := m.h[hashType]
	if !ok {
		return nil, ErrUnsupported
	}
	return h.Sum(nil), nil
}

// Size returns the number of bytes written
func (m *MultiHasher) Size() int64 {
	return m.size
}

// A Set Indicates one or more hash types.
type Set int

// NewHashSet will create a new hash set with the hash types supplied
func NewHashSet(t ...Type) Set {
	h := Set(None)
	return h.Add(t...)
}

// Add one or more hash types to the set.
// Returns the modified hash set.
func (h *Set) Add(t ...Type) Set {
	for _, v := range t {
		*h |= Set(v)
	}
	return *h
}

// Contains returns true if the
func (h Set) Contains(t Type) bool {
	return int(h)&int(t) != 0
}

// Overlap returns the overlapping hash types
func (h Set) Overlap(t Set) Set {
	return Set(int(h) & int(t))
}

// SubsetOf will return true if all types of h
// is present in the set c
func (h Set) SubsetOf(c Set) bool {
	return int(h)|int(c) == int(c)
}

// GetOne will return a hash type.
// Currently the first is returned, but it could be
// improved to return the strongest.
func (h Set) GetOne() Type {
	v := int(h)
	i := uint(0)
	for v != 0 {
		if v&1 != 0 {
			return Type(1 << i)
		}
		i++
		v >>= 1
	}
	return None
}

// Array returns an array of all hash types in the set
func (h Set) Array() (ht []Type) {
	v := int(h)
	i := uint(0)
	for v != 0 {
		if v&1 != 0 {
			ht = append(ht, Type(1<<i))
		}
		i++
		v >>= 1
	}
	return ht
}

// Count returns the number of hash types in the set
func (h Set) Count() int {
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
func (h Set) String() string {
	a := h.Array()
	var r []string
	for _, v := range a {
		r = append(r, v.String())
	}
	return "[" + strings.Join(r, ", ") + "]"
}

// Equals checks to see if src == dst, but ignores empty strings
// and returns true if either is empty.
func Equals(src, dst string) bool {
	if src == "" || dst == "" {
		return true
	}
	return src == dst
}
