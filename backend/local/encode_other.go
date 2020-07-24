//+build !windows,!darwin

package local

import "github.com/rclone/rclone/lib/encoder"

// This is the encoding used by the local backend for non windows platforms
const defaultEnc = encoder.Base
