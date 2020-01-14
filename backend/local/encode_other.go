//+build !windows,!darwin

package local

import (
	"github.com/rclone/rclone/fs/encodings"
)

const defaultEnc = encodings.LocalUnix
