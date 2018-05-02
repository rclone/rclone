// Copyright 2016 Google Inc. All Rights Reserved.
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

package datastore

import (
	"testing"
	"time"

	"cloud.google.com/go/internal/testutil"

	pb "google.golang.org/genproto/googleapis/datastore/v1"
)

func TestInterfaceToProtoNil(t *testing.T) {
	// A nil *Key, or a nil value of any other pointer type, should convert to a NullValue.
	for _, in := range []interface{}{
		(*Key)(nil),
		(*int)(nil),
		(*string)(nil),
		(*bool)(nil),
		(*float64)(nil),
		(*GeoPoint)(nil),
		(*time.Time)(nil),
	} {
		got, err := interfaceToProto(in, false)
		if err != nil {
			t.Fatalf("%T: %v", in, err)
		}
		_, ok := got.ValueType.(*pb.Value_NullValue)
		if !ok {
			t.Errorf("%T: got: %T\nwant: %T", in, got.ValueType, &pb.Value_NullValue{})
		}
	}
}

func TestSaveEntityNested(t *testing.T) {
	type WithKey struct {
		X string
		I int
		K *Key `datastore:"__key__"`
	}

	type NestedWithKey struct {
		Y string
		N WithKey
	}

	type WithoutKey struct {
		X string
		I int
	}

	type NestedWithoutKey struct {
		Y string
		N WithoutKey
	}

	type a struct {
		S string
	}

	type UnexpAnonym struct {
		a
	}

	testCases := []struct {
		desc string
		src  interface{}
		key  *Key
		want *pb.Entity
	}{
		{
			desc: "nested entity with key",
			src: &NestedWithKey{
				Y: "yyy",
				N: WithKey{
					X: "two",
					I: 2,
					K: testKey1a,
				},
			},
			key: testKey0,
			want: &pb.Entity{
				Key: keyToProto(testKey0),
				Properties: map[string]*pb.Value{
					"Y": {ValueType: &pb.Value_StringValue{StringValue: "yyy"}},
					"N": {ValueType: &pb.Value_EntityValue{
						EntityValue: &pb.Entity{
							Key: keyToProto(testKey1a),
							Properties: map[string]*pb.Value{
								"X": {ValueType: &pb.Value_StringValue{StringValue: "two"}},
								"I": {ValueType: &pb.Value_IntegerValue{IntegerValue: 2}},
							},
						},
					}},
				},
			},
		},
		{
			desc: "nested entity with incomplete key",
			src: &NestedWithKey{
				Y: "yyy",
				N: WithKey{
					X: "two",
					I: 2,
					K: incompleteKey,
				},
			},
			key: testKey0,
			want: &pb.Entity{
				Key: keyToProto(testKey0),
				Properties: map[string]*pb.Value{
					"Y": {ValueType: &pb.Value_StringValue{StringValue: "yyy"}},
					"N": {ValueType: &pb.Value_EntityValue{
						EntityValue: &pb.Entity{
							Key: keyToProto(incompleteKey),
							Properties: map[string]*pb.Value{
								"X": {ValueType: &pb.Value_StringValue{StringValue: "two"}},
								"I": {ValueType: &pb.Value_IntegerValue{IntegerValue: 2}},
							},
						},
					}},
				},
			},
		},
		{
			desc: "nested entity without key",
			src: &NestedWithoutKey{
				Y: "yyy",
				N: WithoutKey{
					X: "two",
					I: 2,
				},
			},
			key: testKey0,
			want: &pb.Entity{
				Key: keyToProto(testKey0),
				Properties: map[string]*pb.Value{
					"Y": {ValueType: &pb.Value_StringValue{StringValue: "yyy"}},
					"N": {ValueType: &pb.Value_EntityValue{
						EntityValue: &pb.Entity{
							Properties: map[string]*pb.Value{
								"X": {ValueType: &pb.Value_StringValue{StringValue: "two"}},
								"I": {ValueType: &pb.Value_IntegerValue{IntegerValue: 2}},
							},
						},
					}},
				},
			},
		},
		{
			desc: "key at top level",
			src: &WithKey{
				X: "three",
				I: 3,
				K: testKey0,
			},
			key: testKey0,
			want: &pb.Entity{
				Key: keyToProto(testKey0),
				Properties: map[string]*pb.Value{
					"X": {ValueType: &pb.Value_StringValue{StringValue: "three"}},
					"I": {ValueType: &pb.Value_IntegerValue{IntegerValue: 3}},
				},
			},
		},
		{
			desc: "nested unexported anonymous struct field",
			src: &UnexpAnonym{
				a{S: "hello"},
			},
			key: testKey0,
			want: &pb.Entity{
				Key: keyToProto(testKey0),
				Properties: map[string]*pb.Value{
					"S": {ValueType: &pb.Value_StringValue{StringValue: "hello"}},
				},
			},
		},
	}

	for _, tc := range testCases {
		got, err := saveEntity(tc.key, tc.src)
		if err != nil {
			t.Errorf("saveEntity: %s: %v", tc.desc, err)
			continue
		}

		if !testutil.Equal(tc.want, got) {
			t.Errorf("%s: compare:\ngot:  %#v\nwant: %#v", tc.desc, got, tc.want)
		}
	}
}

