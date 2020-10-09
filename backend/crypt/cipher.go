package crypt

import (
	"bytes"
	"context"
	"crypto/aes"
	gocipher "crypto/cipher"
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/backend/crypt/pkcs7"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
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
	blockHeaderSize     = secretbox.Overhead
	blockDataSize       = 64 * 1024
	blockSize           = blockHeaderSize + blockDataSize
	encryptedSuffix     = ".bin" // when file name encryption is off we add this suffix to make sure the cloud provider doesn't process the file
)

// Errors returned by cipher
var (
	ErrorBadDecryptUTF8          = errors.New("bad decryption - utf-8 invalid")
	ErrorBadDecryptControlChar   = errors.New("bad decryption - contains control chars")
	ErrorNotAMultipleOfBlocksize = errors.New("not a multiple of blocksize")
	ErrorTooShortAfterDecode     = errors.New("too short after base32 decode")
	ErrorTooLongAfterDecode      = errors.New("too long after base32 decode")
	ErrorEncryptedFileTooShort   = errors.New("file is too short to be encrypted")
	ErrorEncryptedFileBadHeader  = errors.New("file has truncated block header")
	ErrorEncryptedBadMagic       = errors.New("not an encrypted file - bad magic string")
	ErrorEncryptedBadBlock       = errors.New("failed to authenticate decrypted block - bad password?")
	ErrorBadBase32Encoding       = errors.New("bad base32 filename encoding")
	ErrorFileClosed              = errors.New("file already closed")
	ErrorNotAnEncryptedFile      = errors.New("not an encrypted file - no \"" + encryptedSuffix + "\" suffix")
	ErrorBadSeek                 = errors.New("Seek beyond end of file")
	defaultSalt                  = []byte{0xA8, 0x0D, 0xF4, 0x3A, 0x8F, 0xBD, 0x03, 0x08, 0xA7, 0xCA, 0xB8, 0x3E, 0x58, 0x1F, 0x86, 0xB1}
	obfuscQuoteRune              = '!'
)

// Global variables
var (
	fileMagicBytes = []byte(fileMagic)
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
		err = errors.Errorf("Unknown file name encryption mode %q", s)
	}
	return mode, err
}

// String turns mode into a human readable string
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

// Cipher defines an encoding and decoding cipher for the crypt backend
type Cipher struct {
	dataKey        [32]byte                  // Key for secretbox
	nameKey        [32]byte                  // 16,24 or 32 bytes
	nameTweak      [nameCipherBlockSize]byte // used to tweak the name crypto
	block          gocipher.Block
	mode           NameEncryptionMode
	buffers        sync.Pool // encrypt/decrypt buffers
	cryptoRand     io.Reader // read crypto random numbers from here
	dirNameEncrypt bool
}

