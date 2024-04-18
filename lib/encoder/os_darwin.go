//go:build darwin

package encoder

// OS is the encoding used by the local backend for macOS
//
// macOS can't store invalid UTF-8, it converts them into %XX encoding
const OS = (Base |
	EncodeInvalidUtf8)
