// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package picobuf

import "google.golang.org/protobuf/encoding/protowire"

// Message is an interface that all generated messages implement.
type Message interface {
	Encode(*Encoder) bool
	Decode(*Decoder)
}

// CustomType defines methods that are used for custom encode or decode behaviors.
type CustomType interface {
	PicoEncode(*Encoder, FieldNumber)
	PicoDecode(*Decoder, FieldNumber)
}

// FieldNumber corresponds to a protobuf field number.
type FieldNumber protowire.Number

// IsValid returns whether the field number is in correct range.
func (field FieldNumber) IsValid() bool { return protowire.Number(field).IsValid() }

// Marshal encodes msg as bytes.
func Marshal(msg Message) ([]byte, error) {
	enc := &Encoder{}
	msg.Encode(enc)
	return enc.Buffer(), nil
}

// MarshalBuffer encodes msg as bytes with buffer.
func MarshalBuffer(msg Message, buffer []byte) ([]byte, error) {
	enc := &Encoder{buffer: buffer[:0]}
	msg.Encode(enc)
	return enc.Buffer(), nil
}

// Unmarshal decodes msg as bytes.
func Unmarshal(data []byte, msg Message) error {
	dec := &Decoder{}
	dec.buffer = data
	dec.Loop(msg.Decode)
	return dec.err
}
