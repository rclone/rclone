package magic

import (
	"bytes"
	"encoding/binary"
)

var (
	// SevenZ matches a 7z archive.
	SevenZ = prefix([]byte{0x37, 0x7A, 0xBC, 0xAF, 0x27, 0x1C})
	// Gzip matches gzip files based on http://www.zlib.org/rfc-gzip.html#header-trailer.
	Gzip = prefix([]byte{0x1f, 0x8b})
	// Fits matches an Flexible Image Transport System file.
	Fits = prefix([]byte{
		0x53, 0x49, 0x4D, 0x50, 0x4C, 0x45, 0x20, 0x20, 0x3D, 0x20,
		0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20,
		0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x54,
	})
	// Xar matches an eXtensible ARchive format file.
	Xar = prefix([]byte{0x78, 0x61, 0x72, 0x21})
	// Bz2 matches a bzip2 file.
	Bz2 = prefix([]byte{0x42, 0x5A, 0x68})
	// Ar matches an ar (Unix) archive file.
	Ar = prefix([]byte{0x21, 0x3C, 0x61, 0x72, 0x63, 0x68, 0x3E})
	// Deb matches a Debian package file.
	Deb = offset([]byte{
		0x64, 0x65, 0x62, 0x69, 0x61, 0x6E, 0x2D,
		0x62, 0x69, 0x6E, 0x61, 0x72, 0x79,
	}, 8)
	// Warc matches a Web ARChive file.
	Warc = prefix([]byte("WARC/1.0"), []byte("WARC/1.1"))
	// Cab matches a Microsoft Cabinet archive file.
	Cab = prefix([]byte("MSCF\x00\x00\x00\x00"))
	// Xz matches an xz compressed stream based on https://tukaani.org/xz/xz-file-format.txt.
	Xz = prefix([]byte{0xFD, 0x37, 0x7A, 0x58, 0x5A, 0x00})
	// Lzip matches an Lzip compressed file.
	Lzip = prefix([]byte{0x4c, 0x5a, 0x49, 0x50})
	// RPM matches an RPM or Delta RPM package file.
	RPM = prefix([]byte{0xed, 0xab, 0xee, 0xdb}, []byte("drpm"))
	// Cpio matches a cpio archive file.
	Cpio = prefix([]byte("070707"), []byte("070701"), []byte("070702"))
	// RAR matches a RAR archive file.
	RAR = prefix([]byte("Rar!\x1A\x07\x00"), []byte("Rar!\x1A\x07\x01\x00"))
)

// InstallShieldCab matches an InstallShield Cabinet archive file.
func InstallShieldCab(raw []byte, _ uint32) bool {
	return len(raw) > 7 &&
		bytes.Equal(raw[0:4], []byte("ISc(")) &&
		raw[6] == 0 &&
		(raw[7] == 1 || raw[7] == 2 || raw[7] == 4)
}

// Zstd matches a Zstandard archive file.
func Zstd(raw []byte, limit uint32) bool {
	return len(raw) >= 4 &&
		(0x22 <= raw[0] && raw[0] <= 0x28 || raw[0] == 0x1E) && // Different Zstandard versions.
		bytes.HasPrefix(raw[1:], []byte{0xB5, 0x2F, 0xFD})
}

// CRX matches a Chrome extension file: a zip archive prepended by a package header.
func CRX(raw []byte, limit uint32) bool {
	const minHeaderLen = 16
	if len(raw) < minHeaderLen || !bytes.HasPrefix(raw, []byte("Cr24")) {
		return false
	}
	pubkeyLen := binary.LittleEndian.Uint32(raw[8:12])
	sigLen := binary.LittleEndian.Uint32(raw[12:16])
	zipOffset := minHeaderLen + pubkeyLen + sigLen
	if uint32(len(raw)) < zipOffset {
		return false
	}
	return Zip(raw[zipOffset:], limit)
}

// Tar matches a (t)ape (ar)chive file.
func Tar(raw []byte, _ uint32) bool {
	// The "magic" header field for files in in UStar (POSIX IEEE P1003.1) archives
	// has the prefix "ustar". The values of the remaining bytes in this field vary
	// by archiver implementation.
	if len(raw) >= 512 && bytes.HasPrefix(raw[257:], []byte{0x75, 0x73, 0x74, 0x61, 0x72}) {
		return true
	}

	if len(raw) < 256 {
		return false
	}

	// The older v7 format has no "magic" field, and therefore must be identified
	// with heuristics based on legal ranges of values for other header fields:
	// https://www.nationalarchives.gov.uk/PRONOM/Format/proFormatSearch.aspx?status=detailReport&id=385&strPageToDisplay=signatures
	rules := []struct {
		min, max uint8
		i        int
	}{
		{0x21, 0xEF, 0},
		{0x30, 0x37, 105},
		{0x20, 0x37, 106},
		{0x00, 0x00, 107},
		{0x30, 0x37, 113},
		{0x20, 0x37, 114},
		{0x00, 0x00, 115},
		{0x30, 0x37, 121},
		{0x20, 0x37, 122},
		{0x00, 0x00, 123},
		{0x30, 0x37, 134},
		{0x30, 0x37, 146},
		{0x30, 0x37, 153},
		{0x00, 0x37, 154},
	}
	for _, r := range rules {
		if raw[r.i] < r.min || raw[r.i] > r.max {
			return false
		}
	}

	for _, i := range []uint8{135, 147, 155} {
		if raw[i] != 0x00 && raw[i] != 0x20 {
			return false
		}
	}

	return true
}
