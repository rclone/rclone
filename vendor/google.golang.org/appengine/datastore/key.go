// Copyright 2011 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package datastore

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"

	"google.golang.org/appengine/internal"
	pb "google.golang.org/appengine/internal/datastore"
)

type KeyRangeCollisionError struct {
	start int64
	end   int64
}

func (e *KeyRangeCollisionError) Error() string {
	return fmt.Sprintf("datastore: Collision when attempting to allocate range [%d, %d]",
		e.start, e.end)
}

type KeyRangeContentionError struct {
	start int64
	end   int64
}

func (e *KeyRangeContentionError) Error() string {
	return fmt.Sprintf("datastore: Contention when attempting to allocate range [%d, %d]",
		e.start, e.end)
}

// Key represents the datastore key for a stored entity, and is immutable.
type Key struct {
	kind      string
	stringID  string
	intID     int64
	parent    *Key
	appID     string
	namespace string
}

// Kind returns the key's kind (also known as entity type).
func (k *Key) Kind() string {
	return k.kind
}

// StringID returns the key's string ID (also known as an entity name or key
// name), which may be "".
func (k *Key) StringID() string {
	return k.stringID
}

// IntID returns the key's integer ID, which may be 0.
func (k *Key) IntID() int64 {
	return k.intID
}

// Parent returns the key's parent key, which may be nil.
func (k *Key) Parent() *Key {
	return k.parent
}

// AppID returns the key's application ID.
func (k *Key) AppID() string {
	return k.appID
}

// Namespace returns the key's namespace.
func (k *Key) Namespace() string {
	return k.namespace
}

// Incomplete returns whether the key does not refer to a stored entity.
// In particular, whether the key has a zero StringID and a zero IntID.
func (k *Key) Incomplete() bool {
	return k.stringID == "" && k.intID == 0
}

// valid returns whether the key is valid.
func (k *Key) valid() bool {
	if k == nil {
		return false
	}
	for ; k != nil; k = k.parent {
		if k.kind == "" || k.appID == "" {
			return false
		}
		if k.stringID != "" && k.intID != 0 {
			return false
		}
		if k.parent != nil {
			if k.parent.Incomplete() {
				return false
			}
			if k.parent.appID != k.appID || k.parent.namespace != k.namespace {
				return false
			}
		}
	}
	return true
}

// Equal returns whether two keys are equal.
func (k *Key) Equal(o *Key) bool {
	for k != nil && o != nil {
		if k.kind != o.kind || k.stringID != o.stringID || k.intID != o.intID || k.appID != o.appID || k.namespace != o.namespace {
			return false
		}
		k, o = k.parent, o.parent
	}
	return k == o
}

// root returns the furthest ancestor of a key, which may be itself.
func (k *Key) root() *Key {
	for k.parent != nil {
		k = k.parent
	}
	return k
}

// marshal marshals the key's string representation to the buffer.
func (k *Key) marshal(b *bytes.Buffer) {
	if k.parent != nil {
		k.parent.marshal(b)
	}
	b.WriteByte('/')
	b.WriteString(k.kind)
	b.WriteByte(',')
	if k.stringID != "" {
		b.WriteString(k.stringID)
	} else {
		b.WriteString(strconv.FormatInt(k.intID, 10))
	}
}

// String returns a string representation of the key.
func (k *Key) String() string {
	if k == nil {
		return ""
	}
	b := bytes.NewBuffer(make([]byte, 0, 512))
	k.marshal(b)
	return b.String()
}

type gobKey struct {
	Kind      string
	StringID  string
	IntID     int64
	Parent    *gobKey
	AppID     string
	Namespace string
}

func keyToGobKey(k *Key) *gobKey {
	if k == nil {
		return nil
	}
	return &gobKey{
		Kind:      k.kind,
		StringID:  k.stringID,
		IntID:     k.intID,
		Parent:    keyToGobKey(k.parent),
		AppID:     k.appID,
		Namespace: k.namespace,
	}
}

func gobKeyToKey(gk *gobKey) *Key {
	if gk == nil {
		return nil
	}
	return &Key{
		kind:      gk.Kind,
		stringID:  gk.StringID,
		intID:     gk.IntID,
		parent:    gobKeyToKey(gk.Parent),
		appID:     gk.AppID,
		namespace: gk.Namespace,
	}
}

func (k *Key) GobEncode() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := gob.NewEncoder(buf).Encode(keyToGobKey(k)); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (k *Key) GobDecode(buf []byte) error {
	gk := new(gobKey)
	if err := gob.NewDecoder(bytes.NewBuffer(buf)).Decode(gk); err != nil {
		return err
	}
	*k = *gobKeyToKey(gk)
	return nil
}

func (k *Key) MarshalJSON() ([]byte, error) {
	return []byte(`"` + k.Encode() + `"`), nil
}

func (k *Key) UnmarshalJSON(buf []byte) error {
	if len(buf) < 2 || buf[0] != '"' || buf[len(buf)-1] != '"' {
		return errors.New("datastore: bad JSON key")
	}
	k2, err := DecodeKey(string(buf[1 : len(buf)-1]))
	if err != nil {
		return err
	}
	*k = *k2
	return nil
}

// Encode returns an opaque representation of the key
// suitable for use in HTML and URLs.
// This is compatible with the Python and Java runtimes.
func (k *Key) Encode() string {
	ref := keyToProto("", k)

	b, err := proto.Marshal(ref)
	if err != nil {
		panic(err)
	}

	// Trailing padding is stripped.
	return strings.TrimRight(base64.URLEncoding.EncodeToString(b), "=")
}

