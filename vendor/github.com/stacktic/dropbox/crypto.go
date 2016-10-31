/*
** Copyright (c) 2014 Arnaud Ysmal.  All Rights Reserved.
**
** Redistribution and use in source and binary forms, with or without
** modification, are permitted provided that the following conditions
** are met:
** 1. Redistributions of source code must retain the above copyright
**    notice, this list of conditions and the following disclaimer.
** 2. Redistributions in binary form must reproduce the above copyright
**    notice, this list of conditions and the following disclaimer in the
**    documentation and/or other materials provided with the distribution.
**
** THIS SOFTWARE IS PROVIDED BY THE AUTHOR ``AS IS'' AND ANY EXPRESS
** OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
** WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
** DISCLAIMED. IN NO EVENT SHALL THE AUTHOR OR CONTRIBUTORS BE LIABLE
** FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
** DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
** SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION)
** HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT
** LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY
** OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF
** SUCH DAMAGE.
 */

package dropbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"io"
	"os"
)

// GenerateKey generates a key by reading length bytes from /dev/random
func GenerateKey(length int) ([]byte, error) {
	var err error
	var fd io.Reader
	var rv []byte

	if fd, err = os.Open("/dev/random"); err != nil {
		return nil, err
	}
	rv = make([]byte, length)
	_, err = io.ReadFull(fd, rv)
	return rv, err
}

func newCrypter(key []byte, in io.Reader, size int, newCipher func(key []byte) (cipher.Block, error)) (io.ReadCloser, int, error) {
	var block cipher.Block
	var err error

	if block, err = newCipher(key); err != nil {
		return nil, 0, err
	}
	outsize := size - size%block.BlockSize() + 2*block.BlockSize()

	rd, wr := io.Pipe()
	go encrypt(block, in, size, wr)
	return rd, outsize, nil
}

func newDecrypter(key []byte, in io.Reader, size int, newCipher func(key []byte) (cipher.Block, error)) (io.ReadCloser, error) {
	var block cipher.Block
	var err error

	if block, err = newCipher(key); err != nil {
		return nil, err
	}

	rd, wr := io.Pipe()
	go decrypt(block, in, size, wr)
	return rd, nil
}

// NewAESDecrypterReader creates and returns a new io.ReadCloser to decrypt the given io.Reader containing size bytes with the given AES key.
// The AES key should be either 16, 24, or 32 bytes to select AES-128, AES-192, or AES-256.
func NewAESDecrypterReader(key []byte, input io.Reader, size int) (io.ReadCloser, error) {
	return newDecrypter(key, input, size, aes.NewCipher)
}

// NewAESCrypterReader creates and returns a new io.ReadCloser to encrypt the given io.Reader containing size bytes with the given AES key.
// The AES key should be either 16, 24, or 32 bytes to select AES-128, AES-192, or AES-256.
func NewAESCrypterReader(key []byte, input io.Reader, size int) (io.ReadCloser, int, error) {
	return newCrypter(key, input, size, aes.NewCipher)
}

func encrypt(block cipher.Block, in io.Reader, size int, out io.WriteCloser) error {
	var err error
	var rd int
	var buf []byte
	var last bool
	var encrypter cipher.BlockMode

	defer out.Close()

	buf = make([]byte, block.BlockSize())

	if _, err = io.ReadFull(rand.Reader, buf); err != nil {
		return err
	}
	encrypter = cipher.NewCBCEncrypter(block, buf)

	if _, err = out.Write(buf); err != nil {
		return err
	}
	for !last {
		if rd, err = io.ReadFull(in, buf); err != nil {
			if err == io.ErrUnexpectedEOF || err == io.EOF {
				buf = buf[:rd]
				buf = append(buf, 0x80)
				for len(buf) < block.BlockSize() {
					buf = append(buf, 0x00)
				}
				last = true
			} else {
				return err
			}
		}
		encrypter.CryptBlocks(buf, buf)
		if _, err = out.Write(buf); err != nil {
			return err
		}
	}
	return nil
}

func decrypt(block cipher.Block, in io.Reader, size int, out io.WriteCloser) error {
	var err error
	var buf []byte
	var count int
	var decrypter cipher.BlockMode

	defer out.Close()

	buf = make([]byte, block.BlockSize())
	if _, err = io.ReadFull(in, buf); err != nil {
		return err
	}
	decrypter = cipher.NewCBCDecrypter(block, buf)

	count = (size - block.BlockSize()) / block.BlockSize()
	for count > 0 && err == nil {
		if _, err = io.ReadFull(in, buf); err == nil {
			decrypter.CryptBlocks(buf, buf)
			if count == 1 {
				for count = block.BlockSize() - 1; buf[count] == 0x00; count-- {
					continue
				}
				if buf[count] == 0x80 {
					buf = buf[:count]
				}
			}
			_, err = out.Write(buf)
		}
		count--
	}
	if err == io.EOF {
		return nil
	}
	return err
}

// FilesPutAES uploads and encrypts size bytes from the input reader to the dst path on Dropbox.
func (db *Dropbox) FilesPutAES(key []byte, input io.ReadCloser, size int64, dst string, overwrite bool, parentRev string) (*Entry, error) {
	var encreader io.ReadCloser
	var outsize int
	var err error

	if encreader, outsize, err = NewAESCrypterReader(key, input, int(size)); err != nil {
		return nil, err
	}
	return db.FilesPut(encreader, int64(outsize), dst, overwrite, parentRev)
}

// UploadFileAES uploads and encrypts the file located in the src path on the local disk to the dst path on Dropbox.
func (db *Dropbox) UploadFileAES(key []byte, src, dst string, overwrite bool, parentRev string) (*Entry, error) {
	var err error
	var fd *os.File
	var fsize int64

	if fd, err = os.Open(src); err != nil {
		return nil, err
	}
	defer fd.Close()

	if fi, err := fd.Stat(); err == nil {
		fsize = fi.Size()
	} else {
		return nil, err
	}
	return db.FilesPutAES(key, fd, fsize, dst, overwrite, parentRev)
}

// DownloadAES downloads and decrypts the file located in the src path on Dropbox and returns a io.ReadCloser.
func (db *Dropbox) DownloadAES(key []byte, src, rev string, offset int) (io.ReadCloser, error) {
	var in io.ReadCloser
	var size int64
	var err error

	if in, size, err = db.Download(src, rev, int64(offset)); err != nil {
		return nil, err
	}
	return NewAESDecrypterReader(key, in, int(size))
}

// DownloadToFileAES downloads and decrypts the file located in the src path on Dropbox to the dst file on the local disk.
func (db *Dropbox) DownloadToFileAES(key []byte, src, dst, rev string) error {
	var input io.ReadCloser
	var fd *os.File
	var err error

	if fd, err = os.Create(dst); err != nil {
		return err
	}
	defer fd.Close()

	if input, err = db.DownloadAES(key, src, rev, 0); err != nil {
		os.Remove(dst)
		return err
	}
	defer input.Close()
	if _, err = io.Copy(fd, input); err != nil {
		os.Remove(dst)
	}
	return err
}
