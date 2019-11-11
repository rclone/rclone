// +build cgo

/*
 * fsop_cgo.go
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
#if !(defined(__APPLE__) || defined(__FreeBSD__) || defined(__NetBSD__) || defined(__OpenBSD__) || defined(__linux__) || defined(_WIN32))
#error platform not supported
#endif

#if defined(__APPLE__) || defined(__FreeBSD__) || defined(__NetBSD__) || defined(__OpenBSD__) || defined(__linux__)

#include <errno.h>
#include <fcntl.h>

#elif defined(_WIN32)

#define EPERM           1
#define ENOENT          2
#define ESRCH           3
#define EINTR           4
#define EIO             5
#define ENXIO           6
#define E2BIG           7
#define ENOEXEC         8
#define EBADF           9
#define ECHILD          10
#define EAGAIN          11
#define ENOMEM          12
#define EACCES          13
#define EFAULT          14
#define EBUSY           16
#define EEXIST          17
#define EXDEV           18
#define ENODEV          19
#define ENOTDIR         20
#define EISDIR          21
#define ENFILE          23
#define EMFILE          24
#define ENOTTY          25
#define EFBIG           27
#define ENOSPC          28
#define ESPIPE          29
#define EROFS           30
#define EMLINK          31
#define EPIPE           32
#define EDOM            33
#define EDEADLK         36
#define ENAMETOOLONG    38
#define ENOLCK          39
#define ENOSYS          40
#define ENOTEMPTY       41
#define EINVAL          22
#define ERANGE          34
#define EILSEQ          42
#define EADDRINUSE      100
#define EADDRNOTAVAIL   101
#define EAFNOSUPPORT    102
#define EALREADY        103
#define EBADMSG         104
#define ECANCELED       105
#define ECONNABORTED    106
#define ECONNREFUSED    107
#define ECONNRESET      108
#define EDESTADDRREQ    109
#define EHOSTUNREACH    110
#define EIDRM           111
#define EINPROGRESS     112
#define EISCONN         113
#define ELOOP           114
#define EMSGSIZE        115
#define ENETDOWN        116
#define ENETRESET       117
#define ENETUNREACH     118
#define ENOBUFS         119
#define ENODATA         120
#define ENOLINK         121
#define ENOMSG          122
#define ENOPROTOOPT     123
#define ENOSR           124
#define ENOSTR          125
#define ENOTCONN        126
#define ENOTRECOVERABLE 127
#define ENOTSOCK        128
#define ENOTSUP         129
#define EOPNOTSUPP      130
#define EOTHER          131
#define EOVERFLOW       132
#define EOWNERDEAD      133
#define EPROTO          134
#define EPROTONOSUPPORT 135
#define EPROTOTYPE      136
#define ETIME           137
#define ETIMEDOUT       138
#define ETXTBSY         139
#define EWOULDBLOCK     140

#include <fcntl.h>
#define O_RDONLY        _O_RDONLY
#define O_WRONLY        _O_WRONLY
#define O_RDWR          _O_RDWR
#define O_APPEND        _O_APPEND
#define O_CREAT         _O_CREAT
#define O_EXCL          _O_EXCL
#define O_TRUNC         _O_TRUNC
#if !defined(O_ACCMODE)
#define O_ACCMODE       (_O_RDONLY|_O_WRONLY|_O_RDWR)
#endif

#endif

#if defined(__linux__) || defined(_WIN32)
// incantation needed for cgo to figure out "kind of name" for ENOATTR
#define ENOATTR         ((int)ENODATA)

#elif defined(__FreeBSD__) || defined(__OpenBSD__)

// ETIME: see https://bugs.freebsd.org/bugzilla/show_bug.cgi?id=225324
// ENODATA: the following is not strictly correct but a lot of project
//     assume that ENODATA == ENOATTR, just because Linux does so.
// ENOSTR, ENOSR: these are not defined anywhere; convert to EINVAL
#define ETIME           ETIMEDOUT
#define ENODATA         ENOATTR
#define ENOSTR          EINVAL
#define ENOSR           EINVAL

#if !defined(ENOLINK)
#define ENOLINK         ENOENT
#endif

#elif defined(__NetBSD__)

// these are not defined anywhere; convert to EINVAL
#define ENOTRECOVERABLE	EINVAL
#define EOWNERDEAD		EINVAL

#endif

#if defined(__APPLE__) || defined(__linux__)
#include <sys/xattr.h>
#elif defined(__FreeBSD__) || defined(__NetBSD__) || defined(__OpenBSD__) || defined(_WIN32)
#define XATTR_CREATE    1
#define XATTR_REPLACE   2
#endif
*/
import "C"

