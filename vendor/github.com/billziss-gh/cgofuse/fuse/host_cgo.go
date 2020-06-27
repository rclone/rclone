// +build cgo

/*
 * host_cgo.go
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

/*
#cgo darwin CFLAGS: -DFUSE_USE_VERSION=28 -D_FILE_OFFSET_BITS=64 -I/usr/local/include/osxfuse/fuse
#cgo darwin LDFLAGS: -L/usr/local/lib -losxfuse

#cgo freebsd CFLAGS: -DFUSE_USE_VERSION=28 -D_FILE_OFFSET_BITS=64 -I/usr/local/include/fuse
#cgo freebsd LDFLAGS: -L/usr/local/lib -lfuse

#cgo netbsd CFLAGS: -DFUSE_USE_VERSION=28 -D_FILE_OFFSET_BITS=64 -D_KERNTYPES
#cgo netbsd LDFLAGS: -lrefuse

#cgo openbsd CFLAGS: -DFUSE_USE_VERSION=28 -D_FILE_OFFSET_BITS=64
#cgo openbsd LDFLAGS: -lfuse

#cgo linux CFLAGS: -DFUSE_USE_VERSION=28 -D_FILE_OFFSET_BITS=64 -I/usr/include/fuse
#cgo linux LDFLAGS: -lfuse

// Use `set CPATH=C:\Program Files (x86)\WinFsp\inc\fuse` on Windows.
// The flag `I/usr/local/include/winfsp` only works on xgo and docker.
#cgo windows CFLAGS: -DFUSE_USE_VERSION=28 -I/usr/local/include/winfsp

#if !(defined(__APPLE__) || defined(__FreeBSD__) || defined(__NetBSD__) || defined(__OpenBSD__) || defined(__linux__) || defined(_WIN32))
#error platform not supported
#endif

#include <stdbool.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

#if defined(__APPLE__) || defined(__FreeBSD__) || defined(__NetBSD__) || defined(__OpenBSD__) || defined(__linux__)

#include <spawn.h>
#include <sys/mount.h>
#include <sys/wait.h>
#include <fuse.h>

#elif defined(_WIN32)

#include <windows.h>

static PVOID cgofuse_init_slow(int hardfail);
static VOID  cgofuse_init_fail(VOID);
static PVOID cgofuse_init_winfsp(VOID);

static CRITICAL_SECTION cgofuse_lock;
static PVOID cgofuse_module = 0;
static BOOLEAN cgofuse_stat_ex = FALSE;

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
	EnterCriticalSection(&cgofuse_lock);
	Module = cgofuse_module;
	if (0 == Module)
	{
		Module = cgofuse_init_winfsp();
		MemoryBarrier();
		cgofuse_module = Module;
	}
	LeaveCriticalSection(&cgofuse_lock);
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

	WCHAR PathBuf[MAX_PATH];
	DWORD Size;
	DWORD RegType;
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
			Result = RegQueryValueExW(RegKey, L"InstallDir", 0,
				&RegType, (LPBYTE)PathBuf, &Size);
			RegCloseKey(RegKey);
			if (ERROR_SUCCESS == Result && REG_SZ != RegType)
				Result = ERROR_FILE_NOT_FOUND;
		}
		if (ERROR_SUCCESS != Result)
			return 0xC0000034;//STATUS_OBJECT_NAME_NOT_FOUND

		if (0 < Size && L'\0' == PathBuf[Size / sizeof(WCHAR) - 1])
			Size -= sizeof(WCHAR);

		RtlCopyMemory(PathBuf + Size / sizeof(WCHAR),
			L"" FSP_DLLPATH, sizeof L"" FSP_DLLPATH);
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

#if defined(__APPLE__) || defined(__FreeBSD__) || defined(__NetBSD__) || defined(__OpenBSD__) || defined(__linux__)
typedef struct stat fuse_stat_t;
typedef struct stat fuse_stat_ex_t;
typedef struct statvfs fuse_statvfs_t;
typedef struct timespec fuse_timespec_t;
typedef mode_t fuse_mode_t;
typedef dev_t fuse_dev_t;
typedef uid_t fuse_uid_t;
typedef gid_t fuse_gid_t;
typedef off_t fuse_off_t;
typedef unsigned long fuse_opt_offset_t;
#elif defined(_WIN32)
typedef struct fuse_stat fuse_stat_t;
typedef struct fuse_stat_ex fuse_stat_ex_t;
typedef struct fuse_statvfs fuse_statvfs_t;
typedef struct fuse_timespec fuse_timespec_t;
typedef unsigned int fuse_opt_offset_t;
#endif

extern int go_hostGetattr(char *path, fuse_stat_t *stbuf);
extern int go_hostReadlink(char *path, char *buf, size_t size);
extern int go_hostMknod(char *path, fuse_mode_t mode, fuse_dev_t dev);
extern int go_hostMkdir(char *path, fuse_mode_t mode);
extern int go_hostUnlink(char *path);
extern int go_hostRmdir(char *path);
extern int go_hostSymlink(char *target, char *newpath);
extern int go_hostRename(char *oldpath, char *newpath);
extern int go_hostLink(char *oldpath, char *newpath);
extern int go_hostChmod(char *path, fuse_mode_t mode);
extern int go_hostChown(char *path, fuse_uid_t uid, fuse_gid_t gid);
extern int go_hostTruncate(char *path, fuse_off_t size);
extern int go_hostOpen(char *path, struct fuse_file_info *fi);
extern int go_hostRead(char *path, char *buf, size_t size, fuse_off_t off,
	struct fuse_file_info *fi);
extern int go_hostWrite(char *path, char *buf, size_t size, fuse_off_t off,
	struct fuse_file_info *fi);
extern int go_hostStatfs(char *path, fuse_statvfs_t *stbuf);
extern int go_hostFlush(char *path, struct fuse_file_info *fi);
extern int go_hostRelease(char *path, struct fuse_file_info *fi);
extern int go_hostFsync(char *path, int datasync, struct fuse_file_info *fi);
extern int go_hostSetxattr(char *path, char *name, char *value, size_t size, int flags);
extern int go_hostGetxattr(char *path, char *name, char *value, size_t size);
extern int go_hostListxattr(char *path, char *namebuf, size_t size);
extern int go_hostRemovexattr(char *path, char *name);
extern int go_hostOpendir(char *path, struct fuse_file_info *fi);
extern int go_hostReaddir(char *path, void *buf, fuse_fill_dir_t filler, fuse_off_t off,
	struct fuse_file_info *fi);
extern int go_hostReleasedir(char *path, struct fuse_file_info *fi);
extern int go_hostFsyncdir(char *path, int datasync, struct fuse_file_info *fi);
extern void *go_hostInit(struct fuse_conn_info *conn);
extern void go_hostDestroy(void *data);
extern int go_hostAccess(char *path, int mask);
extern int go_hostCreate(char *path, fuse_mode_t mode, struct fuse_file_info *fi);
extern int go_hostFtruncate(char *path, fuse_off_t off, struct fuse_file_info *fi);
extern int go_hostFgetattr(char *path, fuse_stat_t *stbuf, struct fuse_file_info *fi);
//extern int go_hostLock(char *path, struct fuse_file_info *fi, int cmd, struct fuse_flock *lock);
extern int go_hostUtimens(char *path, fuse_timespec_t tv[2]);
extern int go_hostSetchgtime(char *path, fuse_timespec_t *tv);
extern int go_hostSetcrtime(char *path, fuse_timespec_t *tv);
extern int go_hostChflags(char *path, uint32_t flags);

static inline void hostAsgnCconninfo(struct fuse_conn_info *conn,
	bool capCaseInsensitive,
	bool capReaddirPlus)
{
#if defined(__APPLE__)
	if (capCaseInsensitive)
		FUSE_ENABLE_CASE_INSENSITIVE(conn);
#elif defined(__FreeBSD__) || defined(__NetBSD__) || defined(__OpenBSD__) || defined(__linux__)
#elif defined(_WIN32)
#if defined(FSP_FUSE_CAP_STAT_EX)
	conn->want |= conn->capable & FSP_FUSE_CAP_STAT_EX;
	cgofuse_stat_ex = 0 != (conn->want & FSP_FUSE_CAP_STAT_EX); // hack!
#endif
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
	int64_t birthtimSec, int64_t birthtimNsec,
	uint32_t flags)
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
	stbuf->st_blksize = blksize;
	stbuf->st_blocks = blocks;
#if defined(__APPLE__)
	stbuf->st_atimespec.tv_sec = atimSec; stbuf->st_atimespec.tv_nsec = atimNsec;
	stbuf->st_mtimespec.tv_sec = mtimSec; stbuf->st_mtimespec.tv_nsec = mtimNsec;
	stbuf->st_ctimespec.tv_sec = ctimSec; stbuf->st_ctimespec.tv_nsec = ctimNsec;
	if (0 != birthtimSec)
	{
		stbuf->st_birthtimespec.tv_sec = birthtimSec;
		stbuf->st_birthtimespec.tv_nsec = birthtimNsec;
	}
	else
	{
		stbuf->st_birthtimespec.tv_sec = ctimSec;
		stbuf->st_birthtimespec.tv_nsec = ctimNsec;
	}
	stbuf->st_flags = flags;
#elif defined(_WIN32)
	stbuf->st_atim.tv_sec = atimSec; stbuf->st_atim.tv_nsec = atimNsec;
	stbuf->st_mtim.tv_sec = mtimSec; stbuf->st_mtim.tv_nsec = mtimNsec;
	stbuf->st_ctim.tv_sec = ctimSec; stbuf->st_ctim.tv_nsec = ctimNsec;
	if (0 != birthtimSec)
	{
		stbuf->st_birthtim.tv_sec = birthtimSec;
		stbuf->st_birthtim.tv_nsec = birthtimNsec;
	}
	else
	{
		stbuf->st_birthtim.tv_sec = ctimSec;
		stbuf->st_birthtim.tv_nsec = ctimNsec;
	}
#if defined(FSP_FUSE_CAP_STAT_EX)
	if (cgofuse_stat_ex)
		((struct fuse_stat_ex *)stbuf)->st_flags = flags;
#endif
#else
	stbuf->st_atim.tv_sec = atimSec; stbuf->st_atim.tv_nsec = atimNsec;
	stbuf->st_mtim.tv_sec = mtimSec; stbuf->st_mtim.tv_nsec = mtimNsec;
	stbuf->st_ctim.tv_sec = ctimSec; stbuf->st_ctim.tv_nsec = ctimNsec;
#endif
}

static inline void hostAsgnCfileinfo(struct fuse_file_info *fi,
	bool direct_io,
	bool keep_cache,
	bool nonseekable,
	uint64_t fh)
{
	fi->direct_io = direct_io;
	fi->keep_cache = keep_cache;
	fi->nonseekable = nonseekable;
	fi->fh = fh;
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
	return go_hostSetxattr(path, name, value, size, flags);
}
static int _hostGetxattr(char *path, char *name, char *value, size_t size,
	uint32_t position)
{
	// OSX uses position only for the resource fork; we do not support it!
	return go_hostGetxattr(path, name, value, size);
}
#else
#define _hostSetxattr go_hostSetxattr
#define _hostGetxattr go_hostGetxattr
#endif

// hostStaticInit, hostFuseInit and hostInit serve different purposes.
//
// hostStaticInit and hostFuseInit are needed to provide static and dynamic initialization
// of the FUSE layer. This is currently useful on Windows only.
//
// hostInit is simply the .init implementation of struct fuse_operations.

static void hostStaticInit(void)
{
#if defined(__APPLE__) || defined(__FreeBSD__) || defined(__NetBSD__) || defined(__OpenBSD__) || defined(__linux__)
#elif defined(_WIN32)
	InitializeCriticalSection(&cgofuse_lock);
#endif
}

static int hostFuseInit(void)
{
#if defined(__APPLE__) || defined(__FreeBSD__) || defined(__NetBSD__) || defined(__OpenBSD__) || defined(__linux__)
	return 1;
#elif defined(_WIN32)
	return 0 != cgofuse_init_fast(0);
#endif
}

static int hostMount(int argc, char *argv[], void *data)
{
	static struct fuse_operations fsop =
	{
		.getattr = (int (*)(const char *, fuse_stat_t *))go_hostGetattr,
		.readlink = (int (*)(const char *, char *, size_t))go_hostReadlink,
		.mknod = (int (*)(const char *, fuse_mode_t, fuse_dev_t))go_hostMknod,
		.mkdir = (int (*)(const char *, fuse_mode_t))go_hostMkdir,
		.unlink = (int (*)(const char *))go_hostUnlink,
		.rmdir = (int (*)(const char *))go_hostRmdir,
		.symlink = (int (*)(const char *, const char *))go_hostSymlink,
		.rename = (int (*)(const char *, const char *))go_hostRename,
		.link = (int (*)(const char *, const char *))go_hostLink,
		.chmod = (int (*)(const char *, fuse_mode_t))go_hostChmod,
		.chown = (int (*)(const char *, fuse_uid_t, fuse_gid_t))go_hostChown,
		.truncate = (int (*)(const char *, fuse_off_t))go_hostTruncate,
		.open = (int (*)(const char *, struct fuse_file_info *))go_hostOpen,
		.read = (int (*)(const char *, char *, size_t, fuse_off_t, struct fuse_file_info *))
			go_hostRead,
		.write = (int (*)(const char *, const char *, size_t, fuse_off_t, struct fuse_file_info *))
			go_hostWrite,
		.statfs = (int (*)(const char *, fuse_statvfs_t *))go_hostStatfs,
		.flush = (int (*)(const char *, struct fuse_file_info *))go_hostFlush,
		.release = (int (*)(const char *, struct fuse_file_info *))go_hostRelease,
		.fsync = (int (*)(const char *, int, struct fuse_file_info *))go_hostFsync,
#if defined(__APPLE__)
		.setxattr = (int (*)(const char *, const char *, const char *, size_t, int, uint32_t))
			_hostSetxattr,
		.getxattr = (int (*)(const char *, const char *, char *, size_t, uint32_t))
			_hostGetxattr,
#else
		.setxattr = (int (*)(const char *, const char *, const char *, size_t, int))_hostSetxattr,
		.getxattr = (int (*)(const char *, const char *, char *, size_t))_hostGetxattr,
#endif
		.listxattr = (int (*)(const char *, char *, size_t))go_hostListxattr,
		.removexattr = (int (*)(const char *, const char *))go_hostRemovexattr,
		.opendir = (int (*)(const char *, struct fuse_file_info *))go_hostOpendir,
		.readdir = (int (*)(const char *, void *, fuse_fill_dir_t, fuse_off_t,
			struct fuse_file_info *))go_hostReaddir,
		.releasedir = (int (*)(const char *, struct fuse_file_info *))go_hostReleasedir,
		.fsyncdir = (int (*)(const char *, int, struct fuse_file_info *))go_hostFsyncdir,
		.init = (void *(*)(struct fuse_conn_info *))go_hostInit,
		.destroy = (void (*)(void *))go_hostDestroy,
		.access = (int (*)(const char *, int))go_hostAccess,
		.create = (int (*)(const char *, fuse_mode_t, struct fuse_file_info *))go_hostCreate,
		.ftruncate = (int (*)(const char *, fuse_off_t, struct fuse_file_info *))go_hostFtruncate,
		.fgetattr = (int (*)(const char *, fuse_stat_t *, struct fuse_file_info *))go_hostFgetattr,
		//.lock = (int (*)(const char *, struct fuse_file_info *, int, struct fuse_flock *))
		//	go_hostFlock,
		.utimens = (int (*)(const char *, const fuse_timespec_t [2]))go_hostUtimens,
#if defined(__APPLE__) || (defined(_WIN32) && defined(FSP_FUSE_CAP_STAT_EX))
		.setchgtime = (int (*)(const char *, const fuse_timespec_t *))go_hostSetchgtime,
		.setcrtime = (int (*)(const char *, const fuse_timespec_t *))go_hostSetcrtime,
		.chflags = (int (*)(const char *, uint32_t))go_hostChflags,
#endif
	};
#if defined(__OpenBSD__)
	return 0 == fuse_main(argc, argv, &fsop, data);
#else
	return 0 == fuse_main_real(argc, argv, &fsop, sizeof fsop, data);
#endif
}

static int hostUnmount(struct fuse *fuse, char *mountpoint)
{
#if defined(__APPLE__) || defined(__FreeBSD__) || defined(__NetBSD__) || defined(__OpenBSD__)
	if (0 == mountpoint)
		return 0;
	// darwin,freebsd,netbsd: unmount is available to non-root
	// openbsd: kern.usermount has been removed and mount/unmount is available to root only
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

static void hostOptSet(struct fuse_opt *opt,
	const char *templ, fuse_opt_offset_t offset, int value)
{
	memset(opt, 0, sizeof *opt);
#if defined(__OpenBSD__)
	opt->templ = templ;
	opt->off = offset;
	opt->val = value;
#else
	opt->templ = templ;
	opt->offset = offset;
	opt->value = value;
#endif
}

static int hostOptParseOptProc(void *opt_data, const char *arg, int key,
	struct fuse_args *outargs)
{
	switch (key)
	{
	default:
		return 0;
	case FUSE_OPT_KEY_NONOPT:
		return 1;
	}
}

static int hostOptParse(struct fuse_args *args, void *data, const struct fuse_opt opts[],
	bool nonopts)
{
	return fuse_opt_parse(args, data, opts, nonopts ? hostOptParseOptProc : 0);
}
*/
import "C"
import "unsafe"

