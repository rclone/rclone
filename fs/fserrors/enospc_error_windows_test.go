//go:build windows

package fserrors

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

func TestIsErrNoSpaceRealWindowsError(t *testing.T) {
	dir := t.TempDir()
	dirp, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		t.Skipf("cannot convert the temporary directory path: %v", err)
	}
	var available, total, free uint64
	if err := windows.GetDiskFreeSpaceEx(dirp, &available, &total, &free); err != nil {
		t.Skipf("cannot read the free space of the temporary directory: %v", err)
	}

	f, err := os.Create(filepath.Join(dir, "truncate"))
	require.NoError(t, err)
	defer func() { require.NoError(t, f.Close()) }()

	err = f.Truncate(int64(free + 1<<30))
	if err == nil {
		t.Skip("real Windows disk-full error coverage lost: volume did not enforce the free-space limit when truncating the file")
	}
	assert.True(t, IsErrNoSpace(err), "error = %v", err)
}

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
