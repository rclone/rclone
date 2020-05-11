// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package memory

// FormatBytes converts number of bytes to appropriately sized string
func FormatBytes(bytes int64) string {
	return Size(bytes).String()
}

// ParseString converts string to number of bytes
func ParseString(s string) (int64, error) {
	var size Size
	err := size.Set(s)
	return size.Int64(), err
}
