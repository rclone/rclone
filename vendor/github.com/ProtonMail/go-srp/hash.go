//  The MIT License
//
//  Copyright (c) 2019 Proton Technologies AG
//
//  Permission is hereby granted, free of charge, to any person obtaining a copy
//  of this software and associated documentation files (the "Software"), to deal
//  in the Software without restriction, including without limitation the rights
//  to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
//  copies of the Software, and to permit persons to whom the Software is
//  furnished to do so, subject to the following conditions:
//
//  The above copyright notice and this permission notice shall be included in
//  all copies or substantial portions of the Software.
//
//  THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
//  IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
//  FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
//  AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
//  LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
//  OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
//  THE SOFTWARE.

package srp

import (
	"bytes"
	"crypto/md5"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/ProtonMail/bcrypt"
)

//based64DotSlash Bcrypt uses an adapted base64 alphabet (using . instead of +, starting with ./ and with no padding).
var based64DotSlash = base64.NewEncoding("./ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789").WithPadding(base64.NoPadding)

// bcryptHash  bcrypt hash function with prefix start strings
//
// Parameters:
//	 - password []byte: the thing we need to keep secret and to a hash. mostly the passwords
//	 - encodedSalt string: a salt must encoded with based64DotSlash. the salt size before encoded is 128 bits for our workflow
// Returns:
//   - hashed []byte: a hashed password
//   - err error: throw error
// Usage:
//
func bcryptHash(password []byte, encodedSalt string) (hashed []byte, err error) {
	return bcrypt.HashBytes(password, []byte("$2y$10$"+encodedSalt))
}

// expandHash extends the byte data for SRP flow
func expandHash(data []byte) []byte {
	part0 := sha512.Sum512(append(data, 0))
	part1 := sha512.Sum512(append(data, 1))
	part2 := sha512.Sum512(append(data, 2))
	part3 := sha512.Sum512(append(data, 3))
	return bytes.Join([][]byte{
		part0[:],
		part1[:],
		part2[:],
		part3[:],
	}, []byte{})
}

// MailboxPassword get mailbox password hash
//
// Parameters:
//	 - password []byte: a mailbox password
//	 - salt []byte: a salt is random 128 bits data
// Returns:
//   - hashed []byte: a hashed password
//   - err error: throw error
func MailboxPassword(password []byte, salt []byte) (hashed []byte, err error) {
	encodedSalt := based64DotSlash.EncodeToString(salt)
	hashed, err = bcryptHash(password, encodedSalt)
	return
}

// HashPassword returns the hash of password argument. Based on version number
// following arguments are used in addition to password:
// * 0, 1, 2: userName and modulus
// * 3, 4: salt and modulus
func HashPassword(authVersion int, password []byte, userName string, salt, modulus []byte) ([]byte, error) {
	switch authVersion {
	case 4, 3:
		return hashPasswordVersion3(password, salt, modulus)
	case 2:
		return hashPasswordVersion2(password, userName, modulus)
	case 1:
		return hashPasswordVersion1(password, userName, modulus)
	case 0:
		return hashPasswordVersion0(password, userName, modulus)
	default:
		return nil, errors.New("pmapi: unsupported auth version")
	}
}

// cleanUserName returns the input string in lower-case without characters `_`,
// `.` and `-`.
func cleanUserName(userName string) string {
	userName = strings.Replace(userName, "-", "", -1)
	userName = strings.Replace(userName, ".", "", -1)
	userName = strings.Replace(userName, "_", "", -1)
	return strings.ToLower(userName)
}

func hashPasswordVersion3(password []byte, salt, modulus []byte) (res []byte, err error) {
	encodedSalt := based64DotSlash.EncodeToString(append(salt, []byte("proton")...))
	crypted, err := bcryptHash(password, encodedSalt)
	if err != nil {
		return
	}

	return expandHash(append(crypted, modulus...)), nil
}

func hashPasswordVersion2(password []byte, userName string, modulus []byte) (res []byte, err error) {
	return hashPasswordVersion1(password, cleanUserName(userName), modulus)
}

func hashPasswordVersion1(password []byte, userName string, modulus []byte) (res []byte, err error) {
	prehashed := md5.Sum([]byte(strings.ToLower(userName)))
	encodedSalt := hex.EncodeToString(prehashed[:])
	crypted, err := bcryptHash(password, encodedSalt)
	if err != nil {
		return
	}

	return expandHash(append(crypted, modulus...)), nil
}

func hashPasswordVersion0(password []byte, userName string, modulus []byte) (res []byte, err error) {
	b64Hash := make([]byte, 88) // 64 bytes in base64
	userAndPass := append([]byte(strings.ToLower(userName)), password...)
	defer clear(userAndPass)
	prehashed := sha512.Sum512(userAndPass)
	defer clear(prehashed[:])
	base64.StdEncoding.Encode(b64Hash, prehashed[:])
	defer clear(b64Hash)
	return hashPasswordVersion1(b64Hash, userName, modulus)
}

func clear(w []byte) {
	for k := range w {
		w[k] = 0x00
	}
}
