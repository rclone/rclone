// Package obscure contains the Obscure and Reveal commands
package obscure

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/rclone/rclone/fs"
)

// crypt internals
var (
	cryptKey = []byte{
		0x9c, 0x93, 0x5b, 0x48, 0x73, 0x0a, 0x55, 0x4d,
		0x6b, 0xfd, 0x7c, 0x63, 0xc8, 0x86, 0xa9, 0x2b,
		0xd3, 0x90, 0x19, 0x8e, 0xb8, 0x12, 0x8a, 0xfb,
		0xf4, 0xde, 0x16, 0x2b, 0x8b, 0x95, 0xf6, 0x38,
	}
	cryptBlock cipher.Block
	cryptRand  = rand.Reader
)

// crypt transforms in to out using iv under AES-CTR.
//
// in and out may be the same buffer.
//
// Note encryption and decryption are the same operation
func crypt(out, in, iv []byte) error {
	if cryptBlock == nil {
		var err error
		cryptBlock, err = aes.NewCipher(cryptKey)
		if err != nil {
			return err
		}
	}
	stream := cipher.NewCTR(cryptBlock, iv)
	stream.XORKeyStream(out, in)
	return nil
}

// Obscure a value
//
// This is done by encrypting with AES-CTR
func Obscure(x string) (string, error) {
	plaintext := []byte(x)
	if math.MaxInt32-aes.BlockSize < len(plaintext) {
		return "", fmt.Errorf("value too large")
	}
	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(cryptRand, iv); err != nil {
		return "", fmt.Errorf("failed to read iv: %w", err)
	}
	if err := crypt(ciphertext[aes.BlockSize:], plaintext, iv); err != nil {
		return "", fmt.Errorf("encrypt failed: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(ciphertext), nil
}

// MustObscure obscures a value, exiting with a fatal error if it failed
func MustObscure(x string) string {
	out, err := Obscure(x)
	if err != nil {
		fs.Fatalf(nil, "Obscure failed: %v", err)
	}
	return out
}

// Reveal an obscured value
func Reveal(x string) (string, error) {
	ciphertext, err := base64.RawURLEncoding.DecodeString(x)
	if err != nil {
		return "", fmt.Errorf("base64 decode failed when revealing password - is it obscured?: %w", err)
	}
	if len(ciphertext) < aes.BlockSize {
		return "", errors.New("input too short when revealing password - is it obscured?")
	}
	buf := ciphertext[aes.BlockSize:]
	iv := ciphertext[:aes.BlockSize]
	if err := crypt(buf, buf, iv); err != nil {
		return "", fmt.Errorf("decrypt failed when revealing password - is it obscured?: %w", err)
	}
	return string(buf), nil
}

// MustReveal reveals an obscured value, exiting with a fatal error if it failed
func MustReveal(x string) string {
	out, err := Reveal(x)
	if err != nil {
		fs.Fatalf(nil, "Reveal failed: %v", err)
	}
	return out
}
