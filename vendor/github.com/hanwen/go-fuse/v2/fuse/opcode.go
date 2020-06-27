// Copyright 2016 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fuse

import (
	"bytes"
	"fmt"
	"log"
	"reflect"
	"runtime"
	"time"
	"unsafe"
)

const (
	_OP_LOOKUP          = uint32(1)
	_OP_FORGET          = uint32(2)
	_OP_GETATTR         = uint32(3)
	_OP_SETATTR         = uint32(4)
	_OP_READLINK        = uint32(5)
	_OP_SYMLINK         = uint32(6)
	_OP_MKNOD           = uint32(8)
	_OP_MKDIR           = uint32(9)
	_OP_UNLINK          = uint32(10)
	_OP_RMDIR           = uint32(11)
	_OP_RENAME          = uint32(12)
	_OP_LINK            = uint32(13)
	_OP_OPEN            = uint32(14)
	_OP_READ            = uint32(15)
	_OP_WRITE           = uint32(16)
	_OP_STATFS          = uint32(17)
	_OP_RELEASE         = uint32(18)
	_OP_FSYNC           = uint32(20)
	_OP_SETXATTR        = uint32(21)
	_OP_GETXATTR        = uint32(22)
	_OP_LISTXATTR       = uint32(23)
	_OP_REMOVEXATTR     = uint32(24)
	_OP_FLUSH           = uint32(25)
	_OP_INIT            = uint32(26)
	_OP_OPENDIR         = uint32(27)
	_OP_READDIR         = uint32(28)
	_OP_RELEASEDIR      = uint32(29)
	_OP_FSYNCDIR        = uint32(30)
	_OP_GETLK           = uint32(31)
	_OP_SETLK           = uint32(32)
	_OP_SETLKW          = uint32(33)
	_OP_ACCESS          = uint32(34)
	_OP_CREATE          = uint32(35)
	_OP_INTERRUPT       = uint32(36)
	_OP_BMAP            = uint32(37)
	_OP_DESTROY         = uint32(38)
	_OP_IOCTL           = uint32(39)
	_OP_POLL            = uint32(40)
	_OP_NOTIFY_REPLY    = uint32(41)
	_OP_BATCH_FORGET    = uint32(42)
	_OP_FALLOCATE       = uint32(43) // protocol version 19.
	_OP_READDIRPLUS     = uint32(44) // protocol version 21.
	_OP_RENAME2         = uint32(45) // protocol version 23.
	_OP_LSEEK           = uint32(46) // protocol version 24
	_OP_COPY_FILE_RANGE = uint32(47) // protocol version 28.

	// The following entries don't have to be compatible across Go-FUSE versions.
	_OP_NOTIFY_INVAL_ENTRY    = uint32(100)
	_OP_NOTIFY_INVAL_INODE    = uint32(101)
	_OP_NOTIFY_STORE_CACHE    = uint32(102)
	_OP_NOTIFY_RETRIEVE_CACHE = uint32(103)
	_OP_NOTIFY_DELETE         = uint32(104) // protocol version 18

	_OPCODE_COUNT = uint32(105)
)

////////////////////////////////////////////////////////////////

func doInit(server *Server, req *request) {
	input := (*InitIn)(req.inData)
	if input.Major != _FUSE_KERNEL_VERSION {
		log.Printf("Major versions does not match. Given %d, want %d\n", input.Major, _FUSE_KERNEL_VERSION)
		req.status = EIO
		return
	}
	if input.Minor < _MINIMUM_MINOR_VERSION {
		log.Printf("Minor version is less than we support. Given %d, want at least %d\n", input.Minor, _MINIMUM_MINOR_VERSION)
		req.status = EIO
		return
	}

	server.reqMu.Lock()
	server.kernelSettings = *input
	server.kernelSettings.Flags = input.Flags & (CAP_ASYNC_READ | CAP_BIG_WRITES | CAP_FILE_OPS |
		CAP_READDIRPLUS | CAP_NO_OPEN_SUPPORT | CAP_PARALLEL_DIROPS)

	if server.opts.EnableLocks {
		server.kernelSettings.Flags |= CAP_FLOCK_LOCKS | CAP_POSIX_LOCKS
	}

	dataCacheMode := input.Flags & CAP_AUTO_INVAL_DATA
	if server.opts.ExplicitDataCacheControl {
		// we don't want CAP_AUTO_INVAL_DATA even if we cannot go into fully explicit mode
		dataCacheMode = 0

		explicit := input.Flags & CAP_EXPLICIT_INVAL_DATA
		if explicit != 0 {
			dataCacheMode = explicit
		}
	}
	server.kernelSettings.Flags |= dataCacheMode

	if input.Minor >= 13 {
		server.setSplice()
	}
	server.reqMu.Unlock()

	out := (*InitOut)(req.outData())
	*out = InitOut{
		Major:               _FUSE_KERNEL_VERSION,
		Minor:               _OUR_MINOR_VERSION,
		MaxReadAhead:        input.MaxReadAhead,
		Flags:               server.kernelSettings.Flags,
		MaxWrite:            uint32(server.opts.MaxWrite),
		CongestionThreshold: uint16(server.opts.MaxBackground * 3 / 4),
		MaxBackground:       uint16(server.opts.MaxBackground),
	}

	if server.opts.MaxReadAhead != 0 && uint32(server.opts.MaxReadAhead) < out.MaxReadAhead {
		out.MaxReadAhead = uint32(server.opts.MaxReadAhead)
	}
	if out.Minor > input.Minor {
		out.Minor = input.Minor
	}

	if out.Minor <= 22 {
		tweaked := *req.handler

		// v8-v22 don't have TimeGran and further fields.
		tweaked.OutputSize = 24
		req.handler = &tweaked
	}

	req.status = OK
}

