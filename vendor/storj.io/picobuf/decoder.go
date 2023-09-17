// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package picobuf

import (
	"fmt"

	"google.golang.org/protobuf/encoding/protowire"
)

const (
	fieldDecodingErrored = FieldNumber(-1)
	fieldDecodingDone    = FieldNumber(-2)
)

// Decoder implements decoding of protobuf messages.
type Decoder struct {
	messageDecodeState
	stack []messageDecodeState
	init  bool
	err   error
}

type messageDecodeState struct {
	pendingField FieldNumber    //nolint: structcheck
	pendingWire  protowire.Type //nolint: structcheck

	buffer []byte
}

// NewDecoder returns a new Decoder.
func NewDecoder(data []byte) *Decoder {
	dec := new(Decoder)
	dec.buffer = data
	return dec
}

// PendingField returns the next field number in the stream.
func (dec *Decoder) PendingField() FieldNumber { return dec.pendingField }

// Err returns error that occurred during decoding.
func (dec *Decoder) Err() error {
	return dec.err
}

func (dec *Decoder) pushState(message []byte) {
	dec.stack = append(dec.stack, dec.messageDecodeState)
	dec.messageDecodeState = messageDecodeState{
		buffer: message,
	}
	dec.nextField(0)
}

func (dec *Decoder) popState() {
	if len(dec.stack) == 0 {
		dec.fail(0, "stack mangled")
		return
	}
	dec.messageDecodeState = dec.stack[len(dec.stack)-1]
	dec.stack = dec.stack[:len(dec.stack)-1]
}

// RepeatedMessage decodes a message.
func (dec *Decoder) RepeatedMessage(field FieldNumber, fn func(c *Decoder)) {
	for field == dec.pendingField {
		if dec.pendingWire != protowire.BytesType {
			dec.fail(field, "expected wire type Bytes")
			return
		}

		message, n := protowire.ConsumeBytes(dec.buffer)
		dec.pushState(message)
		fn(dec)
		dec.popState()

		dec.nextField(n)
	}
}

// RepeatedEnum decodes a repeated enumeration.
func (dec *Decoder) RepeatedEnum(field FieldNumber, add func(x int32)) {
	for field == dec.pendingField {
		switch dec.pendingWire {
		case protowire.BytesType:
			packed, n := protowire.ConsumeBytes(dec.buffer)
			for len(packed) > 0 {
				x, xn := protowire.ConsumeVarint(packed)
				if xn < 0 {
					dec.fail(field, "unable to parse Varint")
					return
				}
				add(int32(x))
				packed = packed[xn:]
			}
			dec.nextField(n)
		case protowire.VarintType:
			x, n := protowire.ConsumeVarint(dec.buffer)
			if n < 0 {
				dec.fail(field, "unable to parse Varint")
				return
			}
			add(int32(x))
			dec.nextField(n)
		default:
			dec.fail(field, "expected wire type Varint")
			return
		}
	}
}

// Message decodes a message.
func (dec *Decoder) Message(field FieldNumber, fn func(*Decoder)) {
	if field != dec.pendingField {
		return
	}
	if dec.pendingWire != protowire.BytesType {
		dec.fail(field, "expected wire type Bytes")
		return
	}

	message, n := protowire.ConsumeBytes(dec.buffer)
	dec.pushState(message)
	dec.Loop(fn)
	dec.popState()

	dec.nextField(n)
}

// PresentMessage decodes an always present message.
func (dec *Decoder) PresentMessage(field FieldNumber, fn func(*Decoder)) {
	if field != dec.pendingField {
		return
	}
	if dec.pendingWire != protowire.BytesType {
		dec.fail(field, "expected wire type Bytes")
		return
	}

	message, n := protowire.ConsumeBytes(dec.buffer)
	dec.pushState(message)
	dec.Loop(fn)
	dec.popState()

	dec.nextField(n)
}

// UnrecognizedFields decodes fields that are not in the provided set.
func (dec *Decoder) UnrecognizedFields(exclude uint64, out *[]byte) {
	for dec.pendingField >= 0 && (dec.pendingField >= 64 || exclude&(1<<uint64(dec.pendingField)) == 0) {
		n := protowire.ConsumeFieldValue(protowire.Number(dec.pendingField), dec.pendingWire, dec.buffer)
		*out = protowire.AppendTag(*out, protowire.Number(dec.pendingField), dec.pendingWire)
		*out = append(*out, dec.buffer[:n]...)
		dec.nextField(n)
	}
}

// Loop loops fields until all messages have been processed.
func (dec *Decoder) Loop(fn func(*Decoder)) {
	if !dec.init {
		dec.nextField(0)
		dec.init = true
	}

	for {
		startingLength := len(dec.buffer)
		fn(dec)
		if !dec.pendingField.IsValid() {
			break
		}
		if len(dec.buffer) == startingLength {
			// we didn't process any of the fields
			n := protowire.ConsumeFieldValue(protowire.Number(dec.pendingField), dec.pendingWire, dec.buffer)
			dec.nextField(n)
		}
	}
}

// Fail fails the decoding process.
func (dec *Decoder) Fail(field FieldNumber, msg string) {
	dec.fail(field, msg)
}

//go:noinline
func (dec *Decoder) fail(field FieldNumber, msg string) {
	// TODO: use static error types
	dec.pendingField = fieldDecodingErrored
	dec.err = fmt.Errorf("failed while parsing %v: %s", field, msg)
}

func (dec *Decoder) nextField(advance int) {
	if advance < 0 || advance > len(dec.buffer) {
		dec.fail(0, "advance outside buffer")
		return
	}
	dec.buffer = dec.buffer[advance:]
	if len(dec.buffer) == 0 {
		dec.pendingField = fieldDecodingDone
		return
	}

	field, wire, n := protowire.ConsumeTag(dec.buffer)
	if n < 0 {
		dec.fail(0, "failed to parse") // TODO: better error message
		return
	}
	dec.buffer = dec.buffer[n:]
	dec.pendingField, dec.pendingWire = FieldNumber(field), wire
}
