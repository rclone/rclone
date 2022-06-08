//go:build !linux
// +build !linux

package mountlib

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/rclone/rclone/fs"
)

// CheckMountEmpty checks if mountpoint folder is empty.
// On non-Linux unixes we list directory to ensure that.
func CheckMountEmpty(mountpoint string) error {
	fp, err := os.Open(mountpoint)
	if err != nil {
		return fmt.Errorf("cannot open: %s: %w", mountpoint, err)
	}
	defer fs.CheckClose(fp, &err)

	_, err = fp.Readdirnames(1)
	if err == io.EOF {
		return nil
	}

	const msg = "directory is not empty, use --allow-non-empty to mount anyway: %s"
	if err == nil {
		return fmt.Errorf(msg, mountpoint)
	}
	return fmt.Errorf(msg+": %w", mountpoint, err)
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