func TestSavePointers(t *testing.T) {
	for _, test := range []struct {
		desc string
		in   interface{}
		want []Property
	}{
		{
			desc: "nil pointers save as nil-valued properties",
			in:   &Pointers{},
			want: []Property{
				Property{Name: "Pi", Value: nil},
				Property{Name: "Ps", Value: nil},
				Property{Name: "Pb", Value: nil},
				Property{Name: "Pf", Value: nil},
				Property{Name: "Pg", Value: nil},
				Property{Name: "Pt", Value: nil},
			},
		},
		{
			desc: "nil omitempty pointers not saved",
			in:   &PointersOmitEmpty{},
			want: []Property(nil),
		},
		{
			desc: "non-nil zero-valued pointers save as zero values",
			in:   populatedPointers(),
			want: []Property{
				Property{Name: "Pi", Value: int64(0)},
				Property{Name: "Ps", Value: ""},
				Property{Name: "Pb", Value: false},
				Property{Name: "Pf", Value: 0.0},
				Property{Name: "Pg", Value: GeoPoint{}},
				Property{Name: "Pt", Value: time.Time{}},
			},
		},
		{
			desc: "non-nil non-zero-valued pointers save as the appropriate values",
			in: func() *Pointers {
				p := populatedPointers()
				*p.Pi = 1
				*p.Ps = "x"
				*p.Pb = true
				*p.Pf = 3.14
				*p.Pg = GeoPoint{Lat: 1, Lng: 2}
				*p.Pt = time.Unix(100, 0)
				return p
			}(),
			want: []Property{
				Property{Name: "Pi", Value: int64(1)},
				Property{Name: "Ps", Value: "x"},
				Property{Name: "Pb", Value: true},
				Property{Name: "Pf", Value: 3.14},
				Property{Name: "Pg", Value: GeoPoint{Lat: 1, Lng: 2}},
				Property{Name: "Pt", Value: time.Unix(100, 0)},
			},
		},
	} {
		got, err := SaveStruct(test.in)
		if err != nil {
			t.Fatalf("%s: %v", test.desc, err)
		}
		if !testutil.Equal(got, test.want) {
			t.Errorf("%s\ngot  %#v\nwant %#v\n", test.desc, got, test.want)
		}
	}
}

func TestSaveEmptySlice(t *testing.T) {
	// Zero-length slice fields are not saved.
	for _, slice := range [][]string{nil, {}} {
		got, err := SaveStruct(&struct{ S []string }{S: slice})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Errorf("%#v: got %d properties, wanted zero", slice, len(got))
		}
	}
}
