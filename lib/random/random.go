// Package random holds a few functions for working with random numbers
package random

import (
	cryptorand "crypto/rand"
	"encoding/base64"
	"encoding/binary"
	mathrand "math/rand"

	"github.com/pkg/errors"
)

// StringFn create a random string for test purposes using the random
// number generator function passed in.
//
// Do not use these for passwords.
func StringFn(n int, randIntn func(n int) int) string {
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
		out[i] = source[randIntn(len(source))]
	}
	return string(out)
}

// String create a random string for test purposes.
//
// Do not use these for passwords.
func String(n int) string {
	return StringFn(n, mathrand.Intn)
}

// Password creates a crypto strong password which is just about
// memorable.  The password is composed of printable ASCII characters
// from the base64 alphabet.
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
		return "", errors.Wrap(err, "password read failed")
	}
	if n != bytes {
		return "", errors.Errorf("password short read: %d", n)
	}
	password = base64.RawURLEncoding.EncodeToString(pw)
	return password, nil
}

// Seed the global math/rand with crypto strong data
//
// This doesn't make it OK to use math/rand in crypto sensitive
// environments - don't do that! However it does help to mitigate the
// problem if that happens accidentally. This would have helped with
// CVE-2020-28924 - #4783
func Seed() error {
	var seed int64
	err := binary.Read(cryptorand.Reader, binary.LittleEndian, &seed)
	if err != nil {
		return errors.Wrap(err, "failed to read random seed")
	}
	mathrand.Seed(seed)
	return nil
}
