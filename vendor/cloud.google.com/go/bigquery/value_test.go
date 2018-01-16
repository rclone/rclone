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
	"encoding/base64"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"cloud.google.com/go/civil"
	"cloud.google.com/go/internal/pretty"
	"cloud.google.com/go/internal/testutil"

	bq "google.golang.org/api/bigquery/v2"
)

func TestConvertBasicValues(t *testing.T) {
	schema := []*FieldSchema{
		{Type: StringFieldType},
		{Type: IntegerFieldType},
		{Type: FloatFieldType},
		{Type: BooleanFieldType},
		{Type: BytesFieldType},
	}
	row := &bq.TableRow{
		F: []*bq.TableCell{
			{V: "a"},
			{V: "1"},
			{V: "1.2"},
			{V: "true"},
			{V: base64.StdEncoding.EncodeToString([]byte("foo"))},
		},
	}
	got, err := convertRow(row, schema)
	if err != nil {
		t.Fatalf("error converting: %v", err)
	}
	want := []Value{"a", int64(1), 1.2, true, []byte("foo")}
	if !testutil.Equal(got, want) {
		t.Errorf("converting basic values: got:\n%v\nwant:\n%v", got, want)
	}
}

func TestConvertTime(t *testing.T) {
	schema := []*FieldSchema{
		{Type: TimestampFieldType},
		{Type: DateFieldType},
		{Type: TimeFieldType},
		{Type: DateTimeFieldType},
	}
	ts := testTimestamp.Round(time.Millisecond)
	row := &bq.TableRow{
		F: []*bq.TableCell{
			{V: fmt.Sprintf("%.10f", float64(ts.UnixNano())/1e9)},
			{V: testDate.String()},
			{V: testTime.String()},
			{V: testDateTime.String()},
		},
	}
	got, err := convertRow(row, schema)
	if err != nil {
		t.Fatalf("error converting: %v", err)
	}
	want := []Value{ts, testDate, testTime, testDateTime}
	for i, g := range got {
		w := want[i]
		if !testutil.Equal(g, w) {
			t.Errorf("#%d: got:\n%v\nwant:\n%v", i, g, w)
		}
	}
	if got[0].(time.Time).Location() != time.UTC {
		t.Errorf("expected time zone UTC: got:\n%v", got)
	}
}

func TestConvertNullValues(t *testing.T) {
	schema := []*FieldSchema{
		{Type: StringFieldType},
	}
	row := &bq.TableRow{
		F: []*bq.TableCell{
			{V: nil},
		},
	}
	got, err := convertRow(row, schema)
	if err != nil {
		t.Fatalf("error converting: %v", err)
	}
	want := []Value{nil}
	if !testutil.Equal(got, want) {
		t.Errorf("converting null values: got:\n%v\nwant:\n%v", got, want)
	}
}

func TestBasicRepetition(t *testing.T) {
	schema := []*FieldSchema{
		{Type: IntegerFieldType, Repeated: true},
	}
	row := &bq.TableRow{
		F: []*bq.TableCell{
			{
				V: []interface{}{
					map[string]interface{}{
						"v": "1",
					},
					map[string]interface{}{
						"v": "2",
					},
					map[string]interface{}{
						"v": "3",
					},
				},
			},
		},
	}
	got, err := convertRow(row, schema)
	if err != nil {
		t.Fatalf("error converting: %v", err)
	}
	want := []Value{[]Value{int64(1), int64(2), int64(3)}}
	if !testutil.Equal(got, want) {
		t.Errorf("converting basic repeated values: got:\n%v\nwant:\n%v", got, want)
	}
}

