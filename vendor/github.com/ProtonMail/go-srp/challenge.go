package srp

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"time"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/curve25519"
)

// Implementation following the "context" package
var DeadlineExceeded error = deadlineExceededError{}

type deadlineExceededError struct{}

func (deadlineExceededError) Error() string {
	return "srp: deadline exceeded calculating proof-of-work challenge"
}
func (deadlineExceededError) Timeout() bool   { return true }
func (deadlineExceededError) Temporary() bool { return true }

const ecdlpPRFKeySize = 32

func unixMilli(currentTime time.Time) int64 {
	return currentTime.UnixNano() / 1e6
}

// ECDLPChallenge computes the base64 solution for a given ECDLP base64 challenge
// within deadlineUnixMilli milliseconds, if any was found. Deadlines are measured on the
// wall clock, not the monotonic clock, due to unreliability on mobile devices.
// deadlineUnixMilli = -1 means unlimited time.
func ECDLPChallenge(b64Challenge string, deadlineUnixMilli int64) (b64Solution string, err error) {
	challenge, err := base64.StdEncoding.DecodeString(b64Challenge)
	if err != nil {
		return "", err
	}

	if len(challenge) != 2*ecdlpPRFKeySize+sha256.Size {
		return "", errors.New("srp: invalid ECDLP challenge length")
	}

	var i uint64
	var point []byte
	buffer := make([]byte, 8)

	for i = 0; ; i++ {
		if deadlineUnixMilli >= 0 && unixMilli(time.Now()) > int64(deadlineUnixMilli) {
			return "", DeadlineExceeded
		}

		prePRF := hmac.New(sha256.New, challenge[:ecdlpPRFKeySize])
		binary.LittleEndian.PutUint64(buffer, i)
		_, _ = prePRF.Write(buffer)
		point, err = curve25519.X25519(prePRF.Sum(nil), curve25519.Basepoint)
		if err != nil {
			return "", err
		}
		postPRF := hmac.New(sha256.New, challenge[ecdlpPRFKeySize:2*ecdlpPRFKeySize])
		_, _ = postPRF.Write(point)

		if bytes.Equal(postPRF.Sum(nil), challenge[2*ecdlpPRFKeySize:]) {
			break
		}
	}
	solution := []byte{}
	solution = append(solution, buffer...)
	solution = append(solution, point...)

	return base64.StdEncoding.EncodeToString(solution), nil
}

const argon2PRFKeySize = 32

// Argon2PreimageChallenge computes the base64 solution for a given Argon2 base64
// challenge within deadlineUnixMilli milliseconds, if any was found. Deadlines are measured
// on the wall clock, not the monotonic clock, due to unreliability on mobile devices.
// deadlineUnixMilli = -1 means unlimited time.
func Argon2PreimageChallenge(b64Challenge string, deadlineUnixMilli int64) (b64Solution string, err error) {
	challenge, err := base64.StdEncoding.DecodeString(b64Challenge)
	if err != nil {
		return "", err
	}

	// Argon2 challenges consist of 3 PRF keys, the hash output, and 4 32-bit argon2 parameters
	if len(challenge) != 3*argon2PRFKeySize+sha256.Size+4*4 {
		return "", errors.New("srp: invalid Argon2 preimage challenge length")
	}
	prfKeys := challenge[:3*argon2PRFKeySize]
	goal := challenge[3*argon2PRFKeySize:][:sha256.Size]
	argon2Params := challenge[3*argon2PRFKeySize+sha256.Size:]

	threads := binary.LittleEndian.Uint32(argon2Params[0:])
	argon2OutputSize := binary.LittleEndian.Uint32(argon2Params[4:])
	memoryCost := binary.LittleEndian.Uint32(argon2Params[8:])
	timeCost := binary.LittleEndian.Uint32(argon2Params[12:])

	var i uint64
	var stage2 []byte
	buffer := make([]byte, 8)

	for i = 0; ; i++ {
		if deadlineUnixMilli >= 0 && unixMilli(time.Now()) > int64(deadlineUnixMilli) {
			return "", DeadlineExceeded
		}

		prePRF := hmac.New(sha256.New, prfKeys[:argon2PRFKeySize])
		binary.LittleEndian.PutUint64(buffer, i)
		_, _ = prePRF.Write(buffer)
		stage2 = argon2.IDKey(prePRF.Sum(nil), prfKeys[argon2PRFKeySize:2*argon2PRFKeySize], timeCost, memoryCost, uint8(threads), argon2OutputSize)
		postPRF := hmac.New(sha256.New, prfKeys[2*argon2PRFKeySize:])
		_, _ = postPRF.Write(stage2)

		if bytes.Equal(postPRF.Sum(nil), goal) {
			break
		}
	}
	solution := []byte{}
	solution = append(solution, buffer...)
	solution = append(solution, stage2...)

	return base64.StdEncoding.EncodeToString(solution), nil
}
