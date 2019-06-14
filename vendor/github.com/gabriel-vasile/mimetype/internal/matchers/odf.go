package matchers

import "bytes"

// Odt matches an OpenDocument Text file.
func Odt(in []byte) bool {
	return bytes.Contains(in, []byte("mimetypeapplication/vnd.oasis.opendocument.text"))
}

// Ott matches an OpenDocument Text Template file.
func Ott(in []byte) bool {
	return bytes.Contains(in, []byte("mimetypeapplication/vnd.oasis.opendocument.text-template"))
}

// Ods matches an OpenDocument Spreadsheet file.
func Ods(in []byte) bool {
	return bytes.Contains(in, []byte("mimetypeapplication/vnd.oasis.opendocument.spreadsheet"))
}

// Ots matches an OpenDocument Spreadsheet Template file.
func Ots(in []byte) bool {
	return bytes.Contains(in, []byte("mimetypeapplication/vnd.oasis.opendocument.spreadsheet-template"))
}

// Odp matches an OpenDocument Presentation file.
func Odp(in []byte) bool {
	return bytes.Contains(in, []byte("mimetypeapplication/vnd.oasis.opendocument.presentation"))
}

// Otp matches an OpenDocument Presentation Template file.
func Otp(in []byte) bool {
	return bytes.Contains(in, []byte("mimetypeapplication/vnd.oasis.opendocument.presentation-template"))
}

// Odg matches an OpenDocument Drawing file.
func Odg(in []byte) bool {
	return bytes.Contains(in, []byte("mimetypeapplication/vnd.oasis.opendocument.graphics"))
}

// Otg matches an OpenDocument Drawing Template file.
func Otg(in []byte) bool {
	return bytes.Contains(in, []byte("mimetypeapplication/vnd.oasis.opendocument.graphics-template"))
}

// Odf matches an OpenDocument Formula file.
func Odf(in []byte) bool {
	return bytes.Contains(in, []byte("mimetypeapplication/vnd.oasis.opendocument.formula"))
}
