// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package encryption

import (
	"github.com/zeebo/errs"

	"storj.io/common/paths"
	"storj.io/common/storj"
)

// The Store allows one to find the matching most encrypted key and path for
// some unencrypted path. It also reports a mapping of encrypted to unencrypted paths
// at the searched for unencrypted path.
//
// For example, if the Store contains the mappings
//
//    b1, u1/u2/u3    => <e1/e2/e3, k3>
//    b1, u1/u2/u3/u4 => <e1/e2/e3/e4, k4>
//    b1, u1/u5       => <e1/e5, k5>
//    b1, u6          => <e6, k6>
//    b1, u6/u7/u8    => <e6/e7/e8, k8>
//    b2, u1          => <e1', k1'>
//
// Then the following lookups have outputs
//
//    b1, u1          => <{e2:u2, e5:u5}, u1, nil>
//    b1, u1/u2/u3    => <{e4:u4}, u1/u2/u3, <u1/u2/u3, e1/e2/e3, k3>>
//    b1, u1/u2/u3/u6 => <{}, u1/u2/u3/, <u1/u2/u3, e1/e2/e3, k3>>
//    b1, u1/u2/u3/u4 => <{}, u1/u2/u3/u4, <u1/u2/u3/u4, e1/e2/e3/e4, k4>>
//    b1, u6/u7       => <{e8:u8}, u6/, <u6, e6, k6>>
//    b2, u1          => <{}, u1, <u1, e1', k1'>>
type Store struct {
	roots             map[string]*node
	defaultKey        *storj.Key
	defaultPathCipher storj.CipherSuite

	// EncryptionBypass makes it so we can interoperate with
	// the network without having encryption keys. paths will be encrypted but
	// base64-encoded, and certain metadata will be unable to be retrieved.
	EncryptionBypass bool
}

// node is a node in the Store graph. It may contain an encryption key and encrypted path,
// a list of children nodes, and data to ensure a bijection between encrypted and unencrypted
// path entries.
type node struct {
	unenc    map[string]*node  // unenc => node
	unencMap map[string]string // unenc => enc
	enc      map[string]*node  // enc => node
	encMap   map[string]string // enc => unenc
	base     *Base
}

// Base represents a key with which to derive further keys at some encrypted/unencrypted path.
type Base struct {
	Unencrypted paths.Unencrypted
	Encrypted   paths.Encrypted
	Key         storj.Key
	PathCipher  storj.CipherSuite
	Default     bool
}

// clone returns a copy of the Base. The implementation can be simple because the
// types of its fields do not contain any references.
func (b *Base) clone() *Base {
	if b == nil {
		return nil
	}
	bc := *b
	return &bc
}

// NewStore constructs a Store.
func NewStore() *Store {
	return &Store{roots: make(map[string]*node)}
}

// newNode constructs a node.
func newNode() *node {
	return &node{
		unenc:    make(map[string]*node),
		unencMap: make(map[string]string),
		enc:      make(map[string]*node),
		encMap:   make(map[string]string),
	}
}

// SetDefaultKey adds a default key to be returned for any lookup that does not match a bucket.
func (s *Store) SetDefaultKey(defaultKey *storj.Key) {
	s.defaultKey = defaultKey
}

// GetDefaultKey returns the default key, or nil if none has been set.
func (s *Store) GetDefaultKey() *storj.Key {
	return s.defaultKey
}

// SetDefaultPathCipher  adds a default path cipher to be returned for any lookup that does not match a bucket.
func (s *Store) SetDefaultPathCipher(defaultPathCipher storj.CipherSuite) {
	s.defaultPathCipher = defaultPathCipher
}

// GetDefaultPathCipher returns the default path cipher, or EncUnspecified if none has been set.
func (s *Store) GetDefaultPathCipher() storj.CipherSuite {
	return s.defaultPathCipher
}

// Add creates a mapping from the unencrypted path to the encrypted path and key. It uses the current default cipher.
func (s *Store) Add(bucket string, unenc paths.Unencrypted, enc paths.Encrypted, key storj.Key) error {
	return s.AddWithCipher(bucket, unenc, enc, key, s.defaultPathCipher)
}

// AddWithCipher creates a mapping from the unencrypted path to the encrypted path and key with the given cipher.
func (s *Store) AddWithCipher(bucket string, unenc paths.Unencrypted, enc paths.Encrypted, key storj.Key, pathCipher storj.CipherSuite) error {
	root, ok := s.roots[bucket]
	if !ok {
		root = newNode()
	}

	// Perform the addition starting at the root node.
	if err := root.add(unenc.Iterator(), enc.Iterator(), &Base{
		Unencrypted: unenc,
		Encrypted:   enc,
		Key:         key,
		PathCipher:  pathCipher,
	}); err != nil {
		return err
	}

	// only update the root for the bucket if the add was successful.
	s.roots[bucket] = root
	return nil
}

