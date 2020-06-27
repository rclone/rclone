/*
 * host.go
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
	"errors"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

// FileSystemHost is used to host a file system.
type FileSystemHost struct {
	fsop FileSystemInterface
	fuse *c_struct_fuse
	mntp string
	sigc chan os.Signal

	capCaseInsensitive, capReaddirPlus bool
}

var (
	hostGuard = sync.Mutex{}
	hostTable = map[unsafe.Pointer]*FileSystemHost{}
)

func hostHandleNew(host *FileSystemHost) unsafe.Pointer {
	p := c_malloc(1)
	hostGuard.Lock()
	hostTable[p] = host
	hostGuard.Unlock()
	return p
}

func hostHandleDel(p unsafe.Pointer) {
	hostGuard.Lock()
	delete(hostTable, p)
	hostGuard.Unlock()
	c_free(p)
}

func hostHandleGet(p unsafe.Pointer) *FileSystemHost {
	hostGuard.Lock()
	host, _ := hostTable[p]
	hostGuard.Unlock()
	return host
}

func copyCstatvfsFromFusestatfs(dst *c_fuse_statvfs_t, src *Statfs_t) {
	c_hostCstatvfsFromFusestatfs(dst,
		c_uint64_t(src.Bsize),
		c_uint64_t(src.Frsize),
		c_uint64_t(src.Blocks),
		c_uint64_t(src.Bfree),
		c_uint64_t(src.Bavail),
		c_uint64_t(src.Files),
		c_uint64_t(src.Ffree),
		c_uint64_t(src.Favail),
		c_uint64_t(src.Fsid),
		c_uint64_t(src.Flag),
		c_uint64_t(src.Namemax))
}

func copyCstatFromFusestat(dst *c_fuse_stat_t, src *Stat_t) {
	c_hostCstatFromFusestat(dst,
		c_uint64_t(src.Dev),
		c_uint64_t(src.Ino),
		c_uint32_t(src.Mode),
		c_uint32_t(src.Nlink),
		c_uint32_t(src.Uid),
		c_uint32_t(src.Gid),
		c_uint64_t(src.Rdev),
		c_int64_t(src.Size),
		c_int64_t(src.Atim.Sec), c_int64_t(src.Atim.Nsec),
		c_int64_t(src.Mtim.Sec), c_int64_t(src.Mtim.Nsec),
		c_int64_t(src.Ctim.Sec), c_int64_t(src.Ctim.Nsec),
		c_int64_t(src.Blksize),
		c_int64_t(src.Blocks),
		c_int64_t(src.Birthtim.Sec), c_int64_t(src.Birthtim.Nsec),
		c_uint32_t(src.Flags))
}

func copyFusetimespecFromCtimespec(dst *Timespec, src *c_fuse_timespec_t) {
	dst.Sec = int64(src.tv_sec)
	dst.Nsec = int64(src.tv_nsec)
}

func recoverAsErrno(errc0 *c_int) {
	if r := recover(); nil != r {
		switch e := r.(type) {
		case Error:
			*errc0 = c_int(e)
		default:
			*errc0 = -c_int(EIO)
		}
	}
}

func hostGetattr(path0 *c_char, stat0 *c_fuse_stat_t) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	stat := &Stat_t{}
	errc := fsop.Getattr(path, stat, ^uint64(0))
	copyCstatFromFusestat(stat0, stat)
	return c_int(errc)
}

func hostReadlink(path0 *c_char, buff0 *c_char, size0 c_size_t) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	errc, rslt := fsop.Readlink(path)
	buff := (*[1 << 30]byte)(unsafe.Pointer(buff0))
	copy(buff[:size0-1], rslt)
	rlen := len(rslt)
	if c_size_t(rlen) < size0 {
		buff[rlen] = 0
	}
	return c_int(errc)
}

func hostMknod(path0 *c_char, mode0 c_fuse_mode_t, dev0 c_fuse_dev_t) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	errc := fsop.Mknod(path, uint32(mode0), uint64(dev0))
	return c_int(errc)
}

func hostMkdir(path0 *c_char, mode0 c_fuse_mode_t) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	errc := fsop.Mkdir(path, uint32(mode0))
	return c_int(errc)
}

func hostUnlink(path0 *c_char) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	errc := fsop.Unlink(path)
	return c_int(errc)
}

func hostRmdir(path0 *c_char) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	errc := fsop.Rmdir(path)
	return c_int(errc)
}

func hostSymlink(target0 *c_char, newpath0 *c_char) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	target, newpath := c_GoString(target0), c_GoString(newpath0)
	errc := fsop.Symlink(target, newpath)
	return c_int(errc)
}

func hostRename(oldpath0 *c_char, newpath0 *c_char) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	oldpath, newpath := c_GoString(oldpath0), c_GoString(newpath0)
	errc := fsop.Rename(oldpath, newpath)
	return c_int(errc)
}

func hostLink(oldpath0 *c_char, newpath0 *c_char) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	oldpath, newpath := c_GoString(oldpath0), c_GoString(newpath0)
	errc := fsop.Link(oldpath, newpath)
	return c_int(errc)
}

func hostChmod(path0 *c_char, mode0 c_fuse_mode_t) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	errc := fsop.Chmod(path, uint32(mode0))
	return c_int(errc)
}

func hostChown(path0 *c_char, uid0 c_fuse_uid_t, gid0 c_fuse_gid_t) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	errc := fsop.Chown(path, uint32(uid0), uint32(gid0))
	return c_int(errc)
}

func hostTruncate(path0 *c_char, size0 c_fuse_off_t) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	errc := fsop.Truncate(path, int64(size0), ^uint64(0))
	return c_int(errc)
}

func hostOpen(path0 *c_char, fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	intf, ok := fsop.(FileSystemOpenEx)
	if ok {
		fi := FileInfo_t{Flags: int(fi0.flags)}
		errc := intf.OpenEx(path, &fi)
		c_hostAsgnCfileinfo(fi0,
			c_bool(fi.DirectIo),
			c_bool(fi.KeepCache),
			c_bool(fi.NonSeekable),
			c_uint64_t(fi.Fh))
		return c_int(errc)
	} else {
		errc, rslt := fsop.Open(path, int(fi0.flags))
		fi0.fh = c_uint64_t(rslt)
		return c_int(errc)
	}
}

func hostRead(path0 *c_char, buff0 *c_char, size0 c_size_t, ofst0 c_fuse_off_t,
	fi0 *c_struct_fuse_file_info) (nbyt0 c_int) {
	defer recoverAsErrno(&nbyt0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	buff := (*[1 << 30]byte)(unsafe.Pointer(buff0))
	nbyt := fsop.Read(path, buff[:size0], int64(ofst0), uint64(fi0.fh))
	return c_int(nbyt)
}

func hostWrite(path0 *c_char, buff0 *c_char, size0 c_size_t, ofst0 c_fuse_off_t,
	fi0 *c_struct_fuse_file_info) (nbyt0 c_int) {
	defer recoverAsErrno(&nbyt0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	buff := (*[1 << 30]byte)(unsafe.Pointer(buff0))
	nbyt := fsop.Write(path, buff[:size0], int64(ofst0), uint64(fi0.fh))
	return c_int(nbyt)
}

func hostStatfs(path0 *c_char, stat0 *c_fuse_statvfs_t) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	stat := &Statfs_t{}
	errc := fsop.Statfs(path, stat)
	if -ENOSYS == errc {
		stat = &Statfs_t{}
		errc = 0
	}
	copyCstatvfsFromFusestatfs(stat0, stat)
	return c_int(errc)
}

func hostFlush(path0 *c_char, fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	errc := fsop.Flush(path, uint64(fi0.fh))
	return c_int(errc)
}

func hostRelease(path0 *c_char, fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	errc := fsop.Release(path, uint64(fi0.fh))
	return c_int(errc)
}

func hostFsync(path0 *c_char, datasync c_int, fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	errc := fsop.Fsync(path, 0 != datasync, uint64(fi0.fh))
	if -ENOSYS == errc {
		errc = 0
	}
	return c_int(errc)
}

func hostSetxattr(path0 *c_char, name0 *c_char, buff0 *c_char, size0 c_size_t,
	flags c_int) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	name := c_GoString(name0)
	buff := (*[1 << 30]byte)(unsafe.Pointer(buff0))
	errc := fsop.Setxattr(path, name, buff[:size0], int(flags))
	return c_int(errc)
}

func hostGetxattr(path0 *c_char, name0 *c_char, buff0 *c_char, size0 c_size_t) (nbyt0 c_int) {
	defer recoverAsErrno(&nbyt0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	name := c_GoString(name0)
	errc, rslt := fsop.Getxattr(path, name)
	if 0 != errc {
		return c_int(errc)
	}
	if 0 != size0 {
		if len(rslt) > int(size0) {
			return -c_int(ERANGE)
		}
		buff := (*[1 << 30]byte)(unsafe.Pointer(buff0))
		copy(buff[:size0], rslt)
	}
	return c_int(len(rslt))
}

func hostListxattr(path0 *c_char, buff0 *c_char, size0 c_size_t) (nbyt0 c_int) {
	defer recoverAsErrno(&nbyt0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
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
		return c_int(errc)
	}
	return c_int(nbyt)
}

func hostRemovexattr(path0 *c_char, name0 *c_char) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	name := c_GoString(name0)
	errc := fsop.Removexattr(path, name)
	return c_int(errc)
}

func hostOpendir(path0 *c_char, fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	errc, rslt := fsop.Opendir(path)
	if -ENOSYS == errc {
		errc = 0
	}
	fi0.fh = c_uint64_t(rslt)
	return c_int(errc)
}

func hostReaddir(path0 *c_char, buff0 unsafe.Pointer, fill0 c_fuse_fill_dir_t, ofst0 c_fuse_off_t,
	fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	fill := func(name1 string, stat1 *Stat_t, off1 int64) bool {
		name := c_CString(name1)
		defer c_free(unsafe.Pointer(name))
		if nil == stat1 {
			return 0 == c_hostFilldir(fill0, buff0, name, nil, c_fuse_off_t(off1))
		} else {
			stat_ex := c_fuse_stat_ex_t{} // support WinFsp fuse_stat_ex
			stat := (*c_fuse_stat_t)(unsafe.Pointer(&stat_ex))
			copyCstatFromFusestat(stat, stat1)
			return 0 == c_hostFilldir(fill0, buff0, name, stat, c_fuse_off_t(off1))
		}
	}
	errc := fsop.Readdir(path, fill, int64(ofst0), uint64(fi0.fh))
	return c_int(errc)
}

func hostReleasedir(path0 *c_char, fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	errc := fsop.Releasedir(path, uint64(fi0.fh))
	return c_int(errc)
}

func hostFsyncdir(path0 *c_char, datasync c_int, fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	errc := fsop.Fsyncdir(path, 0 != datasync, uint64(fi0.fh))
	if -ENOSYS == errc {
		errc = 0
	}
	return c_int(errc)
}

func hostInit(conn0 *c_struct_fuse_conn_info) (user_data unsafe.Pointer) {
	defer func() {
		recover()
	}()
	fctx := c_fuse_get_context()
	user_data = fctx.private_data
	host := hostHandleGet(user_data)
	host.fuse = fctx.fuse
	c_hostAsgnCconninfo(conn0,
		c_bool(host.capCaseInsensitive),
		c_bool(host.capReaddirPlus))
	if nil != host.sigc {
		signal.Notify(host.sigc, syscall.SIGINT, syscall.SIGTERM)
	}
	host.fsop.Init()
	return
}

func hostDestroy(user_data unsafe.Pointer) {
	defer func() {
		recover()
	}()
	if "netbsd" == runtime.GOOS {
		user_data = c_fuse_get_context().private_data
	}
	host := hostHandleGet(user_data)
	host.fsop.Destroy()
	if nil != host.sigc {
		signal.Stop(host.sigc)
	}
	host.fuse = nil
}

func hostAccess(path0 *c_char, mask0 c_int) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	errc := fsop.Access(path, uint32(mask0))
	return c_int(errc)
}

func hostCreate(path0 *c_char, mode0 c_fuse_mode_t, fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	intf, ok := fsop.(FileSystemOpenEx)
	if ok {
		fi := FileInfo_t{Flags: int(fi0.flags)}
		errc := intf.CreateEx(path, uint32(mode0), &fi)
		if -ENOSYS == errc {
			errc = fsop.Mknod(path, S_IFREG|uint32(mode0), 0)
			if 0 == errc {
				errc = intf.OpenEx(path, &fi)
			}
		}
		c_hostAsgnCfileinfo(fi0,
			c_bool(fi.DirectIo),
			c_bool(fi.KeepCache),
			c_bool(fi.NonSeekable),
			c_uint64_t(fi.Fh))
		return c_int(errc)
	} else {
		errc, rslt := fsop.Create(path, int(fi0.flags), uint32(mode0))
		if -ENOSYS == errc {
			errc = fsop.Mknod(path, S_IFREG|uint32(mode0), 0)
			if 0 == errc {
				errc, rslt = fsop.Open(path, int(fi0.flags))
			}
		}
		fi0.fh = c_uint64_t(rslt)
		return c_int(errc)
	}
}

func hostFtruncate(path0 *c_char, size0 c_fuse_off_t, fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	errc := fsop.Truncate(path, int64(size0), uint64(fi0.fh))
	return c_int(errc)
}

func hostFgetattr(path0 *c_char, stat0 *c_fuse_stat_t,
	fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	stat := &Stat_t{}
	errc := fsop.Getattr(path, stat, uint64(fi0.fh))
	copyCstatFromFusestat(stat0, stat)
	return c_int(errc)
}

func hostUtimens(path0 *c_char, tmsp0 *c_fuse_timespec_t) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	if nil == tmsp0 {
		errc := fsop.Utimens(path, nil)
		return c_int(errc)
	} else {
		tmsp := [2]Timespec{}
		tmsa := (*[2]c_fuse_timespec_t)(unsafe.Pointer(tmsp0))
		copyFusetimespecFromCtimespec(&tmsp[0], &tmsa[0])
		copyFusetimespecFromCtimespec(&tmsp[1], &tmsa[1])
		errc := fsop.Utimens(path, tmsp[:])
		return c_int(errc)
	}
}

func hostSetchgtime(path0 *c_char, tmsp0 *c_fuse_timespec_t) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	intf, ok := fsop.(FileSystemSetchgtime)
	if !ok {
		// say we did it!
		return 0
	}
	path := c_GoString(path0)
	tmsp := Timespec{}
	copyFusetimespecFromCtimespec(&tmsp, tmsp0)
	errc := intf.Setchgtime(path, tmsp)
	return c_int(errc)
}

func hostSetcrtime(path0 *c_char, tmsp0 *c_fuse_timespec_t) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	intf, ok := fsop.(FileSystemSetcrtime)
	if !ok {
		// say we did it!
		return 0
	}
	path := c_GoString(path0)
	tmsp := Timespec{}
	copyFusetimespecFromCtimespec(&tmsp, tmsp0)
	errc := intf.Setcrtime(path, tmsp)
	return c_int(errc)
}

func hostChflags(path0 *c_char, flags c_uint32_t) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	intf, ok := fsop.(FileSystemChflags)
	if !ok {
		// say we did it!
		return 0
	}
	path := c_GoString(path0)
	errc := intf.Chflags(path, uint32(flags))
	return c_int(errc)
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
func (host *FileSystemHost) Mount(mountpoint string, opts []string) bool {
	if 0 == c_hostFuseInit() {
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
	argv := make([]*c_char, argc+1)
	argv[0] = c_CString(exec)
	defer c_free(unsafe.Pointer(argv[0]))
	opti := 1
	if "" != mountpoint {
		argv[1] = c_CString(mountpoint)
		defer c_free(unsafe.Pointer(argv[1]))
		opti++
	}
	argv[opti] = c_CString("-f")
	defer c_free(unsafe.Pointer(argv[opti]))
	opti++
	for i := 0; len(opts) > i; i++ {
		argv[i+opti] = c_CString(opts[i])
		defer c_free(unsafe.Pointer(argv[i+opti]))
	}

	/*
	 * Mountpoint extraction
	 *
	 * We need to determine the mountpoint that FUSE is going (to try) to use, so that we
	 * can unmount later.
	 */
	if "" != mountpoint {
		host.mntp = mountpoint
	} else {
		outargs, _ := OptParse(opts, "")
		if 1 <= len(outargs) {
			host.mntp = outargs[0]
		}
	}
	if "" != host.mntp {
		if "windows" != runtime.GOOS || 2 != len(host.mntp) || ':' != host.mntp[1] {
			abs, err := filepath.Abs(host.mntp)
			if nil == err {
				host.mntp = abs
			}
		}
	}
	defer func() {
		host.mntp = ""
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
	return 0 != c_hostMount(c_int(argc), &argv[0], hndl)
}

