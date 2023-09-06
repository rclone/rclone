// Copyright 2016 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fuse

// all of the code for DirEntryList.

import (
	"fmt"
	"unsafe"
)

var eightPadding [8]byte

const direntSize = int(unsafe.Sizeof(_Dirent{}))

// DirEntry is a type for PathFileSystem and NodeFileSystem to return
// directory contents in.
type DirEntry struct {
	// Mode is the file's mode. Only the high bits (eg. S_IFDIR)
	// are considered.
	Mode uint32

	// Name is the basename of the file in the directory.
	Name string

	// Ino is the inode number.
	Ino uint64
}

func (d DirEntry) String() string {
	return fmt.Sprintf("%o: %q ino=%d", d.Mode, d.Name, d.Ino)
}

// DirEntryList holds the return value for READDIR and READDIRPLUS
// opcodes.
type DirEntryList struct {
	buf []byte
	// capacity of the underlying buffer
	size int
	// offset is the requested location in the directory. go-fuse
	// currently counts in number of directory entries, but this is an
	// implementation detail and may change in the future.
	// If `offset` and `fs.fileEntry.dirOffset` disagree, then a
	// directory seek has taken place.
	offset uint64
	// pointer to the last serialized _Dirent. Used by FixMode().
	lastDirent *_Dirent
}

// NewDirEntryList creates a DirEntryList with the given data buffer
// and offset.
func NewDirEntryList(data []byte, off uint64) *DirEntryList {
	return &DirEntryList{
		buf:    data[:0],
		size:   len(data),
		offset: off,
	}
}

// AddDirEntry tries to add an entry, and reports whether it
// succeeded.
func (l *DirEntryList) AddDirEntry(e DirEntry) bool {
	return l.Add(0, e.Name, e.Ino, e.Mode)
}

// Add adds a direntry to the DirEntryList, returning whether it
// succeeded.
func (l *DirEntryList) Add(prefix int, name string, inode uint64, mode uint32) bool {
	if inode == 0 {
		inode = FUSE_UNKNOWN_INO
	}
	padding := (8 - len(name)&7) & 7
	delta := padding + direntSize + len(name) + prefix
	oldLen := len(l.buf)
	newLen := delta + oldLen

	if newLen > l.size {
		return false
	}
	l.buf = l.buf[:newLen]
	oldLen += prefix
	dirent := (*_Dirent)(unsafe.Pointer(&l.buf[oldLen]))
	dirent.Off = l.offset + 1
	dirent.Ino = inode
	dirent.NameLen = uint32(len(name))
	dirent.Typ = modeToType(mode)
	oldLen += direntSize
	copy(l.buf[oldLen:], name)
	oldLen += len(name)

	if padding > 0 {
		copy(l.buf[oldLen:], eightPadding[:padding])
	}

	l.offset = dirent.Off
	return true
}

// AddDirLookupEntry is used for ReadDirPlus. If reserves and zeroizes space
// for an EntryOut struct and serializes a DirEntry.
// On success, it returns pointers to both structs.
// If not enough space was left, it returns two nil pointers.
//
// The resulting READDIRPLUS output buffer looks like this in memory:
// 1) EntryOut{}
// 2) _Dirent{}
// 3) Name (null-terminated)
// 4) Padding to align to 8 bytes
// [repeat]
func (l *DirEntryList) AddDirLookupEntry(e DirEntry) *EntryOut {
	const entryOutSize = int(unsafe.Sizeof(EntryOut{}))
	oldLen := len(l.buf)
	ok := l.Add(entryOutSize, e.Name, e.Ino, e.Mode)
	if !ok {
		return nil
	}
	l.lastDirent = (*_Dirent)(unsafe.Pointer(&l.buf[oldLen+entryOutSize]))
	entryOut := (*EntryOut)(unsafe.Pointer(&l.buf[oldLen]))
	*entryOut = EntryOut{} // zeroize
	return entryOut
}

// modeToType converts a file *mode* (as used in syscall.Stat_t.Mode)
// to a file *type* (as used in _Dirent.Typ).
// Equivalent to IFTODT() in libc (see man 5 dirent).
func modeToType(mode uint32) uint32 {
	return (mode & 0170000) >> 12
}

// FixMode overrides the file mode of the last direntry that was added. This can
// be needed when a directory changes while READDIRPLUS is running.
// Only the file type bits of mode are considered, the rest is masked out.
func (l *DirEntryList) FixMode(mode uint32) {
	l.lastDirent.Typ = modeToType(mode)
}

func (l *DirEntryList) bytes() []byte {
	return l.buf
}
