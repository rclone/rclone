// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

// Package uuid implements UUID v4 based on RFC4122.
package uuid

import (
	"crypto/rand"
	"encoding/hex"
	"io"

	"github.com/zeebo/errs"
)

// Error is the default error class for uuid.
var Error = errs.Class("uuid")

// UUID is big-endian encoded UUID.
//
// UUID can be of any version or variant.
type UUID [16]byte

// New returns a random UUID (version 4 variant 2).
func New() (UUID, error) {
	return newRandomFromReader(rand.Reader)
}

// newRandomFromReader returns a random UUID  (version 4 variant 2)
// using a custom reader.
func newRandomFromReader(r io.Reader) (UUID, error) {
	var uuid UUID
	_, err := io.ReadFull(r, uuid[:])
	if err != nil {
		return uuid, Error.Wrap(err)
	}

	// version 4, variant 2
	uuid[6] = (uuid[6] & 0x0f) | 0x40
	uuid[8] = (uuid[8] & 0x3f) | 0x80
	return uuid, nil
}

// IsZero returns true when all bytes in uuid are 0.
func (uuid UUID) IsZero() bool { return uuid == UUID{} }

// String returns uuid in "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" format.
func (uuid UUID) String() string {
	s := [36]byte{8: '-', 13: '-', 18: '-', 23: '-'}
	hex.Encode(s[0:8], uuid[0:4])
	hex.Encode(s[9:13], uuid[4:6])
	hex.Encode(s[14:18], uuid[6:8])
	hex.Encode(s[19:23], uuid[8:10])
	hex.Encode(s[24:36], uuid[10:16])
	return string(s[:])
}

// FromBytes converts big-endian raw-bytes to an UUID.
//
// FromBytes allows for any version or variant of an UUID.
func FromBytes(bytes []byte) (UUID, error) {
	var uuid UUID
	if len(uuid) != len(bytes) {
		return uuid, Error.New("bytes have wrong length %d expected %d", len(bytes), len(uuid))
	}
	copy(uuid[:], bytes)
	return uuid, nil
}

// FromString parses "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" string form.
//
// FromString allows for any version or variant of an UUID.
func FromString(s string) (UUID, error) {
	var uuid UUID
	if len(s) != 36 {
		return uuid, Error.New("invalid string length %d expected %d", len(s), 36)
	}
	if s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return uuid, Error.New("invalid string")
	}

	var err error
	_, err = hex.Decode(uuid[0:4], []byte(s)[0:8])
	if err != nil {
		return uuid, Error.New("invalid string")
	}
	_, err = hex.Decode(uuid[4:6], []byte(s)[9:13])
	if err != nil {
		return uuid, Error.New("invalid string")
	}
	_, err = hex.Decode(uuid[6:8], []byte(s)[14:18])
	if err != nil {
		return uuid, Error.New("invalid string")
	}
	_, err = hex.Decode(uuid[8:10], []byte(s)[19:23])
	if err != nil {
		return uuid, Error.New("invalid string")
	}
	_, err = hex.Decode(uuid[10:16], []byte(s)[24:36])
	if err != nil {
		return uuid, Error.New("invalid string")
	}

	return uuid, nil
}

// MarshalText marshals UUID in `xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx` form.
func (uuid UUID) MarshalText() ([]byte, error) {
	return []byte(uuid.String()), nil
}

// UnmarshalText unmarshals UUID from `xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`.
func (uuid *UUID) UnmarshalText(b []byte) error {
	x, err := FromString(string(b))
	if err != nil {
		return Error.Wrap(err)
	}
	*uuid = x
	return nil
}

// MarshalJSON marshals UUID in `"xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"` form.
func (uuid UUID) MarshalJSON() ([]byte, error) {
	return []byte(`"` + uuid.String() + `"`), nil
}

// UnmarshalJSON unmarshals UUID from `"xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"`.
func (uuid *UUID) UnmarshalJSON(b []byte) error {
	if len(b) != 36+2 {
		return Error.New("bytes have wrong length %d expected %d", len(b), 36+2)
	}
	if b[0] != '"' && b[len(b)-1] != '"' {
		return Error.New("expected quotes around string")
	}

	x, err := FromString(string(b[1 : len(b)-1]))
	if err != nil {
		return Error.Wrap(err)
	}
	*uuid = x
	return nil
}
