// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package picobuf

import (
	"math"

	"google.golang.org/protobuf/encoding/protowire"
)

// Bool decodes bool protobuf type.
//
//go:noinline
func (dec *Decoder) Bool(field FieldNumber, v *bool) {
	if field != dec.pendingField {
		return
	}
	if dec.pendingWire != protowire.VarintType {
		dec.fail(field, "expected wire type Varint")
		return
	}
	x, n := protowire.ConsumeVarint(dec.buffer)
	if n < 0 {
		dec.fail(field, "unable to parse Varint")
		return
	}
	*v = x == 1
	dec.nextField(n)
}

// RepeatedBool decodes repeated bool protobuf type.
//
//go:noinline
func (dec *Decoder) RepeatedBool(field FieldNumber, v *[]bool) {
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
				*v = append(*v, x == 1)
				packed = packed[xn:]
			}
			dec.nextField(n)
		case protowire.VarintType:
			x, n := protowire.ConsumeVarint(dec.buffer)
			if n < 0 {
				dec.fail(field, "unable to parse Varint")
				return
			}
			*v = append(*v, x == 1)
			dec.nextField(n)
		default:
			dec.fail(field, "expected wire type Varint")
			return
		}
	}
}

// Int32 decodes int32 protobuf type.
//
//go:noinline
func (dec *Decoder) Int32(field FieldNumber, v *int32) {
	if field != dec.pendingField {
		return
	}
	if dec.pendingWire != protowire.VarintType {
		dec.fail(field, "expected wire type Varint")
		return
	}
	x, n := protowire.ConsumeVarint(dec.buffer)
	if n < 0 {
		dec.fail(field, "unable to parse Varint")
		return
	}
	*v = int32(x)
	dec.nextField(n)
}

// RepeatedInt32 decodes repeated int32 protobuf type.
//
//go:noinline
func (dec *Decoder) RepeatedInt32(field FieldNumber, v *[]int32) {
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
				*v = append(*v, int32(x))
				packed = packed[xn:]
			}
			dec.nextField(n)
		case protowire.VarintType:
			x, n := protowire.ConsumeVarint(dec.buffer)
			if n < 0 {
				dec.fail(field, "unable to parse Varint")
				return
			}
			*v = append(*v, int32(x))
			dec.nextField(n)
		default:
			dec.fail(field, "expected wire type Varint")
			return
		}
	}
}

// Int64 decodes int64 protobuf type.
//
//go:noinline
func (dec *Decoder) Int64(field FieldNumber, v *int64) {
	if field != dec.pendingField {
		return
	}
	if dec.pendingWire != protowire.VarintType {
		dec.fail(field, "expected wire type Varint")
		return
	}
	x, n := protowire.ConsumeVarint(dec.buffer)
	if n < 0 {
		dec.fail(field, "unable to parse Varint")
		return
	}
	*v = int64(x)
	dec.nextField(n)
}

// RepeatedInt64 decodes repeated int64 protobuf type.
//
//go:noinline
func (dec *Decoder) RepeatedInt64(field FieldNumber, v *[]int64) {
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
				*v = append(*v, int64(x))
				packed = packed[xn:]
			}
			dec.nextField(n)
		case protowire.VarintType:
			x, n := protowire.ConsumeVarint(dec.buffer)
			if n < 0 {
				dec.fail(field, "unable to parse Varint")
				return
			}
			*v = append(*v, int64(x))
			dec.nextField(n)
		default:
			dec.fail(field, "expected wire type Varint")
			return
		}
	}
}

// Uint32 decodes uint32 protobuf type.
//
//go:noinline
func (dec *Decoder) Uint32(field FieldNumber, v *uint32) {
	if field != dec.pendingField {
		return
	}
	if dec.pendingWire != protowire.VarintType {
		dec.fail(field, "expected wire type Varint")
		return
	}
	x, n := protowire.ConsumeVarint(dec.buffer)
	if n < 0 {
		dec.fail(field, "unable to parse Varint")
		return
	}
	*v = uint32(x)
	dec.nextField(n)
}

