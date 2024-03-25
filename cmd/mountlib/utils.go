package mountlib

import (
	"fmt"
	"io"
	"os"
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

// CheckOverlap checks that root doesn't overlap with a mountpoint
func CheckOverlap(f fs.Fs, mountpoint string) error {
	name := f.Name()
	if name != "" && name != "local" {
		return nil
	}
	rootAbs := absPath(f.Root())
	mountpointAbs := absPath(mountpoint)
	if strings.HasPrefix(rootAbs, mountpointAbs) || strings.HasPrefix(mountpointAbs, rootAbs) {
		const msg = "mount point %q (%q) and directory to be mounted %q (%q) mustn't overlap"
		return fmt.Errorf(msg, mountpoint, mountpointAbs, f.Root(), rootAbs)
	}
	return nil
}

// absPath is a helper function for CheckOverlap
func absPath(path string) string {
	if abs, err := filepath.EvalSymlinks(path); err == nil {
		path = abs
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	path = filepath.ToSlash(path)
	if runtime.GOOS == "windows" {
		// Removes any UNC long path prefix to make sure a simple HasPrefix test
		// in CheckOverlap works when one is UNC (root) and one is not (mountpoint).
		path = strings.TrimPrefix(path, `//?/`)
	}
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	return path
}

// CheckAllowNonEmpty checks --allow-non-empty flag, and if not used verifies that mountpoint is empty.
func CheckAllowNonEmpty(mountpoint string, opt *Options) error {
	if !opt.AllowNonEmpty {
		return CheckMountEmpty(mountpoint)
	}
	return nil
}

// checkMountEmpty checks if mountpoint folder is empty by listing it.
func checkMountEmpty(mountpoint string) error {
	fp, err := os.Open(mountpoint)
	if err != nil {
		return fmt.Errorf("cannot open: %s: %w", mountpoint, err)
	}
	defer fs.CheckClose(fp, &err)

	_, err = fp.Readdirnames(1)
	if err == io.EOF {
		return nil
	}

	const msg = "%q is not empty, use --allow-non-empty to mount anyway"
	if err == nil {
		return fmt.Errorf(msg, mountpoint)
	}
	return fmt.Errorf(msg+": %w", mountpoint, err)
}

// MakeVolumeNameValidOnUnix takes a volume name and returns a variant that is valid on unix systems.
func MakeVolumeNameValidOnUnix(volumeName string) string {
	volumeName = strings.ReplaceAll(volumeName, ":", " ")
	volumeName = strings.ReplaceAll(volumeName, "/", " ")
	volumeName = strings.TrimSpace(volumeName)
	return volumeName
}
