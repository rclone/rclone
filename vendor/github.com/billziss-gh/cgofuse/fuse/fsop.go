/*
 * fsop.go
 *
 * Copyright 2017-2018 Bill Zissimopoulos
 */
/*
 * This file is part of Cgofuse.
 *
 * It is licensed under the MIT license. The full license text can be found
 * in the License.txt file at the root of this project.
 */

// Package fuse allows the creation of user mode file systems in Go.
//
// A user mode file system is a user mode process that receives file system operations
// from the OS FUSE layer and satisfies them in user mode. A user mode file system
// implements the interface FileSystemInterface either directly or by embedding a
// FileSystemBase struct which provides a default (empty) implementation of all methods
// in FileSystemInterface.
//
// In order to expose the user mode file system to the OS, the file system must be hosted
// (mounted) by a FileSystemHost. The FileSystemHost Mount() method is used for this
// purpose.
//
// A note on thread-safety: In general FUSE file systems are expected to protect their
// own data structures. Many FUSE implementations provide a -s command line option that
// when used, it instructs the FUSE implementation to serialize requests. This option
// can be passed to the FileSystemHost Mount() method, when the file system is mounted.
package fuse

import (
	"strconv"
	"sync"
	"time"
)

// Timespec contains a time as the UNIX time in seconds and nanoseconds.
// This structure is analogous to the POSIX struct timespec.
type Timespec struct {
	Sec  int64
	Nsec int64
}

// NewTimespec creates a Timespec from a time.Time.
func NewTimespec(t time.Time) Timespec {
	return Timespec{t.Unix(), int64(t.Nanosecond())}
}

// Now creates a Timespec that contains the current time.
func Now() Timespec {
	return NewTimespec(time.Now())
}

// Time returns the Timespec as a time.Time.
func (ts *Timespec) Time() time.Time {
	return time.Unix(ts.Sec, ts.Nsec)
}

// Statfs_t contains file system information.
// This structure is analogous to the POSIX struct statvfs (NOT struct statfs).
// Not all fields are honored by all FUSE implementations.
type Statfs_t struct {
	// File system block size.
	Bsize uint64

	// Fundamental file system block size.
	Frsize uint64

	// Total number of blocks on file system in units of Frsize.
	Blocks uint64

	// Total number of free blocks.
	Bfree uint64

	// Number of free blocks available to non-privileged process.
	Bavail uint64

	// Total number of file serial numbers.
	Files uint64

	// Total number of free file serial numbers.
	Ffree uint64

	// Number of file serial numbers available to non-privileged process.
	Favail uint64

	// File system ID. [IGNORED]
	Fsid uint64

	// Bit mask of Flag values. [IGNORED]
	Flag uint64

	// Maximum filename length.
	Namemax uint64
}

// Stat_t contains file metadata information.
// This structure is analogous to the POSIX struct stat.
// Not all fields are honored by all FUSE implementations.
type Stat_t struct {
	// Device ID of device containing file. [IGNORED]
	Dev uint64

	// File serial number. [IGNORED unless the use_ino mount option is given.]
	Ino uint64

	// Mode of file.
	Mode uint32

	// Number of hard links to the file.
	Nlink uint32

	// User ID of file.
	Uid uint32

	// Group ID of file.
	Gid uint32

	// Device ID (if file is character or block special).
	Rdev uint64

	// For regular files, the file size in bytes.
	// For symbolic links, the length in bytes of the
	// pathname contained in the symbolic link.
	Size int64

	// Last data access timestamp.
	Atim Timespec

	// Last data modification timestamp.
	Mtim Timespec

	// Last file status change timestamp.
	Ctim Timespec

	// A file system-specific preferred I/O block size for this object.
	Blksize int64

	// Number of blocks allocated for this object.
	Blocks int64

	// File creation (birth) timestamp. [OSX and Windows only]
	Birthtim Timespec

	// BSD flags (UF_*). [OSX and Windows only]
	Flags uint32
}

// FileInfo_t contains open file information.
// This structure is analogous to the FUSE struct fuse_file_info.
type FileInfo_t struct {
	// Open flags: a combination of the fuse.O_* constants.
	Flags int

	// Use direct I/O on this file. [IGNORED on Windows]
	DirectIo bool

	// Do not invalidate file cache. [IGNORED on Windows]
	KeepCache bool

	// File is not seekable. [IGNORED on Windows]
	NonSeekable bool

	// File handle.
	Fh uint64
}