// RepeatedUint32 decodes repeated uint32 protobuf type.
//
//go:noinline
func (dec *Decoder) RepeatedUint32(field FieldNumber, v *[]uint32) {
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
				*v = append(*v, uint32(x))
				packed = packed[xn:]
			}
			dec.nextField(n)
		case protowire.VarintType:
			x, n := protowire.ConsumeVarint(dec.buffer)
			if n < 0 {
				dec.fail(field, "unable to parse Varint")
				return
			}
			*v = append(*v, uint32(x))
			dec.nextField(n)
		default:
			dec.fail(field, "expected wire type Varint")
			return
		}
	}
}

// Uint64 decodes uint64 protobuf type.
//
//go:noinline
func (dec *Decoder) Uint64(field FieldNumber, v *uint64) {
	if field != dec.pendingField {
		return
	}
	if dec.pendingWire != protowire.VarintType {
		dec.fail(field, "expected wire type Varint")
		return
	}
	x, n := protowire.ConsumeVarint(dec.buffer)
	if n < 0 {
		dec.fail(field, "unable to parse Varint")
		return
	}
	*v = x
	dec.nextField(n)
}

// RepeatedUint64 decodes repeated uint64 protobuf type.
//
//go:noinline
func (dec *Decoder) RepeatedUint64(field FieldNumber, v *[]uint64) {
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
				*v = append(*v, x)
				packed = packed[xn:]
			}
			dec.nextField(n)
		case protowire.VarintType:
			x, n := protowire.ConsumeVarint(dec.buffer)
			if n < 0 {
				dec.fail(field, "unable to parse Varint")
				return
			}
			*v = append(*v, x)
			dec.nextField(n)
		default:
			dec.fail(field, "expected wire type Varint")
			return
		}
	}
}

// Sint32 decodes sint32 protobuf type.
//
//go:noinline
func (dec *Decoder) Sint32(field FieldNumber, v *int32) {
	if field != dec.pendingField {
		return
	}
	if dec.pendingWire != protowire.VarintType {
		dec.fail(field, "expected wire type Varint")
		return
	}
	x, n := protowire.ConsumeVarint(dec.buffer)
	if n < 0 {
		dec.fail(field, "unable to parse Varint")
		return
	}
	*v = decodeZigZag32(uint32(x))
	dec.nextField(n)
}

// RepeatedSint32 decodes repeated sint32 protobuf type.
//
//go:noinline
func (dec *Decoder) RepeatedSint32(field FieldNumber, v *[]int32) {
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
				*v = append(*v, decodeZigZag32(uint32(x)))
				packed = packed[xn:]
			}
			dec.nextField(n)
		case protowire.VarintType:
			x, n := protowire.ConsumeVarint(dec.buffer)
			if n < 0 {
				dec.fail(field, "unable to parse Varint")
				return
			}
			*v = append(*v, decodeZigZag32(uint32(x)))
			dec.nextField(n)
		default:
			dec.fail(field, "expected wire type Varint")
			return
		}
	}
}

// Sint64 decodes sint64 protobuf type.
//
//go:noinline
func (dec *Decoder) Sint64(field FieldNumber, v *int64) {
	if field != dec.pendingField {
		return
	}
	if dec.pendingWire != protowire.VarintType {
		dec.fail(field, "expected wire type Varint")
		return
	}
	x, n := protowire.ConsumeVarint(dec.buffer)
	if n < 0 {
		dec.fail(field, "unable to parse Varint")
		return
	}
	*v = protowire.DecodeZigZag(x)
	dec.nextField(n)
}

// RepeatedSint64 decodes repeated sint64 protobuf type.
//
//go:noinline
func (dec *Decoder) RepeatedSint64(field FieldNumber, v *[]int64) {
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
				*v = append(*v, protowire.DecodeZigZag(x))
				packed = packed[xn:]
			}
			dec.nextField(n)
		case protowire.VarintType:
			x, n := protowire.ConsumeVarint(dec.buffer)
			if n < 0 {
				dec.fail(field, "unable to parse Varint")
				return
			}
			*v = append(*v, protowire.DecodeZigZag(x))
			dec.nextField(n)
		default:
			dec.fail(field, "expected wire type Varint")
			return
		}
	}
}