type (
	c_bool                  = C.bool
	c_char                  = C.char
	c_fuse_dev_t            = C.fuse_dev_t
	c_fuse_fill_dir_t       = C.fuse_fill_dir_t
	c_fuse_gid_t            = C.fuse_gid_t
	c_fuse_mode_t           = C.fuse_mode_t
	c_fuse_off_t            = C.fuse_off_t
	c_fuse_opt_offset_t     = C.fuse_opt_offset_t
	c_fuse_stat_t           = C.fuse_stat_t
	c_fuse_stat_ex_t        = C.fuse_stat_ex_t
	c_fuse_statvfs_t        = C.fuse_statvfs_t
	c_fuse_timespec_t       = C.fuse_timespec_t
	c_fuse_uid_t            = C.fuse_uid_t
	c_int                   = C.int
	c_int16_t               = C.int16_t
	c_int32_t               = C.int32_t
	c_int64_t               = C.int64_t
	c_int8_t                = C.int8_t
	c_size_t                = C.size_t
	c_struct_fuse           = C.struct_fuse
	c_struct_fuse_args      = C.struct_fuse_args
	c_struct_fuse_conn_info = C.struct_fuse_conn_info
	c_struct_fuse_context   = C.struct_fuse_context
	c_struct_fuse_file_info = C.struct_fuse_file_info
	c_struct_fuse_opt       = C.struct_fuse_opt
	c_uint16_t              = C.uint16_t
	c_uint32_t              = C.uint32_t
	c_uint64_t              = C.uint64_t
	c_uint8_t               = C.uint8_t
	c_uintptr_t             = C.uintptr_t
	c_unsigned              = C.unsigned
)

