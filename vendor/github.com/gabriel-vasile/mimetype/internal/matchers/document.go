package matchers

import "bytes"

// Pdf matches a Portable Document Format file.
func Pdf(in []byte) bool {
	return len(in) > 4 && bytes.Equal(in[:4], []byte{0x25, 0x50, 0x44, 0x46})
}
