// +build cmount
// +build cgo
// +build !windows

package cmount

import (
	"os"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/cmd/mountlib"
)

func getMountpoint(mountPath string, opt *mountlib.Options) (string, error) {
	fi, err := os.Stat(mountPath)
	if err != nil {
		return "", errors.Wrap(err, "failed to retrieve mount path information")
	}
	if !fi.IsDir() {
		return "", errors.New("mount path is not a directory")
	}
	return mountPath, nil
}
