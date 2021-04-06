package crypt

import (
	"bytes"
	"context"
	"encoding/base32"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/backend/crypt/pkcs7"
	"github.com/rclone/rclone/lib/readers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNameEncryptionMode(t *testing.T) {
	for _, test := range []struct {
		in          string
		expected    NameEncryptionMode
		expectedErr string
	}{
		{"off", NameEncryptionOff, ""},
		{"standard", NameEncryptionStandard, ""},
		{"obfuscate", NameEncryptionObfuscated, ""},
		{"potato", NameEncryptionOff, "Unknown file name encryption mode \"potato\""},
	} {
		actual, actualErr := NewNameEncryptionMode(test.in)
		assert.Equal(t, actual, test.expected)
		if test.expectedErr == "" {
			assert.NoError(t, actualErr)
		} else {
			assert.Error(t, actualErr, test.expectedErr)
		}
	}
}

func TestNewNameEncryptionModeString(t *testing.T) {
	assert.Equal(t, NameEncryptionOff.String(), "off")
	assert.Equal(t, NameEncryptionStandard.String(), "standard")
	assert.Equal(t, NameEncryptionObfuscated.String(), "obfuscate")
	assert.Equal(t, NameEncryptionMode(3).String(), "Unknown mode #3")
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
	c, _ := newCipher(NameEncryptionStandard, "", "", true)
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
	longName := make([]byte, 3328)
	for i := range longName {
		longName[i] = 'a'
	}
	c, _ := newCipher(NameEncryptionStandard, "", "", true)
	for _, test := range []struct {
		in          string
		expectedErr error
	}{
		{"64=", ErrorBadBase32Encoding},
		{"!", base32.CorruptInputError(0)},
		{string(longName), ErrorTooLongAfterDecode},
		{encodeFileName([]byte("a")), ErrorNotAMultipleOfBlocksize},
		{encodeFileName([]byte("123456789abcdef")), ErrorNotAMultipleOfBlocksize},
		{encodeFileName([]byte("123456789abcdef0")), pkcs7.ErrorPaddingTooLong},
	} {
		actual, actualErr := c.decryptSegment(test.in)
		assert.Equal(t, test.expectedErr, actualErr, fmt.Sprintf("in=%q got actual=%q, err = %v %T", test.in, actual, actualErr, actualErr))
	}
}

func TestEncryptFileName(t *testing.T) {
	// First standard mode
	c, _ := newCipher(NameEncryptionStandard, "", "", true)
	assert.Equal(t, "p0e52nreeaj0a5ea7s64m4j72s", c.EncryptFileName("1"))
	assert.Equal(t, "p0e52nreeaj0a5ea7s64m4j72s/l42g6771hnv3an9cgc8cr2n1ng", c.EncryptFileName("1/12"))
	assert.Equal(t, "p0e52nreeaj0a5ea7s64m4j72s/l42g6771hnv3an9cgc8cr2n1ng/qgm4avr35m5loi1th53ato71v0", c.EncryptFileName("1/12/123"))
	assert.Equal(t, "p0e52nreeaj0a5ea7s64m4j72s-v2001-02-03-040506-123", c.EncryptFileName("1-v2001-02-03-040506-123"))
	assert.Equal(t, "p0e52nreeaj0a5ea7s64m4j72s/l42g6771hnv3an9cgc8cr2n1ng-v2001-02-03-040506-123", c.EncryptFileName("1/12-v2001-02-03-040506-123"))
	// Standard mode with directory name encryption off
	c, _ = newCipher(NameEncryptionStandard, "", "", false)
	assert.Equal(t, "p0e52nreeaj0a5ea7s64m4j72s", c.EncryptFileName("1"))
	assert.Equal(t, "1/l42g6771hnv3an9cgc8cr2n1ng", c.EncryptFileName("1/12"))
	assert.Equal(t, "1/12/qgm4avr35m5loi1th53ato71v0", c.EncryptFileName("1/12/123"))
	assert.Equal(t, "p0e52nreeaj0a5ea7s64m4j72s-v2001-02-03-040506-123", c.EncryptFileName("1-v2001-02-03-040506-123"))
	assert.Equal(t, "1/l42g6771hnv3an9cgc8cr2n1ng-v2001-02-03-040506-123", c.EncryptFileName("1/12-v2001-02-03-040506-123"))
	// Now off mode
	c, _ = newCipher(NameEncryptionOff, "", "", true)
	assert.Equal(t, "1/12/123.bin", c.EncryptFileName("1/12/123"))
	// Obfuscation mode
	c, _ = newCipher(NameEncryptionObfuscated, "", "", true)
	assert.Equal(t, "49.6/99.23/150.890/53.!!lipps", c.EncryptFileName("1/12/123/!hello"))
	assert.Equal(t, "49.6/99.23/150.890/53-v2001-02-03-040506-123.!!lipps", c.EncryptFileName("1/12/123/!hello-v2001-02-03-040506-123"))
	assert.Equal(t, "49.6/99.23/150.890/162.uryyB-v2001-02-03-040506-123.GKG", c.EncryptFileName("1/12/123/hello-v2001-02-03-040506-123.txt"))
	assert.Equal(t, "161.\u00e4", c.EncryptFileName("\u00a1"))
	assert.Equal(t, "160.\u03c2", c.EncryptFileName("\u03a0"))
	// Obfuscation mode with directory name encryption off
	c, _ = newCipher(NameEncryptionObfuscated, "", "", false)
	assert.Equal(t, "1/12/123/53.!!lipps", c.EncryptFileName("1/12/123/!hello"))
	assert.Equal(t, "1/12/123/53-v2001-02-03-040506-123.!!lipps", c.EncryptFileName("1/12/123/!hello-v2001-02-03-040506-123"))
	assert.Equal(t, "161.\u00e4", c.EncryptFileName("\u00a1"))
	assert.Equal(t, "160.\u03c2", c.EncryptFileName("\u03a0"))
}

