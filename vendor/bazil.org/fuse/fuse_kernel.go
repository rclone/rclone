// See the file LICENSE for copyright and licensing information.

// Derived from FUSE's fuse_kernel.h, which carries this notice:
/*
   This file defines the kernel interface of FUSE
   Copyright (C) 2001-2007  Miklos Szeredi <miklos@szeredi.hu>


   This -- and only this -- header file may also be distributed under
   the terms of the BSD Licence as follows:

   Copyright (C) 2001-2007 Miklos Szeredi. All rights reserved.

   Redistribution and use in source and binary forms, with or without
   modification, are permitted provided that the following conditions
   are met:
   1. Redistributions of source code must retain the above copyright
      notice, this list of conditions and the following disclaimer.
   2. Redistributions in binary form must reproduce the above copyright
      notice, this list of conditions and the following disclaimer in the
      documentation and/or other materials provided with the distribution.

   THIS SOFTWARE IS PROVIDED BY AUTHOR AND CONTRIBUTORS ``AS IS'' AND
   ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
   IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
   ARE DISCLAIMED.  IN NO EVENT SHALL AUTHOR OR CONTRIBUTORS BE LIABLE
   FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
   DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS
   OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION)
   HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT
   LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY
   OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF
   SUCH DAMAGE.
*/

package fuse

import (
	"fmt"
	"syscall"
	"unsafe"
)

// The FUSE version implemented by the package.
const (
	protoVersionMinMajor = 7
	protoVersionMinMinor = 8
	protoVersionMaxMajor = 7
	protoVersionMaxMinor = 12
)

const (
	rootID = 1
)

type kstatfs struct {
	Blocks  uint64
	Bfree   uint64
	Bavail  uint64
	Files   uint64
	Ffree   uint64
	Bsize   uint32
	Namelen uint32
	Frsize  uint32
	_       uint32
	Spare   [6]uint32
}

type fileLock struct {
	Start uint64
	End   uint64
	Type  uint32
	Pid   uint32
}

// GetattrFlags are bit flags that can be seen in GetattrRequest.
type GetattrFlags uint32

const (
	// Indicates the handle is valid.
	GetattrFh GetattrFlags = 1 << 0
)

var getattrFlagsNames = []flagName{
	{uint32(GetattrFh), "GetattrFh"},
}

func (fl GetattrFlags) String() string {
	return flagString(uint32(fl), getattrFlagsNames)
}

// The SetattrValid are bit flags describing which fields in the SetattrRequest
// are included in the change.
type SetattrValid uint32

const (
	SetattrMode   SetattrValid = 1 << 0
	SetattrUid    SetattrValid = 1 << 1
	SetattrGid    SetattrValid = 1 << 2
	SetattrSize   SetattrValid = 1 << 3
	SetattrAtime  SetattrValid = 1 << 4
	SetattrMtime  SetattrValid = 1 << 5
	SetattrHandle SetattrValid = 1 << 6

	// Linux only(?)
	SetattrAtimeNow  SetattrValid = 1 << 7
	SetattrMtimeNow  SetattrValid = 1 << 8
	SetattrLockOwner SetattrValid = 1 << 9 // http://www.mail-archive.com/git-commits-head@vger.kernel.org/msg27852.html

	// OS X only
	SetattrCrtime   SetattrValid = 1 << 28
	SetattrChgtime  SetattrValid = 1 << 29
	SetattrBkuptime SetattrValid = 1 << 30
	SetattrFlags    SetattrValid = 1 << 31
)

func (fl SetattrValid) Mode() bool      { return fl&SetattrMode != 0 }
func (fl SetattrValid) Uid() bool       { return fl&SetattrUid != 0 }
func (fl SetattrValid) Gid() bool       { return fl&SetattrGid != 0 }
func (fl SetattrValid) Size() bool      { return fl&SetattrSize != 0 }
func (fl SetattrValid) Atime() bool     { return fl&SetattrAtime != 0 }
func (fl SetattrValid) Mtime() bool     { return fl&SetattrMtime != 0 }
func (fl SetattrValid) Handle() bool    { return fl&SetattrHandle != 0 }
func (fl SetattrValid) AtimeNow() bool  { return fl&SetattrAtimeNow != 0 }
func (fl SetattrValid) MtimeNow() bool  { return fl&SetattrMtimeNow != 0 }
func (fl SetattrValid) LockOwner() bool { return fl&SetattrLockOwner != 0 }
func (fl SetattrValid) Crtime() bool    { return fl&SetattrCrtime != 0 }
func (fl SetattrValid) Chgtime() bool   { return fl&SetattrChgtime != 0 }
func (fl SetattrValid) Bkuptime() bool  { return fl&SetattrBkuptime != 0 }
func (fl SetattrValid) Flags() bool     { return fl&SetattrFlags != 0 }

