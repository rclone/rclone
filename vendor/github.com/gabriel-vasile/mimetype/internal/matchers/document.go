package matchers

import "bytes"

// Pdf matches a Portable Document Format file.
func Pdf(in []byte) bool {
	return bytes.HasPrefix(in, []byte{0x25, 0x50, 0x44, 0x46})
}

// DjVu matches a DjVu file.
func DjVu(in []byte) bool {
	if len(in) < 12 {
		return false
	}
	if !bytes.HasPrefix(in, []byte{0x41, 0x54, 0x26, 0x54, 0x46, 0x4F, 0x52, 0x4D}) {
		return false
	}
	return bytes.HasPrefix(in[12:], []byte("DJVM")) ||
		bytes.HasPrefix(in[12:], []byte("DJVU")) ||
		bytes.HasPrefix(in[12:], []byte("DJVI")) ||
		bytes.HasPrefix(in[12:], []byte("THUM"))
}

// Mobi matches a Mobi file.
func Mobi(in []byte) bool {
	return len(in) > 67 && bytes.Equal(in[60:68], []byte("BOOKMOBI"))
}

// Lit matches a Microsoft Lit file.
func Lit(in []byte) bool {
	return bytes.HasPrefix(in, []byte("ITOLITLS"))
}
