// Copyright 2015 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bigquery

import (
	"errors"
	"fmt"
	"reflect"

	"cloud.google.com/go/internal/atomiccache"
	bq "google.golang.org/api/bigquery/v2"
)

// Schema describes the fields in a table or query result.
type Schema []*FieldSchema

type FieldSchema struct {
	// The field name.
	// Must contain only letters (a-z, A-Z), numbers (0-9), or underscores (_),
	// and must start with a letter or underscore.
	// The maximum length is 128 characters.
	Name string

	// A description of the field. The maximum length is 16,384 characters.
	Description string

	// Whether the field may contain multiple values.
	Repeated bool
	// Whether the field is required.  Ignored if Repeated is true.
	Required bool

	// The field data type.  If Type is Record, then this field contains a nested schema,
	// which is described by Schema.
	Type FieldType
	// Describes the nested schema if Type is set to Record.
	Schema Schema
}

func (fs *FieldSchema) toBQ() *bq.TableFieldSchema {
	tfs := &bq.TableFieldSchema{
		Description: fs.Description,
		Name:        fs.Name,
		Type:        string(fs.Type),
	}

	if fs.Repeated {
		tfs.Mode = "REPEATED"
	} else if fs.Required {
		tfs.Mode = "REQUIRED"
	} // else leave as default, which is interpreted as NULLABLE.

	for _, f := range fs.Schema {
		tfs.Fields = append(tfs.Fields, f.toBQ())
	}

	return tfs
}

func (s Schema) toBQ() *bq.TableSchema {
	var fields []*bq.TableFieldSchema
	for _, f := range s {
		fields = append(fields, f.toBQ())
	}
	return &bq.TableSchema{Fields: fields}
}

func bqToFieldSchema(tfs *bq.TableFieldSchema) *FieldSchema {
	fs := &FieldSchema{
		Description: tfs.Description,
		Name:        tfs.Name,
		Repeated:    tfs.Mode == "REPEATED",
		Required:    tfs.Mode == "REQUIRED",
		Type:        FieldType(tfs.Type),
	}

	for _, f := range tfs.Fields {
		fs.Schema = append(fs.Schema, bqToFieldSchema(f))
	}
	return fs
}

func bqToSchema(ts *bq.TableSchema) Schema {
	if ts == nil {
		return nil
	}
	var s Schema
	for _, f := range ts.Fields {
		s = append(s, bqToFieldSchema(f))
	}
	return s
}

type FieldType string

const (
	StringFieldType    FieldType = "STRING"
	BytesFieldType     FieldType = "BYTES"
	IntegerFieldType   FieldType = "INTEGER"
	FloatFieldType     FieldType = "FLOAT"
	BooleanFieldType   FieldType = "BOOLEAN"
	TimestampFieldType FieldType = "TIMESTAMP"
	RecordFieldType    FieldType = "RECORD"
	DateFieldType      FieldType = "DATE"
	TimeFieldType      FieldType = "TIME"
	DateTimeFieldType  FieldType = "DATETIME"
)

var (
	errNoStruct             = errors.New("bigquery: can only infer schema from struct or pointer to struct")
	errUnsupportedFieldType = errors.New("bigquery: unsupported type of field in struct")
	errInvalidFieldName     = errors.New("bigquery: invalid name of field in struct")
	errBadNullable          = errors.New(`bigquery: use "nullable" only for []byte and struct pointers; for all other types, use a NullXXX type`)
)

var typeOfByteSlice = reflect.TypeOf([]byte{})

// InferSchema tries to derive a BigQuery schema from the supplied struct value.
// Each exported struct field is mapped to a field in the schema.
//
// The following BigQuery types are inferred from the corresponding Go types.
// (This is the same mapping as that used for RowIterator.Next.) Fields inferred
// from these types are marked required (non-nullable).
//
//   STRING      string
//   BOOL        bool
//   INTEGER     int, int8, int16, int32, int64, uint8, uint16, uint32
//   FLOAT       float32, float64
//   BYTES       []byte
//   TIMESTAMP   time.Time
//   DATE        civil.Date
//   TIME        civil.Time
//   DATETIME    civil.DateTime
//
// A Go slice or array type is inferred to be a BigQuery repeated field of the
// element type. The element type must be one of the above listed types.
//
// Nullable fields are inferred from the NullXXX types, declared in this package:
//
//   STRING      NullString
//   BOOL        NullBool
//   INTEGER     NullInt64
//   FLOAT       NullFloat64
//   TIMESTAMP   NullTimestamp
//   DATE        NullDate
//   TIME        NullTime
//   DATETIME    NullDateTime

// For a nullable BYTES field, use the type []byte and tag the field "nullable" (see below).
//
// A struct field that is of struct type is inferred to be a required field of type
// RECORD with a schema inferred recursively. For backwards compatibility, a field of
// type pointer to struct is also inferred to be required. To get a nullable RECORD
// field, use the "nullable" tag (see below).
//
// InferSchema returns an error if any of the examined fields is of type uint,
// uint64, uintptr, map, interface, complex64, complex128, func, or chan. Future
// versions may handle these cases without error.
//
// Recursively defined structs are also disallowed.
//
// Struct fields may be tagged in a way similar to the encoding/json package.
// A tag of the form
//     bigquery:"name"
// uses "name" instead of the struct field name as the BigQuery field name.
// A tag of the form
//     bigquery:"-"
// omits the field from the inferred schema.
// The "nullable" option marks the field as nullable (not required). It is only
// needed for []byte and pointer-to-struct fields, and cannot appear on other
// fields. In this example, the Go name of the field is retained:
//     bigquery:",nullable"
func InferSchema(st interface{}) (Schema, error) {
	return inferSchemaReflectCached(reflect.TypeOf(st))
}