func TestNestedRecordContainingRepetition(t *testing.T) {
	schema := []*FieldSchema{
		{
			Type: RecordFieldType,
			Schema: Schema{
				{Type: IntegerFieldType, Repeated: true},
			},
		},
	}
	row := &bq.TableRow{
		F: []*bq.TableCell{
			{
				V: map[string]interface{}{
					"f": []interface{}{
						map[string]interface{}{
							"v": []interface{}{
								map[string]interface{}{"v": "1"},
								map[string]interface{}{"v": "2"},
								map[string]interface{}{"v": "3"},
							},
						},
					},
				},
			},
		},
	}

	got, err := convertRow(row, schema)
	if err != nil {
		t.Fatalf("error converting: %v", err)
	}
	want := []Value{[]Value{[]Value{int64(1), int64(2), int64(3)}}}
	if !testutil.Equal(got, want) {
		t.Errorf("converting basic repeated values: got:\n%v\nwant:\n%v", got, want)
	}
}

func TestRepeatedRecordContainingRepetition(t *testing.T) {
	schema := []*FieldSchema{
		{
			Type:     RecordFieldType,
			Repeated: true,
			Schema: Schema{
				{Type: IntegerFieldType, Repeated: true},
			},
		},
	}
	row := &bq.TableRow{F: []*bq.TableCell{
		{
			V: []interface{}{ // repeated records.
				map[string]interface{}{ // first record.
					"v": map[string]interface{}{ // pointless single-key-map wrapper.
						"f": []interface{}{ // list of record fields.
							map[string]interface{}{ // only record (repeated ints)
								"v": []interface{}{ // pointless wrapper.
									map[string]interface{}{
										"v": "1",
									},
									map[string]interface{}{
										"v": "2",
									},
									map[string]interface{}{
										"v": "3",
									},
								},
							},
						},
					},
				},
				map[string]interface{}{ // second record.
					"v": map[string]interface{}{
						"f": []interface{}{
							map[string]interface{}{
								"v": []interface{}{
									map[string]interface{}{
										"v": "4",
									},
									map[string]interface{}{
										"v": "5",
									},
									map[string]interface{}{
										"v": "6",
									},
								},
							},
						},
					},
				},
			},
		},
	}}

	got, err := convertRow(row, schema)
	if err != nil {
		t.Fatalf("error converting: %v", err)
	}
	want := []Value{ // the row is a list of length 1, containing an entry for the repeated record.
		[]Value{ // the repeated record is a list of length 2, containing an entry for each repetition.
			[]Value{ // the record is a list of length 1, containing an entry for the repeated integer field.
				[]Value{int64(1), int64(2), int64(3)}, // the repeated integer field is a list of length 3.
			},
			[]Value{ // second record
				[]Value{int64(4), int64(5), int64(6)},
			},
		},
	}
	if !testutil.Equal(got, want) {
		t.Errorf("converting repeated records with repeated values: got:\n%v\nwant:\n%v", got, want)
	}
}