func TestDecryptFileName(t *testing.T) {
	for _, test := range []struct {
		mode           NameEncryptionMode
		dirNameEncrypt bool
		in             string
		expected       string
		expectedErr    error
	}{
		{NameEncryptionStandard, true, "p0e52nreeaj0a5ea7s64m4j72s", "1", nil},
		{NameEncryptionStandard, true, "p0e52nreeaj0a5ea7s64m4j72s/l42g6771hnv3an9cgc8cr2n1ng", "1/12", nil},
		{NameEncryptionStandard, true, "p0e52nreeAJ0A5EA7S64M4J72S/L42G6771HNv3an9cgc8cr2n1ng", "1/12", nil},
		{NameEncryptionStandard, true, "p0e52nreeaj0a5ea7s64m4j72s/l42g6771hnv3an9cgc8cr2n1ng/qgm4avr35m5loi1th53ato71v0", "1/12/123", nil},
		{NameEncryptionStandard, true, "p0e52nreeaj0a5ea7s64m4j72s/l42g6771hnv3an9cgc8cr2n1/qgm4avr35m5loi1th53ato71v0", "", ErrorNotAMultipleOfBlocksize},
		{NameEncryptionStandard, false, "1/12/qgm4avr35m5loi1th53ato71v0", "1/12/123", nil},
		{NameEncryptionStandard, true, "p0e52nreeaj0a5ea7s64m4j72s-v2001-02-03-040506-123", "1-v2001-02-03-040506-123", nil},
		{NameEncryptionOff, true, "1/12/123.bin", "1/12/123", nil},
		{NameEncryptionOff, true, "1/12/123.bix", "", ErrorNotAnEncryptedFile},
		{NameEncryptionOff, true, ".bin", "", ErrorNotAnEncryptedFile},
		{NameEncryptionOff, true, "1/12/123-v2001-02-03-040506-123.bin", "1/12/123-v2001-02-03-040506-123", nil},
		{NameEncryptionOff, true, "1/12/123-v1970-01-01-010101-123-v2001-02-03-040506-123.bin", "1/12/123-v1970-01-01-010101-123-v2001-02-03-040506-123", nil},
		{NameEncryptionOff, true, "1/12/123-v1970-01-01-010101-123-v2001-02-03-040506-123.txt.bin", "1/12/123-v1970-01-01-010101-123-v2001-02-03-040506-123.txt", nil},
		{NameEncryptionObfuscated, true, "!.hello", "hello", nil},
		{NameEncryptionObfuscated, true, "hello", "", ErrorNotAnEncryptedFile},
		{NameEncryptionObfuscated, true, "161.\u00e4", "\u00a1", nil},
		{NameEncryptionObfuscated, true, "160.\u03c2", "\u03a0", nil},
		{NameEncryptionObfuscated, false, "1/12/123/53.!!lipps", "1/12/123/!hello", nil},
		{NameEncryptionObfuscated, false, "1/12/123/53-v2001-02-03-040506-123.!!lipps", "1/12/123/!hello-v2001-02-03-040506-123", nil},
	} {
		c, _ := newCipher(test.mode, "", "", test.dirNameEncrypt)
		actual, actualErr := c.DecryptFileName(test.in)
		what := fmt.Sprintf("Testing %q (mode=%v)", test.in, test.mode)
		assert.Equal(t, test.expected, actual, what)
		assert.Equal(t, test.expectedErr, actualErr, what)
	}
}

func TestEncDecMatches(t *testing.T) {
	for _, test := range []struct {
		mode NameEncryptionMode
		in   string
	}{
		{NameEncryptionStandard, "1/2/3/4"},
		{NameEncryptionOff, "1/2/3/4"},
		{NameEncryptionObfuscated, "1/2/3/4/!hello\u03a0"},
		{NameEncryptionObfuscated, "Avatar The Last Airbender"},
	} {
		c, _ := newCipher(test.mode, "", "", true)
		out, err := c.DecryptFileName(c.EncryptFileName(test.in))
		what := fmt.Sprintf("Testing %q (mode=%v)", test.in, test.mode)
		assert.Equal(t, out, test.in, what)
		assert.Equal(t, err, nil, what)
	}
}

