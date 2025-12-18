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
	originalLen := len(data)
	bytesProcessed := 0

	// Track field occurrences for debugging repeated fields
	fieldCounts := make(map[string]int)

	// Log first 40 bytes for debugging
	hexLen := 40
	if hexLen > len(data) {
		hexLen = len(data)
	}
	if hexLen > 0 {
		fmt.Printf("DecodeDynamicMessage: first %d bytes: %x\n", hexLen, data[:hexLen])
	}

	for len(data) > 0 {
		num, typ, n := protowire.ConsumeTag(data)
		if n < 0 {
			// Log position and surrounding bytes for debugging
			pos := originalLen - len(data)
			hexLen := 40
			if hexLen > len(data) {
				hexLen = len(data)
			}
			hexDump := fmt.Sprintf("%x", data[0:hexLen])
			return nil, fmt.Errorf("invalid tag at byte %d/%d, hex context: %s, error: %v", pos, originalLen, hexDump, protowire.ParseError(n))
		}
		data = data[n:]
		bytesProcessed += n

		fieldNum := fmt.Sprintf("%d", num)

		// Track field occurrences
		fieldCounts[fieldNum]++

		switch typ {
		case protowire.VarintType:
			val, n := protowire.ConsumeVarint(data)
			if n < 0 {
				return nil, protowire.ParseError(n)
			}
			data = data[n:]
			// Handle repeated fields
			if existing, exists := result[fieldNum]; exists {
				// Convert to array if not already
				if arr, isArray := existing.([]interface{}); isArray {
					result[fieldNum] = append(arr, val)
				} else {
					result[fieldNum] = []interface{}{existing, val}
				}
			} else {
				result[fieldNum] = val
			}

		case protowire.BytesType:
			val, n := protowire.ConsumeBytes(data)
			if n < 0 {
				return nil, protowire.ParseError(n)
			}
			data = data[n:]

			// Decode the value (nested message or string)
			var decodedVal interface{}
			if len(val) > 0 && len(val) < 10000000 { // sanity check size
				if nested, err := DecodeDynamicMessage(val); err == nil && len(nested) > 0 {
					decodedVal = nested
				} else {
					// If decoding failed, treat as string
					decodedVal = string(val)
				}
			} else if len(val) == 0 {
				decodedVal = ""
			} else {
				// Too large, keep as bytes
				decodedVal = val
			}

			// Handle repeated fields
			if existing, exists := result[fieldNum]; exists {
				// Convert to array if not already
				if arr, isArray := existing.([]interface{}); isArray {
					result[fieldNum] = append(arr, decodedVal)
				} else {
					result[fieldNum] = []interface{}{existing, decodedVal}
				}
			} else {
				result[fieldNum] = decodedVal
			}

		case protowire.Fixed32Type:
			val, n := protowire.ConsumeFixed32(data)
			if n < 0 {
				return nil, protowire.ParseError(n)
			}
			data = data[n:]
			// Handle repeated fields
			if existing, exists := result[fieldNum]; exists {
				if arr, isArray := existing.([]interface{}); isArray {
					result[fieldNum] = append(arr, val)
				} else {
					result[fieldNum] = []interface{}{existing, val}
				}
			} else {
				result[fieldNum] = val
			}

		case protowire.Fixed64Type:
			val, n := protowire.ConsumeFixed64(data)
			if n < 0 {
				return nil, protowire.ParseError(n)
			}
			data = data[n:]
			// Handle repeated fields
			if existing, exists := result[fieldNum]; exists {
				if arr, isArray := existing.([]interface{}); isArray {
					result[fieldNum] = append(arr, val)
				} else {
					result[fieldNum] = []interface{}{existing, val}
				}
			} else {
				result[fieldNum] = val
			}

		case protowire.StartGroupType:
			// Deprecated group type - skip until EndGroupType
			depth := 1
			for len(data) > 0 && depth > 0 {
				num2, typ2, n2 := protowire.ConsumeTag(data)
				if n2 < 0 {
					return nil, fmt.Errorf("invalid group tag at field %d", num)
				}
				data = data[n2:]

				if typ2 == protowire.StartGroupType {
					depth++
				} else if typ2 == protowire.EndGroupType && num2 == num {
					depth--
				} else {
					// Skip the field value
					n3 := protowire.ConsumeFieldValue(num2, typ2, data)
					if n3 < 0 {
						return nil, fmt.Errorf("invalid field value in group at field %d", num)
					}
					data = data[n3:]
				}
			}
			// Don't store group data, just skip it

		case protowire.EndGroupType:
			// End of group - should not appear at top level
			return nil, fmt.Errorf("unexpected end group at field %d", num)

		default:
			// Unknown wire type - try to skip it gracefully
			// This handles any malformed or unknown data
			return nil, fmt.Errorf("unknown wire type %d at field %d (might be corrupted data)", typ, num)
		}
	}

	// Debug: Report any repeated fields at the TOP LEVEL ONLY (to avoid spam from nested messages)
	if originalLen > 1000000 { // Only for large messages (>1MB) which is likely the top-level response
		repeatedFields := make([]string, 0)
		for field, count := range fieldCounts {
			if count > 1 {
				repeatedFields = append(repeatedFields, fmt.Sprintf("%s(x%d)", field, count))
			}
		}
		if len(repeatedFields) > 0 {
			fmt.Printf("DEBUG DecodeDynamicMessage: Repeated fields in %d-byte message: %v\n", originalLen, repeatedFields)
		} else {
			fmt.Printf("DEBUG DecodeDynamicMessage: No repeated fields found in %d-byte message\n", originalLen)
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
