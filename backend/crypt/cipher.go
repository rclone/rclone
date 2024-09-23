package crypt

import (
	"bytes"
	"context"
	"crypto/aes"
	gocipher "crypto/cipher"
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/Max-Sum/base32768"
	keywrap "github.com/nickball/go-aes-key-wrap"
	"github.com/rclone/rclone/backend/crypt/pkcs7"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/lib/readers"
	"github.com/rclone/rclone/lib/version"
	"github.com/rfjakob/eme"
	"golang.org/x/crypto/nacl/secretbox"
	"golang.org/x/crypto/scrypt"
)

// Constants
const (
	nameCipherBlockSize = aes.BlockSize
	fileMagic           = "RCLONE\x00\x00"
	fileMagicSize       = len(fileMagic)
	fileNonceSize       = 24
	fileHeaderSize      = fileMagicSize + fileNonceSize

	fileMagicV2         = "RCLONE\x00\x01"
	fileMagicSizeV2     = len(fileMagicV2)
	fileNonceSizeV2     = 23
	fileReservedBytesV2 = 4
	fileHeaderSizeV2    = fileMagicSizeV2 + fileNonceSizeV2 + fileWrappedCekSize + fileReservedBytesV2
	fileCekSize         = 32
	fileWrappedCekSize  = fileCekSize + 8 // Some overhead of AES RFC 3394 Key Wrapping

	blockHeaderSize      = secretbox.Overhead
	blockDataSize        = 64 * 1024
	blockSize            = blockHeaderSize + blockDataSize
	lastBlockFlag   byte = 1 // Corresponds to: `last_block` https://github.com/miscreant/meta/wiki/STREAM
)

// Errors returned by cipher
var (
	ErrorBadDecryptUTF8          = errors.New("bad decryption - utf-8 invalid")
	ErrorBadDecryptControlChar   = errors.New("bad decryption - contains control chars")
	ErrorNotAMultipleOfBlocksize = errors.New("not a multiple of blocksize")
	ErrorTooShortAfterDecode     = errors.New("too short after base32 decode")
	ErrorTooLongAfterDecode      = errors.New("too long after base32 decode")
	ErrorEncryptedFileTooShort   = errors.New("file is too short to be decrypted")
	ErrorEncryptedFileBadHeader  = errors.New("file has truncated block header")
	ErrorEncryptedCekInvalid     = errors.New("file encryption key is invalid")
	ErrorEncryptedBadMagic       = errors.New("not an encrypted file - bad magic string")
	ErrorEncryptedBadBlock       = errors.New("failed to authenticate decrypted block - bad password?")
	ErrorBadBase32Encoding       = errors.New("bad base32 filename encoding")
	ErrorFileClosed              = errors.New("file already closed")
	ErrorNotAnEncryptedFile      = errors.New("not an encrypted file - does not match suffix")
	ErrorBadSeek                 = errors.New("Seek beyond end of file")
	ErrorSuffixMissingDot        = errors.New("suffix config setting should include a '.'")
	defaultSalt                  = []byte{0xA8, 0x0D, 0xF4, 0x3A, 0x8F, 0xBD, 0x03, 0x08, 0xA7, 0xCA, 0xB8, 0x3E, 0x58, 0x1F, 0x86, 0xB1}
	obfuscQuoteRune              = '!'
)

// Global variables
var (
	fileMagicBytes   = []byte(fileMagic)
	fileMagicBytesV2 = []byte(fileMagicV2)
)

// ReadSeekCloser is the interface of the read handles
type ReadSeekCloser interface {
	io.Reader
	io.Seeker
	io.Closer
	fs.RangeSeeker
}

// OpenRangeSeek opens the file handle at the offset with the limit given
type OpenRangeSeek func(ctx context.Context, offset, limit int64) (io.ReadCloser, error)

// NameEncryptionMode is the type of file name encryption in use
type NameEncryptionMode int

// NameEncryptionMode levels
const (
	NameEncryptionOff NameEncryptionMode = iota
	NameEncryptionStandard
	NameEncryptionObfuscated
)

// CipherVersion
const (
	CipherVersionV1 = "1"
	CipherVersionV2 = "2"
)

// BlockCipher
const (
	BlockCipherXSalsa20 = 0x01 // Used currently
	BlockCipherAESGCM   = 0x02 // Not used, possible future use (increase web performance): https://github.com/rclone/rclone/issues/7192#issuecomment-2353617267
)

// NewNameEncryptionMode turns a string into a NameEncryptionMode
func NewNameEncryptionMode(s string) (mode NameEncryptionMode, err error) {
	s = strings.ToLower(s)
	switch s {
	case "off":
		mode = NameEncryptionOff
	case "standard":
		mode = NameEncryptionStandard
	case "obfuscate":
		mode = NameEncryptionObfuscated
	default:
		err = fmt.Errorf("unknown file name encryption mode %q", s)
	}
	return mode, err
}

// String turns mode into a human-readable string
func (mode NameEncryptionMode) String() (out string) {
	switch mode {
	case NameEncryptionOff:
		out = "off"
	case NameEncryptionStandard:
		out = "standard"
	case NameEncryptionObfuscated:
		out = "obfuscate"
	default:
		out = fmt.Sprintf("Unknown mode #%d", mode)
	}
	return out
}

// fileNameEncoding are the encoding methods dealing with encrypted file names
type fileNameEncoding interface {
	EncodeToString(src []byte) string
	DecodeString(s string) ([]byte, error)
}

// caseInsensitiveBase32Encoding defines a file name encoding
// using a modified version of standard base32 as described in
// RFC4648
//
// The standard encoding is modified in two ways
//   - it becomes lower case (no-one likes upper case filenames!)
//   - we strip the padding character `=`
type caseInsensitiveBase32Encoding struct{}

