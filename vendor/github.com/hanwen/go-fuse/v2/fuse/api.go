// Copyright 2016 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package fuse provides APIs to implement filesystems in
// userspace in terms of raw FUSE protocol.
//
// A filesystem is implemented by implementing its server that provides a
// RawFileSystem interface. Typically the server embeds
// NewDefaultRawFileSystem() and implements only subset of filesystem methods:
//
//	type MyFS struct {
//		fuse.RawFileSystem
//		...
//	}
//
//	func NewMyFS() *MyFS {
//		return &MyFS{
//			RawFileSystem: fuse.NewDefaultRawFileSystem(),
//			...
//		}
//	}
//
//	// Mkdir implements "mkdir" request handler.
//	//
//	// For other requests - not explicitly implemented by MyFS - ENOSYS
//	// will be typically returned to client.
//	func (fs *MyFS) Mkdir(...) {
//		...
//	}
//
// Then the filesystem can be mounted and served to a client (typically OS
// kernel) by creating Server:
//
//	fs := NewMyFS() // implements RawFileSystem
//	fssrv, err := fuse.NewServer(fs, mountpoint, &fuse.MountOptions{...})
//	if err != nil {
//		...
//	}
//
// and letting the server do its work:
//
//	// either synchronously - .Serve() blocks until the filesystem is unmounted.
//	fssrv.Serve()
//
//	// or in the background - .Serve() is spawned in another goroutine, but
//	// before interacting with fssrv from current context we have to wait
//	// until the filesystem mounting is complete.
//	go fssrv.Serve()
//	err = fssrv.WaitMount()
//	if err != nil {
//		...
//	}
//
// The server will serve clients by dispatching their requests to the
// filesystem implementation and conveying responses back. For example "mkdir"
// FUSE request dispatches to call
//
//	fs.Mkdir(*MkdirIn, ..., *EntryOut)
//
// "stat" to call
//
//	fs.GetAttr(*GetAttrIn, *AttrOut)
//
// etc. Please refer to RawFileSystem documentation for details.
//
// Typically, each call of the API happens in its own
// goroutine, so take care to make the file system thread-safe.
//
// Be careful when you access the FUSE mount from the same process. An access can
// tie up two OS threads (one on the request side and one on the FUSE server side).
// This can deadlock if there is no free thread to handle the FUSE server side.
// Run your program with GOMAXPROCS=1 to make the problem easier to reproduce,
// see https://github.com/hanwen/go-fuse/issues/261 for an example of that
// problem.
//
// # Higher level interfaces
//
// As said above this packages provides way to implement filesystems in terms of
// raw FUSE protocol.
//
// Package github.com/hanwen/go-fuse/v2/fs provides way to implement
// filesystems in terms of paths and/or inodes.
//
// # Mount styles
//
// The NewServer() handles mounting the filesystem, which
// involves opening `/dev/fuse` and calling the
// `mount(2)` syscall. The latter needs root permissions.
// This is handled in one of three ways:
//
// 1) go-fuse opens `/dev/fuse` and executes the `fusermount`
// setuid-root helper to call `mount(2)` for us. This is the default.
// Does not need root permissions but needs `fusermount` installed.
//
// 2) If `MountOptions.DirectMount` is set, go-fuse calls `mount(2)` itself.
// Needs root permissions, but works without `fusermount`.
//
// 3) If `mountPoint` has the magic `/dev/fd/N` syntax, it means that that a
// privileged parent process:
//
// * Opened /dev/fuse
//
// * Called mount(2) on a real mountpoint directory that we don't know about
//
// * Inherited the fd to /dev/fuse to us
//
// * Informs us about the fd number via /dev/fd/N
//
// This magic syntax originates from libfuse [1] and allows the FUSE server to
// run without any privileges and without needing `fusermount`, as the parent
// process performs all privileged operations.
//
// The "privileged parent" is usually a container manager like Singularity [2],
// but for testing, it can also be  the `mount.fuse3` helper with the
// `drop_privileges,setuid=$USER` flags. Example below for gocryptfs:
//
//	$ sudo mount.fuse3 "/usr/local/bin/gocryptfs#/tmp/cipher" /tmp/mnt -o drop_privileges,setuid=$USER
//
// [1] https://github.com/libfuse/libfuse/commit/64e11073b9347fcf9c6d1eea143763ba9e946f70
//
// [2] https://sylabs.io/guides/3.7/user-guide/bind_paths_and_mounts.html#fuse-mounts
package fuse