/*
// Lock_t contains file locking information.
// This structure is analogous to the POSIX struct flock.
type Lock_t struct {
	// Type of lock; F_RDLCK, F_WRLCK, F_UNLCK.
	Type int16

	// Flag for starting offset.
	Whence int16

	// Relative offset in bytes.
	Start int64

	// Size; if 0 then until EOF.
	Len int64

	// Process ID of the process holding the lock
	Pid int
}
*/

// FileSystemInterface is the interface that a user mode file system must implement.
//
// The file system will receive an Init() call when the file system is created;
// the Init() call will happen prior to receiving any other file system calls.
// Note that there are no guarantees on the exact timing of when Init() is called.
// For example, it cannot be assumed that the file system is mounted at the time
// the Init() call is received.
//
// The file system will receive a Destroy() call when the file system is destroyed;
// the Destroy() call will always be the last call to be received by the file system.
// Note that depending on how the file system is terminated the file system may not
// receive the Destroy() call. For example, it will not receive the Destroy() call
// if the file system process is forcibly killed.
//
// Except for Init() and Destroy() all file system operations must return 0 on success
// or a FUSE error on failure. To return an error return the NEGATIVE value of a
// particular error.  For example, to report "file not found" return -fuse.ENOENT.
type FileSystemInterface interface {
	// Init is called when the file system is created.
	Init()

	// Destroy is called when the file system is destroyed.
	Destroy()

	// Statfs gets file system statistics.
	Statfs(path string, stat *Statfs_t) int

	// Mknod creates a file node.
	Mknod(path string, mode uint32, dev uint64) int

	// Mkdir creates a directory.
	Mkdir(path string, mode uint32) int

	// Unlink removes a file.
	Unlink(path string) int

	// Rmdir removes a directory.
	Rmdir(path string) int

	// Link creates a hard link to a file.
	Link(oldpath string, newpath string) int

	// Symlink creates a symbolic link.
	Symlink(target string, newpath string) int

	// Readlink reads the target of a symbolic link.
	Readlink(path string) (int, string)

	// Rename renames a file.
	Rename(oldpath string, newpath string) int

	// Chmod changes the permission bits of a file.
	Chmod(path string, mode uint32) int

	// Chown changes the owner and group of a file.
	Chown(path string, uid uint32, gid uint32) int

	// Utimens changes the access and modification times of a file.
	Utimens(path string, tmsp []Timespec) int

	// Access checks file access permissions.
	Access(path string, mask uint32) int

	// Create creates and opens a file.
	// The flags are a combination of the fuse.O_* constants.
	Create(path string, flags int, mode uint32) (int, uint64)

	// Open opens a file.
	// The flags are a combination of the fuse.O_* constants.
	Open(path string, flags int) (int, uint64)

	// Getattr gets file attributes.
	Getattr(path string, stat *Stat_t, fh uint64) int

	// Truncate changes the size of a file.
	Truncate(path string, size int64, fh uint64) int

	// Read reads data from a file.
	Read(path string, buff []byte, ofst int64, fh uint64) int

	// Write writes data to a file.
	Write(path string, buff []byte, ofst int64, fh uint64) int

	// Flush flushes cached file data.
	Flush(path string, fh uint64) int

	// Release closes an open file.
	Release(path string, fh uint64) int

	// Fsync synchronizes file contents.
	Fsync(path string, datasync bool, fh uint64) int

	// Lock performs a file locking operation.
	//Lock(path string, cmd int, lock *Lock_t, fh uint64) int

	// Opendir opens a directory.
	Opendir(path string) (int, uint64)

	// Readdir reads a directory.
	Readdir(path string,
		fill func(name string, stat *Stat_t, ofst int64) bool,
		ofst int64,
		fh uint64) int

	// Releasedir closes an open directory.
	Releasedir(path string, fh uint64) int

	// Fsyncdir synchronizes directory contents.
	Fsyncdir(path string, datasync bool, fh uint64) int

	// Setxattr sets extended attributes.
	Setxattr(path string, name string, value []byte, flags int) int

	// Getxattr gets extended attributes.
	Getxattr(path string, name string) (int, []byte)

	// Removexattr removes extended attributes.
	Removexattr(path string, name string) int

	// Listxattr lists extended attributes.
	Listxattr(path string, fill func(name string) bool) int
}