func c_GoString(s *c_char) string {
	return C.GoString(s)
}
func c_CString(s string) *c_char {
	return C.CString(s)
}

func c_malloc(size c_size_t) unsafe.Pointer {
	return C.malloc(size)
}
func c_calloc(count c_size_t, size c_size_t) unsafe.Pointer {
	return C.calloc(count, size)
}
func c_free(p unsafe.Pointer) {
	C.free(p)
}

func c_fuse_get_context() *c_struct_fuse_context {
	return C.fuse_get_context()
}
func c_fuse_opt_free_args(args *c_struct_fuse_args) {
	C.fuse_opt_free_args(args)
}

func c_hostAsgnCconninfo(conn *c_struct_fuse_conn_info,
	capCaseInsensitive c_bool,
	capReaddirPlus c_bool) {
	C.hostAsgnCconninfo(conn, capCaseInsensitive, capReaddirPlus)
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
	C.hostCstatvfsFromFusestatfs(stbuf,
		bsize,
		frsize,
		blocks,
		bfree,
		bavail,
		files,
		ffree,
		favail,
		fsid,
		flag,
		namemax)
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
	C.hostCstatFromFusestat(stbuf,
		dev,
		ino,
		mode,
		nlink,
		uid,
		gid,
		rdev,
		size,
		atimSec,
		atimNsec,
		mtimSec,
		mtimNsec,
		ctimSec,
		ctimNsec,
		blksize,
		blocks,
		birthtimSec,
		birthtimNsec,
		flags)
}
func c_hostAsgnCfileinfo(fi *c_struct_fuse_file_info,
	direct_io c_bool,
	keep_cache c_bool,
	nonseekable c_bool,
	fh c_uint64_t) {
	C.hostAsgnCfileinfo(fi,
		direct_io,
		keep_cache,
		nonseekable,
		fh)
}
func c_hostFilldir(filler c_fuse_fill_dir_t,
	buf unsafe.Pointer, name *c_char, stbuf *c_fuse_stat_t, off c_fuse_off_t) c_int {
	return C.hostFilldir(filler, buf, name, stbuf, off)
}
func c_hostStaticInit() {
	C.hostStaticInit()
}
func c_hostFuseInit() c_int {
	return C.hostFuseInit()
}
func c_hostMount(argc c_int, argv **c_char, data unsafe.Pointer) c_int {
	return C.hostMount(argc, argv, data)
}
func c_hostUnmount(fuse *c_struct_fuse, mountpoint *c_char) c_int {
	return C.hostUnmount(fuse, mountpoint)
}
func c_hostOptSet(opt *c_struct_fuse_opt,
	templ *c_char, offset c_fuse_opt_offset_t, value c_int) {
	C.hostOptSet(opt, templ, offset, value)
}
func c_hostOptParse(args *c_struct_fuse_args, data unsafe.Pointer, opts *c_struct_fuse_opt,
	nonopts c_bool) c_int {
	return C.hostOptParse(args, data, opts, nonopts)
}