// Error codes reported by FUSE file systems.
const (
	E2BIG           = int(C.E2BIG)
	EACCES          = int(C.EACCES)
	EADDRINUSE      = int(C.EADDRINUSE)
	EADDRNOTAVAIL   = int(C.EADDRNOTAVAIL)
	EAFNOSUPPORT    = int(C.EAFNOSUPPORT)
	EAGAIN          = int(C.EAGAIN)
	EALREADY        = int(C.EALREADY)
	EBADF           = int(C.EBADF)
	EBADMSG         = int(C.EBADMSG)
	EBUSY           = int(C.EBUSY)
	ECANCELED       = int(C.ECANCELED)
	ECHILD          = int(C.ECHILD)
	ECONNABORTED    = int(C.ECONNABORTED)
	ECONNREFUSED    = int(C.ECONNREFUSED)
	ECONNRESET      = int(C.ECONNRESET)
	EDEADLK         = int(C.EDEADLK)
	EDESTADDRREQ    = int(C.EDESTADDRREQ)
	EDOM            = int(C.EDOM)
	EEXIST          = int(C.EEXIST)
	EFAULT          = int(C.EFAULT)
	EFBIG           = int(C.EFBIG)
	EHOSTUNREACH    = int(C.EHOSTUNREACH)
	EIDRM           = int(C.EIDRM)
	EILSEQ          = int(C.EILSEQ)
	EINPROGRESS     = int(C.EINPROGRESS)
	EINTR           = int(C.EINTR)
	EINVAL          = int(C.EINVAL)
	EIO             = int(C.EIO)
	EISCONN         = int(C.EISCONN)
	EISDIR          = int(C.EISDIR)
	ELOOP           = int(C.ELOOP)
	EMFILE          = int(C.EMFILE)
	EMLINK          = int(C.EMLINK)
	EMSGSIZE        = int(C.EMSGSIZE)
	ENAMETOOLONG    = int(C.ENAMETOOLONG)
	ENETDOWN        = int(C.ENETDOWN)
	ENETRESET       = int(C.ENETRESET)
	ENETUNREACH     = int(C.ENETUNREACH)
	ENFILE          = int(C.ENFILE)
	ENOATTR         = int(C.ENOATTR)
	ENOBUFS         = int(C.ENOBUFS)
	ENODATA         = int(C.ENODATA)
	ENODEV          = int(C.ENODEV)
	ENOENT          = int(C.ENOENT)
	ENOEXEC         = int(C.ENOEXEC)
	ENOLCK          = int(C.ENOLCK)
	ENOLINK         = int(C.ENOLINK)
	ENOMEM          = int(C.ENOMEM)
	ENOMSG          = int(C.ENOMSG)
	ENOPROTOOPT     = int(C.ENOPROTOOPT)
	ENOSPC          = int(C.ENOSPC)
	ENOSR           = int(C.ENOSR)
	ENOSTR          = int(C.ENOSTR)
	ENOSYS          = int(C.ENOSYS)
	ENOTCONN        = int(C.ENOTCONN)
	ENOTDIR         = int(C.ENOTDIR)
	ENOTEMPTY       = int(C.ENOTEMPTY)
	ENOTRECOVERABLE = int(C.ENOTRECOVERABLE)
	ENOTSOCK        = int(C.ENOTSOCK)
	ENOTSUP         = int(C.ENOTSUP)
	ENOTTY          = int(C.ENOTTY)
	ENXIO           = int(C.ENXIO)
	EOPNOTSUPP      = int(C.EOPNOTSUPP)
	EOVERFLOW       = int(C.EOVERFLOW)
	EOWNERDEAD      = int(C.EOWNERDEAD)
	EPERM           = int(C.EPERM)
	EPIPE           = int(C.EPIPE)
	EPROTO          = int(C.EPROTO)
	EPROTONOSUPPORT = int(C.EPROTONOSUPPORT)
	EPROTOTYPE      = int(C.EPROTOTYPE)
	ERANGE          = int(C.ERANGE)
	EROFS           = int(C.EROFS)
	ESPIPE          = int(C.ESPIPE)
	ESRCH           = int(C.ESRCH)
	ETIME           = int(C.ETIME)
	ETIMEDOUT       = int(C.ETIMEDOUT)
	ETXTBSY         = int(C.ETXTBSY)
	EWOULDBLOCK     = int(C.EWOULDBLOCK)
	EXDEV           = int(C.EXDEV)
)

// Flags used in FileSystemInterface.Create and FileSystemInterface.Open.
const (
	O_RDONLY  = int(C.O_RDONLY)
	O_WRONLY  = int(C.O_WRONLY)
	O_RDWR    = int(C.O_RDWR)
	O_APPEND  = int(C.O_APPEND)
	O_CREAT   = int(C.O_CREAT)
	O_EXCL    = int(C.O_EXCL)
	O_TRUNC   = int(C.O_TRUNC)
	O_ACCMODE = int(C.O_ACCMODE)
)

// File type and permission bits.
const (
	S_IFMT   = 0170000
	S_IFBLK  = 0060000
	S_IFCHR  = 0020000
	S_IFIFO  = 0010000
	S_IFREG  = 0100000
	S_IFDIR  = 0040000
	S_IFLNK  = 0120000
	S_IFSOCK = 0140000

	S_IRWXU = 00700
	S_IRUSR = 00400
	S_IWUSR = 00200
	S_IXUSR = 00100
	S_IRWXG = 00070
	S_IRGRP = 00040
	S_IWGRP = 00020
	S_IXGRP = 00010
	S_IRWXO = 00007
	S_IROTH = 00004
	S_IWOTH = 00002
	S_IXOTH = 00001
	S_ISUID = 04000
	S_ISGID = 02000
	S_ISVTX = 01000
)

// BSD file flags (Windows file attributes).
const (
	UF_HIDDEN   = 0x00008000
	UF_READONLY = 0x00001000
	UF_SYSTEM   = 0x00000080
	UF_ARCHIVE  = 0x00000800
)

// Options that control Setxattr operation.
const (
	XATTR_CREATE  = int(C.XATTR_CREATE)
	XATTR_REPLACE = int(C.XATTR_REPLACE)
)
