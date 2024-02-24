//go:build !linux
// +build !linux

package mountlib

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

// CanCheckMountReady is set if CheckMountReady is functional
var CanCheckMountReady = false