func doOpen(server *Server, req *request) {
	out := (*OpenOut)(req.outData())
	status := server.fileSystem.Open(req.cancel, (*OpenIn)(req.inData), out)
	req.status = status
	if status != OK {
		return
	}
}

func doCreate(server *Server, req *request) {
	out := (*CreateOut)(req.outData())
	status := server.fileSystem.Create(req.cancel, (*CreateIn)(req.inData), req.filenames[0], out)
	req.status = status
}

func doReadDir(server *Server, req *request) {
	in := (*ReadIn)(req.inData)
	buf := server.allocOut(req, in.Size)
	out := NewDirEntryList(buf, uint64(in.Offset))

	code := server.fileSystem.ReadDir(req.cancel, in, out)
	req.flatData = out.bytes()
	req.status = code
}

func doReadDirPlus(server *Server, req *request) {
	in := (*ReadIn)(req.inData)
	buf := server.allocOut(req, in.Size)
	out := NewDirEntryList(buf, uint64(in.Offset))

	code := server.fileSystem.ReadDirPlus(req.cancel, in, out)
	req.flatData = out.bytes()
	req.status = code
}

func doOpenDir(server *Server, req *request) {
	out := (*OpenOut)(req.outData())
	status := server.fileSystem.OpenDir(req.cancel, (*OpenIn)(req.inData), out)
	req.status = status
}

func doSetattr(server *Server, req *request) {
	out := (*AttrOut)(req.outData())
	req.status = server.fileSystem.SetAttr(req.cancel, (*SetAttrIn)(req.inData), out)
}

func doWrite(server *Server, req *request) {
	n, status := server.fileSystem.Write(req.cancel, (*WriteIn)(req.inData), req.arg)
	o := (*WriteOut)(req.outData())
	o.Size = n
	req.status = status
}

func doNotifyReply(server *Server, req *request) {
	reply := (*NotifyRetrieveIn)(req.inData)
	server.retrieveMu.Lock()
	reading := server.retrieveTab[reply.Unique]
	delete(server.retrieveTab, reply.Unique)
	server.retrieveMu.Unlock()

	badf := func(format string, argv ...interface{}) {
		log.Printf("notify reply: "+format, argv...)
	}

	if reading == nil {
		badf("unexpected unique - ignoring")
		return
	}

	reading.n = 0
	reading.st = EIO
	defer close(reading.ready)

	if reading.nodeid != reply.NodeId {
		badf("inode mismatch: expected %s, got %s", reading.nodeid, reply.NodeId)
		return
	}

	if reading.offset != reply.Offset {
		badf("offset mismatch: expected @%d, got @%d", reading.offset, reply.Offset)
		return
	}

	if len(reading.dest) < len(req.arg) {
		badf("too much data: requested %db, got %db (will use only %db)", len(reading.dest), len(req.arg), len(reading.dest))
	}

	reading.n = copy(reading.dest, req.arg)
	reading.st = OK
}

const _SECURITY_CAPABILITY = "security.capability"
const _SECURITY_ACL = "system.posix_acl_access"
const _SECURITY_ACL_DEFAULT = "system.posix_acl_default"

