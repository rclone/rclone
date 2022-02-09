package mountlib

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/rclone/rclone/fs"
)

// ClipBlocks clips the blocks pointed to the OS max
func ClipBlocks(b *uint64) {
	var max uint64
	switch runtime.GOOS {
	case "windows":
		if runtime.GOARCH == "386" {
			max = (1 << 32) - 1
		} else {
			max = (1 << 43) - 1
		}
	case "darwin":
		// OSX FUSE only supports 32 bit number of blocks
		// https://github.com/osxfuse/osxfuse/issues/396
		max = (1 << 32) - 1
	default:
		// no clipping
		return
	}
	if *b > max {
		*b = max
	}
}

// CheckOverlap checks that root doesn't overlap with mountpoint
func (m *MountPoint) CheckOverlap() error {
	name := m.Fs.Name()
	if name != "" && name != "local" {
		return nil
	}
	rootAbs := absPath(m.Fs.Root())
	mountpointAbs := absPath(m.MountPoint)
	if strings.HasPrefix(rootAbs, mountpointAbs) || strings.HasPrefix(mountpointAbs, rootAbs) {
		const msg = "mount point %q and directory to be mounted %q mustn't overlap"
		return fmt.Errorf(msg, m.MountPoint, m.Fs.Root())
	}
	return nil
}

// absPath is a helper function for MountPoint.CheckOverlap
func absPath(path string) string {
	if abs, err := filepath.EvalSymlinks(path); err == nil {
		path = abs
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	path = filepath.ToSlash(path)
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	return path
}

// CheckAllowings informs about ignored flags on Windows. If not on Windows
// and not --allow-non-empty flag is used, verify that mountpoint is empty.
func (m *MountPoint) CheckAllowings() error {
	opt := &m.MountOpt
	if runtime.GOOS == "windows" {
		if opt.AllowNonEmpty {
			fs.Logf(nil, "--allow-non-empty flag does nothing on Windows")
		}
		if opt.AllowRoot {
			fs.Logf(nil, "--allow-root flag does nothing on Windows")
		}
		if opt.AllowOther {
			fs.Logf(nil, "--allow-other flag does nothing on Windows")
		}
		return nil
	}
	if !opt.AllowNonEmpty {
		return CheckMountEmpty(m.MountPoint)
	}
	return nil
}

// SetVolumeName with sensible default
func (m *MountPoint) SetVolumeName(vol string) {
	if vol == "" {
		vol = fs.ConfigString(m.Fs)
	}
	m.MountOpt.SetVolumeName(vol)
}

// SetVolumeName removes special characters from volume name if necessary
func (o *Options) SetVolumeName(vol string) {
	vol = strings.ReplaceAll(vol, ":", " ")
	vol = strings.ReplaceAll(vol, "/", " ")
	vol = strings.TrimSpace(vol)
	if runtime.GOOS == "windows" && len(vol) > 32 {
		vol = vol[:32]
	}
	o.VolumeName = vol
}

// SetDeviceName with sensible default
func (m *MountPoint) SetDeviceName(dev string) {
	if dev == "" {
		dev = fs.ConfigString(m.Fs)
	}
	m.MountOpt.DeviceName = dev
}