func TestRepeatedRecordContainingRecord(t *testing.T) {
	schema := []*FieldSchema{
		{
			Type:     RecordFieldType,
			Repeated: true,
			Schema: Schema{
				{
					Type: StringFieldType,
				},
				{
					Type: RecordFieldType,
					Schema: Schema{
						{Type: IntegerFieldType},
						{Type: StringFieldType},
					},
				},
			},
		},
	}
	row := &bq.TableRow{F: []*bq.TableCell{
		{
			V: []interface{}{ // repeated records.
				map[string]interface{}{ // first record.
					"v": map[string]interface{}{ // pointless single-key-map wrapper.
						"f": []interface{}{ // list of record fields.
							map[string]interface{}{ // first record field (name)
								"v": "first repeated record",
							},
							map[string]interface{}{ // second record field (nested record).
								"v": map[string]interface{}{ // pointless single-key-map wrapper.
									"f": []interface{}{ // nested record fields
										map[string]interface{}{
											"v": "1",
										},
										map[string]interface{}{
											"v": "two",
										},
									},
								},
							},
						},
					},
				},
				map[string]interface{}{ // second record.
					"v": map[string]interface{}{
						"f": []interface{}{
							map[string]interface{}{
								"v": "second repeated record",
							},
							map[string]interface{}{
								"v": map[string]interface{}{
									"f": []interface{}{
										map[string]interface{}{
											"v": "3",
										},
										map[string]interface{}{
											"v": "four",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}}

	got, err := convertRow(row, schema)
	if err != nil {
		t.Fatalf("error converting: %v", err)
	}
	// TODO: test with flattenresults.
	want := []Value{ // the row is a list of length 1, containing an entry for the repeated record.
		[]Value{ // the repeated record is a list of length 2, containing an entry for each repetition.
			[]Value{ // record contains a string followed by a nested record.
				"first repeated record",
				[]Value{
					int64(1),
					"two",
				},
			},
			[]Value{ // second record.
				"second repeated record",
				[]Value{
					int64(3),
					"four",
				},
			},
		},
	}
	if !testutil.Equal(got, want) {
		t.Errorf("converting repeated records containing record : got:\n%v\nwant:\n%v", got, want)
	}
}

func TestConvertRowErrors(t *testing.T) {
	// mismatched lengths
	if _, err := convertRow(&bq.TableRow{F: []*bq.TableCell{{V: ""}}}, Schema{}); err == nil {
		t.Error("got nil, want error")
	}
	v3 := map[string]interface{}{"v": 3}
	for _, test := range []struct {
		value interface{}
		fs    FieldSchema
	}{
		{3, FieldSchema{Type: IntegerFieldType}}, // not a string
		{[]interface{}{v3}, // not a string, repeated
			FieldSchema{Type: IntegerFieldType, Repeated: true}},
		{map[string]interface{}{"f": []interface{}{v3}}, // not a string, nested
			FieldSchema{Type: RecordFieldType, Schema: Schema{{Type: IntegerFieldType}}}},
		{map[string]interface{}{"f": []interface{}{v3}}, // wrong length, nested
			FieldSchema{Type: RecordFieldType, Schema: Schema{}}},
	} {
		_, err := convertRow(
			&bq.TableRow{F: []*bq.TableCell{{V: test.value}}},
			Schema{&test.fs})
		if err == nil {
			t.Errorf("value %v, fs %v: got nil, want error", test.value, test.fs)
		}
	}

	// bad field type
	if _, err := convertBasicType("", FieldType("BAD")); err == nil {
		t.Error("got nil, want error")
	}
}

func TestValuesSaverConvertsToMap(t *testing.T) {
	testCases := []struct {
		vs           ValuesSaver
		wantInsertID string
		wantRow      map[string]Value
	}{
		{
			vs: ValuesSaver{
				Schema: []*FieldSchema{
					{Name: "intField", Type: IntegerFieldType},
					{Name: "strField", Type: StringFieldType},
					{Name: "dtField", Type: DateTimeFieldType},
				},
				InsertID: "iid",
				Row: []Value{1, "a",
					civil.DateTime{civil.Date{1, 2, 3}, civil.Time{4, 5, 6, 7000}}},
			},
			wantInsertID: "iid",
			wantRow: map[string]Value{"intField": 1, "strField": "a",
				"dtField": "0001-02-03 04:05:06.000007"},
		},
		{
			vs: ValuesSaver{
				Schema: []*FieldSchema{
					{Name: "intField", Type: IntegerFieldType},
					{
						Name: "recordField",
						Type: RecordFieldType,
						Schema: []*FieldSchema{
							{Name: "nestedInt", Type: IntegerFieldType, Repeated: true},
						},
					},
				},
				InsertID: "iid",
				Row:      []Value{1, []Value{[]Value{2, 3}}},
			},
			wantInsertID: "iid",
			wantRow: map[string]Value{
				"intField": 1,
				"recordField": map[string]Value{
					"nestedInt": []Value{2, 3},
				},
			},
		},
		{ // repeated nested field
			vs: ValuesSaver{
				Schema: Schema{
					{
						Name: "records",
						Type: RecordFieldType,
						Schema: Schema{
							{Name: "x", Type: IntegerFieldType},
							{Name: "y", Type: IntegerFieldType},
						},
						Repeated: true,
					},
				},
				InsertID: "iid",
				Row: []Value{ // a row is a []Value
					[]Value{ // repeated field's value is a []Value
						[]Value{1, 2}, // first record of the repeated field
						[]Value{3, 4}, // second record
					},
				},
			},
			wantInsertID: "iid",
			wantRow: map[string]Value{
				"records": []Value{
					map[string]Value{"x": 1, "y": 2},
					map[string]Value{"x": 3, "y": 4},
				},
			},
		},
	}
	for _, tc := range testCases {
		gotRow, gotInsertID, err := tc.vs.Save()
		if err != nil {
			t.Errorf("Expected successful save; got: %v", err)
			continue
		}
		if !testutil.Equal(gotRow, tc.wantRow) {
			t.Errorf("%v row:\ngot:\n%+v\nwant:\n%+v", tc.vs, gotRow, tc.wantRow)
		}
		if !testutil.Equal(gotInsertID, tc.wantInsertID) {
			t.Errorf("%v ID:\ngot:\n%+v\nwant:\n%+v", tc.vs, gotInsertID, tc.wantInsertID)
		}
	}
}

func TestValuesToMapErrors(t *testing.T) {
	for _, test := range []struct {
		values []Value
		schema Schema
	}{
		{ // mismatched length
			[]Value{1},
			Schema{},
		},
		{ // nested record not a slice
			[]Value{1},
			Schema{{Type: RecordFieldType}},
		},
		{ // nested record mismatched length
			[]Value{[]Value{1}},
			Schema{{Type: RecordFieldType}},
		},
		{ // nested repeated record not a slice
			[]Value{[]Value{1}},
			Schema{{Type: RecordFieldType, Repeated: true}},
		},
		{ // nested repeated record mismatched length
			[]Value{[]Value{[]Value{1}}},
			Schema{{Type: RecordFieldType, Repeated: true}},
		},
	} {
		_, err := valuesToMap(test.values, test.schema)
		if err == nil {
			t.Errorf("%v, %v: got nil, want error", test.values, test.schema)
		}
	}
}

func TestStructSaver(t *testing.T) {
	schema := Schema{
		{Name: "s", Type: StringFieldType},
		{Name: "r", Type: IntegerFieldType, Repeated: true},
		{Name: "t", Type: TimeFieldType},
		{Name: "tr", Type: TimeFieldType, Repeated: true},
		{Name: "nested", Type: RecordFieldType, Schema: Schema{
			{Name: "b", Type: BooleanFieldType},
		}},
		{Name: "rnested", Type: RecordFieldType, Repeated: true, Schema: Schema{
			{Name: "b", Type: BooleanFieldType},
		}},
	}

	type (
		N struct{ B bool }
		T struct {
			S       string
			R       []int
			T       civil.Time
			TR      []civil.Time
			Nested  *N
			Rnested []*N
		}
	)

	check := func(msg string, in interface{}, want map[string]Value) {
		ss := StructSaver{
			Schema:   schema,
			InsertID: "iid",
			Struct:   in,
		}
		got, gotIID, err := ss.Save()
		if err != nil {
			t.Fatalf("%s: %v", msg, err)
		}
		if wantIID := "iid"; gotIID != wantIID {
			t.Errorf("%s: InsertID: got %q, want %q", msg, gotIID, wantIID)
		}
		if !testutil.Equal(got, want) {
			t.Errorf("%s:\ngot\n%#v\nwant\n%#v", msg, got, want)
		}
	}
	ct1 := civil.Time{1, 2, 3, 4000}
	ct2 := civil.Time{5, 6, 7, 8000}
	in := T{
		S:       "x",
		R:       []int{1, 2},
		T:       ct1,
		TR:      []civil.Time{ct1, ct2},
		Nested:  &N{B: true},
		Rnested: []*N{{true}, {false}},
	}
	want := map[string]Value{
		"s":       "x",
		"r":       []int{1, 2},
		"t":       "01:02:03.000004",
		"tr":      []string{"01:02:03.000004", "05:06:07.000008"},
		"nested":  map[string]Value{"b": true},
		"rnested": []Value{map[string]Value{"b": true}, map[string]Value{"b": false}},
	}
	check("all values", in, want)
	check("all values, ptr", &in, want)
	check("empty struct", T{}, map[string]Value{"s": "", "t": "00:00:00"})

	// Missing and extra fields ignored.
	type T2 struct {
		S string
		// missing R, Nested, RNested
		Extra int
	}
	check("missing and extra", T2{S: "x"}, map[string]Value{"s": "x"})

	check("nils in slice", T{Rnested: []*N{{true}, nil, {false}}},
		map[string]Value{
			"s":       "",
			"t":       "00:00:00",
			"rnested": []Value{map[string]Value{"b": true}, map[string]Value(nil), map[string]Value{"b": false}},
		})
}

func TestStructSaverErrors(t *testing.T) {
	type (
		badField struct {
			I int `bigquery:"@"`
		}
		badR  struct{ R int }
		badRN struct{ R []int }
	)

	for i, test := range []struct {
		struct_ interface{}
		schema  Schema
	}{
		{0, nil},                                              // not a struct
		{&badField{}, nil},                                    // bad field name
		{&badR{}, Schema{{Name: "r", Repeated: true}}},        // repeated field has bad type
		{&badR{}, Schema{{Name: "r", Type: RecordFieldType}}}, // nested field has bad type
		{&badRN{[]int{0}}, // nested repeated field has bad type
			Schema{{Name: "r", Type: RecordFieldType, Repeated: true}}},
	} {
		ss := &StructSaver{Struct: test.struct_, Schema: test.schema}
		_, _, err := ss.Save()
		if err == nil {
			t.Errorf("#%d, %v, %v: got nil, want error", i, test.struct_, test.schema)
		}
	}
}

func TestConvertRows(t *testing.T) {
	schema := []*FieldSchema{
		{Type: StringFieldType},
		{Type: IntegerFieldType},
		{Type: FloatFieldType},
		{Type: BooleanFieldType},
	}
	rows := []*bq.TableRow{
		{F: []*bq.TableCell{
			{V: "a"},
			{V: "1"},
			{V: "1.2"},
			{V: "true"},
		}},
		{F: []*bq.TableCell{
			{V: "b"},
			{V: "2"},
			{V: "2.2"},
			{V: "false"},
		}},
	}
	want := [][]Value{
		{"a", int64(1), 1.2, true},
		{"b", int64(2), 2.2, false},
	}
	got, err := convertRows(rows, schema)
	if err != nil {
		t.Fatalf("got %v, want nil", err)
	}
	if !testutil.Equal(got, want) {
		t.Errorf("\ngot  %v\nwant %v", got, want)
	}

	rows[0].F[0].V = 1
	_, err = convertRows(rows, schema)
	if err == nil {
		t.Error("got nil, want error")
	}
}

func TestValueList(t *testing.T) {
	schema := Schema{
		{Name: "s", Type: StringFieldType},
		{Name: "i", Type: IntegerFieldType},
		{Name: "f", Type: FloatFieldType},
		{Name: "b", Type: BooleanFieldType},
	}
	want := []Value{"x", 7, 3.14, true}
	var got []Value
	vl := (*valueList)(&got)
	if err := vl.Load(want, schema); err != nil {
		t.Fatal(err)
	}

	if !testutil.Equal(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}

	// Load truncates, not appends.
	// https://github.com/GoogleCloudPlatform/google-cloud-go/issues/437
	if err := vl.Load(want, schema); err != nil {
		t.Fatal(err)
	}
	if !testutil.Equal(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestValueMap(t *testing.T) {
	ns := Schema{
		{Name: "x", Type: IntegerFieldType},
		{Name: "y", Type: IntegerFieldType},
	}
	schema := Schema{
		{Name: "s", Type: StringFieldType},
		{Name: "i", Type: IntegerFieldType},
		{Name: "f", Type: FloatFieldType},
		{Name: "b", Type: BooleanFieldType},
		{Name: "n", Type: RecordFieldType, Schema: ns},
		{Name: "rn", Type: RecordFieldType, Schema: ns, Repeated: true},
	}
	in := []Value{"x", 7, 3.14, true,
		[]Value{1, 2},
		[]Value{[]Value{3, 4}, []Value{5, 6}},
	}
	var vm valueMap
	if err := vm.Load(in, schema); err != nil {
		t.Fatal(err)
	}
	want := map[string]Value{
		"s": "x",
		"i": 7,
		"f": 3.14,
		"b": true,
		"n": map[string]Value{"x": 1, "y": 2},
		"rn": []Value{
			map[string]Value{"x": 3, "y": 4},
			map[string]Value{"x": 5, "y": 6},
		},
	}
	if !testutil.Equal(vm, valueMap(want)) {
		t.Errorf("got\n%+v\nwant\n%+v", vm, want)
	}

}

var (
	// For testing StructLoader
	schema2 = Schema{
		{Name: "s", Type: StringFieldType},
		{Name: "s2", Type: StringFieldType},
		{Name: "by", Type: BytesFieldType},
		{Name: "I", Type: IntegerFieldType},
		{Name: "F", Type: FloatFieldType},
		{Name: "B", Type: BooleanFieldType},
		{Name: "TS", Type: TimestampFieldType},
		{Name: "D", Type: DateFieldType},
		{Name: "T", Type: TimeFieldType},
		{Name: "DT", Type: DateTimeFieldType},
		{Name: "nested", Type: RecordFieldType, Schema: Schema{
			{Name: "nestS", Type: StringFieldType},
			{Name: "nestI", Type: IntegerFieldType},
		}},
		{Name: "t", Type: StringFieldType},
	}

	testTimestamp = time.Date(2016, 11, 5, 7, 50, 22, 8, time.UTC)
	testDate      = civil.Date{2016, 11, 5}
	testTime      = civil.Time{7, 50, 22, 8}
	testDateTime  = civil.DateTime{testDate, testTime}

	testValues = []Value{"x", "y", []byte{1, 2, 3}, int64(7), 3.14, true,
		testTimestamp, testDate, testTime, testDateTime,
		[]Value{"nested", int64(17)}, "z"}
)

type testStruct1 struct {
	B bool
	I int
	times
	S      string
	S2     String
	By     []byte
	s      string
	F      float64
	Nested nested
	Tagged string `bigquery:"t"`
}

type String string

type nested struct {
	NestS string
	NestI int
}

type times struct {
	TS time.Time
	T  civil.Time
	D  civil.Date
	DT civil.DateTime
}

func TestStructLoader(t *testing.T) {
	var ts1 testStruct1
	if err := load(&ts1, schema2, testValues); err != nil {
		t.Fatal(err)
	}
	// Note: the schema field named "s" gets matched to the exported struct
	// field "S", not the unexported "s".
	want := &testStruct1{
		B:      true,
		I:      7,
		F:      3.14,
		times:  times{TS: testTimestamp, T: testTime, D: testDate, DT: testDateTime},
		S:      "x",
		S2:     "y",
		By:     []byte{1, 2, 3},
		Nested: nested{NestS: "nested", NestI: 17},
		Tagged: "z",
	}
	if !testutil.Equal(&ts1, want, cmp.AllowUnexported(testStruct1{})) {
		t.Errorf("got %+v, want %+v", pretty.Value(ts1), pretty.Value(*want))
		d, _, err := pretty.Diff(*want, ts1)
		if err == nil {
			t.Logf("diff:\n%s", d)
		}
	}

	// Test pointers to nested structs.
	type nestedPtr struct{ Nested *nested }
	var np nestedPtr
	if err := load(&np, schema2, testValues); err != nil {
		t.Fatal(err)
	}
	want2 := &nestedPtr{Nested: &nested{NestS: "nested", NestI: 17}}
	if !testutil.Equal(&np, want2) {
		t.Errorf("got %+v, want %+v", pretty.Value(np), pretty.Value(*want2))
	}

	// Existing values should be reused.
	nst := &nested{NestS: "x", NestI: -10}
	np = nestedPtr{Nested: nst}
	if err := load(&np, schema2, testValues); err != nil {
		t.Fatal(err)
	}
	if !testutil.Equal(&np, want2) {
		t.Errorf("got %+v, want %+v", pretty.Value(np), pretty.Value(*want2))
	}
	if np.Nested != nst {
		t.Error("nested struct pointers not equal")
	}
}

type repStruct struct {
	Nums      []int
	ShortNums [2]int // to test truncation
	LongNums  [5]int // to test padding with zeroes
	Nested    []*nested
}

var (
	repSchema = Schema{
		{Name: "nums", Type: IntegerFieldType, Repeated: true},
		{Name: "shortNums", Type: IntegerFieldType, Repeated: true},
		{Name: "longNums", Type: IntegerFieldType, Repeated: true},
		{Name: "nested", Type: RecordFieldType, Repeated: true, Schema: Schema{
			{Name: "nestS", Type: StringFieldType},
			{Name: "nestI", Type: IntegerFieldType},
		}},
	}
	v123      = []Value{int64(1), int64(2), int64(3)}
	repValues = []Value{v123, v123, v123,
		[]Value{
			[]Value{"x", int64(1)},
			[]Value{"y", int64(2)},
		},
	}
)

func TestStructLoaderRepeated(t *testing.T) {
	var r1 repStruct
	if err := load(&r1, repSchema, repValues); err != nil {
		t.Fatal(err)
	}
	want := repStruct{
		Nums:      []int{1, 2, 3},
		ShortNums: [...]int{1, 2}, // extra values discarded
		LongNums:  [...]int{1, 2, 3, 0, 0},
		Nested:    []*nested{{"x", 1}, {"y", 2}},
	}
	if !testutil.Equal(r1, want) {
		t.Errorf("got %+v, want %+v", pretty.Value(r1), pretty.Value(want))
	}

	r2 := repStruct{
		Nums:     []int{-1, -2, -3, -4, -5},    // truncated to zero and appended to
		LongNums: [...]int{-1, -2, -3, -4, -5}, // unset elements are zeroed
	}
	if err := load(&r2, repSchema, repValues); err != nil {
		t.Fatal(err)
	}
	if !testutil.Equal(r2, want) {
		t.Errorf("got %+v, want %+v", pretty.Value(r2), pretty.Value(want))
	}
	if got, want := cap(r2.Nums), 5; got != want {
		t.Errorf("cap(r2.Nums) = %d, want %d", got, want)
	}

	// Short slice case.
	r3 := repStruct{Nums: []int{-1}}
	if err := load(&r3, repSchema, repValues); err != nil {
		t.Fatal(err)
	}
	if !testutil.Equal(r3, want) {
		t.Errorf("got %+v, want %+v", pretty.Value(r3), pretty.Value(want))
	}
	if got, want := cap(r3.Nums), 3; got != want {
		t.Errorf("cap(r3.Nums) = %d, want %d", got, want)
	}

}

func TestStructLoaderOverflow(t *testing.T) {
	type S struct {
		I int16
		F float32
	}
	schema := Schema{
		{Name: "I", Type: IntegerFieldType},
		{Name: "F", Type: FloatFieldType},
	}
	var s S
	if err := load(&s, schema, []Value{int64(math.MaxInt16 + 1), 0}); err == nil {
		t.Error("int: got nil, want error")
	}
	if err := load(&s, schema, []Value{int64(0), math.MaxFloat32 * 2}); err == nil {
		t.Error("float: got nil, want error")
	}
}

func TestStructLoaderFieldOverlap(t *testing.T) {
	// It's OK if the struct has fields that the schema does not, and vice versa.
	type S1 struct {
		I int
		X [][]int // not in the schema; does not even correspond to a valid BigQuery type
		// many schema fields missing
	}
	var s1 S1
	if err := load(&s1, schema2, testValues); err != nil {
		t.Fatal(err)
	}
	want1 := S1{I: 7}
	if !testutil.Equal(s1, want1) {
		t.Errorf("got %+v, want %+v", pretty.Value(s1), pretty.Value(want1))
	}

	// It's even valid to have no overlapping fields at all.
	type S2 struct{ Z int }

	var s2 S2
	if err := load(&s2, schema2, testValues); err != nil {
		t.Fatal(err)
	}
	want2 := S2{}
	if !testutil.Equal(s2, want2) {
		t.Errorf("got %+v, want %+v", pretty.Value(s2), pretty.Value(want2))
	}
}

func TestStructLoaderErrors(t *testing.T) {
	check := func(sp interface{}) {
		var sl structLoader
		err := sl.set(sp, schema2)
		if err == nil {
			t.Errorf("%T: got nil, want error", sp)
		}
	}

	type bad1 struct{ F int32 } // wrong type for FLOAT column
	check(&bad1{})

	type bad2 struct{ I uint } // unsupported integer type
	check(&bad2{})

	type bad3 struct {
		I int `bigquery:"@"`
	} // bad field name
	check(&bad3{})

	type bad4 struct{ Nested int } // non-struct for nested field
	check(&bad4{})

	type bad5 struct{ Nested struct{ NestS int } } // bad nested struct
	check(&bad5{})

	bad6 := &struct{ Nums int }{} // non-slice for repeated field
	sl := structLoader{}
	err := sl.set(bad6, repSchema)
	if err == nil {
		t.Errorf("%T: got nil, want error", bad6)
	}

	// sl.set's error is sticky, with even good input.
	err2 := sl.set(&repStruct{}, repSchema)
	if err2 != err {
		t.Errorf("%v != %v, expected equal", err2, err)
	}
	// sl.Load is similarly sticky
	err2 = sl.Load(nil, nil)
	if err2 != err {
		t.Errorf("%v != %v, expected equal", err2, err)
	}

	// Null values.
	schema := Schema{
		{Name: "i", Type: IntegerFieldType},
		{Name: "f", Type: FloatFieldType},
		{Name: "b", Type: BooleanFieldType},
		{Name: "s", Type: StringFieldType},
		{Name: "by", Type: BytesFieldType},
		{Name: "d", Type: DateFieldType},
	}
	type s struct {
		I  int
		F  float64
		B  bool
		S  string
		By []byte
		D  civil.Date
	}
	vals := []Value{int64(0), 0.0, false, "", []byte{}, testDate}
	if err := load(&s{}, schema, vals); err != nil {
		t.Fatal(err)
	}
	for i, e := range vals {
		vals[i] = nil
		got := load(&s{}, schema, vals)
		if got != errNoNulls {
			t.Errorf("#%d: got %v, want %v", i, got, errNoNulls)
		}
		vals[i] = e
	}

	// Using more than one struct type with the same structLoader.
	type different struct {
		B bool
		I int
		times
		S    string
		s    string
		Nums []int
	}

	sl = structLoader{}
	if err := sl.set(&testStruct1{}, schema2); err != nil {
		t.Fatal(err)
	}
	err = sl.set(&different{}, schema2)
	if err == nil {
		t.Error("different struct types: got nil, want error")
	}
}

func load(pval interface{}, schema Schema, vals []Value) error {
	var sl structLoader
	if err := sl.set(pval, schema); err != nil {
		return err
	}
	return sl.Load(vals, nil)
}

func BenchmarkStructLoader_NoCompile(b *testing.B) {
	benchmarkStructLoader(b, false)
}

func BenchmarkStructLoader_Compile(b *testing.B) {
	benchmarkStructLoader(b, true)
}

func benchmarkStructLoader(b *testing.B, compile bool) {
	var ts1 testStruct1
	for i := 0; i < b.N; i++ {
		var sl structLoader
		for j := 0; j < 10; j++ {
			if err := load(&ts1, schema2, testValues); err != nil {
				b.Fatal(err)
			}
			if !compile {
				sl.typ = nil
			}
		}
	}
}