func TestEncryptDirName(t *testing.T) {
	// First standard mode
	c, _ := newCipher(NameEncryptionStandard, "", "", true)
	assert.Equal(t, "p0e52nreeaj0a5ea7s64m4j72s", c.EncryptDirName("1"))
	assert.Equal(t, "p0e52nreeaj0a5ea7s64m4j72s/l42g6771hnv3an9cgc8cr2n1ng", c.EncryptDirName("1/12"))
	assert.Equal(t, "p0e52nreeaj0a5ea7s64m4j72s/l42g6771hnv3an9cgc8cr2n1ng/qgm4avr35m5loi1th53ato71v0", c.EncryptDirName("1/12/123"))
	// Standard mode with dir name encryption off
	c, _ = newCipher(NameEncryptionStandard, "", "", false)
	assert.Equal(t, "1/12", c.EncryptDirName("1/12"))
	assert.Equal(t, "1/12/123", c.EncryptDirName("1/12/123"))
	// Now off mode
	c, _ = newCipher(NameEncryptionOff, "", "", true)
	assert.Equal(t, "1/12/123", c.EncryptDirName("1/12/123"))
}

func TestDecryptDirName(t *testing.T) {
	for _, test := range []struct {
		mode           NameEncryptionMode
		dirNameEncrypt bool
		in             string
		expected       string
		expectedErr    error
	}{
		{NameEncryptionStandard, true, "p0e52nreeaj0a5ea7s64m4j72s", "1", nil},
		{NameEncryptionStandard, true, "p0e52nreeaj0a5ea7s64m4j72s/l42g6771hnv3an9cgc8cr2n1ng", "1/12", nil},
		{NameEncryptionStandard, true, "p0e52nreeAJ0A5EA7S64M4J72S/L42G6771HNv3an9cgc8cr2n1ng", "1/12", nil},
		{NameEncryptionStandard, true, "p0e52nreeaj0a5ea7s64m4j72s/l42g6771hnv3an9cgc8cr2n1ng/qgm4avr35m5loi1th53ato71v0", "1/12/123", nil},
		{NameEncryptionStandard, true, "p0e52nreeaj0a5ea7s64m4j72s/l42g6771hnv3an9cgc8cr2n1/qgm4avr35m5loi1th53ato71v0", "", ErrorNotAMultipleOfBlocksize},
		{NameEncryptionStandard, false, "p0e52nreeaj0a5ea7s64m4j72s/l42g6771hnv3an9cgc8cr2n1ng", "p0e52nreeaj0a5ea7s64m4j72s/l42g6771hnv3an9cgc8cr2n1ng", nil},
		{NameEncryptionStandard, false, "1/12/123", "1/12/123", nil},
		{NameEncryptionOff, true, "1/12/123.bin", "1/12/123.bin", nil},
		{NameEncryptionOff, true, "1/12/123", "1/12/123", nil},
		{NameEncryptionOff, true, ".bin", ".bin", nil},
	} {
		c, _ := newCipher(test.mode, "", "", test.dirNameEncrypt)
		actual, actualErr := c.DecryptDirName(test.in)
		what := fmt.Sprintf("Testing %q (mode=%v)", test.in, test.mode)
		assert.Equal(t, test.expected, actual, what)
		assert.Equal(t, test.expectedErr, actualErr, what)
	}
}

