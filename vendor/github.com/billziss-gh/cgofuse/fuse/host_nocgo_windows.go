// +build !cgo,windows

/*
 * host_nocgo_windows.go
 *
 * Copyright 2017-2018 Bill Zissimopoulos
 */
/*
 * This file is part of Cgofuse.
 *
 * It is licensed under the MIT license. The full license text can be found
 * in the License.txt file at the root of this project.
 */

package fuse

import (
	"path/filepath"
	"sync"
	"syscall"
	"unsafe"
)

type align64 uint32 // align to 64-bits (necessary when compiling for 32bit; see golang/go#599)

type fuse_operations struct {
	getattr     uintptr
	getdir      uintptr
	readlink    uintptr
	mknod       uintptr
	mkdir       uintptr
	unlink      uintptr
	rmdir       uintptr
	symlink     uintptr
	rename      uintptr
	link        uintptr
	chmod       uintptr
	chown       uintptr
	truncate    uintptr
	utime       uintptr
	open        uintptr
	read        uintptr
	write       uintptr
	statfs      uintptr
	flush       uintptr
	release     uintptr
	fsync       uintptr
	setxattr    uintptr
	getxattr    uintptr
	listxattr   uintptr
	removexattr uintptr
	opendir     uintptr
	readdir     uintptr
	releasedir  uintptr
	fsyncdir    uintptr
	init        uintptr
	destroy     uintptr
	access      uintptr
	create      uintptr
	ftruncate   uintptr
	fgetattr    uintptr
	lock        uintptr
	utimens     uintptr
	bmap        uintptr
	flags       uint32
	ioctl       uintptr
	poll        uintptr
	write_buf   uintptr
	read_buf    uintptr
	flock       uintptr
	fallocate   uintptr
	reserved00  uintptr
	reserved01  uintptr
	reserved02  uintptr
	statfs_x    uintptr
	setvolname  uintptr
	exchange    uintptr
	getxtimes   uintptr
	setbkuptime uintptr
	setchgtime  uintptr
	setcrtime   uintptr
	chflags     uintptr
	setattr_x   uintptr
	fsetattr_x  uintptr
}

type fuse_stat_t struct {
	st_dev      c_fuse_dev_t
	_           align64
	st_ino      c_fuse_ino_t
	st_mode     c_fuse_mode_t
	st_nlink    c_fuse_nlink_t
	st_uid      c_fuse_uid_t
	st_gid      c_fuse_gid_t
	st_rdev     c_fuse_dev_t
	_           align64
	st_size     c_fuse_off_t
	st_atim     c_fuse_timespec_t
	st_mtim     c_fuse_timespec_t
	st_ctim     c_fuse_timespec_t
	st_blksize  c_fuse_blksize_t
	_           align64
	st_blocks   c_fuse_blkcnt_t
	st_birthtim c_fuse_timespec_t
}

type fuse_stat_ex_t struct {
	fuse_stat_t
	st_flags      c_uint32_t
	st_reserved32 [3]c_uint32_t
	st_reserved64 [2]c_uint64_t
}

type fuse_statvfs_t struct {
	f_bsize   uintptr
	f_frsize  uintptr
	f_blocks  c_fuse_fsblkcnt_t
	f_bfree   c_fuse_fsblkcnt_t
	f_bavail  c_fuse_fsblkcnt_t
	f_files   c_fuse_fsfilcnt_t
	f_ffree   c_fuse_fsfilcnt_t
	f_favail  c_fuse_fsfilcnt_t
	f_fsid    uintptr
	f_flag    uintptr
	f_namemax uintptr
}

type fuse_timespec_t struct {
	tv_sec  uintptr
	tv_nsec uintptr
}

type struct_fuse struct {
	_ struct{}
}

type struct_fuse_args struct {
	argc      c_int
	argv      **c_char
	allocated c_int
}

type struct_fuse_conn_info struct {
	proto_major   c_unsigned
	proto_minor   c_unsigned
	async_read    c_unsigned
	max_write     c_unsigned
	max_readahead c_unsigned
	capable       c_unsigned
	want          c_unsigned
	reserved      [25]c_unsigned
}

type struct_fuse_context struct {
	fuse         *c_struct_fuse
	uid          c_fuse_uid_t
	gid          c_fuse_gid_t
	pid          c_fuse_pid_t
	private_data unsafe.Pointer
	umask        c_fuse_mode_t
}

type struct_fuse_file_info struct {
	flags      c_int
	fh_old     c_unsigned
	writepage  c_int
	bits       c_uint32_t
	fh         c_uint64_t
	lock_owner c_uint64_t
}

type struct_fuse_opt struct {
	templ  *c_char
	offset c_fuse_opt_offset_t
	value  c_int
}

