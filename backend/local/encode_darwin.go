//+build darwin

package local

import "github.com/rclone/rclone/lib/encoder"

// This is the encoding used by the local backend for macOS
//
// macOS can't store invalid UTF-8, it converts them into %XX encoding
const defaultEnc = (encoder.Base |
	encoder.EncodeInvalidUtf8)
