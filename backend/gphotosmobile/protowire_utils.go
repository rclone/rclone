package gphotosmobile

// protowire_utils provides raw protobuf wire encoding/decoding utilities.
// This is the Go equivalent of what blackboxprotobuf does in Python -
// working with protobuf at the wire format level without compiled .proto files.

import (
	"encoding/binary"
	"fmt"
	"math"
)

// WireType constants
const (
	WireVarint = 0
	Wire64Bit  = 1
	WireBytes  = 2
	Wire32Bit  = 5
)

// ProtoValue represents a decoded protobuf field value
type ProtoValue struct {
	// Varint value (WireVarint)
	Varint uint64
	// Fixed64 value (Wire64Bit)
	Fixed64 uint64
	// Fixed32 value (Wire32Bit)
	Fixed32 uint32
	// Bytes value (WireBytes) - could be string, bytes, or embedded message
	Bytes []byte
	// WireType
	WireType int
}

// ProtoMap represents a decoded protobuf message as field_number -> []ProtoValue
// (repeated fields have multiple values)
type ProtoMap map[uint64][]ProtoValue

// DecodeRaw decodes raw protobuf bytes into a ProtoMap
func DecodeRaw(data []byte) (ProtoMap, error) {
	result := make(ProtoMap)
	pos := 0

	for pos < len(data) {
		// Read field tag (field_number << 3 | wire_type)
		tag, n := decodeVarint(data[pos:])
		if n == 0 {
			return nil, fmt.Errorf("failed to decode tag at position %d", pos)
		}
		pos += n

		fieldNum := tag >> 3
		wireType := int(tag & 0x7)

		var val ProtoValue
		val.WireType = wireType

		switch wireType {
		case WireVarint:
			v, n := decodeVarint(data[pos:])
			if n == 0 {
				return nil, fmt.Errorf("failed to decode varint at position %d", pos)
			}
			pos += n
			val.Varint = v

		case Wire64Bit:
			if pos+8 > len(data) {
				return nil, fmt.Errorf("truncated 64-bit value at position %d", pos)
			}
			val.Fixed64 = binary.LittleEndian.Uint64(data[pos : pos+8])
			pos += 8

		case WireBytes:
			length, n := decodeVarint(data[pos:])
			if n == 0 {
				return nil, fmt.Errorf("failed to decode length at position %d", pos)
			}
			pos += n
			if pos+int(length) > len(data) {
				return nil, fmt.Errorf("truncated bytes at position %d (need %d, have %d)", pos, length, len(data)-pos)
			}
			val.Bytes = make([]byte, length)
			copy(val.Bytes, data[pos:pos+int(length)])
			pos += int(length)

		case Wire32Bit:
			if pos+4 > len(data) {
				return nil, fmt.Errorf("truncated 32-bit value at position %d", pos)
			}
			val.Fixed32 = binary.LittleEndian.Uint32(data[pos : pos+4])
			pos += 4

		default:
			return nil, fmt.Errorf("unknown wire type %d at position %d", wireType, pos)
		}

		result[fieldNum] = append(result[fieldNum], val)
	}

	return result, nil
}

// GetString gets a string value from a field
func (m ProtoMap) GetString(fieldNum uint64) string {
	vals, ok := m[fieldNum]
	if !ok || len(vals) == 0 {
		return ""
	}
	return string(vals[0].Bytes)
}

// GetBytes gets bytes value from a field
func (m ProtoMap) GetBytes(fieldNum uint64) []byte {
	vals, ok := m[fieldNum]
	if !ok || len(vals) == 0 {
		return nil
	}
	return vals[0].Bytes
}

// GetVarint gets a varint value from a field
func (m ProtoMap) GetVarint(fieldNum uint64) int64 {
	vals, ok := m[fieldNum]
	if !ok || len(vals) == 0 {
		return 0
	}
	return int64(vals[0].Varint)
}

// GetUint gets unsigned varint value
func (m ProtoMap) GetUint(fieldNum uint64) uint64 {
	vals, ok := m[fieldNum]
	if !ok || len(vals) == 0 {
		return 0
	}
	return vals[0].Varint
}

// GetFixed32 gets fixed32 value
func (m ProtoMap) GetFixed32(fieldNum uint64) uint32 {
	vals, ok := m[fieldNum]
	if !ok || len(vals) == 0 {
		return 0
	}
	return vals[0].Fixed32
}

// GetMessage decodes an embedded message from a field
func (m ProtoMap) GetMessage(fieldNum uint64) (ProtoMap, error) {
	vals, ok := m[fieldNum]
	if !ok || len(vals) == 0 {
		return nil, fmt.Errorf("field %d not found", fieldNum)
	}
	if vals[0].WireType != WireBytes {
		return nil, fmt.Errorf("field %d is not bytes type", fieldNum)
	}
	return DecodeRaw(vals[0].Bytes)
}