type (
	c_bool                  = bool
	c_char                  = byte
	c_fuse_blkcnt_t         = int64
	c_fuse_blksize_t        = int32
	c_fuse_dev_t            = uint32
	c_fuse_fill_dir_t       = uintptr
	c_fuse_fsblkcnt_t       = uintptr
	c_fuse_fsfilcnt_t       = uintptr
	c_fuse_gid_t            = uint32
	c_fuse_ino_t            = uint64
	c_fuse_mode_t           = uint32
	c_fuse_nlink_t          = uint16
	c_fuse_off_t            = int64
	c_fuse_opt_offset_t     = uint32
	c_fuse_pid_t            = int32
	c_fuse_stat_t           = fuse_stat_t
	c_fuse_stat_ex_t        = fuse_stat_ex_t
	c_fuse_statvfs_t        = fuse_statvfs_t
	c_fuse_timespec_t       = fuse_timespec_t
	c_fuse_uid_t            = uint32
	c_int                   = int32
	c_int16_t               = int16
	c_int32_t               = int32
	c_int64_t               = int64
	c_int8_t                = int8
	c_size_t                = uintptr
	c_struct_fuse           = struct_fuse
	c_struct_fuse_args      = struct_fuse_args
	c_struct_fuse_conn_info = struct_fuse_conn_info
	c_struct_fuse_context   = struct_fuse_context
	c_struct_fuse_file_info = struct_fuse_file_info
	c_struct_fuse_opt       = struct_fuse_opt
	c_uint16_t              = uint16
	c_uint32_t              = uint32
	c_uint64_t              = uint64
	c_uint8_t               = uint8
	c_uintptr_t             = uintptr
	c_unsigned              = uint32
)

var (
	kernel32       = syscall.MustLoadDLL("kernel32.dll")
	getProcessHeap = kernel32.MustFindProc("GetProcessHeap")
	heapAlloc      = kernel32.MustFindProc("HeapAlloc")
	heapFree       = kernel32.MustFindProc("HeapFree")
	processHeap    uintptr

	/*
	 * It appears safe to call cdecl functions from Go. Is it really?
	 * https://codereview.appspot.com/4961045/
	 */
	fuseOnce                 sync.Once
	fuseDll                  *syscall.DLL
	fuse_version             *syscall.Proc
	fuse_mount               *syscall.Proc
	fuse_unmount             *syscall.Proc
	fuse_parse_cmdline       *syscall.Proc
	fuse_main_real           *syscall.Proc
	fuse_is_lib_option       *syscall.Proc
	fuse_new                 *syscall.Proc
	fuse_destroy             *syscall.Proc
	fuse_loop                *syscall.Proc
	fuse_loop_mt             *syscall.Proc
	fuse_exit                *syscall.Proc
	fuse_get_context         *syscall.Proc
	fuse_opt_parse           *syscall.Proc
	fuse_opt_add_arg         *syscall.Proc
	fuse_opt_insert_arg      *syscall.Proc
	fuse_opt_free_args       *syscall.Proc
	fuse_opt_add_opt         *syscall.Proc
	fuse_opt_add_opt_escaped *syscall.Proc
	fuse_opt_match           *syscall.Proc

	hostOptParseOptProc = syscall.NewCallbackCDecl(c_hostOptParseOptProc)

	cgofuse_stat_ex bool
)

const (
	FSP_FUSE_CAP_CASE_INSENSITIVE = 1 << 29
	FSP_FUSE_CAP_READDIR_PLUS     = 1 << 21
	FSP_FUSE_CAP_STAT_EX          = 1 << 23

	FUSE_OPT_KEY_NONOPT = -2
)

func init() {
	processHeap, _, _ = getProcessHeap.Call()
}

func c_GoString(s *c_char) string {
	if nil == s {
		return ""
	}
	q := (*[1 << 30]c_char)(unsafe.Pointer(s))
	l := 0
	for 0 != q[l] {
		l++
	}
	return string(q[:l])
}
func c_CString(s string) *c_char {
	p := c_malloc(c_size_t(len(s) + 1))
	q := (*[1 << 30]c_char)(p)
	copy(q[:], s)
	q[len(s)] = 0
	return (*c_char)(p)
}

func c_malloc(size c_size_t) unsafe.Pointer {
	p, _, _ := heapAlloc.Call(processHeap, 0, size)
	if 0 == p {
		panic("runtime: C malloc failed")
	}
	return unsafe.Pointer(p)
}
func c_calloc(count c_size_t, size c_size_t) unsafe.Pointer {
	p, _, _ := heapAlloc.Call(processHeap, 8 /*HEAP_ZERO_MEMORY*/, count*size)
	return unsafe.Pointer(p)
}
func c_free(p unsafe.Pointer) {
	if nil != p {
		heapFree.Call(processHeap, 0, uintptr(p))
	}
}

func c_fuse_get_context() *c_struct_fuse_context {
	p, _, _ := fuse_get_context.Call()
	return (*c_struct_fuse_context)(unsafe.Pointer(p))
}
func c_fuse_opt_free_args(args *c_struct_fuse_args) {
	fuse_opt_free_args.Call(uintptr(unsafe.Pointer(args)))
}

