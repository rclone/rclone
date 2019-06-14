package matchers

import "bytes"

// Woff matches a Web Open Font Format file.
func Woff(in []byte) bool {
	return len(in) > 4 && bytes.Equal(in[:4], []byte("wOFF"))
}

// Woff2 matches a Web Open Font Format version 2 file.
func Woff2(in []byte) bool {
	return len(in) > 4 && bytes.Equal(in[:4], []byte("wOF2"))
}
