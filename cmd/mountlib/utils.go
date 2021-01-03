package mountlib

import (
	"io"
	"os"
	"runtime"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
)

// CheckMountEmpty checks if folder is empty
func CheckMountEmpty(mountpoint string) error {
	fp, fpErr := os.Open(mountpoint)

	if fpErr != nil {
		return errors.Wrap(fpErr, "Can not open: "+mountpoint)
	}
	defer fs.CheckClose(fp, &fpErr)

	_, fpErr = fp.Readdirnames(1)

	if fpErr == io.EOF {
		return nil
	}

	msg := "Directory is not empty: " + mountpoint + " If you want to mount it anyway use: --allow-non-empty option"
	if fpErr == nil {
		return errors.New(msg)
	}
	return errors.Wrap(fpErr, msg)
}

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
