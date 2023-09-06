// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

import (
	"database/sql/driver"

	"github.com/zeebo/errs"
)

// ErrSerialNumber is used when something goes wrong with a serial number.
var ErrSerialNumber = errs.Class("serial number")

// SerialNumber is the unique identifier for pieces.
type SerialNumber [16]byte

// SerialNumberFromString decodes an base32 encoded.
func SerialNumberFromString(s string) (SerialNumber, error) {
	idBytes, err := base32Encoding.DecodeString(s)
	if err != nil {
		return SerialNumber{}, ErrSerialNumber.Wrap(err)
	}
	return SerialNumberFromBytes(idBytes)
}

// SerialNumberFromBytes converts a byte slice into a serial number.
func SerialNumberFromBytes(b []byte) (SerialNumber, error) {
	if len(b) != len(SerialNumber{}) {
		return SerialNumber{}, ErrSerialNumber.New("not enough bytes to make a serial number; have %d, need %d", len(b), len(NodeID{}))
	}

	var id SerialNumber
	copy(id[:], b)
	return id, nil
}

// IsZero returns whether serial number is unassigned.
func (id SerialNumber) IsZero() bool {
	return id == SerialNumber{}
}

// Less returns whether id is smaller than other in lexicographic order.
func (id SerialNumber) Less(other SerialNumber) bool {
	for k, v := range id {
		if v < other[k] {
			return true
		} else if v > other[k] {
			return false
		}
	}
	return false
}

// String representation of the serial number.
func (id SerialNumber) String() string { return base32Encoding.EncodeToString(id.Bytes()) }

// Bytes returns bytes of the serial number.
func (id SerialNumber) Bytes() []byte { return id[:] }

// Marshal serializes a serial number.
func (id SerialNumber) Marshal() ([]byte, error) {
	return id.Bytes(), nil
}

// MarshalTo serializes a serial number into the passed byte slice.
func (id *SerialNumber) MarshalTo(data []byte) (n int, err error) {
	n = copy(data, id.Bytes())
	return n, nil
}

// Unmarshal deserializes a serial number.
func (id *SerialNumber) Unmarshal(data []byte) error {
	var err error
	*id, err = SerialNumberFromBytes(data)
	return err
}

// Size returns the length of a serial number (implements gogo's custom type interface).
func (id *SerialNumber) Size() int {
	return len(id)
}

// MarshalText serializes a serial number to a base32 string.
func (id SerialNumber) MarshalText() ([]byte, error) {
	return []byte(id.String()), nil
}

// UnmarshalText deserializes a base32 string to a serial number.
func (id *SerialNumber) UnmarshalText(data []byte) error {
	var err error
	*id, err = SerialNumberFromString(string(data))
	if err != nil {
		return err
	}
	return nil
}

// Value set a SerialNumber to a database field.
func (id SerialNumber) Value() (driver.Value, error) {
	return id.Bytes(), nil
}

// Scan extracts a SerialNumber from a database field.
func (id *SerialNumber) Scan(src interface{}) (err error) {
	b, ok := src.([]byte)
	if !ok {
		return ErrSerialNumber.New("SerialNumber Scan expects []byte")
	}
	n, err := SerialNumberFromBytes(b)
	*id = n
	return err
}
