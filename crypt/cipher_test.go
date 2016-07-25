package crypt

import (
	"bytes"
	"encoding/base32"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/ncw/rclone/crypt/pkcs7"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidString(t *testing.T) {
	for _, test := range []struct {
		in       string
		expected error
	}{
		{"", nil},
		{"\x01", ErrorBadDecryptControlChar},
		{"a\x02", ErrorBadDecryptControlChar},
		{"abc\x03", ErrorBadDecryptControlChar},
		{"abc\x04def", ErrorBadDecryptControlChar},
		{"\x05d", ErrorBadDecryptControlChar},
		{"\x06def", ErrorBadDecryptControlChar},
		{"\x07", ErrorBadDecryptControlChar},
		{"\x08", ErrorBadDecryptControlChar},
		{"\x09", ErrorBadDecryptControlChar},
		{"\x0A", ErrorBadDecryptControlChar},
		{"\x0B", ErrorBadDecryptControlChar},
		{"\x0C", ErrorBadDecryptControlChar},
		{"\x0D", ErrorBadDecryptControlChar},
		{"\x0E", ErrorBadDecryptControlChar},
		{"\x0F", ErrorBadDecryptControlChar},
		{"\x10", ErrorBadDecryptControlChar},
		{"\x11", ErrorBadDecryptControlChar},
		{"\x12", ErrorBadDecryptControlChar},
		{"\x13", ErrorBadDecryptControlChar},
		{"\x14", ErrorBadDecryptControlChar},
		{"\x15", ErrorBadDecryptControlChar},
		{"\x16", ErrorBadDecryptControlChar},
		{"\x17", ErrorBadDecryptControlChar},
		{"\x18", ErrorBadDecryptControlChar},
		{"\x19", ErrorBadDecryptControlChar},
		{"\x1A", ErrorBadDecryptControlChar},
		{"\x1B", ErrorBadDecryptControlChar},
		{"\x1C", ErrorBadDecryptControlChar},
		{"\x1D", ErrorBadDecryptControlChar},
		{"\x1E", ErrorBadDecryptControlChar},
		{"\x1F", ErrorBadDecryptControlChar},
		{"\x20", nil},
		{"\x7E", nil},
		{"\x7F", ErrorBadDecryptControlChar},
		{"£100", nil},
		{`hello? sausage/êé/Hello, 世界/ " ' @ < > & ?/z.txt`, nil},
		{"£100", nil},
		// Following tests from http://www.php.net/manual/en/reference.pcre.pattern.modifiers.php#54805
		{"a", nil},                                        // Valid ASCII
		{"\xc3\xb1", nil},                                 // Valid 2 Octet Sequence
		{"\xc3\x28", ErrorBadDecryptUTF8},                 // Invalid 2 Octet Sequence
		{"\xa0\xa1", ErrorBadDecryptUTF8},                 // Invalid Sequence Identifier
		{"\xe2\x82\xa1", nil},                             // Valid 3 Octet Sequence
		{"\xe2\x28\xa1", ErrorBadDecryptUTF8},             // Invalid 3 Octet Sequence (in 2nd Octet)
		{"\xe2\x82\x28", ErrorBadDecryptUTF8},             // Invalid 3 Octet Sequence (in 3rd Octet)
		{"\xf0\x90\x8c\xbc", nil},                         // Valid 4 Octet Sequence
		{"\xf0\x28\x8c\xbc", ErrorBadDecryptUTF8},         // Invalid 4 Octet Sequence (in 2nd Octet)
		{"\xf0\x90\x28\xbc", ErrorBadDecryptUTF8},         // Invalid 4 Octet Sequence (in 3rd Octet)
		{"\xf0\x28\x8c\x28", ErrorBadDecryptUTF8},         // Invalid 4 Octet Sequence (in 4th Octet)
		{"\xf8\xa1\xa1\xa1\xa1", ErrorBadDecryptUTF8},     // Valid 5 Octet Sequence (but not Unicode!)
		{"\xfc\xa1\xa1\xa1\xa1\xa1", ErrorBadDecryptUTF8}, // Valid 6 Octet Sequence (but not Unicode!)
	} {
		actual := checkValidString([]byte(test.in))
		assert.Equal(t, actual, test.expected, fmt.Sprintf("in=%q", test.in))
	}
}