func (fl SetattrValid) String() string {
	return flagString(uint32(fl), setattrValidNames)
}

var setattrValidNames = []flagName{
	{uint32(SetattrMode), "SetattrMode"},
	{uint32(SetattrUid), "SetattrUid"},
	{uint32(SetattrGid), "SetattrGid"},
	{uint32(SetattrSize), "SetattrSize"},
	{uint32(SetattrAtime), "SetattrAtime"},
	{uint32(SetattrMtime), "SetattrMtime"},
	{uint32(SetattrHandle), "SetattrHandle"},
	{uint32(SetattrAtimeNow), "SetattrAtimeNow"},
	{uint32(SetattrMtimeNow), "SetattrMtimeNow"},
	{uint32(SetattrLockOwner), "SetattrLockOwner"},
	{uint32(SetattrCrtime), "SetattrCrtime"},
	{uint32(SetattrChgtime), "SetattrChgtime"},
	{uint32(SetattrBkuptime), "SetattrBkuptime"},
	{uint32(SetattrFlags), "SetattrFlags"},
}

// Flags that can be seen in OpenRequest.Flags.
const (
	// Access modes. These are not 1-bit flags, but alternatives where
	// only one can be chosen. See the IsReadOnly etc convenience
	// methods.
	OpenReadOnly  OpenFlags = syscall.O_RDONLY
	OpenWriteOnly OpenFlags = syscall.O_WRONLY
	OpenReadWrite OpenFlags = syscall.O_RDWR

	// File was opened in append-only mode, all writes will go to end
	// of file. OS X does not provide this information.
	OpenAppend    OpenFlags = syscall.O_APPEND
	OpenCreate    OpenFlags = syscall.O_CREAT
	OpenDirectory OpenFlags = syscall.O_DIRECTORY
	OpenExclusive OpenFlags = syscall.O_EXCL
	OpenNonblock  OpenFlags = syscall.O_NONBLOCK
	OpenSync      OpenFlags = syscall.O_SYNC
	OpenTruncate  OpenFlags = syscall.O_TRUNC
)

// OpenAccessModeMask is a bitmask that separates the access mode
// from the other flags in OpenFlags.
const OpenAccessModeMask OpenFlags = syscall.O_ACCMODE

// OpenFlags are the O_FOO flags passed to open/create/etc calls. For
// example, os.O_WRONLY | os.O_APPEND.
type OpenFlags uint32

func (fl OpenFlags) String() string {
	// O_RDONLY, O_RWONLY, O_RDWR are not flags
	s := accModeName(fl & OpenAccessModeMask)
	flags := uint32(fl &^ OpenAccessModeMask)
	if flags != 0 {
		s = s + "+" + flagString(flags, openFlagNames)
	}
	return s
}

// Return true if OpenReadOnly is set.
func (fl OpenFlags) IsReadOnly() bool {
	return fl&OpenAccessModeMask == OpenReadOnly
}

// Return true if OpenWriteOnly is set.
func (fl OpenFlags) IsWriteOnly() bool {
	return fl&OpenAccessModeMask == OpenWriteOnly
}

// Return true if OpenReadWrite is set.
func (fl OpenFlags) IsReadWrite() bool {
	return fl&OpenAccessModeMask == OpenReadWrite
}

func accModeName(flags OpenFlags) string {
	switch flags {
	case OpenReadOnly:
		return "OpenReadOnly"
	case OpenWriteOnly:
		return "OpenWriteOnly"
	case OpenReadWrite:
		return "OpenReadWrite"
	default:
		return ""
	}
}

var openFlagNames = []flagName{
	{uint32(OpenAppend), "OpenAppend"},
	{uint32(OpenCreate), "OpenCreate"},
	{uint32(OpenDirectory), "OpenDirectory"},
	{uint32(OpenExclusive), "OpenExclusive"},
	{uint32(OpenNonblock), "OpenNonblock"},
	{uint32(OpenSync), "OpenSync"},
	{uint32(OpenTruncate), "OpenTruncate"},
}

// The OpenResponseFlags are returned in the OpenResponse.
type OpenResponseFlags uint32

const (
	OpenDirectIO    OpenResponseFlags = 1 << 0 // bypass page cache for this open file
	OpenKeepCache   OpenResponseFlags = 1 << 1 // don't invalidate the data cache on open
	OpenNonSeekable OpenResponseFlags = 1 << 2 // mark the file as non-seekable (not supported on OS X)

	OpenPurgeAttr OpenResponseFlags = 1 << 30 // OS X
	OpenPurgeUBC  OpenResponseFlags = 1 << 31 // OS X
)