// GetRepeatedMessages decodes repeated embedded messages
func (m ProtoMap) GetRepeatedMessages(fieldNum uint64) ([]ProtoMap, error) {
	vals, ok := m[fieldNum]
	if !ok || len(vals) == 0 {
		return nil, nil
	}
	var results []ProtoMap
	for _, val := range vals {
		if val.WireType != WireBytes {
			continue
		}
		decoded, err := DecodeRaw(val.Bytes)
		if err != nil {
			continue // skip malformed entries
		}
		results = append(results, decoded)
	}
	return results, nil
}

// GetRepeatedStrings gets repeated string values
func (m ProtoMap) GetRepeatedStrings(fieldNum uint64) []string {
	vals, ok := m[fieldNum]
	if !ok {
		return nil
	}
	var result []string
	for _, v := range vals {
		if v.WireType == WireBytes {
			result = append(result, string(v.Bytes))
		}
	}
	return result
}

// Has checks if a field exists
func (m ProtoMap) Has(fieldNum uint64) bool {
	vals, ok := m[fieldNum]
	return ok && len(vals) > 0
}

// --- Encoding ---

// ProtoBuilder builds raw protobuf bytes
type ProtoBuilder struct {
	buf []byte
}

// NewProtoBuilder creates a new ProtoBuilder
func NewProtoBuilder() *ProtoBuilder {
	return &ProtoBuilder{}
}

// Bytes returns the built protobuf bytes
func (b *ProtoBuilder) Bytes() []byte {
	return b.buf
}

// AddVarint adds a varint field
func (b *ProtoBuilder) AddVarint(fieldNum uint64, value uint64) {
	b.buf = appendVarint(b.buf, (fieldNum<<3)|WireVarint)
	b.buf = appendVarint(b.buf, value)
}

// AddSignedVarint adds a signed varint field (using zigzag encoding)
func (b *ProtoBuilder) AddSignedVarint(fieldNum uint64, value int64) {
	b.AddVarint(fieldNum, uint64(value))
}

// AddFixed32 adds a fixed32 field
func (b *ProtoBuilder) AddFixed32(fieldNum uint64, value uint32) {
	b.buf = appendVarint(b.buf, (fieldNum<<3)|Wire32Bit)
	b.buf = binary.LittleEndian.AppendUint32(b.buf, value)
}

// AddFixed64 adds a fixed64 field
func (b *ProtoBuilder) AddFixed64(fieldNum uint64, value uint64) {
	b.buf = appendVarint(b.buf, (fieldNum<<3)|Wire64Bit)
	b.buf = binary.LittleEndian.AppendUint64(b.buf, value)
}

// AddBytes adds a bytes field
func (b *ProtoBuilder) AddBytes(fieldNum uint64, value []byte) {
	b.buf = appendVarint(b.buf, (fieldNum<<3)|WireBytes)
	b.buf = appendVarint(b.buf, uint64(len(value)))
	b.buf = append(b.buf, value...)
}

// AddString adds a string field
func (b *ProtoBuilder) AddString(fieldNum uint64, value string) {
	b.AddBytes(fieldNum, []byte(value))
}

// AddMessage adds an embedded message field
func (b *ProtoBuilder) AddMessage(fieldNum uint64, msg *ProtoBuilder) {
	b.AddBytes(fieldNum, msg.Bytes())
}

// AddEmptyMessage adds an empty embedded message field
func (b *ProtoBuilder) AddEmptyMessage(fieldNum uint64) {
	b.AddBytes(fieldNum, []byte{})
}

// AddRepeatedVarint adds repeated varint values
func (b *ProtoBuilder) AddRepeatedVarint(fieldNum uint64, values []uint64) {
	for _, v := range values {
		b.AddVarint(fieldNum, v)
	}
}

// --- Low-level helpers ---

func decodeVarint(data []byte) (uint64, int) {
	var value uint64
	var shift uint
	for i, b := range data {
		if i >= 10 {
			return 0, 0 // overflow
		}
		value |= uint64(b&0x7F) << shift
		if b < 0x80 {
			return value, i + 1
		}
		shift += 7
	}
	return 0, 0
}

func appendVarint(buf []byte, value uint64) []byte {
	for value >= 0x80 {
		buf = append(buf, byte(value)|0x80)
		value >>= 7
	}
	buf = append(buf, byte(value))
	return buf
}

// Float conversion utilities matching Python gpmc utils

// Int64ToFloat converts a 64-bit integer to IEEE 754 double
func Int64ToFloat(num int64) float64 {
	return math.Float64frombits(uint64(num))
}

// Int32ToFloat converts a 32-bit integer to IEEE 754 float
func Int32ToFloat(num int32) float32 {
	return math.Float32frombits(uint32(num))
}

// Fixed32ToFloat converts a scaled 32-bit signed integer to float (n / 10^7)
func Fixed32ToFloat(n uint64) float64 {
	signed := int64(n)
	if signed > 2147483647 { // 2^31 - 1
		signed -= 4294967296 // 2^32
	}
	return float64(signed) / 1e7
}