func TestEncodeFileName(t *testing.T) {
	for _, test := range []struct {
		in       string
		expected string
	}{
		{"", ""},
		{"1", "64"},
		{"12", "64p0"},
		{"123", "64p36"},
		{"1234", "64p36d0"},
		{"12345", "64p36d1l"},
		{"123456", "64p36d1l6o"},
		{"1234567", "64p36d1l6org"},
		{"12345678", "64p36d1l6orjg"},
		{"123456789", "64p36d1l6orjge8"},
		{"1234567890", "64p36d1l6orjge9g"},
		{"12345678901", "64p36d1l6orjge9g64"},
		{"123456789012", "64p36d1l6orjge9g64p0"},
		{"1234567890123", "64p36d1l6orjge9g64p36"},
		{"12345678901234", "64p36d1l6orjge9g64p36d0"},
		{"123456789012345", "64p36d1l6orjge9g64p36d1l"},
		{"1234567890123456", "64p36d1l6orjge9g64p36d1l6o"},
	} {
		actual := encodeFileName([]byte(test.in))
		assert.Equal(t, actual, test.expected, fmt.Sprintf("in=%q", test.in))
		recovered, err := decodeFileName(test.expected)
		assert.NoError(t, err)
		assert.Equal(t, string(recovered), test.in, fmt.Sprintf("reverse=%q", test.expected))
		in := strings.ToUpper(test.expected)
		recovered, err = decodeFileName(in)
		assert.NoError(t, err)
		assert.Equal(t, string(recovered), test.in, fmt.Sprintf("reverse=%q", in))
	}
}

func TestDecodeFileName(t *testing.T) {
	// We've tested decoding the valid ones above, now concentrate on the invalid ones
	for _, test := range []struct {
		in          string
		expectedErr error
	}{
		{"64=", ErrorBadBase32Encoding},
		{"!", base32.CorruptInputError(0)},
		{"hello=hello", base32.CorruptInputError(5)},
	} {
		actual, actualErr := decodeFileName(test.in)
		assert.Equal(t, test.expectedErr, actualErr, fmt.Sprintf("in=%q got actual=%q, err = %v %T", test.in, actual, actualErr, actualErr))
	}
}

func TestEncryptSegment(t *testing.T) {
	c, _ := newCipher(0, "")
	for _, test := range []struct {
		in       string
		expected string
	}{
		{"", ""},
		{"1", "p0e52nreeaj0a5ea7s64m4j72s"},
		{"12", "l42g6771hnv3an9cgc8cr2n1ng"},
		{"123", "qgm4avr35m5loi1th53ato71v0"},
		{"1234", "8ivr2e9plj3c3esisjpdisikos"},
		{"12345", "rh9vu63q3o29eqmj4bg6gg7s44"},
		{"123456", "bn717l3alepn75b2fb2ejmi4b4"},
		{"1234567", "n6bo9jmb1qe3b1ogtj5qkf19k8"},
		{"12345678", "u9t24j7uaq94dh5q53m3s4t9ok"},
		{"123456789", "37hn305g6j12d1g0kkrl7ekbs4"},
		{"1234567890", "ot8d91eplaglb62k2b1trm2qv0"},
		{"12345678901", "h168vvrgb53qnrtvvmb378qrcs"},
		{"123456789012", "s3hsdf9e29ithrqbjqu01t8q2s"},
		{"1234567890123", "cf3jimlv1q2oc553mv7s3mh3eo"},
		{"12345678901234", "moq0uqdlqrblrc5pa5u5c7hq9g"},
		{"123456789012345", "eeam3li4rnommi3a762h5n7meg"},
		{"1234567890123456", "mijbj0frqf6ms7frcr6bd9h0env53jv96pjaaoirk7forcgpt70g"},
	} {
		actual := c.encryptSegment(test.in)
		assert.Equal(t, test.expected, actual, fmt.Sprintf("Testing %q", test.in))
		recovered, err := c.decryptSegment(test.expected)
		assert.NoError(t, err, fmt.Sprintf("Testing reverse %q", test.expected))
		assert.Equal(t, test.in, recovered, fmt.Sprintf("Testing reverse %q", test.expected))
		in := strings.ToUpper(test.expected)
		recovered, err = c.decryptSegment(in)
		assert.NoError(t, err, fmt.Sprintf("Testing reverse %q", in))
		assert.Equal(t, test.in, recovered, fmt.Sprintf("Testing reverse %q", in))
	}
}