// Types for users to implement.

// The result of Read is an array of bytes, but for performance
// reasons, we can also return data as a file-descriptor/offset/size
// tuple.  If the backing store for a file is another filesystem, this
// reduces the amount of copying between the kernel and the FUSE
// server.  The ReadResult interface captures both cases.
type ReadResult interface {
	// Returns the raw bytes for the read, possibly using the
	// passed buffer. The buffer should be larger than the return
	// value from Size.
	Bytes(buf []byte) ([]byte, Status)

	// Size returns how many bytes this return value takes at most.
	Size() int

	// Done() is called after sending the data to the kernel.
	Done()
}

type MountOptions struct {
	AllowOther bool

	// Options are passed as -o string to fusermount.
	Options []string

	// Default is _DEFAULT_BACKGROUND_TASKS, 12.  This numbers
	// controls the allowed number of requests that relate to
	// async I/O.  Concurrency for synchronous I/O is not limited.
	MaxBackground int

	// MaxWrite is the max size for read and write requests. If 0, use
	// go-fuse default (currently 64 kiB).
	// This number is internally capped at MAX_KERNEL_WRITE (higher values don't make
	// sense).
	//
	// Non-direct-io reads are mostly served via kernel readahead, which is
	// additionally subject to the MaxReadAhead limit.
	//
	// Implementation notes:
	//
	// There's four values the Linux kernel looks at when deciding the request size:
	// * MaxWrite, passed via InitOut.MaxWrite. Limits the WRITE size.
	// * max_read, passed via a string mount option. Limits the READ size.
	//   go-fuse sets max_read equal to MaxWrite.
	//   You can see the current max_read value in /proc/self/mounts .
	// * MaxPages, passed via InitOut.MaxPages. In Linux 4.20 and later, the value
	//   can go up to 1 MiB and go-fuse calculates the MaxPages value acc.
	//   to MaxWrite, rounding up.
	//   On older kernels, the value is fixed at 128 kiB and the
	//   passed value is ignored. No request can be larger than MaxPages, so
	//   READ and WRITE are effectively capped at MaxPages.
	// * MaxReadAhead, passed via InitOut.MaxReadAhead.
	MaxWrite int

	// MaxReadAhead is the max read ahead size to use. It controls how much data the
	// kernel reads in advance to satisfy future read requests from applications.
	// How much exactly is subject to clever heuristics in the kernel
	// (see https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git/tree/mm/readahead.c?h=v6.2-rc5#n375
	// if you are brave) and hence also depends on the kernel version.
	//
	// If 0, use kernel default. This number is capped at the kernel maximum
	// (128 kiB on Linux) and cannot be larger than MaxWrite.
	//
	// MaxReadAhead only affects buffered reads (=non-direct-io), but even then, the
	// kernel can and does send larger reads to satisfy read reqests from applications
	// (up to MaxWrite or VM_READAHEAD_PAGES=128 kiB, whichever is less).
	MaxReadAhead int

	// If IgnoreSecurityLabels is set, all security related xattr
	// requests will return NO_DATA without passing through the
	// user defined filesystem.  You should only set this if you
	// file system implements extended attributes, and you are not
	// interested in security labels.
	IgnoreSecurityLabels bool // ignoring labels should be provided as a fusermount mount option.

	// If RememberInodes is set, we will never forget inodes.
	// This may be useful for NFS.
	RememberInodes bool

	// Values shown in "df -T" and friends
	// First column, "Filesystem"
	FsName string

	// Second column, "Type", will be shown as "fuse." + Name
	Name string

	// If set, wrap the file system in a single-threaded locking wrapper.
	SingleThreaded bool

	// If set, return ENOSYS for Getxattr calls, so the kernel does not issue any
	// Xattr operations at all.
	DisableXAttrs bool

	// If set, print debugging information.
	Debug bool

	// If set, ask kernel to forward file locks to FUSE. If using,
	// you must implement the GetLk/SetLk/SetLkw methods.
	EnableLocks bool

	// If set, the kernel caches all Readlink return values. The
	// filesystem must use content notification to force the
	// kernel to issue a new Readlink call.
	EnableSymlinkCaching bool

	// If set, ask kernel not to do automatic data cache invalidation.
	// The filesystem is fully responsible for invalidating data cache.
	ExplicitDataCacheControl bool

	// SyncRead is off by default, which means that go-fuse enable the
	// FUSE_CAP_ASYNC_READ capability.
	// The kernel then submits multiple concurrent reads to service
	// userspace requests and kernel readahead.
	//
	// Setting SyncRead disables the FUSE_CAP_ASYNC_READ capability.
	// The kernel then only sends one read request per file handle at a time,
	// and orders the requests by offset.
	//
	// This is useful if reading out of order or concurrently is expensive for
	// (example: Amazon Cloud Drive).
	//
	// See the comment to FUSE_CAP_ASYNC_READ in
	// https://github.com/libfuse/libfuse/blob/master/include/fuse_common.h
	// for more details.
	SyncRead bool

	// If set, fuse will first attempt to use syscall.Mount instead of
	// fusermount to mount the filesystem. This will not update /etc/mtab
	// but might be needed if fusermount is not available.
	// Also, Server.Unmount will attempt syscall.Unmount before calling
	// fusermount.
	DirectMount bool

	// DirectMountStrict is like DirectMount but no fallback to fusermount is
	// performed. If both DirectMount and DirectMountStrict are set,
	// DirectMountStrict wins.
	DirectMountStrict bool

	// DirectMountFlags are the mountflags passed to syscall.Mount. If zero, the
	// default value used by fusermount are used: syscall.MS_NOSUID|syscall.MS_NODEV.
	//
	// If you actually *want* zero flags, pass syscall.MS_MGC_VAL, which is ignored
	// by the kernel. See `man 2 mount` for details about MS_MGC_VAL.
	DirectMountFlags uintptr

	// EnableAcls enables kernel ACL support.
	//
	// See the comments to FUSE_CAP_POSIX_ACL
	// in https://github.com/libfuse/libfuse/blob/master/include/fuse_common.h
	// for details.
	EnableAcl bool

	// Disable ReadDirPlus capability so ReadDir is used instead. Simple
	// directory queries (i.e. 'ls' without '-l') can be faster with
	// ReadDir, as no per-file stat calls are needed
	DisableReadDirPlus bool
}

