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
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"time"

	"cloud.google.com/go/civil"
)

// NullInt64 represents a BigQuery INT64 that may be NULL.
type NullInt64 struct {
	Int64 int64
	Valid bool // Valid is true if Int64 is not NULL.
}

func (n NullInt64) String() string { return nullstr(n.Valid, n.Int64) }

// NullString represents a BigQuery STRING that may be NULL.
type NullString struct {
	StringVal string
	Valid     bool // Valid is true if StringVal is not NULL.
}

func (n NullString) String() string { return nullstr(n.Valid, n.StringVal) }

// NullFloat64 represents a BigQuery FLOAT64 that may be NULL.
type NullFloat64 struct {
	Float64 float64
	Valid   bool // Valid is true if Float64 is not NULL.
}

func (n NullFloat64) String() string { return nullstr(n.Valid, n.Float64) }

// NullBool represents a BigQuery BOOL that may be NULL.
type NullBool struct {
	Bool  bool
	Valid bool // Valid is true if Bool is not NULL.
}

func (n NullBool) String() string { return nullstr(n.Valid, n.Bool) }

// NullTimestamp represents a BigQuery TIMESTAMP that may be null.
type NullTimestamp struct {
	Timestamp time.Time
	Valid     bool // Valid is true if Time is not NULL.
}

func (n NullTimestamp) String() string { return nullstr(n.Valid, n.Timestamp) }

// NullDate represents a BigQuery DATE that may be null.
type NullDate struct {
	Date  civil.Date
	Valid bool // Valid is true if Date is not NULL.
}

func (n NullDate) String() string { return nullstr(n.Valid, n.Date) }

// NullTime represents a BigQuery TIME that may be null.
type NullTime struct {
	Time  civil.Time
	Valid bool // Valid is true if Time is not NULL.
}

func (n NullTime) String() string {
	if !n.Valid {
		return "<null>"
	}
	return CivilTimeString(n.Time)
}

// NullDateTime represents a BigQuery DATETIME that may be null.
type NullDateTime struct {
	DateTime civil.DateTime
	Valid    bool // Valid is true if DateTime is not NULL.
}

func (n NullDateTime) String() string {
	if !n.Valid {
		return "<null>"
	}
	return CivilDateTimeString(n.DateTime)
}

func (n NullInt64) MarshalJSON() ([]byte, error)     { return nulljson(n.Valid, n.Int64) }
func (n NullFloat64) MarshalJSON() ([]byte, error)   { return nulljson(n.Valid, n.Float64) }
func (n NullBool) MarshalJSON() ([]byte, error)      { return nulljson(n.Valid, n.Bool) }
func (n NullString) MarshalJSON() ([]byte, error)    { return nulljson(n.Valid, n.StringVal) }
func (n NullTimestamp) MarshalJSON() ([]byte, error) { return nulljson(n.Valid, n.Timestamp) }
func (n NullDate) MarshalJSON() ([]byte, error)      { return nulljson(n.Valid, n.Date) }

func (n NullTime) MarshalJSON() ([]byte, error) {
	if !n.Valid {
		return jsonNull, nil
	}
	return []byte(`"` + CivilTimeString(n.Time) + `"`), nil
}

func (n NullDateTime) MarshalJSON() ([]byte, error) {
	if !n.Valid {
		return jsonNull, nil
	}
	return []byte(`"` + CivilDateTimeString(n.DateTime) + `"`), nil
}

func nullstr(valid bool, v interface{}) string {
	if !valid {
		return "NULL"
	}
	return fmt.Sprint(v)
}

var jsonNull = []byte("null")

func nulljson(valid bool, v interface{}) ([]byte, error) {
	if !valid {
		return jsonNull, nil
	}
	return json.Marshal(v)
}