func TestDecryptSegment(t *testing.T) {
	// We've tested the forwards above, now concentrate on the errors
	c, _ := newCipher(0, "")
	for _, test := range []struct {
		in          string
		expectedErr error
	}{
		{"64=", ErrorBadBase32Encoding},
		{"!", base32.CorruptInputError(0)},
		{encodeFileName([]byte("a")), ErrorNotAMultipleOfBlocksize},
		{encodeFileName([]byte("123456789abcdef")), ErrorNotAMultipleOfBlocksize},
		{encodeFileName([]byte("123456789abcdef0")), pkcs7.ErrorPaddingTooLong},
		{c.encryptSegment("\x01"), ErrorBadDecryptControlChar},
		{c.encryptSegment("\xc3\x28"), ErrorBadDecryptUTF8},
	} {
		actual, actualErr := c.decryptSegment(test.in)
		assert.Equal(t, test.expectedErr, actualErr, fmt.Sprintf("in=%q got actual=%q, err = %v %T", test.in, actual, actualErr, actualErr))
	}
}

func TestSpreadName(t *testing.T) {
	for _, test := range []struct {
		n        int
		in       string
		expected string
	}{
		{3, "", ""},
		{0, "abcdefg", "abcdefg"},
		{1, "abcdefg", "a/abcdefg"},
		{2, "abcdefg", "a/b/abcdefg"},
		{3, "abcdefg", "a/b/c/abcdefg"},
		{4, "abcdefg", "a/b/c/d/abcdefg"},
		{4, "abcd", "a/b/c/d/abcd"},
		{4, "abc", "a/b/c/abc"},
		{4, "ab", "a/b/ab"},
		{4, "a", "a/a"},
	} {
		actual := spreadName(test.n, test.in)
		assert.Equal(t, test.expected, actual, fmt.Sprintf("Testing %d,%q", test.n, test.in))
		recovered, err := unspreadName(test.expected)
		assert.NoError(t, err, fmt.Sprintf("Testing reverse %q", test.expected))
		assert.Equal(t, test.in, recovered, fmt.Sprintf("Testing reverse %q", test.expected))
	}
}

func TestUnspreadName(t *testing.T) {
	// We've tested the forwards above, now concentrate on the errors
	for _, test := range []struct {
		in          string
		expectedErr error
	}{
		{"aa/bc", ErrorBadSpreadNotSingleChar},
		{"/", ErrorBadSpreadNotSingleChar},
		{"a/", ErrorBadSpreadResultTooShort},
		{"a/b/c/ab", ErrorBadSpreadResultTooShort},
		{"a/b/x/abc", ErrorBadSpreadDidntMatch},
		{"a/b/c/ABC", nil},
	} {
		actual, actualErr := unspreadName(test.in)
		assert.Equal(t, test.expectedErr, actualErr, fmt.Sprintf("in=%q got actual=%q, err = %v %T", test.in, actual, actualErr, actualErr))
	}
}

func TestEncryptName(t *testing.T) {
	// First no flatten
	c, _ := newCipher(0, "")
	assert.Equal(t, "p0e52nreeaj0a5ea7s64m4j72s", c.EncryptName("1"))
	assert.Equal(t, "p0e52nreeaj0a5ea7s64m4j72s/l42g6771hnv3an9cgc8cr2n1ng", c.EncryptName("1/12"))
	assert.Equal(t, "p0e52nreeaj0a5ea7s64m4j72s/l42g6771hnv3an9cgc8cr2n1ng/qgm4avr35m5loi1th53ato71v0", c.EncryptName("1/12/123"))
	// Now with flatten
	c, _ = newCipher(3, "")
	assert.Equal(t, "k/g/t/kgtickdcigo7600huebjl3ubu4", c.EncryptName("1/12/123"))
}

