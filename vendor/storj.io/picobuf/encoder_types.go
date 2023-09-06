// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package picobuf

import (
	"math"

	"google.golang.org/protobuf/encoding/protowire"
)

// Bool encodes non-default bool protobuf type.
//
//go:noinline
func (enc *Encoder) Bool(field FieldNumber, v *bool) {
	if !*v {
		return
	}
	enc.buffer = appendTag(enc.buffer, field, protowire.VarintType)
	enc.buffer = protowire.AppendVarint(enc.buffer, encodeBool64(*v))
}

// RepeatedBool encodes non-empty repeated bool protobuf type.
//
//go:noinline
func (enc *Encoder) RepeatedBool(field FieldNumber, v *[]bool) {
	if len(*v) == 0 {
		return
	}
	enc.buffer = appendTag(enc.buffer, field, protowire.BytesType)
	enc.buffer = protowire.AppendVarint(enc.buffer, uint64(len(*v)))
	for _, x := range *v {
		enc.buffer = append(enc.buffer, encodeBool8(x))
	}
}

// Int32 encodes non-default int32 protobuf type.
//
//go:noinline
func (enc *Encoder) Int32(field FieldNumber, v *int32) {
	if *v == 0 {
		return
	}
	enc.buffer = appendTag(enc.buffer, field, protowire.VarintType)
	enc.buffer = protowire.AppendVarint(enc.buffer, uint64(*v))
}

// RepeatedInt32 encodes non-empty repeated int32 protobuf type.
//
//go:noinline
func (enc *Encoder) RepeatedInt32(field FieldNumber, v *[]int32) {
	if len(*v) == 0 {
		return
	}
	enc.alwaysAnyBytes(field, func() {
		for _, x := range *v {
			enc.buffer = protowire.AppendVarint(enc.buffer, uint64(x))
		}
	})
}

// Int64 encodes non-default int64 protobuf type.
//
//go:noinline
func (enc *Encoder) Int64(field FieldNumber, v *int64) {
	if *v == 0 {
		return
	}
	enc.buffer = appendTag(enc.buffer, field, protowire.VarintType)
	enc.buffer = protowire.AppendVarint(enc.buffer, uint64(*v))
}

// RepeatedInt64 encodes non-empty repeated int64 protobuf type.
//
//go:noinline
func (enc *Encoder) RepeatedInt64(field FieldNumber, v *[]int64) {
	if len(*v) == 0 {
		return
	}
	enc.alwaysAnyBytes(field, func() {
		for _, x := range *v {
			enc.buffer = protowire.AppendVarint(enc.buffer, uint64(x))
		}
	})
}

// Uint32 encodes non-default uint32 protobuf type.
//
//go:noinline
func (enc *Encoder) Uint32(field FieldNumber, v *uint32) {
	if *v == 0 {
		return
	}
	enc.buffer = appendTag(enc.buffer, field, protowire.VarintType)
	enc.buffer = protowire.AppendVarint(enc.buffer, uint64(*v))
}

// RepeatedUint32 encodes non-empty repeated uint32 protobuf type.
//
//go:noinline
func (enc *Encoder) RepeatedUint32(field FieldNumber, v *[]uint32) {
	if len(*v) == 0 {
		return
	}
	enc.alwaysAnyBytes(field, func() {
		for _, x := range *v {
			enc.buffer = protowire.AppendVarint(enc.buffer, uint64(x))
		}
	})
}

// Uint64 encodes non-default uint64 protobuf type.
//
//go:noinline
func (enc *Encoder) Uint64(field FieldNumber, v *uint64) {
	if *v == 0 {
		return
	}
	enc.buffer = appendTag(enc.buffer, field, protowire.VarintType)
	enc.buffer = protowire.AppendVarint(enc.buffer, *v)
}

// RepeatedUint64 encodes non-empty repeated uint64 protobuf type.
//
//go:noinline
func (enc *Encoder) RepeatedUint64(field FieldNumber, v *[]uint64) {
	if len(*v) == 0 {
		return
	}
	enc.alwaysAnyBytes(field, func() {
		for _, x := range *v {
			enc.buffer = protowire.AppendVarint(enc.buffer, x)
		}
	})
}

