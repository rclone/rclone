//go:build windows && (amd64 || arm64)

package local

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

var (
	shell32          = syscall.NewLazyDLL("shell32.dll")
	shFileOperationW = shell32.NewProc("SHFileOperationW")
)

const (
	foDelete          = 3
	fofAllowUndo      = 0x0040
	fofNoConfirmation = 0x0010
	fofSilent         = 0x0004
	fofNoErrorUI      = 0x0400
)

// shFileOpStructW matches SHFILEOPSTRUCTW on 64 bit Windows where the
// struct uses natural alignment. On 32 bit Windows the struct is byte
// packed which can't be expressed in Go - hence the build constraint
// above and the fallback in trash_unsupported.go.
type shFileOpStructW struct {
	hwnd                  uintptr
	wFunc                 uint32
	pFrom                 *uint16
	pTo                   *uint16
	fFlags                uint16
	fAnyOperationsAborted int32
	hNameMappings         uintptr
	lpszProgressTitle     *uint16
}

// moveToTrash sends the file or directory to the Windows Recycle Bin
// using SHFileOperationW with FOF_ALLOWUNDO.
func moveToTrash(path string) error {
	// SHFileOperation does not accept \\?\ extended-length paths
	if strings.HasPrefix(path, `\\?\UNC\`) {
		path = `\\` + strings.TrimPrefix(path, `\\?\UNC\`)
	} else {
		path = strings.TrimPrefix(path, `\\?\`)
	}
	// pFrom must be double-NUL terminated
	p, err := syscall.UTF16FromString(path)
	if err != nil {
		return err
	}
	p = append(p, 0)
	op := shFileOpStructW{
		wFunc:  foDelete,
		pFrom:  &p[0],
		fFlags: fofAllowUndo | fofNoConfirmation | fofSilent | fofNoErrorUI,
	}
	ret, _, _ := shFileOperationW.Call(uintptr(unsafe.Pointer(&op)))
	if ret != 0 {
		return fmt.Errorf("failed to move %q to the recycle bin: SHFileOperation error %#x", path, ret)
	}
	if op.fAnyOperationsAborted != 0 {
		return fmt.Errorf("moving %q to the recycle bin was aborted", path)
	}
	return nil
}