func TestDecryptName(t *testing.T) {
	for _, test := range []struct {
		flatten     int
		in          string
		expected    string
		expectedErr error
	}{
		{0, "p0e52nreeaj0a5ea7s64m4j72s", "1", nil},
		{0, "p0e52nreeaj0a5ea7s64m4j72s/l42g6771hnv3an9cgc8cr2n1ng", "1/12", nil},
		{0, "p0e52nreeAJ0A5EA7S64M4J72S/L42G6771HNv3an9cgc8cr2n1ng", "1/12", nil},
		{0, "p0e52nreeaj0a5ea7s64m4j72s/l42g6771hnv3an9cgc8cr2n1ng/qgm4avr35m5loi1th53ato71v0", "1/12/123", nil},
		{0, "p0e52nreeaj0a5ea7s64m4j72s/l42g6771hnv3an9cgc8cr2n1/qgm4avr35m5loi1th53ato71v0", "", ErrorNotAMultipleOfBlocksize},
		{3, "k/g/t/kgtickdcigo7600huebjl3ubu4", "1/12/123", nil},
		{1, "k/g/t/kgtickdcigo7600huebjl3ubu4", "1/12/123", nil},
		{1, "k/g/t/i/kgtickdcigo7600huebjl3ubu4", "1/12/123", nil},
		{1, "k/x/t/i/kgtickdcigo7600huebjl3ubu4", "", ErrorBadSpreadDidntMatch},
	} {
		c, _ := newCipher(test.flatten, "")
		actual, actualErr := c.DecryptName(test.in)
		what := fmt.Sprintf("Testing %q (flatten=%d)", test.in, test.flatten)
		assert.Equal(t, test.expected, actual, what)
		assert.Equal(t, test.expectedErr, actualErr, what)
	}
}

func TestEncryptedSize(t *testing.T) {
	c, _ := newCipher(0, "")
	for _, test := range []struct {
		in       int64
		expected int64
	}{
		{0, 32},
		{1, 32 + 16 + 1},
		{65536, 32 + 16 + 65536},
		{65537, 32 + 16 + 65536 + 16 + 1},
		{1 << 20, 32 + 16*(16+65536)},
		{(1 << 20) + 65535, 32 + 16*(16+65536) + 16 + 65535},
		{1 << 30, 32 + 16384*(16+65536)},
		{(1 << 40) + 1, 32 + 16777216*(16+65536) + 16 + 1},
	} {
		actual := c.EncryptedSize(test.in)
		assert.Equal(t, test.expected, actual, fmt.Sprintf("Testing %d", test.in))
		recovered, err := c.DecryptedSize(test.expected)
		assert.NoError(t, err, fmt.Sprintf("Testing reverse %d", test.expected))
		assert.Equal(t, test.in, recovered, fmt.Sprintf("Testing reverse %d", test.expected))
	}
}

func TestDecryptedSize(t *testing.T) {
	// Test the errors since we tested the reverse above
	c, _ := newCipher(0, "")
	for _, test := range []struct {
		in          int64
		expectedErr error
	}{
		{0, ErrorEncryptedFileTooShort},
		{0, ErrorEncryptedFileTooShort},
		{1, ErrorEncryptedFileTooShort},
		{7, ErrorEncryptedFileTooShort},
		{32 + 1, ErrorEncryptedFileBadHeader},
		{32 + 16, ErrorEncryptedFileBadHeader},
		{32 + 16 + 65536 + 1, ErrorEncryptedFileBadHeader},
		{32 + 16 + 65536 + 16, ErrorEncryptedFileBadHeader},
	} {
		_, actualErr := c.DecryptedSize(test.in)
		assert.Equal(t, test.expectedErr, actualErr, fmt.Sprintf("Testing %d", test.in))
	}
}

func TestNoncePointer(t *testing.T) {
	var x nonce
	assert.Equal(t, (*[24]byte)(&x), x.pointer())
}

func TestNonceFromReader(t *testing.T) {
	var x nonce
	buf := bytes.NewBufferString("123456789abcdefghijklmno")
	err := x.fromReader(buf)
	assert.NoError(t, err)
	assert.Equal(t, nonce{'1', '2', '3', '4', '5', '6', '7', '8', '9', 'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o'}, x)
	buf = bytes.NewBufferString("123456789abcdefghijklmn")
	err = x.fromReader(buf)
	assert.Error(t, err, "short read of nonce")
}