//export go_hostGetattr
func go_hostGetattr(path0 *c_char, stat0 *c_fuse_stat_t) (errc0 c_int) {
	return hostGetattr(path0, stat0)
}

//export go_hostReadlink
func go_hostReadlink(path0 *c_char, buff0 *c_char, size0 c_size_t) (errc0 c_int) {
	return hostReadlink(path0, buff0, size0)
}

//export go_hostMknod
func go_hostMknod(path0 *c_char, mode0 c_fuse_mode_t, dev0 c_fuse_dev_t) (errc0 c_int) {
	return hostMknod(path0, mode0, dev0)
}

//export go_hostMkdir
func go_hostMkdir(path0 *c_char, mode0 c_fuse_mode_t) (errc0 c_int) {
	return hostMkdir(path0, mode0)
}

//export go_hostUnlink
func go_hostUnlink(path0 *c_char) (errc0 c_int) {
	return hostUnlink(path0)
}

//export go_hostRmdir
func go_hostRmdir(path0 *c_char) (errc0 c_int) {
	return hostRmdir(path0)
}

//export go_hostSymlink
func go_hostSymlink(target0 *c_char, newpath0 *c_char) (errc0 c_int) {
	return hostSymlink(target0, newpath0)
}

//export go_hostRename
func go_hostRename(oldpath0 *c_char, newpath0 *c_char) (errc0 c_int) {
	return hostRename(oldpath0, newpath0)
}

