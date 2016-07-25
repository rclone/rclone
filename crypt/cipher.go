package crypt

import (
	"bytes"
	"crypto/aes"
	gocipher "crypto/cipher"
	"crypto/rand"
	"encoding/base32"
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

// Constancs
const (
	nameCipherBlockSize = aes.BlockSize
	fileMagic           = "RCLONE\x00\x00"
	fileMagicSize       = len(fileMagic)
	fileNonceSize       = 24
	fileHeaderSize      = fileMagicSize + fileNonceSize
	blockHeaderSize     = secretbox.Overhead
	blockDataSize       = 64 * 1024
	blockSize           = blockHeaderSize + blockDataSize
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
	ErrorBadSpreadNotSingleChar  = errors.New("bad unspread - not single character")
	ErrorBadSpreadResultTooShort = errors.New("bad unspread - result too short")
	ErrorBadSpreadDidntMatch     = errors.New("bad unspread - directory prefix didn't match")
	ErrorFileClosed              = errors.New("file already closed")
	scryptSalt                   = []byte{0xA8, 0x0D, 0xF4, 0x3A, 0x8F, 0xBD, 0x03, 0x08, 0xA7, 0xCA, 0xB8, 0x3E, 0x58, 0x1F, 0x86, 0xB1}
)

// Global variables
var (
	fileMagicBytes = []byte(fileMagic)
)

// Cipher is used to swap out the encryption implementations
type Cipher interface {
	// EncryptName encrypts a file path
	EncryptName(string) string
	// DecryptName decrypts a file path, returns error if decrypt was invalid
	DecryptName(string) (string, error)
	// EncryptData
	EncryptData(io.Reader) (io.Reader, error)
	// DecryptData
	DecryptData(io.ReadCloser) (io.ReadCloser, error)
	// EncryptedSize calculates the size of the data when encrypted
	EncryptedSize(int64) int64
	// DecryptedSize calculates the size of the data when decrypted
	DecryptedSize(int64) (int64, error)
}

type cipher struct {
	dataKey    [32]byte                  // Key for secretbox
	nameKey    [32]byte                  // 16,24 or 32 bytes
	nameTweak  [nameCipherBlockSize]byte // used to tweak the name crypto
	block      gocipher.Block
	flatten    int       // set flattening level - 0 is off
	buffers    sync.Pool // encrypt/decrypt buffers
	cryptoRand io.Reader // read crypto random numbers from here
}

func newCipher(flatten int, password string) (*cipher, error) {
	c := &cipher{
		flatten:    flatten,
		cryptoRand: rand.Reader,
	}
	c.buffers.New = func() interface{} {
		return make([]byte, blockSize)
	}
	err := c.Key(password)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// Key creates all the internal keys from the password passed in using
// scrypt.  We use a fixed salt just to make attackers lives slighty
// harder than using no salt.
//
// Note that empty passsword makes all 0x00 keys which is used in the
// tests.
func (c *cipher) Key(password string) (err error) {
	const keySize = len(c.dataKey) + len(c.nameKey) + len(c.nameTweak)
	var key []byte
	if password == "" {
		key = make([]byte, keySize)
	} else {
		key, err = scrypt.Key([]byte(password), scryptSalt, 16384, 8, 1, keySize)
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

// spread a name over the given number of directory levels
//
// if in isn't long enough dirs will be reduces
func spreadName(dirs int, in string) string {
	if dirs > len(in) {
		dirs = len(in)
	}
	prefix := ""
	for i := 0; i < dirs; i++ {
		prefix += string(in[i]) + "/"
	}
	return prefix + in
}

// reverse spreadName, returning an error if not in spread format
//
// This decodes any level of spreading
func unspreadName(in string) (string, error) {
	in = strings.ToLower(in)
	segments := strings.Split(in, "/")
	if len(segments) == 0 {
		return in, nil
	}
	out := segments[len(segments)-1]
	segments = segments[:len(segments)-1]
	for i, s := range segments {
		if len(s) != 1 {
			return "", ErrorBadSpreadNotSingleChar
		}
		if i >= len(out) {
			return "", ErrorBadSpreadResultTooShort
		}
		if s[0] != out[i] {
			return "", ErrorBadSpreadDidntMatch
		}
	}
	return out, nil
}

// EncryptName encrypts a file path
func (c *cipher) EncryptName(in string) string {
	if c.flatten > 0 {
		return spreadName(c.flatten, c.encryptSegment(in))
	}
	segments := strings.Split(in, "/")
	for i := range segments {
		segments[i] = c.encryptSegment(segments[i])
	}
	return strings.Join(segments, "/")
}

// DecryptName decrypts a file path
func (c *cipher) DecryptName(in string) (string, error) {
	if c.flatten > 0 {
		unspread, err := unspreadName(in)
		if err != nil {
			return "", err
		}
		return c.decryptSegment(unspread)
	}
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

// increment to add 1 to the nonce
func (n *nonce) increment() {
	for i := 0; i < len(*n); i++ {
		digit := (*n)[i]
		newDigit := digit + 1
		(*n)[i] = newDigit
		if newDigit >= digit {
			// exit if no carry
			break
		}
	}
}

// encrypter encrypts an io.Reader on the fly
type encrypter struct {
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
	if fh.err != nil {
		return 0, fh.err
	}
	if fh.bufIndex >= fh.bufSize {
		// Read data
		// FIXME should overlap the reads with a go-routine and 2 buffers?
		readBuf := fh.readBuf[:blockDataSize]
		n, err = io.ReadFull(fh.in, readBuf)
		if err == io.EOF {
			// ReadFull only returns n=0 and EOF
			return fh.finish(io.EOF)
		} else if err == io.ErrUnexpectedEOF {
			// Next read will return EOF
		} else if err != nil {
			return fh.finish(err)
		}
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
	fh.c.putBlock(fh.readBuf)
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
	rc       io.ReadCloser
	nonce    nonce
	c        *cipher
	buf      []byte
	readBuf  []byte
	bufIndex int
	bufSize  int
	err      error
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
	return fh, nil
}

// Read as per io.Reader
func (fh *decrypter) Read(p []byte) (n int, err error) {
	if fh.err != nil {
		return 0, fh.err
	}
	if fh.bufIndex >= fh.bufSize {
		// Read data
		// FIXME should overlap the reads with a go-routine and 2 buffers?
		readBuf := fh.readBuf
		n, err = io.ReadFull(fh.rc, readBuf)
		if err == io.EOF {
			// ReadFull only returns n=0 and EOF
			return 0, fh.finish(io.EOF)
		} else if err == io.ErrUnexpectedEOF {
			// Next read will return EOF
		} else if err != nil {
			return 0, fh.finish(err)
		}
		// Check header + 1 byte exists
		if n <= blockHeaderSize {
			return 0, fh.finish(ErrorEncryptedFileBadHeader)
		}
		// Decrypt the block using the nonce
		block := fh.buf
		_, ok := secretbox.Open(block[:0], readBuf[:n], fh.nonce.pointer(), &fh.c.dataKey)
		if !ok {
			return 0, fh.finish(ErrorEncryptedBadBlock)
		}
		fh.bufIndex = 0
		fh.bufSize = n - blockHeaderSize
		fh.nonce.increment()
	}
	n = copy(p, fh.buf[fh.bufIndex:fh.bufSize])
	fh.bufIndex += n
	return n, nil
}

// finish sets the final error and tidies up
func (fh *decrypter) finish(err error) error {
	if fh.err != nil {
		return fh.err
	}
	fh.err = err
	fh.c.putBlock(fh.buf)
	fh.c.putBlock(fh.readBuf)
	return err
}

// Close
func (fh *decrypter) Close() error {
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
