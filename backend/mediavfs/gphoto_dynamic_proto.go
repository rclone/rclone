package mediavfs

import (
	"fmt"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

// EncodeDynamicMessage encodes a nested map structure to protobuf using Google's official library
// This replaces our custom encoder with the official implementation
func EncodeDynamicMessage(data map[string]interface{}) ([]byte, error) {
	buf := make([]byte, 0, 1024)
	return appendMessage(buf, data)
}

// appendMessage appends a message to the buffer
func appendMessage(buf []byte, data map[string]interface{}) ([]byte, error) {
	for fieldNumStr, value := range data {
		var fieldNum int
		fmt.Sscanf(fieldNumStr, "%d", &fieldNum)

		var err error
		buf, err = appendField(buf, protowire.Number(fieldNum), value)
		if err != nil {
			return nil, err
		}
	}
	return buf, nil
}

// appendField appends a field to the buffer
func appendField(buf []byte, num protowire.Number, value interface{}) ([]byte, error) {
	switch v := value.(type) {
	case int:
		return appendVarint(buf, num, int64(v))
	case int32:
		return appendVarint(buf, num, int64(v))
	case int64:
		return appendVarint(buf, num, v)
	case uint32:
		return appendVarint(buf, num, int64(v))
	case uint64:
		return appendVarint(buf, num, int64(v))
	case float64:
		return appendVarint(buf, num, int64(v))
	case string:
		return appendString(buf, num, v)
	case []byte:
		return appendBytes(buf, num, v)
	case map[string]interface{}:
		return appendNestedMessage(buf, num, v)
	case []interface{}:
		// Repeated field - encode each element
		for _, item := range v {
			var err error
			buf, err = appendField(buf, num, item)
			if err != nil {
				return nil, err
			}
		}
		return buf, nil
	default:
		return buf, fmt.Errorf("unsupported type: %T", value)
	}
}

// appendVarint appends a varint field
func appendVarint(buf []byte, num protowire.Number, value int64) ([]byte, error) {
	buf = protowire.AppendTag(buf, num, protowire.VarintType)
	buf = protowire.AppendVarint(buf, uint64(value))
	return buf, nil
}

// appendString appends a string field
func appendString(buf []byte, num protowire.Number, value string) ([]byte, error) {
	buf = protowire.AppendTag(buf, num, protowire.BytesType)
	buf = protowire.AppendString(buf, value)
	return buf, nil
}

// appendBytes appends a bytes field
func appendBytes(buf []byte, num protowire.Number, value []byte) ([]byte, error) {
	buf = protowire.AppendTag(buf, num, protowire.BytesType)
	buf = protowire.AppendBytes(buf, value)
	return buf, nil
}

// appendNestedMessage appends a nested message field
func appendNestedMessage(buf []byte, num protowire.Number, value map[string]interface{}) ([]byte, error) {
	// Encode the nested message first
	nestedBuf, err := appendMessage(nil, value)
	if err != nil {
		return nil, err
	}

	// Then append it as a bytes field
	buf = protowire.AppendTag(buf, num, protowire.BytesType)
	buf = protowire.AppendBytes(buf, nestedBuf)
	return buf, nil
}

// DecodeDynamicMessage decodes a protobuf message to a map structure
func DecodeDynamicMessage(data []byte) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for len(data) > 0 {
		num, typ, n := protowire.ConsumeTag(data)
		if n < 0 {
			return nil, protowire.ParseError(n)
		}
		data = data[n:]

		fieldNum := fmt.Sprintf("%d", num)

		switch typ {
		case protowire.VarintType:
			val, n := protowire.ConsumeVarint(data)
			if n < 0 {
				return nil, protowire.ParseError(n)
			}
			data = data[n:]
			result[fieldNum] = val

		case protowire.BytesType:
			val, n := protowire.ConsumeBytes(data)
			if n < 0 {
				return nil, protowire.ParseError(n)
			}
			data = data[n:]

			// Try to decode as nested message
			if nested, err := DecodeDynamicMessage(val); err == nil && len(nested) > 0 {
				result[fieldNum] = nested
			} else {
				// If it fails, treat as string/bytes
				result[fieldNum] = string(val)
			}

		case protowire.Fixed32Type:
			val, n := protowire.ConsumeFixed32(data)
			if n < 0 {
				return nil, protowire.ParseError(n)
			}
			data = data[n:]
			result[fieldNum] = val

		case protowire.Fixed64Type:
			val, n := protowire.ConsumeFixed64(data)
			if n < 0 {
				return nil, protowire.ParseError(n)
			}
			data = data[n:]
			result[fieldNum] = val

		default:
			return nil, fmt.Errorf("unknown wire type: %v", typ)
		}
	}

	return result, nil
}

// Helper to check if message is well-formed
func ValidateMessage(data []byte) error {
	_, err := DecodeDynamicMessage(data)
	return err
}

// Size returns the encoded size of a message
func MessageSize(data map[string]interface{}) int {
	encoded, err := EncodeDynamicMessage(data)
	if err != nil {
		return 0
	}
	return len(encoded)
}

// Clone creates a deep copy of a message
func CloneMessage(data map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range data {
		if nested, ok := v.(map[string]interface{}); ok {
			result[k] = CloneMessage(nested)
		} else {
			result[k] = v
		}
	}
	return result
}

var _ proto.Message // Ensure we're compatible with proto.Message interface
