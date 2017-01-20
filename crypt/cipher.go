package crypt

import (
	"bytes"
	"crypto/aes"
	gocipher "crypto/cipher"
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"io"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/ncw/rclone/crypt/pkcs7"
	"github.com/pkg/errors"

	"golang.org/x/crypto/nacl/secretbox"
	"golang.org/x/crypto/scrypt"

	"github.com/rfjakob/eme"
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
	ErrorEncryptedFileTooShort   = errors.New("file is too short to be encrypted")
	ErrorEncryptedFileBadHeader  = errors.New("file has truncated block header")
	ErrorEncryptedBadMagic       = errors.New("not an encrypted file - bad magic string")
	ErrorEncryptedBadBlock       = errors.New("failed to authenticate decrypted block - bad password?")
	ErrorBadBase32Encoding       = errors.New("bad base32 filename encoding")
	ErrorFileClosed              = errors.New("file already closed")
	ErrorNotAnEncryptedFile      = errors.New("not an encrypted file - no \"" + encryptedSuffix + "\" suffix")
	ErrorBadSeek                 = errors.New("Seek beyond end of file")
	defaultSalt                  = []byte{0xA8, 0x0D, 0xF4, 0x3A, 0x8F, 0xBD, 0x03, 0x08, 0xA7, 0xCA, 0xB8, 0x3E, 0x58, 0x1F, 0x86, 0xB1}
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
}

// OpenAtOffset opens the file handle at the offset given
type OpenAtOffset func(offset int64) (io.ReadCloser, error)

// Cipher is used to swap out the encryption implementations
type Cipher interface {
	// EncryptFileName encrypts a file path
	EncryptFileName(string) string
	// DecryptFileName decrypts a file path, returns error if decrypt was invalid
	DecryptFileName(string) (string, error)
	// EncryptDirName encrypts a directory path
	EncryptDirName(string) string
	// DecryptDirName decrypts a directory path, returns error if decrypt was invalid
	DecryptDirName(string) (string, error)
	// EncryptData
	EncryptData(io.Reader) (io.Reader, error)
	// DecryptData
	DecryptData(io.ReadCloser) (io.ReadCloser, error)
	// DecryptDataSeek decrypt at a given position
	DecryptDataSeek(open OpenAtOffset, offset int64) (ReadSeekCloser, error)
	// EncryptedSize calculates the size of the data when encrypted
	EncryptedSize(int64) int64
	// DecryptedSize calculates the size of the data when decrypted
	DecryptedSize(int64) (int64, error)
}

// NameEncryptionMode is the type of file name encryption in use
type NameEncryptionMode int

// NameEncryptionMode levels
const (
	NameEncryptionOff NameEncryptionMode = iota
	NameEncryptionStandard
)

// NewNameEncryptionMode turns a string into a NameEncryptionMode
func NewNameEncryptionMode(s string) (mode NameEncryptionMode, err error) {
	s = strings.ToLower(s)
	switch s {
	case "off":
		mode = NameEncryptionOff
	case "standard":
		mode = NameEncryptionStandard
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
	default:
		out = fmt.Sprintf("Unknown mode #%d", mode)
	}
	return out
}

type cipher struct {
	dataKey    [32]byte                  // Key for secretbox
	nameKey    [32]byte                  // 16,24 or 32 bytes
	nameTweak  [nameCipherBlockSize]byte // used to tweak the name crypto
	block      gocipher.Block
	mode       NameEncryptionMode
	buffers    sync.Pool // encrypt/decrypt buffers
	cryptoRand io.Reader // read crypto random numbers from here
}

