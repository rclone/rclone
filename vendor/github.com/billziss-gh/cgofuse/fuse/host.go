/*
 * host.go
 *
 * Copyright 2017 Bill Zissimopoulos
 */
/*
 * This file is part of Cgofuse.
 *
 * It is licensed under the MIT license. The full license text can be found
 * in the License.txt file at the root of this project.
 */

package fuse

/*
#cgo darwin CFLAGS: -DFUSE_USE_VERSION=28 -D_FILE_OFFSET_BITS=64 -I/usr/local/include/osxfuse/fuse
#cgo darwin LDFLAGS: -L/usr/local/lib -losxfuse
#cgo linux CFLAGS: -DFUSE_USE_VERSION=28 -D_FILE_OFFSET_BITS=64 -I/usr/include/fuse
#cgo linux LDFLAGS: -lfuse
#cgo windows CFLAGS: -D_WIN32_WINNT=0x0600 -DFUSE_USE_VERSION=28

#if !(defined(__APPLE__) || defined(__linux__) || defined(_WIN32))
#error platform not supported
#endif

#include <stdbool.h>
#include <stdlib.h>
#include <string.h>

#if defined(__APPLE__) || defined(__linux__)

#include <spawn.h>
#include <sys/mount.h>
#include <sys/wait.h>
#include <fuse.h>

#elif defined(_WIN32)

#include <windows.h>

static PVOID cgofuse_init_slow(int hardfail);
static VOID  cgofuse_init_fail(VOID);
static PVOID cgofuse_init_winfsp(VOID);

static SRWLOCK cgofuse_lock = SRWLOCK_INIT;
static PVOID cgofuse_module = 0;

static inline PVOID cgofuse_init_fast(int hardfail)
{
	PVOID Module = cgofuse_module;
	MemoryBarrier();
	if (0 == Module)
		Module = cgofuse_init_slow(hardfail);
	return Module;
}

static PVOID cgofuse_init_slow(int hardfail)
{
	PVOID Module;
	AcquireSRWLockExclusive(&cgofuse_lock);
	Module = cgofuse_module;
	if (0 == Module)
	{
		Module = cgofuse_init_winfsp();
		MemoryBarrier();
		cgofuse_module = Module;
	}
	ReleaseSRWLockExclusive(&cgofuse_lock);
	if (0 == Module && hardfail)
		cgofuse_init_fail();
	return Module;
}

static VOID cgofuse_init_fail(VOID)
{
	static const char *message = "cgofuse: cannot find winfsp\n";
	DWORD BytesTransferred;
	WriteFile(GetStdHandle(STD_ERROR_HANDLE), message, lstrlenA(message), &BytesTransferred, 0);
	ExitProcess(ERROR_DLL_NOT_FOUND);
}

#define FSP_FUSE_API                    static
#define FSP_FUSE_API_NAME(api)          (* pfn_ ## api)
#define FSP_FUSE_API_CALL(api)          (cgofuse_init_fast(1), pfn_ ## api)
#define FSP_FUSE_SYM(proto, ...)        static inline proto { __VA_ARGS__ }
#include <fuse_common.h>
#include <fuse.h>
#include <fuse_opt.h>

static NTSTATUS FspLoad(PVOID *PModule)
{
#if defined(_WIN64)
#define FSP_DLLNAME                     "winfsp-x64.dll"
#else
#define FSP_DLLNAME                     "winfsp-x86.dll"
#endif
#define FSP_DLLPATH                     "bin\\" FSP_DLLNAME

	WINADVAPI
	LSTATUS
	APIENTRY
	RegGetValueW(
		HKEY hkey,
		LPCWSTR lpSubKey,
		LPCWSTR lpValue,
		DWORD dwFlags,
		LPDWORD pdwType,
		PVOID pvData,
		LPDWORD pcbData);

	WCHAR PathBuf[MAX_PATH];
	DWORD Size;
	HKEY RegKey;
	LONG Result;
	HMODULE Module;

	if (0 != PModule)
		*PModule = 0;

	Module = LoadLibraryW(L"" FSP_DLLNAME);
	if (0 == Module)
	{
		Result = RegOpenKeyExW(HKEY_LOCAL_MACHINE, L"Software\\WinFsp",
			0, KEY_READ | KEY_WOW64_32KEY, &RegKey);
		if (ERROR_SUCCESS == Result)
		{
			Size = sizeof PathBuf - sizeof L"" FSP_DLLPATH + sizeof(WCHAR);
			Result = RegGetValueW(RegKey, 0, L"InstallDir",
				RRF_RT_REG_SZ, 0, PathBuf, &Size);
			RegCloseKey(RegKey);
		}
		if (ERROR_SUCCESS != Result)
			return 0xC0000034;//STATUS_OBJECT_NAME_NOT_FOUND

		RtlCopyMemory(PathBuf + (Size / sizeof(WCHAR) - 1), L"" FSP_DLLPATH, sizeof L"" FSP_DLLPATH);
		Module = LoadLibraryW(PathBuf);
		if (0 == Module)
			return 0xC0000135;//STATUS_DLL_NOT_FOUND
	}

	if (0 != PModule)
		*PModule = Module;

	return 0;//STATUS_SUCCESS

#undef FSP_DLLNAME
#undef FSP_DLLPATH
}

#define CGOFUSE_GET_API(h, n)           \
	if (0 == (*(void **)&(pfn_ ## n) = GetProcAddress(Module, #n)))\
		return 0;

static PVOID cgofuse_init_winfsp(VOID)
{
	PVOID Module;
	NTSTATUS Result;

	Result = FspLoad(&Module);
	if (0 > Result)
		return 0;

	// fuse_common.h
	CGOFUSE_GET_API(h, fsp_fuse_version);
	CGOFUSE_GET_API(h, fsp_fuse_mount);
	CGOFUSE_GET_API(h, fsp_fuse_unmount);
	CGOFUSE_GET_API(h, fsp_fuse_parse_cmdline);
	CGOFUSE_GET_API(h, fsp_fuse_ntstatus_from_errno);

	// fuse.h
	CGOFUSE_GET_API(h, fsp_fuse_main_real);
	CGOFUSE_GET_API(h, fsp_fuse_is_lib_option);
	CGOFUSE_GET_API(h, fsp_fuse_new);
	CGOFUSE_GET_API(h, fsp_fuse_destroy);
	CGOFUSE_GET_API(h, fsp_fuse_loop);
	CGOFUSE_GET_API(h, fsp_fuse_loop_mt);
	CGOFUSE_GET_API(h, fsp_fuse_exit);
	CGOFUSE_GET_API(h, fsp_fuse_get_context);

	// fuse_opt.h
	CGOFUSE_GET_API(h, fsp_fuse_opt_parse);
	CGOFUSE_GET_API(h, fsp_fuse_opt_add_arg);
	CGOFUSE_GET_API(h, fsp_fuse_opt_insert_arg);
	CGOFUSE_GET_API(h, fsp_fuse_opt_free_args);
	CGOFUSE_GET_API(h, fsp_fuse_opt_add_opt);
	CGOFUSE_GET_API(h, fsp_fuse_opt_add_opt_escaped);
	CGOFUSE_GET_API(h, fsp_fuse_opt_match);

	return Module;
}

#endif

#if defined(__APPLE__) || defined(__linux__)
typedef struct stat fuse_stat_t;
typedef struct statvfs fuse_statvfs_t;
typedef struct timespec fuse_timespec_t;
typedef mode_t fuse_mode_t;
typedef dev_t fuse_dev_t;
typedef uid_t fuse_uid_t;
typedef gid_t fuse_gid_t;
typedef off_t fuse_off_t;
#elif defined(_WIN32)
typedef struct fuse_stat fuse_stat_t;
typedef struct fuse_statvfs fuse_statvfs_t;
typedef struct fuse_timespec fuse_timespec_t;
#endif

extern int hostGetattr(char *path, fuse_stat_t *stbuf);
extern int hostReadlink(char *path, char *buf, size_t size);
extern int hostMknod(char *path, fuse_mode_t mode, fuse_dev_t dev);
extern int hostMkdir(char *path, fuse_mode_t mode);
extern int hostUnlink(char *path);
extern int hostRmdir(char *path);
extern int hostSymlink(char *target, char *newpath);
extern int hostRename(char *oldpath, char *newpath);
extern int hostLink(char *oldpath, char *newpath);
extern int hostChmod(char *path, fuse_mode_t mode);
extern int hostChown(char *path, fuse_uid_t uid, fuse_gid_t gid);
extern int hostTruncate(char *path, fuse_off_t size);
extern int hostOpen(char *path, struct fuse_file_info *fi);
extern int hostRead(char *path, char *buf, size_t size, fuse_off_t off,
	struct fuse_file_info *fi);
extern int hostWrite(char *path, char *buf, size_t size, fuse_off_t off,
	struct fuse_file_info *fi);
extern int hostStatfs(char *path, fuse_statvfs_t *stbuf);
extern int hostFlush(char *path, struct fuse_file_info *fi);
extern int hostRelease(char *path, struct fuse_file_info *fi);
extern int hostFsync(char *path, int datasync, struct fuse_file_info *fi);
extern int hostSetxattr(char *path, char *name, char *value, size_t size, int flags);
extern int hostGetxattr(char *path, char *name, char *value, size_t size);
extern int hostListxattr(char *path, char *namebuf, size_t size);
extern int hostRemovexattr(char *path, char *name);
extern int hostOpendir(char *path, struct fuse_file_info *fi);
extern int hostReaddir(char *path, void *buf, fuse_fill_dir_t filler, fuse_off_t off,
	struct fuse_file_info *fi);
extern int hostReleasedir(char *path, struct fuse_file_info *fi);
extern int hostFsyncdir(char *path, int datasync, struct fuse_file_info *fi);
extern void *hostInit(struct fuse_conn_info *conn);
extern void hostDestroy(void *data);
extern int hostAccess(char *path, int mask);
extern int hostCreate(char *path, fuse_mode_t mode, struct fuse_file_info *fi);
extern int hostFtruncate(char *path, fuse_off_t off, struct fuse_file_info *fi);
extern int hostFgetattr(char *path, fuse_stat_t *stbuf, struct fuse_file_info *fi);
//extern int hostLock(char *path, struct fuse_file_info *fi, int cmd, struct fuse_flock *lock);
extern int hostUtimens(char *path, fuse_timespec_t tv[2]);

static inline void hostAsgnCconninfo(struct fuse_conn_info *conn,
	bool capCaseInsensitive,
	bool capReaddirPlus)
{
#if defined(__APPLE__)
	if (capCaseInsensitive)
		FUSE_ENABLE_CASE_INSENSITIVE(conn);
#elif defined(__linux__)
#elif defined(_WIN32)
	if (capCaseInsensitive)
		conn->want |= conn->capable & FSP_FUSE_CAP_CASE_INSENSITIVE;
	if (capReaddirPlus)
		conn->want |= conn->capable & FSP_FUSE_CAP_READDIR_PLUS;
#endif
}

static inline void hostCstatvfsFromFusestatfs(fuse_statvfs_t *stbuf,
	uint64_t bsize,
	uint64_t frsize,
	uint64_t blocks,
	uint64_t bfree,
	uint64_t bavail,
	uint64_t files,
	uint64_t ffree,
	uint64_t favail,
	uint64_t fsid,
	uint64_t flag,
	uint64_t namemax)
{
	memset(stbuf, 0, sizeof *stbuf);
	stbuf->f_bsize = bsize;
	stbuf->f_frsize = frsize;
	stbuf->f_blocks = blocks;
	stbuf->f_bfree = bfree;
	stbuf->f_bavail = bavail;
	stbuf->f_files = files;
	stbuf->f_ffree = ffree;
	stbuf->f_favail = favail;
	stbuf->f_fsid = fsid;
	stbuf->f_flag = flag;
	stbuf->f_namemax = namemax;
}

static inline void hostCstatFromFusestat(fuse_stat_t *stbuf,
	uint64_t dev,
	uint64_t ino,
	uint32_t mode,
	uint32_t nlink,
	uint32_t uid,
	uint32_t gid,
	uint64_t rdev,
	int64_t size,
	int64_t atimSec, int64_t atimNsec,
	int64_t mtimSec, int64_t mtimNsec,
	int64_t ctimSec, int64_t ctimNsec,
	int64_t blksize,
	int64_t blocks,
	int64_t birthtimSec, int64_t birthtimNsec)
{
	memset(stbuf, 0, sizeof *stbuf);
	stbuf->st_dev = dev;
	stbuf->st_ino = ino;
	stbuf->st_mode = mode;
	stbuf->st_nlink = nlink;
	stbuf->st_uid = uid;
	stbuf->st_gid = gid;
	stbuf->st_rdev = rdev;
	stbuf->st_size = size;
#if defined(__APPLE__)
	stbuf->st_atimespec.tv_sec = atimSec; stbuf->st_atimespec.tv_nsec = atimNsec;
	stbuf->st_mtimespec.tv_sec = mtimSec; stbuf->st_mtimespec.tv_nsec = mtimNsec;
	stbuf->st_ctimespec.tv_sec = ctimSec; stbuf->st_ctimespec.tv_nsec = ctimNsec;
	stbuf->st_birthtimespec.tv_sec = birthtimSec; stbuf->st_birthtimespec.tv_nsec = birthtimNsec;
#else
	stbuf->st_atim.tv_sec = atimSec; stbuf->st_atim.tv_nsec = atimNsec;
	stbuf->st_mtim.tv_sec = mtimSec; stbuf->st_mtim.tv_nsec = mtimNsec;
	stbuf->st_ctim.tv_sec = ctimSec; stbuf->st_ctim.tv_nsec = ctimNsec;
#endif
	stbuf->st_blksize = blksize;
	stbuf->st_blocks = blocks;
}

static inline int hostFilldir(fuse_fill_dir_t filler, void *buf,
	char *name, fuse_stat_t *stbuf, fuse_off_t off)
{
	return filler(buf, name, stbuf, off);
}

#if defined(__APPLE__)
static int _hostSetxattr(char *path, char *name, char *value, size_t size, int flags,
	uint32_t position)
{
	// OSX uses position only for the resource fork; we do not support it!
	return hostSetxattr(path, name, value, size, flags);
}
static int _hostGetxattr(char *path, char *name, char *value, size_t size,
	uint32_t position)
{
	// OSX uses position only for the resource fork; we do not support it!
	return hostGetxattr(path, name, value, size);
}
#else
#define _hostSetxattr hostSetxattr
#define _hostGetxattr hostGetxattr
#endif

static int hostInitializeFuse(void)
{
#if defined(__APPLE__) || defined(__linux__)
	return 1;
#elif defined(_WIN32)
	return 0 != cgofuse_init_fast(0);
#endif
}

static int hostMountpointOptProc(void *opt_data, const char *arg, int key,
	struct fuse_args *outargs)
{
	char **pmountpoint = opt_data;
	switch (key)
	{
	default:
		return 1;
	case FUSE_OPT_KEY_NONOPT:
		if (0 == *pmountpoint)
		{
			size_t size = strlen(arg) + 1;
			*pmountpoint = malloc(size);
			if (0 == *pmountpoint)
				return -1;
			memcpy(*pmountpoint, arg, size);
		}
		return 1;
	}
}

static const char *hostMountpoint(int argc, char *argv[])
{
	static struct fuse_opt opts[] = { FUSE_OPT_END };
	struct fuse_args args = FUSE_ARGS_INIT(argc, argv);
	char *mountpoint = 0;
	if (-1 == fuse_opt_parse(&args, &mountpoint, opts, hostMountpointOptProc))
		return 0;
	fuse_opt_free_args(&args);
	return mountpoint;
}

static int hostMount(int argc, char *argv[], void *data)
{
#if defined(__GNUC__)
#pragma GCC diagnostic push
#pragma GCC diagnostic ignored "-Wincompatible-pointer-types"
#endif
	static struct fuse_operations fsop =
	{
		.getattr = (int (*)())hostGetattr,
		.readlink = (int (*)())hostReadlink,
		.mknod = (int (*)())hostMknod,
		.mkdir = (int (*)())hostMkdir,
		.unlink = (int (*)())hostUnlink,
		.rmdir = (int (*)())hostRmdir,
		.symlink = (int (*)())hostSymlink,
		.rename = (int (*)())hostRename,
		.link = (int (*)())hostLink,
		.chmod = (int (*)())hostChmod,
		.chown = (int (*)())hostChown,
		.truncate = (int (*)())hostTruncate,
		.open = (int (*)())hostOpen,
		.read = (int (*)())hostRead,
		.write = (int (*)())hostWrite,
		.statfs = (int (*)())hostStatfs,
		.flush = (int (*)())hostFlush,
		.release = (int (*)())hostRelease,
		.fsync = (int (*)())hostFsync,
		.setxattr = (int (*)())_hostSetxattr,
		.getxattr = (int (*)())_hostGetxattr,
		.listxattr = (int (*)())hostListxattr,
		.removexattr = (int (*)())hostRemovexattr,
		.opendir = (int (*)())hostOpendir,
		.readdir = (int (*)())hostReaddir,
		.releasedir = (int (*)())hostReleasedir,
		.fsyncdir = (int (*)())hostFsyncdir,
		.init = (void *(*)())hostInit,
		.destroy = (void (*)())hostDestroy,
		.access = (int (*)())hostAccess,
		.create = (int (*)())hostCreate,
		.ftruncate = (int (*)())hostFtruncate,
		.fgetattr = (int (*)())hostFgetattr,
		//.lock = (int (*)())hostFlock,
		.utimens = (int (*)())hostUtimens,
	};
#if defined(__GNUC__)
#pragma GCC diagnostic pop
#endif
	return 0 == fuse_main_real(argc, argv, &fsop, sizeof fsop, data);
}

static int hostUnmount(struct fuse *fuse, char *mountpoint)
{
#if defined(__APPLE__)
	if (0 == mountpoint)
		return 0;
	// darwin: unmount is available to non-root
	return 0 == unmount(mountpoint, MNT_FORCE);
#elif defined(__linux__)
	if (0 == mountpoint)
		return 0;
	// linux: try umount2 first in case we are root
	if (0 == umount2(mountpoint, MNT_DETACH))
		return 1;
	// linux: umount2 failed; try fusermount
	char *argv[] =
	{
		"/bin/fusermount",
		"-z",
		"-u",
		mountpoint,
		0,
	};
	pid_t pid = 0;
	int status = 0;
	return
		0 == posix_spawn(&pid, argv[0], 0, 0, argv, 0) &&
		pid == waitpid(pid, &status, 0) &&
		WIFEXITED(status) && 0 == WEXITSTATUS(status);
#elif defined(_WIN32)
	// windows/winfsp: fuse_exit just works from anywhere
	fuse_exit(fuse);
	return 1;
#endif
}
*/
import "C"
import (
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

// FileSystemHost is used to host a file system.
type FileSystemHost struct {
	fsop FileSystemInterface
	fuse *C.struct_fuse
	mntp *C.char
	sigc chan os.Signal

	capCaseInsensitive, capReaddirPlus bool
}

var (
	hostGuard = sync.Mutex{}
	hostTable = map[unsafe.Pointer]*FileSystemHost{}
)

func hostHandleNew(host *FileSystemHost) unsafe.Pointer {
	p := C.malloc(1)
	hostGuard.Lock()
	defer hostGuard.Unlock()
	hostTable[p] = host
	return p
}

func hostHandleDel(p unsafe.Pointer) *FileSystemHost {
	hostGuard.Lock()
	defer hostGuard.Unlock()
	if host, ok := hostTable[p]; ok {
		delete(hostTable, p)
		C.free(p)
		return host
	}
	return nil
}

func hostHandleGet(p unsafe.Pointer) *FileSystemHost {
	hostGuard.Lock()
	defer hostGuard.Unlock()
	if host, ok := hostTable[p]; ok {
		return host
	}
	return nil
}

func copyCstatvfsFromFusestatfs(dst *C.fuse_statvfs_t, src *Statfs_t) {
	C.hostCstatvfsFromFusestatfs(dst,
		C.uint64_t(src.Bsize),
		C.uint64_t(src.Frsize),
		C.uint64_t(src.Blocks),
		C.uint64_t(src.Bfree),
		C.uint64_t(src.Bavail),
		C.uint64_t(src.Files),
		C.uint64_t(src.Ffree),
		C.uint64_t(src.Favail),
		C.uint64_t(src.Fsid),
		C.uint64_t(src.Flag),
		C.uint64_t(src.Namemax))
}

func copyCstatFromFusestat(dst *C.fuse_stat_t, src *Stat_t) {
	C.hostCstatFromFusestat(dst,
		C.uint64_t(src.Dev),
		C.uint64_t(src.Ino),
		C.uint32_t(src.Mode),
		C.uint32_t(src.Nlink),
		C.uint32_t(src.Uid),
		C.uint32_t(src.Gid),
		C.uint64_t(src.Rdev),
		C.int64_t(src.Size),
		C.int64_t(src.Atim.Sec), C.int64_t(src.Atim.Nsec),
		C.int64_t(src.Mtim.Sec), C.int64_t(src.Mtim.Nsec),
		C.int64_t(src.Ctim.Sec), C.int64_t(src.Ctim.Nsec),
		C.int64_t(src.Blksize),
		C.int64_t(src.Blocks),
		C.int64_t(src.Birthtim.Sec), C.int64_t(src.Birthtim.Nsec))
}

func copyFusetimespecFromCtimespec(dst *Timespec, src *C.fuse_timespec_t) {
	dst.Sec = int64(src.tv_sec)
	dst.Nsec = int64(src.tv_nsec)
}

func recoverAsErrno(errc0 *C.int) {
	if r := recover(); nil != r {
		switch e := r.(type) {
		case Error:
			*errc0 = C.int(e)
		default:
			*errc0 = -C.int(EIO)
		}
	}
}

//export hostGetattr
func hostGetattr(path0 *C.char, stat0 *C.fuse_stat_t) (errc0 C.int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(C.fuse_get_context().private_data).fsop
	path := C.GoString(path0)
	stat := &Stat_t{}
	errc := fsop.Getattr(path, stat, ^uint64(0))
	copyCstatFromFusestat(stat0, stat)
	return C.int(errc)
}

//export hostReadlink
func hostReadlink(path0 *C.char, buff0 *C.char, size0 C.size_t) (errc0 C.int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(C.fuse_get_context().private_data).fsop
	path := C.GoString(path0)
	errc, rslt := fsop.Readlink(path)
	buff := (*[1 << 30]byte)(unsafe.Pointer(buff0))
	copy(buff[:size0-1], rslt)
	rlen := len(rslt)
	if C.size_t(rlen) < size0 {
		buff[rlen] = 0
	}
	return C.int(errc)
}

//export hostMknod
func hostMknod(path0 *C.char, mode0 C.fuse_mode_t, dev0 C.fuse_dev_t) (errc0 C.int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(C.fuse_get_context().private_data).fsop
	path := C.GoString(path0)
	errc := fsop.Mknod(path, uint32(mode0), uint64(dev0))
	return C.int(errc)
}

//export hostMkdir
func hostMkdir(path0 *C.char, mode0 C.fuse_mode_t) (errc0 C.int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(C.fuse_get_context().private_data).fsop
	path := C.GoString(path0)
	errc := fsop.Mkdir(path, uint32(mode0))
	return C.int(errc)
}

//export hostUnlink
func hostUnlink(path0 *C.char) (errc0 C.int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(C.fuse_get_context().private_data).fsop
	path := C.GoString(path0)
	errc := fsop.Unlink(path)
	return C.int(errc)
}

//export hostRmdir
func hostRmdir(path0 *C.char) (errc0 C.int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(C.fuse_get_context().private_data).fsop
	path := C.GoString(path0)
	errc := fsop.Rmdir(path)
	return C.int(errc)
}

//export hostSymlink
func hostSymlink(target0 *C.char, newpath0 *C.char) (errc0 C.int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(C.fuse_get_context().private_data).fsop
	target, newpath := C.GoString(target0), C.GoString(newpath0)
	errc := fsop.Symlink(target, newpath)
	return C.int(errc)
}

//export hostRename
func hostRename(oldpath0 *C.char, newpath0 *C.char) (errc0 C.int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(C.fuse_get_context().private_data).fsop
	oldpath, newpath := C.GoString(oldpath0), C.GoString(newpath0)
	errc := fsop.Rename(oldpath, newpath)
	return C.int(errc)
}

//export hostLink
func hostLink(oldpath0 *C.char, newpath0 *C.char) (errc0 C.int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(C.fuse_get_context().private_data).fsop
	oldpath, newpath := C.GoString(oldpath0), C.GoString(newpath0)
	errc := fsop.Link(oldpath, newpath)
	return C.int(errc)
}

//export hostChmod
func hostChmod(path0 *C.char, mode0 C.fuse_mode_t) (errc0 C.int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(C.fuse_get_context().private_data).fsop
	path := C.GoString(path0)
	errc := fsop.Chmod(path, uint32(mode0))
	return C.int(errc)
}

//export hostChown
func hostChown(path0 *C.char, uid0 C.fuse_uid_t, gid0 C.fuse_gid_t) (errc0 C.int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(C.fuse_get_context().private_data).fsop
	path := C.GoString(path0)
	errc := fsop.Chown(path, uint32(uid0), uint32(gid0))
	return C.int(errc)
}

//export hostTruncate
func hostTruncate(path0 *C.char, size0 C.fuse_off_t) (errc0 C.int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(C.fuse_get_context().private_data).fsop
	path := C.GoString(path0)
	errc := fsop.Truncate(path, int64(size0), ^uint64(0))
	return C.int(errc)
}

//export hostOpen
func hostOpen(path0 *C.char, fi0 *C.struct_fuse_file_info) (errc0 C.int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(C.fuse_get_context().private_data).fsop
	path := C.GoString(path0)
	errc, rslt := fsop.Open(path, int(fi0.flags))
	fi0.fh = C.uint64_t(rslt)
	return C.int(errc)
}

//export hostRead
func hostRead(path0 *C.char, buff0 *C.char, size0 C.size_t, ofst0 C.fuse_off_t,
	fi0 *C.struct_fuse_file_info) (nbyt0 C.int) {
	defer recoverAsErrno(&nbyt0)
	fsop := hostHandleGet(C.fuse_get_context().private_data).fsop
	path := C.GoString(path0)
	buff := (*[1 << 30]byte)(unsafe.Pointer(buff0))
	nbyt := fsop.Read(path, buff[:size0], int64(ofst0), uint64(fi0.fh))
	return C.int(nbyt)
}

//export hostWrite
func hostWrite(path0 *C.char, buff0 *C.char, size0 C.size_t, ofst0 C.fuse_off_t,
	fi0 *C.struct_fuse_file_info) (nbyt0 C.int) {
	defer recoverAsErrno(&nbyt0)
	fsop := hostHandleGet(C.fuse_get_context().private_data).fsop
	path := C.GoString(path0)
	buff := (*[1 << 30]byte)(unsafe.Pointer(buff0))
	nbyt := fsop.Write(path, buff[:size0], int64(ofst0), uint64(fi0.fh))
	return C.int(nbyt)
}

//export hostStatfs
func hostStatfs(path0 *C.char, stat0 *C.fuse_statvfs_t) (errc0 C.int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(C.fuse_get_context().private_data).fsop
	path := C.GoString(path0)
	stat := &Statfs_t{}
	errc := fsop.Statfs(path, stat)
	if -ENOSYS == errc {
		stat = &Statfs_t{}
		errc = 0
	}
	copyCstatvfsFromFusestatfs(stat0, stat)
	return C.int(errc)
}

//export hostFlush
func hostFlush(path0 *C.char, fi0 *C.struct_fuse_file_info) (errc0 C.int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(C.fuse_get_context().private_data).fsop
	path := C.GoString(path0)
	errc := fsop.Flush(path, uint64(fi0.fh))
	return C.int(errc)
}

//export hostRelease
func hostRelease(path0 *C.char, fi0 *C.struct_fuse_file_info) (errc0 C.int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(C.fuse_get_context().private_data).fsop
	path := C.GoString(path0)
	errc := fsop.Release(path, uint64(fi0.fh))
	return C.int(errc)
}

//export hostFsync
func hostFsync(path0 *C.char, datasync C.int, fi0 *C.struct_fuse_file_info) (errc0 C.int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(C.fuse_get_context().private_data).fsop
	path := C.GoString(path0)
	errc := fsop.Fsync(path, 0 != datasync, uint64(fi0.fh))
	if -ENOSYS == errc {
		errc = 0
	}
	return C.int(errc)
}

//export hostSetxattr
func hostSetxattr(path0 *C.char, name0 *C.char, buff0 *C.char, size0 C.size_t,
	flags C.int) (errc0 C.int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(C.fuse_get_context().private_data).fsop
	path := C.GoString(path0)
	name := C.GoString(name0)
	buff := (*[1 << 30]byte)(unsafe.Pointer(buff0))
	errc := fsop.Setxattr(path, name, buff[:size0], int(flags))
	return C.int(errc)
}

//export hostGetxattr
func hostGetxattr(path0 *C.char, name0 *C.char, buff0 *C.char, size0 C.size_t) (nbyt0 C.int) {
	defer recoverAsErrno(&nbyt0)
	fsop := hostHandleGet(C.fuse_get_context().private_data).fsop
	path := C.GoString(path0)
	name := C.GoString(name0)
	errc, rslt := fsop.Getxattr(path, name)
	if 0 != errc {
		return C.int(errc)
	}
	if 0 != size0 {
		if len(rslt) > int(size0) {
			return -C.int(ERANGE)
		}
		buff := (*[1 << 30]byte)(unsafe.Pointer(buff0))
		copy(buff[:size0], rslt)
	}
	return C.int(len(rslt))
}

//export hostListxattr
func hostListxattr(path0 *C.char, buff0 *C.char, size0 C.size_t) (nbyt0 C.int) {
	defer recoverAsErrno(&nbyt0)
	fsop := hostHandleGet(C.fuse_get_context().private_data).fsop
	path := C.GoString(path0)
	buff := (*[1 << 30]byte)(unsafe.Pointer(buff0))
	size := int(size0)
	nbyt := 0
	fill := func(name1 string) bool {
		nlen := len(name1)
		if 0 != size {
			if nbyt+nlen+1 > size {
				return false
			}
			copy(buff[nbyt:nbyt+nlen], name1)
			buff[nbyt+nlen] = 0
		}
		nbyt += nlen + 1
		return true
	}
	errc := fsop.Listxattr(path, fill)
	if 0 != errc {
		return C.int(errc)
	}
	return C.int(nbyt)
}

//export hostRemovexattr
func hostRemovexattr(path0 *C.char, name0 *C.char) (errc0 C.int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(C.fuse_get_context().private_data).fsop
	path := C.GoString(path0)
	name := C.GoString(name0)
	errc := fsop.Removexattr(path, name)
	return C.int(errc)
}

//export hostOpendir
func hostOpendir(path0 *C.char, fi0 *C.struct_fuse_file_info) (errc0 C.int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(C.fuse_get_context().private_data).fsop
	path := C.GoString(path0)
	errc, rslt := fsop.Opendir(path)
	if -ENOSYS == errc {
		errc = 0
	}
	fi0.fh = C.uint64_t(rslt)
	return C.int(errc)
}

//export hostReaddir
func hostReaddir(path0 *C.char, buff0 unsafe.Pointer, fill0 C.fuse_fill_dir_t, ofst0 C.fuse_off_t,
	fi0 *C.struct_fuse_file_info) (errc0 C.int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(C.fuse_get_context().private_data).fsop
	path := C.GoString(path0)
	fill := func(name1 string, stat1 *Stat_t, off1 int64) bool {
		name := C.CString(name1)
		defer C.free(unsafe.Pointer(name))
		if nil == stat1 {
			return 0 == C.hostFilldir(fill0, buff0, name, nil, C.fuse_off_t(off1))
		} else {
			stat := C.fuse_stat_t{}
			copyCstatFromFusestat(&stat, stat1)
			return 0 == C.hostFilldir(fill0, buff0, name, &stat, C.fuse_off_t(off1))
		}
	}
	errc := fsop.Readdir(path, fill, int64(ofst0), uint64(fi0.fh))
	return C.int(errc)
}

//export hostReleasedir
func hostReleasedir(path0 *C.char, fi0 *C.struct_fuse_file_info) (errc0 C.int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(C.fuse_get_context().private_data).fsop
	path := C.GoString(path0)
	errc := fsop.Releasedir(path, uint64(fi0.fh))
	return C.int(errc)
}

//export hostFsyncdir
func hostFsyncdir(path0 *C.char, datasync C.int, fi0 *C.struct_fuse_file_info) (errc0 C.int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(C.fuse_get_context().private_data).fsop
	path := C.GoString(path0)
	errc := fsop.Fsyncdir(path, 0 != datasync, uint64(fi0.fh))
	if -ENOSYS == errc {
		errc = 0
	}
	return C.int(errc)
}

//export hostInit
func hostInit(conn0 *C.struct_fuse_conn_info) (user_data unsafe.Pointer) {
	defer recover()
	fctx := C.fuse_get_context()
	user_data = fctx.private_data
	host := hostHandleGet(user_data)
	host.fuse = fctx.fuse
	C.hostAsgnCconninfo(conn0,
		C.bool(host.capCaseInsensitive),
		C.bool(host.capReaddirPlus))
	if nil != host.sigc {
		signal.Notify(host.sigc, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
	}
	host.fsop.Init()
	return
}

//export hostDestroy
func hostDestroy(user_data unsafe.Pointer) {
	defer recover()
	host := hostHandleGet(user_data)
	host.fsop.Destroy()
	if nil != host.sigc {
		signal.Stop(host.sigc)
	}
	host.fuse = nil
}

//export hostAccess
func hostAccess(path0 *C.char, mask0 C.int) (errc0 C.int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(C.fuse_get_context().private_data).fsop
	path := C.GoString(path0)
	errc := fsop.Access(path, uint32(mask0))
	return C.int(errc)
}

//export hostCreate
func hostCreate(path0 *C.char, mode0 C.fuse_mode_t, fi0 *C.struct_fuse_file_info) (errc0 C.int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(C.fuse_get_context().private_data).fsop
	path := C.GoString(path0)
	errc, rslt := fsop.Create(path, int(fi0.flags), uint32(mode0))
	if -ENOSYS == errc {
		errc = fsop.Mknod(path, S_IFREG|uint32(mode0), 0)
		if 0 == errc {
			errc, rslt = fsop.Open(path, int(fi0.flags))
		}
	}
	fi0.fh = C.uint64_t(rslt)
	return C.int(errc)
}

//export hostFtruncate
func hostFtruncate(path0 *C.char, size0 C.fuse_off_t, fi0 *C.struct_fuse_file_info) (errc0 C.int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(C.fuse_get_context().private_data).fsop
	path := C.GoString(path0)
	errc := fsop.Truncate(path, int64(size0), uint64(fi0.fh))
	return C.int(errc)
}

//export hostFgetattr
func hostFgetattr(path0 *C.char, stat0 *C.fuse_stat_t,
	fi0 *C.struct_fuse_file_info) (errc0 C.int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(C.fuse_get_context().private_data).fsop
	path := C.GoString(path0)
	stat := &Stat_t{}
	errc := fsop.Getattr(path, stat, uint64(fi0.fh))
	copyCstatFromFusestat(stat0, stat)
	return C.int(errc)
}

//export hostUtimens
func hostUtimens(path0 *C.char, tmsp0 *C.fuse_timespec_t) (errc0 C.int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(C.fuse_get_context().private_data).fsop
	path := C.GoString(path0)
	if nil == tmsp0 {
		errc := fsop.Utimens(path, nil)
		return C.int(errc)
	} else {
		tmsp := [2]Timespec{}
		tmsa := (*[2]C.fuse_timespec_t)(unsafe.Pointer(tmsp0))
		copyFusetimespecFromCtimespec(&tmsp[0], &tmsa[0])
		copyFusetimespecFromCtimespec(&tmsp[1], &tmsa[1])
		errc := fsop.Utimens(path, tmsp[:])
		return C.int(errc)
	}
}

// NewFileSystemHost creates a file system host.
func NewFileSystemHost(fsop FileSystemInterface) *FileSystemHost {
	host := &FileSystemHost{}
	host.fsop = fsop
	return host
}

// SetCapCaseInsensitive informs the host that the hosted file system is case insensitive
// [OSX and Windows only].
func (host *FileSystemHost) SetCapCaseInsensitive(value bool) {
	host.capCaseInsensitive = value
}

// SetCapReaddirPlus informs the host that the hosted file system has the readdir-plus
// capability [Windows only]. A file system that has the readdir-plus capability can send
// full stat information during Readdir, thus avoiding extraneous Getattr calls.
func (host *FileSystemHost) SetCapReaddirPlus(value bool) {
	host.capReaddirPlus = value
}

// Mount mounts a file system on the given mountpoint with the mount options in opts.
//
// Many of the mount options in opts are specific to the underlying FUSE implementation.
// Some of the common options include:
//
//     -h   --help            print help
//     -V   --version         print FUSE version
//     -d   -o debug          enable FUSE debug output
//     -s                     disable multi-threaded operation
//
// Please refer to the individual FUSE implementation documentation for additional options.
//
// It is allowed for the mountpoint to be the empty string ("") in which case opts is assumed
// to contain the mountpoint. It is also allowed for opts to be nil, although in this case the
// mountpoint must be non-empty.
//
// The file system is considered mounted only after its Init() method has been called
// and before its Destroy() method has been called.
func (host *FileSystemHost) Mount(mountpoint string, opts []string) bool {
	if 0 == C.hostInitializeFuse() {
		panic("cgofuse: cannot find winfsp")
	}

	/*
	 * Command line handling
	 *
	 * We must prepare a command line to send to FUSE. This command line will look like this:
	 *
	 *     execname [mountpoint] "-f" [opts...] NULL
	 *
	 * We add the "-f" option because Go cannot handle daemonization (at least on OSX).
	 */
	exec := "<UNKNOWN>"
	if 0 < len(os.Args) {
		exec = os.Args[0]
	}
	argc := len(opts) + 2
	if "" != mountpoint {
		argc++
	}
	argv := make([]*C.char, argc+1)
	argv[0] = C.CString(exec)
	defer C.free(unsafe.Pointer(argv[0]))
	opti := 1
	if "" != mountpoint {
		argv[1] = C.CString(mountpoint)
		defer C.free(unsafe.Pointer(argv[1]))
		opti++
	}
	argv[opti] = C.CString("-f")
	defer C.free(unsafe.Pointer(argv[opti]))
	opti++
	for i := 0; len(opts) > i; i++ {
		argv[i+opti] = C.CString(opts[i])
		defer C.free(unsafe.Pointer(argv[i+opti]))
	}

	/*
	 * Mountpoint extraction
	 *
	 * We need to determine the mountpoint that FUSE is going (to try) to use, so that we
	 * can unmount later.
	 */
	host.mntp = C.hostMountpoint(C.int(argc), &argv[0])
	defer func() {
		C.free(unsafe.Pointer(host.mntp))
		host.mntp = nil
	}()

	/*
	 * Handle zombie mounts
	 *
	 * FUSE on UNIX does not automatically unmount the file system, leaving behind "zombie"
	 * mounts. So set things up to always unmount the file system (unless forcibly terminated).
	 * This has the added benefit that the file system Destroy() always gets called.
	 *
	 * On Windows (WinFsp) this is handled by the FUSE layer and we do not have to do anything.
	 */
	if "windows" != runtime.GOOS {
		done := make(chan bool)
		defer func() {
			<-done
		}()
		host.sigc = make(chan os.Signal, 1)
		defer close(host.sigc)
		go func() {
			_, ok := <-host.sigc
			if ok {
				host.Unmount()
			}
			close(done)
		}()
	}

	/*
	 * Tell FUSE to do its job!
	 */
	hndl := hostHandleNew(host)
	defer hostHandleDel(hndl)
	return 0 != C.hostMount(C.int(argc), &argv[0], hndl)
}

// Unmount unmounts a mounted file system.
// Unmount may be called at any time after the Init() method has been called
// and before the Destroy() method has been called.
func (host *FileSystemHost) Unmount() bool {
	if nil == host.fuse {
		return false
	}
	return 0 != C.hostUnmount(host.fuse, host.mntp)
}

// Getcontext gets information related to a file system operation.
func Getcontext() (uid uint32, gid uint32, pid int) {
	uid = uint32(C.fuse_get_context().uid)
	gid = uint32(C.fuse_get_context().gid)
	pid = int(C.fuse_get_context().pid)
	return
}