// FileSystemOpenEx is the interface that wraps the OpenEx and CreateEx methods.
//
// OpenEx and CreateEx are similar to Open and Create except that they allow
// direct manipulation of the FileInfo_t struct (which is analogous to the
// FUSE struct fuse_file_info).
type FileSystemOpenEx interface {
	CreateEx(path string, mode uint32, fi *FileInfo_t) int
	OpenEx(path string, fi *FileInfo_t) int
}

// FileSystemChflags is the interface that wraps the Chflags method.
//
// Chflags changes the BSD file flags (Windows file attributes). [OSX and Windows only]
type FileSystemChflags interface {
	Chflags(path string, flags uint32) int
}

// FileSystemSetcrtime is the interface that wraps the Setcrtime method.
//
// Setcrtime changes the file creation (birth) time. [OSX and Windows only]
type FileSystemSetcrtime interface {
	Setcrtime(path string, tmsp Timespec) int
}

// FileSystemSetchgtime is the interface that wraps the Setchgtime method.
//
// Setchgtime changes the file change (ctime) time. [OSX and Windows only]
type FileSystemSetchgtime interface {
	Setchgtime(path string, tmsp Timespec) int
}

// Error encapsulates a FUSE error code. In some rare circumstances it is useful
// to signal an error to the FUSE layer by boxing the error code using Error and
// calling panic(). The FUSE layer will recover and report the boxed error code
// to the OS.
type Error int

var errorStringMap map[Error]string
var errorStringOnce sync.Once

func (self Error) Error() string {
	errorStringOnce.Do(func() {
		errorStringMap = make(map[Error]string)
		for _, i := range errorStrings {
			errorStringMap[Error(-i.errc)] = i.errs
		}
	})

	if 0 <= self {
		return strconv.Itoa(int(self))
	} else {
		if errs, ok := errorStringMap[self]; ok {
			return "-fuse." + errs
		}
		return "fuse.Error(" + strconv.Itoa(int(self)) + ")"
	}
}

func (self Error) String() string {
	return self.Error()
}

func (self Error) GoString() string {
	return self.Error()
}

var _ error = (*Error)(nil)

// FileSystemBase provides default implementations of the methods in FileSystemInterface.
// The default implementations are either empty or return -ENOSYS to signal that the
// file system does not implement a particular operation to the FUSE layer.
type FileSystemBase struct {
}

// Init is called when the file system is created.
// The FileSystemBase implementation does nothing.
func (*FileSystemBase) Init() {
}

// Destroy is called when the file system is destroyed.
// The FileSystemBase implementation does nothing.
func (*FileSystemBase) Destroy() {
}

// Statfs gets file system statistics.
// The FileSystemBase implementation returns -ENOSYS.
func (*FileSystemBase) Statfs(path string, stat *Statfs_t) int {
	return -ENOSYS
}

// Mknod creates a file node.
// The FileSystemBase implementation returns -ENOSYS.
func (*FileSystemBase) Mknod(path string, mode uint32, dev uint64) int {
	return -ENOSYS
}

// Mkdir creates a directory.
// The FileSystemBase implementation returns -ENOSYS.
func (*FileSystemBase) Mkdir(path string, mode uint32) int {
	return -ENOSYS
}

// Unlink removes a file.
// The FileSystemBase implementation returns -ENOSYS.
func (*FileSystemBase) Unlink(path string) int {
	return -ENOSYS
}

// Rmdir removes a directory.
// The FileSystemBase implementation returns -ENOSYS.
func (*FileSystemBase) Rmdir(path string) int {
	return -ENOSYS
}

// Link creates a hard link to a file.
// The FileSystemBase implementation returns -ENOSYS.
func (*FileSystemBase) Link(oldpath string, newpath string) int {
	return -ENOSYS
}

// Symlink creates a symbolic link.
// The FileSystemBase implementation returns -ENOSYS.
func (*FileSystemBase) Symlink(target string, newpath string) int {
	return -ENOSYS
}

// Readlink reads the target of a symbolic link.
// The FileSystemBase implementation returns -ENOSYS.
func (*FileSystemBase) Readlink(path string) (int, string) {
	return -ENOSYS, ""
}

// Rename renames a file.
// The FileSystemBase implementation returns -ENOSYS.
func (*FileSystemBase) Rename(oldpath string, newpath string) int {
	return -ENOSYS
}

// Chmod changes the permission bits of a file.
// The FileSystemBase implementation returns -ENOSYS.
func (*FileSystemBase) Chmod(path string, mode uint32) int {
	return -ENOSYS
}

// Chown changes the owner and group of a file.
// The FileSystemBase implementation returns -ENOSYS.
func (*FileSystemBase) Chown(path string, uid uint32, gid uint32) int {
	return -ENOSYS
}