// EncodeToString encodes a string using the modified version of
// base32 encoding.
func (caseInsensitiveBase32Encoding) EncodeToString(src []byte) string {
	encoded := base32.HexEncoding.EncodeToString(src)
	encoded = strings.TrimRight(encoded, "=")
	return strings.ToLower(encoded)
}

// DecodeString decodes a string as encoded by EncodeToString
func (caseInsensitiveBase32Encoding) DecodeString(s string) ([]byte, error) {
	if strings.HasSuffix(s, "=") {
		return nil, ErrorBadBase32Encoding
	}
	// First figure out how many padding characters to add
	roundUpToMultipleOf8 := (len(s) + 7) &^ 7
	equals := roundUpToMultipleOf8 - len(s)
	s = strings.ToUpper(s) + "========"[:equals]
	return base32.HexEncoding.DecodeString(s)
}

// NewNameEncoding creates a NameEncoding from a string
func NewNameEncoding(s string) (enc fileNameEncoding, err error) {
	s = strings.ToLower(s)
	switch s {
	case "base32":
		enc = caseInsensitiveBase32Encoding{}
	case "base64":
		enc = base64.RawURLEncoding
	case "base32768":
		enc = base32768.SafeEncoding
	default:
		err = fmt.Errorf("unknown file name encoding mode %q", s)
	}
	return enc, err
}

// Cipher defines an encoding and decoding cipher for the crypt backend
type Cipher struct {
	dataKey         [32]byte                  // Key for secretbox
	nameKey         [32]byte                  // 16,24 or 32 bytes
	nameTweak       [nameCipherBlockSize]byte // used to tweak the name crypto
	block           gocipher.Block
	mode            NameEncryptionMode
	fileNameEnc     fileNameEncoding
	buffers         sync.Pool // encrypt/decrypt buffers
	cryptoRand      io.Reader // read crypto random numbers from here
	dirNameEncrypt  bool
	passBadBlocks   bool // if set passed bad blocks as zeroed blocks
	encryptedSuffix string
	version         string
}

// newCipher initialises the cipher.  If salt is "" then it uses a built in salt val
func newCipher(mode NameEncryptionMode, password, salt string, dirNameEncrypt bool, enc fileNameEncoding) (*Cipher, error) {
	c := &Cipher{
		mode:            mode,
		fileNameEnc:     enc,
		cryptoRand:      rand.Reader,
		dirNameEncrypt:  dirNameEncrypt,
		encryptedSuffix: ".bin",
		version:         CipherVersionV1, // Default to satisfy existing: `cipher_test.go`. For Rclone overridden by: `setCipherVersion` inside `newCipherForConfig` call.
	}
	c.buffers.New = func() interface{} {
		return new([blockSize]byte)
	}
	err := c.Key(password, salt)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// setEncryptedSuffix set suffix, or an empty string
func (c *Cipher) setEncryptedSuffix(suffix string) {
	if strings.EqualFold(suffix, "none") {
		c.encryptedSuffix = ""
		return
	}
	if !strings.HasPrefix(suffix, ".") {
		fs.Errorf(nil, "crypt: bad suffix: %v", ErrorSuffixMissingDot)
		suffix = "." + suffix
	}
	c.encryptedSuffix = suffix
}

// Call to set bad block pass through
func (c *Cipher) setPassBadBlocks(passBadBlocks bool) {
	c.passBadBlocks = passBadBlocks
}

func (c *Cipher) setCipherVersion(cipherVersion string) {
	c.version = cipherVersion
}

func (c *Cipher) getFileHeaderSize() int {
	if c.version == CipherVersionV2 {
		return fileHeaderSizeV2
	}
	return fileHeaderSize
}

func (c *Cipher) getFileMagicSize() int {
	if c.version == CipherVersionV2 {
		return fileMagicSizeV2
	}
	return fileMagicSize
}

func (c *Cipher) getFileMagicBytes() []byte {
	if c.version == CipherVersionV2 {
		return fileMagicBytesV2
	}
	return fileMagicBytes
}

func (c *Cipher) getFileNonceSize() int { // Effective nonce size. Both V1 and V2 use 24 bytes, but V2 consists 23+1 bytes nonce, where last byte is calculated dynamically depending if the processing block is last (truncation protection)
	if c.version == CipherVersionV2 {
		return fileNonceSizeV2
	}
	return fileNonceSize
}

// Key creates all the internal keys from the password passed in using
// scrypt.
//
// If salt is "" we use a fixed salt just to make attackers lives
// slightly harder than using no salt.
//
// Note that empty password makes all 0x00 keys which is used in the
// tests.
func (c *Cipher) Key(password, salt string) (err error) {
	const keySize = len(c.dataKey) + len(c.nameKey) + len(c.nameTweak)
	var saltBytes = defaultSalt
	if salt != "" {
		saltBytes = []byte(salt)
	}
	var key []byte
	if password == "" {
		key = make([]byte, keySize)
	} else {
		key, err = scrypt.Key([]byte(password), saltBytes, 16384, 8, 1, keySize)
		if err != nil {
			return err
		}
	}
	copy(c.dataKey[:], key)
	copy(c.nameKey[:], key[len(c.dataKey):])
	copy(c.nameTweak[:], key[len(c.dataKey)+len(c.nameKey):])
	// Key the name cipher
	c.block, err = aes.NewCipher(c.nameKey[:])
	return err
}

// getBlock gets a block from the pool of size blockSize
func (c *Cipher) getBlock() *[blockSize]byte {
	return c.buffers.Get().(*[blockSize]byte)
}

// putBlock returns a block to the pool of size blockSize
func (c *Cipher) putBlock(buf *[blockSize]byte) {
	c.buffers.Put(buf)
}

// encryptSegment encrypts a path segment
//
// This uses EME with AES.
//
// EME (ECB-Mix-ECB) is a wide-block encryption mode presented in the
// 2003 paper "A Parallelizable Enciphering Mode" by Halevi and
// Rogaway.
//
// This makes for deterministic encryption which is what we want - the
// same filename must encrypt to the same thing.
//
// This means that
//   - filenames with the same name will encrypt the same
//   - filenames which start the same won't have a common prefix
func (c *Cipher) encryptSegment(plaintext string) string {
	if plaintext == "" {
		return ""
	}
	paddedPlaintext := pkcs7.Pad(nameCipherBlockSize, []byte(plaintext))
	ciphertext := eme.Transform(c.block, c.nameTweak[:], paddedPlaintext, eme.DirectionEncrypt)
	return c.fileNameEnc.EncodeToString(ciphertext)
}

// decryptSegment decrypts a path segment
func (c *Cipher) decryptSegment(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	rawCiphertext, err := c.fileNameEnc.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}
	if len(rawCiphertext)%nameCipherBlockSize != 0 {
		return "", ErrorNotAMultipleOfBlocksize
	}
	if len(rawCiphertext) == 0 {
		// not possible if decodeFilename() working correctly
		return "", ErrorTooShortAfterDecode
	}
	if len(rawCiphertext) > 2048 {
		return "", ErrorTooLongAfterDecode
	}
	paddedPlaintext := eme.Transform(c.block, c.nameTweak[:], rawCiphertext, eme.DirectionDecrypt)
	plaintext, err := pkcs7.Unpad(nameCipherBlockSize, paddedPlaintext)
	if err != nil {
		return "", err
	}
	return string(plaintext), err
}

