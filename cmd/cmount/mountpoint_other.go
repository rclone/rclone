//go:build cmount && cgo && !windows

package cmount

import (
	"errors"
	"fmt"
	"os"

	"github.com/rclone/rclone/cmd/mountlib"
	"github.com/rclone/rclone/fs"
)

func getMountpoint(f fs.Fs, mountPath string, opt *mountlib.Options) (string, error) {
	fi, err := os.Stat(mountPath)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve mount path information: %w", err)
	}
	if !fi.IsDir() {
		return "", errors.New("mount path is not a directory")
	}
	if err = mountlib.CheckOverlap(f, mountPath); err != nil {
		return "", err
	}
	if err = mountlib.CheckAllowNonEmpty(mountPath, opt); err != nil {
		return "", err
	}
	return mountPath, nil
}