func c_hostAsgnCconninfo(conn *c_struct_fuse_conn_info,
	capCaseInsensitive c_bool,
	capReaddirPlus c_bool) {
	conn.want |= conn.capable & FSP_FUSE_CAP_STAT_EX
	cgofuse_stat_ex = 0 != conn.want&FSP_FUSE_CAP_STAT_EX // hack!
	if capCaseInsensitive {
		conn.want |= conn.capable & FSP_FUSE_CAP_CASE_INSENSITIVE
	}
	if capReaddirPlus {
		conn.want |= conn.capable & FSP_FUSE_CAP_READDIR_PLUS
	}
}
func c_hostCstatvfsFromFusestatfs(stbuf *c_fuse_statvfs_t,
	bsize c_uint64_t,
	frsize c_uint64_t,
	blocks c_uint64_t,
	bfree c_uint64_t,
	bavail c_uint64_t,
	files c_uint64_t,
	ffree c_uint64_t,
	favail c_uint64_t,
	fsid c_uint64_t,
	flag c_uint64_t,
	namemax c_uint64_t) {
	*stbuf = c_fuse_statvfs_t{
		f_bsize:   uintptr(bsize),
		f_frsize:  uintptr(frsize),
		f_blocks:  c_fuse_fsblkcnt_t(blocks),
		f_bfree:   c_fuse_fsblkcnt_t(bfree),
		f_bavail:  c_fuse_fsblkcnt_t(bavail),
		f_files:   c_fuse_fsfilcnt_t(files),
		f_ffree:   c_fuse_fsfilcnt_t(ffree),
		f_favail:  c_fuse_fsfilcnt_t(favail),
		f_fsid:    uintptr(fsid),
		f_flag:    uintptr(flag),
		f_namemax: uintptr(namemax),
	}
}
func c_hostCstatFromFusestat(stbuf *c_fuse_stat_t,
	dev c_uint64_t,
	ino c_uint64_t,
	mode c_uint32_t,
	nlink c_uint32_t,
	uid c_uint32_t,
	gid c_uint32_t,
	rdev c_uint64_t,
	size c_int64_t,
	atimSec c_int64_t, atimNsec c_int64_t,
	mtimSec c_int64_t, mtimNsec c_int64_t,
	ctimSec c_int64_t, ctimNsec c_int64_t,
	blksize c_int64_t,
	blocks c_int64_t,
	birthtimSec c_int64_t, birthtimNsec c_int64_t,
	flags c_uint32_t) {
	if !cgofuse_stat_ex {
		*stbuf = c_fuse_stat_t{
			st_dev:     c_fuse_dev_t(dev),
			st_ino:     c_fuse_ino_t(ino),
			st_mode:    c_fuse_mode_t(mode),
			st_nlink:   c_fuse_nlink_t(nlink),
			st_uid:     c_fuse_uid_t(uid),
			st_gid:     c_fuse_gid_t(gid),
			st_rdev:    c_fuse_dev_t(rdev),
			st_size:    c_fuse_off_t(size),
			st_blksize: c_fuse_blksize_t(blksize),
			st_blocks:  c_fuse_blkcnt_t(blocks),
			st_atim: c_fuse_timespec_t{
				tv_sec:  uintptr(atimSec),
				tv_nsec: uintptr(atimNsec),
			},
			st_mtim: c_fuse_timespec_t{
				tv_sec:  uintptr(mtimSec),
				tv_nsec: uintptr(mtimNsec),
			},
			st_ctim: c_fuse_timespec_t{
				tv_sec:  uintptr(ctimSec),
				tv_nsec: uintptr(ctimNsec),
			},
		}
	} else {
		*(*fuse_stat_ex_t)(unsafe.Pointer(stbuf)) = fuse_stat_ex_t{
			fuse_stat_t: c_fuse_stat_t{
				st_dev:     c_fuse_dev_t(dev),
				st_ino:     c_fuse_ino_t(ino),
				st_mode:    c_fuse_mode_t(mode),
				st_nlink:   c_fuse_nlink_t(nlink),
				st_uid:     c_fuse_uid_t(uid),
				st_gid:     c_fuse_gid_t(gid),
				st_rdev:    c_fuse_dev_t(rdev),
				st_size:    c_fuse_off_t(size),
				st_blksize: c_fuse_blksize_t(blksize),
				st_blocks:  c_fuse_blkcnt_t(blocks),
				st_atim: c_fuse_timespec_t{
					tv_sec:  uintptr(atimSec),
					tv_nsec: uintptr(atimNsec),
				},
				st_mtim: c_fuse_timespec_t{
					tv_sec:  uintptr(mtimSec),
					tv_nsec: uintptr(mtimNsec),
				},
				st_ctim: c_fuse_timespec_t{
					tv_sec:  uintptr(ctimSec),
					tv_nsec: uintptr(ctimNsec),
				},
			},
			st_flags: flags,
		}
	}
	if 0 != birthtimSec {
		stbuf.st_birthtim.tv_sec = uintptr(birthtimSec)
		stbuf.st_birthtim.tv_nsec = uintptr(birthtimNsec)
	} else {
		stbuf.st_birthtim.tv_sec = uintptr(ctimSec)
		stbuf.st_birthtim.tv_nsec = uintptr(ctimNsec)
	}
}
func c_hostAsgnCfileinfo(fi *c_struct_fuse_file_info,
	direct_io c_bool,
	keep_cache c_bool,
	nonseekable c_bool,
	fh c_uint64_t) {
	if direct_io {
		fi.bits |= 1
	}
	if keep_cache {
		fi.bits |= 2
	}
	if nonseekable {
		fi.bits |= 8
	}
	fi.fh = fh
}
func c_hostFilldir(filler c_fuse_fill_dir_t,
	buf unsafe.Pointer, name *c_char, stbuf *c_fuse_stat_t, off c_fuse_off_t) c_int {
	var r uintptr
	if uint64(0xffffffff) < uint64(^uintptr(0)) {
		r, _, _ = syscall.Syscall6(filler, 4,
			uintptr(buf),
			uintptr(unsafe.Pointer(name)),
			uintptr(unsafe.Pointer(stbuf)),
			uintptr(off),
			0,
			0)
	} else {
		r, _, _ = syscall.Syscall6(filler, 5,
			uintptr(buf),
			uintptr(unsafe.Pointer(name)),
			uintptr(unsafe.Pointer(stbuf)),
			uintptr(off),
			uintptr(off>>32),
			0)
	}
	return c_int(r)
}
func c_hostStaticInit() {
}
func c_hostFuseInit() c_int {
	fuseOnce.Do(func() {
		fuseDll, _ = fspload()
		if nil != fuseDll {
			fuse_version = fuseDll.MustFindProc("fuse_version")
			fuse_mount = fuseDll.MustFindProc("fuse_mount")
			fuse_unmount = fuseDll.MustFindProc("fuse_unmount")
			fuse_parse_cmdline = fuseDll.MustFindProc("fuse_parse_cmdline")
			fuse_main_real = fuseDll.MustFindProc("fuse_main_real")
			fuse_is_lib_option = fuseDll.MustFindProc("fuse_is_lib_option")
			fuse_new = fuseDll.MustFindProc("fuse_new")
			fuse_destroy = fuseDll.MustFindProc("fuse_destroy")
			fuse_loop = fuseDll.MustFindProc("fuse_loop")
			fuse_loop_mt = fuseDll.MustFindProc("fuse_loop_mt")
			fuse_exit = fuseDll.MustFindProc("fuse_exit")
			fuse_get_context = fuseDll.MustFindProc("fuse_get_context")
			fuse_opt_parse = fuseDll.MustFindProc("fuse_opt_parse")
			fuse_opt_add_arg = fuseDll.MustFindProc("fuse_opt_add_arg")
			fuse_opt_insert_arg = fuseDll.MustFindProc("fuse_opt_insert_arg")
			fuse_opt_free_args = fuseDll.MustFindProc("fuse_opt_free_args")
			fuse_opt_add_opt = fuseDll.MustFindProc("fuse_opt_add_opt")
			fuse_opt_add_opt_escaped = fuseDll.MustFindProc("fuse_opt_add_opt_escaped")
			fuse_opt_match = fuseDll.MustFindProc("fuse_opt_match")
		}
	})
	if nil == fuseDll {
		return 0
	}
	return 1
}
func c_hostMount(argc c_int, argv **c_char, data unsafe.Pointer) c_int {
	r, _, _ := fuse_main_real.Call(
		uintptr(argc),
		uintptr(unsafe.Pointer(argv)),
		uintptr(unsafe.Pointer(&fsop)),
		unsafe.Sizeof(fsop),
		uintptr(data))
	if 0 == r {
		return 1
	}
	return 0
}
func c_hostUnmount(fuse *c_struct_fuse, mountpoint *c_char) c_int {
	fuse_exit.Call(uintptr(unsafe.Pointer(fuse)))
	return 1
}
func c_hostOptSet(opt *c_struct_fuse_opt,
	templ *c_char, offset c_fuse_opt_offset_t, value c_int) {
	*opt = c_struct_fuse_opt{
		templ:  templ,
		offset: offset,
		value:  value,
	}
}
func c_hostOptParseOptProc(opt_data uintptr, arg uintptr, key uintptr, outargs uintptr) uintptr {
	switch c_int(key) {
	default:
		return 0
	case FUSE_OPT_KEY_NONOPT:
		return 1
	}
}
func c_hostOptParse(args *c_struct_fuse_args, data unsafe.Pointer, opts *c_struct_fuse_opt,
	nonopts c_bool) c_int {
	var callback uintptr
	if nonopts {
		callback = hostOptParseOptProc
	}
	r, _, _ := fuse_opt_parse.Call(
		uintptr(unsafe.Pointer(args)),
		uintptr(data),
		uintptr(unsafe.Pointer(opts)),
		callback)
	return c_int(r)
}

