//go:build !linux
// +build !linux

package mountlib

import (
	"io"
	"os"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
)

// CheckMountEmpty checks if mountpoint folder is empty.
// On non-Linux unixes we list directory to ensure that.
func CheckMountEmpty(mountpoint string) error {
	fp, err := os.Open(mountpoint)
	if err != nil {
		return errors.Wrapf(err, "Can not open: %s", mountpoint)
	}
	defer fs.CheckClose(fp, &err)

	_, err = fp.Readdirnames(1)
	if err == io.EOF {
		return nil
	}

	const msg = "Directory is not empty, use --allow-non-empty to mount anyway: %s"
	if err == nil {
		return errors.Errorf(msg, mountpoint)
	}
	return errors.Wrapf(err, msg, mountpoint)
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