//export go_hostLink
func go_hostLink(oldpath0 *c_char, newpath0 *c_char) (errc0 c_int) {
	return hostLink(oldpath0, newpath0)
}

//export go_hostChmod
func go_hostChmod(path0 *c_char, mode0 c_fuse_mode_t) (errc0 c_int) {
	return hostChmod(path0, mode0)
}

//export go_hostChown
func go_hostChown(path0 *c_char, uid0 c_fuse_uid_t, gid0 c_fuse_gid_t) (errc0 c_int) {
	return hostChown(path0, uid0, gid0)
}

//export go_hostTruncate
func go_hostTruncate(path0 *c_char, size0 c_fuse_off_t) (errc0 c_int) {
	return hostTruncate(path0, size0)
}

//export go_hostOpen
func go_hostOpen(path0 *c_char, fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	return hostOpen(path0, fi0)
}

//export go_hostRead
func go_hostRead(path0 *c_char, buff0 *c_char, size0 c_size_t, ofst0 c_fuse_off_t,
	fi0 *c_struct_fuse_file_info) (nbyt0 c_int) {
	return hostRead(path0, buff0, size0, ofst0, fi0)
}

//export go_hostWrite
func go_hostWrite(path0 *c_char, buff0 *c_char, size0 c_size_t, ofst0 c_fuse_off_t,
	fi0 *c_struct_fuse_file_info) (nbyt0 c_int) {
	return hostWrite(path0, buff0, size0, ofst0, fi0)
}