// add places the paths and base into the node tree structure.
func (n *node) add(unenc, enc paths.Iterator, base *Base) error {
	if unenc.Done() != enc.Done() {
		return errs.New("encrypted and unencrypted paths had different number of components")
	}

	// If we're done walking the paths, this node must have the provided base.
	if unenc.Done() {
		n.base = base
		return nil
	}

	// Walk to the next parts and ensure they're consistent with previous additions.
	unencPart, encPart := unenc.Next(), enc.Next()
	if exUnencPart, ok := n.encMap[encPart]; ok && exUnencPart != unencPart {
		return errs.New("conflicting encrypted parts for unencrypted path")
	}
	if exEncPart, ok := n.unencMap[unencPart]; ok && exEncPart != encPart {
		return errs.New("conflicting encrypted parts for unencrypted path")
	}

	// Look up the child node. Since we're sure the unenc and enc mappings are
	// consistent, we can look it up in one map and unconditionally insert it
	// into both maps if necessary.
	child, ok := n.unenc[unencPart]
	if !ok {
		child = newNode()
	}

	// Recurse to the next node in the tree.
	if err := child.add(unenc, enc, base); err != nil {
		return err
	}

	// Only add to the maps if the child add was successful.
	n.unencMap[unencPart] = encPart
	n.encMap[encPart] = unencPart
	n.unenc[unencPart] = child
	n.enc[encPart] = child
	return nil
}

// LookupUnencrypted finds the matching most unencrypted path added to the Store, reports how
// much of the path matched, any known unencrypted paths at the requested path, and if a key
// and encrypted path exists for some prefix of the unencrypted path.
func (s *Store) LookupUnencrypted(bucket string, path paths.Unencrypted) (
	revealed map[string]string, consumed paths.Unencrypted, base *Base) {

	root, ok := s.roots[bucket]
	if ok {
		var rawConsumed string
		revealed, rawConsumed, base = root.lookup(path.Iterator(), "", nil, true)
		consumed = paths.NewUnencrypted(rawConsumed)
	}
	if base == nil && s.defaultKey != nil {
		return nil, paths.Unencrypted{}, s.defaultBase()
	}
	return revealed, consumed, base.clone()
}

// LookupEncrypted finds the matching most encrypted path added to the Store, reports how
// much of the path matched, any known encrypted paths at the requested path, and if a key
// an encrypted path exists for some prefix of the encrypted path.
func (s *Store) LookupEncrypted(bucket string, path paths.Encrypted) (
	revealed map[string]string, consumed paths.Encrypted, base *Base) {

	root, ok := s.roots[bucket]
	if ok {
		var rawConsumed string
		revealed, rawConsumed, base = root.lookup(path.Iterator(), "", nil, false)
		consumed = paths.NewEncrypted(rawConsumed)
	}
	if base == nil && s.defaultKey != nil {
		return nil, paths.Encrypted{}, s.defaultBase()
	}
	return revealed, consumed, base.clone()
}

func (s *Store) defaultBase() *Base {
	return &Base{
		Key:        *s.defaultKey,
		PathCipher: s.defaultPathCipher,
		Default:    true,
	}
}

// lookup searches for the path in the node tree structure.
func (n *node) lookup(path paths.Iterator, bestConsumed string, bestBase *Base, unenc bool) (
	map[string]string, string, *Base) {

	// Keep track of the best match so far.
	if n.base != nil || bestBase == nil {
		bestBase, bestConsumed = n.base, path.Consumed()
	}

	// Pick the tree we're walking down based on the unenc bool.
	revealed, children := n.unencMap, n.enc
	if unenc {
		revealed, children = n.encMap, n.unenc
	}

	// If we're done walking the path, then return our best match along with the
	// revealed paths at this node.
	if path.Done() {
		return revealed, bestConsumed, bestBase
	}

	// Walk to the next node in the tree. If there is no node, then report our best match.
	child, ok := children[path.Next()]
	if !ok {
		return nil, bestConsumed, bestBase
	}

	// Recurse to the next node in the tree.
	return child.lookup(path, bestConsumed, bestBase, unenc)
}

// Iterate executes the callback with every value that has been Added to the Store.
// NOTE: This call is lossy! Please upgrade any code paths to use IterateWithCipher!
func (s *Store) Iterate(fn func(string, paths.Unencrypted, paths.Encrypted, storj.Key) error) error {
	for bucket, root := range s.roots {
		if err := root.iterate(fn, bucket); err != nil {
			return err
		}
	}
	return nil
}

// iterate calls the callback if the node has a base, and recurses to its children.
func (n *node) iterate(fn func(string, paths.Unencrypted, paths.Encrypted, storj.Key) error, bucket string) error {
	if n.base != nil {
		err := fn(bucket, n.base.Unencrypted, n.base.Encrypted, n.base.Key)
		if err != nil {
			return err
		}
	}

	// recurse down only the unenc map, as the enc map should be the same.
	for _, child := range n.unenc {
		err := child.iterate(fn, bucket)
		if err != nil {
			return err
		}
	}

	return nil
}

// IterateWithCipher executes the callback with every value that has been Added to the Store.
func (s *Store) IterateWithCipher(fn func(string, paths.Unencrypted, paths.Encrypted, storj.Key, storj.CipherSuite) error) error {
	for bucket, root := range s.roots {
		if err := root.iterateWithCipher(fn, bucket); err != nil {
			return err
		}
	}
	return nil
}

// iterateWithCipher calls the callback if the node has a base, and recurses to its children.
func (n *node) iterateWithCipher(fn func(string, paths.Unencrypted, paths.Encrypted, storj.Key, storj.CipherSuite) error, bucket string) error {
	if n.base != nil {
		err := fn(bucket, n.base.Unencrypted, n.base.Encrypted, n.base.Key, n.base.PathCipher)
		if err != nil {
			return err
		}
	}

	// recurse down only the unenc map, as the enc map should be the same.
	for _, child := range n.unenc {
		err := child.iterateWithCipher(fn, bucket)
		if err != nil {
			return err
		}
	}

	return nil
}
