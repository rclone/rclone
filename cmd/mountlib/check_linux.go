//go:build linux
// +build linux

package mountlib

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/artyom/mtab"
)

const (
	mtabPath     = "/proc/mounts"
	pollInterval = 100 * time.Millisecond
)

// CheckMountEmpty checks if folder is not already a mountpoint.
// On Linux we use the OS-specific /proc/mount API so the check won't access the path.
// Directories marked as "mounted" by autofs are considered not mounted.
func CheckMountEmpty(mountpoint string) error {
	const msg = "directory already mounted, use --allow-non-empty to mount anyway: %s"

	mountpointAbs, err := filepath.Abs(mountpoint)
	if err != nil {
		return fmt.Errorf("cannot get absolute path: %s: %w", mountpoint, err)
	}

	entries, err := mtab.Entries(mtabPath)
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", mtabPath, err)
	}
	for _, entry := range entries {
		if entry.Dir == mountpointAbs && entry.Type != "autofs" {
			return fmt.Errorf(msg, mountpointAbs)
		}
	}
	return nil
}

// CheckMountReady checks whether mountpoint is mounted by rclone.
// Only mounts with type "rclone" or "fuse.rclone" count.
func CheckMountReady(mountpoint string) error {
	mountpointAbs, err := filepath.Abs(mountpoint)
	if err != nil {
		return fmt.Errorf("cannot get absolute path: %s: %w", mountpoint, err)
	}
	entries, err := mtab.Entries(mtabPath)
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", mtabPath, err)
	}
	for _, entry := range entries {
		if entry.Dir == mountpointAbs && strings.Contains(entry.Type, "rclone") {
			return nil
		}
	}
	return errors.New("mount not ready")
}

// WaitMountReady waits until mountpoint is mounted by rclone.
func WaitMountReady(mountpoint string, timeout time.Duration) (err error) {
	endTime := time.Now().Add(timeout)
	for {
		err = CheckMountReady(mountpoint)
		delay := time.Until(endTime)
		if err == nil || delay <= 0 {
			break
		}
		if delay > pollInterval {
			delay = pollInterval
		}
		time.Sleep(delay)
	}
	return
}
