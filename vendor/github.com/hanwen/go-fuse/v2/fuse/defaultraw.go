// Copyright 2016 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fuse

import (
	"os"
)

// NewDefaultRawFileSystem returns ENOSYS (not implemented) for all
// operations.
func NewDefaultRawFileSystem() RawFileSystem {
	return (*defaultRawFileSystem)(nil)
}

type defaultRawFileSystem struct{}

func (fs *defaultRawFileSystem) Init(*Server) {
}

func (fs *defaultRawFileSystem) String() string {
	return os.Args[0]
}

func (fs *defaultRawFileSystem) SetDebug(dbg bool) {
}

func (fs *defaultRawFileSystem) StatFs(cancel <-chan struct{}, header *InHeader, out *StatfsOut) Status {
	return ENOSYS
}

func (fs *defaultRawFileSystem) Lookup(cancel <-chan struct{}, header *InHeader, name string, out *EntryOut) (code Status) {
	return ENOSYS
}

func (fs *defaultRawFileSystem) Forget(nodeID, nlookup uint64) {
}

func (fs *defaultRawFileSystem) GetAttr(cancel <-chan struct{}, input *GetAttrIn, out *AttrOut) (code Status) {
	return ENOSYS
}

func (fs *defaultRawFileSystem) Open(cancel <-chan struct{}, input *OpenIn, out *OpenOut) (status Status) {
	return OK
}

func (fs *defaultRawFileSystem) SetAttr(cancel <-chan struct{}, input *SetAttrIn, out *AttrOut) (code Status) {
	return ENOSYS
}

func (fs *defaultRawFileSystem) Readlink(cancel <-chan struct{}, header *InHeader) (out []byte, code Status) {
	return nil, ENOSYS
}

func (fs *defaultRawFileSystem) Mknod(cancel <-chan struct{}, input *MknodIn, name string, out *EntryOut) (code Status) {
	return ENOSYS
}

func (fs *defaultRawFileSystem) Mkdir(cancel <-chan struct{}, input *MkdirIn, name string, out *EntryOut) (code Status) {
	return ENOSYS
}

func (fs *defaultRawFileSystem) Unlink(cancel <-chan struct{}, header *InHeader, name string) (code Status) {
	return ENOSYS
}

func (fs *defaultRawFileSystem) Rmdir(cancel <-chan struct{}, header *InHeader, name string) (code Status) {
	return ENOSYS
}

func (fs *defaultRawFileSystem) Symlink(cancel <-chan struct{}, header *InHeader, pointedTo string, linkName string, out *EntryOut) (code Status) {
	return ENOSYS
}

func (fs *defaultRawFileSystem) Rename(cancel <-chan struct{}, input *RenameIn, oldName string, newName string) (code Status) {
	return ENOSYS
}

func (fs *defaultRawFileSystem) Link(cancel <-chan struct{}, input *LinkIn, name string, out *EntryOut) (code Status) {
	return ENOSYS
}

func (fs *defaultRawFileSystem) GetXAttr(cancel <-chan struct{}, header *InHeader, attr string, dest []byte) (size uint32, code Status) {
	return 0, ENOSYS
}

func (fs *defaultRawFileSystem) SetXAttr(cancel <-chan struct{}, input *SetXAttrIn, attr string, data []byte) Status {
	return ENOSYS
}

func (fs *defaultRawFileSystem) ListXAttr(cancel <-chan struct{}, header *InHeader, dest []byte) (n uint32, code Status) {
	return 0, ENOSYS
}

func (fs *defaultRawFileSystem) RemoveXAttr(cancel <-chan struct{}, header *InHeader, attr string) Status {
	return ENOSYS
}

func (fs *defaultRawFileSystem) Access(cancel <-chan struct{}, input *AccessIn) (code Status) {
	return ENOSYS
}

func (fs *defaultRawFileSystem) Create(cancel <-chan struct{}, input *CreateIn, name string, out *CreateOut) (code Status) {
	return ENOSYS
}

func (fs *defaultRawFileSystem) OpenDir(cancel <-chan struct{}, input *OpenIn, out *OpenOut) (status Status) {
	return ENOSYS
}

func (fs *defaultRawFileSystem) Read(cancel <-chan struct{}, input *ReadIn, buf []byte) (ReadResult, Status) {
	return nil, ENOSYS
}

func (fs *defaultRawFileSystem) GetLk(cancel <-chan struct{}, in *LkIn, out *LkOut) (code Status) {
	return ENOSYS
}

func (fs *defaultRawFileSystem) SetLk(cancel <-chan struct{}, in *LkIn) (code Status) {
	return ENOSYS
}

func (fs *defaultRawFileSystem) SetLkw(cancel <-chan struct{}, in *LkIn) (code Status) {
	return ENOSYS
}

func (fs *defaultRawFileSystem) Release(cancel <-chan struct{}, input *ReleaseIn) {
}

func (fs *defaultRawFileSystem) Write(cancel <-chan struct{}, input *WriteIn, data []byte) (written uint32, code Status) {
	return 0, ENOSYS
}

func (fs *defaultRawFileSystem) Flush(cancel <-chan struct{}, input *FlushIn) Status {
	return OK
}

func (fs *defaultRawFileSystem) Fsync(cancel <-chan struct{}, input *FsyncIn) (code Status) {
	return ENOSYS
}

func (fs *defaultRawFileSystem) ReadDir(cancel <-chan struct{}, input *ReadIn, l *DirEntryList) Status {
	return ENOSYS
}

func (fs *defaultRawFileSystem) ReadDirPlus(cancel <-chan struct{}, input *ReadIn, l *DirEntryList) Status {
	return ENOSYS
}

func (fs *defaultRawFileSystem) ReleaseDir(input *ReleaseIn) {
}

func (fs *defaultRawFileSystem) FsyncDir(cancel <-chan struct{}, input *FsyncIn) (code Status) {
	return ENOSYS
}

func (fs *defaultRawFileSystem) Fallocate(cancel <-chan struct{}, in *FallocateIn) (code Status) {
	return ENOSYS
}

func (fs *defaultRawFileSystem) CopyFileRange(cancel <-chan struct{}, input *CopyFileRangeIn) (written uint32, code Status) {
	return 0, ENOSYS
}

func (fs *defaultRawFileSystem) Lseek(cancel <-chan struct{}, in *LseekIn, out *LseekOut) Status {
	return ENOSYS
}
