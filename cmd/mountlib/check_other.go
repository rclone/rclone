//go:build !linux
// +build !linux

package mountlib

import (
	"time"
)

// CheckMountEmpty checks if mountpoint folder is empty.
// On non-Linux unixes we list directory to ensure that.
func CheckMountEmpty(mountpoint string) error {
	return checkMountEmpty(mountpoint)
}

// CheckMountReady should check if mountpoint is mounted by rclone.
// The check is implemented only for Linux so this does nothing.
func CheckMountReady(mountpoint string) error {
	return nil
}

// WaitMountReady should wait until mountpoint is mounted by rclone.
// The check is implemented only for Linux so we just sleep a little.
func WaitMountReady(mountpoint string, timeout time.Duration) error {
	time.Sleep(timeout)
	return nil
}
