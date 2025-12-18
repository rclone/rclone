package mediavfs

import (
	"encoding/binary"
	"fmt"
	"sort"
)

// FieldType represents a protobuf field type
type FieldType string

const (
	FieldTypeInt     FieldType = "int"
	FieldTypeString  FieldType = "string"
	FieldTypeBytes   FieldType = "bytes"
	FieldTypeMessage FieldType = "message"
	FieldTypeFixed32 FieldType = "fixed32"
	FieldTypeFixed64 FieldType = "fixed64"
)

// FieldDefinition defines a protobuf field's structure
type FieldDefinition struct {
	Type           FieldType
	FieldOrder     []string                    // For messages, defines field encoding order
	MessageTypedef map[string]*FieldDefinition // For messages, defines nested structure
	SeenRepeated   bool                        // If true, this field can appear multiple times
}

// TypedProtoEncoder encodes protobuf messages with type definitions
type TypedProtoEncoder struct {
	buf []byte
}

// NewTypedProtoEncoder creates a new encoder
func NewTypedProtoEncoder() *TypedProtoEncoder {
	return &TypedProtoEncoder{buf: make([]byte, 0, 1024)}
}

// Bytes returns the encoded bytes
func (e *TypedProtoEncoder) Bytes() []byte {
	return e.buf
}

// EncodeMessage encodes a message with type definition
func EncodeMessage(data interface{}, typedef *FieldDefinition) ([]byte, error) {
	encoder := NewTypedProtoEncoder()
	if err := encoder.encodeValue(data, typedef); err != nil {
		return nil, err
	}
	return encoder.Bytes(), nil
}

// encodeValue encodes a value based on its type definition
func (e *TypedProtoEncoder) encodeValue(data interface{}, typedef *FieldDefinition) error {
	switch typedef.Type {
	case FieldTypeMessage:
		return e.encodeMessageValue(data, typedef)
	case FieldTypeInt:
		return e.encodeIntValue(data)
	case FieldTypeString:
		return e.encodeStringValue(data)
	case FieldTypeBytes:
		return e.encodeBytesValue(data)
	default:
		return fmt.Errorf("unsupported type: %s", typedef.Type)
	}
}

// encodeMessageValue encodes a message (map or nested structure)
func (e *TypedProtoEncoder) encodeMessageValue(data interface{}, typedef *FieldDefinition) error {
	dataMap, ok := data.(map[string]interface{})
	if !ok {
		// Empty message
		return nil
	}

	// If typedef has field_order, encode in that order
	var fieldOrder []string
	if len(typedef.FieldOrder) > 0 {
		fieldOrder = typedef.FieldOrder
	} else {
		// Otherwise, encode fields in sorted order
		for fieldNum := range dataMap {
			fieldOrder = append(fieldOrder, fieldNum)
		}
		sort.Strings(fieldOrder)
	}

	for _, fieldNumStr := range fieldOrder {
		value, exists := dataMap[fieldNumStr]
		if !exists {
			continue
		}

		var fieldNum int
		fmt.Sscanf(fieldNumStr, "%d", &fieldNum)

		// Get field definition if available
		var fieldDef *FieldDefinition
		if typedef.MessageTypedef != nil {
			fieldDef = typedef.MessageTypedef[fieldNumStr]
		}

		// If no field definition, infer type from value
		if fieldDef == nil {
			fieldDef = inferFieldType(value)
		}

		// Handle repeated fields
		if fieldDef.SeenRepeated {
			// Encode as repeated field
			if slice, ok := value.([]interface{}); ok {
				for _, item := range slice {
					if err := e.encodeField(fieldNum, item, fieldDef); err != nil {
						return err
					}
				}
				continue
			}
		}

		// Encode single field
		if err := e.encodeField(fieldNum, value, fieldDef); err != nil {
			return err
		}
	}

	return nil
}

// encodeField encodes a single field with its tag
func (e *TypedProtoEncoder) encodeField(fieldNum int, value interface{}, typedef *FieldDefinition) error {
	switch typedef.Type {
	case FieldTypeInt:
		e.buf = append(e.buf, encodeTag(fieldNum, 0)...)
		return e.encodeIntValue(value)

	case FieldTypeString:
		e.buf = append(e.buf, encodeTag(fieldNum, 2)...)
		return e.encodeStringValue(value)

	case FieldTypeBytes:
		e.buf = append(e.buf, encodeTag(fieldNum, 2)...)
		return e.encodeBytesValue(value)

	case FieldTypeMessage:
		// Encode nested message
		nested := NewTypedProtoEncoder()
		if err := nested.encodeValue(value, typedef); err != nil {
			return err
		}
		nestedBytes := nested.Bytes()

		e.buf = append(e.buf, encodeTag(fieldNum, 2)...)
		e.buf = append(e.buf, encodeVarint(uint64(len(nestedBytes)))...)
		e.buf = append(e.buf, nestedBytes...)
		return nil

	default:
		return fmt.Errorf("unsupported field type: %s", typedef.Type)
	}
}

// encodeIntValue encodes an integer value (already has tag written)
func (e *TypedProtoEncoder) encodeIntValue(value interface{}) error {
	var intVal int64
	switch v := value.(type) {
	case int:
		intVal = int64(v)
	case int32:
		intVal = int64(v)
	case int64:
		intVal = v
	case uint32:
		intVal = int64(v)
	case uint64:
		intVal = int64(v)
	case float64:
		intVal = int64(v)
	default:
		return fmt.Errorf("cannot convert %T to int", value)
	}

	e.buf = append(e.buf, encodeVarint(uint64(intVal))...)
	return nil
}

// encodeStringValue encodes a string value (already has tag written)
func (e *TypedProtoEncoder) encodeStringValue(value interface{}) error {
	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("expected string, got %T", value)
	}

	e.buf = append(e.buf, encodeVarint(uint64(len(str)))...)
	e.buf = append(e.buf, []byte(str)...)
	return nil
}

// encodeBytesValue encodes a bytes value (already has tag written)
func (e *TypedProtoEncoder) encodeBytesValue(value interface{}) error {
	var bytes []byte

	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("expected bytes or string, got %T", value)
	}

	e.buf = append(e.buf, encodeVarint(uint64(len(bytes)))...)
	e.buf = append(e.buf, bytes...)
	return nil
}

// inferFieldType infers the field type from a Go value
func inferFieldType(value interface{}) *FieldDefinition {
	switch v := value.(type) {
	case int, int32, int64, uint32, uint64:
		return &FieldDefinition{Type: FieldTypeInt}
	case string:
		return &FieldDefinition{Type: FieldTypeString}
	case []byte:
		return &FieldDefinition{Type: FieldTypeBytes}
	case map[string]interface{}:
		return &FieldDefinition{Type: FieldTypeMessage, MessageTypedef: map[string]*FieldDefinition{}}
	case []interface{}:
		// Repeated field - infer from first element
		if len(v) > 0 {
			def := inferFieldType(v[0])
			def.SeenRepeated = true
			return def
		}
		return &FieldDefinition{Type: FieldTypeInt, SeenRepeated: true}
	default:
		return &FieldDefinition{Type: FieldTypeMessage, MessageTypedef: map[string]*FieldDefinition{}}
	}
}
