//go:build windows

package fserrors

import (
	"golang.org/x/sys/windows"
)

func init() {
	// Windows does not return syscall.ENOSPC, which Go defines here as a
	// value in its application reserved range. A full disk reports these.
	// https://learn.microsoft.com/en-us/windows/win32/debug/system-error-codes--0-499-
	noSpaceErrors = append(noSpaceErrors,
		windows.ERROR_DISK_FULL,
		windows.ERROR_HANDLE_DISK_FULL,
	)
}