// Fixed32 decodes fixed32 protobuf type.
//
//go:noinline
func (dec *Decoder) Fixed32(field FieldNumber, v *uint32) {
	if field != dec.pendingField {
		return
	}
	if dec.pendingWire != protowire.Fixed32Type {
		dec.fail(field, "expected wire type Fixed32")
		return
	}
	x, n := protowire.ConsumeFixed32(dec.buffer)
	if n < 0 {
		dec.fail(field, "unable to parse Fixed32")
		return
	}
	*v = x
	dec.nextField(n)
}

// RepeatedFixed32 decodes repeated fixed32 protobuf type.
//
//go:noinline
func (dec *Decoder) RepeatedFixed32(field FieldNumber, v *[]uint32) {
	for field == dec.pendingField {
		switch dec.pendingWire {
		case protowire.BytesType:
			packed, n := protowire.ConsumeBytes(dec.buffer)
			for len(packed) > 0 {
				x, xn := protowire.ConsumeFixed32(packed)
				if xn < 0 {
					dec.fail(field, "unable to parse Fixed32")
					return
				}
				*v = append(*v, x)
				packed = packed[xn:]
			}
			dec.nextField(n)
		case protowire.Fixed32Type:
			x, n := protowire.ConsumeFixed32(dec.buffer)
			if n < 0 {
				dec.fail(field, "unable to parse Fixed32")
				return
			}
			*v = append(*v, x)
			dec.nextField(n)
		default:
			dec.fail(field, "expected wire type Fixed32")
			return
		}
	}
}

// Fixed64 decodes fixed64 protobuf type.
//
//go:noinline
func (dec *Decoder) Fixed64(field FieldNumber, v *uint64) {
	if field != dec.pendingField {
		return
	}
	if dec.pendingWire != protowire.Fixed64Type {
		dec.fail(field, "expected wire type Fixed64")
		return
	}
	x, n := protowire.ConsumeFixed64(dec.buffer)
	if n < 0 {
		dec.fail(field, "unable to parse Fixed64")
		return
	}
	*v = x
	dec.nextField(n)
}

// RepeatedFixed64 decodes repeated fixed64 protobuf type.
//
//go:noinline
func (dec *Decoder) RepeatedFixed64(field FieldNumber, v *[]uint64) {
	for field == dec.pendingField {
		switch dec.pendingWire {
		case protowire.BytesType:
			packed, n := protowire.ConsumeBytes(dec.buffer)
			for len(packed) > 0 {
				x, xn := protowire.ConsumeFixed64(packed)
				if xn < 0 {
					dec.fail(field, "unable to parse Fixed64")
					return
				}
				*v = append(*v, x)
				packed = packed[xn:]
			}
			dec.nextField(n)
		case protowire.Fixed64Type:
			x, n := protowire.ConsumeFixed64(dec.buffer)
			if n < 0 {
				dec.fail(field, "unable to parse Fixed64")
				return
			}
			*v = append(*v, x)
			dec.nextField(n)
		default:
			dec.fail(field, "expected wire type Fixed64")
			return
		}
	}
}

// Sfixed32 decodes sfixed32 protobuf type.
//
//go:noinline
func (dec *Decoder) Sfixed32(field FieldNumber, v *int32) {
	if field != dec.pendingField {
		return
	}
	if dec.pendingWire != protowire.Fixed32Type {
		dec.fail(field, "expected wire type Fixed32")
		return
	}
	x, n := protowire.ConsumeFixed32(dec.buffer)
	if n < 0 {
		dec.fail(field, "unable to parse Fixed32")
		return
	}
	*v = decodeZigZag32(x)
	dec.nextField(n)
}

