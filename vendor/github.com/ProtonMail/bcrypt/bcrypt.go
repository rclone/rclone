package bcrypt

import (
	"bytes"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
)

var (
	InvalidRounds = errors.New("bcrypt: Invalid rounds parameter")
	InvalidSalt   = errors.New("bcrypt: Invalid salt supplied")
)

const (
	MaxRounds      = 31
	MinRounds      = 4
	DefaultRounds  = 12
	SaltLen        = 16
	BlowfishRounds = 16
)

var enc = base64.NewEncoding("./ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789")

// Helper function to build the bcrypt hash string
// payload takes :
//		* []byte -> which it base64 encodes it (trims padding "=") and writes it to the buffer
//		* string -> which it writes straight to the buffer
func build_bcrypt_str(minor byte, rounds uint, payload ...interface{}) []byte {
	rs := bytes.NewBuffer(make([]byte, 0, 61))
	rs.WriteString("$2")
	if minor >= 'a' {
		rs.WriteByte(minor)
	}

	rs.WriteByte('$')
	if rounds < 10 {
		rs.WriteByte('0')
	}

	rs.WriteString(strconv.FormatUint(uint64(rounds), 10))
	rs.WriteByte('$')
	for _, p := range payload {
		if pb, ok := p.([]byte); ok {
			rs.WriteString(strings.TrimRight(enc.EncodeToString(pb), "="))
		} else if ps, ok := p.(string); ok {
			rs.WriteString(ps)
		}
	}
	return rs.Bytes()
}

// Salt generation
func Salt(rounds ...int) (string, error) {
	rb, err := SaltBytes(rounds...)
	return string(rb), err
}

func SaltBytes(rounds ...int) (salt []byte, err error) {
	r := DefaultRounds
	if len(rounds) > 0 {
		r = rounds[0]
		if r < MinRounds || r > MaxRounds {
			return nil, InvalidRounds
		}
	}

	rnd := make([]byte, SaltLen)
	read, err := rand.Read(rnd)
	if read != SaltLen || err != nil {
		return nil, err
	}

	return build_bcrypt_str('a', uint(r), rnd), nil
}

func consume(r *bytes.Buffer, b byte) bool {
	got, err := r.ReadByte()
	if err != nil {
		return false
	}
	if got != b {
		r.UnreadByte()
		return false
	}

	return true
}

func Hash(password string, salt ...string) (ps string, err error) {
	var s []byte
	var pb []byte

	if len(salt) == 0 {
		s, err = SaltBytes()
		if err != nil {
			return
		}
	} else if len(salt) > 0 {
		s = []byte(salt[0])
	}

	pb, err = HashBytes([]byte(password), s)
	return string(pb), err
}

func HashBytes(password []byte, salt ...[]byte) (hash []byte, err error) {
	var s []byte

	if len(salt) == 0 {
		s, err = SaltBytes()
		if err != nil {
			return
		}
	} else if len(salt) > 0 {
		s = salt[0]
	}

	// TODO: use a regex? I hear go has bad regex performance a simple FSM seems faster
	// 			"^\\$2([a-z]?)\\$([0-3][0-9])\\$([\\./A-Za-z0-9]{22}+)"

	// Ok, extract the required information
	minor := byte(0)
	sr := bytes.NewBuffer(s)

	if !consume(sr, '$') || !consume(sr, '2') {
		return nil, InvalidSalt
	}

	if !consume(sr, '$') {
		minor, _ = sr.ReadByte()
		if (minor != 'a' && minor != 'y') || !consume(sr, '$') {
			return nil, InvalidSalt
		}
	}

	rounds_bytes := make([]byte, 2)
	read, err := sr.Read(rounds_bytes)
	if err != nil || read != 2 {
		return nil, InvalidSalt
	}

	if !consume(sr, '$') {
		return nil, InvalidSalt
	}

	var rounds64 uint64
	rounds64, err = strconv.ParseUint(string(rounds_bytes), 10, 0)
	if err != nil {
		return nil, InvalidSalt
	}

	rounds := uint(rounds64)

	// TODO: can't we use base64.NewDecoder(enc, sr) ?
	salt_bytes := make([]byte, 22)
	read, err = sr.Read(salt_bytes)
	if err != nil || read != 22 {
		return nil, InvalidSalt
	}

	var saltb []byte
	// encoding/base64 expects 4 byte blocks padded, since bcrypt uses only 22 bytes we need to go up
	saltb, err = enc.DecodeString(string(salt_bytes) + "==")
	if err != nil {
		return nil, err
	}

	// cipher expects null terminated input (go initializes everything with zero values so this works)
	password_term := make([]byte, len(password)+1)
	copy(password_term, password)

	hashed := crypt_raw(password_term, saltb[:SaltLen], rounds)
	clear(password_term)
	return build_bcrypt_str(minor, rounds, string(salt_bytes), hashed[:len(bf_crypt_ciphertext)*4-1]), nil
}

func Match(password, hash string) bool {
	return MatchBytes([]byte(password), []byte(hash))
}

func MatchBytes(password []byte, hash []byte) bool {
	h, err := HashBytes(password, hash)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(h, hash) == 1
}

func clear(w []byte) {
	for k := range w {
		w[k] = 0
	}
}
