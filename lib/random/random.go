// Package random holds a few functions for working with random numbers
package random

import (
	cryptorand "crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

// StringFn create a random string for test purposes using the random
// number generator function passed in.
//
// Do not use these for passwords.
func StringFn(n int, randReader io.Reader) string {
	const (
		vowel     = "aeiou"
		consonant = "bcdfghjklmnpqrstvwxyz"
		digit     = "0123456789"
	)
	var (
		pattern = []string{consonant, vowel, consonant, vowel, consonant, vowel, consonant, digit}
		out     = make([]byte, n)
		p       = 0
	)
	_, err := io.ReadFull(randReader, out)
	if err != nil {
		panic(fmt.Sprintf("internal error: failed to read from random reader: %v", err))
	}
	for i := range out {
		source := pattern[p]
		p = (p + 1) % len(pattern)
		// this generation method means the distribution is slightly biased. However these
		// strings are not for passwords so this is deemed OK.
		out[i] = source[out[i]%byte(len(source))]
	}
	return string(out)
}

// String create a random string for test purposes.
//
// Do not use these for passwords.
func String(n int) string {
	return StringFn(n, cryptorand.Reader)
}

// Password creates a crypto strong password which is just about
// memorable.  The password is composed of printable ASCII characters
// from the URL encoding base64 alphabet (A-Za-z0-9_-).
//
// Requires password strength in bits.
// 64 is just about memorable
// 128 is secure
func Password(bits int) (password string, err error) {
	bytes := bits / 8
	if bits%8 != 0 {
		bytes++
	}
	var pw = make([]byte, bytes)
	n, err := cryptorand.Read(pw)
	if err != nil {
		return "", fmt.Errorf("password read failed: %w", err)
	}
	if n != bytes {
		return "", fmt.Errorf("password short read: %d", n)
	}
	password = base64.RawURLEncoding.EncodeToString(pw)
	return password, nil
}