// newCipher initialises the cipher.  If salt is "" then it uses a built in salt val
func newCipher(mode NameEncryptionMode, password, salt string) (*cipher, error) {
	c := &cipher{
		mode:       mode,
		cryptoRand: rand.Reader,
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
// Note that empty passsword makes all 0x00 keys which is used in the
// tests.
func (c *cipher) Key(password, salt string) (err error) {
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
func (c *cipher) getBlock() []byte {
	return c.buffers.Get().([]byte)
}

// putBlock returns a block to the pool of size blockSize
func (c *cipher) putBlock(buf []byte) {
	if len(buf) != blockSize {
		panic("bad blocksize returned to pool")
	}
	c.buffers.Put(buf)
}

// check to see if the byte string is valid with no control characters
// from 0x00 to 0x1F and is a valid UTF-8 string
func checkValidString(buf []byte) error {
	for i := range buf {
		c := buf[i]
		if c >= 0x00 && c < 0x20 || c == 0x7F {
			return ErrorBadDecryptControlChar
		}
	}
	if !utf8.Valid(buf) {
		return ErrorBadDecryptUTF8
	}
	return nil
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
// This makes for determinstic encryption which is what we want - the
// same filename must encrypt to the same thing.
//
// This means that
//  * filenames with the same name will encrypt the same
//  * filenames which start the same won't have a common prefix
func (c *cipher) encryptSegment(plaintext string) string {
	if plaintext == "" {
		return ""
	}
	paddedPlaintext := pkcs7.Pad(nameCipherBlockSize, []byte(plaintext))
	ciphertext := eme.Transform(c.block, c.nameTweak[:], paddedPlaintext, eme.DirectionEncrypt)
	return encodeFileName(ciphertext)
}

// decryptSegment decrypts a path segment
func (c *cipher) decryptSegment(ciphertext string) (string, error) {
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
	paddedPlaintext := eme.Transform(c.block, c.nameTweak[:], rawCiphertext, eme.DirectionDecrypt)
	plaintext, err := pkcs7.Unpad(nameCipherBlockSize, paddedPlaintext)
	if err != nil {
		return "", err
	}
	err = checkValidString(plaintext)
	if err != nil {
		return "", err
	}
	return string(plaintext), err
}

// encryptFileName encrypts a file path
func (c *cipher) encryptFileName(in string) string {
	segments := strings.Split(in, "/")
	for i := range segments {
		segments[i] = c.encryptSegment(segments[i])
	}
	return strings.Join(segments, "/")
}

// EncryptFileName encrypts a file path
func (c *cipher) EncryptFileName(in string) string {
	if c.mode == NameEncryptionOff {
		return in + encryptedSuffix
	}
	return c.encryptFileName(in)
}

// EncryptDirName encrypts a directory path
func (c *cipher) EncryptDirName(in string) string {
	if c.mode == NameEncryptionOff {
		return in
	}
	return c.encryptFileName(in)
}

// decryptFileName decrypts a file path
func (c *cipher) decryptFileName(in string) (string, error) {
	segments := strings.Split(in, "/")
	for i := range segments {
		var err error
		segments[i], err = c.decryptSegment(segments[i])
		if err != nil {
			return "", err
		}
	}
	return strings.Join(segments, "/"), nil
}

// DecryptFileName decrypts a file path
func (c *cipher) DecryptFileName(in string) (string, error) {
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
func (c *cipher) DecryptDirName(in string) (string, error) {
	if c.mode == NameEncryptionOff {
		return in, nil
	}
	return c.decryptFileName(in)
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

// add an uint64 to the nonce
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
	c        *cipher
	nonce    nonce
	buf      []byte
	readBuf  []byte
	bufIndex int
	bufSize  int
	err      error
}

// newEncrypter creates a new file handle encrypting on the fly
func (c *cipher) newEncrypter(in io.Reader) (*encrypter, error) {
	fh := &encrypter{
		in:      in,
		c:       c,
		buf:     c.getBlock(),
		readBuf: c.getBlock(),
		bufSize: fileHeaderSize,
	}
	// Initialise nonce
	err := fh.nonce.fromReader(c.cryptoRand)
	if err != nil {
		return nil, err
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
		// Write nonce to start of block
		copy(fh.buf, fh.nonce[:])
		// Encrypt the block using the nonce
		block := fh.buf
		secretbox.Seal(block[:0], readBuf[:n], fh.nonce.pointer(), &fh.c.dataKey)
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
func (c *cipher) EncryptData(in io.Reader) (io.Reader, error) {
	out, err := c.newEncrypter(in)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// decrypter decrypts an io.ReaderCloser on the fly
type decrypter struct {
	mu           sync.Mutex
	rc           io.ReadCloser
	nonce        nonce
	initialNonce nonce
	c            *cipher
	buf          []byte
	readBuf      []byte
	bufIndex     int
	bufSize      int
	err          error
	open         OpenAtOffset
}

// newDecrypter creates a new file handle decrypting on the fly
func (c *cipher) newDecrypter(rc io.ReadCloser) (*decrypter, error) {
	fh := &decrypter{
		rc:      rc,
		c:       c,
		buf:     c.getBlock(),
		readBuf: c.getBlock(),
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
	// retreive the nonce
	fh.nonce.fromBuf(readBuf[fileMagicSize:])
	fh.initialNonce = fh.nonce
	return fh, nil
}

// newDecrypterSeek creates a new file handle decrypting on the fly
func (c *cipher) newDecrypterSeek(open OpenAtOffset, offset int64) (fh *decrypter, err error) {
	// Open initially with no seek
	rc, err := open(0)
	if err != nil {
		return nil, err
	}
	// Open the stream which fills in the nonce
	fh, err = c.newDecrypter(rc)
	if err != nil {
		return nil, err
	}
	fh.open = open // will be called by fh.Seek
	if offset != 0 {
		_, err = fh.Seek(offset, 0)
		if err != nil {
			_ = fh.Close()
			return nil, err
		}
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
	block := fh.buf
	_, ok := secretbox.Open(block[:0], readBuf[:n], fh.nonce.pointer(), &fh.c.dataKey)
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
	n = copy(p, fh.buf[fh.bufIndex:fh.bufSize])
	fh.bufIndex += n
	return n, nil
}

// Seek as per io.Seeker
func (fh *decrypter) Seek(offset int64, whence int) (int64, error) {
	fh.mu.Lock()
	defer fh.mu.Unlock()

	if fh.open == nil {
		return 0, fh.finish(errors.New("can't seek - not initialised with newDecrypterSeek"))
	}
	if whence != 0 {
		return 0, fh.finish(errors.New("can only seek from the start"))
	}

	// Reset error or return it if not EOF
	if fh.err == io.EOF {
		fh.unFinish()
	} else if fh.err != nil {
		return 0, fh.err
	}

	// blocks we need to seek, plus bytes we need to discard
	blocks, discard := offset/blockDataSize, offset%blockDataSize

	// Offset in underlying stream we need to seek
	underlyingOffset := int64(fileHeaderSize) + blocks*(blockHeaderSize+blockDataSize)

	// Move the nonce on the correct number of blocks from the start
	fh.nonce = fh.initialNonce
	fh.nonce.add(uint64(blocks))

	// Can we seek underlying stream directly?
	if do, ok := fh.rc.(io.Seeker); ok {
		// Seek underlying stream directly
		_, err := do.Seek(underlyingOffset, 0)
		if err != nil {
			return 0, fh.finish(err)
		}
	} else {
		// if not reopen with seek
		_ = fh.rc.Close() // close underlying file
		fh.rc = nil

		// Re-open the underlying object with the offset given
		rc, err := fh.open(underlyingOffset)
		if err != nil {
			return 0, fh.finish(errors.Wrap(err, "couldn't reopen file with offset"))
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

	return offset, nil
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
func (c *cipher) DecryptData(rc io.ReadCloser) (io.ReadCloser, error) {
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
func (c *cipher) DecryptDataSeek(open OpenAtOffset, offset int64) (ReadSeekCloser, error) {
	out, err := c.newDecrypterSeek(open, offset)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// EncryptedSize calculates the size of the data when encrypted
func (c *cipher) EncryptedSize(size int64) int64 {
	blocks, residue := size/blockDataSize, size%blockDataSize
	encryptedSize := int64(fileHeaderSize) + blocks*(blockHeaderSize+blockDataSize)
	if residue != 0 {
		encryptedSize += blockHeaderSize + residue
	}
	return encryptedSize
}

// DecryptedSize calculates the size of the data when decrypted
func (c *cipher) DecryptedSize(size int64) (int64, error) {
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
	_ Cipher        = (*cipher)(nil)
	_ io.ReadCloser = (*decrypter)(nil)
	_ io.Reader     = (*encrypter)(nil)
)