// Simple obfuscation routines
func (c *Cipher) obfuscateSegment(plaintext string) string {
	if plaintext == "" {
		return ""
	}

	// If the string isn't valid UTF8 then don't rotate; just
	// prepend a !.
	if !utf8.ValidString(plaintext) {
		return "!." + plaintext
	}

	// Calculate a simple rotation based on the filename and
	// the nameKey
	var dir int
	for _, runeValue := range plaintext {
		dir += int(runeValue)
	}
	dir %= 256

	// We'll use this number to store in the result filename...
	var result bytes.Buffer
	_, _ = result.WriteString(strconv.Itoa(dir) + ".")

	// but we'll augment it with the nameKey for real calculation
	for i := 0; i < len(c.nameKey); i++ {
		dir += int(c.nameKey[i])
	}

	// Now for each character, depending on the range it is in
	// we will actually rotate a different amount
	for _, runeValue := range plaintext {
		switch {
		case runeValue == obfuscQuoteRune:
			// Quote the Quote character
			_, _ = result.WriteRune(obfuscQuoteRune)
			_, _ = result.WriteRune(obfuscQuoteRune)

		case runeValue >= '0' && runeValue <= '9':
			// Number
			thisdir := (dir % 9) + 1
			newRune := '0' + (int(runeValue)-'0'+thisdir)%10
			_, _ = result.WriteRune(rune(newRune))

		case (runeValue >= 'A' && runeValue <= 'Z') ||
			(runeValue >= 'a' && runeValue <= 'z'):
			// ASCII letter.  Try to avoid trivial A->a mappings
			thisdir := dir%25 + 1
			// Calculate the offset of this character in A-Za-z
			pos := int(runeValue - 'A')
			if pos >= 26 {
				pos -= 6 // It's lower case
			}
			// Rotate the character to the new location
			pos = (pos + thisdir) % 52
			if pos >= 26 {
				pos += 6 // and handle lower case offset again
			}
			_, _ = result.WriteRune(rune('A' + pos))

		case runeValue >= 0xA0 && runeValue <= 0xFF:
			// Latin 1 supplement
			thisdir := (dir % 95) + 1
			newRune := 0xA0 + (int(runeValue)-0xA0+thisdir)%96
			_, _ = result.WriteRune(rune(newRune))

		case runeValue >= 0x100:
			// Some random Unicode range; we have no good rules here
			thisdir := (dir % 127) + 1
			base := int(runeValue - runeValue%256)
			newRune := rune(base + (int(runeValue)-base+thisdir)%256)
			// If the new character isn't a valid UTF8 char
			// then don't rotate it.  Quote it instead
			if !utf8.ValidRune(newRune) {
				_, _ = result.WriteRune(obfuscQuoteRune)
				_, _ = result.WriteRune(runeValue)
			} else {
				_, _ = result.WriteRune(newRune)
			}

		default:
			// Leave character untouched
			_, _ = result.WriteRune(runeValue)
		}
	}
	return result.String()
}