// RepeatedSfixed32 decodes repeated sfixed32 protobuf type.
//
//go:noinline
func (dec *Decoder) RepeatedSfixed32(field FieldNumber, v *[]int32) {
	for field == dec.pendingField {
		switch dec.pendingWire {
		case protowire.BytesType:
			packed, n := protowire.ConsumeBytes(dec.buffer)
			for len(packed) > 0 {
				x, xn := protowire.ConsumeFixed32(packed)
				if xn < 0 {
					dec.fail(field, "unable to parse Fixed32")
					return
				}
				*v = append(*v, decodeZigZag32(x))
				packed = packed[xn:]
			}
			dec.nextField(n)
		case protowire.Fixed32Type:
			x, n := protowire.ConsumeFixed32(dec.buffer)
			if n < 0 {
				dec.fail(field, "unable to parse Fixed32")
				return
			}
			*v = append(*v, decodeZigZag32(x))
			dec.nextField(n)
		default:
			dec.fail(field, "expected wire type Fixed32")
			return
		}
	}
}

// Sfixed64 decodes sfixed64 protobuf type.
//
//go:noinline
func (dec *Decoder) Sfixed64(field FieldNumber, v *int64) {
	if field != dec.pendingField {
		return
	}
	if dec.pendingWire != protowire.Fixed64Type {
		dec.fail(field, "expected wire type Fixed64")
		return
	}
	x, n := protowire.ConsumeFixed64(dec.buffer)
	if n < 0 {
		dec.fail(field, "unable to parse Fixed64")
		return
	}
	*v = protowire.DecodeZigZag(x)
	dec.nextField(n)
}

// RepeatedSfixed64 decodes repeated sfixed64 protobuf type.
//
//go:noinline
func (dec *Decoder) RepeatedSfixed64(field FieldNumber, v *[]int64) {
	for field == dec.pendingField {
		switch dec.pendingWire {
		case protowire.BytesType:
			packed, n := protowire.ConsumeBytes(dec.buffer)
			for len(packed) > 0 {
				x, xn := protowire.ConsumeFixed64(packed)
				if xn < 0 {
					dec.fail(field, "unable to parse Fixed64")
					return
				}
				*v = append(*v, protowire.DecodeZigZag(x))
				packed = packed[xn:]
			}
			dec.nextField(n)
		case protowire.Fixed64Type:
			x, n := protowire.ConsumeFixed64(dec.buffer)
			if n < 0 {
				dec.fail(field, "unable to parse Fixed64")
				return
			}
			*v = append(*v, protowire.DecodeZigZag(x))
			dec.nextField(n)
		default:
			dec.fail(field, "expected wire type Fixed64")
			return
		}
	}
}

// Float decodes float protobuf type.
//
//go:noinline
func (dec *Decoder) Float(field FieldNumber, v *float32) {
	if field != dec.pendingField {
		return
	}
	if dec.pendingWire != protowire.Fixed32Type {
		dec.fail(field, "expected wire type Fixed32")
		return
	}
	x, n := protowire.ConsumeFixed32(dec.buffer)
	if n < 0 {
		dec.fail(field, "unable to parse Fixed32")
		return
	}
	*v = math.Float32frombits(x)
	dec.nextField(n)
}

// RepeatedFloat decodes repeated float protobuf type.
//
//go:noinline
func (dec *Decoder) RepeatedFloat(field FieldNumber, v *[]float32) {
	for field == dec.pendingField {
		switch dec.pendingWire {
		case protowire.BytesType:
			packed, n := protowire.ConsumeBytes(dec.buffer)
			for len(packed) > 0 {
				x, xn := protowire.ConsumeFixed32(packed)
				if xn < 0 {
					dec.fail(field, "unable to parse Fixed32")
					return
				}
				*v = append(*v, math.Float32frombits(x))
				packed = packed[xn:]
			}
			dec.nextField(n)
		case protowire.Fixed32Type:
			x, n := protowire.ConsumeFixed32(dec.buffer)
			if n < 0 {
				dec.fail(field, "unable to parse Fixed32")
				return
			}
			*v = append(*v, math.Float32frombits(x))
			dec.nextField(n)
		default:
			dec.fail(field, "expected wire type Fixed32")
			return
		}
	}
}

