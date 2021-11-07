//go:build cmount && cgo && !windows
// +build cmount,cgo,!windows

package cmount

import (
	"errors"
	"fmt"
	"os"

	"github.com/rclone/rclone/cmd/mountlib"
)

func getMountpoint(mountPath string, opt *mountlib.Options) (string, error) {
	fi, err := os.Stat(mountPath)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve mount path information: %w", err)
	}
	if !fi.IsDir() {
		return "", errors.New("mount path is not a directory")
	}
	return mountPath, nil
}