func TestEncryptedSize(t *testing.T) {
	c, _ := newCipher(NameEncryptionStandard, "", "", true)
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
	c, _ := newCipher(NameEncryptionStandard, "", "", true)
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

func TestNonceAdd(t *testing.T) {
	for _, test := range []struct {
		add uint64
		in  nonce
		out nonce
	}{
		{
			0x01,
			nonce{0x00},
			nonce{0x01},
		},
		{
			0xFF,
			nonce{0xFF},
			nonce{0xFE, 0x01},
		},
		{
			0xFFFF,
			nonce{0xFF, 0xFF},
			nonce{0xFE, 0xFF, 0x01},
		},
		{
			0xFFFFFF,
			nonce{0xFF, 0xFF, 0xFF},
			nonce{0xFE, 0xFF, 0xFF, 0x01},
		},
		{
			0xFFFFFFFF,
			nonce{0xFF, 0xFF, 0xFF, 0xFF},
			nonce{0xFe, 0xFF, 0xFF, 0xFF, 0x01},
		},
		{
			0xFFFFFFFFFF,
			nonce{0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			nonce{0xFE, 0xFF, 0xFF, 0xFF, 0xFF, 0x01},
		},
		{
			0xFFFFFFFFFFFF,
			nonce{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			nonce{0xFE, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x01},
		},
		{
			0xFFFFFFFFFFFFFF,
			nonce{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			nonce{0xFE, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x01},
		},
		{
			0xFFFFFFFFFFFFFFFF,
			nonce{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			nonce{0xFE, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x01},
		},
		{
			0xFFFFFFFFFFFFFFFF,
			nonce{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			nonce{0xFE, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0x01},
		},
		{
			0xFFFFFFFFFFFFFFFF,
			nonce{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			nonce{0xFE, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0x00, 0x01},
		},
		{
			0xFFFFFFFFFFFFFFFF,
			nonce{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			nonce{0xFE, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0x00, 0x00, 0x01},
		},
		{
			0xFFFFFFFFFFFFFFFF,
			nonce{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			nonce{0xFE, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0x00, 0x00, 0x00, 0x01},
		},
		{
			0xFFFFFFFFFFFFFFFF,
			nonce{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			nonce{0xFE, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
		},
		{
			0xFFFFFFFFFFFFFFFF,
			nonce{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			nonce{0xFE, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
		},
		{
			0xFFFFFFFFFFFFFFFF,
			nonce{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			nonce{0xFE, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
		},
		{
			0xFFFFFFFFFFFFFFFF,
			nonce{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			nonce{0xFE, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
		},
		{
			0xFFFFFFFFFFFFFFFF,
			nonce{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			nonce{0xFE, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
		},
		{
			0xFFFFFFFFFFFFFFFF,
			nonce{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			nonce{0xFE, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
		},
		{
			0xFFFFFFFFFFFFFFFF,
			nonce{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			nonce{0xFE, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
		},
		{
			0xFFFFFFFFFFFFFFFF,
			nonce{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			nonce{0xFE, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
		},
		{
			0xFFFFFFFFFFFFFFFF,
			nonce{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			nonce{0xFE, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
		},
		{
			0xFFFFFFFFFFFFFFFF,
			nonce{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			nonce{0xFE, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
		},
		{
			0xFFFFFFFFFFFFFFFF,
			nonce{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			nonce{0xFE, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
		},
		{
			0xFFFFFFFFFFFFFFFF,
			nonce{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			nonce{0xFE, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		},
	} {
		x := test.in
		x.add(test.add)
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
	source := newRandomSource(1e8)
	sink := newRandomSource(1e8)
	n, err := io.Copy(sink, source)
	assert.NoError(t, err)
	assert.Equal(t, int64(1e8), n)

	source = newRandomSource(1e8)
	buf := make([]byte, 16)
	_, _ = source.Read(buf)
	sink = newRandomSource(1e8)
	_, err = io.Copy(sink, source)
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
	c, err := newCipher(NameEncryptionStandard, "", "", true)
	assert.NoError(t, err)
	c.cryptoRand = &zeroes{} // zero out the nonce
	buf := make([]byte, bufSize)
	source := newRandomSource(copySize)
	encrypted, err := c.newEncrypter(source, nil)
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
	testEncryptDecrypt(t, 1, 1e7)
}

func TestEncryptDecrypt32(t *testing.T) {
	testEncryptDecrypt(t, 32, 1e8)
}

func TestEncryptDecrypt4096(t *testing.T) {
	testEncryptDecrypt(t, 4096, 1e8)
}

func TestEncryptDecrypt65536(t *testing.T) {
	testEncryptDecrypt(t, 65536, 1e8)
}

func TestEncryptDecrypt65537(t *testing.T) {
	testEncryptDecrypt(t, 65537, 1e8)
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
		c, err := newCipher(NameEncryptionStandard, "", "", true)
		assert.NoError(t, err)
		c.cryptoRand = newRandomSource(1e8) // nodge the crypto rand generator

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
	c, err := newCipher(NameEncryptionStandard, "", "", true)
	assert.NoError(t, err)
	c.cryptoRand = newRandomSource(1e8) // nodge the crypto rand generator

	z := &zeroes{}

	fh, err := c.newEncrypter(z, nil)
	assert.NoError(t, err)
	assert.Equal(t, nonce{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18}, fh.nonce)
	assert.Equal(t, []byte{'R', 'C', 'L', 'O', 'N', 'E', 0x00, 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18}, fh.buf[:32])

	// Test error path
	c.cryptoRand = bytes.NewBufferString("123456789abcdefghijklmn")
	fh, err = c.newEncrypter(z, nil)
	assert.Nil(t, fh)
	assert.Error(t, err, "short read of nonce")

}

// Test the stream returning 0, io.ErrUnexpectedEOF - this used to
// cause a fatal loop
func TestNewEncrypterErrUnexpectedEOF(t *testing.T) {
	c, err := newCipher(NameEncryptionStandard, "", "", true)
	assert.NoError(t, err)

	in := &readers.ErrorReader{Err: io.ErrUnexpectedEOF}
	fh, err := c.newEncrypter(in, nil)
	assert.NoError(t, err)

	n, err := io.CopyN(ioutil.Discard, fh, 1e6)
	assert.Equal(t, io.ErrUnexpectedEOF, err)
	assert.Equal(t, int64(32), n)
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
	c, err := newCipher(NameEncryptionStandard, "", "", true)
	assert.NoError(t, err)
	c.cryptoRand = newRandomSource(1e8) // nodge the crypto rand generator

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

	er := &readers.ErrorReader{Err: errors.New("potato")}
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

// Test the stream returning 0, io.ErrUnexpectedEOF
func TestNewDecrypterErrUnexpectedEOF(t *testing.T) {
	c, err := newCipher(NameEncryptionStandard, "", "", true)
	assert.NoError(t, err)

	in2 := &readers.ErrorReader{Err: io.ErrUnexpectedEOF}
	in1 := bytes.NewBuffer(file16)
	in := ioutil.NopCloser(io.MultiReader(in1, in2))

	fh, err := c.newDecrypter(in)
	assert.NoError(t, err)

	n, err := io.CopyN(ioutil.Discard, fh, 1e6)
	assert.Equal(t, io.ErrUnexpectedEOF, err)
	assert.Equal(t, int64(16), n)
}

func TestNewDecrypterSeekLimit(t *testing.T) {
	c, err := newCipher(NameEncryptionStandard, "", "", true)
	assert.NoError(t, err)
	c.cryptoRand = &zeroes{} // nodge the crypto rand generator

	// Make random data
	const dataSize = 150000
	plaintext, err := ioutil.ReadAll(newRandomSource(dataSize))
	assert.NoError(t, err)

	// Encrypt the data
	buf := bytes.NewBuffer(plaintext)
	encrypted, err := c.EncryptData(buf)
	assert.NoError(t, err)
	ciphertext, err := ioutil.ReadAll(encrypted)
	assert.NoError(t, err)

	trials := []int{0, 1, 2, 3, 4, 5, 7, 8, 9, 15, 16, 17, 31, 32, 33, 63, 64, 65,
		127, 128, 129, 255, 256, 257, 511, 512, 513, 1023, 1024, 1025, 2047, 2048, 2049,
		4095, 4096, 4097, 8191, 8192, 8193, 16383, 16384, 16385, 32767, 32768, 32769,
		65535, 65536, 65537, 131071, 131072, 131073, dataSize - 1, dataSize}
	limits := []int{-1, 0, 1, 65535, 65536, 65537, 131071, 131072, 131073}

	// Open stream with a seek of underlyingOffset
	var reader io.ReadCloser
	open := func(ctx context.Context, underlyingOffset, underlyingLimit int64) (io.ReadCloser, error) {
		end := len(ciphertext)
		if underlyingLimit >= 0 {
			end = int(underlyingOffset + underlyingLimit)
			if end > len(ciphertext) {
				end = len(ciphertext)
			}
		}
		reader = ioutil.NopCloser(bytes.NewBuffer(ciphertext[int(underlyingOffset):end]))
		return reader, nil
	}

	inBlock := make([]byte, dataSize)

	// Check the seek worked by reading a block and checking it
	// against what it should be
	check := func(rc io.Reader, offset, limit int) {
		n, err := io.ReadFull(rc, inBlock)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			require.NoError(t, err)
		}
		seekedDecrypted := inBlock[:n]

		what := fmt.Sprintf("offset = %d, limit = %d", offset, limit)
		if limit >= 0 {
			assert.Equal(t, limit, n, what)
		}
		require.Equal(t, plaintext[offset:offset+n], seekedDecrypted, what)

		// We should have completely emptied the reader at this point
		n, err = reader.Read(inBlock)
		assert.Equal(t, io.EOF, err)
		assert.Equal(t, 0, n)
	}

	// Now try decoding it with an open/seek
	for _, offset := range trials {
		for _, limit := range limits {
			if offset+limit > len(plaintext) {
				continue
			}
			rc, err := c.DecryptDataSeek(context.Background(), open, int64(offset), int64(limit))
			assert.NoError(t, err)

			check(rc, offset, limit)
		}
	}

	// Try decoding it with a single open and lots of seeks
	fh, err := c.DecryptDataSeek(context.Background(), open, 0, -1)
	assert.NoError(t, err)
	for _, offset := range trials {
		for _, limit := range limits {
			if offset+limit > len(plaintext) {
				continue
			}
			_, err := fh.RangeSeek(context.Background(), int64(offset), io.SeekStart, int64(limit))
			assert.NoError(t, err)

			check(fh, offset, limit)
		}
	}

	// Do some checks on the open callback
	for _, test := range []struct {
		offset, limit         int64
		wantOffset, wantLimit int64
	}{
		// unlimited
		{0, -1, int64(fileHeaderSize), -1},
		{1, -1, int64(fileHeaderSize), -1},
		{blockDataSize - 1, -1, int64(fileHeaderSize), -1},
		{blockDataSize, -1, int64(fileHeaderSize) + blockSize, -1},
		{blockDataSize + 1, -1, int64(fileHeaderSize) + blockSize, -1},
		// limit=1
		{0, 1, int64(fileHeaderSize), blockSize},
		{1, 1, int64(fileHeaderSize), blockSize},
		{blockDataSize - 1, 1, int64(fileHeaderSize), blockSize},
		{blockDataSize, 1, int64(fileHeaderSize) + blockSize, blockSize},
		{blockDataSize + 1, 1, int64(fileHeaderSize) + blockSize, blockSize},
		// limit=100
		{0, 100, int64(fileHeaderSize), blockSize},
		{1, 100, int64(fileHeaderSize), blockSize},
		{blockDataSize - 1, 100, int64(fileHeaderSize), 2 * blockSize},
		{blockDataSize, 100, int64(fileHeaderSize) + blockSize, blockSize},
		{blockDataSize + 1, 100, int64(fileHeaderSize) + blockSize, blockSize},
		// limit=blockDataSize-1
		{0, blockDataSize - 1, int64(fileHeaderSize), blockSize},
		{1, blockDataSize - 1, int64(fileHeaderSize), blockSize},
		{blockDataSize - 1, blockDataSize - 1, int64(fileHeaderSize), 2 * blockSize},
		{blockDataSize, blockDataSize - 1, int64(fileHeaderSize) + blockSize, blockSize},
		{blockDataSize + 1, blockDataSize - 1, int64(fileHeaderSize) + blockSize, blockSize},
		// limit=blockDataSize
		{0, blockDataSize, int64(fileHeaderSize), blockSize},
		{1, blockDataSize, int64(fileHeaderSize), 2 * blockSize},
		{blockDataSize - 1, blockDataSize, int64(fileHeaderSize), 2 * blockSize},
		{blockDataSize, blockDataSize, int64(fileHeaderSize) + blockSize, blockSize},
		{blockDataSize + 1, blockDataSize, int64(fileHeaderSize) + blockSize, 2 * blockSize},
		// limit=blockDataSize+1
		{0, blockDataSize + 1, int64(fileHeaderSize), 2 * blockSize},
		{1, blockDataSize + 1, int64(fileHeaderSize), 2 * blockSize},
		{blockDataSize - 1, blockDataSize + 1, int64(fileHeaderSize), 2 * blockSize},
		{blockDataSize, blockDataSize + 1, int64(fileHeaderSize) + blockSize, 2 * blockSize},
		{blockDataSize + 1, blockDataSize + 1, int64(fileHeaderSize) + blockSize, 2 * blockSize},
	} {
		what := fmt.Sprintf("offset = %d, limit = %d", test.offset, test.limit)
		callCount := 0
		testOpen := func(ctx context.Context, underlyingOffset, underlyingLimit int64) (io.ReadCloser, error) {
			switch callCount {
			case 0:
				assert.Equal(t, int64(0), underlyingOffset, what)
				assert.Equal(t, int64(-1), underlyingLimit, what)
			case 1:
				assert.Equal(t, test.wantOffset, underlyingOffset, what)
				assert.Equal(t, test.wantLimit, underlyingLimit, what)
			default:
				t.Errorf("Too many calls %d for %s", callCount+1, what)
			}
			callCount++
			return open(ctx, underlyingOffset, underlyingLimit)
		}
		fh, err := c.DecryptDataSeek(context.Background(), testOpen, 0, -1)
		assert.NoError(t, err)
		gotOffset, err := fh.RangeSeek(context.Background(), test.offset, io.SeekStart, test.limit)
		assert.NoError(t, err)
		assert.Equal(t, gotOffset, test.offset)
	}
}

func TestDecrypterCalculateUnderlying(t *testing.T) {
	for _, test := range []struct {
		offset, limit           int64
		wantOffset, wantLimit   int64
		wantDiscard, wantBlocks int64
	}{
		// unlimited
		{0, -1, int64(fileHeaderSize), -1, 0, 0},
		{1, -1, int64(fileHeaderSize), -1, 1, 0},
		{blockDataSize - 1, -1, int64(fileHeaderSize), -1, blockDataSize - 1, 0},
		{blockDataSize, -1, int64(fileHeaderSize) + blockSize, -1, 0, 1},
		{blockDataSize + 1, -1, int64(fileHeaderSize) + blockSize, -1, 1, 1},
		// limit=1
		{0, 1, int64(fileHeaderSize), blockSize, 0, 0},
		{1, 1, int64(fileHeaderSize), blockSize, 1, 0},
		{blockDataSize - 1, 1, int64(fileHeaderSize), blockSize, blockDataSize - 1, 0},
		{blockDataSize, 1, int64(fileHeaderSize) + blockSize, blockSize, 0, 1},
		{blockDataSize + 1, 1, int64(fileHeaderSize) + blockSize, blockSize, 1, 1},
		// limit=100
		{0, 100, int64(fileHeaderSize), blockSize, 0, 0},
		{1, 100, int64(fileHeaderSize), blockSize, 1, 0},
		{blockDataSize - 1, 100, int64(fileHeaderSize), 2 * blockSize, blockDataSize - 1, 0},
		{blockDataSize, 100, int64(fileHeaderSize) + blockSize, blockSize, 0, 1},
		{blockDataSize + 1, 100, int64(fileHeaderSize) + blockSize, blockSize, 1, 1},
		// limit=blockDataSize-1
		{0, blockDataSize - 1, int64(fileHeaderSize), blockSize, 0, 0},
		{1, blockDataSize - 1, int64(fileHeaderSize), blockSize, 1, 0},
		{blockDataSize - 1, blockDataSize - 1, int64(fileHeaderSize), 2 * blockSize, blockDataSize - 1, 0},
		{blockDataSize, blockDataSize - 1, int64(fileHeaderSize) + blockSize, blockSize, 0, 1},
		{blockDataSize + 1, blockDataSize - 1, int64(fileHeaderSize) + blockSize, blockSize, 1, 1},
		// limit=blockDataSize
		{0, blockDataSize, int64(fileHeaderSize), blockSize, 0, 0},
		{1, blockDataSize, int64(fileHeaderSize), 2 * blockSize, 1, 0},
		{blockDataSize - 1, blockDataSize, int64(fileHeaderSize), 2 * blockSize, blockDataSize - 1, 0},
		{blockDataSize, blockDataSize, int64(fileHeaderSize) + blockSize, blockSize, 0, 1},
		{blockDataSize + 1, blockDataSize, int64(fileHeaderSize) + blockSize, 2 * blockSize, 1, 1},
		// limit=blockDataSize+1
		{0, blockDataSize + 1, int64(fileHeaderSize), 2 * blockSize, 0, 0},
		{1, blockDataSize + 1, int64(fileHeaderSize), 2 * blockSize, 1, 0},
		{blockDataSize - 1, blockDataSize + 1, int64(fileHeaderSize), 2 * blockSize, blockDataSize - 1, 0},
		{blockDataSize, blockDataSize + 1, int64(fileHeaderSize) + blockSize, 2 * blockSize, 0, 1},
		{blockDataSize + 1, blockDataSize + 1, int64(fileHeaderSize) + blockSize, 2 * blockSize, 1, 1},
	} {
		what := fmt.Sprintf("offset = %d, limit = %d", test.offset, test.limit)
		underlyingOffset, underlyingLimit, discard, blocks := calculateUnderlying(test.offset, test.limit)
		assert.Equal(t, test.wantOffset, underlyingOffset, what)
		assert.Equal(t, test.wantLimit, underlyingLimit, what)
		assert.Equal(t, test.wantDiscard, discard, what)
		assert.Equal(t, test.wantBlocks, blocks, what)
	}
}

func TestDecrypterRead(t *testing.T) {
	c, err := newCipher(NameEncryptionStandard, "", "", true)
	assert.NoError(t, err)

	// Test truncating the file at each possible point
	for i := 0; i < len(file16)-1; i++ {
		what := fmt.Sprintf("truncating to %d/%d", i, len(file16))
		cd := newCloseDetector(bytes.NewBuffer(file16[:i]))
		fh, err := c.newDecrypter(cd)
		if i < fileHeaderSize {
			assert.EqualError(t, err, ErrorEncryptedFileTooShort.Error(), what)
			continue
		}
		if err != nil {
			assert.NoError(t, err, what)
			continue
		}
		_, err = ioutil.ReadAll(fh)
		var expectedErr error
		switch {
		case i == fileHeaderSize:
			// This would normally produce an error *except* on the first block
			expectedErr = nil
		default:
			expectedErr = io.ErrUnexpectedEOF
		}
		if expectedErr != nil {
			assert.EqualError(t, err, expectedErr.Error(), what)
		} else {
			assert.NoError(t, err, what)
		}
		assert.Equal(t, 0, cd.closed, what)
	}

	// Test producing an error on the file on Read the underlying file
	in1 := bytes.NewBuffer(file1)
	in2 := &readers.ErrorReader{Err: errors.New("potato")}
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
	c, err := newCipher(NameEncryptionStandard, "", "", true)
	assert.NoError(t, err)

	cd := newCloseDetector(bytes.NewBuffer(file16))
	fh, err := c.newDecrypter(cd)
	assert.NoError(t, err)
	assert.Equal(t, 0, cd.closed)

	// close before reading
	assert.Equal(t, nil, fh.err)
	err = fh.Close()
	assert.NoError(t, err)
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
	assert.NoError(t, err)
	assert.Equal(t, ErrorFileClosed, fh.err)
	assert.Equal(t, 1, cd.closed)
}

func TestPutGetBlock(t *testing.T) {
	c, err := newCipher(NameEncryptionStandard, "", "", true)
	assert.NoError(t, err)

	block := c.getBlock()
	c.putBlock(block)
	c.putBlock(block)

	assert.Panics(t, func() { c.putBlock(block[:len(block)-1]) })
}

func TestKey(t *testing.T) {
	c, err := newCipher(NameEncryptionStandard, "", "", true)
	assert.NoError(t, err)

	// Check zero keys OK
	assert.Equal(t, [32]byte{}, c.dataKey)
	assert.Equal(t, [32]byte{}, c.nameKey)
	assert.Equal(t, [16]byte{}, c.nameTweak)

	require.NoError(t, c.Key("potato", ""))
	assert.Equal(t, [32]byte{0x74, 0x55, 0xC7, 0x1A, 0xB1, 0x7C, 0x86, 0x5B, 0x84, 0x71, 0xF4, 0x7B, 0x79, 0xAC, 0xB0, 0x7E, 0xB3, 0x1D, 0x56, 0x78, 0xB8, 0x0C, 0x7E, 0x2E, 0xAF, 0x4F, 0xC8, 0x06, 0x6A, 0x9E, 0xE4, 0x68}, c.dataKey)
	assert.Equal(t, [32]byte{0x76, 0x5D, 0xA2, 0x7A, 0xB1, 0x5D, 0x77, 0xF9, 0x57, 0x96, 0x71, 0x1F, 0x7B, 0x93, 0xAD, 0x63, 0xBB, 0xB4, 0x84, 0x07, 0x2E, 0x71, 0x80, 0xA8, 0xD1, 0x7A, 0x9B, 0xBE, 0xC1, 0x42, 0x70, 0xD0}, c.nameKey)
	assert.Equal(t, [16]byte{0xC1, 0x8D, 0x59, 0x32, 0xF5, 0x5B, 0x28, 0x28, 0xC5, 0xE1, 0xE8, 0x72, 0x15, 0x52, 0x03, 0x10}, c.nameTweak)

	require.NoError(t, c.Key("Potato", ""))
	assert.Equal(t, [32]byte{0xAE, 0xEA, 0x6A, 0xD3, 0x47, 0xDF, 0x75, 0xB9, 0x63, 0xCE, 0x12, 0xF5, 0x76, 0x23, 0xE9, 0x46, 0xD4, 0x2E, 0xD8, 0xBF, 0x3E, 0x92, 0x8B, 0x39, 0x24, 0x37, 0x94, 0x13, 0x3E, 0x5E, 0xF7, 0x5E}, c.dataKey)
	assert.Equal(t, [32]byte{0x54, 0xF7, 0x02, 0x6E, 0x8A, 0xFC, 0x56, 0x0A, 0x86, 0x63, 0x6A, 0xAB, 0x2C, 0x9C, 0x51, 0x62, 0xE5, 0x1A, 0x12, 0x23, 0x51, 0x83, 0x6E, 0xAF, 0x50, 0x42, 0x0F, 0x98, 0x1C, 0x86, 0x0A, 0x19}, c.nameKey)
	assert.Equal(t, [16]byte{0xF8, 0xC1, 0xB6, 0x27, 0x2D, 0x52, 0x9B, 0x4A, 0x8F, 0xDA, 0xEB, 0x42, 0x4A, 0x28, 0xDD, 0xF3}, c.nameTweak)

	require.NoError(t, c.Key("potato", "sausage"))
	assert.Equal(t, [32]uint8{0x8e, 0x9b, 0x6b, 0x99, 0xf8, 0x69, 0x4, 0x67, 0xa0, 0x71, 0xf9, 0xcb, 0x92, 0xd0, 0xaa, 0x78, 0x7f, 0x8f, 0xf1, 0x78, 0xbe, 0xc9, 0x6f, 0x99, 0x9f, 0xd5, 0x20, 0x6e, 0x64, 0x4a, 0x1b, 0x50}, c.dataKey)
	assert.Equal(t, [32]uint8{0x3e, 0xa9, 0x5e, 0xf6, 0x81, 0x78, 0x2d, 0xc9, 0xd9, 0x95, 0x5d, 0x22, 0x5b, 0xfd, 0x44, 0x2c, 0x6f, 0x5d, 0x68, 0x97, 0xb0, 0x29, 0x1, 0x5c, 0x6f, 0x46, 0x2e, 0x2a, 0x9d, 0xae, 0x2c, 0xe3}, c.nameKey)
	assert.Equal(t, [16]uint8{0xf1, 0x7f, 0xd7, 0x14, 0x1d, 0x65, 0x27, 0x4f, 0x36, 0x3f, 0xc2, 0xa0, 0x4d, 0xd2, 0x14, 0x8a}, c.nameTweak)

	require.NoError(t, c.Key("potato", "Sausage"))
	assert.Equal(t, [32]uint8{0xda, 0x81, 0x8c, 0x67, 0xef, 0x11, 0xf, 0xc8, 0xd5, 0xc8, 0x62, 0x4b, 0x7f, 0xe2, 0x9e, 0x35, 0x35, 0xb0, 0x8d, 0x79, 0x84, 0x89, 0xac, 0xcb, 0xa0, 0xff, 0x2, 0x72, 0x3, 0x1a, 0x5e, 0x64}, c.dataKey)
	assert.Equal(t, [32]uint8{0x2, 0x81, 0x7e, 0x7b, 0xea, 0x99, 0x81, 0x5a, 0xd0, 0x2d, 0xb9, 0x64, 0x48, 0xb0, 0x28, 0x27, 0x7c, 0x20, 0xb4, 0xd4, 0xa4, 0x68, 0xad, 0x4e, 0x5c, 0x29, 0xf, 0x79, 0xef, 0xee, 0xdb, 0x3b}, c.nameKey)
	assert.Equal(t, [16]uint8{0x9a, 0xb5, 0xb, 0x3d, 0xcb, 0x60, 0x59, 0x55, 0xa5, 0x4d, 0xe6, 0xb6, 0x47, 0x3, 0x23, 0xe2}, c.nameTweak)

	require.NoError(t, c.Key("", ""))
	assert.Equal(t, [32]byte{}, c.dataKey)
	assert.Equal(t, [32]byte{}, c.nameKey)
	assert.Equal(t, [16]byte{}, c.nameTweak)
}
