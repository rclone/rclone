// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package extensions

import (
	"bytes"
	"encoding/binary"
	"io"
	"math/bits"
)

const (
	uint64Size           = 8
	firstCustomTypeID    = 65
	encFirstCustomTypeID = 130 // encoded 65
)

// hardcoded initial part of Revocation gob encoding, its constant until Revocation struct won't change,
// contains definition of Revocation struct with fields names and types.
// https://golang.org/pkg/encoding/gob/
var wireEncoding = []byte{
	64, 255, 129, 3, 1, 1, 10, 82, 101, 118, 111, 99, 97, 116, 105, 111, 110, 1, 255, 130, 0,
	1, 3, 1, 9, 84, 105, 109, 101, 115, 116, 97, 109, 112, 1, 4, 0, 1, 7, 75, 101, 121, 72,
	97, 115, 104, 1, 10, 0, 1, 9, 83, 105, 103, 110, 97, 116, 117, 114, 101, 1, 10, 0, 0, 0,
}

type revocationEncoder struct {
	value *bytes.Buffer
}

func (encoder *revocationEncoder) encode(revocation Revocation) ([]byte, error) {
	encoder.value = new(bytes.Buffer)

	encoder.encodeInt(firstCustomTypeID)
	delta := uint64(1)
	if revocation.Timestamp != 0 {
		encoder.encodeUint(delta)
		encoder.encodeInt(revocation.Timestamp)
	} else {
		delta++
	}

	if len(revocation.KeyHash) > 0 {
		encoder.encodeUint(delta)
		encoder.encodeUint(uint64(len(revocation.KeyHash)))
		encoder.writeBytes(revocation.KeyHash)
		delta = uint64(1)
	} else {
		delta++
	}

	if len(revocation.Signature) > 0 {
		encoder.encodeUint(delta)
		encoder.encodeUint(uint64(len(revocation.Signature)))
		encoder.writeBytes(revocation.Signature)
	}

	encoder.encodeUint(0)

	valueLength := encoder.value.Len()

	encoder.encodeUint(uint64(valueLength))

	value := encoder.value.Bytes()
	lengthData := value[valueLength:]
	valueData := value[:valueLength]
	return append(wireEncoding, append(lengthData, valueData...)...), nil
}

func (encoder *revocationEncoder) encodeInt(i int64) {
	var x uint64
	if i < 0 {
		x = uint64(^i<<1) | 1
	} else {
		x = uint64(i << 1)
	}
	encoder.encodeUint(x)
}

func (encoder *revocationEncoder) encodeUint(x uint64) {
	if x <= 0x7F {
		encoder.writeByte(uint8(x))
		return
	}

	var stateBuf [1 + uint64Size]byte
	binary.BigEndian.PutUint64(stateBuf[1:], x)
	bc := bits.LeadingZeros64(x) >> 3     // 8 - bytelen(x)
	stateBuf[bc] = uint8(bc - uint64Size) // and then we subtract 8 to get -bytelen(x)

	encoder.writeBytes(stateBuf[bc : uint64Size+1])
}

func (encoder *revocationEncoder) writeByte(x byte) {
	encoder.value.WriteByte(x)
}

func (encoder *revocationEncoder) writeBytes(x []byte) {
	encoder.value.Write(x)
}

type revocationDecoder struct {
	data *bytes.Buffer
}

func (decoder *revocationDecoder) decode(data []byte) (revocation Revocation, err error) {
	decoder.data = bytes.NewBuffer(data)

	wire := make([]byte, len(wireEncoding))
	_, err = io.ReadFull(decoder.data, wire)
	if err != nil {
		return revocation, ErrRevocation.Wrap(err)
	}
	if !bytes.Equal(wire, wireEncoding) {
		return revocation, ErrRevocation.New("invalid revocation encoding")
	}

	length, err := decoder.decodeUint()
	if err != nil {
		return revocation, ErrRevocation.Wrap(err)
	}

	if length != uint64(len(decoder.data.Bytes())) {
		return revocation, ErrRevocation.New("invalid revocation encoding")
	}

	typeID, err := decoder.decodeUint()
	if err != nil {
		return revocation, ErrRevocation.Wrap(err)
	}
	if typeID != encFirstCustomTypeID {
		return revocation, ErrRevocation.Wrap(ErrRevocation.New("invalid revocation encoding"))
	}

	index := uint64(0)
	for {
		field, err := decoder.decodeUint()
		if err != nil {
			return revocation, ErrRevocation.Wrap(err)
		}

		if field == 0 {
			break
		}

		switch field + index {
		case 1:
			revocation.Timestamp, err = decoder.decodeInt()
			if err != nil {
				return revocation, ErrRevocation.Wrap(err)
			}
		case 2:
			revocation.KeyHash, err = decoder.decodeHash()
			if err != nil {
				return revocation, ErrRevocation.Wrap(err)
			}
		case 3:
			revocation.Signature, err = decoder.decodeHash()
			if err != nil {
				return revocation, ErrRevocation.Wrap(err)
			}
		default:
			return revocation, ErrRevocation.New("invalid field")
		}

		index += field
	}

	return revocation, nil
}

func (decoder *revocationDecoder) decodeHash() ([]byte, error) {
	length, err := decoder.decodeUint()
	if err != nil {
		return nil, ErrRevocation.Wrap(err)
	}

	n := int(length)
	if uint64(n) != length || decoder.data.Len() < n || n < 0 {
		return nil, ErrRevocation.New("invalid hash length: %d", length)
	}

	buf := make([]byte, n)
	_, err = io.ReadFull(decoder.data, buf)
	if err != nil {
		return nil, ErrRevocation.Wrap(err)
	}
	return buf, nil
}

func (decoder *revocationDecoder) decodeUint() (x uint64, err error) {
	b, err := decoder.data.ReadByte()
	if err != nil {
		return 0, ErrRevocation.Wrap(err)
	}
	if b <= 0x7f {
		return uint64(b), nil
	}
	n := -int(int8(b))
	if n > uint64Size || n < 0 {
		return 0, ErrRevocation.New("encoded unsigned integer out of range")
	}
	buf := make([]byte, n)
	read, err := io.ReadFull(decoder.data, buf)
	if err != nil {
		return 0, ErrRevocation.Wrap(err)
	}
	if read < n {
		return 0, ErrRevocation.New("invalid uint data length %d: exceeds input size %d", n, len(buf))
	}
	// Don't need to check error; it's safe to loop regardless.
	// Could check that the high byte is zero but it's not worth it.
	for _, b := range buf {
		x = x<<8 | uint64(b)
	}
	return x, nil
}

func (decoder *revocationDecoder) decodeInt() (int64, error) {
	x, err := decoder.decodeUint()
	if err != nil {
		return 0, err
	}
	if x&1 != 0 {
		return ^int64(x >> 1), nil
	}
	return int64(x >> 1), nil
}