func doGetXAttr(server *Server, req *request) {
	if server.opts.DisableXAttrs {
		req.status = ENOSYS
		return
	}

	if server.opts.IgnoreSecurityLabels && req.inHeader.Opcode == _OP_GETXATTR {
		fn := req.filenames[0]
		if fn == _SECURITY_CAPABILITY || fn == _SECURITY_ACL_DEFAULT ||
			fn == _SECURITY_ACL {
			req.status = ENOATTR
			return
		}
	}

	input := (*GetXAttrIn)(req.inData)

	req.flatData = server.allocOut(req, input.Size)
	out := (*GetXAttrOut)(req.outData())

	var n uint32
	switch req.inHeader.Opcode {
	case _OP_GETXATTR:
		n, req.status = server.fileSystem.GetXAttr(req.cancel, req.inHeader, req.filenames[0], req.flatData)
	case _OP_LISTXATTR:
		n, req.status = server.fileSystem.ListXAttr(req.cancel, req.inHeader, req.flatData)
	default:
		req.status = ENOSYS
	}

	if input.Size == 0 && req.status == ERANGE {
		// For input.size==0, returning ERANGE is an error.
		req.status = OK
		out.Size = n
	} else if req.status.Ok() {
		// ListXAttr called with an empty buffer returns the current size of
		// the list but does not touch the buffer (see man 2 listxattr).
		if len(req.flatData) > 0 {
			req.flatData = req.flatData[:n]
		}
		out.Size = n
	} else {
		req.flatData = req.flatData[:0]
	}
}

func doGetAttr(server *Server, req *request) {
	out := (*AttrOut)(req.outData())
	s := server.fileSystem.GetAttr(req.cancel, (*GetAttrIn)(req.inData), out)
	req.status = s
}

// doForget - forget one NodeId
func doForget(server *Server, req *request) {
	if !server.opts.RememberInodes {
		server.fileSystem.Forget(req.inHeader.NodeId, (*ForgetIn)(req.inData).Nlookup)
	}
}

// doBatchForget - forget a list of NodeIds
func doBatchForget(server *Server, req *request) {
	in := (*_BatchForgetIn)(req.inData)
	wantBytes := uintptr(in.Count) * unsafe.Sizeof(_ForgetOne{})
	if uintptr(len(req.arg)) < wantBytes {
		// We have no return value to complain, so log an error.
		log.Printf("Too few bytes for batch forget. Got %d bytes, want %d (%d entries)",
			len(req.arg), wantBytes, in.Count)
	}

	h := &reflect.SliceHeader{
		Data: uintptr(unsafe.Pointer(&req.arg[0])),
		Len:  int(in.Count),
		Cap:  int(in.Count),
	}

	forgets := *(*[]_ForgetOne)(unsafe.Pointer(h))
	for i, f := range forgets {
		if server.opts.Debug {
			log.Printf("doBatchForget: rx %d %d/%d: FORGET i%d {Nlookup=%d}",
				req.inHeader.Unique, i+1, len(forgets), f.NodeId, f.Nlookup)
		}
		if f.NodeId == pollHackInode {
			continue
		}
		server.fileSystem.Forget(f.NodeId, f.Nlookup)
	}
}

func doReadlink(server *Server, req *request) {
	req.flatData, req.status = server.fileSystem.Readlink(req.cancel, req.inHeader)
}

func doLookup(server *Server, req *request) {
	out := (*EntryOut)(req.outData())
	s := server.fileSystem.Lookup(req.cancel, req.inHeader, req.filenames[0], out)
	req.status = s
}

func doMknod(server *Server, req *request) {
	out := (*EntryOut)(req.outData())

	req.status = server.fileSystem.Mknod(req.cancel, (*MknodIn)(req.inData), req.filenames[0], out)
}

func doMkdir(server *Server, req *request) {
	out := (*EntryOut)(req.outData())
	req.status = server.fileSystem.Mkdir(req.cancel, (*MkdirIn)(req.inData), req.filenames[0], out)
}

func doUnlink(server *Server, req *request) {
	req.status = server.fileSystem.Unlink(req.cancel, req.inHeader, req.filenames[0])
}

func doRmdir(server *Server, req *request) {
	req.status = server.fileSystem.Rmdir(req.cancel, req.inHeader, req.filenames[0])
}

func doLink(server *Server, req *request) {
	out := (*EntryOut)(req.outData())
	req.status = server.fileSystem.Link(req.cancel, (*LinkIn)(req.inData), req.filenames[0], out)
}

func doRead(server *Server, req *request) {
	in := (*ReadIn)(req.inData)
	buf := server.allocOut(req, in.Size)

	req.readResult, req.status = server.fileSystem.Read(req.cancel, in, buf)
	if fd, ok := req.readResult.(*readResultFd); ok {
		req.fdData = fd
		req.flatData = nil
	} else if req.readResult != nil && req.status.Ok() {
		req.flatData, req.status = req.readResult.Bytes(buf)
	}
}

func doFlush(server *Server, req *request) {
	req.status = server.fileSystem.Flush(req.cancel, (*FlushIn)(req.inData))
}

func doRelease(server *Server, req *request) {
	server.fileSystem.Release(req.cancel, (*ReleaseIn)(req.inData))
}

func doFsync(server *Server, req *request) {
	req.status = server.fileSystem.Fsync(req.cancel, (*FsyncIn)(req.inData))
}

