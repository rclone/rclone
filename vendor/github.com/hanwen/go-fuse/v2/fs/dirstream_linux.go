// Copyright 2019 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fs

import (
	"sync"
	"syscall"
	"unsafe"

	"github.com/hanwen/go-fuse/v2/fuse"
)

type loopbackDirStream struct {
	buf  []byte
	todo []byte

	// Protects fd so we can guard against double close
	mu sync.Mutex
	fd int
}

// NewLoopbackDirStream open a directory for reading as a DirStream
func NewLoopbackDirStream(name string) (DirStream, syscall.Errno) {
	fd, err := syscall.Open(name, syscall.O_DIRECTORY, 0755)
	if err != nil {
		return nil, ToErrno(err)
	}

	ds := &loopbackDirStream{
		buf: make([]byte, 4096),
		fd:  fd,
	}

	if err := ds.load(); err != 0 {
		ds.Close()
		return nil, err
	}
	return ds, OK
}

func (ds *loopbackDirStream) Close() {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	if ds.fd != -1 {
		syscall.Close(ds.fd)
		ds.fd = -1
	}
}

func (ds *loopbackDirStream) HasNext() bool {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	return len(ds.todo) > 0
}

func (ds *loopbackDirStream) Next() (fuse.DirEntry, syscall.Errno) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	de := (*syscall.Dirent)(unsafe.Pointer(&ds.todo[0]))

	nameBytes := ds.todo[unsafe.Offsetof(syscall.Dirent{}.Name):de.Reclen]
	ds.todo = ds.todo[de.Reclen:]

	// After the loop, l contains the index of the first '\0'.
	l := 0
	for l = range nameBytes {
		if nameBytes[l] == 0 {
			break
		}
	}
	nameBytes = nameBytes[:l]
	result := fuse.DirEntry{
		Ino:  de.Ino,
		Mode: (uint32(de.Type) << 12),
		Name: string(nameBytes),
	}
	return result, ds.load()
}

func (ds *loopbackDirStream) load() syscall.Errno {
	if len(ds.todo) > 0 {
		return OK
	}

	n, err := syscall.Getdents(ds.fd, ds.buf)
	if err != nil {
		return ToErrno(err)
	}
	ds.todo = ds.buf[:n]
	return OK
}