func fspload() (dll *syscall.DLL, err error) {
	dllname := ""
	if uint64(0xffffffff) < uint64(^uintptr(0)) {
		dllname = "winfsp-x64.dll"
	} else {
		dllname = "winfsp-x86.dll"
	}

	dll, err = syscall.LoadDLL(dllname)
	if nil == dll {
		var pathbuf [syscall.MAX_PATH]uint16
		var regkey syscall.Handle
		var regtype, size uint32

		kname, _ := syscall.UTF16PtrFromString("Software\\WinFsp")
		err = syscall.RegOpenKeyEx(syscall.HKEY_LOCAL_MACHINE, kname,
			0, syscall.KEY_READ|syscall.KEY_WOW64_32KEY, &regkey)
		if nil != err {
			err = syscall.ERROR_MOD_NOT_FOUND
			return
		}

		vname, _ := syscall.UTF16PtrFromString("InstallDir")
		size = uint32(len(pathbuf) * 2)
		err = syscall.RegQueryValueEx(regkey, vname,
			nil, &regtype, (*byte)(unsafe.Pointer(&pathbuf)), &size)
		syscall.RegCloseKey(regkey)
		if nil != err || syscall.REG_SZ != regtype {
			err = syscall.ERROR_MOD_NOT_FOUND
			return
		}

		if 0 < size && 0 == pathbuf[size/2-1] {
			size -= 2
		}

		path := syscall.UTF16ToString(pathbuf[:size/2])
		dllpath := filepath.Join(path, "bin", dllname)

		dll, err = syscall.LoadDLL(dllpath)
		if nil != err {
			err = syscall.ERROR_MOD_NOT_FOUND
			return
		}
	}

	return
}

var fsop fuse_operations

