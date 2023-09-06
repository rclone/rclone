package magic

import (
	"bytes"
	"encoding/binary"
	"strings"
)

var (
	// Odt matches an OpenDocument Text file.
	Odt = offset([]byte("mimetypeapplication/vnd.oasis.opendocument.text"), 30)
	// Ott matches an OpenDocument Text Template file.
	Ott = offset([]byte("mimetypeapplication/vnd.oasis.opendocument.text-template"), 30)
	// Ods matches an OpenDocument Spreadsheet file.
	Ods = offset([]byte("mimetypeapplication/vnd.oasis.opendocument.spreadsheet"), 30)
	// Ots matches an OpenDocument Spreadsheet Template file.
	Ots = offset([]byte("mimetypeapplication/vnd.oasis.opendocument.spreadsheet-template"), 30)
	// Odp matches an OpenDocument Presentation file.
	Odp = offset([]byte("mimetypeapplication/vnd.oasis.opendocument.presentation"), 30)
	// Otp matches an OpenDocument Presentation Template file.
	Otp = offset([]byte("mimetypeapplication/vnd.oasis.opendocument.presentation-template"), 30)
	// Odg matches an OpenDocument Drawing file.
	Odg = offset([]byte("mimetypeapplication/vnd.oasis.opendocument.graphics"), 30)
	// Otg matches an OpenDocument Drawing Template file.
	Otg = offset([]byte("mimetypeapplication/vnd.oasis.opendocument.graphics-template"), 30)
	// Odf matches an OpenDocument Formula file.
	Odf = offset([]byte("mimetypeapplication/vnd.oasis.opendocument.formula"), 30)
	// Odc matches an OpenDocument Chart file.
	Odc = offset([]byte("mimetypeapplication/vnd.oasis.opendocument.chart"), 30)
	// Epub matches an EPUB file.
	Epub = offset([]byte("mimetypeapplication/epub+zip"), 30)
	// Sxc matches an OpenOffice Spreadsheet file.
	Sxc = offset([]byte("mimetypeapplication/vnd.sun.xml.calc"), 30)
)

// Zip matches a zip archive.
func Zip(raw []byte, limit uint32) bool {
	return len(raw) > 3 &&
		raw[0] == 0x50 && raw[1] == 0x4B &&
		(raw[2] == 0x3 || raw[2] == 0x5 || raw[2] == 0x7) &&
		(raw[3] == 0x4 || raw[3] == 0x6 || raw[3] == 0x8)
}

// Jar matches a Java archive file.
func Jar(raw []byte, limit uint32) bool {
	return zipContains(raw, "META-INF/MANIFEST.MF")
}

// zipTokenizer holds the source zip file and scanned index.
type zipTokenizer struct {
	in []byte
	i  int // current index
}

// next returns the next file name from the zip headers.
// https://web.archive.org/web/20191129114319/https://users.cs.jmu.edu/buchhofp/forensics/formats/pkzip.html
func (t *zipTokenizer) next() (fileName string) {
	if t.i > len(t.in) {
		return
	}
	in := t.in[t.i:]
	// pkSig is the signature of the zip local file header.
	pkSig := []byte("PK\003\004")
	pkIndex := bytes.Index(in, pkSig)
	// 30 is the offset of the file name in the header.
	fNameOffset := pkIndex + 30
	// end if signature not found or file name offset outside of file.
	if pkIndex == -1 || fNameOffset > len(in) {
		return
	}

	fNameLen := int(binary.LittleEndian.Uint16(in[pkIndex+26 : pkIndex+28]))
	if fNameLen <= 0 || fNameOffset+fNameLen > len(in) {
		return
	}
	t.i += fNameOffset + fNameLen
	return string(in[fNameOffset : fNameOffset+fNameLen])
}

// zipContains returns true if the zip file headers from in contain any of the paths.
func zipContains(in []byte, paths ...string) bool {
	t := zipTokenizer{in: in}
	for i, tok := 0, t.next(); tok != ""; i, tok = i+1, t.next() {
		for p := range paths {
			if strings.HasPrefix(tok, paths[p]) {
				return true
			}
		}
	}

	return false
}
