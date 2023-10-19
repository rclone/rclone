//go:build linux
// +build linux

package mountlib

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/moby/sys/mountinfo"
)

const (
	pollInterval = 100 * time.Millisecond
)

// CheckMountEmpty checks if folder is not already a mountpoint.
// On Linux we use the OS-specific /proc/self/mountinfo API so the check won't access the path.
// Directories marked as "mounted" by autofs are considered not mounted.
func CheckMountEmpty(mountpoint string) error {
	const msg = "directory already mounted, use --allow-non-empty to mount anyway: %s"

	mountpointAbs, err := filepath.Abs(mountpoint)
	if err != nil {
		return fmt.Errorf("cannot get absolute path: %s: %w", mountpoint, err)
	}

	infos, err := mountinfo.GetMounts(mountinfo.SingleEntryFilter(mountpointAbs))
	if err != nil {
		return fmt.Errorf("cannot get mounts: %w", err)
	}

	foundAutofs := false
	for _, info := range infos {
		if info.FSType != "autofs" {
			return fmt.Errorf(msg, mountpointAbs)
		}
		foundAutofs = true
	}
	// It isn't safe to list an autofs in the middle of mounting
	if foundAutofs {
		return nil
	}

	return checkMountEmpty(mountpoint)
}

// singleEntryFilter looks for a specific entry.
//
// It may appear more than once and we return all of them if so.
func singleEntryFilter(mp string) mountinfo.FilterFunc {
	return func(m *mountinfo.Info) (skip, stop bool) {
		return m.Mountpoint != mp, false
	}
}

// CheckMountReady checks whether mountpoint is mounted by rclone.
// Only mounts with type "rclone" or "fuse.rclone" count.
func CheckMountReady(mountpoint string) error {
	const msg = "mount not ready: %s"

	mountpointAbs, err := filepath.Abs(mountpoint)
	if err != nil {
		return fmt.Errorf("cannot get absolute path: %s: %w", mountpoint, err)
	}

	infos, err := mountinfo.GetMounts(singleEntryFilter(mountpointAbs))
	if err != nil {
		return fmt.Errorf("cannot get mounts: %w", err)
	}

	for _, info := range infos {
		if strings.Contains(info.FSType, "rclone") {
			return nil
		}
	}

	return fmt.Errorf(msg, mountpointAbs)
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