func doReleaseDir(server *Server, req *request) {
	server.fileSystem.ReleaseDir((*ReleaseIn)(req.inData))
}

func doFsyncDir(server *Server, req *request) {
	req.status = server.fileSystem.FsyncDir(req.cancel, (*FsyncIn)(req.inData))
}

func doSetXAttr(server *Server, req *request) {
	splits := bytes.SplitN(req.arg, []byte{0}, 2)
	req.status = server.fileSystem.SetXAttr(req.cancel, (*SetXAttrIn)(req.inData), string(splits[0]), splits[1])
}

func doRemoveXAttr(server *Server, req *request) {
	req.status = server.fileSystem.RemoveXAttr(req.cancel, req.inHeader, req.filenames[0])
}

func doAccess(server *Server, req *request) {
	req.status = server.fileSystem.Access(req.cancel, (*AccessIn)(req.inData))
}

func doSymlink(server *Server, req *request) {
	out := (*EntryOut)(req.outData())
	req.status = server.fileSystem.Symlink(req.cancel, req.inHeader, req.filenames[1], req.filenames[0], out)
}

func doRename(server *Server, req *request) {
	in1 := (*Rename1In)(req.inData)
	in := RenameIn{
		InHeader: in1.InHeader,
		Newdir:   in1.Newdir,
	}
	req.status = server.fileSystem.Rename(req.cancel, &in, req.filenames[0], req.filenames[1])
}

func doRename2(server *Server, req *request) {
	req.status = server.fileSystem.Rename(req.cancel, (*RenameIn)(req.inData), req.filenames[0], req.filenames[1])
}

func doStatFs(server *Server, req *request) {
	out := (*StatfsOut)(req.outData())
	req.status = server.fileSystem.StatFs(req.cancel, req.inHeader, out)
	if req.status == ENOSYS && runtime.GOOS == "darwin" {
		// OSX FUSE requires Statfs to be implemented for the
		// mount to succeed.
		*out = StatfsOut{}
		req.status = OK
	}
}

func doIoctl(server *Server, req *request) {
	req.status = ENOSYS
}

func doDestroy(server *Server, req *request) {
	req.status = OK
}

func doFallocate(server *Server, req *request) {
	req.status = server.fileSystem.Fallocate(req.cancel, (*FallocateIn)(req.inData))
}

func doGetLk(server *Server, req *request) {
	req.status = server.fileSystem.GetLk(req.cancel, (*LkIn)(req.inData), (*LkOut)(req.outData()))
}

func doSetLk(server *Server, req *request) {
	req.status = server.fileSystem.SetLk(req.cancel, (*LkIn)(req.inData))
}

func doSetLkw(server *Server, req *request) {
	req.status = server.fileSystem.SetLkw(req.cancel, (*LkIn)(req.inData))
}

func doLseek(server *Server, req *request) {
	in := (*LseekIn)(req.inData)
	out := (*LseekOut)(req.outData())
	req.status = server.fileSystem.Lseek(req.cancel, in, out)
}

func doCopyFileRange(server *Server, req *request) {
	in := (*CopyFileRangeIn)(req.inData)
	out := (*WriteOut)(req.outData())

	out.Size, req.status = server.fileSystem.CopyFileRange(req.cancel, in)
}

func doInterrupt(server *Server, req *request) {
	input := (*InterruptIn)(req.inData)
	server.reqMu.Lock()
	defer server.reqMu.Unlock()

	// This is slow, but this operation is rare.
	for _, inflight := range server.reqInflight {
		if input.Unique == inflight.inHeader.Unique && !inflight.interrupted {
			close(inflight.cancel)
			inflight.interrupted = true
			req.status = OK
			return
		}
	}

	// not found; wait for a bit
	time.Sleep(10 * time.Microsecond)
	req.status = EAGAIN
}

////////////////////////////////////////////////////////////////

type operationFunc func(*Server, *request)
type castPointerFunc func(unsafe.Pointer) interface{}

type operationHandler struct {
	Name        string
	Func        operationFunc
	InputSize   uintptr
	OutputSize  uintptr
	DecodeIn    castPointerFunc
	DecodeOut   castPointerFunc
	FileNames   int
	FileNameOut bool
}

var operationHandlers []*operationHandler

func operationName(op uint32) string {
	h := getHandler(op)
	if h == nil {
		return "unknown"
	}
	return h.Name
}

func getHandler(o uint32) *operationHandler {
	if o >= _OPCODE_COUNT {
		return nil
	}
	return operationHandlers[o]
}

var maxInputSize uintptr