// Sint32 encodes non-default sint32 protobuf type.
//
//go:noinline
func (enc *Encoder) Sint32(field FieldNumber, v *int32) {
	if *v == 0 {
		return
	}
	enc.buffer = appendTag(enc.buffer, field, protowire.VarintType)
	enc.buffer = protowire.AppendVarint(enc.buffer, uint64(encodeZigZag32(*v)))
}

// RepeatedSint32 encodes non-empty repeated sint32 protobuf type.
//
//go:noinline
func (enc *Encoder) RepeatedSint32(field FieldNumber, v *[]int32) {
	if len(*v) == 0 {
		return
	}
	enc.alwaysAnyBytes(field, func() {
		for _, x := range *v {
			enc.buffer = protowire.AppendVarint(enc.buffer, uint64(encodeZigZag32(x)))
		}
	})
}

// Sint64 encodes non-default sint64 protobuf type.
//
//go:noinline
func (enc *Encoder) Sint64(field FieldNumber, v *int64) {
	if *v == 0 {
		return
	}
	enc.buffer = appendTag(enc.buffer, field, protowire.VarintType)
	enc.buffer = protowire.AppendVarint(enc.buffer, protowire.EncodeZigZag(*v))
}

// RepeatedSint64 encodes non-empty repeated sint64 protobuf type.
//
//go:noinline
func (enc *Encoder) RepeatedSint64(field FieldNumber, v *[]int64) {
	if len(*v) == 0 {
		return
	}
	enc.alwaysAnyBytes(field, func() {
		for _, x := range *v {
			enc.buffer = protowire.AppendVarint(enc.buffer, protowire.EncodeZigZag(x))
		}
	})
}

// Fixed32 encodes non-default fixed32 protobuf type.
//
//go:noinline
func (enc *Encoder) Fixed32(field FieldNumber, v *uint32) {
	if *v == 0 {
		return
	}
	enc.buffer = appendTag(enc.buffer, field, protowire.Fixed32Type)
	enc.buffer = protowire.AppendFixed32(enc.buffer, *v)
}

// RepeatedFixed32 encodes non-empty repeated fixed32 protobuf type.
//
//go:noinline
func (enc *Encoder) RepeatedFixed32(field FieldNumber, v *[]uint32) {
	if len(*v) == 0 {
		return
	}
	enc.buffer = appendTag(enc.buffer, field, protowire.BytesType)
	enc.buffer = protowire.AppendVarint(enc.buffer, uint64(len(*v)*4))
	for _, x := range *v {
		enc.buffer = protowire.AppendFixed32(enc.buffer, x)
	}
}

// Fixed64 encodes non-default fixed64 protobuf type.
//
//go:noinline
func (enc *Encoder) Fixed64(field FieldNumber, v *uint64) {
	if *v == 0 {
		return
	}
	enc.buffer = appendTag(enc.buffer, field, protowire.Fixed64Type)
	enc.buffer = protowire.AppendFixed64(enc.buffer, *v)
}

// RepeatedFixed64 encodes non-empty repeated fixed64 protobuf type.
//
//go:noinline
func (enc *Encoder) RepeatedFixed64(field FieldNumber, v *[]uint64) {
	if len(*v) == 0 {
		return
	}
	enc.buffer = appendTag(enc.buffer, field, protowire.BytesType)
	enc.buffer = protowire.AppendVarint(enc.buffer, uint64(len(*v)*8))
	for _, x := range *v {
		enc.buffer = protowire.AppendFixed64(enc.buffer, x)
	}
}

// Sfixed32 encodes non-default sfixed32 protobuf type.
//
//go:noinline
func (enc *Encoder) Sfixed32(field FieldNumber, v *int32) {
	if *v == 0 {
		return
	}
	enc.buffer = appendTag(enc.buffer, field, protowire.Fixed32Type)
	enc.buffer = protowire.AppendFixed32(enc.buffer, encodeZigZag32(*v))
}

// RepeatedSfixed32 encodes non-empty repeated sfixed32 protobuf type.
//
//go:noinline
func (enc *Encoder) RepeatedSfixed32(field FieldNumber, v *[]int32) {
	if len(*v) == 0 {
		return
	}
	enc.buffer = appendTag(enc.buffer, field, protowire.BytesType)
	enc.buffer = protowire.AppendVarint(enc.buffer, uint64(len(*v)*4))
	for _, x := range *v {
		enc.buffer = protowire.AppendFixed32(enc.buffer, encodeZigZag32(x))
	}
}