func (n *NullInt64) UnmarshalJSON(b []byte) error {
	n.Valid = false
	n.Int64 = 0
	if bytes.Equal(b, jsonNull) {
		return nil
	}

	if err := json.Unmarshal(b, &n.Int64); err != nil {
		return err
	}
	n.Valid = true
	return nil
}

func (n *NullFloat64) UnmarshalJSON(b []byte) error {
	n.Valid = false
	n.Float64 = 0
	if bytes.Equal(b, jsonNull) {
		return nil
	}

	if err := json.Unmarshal(b, &n.Float64); err != nil {
		return err
	}
	n.Valid = true
	return nil
}

func (n *NullBool) UnmarshalJSON(b []byte) error {
	n.Valid = false
	n.Bool = false
	if bytes.Equal(b, jsonNull) {
		return nil
	}

	if err := json.Unmarshal(b, &n.Bool); err != nil {
		return err
	}
	n.Valid = true
	return nil
}

func (n *NullString) UnmarshalJSON(b []byte) error {
	n.Valid = false
	n.StringVal = ""
	if bytes.Equal(b, jsonNull) {
		return nil
	}

	if err := json.Unmarshal(b, &n.StringVal); err != nil {
		return err
	}
	n.Valid = true
	return nil
}

func (n *NullTimestamp) UnmarshalJSON(b []byte) error {
	n.Valid = false
	n.Timestamp = time.Time{}
	if bytes.Equal(b, jsonNull) {
		return nil
	}

	if err := json.Unmarshal(b, &n.Timestamp); err != nil {
		return err
	}
	n.Valid = true
	return nil
}

func (n *NullDate) UnmarshalJSON(b []byte) error {
	n.Valid = false
	n.Date = civil.Date{}
	if bytes.Equal(b, jsonNull) {
		return nil
	}

	if err := json.Unmarshal(b, &n.Date); err != nil {
		return err
	}
	n.Valid = true
	return nil
}

func (n *NullTime) UnmarshalJSON(b []byte) error {
	n.Valid = false
	n.Time = civil.Time{}
	if bytes.Equal(b, jsonNull) {
		return nil
	}

	s, err := strconv.Unquote(string(b))
	if err != nil {
		return err
	}

	t, err := civil.ParseTime(s)
	if err != nil {
		return err
	}
	n.Time = t

	n.Valid = true
	return nil
}

func (n *NullDateTime) UnmarshalJSON(b []byte) error {
	n.Valid = false
	n.DateTime = civil.DateTime{}
	if bytes.Equal(b, jsonNull) {
		return nil
	}

	s, err := strconv.Unquote(string(b))
	if err != nil {
		return err
	}

	dt, err := parseCivilDateTime(s)
	if err != nil {
		return err
	}
	n.DateTime = dt

	n.Valid = true
	return nil
}

var (
	typeOfNullInt64     = reflect.TypeOf(NullInt64{})
	typeOfNullFloat64   = reflect.TypeOf(NullFloat64{})
	typeOfNullBool      = reflect.TypeOf(NullBool{})
	typeOfNullString    = reflect.TypeOf(NullString{})
	typeOfNullTimestamp = reflect.TypeOf(NullTimestamp{})
	typeOfNullDate      = reflect.TypeOf(NullDate{})
	typeOfNullTime      = reflect.TypeOf(NullTime{})
	typeOfNullDateTime  = reflect.TypeOf(NullDateTime{})
)

func nullableFieldType(t reflect.Type) FieldType {
	switch t {
	case typeOfNullInt64:
		return IntegerFieldType
	case typeOfNullFloat64:
		return FloatFieldType
	case typeOfNullBool:
		return BooleanFieldType
	case typeOfNullString:
		return StringFieldType
	case typeOfNullTimestamp:
		return TimestampFieldType
	case typeOfNullDate:
		return DateFieldType
	case typeOfNullTime:
		return TimeFieldType
	case typeOfNullDateTime:
		return DateTimeFieldType
	default:
		return ""
	}
}
