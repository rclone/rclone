//go:build windows

package fserrors

import (
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/windows"
)

func TestIsErrNoSpaceWindows(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"syscall.ENOSPC", syscall.ENOSPC, true},
		{"ERROR_DISK_FULL", windows.ERROR_DISK_FULL, true},
		{"ERROR_HANDLE_DISK_FULL", windows.ERROR_HANDLE_DISK_FULL, true},
		{"openat PathError", &os.PathError{Op: "openat", Path: "file", Err: windows.ERROR_DISK_FULL}, true},
		{"mkdirat PathError", &os.PathError{Op: "mkdirat", Path: "dir", Err: windows.ERROR_DISK_FULL}, true},
		{"SyscallError", os.NewSyscallError("write", windows.ERROR_HANDLE_DISK_FULL), true},
		{"ERROR_ACCESS_DENIED", windows.ERROR_ACCESS_DENIED, false},
		{"access denied PathError", &os.PathError{Op: "openat", Path: "file", Err: windows.ERROR_ACCESS_DENIED}, false},
		{"nil", nil, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, IsErrNoSpace(test.err))
		})
	}
}