// Double decodes double protobuf type.
//
//go:noinline
func (dec *Decoder) Double(field FieldNumber, v *float64) {
	if field != dec.pendingField {
		return
	}
	if dec.pendingWire != protowire.Fixed64Type {
		dec.fail(field, "expected wire type Fixed64")
		return
	}
	x, n := protowire.ConsumeFixed64(dec.buffer)
	if n < 0 {
		dec.fail(field, "unable to parse Fixed64")
		return
	}
	*v = math.Float64frombits(x)
	dec.nextField(n)
}

// RepeatedDouble decodes repeated double protobuf type.
//
//go:noinline
func (dec *Decoder) RepeatedDouble(field FieldNumber, v *[]float64) {
	for field == dec.pendingField {
		switch dec.pendingWire {
		case protowire.BytesType:
			packed, n := protowire.ConsumeBytes(dec.buffer)
			for len(packed) > 0 {
				x, xn := protowire.ConsumeFixed64(packed)
				if xn < 0 {
					dec.fail(field, "unable to parse Fixed64")
					return
				}
				*v = append(*v, math.Float64frombits(x))
				packed = packed[xn:]
			}
			dec.nextField(n)
		case protowire.Fixed64Type:
			x, n := protowire.ConsumeFixed64(dec.buffer)
			if n < 0 {
				dec.fail(field, "unable to parse Fixed64")
				return
			}
			*v = append(*v, math.Float64frombits(x))
			dec.nextField(n)
		default:
			dec.fail(field, "expected wire type Fixed64")
			return
		}
	}
}

// String decodes string protobuf type.
//
//go:noinline
func (dec *Decoder) String(field FieldNumber, v *string) {
	if field != dec.pendingField {
		return
	}
	if dec.pendingWire != protowire.BytesType {
		dec.fail(field, "expected wire type Bytes")
		return
	}
	x, n := protowire.ConsumeString(dec.buffer)
	if n < 0 {
		dec.fail(field, "unable to parse String")
		return
	}
	*v = x
	dec.nextField(n)
}

// RepeatedString decodes repeated string protobuf type.
//
//go:noinline
func (dec *Decoder) RepeatedString(field FieldNumber, v *[]string) {
	for field == dec.pendingField {
		switch dec.pendingWire {
		case protowire.BytesType:
			x, n := protowire.ConsumeString(dec.buffer)
			if n < 0 {
				dec.fail(field, "unable to parse String")
				return
			}
			*v = append(*v, x)
			dec.nextField(n)
		default:
			dec.fail(field, "expected wire type Bytes")
			return
		}
	}
}

// Bytes decodes bytes protobuf type.
//
//go:noinline
func (dec *Decoder) Bytes(field FieldNumber, v *[]byte) {
	if field != dec.pendingField {
		return
	}
	if dec.pendingWire != protowire.BytesType {
		dec.fail(field, "expected wire type Bytes")
		return
	}
	x, n := protowire.ConsumeBytes(dec.buffer)
	if n < 0 {
		dec.fail(field, "unable to parse Bytes")
		return
	}
	*v = x
	dec.nextField(n)
}

// RepeatedBytes decodes repeated bytes protobuf type.
//
//go:noinline
func (dec *Decoder) RepeatedBytes(field FieldNumber, v *[][]byte) {
	for field == dec.pendingField {
		switch dec.pendingWire {
		case protowire.BytesType:
			x, n := protowire.ConsumeBytes(dec.buffer)
			if n < 0 {
				dec.fail(field, "unable to parse Bytes")
				return
			}
			*v = append(*v, x)
			dec.nextField(n)
		default:
			dec.fail(field, "expected wire type Bytes")
			return
		}
	}
}