// Utimens changes the access and modification times of a file.
// The FileSystemBase implementation returns -ENOSYS.
func (*FileSystemBase) Utimens(path string, tmsp []Timespec) int {
	return -ENOSYS
}

// Access checks file access permissions.
// The FileSystemBase implementation returns -ENOSYS.
func (*FileSystemBase) Access(path string, mask uint32) int {
	return -ENOSYS
}

// Create creates and opens a file.
// The flags are a combination of the fuse.O_* constants.
// The FileSystemBase implementation returns -ENOSYS.
func (*FileSystemBase) Create(path string, flags int, mode uint32) (int, uint64) {
	return -ENOSYS, ^uint64(0)
}

// Open opens a file.
// The flags are a combination of the fuse.O_* constants.
// The FileSystemBase implementation returns -ENOSYS.
func (*FileSystemBase) Open(path string, flags int) (int, uint64) {
	return -ENOSYS, ^uint64(0)
}

// Getattr gets file attributes.
// The FileSystemBase implementation returns -ENOSYS.
func (*FileSystemBase) Getattr(path string, stat *Stat_t, fh uint64) int {
	return -ENOSYS
}

// Truncate changes the size of a file.
// The FileSystemBase implementation returns -ENOSYS.
func (*FileSystemBase) Truncate(path string, size int64, fh uint64) int {
	return -ENOSYS
}

// Read reads data from a file.
// The FileSystemBase implementation returns -ENOSYS.
func (*FileSystemBase) Read(path string, buff []byte, ofst int64, fh uint64) int {
	return -ENOSYS
}

// Write writes data to a file.
// The FileSystemBase implementation returns -ENOSYS.
func (*FileSystemBase) Write(path string, buff []byte, ofst int64, fh uint64) int {
	return -ENOSYS
}

// Flush flushes cached file data.
// The FileSystemBase implementation returns -ENOSYS.
func (*FileSystemBase) Flush(path string, fh uint64) int {
	return -ENOSYS
}

// Release closes an open file.
// The FileSystemBase implementation returns -ENOSYS.
func (*FileSystemBase) Release(path string, fh uint64) int {
	return -ENOSYS
}

// Fsync synchronizes file contents.
// The FileSystemBase implementation returns -ENOSYS.
func (*FileSystemBase) Fsync(path string, datasync bool, fh uint64) int {
	return -ENOSYS
}

/*
// Lock performs a file locking operation.
// The FileSystemBase implementation returns -ENOSYS.
func (*FileSystemBase) Lock(path string, cmd int, lock *Lock_t, fh uint64) int {
	return -ENOSYS
}
*/

// Opendir opens a directory.
// The FileSystemBase implementation returns -ENOSYS.
func (*FileSystemBase) Opendir(path string) (int, uint64) {
	return -ENOSYS, ^uint64(0)
}

// Readdir reads a directory.
// The FileSystemBase implementation returns -ENOSYS.
func (*FileSystemBase) Readdir(path string,
	fill func(name string, stat *Stat_t, ofst int64) bool,
	ofst int64,
	fh uint64) int {
	return -ENOSYS
}

// Releasedir closes an open directory.
// The FileSystemBase implementation returns -ENOSYS.
func (*FileSystemBase) Releasedir(path string, fh uint64) int {
	return -ENOSYS
}

// Fsyncdir synchronizes directory contents.
// The FileSystemBase implementation returns -ENOSYS.
func (*FileSystemBase) Fsyncdir(path string, datasync bool, fh uint64) int {
	return -ENOSYS
}

// Setxattr sets extended attributes.
// The FileSystemBase implementation returns -ENOSYS.
func (*FileSystemBase) Setxattr(path string, name string, value []byte, flags int) int {
	return -ENOSYS
}

// Getxattr gets extended attributes.
// The FileSystemBase implementation returns -ENOSYS.
func (*FileSystemBase) Getxattr(path string, name string) (int, []byte) {
	return -ENOSYS, nil
}

// Removexattr removes extended attributes.
// The FileSystemBase implementation returns -ENOSYS.
func (*FileSystemBase) Removexattr(path string, name string) int {
	return -ENOSYS
}

// Listxattr lists extended attributes.
// The FileSystemBase implementation returns -ENOSYS.
func (*FileSystemBase) Listxattr(path string, fill func(name string) bool) int {
	return -ENOSYS
}

var _ FileSystemInterface = (*FileSystemBase)(nil)