// RawFileSystem is an interface close to the FUSE wire protocol.
//
// Unless you really know what you are doing, you should not implement
// this, but rather the nodefs.Node or pathfs.FileSystem interfaces; the
// details of getting interactions with open files, renames, and threading
// right etc. are somewhat tricky and not very interesting.
//
// Each FUSE request results in a corresponding method called by Server.
// Several calls may be made simultaneously, because the server typically calls
// each method in separate goroutine.
//
// A null implementation is provided by NewDefaultRawFileSystem.
//
// After a successful FUSE API call returns, you may not read input or
// write output data: for performance reasons, memory is reused for
// following requests, and reading/writing the request data will lead
// to race conditions.  If you spawn a background routine from a FUSE
// API call, any incoming request data it wants to reference should be
// copied over.
//
// If a FUSE API call is canceled (which is signaled by closing the
// `cancel` channel), the API call should return EINTR. In this case,
// the outstanding request data is not reused, so the API call may
// return EINTR without ensuring that child contexts have successfully
// completed.
type RawFileSystem interface {
	String() string

	// If called, provide debug output through the log package.
	SetDebug(debug bool)

	// Lookup is called by the kernel when the VFS wants to know
	// about a file inside a directory. Many lookup calls can
	// occur in parallel, but only one call happens for each (dir,
	// name) pair.
	Lookup(cancel <-chan struct{}, header *InHeader, name string, out *EntryOut) (status Status)

	// Forget is called when the kernel discards entries from its
	// dentry cache. This happens on unmount, and when the kernel
	// is short on memory. Since it is not guaranteed to occur at
	// any moment, and since there is no return value, Forget
	// should not do I/O, as there is no channel to report back
	// I/O errors.
	Forget(nodeid, nlookup uint64)

	// Attributes.
	GetAttr(cancel <-chan struct{}, input *GetAttrIn, out *AttrOut) (code Status)
	SetAttr(cancel <-chan struct{}, input *SetAttrIn, out *AttrOut) (code Status)

	// Modifying structure.
	Mknod(cancel <-chan struct{}, input *MknodIn, name string, out *EntryOut) (code Status)
	Mkdir(cancel <-chan struct{}, input *MkdirIn, name string, out *EntryOut) (code Status)
	Unlink(cancel <-chan struct{}, header *InHeader, name string) (code Status)
	Rmdir(cancel <-chan struct{}, header *InHeader, name string) (code Status)
	Rename(cancel <-chan struct{}, input *RenameIn, oldName string, newName string) (code Status)
	Link(cancel <-chan struct{}, input *LinkIn, filename string, out *EntryOut) (code Status)

	Symlink(cancel <-chan struct{}, header *InHeader, pointedTo string, linkName string, out *EntryOut) (code Status)
	Readlink(cancel <-chan struct{}, header *InHeader) (out []byte, code Status)
	Access(cancel <-chan struct{}, input *AccessIn) (code Status)

	// Extended attributes.

	// GetXAttr reads an extended attribute, and should return the
	// number of bytes. If the buffer is too small, return ERANGE,
	// with the required buffer size.
	GetXAttr(cancel <-chan struct{}, header *InHeader, attr string, dest []byte) (sz uint32, code Status)

	// ListXAttr lists extended attributes as '\0' delimited byte
	// slice, and return the number of bytes. If the buffer is too
	// small, return ERANGE, with the required buffer size.
	ListXAttr(cancel <-chan struct{}, header *InHeader, dest []byte) (uint32, Status)

	// SetAttr writes an extended attribute.
	SetXAttr(cancel <-chan struct{}, input *SetXAttrIn, attr string, data []byte) Status

	// RemoveXAttr removes an extended attribute.
	RemoveXAttr(cancel <-chan struct{}, header *InHeader, attr string) (code Status)

	// File handling.
	Create(cancel <-chan struct{}, input *CreateIn, name string, out *CreateOut) (code Status)
	Open(cancel <-chan struct{}, input *OpenIn, out *OpenOut) (status Status)
	Read(cancel <-chan struct{}, input *ReadIn, buf []byte) (ReadResult, Status)
	Lseek(cancel <-chan struct{}, in *LseekIn, out *LseekOut) Status

	// File locking
	GetLk(cancel <-chan struct{}, input *LkIn, out *LkOut) (code Status)
	SetLk(cancel <-chan struct{}, input *LkIn) (code Status)
	SetLkw(cancel <-chan struct{}, input *LkIn) (code Status)

	Release(cancel <-chan struct{}, input *ReleaseIn)
	Write(cancel <-chan struct{}, input *WriteIn, data []byte) (written uint32, code Status)
	CopyFileRange(cancel <-chan struct{}, input *CopyFileRangeIn) (written uint32, code Status)

	Flush(cancel <-chan struct{}, input *FlushIn) Status
	Fsync(cancel <-chan struct{}, input *FsyncIn) (code Status)
	Fallocate(cancel <-chan struct{}, input *FallocateIn) (code Status)

	// Directory handling
	OpenDir(cancel <-chan struct{}, input *OpenIn, out *OpenOut) (status Status)
	ReadDir(cancel <-chan struct{}, input *ReadIn, out *DirEntryList) Status
	ReadDirPlus(cancel <-chan struct{}, input *ReadIn, out *DirEntryList) Status
	ReleaseDir(input *ReleaseIn)
	FsyncDir(cancel <-chan struct{}, input *FsyncIn) (code Status)

	StatFs(cancel <-chan struct{}, input *InHeader, out *StatfsOut) (code Status)

	// This is called on processing the first request. The
	// filesystem implementation can use the server argument to
	// talk back to the kernel (through notify methods).
	Init(*Server)
}