func (fl OpenResponseFlags) String() string {
	return flagString(uint32(fl), openResponseFlagNames)
}

var openResponseFlagNames = []flagName{
	{uint32(OpenDirectIO), "OpenDirectIO"},
	{uint32(OpenKeepCache), "OpenKeepCache"},
	{uint32(OpenNonSeekable), "OpenNonSeekable"},
	{uint32(OpenPurgeAttr), "OpenPurgeAttr"},
	{uint32(OpenPurgeUBC), "OpenPurgeUBC"},
}

// The InitFlags are used in the Init exchange.
type InitFlags uint32

const (
	InitAsyncRead     InitFlags = 1 << 0
	InitPosixLocks    InitFlags = 1 << 1
	InitFileOps       InitFlags = 1 << 2
	InitAtomicTrunc   InitFlags = 1 << 3
	InitExportSupport InitFlags = 1 << 4
	InitBigWrites     InitFlags = 1 << 5
	// Do not mask file access modes with umask. Not supported on OS X.
	InitDontMask        InitFlags = 1 << 6
	InitSpliceWrite     InitFlags = 1 << 7
	InitSpliceMove      InitFlags = 1 << 8
	InitSpliceRead      InitFlags = 1 << 9
	InitFlockLocks      InitFlags = 1 << 10
	InitHasIoctlDir     InitFlags = 1 << 11
	InitAutoInvalData   InitFlags = 1 << 12
	InitDoReaddirplus   InitFlags = 1 << 13
	InitReaddirplusAuto InitFlags = 1 << 14
	InitAsyncDIO        InitFlags = 1 << 15
	InitWritebackCache  InitFlags = 1 << 16
	InitNoOpenSupport   InitFlags = 1 << 17

	InitCaseSensitive InitFlags = 1 << 29 // OS X only
	InitVolRename     InitFlags = 1 << 30 // OS X only
	InitXtimes        InitFlags = 1 << 31 // OS X only
)

type flagName struct {
	bit  uint32
	name string
}

var initFlagNames = []flagName{
	{uint32(InitAsyncRead), "InitAsyncRead"},
	{uint32(InitPosixLocks), "InitPosixLocks"},
	{uint32(InitFileOps), "InitFileOps"},
	{uint32(InitAtomicTrunc), "InitAtomicTrunc"},
	{uint32(InitExportSupport), "InitExportSupport"},
	{uint32(InitBigWrites), "InitBigWrites"},
	{uint32(InitDontMask), "InitDontMask"},
	{uint32(InitSpliceWrite), "InitSpliceWrite"},
	{uint32(InitSpliceMove), "InitSpliceMove"},
	{uint32(InitSpliceRead), "InitSpliceRead"},
	{uint32(InitFlockLocks), "InitFlockLocks"},
	{uint32(InitHasIoctlDir), "InitHasIoctlDir"},
	{uint32(InitAutoInvalData), "InitAutoInvalData"},
	{uint32(InitDoReaddirplus), "InitDoReaddirplus"},
	{uint32(InitReaddirplusAuto), "InitReaddirplusAuto"},
	{uint32(InitAsyncDIO), "InitAsyncDIO"},
	{uint32(InitWritebackCache), "InitWritebackCache"},
	{uint32(InitNoOpenSupport), "InitNoOpenSupport"},

	{uint32(InitCaseSensitive), "InitCaseSensitive"},
	{uint32(InitVolRename), "InitVolRename"},
	{uint32(InitXtimes), "InitXtimes"},
}

func (fl InitFlags) String() string {
	return flagString(uint32(fl), initFlagNames)
}

func flagString(f uint32, names []flagName) string {
	var s string

	if f == 0 {
		return "0"
	}

	for _, n := range names {
		if f&n.bit != 0 {
			s += "+" + n.name
			f &^= n.bit
		}
	}
	if f != 0 {
		s += fmt.Sprintf("%+#x", f)
	}
	return s[1:]
}

// The ReleaseFlags are used in the Release exchange.
type ReleaseFlags uint32

const (
	ReleaseFlush ReleaseFlags = 1 << 0
)

func (fl ReleaseFlags) String() string {
	return flagString(uint32(fl), releaseFlagNames)
}

var releaseFlagNames = []flagName{
	{uint32(ReleaseFlush), "ReleaseFlush"},
}

