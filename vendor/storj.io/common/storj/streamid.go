// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

import (
	"database/sql/driver"
	"encoding/base32"

	"github.com/zeebo/errs"
)

// ErrStreamID is used when something goes wrong with a stream ID.
var ErrStreamID = errs.Class("stream ID error")

// streamIDEncoding is base32 without padding.
var streamIDEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// StreamID is the unique identifier for stream related to object.
type StreamID []byte

// StreamIDFromString decodes an base32 encoded.
func StreamIDFromString(s string) (StreamID, error) {
	idBytes, err := streamIDEncoding.DecodeString(s)
	if err != nil {
		return StreamID{}, ErrStreamID.Wrap(err)
	}
	return StreamIDFromBytes(idBytes)
}

// StreamIDFromBytes converts a byte slice into a stream ID.
func StreamIDFromBytes(b []byte) (StreamID, error) {
	id := make([]byte, len(b))
	copy(id, b)
	return id, nil
}

// IsZero returns whether stream ID is unassigned.
func (id StreamID) IsZero() bool {
	return len(id) == 0
}

// String representation of the stream ID.
func (id StreamID) String() string { return streamIDEncoding.EncodeToString(id.Bytes()) }

// Bytes returns bytes of the stream ID.
func (id StreamID) Bytes() []byte { return id[:] }

// Marshal serializes a stream ID.
func (id StreamID) Marshal() ([]byte, error) {
	return id.Bytes(), nil
}

// MarshalTo serializes a stream ID into the passed byte slice.
func (id *StreamID) MarshalTo(data []byte) (n int, err error) {
	n = copy(data, id.Bytes())
	return n, nil
}

// Unmarshal deserializes a stream ID.
func (id *StreamID) Unmarshal(data []byte) error {
	var err error
	*id, err = StreamIDFromBytes(data)
	return err
}

// Size returns the length of a stream ID (implements gogo's custom type interface).
func (id StreamID) Size() int {
	return len(id)
}

// MarshalJSON serializes a stream ID to a json string as bytes.
func (id StreamID) MarshalJSON() ([]byte, error) {
	return []byte(`"` + id.String() + `"`), nil
}

// UnmarshalJSON deserializes a json string (as bytes) to a stream ID.
func (id *StreamID) UnmarshalJSON(data []byte) error {
	var err error
	*id, err = StreamIDFromString(string(data))
	if err != nil {
		return err
	}
	return nil
}

// Value set a stream ID to a database field.
func (id StreamID) Value() (driver.Value, error) {
	return id.Bytes(), nil
}

// Scan extracts a stream ID from a database field.
func (id *StreamID) Scan(src interface{}) (err error) {
	b, ok := src.([]byte)
	if !ok {
		return ErrStreamID.New("Stream ID Scan expects []byte")
	}
	n, err := StreamIDFromBytes(b)
	*id = n
	return err
}