// TODO(jba): replace with sync.Map for Go 1.9.
var schemaCache atomiccache.Cache

type cacheVal struct {
	schema Schema
	err    error
}

func inferSchemaReflectCached(t reflect.Type) (Schema, error) {
	cv := schemaCache.Get(t, func() interface{} {
		s, err := inferSchemaReflect(t)
		return cacheVal{s, err}
	}).(cacheVal)
	return cv.schema, cv.err
}

func inferSchemaReflect(t reflect.Type) (Schema, error) {
	rec, err := hasRecursiveType(t, nil)
	if err != nil {
		return nil, err
	}
	if rec {
		return nil, fmt.Errorf("bigquery: schema inference for recursive type %s", t)
	}
	return inferStruct(t)
}

func inferStruct(t reflect.Type) (Schema, error) {
	switch t.Kind() {
	case reflect.Ptr:
		if t.Elem().Kind() != reflect.Struct {
			return nil, errNoStruct
		}
		t = t.Elem()
		fallthrough

	case reflect.Struct:
		return inferFields(t)
	default:
		return nil, errNoStruct
	}
}

// inferFieldSchema infers the FieldSchema for a Go type
func inferFieldSchema(rt reflect.Type, nullable bool) (*FieldSchema, error) {
	// Only []byte and struct pointers can be tagged nullable.
	if nullable && !(rt == typeOfByteSlice || rt.Kind() == reflect.Ptr && rt.Elem().Kind() == reflect.Struct) {
		return nil, errBadNullable
	}
	switch rt {
	case typeOfByteSlice:
		return &FieldSchema{Required: !nullable, Type: BytesFieldType}, nil
	case typeOfGoTime:
		return &FieldSchema{Required: true, Type: TimestampFieldType}, nil
	case typeOfDate:
		return &FieldSchema{Required: true, Type: DateFieldType}, nil
	case typeOfTime:
		return &FieldSchema{Required: true, Type: TimeFieldType}, nil
	case typeOfDateTime:
		return &FieldSchema{Required: true, Type: DateTimeFieldType}, nil
	}
	if ft := nullableFieldType(rt); ft != "" {
		return &FieldSchema{Required: false, Type: ft}, nil
	}
	if isSupportedIntType(rt) || isSupportedUintType(rt) {
		return &FieldSchema{Required: true, Type: IntegerFieldType}, nil
	}
	switch rt.Kind() {
	case reflect.Slice, reflect.Array:
		et := rt.Elem()
		if et != typeOfByteSlice && (et.Kind() == reflect.Slice || et.Kind() == reflect.Array) {
			// Multi dimensional slices/arrays are not supported by BigQuery
			return nil, errUnsupportedFieldType
		}
		if nullableFieldType(et) != "" {
			// Repeated nullable types are not supported by BigQuery.
			return nil, errUnsupportedFieldType
		}
		f, err := inferFieldSchema(et, false)
		if err != nil {
			return nil, err
		}
		f.Repeated = true
		f.Required = false
		return f, nil
	case reflect.Ptr:
		if rt.Elem().Kind() != reflect.Struct {
			return nil, errUnsupportedFieldType
		}
		fallthrough
	case reflect.Struct:
		nested, err := inferStruct(rt)
		if err != nil {
			return nil, err
		}
		return &FieldSchema{Required: !nullable, Type: RecordFieldType, Schema: nested}, nil
	case reflect.String:
		return &FieldSchema{Required: !nullable, Type: StringFieldType}, nil
	case reflect.Bool:
		return &FieldSchema{Required: !nullable, Type: BooleanFieldType}, nil
	case reflect.Float32, reflect.Float64:
		return &FieldSchema{Required: !nullable, Type: FloatFieldType}, nil
	default:
		return nil, errUnsupportedFieldType
	}
}

// inferFields extracts all exported field types from struct type.
func inferFields(rt reflect.Type) (Schema, error) {
	var s Schema
	fields, err := fieldCache.Fields(rt)
	if err != nil {
		return nil, err
	}
	for _, field := range fields {
		var nullable bool
		for _, opt := range field.ParsedTag.([]string) {
			if opt == nullableTagOption {
				nullable = true
				break
			}
		}
		f, err := inferFieldSchema(field.Type, nullable)
		if err != nil {
			return nil, err
		}
		f.Name = field.Name
		s = append(s, f)
	}
	return s, nil
}

// isSupportedIntType reports whether t is an int type that can be properly
// represented by the BigQuery INTEGER/INT64 type.
func isSupportedIntType(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
		return true
	default:
		return false
	}
}

// isSupportedIntType reports whether t is a uint type that can be properly
// represented by the BigQuery INTEGER/INT64 type.
func isSupportedUintType(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Uint8, reflect.Uint16, reflect.Uint32:
		return true
	default:
		return false
	}
}

// typeList is a linked list of reflect.Types.
type typeList struct {
	t    reflect.Type
	next *typeList
}

func (l *typeList) has(t reflect.Type) bool {
	for l != nil {
		if l.t == t {
			return true
		}
		l = l.next
	}
	return false
}

// hasRecursiveType reports whether t or any type inside t refers to itself, directly or indirectly,
// via exported fields. (Schema inference ignores unexported fields.)
func hasRecursiveType(t reflect.Type, seen *typeList) (bool, error) {
	for t.Kind() == reflect.Ptr || t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return false, nil
	}
	if seen.has(t) {
		return true, nil
	}
	fields, err := fieldCache.Fields(t)
	if err != nil {
		return false, err
	}
	seen = &typeList{t, seen}
	// Because seen is a linked list, additions to it from one field's
	// recursive call will not affect the value for subsequent fields' calls.
	for _, field := range fields {
		ok, err := hasRecursiveType(field.Type, seen)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}