func (c *Cipher) deobfuscateSegment(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	pos := strings.Index(ciphertext, ".")
	if pos == -1 {
		return "", ErrorNotAnEncryptedFile
	} // No .
	num := ciphertext[:pos]
	if num == "!" {
		// No rotation; probably original was not valid unicode
		return ciphertext[pos+1:], nil
	}
	dir, err := strconv.Atoi(num)
	if err != nil {
		return "", ErrorNotAnEncryptedFile // Not a number
	}

	// add the nameKey to get the real rotate distance
	for i := 0; i < len(c.nameKey); i++ {
		dir += int(c.nameKey[i])
	}

	var result bytes.Buffer

	inQuote := false
	for _, runeValue := range ciphertext[pos+1:] {
		switch {
		case inQuote:
			_, _ = result.WriteRune(runeValue)
			inQuote = false

		case runeValue == obfuscQuoteRune:
			inQuote = true

		case runeValue >= '0' && runeValue <= '9':
			// Number
			thisdir := (dir % 9) + 1
			newRune := '0' + int(runeValue) - '0' - thisdir
			if newRune < '0' {
				newRune += 10
			}
			_, _ = result.WriteRune(rune(newRune))

		case (runeValue >= 'A' && runeValue <= 'Z') ||
			(runeValue >= 'a' && runeValue <= 'z'):
			thisdir := dir%25 + 1
			pos := int(runeValue - 'A')
			if pos >= 26 {
				pos -= 6
			}
			pos -= thisdir
			if pos < 0 {
				pos += 52
			}
			if pos >= 26 {
				pos += 6
			}
			_, _ = result.WriteRune(rune('A' + pos))

		case runeValue >= 0xA0 && runeValue <= 0xFF:
			thisdir := (dir % 95) + 1
			newRune := 0xA0 + int(runeValue) - 0xA0 - thisdir
			if newRune < 0xA0 {
				newRune += 96
			}
			_, _ = result.WriteRune(rune(newRune))

		case runeValue >= 0x100:
			thisdir := (dir % 127) + 1
			base := int(runeValue - runeValue%256)
			newRune := rune(base + (int(runeValue) - base - thisdir))
			if int(newRune) < base {
				newRune += 256
			}
			_, _ = result.WriteRune(newRune)

		default:
			_, _ = result.WriteRune(runeValue)

		}
	}

	return result.String(), nil
}

// encryptFileName encrypts a file path
func (c *Cipher) encryptFileName(in string) string {
	segments := strings.Split(in, "/")
	for i := range segments {
		// Skip directory name encryption if the user chose to
		// leave them intact
		if !c.dirNameEncrypt && i != (len(segments)-1) {
			continue
		}

		// Strip version string so that only the non-versioned part
		// of the file name gets encrypted/obfuscated
		hasVersion := false
		var t time.Time
		if i == (len(segments)-1) && version.Match(segments[i]) {
			var s string
			t, s = version.Remove(segments[i])
			// version.Remove can fail, in which case it returns segments[i]
			if s != segments[i] {
				segments[i] = s
				hasVersion = true
			}
		}

		if c.mode == NameEncryptionStandard {
			segments[i] = c.encryptSegment(segments[i])
		} else {
			segments[i] = c.obfuscateSegment(segments[i])
		}

		// Add back a version to the encrypted/obfuscated
		// file name, if we stripped it off earlier
		if hasVersion {
			segments[i] = version.Add(segments[i], t)
		}
	}
	return strings.Join(segments, "/")
}

// EncryptFileName encrypts a file path
func (c *Cipher) EncryptFileName(in string) string {
	if c.mode == NameEncryptionOff {
		return in + c.encryptedSuffix
	}
	return c.encryptFileName(in)
}

// EncryptDirName encrypts a directory path
func (c *Cipher) EncryptDirName(in string) string {
	if c.mode == NameEncryptionOff || !c.dirNameEncrypt {
		return in
	}
	return c.encryptFileName(in)
}

// decryptFileName decrypts a file path
func (c *Cipher) decryptFileName(in string) (string, error) {
	segments := strings.Split(in, "/")
	for i := range segments {
		var err error
		// Skip directory name decryption if the user chose to
		// leave them intact
		if !c.dirNameEncrypt && i != (len(segments)-1) {
			continue
		}

		// Strip version string so that only the non-versioned part
		// of the file name gets decrypted/deobfuscated
		hasVersion := false
		var t time.Time
		if i == (len(segments)-1) && version.Match(segments[i]) {
			var s string
			t, s = version.Remove(segments[i])
			// version.Remove can fail, in which case it returns segments[i]
			if s != segments[i] {
				segments[i] = s
				hasVersion = true
			}
		}

		if c.mode == NameEncryptionStandard {
			segments[i], err = c.decryptSegment(segments[i])
		} else {
			segments[i], err = c.deobfuscateSegment(segments[i])
		}

		if err != nil {
			return "", err
		}

		// Add back a version to the decrypted/deobfuscated
		// file name, if we stripped it off earlier
		if hasVersion {
			segments[i] = version.Add(segments[i], t)
		}
	}
	return strings.Join(segments, "/"), nil
}

// DecryptFileName decrypts a file path
func (c *Cipher) DecryptFileName(in string) (string, error) {
	if c.mode == NameEncryptionOff {
		remainingLength := len(in) - len(c.encryptedSuffix)
		if remainingLength == 0 || !strings.HasSuffix(in, c.encryptedSuffix) {
			return "", ErrorNotAnEncryptedFile
		}
		decrypted := in[:remainingLength]
		if version.Match(decrypted) {
			_, unversioned := version.Remove(decrypted)
			if unversioned == "" {
				return "", ErrorNotAnEncryptedFile
			}
		}
		// Leave the version string on, if it was there
		return decrypted, nil
	}
	return c.decryptFileName(in)
}

// DecryptDirName decrypts a directory path
func (c *Cipher) DecryptDirName(in string) (string, error) {
	if c.mode == NameEncryptionOff || !c.dirNameEncrypt {
		return in, nil
	}
	return c.decryptFileName(in)
}

// NameEncryptionMode returns the encryption mode in use for names
func (c *Cipher) NameEncryptionMode() NameEncryptionMode {
	return c.mode
}

// nonce is an NACL secretbox nonce
type nonce [fileNonceSize]byte
type cek [fileCekSize]byte
type wrappedCek [fileWrappedCekSize]byte

// pointer returns the nonce as a *[24]byte for secretbox
func (n *nonce) pointer() *[fileNonceSize]byte {
	return (*[fileNonceSize]byte)(n)
}

