package times

import (
	"os"
	"syscall"
	"time"
	"unsafe"
)

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

const hasPlatformSpecificStat = true

func platformSpecficStat(name string) (Timespec, error) {
	if findProcErr != nil {
		return nil, findProcErr
	}

	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return statFile(syscall.Handle(f.Fd()))
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
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}