// Opcodes
const (
	opLookup      = 1
	opForget      = 2 // no reply
	opGetattr     = 3
	opSetattr     = 4
	opReadlink    = 5
	opSymlink     = 6
	opMknod       = 8
	opMkdir       = 9
	opUnlink      = 10
	opRmdir       = 11
	opRename      = 12
	opLink        = 13
	opOpen        = 14
	opRead        = 15
	opWrite       = 16
	opStatfs      = 17
	opRelease     = 18
	opFsync       = 20
	opSetxattr    = 21
	opGetxattr    = 22
	opListxattr   = 23
	opRemovexattr = 24
	opFlush       = 25
	opInit        = 26
	opOpendir     = 27
	opReaddir     = 28
	opReleasedir  = 29
	opFsyncdir    = 30
	opGetlk       = 31
	opSetlk       = 32
	opSetlkw      = 33
	opAccess      = 34
	opCreate      = 35
	opInterrupt   = 36
	opBmap        = 37
	opDestroy     = 38
	opIoctl       = 39 // Linux?
	opPoll        = 40 // Linux?

	// OS X
	opSetvolname = 61
	opGetxtimes  = 62
	opExchange   = 63
)

type entryOut struct {
	Nodeid         uint64 // Inode ID
	Generation     uint64 // Inode generation
	EntryValid     uint64 // Cache timeout for the name
	AttrValid      uint64 // Cache timeout for the attributes
	EntryValidNsec uint32
	AttrValidNsec  uint32
	Attr           attr
}

func entryOutSize(p Protocol) uintptr {
	switch {
	case p.LT(Protocol{7, 9}):
		return unsafe.Offsetof(entryOut{}.Attr) + unsafe.Offsetof(entryOut{}.Attr.Blksize)
	default:
		return unsafe.Sizeof(entryOut{})
	}
}

type forgetIn struct {
	Nlookup uint64
}

type getattrIn struct {
	GetattrFlags uint32
	_            uint32
	Fh           uint64
}

type attrOut struct {
	AttrValid     uint64 // Cache timeout for the attributes
	AttrValidNsec uint32
	_             uint32
	Attr          attr
}

func attrOutSize(p Protocol) uintptr {
	switch {
	case p.LT(Protocol{7, 9}):
		return unsafe.Offsetof(attrOut{}.Attr) + unsafe.Offsetof(attrOut{}.Attr.Blksize)
	default:
		return unsafe.Sizeof(attrOut{})
	}
}

// OS X
type getxtimesOut struct {
	Bkuptime     uint64
	Crtime       uint64
	BkuptimeNsec uint32
	CrtimeNsec   uint32
}

type mknodIn struct {
	Mode  uint32
	Rdev  uint32
	Umask uint32
	_     uint32
	// "filename\x00" follows.
}

func mknodInSize(p Protocol) uintptr {
	switch {
	case p.LT(Protocol{7, 12}):
		return unsafe.Offsetof(mknodIn{}.Umask)
	default:
		return unsafe.Sizeof(mknodIn{})
	}
}

type mkdirIn struct {
	Mode  uint32
	Umask uint32
	// filename follows
}

func mkdirInSize(p Protocol) uintptr {
	switch {
	case p.LT(Protocol{7, 12}):
		return unsafe.Offsetof(mkdirIn{}.Umask) + 4
	default:
		return unsafe.Sizeof(mkdirIn{})
	}
}

type renameIn struct {
	Newdir uint64
	// "oldname\x00newname\x00" follows
}

// OS X
type exchangeIn struct {
	Olddir  uint64
	Newdir  uint64
	Options uint64
	// "oldname\x00newname\x00" follows
}

type linkIn struct {
	Oldnodeid uint64
}

type setattrInCommon struct {
	Valid     uint32
	_         uint32
	Fh        uint64
	Size      uint64
	LockOwner uint64 // unused on OS X?
	Atime     uint64
	Mtime     uint64
	Unused2   uint64
	AtimeNsec uint32
	MtimeNsec uint32
	Unused3   uint32
	Mode      uint32
	Unused4   uint32
	Uid       uint32
	Gid       uint32
	Unused5   uint32
}

type openIn struct {
	Flags  uint32
	Unused uint32
}

type openOut struct {
	Fh        uint64
	OpenFlags uint32
	_         uint32
}

type createIn struct {
	Flags uint32
	Mode  uint32
	Umask uint32
	_     uint32
}

func createInSize(p Protocol) uintptr {
	switch {
	case p.LT(Protocol{7, 12}):
		return unsafe.Offsetof(createIn{}.Umask)
	default:
		return unsafe.Sizeof(createIn{})
	}
}