func init() {
	const intSize = 32 + int(^uintptr(0)>>63<<5)
	if uint64(0xffffffff) < uint64(^uintptr(0)) {
		fsop = fuse_operations{
			getattr:     syscall.NewCallbackCDecl(go_hostGetattr64),
			readlink:    syscall.NewCallbackCDecl(go_hostReadlink64),
			mknod:       syscall.NewCallbackCDecl(go_hostMknod64),
			mkdir:       syscall.NewCallbackCDecl(go_hostMkdir64),
			unlink:      syscall.NewCallbackCDecl(go_hostUnlink64),
			rmdir:       syscall.NewCallbackCDecl(go_hostRmdir64),
			symlink:     syscall.NewCallbackCDecl(go_hostSymlink64),
			rename:      syscall.NewCallbackCDecl(go_hostRename64),
			link:        syscall.NewCallbackCDecl(go_hostLink64),
			chmod:       syscall.NewCallbackCDecl(go_hostChmod64),
			chown:       syscall.NewCallbackCDecl(go_hostChown64),
			truncate:    syscall.NewCallbackCDecl(go_hostTruncate64),
			open:        syscall.NewCallbackCDecl(go_hostOpen64),
			read:        syscall.NewCallbackCDecl(go_hostRead64),
			write:       syscall.NewCallbackCDecl(go_hostWrite64),
			statfs:      syscall.NewCallbackCDecl(go_hostStatfs64),
			flush:       syscall.NewCallbackCDecl(go_hostFlush64),
			release:     syscall.NewCallbackCDecl(go_hostRelease64),
			fsync:       syscall.NewCallbackCDecl(go_hostFsync64),
			setxattr:    syscall.NewCallbackCDecl(go_hostSetxattr64),
			getxattr:    syscall.NewCallbackCDecl(go_hostGetxattr64),
			listxattr:   syscall.NewCallbackCDecl(go_hostListxattr64),
			removexattr: syscall.NewCallbackCDecl(go_hostRemovexattr64),
			opendir:     syscall.NewCallbackCDecl(go_hostOpendir64),
			readdir:     syscall.NewCallbackCDecl(go_hostReaddir64),
			releasedir:  syscall.NewCallbackCDecl(go_hostReleasedir64),
			fsyncdir:    syscall.NewCallbackCDecl(go_hostFsyncdir64),
			init:        syscall.NewCallbackCDecl(go_hostInit64),
			destroy:     syscall.NewCallbackCDecl(go_hostDestroy64),
			access:      syscall.NewCallbackCDecl(go_hostAccess64),
			create:      syscall.NewCallbackCDecl(go_hostCreate64),
			ftruncate:   syscall.NewCallbackCDecl(go_hostFtruncate64),
			fgetattr:    syscall.NewCallbackCDecl(go_hostFgetattr64),
			utimens:     syscall.NewCallbackCDecl(go_hostUtimens64),
			setchgtime:  syscall.NewCallbackCDecl(go_hostSetchgtime64),
			setcrtime:   syscall.NewCallbackCDecl(go_hostSetcrtime64),
			chflags:     syscall.NewCallbackCDecl(go_hostChflags64),
		}
	} else {
		fsop = fuse_operations{
			getattr:     syscall.NewCallbackCDecl(go_hostGetattr32),
			readlink:    syscall.NewCallbackCDecl(go_hostReadlink32),
			mknod:       syscall.NewCallbackCDecl(go_hostMknod32),
			mkdir:       syscall.NewCallbackCDecl(go_hostMkdir32),
			unlink:      syscall.NewCallbackCDecl(go_hostUnlink32),
			rmdir:       syscall.NewCallbackCDecl(go_hostRmdir32),
			symlink:     syscall.NewCallbackCDecl(go_hostSymlink32),
			rename:      syscall.NewCallbackCDecl(go_hostRename32),
			link:        syscall.NewCallbackCDecl(go_hostLink32),
			chmod:       syscall.NewCallbackCDecl(go_hostChmod32),
			chown:       syscall.NewCallbackCDecl(go_hostChown32),
			truncate:    syscall.NewCallbackCDecl(go_hostTruncate32),
			open:        syscall.NewCallbackCDecl(go_hostOpen32),
			read:        syscall.NewCallbackCDecl(go_hostRead32),
			write:       syscall.NewCallbackCDecl(go_hostWrite32),
			statfs:      syscall.NewCallbackCDecl(go_hostStatfs32),
			flush:       syscall.NewCallbackCDecl(go_hostFlush32),
			release:     syscall.NewCallbackCDecl(go_hostRelease32),
			fsync:       syscall.NewCallbackCDecl(go_hostFsync32),
			setxattr:    syscall.NewCallbackCDecl(go_hostSetxattr32),
			getxattr:    syscall.NewCallbackCDecl(go_hostGetxattr32),
			listxattr:   syscall.NewCallbackCDecl(go_hostListxattr32),
			removexattr: syscall.NewCallbackCDecl(go_hostRemovexattr32),
			opendir:     syscall.NewCallbackCDecl(go_hostOpendir32),
			readdir:     syscall.NewCallbackCDecl(go_hostReaddir32),
			releasedir:  syscall.NewCallbackCDecl(go_hostReleasedir32),
			fsyncdir:    syscall.NewCallbackCDecl(go_hostFsyncdir32),
			init:        syscall.NewCallbackCDecl(go_hostInit32),
			destroy:     syscall.NewCallbackCDecl(go_hostDestroy32),
			access:      syscall.NewCallbackCDecl(go_hostAccess32),
			create:      syscall.NewCallbackCDecl(go_hostCreate32),
			ftruncate:   syscall.NewCallbackCDecl(go_hostFtruncate32),
			fgetattr:    syscall.NewCallbackCDecl(go_hostFgetattr32),
			utimens:     syscall.NewCallbackCDecl(go_hostUtimens32),
			setchgtime:  syscall.NewCallbackCDecl(go_hostSetchgtime32),
			setcrtime:   syscall.NewCallbackCDecl(go_hostSetcrtime32),
			chflags:     syscall.NewCallbackCDecl(go_hostChflags32),
		}
	}
}