// DecodeKey decodes a key from the opaque representation returned by Encode.
func DecodeKey(encoded string) (*Key, error) {
	// Re-add padding.
	if m := len(encoded) % 4; m != 0 {
		encoded += strings.Repeat("=", 4-m)
	}

	b, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}

	ref := new(pb.Reference)
	if err := proto.Unmarshal(b, ref); err != nil {
		return nil, err
	}

	return protoToKey(ref)
}

// NewIncompleteKey creates a new incomplete key.
// kind cannot be empty.
func NewIncompleteKey(c context.Context, kind string, parent *Key) *Key {
	return NewKey(c, kind, "", 0, parent)
}

// NewKey creates a new key.
// kind cannot be empty.
// Either one or both of stringID and intID must be zero. If both are zero,
// the key returned is incomplete.
// parent must either be a complete key or nil.
func NewKey(c context.Context, kind, stringID string, intID int64, parent *Key) *Key {
	// If there's a parent key, use its namespace.
	// Otherwise, use any namespace attached to the context.
	var namespace string
	if parent != nil {
		namespace = parent.namespace
	} else {
		namespace = internal.NamespaceFromContext(c)
	}

	return &Key{
		kind:      kind,
		stringID:  stringID,
		intID:     intID,
		parent:    parent,
		appID:     internal.FullyQualifiedAppID(c),
		namespace: namespace,
	}
}

// AllocateIDs returns a range of n integer IDs with the given kind and parent
// combination. kind cannot be empty; parent may be nil. The IDs in the range
// returned will not be used by the datastore's automatic ID sequence generator
// and may be used with NewKey without conflict.
//
// The range is inclusive at the low end and exclusive at the high end. In
// other words, valid intIDs x satisfy low <= x && x < high.
//
// If no error is returned, low + n == high.
func AllocateIDs(c context.Context, kind string, parent *Key, n int) (low, high int64, err error) {
	if kind == "" {
		return 0, 0, errors.New("datastore: AllocateIDs given an empty kind")
	}
	if n < 0 {
		return 0, 0, fmt.Errorf("datastore: AllocateIDs given a negative count: %d", n)
	}
	if n == 0 {
		return 0, 0, nil
	}
	req := &pb.AllocateIdsRequest{
		ModelKey: keyToProto("", NewIncompleteKey(c, kind, parent)),
		Size:     proto.Int64(int64(n)),
	}
	res := &pb.AllocateIdsResponse{}
	if err := internal.Call(c, "datastore_v3", "AllocateIds", req, res); err != nil {
		return 0, 0, err
	}
	// The protobuf is inclusive at both ends. Idiomatic Go (e.g. slices, for loops)
	// is inclusive at the low end and exclusive at the high end, so we add 1.
	low = res.GetStart()
	high = res.GetEnd() + 1
	if low+int64(n) != high {
		return 0, 0, fmt.Errorf("datastore: internal error: could not allocate %d IDs", n)
	}
	return low, high, nil
}

// AllocateIDRange allocates a range of IDs with specific endpoints.
// The range is inclusive at both the low and high end. Once these IDs have been
// allocated, you can manually assign them to newly created entities.
//
// The Datastore's automatic ID allocator never assigns a key that has already
// been allocated (either through automatic ID allocation or through an explicit
// AllocateIDs call). As a result, entities written to the given key range will
// never be overwritten. However, writing entities with manually assigned keys in
// this range may overwrite existing entities (or new entities written by a separate
// request), depending on the error returned.
//
// Use this only if you have an existing numeric ID range that you want to reserve
// (for example, bulk loading entities that already have IDs). If you don't care
// about which IDs you receive, use AllocateIDs instead.
//
// AllocateIDRange returns nil if the range is successfully allocated. If one or more
// entities with an ID in the given range already exist, it returns a KeyRangeCollisionError.
// If the Datastore has already cached IDs in this range (e.g. from a previous call to
// AllocateIDRange), it returns a KeyRangeContentionError. Errors of other types indicate
// problems with arguments or an error returned directly from the Datastore.
func AllocateIDRange(c context.Context, kind string, parent *Key, start, end int64) (err error) {
	if kind == "" {
		return errors.New("datastore: AllocateIDRange given an empty kind")
	}

	if start < 1 || end < 1 {
		return errors.New("datastore: AllocateIDRange start and end must both be greater than 0")
	}

	if start > end {
		return errors.New("datastore: AllocateIDRange start must be before end")
	}

	req := &pb.AllocateIdsRequest{
		ModelKey: keyToProto("", NewIncompleteKey(c, kind, parent)),
		Max:      proto.Int64(end),
	}
	res := &pb.AllocateIdsResponse{}
	if err := internal.Call(c, "datastore_v3", "AllocateIds", req, res); err != nil {
		return err
	}

	// Check for collisions, i.e. existing entities with IDs in this range.
	// We could do this before the allocation, but we'd still have to do it
	// afterward as well to catch the race condition where an entity is inserted
	// after that initial check but before the allocation. Skip the up-front check
	// and just do it once.
	q := NewQuery(kind).Filter("__key__ >=", NewKey(c, kind, "", start, parent)).
		Filter("__key__ <=", NewKey(c, kind, "", end, parent)).KeysOnly().Limit(1)

	keys, err := q.GetAll(c, nil)
	if err != nil {
		return err
	}
	if len(keys) != 0 {
		return &KeyRangeCollisionError{start: start, end: end}
	}

	// Check for a race condition, i.e. cases where the datastore may have
	// cached ID batches that contain IDs in this range.
	if start < res.GetStart() {
		return &KeyRangeContentionError{start: start, end: end}
	}

	return nil
}
