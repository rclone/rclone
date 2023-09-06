// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

import (
	"github.com/zeebo/errs"
)

// ErrSegmentID is used when something goes wrong with a segment ID.
var ErrSegmentID = errs.Class("segment ID")

// SegmentID is the unique identifier for segment related to object.
type SegmentID []byte

// SegmentIDFromString decodes an base32 encoded.
func SegmentIDFromString(s string) (SegmentID, error) {
	idBytes, err := base32Encoding.DecodeString(s)
	if err != nil {
		return SegmentID{}, ErrSegmentID.Wrap(err)
	}
	return SegmentIDFromBytes(idBytes)
}

// SegmentIDFromBytes converts a byte slice into a segment ID.
func SegmentIDFromBytes(b []byte) (SegmentID, error) {
	// return error will be used in future implementation
	id := make([]byte, len(b))
	copy(id, b)
	return id, nil
}

// IsZero returns whether segment ID is unassigned.
func (id SegmentID) IsZero() bool {
	return len(id) == 0
}

// String representation of the segment ID.
func (id SegmentID) String() string { return base32Encoding.EncodeToString(id.Bytes()) }

// Bytes returns bytes of the segment ID.
func (id SegmentID) Bytes() []byte { return id[:] }

// Marshal serializes a segment ID (implements gogo's custom type interface).
func (id SegmentID) Marshal() ([]byte, error) {
	return id.Bytes(), nil
}

// MarshalTo serializes a segment ID into the passed byte slice (implements gogo's custom type interface).
func (id *SegmentID) MarshalTo(data []byte) (n int, err error) {
	return copy(data, id.Bytes()), nil
}

// Unmarshal deserializes a segment ID (implements gogo's custom type interface).
func (id *SegmentID) Unmarshal(data []byte) error {
	var err error
	*id, err = SegmentIDFromBytes(data)
	return err
}

// Size returns the length of a segment ID (implements gogo's custom type interface).
func (id SegmentID) Size() int {
	return len(id)
}

// MarshalText serializes a segment ID to a base32 string.
func (id SegmentID) MarshalText() ([]byte, error) {
	return []byte(id.String()), nil
}

// UnmarshalText deserializes a base32 string to a segment ID.
func (id *SegmentID) UnmarshalText(data []byte) error {
	var err error
	*id, err = SegmentIDFromString(string(data))
	return err
}
