// Copyright 2019 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fs

import (
	"syscall"

	"github.com/hanwen/go-fuse/v2/fuse"
)

type dirArray struct {
	entries []fuse.DirEntry
}

func (a *dirArray) HasNext() bool {
	return len(a.entries) > 0
}

func (a *dirArray) Next() (fuse.DirEntry, syscall.Errno) {
	e := a.entries[0]
	a.entries = a.entries[1:]
	return e, 0
}

func (a *dirArray) Close() {

}

// NewListDirStream wraps a slice of DirEntry as a DirStream.
func NewListDirStream(list []fuse.DirEntry) DirStream {
	return &dirArray{list}
}
