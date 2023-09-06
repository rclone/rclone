package gomime

import (
	"encoding/base64"
	"errors"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/transform"
)

// utf7Decoder copied from: https://github.com/cention-sany/utf7/blob/master/utf7.go
// We need `encoding.Decoder` instead of function `UTF7DecodeBytes`
type utf7Decoder struct {
	transform.NopResetter
}

// NewUtf7Decoder return decoder for utf7
func NewUtf7Decoder() *encoding.Decoder {
	return &encoding.Decoder{Transformer: utf7Decoder{}}
}

const (
	uRepl = '\uFFFD' // Unicode replacement code point
	u7min = 0x20     // Minimum self-representing UTF-7 value
	u7max = 0x7E     // Maximum self-representing UTF-7 value
)

// ErrBadUTF7 is returned to indicate the invalid modified UTF-7 encoding.
var ErrBadUTF7 = errors.New("utf7: bad utf-7 encoding")

const modifiedbase64 = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

var u7enc = base64.NewEncoding(modifiedbase64)

func isModifiedBase64(r byte) bool {
	if r >= 'A' && r <= 'Z' {
		return true
	} else if r >= 'a' && r <= 'z' {
		return true
	} else if r >= '0' && r <= '9' {
		return true
	} else if r == '+' || r == '/' {
		return true
	}
	return false
}

func (d utf7Decoder) Transform(dst, src []byte, atEOF bool) (nDst, nSrc int, err error) {
	var implicit bool
	var tmp int

	nd, n := len(dst), len(src)
	if n == 0 && !atEOF {
		return 0, 0, transform.ErrShortSrc
	}
	for ; nSrc < n; nSrc++ {
		if nDst >= nd {
			return nDst, nSrc, transform.ErrShortDst
		}
		if c := src[nSrc]; ((c < u7min || c > u7max) &&
			c != '\t' && c != '\r' && c != '\n') ||
			c == '~' || c == '\\' {
			return nDst, nSrc, ErrBadUTF7 // Illegal code point in ASCII mode
		} else if c != '+' {
			dst[nDst] = c // character is self-representing
			nDst++
			continue
		}
		// found '+'
		start := nSrc + 1
		tmp = nSrc // nSrc still points to '+', tmp points to the end of BASE64
		// Find the end of the Base64 or "+-" segment
		implicit = false
		for tmp++; tmp < n && src[tmp] != '-'; tmp++ {
			if !isModifiedBase64(src[tmp]) {
				if tmp == start {
					return nDst, tmp, ErrBadUTF7 // '+' next char must modified base64
				}
				// implicit shift back to ASCII - no need '-' character
				implicit = true
				break
			}
		}
		if tmp == start {
			if tmp == n {
				// did not find '-' sign and '+' is the last character
				// total nSrc not includes '+'
				if atEOF {
					return nDst, nSrc, ErrBadUTF7 // '+' can not be at the end
				}
				// '+' can not be at the end, the source si too short
				return nDst, nSrc, transform.ErrShortSrc
			}
			dst[nDst] = '+' // Escape sequence "+-"
			nDst++
		} else if tmp == n && !atEOF {
			// no eof found, the source is too short
			return nDst, nSrc, transform.ErrShortSrc
		} else if b := utf7dec(src[start:tmp]); len(b) > 0 {
			if len(b)+nDst > nd {
				// need more space in dst for the decoded modified BASE64 unicode
				// total nSrc is not including '+'
				return nDst, nSrc, transform.ErrShortDst
			}
			copy(dst[nDst:], b) // Control or non-ASCII code points in Base64
			nDst += len(b)
			if implicit {
				if nDst >= nd {
					return nDst, tmp, transform.ErrShortDst
				}
				dst[nDst] = src[tmp] // implicit shift
				nDst++
			}
			if tmp == n {
				return nDst, tmp, nil
			}
		} else {
			return nDst, nSrc, ErrBadUTF7 // bad encoding
		}
		nSrc = tmp
	}
	return
}

// utf7dec extracts UTF-16-BE bytes from Base64 data and converts them to UTF-8.
// A nil slice is returned if the encoding is invalid.
func utf7dec(b64 []byte) []byte {
	var b []byte

	// Allocate a single block of memory large enough to store the Base64 data
	// (if padding is required), UTF-16-BE bytes, and decoded UTF-8 bytes.
	// Since a 2-byte UTF-16 sequence may expand into a 3-byte UTF-8 sequence,
	// double the space allocation for UTF-8.
	if n := len(b64); b64[n-1] == '=' {
		return nil
	} else if n&3 == 0 {
		b = make([]byte, u7enc.DecodedLen(n)*3)
	} else {
		n += 4 - n&3
		b = make([]byte, n+u7enc.DecodedLen(n)*3)
		copy(b[copy(b, b64):n], []byte("=="))
		b64, b = b[:n], b[n:]
	}

	// Decode Base64 into the first 1/3rd of b
	n, err := u7enc.Decode(b, b64)
	if err != nil || n&1 == 1 {
		return nil
	}

	// Decode UTF-16-BE into the remaining 2/3rds of b
	b, s := b[:n], b[n:]
	j := 0
	for i := 0; i < n; i += 2 {
		r := rune(b[i])<<8 | rune(b[i+1])
		if utf16.IsSurrogate(r) {
			if i += 2; i == n {
				return nil
			}
			r2 := rune(b[i])<<8 | rune(b[i+1])
			if r = utf16.DecodeRune(r, r2); r == uRepl {
				return nil
			}
		}
		j += utf8.EncodeRune(s[j:], r)
	}
	return s[:j]
}