// fromReader fills the nonce from an io.Reader - normally the OSes
// crypto random number generator
func (n *nonce) fromReader(in io.Reader, expectedNonceSize int) error {
	read, err := readers.ReadFill(in, (*n)[:expectedNonceSize])
	if read != expectedNonceSize {
		return fmt.Errorf("short read of nonce: %w", err)
	}
	return nil
}

func (n *cek) fromReader(in io.Reader) error {
	read, err := readers.ReadFill(in, (*n)[:])
	if read != fileCekSize {
		return fmt.Errorf("short read of CEK: %w", err)
	}
	return nil
}

func (n *wrappedCek) fromBuf(buf []byte) {
	read := copy((*n)[:], buf)
	if read != fileWrappedCekSize {
		panic("buffer to short to read wrapped CEK")
	}
}

// fromBuf fills the nonce from the buffer passed in
func (n *nonce) fromBuf(buf []byte, expectedNonceSize int) {
	read := copy((*n)[:], buf)
	if read != expectedNonceSize {
		panic("buffer to short to read nonce")
	}
}

// carry 1 up the nonce from position i
func (n *nonce) carry(i int) {
	for ; i < len(*n); i++ {
		digit := (*n)[i]
		newDigit := digit + 1
		(*n)[i] = newDigit
		if newDigit >= digit {
			// exit if no carry
			break
		}
	}
}

// increment to add 1 to the nonce
func (n *nonce) increment() {
	n.carry(0)
}

// add a uint64 to the nonce
func (n *nonce) add(x uint64) {
	carry := uint16(0)
	for i := 0; i < 8; i++ {
		digit := (*n)[i]
		xDigit := byte(x)
		x >>= 8
		carry += uint16(digit) + uint16(xDigit)
		(*n)[i] = byte(carry)
		carry >>= 8
	}
	if carry != 0 {
		n.carry(8)
	}
}

// encrypter encrypts an io.Reader on the fly
type encrypter struct {
	mu       sync.Mutex
	in       io.Reader
	cek      cek // File contents encryption key
	c        *Cipher
	nonce    nonce
	buf      *[blockSize]byte
	readBuf  *[blockSize]byte
	bufIndex int
	bufSize  int
	err      error
}

// newEncrypter creates a new file handle encrypting on the fly
func (c *Cipher) newEncrypter(in io.Reader, nonce *nonce, cek *cek) (*encrypter, error) {
	fh := &encrypter{
		in:      in,
		c:       c,
		buf:     c.getBlock(),
		readBuf: c.getBlock(),
		bufSize: c.getFileHeaderSize(),
	}
	// Initialise nonce
	if nonce != nil {
		fh.nonce = *nonce
	} else {
		err := fh.nonce.fromReader(c.cryptoRand, c.getFileNonceSize())
		if err != nil {
			return nil, err
		}
	}
	// Copy magic into buffer
	copy((*fh.buf)[:], c.getFileMagicBytes())

	// Copy nonce into buffer
	copy((*fh.buf)[c.getFileMagicSize():], fh.nonce[:c.getFileNonceSize()])

	if c.version == CipherVersionV1 {
		fh.cek = fh.c.dataKey
	}

	if c.version == CipherVersionV2 {
		if cek != nil {
			fh.cek = *cek
		} else { // Generate random CEK
			err := fh.cek.fromReader(c.cryptoRand)
			if err != nil {
				return nil, err
			}
		}

		// WRAP KEY - START
		kek := fh.c.dataKey[:]
		cipher, err := aes.NewCipher(kek)
		if err != nil {
			return nil, err
		}

		wrappedCek, err := keywrap.Wrap(cipher, fh.cek[:]) // Wrap encryption key, 32 bytes cek, will result in 40 bytes ciphertext
		if err != nil {
			return nil, err
		}
		// WRAP KEY - END

		// Copy file encryption key
		copy((*fh.buf)[c.getFileMagicSize()+c.getFileNonceSize():], wrappedCek)

		// Copy 4 reserved bytes:
		//	- cipher type (XSalsa20, AES-GCM etc.),
		//	- not used,
		//	- not used,
		//	- not used
		copy((*fh.buf)[c.getFileMagicSize()+c.getFileNonceSize()+fileWrappedCekSize:], []byte{BlockCipherXSalsa20, 0x00, 0x00, 0x00})
	}

	return fh, nil
}

// Read as per io.Reader
func (fh *encrypter) Read(p []byte) (n int, err error) {
	fh.mu.Lock()
	defer fh.mu.Unlock()

	if fh.err != nil {
		return 0, fh.err
	}
	if fh.bufIndex >= fh.bufSize {
		// Read data
		// FIXME should overlap the reads with a go-routine and 2 buffers?
		readBuf := (*fh.readBuf)[:blockDataSize]
		n, err = readers.ReadFill(fh.in, readBuf)
		if n == 0 {
			return fh.finish(err)
		}

		isLastBlock := n < blockDataSize

		if fh.c.version == CipherVersionV2 && isLastBlock {
			fh.nonce[len(fh.nonce)-1] = lastBlockFlag // Set last block flag as the last byte of nonce
		}

		// possibly err != nil here, but we will process the
		// data and the next call to ReadFill will return 0, err
		// Encrypt the block using the nonce
		secretbox.Seal((*fh.buf)[:0], readBuf[:n], fh.nonce.pointer(), (*[32]byte)(&fh.cek))
		fh.bufIndex = 0
		fh.bufSize = blockHeaderSize + n
		fh.nonce.increment()
	}
	n = copy(p, (*fh.buf)[fh.bufIndex:fh.bufSize])
	fh.bufIndex += n
	return n, nil
}

// finish sets the final error and tidies up
func (fh *encrypter) finish(err error) (int, error) {
	if fh.err != nil {
		return 0, fh.err
	}
	fh.err = err
	fh.c.putBlock(fh.buf)
	fh.buf = nil
	fh.c.putBlock(fh.readBuf)
	fh.readBuf = nil
	return 0, err
}