func init() {
	operationHandlers = make([]*operationHandler, _OPCODE_COUNT)
	for i := range operationHandlers {
		operationHandlers[i] = &operationHandler{Name: fmt.Sprintf("OPCODE-%d", i)}
	}

	fileOps := []uint32{_OP_READLINK, _OP_NOTIFY_INVAL_ENTRY, _OP_NOTIFY_DELETE}
	for _, op := range fileOps {
		operationHandlers[op].FileNameOut = true
	}

	maxInputSize = 0
	for op, sz := range map[uint32]uintptr{
		_OP_FORGET:          unsafe.Sizeof(ForgetIn{}),
		_OP_BATCH_FORGET:    unsafe.Sizeof(_BatchForgetIn{}),
		_OP_GETATTR:         unsafe.Sizeof(GetAttrIn{}),
		_OP_SETATTR:         unsafe.Sizeof(SetAttrIn{}),
		_OP_MKNOD:           unsafe.Sizeof(MknodIn{}),
		_OP_MKDIR:           unsafe.Sizeof(MkdirIn{}),
		_OP_RENAME:          unsafe.Sizeof(Rename1In{}),
		_OP_LINK:            unsafe.Sizeof(LinkIn{}),
		_OP_OPEN:            unsafe.Sizeof(OpenIn{}),
		_OP_READ:            unsafe.Sizeof(ReadIn{}),
		_OP_WRITE:           unsafe.Sizeof(WriteIn{}),
		_OP_RELEASE:         unsafe.Sizeof(ReleaseIn{}),
		_OP_FSYNC:           unsafe.Sizeof(FsyncIn{}),
		_OP_SETXATTR:        unsafe.Sizeof(SetXAttrIn{}),
		_OP_GETXATTR:        unsafe.Sizeof(GetXAttrIn{}),
		_OP_LISTXATTR:       unsafe.Sizeof(GetXAttrIn{}),
		_OP_FLUSH:           unsafe.Sizeof(FlushIn{}),
		_OP_INIT:            unsafe.Sizeof(InitIn{}),
		_OP_OPENDIR:         unsafe.Sizeof(OpenIn{}),
		_OP_READDIR:         unsafe.Sizeof(ReadIn{}),
		_OP_RELEASEDIR:      unsafe.Sizeof(ReleaseIn{}),
		_OP_FSYNCDIR:        unsafe.Sizeof(FsyncIn{}),
		_OP_GETLK:           unsafe.Sizeof(LkIn{}),
		_OP_SETLK:           unsafe.Sizeof(LkIn{}),
		_OP_SETLKW:          unsafe.Sizeof(LkIn{}),
		_OP_ACCESS:          unsafe.Sizeof(AccessIn{}),
		_OP_CREATE:          unsafe.Sizeof(CreateIn{}),
		_OP_INTERRUPT:       unsafe.Sizeof(InterruptIn{}),
		_OP_BMAP:            unsafe.Sizeof(_BmapIn{}),
		_OP_IOCTL:           unsafe.Sizeof(_IoctlIn{}),
		_OP_POLL:            unsafe.Sizeof(_PollIn{}),
		_OP_NOTIFY_REPLY:    unsafe.Sizeof(NotifyRetrieveIn{}),
		_OP_FALLOCATE:       unsafe.Sizeof(FallocateIn{}),
		_OP_READDIRPLUS:     unsafe.Sizeof(ReadIn{}),
		_OP_RENAME2:         unsafe.Sizeof(RenameIn{}),
		_OP_LSEEK:           unsafe.Sizeof(LseekIn{}),
		_OP_COPY_FILE_RANGE: unsafe.Sizeof(CopyFileRangeIn{}),
	} {
		operationHandlers[op].InputSize = sz
		if sz > maxInputSize {
			maxInputSize = sz
		}
	}

	for op, sz := range map[uint32]uintptr{
		_OP_LOOKUP:                unsafe.Sizeof(EntryOut{}),
		_OP_GETATTR:               unsafe.Sizeof(AttrOut{}),
		_OP_SETATTR:               unsafe.Sizeof(AttrOut{}),
		_OP_SYMLINK:               unsafe.Sizeof(EntryOut{}),
		_OP_MKNOD:                 unsafe.Sizeof(EntryOut{}),
		_OP_MKDIR:                 unsafe.Sizeof(EntryOut{}),
		_OP_LINK:                  unsafe.Sizeof(EntryOut{}),
		_OP_OPEN:                  unsafe.Sizeof(OpenOut{}),
		_OP_WRITE:                 unsafe.Sizeof(WriteOut{}),
		_OP_STATFS:                unsafe.Sizeof(StatfsOut{}),
		_OP_GETXATTR:              unsafe.Sizeof(GetXAttrOut{}),
		_OP_LISTXATTR:             unsafe.Sizeof(GetXAttrOut{}),
		_OP_INIT:                  unsafe.Sizeof(InitOut{}),
		_OP_OPENDIR:               unsafe.Sizeof(OpenOut{}),
		_OP_GETLK:                 unsafe.Sizeof(LkOut{}),
		_OP_CREATE:                unsafe.Sizeof(CreateOut{}),
		_OP_BMAP:                  unsafe.Sizeof(_BmapOut{}),
		_OP_IOCTL:                 unsafe.Sizeof(_IoctlOut{}),
		_OP_POLL:                  unsafe.Sizeof(_PollOut{}),
		_OP_NOTIFY_INVAL_ENTRY:    unsafe.Sizeof(NotifyInvalEntryOut{}),
		_OP_NOTIFY_INVAL_INODE:    unsafe.Sizeof(NotifyInvalInodeOut{}),
		_OP_NOTIFY_STORE_CACHE:    unsafe.Sizeof(NotifyStoreOut{}),
		_OP_NOTIFY_RETRIEVE_CACHE: unsafe.Sizeof(NotifyRetrieveOut{}),
		_OP_NOTIFY_DELETE:         unsafe.Sizeof(NotifyInvalDeleteOut{}),
		_OP_LSEEK:                 unsafe.Sizeof(LseekOut{}),
		_OP_COPY_FILE_RANGE:       unsafe.Sizeof(WriteOut{}),
	} {
		operationHandlers[op].OutputSize = sz
	}

	for op, v := range map[uint32]string{
		_OP_LOOKUP:                "LOOKUP",
		_OP_FORGET:                "FORGET",
		_OP_BATCH_FORGET:          "BATCH_FORGET",
		_OP_GETATTR:               "GETATTR",
		_OP_SETATTR:               "SETATTR",
		_OP_READLINK:              "READLINK",
		_OP_SYMLINK:               "SYMLINK",
		_OP_MKNOD:                 "MKNOD",
		_OP_MKDIR:                 "MKDIR",
		_OP_UNLINK:                "UNLINK",
		_OP_RMDIR:                 "RMDIR",
		_OP_RENAME:                "RENAME",
		_OP_LINK:                  "LINK",
		_OP_OPEN:                  "OPEN",
		_OP_READ:                  "READ",
		_OP_WRITE:                 "WRITE",
		_OP_STATFS:                "STATFS",
		_OP_RELEASE:               "RELEASE",
		_OP_FSYNC:                 "FSYNC",
		_OP_SETXATTR:              "SETXATTR",
		_OP_GETXATTR:              "GETXATTR",
		_OP_LISTXATTR:             "LISTXATTR",
		_OP_REMOVEXATTR:           "REMOVEXATTR",
		_OP_FLUSH:                 "FLUSH",
		_OP_INIT:                  "INIT",
		_OP_OPENDIR:               "OPENDIR",
		_OP_READDIR:               "READDIR",
		_OP_RELEASEDIR:            "RELEASEDIR",
		_OP_FSYNCDIR:              "FSYNCDIR",
		_OP_GETLK:                 "GETLK",
		_OP_SETLK:                 "SETLK",
		_OP_SETLKW:                "SETLKW",
		_OP_ACCESS:                "ACCESS",
		_OP_CREATE:                "CREATE",
		_OP_INTERRUPT:             "INTERRUPT",
		_OP_BMAP:                  "BMAP",
		_OP_DESTROY:               "DESTROY",
		_OP_IOCTL:                 "IOCTL",
		_OP_POLL:                  "POLL",
		_OP_NOTIFY_REPLY:          "NOTIFY_REPLY",
		_OP_NOTIFY_INVAL_ENTRY:    "NOTIFY_INVAL_ENTRY",
		_OP_NOTIFY_INVAL_INODE:    "NOTIFY_INVAL_INODE",
		_OP_NOTIFY_STORE_CACHE:    "NOTIFY_STORE",
		_OP_NOTIFY_RETRIEVE_CACHE: "NOTIFY_RETRIEVE",
		_OP_NOTIFY_DELETE:         "NOTIFY_DELETE",
		_OP_FALLOCATE:             "FALLOCATE",
		_OP_READDIRPLUS:           "READDIRPLUS",
		_OP_RENAME2:               "RENAME2",
		_OP_LSEEK:                 "LSEEK",
		_OP_COPY_FILE_RANGE:       "COPY_FILE_RANGE",
	} {
		operationHandlers[op].Name = v
	}

	for op, v := range map[uint32]operationFunc{
		_OP_OPEN:            doOpen,
		_OP_READDIR:         doReadDir,
		_OP_WRITE:           doWrite,
		_OP_OPENDIR:         doOpenDir,
		_OP_CREATE:          doCreate,
		_OP_SETATTR:         doSetattr,
		_OP_GETXATTR:        doGetXAttr,
		_OP_LISTXATTR:       doGetXAttr,
		_OP_GETATTR:         doGetAttr,
		_OP_FORGET:          doForget,
		_OP_BATCH_FORGET:    doBatchForget,
		_OP_READLINK:        doReadlink,
		_OP_INIT:            doInit,
		_OP_LOOKUP:          doLookup,
		_OP_MKNOD:           doMknod,
		_OP_MKDIR:           doMkdir,
		_OP_UNLINK:          doUnlink,
		_OP_RMDIR:           doRmdir,
		_OP_LINK:            doLink,
		_OP_READ:            doRead,
		_OP_FLUSH:           doFlush,
		_OP_RELEASE:         doRelease,
		_OP_FSYNC:           doFsync,
		_OP_RELEASEDIR:      doReleaseDir,
		_OP_FSYNCDIR:        doFsyncDir,
		_OP_SETXATTR:        doSetXAttr,
		_OP_REMOVEXATTR:     doRemoveXAttr,
		_OP_GETLK:           doGetLk,
		_OP_SETLK:           doSetLk,
		_OP_SETLKW:          doSetLkw,
		_OP_ACCESS:          doAccess,
		_OP_SYMLINK:         doSymlink,
		_OP_RENAME:          doRename,
		_OP_STATFS:          doStatFs,
		_OP_IOCTL:           doIoctl,
		_OP_DESTROY:         doDestroy,
		_OP_NOTIFY_REPLY:    doNotifyReply,
		_OP_FALLOCATE:       doFallocate,
		_OP_READDIRPLUS:     doReadDirPlus,
		_OP_RENAME2:         doRename2,
		_OP_INTERRUPT:       doInterrupt,
		_OP_COPY_FILE_RANGE: doCopyFileRange,
		_OP_LSEEK:           doLseek,
	} {
		operationHandlers[op].Func = v
	}

	// Outputs.
	for op, f := range map[uint32]castPointerFunc{
		_OP_LOOKUP:                func(ptr unsafe.Pointer) interface{} { return (*EntryOut)(ptr) },
		_OP_OPEN:                  func(ptr unsafe.Pointer) interface{} { return (*OpenOut)(ptr) },
		_OP_OPENDIR:               func(ptr unsafe.Pointer) interface{} { return (*OpenOut)(ptr) },
		_OP_GETATTR:               func(ptr unsafe.Pointer) interface{} { return (*AttrOut)(ptr) },
		_OP_CREATE:                func(ptr unsafe.Pointer) interface{} { return (*CreateOut)(ptr) },
		_OP_LINK:                  func(ptr unsafe.Pointer) interface{} { return (*EntryOut)(ptr) },
		_OP_SETATTR:               func(ptr unsafe.Pointer) interface{} { return (*AttrOut)(ptr) },
		_OP_INIT:                  func(ptr unsafe.Pointer) interface{} { return (*InitOut)(ptr) },
		_OP_MKDIR:                 func(ptr unsafe.Pointer) interface{} { return (*EntryOut)(ptr) },
		_OP_NOTIFY_INVAL_ENTRY:    func(ptr unsafe.Pointer) interface{} { return (*NotifyInvalEntryOut)(ptr) },
		_OP_NOTIFY_INVAL_INODE:    func(ptr unsafe.Pointer) interface{} { return (*NotifyInvalInodeOut)(ptr) },
		_OP_NOTIFY_STORE_CACHE:    func(ptr unsafe.Pointer) interface{} { return (*NotifyStoreOut)(ptr) },
		_OP_NOTIFY_RETRIEVE_CACHE: func(ptr unsafe.Pointer) interface{} { return (*NotifyRetrieveOut)(ptr) },
		_OP_NOTIFY_DELETE:         func(ptr unsafe.Pointer) interface{} { return (*NotifyInvalDeleteOut)(ptr) },
		_OP_STATFS:                func(ptr unsafe.Pointer) interface{} { return (*StatfsOut)(ptr) },
		_OP_SYMLINK:               func(ptr unsafe.Pointer) interface{} { return (*EntryOut)(ptr) },
		_OP_GETLK:                 func(ptr unsafe.Pointer) interface{} { return (*LkOut)(ptr) },
		_OP_LSEEK:                 func(ptr unsafe.Pointer) interface{} { return (*LseekOut)(ptr) },
		_OP_COPY_FILE_RANGE:       func(ptr unsafe.Pointer) interface{} { return (*WriteOut)(ptr) },
	} {
		operationHandlers[op].DecodeOut = f
	}

	// Inputs.
	for op, f := range map[uint32]castPointerFunc{
		_OP_FLUSH:           func(ptr unsafe.Pointer) interface{} { return (*FlushIn)(ptr) },
		_OP_GETATTR:         func(ptr unsafe.Pointer) interface{} { return (*GetAttrIn)(ptr) },
		_OP_SETXATTR:        func(ptr unsafe.Pointer) interface{} { return (*SetXAttrIn)(ptr) },
		_OP_GETXATTR:        func(ptr unsafe.Pointer) interface{} { return (*GetXAttrIn)(ptr) },
		_OP_LISTXATTR:       func(ptr unsafe.Pointer) interface{} { return (*GetXAttrIn)(ptr) },
		_OP_SETATTR:         func(ptr unsafe.Pointer) interface{} { return (*SetAttrIn)(ptr) },
		_OP_INIT:            func(ptr unsafe.Pointer) interface{} { return (*InitIn)(ptr) },
		_OP_IOCTL:           func(ptr unsafe.Pointer) interface{} { return (*_IoctlIn)(ptr) },
		_OP_OPEN:            func(ptr unsafe.Pointer) interface{} { return (*OpenIn)(ptr) },
		_OP_MKNOD:           func(ptr unsafe.Pointer) interface{} { return (*MknodIn)(ptr) },
		_OP_CREATE:          func(ptr unsafe.Pointer) interface{} { return (*CreateIn)(ptr) },
		_OP_READ:            func(ptr unsafe.Pointer) interface{} { return (*ReadIn)(ptr) },
		_OP_WRITE:           func(ptr unsafe.Pointer) interface{} { return (*WriteIn)(ptr) },
		_OP_READDIR:         func(ptr unsafe.Pointer) interface{} { return (*ReadIn)(ptr) },
		_OP_ACCESS:          func(ptr unsafe.Pointer) interface{} { return (*AccessIn)(ptr) },
		_OP_FORGET:          func(ptr unsafe.Pointer) interface{} { return (*ForgetIn)(ptr) },
		_OP_BATCH_FORGET:    func(ptr unsafe.Pointer) interface{} { return (*_BatchForgetIn)(ptr) },
		_OP_LINK:            func(ptr unsafe.Pointer) interface{} { return (*LinkIn)(ptr) },
		_OP_MKDIR:           func(ptr unsafe.Pointer) interface{} { return (*MkdirIn)(ptr) },
		_OP_RELEASE:         func(ptr unsafe.Pointer) interface{} { return (*ReleaseIn)(ptr) },
		_OP_RELEASEDIR:      func(ptr unsafe.Pointer) interface{} { return (*ReleaseIn)(ptr) },
		_OP_FALLOCATE:       func(ptr unsafe.Pointer) interface{} { return (*FallocateIn)(ptr) },
		_OP_NOTIFY_REPLY:    func(ptr unsafe.Pointer) interface{} { return (*NotifyRetrieveIn)(ptr) },
		_OP_READDIRPLUS:     func(ptr unsafe.Pointer) interface{} { return (*ReadIn)(ptr) },
		_OP_RENAME:          func(ptr unsafe.Pointer) interface{} { return (*Rename1In)(ptr) },
		_OP_GETLK:           func(ptr unsafe.Pointer) interface{} { return (*LkIn)(ptr) },
		_OP_SETLK:           func(ptr unsafe.Pointer) interface{} { return (*LkIn)(ptr) },
		_OP_SETLKW:          func(ptr unsafe.Pointer) interface{} { return (*LkIn)(ptr) },
		_OP_RENAME2:         func(ptr unsafe.Pointer) interface{} { return (*RenameIn)(ptr) },
		_OP_INTERRUPT:       func(ptr unsafe.Pointer) interface{} { return (*InterruptIn)(ptr) },
		_OP_LSEEK:           func(ptr unsafe.Pointer) interface{} { return (*LseekIn)(ptr) },
		_OP_COPY_FILE_RANGE: func(ptr unsafe.Pointer) interface{} { return (*CopyFileRangeIn)(ptr) },
	} {
		operationHandlers[op].DecodeIn = f
	}

	// File name args.
	for op, count := range map[uint32]int{
		_OP_CREATE:      1,
		_OP_SETXATTR:    1,
		_OP_GETXATTR:    1,
		_OP_LINK:        1,
		_OP_LOOKUP:      1,
		_OP_MKDIR:       1,
		_OP_MKNOD:       1,
		_OP_REMOVEXATTR: 1,
		_OP_RENAME:      2,
		_OP_RENAME2:     2,
		_OP_RMDIR:       1,
		_OP_SYMLINK:     2,
		_OP_UNLINK:      1,
	} {
		operationHandlers[op].FileNames = count
	}

	var r request
	sizeOfOutHeader := unsafe.Sizeof(OutHeader{})
	for code, h := range operationHandlers {
		if h.OutputSize+sizeOfOutHeader > unsafe.Sizeof(r.outBuf) {
			log.Panicf("request output buffer too small: code %v, sz %d + %d %v", code, h.OutputSize, sizeOfOutHeader, h)
		}
	}
}