type releaseIn struct {
	Fh           uint64
	Flags        uint32
	ReleaseFlags uint32
	LockOwner    uint32
}

type flushIn struct {
	Fh         uint64
	FlushFlags uint32
	_          uint32
	LockOwner  uint64
}

type readIn struct {
	Fh        uint64
	Offset    uint64
	Size      uint32
	ReadFlags uint32
	LockOwner uint64
	Flags     uint32
	_         uint32
}

func readInSize(p Protocol) uintptr {
	switch {
	case p.LT(Protocol{7, 9}):
		return unsafe.Offsetof(readIn{}.ReadFlags) + 4
	default:
		return unsafe.Sizeof(readIn{})
	}
}

// The ReadFlags are passed in ReadRequest.
type ReadFlags uint32

const (
	// LockOwner field is valid.
	ReadLockOwner ReadFlags = 1 << 1
)

var readFlagNames = []flagName{
	{uint32(ReadLockOwner), "ReadLockOwner"},
}

func (fl ReadFlags) String() string {
	return flagString(uint32(fl), readFlagNames)
}

type writeIn struct {
	Fh         uint64
	Offset     uint64
	Size       uint32
	WriteFlags uint32
	LockOwner  uint64
	Flags      uint32
	_          uint32
}

func writeInSize(p Protocol) uintptr {
	switch {
	case p.LT(Protocol{7, 9}):
		return unsafe.Offsetof(writeIn{}.LockOwner)
	default:
		return unsafe.Sizeof(writeIn{})
	}
}

type writeOut struct {
	Size uint32
	_    uint32
}

// The WriteFlags are passed in WriteRequest.
type WriteFlags uint32

const (
	WriteCache WriteFlags = 1 << 0
	// LockOwner field is valid.
	WriteLockOwner WriteFlags = 1 << 1
)

var writeFlagNames = []flagName{
	{uint32(WriteCache), "WriteCache"},
	{uint32(WriteLockOwner), "WriteLockOwner"},
}

func (fl WriteFlags) String() string {
	return flagString(uint32(fl), writeFlagNames)
}

const compatStatfsSize = 48

type statfsOut struct {
	St kstatfs
}

type fsyncIn struct {
	Fh         uint64
	FsyncFlags uint32
	_          uint32
}

type setxattrInCommon struct {
	Size  uint32
	Flags uint32
}

func (setxattrInCommon) position() uint32 {
	return 0
}

type getxattrInCommon struct {
	Size uint32
	_    uint32
}

func (getxattrInCommon) position() uint32 {
	return 0
}

type getxattrOut struct {
	Size uint32
	_    uint32
}

type lkIn struct {
	Fh      uint64
	Owner   uint64
	Lk      fileLock
	LkFlags uint32
	_       uint32
}

func lkInSize(p Protocol) uintptr {
	switch {
	case p.LT(Protocol{7, 9}):
		return unsafe.Offsetof(lkIn{}.LkFlags)
	default:
		return unsafe.Sizeof(lkIn{})
	}
}

type lkOut struct {
	Lk fileLock
}

type accessIn struct {
	Mask uint32
	_    uint32
}

type initIn struct {
	Major        uint32
	Minor        uint32
	MaxReadahead uint32
	Flags        uint32
}

const initInSize = int(unsafe.Sizeof(initIn{}))

type initOut struct {
	Major        uint32
	Minor        uint32
	MaxReadahead uint32
	Flags        uint32
	Unused       uint32
	MaxWrite     uint32
}

type interruptIn struct {
	Unique uint64
}

type bmapIn struct {
	Block     uint64
	BlockSize uint32
	_         uint32
}

type bmapOut struct {
	Block uint64
}

type inHeader struct {
	Len    uint32
	Opcode uint32
	Unique uint64
	Nodeid uint64
	Uid    uint32
	Gid    uint32
	Pid    uint32
	_      uint32
}

const inHeaderSize = int(unsafe.Sizeof(inHeader{}))

type outHeader struct {
	Len    uint32
	Error  int32
	Unique uint64
}

type dirent struct {
	Ino     uint64
	Off     uint64
	Namelen uint32
	Type    uint32
	Name    [0]byte
}

const direntSize = 8 + 8 + 4 + 4

const (
	notifyCodePoll       int32 = 1
	notifyCodeInvalInode int32 = 2
	notifyCodeInvalEntry int32 = 3
)

type notifyInvalInodeOut struct {
	Ino uint64
	Off int64
	Len int64
}

type notifyInvalEntryOut struct {
	Parent  uint64
	Namelen uint32
	_       uint32
}