// Encrypt data encrypts the data stream
func (c *Cipher) encryptData(in io.Reader) (io.Reader, *encrypter, error) {
	in, wrap := accounting.UnWrap(in) // unwrap the accounting off the Reader
	out, err := c.newEncrypter(in, nil, nil)
	if err != nil {
		return nil, nil, err
	}
	return wrap(out), out, nil // and wrap the accounting back on
}

// EncryptData encrypts the data stream
func (c *Cipher) EncryptData(in io.Reader) (io.Reader, error) {
	out, _, err := c.encryptData(in)
	return out, err
}

// decrypter decrypts an io.ReaderCloser on the fly
type decrypter struct {
	mu           sync.Mutex
	rc           io.ReadCloser
	nonce        nonce
	initialNonce nonce
	cek          [32]byte // File contents decryption key
	c            *Cipher
	buf          *[blockSize]byte
	readBuf      *[blockSize]byte
	bufIndex     int
	bufSize      int
	err          error
	limit        int64 // limit of bytes to read, -1 for unlimited
	open         OpenRangeSeek
}

// newDecrypter creates a new file handle decrypting on the fly
func (c *Cipher) newDecrypter(rc io.ReadCloser) (*decrypter, error) {
	fh := &decrypter{
		rc:      rc,
		c:       c,
		buf:     c.getBlock(),
		readBuf: c.getBlock(),
		limit:   -1,
	}
	// Read file header (magic + nonce)
	readBuf := (*fh.readBuf)[:fileHeaderSize] // We read the `fileHeaderSize` first to determine the cipher version and get the nonce, then if it's V2 we read remaining bytes to satisfy `fileHeaderSizeV2`
	n, err := readers.ReadFill(fh.rc, readBuf)
	if n < fileHeaderSize && err == io.EOF {
		// This read from 0..fileHeaderSize-1 bytes
		return nil, fh.finishAndClose(ErrorEncryptedFileTooShort)
	} else if err != io.EOF && err != nil {
		return nil, fh.finishAndClose(err)
	}

	isMagicHeader := bytes.Equal(readBuf[:fileMagicSize], fileMagicBytes)
	isMagicHeaderV2 := bytes.Equal(readBuf[:fileMagicSizeV2], fileMagicBytesV2)

	if isMagicHeader {
		c.setCipherVersion(CipherVersionV1)
		fh.cek = fh.c.dataKey
	} else if isMagicHeaderV2 {
		c.setCipherVersion(CipherVersionV2)
	} else {
		return nil, fh.finishAndClose(ErrorEncryptedBadMagic)
	}

	offsetStart := c.getFileMagicSize()
	offsetEnd := offsetStart + c.getFileNonceSize()
	fh.nonce.fromBuf(readBuf[offsetStart:offsetEnd], c.getFileNonceSize())
	fh.initialNonce = fh.nonce

	if c.version == CipherVersionV2 { // V2 cipher has a longer header, so after reading V1 header, we need to read remaining bytes from the reader to finialize V2 cipher initialization
		remainingBytesFromV1Buffer := fileNonceSize - fileNonceSizeV2 // V2 nonce is 1 byte shorter, but this byte was already read to the `readBuf`, we use: `combinedBuffer` to append it to the beginning.
		lastByte := readBuf[:len(readBuf)-remainingBytesFromV1Buffer]

		remainingBytesReadSize := fileHeaderSizeV2 - fileHeaderSize
		offsetStart = n // Previously read: `fileHeaderSize` bytes
		offsetEnd = offsetStart + remainingBytesReadSize
		readBuf = (*fh.readBuf)[offsetStart:offsetEnd]
		n, err = readers.ReadFill(fh.rc, readBuf)

		if n < remainingBytesReadSize && err == io.EOF {
			return nil, fh.finishAndClose(ErrorEncryptedFileTooShort)
		} else if err != io.EOF && err != nil {
			return nil, fh.finishAndClose(err)
		}

		combinedBuffer := append(lastByte, readBuf...)

		var wrappedCek wrappedCek
		wrappedCek.fromBuf(combinedBuffer) // retrieve wrapped file encryption key

		kek := fh.c.dataKey[:]
		cipher, err := aes.NewCipher(kek)
		if err != nil {
			return nil, fh.finishAndClose(err)
		}

		fileKey, err := keywrap.Unwrap(cipher, wrappedCek[:])
		if err != nil {
			return nil, fh.finishAndClose(ErrorEncryptedCekInvalid)
		}

		fh.cek = [32]byte(fileKey)
	}

	return fh, nil
}

// newDecrypterSeek creates a new file handle decrypting on the fly
func (c *Cipher) newDecrypterSeek(ctx context.Context, open OpenRangeSeek, offset, limit int64) (fh *decrypter, err error) {
	var rc io.ReadCloser
	doRangeSeek := false
	setLimit := false
	// Open initially with no seek
	if offset == 0 && limit < 0 {
		// If no offset or limit then open whole file
		rc, err = open(ctx, 0, -1)
	} else if offset == 0 {
		// If no offset open the header + limit worth of the file
		_, underlyingLimit, _, _ := calculateUnderlying(offset, limit, c.getFileHeaderSize())
		rc, err = open(ctx, 0, int64(c.getFileHeaderSize())+underlyingLimit)
		setLimit = true
	} else {
		// Otherwise just read the header to start with
		rc, err = open(ctx, 0, int64(c.getFileHeaderSize()))
		doRangeSeek = true
	}
	if err != nil {
		return nil, err
	}
	// Open the stream which fills in the nonce
	fh, err = c.newDecrypter(rc)
	if err != nil {
		return nil, err
	}
	fh.open = open // will be called by fh.RangeSeek
	if doRangeSeek {
		_, err = fh.RangeSeek(ctx, offset, io.SeekStart, limit)
		if err != nil {
			_ = fh.Close()
			return nil, err
		}
	}
	if setLimit {
		fh.limit = limit
	}
	return fh, nil
}