// newCipher initialises the cipher.  If salt is "" then it uses a built in salt val
func newCipher(mode NameEncryptionMode, password, salt string, dirNameEncrypt bool) (*Cipher, error) {
	c := &Cipher{
		mode:           mode,
		cryptoRand:     rand.Reader,
		dirNameEncrypt: dirNameEncrypt,
	}
	c.buffers.New = func() interface{} {
		return make([]byte, blockSize)
	}
	err := c.Key(password, salt)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// Key creates all the internal keys from the password passed in using
// scrypt.
//
// If salt is "" we use a fixed salt just to make attackers lives
// slighty harder than using no salt.
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
func (c *Cipher) getBlock() []byte {
	return c.buffers.Get().([]byte)
}

// putBlock returns a block to the pool of size blockSize
func (c *Cipher) putBlock(buf []byte) {
	if len(buf) != blockSize {
		panic("bad blocksize returned to pool")
	}
	c.buffers.Put(buf)
}

// encodeFileName encodes a filename using a modified version of
// standard base32 as described in RFC4648
//
// The standard encoding is modified in two ways
//  * it becomes lower case (no-one likes upper case filenames!)
//  * we strip the padding character `=`
func encodeFileName(in []byte) string {
	encoded := base32.HexEncoding.EncodeToString(in)
	encoded = strings.TrimRight(encoded, "=")
	return strings.ToLower(encoded)
}

// decodeFileName decodes a filename as encoded by encodeFileName
func decodeFileName(in string) ([]byte, error) {
	if strings.HasSuffix(in, "=") {
		return nil, ErrorBadBase32Encoding
	}
	// First figure out how many padding characters to add
	roundUpToMultipleOf8 := (len(in) + 7) &^ 7
	equals := roundUpToMultipleOf8 - len(in)
	in = strings.ToUpper(in) + "========"[:equals]
	return base32.HexEncoding.DecodeString(in)
}

// encryptSegment encrypts a path segment
//
// This uses EME with AES
//
// EME (ECB-Mix-ECB) is a wide-block encryption mode presented in the
// 2003 paper "A Parallelizable Enciphering Mode" by Halevi and
// Rogaway.
//
// This makes for deterministic encryption which is what we want - the
// same filename must encrypt to the same thing.
//
// This means that
//  * filenames with the same name will encrypt the same
//  * filenames which start the same won't have a common prefix
func (c *Cipher) encryptSegment(plaintext string) string {
	if plaintext == "" {
		return ""
	}
	paddedPlaintext := pkcs7.Pad(nameCipherBlockSize, []byte(plaintext))
	ciphertext := eme.Transform(c.block, c.nameTweak[:], paddedPlaintext, eme.DirectionEncrypt)
	return encodeFileName(ciphertext)
}

// decryptSegment decrypts a path segment
func (c *Cipher) decryptSegment(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	rawCiphertext, err := decodeFileName(ciphertext)
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
	dir = dir % 256

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
			pos = pos - thisdir
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
		if c.mode == NameEncryptionStandard {
			segments[i] = c.encryptSegment(segments[i])
		} else {
			segments[i] = c.obfuscateSegment(segments[i])
		}
	}
	return strings.Join(segments, "/")
}

// EncryptFileName encrypts a file path
func (c *Cipher) EncryptFileName(in string) string {
	if c.mode == NameEncryptionOff {
		return in + encryptedSuffix
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
		if c.mode == NameEncryptionStandard {
			segments[i], err = c.decryptSegment(segments[i])
		} else {
			segments[i], err = c.deobfuscateSegment(segments[i])
		}

		if err != nil {
			return "", err
		}
	}
	return strings.Join(segments, "/"), nil
}