// Sfixed64 encodes non-default sfixed64 protobuf type.
//
//go:noinline
func (enc *Encoder) Sfixed64(field FieldNumber, v *int64) {
	if *v == 0 {
		return
	}
	enc.buffer = appendTag(enc.buffer, field, protowire.Fixed64Type)
	enc.buffer = protowire.AppendFixed64(enc.buffer, protowire.EncodeZigZag(*v))
}

// RepeatedSfixed64 encodes non-empty repeated sfixed64 protobuf type.
//
//go:noinline
func (enc *Encoder) RepeatedSfixed64(field FieldNumber, v *[]int64) {
	if len(*v) == 0 {
		return
	}
	enc.buffer = appendTag(enc.buffer, field, protowire.BytesType)
	enc.buffer = protowire.AppendVarint(enc.buffer, uint64(len(*v)*8))
	for _, x := range *v {
		enc.buffer = protowire.AppendFixed64(enc.buffer, protowire.EncodeZigZag(x))
	}
}

// Float encodes non-default float protobuf type.
//
//go:noinline
func (enc *Encoder) Float(field FieldNumber, v *float32) {
	if *v == 0 {
		return
	}
	enc.buffer = appendTag(enc.buffer, field, protowire.Fixed32Type)
	enc.buffer = protowire.AppendFixed32(enc.buffer, math.Float32bits(*v))
}

// RepeatedFloat encodes non-empty repeated float protobuf type.
//
//go:noinline
func (enc *Encoder) RepeatedFloat(field FieldNumber, v *[]float32) {
	if len(*v) == 0 {
		return
	}
	enc.buffer = appendTag(enc.buffer, field, protowire.BytesType)
	enc.buffer = protowire.AppendVarint(enc.buffer, uint64(len(*v)*4))
	for _, x := range *v {
		enc.buffer = protowire.AppendFixed32(enc.buffer, math.Float32bits(x))
	}
}

// Double encodes non-default double protobuf type.
//
//go:noinline
func (enc *Encoder) Double(field FieldNumber, v *float64) {
	if *v == 0 {
		return
	}
	enc.buffer = appendTag(enc.buffer, field, protowire.Fixed64Type)
	enc.buffer = protowire.AppendFixed64(enc.buffer, math.Float64bits(*v))
}

// RepeatedDouble encodes non-empty repeated double protobuf type.
//
//go:noinline
func (enc *Encoder) RepeatedDouble(field FieldNumber, v *[]float64) {
	if len(*v) == 0 {
		return
	}
	enc.buffer = appendTag(enc.buffer, field, protowire.BytesType)
	enc.buffer = protowire.AppendVarint(enc.buffer, uint64(len(*v)*8))
	for _, x := range *v {
		enc.buffer = protowire.AppendFixed64(enc.buffer, math.Float64bits(x))
	}
}

// String encodes non-default string protobuf type.
//
//go:noinline
func (enc *Encoder) String(field FieldNumber, v *string) {
	if len(*v) == 0 {
		return
	}
	enc.buffer = appendTag(enc.buffer, field, protowire.BytesType)
	enc.buffer = protowire.AppendString(enc.buffer, *v)
}

// RepeatedString encodes non-empty repeated string protobuf type.
//
//go:noinline
func (enc *Encoder) RepeatedString(field FieldNumber, v *[]string) {
	if len(*v) == 0 {
		return
	}
	for _, x := range *v {
		enc.buffer = appendTag(enc.buffer, field, protowire.BytesType)
		enc.buffer = protowire.AppendString(enc.buffer, x)
	}
}

// Bytes encodes non-default bytes protobuf type.
//
//go:noinline
func (enc *Encoder) Bytes(field FieldNumber, v *[]byte) {
	if len(*v) == 0 {
		return
	}
	enc.buffer = appendTag(enc.buffer, field, protowire.BytesType)
	enc.buffer = protowire.AppendBytes(enc.buffer, *v)
}

// RepeatedBytes encodes non-empty repeated bytes protobuf type.
//
//go:noinline
func (enc *Encoder) RepeatedBytes(field FieldNumber, v *[][]byte) {
	if len(*v) == 0 {
		return
	}
	for _, x := range *v {
		enc.buffer = appendTag(enc.buffer, field, protowire.BytesType)
		enc.buffer = protowire.AppendBytes(enc.buffer, x)
	}
}