// read data into internal buffer - call with fh.mu held
func (fh *decrypter) fillBuffer() (err error) {
	// FIXME should overlap the reads with a go-routine and 2 buffers?
	readBuf := fh.readBuf
	n, err := readers.ReadFill(fh.rc, (*readBuf)[:])
	if n == 0 {
		return err
	}
	// possibly err != nil here, but we will process the data and
	// the next call to ReadFull will return 0, err

	// Check header + 1 byte exists
	if n <= blockHeaderSize {
		if err != nil && err != io.EOF {
			return err // return pending error as it is likely more accurate
		}
		return ErrorEncryptedFileBadHeader
	}

	if fh.c.version == CipherVersionV2 && n < blockDataSize { // last block
		fh.nonce[len(fh.nonce)-1] = lastBlockFlag // Set last block flag at the last byte
	}

	// Decrypt the block using the nonce
	_, ok := secretbox.Open((*fh.buf)[:0], (*readBuf)[:n], fh.nonce.pointer(), &fh.cek)
	if !ok {
		if err != nil && err != io.EOF {
			return err // return pending error as it is likely more accurate
		}
		if !fh.c.passBadBlocks {
			return ErrorEncryptedBadBlock
		}
		fs.Errorf(nil, "crypt: ignoring: %v", ErrorEncryptedBadBlock)
		// Zero out the bad block and continue
		for i := range (*fh.buf)[:n] {
			fh.buf[i] = 0
		}
	}
	fh.bufIndex = 0
	fh.bufSize = n - blockHeaderSize
	fh.nonce.increment()
	return nil
}

// Read as per io.Reader
func (fh *decrypter) Read(p []byte) (n int, err error) {
	fh.mu.Lock()
	defer fh.mu.Unlock()

	if fh.err != nil {
		return 0, fh.err
	}
	if fh.bufIndex >= fh.bufSize {
		err = fh.fillBuffer()
		if err != nil {
			return 0, fh.finish(err)
		}
	}
	toCopy := fh.bufSize - fh.bufIndex
	if fh.limit >= 0 && fh.limit < int64(toCopy) {
		toCopy = int(fh.limit)
	}
	n = copy(p, (*fh.buf)[fh.bufIndex:fh.bufIndex+toCopy])
	fh.bufIndex += n
	if fh.limit >= 0 {
		fh.limit -= int64(n)
		if fh.limit == 0 {
			return n, fh.finish(io.EOF)
		}
	}
	return n, nil
}

// calculateUnderlying converts an (offset, limit, headerSize) in an encrypted file
// into an (underlyingOffset, underlyingLimit) for the underlying file.
//
// It also returns number of bytes to discard after reading the first
// block and number of blocks this is from the start so the nonce can
// be incremented.
func calculateUnderlying(offset, limit int64, headerSize int) (underlyingOffset, underlyingLimit, discard, blocks int64) {
	// blocks we need to seek, plus bytes we need to discard
	blocks, discard = offset/blockDataSize, offset%blockDataSize

	// Offset in underlying stream we need to seek
	underlyingOffset = int64(headerSize) + blocks*(blockHeaderSize+blockDataSize)

	// work out how many blocks we need to read
	underlyingLimit = int64(-1)
	if limit >= 0 {
		// bytes to read beyond the first block
		bytesToRead := limit - (blockDataSize - discard)

		// Read the first block
		blocksToRead := int64(1)

		if bytesToRead > 0 {
			// Blocks that need to be read plus left over blocks
			extraBlocksToRead, endBytes := bytesToRead/blockDataSize, bytesToRead%blockDataSize
			if endBytes != 0 {
				// If left over bytes must read another block
				extraBlocksToRead++
			}
			blocksToRead += extraBlocksToRead
		}

		// Must read a whole number of blocks
		underlyingLimit = blocksToRead * (blockHeaderSize + blockDataSize)
	}
	return
}

// RangeSeek behaves like a call to Seek(offset int64, whence
// int) with the output wrapped in an io.LimitedReader
// limiting the total length to limit.
//
// RangeSeek with a limit of < 0 is equivalent to a regular Seek.
func (fh *decrypter) RangeSeek(ctx context.Context, offset int64, whence int, limit int64) (int64, error) {
	fh.mu.Lock()
	defer fh.mu.Unlock()

	if fh.open == nil {
		return 0, fh.finish(errors.New("can't seek - not initialised with newDecrypterSeek"))
	}
	if whence != io.SeekStart {
		return 0, fh.finish(errors.New("can only seek from the start"))
	}

	// Reset error or return it if not EOF
	if fh.err == io.EOF {
		fh.unFinish()
	} else if fh.err != nil {
		return 0, fh.err
	}

	underlyingOffset, underlyingLimit, discard, blocks := calculateUnderlying(offset, limit, fh.c.getFileHeaderSize())

	// Move the nonce on the correct number of blocks from the start
	fh.nonce = fh.initialNonce
	fh.nonce.add(uint64(blocks))

	// Can we seek underlying stream directly?
	if do, ok := fh.rc.(fs.RangeSeeker); ok {
		// Seek underlying stream directly
		_, err := do.RangeSeek(ctx, underlyingOffset, 0, underlyingLimit)
		if err != nil {
			return 0, fh.finish(err)
		}
	} else {
		// if not reopen with seek
		_ = fh.rc.Close() // close underlying file
		fh.rc = nil

		// Re-open the underlying object with the offset given
		rc, err := fh.open(ctx, underlyingOffset, underlyingLimit)
		if err != nil {
			return 0, fh.finish(fmt.Errorf("couldn't reopen file with offset and limit: %w", err))
		}

		// Set the file handle
		fh.rc = rc
	}

	// Fill the buffer
	err := fh.fillBuffer()
	if err != nil {
		return 0, fh.finish(err)
	}

	// Discard bytes from the buffer
	if int(discard) > fh.bufSize {
		return 0, fh.finish(ErrorBadSeek)
	}
	fh.bufIndex = int(discard)

	// Set the limit
	fh.limit = limit

	return offset, nil
}