// Unmount unmounts a mounted file system.
// Unmount may be called at any time after the Init() method has been called
// and before the Destroy() method has been called.
func (host *FileSystemHost) Unmount() bool {
	if nil == host.fuse {
		return false
	}
	var mntp *c_char
	if "" != host.mntp {
		mntp = c_CString(host.mntp)
	}
	return 0 != c_hostUnmount(host.fuse, mntp)
}

// Getcontext gets information related to a file system operation.
func Getcontext() (uid uint32, gid uint32, pid int) {
	context := c_fuse_get_context()
	uid = uint32(context.uid)
	gid = uint32(context.gid)
	pid = int(context.pid)
	return
}

func optNormBool(opt string) string {
	if i := strings.Index(opt, "=%"); -1 != i {
		switch opt[i+2:] {
		case "d", "o", "x", "X":
			return opt
		case "v":
			return opt[:i+1]
		default:
			panic("unknown format " + opt[i+1:])
		}
	} else {
		return opt
	}
}

func optNormInt(opt string, modf string) string {
	if i := strings.Index(opt, "=%"); -1 != i {
		switch opt[i+2:] {
		case "d", "o", "x", "X":
			return opt[:i+2] + modf + opt[i+2:]
		case "v":
			return opt[:i+2] + modf + "i"
		default:
			panic("unknown format " + opt[i+1:])
		}
	} else if strings.HasSuffix(opt, "=") {
		return opt + "%" + modf + "i"
	} else {
		return opt + "=%" + modf + "i"
	}
}

