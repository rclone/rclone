package hash

import (
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"strings"

	"github.com/ncw/rclone/backend/dropbox/dbhash"
	"github.com/ncw/rclone/backend/onedrive/quickxorhash"
	"github.com/pkg/errors"
)

// Type indicates a standard hashing algorithm
type Type int

// ErrUnsupported should be returned by filesystem,
// if it is requested to deliver an unsupported hash type.
var ErrUnsupported = errors.New("hash type not supported")

const (
	// MD5 indicates MD5 support
	MD5 Type = 1 << iota

	// SHA1 indicates SHA-1 support
	SHA1

	// Dropbox indicates Dropbox special hash
	// https://www.dropbox.com/developers/reference/content-hash
	Dropbox

	// QuickXorHash indicates Microsoft onedrive hash
	// https://docs.microsoft.com/en-us/onedrive/developer/code-snippets/quickxorhash
	QuickXorHash

	// None indicates no hashes are supported
	None Type = 0
)

// Supported returns a set of all the supported hashes by
// HashStream and MultiHasher.
var Supported = NewHashSet(MD5, SHA1, Dropbox, QuickXorHash)

// Width returns the width in characters for any HashType
var Width = map[Type]int{
	MD5:          32,
	SHA1:         40,
	Dropbox:      64,
	QuickXorHash: 40,
}

// Stream will calculate hashes of all supported hash types.
func Stream(r io.Reader) (map[Type]string, error) {
	return StreamTypes(r, Supported)
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
	switch h {
	case None:
		return "None"
	case MD5:
		return "MD5"
	case SHA1:
		return "SHA-1"
	case Dropbox:
		return "DropboxHash"
	case QuickXorHash:
		return "QuickXorHash"
	default:
		err := fmt.Sprintf("internal error: unknown hash type: 0x%x", int(h))
		panic(err)
	}
}

// Set a Type from a flag
func (h *Type) Set(s string) error {
	switch s {
	case "None":
		*h = None
	case "MD5":
		*h = MD5
	case "SHA-1":
		*h = SHA1
	case "DropboxHash":
		*h = Dropbox
	case "QuickXorHash":
		*h = QuickXorHash
	default:
		return errors.Errorf("Unknown hash type %q", s)
	}
	return nil
}

// Type of the value
func (h Type) Type() string {
	return "string"
}

// fromTypes will return hashers for all the requested types.
// The types must be a subset of SupportedHashes,
// and this function must support all types.
func fromTypes(set Set) (map[Type]hash.Hash, error) {
	if !set.SubsetOf(Supported) {
		return nil, errors.Errorf("requested set %08x contains unknown hash types", int(set))
	}
	var hashers = make(map[Type]hash.Hash)
	types := set.Array()
	for _, t := range types {
		switch t {
		case MD5:
			hashers[t] = md5.New()
		case SHA1:
			hashers[t] = sha1.New()
		case Dropbox:
			hashers[t] = dbhash.New()
		case QuickXorHash:
			hashers[t] = quickxorhash.New()
		default:
			err := fmt.Sprintf("internal error: Unsupported hash type %v", t)
			panic(err)
		}
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
	h, err := NewMultiHasherTypes(Supported)
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
