package times

import (
	"os"
	"syscall"
	"time"
	"unsafe"
)

// Stat returns the Timespec for the given filename.
func Stat(name string) (Timespec, error) {
	ts, err := platformSpecficStat(name)
	if err == nil {
		return ts, err
	}

	return stat(name, os.Stat)
}

// Lstat returns the Timespec for the given filename, and does not follow Symlinks.
func Lstat(name string) (Timespec, error) {
	ts, err := platformSpecficLstat(name)
	if err == nil {
		return ts, err
	}

	return stat(name, os.Lstat)
}

type timespecEx struct {
	atime
	mtime
	ctime
	btime
}

// StatFile finds a Windows Timespec with ChangeTime.
func StatFile(file *os.File) (Timespec, error) {
	return statFile(syscall.Handle(file.Fd()))
}

func statFile(h syscall.Handle) (Timespec, error) {
	var fileInfo fileBasicInfo
	if err := getFileInformationByHandleEx(h, &fileInfo); err != nil {
		return nil, err
	}

	var t timespecEx
	t.atime.v = time.Unix(0, fileInfo.LastAccessTime.Nanoseconds())
	t.mtime.v = time.Unix(0, fileInfo.LastWriteTime.Nanoseconds())
	t.ctime.v = time.Unix(0, fileInfo.ChangeTime.Nanoseconds())
	t.btime.v = time.Unix(0, fileInfo.CreationTime.Nanoseconds())
	return t, nil
}

func platformSpecficLstat(name string) (Timespec, error) {
	if findProcErr != nil {
		return nil, findProcErr
	}

	isSym, err := isSymlink(name)
	if err != nil {
		return nil, err
	}

	var attrs = uint32(syscall.FILE_FLAG_BACKUP_SEMANTICS)
	if isSym {
		attrs |= syscall.FILE_FLAG_OPEN_REPARSE_POINT
	}

	return openHandleAndStat(name, attrs)
}

func isSymlink(name string) (bool, error) {
	fi, err := os.Lstat(name)
	if err != nil {
		return false, err
	}
	return fi.Mode()&os.ModeSymlink != 0, nil
}

func platformSpecficStat(name string) (Timespec, error) {
	if findProcErr != nil {
		return nil, findProcErr
	}

	return openHandleAndStat(name, syscall.FILE_FLAG_BACKUP_SEMANTICS)
}

func openHandleAndStat(name string, attrs uint32) (Timespec, error) {
	pathp, e := syscall.UTF16PtrFromString(name)
	if e != nil {
		return nil, e
	}
	h, e := syscall.CreateFile(pathp,
		syscall.FILE_WRITE_ATTRIBUTES, syscall.FILE_SHARE_WRITE, nil,
		syscall.OPEN_EXISTING, attrs, 0)
	if e != nil {
		return nil, e
	}
	defer syscall.Close(h)

	return statFile(h)
}

var (
	findProcErr                      error
	procGetFileInformationByHandleEx *syscall.Proc
)

func init() {
	var modkernel32 *syscall.DLL
	if modkernel32, findProcErr = syscall.LoadDLL("kernel32.dll"); findProcErr == nil {
		procGetFileInformationByHandleEx, findProcErr = modkernel32.FindProc("GetFileInformationByHandleEx")
	}
}

// fileBasicInfo holds the C++ data for FileTimes.
//
// https://msdn.microsoft.com/en-us/library/windows/desktop/aa364217(v=vs.85).aspx
type fileBasicInfo struct {
	CreationTime   syscall.Filetime
	LastAccessTime syscall.Filetime
	LastWriteTime  syscall.Filetime
	ChangeTime     syscall.Filetime
	FileAttributes uint32
	_              uint32 // padding
}

type fileInformationClass int

const (
	fileBasicInfoClass fileInformationClass = iota
)

func getFileInformationByHandleEx(handle syscall.Handle, data *fileBasicInfo) (err error) {
	if findProcErr != nil {
		return findProcErr
	}

	r1, _, e1 := syscall.Syscall6(procGetFileInformationByHandleEx.Addr(), 4, uintptr(handle), uintptr(fileBasicInfoClass), uintptr(unsafe.Pointer(data)), unsafe.Sizeof(*data), 0, 0)
	if r1 == 0 {
		err = syscall.EINVAL
		if e1 != 0 {
			err = error(e1)
		}
	}
	return
}
