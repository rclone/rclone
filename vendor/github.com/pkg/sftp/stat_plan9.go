package sftp

import (
	"os"
	"syscall"
)

var EBADF = syscall.NewError("fd out of range or not open")

func wrapPathError(filepath string, err error) error {
	if errno, ok := err.(syscall.ErrorString); ok {
		return &os.PathError{Path: filepath, Err: errno}
	}
	return err
}

// translateErrno translates a syscall error number to a SFTP error code.
func translateErrno(errno syscall.ErrorString) uint32 {
	switch errno {
	case "":
		return sshFxOk
	case syscall.ENOENT:
		return sshFxNoSuchFile
	case syscall.EPERM:
		return sshFxPermissionDenied
	}

	return sshFxFailure
}

func translateSyscallError(err error) (uint32, bool) {
	switch e := err.(type) {
	case syscall.ErrorString:
		return translateErrno(e), true
	case *os.PathError:
		debug("statusFromError,pathError: error is %T %#v", e.Err, e.Err)
		if errno, ok := e.Err.(syscall.ErrorString); ok {
			return translateErrno(errno), true
		}
	}
	return 0, false
}

// isRegular returns true if the mode describes a regular file.
func isRegular(mode uint32) bool {
	return mode&S_IFMT == syscall.S_IFREG
}

// toFileMode converts sftp filemode bits to the os.FileMode specification
func toFileMode(mode uint32) os.FileMode {
	var fm = os.FileMode(mode & 0777)

	switch mode & S_IFMT {
	case syscall.S_IFBLK:
		fm |= os.ModeDevice
	case syscall.S_IFCHR:
		fm |= os.ModeDevice | os.ModeCharDevice
	case syscall.S_IFDIR:
		fm |= os.ModeDir
	case syscall.S_IFIFO:
		fm |= os.ModeNamedPipe
	case syscall.S_IFLNK:
		fm |= os.ModeSymlink
	case syscall.S_IFREG:
		// nothing to do
	case syscall.S_IFSOCK:
		fm |= os.ModeSocket
	}

	return fm
}

// fromFileMode converts from the os.FileMode specification to sftp filemode bits
func fromFileMode(mode os.FileMode) uint32 {
	ret := uint32(mode & os.ModePerm)

	switch mode & os.ModeType {
	case os.ModeDevice | os.ModeCharDevice:
		ret |= syscall.S_IFCHR
	case os.ModeDevice:
		ret |= syscall.S_IFBLK
	case os.ModeDir:
		ret |= syscall.S_IFDIR
	case os.ModeNamedPipe:
		ret |= syscall.S_IFIFO
	case os.ModeSymlink:
		ret |= syscall.S_IFLNK
	case 0:
		ret |= syscall.S_IFREG
	case os.ModeSocket:
		ret |= syscall.S_IFSOCK
	}

	return ret
}

// Plan 9 doesn't have setuid, setgid or sticky, but a Plan 9 client should
// be able to send these bits to a POSIX server.
const (
	s_ISUID = 04000
	s_ISGID = 02000
	s_ISVTX = 01000
)