func optNormStr(opt string) string {
	if i := strings.Index(opt, "=%"); -1 != i {
		switch opt[i+2:] {
		case "s", "v":
			return opt[:i+2] + "s"
		default:
			panic("unknown format " + opt[i+1:])
		}
	} else if strings.HasSuffix(opt, "=") {
		return opt + "%s"
	} else {
		return opt + "=%s"
	}
}

// OptParse parses the FUSE command line arguments in args as determined by format
// and stores the resulting values in vals, which must be pointers. It returns a
// list of unparsed arguments or nil if an error happens.
//
// The format may be empty or non-empty. An empty format is taken as a special
// instruction to OptParse to only return all non-option arguments in outargs.
//
// A non-empty format is a space separated list of acceptable FUSE options. Each
// option is matched with a corresponding pointer value in vals. The combination
// of the option and the type of the corresponding pointer value, determines how
// the option is used. The allowed pointer types are pointer to bool, pointer to
// an integer type and pointer to string.
//
// For pointer to bool types:
//
//     -x                       Match -x without parameter.
//     -foo --foo               As above for -foo or --foo.
//     foo                      Match "-o foo".
//     -x= -foo= --foo= foo=    Match option with parameter.
//     -x=%VERB ... foo=%VERB   Match option with parameter of syntax.
//                              Allowed verbs: d,o,x,X,v
//                              - d,o,x,X: set to true if parameter non-0.
//                              - v: set to true if parameter present.
//
//     The formats -x=, and -x=%v are equivalent.
//
// For pointer to other types:
//
//     -x                       Match -x with parameter (-x=PARAM).
//     -foo --foo               As above for -foo or --foo.
//     foo                      Match "-o foo=PARAM".
//     -x= -foo= --foo= foo=    Match option with parameter.
//     -x=%VERB ... foo=%VERB   Match option with parameter of syntax.
//                              Allowed verbs for pointer to int types: d,o,x,X,v
//                              Allowed verbs for pointer to string types: s,v
//
//     The formats -x, -x=, and -x=%v are equivalent.
//
// For example:
//
//     var f bool
//     var set_attr_timeout bool
//     var attr_timeout int
//     var umask uint32
//     outargs, err := OptParse(args, "-f attr_timeout= attr_timeout umask=%o",
//         &f, &set_attr_timeout, &attr_timeout, &umask)
//
// Will accept a command line of:
//
//     $ program -f -o attr_timeout=42,umask=077
//
// And will set variables as follows:
//
//     f == true
//     set_attr_timeout == true
//     attr_timeout == 42
//     umask == 077
//
func OptParse(args []string, format string, vals ...interface{}) (outargs []string, err error) {
	if 0 == c_hostFuseInit() {
		panic("cgofuse: cannot find winfsp")
	}

	defer func() {
		if r := recover(); nil != r {
			if s, ok := r.(string); ok {
				outargs = nil
				err = errors.New("OptParse: " + s)
			} else {
				panic(r)
			}
		}
	}()

	var opts []string
	var nonopts bool
	if "" == format {
		opts = make([]string, 0)
		nonopts = true
	} else {
		opts = strings.Split(format, " ")
	}

	align := int(2 * unsafe.Sizeof(c_size_t(0))) // match malloc alignment (usually 8 or 16)

	fuse_opts := make([]c_struct_fuse_opt, len(opts)+1)
	for i := 0; len(opts) > i; i++ {
		var templ *c_char
		switch vals[i].(type) {
		case *bool:
			templ = c_CString(optNormBool(opts[i]))
		case *int:
			templ = c_CString(optNormInt(opts[i], ""))
		case *int8:
			templ = c_CString(optNormInt(opts[i], "hh"))
		case *int16:
			templ = c_CString(optNormInt(opts[i], "h"))
		case *int32:
			templ = c_CString(optNormInt(opts[i], ""))
		case *int64:
			templ = c_CString(optNormInt(opts[i], "ll"))
		case *uint:
			templ = c_CString(optNormInt(opts[i], ""))
		case *uint8:
			templ = c_CString(optNormInt(opts[i], "hh"))
		case *uint16:
			templ = c_CString(optNormInt(opts[i], "h"))
		case *uint32:
			templ = c_CString(optNormInt(opts[i], ""))
		case *uint64:
			templ = c_CString(optNormInt(opts[i], "ll"))
		case *uintptr:
			templ = c_CString(optNormInt(opts[i], "ll"))
		case *string:
			templ = c_CString(optNormStr(opts[i]))
		}
		defer c_free(unsafe.Pointer(templ))

		c_hostOptSet(&fuse_opts[i], templ, c_fuse_opt_offset_t(i*align), 1)
	}

	fuse_args := c_struct_fuse_args{}
	defer c_fuse_opt_free_args(&fuse_args)
	argc := 1 + len(args)
	argp := c_calloc(c_size_t(argc+1), c_size_t(unsafe.Sizeof((*c_char)(nil))))
	argv := (*[1 << 16]*c_char)(argp)
	argv[0] = c_CString("<UNKNOWN>")
	for i := 0; len(args) > i; i++ {
		argv[1+i] = c_CString(args[i])
	}
	fuse_args.allocated = 1
	fuse_args.argc = c_int(argc)
	fuse_args.argv = (**c_char)(&argv[0])

	data := c_calloc(c_size_t(len(opts)), c_size_t(align))
	defer c_free(data)

	if -1 == c_hostOptParse(&fuse_args, data, &fuse_opts[0], c_bool(nonopts)) {
		panic("failed")
	}

	for i := 0; len(opts) > i; i++ {
		switch v := vals[i].(type) {
		case *bool:
			*v = 0 != int(*(*c_int)(unsafe.Pointer(uintptr(data) + uintptr(i*align))))
		case *int:
			*v = int(*(*c_int)(unsafe.Pointer(uintptr(data) + uintptr(i*align))))
		case *int8:
			*v = int8(*(*c_int8_t)(unsafe.Pointer(uintptr(data) + uintptr(i*align))))
		case *int16:
			*v = int16(*(*c_int16_t)(unsafe.Pointer(uintptr(data) + uintptr(i*align))))
		case *int32:
			*v = int32(*(*c_int32_t)(unsafe.Pointer(uintptr(data) + uintptr(i*align))))
		case *int64:
			*v = int64(*(*c_int64_t)(unsafe.Pointer(uintptr(data) + uintptr(i*align))))
		case *uint:
			*v = uint(*(*c_unsigned)(unsafe.Pointer(uintptr(data) + uintptr(i*align))))
		case *uint8:
			*v = uint8(*(*c_uint8_t)(unsafe.Pointer(uintptr(data) + uintptr(i*align))))
		case *uint16:
			*v = uint16(*(*c_uint16_t)(unsafe.Pointer(uintptr(data) + uintptr(i*align))))
		case *uint32:
			*v = uint32(*(*c_uint32_t)(unsafe.Pointer(uintptr(data) + uintptr(i*align))))
		case *uint64:
			*v = uint64(*(*c_uint64_t)(unsafe.Pointer(uintptr(data) + uintptr(i*align))))
		case *uintptr:
			*v = uintptr(*(*c_uintptr_t)(unsafe.Pointer(uintptr(data) + uintptr(i*align))))
		case *string:
			s := *(**c_char)(unsafe.Pointer(uintptr(data) + uintptr(i*align)))
			*v = c_GoString(s)
			c_free(unsafe.Pointer(s))
		}
	}

	if 1 >= fuse_args.argc {
		outargs = make([]string, 0)
	} else {
		outargs = make([]string, fuse_args.argc-1)
		for i := 1; int(fuse_args.argc) > i; i++ {
			outargs[i-1] = c_GoString((*[1 << 16]*c_char)(unsafe.Pointer(fuse_args.argv))[i])
		}
	}

	if nonopts && 1 <= len(outargs) && "--" == outargs[0] {
		outargs = outargs[1:]
	}

	return
}

func init() {
	c_hostStaticInit()
}
