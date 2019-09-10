// Package random holds a few functions for working with random numbers
package random

import (
	"encoding/base64"
	"math/rand"

	"github.com/pkg/errors"
)

// String create a random string for test purposes.
//
// Do not use these for passwords.
func String(n int) string {
	const (
		vowel     = "aeiou"
		consonant = "bcdfghjklmnpqrstvwxyz"
		digit     = "0123456789"
	)
	pattern := []string{consonant, vowel, consonant, vowel, consonant, vowel, consonant, digit}
	out := make([]byte, n)
	p := 0
	for i := range out {
		source := pattern[p]
		p = (p + 1) % len(pattern)
		out[i] = source[rand.Intn(len(source))]
	}
	return string(out)
}

// Password creates a crypto strong password which is just about
// memorable.  The password is composed of printable ASCII characters
// from the base64 alphabet.
//
// Requres password strength in bits.
// 64 is just about memorable
// 128 is secure
func Password(bits int) (password string, err error) {
	bytes := bits / 8
	if bits%8 != 0 {
		bytes++
	}
	var pw = make([]byte, bytes)
	n, err := rand.Read(pw)
	if err != nil {
		return "", errors.Wrap(err, "password read failed")
	}
	if n != bytes {
		return "", errors.Errorf("password short read: %d", n)
	}
	password = base64.RawURLEncoding.EncodeToString(pw)
	return password, nil
}
