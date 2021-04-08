//+build windows

package file

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"syscall"
)

// OpenFile is the generalized open call; most users will use Open or Create
// instead. It opens the named file with specified flag (O_RDONLY etc.) and
// perm (before umask), if applicable. If successful, methods on the returned
// File can be used for I/O. If there is an error, it will be of type
// *PathError.
//
// Under both Unix and Windows this will allow open files to be
// renamed and or deleted.
func OpenFile(path string, mode int, perm os.FileMode) (*os.File, error) {
	// This code copied from syscall_windows.go in the go source and then
	// modified to support renaming and deleting open files by adding
	// FILE_SHARE_DELETE.
	//
	// https://docs.microsoft.com/en-us/windows/desktop/api/fileapi/nf-fileapi-createfilea#file_share_delete
	if len(path) == 0 {
		return nil, syscall.ERROR_FILE_NOT_FOUND
	}
	pathp, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	var access uint32
	switch mode & (syscall.O_RDONLY | syscall.O_WRONLY | syscall.O_RDWR) {
	case syscall.O_RDONLY:
		access = syscall.GENERIC_READ
	case syscall.O_WRONLY:
		access = syscall.GENERIC_WRITE
	case syscall.O_RDWR:
		access = syscall.GENERIC_READ | syscall.GENERIC_WRITE
	}
	if mode&syscall.O_CREAT != 0 {
		access |= syscall.GENERIC_WRITE
	}
	if mode&syscall.O_APPEND != 0 {
		access &^= syscall.GENERIC_WRITE
		access |= syscall.FILE_APPEND_DATA
	}
	sharemode := uint32(syscall.FILE_SHARE_READ | syscall.FILE_SHARE_WRITE | syscall.FILE_SHARE_DELETE)
	var createmode uint32
	switch {
	case mode&(syscall.O_CREAT|syscall.O_EXCL) == (syscall.O_CREAT | syscall.O_EXCL):
		createmode = syscall.CREATE_NEW
	case mode&(syscall.O_CREAT|syscall.O_TRUNC) == (syscall.O_CREAT | syscall.O_TRUNC):
		createmode = syscall.CREATE_ALWAYS
	case mode&syscall.O_CREAT == syscall.O_CREAT:
		createmode = syscall.OPEN_ALWAYS
	case mode&syscall.O_TRUNC == syscall.O_TRUNC:
		createmode = syscall.TRUNCATE_EXISTING
	default:
		createmode = syscall.OPEN_EXISTING
	}
	h, e := syscall.CreateFile(pathp, access, sharemode, nil, createmode, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if e != nil {
		return nil, e
	}
	return os.NewFile(uintptr(h), path), nil
}

// IsReserved checks if path contains a reserved name
func IsReserved(path string) error {
	if path == "" {
		return errors.New("path is empty")
	}
	base := filepath.Base(path)
	// If the path is empty or reduces to ".", Base returns ".".
	if base == "." {
		return errors.New("path is '.'")
	}
	// If the path consists entirely of separators, Base returns a single separator.
	if base == string(filepath.Separator) {
		return errors.New("path consists entirely of separators")
	}
	// Do not end a file or directory name with a space or a period. Although the underlying
	// file system may support such names, the Windows shell and user interface does not.
	// (https://docs.microsoft.com/en-gb/windows/win32/fileio/naming-a-file)
	suffix := base[len(base)-1]
	switch suffix {
	case ' ':
		return errors.New("base file name ends with a space")
	case '.':
		return errors.New("base file name ends with a period")
	}
	// Do not use names of legacy (DOS) devices, not even as basename without extension,
	// as this will refer to the actual device.
	// (https://docs.microsoft.com/en-gb/windows/win32/fileio/naming-a-file)
	if reserved, _ := regexp.MatchString(`^(?i:con|prn|aux|nul|com[1-9]|lpt[1-9])(?:\.|$)`, base); reserved {
		return errors.New("base file name is reserved windows device name (CON, PRN, AUX, NUL, COM[1-9], LPT[1-9])")
	}
	return nil
}