// 64-bit

func go_hostGetattr64(path0 *c_char, stat0 *c_fuse_stat_t) (errc0 uintptr) {
	return uintptr(int(hostGetattr(path0, stat0)))
}

func go_hostReadlink64(path0 *c_char, buff0 *c_char, size0 uintptr) (errc0 uintptr) {
	return uintptr(int(hostReadlink(path0, buff0, c_size_t(size0))))
}

func go_hostMknod64(path0 *c_char, mode0 uintptr, dev0 uintptr) (errc0 uintptr) {
	return uintptr(int(hostMknod(path0, c_fuse_mode_t(mode0), c_fuse_dev_t(dev0))))
}

func go_hostMkdir64(path0 *c_char, mode0 uintptr) (errc0 uintptr) {
	return uintptr(int(hostMkdir(path0, c_fuse_mode_t(mode0))))
}

func go_hostUnlink64(path0 *c_char) (errc0 uintptr) {
	return uintptr(int(hostUnlink(path0)))
}

func go_hostRmdir64(path0 *c_char) (errc0 uintptr) {
	return uintptr(int(hostRmdir(path0)))
}

func go_hostSymlink64(target0 *c_char, newpath0 *c_char) (errc0 uintptr) {
	return uintptr(int(hostSymlink(target0, newpath0)))
}

func go_hostRename64(oldpath0 *c_char, newpath0 *c_char) (errc0 uintptr) {
	return uintptr(int(hostRename(oldpath0, newpath0)))
}

func go_hostLink64(oldpath0 *c_char, newpath0 *c_char) (errc0 uintptr) {
	return uintptr(int(hostLink(oldpath0, newpath0)))
}

func go_hostChmod64(path0 *c_char, mode0 uintptr) (errc0 uintptr) {
	return uintptr(int(hostChmod(path0, c_fuse_mode_t(mode0))))
}

func go_hostChown64(path0 *c_char, uid0 uintptr, gid0 uintptr) (errc0 uintptr) {
	return uintptr(int(hostChown(path0, c_fuse_uid_t(uid0), c_fuse_gid_t(gid0))))
}

func go_hostTruncate64(path0 *c_char, size0 uintptr) (errc0 uintptr) {
	return uintptr(int(hostTruncate(path0, c_fuse_off_t(size0))))
}

func go_hostOpen64(path0 *c_char, fi0 *c_struct_fuse_file_info) (errc0 uintptr) {
	return uintptr(int(hostOpen(path0, fi0)))
}

func go_hostRead64(path0 *c_char, buff0 *c_char, size0 uintptr, ofst0 uintptr,
	fi0 *c_struct_fuse_file_info) (nbyt0 uintptr) {
	return uintptr(int(hostRead(path0, buff0, c_size_t(size0), c_fuse_off_t(ofst0), fi0)))
}

func go_hostWrite64(path0 *c_char, buff0 *c_char, size0 c_size_t, ofst0 c_fuse_off_t,
	fi0 *c_struct_fuse_file_info) (nbyt0 uintptr) {
	return uintptr(int(hostWrite(path0, buff0, c_size_t(size0), c_fuse_off_t(ofst0), fi0)))
}

func go_hostStatfs64(path0 *c_char, stat0 *c_fuse_statvfs_t) (errc0 uintptr) {
	return uintptr(int(hostStatfs(path0, stat0)))
}

func go_hostFlush64(path0 *c_char, fi0 *c_struct_fuse_file_info) (errc0 uintptr) {
	return uintptr(int(hostFlush(path0, fi0)))
}

func go_hostRelease64(path0 *c_char, fi0 *c_struct_fuse_file_info) (errc0 uintptr) {
	return uintptr(int(hostRelease(path0, fi0)))
}

func go_hostFsync64(path0 *c_char, datasync c_int, fi0 *c_struct_fuse_file_info) (errc0 uintptr) {
	return uintptr(int(hostFsync(path0, c_int(datasync), fi0)))
}

func go_hostSetxattr64(path0 *c_char, name0 *c_char, buff0 *c_char, size0 uintptr,
	flags uintptr) (errc0 uintptr) {
	return uintptr(int(hostSetxattr(path0, name0, buff0, c_size_t(size0), c_int(flags))))
}

func go_hostGetxattr64(path0 *c_char, name0 *c_char, buff0 *c_char, size0 uintptr) (nbyt0 uintptr) {
	return uintptr(int(hostGetxattr(path0, name0, buff0, c_size_t(size0))))
}

func go_hostListxattr64(path0 *c_char, buff0 *c_char, size0 uintptr) (nbyt0 uintptr) {
	return uintptr(int(hostListxattr(path0, buff0, c_size_t(size0))))
}

func go_hostRemovexattr64(path0 *c_char, name0 *c_char) (errc0 uintptr) {
	return uintptr(int(hostRemovexattr(path0, name0)))
}

func go_hostOpendir64(path0 *c_char, fi0 *c_struct_fuse_file_info) (errc0 uintptr) {
	return uintptr(int(hostOpendir(path0, fi0)))
}

