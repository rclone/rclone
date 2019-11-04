// +build darwin

// Copyright 2019 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fs

import (
	"context"
	"syscall"
	"time"
	"unsafe"

	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/hanwen/go-fuse/v2/internal/utimens"
)

func (n *loopbackNode) Getxattr(ctx context.Context, attr string, dest []byte) (uint32, syscall.Errno) {
	return 0, syscall.ENOSYS
}

func (n *loopbackNode) Setxattr(ctx context.Context, attr string, data []byte, flags uint32) syscall.Errno {
	return syscall.ENOSYS
}

func (n *loopbackNode) Removexattr(ctx context.Context, attr string) syscall.Errno {
	return syscall.ENOSYS
}

func (n *loopbackNode) Listxattr(ctx context.Context, dest []byte) (uint32, syscall.Errno) {
	return 0, syscall.ENOSYS
}

func (n *loopbackNode) renameExchange(name string, newparent *loopbackNode, newName string) syscall.Errno {
	return syscall.ENOSYS
}

func (f *loopbackFile) Allocate(ctx context.Context, off uint64, sz uint64, mode uint32) syscall.Errno {
	// TODO: Handle `mode` parameter.

	// From `man fcntl` on OSX:
	//     The F_PREALLOCATE command operates on the following structure:
	//
	//             typedef struct fstore {
	//                 u_int32_t fst_flags;      /* IN: flags word */
	//                 int       fst_posmode;    /* IN: indicates offset field */
	//                 off_t     fst_offset;     /* IN: start of the region */
	//                 off_t     fst_length;     /* IN: size of the region */
	//                 off_t     fst_bytesalloc; /* OUT: number of bytes allocated */
	//             } fstore_t;
	//
	//     The flags (fst_flags) for the F_PREALLOCATE command are as follows:
	//
	//           F_ALLOCATECONTIG   Allocate contiguous space.
	//
	//           F_ALLOCATEALL      Allocate all requested space or no space at all.
	//
	//     The position modes (fst_posmode) for the F_PREALLOCATE command indicate how to use the offset field.  The modes are as fol-
	//     lows:
	//
	//           F_PEOFPOSMODE   Allocate from the physical end of file.
	//
	//           F_VOLPOSMODE    Allocate from the volume offset.

	k := struct {
		Flags      uint32 // u_int32_t
		Posmode    int64  // int
		Offset     int64  // off_t
		Length     int64  // off_t
		Bytesalloc int64  // off_t
	}{
		0,
		0,
		int64(off),
		int64(sz),
		0,
	}

	// Linux version for reference:
	// err := syscall.Fallocate(int(f.File.Fd()), mode, int64(off), int64(sz))
	_, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(f.fd), uintptr(syscall.F_PREALLOCATE), uintptr(unsafe.Pointer(&k)))

	return errno
}

// timeToTimeval - Convert time.Time to syscall.Timeval
//
// Note: This does not use syscall.NsecToTimespec because
// that does not work properly for times before 1970,
// see https://github.com/golang/go/issues/12777
func timeToTimeval(t *time.Time) syscall.Timeval {
	var tv syscall.Timeval
	tv.Usec = int32(t.Nanosecond() / 1000)
	tv.Sec = t.Unix()
	return tv
}

// MacOS before High Sierra lacks utimensat() and UTIME_OMIT.
// We emulate using utimes() and extra Getattr() calls.
func (f *loopbackFile) utimens(a *time.Time, m *time.Time) syscall.Errno {
	var attr fuse.AttrOut
	if a == nil || m == nil {
		errno := f.Getattr(context.Background(), &attr)
		if errno != 0 {
			return errno
		}
	}
	tv := utimens.Fill(a, m, &attr.Attr)
	err := syscall.Futimes(int(f.fd), tv)
	return ToErrno(err)
}

func (n *loopbackNode) CopyFileRange(ctx context.Context, fhIn FileHandle,
	offIn uint64, out *Inode, fhOut FileHandle, offOut uint64,
	len uint64, flags uint64) (uint32, syscall.Errno) {
	return 0, syscall.ENOSYS
}