func TestNonceFromBuf(t *testing.T) {
	var x nonce
	buf := []byte("123456789abcdefghijklmnoXXXXXXXX")
	x.fromBuf(buf)
	assert.Equal(t, nonce{'1', '2', '3', '4', '5', '6', '7', '8', '9', 'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o'}, x)
	buf = []byte("0123456789abcdefghijklmn")
	x.fromBuf(buf)
	assert.Equal(t, nonce{'0', '1', '2', '3', '4', '5', '6', '7', '8', '9', 'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n'}, x)
	buf = []byte("0123456789abcdefghijklm")
	assert.Panics(t, func() { x.fromBuf(buf) })
}

func TestNonceIncrement(t *testing.T) {
	for _, test := range []struct {
		in  nonce
		out nonce
	}{
		{
			nonce{0x00},
			nonce{0x01},
		},
		{
			nonce{0xFF},
			nonce{0x00, 0x01},
		},
		{
			nonce{0xFF, 0xFF},
			nonce{0x00, 0x00, 0x01},
		},
		{
			nonce{0xFF, 0xFF, 0xFF},
			nonce{0x00, 0x00, 0x00, 0x01},
		},
		{
			nonce{0xFF, 0xFF, 0xFF, 0xFF},
			nonce{0x00, 0x00, 0x00, 0x00, 0x01},
		},
		{
			nonce{0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			nonce{0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
		},
		{
			nonce{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			nonce{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
		},
		{
			nonce{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			nonce{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
		},
		{
			nonce{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			nonce{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
		},
		{
			nonce{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			nonce{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
		},
		{
			nonce{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			nonce{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
		},
		{
			nonce{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			nonce{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
		},
		{
			nonce{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			nonce{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
		},
		{
			nonce{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			nonce{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
		},
		{
			nonce{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			nonce{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
		},
		{
			nonce{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			nonce{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
		},
		{
			nonce{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			nonce{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
		},
		{
			nonce{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			nonce{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
		},
		{
			nonce{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			nonce{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
		},
		{
			nonce{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			nonce{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
		},
		{
			nonce{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			nonce{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
		},
		{
			nonce{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			nonce{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
		},
		{
			nonce{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			nonce{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
		},
		{
			nonce{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			nonce{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
		},
		{
			nonce{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			nonce{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		},
	} {
		x := test.in
		x.increment()
		assert.Equal(t, test.out, x)
	}
}

// randomSource can read or write a random sequence
type randomSource struct {
	counter int64
	size    int64
}

func newRandomSource(size int64) *randomSource {
	return &randomSource{
		size: size,
	}
}

func (r *randomSource) next() byte {
	r.counter++
	return byte(r.counter % 257)
}

func (r *randomSource) Read(p []byte) (n int, err error) {
	for i := range p {
		if r.counter >= r.size {
			err = io.EOF
			break
		}
		p[i] = r.next()
		n++
	}
	return n, err
}

func (r *randomSource) Write(p []byte) (n int, err error) {
	for i := range p {
		if p[i] != r.next() {
			return 0, errors.Errorf("Error in stream at %d", r.counter)
		}
	}
	return len(p), nil
}

func (r *randomSource) Close() error { return nil }

// Check interfaces
var (
	_ io.ReadCloser  = (*randomSource)(nil)
	_ io.WriteCloser = (*randomSource)(nil)
)

// Test test infrastructure first!
func TestRandomSource(t *testing.T) {
	source := newRandomSource(1E8)
	sink := newRandomSource(1E8)
	n, err := io.Copy(sink, source)
	assert.NoError(t, err)
	assert.Equal(t, int64(1E8), n)

	source = newRandomSource(1E8)
	buf := make([]byte, 16)
	_, _ = source.Read(buf)
	sink = newRandomSource(1E8)
	n, err = io.Copy(sink, source)
	assert.Error(t, err, "Error in stream")
}

type zeroes struct{}

func (z *zeroes) Read(p []byte) (n int, err error) {
	for i := range p {
		p[i] = 0
		n++
	}
	return n, nil
}

// Test encrypt decrypt with different buffer sizes
func testEncryptDecrypt(t *testing.T, bufSize int, copySize int64) {
	c, err := newCipher(0, "")
	assert.NoError(t, err)
	c.cryptoRand = &zeroes{} // zero out the nonce
	buf := make([]byte, bufSize)
	source := newRandomSource(copySize)
	encrypted, err := c.newEncrypter(source)
	assert.NoError(t, err)
	decrypted, err := c.newDecrypter(ioutil.NopCloser(encrypted))
	assert.NoError(t, err)
	sink := newRandomSource(copySize)
	n, err := io.CopyBuffer(sink, decrypted, buf)
	assert.NoError(t, err)
	assert.Equal(t, copySize, n)
	blocks := copySize / blockSize
	if (copySize % blockSize) != 0 {
		blocks++
	}
	var expectedNonce = nonce{byte(blocks), byte(blocks >> 8), byte(blocks >> 16), byte(blocks >> 32)}
	assert.Equal(t, expectedNonce, encrypted.nonce)
	assert.Equal(t, expectedNonce, decrypted.nonce)
}

func TestEncryptDecrypt1(t *testing.T) {
	testEncryptDecrypt(t, 1, 1E7)
}

func TestEncryptDecrypt32(t *testing.T) {
	testEncryptDecrypt(t, 32, 1E8)
}

func TestEncryptDecrypt4096(t *testing.T) {
	testEncryptDecrypt(t, 4096, 1E8)
}

func TestEncryptDecrypt65536(t *testing.T) {
	testEncryptDecrypt(t, 65536, 1E8)
}

func TestEncryptDecrypt65537(t *testing.T) {
	testEncryptDecrypt(t, 65537, 1E8)
}

var (
	file0 = []byte{
		0x52, 0x43, 0x4c, 0x4f, 0x4e, 0x45, 0x00, 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18,
	}
	file1 = []byte{
		0x52, 0x43, 0x4c, 0x4f, 0x4e, 0x45, 0x00, 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18,
		0x09, 0x5b, 0x44, 0x6c, 0xd6, 0x23, 0x7b, 0xbc, 0xb0, 0x8d, 0x09, 0xfb, 0x52, 0x4c, 0xe5, 0x65,
		0xAA,
	}
	file16 = []byte{
		0x52, 0x43, 0x4c, 0x4f, 0x4e, 0x45, 0x00, 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18,
		0xb9, 0xc4, 0x55, 0x2a, 0x27, 0x10, 0x06, 0x29, 0x18, 0x96, 0x0a, 0x3e, 0x60, 0x8c, 0x29, 0xb9,
		0xaa, 0x8a, 0x5e, 0x1e, 0x16, 0x5b, 0x6d, 0x07, 0x5d, 0xe4, 0xe9, 0xbb, 0x36, 0x7f, 0xd6, 0xd4,
	}
)

func TestEncryptData(t *testing.T) {
	for _, test := range []struct {
		in       []byte
		expected []byte
	}{
		{[]byte{}, file0},
		{[]byte{1}, file1},
		{[]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, file16},
	} {
		c, err := newCipher(0, "")
		assert.NoError(t, err)
		c.cryptoRand = newRandomSource(1E8) // nodge the crypto rand generator

		// Check encode works
		buf := bytes.NewBuffer(test.in)
		encrypted, err := c.EncryptData(buf)
		assert.NoError(t, err)
		out, err := ioutil.ReadAll(encrypted)
		assert.NoError(t, err)
		assert.Equal(t, test.expected, out)

		// Check we can decode the data properly too...
		buf = bytes.NewBuffer(out)
		decrypted, err := c.DecryptData(ioutil.NopCloser(buf))
		assert.NoError(t, err)
		out, err = ioutil.ReadAll(decrypted)
		assert.NoError(t, err)
		assert.Equal(t, test.in, out)
	}
}

func TestNewEncrypter(t *testing.T) {
	c, err := newCipher(0, "")
	assert.NoError(t, err)
	c.cryptoRand = newRandomSource(1E8) // nodge the crypto rand generator

	z := &zeroes{}

	fh, err := c.newEncrypter(z)
	assert.NoError(t, err)
	assert.Equal(t, nonce{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18}, fh.nonce)
	assert.Equal(t, []byte{'R', 'C', 'L', 'O', 'N', 'E', 0x00, 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18}, fh.buf[:32])

	// Test error path
	c.cryptoRand = bytes.NewBufferString("123456789abcdefghijklmn")
	fh, err = c.newEncrypter(z)
	assert.Nil(t, fh)
	assert.Error(t, err, "short read of nonce")

}

type errorReader struct {
	err error
}

func (er errorReader) Read(p []byte) (n int, err error) {
	return 0, er.err
}

type closeDetector struct {
	io.Reader
	closed int
}

func newCloseDetector(in io.Reader) *closeDetector {
	return &closeDetector{
		Reader: in,
	}
}

func (c *closeDetector) Close() error {
	c.closed++
	return nil
}

func TestNewDecrypter(t *testing.T) {
	c, err := newCipher(0, "")
	assert.NoError(t, err)
	c.cryptoRand = newRandomSource(1E8) // nodge the crypto rand generator

	cd := newCloseDetector(bytes.NewBuffer(file0))
	fh, err := c.newDecrypter(cd)
	assert.NoError(t, err)
	// check nonce is in place
	assert.Equal(t, file0[8:32], fh.nonce[:])
	assert.Equal(t, 0, cd.closed)

	// Test error paths
	for i := range file0 {
		cd := newCloseDetector(bytes.NewBuffer(file0[:i]))
		fh, err = c.newDecrypter(cd)
		assert.Nil(t, fh)
		assert.Error(t, err, ErrorEncryptedFileTooShort.Error())
		assert.Equal(t, 1, cd.closed)
	}

	er := &errorReader{errors.New("potato")}
	cd = newCloseDetector(er)
	fh, err = c.newDecrypter(cd)
	assert.Nil(t, fh)
	assert.Error(t, err, "potato")
	assert.Equal(t, 1, cd.closed)

	// bad magic
	file0copy := make([]byte, len(file0))
	copy(file0copy, file0)
	for i := range fileMagic {
		file0copy[i] ^= 0x1
		cd := newCloseDetector(bytes.NewBuffer(file0copy))
		fh, err := c.newDecrypter(cd)
		assert.Nil(t, fh)
		assert.Error(t, err, ErrorEncryptedBadMagic.Error())
		file0copy[i] ^= 0x1
		assert.Equal(t, 1, cd.closed)
	}
}

func TestDecrypterRead(t *testing.T) {
	c, err := newCipher(0, "")
	assert.NoError(t, err)

	// Test truncating the header
	for i := 1; i < blockHeaderSize; i++ {
		cd := newCloseDetector(bytes.NewBuffer(file1[:len(file1)-i]))
		fh, err := c.newDecrypter(cd)
		assert.NoError(t, err)
		_, err = ioutil.ReadAll(fh)
		assert.Error(t, err, ErrorEncryptedFileBadHeader.Error())
		assert.Equal(t, 0, cd.closed)
	}

	// Test producing an error on the file on Read the underlying file
	in1 := bytes.NewBuffer(file1)
	in2 := &errorReader{errors.New("potato")}
	in := io.MultiReader(in1, in2)
	cd := newCloseDetector(in)
	fh, err := c.newDecrypter(cd)
	assert.NoError(t, err)
	_, err = ioutil.ReadAll(fh)
	assert.Error(t, err, "potato")
	assert.Equal(t, 0, cd.closed)

	// Test corrupting the input
	// shouldn't be able to corrupt any byte without some sort of error
	file16copy := make([]byte, len(file16))
	copy(file16copy, file16)
	for i := range file16copy {
		file16copy[i] ^= 0xFF
		fh, err := c.newDecrypter(ioutil.NopCloser(bytes.NewBuffer(file16copy)))
		if i < fileMagicSize {
			assert.Error(t, err, ErrorEncryptedBadMagic.Error())
			assert.Nil(t, fh)
		} else {
			assert.NoError(t, err)
			_, err = ioutil.ReadAll(fh)
			assert.Error(t, err, ErrorEncryptedFileBadHeader.Error())
		}
		file16copy[i] ^= 0xFF
	}
}

func TestDecrypterClose(t *testing.T) {
	c, err := newCipher(0, "")
	assert.NoError(t, err)

	cd := newCloseDetector(bytes.NewBuffer(file16))
	fh, err := c.newDecrypter(cd)
	assert.NoError(t, err)
	assert.Equal(t, 0, cd.closed)

	// close before reading
	assert.Equal(t, nil, fh.err)
	err = fh.Close()
	assert.Equal(t, ErrorFileClosed, fh.err)
	assert.Equal(t, 1, cd.closed)

	// double close
	err = fh.Close()
	assert.Error(t, err, ErrorFileClosed.Error())
	assert.Equal(t, 1, cd.closed)

	// try again reading the file this time
	cd = newCloseDetector(bytes.NewBuffer(file1))
	fh, err = c.newDecrypter(cd)
	assert.NoError(t, err)
	assert.Equal(t, 0, cd.closed)

	// close after reading
	out, err := ioutil.ReadAll(fh)
	assert.NoError(t, err)
	assert.Equal(t, []byte{1}, out)
	assert.Equal(t, io.EOF, fh.err)
	err = fh.Close()
	assert.Equal(t, ErrorFileClosed, fh.err)
	assert.Equal(t, 1, cd.closed)
}

func TestPutGetBlock(t *testing.T) {
	c, err := newCipher(0, "")
	assert.NoError(t, err)

	block := c.getBlock()
	c.putBlock(block)
	c.putBlock(block)

	assert.Panics(t, func() { c.putBlock(block[:len(block)-1]) })
}

func TestKey(t *testing.T) {
	c, err := newCipher(0, "")
	assert.NoError(t, err)

	// Check zero keys OK
	assert.Equal(t, [32]byte{}, c.dataKey)
	assert.Equal(t, [32]byte{}, c.nameKey)
	assert.Equal(t, [16]byte{}, c.nameTweak)

	require.NoError(t, c.Key("potato"))
	assert.Equal(t, [32]byte{0x74, 0x55, 0xC7, 0x1A, 0xB1, 0x7C, 0x86, 0x5B, 0x84, 0x71, 0xF4, 0x7B, 0x79, 0xAC, 0xB0, 0x7E, 0xB3, 0x1D, 0x56, 0x78, 0xB8, 0x0C, 0x7E, 0x2E, 0xAF, 0x4F, 0xC8, 0x06, 0x6A, 0x9E, 0xE4, 0x68}, c.dataKey)
	assert.Equal(t, [32]byte{0x76, 0x5D, 0xA2, 0x7A, 0xB1, 0x5D, 0x77, 0xF9, 0x57, 0x96, 0x71, 0x1F, 0x7B, 0x93, 0xAD, 0x63, 0xBB, 0xB4, 0x84, 0x07, 0x2E, 0x71, 0x80, 0xA8, 0xD1, 0x7A, 0x9B, 0xBE, 0xC1, 0x42, 0x70, 0xD0}, c.nameKey)
	assert.Equal(t, [16]byte{0xC1, 0x8D, 0x59, 0x32, 0xF5, 0x5B, 0x28, 0x28, 0xC5, 0xE1, 0xE8, 0x72, 0x15, 0x52, 0x03, 0x10}, c.nameTweak)

	require.NoError(t, c.Key("Potato"))
	assert.Equal(t, [32]byte{0xAE, 0xEA, 0x6A, 0xD3, 0x47, 0xDF, 0x75, 0xB9, 0x63, 0xCE, 0x12, 0xF5, 0x76, 0x23, 0xE9, 0x46, 0xD4, 0x2E, 0xD8, 0xBF, 0x3E, 0x92, 0x8B, 0x39, 0x24, 0x37, 0x94, 0x13, 0x3E, 0x5E, 0xF7, 0x5E}, c.dataKey)
	assert.Equal(t, [32]byte{0x54, 0xF7, 0x02, 0x6E, 0x8A, 0xFC, 0x56, 0x0A, 0x86, 0x63, 0x6A, 0xAB, 0x2C, 0x9C, 0x51, 0x62, 0xE5, 0x1A, 0x12, 0x23, 0x51, 0x83, 0x6E, 0xAF, 0x50, 0x42, 0x0F, 0x98, 0x1C, 0x86, 0x0A, 0x19}, c.nameKey)
	assert.Equal(t, [16]byte{0xF8, 0xC1, 0xB6, 0x27, 0x2D, 0x52, 0x9B, 0x4A, 0x8F, 0xDA, 0xEB, 0x42, 0x4A, 0x28, 0xDD, 0xF3}, c.nameTweak)

	require.NoError(t, c.Key(""))
	assert.Equal(t, [32]byte{}, c.dataKey)
	assert.Equal(t, [32]byte{}, c.nameKey)
	assert.Equal(t, [16]byte{}, c.nameTweak)
}