func go_hostReaddir64(path0 *c_char,
	buff0 unsafe.Pointer, fill0 c_fuse_fill_dir_t, ofst0 uintptr,
	fi0 *c_struct_fuse_file_info) (errc0 uintptr) {
	return uintptr(int(hostReaddir(path0, buff0, fill0, c_fuse_off_t(ofst0), fi0)))
}

func go_hostReleasedir64(path0 *c_char, fi0 *c_struct_fuse_file_info) (errc0 uintptr) {
	return uintptr(int(hostReleasedir(path0, fi0)))
}

func go_hostFsyncdir64(path0 *c_char, datasync uintptr,
	fi0 *c_struct_fuse_file_info) (errc0 uintptr) {
	return uintptr(int(hostFsyncdir(path0, c_int(datasync), fi0)))
}

func go_hostInit64(conn0 *c_struct_fuse_conn_info) (user_data unsafe.Pointer) {
	return hostInit(conn0)
}

func go_hostDestroy64(user_data unsafe.Pointer) uintptr {
	hostDestroy(user_data)
	return 0
}

func go_hostAccess64(path0 *c_char, mask0 uintptr) (errc0 uintptr) {
	return uintptr(int(hostAccess(path0, c_int(mask0))))
}

func go_hostCreate64(path0 *c_char, mode0 uintptr, fi0 *c_struct_fuse_file_info) (errc0 uintptr) {
	return uintptr(int(hostCreate(path0, c_fuse_mode_t(mode0), fi0)))
}

func go_hostFtruncate64(path0 *c_char, size0 uintptr,
	fi0 *c_struct_fuse_file_info) (errc0 uintptr) {
	return uintptr(int(hostFtruncate(path0, c_fuse_off_t(size0), fi0)))
}

func go_hostFgetattr64(path0 *c_char, stat0 *c_fuse_stat_t,
	fi0 *c_struct_fuse_file_info) (errc0 uintptr) {
	return uintptr(int(hostFgetattr(path0, stat0, fi0)))
}

func go_hostUtimens64(path0 *c_char, tmsp0 *c_fuse_timespec_t) (errc0 uintptr) {
	return uintptr(int(hostUtimens(path0, tmsp0)))
}

func go_hostSetchgtime64(path0 *c_char, tmsp0 *c_fuse_timespec_t) (errc0 uintptr) {
	return uintptr(int(hostSetchgtime(path0, tmsp0)))
}

func go_hostSetcrtime64(path0 *c_char, tmsp0 *c_fuse_timespec_t) (errc0 uintptr) {
	return uintptr(int(hostSetcrtime(path0, tmsp0)))
}

func go_hostChflags64(path0 *c_char, flags c_uint32_t) (errc0 uintptr) {
	return uintptr(int(hostChflags(path0, flags)))
}

// 32-bit

func go_hostGetattr32(path0 *c_char, stat0 *c_fuse_stat_t) (errc0 uintptr) {
	return uintptr(int(hostGetattr(path0, stat0)))
}

func go_hostReadlink32(path0 *c_char, buff0 *c_char, size0 uintptr) (errc0 uintptr) {
	return uintptr(int(hostReadlink(path0, buff0, c_size_t(size0))))
}

func go_hostMknod32(path0 *c_char, mode0 uintptr, dev0 uintptr) (errc0 uintptr) {
	return uintptr(int(hostMknod(path0, c_fuse_mode_t(mode0), c_fuse_dev_t(dev0))))
}

func go_hostMkdir32(path0 *c_char, mode0 uintptr) (errc0 uintptr) {
	return uintptr(int(hostMkdir(path0, c_fuse_mode_t(mode0))))
}

func go_hostUnlink32(path0 *c_char) (errc0 uintptr) {
	return uintptr(int(hostUnlink(path0)))
}

func go_hostRmdir32(path0 *c_char) (errc0 uintptr) {
	return uintptr(int(hostRmdir(path0)))
}

func go_hostSymlink32(target0 *c_char, newpath0 *c_char) (errc0 uintptr) {
	return uintptr(int(hostSymlink(target0, newpath0)))
}

func go_hostRename32(oldpath0 *c_char, newpath0 *c_char) (errc0 uintptr) {
	return uintptr(int(hostRename(oldpath0, newpath0)))
}

func go_hostLink32(oldpath0 *c_char, newpath0 *c_char) (errc0 uintptr) {
	return uintptr(int(hostLink(oldpath0, newpath0)))
}

func go_hostChmod32(path0 *c_char, mode0 uintptr) (errc0 uintptr) {
	return uintptr(int(hostChmod(path0, c_fuse_mode_t(mode0))))
}

func go_hostChown32(path0 *c_char, uid0 uintptr, gid0 uintptr) (errc0 uintptr) {
	return uintptr(int(hostChown(path0, c_fuse_uid_t(uid0), c_fuse_gid_t(gid0))))
}

func go_hostTruncate32(path0 *c_char, lsize0, hsize0 uintptr) (errc0 uintptr) {
	return uintptr(int(hostTruncate(path0, (c_fuse_off_t(hsize0)<<32)|c_fuse_off_t(lsize0))))
}

func go_hostOpen32(path0 *c_char, fi0 *c_struct_fuse_file_info) (errc0 uintptr) {
	return uintptr(int(hostOpen(path0, fi0)))
}

