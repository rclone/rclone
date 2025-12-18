package mediavfs

import (
	"encoding/binary"
	"fmt"
	"io"
)

// ProtoEncoder provides dynamic protobuf encoding similar to Python's blackboxprotobuf
type ProtoEncoder struct {
	buf []byte
}

// NewProtoEncoder creates a new protobuf encoder
func NewProtoEncoder() *ProtoEncoder {
	return &ProtoEncoder{buf: make([]byte, 0, 256)}
}

// Bytes returns the encoded protobuf bytes
func (e *ProtoEncoder) Bytes() []byte {
	return e.buf
}

// encodeVarint encodes a varint value
func encodeVarint(value uint64) []byte {
	buf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(buf, value)
	return buf[:n]
}

// encodeZigZag encodes a signed integer using zigzag encoding
func encodeZigZag(value int64) uint64 {
	return uint64((value << 1) ^ (value >> 63))
}

// encodeTag encodes a field tag and wire type
func encodeTag(fieldNum int, wireType int) []byte {
	return encodeVarint(uint64((fieldNum << 3) | wireType))
}

// EncodeInt32 encodes an int32 field
func (e *ProtoEncoder) EncodeInt32(fieldNum int, value int32) {
	e.buf = append(e.buf, encodeTag(fieldNum, 0)...) // wire type 0 = varint
	e.buf = append(e.buf, encodeVarint(uint64(value))...)
}

// EncodeInt64 encodes an int64 field
func (e *ProtoEncoder) EncodeInt64(fieldNum int, value int64) {
	e.buf = append(e.buf, encodeTag(fieldNum, 0)...)
	e.buf = append(e.buf, encodeVarint(uint64(value))...)
}

// EncodeUInt64 encodes a uint64 field
func (e *ProtoEncoder) EncodeUInt64(fieldNum int, value uint64) {
	e.buf = append(e.buf, encodeTag(fieldNum, 0)...)
	e.buf = append(e.buf, encodeVarint(value)...)
}

// EncodeString encodes a string field
func (e *ProtoEncoder) EncodeString(fieldNum int, value string) {
	e.buf = append(e.buf, encodeTag(fieldNum, 2)...) // wire type 2 = length-delimited
	e.buf = append(e.buf, encodeVarint(uint64(len(value)))...)
	e.buf = append(e.buf, []byte(value)...)
}

// EncodeBytes encodes a bytes field
func (e *ProtoEncoder) EncodeBytes(fieldNum int, value []byte) {
	e.buf = append(e.buf, encodeTag(fieldNum, 2)...)
	e.buf = append(e.buf, encodeVarint(uint64(len(value)))...)
	e.buf = append(e.buf, value...)
}

// EncodeMessage encodes a nested message field
func (e *ProtoEncoder) EncodeMessage(fieldNum int, msg []byte) {
	e.buf = append(e.buf, encodeTag(fieldNum, 2)...)
	e.buf = append(e.buf, encodeVarint(uint64(len(msg)))...)
	e.buf = append(e.buf, msg...)
}

// ProtoDecoder provides dynamic protobuf decoding
type ProtoDecoder struct {
	buf []byte
	pos int
}

// NewProtoDecoder creates a new protobuf decoder
func NewProtoDecoder(data []byte) *ProtoDecoder {
	return &ProtoDecoder{buf: data, pos: 0}
}

// decodeVarint decodes a varint from the buffer
func (d *ProtoDecoder) decodeVarint() (uint64, error) {
	value, n := binary.Uvarint(d.buf[d.pos:])
	if n <= 0 {
		return 0, fmt.Errorf("failed to decode varint")
	}
	d.pos += n
	return value, nil
}

// DecodeField decodes the next field and returns (fieldNum, wireType, value)
func (d *ProtoDecoder) DecodeField() (int, int, interface{}, error) {
	if d.pos >= len(d.buf) {
		return 0, 0, nil, io.EOF
	}

	// Decode tag
	tag, err := d.decodeVarint()
	if err != nil {
		return 0, 0, nil, err
	}

	fieldNum := int(tag >> 3)
	wireType := int(tag & 0x7)

	switch wireType {
	case 0: // Varint
		value, err := d.decodeVarint()
		if err != nil {
			return 0, 0, nil, err
		}
		return fieldNum, wireType, value, nil

	case 1: // 64-bit
		if d.pos+8 > len(d.buf) {
			return 0, 0, nil, fmt.Errorf("insufficient bytes for 64-bit value")
		}
		value := binary.LittleEndian.Uint64(d.buf[d.pos : d.pos+8])
		d.pos += 8
		return fieldNum, wireType, value, nil

	case 2: // Length-delimited
		length, err := d.decodeVarint()
		if err != nil {
			return 0, 0, nil, err
		}
		if d.pos+int(length) > len(d.buf) {
			return 0, 0, nil, fmt.Errorf("insufficient bytes for length-delimited value")
		}
		value := d.buf[d.pos : d.pos+int(length)]
		d.pos += int(length)
		return fieldNum, wireType, value, nil

	case 5: // 32-bit
		if d.pos+4 > len(d.buf) {
			return 0, 0, nil, fmt.Errorf("insufficient bytes for 32-bit value")
		}
		value := binary.LittleEndian.Uint32(d.buf[d.pos : d.pos+4])
		d.pos += 4
		return fieldNum, wireType, value, nil

	default:
		return 0, 0, nil, fmt.Errorf("unknown wire type: %d", wireType)
	}
}

// DecodeToMap decodes protobuf bytes into a map structure similar to Python's approach
func DecodeToMap(data []byte) (map[string]interface{}, error) {
	decoder := NewProtoDecoder(data)
	result := make(map[string]interface{})

	for {
		fieldNum, wireType, value, err := decoder.DecodeField()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		key := fmt.Sprintf("%d", fieldNum)

		// If it's a length-delimited field, try to decode as nested message
		if wireType == 2 {
			if bytes, ok := value.([]byte); ok {
				// Try to decode as nested message
				nested, err := DecodeToMap(bytes)
				if err == nil && len(nested) > 0 {
					result[key] = nested
				} else {
					// If it fails, treat as bytes/string
					result[key] = string(bytes)
				}
			}
		} else {
			result[key] = value
		}
	}

	return result, nil
}

// BuildNestedMessage builds a nested protobuf message from a map structure
func BuildNestedMessage(data map[string]interface{}) []byte {
	encoder := NewProtoEncoder()

	// Sort keys to ensure consistent encoding
	for fieldNumStr, value := range data {
		var fieldNum int
		fmt.Sscanf(fieldNumStr, "%d", &fieldNum)

		switch v := value.(type) {
		case int:
			encoder.EncodeInt64(fieldNum, int64(v))
		case int32:
			encoder.EncodeInt32(fieldNum, v)
		case int64:
			encoder.EncodeInt64(fieldNum, v)
		case uint64:
			encoder.EncodeUInt64(fieldNum, v)
		case string:
			encoder.EncodeString(fieldNum, v)
		case []byte:
			encoder.EncodeBytes(fieldNum, v)
		case map[string]interface{}:
			// Nested message
			nested := BuildNestedMessage(v)
			encoder.EncodeMessage(fieldNum, nested)
		}
	}

	return encoder.Bytes()
}