// Seek implements the io.Seeker interface
func (fh *decrypter) Seek(offset int64, whence int) (int64, error) {
	return fh.RangeSeek(context.TODO(), offset, whence, -1)
}

// finish sets the final error and tidies up
func (fh *decrypter) finish(err error) error {
	if fh.err != nil {
		return fh.err
	}
	fh.err = err
	fh.c.putBlock(fh.buf)
	fh.buf = nil
	fh.c.putBlock(fh.readBuf)
	fh.readBuf = nil
	return err
}

// unFinish undoes the effects of finish
func (fh *decrypter) unFinish() {
	// Clear error
	fh.err = nil

	// reinstate the buffers
	fh.buf = fh.c.getBlock()
	fh.readBuf = fh.c.getBlock()

	// Empty the buffer
	fh.bufIndex = 0
	fh.bufSize = 0
}

// Close
func (fh *decrypter) Close() error {
	fh.mu.Lock()
	defer fh.mu.Unlock()

	// Check already closed
	if fh.err == ErrorFileClosed {
		return fh.err
	}
	// Closed before reading EOF so not finish()ed yet
	if fh.err == nil {
		_ = fh.finish(io.EOF)
	}
	// Show file now closed
	fh.err = ErrorFileClosed
	if fh.rc == nil {
		return nil
	}
	return fh.rc.Close()
}

// finishAndClose does finish then Close()
//
// Used when we are returning a nil fh from new
func (fh *decrypter) finishAndClose(err error) error {
	_ = fh.finish(err)
	_ = fh.Close()
	return err
}

// DecryptData decrypts the data stream
func (c *Cipher) DecryptData(rc io.ReadCloser) (io.ReadCloser, error) {
	out, err := c.newDecrypter(rc)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// DecryptDataSeek decrypts the data stream from offset
//
// The open function must return a ReadCloser opened to the offset supplied.
//
// You must use this form of DecryptData if you might want to Seek the file handle
func (c *Cipher) DecryptDataSeek(ctx context.Context, open OpenRangeSeek, offset, limit int64) (ReadSeekCloser, error) {
	out, err := c.newDecrypterSeek(ctx, open, offset, limit)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// EncryptedSize calculates the size of the data when encrypted
func (c *Cipher) EncryptedSize(size int64) int64 {
	blocks, residue := size/blockDataSize, size%blockDataSize
	encryptedSize := int64(c.getFileHeaderSize()) + blocks*(blockHeaderSize+blockDataSize)

	if c.version == CipherVersionV2 {
		encryptedSize += HashEncryptedSizeWithHeader
	}

	if residue != 0 {
		encryptedSize += blockHeaderSize + residue
	}
	return encryptedSize
}

// DecryptedSize calculates the size of the data when decrypted
func (c *Cipher) DecryptedSize(size int64) (int64, error) {
	size -= int64(c.getFileHeaderSize()) // WARNING: DecryptedSize might return invalid value before `newDecrypter()` is called if V2 cipher is enabled and user tries to read V1 object. Eventually `newDecrypter()` will be called, which will then call: `setCipherVersion()` which would then configure: `getFileHeaderSize()` effectively making subsequent calls to `DecryptedSize()` return exact value.
	if c.version == CipherVersionV1 && size < 0 {
		return 0, ErrorEncryptedFileTooShort
	}

	if c.version == CipherVersionV2 {
		size -= HashEncryptedSizeWithHeader
	}

	v1CipherOverhead := int64(fileHeaderSize)
	v2CipherOverhead := int64(fileHeaderSizeV2) + HashEncryptedSizeWithHeader
	sizeDiff := v1CipherOverhead - v2CipherOverhead
	// Below check was "loosened" to prevent code from return ErrorEncryptedFileTooShort when reading small `CipherVersionV1` encrypted object when `CipherVersionV2` is enabled as a config.
	// If V2 cipher is enabled then we assume file was encrypted using V2 header (but this is just assumption we can't be sure until `setCipherVersion()` is called).
	// V2 format has longer header and introduces concept of a footer. If we by any chance read small V1 object and assume it's a V2 header we may end up getting the negative decrypted size and trigger this check - preventing reading the file.
	// The `setCipherVersion()` is called once we actually read the file contents.
	// @TODO Don't use: "loosened" approach once: `setCipherVersion()` is called. Distinct implicit (assumed from config) cipher version assumption and explicit (set by setCipherVersion())
	if c.version == CipherVersionV2 && size < sizeDiff {
		return 0, ErrorEncryptedFileTooShort
	}

	blocks, residue := size/blockSize, size%blockSize
	decryptedSize := blocks * blockDataSize
	if residue != 0 {
		residue -= blockHeaderSize
		if residue <= 0 {
			return 0, ErrorEncryptedFileBadHeader
		}
	}
	decryptedSize += residue
	return decryptedSize, nil
}

// check interfaces
var (
	_ io.ReadCloser  = (*decrypter)(nil)
	_ io.Seeker      = (*decrypter)(nil)
	_ fs.RangeSeeker = (*decrypter)(nil)
	_ io.Reader      = (*encrypter)(nil)
)