func go_hostRead32(path0 *c_char, buff0 *c_char, size0 uintptr, lofst0, hofst0 uintptr,
	fi0 *c_struct_fuse_file_info) (nbyt0 uintptr) {
	return uintptr(int(hostRead(path0,
		buff0, c_size_t(size0), (c_fuse_off_t(hofst0)<<32)|c_fuse_off_t(lofst0), fi0)))
}

func go_hostWrite32(path0 *c_char, buff0 *c_char, size0 c_size_t, lofst0, hofst0 uintptr,
	fi0 *c_struct_fuse_file_info) (nbyt0 uintptr) {
	return uintptr(int(hostWrite(path0,
		buff0, c_size_t(size0), (c_fuse_off_t(hofst0)<<32)|c_fuse_off_t(lofst0), fi0)))
}

func go_hostStatfs32(path0 *c_char, stat0 *c_fuse_statvfs_t) (errc0 uintptr) {
	return uintptr(int(hostStatfs(path0, stat0)))
}

func go_hostFlush32(path0 *c_char, fi0 *c_struct_fuse_file_info) (errc0 uintptr) {
	return uintptr(int(hostFlush(path0, fi0)))
}

func go_hostRelease32(path0 *c_char, fi0 *c_struct_fuse_file_info) (errc0 uintptr) {
	return uintptr(int(hostRelease(path0, fi0)))
}

func go_hostFsync32(path0 *c_char, datasync c_int, fi0 *c_struct_fuse_file_info) (errc0 uintptr) {
	return uintptr(int(hostFsync(path0, c_int(datasync), fi0)))
}

func go_hostSetxattr32(path0 *c_char, name0 *c_char, buff0 *c_char, size0 uintptr,
	flags uintptr) (errc0 uintptr) {
	return uintptr(int(hostSetxattr(path0, name0, buff0, c_size_t(size0), c_int(flags))))
}

func go_hostGetxattr32(path0 *c_char, name0 *c_char, buff0 *c_char, size0 uintptr) (nbyt0 uintptr) {
	return uintptr(int(hostGetxattr(path0, name0, buff0, c_size_t(size0))))
}

func go_hostListxattr32(path0 *c_char, buff0 *c_char, size0 uintptr) (nbyt0 uintptr) {
	return uintptr(int(hostListxattr(path0, buff0, c_size_t(size0))))
}

func go_hostRemovexattr32(path0 *c_char, name0 *c_char) (errc0 uintptr) {
	return uintptr(int(hostRemovexattr(path0, name0)))
}

func go_hostOpendir32(path0 *c_char, fi0 *c_struct_fuse_file_info) (errc0 uintptr) {
	return uintptr(int(hostOpendir(path0, fi0)))
}

func go_hostReaddir32(path0 *c_char,
	buff0 unsafe.Pointer, fill0 c_fuse_fill_dir_t, lofst0, hofst0 uintptr,
	fi0 *c_struct_fuse_file_info) (errc0 uintptr) {
	return uintptr(int(hostReaddir(path0,
		buff0, fill0, (c_fuse_off_t(hofst0)<<32)|c_fuse_off_t(lofst0), fi0)))
}

func go_hostReleasedir32(path0 *c_char, fi0 *c_struct_fuse_file_info) (errc0 uintptr) {
	return uintptr(int(hostReleasedir(path0, fi0)))
}

func go_hostFsyncdir32(path0 *c_char, datasync uintptr,
	fi0 *c_struct_fuse_file_info) (errc0 uintptr) {
	return uintptr(int(hostFsyncdir(path0, c_int(datasync), fi0)))
}

func go_hostInit32(conn0 *c_struct_fuse_conn_info) (user_data unsafe.Pointer) {
	return hostInit(conn0)
}

func go_hostDestroy32(user_data unsafe.Pointer) uintptr {
	hostDestroy(user_data)
	return 0
}

func go_hostAccess32(path0 *c_char, mask0 uintptr) (errc0 uintptr) {
	return uintptr(int(hostAccess(path0, c_int(mask0))))
}

func go_hostCreate32(path0 *c_char, mode0 uintptr, fi0 *c_struct_fuse_file_info) (errc0 uintptr) {
	return uintptr(int(hostCreate(path0, c_fuse_mode_t(mode0), fi0)))
}

func go_hostFtruncate32(path0 *c_char, lsize0, hsize0 uintptr,
	fi0 *c_struct_fuse_file_info) (errc0 uintptr) {
	return uintptr(int(hostFtruncate(path0, (c_fuse_off_t(hsize0)<<32)|c_fuse_off_t(lsize0), fi0)))
}

func go_hostFgetattr32(path0 *c_char, stat0 *c_fuse_stat_t,
	fi0 *c_struct_fuse_file_info) (errc0 uintptr) {
	return uintptr(int(hostFgetattr(path0, stat0, fi0)))
}

func go_hostUtimens32(path0 *c_char, tmsp0 *c_fuse_timespec_t) (errc0 uintptr) {
	return uintptr(int(hostUtimens(path0, tmsp0)))
}

func go_hostSetchgtime32(path0 *c_char, tmsp0 *c_fuse_timespec_t) (errc0 uintptr) {
	return uintptr(int(hostSetchgtime(path0, tmsp0)))
}

func go_hostSetcrtime32(path0 *c_char, tmsp0 *c_fuse_timespec_t) (errc0 uintptr) {
	return uintptr(int(hostSetcrtime(path0, tmsp0)))
}

func go_hostChflags32(path0 *c_char, flags c_uint32_t) (errc0 uintptr) {
	return uintptr(int(hostChflags(path0, flags)))
}
