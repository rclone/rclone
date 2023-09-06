// Copyright 2019 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fs

import (
	"context"
	"sync"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fuse"
)

// MemRegularFile is a filesystem node that holds a read-only data
// slice in memory.
type MemRegularFile struct {
	Inode

	mu   sync.Mutex
	Data []byte
	Attr fuse.Attr
}

var _ = (NodeOpener)((*MemRegularFile)(nil))
var _ = (NodeReader)((*MemRegularFile)(nil))
var _ = (NodeWriter)((*MemRegularFile)(nil))
var _ = (NodeSetattrer)((*MemRegularFile)(nil))
var _ = (NodeFlusher)((*MemRegularFile)(nil))

func (f *MemRegularFile) Open(ctx context.Context, flags uint32) (fh FileHandle, fuseFlags uint32, errno syscall.Errno) {
	return nil, fuse.FOPEN_KEEP_CACHE, OK
}

func (f *MemRegularFile) Write(ctx context.Context, fh FileHandle, data []byte, off int64) (uint32, syscall.Errno) {
	f.mu.Lock()
	defer f.mu.Unlock()
	end := int64(len(data)) + off
	if int64(len(f.Data)) < end {
		n := make([]byte, end)
		copy(n, f.Data)
		f.Data = n
	}

	copy(f.Data[off:off+int64(len(data))], data)

	return uint32(len(data)), 0
}

var _ = (NodeGetattrer)((*MemRegularFile)(nil))

func (f *MemRegularFile) Getattr(ctx context.Context, fh FileHandle, out *fuse.AttrOut) syscall.Errno {
	f.mu.Lock()
	defer f.mu.Unlock()
	out.Attr = f.Attr
	out.Attr.Size = uint64(len(f.Data))
	return OK
}

func (f *MemRegularFile) Setattr(ctx context.Context, fh FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	f.mu.Lock()
	defer f.mu.Unlock()
	if sz, ok := in.GetSize(); ok {
		f.Data = f.Data[:sz]
	}
	out.Attr = f.Attr
	out.Size = uint64(len(f.Data))
	return OK
}

func (f *MemRegularFile) Flush(ctx context.Context, fh FileHandle) syscall.Errno {
	return 0
}

func (f *MemRegularFile) Read(ctx context.Context, fh FileHandle, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	f.mu.Lock()
	defer f.mu.Unlock()
	end := int(off) + len(dest)
	if end > len(f.Data) {
		end = len(f.Data)
	}
	return fuse.ReadResultData(f.Data[off:end]), OK
}

// MemSymlink is an inode holding a symlink in memory.
type MemSymlink struct {
	Inode
	Attr fuse.Attr
	Data []byte
}

var _ = (NodeReadlinker)((*MemSymlink)(nil))

func (l *MemSymlink) Readlink(ctx context.Context) ([]byte, syscall.Errno) {
	return l.Data, OK
}

var _ = (NodeGetattrer)((*MemSymlink)(nil))

func (l *MemSymlink) Getattr(ctx context.Context, fh FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Attr = l.Attr
	return OK
}
