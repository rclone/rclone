package matchers

import "bytes"

// Png matches a Portable Network Graphics file.
func Png(in []byte) bool {
	return bytes.HasPrefix(in, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})
}

// Jpg matches a Joint Photographic Experts Group file.
func Jpg(in []byte) bool {
	return bytes.HasPrefix(in, []byte{0xFF, 0xD8, 0xFF})
}

// isJpeg2k matches a generic JPEG2000 file.
func isJpeg2k(in []byte) bool {
	if len(in) < 24 {
		return false
	}

	signature := in[4:8]
	return bytes.Equal(signature, []byte{0x6A, 0x50, 0x20, 0x20}) ||
		bytes.Equal(signature, []byte{0x6A, 0x50, 0x32, 0x20})
}

// Jp2 matches a JPEG 2000 Image file (ISO 15444-1).
func Jp2(in []byte) bool {
	return isJpeg2k(in) && bytes.Equal(in[20:24], []byte{0x6a, 0x70, 0x32, 0x20})
}

// Jpx matches a JPEG 2000 Image file (ISO 15444-2).
func Jpx(in []byte) bool {
	return isJpeg2k(in) && bytes.Equal(in[20:24], []byte{0x6a, 0x70, 0x78, 0x20})
}

// Jpm matches a JPEG 2000 Image file (ISO 15444-6).
func Jpm(in []byte) bool {
	return isJpeg2k(in) && bytes.Equal(in[20:24], []byte{0x6a, 0x70, 0x6D, 0x20})
}

// Gif matches a Graphics Interchange Format file.
func Gif(in []byte) bool {
	return bytes.HasPrefix(in, []byte("GIF87a")) ||
		bytes.HasPrefix(in, []byte("GIF89a"))
}

// Webp matches a WebP file.
func Webp(in []byte) bool {
	return len(in) > 12 &&
		bytes.Equal(in[0:4], []byte("RIFF")) &&
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
	return bytes.HasPrefix(in, []byte{0x00, 0x00, 0x01, 0x00})
}

// Icns matches an ICNS (Apple Icon Image format) file.
func Icns(in []byte) bool {
	return bytes.HasPrefix(in, []byte("icns"))
}

// Tiff matches a Tagged Image File Format file.
func Tiff(in []byte) bool {
	return bytes.HasPrefix(in, []byte{0x49, 0x49, 0x2A, 0x00}) ||
		bytes.HasPrefix(in, []byte{0x4D, 0x4D, 0x00, 0x2A})
}

// Bpg matches a Better Portable Graphics file.
func Bpg(in []byte) bool {
	return bytes.HasPrefix(in, []byte{0x42, 0x50, 0x47, 0xFB})
}

// Dwg matches a CAD drawing file.
func Dwg(in []byte) bool {
	if len(in) < 6 || in[0] != 0x41 || in[1] != 0x43 {
		return false
	}
	dwgVersions := [][]byte{
		{0x31, 0x2E, 0x34, 0x30},
		{0x31, 0x2E, 0x35, 0x30},
		{0x32, 0x2E, 0x31, 0x30},
		{0x31, 0x30, 0x30, 0x32},
		{0x31, 0x30, 0x30, 0x33},
		{0x31, 0x30, 0x30, 0x34},
		{0x31, 0x30, 0x30, 0x36},
		{0x31, 0x30, 0x30, 0x39},
		{0x31, 0x30, 0x31, 0x32},
		{0x31, 0x30, 0x31, 0x34},
		{0x31, 0x30, 0x31, 0x35},
		{0x31, 0x30, 0x31, 0x38},
		{0x31, 0x30, 0x32, 0x31},
		{0x31, 0x30, 0x32, 0x34},
		{0x31, 0x30, 0x33, 0x32},
	}

	for _, d := range dwgVersions {
		if bytes.Equal(in[2:6], d) {
			return true
		}
	}

	return false
}

// Heic matches a High Efficiency Image Coding (HEIC) file.
func Heic(in []byte) bool {
	if len(in) <= 12 {
		return false
	}

	return bytes.Equal(in[4:12], []byte("ftypheic")) ||
		bytes.Equal(in[4:12], []byte("ftypheix"))
}

// HeicSequence matches a High Efficiency Image Coding (HEIC) file sequence.
func HeicSequence(in []byte) bool {
	if len(in) <= 12 {
		return false
	}

	return bytes.Equal(in[4:12], []byte("ftyphevc")) ||
		bytes.Equal(in[4:12], []byte("ftyphevx"))
}

// Heif matches a High Efficiency Image File Format (HEIF) file.
func Heif(in []byte) bool {
	if len(in) <= 12 {
		return false
	}

	return bytes.Equal(in[4:12], []byte("ftypmif1")) ||
		bytes.Equal(in[4:12], []byte("ftypheim")) ||
		bytes.Equal(in[4:12], []byte("ftypheis")) ||
		bytes.Equal(in[4:12], []byte("ftypavic"))
}

// HeifSequence matches a High Efficiency Image File Format (HEIF) file sequence.
func HeifSequence(in []byte) bool {
	if len(in) <= 12 {
		return false
	}

	return bytes.Equal(in[4:12], []byte("ftypmsf1")) ||
		bytes.Equal(in[4:12], []byte("ftyphevm")) ||
		bytes.Equal(in[4:12], []byte("ftyphevs")) ||
		bytes.Equal(in[4:12], []byte("ftypavcs"))
}