//export go_hostStatfs
func go_hostStatfs(path0 *c_char, stat0 *c_fuse_statvfs_t) (errc0 c_int) {
	return hostStatfs(path0, stat0)
}

//export go_hostFlush
func go_hostFlush(path0 *c_char, fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	return hostFlush(path0, fi0)
}

//export go_hostRelease
func go_hostRelease(path0 *c_char, fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	return hostRelease(path0, fi0)
}

//export go_hostFsync
func go_hostFsync(path0 *c_char, datasync c_int, fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	return hostFsync(path0, datasync, fi0)
}

//export go_hostSetxattr
func go_hostSetxattr(path0 *c_char, name0 *c_char, buff0 *c_char, size0 c_size_t,
	flags c_int) (errc0 c_int) {
	return hostSetxattr(path0, name0, buff0, size0, flags)
}

//export go_hostGetxattr
func go_hostGetxattr(path0 *c_char, name0 *c_char, buff0 *c_char, size0 c_size_t) (nbyt0 c_int) {
	return hostGetxattr(path0, name0, buff0, size0)
}

//export go_hostListxattr
func go_hostListxattr(path0 *c_char, buff0 *c_char, size0 c_size_t) (nbyt0 c_int) {
	return hostListxattr(path0, buff0, size0)
}

