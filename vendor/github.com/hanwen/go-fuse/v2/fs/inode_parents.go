// Copyright 2021 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fs

// inodeParents stores zero or more parents of an Inode,
// remembering which one is the most recent.
//
// No internal locking: the caller is responsible for preventing
// concurrent access.
type inodeParents struct {
	// newest is the most-recently add()'ed parent.
	// nil when we don't have any parents.
	newest *parentData
	// other are parents in addition to the newest.
	// nil or empty when we have <= 1 parents.
	other map[parentData]struct{}
}

// add adds a parent to the store.
func (p *inodeParents) add(n parentData) {
	// one and only parent
	if p.newest == nil {
		p.newest = &n
	}
	// already known as `newest`
	if *p.newest == n {
		return
	}
	// old `newest` gets displaced into `other`
	if p.other == nil {
		p.other = make(map[parentData]struct{})
	}
	p.other[*p.newest] = struct{}{}
	// new parent becomes `newest` (possibly moving up from `other`)
	delete(p.other, n)
	p.newest = &n
}

// get returns the most recent parent
// or nil if there is no parent at all.
func (p *inodeParents) get() *parentData {
	return p.newest
}

// all returns all known parents
// or nil if there is no parent at all.
func (p *inodeParents) all() []parentData {
	count := p.count()
	if count == 0 {
		return nil
	}
	out := make([]parentData, 0, count)
	out = append(out, *p.newest)
	for i := range p.other {
		out = append(out, i)
	}
	return out
}

func (p *inodeParents) delete(n parentData) {
	// We have zero parents, so we can't delete any.
	if p.newest == nil {
		return
	}
	// If it's not the `newest` it must be in `other` (or nowhere).
	if *p.newest != n {
		delete(p.other, n)
		return
	}
	// We want to delete `newest`, but there is no other to replace it.
	if len(p.other) == 0 {
		p.newest = nil
		return
	}
	// Move random entry from `other` over `newest`.
	var i parentData
	for i = range p.other {
		p.newest = &i
		break
	}
	delete(p.other, i)
}

func (p *inodeParents) clear() {
	p.newest = nil
	p.other = nil
}

func (p *inodeParents) count() int {
	if p.newest == nil {
		return 0
	}
	return 1 + len(p.other)
}

type parentData struct {
	name   string
	parent *Inode
}
