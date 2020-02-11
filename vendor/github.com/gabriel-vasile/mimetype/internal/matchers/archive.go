package matchers

import "bytes"

// Zip matches a zip archive.
func Zip(in []byte) bool {
	return len(in) > 3 &&
		in[0] == 0x50 && in[1] == 0x4B &&
		(in[2] == 0x3 || in[2] == 0x5 || in[2] == 0x7) &&
		(in[3] == 0x4 || in[3] == 0x6 || in[3] == 0x8)
}

// SevenZ matches a 7z archive.
func SevenZ(in []byte) bool {
	return bytes.HasPrefix(in, []byte{0x37, 0x7A, 0xBC, 0xAF, 0x27, 0x1C})
}

// Epub matches an EPUB file.
func Epub(in []byte) bool {
	return len(in) > 58 && bytes.Equal(in[30:58], []byte("mimetypeapplication/epub+zip"))
}

// Jar matches a Java archive file.
func Jar(in []byte) bool {
	return bytes.Contains(in, []byte("META-INF/MANIFEST.MF"))
}

// Gzip matched gzip files based on http://www.zlib.org/rfc-gzip.html#header-trailer.
func Gzip(in []byte) bool {
	return bytes.HasPrefix(in, []byte{0x1f, 0x8b})
}

// Crx matches a Chrome extension file: a zip archive prepended by "Cr24".
func Crx(in []byte) bool {
	return bytes.HasPrefix(in, []byte("Cr24"))
}

// Tar matches a (t)ape (ar)chive file.
func Tar(in []byte) bool {
	return len(in) > 262 && bytes.Equal(in[257:262], []byte("ustar"))
}

// Fits matches an Flexible Image Transport System file.
func Fits(in []byte) bool {
	return bytes.HasPrefix(in, []byte{
		0x53, 0x49, 0x4D, 0x50, 0x4C, 0x45, 0x20, 0x20, 0x3D, 0x20,
		0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20,
		0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x54,
	})
}

// Xar matches an eXtensible ARchive format file.
func Xar(in []byte) bool {
	return bytes.HasPrefix(in, []byte{0x78, 0x61, 0x72, 0x21})
}

// Bz2 matches a bzip2 file.
func Bz2(in []byte) bool {
	return bytes.HasPrefix(in, []byte{0x42, 0x5A, 0x68})
}

// Ar matches an ar (Unix) archive file.
func Ar(in []byte) bool {
	return bytes.HasPrefix(in, []byte{0x21, 0x3C, 0x61, 0x72, 0x63, 0x68, 0x3E})
}

// Deb matches a Debian package file.
func Deb(in []byte) bool {
	return len(in) > 8 && bytes.HasPrefix(in[8:], []byte{
		0x64, 0x65, 0x62, 0x69, 0x61, 0x6E, 0x2D,
		0x62, 0x69, 0x6E, 0x61, 0x72, 0x79,
	})
}

// Rar matches a RAR archive file.
func Rar(in []byte) bool {
	if !bytes.HasPrefix(in, []byte{0x52, 0x61, 0x72, 0x21, 0x1A, 0x07}) {
		return false
	}
	return len(in) > 8 && (bytes.Equal(in[6:8], []byte{0x01, 0x00}) || in[6] == 0x00)
}

// Warc matches a Web ARChive file.
func Warc(in []byte) bool {
	return bytes.HasPrefix(in, []byte("WARC/"))
}

// Zstd matches a Zstandard archive file.
func Zstd(in []byte) bool {
	return len(in) >= 4 &&
		(0x22 <= in[0] && in[0] <= 0x28 || in[0] == 0x1E) && // Different Zstandard versions.
		bytes.HasPrefix(in[1:], []byte{0xB5, 0x2F, 0xFD})
}

// Cab matches a Cabinet archive file.
func Cab(in []byte) bool {
	return bytes.HasPrefix(in, []byte("MSCF"))
}