// DecryptFileName decrypts a file path
func (c *Cipher) DecryptFileName(in string) (string, error) {
	if c.mode == NameEncryptionOff {
		remainingLength := len(in) - len(encryptedSuffix)
		if remainingLength > 0 && strings.HasSuffix(in, encryptedSuffix) {
			return in[:remainingLength], nil
		}
		return "", ErrorNotAnEncryptedFile
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

// pointer returns the nonce as a *[24]byte for secretbox
func (n *nonce) pointer() *[fileNonceSize]byte {
	return (*[fileNonceSize]byte)(n)
}

// fromReader fills the nonce from an io.Reader - normally the OSes
// crypto random number generator
func (n *nonce) fromReader(in io.Reader) error {
	read, err := io.ReadFull(in, (*n)[:])
	if read != fileNonceSize {
		return errors.Wrap(err, "short read of nonce")
	}
	return nil
}

// fromBuf fills the nonce from the buffer passed in
func (n *nonce) fromBuf(buf []byte) {
	read := copy((*n)[:], buf)
	if read != fileNonceSize {
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
	c        *Cipher
	nonce    nonce
	buf      []byte
	readBuf  []byte
	bufIndex int
	bufSize  int
	err      error
}

// newEncrypter creates a new file handle encrypting on the fly
func (c *Cipher) newEncrypter(in io.Reader, nonce *nonce) (*encrypter, error) {
	fh := &encrypter{
		in:      in,
		c:       c,
		buf:     c.getBlock(),
		readBuf: c.getBlock(),
		bufSize: fileHeaderSize,
	}
	// Initialise nonce
	if nonce != nil {
		fh.nonce = *nonce
	} else {
		err := fh.nonce.fromReader(c.cryptoRand)
		if err != nil {
			return nil, err
		}
	}
	// Copy magic into buffer
	copy(fh.buf, fileMagicBytes)
	// Copy nonce into buffer
	copy(fh.buf[fileMagicSize:], fh.nonce[:])
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
		readBuf := fh.readBuf[:blockDataSize]
		n, err = io.ReadFull(fh.in, readBuf)
		if n == 0 {
			// err can't be nil since:
			// n == len(buf) if and only if err == nil.
			return fh.finish(err)
		}
		// possibly err != nil here, but we will process the
		// data and the next call to ReadFull will return 0, err
		// Encrypt the block using the nonce
		secretbox.Seal(fh.buf[:0], readBuf[:n], fh.nonce.pointer(), &fh.c.dataKey)
		fh.bufIndex = 0
		fh.bufSize = blockHeaderSize + n
		fh.nonce.increment()
	}
	n = copy(p, fh.buf[fh.bufIndex:fh.bufSize])
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
	out, err := c.newEncrypter(in, nil)
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
	c            *Cipher
	buf          []byte
	readBuf      []byte
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
	readBuf := fh.readBuf[:fileHeaderSize]
	_, err := io.ReadFull(fh.rc, readBuf)
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		// This read from 0..fileHeaderSize-1 bytes
		return nil, fh.finishAndClose(ErrorEncryptedFileTooShort)
	} else if err != nil {
		return nil, fh.finishAndClose(err)
	}
	// check the magic
	if !bytes.Equal(readBuf[:fileMagicSize], fileMagicBytes) {
		return nil, fh.finishAndClose(ErrorEncryptedBadMagic)
	}
	// retrieve the nonce
	fh.nonce.fromBuf(readBuf[fileMagicSize:])
	fh.initialNonce = fh.nonce
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
		_, underlyingLimit, _, _ := calculateUnderlying(offset, limit)
		rc, err = open(ctx, 0, int64(fileHeaderSize)+underlyingLimit)
		setLimit = true
	} else {
		// Otherwise just read the header to start with
		rc, err = open(ctx, 0, int64(fileHeaderSize))
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
	n, err := io.ReadFull(fh.rc, readBuf)
	if n == 0 {
		// err can't be nil since:
		// n == len(buf) if and only if err == nil.
		return err
	}
	// possibly err != nil here, but we will process the data and
	// the next call to ReadFull will return 0, err

	// Check header + 1 byte exists
	if n <= blockHeaderSize {
		if err != nil {
			return err // return pending error as it is likely more accurate
		}
		return ErrorEncryptedFileBadHeader
	}
	// Decrypt the block using the nonce
	_, ok := secretbox.Open(fh.buf[:0], readBuf[:n], fh.nonce.pointer(), &fh.c.dataKey)
	if !ok {
		if err != nil {
			return err // return pending error as it is likely more accurate
		}
		return ErrorEncryptedBadBlock
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
	n = copy(p, fh.buf[fh.bufIndex:fh.bufIndex+toCopy])
	fh.bufIndex += n
	if fh.limit >= 0 {
		fh.limit -= int64(n)
		if fh.limit == 0 {
			return n, fh.finish(io.EOF)
		}
	}
	return n, nil
}

// calculateUnderlying converts an (offset, limit) in a crypted file
// into an (underlyingOffset, underlyingLimit) for the underlying
// file.
//
// It also returns number of bytes to discard after reading the first
// block and number of blocks this is from the start so the nonce can
// be incremented.
func calculateUnderlying(offset, limit int64) (underlyingOffset, underlyingLimit, discard, blocks int64) {
	// blocks we need to seek, plus bytes we need to discard
	blocks, discard = offset/blockDataSize, offset%blockDataSize

	// Offset in underlying stream we need to seek
	underlyingOffset = int64(fileHeaderSize) + blocks*(blockHeaderSize+blockDataSize)

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

	underlyingOffset, underlyingLimit, discard, blocks := calculateUnderlying(offset, limit)

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
			return 0, fh.finish(errors.Wrap(err, "couldn't reopen file with offset and limit"))
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
// The open function must return a ReadCloser opened to the offset supplied
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
	encryptedSize := int64(fileHeaderSize) + blocks*(blockHeaderSize+blockDataSize)
	if residue != 0 {
		encryptedSize += blockHeaderSize + residue
	}
	return encryptedSize
}

// DecryptedSize calculates the size of the data when decrypted
func (c *Cipher) DecryptedSize(size int64) (int64, error) {
	size -= int64(fileHeaderSize)
	if size < 0 {
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
