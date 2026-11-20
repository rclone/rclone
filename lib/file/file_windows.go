//go:build windows

package file

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
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
	h, e := syscall.CreateFile(pathp, access, sharemode, nil, createmode, syscall.FILE_ATTRIBUTE_NORMAL|syscall.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if e != nil {
		return nil, e
	}
	return os.NewFile(uintptr(h), path), nil
}

// fileRenameInfo is the FILE_RENAME_INFO structure passed to
// SetFileInformationByHandle with FileRenameInfoEx.
//
// It is variable length: FileName holds the new name as UTF-16 and
// FileNameLength is its length in bytes.
//
// https://learn.microsoft.com/en-us/windows-hardware/drivers/ddi/ntifs/ns-ntifs-_file_rename_information
type fileRenameInfo struct {
	Flags          uint32
	RootDirectory  windows.Handle
	FileNameLength uint32
	FileName       [1]uint16
}

// renameEx renames oldpath to newpath using SetFileInformationByHandle
// with the POSIX semantics flags.
//
// Unlike os.Rename (which calls MoveFileEx) this can replace newpath when
// it has open handles, providing those handles were opened with
// FILE_SHARE_DELETE, which is how OpenFile opens files.
func renameEx(oldpath, newpath string) error {
	oldp, err := windows.UTF16PtrFromString(oldpath)
	if err != nil {
		return err
	}
	// The file must be opened with DELETE access for
	// SetFileInformationByHandle to rename it.
	handle, err := windows.CreateFile(oldp, windows.DELETE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return err
	}
	defer func() {
		_ = windows.CloseHandle(handle)
	}()

	// Build the variable length FILE_RENAME_INFO. The struct already
	// contains one UTF-16 character so account for that when sizing.
	name := utf16.Encode([]rune(newpath))
	buffer := make([]byte, int(unsafe.Sizeof(fileRenameInfo{}))-2+len(name)*2)
	info := (*fileRenameInfo)(unsafe.Pointer(&buffer[0]))
	info.Flags = windows.FILE_RENAME_REPLACE_IF_EXISTS | windows.FILE_RENAME_POSIX_SEMANTICS
	info.RootDirectory = 0
	info.FileNameLength = uint32(len(name) * 2)
	copy(unsafe.Slice(&info.FileName[0], len(name)), name)
	return windows.SetFileInformationByHandle(handle, windows.FileRenameInfoEx, &buffer[0], uint32(len(buffer)))
}

// Rename renames (moves) oldpath to newpath, replacing newpath if it
// exists.
//
// This is the same as os.Rename on Unix. On Windows os.Rename uses
// MoveFileEx which fails with ERROR_ACCESS_DENIED when either path has
// open handles, even when those handles were opened with
// FILE_SHARE_DELETE, and the VFS cache deliberately keeps cache files
// open (see OpenFile and issue #9943). That makes the common editor
// pattern of writing a temporary file and renaming it over the target
// fail.
//
// So on Windows this renames with SetFileInformationByHandle and
// FileRenameInfoEx first, which can replace an open destination, and
// falls back to os.Rename when that is not supported (eg Windows before
// 10 1607).
func Rename(oldpath, newpath string) error {
	err := renameEx(oldpath, newpath)
	if err == nil {
		return nil
	}
	errOld := os.Rename(oldpath, newpath)
	if errOld == nil {
		return nil
	}
	return fmt.Errorf("%w (rename with FileRenameInfoEx failed with: %v)", errOld, err)
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