//export go_hostRemovexattr
func go_hostRemovexattr(path0 *c_char, name0 *c_char) (errc0 c_int) {
	return hostRemovexattr(path0, name0)
}

//export go_hostOpendir
func go_hostOpendir(path0 *c_char, fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	return hostOpendir(path0, fi0)
}

//export go_hostReaddir
func go_hostReaddir(path0 *c_char,
	buff0 unsafe.Pointer, fill0 c_fuse_fill_dir_t, ofst0 c_fuse_off_t,
	fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	return hostReaddir(path0, buff0, fill0, ofst0, fi0)
}

//export go_hostReleasedir
func go_hostReleasedir(path0 *c_char, fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	return hostReleasedir(path0, fi0)
}

//export go_hostFsyncdir
func go_hostFsyncdir(path0 *c_char, datasync c_int, fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	return hostFsyncdir(path0, datasync, fi0)
}

//export go_hostInit
func go_hostInit(conn0 *c_struct_fuse_conn_info) (user_data unsafe.Pointer) {
	return hostInit(conn0)
}

//export go_hostDestroy
func go_hostDestroy(user_data unsafe.Pointer) {
	hostDestroy(user_data)
}

//export go_hostAccess
func go_hostAccess(path0 *c_char, mask0 c_int) (errc0 c_int) {
	return hostAccess(path0, mask0)
}

//export go_hostCreate
func go_hostCreate(path0 *c_char, mode0 c_fuse_mode_t, fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	return hostCreate(path0, mode0, fi0)
}

//export go_hostFtruncate
func go_hostFtruncate(path0 *c_char, size0 c_fuse_off_t,
	fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	return hostFtruncate(path0, size0, fi0)
}

//export go_hostFgetattr
func go_hostFgetattr(path0 *c_char, stat0 *c_fuse_stat_t,
	fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	return hostFgetattr(path0, stat0, fi0)
}

//export go_hostUtimens
func go_hostUtimens(path0 *c_char, tmsp0 *c_fuse_timespec_t) (errc0 c_int) {
	return hostUtimens(path0, tmsp0)
}

//export go_hostSetchgtime
func go_hostSetchgtime(path0 *c_char, tmsp0 *c_fuse_timespec_t) (errc0 c_int) {
	return hostSetchgtime(path0, tmsp0)
}

//export go_hostSetcrtime
func go_hostSetcrtime(path0 *c_char, tmsp0 *c_fuse_timespec_t) (errc0 c_int) {
	return hostSetcrtime(path0, tmsp0)
}

//export go_hostChflags
func go_hostChflags(path0 *c_char, flags c_uint32_t) (errc0 c_int) {
	return hostChflags(path0, flags)
}
