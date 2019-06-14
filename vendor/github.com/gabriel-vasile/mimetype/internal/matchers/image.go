package matchers

import "bytes"

// Png matches a Portable Network Graphics file.
func Png(in []byte) bool {
	return len(in) > 8 &&
		bytes.Equal(in[:8], []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})
}

// Jpg matches a Joint Photographic Experts Group file.
func Jpg(in []byte) bool {
	return len(in) > 3 && bytes.Equal(in[:3], []byte{0xFF, 0xD8, 0xFF})
}

// Gif matches a Graphics Interchange Format file.
func Gif(in []byte) bool {
	return bytes.HasPrefix(in, []byte("GIF87a")) ||
		bytes.HasPrefix(in, []byte("GIF89a"))
}

// Webp matches a WebP file.
func Webp(in []byte) bool {
	return len(in) > 12 &&
		bytes.Equal(in[0:4], []byte{0x52, 0x49, 0x46, 0x46}) &&
		bytes.Equal(in[8:12], []byte{0x57, 0x45, 0x42, 0x50})
}

// Bmp matches a bitmap image file.
func Bmp(in []byte) bool {
	return len(in) > 1 && in[0] == 0x42 && in[1] == 0x4D
}

// Ps matches a PostScript file.
func Ps(in []byte) bool {
	return bytes.HasPrefix(in, []byte("%!PS-Adobe-"))
}

// Psd matches a Photoshop Document file.
func Psd(in []byte) bool {
	return bytes.HasPrefix(in, []byte("8BPS"))
}

// Ico matches an ICO file.
func Ico(in []byte) bool {
	return len(in) > 3 &&
		in[0] == 0x00 && in[1] == 0x00 &&
		in[2] == 0x01 && in[3] == 0x00
}

// Tiff matches a Tagged Image File Format file.
func Tiff(in []byte) bool {
	return len(in) > 4 &&
		(bytes.Equal(in[:4], []byte{0x49, 0x49, 0x2A, 0x00}) ||
			bytes.Equal(in[:4], []byte{0x4D, 0x4D, 0x00, 0x2A}))
}
